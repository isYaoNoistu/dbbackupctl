package engine

import (
	"context"
)

// Engine defines the interface for database backup engines
type Engine interface {
	// Name returns the engine name (mysql, postgresql)
	Name() string

	// CheckDependency checks if required tools are available
	CheckDependency(ctx context.Context) error

	// CheckConnection checks if database connection is working
	CheckConnection(ctx context.Context, job JobConfig) error

	// EstimateSize estimates the backup size in bytes
	EstimateSize(ctx context.Context, job JobConfig, databases []string) (int64, error)

	// Backup performs the backup operation
	Backup(ctx context.Context, job JobConfig, target BackupTarget) (*BackupResult, error)

	// RestorePlan generates a restore plan without executing
	RestorePlan(ctx context.Context, record BackupRecord, opt RestoreOptions) (*RestorePlan, error)

	// Restore performs the restore operation
	Restore(ctx context.Context, record BackupRecord, opt RestoreOptions) (*RestoreResult, error)
}

// JobConfig holds job configuration
type JobConfig struct {
	Name     string
	Host     string
	Port     int
	User     string
	Password string
	Database string
	// Additional options specific to the job
	Options map[string]interface{}
}

// BackupTarget holds backup target information
type BackupTarget struct {
	BackupDir    string
	Databases    []string
	Timestamp    string
	BackupID     string
	Compression  CompressionConfig
}

// CompressionConfig holds compression configuration
type CompressionConfig struct {
	Enabled bool
	Type    string // zstd, gzip, none
	Level   int
	Threads int
}

// BackupResult holds backup operation result
type BackupResult struct {
	BackupID     string
	BackupDir    string
	Databases    []string
	Artifacts    []Artifact
	StartedAt    string
	FinishedAt   string
	DurationSec  int64
	Status       string
	Error        error
}

// Artifact holds information about a backup artifact
type Artifact struct {
	Database     string
	File         string
	Path         string
	SizeBytes    int64
	Compression  string
	ChecksumType string
	Checksum     string
}

// BackupRecord holds a backup record from the index
type BackupRecord struct {
	BackupID      string `json:"backup_id"`
	DBType        string `json:"db_type"`
	Job           string `json:"job"`
	Status        string `json:"status"`
	StartedAt     string `json:"started_at"`
	DurationSec   int64  `json:"duration_seconds"`
	SizeBytes     int64  `json:"size_bytes"`
	BackupDir     string `json:"backup_dir"`
	Manifest      string `json:"manifest"`
}

// RestoreOptions holds restore options
type RestoreOptions struct {
	TargetDB       string
	SourceDB       string
	AllowOverwrite bool
	Execute        bool
	JobConfig      JobConfig
}

// RestorePlan holds restore plan information
type RestorePlan struct {
	BackupID     string
	SourceDB     string
	TargetDB     string
	BackupDir    string
	Artifacts    []Artifact
	ChecksumOK   bool
	Commands     []string
}

// RestoreResult holds restore operation result
type RestoreResult struct {
	BackupID     string
	TargetDB     string
	StartedAt    string
	FinishedAt   string
	DurationSec  int64
	Status       string
	Error        error
}
