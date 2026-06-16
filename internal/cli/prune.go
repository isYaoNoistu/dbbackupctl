package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/retention"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	var (
		mysql      bool
		postgresql bool
		job        string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Apply retention policies and prune old backups",
		Long: `Apply retention policies to delete old backups.

Retention policies are configured in core.env:
  - DBB_RETENTION_KEEP_LAST: Keep last N backups
  - DBB_RETENTION_KEEP_DAYS: Keep backups for N days
  - DBB_RETENTION_MAX_TOTAL_SIZE: Maximum total size

This command also cleans up failed backups based on DBB_RETENTION_KEEP_FAILED_LAST.`,
		Example: `  dbbackupctl prune
  dbbackupctl prune --mysql
  dbbackupctl prune --postgresql
  dbbackupctl prune --job prod
  dbbackupctl prune --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrune(mysql, postgresql, job, dryRun)
		},
	}

	cmd.Flags().BoolVar(&mysql, "mysql", false, "Prune MySQL backups only")
	cmd.Flags().BoolVar(&postgresql, "postgresql", false, "Prune PostgreSQL backups only")
	cmd.Flags().StringVar(&job, "job", "", "Prune specific job backups")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pruned without actually deleting")

	return cmd
}

func runPrune(mysql, postgresql bool, job string, dryRun bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Parse max total size
	var maxSize int64
	if cfg.Core.RetentionMaxTotalSize != "" {
		maxSize, err = configenv.ParseSize(cfg.Core.RetentionMaxTotalSize)
		if err != nil {
			return fmt.Errorf("parsing max total size: %w", err)
		}
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query all records
	records, err := store.Query(index.QueryFilter{
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("querying index: %w", err)
	}

	// Filter by type
	var filtered []index.BackupRecord
	for _, r := range records {
		if mysql && r.DBType != "mysql" {
			continue
		}
		if postgresql && r.DBType != "postgresql" {
			continue
		}
		if job != "" && r.Job != job {
			continue
		}
		filtered = append(filtered, r)
	}

	// Group by job
	jobRecords := make(map[string][]index.BackupRecord)
	for _, r := range filtered {
		key := r.DBType + "/" + r.Job
		jobRecords[key] = append(jobRecords[key], r)
	}

	// Apply retention policies
	keeper := retention.NewKeeper(
		cfg.Core.RetentionKeepLast,
		cfg.Core.RetentionKeepDays,
		maxSize,
		cfg.Core.RetentionKeepFailedLast,
	)

	var toDelete []index.BackupRecord
	for _, records := range jobRecords {
		// Sort by start time (newest first)
		sort.Slice(records, func(i, j int) bool {
			return records[i].StartedAt.After(records[j].StartedAt)
		})

		// Convert to retention records
		retRecords := make([]retention.BackupRecord, len(records))
		for i, r := range records {
			retRecords[i] = retention.BackupRecord{
				BackupID:  r.BackupID,
				DBType:    r.DBType,
				Job:       r.Job,
				Status:    r.Status,
				StartedAt: r.StartedAt,
				SizeBytes: r.SizeBytes,
				BackupDir: r.BackupDir,
			}
		}

		// Apply retention policy
		toPrune := keeper.Prune(retRecords)

		// Convert back to index records
		for _, p := range toPrune {
			for _, r := range records {
				if r.BackupID == p.BackupID {
					toDelete = append(toDelete, r)
					break
				}
			}
		}
	}

	// Dry run mode
	if dryRun {
		fmt.Printf("Dry run - would delete %d backups:\n", len(toDelete))
		for _, r := range toDelete {
			fmt.Printf("  %s (%s/%s) - %s\n", r.BackupID, r.DBType, r.Job, r.BackupDir)
		}
		return nil
	}

	// Delete backups
	deleted := 0
	for _, r := range toDelete {
		if err := deleteBackupDir(r.BackupDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", r.BackupDir, err)
			continue
		}
		deleted++
	}

	fmt.Printf("Pruned %d backups\n", deleted)
	return nil
}

// deleteBackupDir deletes a backup directory
func deleteBackupDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}