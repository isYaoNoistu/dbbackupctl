package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/checksum"
	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/disk"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/mysql"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/postgresql"
	"github.com/isYaoNoistu/dbbackupctl/internal/exiterr"
	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/lock"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
	"github.com/isYaoNoistu/dbbackupctl/internal/retention"
)

// BackupOptions holds backup options
type BackupOptions struct {
	DryRun     bool
	NoCompress bool
	NoPrune    bool
	Force      bool
}

// BackupRunner handles backup operations
type BackupRunner struct {
	Config     *configenv.Config
	IndexStore *index.Store
	Manifest   *manifest.Writer
}

// NewBackupRunner creates a new backup runner
func NewBackupRunner(cfg *configenv.Config) *BackupRunner {
	return &BackupRunner{
		Config:     cfg,
		IndexStore: index.NewStore(cfg.Core.IndexFile),
		Manifest:   manifest.NewWriter(),
	}
}

// BackupMySQL performs MySQL backup
func (r *BackupRunner) BackupMySQL(ctx context.Context, jobName string, opt BackupOptions) error {
	startTime := time.Now()

	// Convert config to engine params
	engineJob, target, err := ConvertMySQLJob(r.Config, jobName)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Dry run mode
	if opt.DryRun {
		return r.printDryRun("mysql", jobName, target)
	}

	// Acquire lock
	lm := lock.NewManager(r.Config.Core.LockDir)
	if err := lm.Acquire("mysql", jobName, opt.Force); err != nil {
		return exiterr.New(exiterr.ExitLockConflict, err)
	}
	defer lm.Release("mysql", jobName)

	// Create engine
	eng := mysql.NewEngine()

	// Check dependency
	if err := eng.CheckDependency(ctx); err != nil {
		return exiterr.New(exiterr.ExitDependency, err)
	}

	// Check connection
	if err := eng.CheckConnection(ctx, engineJob); err != nil {
		return exiterr.New(exiterr.ExitDBConnection, err)
	}

	// Estimate size
	databases, err := r.resolveDatabases(ctx, eng, engineJob, target.Databases)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}
	target.Databases = databases

	estimated, err := eng.EstimateSize(ctx, engineJob, databases)
	if err != nil {
		// Non-fatal, continue without estimate
		estimated = 0
	}

	// Check disk space
	if r.Config.Core.DiskGuardEnabled {
		minFree, err := configenv.ParseSize(r.Config.Core.DiskMinFreeSize)
		if err != nil {
			r.writeFailedRecord("mysql", jobName, target, startTime, err)
			return exiterr.New(exiterr.ExitConfig, fmt.Errorf("解析磁盘最小剩余空间失败: %w", err))
		}
		guard := disk.NewGuard(
			minFree,
			r.Config.Core.DiskMinFreePercent,
			r.Config.Core.DiskEstimateBufferPercent,
		)
		if err := guard.CheckDiskSpace(target.BackupDir, estimated); err != nil {
			r.writeFailedRecord("mysql", jobName, target, startTime, err)
			return exiterr.New(exiterr.ExitDiskInsufficient, err)
		}
	}

	// Perform backup
	result, err := eng.Backup(ctx, engineJob, target)
	if err != nil {
		// Write failed record to index
		r.writeFailedRecord("mysql", jobName, target, startTime, err)
		return exiterr.New(exiterr.ExitBackupFailed, err)
	}

	// Calculate checksum for each artifact
	if r.Config.Core.ChecksumEnabled {
		for i := range result.Artifacts {
			artifact := &result.Artifacts[i]
			checksumVal, err := checksum.FileSHA256(artifact.Path)
			if err != nil {
				r.writeFailedRecord("mysql", jobName, target, startTime, err)
				return exiterr.New(exiterr.ExitChecksumFailed, fmt.Errorf("计算 %s 的 checksum 失败: %w", artifact.Path, err))
			}
			artifact.ChecksumType = "sha256"
			artifact.Checksum = checksumVal
		}
	}

	// Write manifest
	m := r.buildManifest("mysql", jobName, result)
	if err := r.Manifest.Write(m, result.BackupDir); err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("写入 manifest 失败: %w", err))
	}

	// Write index record
	record := r.buildRecord("mysql", jobName, result, startTime)
	if err := r.IndexStore.Append(record); err != nil {
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("写入索引失败: %w", err))
	}

	// Run retention if enabled
	if !opt.NoPrune && r.Config.Core.RetentionPruneAfterBackup {
		r.runRetention("mysql", jobName)
	}

	fmt.Printf("备份完成：%s\n", result.BackupID)
	fmt.Printf("  备份目录: %s\n", result.BackupDir)
	fmt.Printf("  耗时: %d 秒\n", result.DurationSec)
	fmt.Printf("  数据库: %v\n", result.Databases)

	return nil
}

