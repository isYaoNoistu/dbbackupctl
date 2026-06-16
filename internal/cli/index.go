package cli

import (
	"fmt"

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
	// TODO: Implement index verify logic
	fmt.Println("Index verify command called")
	return nil
}

func runIndexRebuild(backupRoot string) error {
	// TODO: Implement index rebuild logic
	fmt.Println("Index rebuild command called")
	fmt.Printf("Backup root: %s\n", backupRoot)
	return nil
}