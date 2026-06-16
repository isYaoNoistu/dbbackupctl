package cli

import (
	"fmt"

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
	// TODO: Implement show logic
	fmt.Println("Show command called")
	fmt.Printf("DB Type: %s\n", dbType)
	fmt.Printf("Last: %d\n", last)
	fmt.Printf("Job: %s\n", job)
	fmt.Printf("JSON: %v\n", jsonOutput)
	return nil
}