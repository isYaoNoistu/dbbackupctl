package postgresql

import (
	"context"
	"fmt"

	"github.com/dbbackupctl/dbbackupctl/internal/engine"
)

// Engine implements the PostgreSQL backup engine
type Engine struct {
	inspector *Inspector
	backuper  *Backuper
	restorer  *Restorer
}

// NewEngine creates a new PostgreSQL engine
func NewEngine() *Engine {
	return &Engine{
		inspector: NewInspector(),
		backuper:  NewBackuper(),
		restorer:  NewRestorer(),
	}
}

// Name returns the engine name
func (e *Engine) Name() string {
	return "postgresql"
}

// CheckDependency checks if required tools are available
func (e *Engine) CheckDependency(ctx context.Context) error {
	return e.inspector.CheckDependency(ctx)
}

// CheckConnection checks if database connection is working
func (e *Engine) CheckConnection(ctx context.Context, job engine.JobConfig) error {
	return e.inspector.CheckConnection(ctx, job)
}

// GetDatabases returns a list of databases
func (e *Engine) GetDatabases(ctx context.Context, job engine.JobConfig, includeTemplate, includePostgres bool) ([]string, error) {
	return e.inspector.GetDatabases(ctx, job, includeTemplate, includePostgres)
}

// EstimateSize estimates the backup size in bytes
func (e *Engine) EstimateSize(ctx context.Context, job engine.JobConfig, databases []string) (int64, error) {
	return e.inspector.EstimateSize(ctx, job, databases)
}

// Backup performs the backup operation
func (e *Engine) Backup(ctx context.Context, job engine.JobConfig, target engine.BackupTarget) (*engine.BackupResult, error) {
	return e.backuper.Backup(ctx, job, target)
}

// RestorePlan generates a restore plan without executing
func (e *Engine) RestorePlan(ctx context.Context, record engine.BackupRecord, opt engine.RestoreOptions) (*engine.RestorePlan, error) {
	return e.restorer.RestorePlan(ctx, record, opt)
}

// Restore performs the restore operation
func (e *Engine) Restore(ctx context.Context, record engine.BackupRecord, opt engine.RestoreOptions) (*engine.RestoreResult, error) {
	return e.restorer.Restore(ctx, record, opt)
}

// GetEngine returns the engine as interface
func GetEngine() engine.Engine {
	return NewEngine()
}

// ValidateJob validates a PostgreSQL job configuration
func ValidateJob(job engine.JobConfig) error {
	if job.Host == "" {
		return fmt.Errorf("host is required")
	}
	if job.Port == 0 {
		return fmt.Errorf("port is required")
	}
	if job.User == "" {
		return fmt.Errorf("user is required")
	}
	return nil
}