package checker

import (
	"context"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
)

// Checker performs environment checks
type Checker struct {
	Config     *configenv.Config
	ConfigDir  string
	CheckMySQL bool
	CheckPg    bool
	JobName    string
}

// NewChecker creates a new checker
func NewChecker(cfg *configenv.Config, configDir string) *Checker {
	return &Checker{
		Config:    cfg,
		ConfigDir: configDir,
	}
}

// Run runs all checks
func (c *Checker) Run(ctx context.Context) *CheckReport {
	report := &CheckReport{
		StartedAt: time.Now(),
	}

	// Check configuration files
	c.checkConfigFiles(report)

	// Check directories
	c.checkDirectories(report)

	// Check permissions
	c.checkPermissions(report)

	// Check dependencies
	c.checkDependencies(report)

	// Check disk space
	c.checkDiskSpace(report)

	// Check database connections
	if c.CheckMySQL || (!c.CheckMySQL && !c.CheckPg) {
		c.checkMySQLConnection(ctx, report)
	}
	if c.CheckPg || (!c.CheckMySQL && !c.CheckPg) {
		c.checkPostgreSQLConnection(ctx, report)
	}

	// Check locks
	c.checkLocks(report)

	report.FinishedAt = time.Now()
	return report
}

// addItem adds a check item to the report
func (c *Checker) addItem(report *CheckReport, name string, status CheckStatus, message, detail string) {
	report.Items = append(report.Items, CheckItem{
		Name:    name,
		Status:  status,
		Message: message,
		Detail:  detail,
	})
}