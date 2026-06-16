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

// Backuper handles MySQL backup operations
type Backuper struct {
	inspector *Inspector
}

// NewBackuper creates a new MySQL backuper
func NewBackuper() *Backuper {
	return &Backuper{
		inspector: NewInspector(),
	}
}

// Backup performs MySQL backup
func (b *Backuper) Backup(ctx context.Context, job engine.JobConfig, target engine.BackupTarget) (*engine.BackupResult, error) {
	startTime := time.Now()

	result := &engine.BackupResult{
		BackupID:  target.BackupID,
		BackupDir: target.BackupDir,
		Databases: target.Databases,
		StartedAt: startTime.Format(time.RFC3339),
		Status:    "success",
	}

	// Create backup directory
	if err := os.MkdirAll(target.BackupDir, 0755); err != nil {
		return nil, fmt.Errorf("creating backup directory: %w", err)
	}

	// Backup each database
	for _, db := range target.Databases {
		artifact, err := b.backupDatabase(ctx, job, target, db)
		if err != nil {
			result.Status = "failed"
			result.Error = err
			result.FinishedAt = time.Now().Format(time.RFC3339)
			result.DurationSec = int64(time.Since(startTime).Seconds())
			return result, err
		}
		result.Artifacts = append(result.Artifacts, *artifact)
	}

	result.FinishedAt = time.Now().Format(time.RFC3339)
	result.DurationSec = int64(time.Since(startTime).Seconds())

	return result, nil
}

// backupDatabase backs up a single database
func (b *Backuper) backupDatabase(ctx context.Context, job engine.JobConfig, target engine.BackupTarget, database string) (*engine.Artifact, error) {
	// Build output file path
	outputFile := filepath.Join(target.BackupDir, database+".sql.zst")

	// Build mysqldump command
	dumpArgs := b.buildDumpArgs(job, database)

	// Build compression command
	compressArgs := []string{"-T0", "-3", "-o", outputFile}

	// Create pipeline: mysqldump | zstd
	// For now, we'll use a simple approach
	dumpCmd := exec.CommandContext(ctx, "mysqldump", dumpArgs...)
	compressCmd := exec.CommandContext(ctx, "zstd", compressArgs...)

	// Set environment for password
	dumpCmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", job.Password))

	// Create pipe
	pipe, err := dumpCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}
	compressCmd.Stdin = pipe

	// Capture stderr
	dumpStderr, _ := dumpCmd.StderrPipe()
	compressStderr, _ := compressCmd.StderrPipe()

	// Start commands
	if err := compressCmd.Start(); err != nil {
		return nil, fmt.Errorf("starting zstd: %w", err)
	}
	if err := dumpCmd.Start(); err != nil {
		return nil, fmt.Errorf("starting mysqldump: %w", err)
	}

	// Wait for mysqldump to finish
	if err := dumpCmd.Wait(); err != nil {
		// Read stderr for error message
		buf := make([]byte, 1024)
		n, _ := dumpStderr.Read(buf)
		return nil, fmt.Errorf("mysqldump failed: %s", string(buf[:n]))
	}

	// Wait for zstd to finish
	if err := compressCmd.Wait(); err != nil {
		buf := make([]byte, 1024)
		n, _ := compressStderr.Read(buf)
		return nil, fmt.Errorf("zstd failed: %s", string(buf[:n]))
	}

	// Get file info
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	return &engine.Artifact{
		Database:    database,
		File:        database + ".sql.zst",
		Path:        outputFile,
		SizeBytes:   fileInfo.Size(),
		Compression: "zstd",
	}, nil
}

// buildDumpArgs builds mysqldump arguments
func (b *Backuper) buildDumpArgs(job engine.JobConfig, database string) []string {
	args := []string{
		"-h", job.Host,
		"-P", fmt.Sprintf("%d", job.Port),
		"-u", job.User,
		"--single-transaction",
		"--quick",
		"--routines",
		"--events",
		"--triggers",
		"--hex-blob",
		"--set-gtid-purged=OFF",
		"--column-statistics=0",
		database,
	}

	// Add options from job config
	if opts, ok := job.Options["single_transaction"]; ok && !opts.(bool) {
		args = removeArg(args, "--single-transaction")
	}
	if opts, ok := job.Options["quick"]; ok && !opts.(bool) {
		args = removeArg(args, "--quick")
	}
	if opts, ok := job.Options["routines"]; ok && !opts.(bool) {
		args = removeArg(args, "--routines")
	}
	if opts, ok := job.Options["events"]; ok && !opts.(bool) {
		args = removeArg(args, "--events")
	}
	if opts, ok := job.Options["triggers"]; ok && !opts.(bool) {
		args = removeArg(args, "--triggers")
	}

	return args
}

// removeArg removes an argument from the list
func removeArg(args []string, arg string) []string {
	result := make([]string, 0, len(args))
	for _, a := range args {
		if a != arg {
			result = append(result, a)
		}
	}
	return result
}