package mysql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/checksum"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
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

	// Read manifest
	mw := manifest.NewWriter()
	m, err := mw.Read(record.BackupDir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	// Verify manifest status
	if m.Status != "success" {
		return nil, fmt.Errorf("backup %s failed, cannot restore", record.BackupID)
	}

	// Build plan
	plan := &engine.RestorePlan{
		BackupID:  record.BackupID,
		SourceDB:  m.Job,
		TargetDB:  opt.TargetDB,
		BackupDir: record.BackupDir,
		ChecksumOK: true,
	}

	// Verify checksum for each artifact
	for _, a := range m.Artifacts {
		if a.Checksum != "" {
			ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
			if err != nil {
				plan.ChecksumOK = false
				plan.Artifacts = append(plan.Artifacts, engine.Artifact{
					Database: a.Database,
					File:     a.File,
					Path:     a.Path,
				})
				continue
			}
			if !ok {
				plan.ChecksumOK = false
			}
		}

		plan.Artifacts = append(plan.Artifacts, engine.Artifact{
			Database:     a.Database,
			File:         a.File,
			Path:         a.Path,
			SizeBytes:    a.SizeBytes,
			Compression:  a.Compression,
			ChecksumType: a.ChecksumType,
			Checksum:     a.Checksum,
		})

		// Build restore command using job config
		host := "127.0.0.1"
		port := 3306
		user := "root"
		if opt.JobConfig.Host != "" {
			host = opt.JobConfig.Host
		}
		if opt.JobConfig.Port != 0 {
			port = opt.JobConfig.Port
		}
		if opt.JobConfig.User != "" {
			user = opt.JobConfig.User
		}

		cmd := fmt.Sprintf("zstd -dc %s | mysql -h %s -P %d -u %s %s",
			a.Path, host, port, user, opt.TargetDB)
		plan.Commands = append(plan.Commands, cmd)
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

	// Read manifest
	mw := manifest.NewWriter()
	m, err := mw.Read(record.BackupDir)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	// Verify checksum
	if opt.JobConfig.Options["skip_checksum"] != true {
		for _, a := range m.Artifacts {
			if a.Checksum != "" {
				ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
				if err != nil {
					result.Status = "failed"
					result.Error = fmt.Errorf("checksum verification failed for %s: %w", a.Path, err)
					result.FinishedAt = time.Now().Format(time.RFC3339)
					result.DurationSec = int64(time.Since(startTime).Seconds())
					return result, result.Error
				}
				if !ok {
					result.Status = "failed"
					result.Error = fmt.Errorf("checksum mismatch for %s", a.Path)
					result.FinishedAt = time.Now().Format(time.RFC3339)
					result.DurationSec = int64(time.Since(startTime).Seconds())
					return result, result.Error
				}
			}
		}
	}

	// Get connection info from job config
	host := "127.0.0.1"
	port := 3306
	user := "root"
	password := ""
	if opt.JobConfig.Host != "" {
		host = opt.JobConfig.Host
	}
	if opt.JobConfig.Port != 0 {
		port = opt.JobConfig.Port
	}
	if opt.JobConfig.User != "" {
		user = opt.JobConfig.User
	}
	if opt.JobConfig.Password != "" {
		password = opt.JobConfig.Password
	}

	// Create target database
	if err := r.createDatabase(ctx, host, port, user, password, opt.TargetDB); err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	// Restore each artifact
	for _, a := range m.Artifacts {
		if a.Database == "__globals__" {
			continue // Skip globals for MySQL
		}
		if err := r.restoreDatabase(ctx, a.Path, host, port, user, password, opt.TargetDB); err != nil {
			result.Status = "failed"
			result.Error = err
			result.FinishedAt = time.Now().Format(time.RFC3339)
			result.DurationSec = int64(time.Since(startTime).Seconds())
			return result, err
		}
	}

	result.FinishedAt = time.Now().Format(time.RFC3339)
	result.DurationSec = int64(time.Since(startTime).Seconds())

	return result, nil
}

// createDatabase creates the target database
func (r *Restorer) createDatabase(ctx context.Context, host string, port int, user, password, targetDB string) error {
	createCmd := exec.CommandContext(ctx, "mysql",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		"-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4;", targetDB),
	)

	if password != "" {
		createCmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	}

	output, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating database: %s", string(output))
	}

	return nil
}

// restoreDatabase restores a single database
func (r *Restorer) restoreDatabase(ctx context.Context, sourceFile, host string, port int, user, password, targetDB string) error {
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	mysqlCmd := exec.CommandContext(ctx, "mysql",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		targetDB,
	)

	if password != "" {
		mysqlCmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	}

	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	mysqlCmd.Stdin = pipe

	zstdStderr, _ := zstdCmd.StderrPipe()
	mysqlStderr, _ := mysqlCmd.StderrPipe()

	if err := mysqlCmd.Start(); err != nil {
		return fmt.Errorf("starting mysql: %w", err)
	}
	if err := zstdCmd.Start(); err != nil {
		return fmt.Errorf("starting zstd: %w", err)
	}

	if err := zstdCmd.Wait(); err != nil {
		buf := make([]byte, 4096)
		n, _ := zstdStderr.Read(buf)
		return fmt.Errorf("zstd failed: %s", string(buf[:n]))
	}

	if err := mysqlCmd.Wait(); err != nil {
		buf := make([]byte, 4096)
		n, _ := mysqlStderr.Read(buf)
		return fmt.Errorf("mysql failed: %s", string(buf[:n]))
	}

	return nil
}