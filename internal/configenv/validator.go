package configenv

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Validator handles configuration validation
type Validator struct{}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the entire configuration
func (v *Validator) Validate(cfg *Config) error {
	if err := v.validateCore(&cfg.Core); err != nil {
		return fmt.Errorf("core config: %w", err)
	}

	if cfg.MySQL.Enabled {
		if err := v.validateMySQL(&cfg.MySQL); err != nil {
			return fmt.Errorf("mysql config: %w", err)
		}
	}

	if cfg.PostgreSQL.Enabled {
		if err := v.validatePostgreSQL(&cfg.PostgreSQL); err != nil {
			return fmt.Errorf("postgresql config: %w", err)
		}
	}

	return nil
}

// validateCore validates core configuration
func (v *Validator) validateCore(cfg *CoreConfig) error {
	// Validate compress type
	validCompressTypes := map[string]bool{"zstd": true, "gzip": true, "none": true}
	if !validCompressTypes[cfg.CompressType] {
		return fmt.Errorf("invalid compress_type: %s (must be zstd, gzip, or none)", cfg.CompressType)
	}

	// Validate checksum type
	if cfg.ChecksumType != "sha256" {
		return fmt.Errorf("invalid checksum_type: %s (must be sha256)", cfg.ChecksumType)
	}

	// Validate compress level
	if cfg.CompressType == "zstd" && (cfg.CompressLevel < 1 || cfg.CompressLevel > 22) {
		return fmt.Errorf("invalid compress_level for zstd: %d (must be 1-22)", cfg.CompressLevel)
	}
	if cfg.CompressType == "gzip" && (cfg.CompressLevel < 1 || cfg.CompressLevel > 9) {
		return fmt.Errorf("invalid compress_level for gzip: %d (must be 1-9)", cfg.CompressLevel)
	}

	return nil
}

// validateMySQL validates MySQL configuration
func (v *Validator) validateMySQL(cfg *MySQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("MYSQL_ENABLED=true but MYSQL_JOBS is empty")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("job %s not found in configuration", jobName)
		}

		if err := v.validateMySQLJob(jobName, &job); err != nil {
			return err
		}
	}

	return nil
}

// validateMySQLJob validates a MySQL job configuration
func (v *Validator) validateMySQLJob(name string, cfg *MySQLJobConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("job %s: HOST is required", name)
	}
	if cfg.Port == 0 {
		return fmt.Errorf("job %s: PORT is required", name)
	}
	if cfg.User == "" {
		return fmt.Errorf("job %s: USER is required", name)
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("job %s: PASSWORD_ENV is required", name)
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("job %s: DATABASES is required", name)
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("job %s: BACKUP_DIR is required", name)
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("job %s: %w", name, err)
	}

	return nil
}

// validatePostgreSQL validates PostgreSQL configuration
func (v *Validator) validatePostgreSQL(cfg *PostgreSQLConfig) error {
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("POSTGRES_ENABLED=true but POSTGRES_JOBS is empty")
	}

	for _, jobName := range cfg.Jobs {
		job, ok := cfg.JobConfigs[jobName]
		if !ok {
			return fmt.Errorf("job %s not found in configuration", jobName)
		}

		if err := v.validatePostgreSQLJob(jobName, &job); err != nil {
			return err
		}
	}

	return nil
}

// validatePostgreSQLJob validates a PostgreSQL job configuration
func (v *Validator) validatePostgreSQLJob(name string, cfg *PostgreSQLJobConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("job %s: HOST is required", name)
	}
	if cfg.Port == 0 {
		return fmt.Errorf("job %s: PORT is required", name)
	}
	if cfg.User == "" {
		return fmt.Errorf("job %s: USER is required", name)
	}
	if cfg.PasswordEnv == "" {
		return fmt.Errorf("job %s: PASSWORD_ENV is required", name)
	}
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("job %s: DATABASES is required", name)
	}
	if cfg.BackupDir == "" {
		return fmt.Errorf("job %s: BACKUP_DIR is required", name)
	}

	// Validate dump format
	validFormats := map[string]bool{"custom": true, "plain": true}
	if !validFormats[cfg.DumpFormat] {
		return fmt.Errorf("job %s: invalid DUMP_FORMAT: %s (must be custom or plain)", name, cfg.DumpFormat)
	}

	// Validate backup dir is not a dangerous path
	if err := v.validateBackupDir(cfg.BackupDir); err != nil {
		return fmt.Errorf("job %s: %w", name, err)
	}

	return nil
}

// validateBackupDir validates backup directory is not a dangerous path
func (v *Validator) validateBackupDir(dir string) error {
	// Clean the path
	cleaned := filepath.Clean(dir)

	// Check for dangerous paths
	dangerousPaths := []string{"/", "/data", "/tmp", "/var", "/etc", "/usr", "/home"}
	for _, dangerous := range dangerousPaths {
		if cleaned == dangerous {
			return fmt.Errorf("backup_dir cannot be a dangerous path: %s", dir)
		}
	}

	// Check if path is too short
	if len(strings.Split(cleaned, "/")) < 4 {
		return fmt.Errorf("backup_dir path is too short: %s", dir)
	}

	return nil
}