package postgresql

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

// Backuper handles PostgreSQL backup operations
type Backuper struct {
	inspector *Inspector
}

// NewBackuper creates a new PostgreSQL backuper
func NewBackuper() *Backuper {
	return &Backuper{
		inspector: NewInspector(),
	}
}

// Backup performs PostgreSQL backup
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

	// Build compressor from target compression config
	comp := compress.NewCompressor(target.Compression.Type, target.Compression.Level)
	ext := comp.Extension()

	// Backup globals if configured
	includeGlobals := false
	if v, ok := job.Options["include_globals"]; ok {
		if b, ok := v.(bool); ok {
			includeGlobals = b
		}
	}
	if includeGlobals {
		if err := b.backupGlobals(ctx, job, target); err != nil {
			result.Status = "failed"
			result.Error = err
			result.FinishedAt = time.Now().Format(time.RFC3339)
			result.DurationSec = int64(time.Since(startTime).Seconds())
			return result, err
		}
		// Add globals artifact
		globalsFile := filepath.Join(target.BackupDir, "globals.sql"+ext)
		if fileInfo, err := os.Stat(globalsFile); err == nil {
			compressionType := "none"
			if target.Compression.Enabled {
				compressionType = string(comp.Type)
			}
			result.Artifacts = append(result.Artifacts, engine.Artifact{
				Database:    "__globals__",
				File:        "globals.sql" + ext,
				Path:        globalsFile,
				SizeBytes:   fileInfo.Size(),
				Compression: compressionType,
			})
		}
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

// backupGlobals backs up global objects (roles, tablespaces)
func (b *Backuper) backupGlobals(ctx context.Context, job engine.JobConfig, target engine.BackupTarget) error {
	comp := compress.NewCompressor(target.Compression.Type, target.Compression.Level)
	ext := comp.Extension()
	outputFile := filepath.Join(target.BackupDir, "globals.sql"+ext)

	// Build pg_dumpall command for globals
	dumpArgs := []string{
		"-h", job.Host,
		"-p", fmt.Sprintf("%d", job.Port),
		"-U", job.User,
		"--globals-only",
	}

	dumpCmd := exec.CommandContext(ctx, "pg_dumpall", dumpArgs...)
	dumpCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", job.Password))
	dumpStderr, _ := dumpCmd.StderrPipe()

	if target.Compression.Enabled && comp.Type != compress.CompressionNone {
		cmdName, cmdArgs := comp.CompressCommand()
		compressArgs := append(cmdArgs, "-o", outputFile)
		compressCmd := exec.CommandContext(ctx, cmdName, compressArgs...)

		pipe, err := dumpCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("creating pipe: %w", err)
		}
		compressCmd.Stdin = pipe
		compressStderr, _ := compressCmd.StderrPipe()

		if err := compressCmd.Start(); err != nil {
			return fmt.Errorf("starting compressor: %w", err)
		}
		if err := dumpCmd.Start(); err != nil {
			return fmt.Errorf("starting pg_dumpall: %w", err)
		}

		if err := dumpCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return fmt.Errorf("pg_dumpall failed: %s", string(buf[:n]))
		}
		if err := compressCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := compressStderr.Read(buf)
			return fmt.Errorf("compressor failed: %s", string(buf[:n]))
		}
	} else {
		outFile, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		dumpCmd.Stdout = outFile

		if err := dumpCmd.Run(); err != nil {
			outFile.Close()
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return fmt.Errorf("pg_dumpall failed: %s", string(buf[:n]))
		}
		outFile.Close()
	}

	return nil
}

// backupDatabase backs up a single database
func (b *Backuper) backupDatabase(ctx context.Context, job engine.JobConfig, target engine.BackupTarget, database string) (*engine.Artifact, error) {
	comp := compress.NewCompressor(target.Compression.Type, target.Compression.Level)
	ext := comp.Extension()

	// Build output file path
	outputFile := filepath.Join(target.BackupDir, database+".dump"+ext)

	// Build pg_dump command
	dumpArgs := b.buildDumpArgs(job, database)

	dumpCmd := exec.CommandContext(ctx, "pg_dump", dumpArgs...)
	dumpCmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", job.Password))
	dumpStderr, _ := dumpCmd.StderrPipe()

	if target.Compression.Enabled && comp.Type != compress.CompressionNone {
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
			return nil, fmt.Errorf("starting pg_dump: %w", err)
		}

		if err := dumpCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return nil, fmt.Errorf("pg_dump failed: %s", string(buf[:n]))
		}
		if err := compressCmd.Wait(); err != nil {
			buf := make([]byte, 1024)
			n, _ := compressStderr.Read(buf)
			return nil, fmt.Errorf("compressor failed: %s", string(buf[:n]))
		}
	} else {
		outFile, err := os.Create(outputFile)
		if err != nil {
			return nil, fmt.Errorf("creating output file: %w", err)
		}
		dumpCmd.Stdout = outFile

		if err := dumpCmd.Run(); err != nil {
			outFile.Close()
			buf := make([]byte, 1024)
			n, _ := dumpStderr.Read(buf)
			return nil, fmt.Errorf("pg_dump failed: %s", string(buf[:n]))
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
		File:        database + ".dump" + ext,
		Path:        outputFile,
		SizeBytes:   fileInfo.Size(),
		Compression: compressionType,
	}, nil
}

// buildDumpArgs builds pg_dump arguments
func (b *Backuper) buildDumpArgs(job engine.JobConfig, database string) []string {
	args := []string{
		"-h", job.Host,
		"-p", fmt.Sprintf("%d", job.Port),
		"-U", job.User,
		"-d", database,
		"-F", "c",  // custom format
		"-Z", "0",  // no compression (we'll use zstd)
		"--no-owner",
		"--no-privileges",
	}

	// Add options from job config
	if opts, ok := job.Options["dump_format"]; ok {
		format := opts.(string)
		switch format {
		case "plain":
			args = replaceArg(args, "-F", "c", "-F", "p")
		case "tar":
			args = replaceArg(args, "-F", "c", "-F", "t")
		case "directory":
			args = replaceArg(args, "-F", "c", "-F", "d")
		}
	}
	if opts, ok := job.Options["no_owner"]; ok && !opts.(bool) {
		args = removeArg(args, "--no-owner")
	}
	if opts, ok := job.Options["no_privileges"]; ok && !opts.(bool) {
		args = removeArg(args, "--no-privileges")
	}

	return args
}

// replaceArg replaces an argument value in the list
func replaceArg(args []string, oldFlag, oldVal, newFlag, newVal string) []string {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) && args[i] == oldFlag && args[i+1] == oldVal {
			result = append(result, newFlag, newVal)
			i++ // skip the value
		} else {
			result = append(result, args[i])
		}
	}
	return result
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
