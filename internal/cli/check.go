package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/isYaoNoistu/dbbackupctl/internal/checker"
	"github.com/isYaoNoistu/dbbackupctl/internal/exiterr"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var (
		mysql      bool
		postgresql bool
		job        string
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check config, dependencies, permissions and disk space",
		Long: `Check the environment and configuration for dbbackupctl.

This command verifies:
  - Configuration files exist and are valid
  - Environment variables are properly set
  - secret.env has correct permissions (600)
  - Backup directories exist and are writable
  - Required tools are available (mysqldump, pg_dump, zstd, etc.)
  - Database connections are working
  - Disk space meets requirements
  - No stale locks exist`,
		Example: `  dbbackupctl check
  dbbackupctl check --mysql
  dbbackupctl check --postgresql
  dbbackupctl check --job prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(mysql, postgresql, job)
		},
	}

	cmd.Flags().BoolVar(&mysql, "mysql", false, "Check MySQL configuration only")
	cmd.Flags().BoolVar(&postgresql, "postgresql", false, "Check PostgreSQL configuration only")
	cmd.Flags().StringVar(&job, "job", "", "Check specific job configuration")

	return cmd
}

func runCheck(mysql, postgresql bool, job string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		// If config loading fails, try to create a basic check report
		fmt.Fprintf(os.Stderr, "Warning: Cannot load config: %v\n", err)
		fmt.Println("Check FAILED: Configuration error")
		return exiterr.New(exiterr.ExitConfig, err)
	}

	// Create checker
	ch := checker.NewChecker(cfg, GetConfigDir())
	ch.CheckMySQL = mysql
	ch.CheckPg = postgresql
	ch.JobName = job

	// Run checks
	report := ch.Run(context.Background())

	// Output as JSON if requested
	if IsJSONOutput() {
		return printCheckJSON(report)
	}

	// Output as table
	printCheckTable(report)

	// Return error if any check failed
	if report.HasFailure() {
		return exiterr.New(exiterr.ExitGeneral, fmt.Errorf("check failed"))
	}

	return nil
}

func printCheckJSON(report *checker.CheckReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printCheckTable(report *checker.CheckReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintf(w, "STATUS\tCHECK\tMESSAGE\n")
	fmt.Fprintf(w, "------\t-----\t-------\n")

	// Print items
	for _, item := range report.Items {
		status := string(item.Status)
		switch item.Status {
		case checker.CheckOK:
			status = "OK"
		case checker.CheckWarn:
			status = "WARN"
		case checker.CheckFail:
			status = "FAIL"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", status, item.Name, item.Message)
	}

	w.Flush()

	// Print summary
	fmt.Printf("\n%s\n", report.Summary())
}