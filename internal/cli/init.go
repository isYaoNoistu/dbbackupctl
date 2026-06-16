package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed configs/*.env.example
var configTemplates embed.FS

func newInitCmd() *cobra.Command {
	var (
		configDir string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate env config templates",
		Long: `Generate configuration file templates for dbbackupctl.

This command creates the necessary directories and example configuration files:
  - /etc/dbbackupctl/core.env.example
  - /etc/dbbackupctl/mysql.env.example
  - /etc/dbbackupctl/postgresql.env.example
  - /etc/dbbackupctl/secret.env.example

It also creates the required data and log directories:
  - /data/dbbackupctl
  - /data/backup
  - /var/log/dbbackupctl
  - /var/lock/dbbackupctl`,
		Example: `  dbbackupctl init
  dbbackupctl init --config-dir /etc/dbbackupctl
  dbbackupctl init --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(configDir, force)
		},
	}

	cmd.Flags().StringVar(&configDir, "config-dir", "/etc/dbbackupctl", "Configuration directory path")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing configuration files")

	return cmd
}

func runInit(configDir string, force bool) error {
	// Define directories to create
	dirs := []string{
		configDir,
		"/data/dbbackupctl",
		"/data/dbbackupctl/index",
		"/data/dbbackupctl/tmp",
		"/data/backup",
		"/var/log/dbbackupctl",
		"/var/lock/dbbackupctl",
	}

	// Create directories
	fmt.Println("Creating directories...")
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		fmt.Printf("  Created: %s\n", dir)
	}

	// Define config files to create
	configFiles := []struct {
		name string
		src  string
	}{
		{"core.env.example", "configs/core.env.example"},
		{"mysql.env.example", "configs/mysql.env.example"},
		{"postgresql.env.example", "configs/postgresql.env.example"},
		{"secret.env.example", "configs/secret.env.example"},
	}

	// Create config files
	fmt.Println("\nCreating configuration templates...")
	for _, cf := range configFiles {
		destPath := filepath.Join(configDir, cf.name)

		// Check if file exists
		if _, err := os.Stat(destPath); err == nil && !force {
			fmt.Printf("  Skipped: %s (already exists, use --force to overwrite)\n", destPath)
			continue
		}

		// Read template content
		content, err := configTemplates.ReadFile(cf.src)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", cf.src, err)
		}

		// Write file
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", destPath, err)
		}
		fmt.Printf("  Created: %s\n", destPath)
	}

	// Print next steps
	fmt.Println("\nInitialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit the configuration files in", configDir)
	fmt.Println("  2. Set passwords in secret.env")
	fmt.Printf("  3. Set permissions: chmod 600 %s/secret.env\n", configDir)
	fmt.Println("  4. Run 'dbbackupctl check' to verify configuration")

	return nil
}