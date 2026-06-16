package mysql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/compress"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
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
	// Build compressor from target compression config
	comp := compress.NewCompressor(target.Compression.Type, target.Compression.Level)
	ext := comp.Extension()

	// Build output file path
	outputFile := filepath.Join(target.BackupDir, database+".sql"+ext)

	// Build mysqldump command
	dumpArgs := b.buildDumpArgs(job, database)

	dumpCmd := exec.CommandContext(ctx, "mysqldump", dumpArgs...)

	// Set environment for password
	dumpCmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", job.Password))

	// Capture stderr
	dumpStderr, _ := dumpCmd.StderrPipe()

	if target.Compression.Enabled && comp.Type != compress.CompressionNone {
		// Create pipeline: mysqldump | compressor
		cmdName, cmdArgs := comp.CompressCommand()
		compressArgs := append(cmdArgs, "-o", outputFile)
		compressCmd := exec.CommandContext(ctx, cmdName, compressArgs...)

		pipe, err := dumpCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("creating pipe: %w", err)
		}
		compressCmd.Stdin = pipe
		compressStderr, _ := compressCmd.StderrPipe()

		if err := compressCmd.Start(); err != nil {
			return nil, fmt.Errorf("starting compressor: %w", err)
		}
		if err := dumpCmd.Start(); err != nil {
			return nil, fmt.Errorf("starting mysqldump: %w", err)
		}

		if err := dumpCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return nil, fmt.Errorf("mysqldump failed: %s", string(buf[:n]))
		}
		if err := compressCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := compressStderr.Read(buf)
			return nil, fmt.Errorf("compressor failed: %s", string(buf[:n]))
		}
	} else {
		// No compression - write directly to file
		outFile, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("creating output file: %w", err)
		}
		dumpCmd.Stdout = outFile

		if err := dumpCmd.Run(); err != nil {
			outFile.Close()
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return nil, fmt.Errorf("mysqldump failed: %s", string(buf[:n]))
		}
		outFile.Close()
	}

	// Get file info
	fileInfo, err := os.Stat(outputFile)
	if err != nil {
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	compressionType := "none"
	if target.Compression.Enabled {
		compressionType = string(comp.Type)
	}

	return &engine.Artifact{
		Database:    database,
		File:        database + ".sql" + ext,
		Path:        outputFile,
		SizeBytes:   fileInfo.Size(),
		Compression: compressionType,
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
