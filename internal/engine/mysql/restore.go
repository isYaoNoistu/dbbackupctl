package mysql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dbbackupctl/dbbackupctl/internal/engine"
)

// Restorer handles MySQL restore operations
type Restorer struct {
	inspector *Inspector
}

// NewRestorer creates a new MySQL restorer
func NewRestorer() *Restorer {
	return &Restorer{
		inspector: NewInspector(),
	}
}

// RestorePlan generates a restore plan without executing
func (r *Restorer) RestorePlan(ctx context.Context, record engine.BackupRecord, opt engine.RestoreOptions) (*engine.RestorePlan, error) {
	// Read manifest to get artifact information
	manifestPath := filepath.Join(record.BackupDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("manifest not found: %s", manifestPath)
	}

	// For now, return a basic plan
	plan := &engine.RestorePlan{
		BackupID:  record.BackupID,
		TargetDB:  opt.TargetDB,
		BackupDir: record.BackupDir,
		ChecksumOK: true, // TODO: Implement checksum verification
	}

	// Build restore commands
	// For each .sql.zst file in the backup directory
	entries, err := os.ReadDir(record.BackupDir)
	if err != nil {
		return nil, fmt.Errorf("reading backup directory: %w", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".zst" && entry.Name() != "globals.sql.zst" {
			sourceFile := filepath.Join(record.BackupDir, entry.Name())
			plan.Artifacts = append(plan.Artifacts, engine.Artifact{
				File: entry.Name(),
				Path: sourceFile,
			})

			// Build command: zstd -dc file.sql.zst | mysql -h ... -u ... -p target_db
			cmd := fmt.Sprintf("zstd -dc %s | mysql -h %s -P %d -u %s -p %s",
				sourceFile, "127.0.0.1", 3306, "root", opt.TargetDB)
			plan.Commands = append(plan.Commands, cmd)
		}
	}

	return plan, nil
}

// Restore performs the restore operation
func (r *Restorer) Restore(ctx context.Context, record engine.BackupRecord, opt engine.RestoreOptions) (*engine.RestoreResult, error) {
	startTime := time.Now()

	result := &engine.RestoreResult{
		BackupID:  record.BackupID,
		TargetDB:  opt.TargetDB,
		StartedAt: startTime.Format(time.RFC3339),
		Status:    "success",
	}

	// First, create the target database
	if err := r.createDatabase(ctx, opt); err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	// Restore each artifact
	entries, err := os.ReadDir(record.BackupDir)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".zst" && entry.Name() != "globals.sql.zst" {
			sourceFile := filepath.Join(record.BackupDir, entry.Name())
			if err := r.restoreDatabase(ctx, sourceFile, opt); err != nil {
				result.Status = "failed"
				result.Error = err
				result.FinishedAt = time.Now().Format(time.RFC3339)
				result.DurationSec = int64(time.Since(startTime).Seconds())
				return result, err
			}
		}
	}

	result.FinishedAt = time.Now().Format(time.RFC3339)
	result.DurationSec = int64(time.Since(startTime).Seconds())

	return result, nil
}

// createDatabase creates the target database
func (r *Restorer) createDatabase(ctx context.Context, opt engine.RestoreOptions) error {
	// Build CREATE DATABASE command
	createCmd := exec.CommandContext(ctx, "mysql",
		"-h", "127.0.0.1",
		"-P", "3306",
		"-u", "root",
		"-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4;", opt.TargetDB),
	)

	// Set password from environment
	createCmd.Env = append(os.Environ(), "MYSQL_PWD="+os.Getenv("MYSQL_PROD_RESTORE_PASSWORD"))

	// Capture stderr
	stderr, _ := createCmd.StderrPipe()

	if err := createCmd.Run(); err != nil {
		buf := make([]byte, 1024)
		n, _ := stderr.Read(buf)
		return fmt.Errorf("creating database: %s", string(buf[:n]))
	}

	return nil
}

// restoreDatabase restores a single database
func (r *Restorer) restoreDatabase(ctx context.Context, sourceFile string, opt engine.RestoreOptions) error {
	// Build pipeline: zstd -dc file.sql.zst | mysql -h ... -u ... target_db
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	mysqlCmd := exec.CommandContext(ctx, "mysql",
		"-h", "127.0.0.1",
		"-P", "3306",
		"-u", "root",
		opt.TargetDB,
	)

	// Set password from environment
	mysqlCmd.Env = append(os.Environ(), "MYSQL_PWD="+os.Getenv("MYSQL_PROD_RESTORE_PASSWORD"))

	// Create pipe
	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	mysqlCmd.Stdin = pipe

	// Capture stderr
	zstdStderr, _ := zstdCmd.StderrPipe()
	mysqlStderr, _ := mysqlCmd.StderrPipe()

	// Start commands
	if err := mysqlCmd.Start(); err != nil {
		return fmt.Errorf("starting mysql: %w", err)
	}
	if err := zstdCmd.Start(); err != nil {
		return fmt.Errorf("starting zstd: %w", err)
	}

	// Wait for zstd to finish
	if err := zstdCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := zstdStderr.Read(buf)
		return fmt.Errorf("zstd failed: %s", string(buf[:n]))
	}

	// Wait for mysql to finish
	if err := mysqlCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := mysqlStderr.Read(buf)
		return fmt.Errorf("mysql failed: %s", string(buf[:n]))
	}

	return nil
}