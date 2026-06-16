package manifest

import "time"

// Manifest holds backup manifest information
type Manifest struct {
	Version        string            `json:"version"`
	BackupID       string            `json:"backup_id"`
	DBType         string            `json:"db_type"`
	Job            string            `json:"job"`
	Status         string            `json:"status"`
	BackupMode     string            `json:"backup_mode"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     time.Time         `json:"finished_at"`
	DurationSec    int64             `json:"duration_seconds"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	User           string            `json:"user"`
	Databases      []string          `json:"databases"`
	BackupDir      string            `json:"backup_dir"`
	Artifacts      []Artifact        `json:"artifacts"`
	Command        CommandInfo       `json:"command"`
	Retention      RetentionInfo     `json:"retention"`
	Error          *ErrorInfo        `json:"error,omitempty"`
}

// Artifact holds information about a backup artifact
type Artifact struct {
	Database     string `json:"database"`
	File         string `json:"file"`
	Path         string `json:"path"`
	SizeBytes    int64  `json:"size_bytes"`
	Compression  string `json:"compression"`
	ChecksumType string `json:"checksum_type,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
}

// CommandInfo holds information about the backup command
type CommandInfo struct {
	Binary       string   `json:"binary"`
	Version      string   `json:"version"`
	ArgsRedacted []string `json:"args_redacted"`
}

// RetentionInfo holds retention policy information
type RetentionInfo struct {
	KeepLast     int    `json:"keep_last"`
	KeepDays     int    `json:"keep_days"`
	MaxTotalSize string `json:"max_total_size"`
}

// ErrorInfo holds error information
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Log     string `json:"log,omitempty"`
}