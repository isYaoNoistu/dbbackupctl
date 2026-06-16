package cli

import (
	"context"
	"fmt"

	"github.com/isYaoNoistu/dbbackupctl/internal/app"
	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/spf13/cobra"
)

func newMySQLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mysql",
		Short: "MySQL backup and restore commands",
		Long: `MySQL backup and restore operations.

Available subcommands:
  backup    - Create a MySQL backup
  restore   - Restore a MySQL backup

Configuration:
  MySQL jobs are configured in /etc/dbbackupctl/mysql.env
  Passwords are read from environment variables or /etc/dbbackupctl/secret.env`,
		Example: `  dbbackupctl mysql backup --job prod
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --target-db aloof_restore
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --target-db aloof_restore --execute`,
	}

	cmd.AddCommand(
		newMySQLBackupCmd(),
		newMySQLRestoreCmd(),
	)

	return cmd
}

func newMySQLBackupCmd() *cobra.Command {
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
		Short: "Create a MySQL backup",
		Long: `Create a logical backup of MySQL databases using mysqldump.

The backup is compressed using zstd by default and stored in the configured
backup directory. A manifest.json and backup record are created for each backup.

Password reading order:
  1. Environment variable specified by MYSQL_<JOB>_PASSWORD_ENV
  2. Value from /etc/dbbackupctl/secret.env
  3. File specified by MYSQL_<JOB>_PASSWORD_FILE

Examples:
  dbbackupctl mysql backup --job prod
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql backup --job prod --no-compress`,
		Example: `  dbbackupctl mysql backup --job prod
  dbbackupctl mysql backup --all
  dbbackupctl mysql backup --job prod --dry-run
  dbbackupctl mysql backup --job prod --no-compress
  dbbackupctl mysql backup --job prod --no-prune
  dbbackupctl mysql backup --job prod --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMySQLBackup(job, all, dryRun, noCompress, noPrune, force)
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

func newMySQLRestoreCmd() *cobra.Command {
	var (
		id             string
		sourceDB       string
		targetDB       string
		execute        bool
		allowOverwrite bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a MySQL backup",
		Long: `Restore a MySQL backup to a target database.

By default, this command only shows the restore plan without executing.
Use --execute to actually perform the restore.

Safety features:
  - Checksum verification before restore
  - Requires --allow-overwrite to restore to the original database
  - Restore log is saved for audit

Examples:
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof_restore
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof_restore --execute
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof --allow-overwrite --execute`,
		Example: `  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof_restore
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof_restore --execute
  dbbackupctl mysql restore --id mysql-prod-20260616-020000 --source-db aloof --target-db aloof --allow-overwrite --execute`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMySQLRestore(id, sourceDB, targetDB, execute, allowOverwrite)
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

func runMySQLBackup(job string, all, dryRun, noCompress, noPrune, force bool) error {
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
		for _, j := range cfg.MySQL.Jobs {
			if err := runner.BackupMySQL(context.Background(), j, opt); err != nil {
				return err
			}
		}
		return nil
	}

	// Backup single job
	if job == "" {
		return fmt.Errorf("--job is required unless --all is used")
	}

	return runner.BackupMySQL(context.Background(), job, opt)
}

func runMySQLRestore(id, sourceDB, targetDB string, execute, allowOverwrite bool) error {
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

	return runner.RestoreMySQL(context.Background(), id, opt)
}

func loadConfig() (*configenv.Config, error) {
	// Use global config directory
	loader := configenv.NewLoader(GetConfigDir())
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Validate configuration
	validator := configenv.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}
