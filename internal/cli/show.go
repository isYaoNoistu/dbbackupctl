package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/isYaoNoistu/dbbackupctl/internal/index"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show backup records",
		Long: `Show backup records from the local index.

Available subcommands:
  mysql       - Show MySQL backup records
  postgresql  - Show PostgreSQL backup records
  all         - Show all backup records

The backup records are stored in /data/dbbackupctl/index/backup_records.jsonl`,
		Example: `  dbbackupctl show mysql
  dbbackupctl show mysql --last 10
  dbbackupctl show mysql --job prod
  dbbackupctl show postgresql
  dbbackupctl show postgresql --last 10
  dbbackupctl show all
  dbbackupctl show mysql --json`,
	}

	cmd.AddCommand(
		newShowMySQLCmd(),
		newShowPostgreSQLCmd(),
		newShowAllCmd(),
	)

	return cmd
}

func newShowMySQLCmd() *cobra.Command {
	var (
		last int
		job  string
		json bool
	)

	cmd := &cobra.Command{
		Use:   "mysql",
		Short: "Show MySQL backup records",
		Long: `Show MySQL backup records from the local index.

Default output shows the last 5 records in table format.
Use --last to change the number of records shown.
Use --json for machine-readable output.`,
		Example: `  dbbackupctl show mysql
  dbbackupctl show mysql --last 10
  dbbackupctl show mysql --job prod
  dbbackupctl show mysql --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("mysql", last, job, json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "Number of records to show")
	cmd.Flags().StringVar(&job, "job", "", "Filter by job name")
	cmd.Flags().BoolVar(&json, "json", false, "Output in JSON format")

	return cmd
}

func newShowPostgreSQLCmd() *cobra.Command {
	var (
		last int
		job  string
		json bool
	)

	cmd := &cobra.Command{
		Use:   "postgresql",
		Short: "Show PostgreSQL backup records",
		Long: `Show PostgreSQL backup records from the local index.

Default output shows the last 5 records in table format.
Use --last to change the number of records shown.
Use --json for machine-readable output.`,
		Example: `  dbbackupctl show postgresql
  dbbackupctl show postgresql --last 10
  dbbackupctl show postgresql --job prod
  dbbackupctl show postgresql --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("postgresql", last, job, json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "Number of records to show")
	cmd.Flags().StringVar(&job, "job", "", "Filter by job name")
	cmd.Flags().BoolVar(&json, "json", false, "Output in JSON format")

	return cmd
}

func newShowAllCmd() *cobra.Command {
	var (
		last int
		json bool
	)

	cmd := &cobra.Command{
		Use:   "all",
		Short: "Show all backup records",
		Long: `Show all backup records (MySQL and PostgreSQL) from the local index.

Default output shows the last 5 records in table format.
Use --last to change the number of records shown.
Use --json for machine-readable output.`,
		Example: `  dbbackupctl show all
  dbbackupctl show all --last 20
  dbbackupctl show all --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow("all", last, "", json)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "Number of records to show")
	cmd.Flags().BoolVar(&json, "json", false, "Output in JSON format")

	return cmd
}

func runShow(dbType string, last int, job string, jsonOutput bool) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create index store
	store := index.NewStore(cfg.Core.IndexFile)

	// Query records
	records, err := store.Query(index.QueryFilter{
		DBType: dbType,
		Job:    job,
		Limit:  last,
	})
	if err != nil {
		return fmt.Errorf("querying index: %w", err)
	}

	// Output as JSON
	if jsonOutput {
		return printJSON(records)
	}

	// Output as table
	return printTable(records)
}

func printJSON(records []index.BackupRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printTable(records []index.BackupRecord) error {
	if len(records) == 0 {
		fmt.Println("No backup records found.")
		return nil
	}

	// Create tabwriter
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintf(w, "BACKUP ID\tTYPE\tJOB\tSTATUS\tSTARTED AT\tDURATION\tSIZE\tPATH\n")
	fmt.Fprintf(w, "----------\t----\t---\t------\t----------\t--------\t----\t----\n")

	// Print records
	for _, r := range records {
		// Format duration
		duration := formatDuration(r.DurationSec)

		// Format size
		size := formatSize(r.SizeBytes)

		// Format started at
		startedAt := r.StartedAt.Format("2006-01-02 15:04:05")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.BackupID,
			r.DBType,
			r.Job,
			r.Status,
			startedAt,
			duration,
			size,
			r.BackupDir,
		)
	}

	w.Flush()
	return nil
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh%dm%ds", seconds/3600, (seconds%3600)/60, seconds%60)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
