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
			"未找到 core.env，将使用默认值",
			fmt.Sprintf("期望位置: %s", coreEnv))
	} else {
		c.addItem(report, "config.core.env", CheckOK,
			"core.env 存在", "")
	}

	// Check mysql.env if MySQL is enabled
	if c.Config.MySQL.Enabled {
		mysqlEnv := filepath.Join(c.ConfigDir, "mysql.env")
		if _, err := os.Stat(mysqlEnv); os.IsNotExist(err) {
			c.addItem(report, "config.mysql.env", CheckFail,
				"MySQL 已启用，但未找到 mysql.env",
				fmt.Sprintf("期望位置: %s", mysqlEnv))
		} else {
			c.addItem(report, "config.mysql.env", CheckOK,
				"mysql.env 存在", "")
		}
	}

	// Check postgresql.env if PostgreSQL is enabled
	if c.Config.PostgreSQL.Enabled {
		pgEnv := filepath.Join(c.ConfigDir, "postgresql.env")
		if _, err := os.Stat(pgEnv); os.IsNotExist(err) {
			c.addItem(report, "config.postgresql.env", CheckFail,
				"PostgreSQL 已启用，但未找到 postgresql.env",
				fmt.Sprintf("期望位置: %s", pgEnv))
		} else {
			c.addItem(report, "config.postgresql.env", CheckOK,
				"postgresql.env 存在", "")
		}
	}

	// Check secret.env
	secretEnv := filepath.Join(c.ConfigDir, "secret.env")
	if _, err := os.Stat(secretEnv); os.IsNotExist(err) {
		c.addItem(report, "config.secret.env", CheckWarn,
			"未找到 secret.env",
			"密码必须通过当前进程环境变量或 password_file 提供")
	} else {
		c.addItem(report, "config.secret.env", CheckOK,
			"secret.env 存在", "")
	}
}
