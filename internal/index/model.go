package index

import "time"

// BackupRecord holds a backup record for the index
type BackupRecord struct {
	BackupID    string    `json:"backup_id"`
	DBType      string    `json:"db_type"`
	Job         string    `json:"job"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	DurationSec int64     `json:"duration_seconds"`
	SizeBytes   int64     `json:"size_bytes"`
	BackupDir   string    `json:"backup_dir"`
	Manifest    string    `json:"manifest"`
}

// QueryFilter holds query filters for backup records
type QueryFilter struct {
	DBType string
	Job    string
	Limit  int
}
