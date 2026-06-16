package postgresql

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/checksum"
	"github.com/isYaoNoistu/dbbackupctl/internal/compress"
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
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}

	// Verify manifest status
	if m.Status != "success" {
		return nil, fmt.Errorf("备份 %s 状态为失败，不能恢复", record.BackupID)
	}

	// Build plan
	plan := &engine.RestorePlan{
		BackupID:   record.BackupID,
		SourceDB:   opt.SourceDB,
		TargetDB:   opt.TargetDB,
		BackupDir:  record.BackupDir,
		ChecksumOK: true,
	}

	// Get connection info from job config
	host, port, user, _, _, err := postgreSQLRestoreConnection(opt.JobConfig)
	if err != nil {
		return nil, err
	}

	artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB, opt.IncludeGlobals)
	if err != nil {
		return nil, err
	}

	// Verify checksum and build commands
	for _, a := range artifacts {
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
		cmdName, cmdArgs := decompressCommand(a)
		if a.Database == "__globals__" {
			cmd := fmt.Sprintf("%s %s | psql -h %s -p %d -U %s -d postgres",
				cmdName, strings.Join(cmdArgs, " "), host, port, user)
			plan.Commands = append(plan.Commands, cmd)
		} else {
			dumpFormat, _ := opt.JobConfig.Options["dump_format"].(string)
			if dumpFormat == "custom" || dumpFormat == "tar" {
				if a.Compression == "none" || a.Compression == "" {
					cmd := fmt.Sprintf("pg_restore -h %s -p %d -U %s -d %s %s",
						host, port, user, opt.TargetDB, a.Path)
					plan.Commands = append(plan.Commands, cmd)
				} else {
					cmd := fmt.Sprintf("%s %s | pg_restore -h %s -p %d -U %s -d %s",
						cmdName, strings.Join(cmdArgs, " "), host, port, user, opt.TargetDB)
					plan.Commands = append(plan.Commands, cmd)
				}
			} else {
				cmd := fmt.Sprintf("%s %s | psql -h %s -p %d -U %s -d %s",
					cmdName, strings.Join(cmdArgs, " "), host, port, user, opt.TargetDB)
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
		artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB, opt.IncludeGlobals)
		if err != nil {
			result.Status = "failed"
			result.Error = err
			result.FinishedAt = time.Now().Format(time.RFC3339)
			result.DurationSec = int64(time.Since(startTime).Seconds())
			return result, err
		}
		for _, a := range artifacts {
			if a.Checksum != "" {
				ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
				if err != nil {
					result.Status = "failed"
					result.Error = fmt.Errorf("校验 %s 的 checksum 失败: %w", a.Path, err)
					result.FinishedAt = time.Now().Format(time.RFC3339)
					result.DurationSec = int64(time.Since(startTime).Seconds())
					return result, result.Error
				}
				if !ok {
					result.Status = "failed"
					result.Error = fmt.Errorf("%s 的 checksum 不匹配", a.Path)
					result.FinishedAt = time.Now().Format(time.RFC3339)
					result.DurationSec = int64(time.Since(startTime).Seconds())
					return result, result.Error
				}
			}
		}
	}

	// Get connection info from job config
	host, port, user, password, sslMode, err := postgreSQLRestoreConnection(opt.JobConfig)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB, opt.IncludeGlobals)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
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
	for _, a := range artifacts {
		if a.Database == "__globals__" {
			// Restore globals
			if err := r.restoreGlobals(ctx, a, host, port, user, password, sslMode); err != nil {
				result.Status = "failed"
				result.Error = err
				result.FinishedAt = time.Now().Format(time.RFC3339)
				result.DurationSec = int64(time.Since(startTime).Seconds())
				return result, err
			}
			continue
		}

		dumpFormat, _ := opt.JobConfig.Options["dump_format"].(string)
		if dumpFormat == "custom" || dumpFormat == "tar" {
			if err := r.restoreCustom(ctx, a, host, port, user, password, sslMode, opt.TargetDB); err != nil {
				result.Status = "failed"
				result.Error = err
				result.FinishedAt = time.Now().Format(time.RFC3339)
				result.DurationSec = int64(time.Since(startTime).Seconds())
				return result, err
			}
		} else {
			if err := r.restorePlain(ctx, a, host, port, user, password, sslMode, opt.TargetDB); err != nil {
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
		return fmt.Errorf("创建数据库失败: %s", string(output))
	}

	return nil
}

// restoreGlobals restores global objects
func (r *Restorer) restoreGlobals(ctx context.Context, artifact manifest.Artifact, host string, port int, user, password, sslMode string) error {
	cmdName, cmdArgs := decompressCommand(artifact)
	decompressCmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	psqlCmd := exec.CommandContext(ctx, "psql",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-d", "postgres",
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	psqlCmd.Env = env

	pipe, err := decompressCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}
	psqlCmd.Stdin = pipe

	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("启动 psql 失败: %w", err)
	}
	if err := decompressCmd.Start(); err != nil {
		return fmt.Errorf("启动解压器失败: %w", err)
	}

	if err := decompressCmd.Wait(); err != nil {
		return fmt.Errorf("解压器执行失败: %w", err)
	}
	if err := psqlCmd.Wait(); err != nil {
		return fmt.Errorf("psql 执行失败: %w", err)
	}

	return nil
}

