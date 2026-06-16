package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/checksum"
	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/mysql"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine/postgresql"
	"github.com/isYaoNoistu/dbbackupctl/internal/exiterr"
	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
)

// RestoreOptions holds restore options
type RestoreOptions struct {
	SourceDB       string
	TargetDB       string
	Execute        bool
	AllowOverwrite bool
	IncludeGlobals bool
}

// RestoreRunner handles restore operations
type RestoreRunner struct {
	Config     *configenv.Config
	IndexStore *index.Store
	Manifest   *manifest.Writer
}

// NewRestoreRunner creates a new restore runner
func NewRestoreRunner(cfg *configenv.Config) *RestoreRunner {
	return &RestoreRunner{
		Config:     cfg,
		IndexStore: index.NewStore(cfg.Core.IndexFile),
		Manifest:   manifest.NewWriter(),
	}
}

// RestoreMySQL restores a MySQL backup
func (r *RestoreRunner) RestoreMySQL(ctx context.Context, backupID string, opt RestoreOptions) error {
	// Find backup record
	record, err := r.IndexStore.FindByID(backupID)
	if err != nil {
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("未找到备份记录: %w", err))
	}

	// Read manifest
	m, err := r.Manifest.Read(record.BackupDir)
	if err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("读取 manifest 失败: %w", err))
	}

	// Find source database in manifest
	sourceDB, err := r.findSourceDB(m, opt.SourceDB)
	if err != nil {
		return err
	}
	opt.SourceDB = sourceDB

	// Check allow-overwrite
	if sourceDB == opt.TargetDB && !opt.AllowOverwrite {
		return exiterr.Newf(exiterr.ExitRestoreFailed,
			"目标数据库与源数据库相同，如确认要恢复到原库，请增加 --allow-overwrite")
	}

	// Verify checksum
	if r.Config.Core.ChecksumEnabled {
		if err := r.verifyChecksum(m); err != nil {
			return exiterr.New(exiterr.ExitChecksumFailed, err)
		}
	}

	// Get restore connection config
	restoreJob, err := ConvertMySQLRestoreJob(r.Config, record.Job)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Create engine
	eng := mysql.NewEngine()

	// Build restore options with job config
	restoreOpt := engine.RestoreOptions{
		TargetDB:       opt.TargetDB,
		SourceDB:       sourceDB,
		AllowOverwrite: opt.AllowOverwrite,
		Execute:        opt.Execute,
		JobConfig:      restoreJob,
	}

	// Convert index record to engine record
	engineRecord := convertToEngineRecord(record)

	// Generate restore plan
	plan, err := eng.RestorePlan(ctx, engineRecord, restoreOpt)
	if err != nil {
		return exiterr.New(exiterr.ExitRestoreFailed, err)
	}

	// Print restore plan
	r.printRestorePlan(plan)

	// Execute if requested
	if opt.Execute {
		result, err := eng.Restore(ctx, engineRecord, restoreOpt)
		if err != nil {
			return exiterr.New(exiterr.ExitRestoreFailed, err)
		}

		fmt.Printf("恢复完成：%s\n", result.BackupID)
		fmt.Printf("  目标数据库: %s\n", result.TargetDB)
		fmt.Printf("  耗时: %d 秒\n", result.DurationSec)

		// Write restore record
		r.writeRestoreRecord(record, opt, result)
	}

	return nil
}

// RestorePostgreSQL restores a PostgreSQL backup
func (r *RestoreRunner) RestorePostgreSQL(ctx context.Context, backupID string, opt RestoreOptions) error {
	// Find backup record
	record, err := r.IndexStore.FindByID(backupID)
	if err != nil {
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("未找到备份记录: %w", err))
	}

	// Read manifest
	m, err := r.Manifest.Read(record.BackupDir)
	if err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("读取 manifest 失败: %w", err))
	}

	// Find source database in manifest
	sourceDB, err := r.findSourceDB(m, opt.SourceDB)
	if err != nil {
		return err
	}
	opt.SourceDB = sourceDB

	// Check allow-overwrite
	if sourceDB == opt.TargetDB && !opt.AllowOverwrite {
		return exiterr.Newf(exiterr.ExitRestoreFailed,
			"目标数据库与源数据库相同，如确认要恢复到原库，请增加 --allow-overwrite")
	}

	// Verify checksum
	if r.Config.Core.ChecksumEnabled {
		if err := r.verifyChecksum(m); err != nil {
			return exiterr.New(exiterr.ExitChecksumFailed, err)
		}
	}

	// Get restore connection config
	restoreJob, err := ConvertPostgreSQLRestoreJob(r.Config, record.Job)
	if err != nil {
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Create engine
	eng := postgresql.NewEngine()

	// Build restore options
	restoreOpt := engine.RestoreOptions{
		TargetDB:       opt.TargetDB,
		SourceDB:       sourceDB,
		AllowOverwrite: opt.AllowOverwrite,
		Execute:        opt.Execute,
		JobConfig:      restoreJob,
		IncludeGlobals: opt.IncludeGlobals,
	}

	// Convert index record to engine record
	engineRecord := convertToEngineRecord(record)

	// Generate restore plan
	plan, err := eng.RestorePlan(ctx, engineRecord, restoreOpt)
	if err != nil {
		return exiterr.New(exiterr.ExitRestoreFailed, err)
	}

	// Print restore plan
	r.printRestorePlan(plan)

	// Execute if requested
	if opt.Execute {
		result, err := eng.Restore(ctx, engineRecord, restoreOpt)
		if err != nil {
			return exiterr.New(exiterr.ExitRestoreFailed, err)
		}

		fmt.Printf("恢复完成：%s\n", result.BackupID)
		fmt.Printf("  目标数据库: %s\n", result.TargetDB)
		fmt.Printf("  耗时: %d 秒\n", result.DurationSec)

		// Write restore record
		r.writeRestoreRecord(record, opt, result)
	}

	return nil
}

