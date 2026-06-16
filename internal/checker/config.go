package checker

import (
	"fmt"
	"os"
	"path/filepath"
)

// checkConfigFiles checks configuration files
func (c *Checker) checkConfigFiles(report *CheckReport) {
	// Check core.env
	coreEnv := filepath.Join(c.ConfigDir, "core.env")
	if _, err := os.Stat(coreEnv); os.IsNotExist(err) {
		c.addItem(report, "config.core.env", CheckWarn,
			"core.env not found, using defaults",
			fmt.Sprintf("Expected at: %s", coreEnv))
	} else {
		c.addItem(report, "config.core.env", CheckOK,
			"core.env exists", "")
	}

	// Check mysql.env if MySQL is enabled
	if c.Config.MySQL.Enabled {
		mysqlEnv := filepath.Join(c.ConfigDir, "mysql.env")
		if _, err := os.Stat(mysqlEnv); os.IsNotExist(err) {
			c.addItem(report, "config.mysql.env", CheckFail,
				"mysql.env not found but MySQL is enabled",
				fmt.Sprintf("Expected at: %s", mysqlEnv))
		} else {
			c.addItem(report, "config.mysql.env", CheckOK,
				"mysql.env exists", "")
		}
	}

	// Check postgresql.env if PostgreSQL is enabled
	if c.Config.PostgreSQL.Enabled {
		pgEnv := filepath.Join(c.ConfigDir, "postgresql.env")
		if _, err := os.Stat(pgEnv); os.IsNotExist(err) {
			c.addItem(report, "config.postgresql.env", CheckFail,
				"postgresql.env not found but PostgreSQL is enabled",
				fmt.Sprintf("Expected at: %s", pgEnv))
		} else {
			c.addItem(report, "config.postgresql.env", CheckOK,
				"postgresql.env exists", "")
		}
	}

	// Check secret.env
	secretEnv := filepath.Join(c.ConfigDir, "secret.env")
	if _, err := os.Stat(secretEnv); os.IsNotExist(err) {
		c.addItem(report, "config.secret.env", CheckWarn,
			"secret.env not found",
			"Passwords must be set via environment variables")
	} else {
		c.addItem(report, "config.secret.env", CheckOK,
			"secret.env exists", "")
	}
}