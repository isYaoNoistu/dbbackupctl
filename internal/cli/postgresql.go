package cli

import (
	"context"
	"fmt"

	"github.com/isYaoNoistu/dbbackupctl/internal/app"
	"github.com/spf13/cobra"
)

func newPostgreSQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgresql",
		Short: "PostgreSQL backup and restore commands",
		Long: `PostgreSQL backup and restore operations.

Available subcommands:
  backup    - Create a PostgreSQL backup
  restore   - Restore a PostgreSQL backup

Configuration:
  PostgreSQL jobs are configured in /etc/dbbackupctl/postgresql.env
  Passwords are read from environment variables or /etc/dbbackupctl/secret.env`,
		Example: `  dbbackupctl postgresql backup --job prod
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore --execute`,
	}

	cmd.AddCommand(
		newPostgreSQLBackupCmd(),
		newPostgreSQLRestoreCmd(),
	)

	return cmd
}

func newPostgreSQLBackupCmd() *cobra.Command {
	var (
		job        string
		all        bool
		dryRun     bool
		noCompress bool
		noPrune    bool
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a PostgreSQL backup",
		Long: `Create a logical backup of PostgreSQL databases using pg_dump.

The backup is compressed using zstd by default and stored in the configured
backup directory. A manifest.json and backup record are created for each backup.

Password reading order:
  1. Environment variable specified by POSTGRES_<JOB>_PASSWORD_ENV
  2. Value from /etc/dbbackupctl/secret.env
  3. File specified by POSTGRES_<JOB>_PASSWORD_FILE

Examples:
  dbbackupctl postgresql backup --job prod
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql backup --job prod --no-compress`,
		Example: `  dbbackupctl postgresql backup --job prod
  dbbackupctl postgresql backup --all
  dbbackupctl postgresql backup --job prod --dry-run
  dbbackupctl postgresql backup --job prod --no-compress
  dbbackupctl postgresql backup --job prod --no-prune
  dbbackupctl postgresql backup --job prod --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostgreSQLBackup(job, all, dryRun, noCompress, noPrune, force)
		},
	}

	cmd.Flags().StringVar(&job, "job", "", "Job name to backup (required unless --all)")
	cmd.Flags().BoolVar(&all, "all", false, "Backup all enabled jobs")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show backup plan without executing")
	cmd.Flags().BoolVar(&noCompress, "no-compress", false, "Disable compression (debug only)")
	cmd.Flags().BoolVar(&noPrune, "no-prune", false, "Skip retention policy after backup")
	cmd.Flags().BoolVar(&force, "force", false, "Clean stale lock before executing")

	return cmd
}

func newPostgreSQLRestoreCmd() *cobra.Command {
	var (
		id             string
		sourceDB       string
		targetDB       string
		execute        bool
		allowOverwrite bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a PostgreSQL backup",
		Long: `Restore a PostgreSQL backup to a target database.

By default, this command only shows the restore plan without executing.
Use --execute to actually perform the restore.

Safety features:
  - Checksum verification before restore
  - Requires --allow-overwrite to restore to the original database
  - Restore log is saved for audit

Examples:
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore --execute
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb --allow-overwrite --execute`,
		Example: `  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb_restore --execute
  dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --source-db appdb --target-db appdb --allow-overwrite --execute`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostgreSQLRestore(id, sourceDB, targetDB, execute, allowOverwrite)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Backup ID to restore (required)")
	cmd.Flags().StringVar(&sourceDB, "source-db", "", "Source database name (required if backup contains multiple databases)")
	cmd.Flags().StringVar(&targetDB, "target-db", "", "Target database name (required)")
	cmd.Flags().BoolVar(&execute, "execute", false, "Execute restore (default: plan only)")
	cmd.Flags().BoolVar(&allowOverwrite, "allow-overwrite", false, "Allow overwrite original database")

	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("target-db")

	return cmd
}

func runPostgreSQLBackup(job string, all, dryRun, noCompress, noPrune, force bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create backup runner
	runner := app.NewBackupRunner(cfg)

	// Build options
	opt := app.BackupOptions{
		DryRun:     dryRun,
		NoCompress: noCompress,
		NoPrune:    noPrune,
		Force:      force,
	}

	// Backup all jobs
	if all {
		for _, j := range cfg.PostgreSQL.Jobs {
			if err := runner.BackupPostgreSQL(context.Background(), j, opt); err != nil {
				return err
			}
		}
		return nil
	}

	// Backup single job
	if job == "" {
		return fmt.Errorf("--job is required unless --all is used")
	}

	return runner.BackupPostgreSQL(context.Background(), job, opt)
}

func runPostgreSQLRestore(id, sourceDB, targetDB string, execute, allowOverwrite bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create restore runner
	runner := app.NewRestoreRunner(cfg)

	// Build options
	opt := app.RestoreOptions{
		SourceDB:       sourceDB,
		TargetDB:       targetDB,
		Execute:        execute,
		AllowOverwrite: allowOverwrite,
	}

	return runner.RestorePostgreSQL(context.Background(), id, opt)
}