// BackupPostgreSQL performs PostgreSQL backup
func (r *BackupRunner) BackupPostgreSQL(ctx context.Context, jobName string, opt BackupOptions) error {
	startTime := time.Now()

	// Convert config to engine params
	engineJob, target, err := ConvertPostgreSQLJob(r.Config, jobName)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Dry run mode
	if opt.DryRun {
		return r.printDryRun("postgresql", jobName, target)
	}

	// Acquire lock
	lm := lock.NewManager(r.Config.Core.LockDir)
	if err := lm.Acquire("postgresql", jobName, opt.Force); err != nil {
		return exiterr.New(exiterr.ExitLockConflict, err)
	}
	defer lm.Release("postgresql", jobName)

	// Create engine
	eng := postgresql.NewEngine()

	// Check dependency
	if err := eng.CheckDependency(ctx); err != nil {
		return exiterr.New(exiterr.ExitDependency, err)
	}

	// Check connection
	if err := eng.CheckConnection(ctx, engineJob); err != nil {
		return exiterr.New(exiterr.ExitDBConnection, err)
	}

	// Resolve databases
	databases, err := r.resolveDatabases(ctx, eng, engineJob, target.Databases)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}
	target.Databases = databases

	// Estimate size
	estimated, err := eng.EstimateSize(ctx, engineJob, databases)
	if err != nil {
		estimated = 0
	}

	// Check disk space
	if r.Config.Core.DiskGuardEnabled {
		minFree, err := configenv.ParseSize(r.Config.Core.DiskMinFreeSize)
		if err != nil {
			r.writeFailedRecord("postgresql", jobName, target, startTime, err)
			return exiterr.New(exiterr.ExitConfig, fmt.Errorf("解析磁盘最小剩余空间失败: %w", err))
		}
		guard := disk.NewGuard(
			minFree,
			r.Config.Core.DiskMinFreePercent,
			r.Config.Core.DiskEstimateBufferPercent,
		)
		if err := guard.CheckDiskSpace(target.BackupDir, estimated); err != nil {
			r.writeFailedRecord("postgresql", jobName, target, startTime, err)
			return exiterr.New(exiterr.ExitDiskInsufficient, err)
		}
	}

	// Perform backup
	result, err := eng.Backup(ctx, engineJob, target)
	if err != nil {
		r.writeFailedRecord("postgresql", jobName, target, startTime, err)
		return exiterr.New(exiterr.ExitBackupFailed, err)
	}

	// Calculate checksum for each artifact
	if r.Config.Core.ChecksumEnabled {
		for i := range result.Artifacts {
			artifact := &result.Artifacts[i]
			checksumVal, err := checksum.FileSHA256(artifact.Path)
			if err != nil {
				r.writeFailedRecord("postgresql", jobName, target, startTime, err)
				return exiterr.New(exiterr.ExitChecksumFailed, fmt.Errorf("计算 %s 的 checksum 失败: %w", artifact.Path, err))
			}
			artifact.ChecksumType = "sha256"
			artifact.Checksum = checksumVal
		}
	}

	// Write manifest
	m := r.buildManifest("postgresql", jobName, result)
	if err := r.Manifest.Write(m, result.BackupDir); err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("写入 manifest 失败: %w", err))
	}

	// Write index record
	record := r.buildRecord("postgresql", jobName, result, startTime)
	if err := r.IndexStore.Append(record); err != nil {
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("写入索引失败: %w", err))
	}

	// Run retention if enabled
	if !opt.NoPrune && r.Config.Core.RetentionPruneAfterBackup {
		r.runRetention("postgresql", jobName)
	}

	fmt.Printf("备份完成：%s\n", result.BackupID)
	fmt.Printf("  备份目录: %s\n", result.BackupDir)
	fmt.Printf("  耗时: %d 秒\n", result.DurationSec)
	fmt.Printf("  数据库: %v\n", result.Databases)

	return nil
}

