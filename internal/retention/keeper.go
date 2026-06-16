package retention

import (
	"time"
)

// BackupRecord represents a backup record for retention
type BackupRecord struct {
	BackupID  string
	DBType    string
	Job       string
	Status    string
	StartedAt time.Time
	SizeBytes int64
	BackupDir string
}

// Keeper handles backup retention policies
type Keeper struct {
	KeepLast      int
	KeepDays      int
	MaxTotalSize  int64
	KeepFailedLast int
}

// NewKeeper creates a new retention keeper
func NewKeeper(keepLast, keepDays int, maxSize int64, keepFailedLast int) *Keeper {
	return &Keeper{
		KeepLast:      keepLast,
		KeepDays:      keepDays,
		MaxTotalSize:  maxSize,
		KeepFailedLast: keepFailedLast,
	}
}

// Prune returns records that should be pruned
func (k *Keeper) Prune(records []BackupRecord) []BackupRecord {
	if len(records) == 0 {
		return nil
	}

	// Separate successful and failed records
	var successful, failed []BackupRecord
	for _, r := range records {
		if r.Status == "success" {
			successful = append(successful, r)
		} else {
			failed = append(failed, r)
		}
	}

	var toKeep []BackupRecord
	var toPrune []BackupRecord

	// Apply KeepLast policy
	if k.KeepLast > 0 && len(successful) > k.KeepLast {
		toKeep = append(toKeep, successful[:k.KeepLast]...)
		toPrune = append(toPrune, successful[k.KeepLast:]...)
	} else {
		toKeep = append(toKeep, successful...)
	}

	// Apply KeepDays policy
	if k.KeepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -k.KeepDays)
		var kept []BackupRecord
		for _, r := range toKeep {
			if r.StartedAt.Before(cutoff) {
				toPrune = append(toPrune, r)
			} else {
				kept = append(kept, r)
			}
		}
		toKeep = kept
	}

	// Apply MaxTotalSize policy
	if k.MaxTotalSize > 0 {
		var totalSize int64
		var kept []BackupRecord
		for _, r := range toKeep {
			totalSize += r.SizeBytes
			if totalSize > k.MaxTotalSize {
				toPrune = append(toPrune, r)
			} else {
				kept = append(kept, r)
			}
		}
		toKeep = kept
	}

	// Handle failed records
	if k.KeepFailedLast > 0 && len(failed) > k.KeepFailedLast {
		toPrune = append(toPrune, failed[k.KeepFailedLast:]...)
	}

	return toPrune
}