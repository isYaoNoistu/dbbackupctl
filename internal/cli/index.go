package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/isYaoNoistu/dbbackupctl/internal/manifest"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage local backup index",
		Long: `Manage the local backup index (backup_records.jsonl).

Available subcommands:
  verify  - Verify index integrity
  rebuild - Rebuild index from backup directories

The index file is located at:
  /data/dbbackupctl/index/backup_records.jsonl`,
		Example: `  dbbackupctl index verify
  dbbackupctl index rebuild
  dbbackupctl index rebuild --backup-root /data/backup`,
	}

	cmd.AddCommand(
		newIndexVerifyCmd(),
		newIndexRebuildCmd(),
	)

	return cmd
}

func newIndexVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify index integrity",
		Long: `Verify the integrity of the backup index.

This command checks:
  - Index file exists and is readable
  - Each record has required fields
  - Referenced manifest files exist
  - Referenced backup directories exist`,
		Example: `  dbbackupctl index verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndexVerify()
		},
	}

	return cmd
}

func newIndexRebuildCmd() *cobra.Command {
	var backupRoot string

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild index from backup directories",
		Long: `Rebuild the backup index by scanning backup directories.

This command:
  - Scans all backup directories under backup_root
  - Reads manifest.json from each backup
  - Rebuilds backup_records.jsonl

Use this when:
  - The index file is corrupted or missing
  - Backup directories were manually moved
  - After a backup_root migration`,
		Example: `  dbbackupctl index rebuild
  dbbackupctl index rebuild --backup-root /data/backup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndexRebuild(backupRoot)
		},
	}

	cmd.Flags().StringVar(&backupRoot, "backup-root", "", "Backup root directory (default: from config)")

	return cmd
}

func runIndexVerify() error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query all records
	records, err := store.Query(index.QueryFilter{Limit: 10000})
	if err != nil {
		return fmt.Errorf("reading index: %w", err)
	}

	fmt.Printf("Found %d records in index\n", len(records))

	// Verify each record
	issues := 0
	for _, r := range records {
		// Check required fields
		if r.BackupID == "" {
			fmt.Printf("  ISSUE: Record missing backup_id\n")
			issues++
			continue
		}

		// Check manifest exists
		manifestPath := filepath.Join(r.BackupDir, "manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			fmt.Printf("  ISSUE: %s - manifest not found at %s\n", r.BackupID, manifestPath)
			issues++
			continue
		}

		// Check backup dir exists
		if _, err := os.Stat(r.BackupDir); os.IsNotExist(err) {
			fmt.Printf("  ISSUE: %s - backup dir not found at %s\n", r.BackupID, r.BackupDir)
			issues++
			continue
		}

		// Read and verify manifest
		mw := manifest.NewWriter()
		m, err := mw.Read(r.BackupDir)
		if err != nil {
			fmt.Printf("  ISSUE: %s - cannot read manifest: %v\n", r.BackupID, err)
			issues++
			continue
		}

		// Verify manifest matches index
		if m.BackupID != r.BackupID {
			fmt.Printf("  ISSUE: %s - backup_id mismatch with manifest\n", r.BackupID)
			issues++
			continue
		}

		fmt.Printf("  OK: %s\n", r.BackupID)
	}

	if issues > 0 {
		fmt.Printf("\nFound %d issues. Run 'dbbackupctl index rebuild' to fix.\n", issues)
	} else {
		fmt.Printf("\nAll records verified successfully.\n")
	}

	return nil
}

func runIndexRebuild(backupRoot string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Use configured backup root if not specified
	if backupRoot == "" {
		backupRoot = cfg.Core.BackupRoot
		if backupRoot == "" {
			backupRoot = "/data/backup"
		}
	}

	fmt.Printf("Scanning backup directories under: %s\n", backupRoot)

	// Find all manifest.json files
	var records []index.BackupRecord
	err = filepath.Walk(backupRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.Name() == "manifest.json" {
			// Read manifest
			dir := filepath.Dir(path)
			mw := manifest.NewWriter()
			m, err := mw.Read(dir)
			if err != nil {
				fmt.Printf("  WARN: Cannot read manifest at %s: %v\n", path, err)
				return nil
			}

			// Create record
			record := index.BackupRecord{
				BackupID:  m.BackupID,
				DBType:    m.DBType,
				Job:       m.Job,
				Status:    m.Status,
				StartedAt: m.StartedAt,
				DurationSec: m.DurationSec,
				BackupDir: m.BackupDir,
				Manifest:  path,
			}

			// Calculate total size
			for _, a := range m.Artifacts {
				record.SizeBytes += a.SizeBytes
			}

			records = append(records, record)
			fmt.Printf("  Found: %s (%s)\n", m.BackupID, m.Status)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning backup directories: %w", err)
	}

	fmt.Printf("\nFound %d backups\n", len(records))

	// Write new index file
	indexFile := cfg.Core.IndexFile
	if indexFile == "" {
		indexFile = "/data/dbbackupctl/index/backup_records.jsonl"
	}

	// Create backup of old index
	if _, err := os.Stat(indexFile); err == nil {
		backupFile := indexFile + ".bak"
		if err := copyFile(indexFile, backupFile); err != nil {
			fmt.Printf("  WARN: Cannot backup old index: %v\n", err)
		} else {
			fmt.Printf("  Backed up old index to: %s\n", backupFile)
		}
	}

	// Write new index
	if err := os.MkdirAll(filepath.Dir(indexFile), 0750); err != nil {
		return fmt.Errorf("creating index directory: %w", err)
	}

	f, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("creating index file: %w", err)
	}
	defer f.Close()

	for _, r := range records {
		data, err := json.Marshal(r)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		f.Write(data)
	}

	fmt.Printf("\nIndex rebuilt successfully: %s\n", indexFile)
	fmt.Printf("Total records: %d\n", len(records))

	return nil
}

// copyFile copies a file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0640)
}

func init() {
	// Suppress unused import warning
	_ = strings.Contains
}