// findSourceDB finds the source database from manifest
func (r *RestoreRunner) findSourceDB(m *manifest.Manifest, sourceDB string) (string, error) {
	// If source DB is specified, use it
	if sourceDB != "" {
		// Verify it exists in manifest
		for _, a := range m.Artifacts {
			if a.Database == sourceDB {
				return sourceDB, nil
			}
		}
		return "", exiterr.Newf(exiterr.ExitConfig, "备份中未找到源数据库 %s", sourceDB)
	}

	// If only one database, use it
	databases := make(map[string]bool)
	for _, a := range m.Artifacts {
		if a.Database != "__globals__" {
			databases[a.Database] = true
		}
	}

	if len(databases) == 1 {
		for db := range databases {
			return db, nil
		}
	}

	return "", exiterr.Newf(exiterr.ExitConfig,
		"备份包含多个数据库，请指定 --source-db 或 --database")
}

// verifyChecksum verifies checksum for all artifacts
func (r *RestoreRunner) verifyChecksum(m *manifest.Manifest) error {
	for _, a := range m.Artifacts {
		if a.Checksum == "" {
			continue
		}

		ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
		if err != nil {
			return fmt.Errorf("校验 %s 的 checksum 失败: %w", a.Path, err)
		}
		if !ok {
			return fmt.Errorf("%s 的 checksum 不匹配", a.Path)
		}
	}
	return nil
}

// printRestorePlan prints the restore plan
func (r *RestoreRunner) printRestorePlan(plan *engine.RestorePlan) {
	fmt.Printf("恢复计划：\n")
	fmt.Printf("  备份ID: %s\n", plan.BackupID)
	fmt.Printf("  源数据库: %s\n", plan.SourceDB)
	fmt.Printf("  目标数据库: %s\n", plan.TargetDB)
	fmt.Printf("  备份目录: %s\n", plan.BackupDir)
	fmt.Printf("  校验和通过: %v\n", plan.ChecksumOK)
	fmt.Printf("\n将执行的命令：\n")
	for _, cmd := range plan.Commands {
		fmt.Printf("  %s\n", cmd)
	}
}

// writeRestoreRecord writes restore record to index
func (r *RestoreRunner) writeRestoreRecord(record *index.BackupRecord, opt RestoreOptions, result *engine.RestoreResult) {
	restoreRecord := struct {
		BackupID       string    `json:"backup_id"`
		DBType         string    `json:"db_type"`
		Job            string    `json:"job"`
		SourceDB       string    `json:"source_db"`
		TargetDB       string    `json:"target_db"`
		Status         string    `json:"status"`
		Execute        bool      `json:"execute"`
		AllowOverwrite bool      `json:"allow_overwrite"`
		IncludeGlobals bool      `json:"include_globals"`
		StartedAt      string    `json:"started_at"`
		FinishedAt     string    `json:"finished_at"`
		DurationSec    int64     `json:"duration_seconds"`
		RecordedAt     time.Time `json:"recorded_at"`
	}{
		BackupID:       record.BackupID,
		DBType:         record.DBType,
		Job:            record.Job,
		SourceDB:       opt.SourceDB,
		TargetDB:       opt.TargetDB,
		Status:         result.Status,
		Execute:        opt.Execute,
		AllowOverwrite: opt.AllowOverwrite,
		IncludeGlobals: opt.IncludeGlobals,
		StartedAt:      result.StartedAt,
		FinishedAt:     result.FinishedAt,
		DurationSec:    result.DurationSec,
		RecordedAt:     time.Now(),
	}

	path := r.Config.Core.RestoreLogFile
	if path == "" {
		path = filepath.Join(r.Config.Core.DataDir, "index", "restore_records.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Printf("警告：创建恢复记录目录失败：%v\n", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		fmt.Printf("警告：打开恢复记录文件失败：%v\n", err)
		return
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(restoreRecord); err != nil {
		fmt.Printf("警告：写入恢复记录失败：%v\n", err)
	}
}

// convertToEngineRecord converts index.BackupRecord to engine.BackupRecord
func convertToEngineRecord(record *index.BackupRecord) engine.BackupRecord {
	return engine.BackupRecord{
		BackupID:    record.BackupID,
		DBType:      record.DBType,
		Job:         record.Job,
		Status:      record.Status,
		StartedAt:   record.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		DurationSec: record.DurationSec,
		SizeBytes:   record.SizeBytes,
		BackupDir:   record.BackupDir,
		Manifest:    record.Manifest,
	}
}
