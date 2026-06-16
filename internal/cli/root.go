package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
)

// Run initializes and executes the root command
func Run(ver, com, dat string) error {
	version = ver
	commit = com
	date = dat

	rootCmd := &cobra.Command{
		Use:   "dbbackupctl",
		Short: "A lightweight local backup CLI for MySQL and PostgreSQL",
		Long: `dbbackupctl is a lightweight local backup CLI for MySQL and PostgreSQL.

It is designed to run directly on database servers without Web UI, Docker,
internal metadata database or background API server.

Features:
  - MySQL and PostgreSQL logical backup with streaming compression
  - Backup retention policies (count, days, size)
  - Disk space protection
  - Local JSONL index for backup records
  - Restore with checksum verification
  - Task locking to prevent concurrent backups`,
		Example: `  dbbackupctl init
  dbbackupctl check
  dbbackupctl mysql backup --job prod
  dbbackupctl postgresql backup --job prod
  dbbackupctl show mysql
  dbbackupctl show postgresql --last 10
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --target-db aloof_restore --execute`,
		SilenceUsage: true,
	}

	// Add subcommands
	rootCmd.AddCommand(
		newInitCmd(),
		newCheckCmd(),
		newMySQLCmd(),
		newPostgreSQLCmd(),
		newShowCmd(),
		newPruneCmd(),
		newIndexCmd(),
		newVersionCmd(),
	)

	return rootCmd.Execute()
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("dbbackupctl %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  date:   %s\n", date)
		},
	}
}