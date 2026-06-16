package mysql

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
		return nil, fmt.Errorf("未找到 manifest: %s", manifestPath)
	}

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

	artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB)
	if err != nil {
		return nil, err
	}

	// Verify checksum for each artifact
	for _, a := range artifacts {
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

		host, port, user, _, err := mysqlRestoreConnection(opt.JobConfig)
		if err != nil {
			return nil, err
		}

		cmdName, cmdArgs := decompressCommand(a)
		cmd := fmt.Sprintf("%s %s | mysql -h %s -P %d -u %s %s",
			cmdName, strings.Join(cmdArgs, " "), host, port, user, opt.TargetDB)
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
		artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB)
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
	host, port, user, password, err := mysqlRestoreConnection(opt.JobConfig)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
	}

	artifacts, err := r.selectedArtifacts(record.BackupDir, m, opt.SourceDB)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.FinishedAt = time.Now().Format(time.RFC3339)
		result.DurationSec = int64(time.Since(startTime).Seconds())
		return result, err
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
	for _, a := range artifacts {
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
		return fmt.Errorf("创建数据库失败: %s", string(output))
	}

	return nil
}

// restoreDatabase restores a single database
func (r *Restorer) restoreDatabase(ctx context.Context, sourceFile, host string, port int, user, password, targetDB string) error {
	cmdName, cmdArgs := decompressCommand(manifest.Artifact{Path: sourceFile, Compression: compressionFromPath(sourceFile)})
	decompressCmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	mysqlCmd := exec.CommandContext(ctx, "mysql",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		targetDB,
	)

	if password != "" {
		mysqlCmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	}

	pipe, err := decompressCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}
	mysqlCmd.Stdin = pipe

	decompressStderr, _ := decompressCmd.StderrPipe()
	mysqlStderr, _ := mysqlCmd.StderrPipe()

	if err := mysqlCmd.Start(); err != nil {
		return fmt.Errorf("启动 mysql 失败: %w", err)
	}
	if err := decompressCmd.Start(); err != nil {
		return fmt.Errorf("启动解压器失败: %w", err)
	}

	if err := decompressCmd.Wait(); err != nil {
		buf := make([]byte, 4096)
		n, _ := decompressStderr.Read(buf)
		return fmt.Errorf("解压器执行失败: %s", string(buf[:n]))
	}

	if err := mysqlCmd.Wait(); err != nil {
		buf := make([]byte, 4096)
		n, _ := mysqlStderr.Read(buf)
		return fmt.Errorf("mysql 执行失败: %s", string(buf[:n]))
	}

	return nil
}

func (r *Restorer) selectedArtifacts(backupDir string, m *manifest.Manifest, sourceDB string) ([]manifest.Artifact, error) {
	var artifacts []manifest.Artifact
	for _, a := range m.Artifacts {
		if a.Database == "__globals__" {
			continue
		}
		if sourceDB != "" && a.Database != sourceDB {
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
		compressionType = compressionFromPath(a.Path)
	}
	c := compress.NewCompressor(compressionType, 0)
	return c.DecompressCommand(a.Path)
}

func compressionFromPath(path string) string {
	switch {
	case strings.HasSuffix(path, ".zst"):
		return "zstd"
	case strings.HasSuffix(path, ".gz"):
		return "gzip"
	default:
		return "none"
	}
}

func mysqlRestoreConnection(job engine.JobConfig) (string, int, string, string, error) {
	if job.Host == "" {
		return "", 0, "", "", fmt.Errorf("恢复 host 必填")
	}
	if job.Port == 0 {
		return "", 0, "", "", fmt.Errorf("恢复 port 必填")
	}
	if job.User == "" {
		return "", 0, "", "", fmt.Errorf("恢复 user 必填")
	}
	return job.Host, job.Port, job.User, job.Password, nil
}
