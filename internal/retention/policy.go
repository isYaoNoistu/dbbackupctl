package retention

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Policy holds retention policy configuration
type Policy struct {
	KeepLast       int
	KeepDays       int
	KeepFailedLast int
	MaxTotalSize   int64
}

// BackupEntry holds information about a backup for retention
type BackupEntry struct {
	Path       string
	BackupID   string
	Job        string
	DBType     string
	Status     string
	StartedAt  time.Time
	SizeBytes  int64
}

// Manager handles retention policy enforcement
type Manager struct {
	policy Policy
}

// NewManager creates a new retention manager
func NewManager(policy Policy) *Manager {
	return &Manager{
		policy: policy,
	}
}

// GetBackupsToDelete returns a list of backups to delete based on retention policy
func (m *Manager) GetBackupsToDelete(backupDir string, dbType, job string) ([]BackupEntry, error) {
	// Scan backup directory
	entries, err := m.scanBackups(backupDir, dbType, job)
	if err != nil {
		return nil, fmt.Errorf("scanning backups: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Sort by time (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedAt.After(entries[j].StartedAt)
	})

	// Separate successful and failed backups
	var successful, failed []BackupEntry
	for _, entry := range entries {
		if entry.Status == "success" {
			successful = append(successful, entry)
		} else {
			failed = append(failed, entry)
		}
	}

	// Determine which successful backups to keep
	toKeep := make(map[string]bool)

	// Keep by count
	if m.policy.KeepLast > 0 {
		for i := 0; i < m.policy.KeepLast && i < len(successful); i++ {
			toKeep[successful[i].BackupID] = true
		}
	}

	// Keep by days
	if m.policy.KeepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -m.policy.KeepDays)
		for _, entry := range successful {
			if entry.StartedAt.After(cutoff) {
				toKeep[entry.BackupID] = true
			}
		}
	}

	// Determine which failed backups to keep
	if m.policy.KeepFailedLast > 0 {
		for i := 0; i < m.policy.KeepFailedLast && i < len(failed); i++ {
			toKeep[failed[i].BackupID] = true
		}
	}

	// Check total size
	if m.policy.MaxTotalSize > 0 {
		var totalSize int64
		for _, entry := range successful {
			if toKeep[entry.BackupID] {
				totalSize += entry.SizeBytes
			}
		}

		// If total size exceeds max, remove oldest backups
		if totalSize > m.policy.MaxTotalSize {
			// Sort successful backups by time (oldest first)
			sort.Slice(successful, func(i, j int) bool {
				return successful[i].StartedAt.Before(successful[j].StartedAt)
			})

			for _, entry := range successful {
				if totalSize <= m.policy.MaxTotalSize {
					break
				}
				if toKeep[entry.BackupID] {
					toKeep[entry.BackupID] = false
					totalSize -= entry.SizeBytes
				}
			}
		}
	}

	// Collect backups to delete
	var toDelete []BackupEntry
	for _, entry := range entries {
		if !toKeep[entry.BackupID] {
			toDelete = append(toDelete, entry)
		}
	}

	return toDelete, nil
}

// scanBackups scans backup directory for backups
func (m *Manager) scanBackups(backupDir string, dbType, job string) ([]BackupEntry, error) {
	var entries []BackupEntry

	// Walk through backup directory
	err := filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Look for manifest.json files
		if info.Name() == "manifest.json" {
			entry, err := m.readManifest(path, dbType, job)
			if err != nil {
				return nil // Skip invalid manifests
			}
			entries = append(entries, *entry)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking backup directory: %w", err)
	}

	return entries, nil
}

// readManifest reads backup information from manifest file
func (m *Manager) readManifest(manifestPath string, dbType, job string) (*BackupEntry, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest struct {
		BackupID  string    `json:"backup_id"`
		DBType    string    `json:"db_type"`
		Job       string    `json:"job"`
		Status    string    `json:"status"`
		StartedAt time.Time `json:"started_at"`
		Artifacts []struct {
			SizeBytes int64 `json:"size_bytes"`
		} `json:"artifacts"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	// Filter by dbType and job
	if dbType != "" && manifest.DBType != dbType {
		return nil, fmt.Errorf("db type mismatch")
	}
	if job != "" && manifest.Job != job {
		return nil, fmt.Errorf("job mismatch")
	}

	// Calculate total size
	var totalSize int64
	for _, artifact := range manifest.Artifacts {
		totalSize += artifact.SizeBytes
	}

	return &BackupEntry{
		Path:      filepath.Dir(manifestPath),
		BackupID:  manifest.BackupID,
		Job:       manifest.Job,
		DBType:    manifest.DBType,
		Status:    manifest.Status,
		StartedAt: manifest.StartedAt,
		SizeBytes: totalSize,
	}, nil
}

// DeleteBackup deletes a backup directory
func (m *Manager) DeleteBackup(entry BackupEntry) error {
	return os.RemoveAll(entry.Path)
}
