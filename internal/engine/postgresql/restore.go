package postgresql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dbbackupctl/dbbackupctl/internal/engine"
)

// Restorer handles PostgreSQL restore operations
type Restorer struct {
	inspector *Inspector
}

// NewRestorer creates a new PostgreSQL restorer
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
	entries, err := os.ReadDir(record.BackupDir)
	if err != nil {
		return nil, fmt.Errorf("reading backup directory: %w", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".zst" {
			sourceFile := filepath.Join(record.BackupDir, entry.Name())
			plan.Artifacts = append(plan.Artifacts, engine.Artifact{
				File: entry.Name(),
				Path: sourceFile,
			})

			// Build command based on file type
			var cmd string
			if entry.Name() == "globals.sql.zst" {
				// Globals: zstd -dc globals.sql.zst | psql -h ... -U ... -d target_db
				cmd = fmt.Sprintf("zstd -dc %s | psql -h %s -p %d -U %s -d %s",
					sourceFile, "127.0.0.1", 5432, "postgres", opt.TargetDB)
			} else if filepath.Ext(entry.Name()) == ".dump.zst" {
				// Custom format: zstd -dc file.dump.zst | pg_restore -h ... -U ... -d target_db
				cmd = fmt.Sprintf("zstd -dc %s | pg_restore -h %s -p %d -U %s -d %s --verbose",
					sourceFile, "127.0.0.1", 5432, "postgres", opt.TargetDB)
			} else {
				// Plain SQL: zstd -dc file.sql.zst | psql -h ... -U ... -d target_db
				cmd = fmt.Sprintf("zstd -dc %s | psql -h %s -p %d -U %s -d %s",
					sourceFile, "127.0.0.1", 5432, "postgres", opt.TargetDB)
			}
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
		if filepath.Ext(entry.Name()) == ".zst" {
			sourceFile := filepath.Join(record.BackupDir, entry.Name())
			if err := r.restoreArtifact(ctx, sourceFile, entry.Name(), opt); err != nil {
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
	// Build createdb command
	createCmd := exec.CommandContext(ctx, "createdb",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "postgres",
		opt.TargetDB,
	)

	// Set password from environment
	createCmd.Env = append(os.Environ(), "PGPASSWORD="+os.Getenv("POSTGRES_PROD_RESTORE_PASSWORD"))

	// Capture stderr
	stderr, _ := createCmd.StderrPipe()

	if err := createCmd.Run(); err != nil {
		buf := make([]byte, 1024)
		n, _ := stderr.Read(buf)
		return fmt.Errorf("creating database: %s", string(buf[:n]))
	}

	return nil
}

// restoreArtifact restores a single artifact
func (r *Restorer) restoreArtifact(ctx context.Context, sourceFile, fileName string, opt engine.RestoreOptions) error {
	if fileName == "globals.sql.zst" {
		return r.restoreGlobals(ctx, sourceFile, opt)
	}

	// Check if it's a custom format (.dump.zst) or plain SQL (.sql.zst)
	baseName := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	if filepath.Ext(baseName) == ".dump" {
		return r.restoreCustomFormat(ctx, sourceFile, opt)
	}

	return r.restorePlainSQL(ctx, sourceFile, opt)
}

// restoreGlobals restores global objects
func (r *Restorer) restoreGlobals(ctx context.Context, sourceFile string, opt engine.RestoreOptions) error {
	// Build pipeline: zstd -dc globals.sql.zst | psql -h ... -U ... -d target_db
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	psqlCmd := exec.CommandContext(ctx, "psql",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "postgres",
		"-d", opt.TargetDB,
	)

	// Set password from environment
	psqlCmd.Env = append(os.Environ(), "PGPASSWORD="+os.Getenv("POSTGRES_PROD_RESTORE_PASSWORD"))

	// Create pipe
	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	psqlCmd.Stdin = pipe

	// Capture stderr
	zstdStderr, _ := zstdCmd.StderrPipe()
	psqlStderr, _ := psqlCmd.StderrPipe()

	// Start commands
	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("starting psql: %w", err)
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

	// Wait for psql to finish
	if err := psqlCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := psqlStderr.Read(buf)
		return fmt.Errorf("psql failed: %s", string(buf[:n]))
	}

	return nil
}

// restoreCustomFormat restores custom format backup
func (r *Restorer) restoreCustomFormat(ctx context.Context, sourceFile string, opt engine.RestoreOptions) error {
	// Build pipeline: zstd -dc file.dump.zst | pg_restore -h ... -U ... -d target_db
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	restoreCmd := exec.CommandContext(ctx, "pg_restore",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "postgres",
		"-d", opt.TargetDB,
		"--verbose",
	)

	// Set password from environment
	restoreCmd.Env = append(os.Environ(), "PGPASSWORD="+os.Getenv("POSTGRES_PROD_RESTORE_PASSWORD"))

	// Create pipe
	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	restoreCmd.Stdin = pipe

	// Capture stderr
	zstdStderr, _ := zstdCmd.StderrPipe()
	restoreStderr, _ := restoreCmd.StderrPipe()

	// Start commands
	if err := restoreCmd.Start(); err != nil {
		return fmt.Errorf("starting pg_restore: %w", err)
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

	// Wait for pg_restore to finish
	if err := restoreCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := restoreStderr.Read(buf)
		return fmt.Errorf("pg_restore failed: %s", string(buf[:n]))
	}

	return nil
}

// restorePlainSQL restores plain SQL backup
func (r *Restorer) restorePlainSQL(ctx context.Context, sourceFile string, opt engine.RestoreOptions) error {
	// Build pipeline: zstd -dc file.sql.zst | psql -h ... -U ... -d target_db
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	psqlCmd := exec.CommandContext(ctx, "psql",
		"-h", "127.0.0.1",
		"-p", "5432",
		"-U", "postgres",
		"-d", opt.TargetDB,
	)

	// Set password from environment
	psqlCmd.Env = append(os.Environ(), "PGPASSWORD="+os.Getenv("POSTGRES_PROD_RESTORE_PASSWORD"))

	// Create pipe
	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	psqlCmd.Stdin = pipe

	// Capture stderr
	zstdStderr, _ := zstdCmd.StderrPipe()
	psqlStderr, _ := psqlCmd.StderrPipe()

	// Start commands
	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("starting psql: %w", err)
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

	// Wait for psql to finish
	if err := psqlCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := psqlStderr.Read(buf)
		return fmt.Errorf("psql failed: %s", string(buf[:n]))
	}

	return nil
}