// restoreCustom restores a custom format backup
func (r *Restorer) restoreCustom(ctx context.Context, artifact manifest.Artifact, host string, port int, user, password, sslMode, targetDB string) error {
	if artifact.Compression != "" && artifact.Compression != "none" {
		cmdName, cmdArgs := decompressCommand(artifact)
		decompressCmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
		restoreCmd := exec.CommandContext(ctx, "pg_restore",
			"-h", host,
			"-p", fmt.Sprintf("%d", port),
			"-U", user,
			"-d", targetDB,
			"--no-owner",
			"--no-privileges",
		)

		env := os.Environ()
		if password != "" {
			env = append(env, "PGPASSWORD="+password)
		}
		env = append(env, "PGSSLMODE="+sslMode)
		restoreCmd.Env = env

		pipe, err := decompressCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("创建管道失败: %w", err)
		}
		restoreCmd.Stdin = pipe

		if err := restoreCmd.Start(); err != nil {
			return fmt.Errorf("启动 pg_restore 失败: %w", err)
		}
		if err := decompressCmd.Start(); err != nil {
			return fmt.Errorf("启动解压器失败: %w", err)
		}
		if err := decompressCmd.Wait(); err != nil {
			return fmt.Errorf("解压器执行失败: %w", err)
		}
		if err := restoreCmd.Wait(); err != nil {
			return fmt.Errorf("pg_restore 执行失败: %w", err)
		}
		return nil
	}

	cmd := exec.CommandContext(ctx, "pg_restore",
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"-U", user,
		"-d", targetDB,
		"--no-owner",
		"--no-privileges",
		artifact.Path,
	)

	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	env = append(env, "PGSSLMODE="+sslMode)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore 执行失败: %s", string(output))
	}

	return nil
}

// restorePlain restores a plain SQL backup
func (r *Restorer) restorePlain(ctx context.Context, artifact manifest.Artifact, host string, port int, user, password, sslMode, targetDB string) error {
	cmdName, cmdArgs := decompressCommand(artifact)
	decompressCmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
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

	pipe, err := decompressCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}
	psqlCmd.Stdin = pipe

	if err := psqlCmd.Start(); err != nil {
		return fmt.Errorf("启动 psql 失败: %w", err)
	}
	if err := decompressCmd.Start(); err != nil {
		return fmt.Errorf("启动解压器失败: %w", err)
	}

	if err := decompressCmd.Wait(); err != nil {
		return fmt.Errorf("解压器执行失败: %w", err)
	}
	if err := psqlCmd.Wait(); err != nil {
		return fmt.Errorf("psql 执行失败: %w", err)
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

func (r *Restorer) selectedArtifacts(backupDir string, m *manifest.Manifest, sourceDB string, includeGlobals bool) ([]manifest.Artifact, error) {
	var artifacts []manifest.Artifact
	for _, a := range m.Artifacts {
		if a.Database == "__globals__" {
			if !includeGlobals {
				continue
			}
		} else if sourceDB != "" && a.Database != sourceDB {
			continue
		}
		if err := validateArtifactPath(backupDir, a.Path); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("未找到源数据库 %q 的可恢复备份文件", sourceDB)
	}
	return artifacts, nil
}

func validateArtifactPath(backupDir, artifactPath string) error {
	backupAbs, err := filepath.Abs(backupDir)
	if err != nil {
		return fmt.Errorf("解析备份目录失败: %w", err)
	}
	artifactAbs, err := filepath.Abs(artifactPath)
	if err != nil {
		return fmt.Errorf("解析备份文件路径失败: %w", err)
	}
	rel, err := filepath.Rel(backupAbs, artifactAbs)
	if err != nil {
		return fmt.Errorf("检查备份文件路径失败: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("备份文件路径越过备份目录: %s", artifactPath)
	}
	return nil
}

func decompressCommand(a manifest.Artifact) (string, []string) {
	compressionType := a.Compression
	if compressionType == "" {
		switch {
		case strings.HasSuffix(a.Path, ".zst"):
			compressionType = "zstd"
		case strings.HasSuffix(a.Path, ".gz"):
			compressionType = "gzip"
		default:
			compressionType = "none"
		}
	}
	c := compress.NewCompressor(compressionType, 0)
	return c.DecompressCommand(a.Path)
}

func postgreSQLRestoreConnection(job engine.JobConfig) (string, int, string, string, string, error) {
	if job.Host == "" {
		return "", 0, "", "", "", fmt.Errorf("恢复 host 必填")
	}
	if job.Port == 0 {
		return "", 0, "", "", "", fmt.Errorf("恢复 port 必填")
	}
	if job.User == "" {
		return "", 0, "", "", "", fmt.Errorf("恢复 user 必填")
	}
	sslMode := "disable"
	if job.Options != nil {
		if v, ok := job.Options["sslmode"].(string); ok && v != "" {
			sslMode = v
		}
	}
	return job.Host, job.Port, job.User, job.Password, sslMode, nil
}