// resolveDatabases resolves database list
func (r *BackupRunner) resolveDatabases(ctx context.Context, eng engine.Engine, job engine.JobConfig, databases []string) ([]string, error) {
	// Check if wildcard
	for _, db := range databases {
		if db == "*" {
			// Get all databases from engine
			switch e := eng.(type) {
			case *mysql.Engine:
				return e.GetDatabases(ctx, job, false)
			case *postgresql.Engine:
				return e.GetDatabases(ctx, job, false, false)
			}
		}
	}
	return databases, nil
}

// buildManifest builds manifest from backup result
func (r *BackupRunner) buildManifest(dbType, jobName string, result *engine.BackupResult) *manifest.Manifest {
	artifacts := make([]manifest.Artifact, len(result.Artifacts))
	for i, a := range result.Artifacts {
		artifacts[i] = manifest.Artifact{
			Database:     a.Database,
			File:         a.File,
			Path:         a.Path,
			SizeBytes:    a.SizeBytes,
			Compression:  a.Compression,
			ChecksumType: a.ChecksumType,
			Checksum:     a.Checksum,
		}
	}

	startedAt, _ := time.Parse(time.RFC3339, result.StartedAt)
	finishedAt, _ := time.Parse(time.RFC3339, result.FinishedAt)

	return &manifest.Manifest{
		Version:     "1.0",
		BackupID:    result.BackupID,
		DBType:      dbType,
		Job:         jobName,
		Status:      result.Status,
		BackupMode:  "logical",
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		DurationSec: result.DurationSec,
		Artifacts:   artifacts,
		Retention: manifest.RetentionInfo{
			KeepLast:     r.Config.Core.RetentionKeepLast,
			KeepDays:     r.Config.Core.RetentionKeepDays,
			MaxTotalSize: r.Config.Core.RetentionMaxTotalSize,
		},
	}
}

// buildRecord builds backup record from result
func (r *BackupRunner) buildRecord(dbType, jobName string, result *engine.BackupResult, startTime time.Time) index.BackupRecord {
	var totalSize int64
	for _, a := range result.Artifacts {
		totalSize += a.SizeBytes
	}

	return index.BackupRecord{
		BackupID:    result.BackupID,
		DBType:      dbType,
		Job:         jobName,
		Status:      result.Status,
		StartedAt:   startTime,
		DurationSec: result.DurationSec,
		SizeBytes:   totalSize,
		BackupDir:   result.BackupDir,
		Manifest:    result.BackupDir + "/manifest.json",
	}
}

// writeFailedRecord writes a failed backup record and manifest
func (r *BackupRunner) writeFailedRecord(dbType, jobName string, target engine.BackupTarget, startTime time.Time, backupErr error) {
	// Create backup directory if it doesn't exist
	os.MkdirAll(target.BackupDir, 0755)

	// Write failed manifest
	failedManifest := &manifest.Manifest{
		Version:     "1.0",
		BackupID:    target.BackupID,
		DBType:      dbType,
		Job:         jobName,
		Status:      "failed",
		BackupMode:  "logical",
		StartedAt:   startTime,
		FinishedAt:  time.Now(),
		DurationSec: int64(time.Since(startTime).Seconds()),
		Error: &manifest.ErrorInfo{
			Code:    "BACKUP_FAILED",
			Message: backupErr.Error(),
		},
	}
	r.Manifest.Write(failedManifest, target.BackupDir)

	// Write failed record to index
	record := index.BackupRecord{
		BackupID:    target.BackupID,
		DBType:      dbType,
		Job:         jobName,
		Status:      "failed",
		StartedAt:   startTime,
		DurationSec: int64(time.Since(startTime).Seconds()),
		SizeBytes:   0,
		BackupDir:   target.BackupDir,
		Manifest:    target.BackupDir + "/manifest.json",
	}
	r.IndexStore.Append(record)
}

