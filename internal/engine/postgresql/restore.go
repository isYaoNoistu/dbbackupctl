package postgresql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/checksum"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
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

	// Get connection info from job config
	host := "127.0.0.1"
	port := 5432
	user := "postgres"
	if opt.JobConfig.Host != "" {
		host = opt.JobConfig.Host
	}
	if opt.JobConfig.Port != 0 {
		port = opt.JobConfig.Port
	}
	if opt.JobConfig.User != "" {
		user = opt.JobConfig.User
	}

	// Verify checksum and build commands
	for _, a := range m.Artifacts {
		if a.Checksum != "" {
			ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
			if err != nil {
				plan.ChecksumOK = false
			} else if !ok {
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

		// Build restore command
		if a.Database == "__globals__" {
			cmd := fmt.Sprintf("zstd -dc %s | psql -h %s -p %d -U %s",
				a.Path, host, port, user)
			plan.Commands = append(plan.Commands, cmd)
		} else {
			dumpFormat := opt.JobConfig.Options["dump_format"]
			if dumpFormat == "custom" {
				cmd := fmt.Sprintf("pg_restore -h %s -p %d -U %s -d %s %s",
					host, port, user, opt.TargetDB, a.Path)
				plan.Commands = append(plan.Commands, cmd)
			} else {
				cmd := fmt.Sprintf("zstd -dc %s | psql -h %s -p %d -U %s -d %s",
					a.Path, host, port, user, opt.TargetDB)
				plan.Commands = append(plan.Commands, cmd)
			}
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
	port := 5432
	user := "postgres"
	password := ""
	sslMode := "disable"
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
	if v, ok := opt.JobConfig.Options["sslmode"].(string); ok && v != "" {
		sslMode = v
	}

	// Create target database
	if err := r.createDatabase(ctx, host, port, user, password, sslMode, opt.TargetDB); err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	// Restore each artifact
	for _, a := range m.Artifacts {
		if a.Database == "__globals__" {
			// Restore globals
			if err := r.restoreGlobals(ctx, a.Path, host, port, user, password, sslMode); err != nil {
				result.Status = "failed"
				result.Error = err
				result.FinishedAt = time.Now().Format(time.RFC3339)
				result.DurationSec = int64(time.Since(startTime).Seconds())
				return result, err
			}
			continue
		}

		dumpFormat := opt.JobConfig.Options["dump_format"]
		if dumpFormat == "custom" {
			if err := r.restoreCustom(ctx, a.Path, host, port, user, password, sslMode, opt.TargetDB); err != nil {
				result.Status = "failed"
				result.Error = err
				result.FinishedAt = time.Now().Format(time.RFC3339)
				result.DurationSec = int64(time.Since(startTime).Seconds())
				return result, err
			}
		} else {
			if err := r.restorePlain(ctx, a.Path, host, port, user, password, sslMode, opt.TargetDB); err != nil {
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
func (r *Restorer) createDatabase(ctx context.Context, host string, port int, user, password, sslMode, targetDB string) error {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-c", fmt.Sprintf("CREATE DATABASE \"%s\";", targetDB),
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore "already exists" error
		if string(output) == "" || contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("creating database: %s", string(output))
	}

	return nil
}

// restoreGlobals restores global objects
func (r *Restorer) restoreGlobals(ctx context.Context, sourceFile, host string, port int, user, password, sslMode string) error {
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	psqlCmd := exec.CommandContext(ctx, "psql",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	psqlCmd.Env = env

	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	psqlCmd.Stdin = pipe

	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("starting psql: %w", err)
	}
	if err := zstdCmd.Start(); err != nil {
		return fmt.Errorf("starting zstd: %w", err)
	}

	if err := zstdCmd.Wait(); err != nil {
		return fmt.Errorf("zstd failed: %w", err)
	}
	if err := psqlCmd.Wait(); err != nil {
		return fmt.Errorf("psql failed: %w", err)
	}

	return nil
}

// restoreCustom restores a custom format backup
func (r *Restorer) restoreCustom(ctx context.Context, sourceFile, host string, port int, user, password, sslMode, targetDB string) error {
	cmd := exec.CommandContext(ctx, "pg_restore",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-d", targetDB,
		"--no-owner",
		"--no-privileges",
		sourceFile,
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %s", string(output))
	}

	return nil
}

// restorePlain restores a plain SQL backup
func (r *Restorer) restorePlain(ctx context.Context, sourceFile, host string, port int, user, password, sslMode, targetDB string) error {
	zstdCmd := exec.CommandContext(ctx, "zstd", "-dc", sourceFile)
	psqlCmd := exec.CommandContext(ctx, "psql",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-d", targetDB,
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	psqlCmd.Env = env

	pipe, err := zstdCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	psqlCmd.Stdin = pipe

	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("starting psql: %w", err)
	}
	if err := zstdCmd.Start(); err != nil {
		return fmt.Errorf("starting zstd: %w", err)
	}

	if err := zstdCmd.Wait(); err != nil {
		return fmt.Errorf("zstd failed: %w", err)
	}
	if err := psqlCmd.Wait(); err != nil {
		return fmt.Errorf("psql failed: %w", err)
	}

	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}