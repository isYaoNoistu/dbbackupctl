package app

import (
	"context"
	"fmt"

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
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("backup record not found: %w", err))
	}

	// Read manifest
	m, err := r.Manifest.Read(record.BackupDir)
	if err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("reading manifest: %w", err))
	}

	// Find source database in manifest
	sourceDB, err := r.findSourceDB(m, opt.SourceDB)
	if err != nil {
		return err
	}

	// Check allow-overwrite
	if sourceDB == opt.TargetDB && !opt.AllowOverwrite {
		return exiterr.Newf(exiterr.ExitRestoreFailed,
			"target database equals source database, use --allow-overwrite if you really want to restore to original database")
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
		SourceDB:       opt.SourceDB,
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

		fmt.Printf("Restore completed: %s\n", result.BackupID)
		fmt.Printf("  Target DB: %s\n", result.TargetDB)
		fmt.Printf("  Duration: %d seconds\n", result.DurationSec)

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
		return exiterr.New(exiterr.ExitIndexError, fmt.Errorf("backup record not found: %w", err))
	}

	// Read manifest
	m, err := r.Manifest.Read(record.BackupDir)
	if err != nil {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("reading manifest: %w", err))
	}

	// Find source database in manifest
	sourceDB, err := r.findSourceDB(m, opt.SourceDB)
	if err != nil {
		return err
	}

	// Check allow-overwrite
	if sourceDB == opt.TargetDB && !opt.AllowOverwrite {
		return exiterr.Newf(exiterr.ExitRestoreFailed,
			"target database equals source database, use --allow-overwrite if you really want to restore to original database")
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
		AllowOverwrite: opt.AllowOverwrite,
		Execute:        opt.Execute,
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

		fmt.Printf("Restore completed: %s\n", result.BackupID)
		fmt.Printf("  Target DB: %s\n", result.TargetDB)
		fmt.Printf("  Duration: %d seconds\n", result.DurationSec)

		// Write restore record
		r.writeRestoreRecord(record, opt, result)
	}

	_ = restoreJob // Will be used for actual restore
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
		return "", exiterr.Newf(exiterr.ExitConfig, "source database %s not found in backup", sourceDB)
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
		"backup contains multiple databases, please specify --source-db")
}

// verifyChecksum verifies checksum for all artifacts
func (r *RestoreRunner) verifyChecksum(m *manifest.Manifest) error {
	for _, a := range m.Artifacts {
		if a.Checksum == "" {
			continue
		}

		ok, err := checksum.VerifyFileSHA256(a.Path, a.Checksum)
		if err != nil {
			return fmt.Errorf("verifying checksum for %s: %w", a.Path, err)
		}
		if !ok {
			return fmt.Errorf("checksum mismatch for %s", a.Path)
		}
	}
	return nil
}

// printRestorePlan prints the restore plan
func (r *RestoreRunner) printRestorePlan(plan *engine.RestorePlan) {
	fmt.Printf("Restore Plan:\n")
	fmt.Printf("  Backup ID: %s\n", plan.BackupID)
	fmt.Printf("  Source DB: %s\n", plan.SourceDB)
	fmt.Printf("  Target DB: %s\n", plan.TargetDB)
	fmt.Printf("  Backup Dir: %s\n", plan.BackupDir)
	fmt.Printf("  Checksum OK: %v\n", plan.ChecksumOK)
	fmt.Printf("\nCommands to execute:\n")
	for _, cmd := range plan.Commands {
		fmt.Printf("  %s\n", cmd)
	}
}

// writeRestoreRecord writes restore record to index
func (r *RestoreRunner) writeRestoreRecord(record *index.BackupRecord, opt RestoreOptions, result *engine.RestoreResult) {
	// TODO: Write to restore_records.jsonl
	_ = record
	_ = opt
	_ = result
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