// printDryRun prints dry run information
func (r *BackupRunner) printDryRun(dbType, jobName string, target engine.BackupTarget) error {
	fmt.Printf("预演模式：不会执行真实备份\n")
	fmt.Printf("  数据库类型: %s\n", dbType)
	fmt.Printf("  环境: %s\n", jobName)
	fmt.Printf("  备份ID: %s\n", target.BackupID)
	fmt.Printf("  备份目录: %s\n", target.BackupDir)
	fmt.Printf("  数据库: %v\n", target.Databases)
	return nil
}

// runRetention runs retention policy for a job
func (r *BackupRunner) runRetention(dbType, jobName string) {
	// Get job config for retention settings
	var backupDir string
	var keepLast, keepDays int
	var maxSize string
	var keepFailedLast int

	if dbType == "mysql" {
		job := r.Config.MySQL.JobConfigs[jobName]
		backupDir = job.BackupDir
		keepLast = job.RetentionKeepLast
		keepDays = job.RetentionKeepDays
		maxSize = job.RetentionMaxTotalSize
	} else {
		job := r.Config.PostgreSQL.JobConfigs[jobName]
		backupDir = job.BackupDir
		keepLast = job.RetentionKeepLast
		keepDays = job.RetentionKeepDays
		maxSize = job.RetentionMaxTotalSize
	}

	// Use global defaults if job doesn't have specific settings
	if keepLast == 0 {
		keepLast = r.Config.Core.RetentionKeepLast
	}
	if keepDays == 0 {
		keepDays = r.Config.Core.RetentionKeepDays
	}
	if maxSize == "" {
		maxSize = r.Config.Core.RetentionMaxTotalSize
	}
	keepFailedLast = r.Config.Core.RetentionKeepFailedLast

	// Parse max size
	maxSizeBytes, err := configenv.ParseSize(maxSize)
	if err != nil {
		fmt.Printf("警告：解析保留策略最大大小失败：%v\n", err)
		return
	}

	// Query records for this job
	records, err := r.IndexStore.Query(index.QueryFilter{
		DBType: dbType,
		Job:    jobName,
	})
	if err != nil {
		fmt.Printf("警告：查询保留策略记录失败：%v\n", err)
		return
	}

	// Convert to retention records
	retRecords := make([]retention.BackupRecord, len(records))
	for i, rec := range records {
		retRecords[i] = retention.BackupRecord{
			BackupID:  rec.BackupID,
			DBType:    rec.DBType,
			Job:       rec.Job,
			Status:    rec.Status,
			StartedAt: rec.StartedAt,
			SizeBytes: rec.SizeBytes,
			BackupDir: rec.BackupDir,
		}
	}

	// Create keeper and prune
	keeper := retention.NewKeeper(keepLast, keepDays, maxSizeBytes, keepFailedLast)
	toPrune := keeper.Prune(retRecords)

	if len(toPrune) == 0 {
		return
	}

	// Delete pruned backup directories
	deleted := 0
	for _, rec := range toPrune {
		if rec.BackupDir != "" && rec.BackupDir != backupDir {
			fmt.Printf("正在清理备份：%s（目录：%s）\n", rec.BackupID, rec.BackupDir)
			if err := os.RemoveAll(rec.BackupDir); err != nil {
				fmt.Printf("警告：清理 %s 失败：%v\n", rec.BackupID, err)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		if err := r.IndexStore.Rebuild(r.Config.Core.BackupRoot); err != nil {
			fmt.Printf("警告：保留策略执行后重建索引失败：%v\n", err)
		}
	}

	fmt.Printf("保留策略：已清理 %d 个备份\n", deleted)
}
