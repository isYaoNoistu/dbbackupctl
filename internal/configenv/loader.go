package configenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration
type Config struct {
	Core       CoreConfig
	MySQL      MySQLConfig
	PostgreSQL PostgreSQLConfig
	Secret     SecretConfig
}

// Loader handles configuration loading
type Loader struct {
	configDir string
}

// NewLoader creates a new configuration loader
func NewLoader(configDir string) *Loader {
	return &Loader{
		configDir: configDir,
	}
}

// Load loads all configuration files
func (l *Loader) Load() (*Config, error) {
	cfg := &Config{}

	// Load core.env
	if err := l.loadCore(&cfg.Core); err != nil {
		return nil, fmt.Errorf("loading core.env: %w", err)
	}

	// Load mysql.env
	if err := l.loadMySQL(&cfg.MySQL); err != nil {
		return nil, fmt.Errorf("loading mysql.env: %w", err)
	}

	// Load postgresql.env
	if err := l.loadPostgreSQL(&cfg.PostgreSQL); err != nil {
		return nil, fmt.Errorf("loading postgresql.env: %w", err)
	}

	// Load secret.env
	if err := l.loadSecret(&cfg.Secret); err != nil {
		return nil, fmt.Errorf("loading secret.env: %w", err)
	}

	// Apply defaults
	applyCoreDefaults(&cfg.Core)

	return cfg, nil
}

// loadCore loads core.env configuration
func (l *Loader) loadCore(cfg *CoreConfig) error {
	path := filepath.Join(l.configDir, "core.env")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // core.env is optional
	}

	env, err := godotenv.Read(path)
	if err != nil {
		return err
	}

	// Map environment variables to config
	mapCoreConfig(cfg, env)

	return nil
}

// loadMySQL loads mysql.env configuration
func (l *Loader) loadMySQL(cfg *MySQLConfig) error {
	path := filepath.Join(l.configDir, "mysql.env")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // mysql.env is optional
	}

	env, err := godotenv.Read(path)
	if err != nil {
		return err
	}

	// Map environment variables to config
	mapMySQLConfig(cfg, env)

	return nil
}

// loadPostgreSQL loads postgresql.env configuration
func (l *Loader) loadPostgreSQL(cfg *PostgreSQLConfig) error {
	path := filepath.Join(l.configDir, "postgresql.env")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // postgresql.env is optional
	}

	env, err := godotenv.Read(path)
	if err != nil {
		return err
	}

	// Map environment variables to config
	mapPostgreSQLConfig(cfg, env)

	return nil
}

// loadSecret loads secret.env configuration
func (l *Loader) loadSecret(cfg *SecretConfig) error {
	path := filepath.Join(l.configDir, "secret.env")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // secret.env is optional
	}

	env, err := godotenv.Read(path)
	if err != nil {
		return err
	}

	// Map environment variables to config
	mapSecretConfig(cfg, env)

	return nil
}

// mapCoreConfig maps environment variables to CoreConfig
func mapCoreConfig(cfg *CoreConfig, env map[string]string) {
	if v, ok := env["DBB_CONFIG_DIR"]; ok {
		cfg.ConfigDir = v
	}
	if v, ok := env["DBB_DATA_DIR"]; ok {
		cfg.DataDir = v
	}
	if v, ok := env["DBB_BACKUP_ROOT"]; ok {
		cfg.BackupRoot = v
	}
	if v, ok := env["DBB_TMP_DIR"]; ok {
		cfg.TmpDir = v
	}
	if v, ok := env["DBB_LOG_DIR"]; ok {
		cfg.LogDir = v
	}
	if v, ok := env["DBB_LOCK_DIR"]; ok {
		cfg.LockDir = v
	}
	if v, ok := env["DBB_TIMEZONE"]; ok {
		cfg.Timezone = v
	}
	if v, ok := env["DBB_LOG_FORMAT"]; ok {
		cfg.LogFormat = v
	}
	if v, ok := env["DBB_LOG_LEVEL"]; ok {
		cfg.LogLevel = v
	}
	if v, ok := env["DBB_DEFAULT_TIMEOUT"]; ok {
		cfg.DefaultTimeout = v
	}
	if v, ok := env["DBB_COMPRESS_ENABLED"]; ok {
		cfg.CompressEnabled = v == "true"
	}
	if v, ok := env["DBB_COMPRESS_TYPE"]; ok {
		cfg.CompressType = v
	}
	if v, ok := env["DBB_COMPRESS_LEVEL"]; ok {
		cfg.CompressLevel = parseInt(v, 3)
	}
	if v, ok := env["DBB_COMPRESS_THREADS"]; ok {
		cfg.CompressThreads = parseInt(v, 0)
	}
	if v, ok := env["DBB_STREAM_COMPRESS"]; ok {
		cfg.StreamCompress = v == "true"
	}
	if v, ok := env["DBB_CHECKSUM_ENABLED"]; ok {
		cfg.ChecksumEnabled = v == "true"
	}
	if v, ok := env["DBB_CHECKSUM_TYPE"]; ok {
		cfg.ChecksumType = v
	}
	if v, ok := env["DBB_RETENTION_KEEP_LAST"]; ok {
		cfg.RetentionKeepLast = parseInt(v, 7)
	}
	if v, ok := env["DBB_RETENTION_KEEP_DAYS"]; ok {
		cfg.RetentionKeepDays = parseInt(v, 7)
	}
	if v, ok := env["DBB_RETENTION_KEEP_FAILED_LAST"]; ok {
		cfg.RetentionKeepFailedLast = parseInt(v, 3)
	}
	if v, ok := env["DBB_RETENTION_MAX_TOTAL_SIZE"]; ok {
		cfg.RetentionMaxTotalSize = v
	}
	if v, ok := env["DBB_RETENTION_PRUNE_BEFORE_BACKUP"]; ok {
		cfg.RetentionPruneBeforeBackup = v == "true"
	}
	if v, ok := env["DBB_RETENTION_PRUNE_AFTER_BACKUP"]; ok {
		cfg.RetentionPruneAfterBackup = v == "true"
	}
	if v, ok := env["DBB_DISK_GUARD_ENABLED"]; ok {
		cfg.DiskGuardEnabled = v == "true"
	}
	if v, ok := env["DBB_DISK_MIN_FREE_SIZE"]; ok {
		cfg.DiskMinFreeSize = v
	}
	if v, ok := env["DBB_DISK_MIN_FREE_PERCENT"]; ok {
		cfg.DiskMinFreePercent = parseInt(v, 15)
	}
	if v, ok := env["DBB_DISK_ESTIMATE_BUFFER_PERCENT"]; ok {
		cfg.DiskEstimateBufferPercent = parseInt(v, 20)
	}
	if v, ok := env["DBB_INDEX_FILE"]; ok {
		cfg.IndexFile = v
	}
	if v, ok := env["DBB_COMMAND_LOG_FILE"]; ok {
		cfg.CommandLogFile = v
	}
	if v, ok := env["DBB_RESTORE_LOG_FILE"]; ok {
		cfg.RestoreLogFile = v
	}
}

// mapMySQLConfig maps environment variables to MySQLConfig
func mapMySQLConfig(cfg *MySQLConfig, env map[string]string) {
	// Initialize JobConfigs map if nil
	if cfg.JobConfigs == nil {
		cfg.JobConfigs = make(map[string]MySQLJobConfig)
	}

	if v, ok := env["MYSQL_ENABLED"]; ok {
		cfg.Enabled = v == "true"
	}
	if v, ok := env["MYSQL_JOBS"]; ok {
		cfg.Jobs = parseStringList(v)
	}

	// Parse job configurations
	for _, job := range cfg.Jobs {
		jobCfg := MySQLJobConfig{}
		prefix := fmt.Sprintf("MYSQL_%s_", strings.ToUpper(job))

		if v, ok := env[prefix+"ENABLED"]; ok {
			jobCfg.Enabled = v == "true"
		}
		if v, ok := env[prefix+"HOST"]; ok {
			jobCfg.Host = v
		}
		if v, ok := env[prefix+"PORT"]; ok {
			jobCfg.Port = parseInt(v, 3306)
		}
		if v, ok := env[prefix+"USER"]; ok {
			jobCfg.User = v
		}
		if v, ok := env[prefix+"PASSWORD_ENV"]; ok {
			jobCfg.PasswordEnv = v
		}
		if v, ok := env[prefix+"DATABASES"]; ok {
			jobCfg.Databases = parseStringList(v)
		}
		if v, ok := env[prefix+"INCLUDE_SYSTEM_DATABASES"]; ok {
			jobCfg.IncludeSystemDatabases = v == "true"
		}
		if v, ok := env[prefix+"BACKUP_MODE"]; ok {
			jobCfg.BackupMode = v
		}
		if v, ok := env[prefix+"OUTPUT_MODE"]; ok {
			jobCfg.OutputMode = v
		}
		if v, ok := env[prefix+"BACKUP_DIR"]; ok {
			jobCfg.BackupDir = v
		}
		if v, ok := env[prefix+"SINGLE_TRANSACTION"]; ok {
			jobCfg.SingleTransaction = v == "true"
		}
		if v, ok := env[prefix+"QUICK"]; ok {
			jobCfg.Quick = v == "true"
		}
		if v, ok := env[prefix+"ROUTINES"]; ok {
			jobCfg.Routines = v == "true"
		}
		if v, ok := env[prefix+"EVENTS"]; ok {
			jobCfg.Events = v == "true"
		}
		if v, ok := env[prefix+"TRIGGERS"]; ok {
			jobCfg.Triggers = v == "true"
		}
		if v, ok := env[prefix+"HEX_BLOB"]; ok {
			jobCfg.HexBlob = v == "true"
		}
		if v, ok := env[prefix+"SET_GTID_PURGED"]; ok {
			jobCfg.SetGtidPurged = v
		}
		if v, ok := env[prefix+"COLUMN_STATISTICS"]; ok {
			jobCfg.ColumnStatistics = v == "true"
		}
		if v, ok := env[prefix+"LOCK_TABLES"]; ok {
			jobCfg.LockTables = v == "true"
		}
		if v, ok := env[prefix+"DUMP_CREATE_DATABASE"]; ok {
			jobCfg.DumpCreateDatabase = v == "true"
		}
		if v, ok := env[prefix+"RESTORE_HOST"]; ok {
			jobCfg.RestoreHost = v
		}
		if v, ok := env[prefix+"RESTORE_PORT"]; ok {
			jobCfg.RestorePort = parseInt(v, 3306)
		}
		if v, ok := env[prefix+"RESTORE_USER"]; ok {
			jobCfg.RestoreUser = v
		}
		if v, ok := env[prefix+"RESTORE_PASSWORD_ENV"]; ok {
			jobCfg.RestorePasswordEnv = v
		}
		if v, ok := env[prefix+"RETENTION_KEEP_LAST"]; ok {
			jobCfg.RetentionKeepLast = parseInt(v, 0)
		}
		if v, ok := env[prefix+"RETENTION_KEEP_DAYS"]; ok {
			jobCfg.RetentionKeepDays = parseInt(v, 0)
		}
		if v, ok := env[prefix+"RETENTION_MAX_TOTAL_SIZE"]; ok {
			jobCfg.RetentionMaxTotalSize = v
		}

		cfg.JobConfigs[job] = jobCfg
	}
}

// mapPostgreSQLConfig maps environment variables to PostgreSQLConfig
func mapPostgreSQLConfig(cfg *PostgreSQLConfig, env map[string]string) {
	// Initialize JobConfigs map if nil
	if cfg.JobConfigs == nil {
		cfg.JobConfigs = make(map[string]PostgreSQLJobConfig)
	}

	if v, ok := env["POSTGRES_ENABLED"]; ok {
		cfg.Enabled = v == "true"
	}
	if v, ok := env["POSTGRES_JOBS"]; ok {
		cfg.Jobs = parseStringList(v)
	}

	// Parse job configurations
	for _, job := range cfg.Jobs {
		jobCfg := PostgreSQLJobConfig{}
		prefix := fmt.Sprintf("POSTGRES_%s_", strings.ToUpper(job))

		if v, ok := env[prefix+"ENABLED"]; ok {
			jobCfg.Enabled = v == "true"
		}
		if v, ok := env[prefix+"HOST"]; ok {
			jobCfg.Host = v
		}
		if v, ok := env[prefix+"PORT"]; ok {
			jobCfg.Port = parseInt(v, 5432)
		}
		if v, ok := env[prefix+"USER"]; ok {
			jobCfg.User = v
		}
		if v, ok := env[prefix+"PASSWORD_ENV"]; ok {
			jobCfg.PasswordEnv = v
		}
		if v, ok := env[prefix+"SSLMODE"]; ok {
			jobCfg.SSLMode = v
		}
		if v, ok := env[prefix+"DATABASES"]; ok {
			jobCfg.Databases = parseStringList(v)
		}
		if v, ok := env[prefix+"INCLUDE_TEMPLATE_DATABASES"]; ok {
			jobCfg.IncludeTemplateDatabases = v == "true"
		}
		if v, ok := env[prefix+"INCLUDE_POSTGRES_DATABASE"]; ok {
			jobCfg.IncludePostgresDatabase = v == "true"
		}
		if v, ok := env[prefix+"BACKUP_MODE"]; ok {
			jobCfg.BackupMode = v
		}
		if v, ok := env[prefix+"DUMP_FORMAT"]; ok {
			jobCfg.DumpFormat = v
		}
		if v, ok := env[prefix+"INCLUDE_GLOBALS"]; ok {
			jobCfg.IncludeGlobals = v == "true"
		}
		if v, ok := env[prefix+"NO_OWNER"]; ok {
			jobCfg.NoOwner = v == "true"
		}
		if v, ok := env[prefix+"NO_PRIVILEGES"]; ok {
			jobCfg.NoPrivileges = v == "true"
		}
		if v, ok := env[prefix+"JOBS"]; ok {
			jobCfg.Jobs = parseInt(v, 1)
		}
		if v, ok := env[prefix+"BACKUP_DIR"]; ok {
			jobCfg.BackupDir = v
		}
		if v, ok := env[prefix+"RESTORE_HOST"]; ok {
			jobCfg.RestoreHost = v
		}
		if v, ok := env[prefix+"RESTORE_PORT"]; ok {
			jobCfg.RestorePort = parseInt(v, 5432)
		}
		if v, ok := env[prefix+"RESTORE_USER"]; ok {
			jobCfg.RestoreUser = v
		}
		if v, ok := env[prefix+"RESTORE_PASSWORD_ENV"]; ok {
			jobCfg.RestorePasswordEnv = v
		}
		if v, ok := env[prefix+"RESTORE_SSLMODE"]; ok {
			jobCfg.RestoreSSLMode = v
		}
		if v, ok := env[prefix+"RETENTION_KEEP_LAST"]; ok {
			jobCfg.RetentionKeepLast = parseInt(v, 0)
		}
		if v, ok := env[prefix+"RETENTION_KEEP_DAYS"]; ok {
			jobCfg.RetentionKeepDays = parseInt(v, 0)
		}
		if v, ok := env[prefix+"RETENTION_MAX_TOTAL_SIZE"]; ok {
			jobCfg.RetentionMaxTotalSize = v
		}

		cfg.JobConfigs[job] = jobCfg
	}
}

// mapSecretConfig maps environment variables to SecretConfig
func mapSecretConfig(cfg *SecretConfig, env map[string]string) {
	// Store all password values
	cfg.Passwords = make(map[string]string)
	for k, v := range env {
		if strings.HasSuffix(k, "_PASSWORD") || strings.HasSuffix(k, "_KEY") {
			cfg.Passwords[k] = v
		}
	}
}

// applyCoreDefaults applies default values to CoreConfig
func applyCoreDefaults(cfg *CoreConfig) {
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = "/etc/dbbackupctl"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data/dbbackupctl"
	}
	if cfg.BackupRoot == "" {
		cfg.BackupRoot = "/data/backup"
	}
	if cfg.TmpDir == "" {
		cfg.TmpDir = "/data/dbbackupctl/tmp"
	}
	if cfg.LogDir == "" {
		cfg.LogDir = "/var/log/dbbackupctl"
	}
	if cfg.LockDir == "" {
		cfg.LockDir = "/var/lock/dbbackupctl"
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "json"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.DefaultTimeout == "" {
		cfg.DefaultTimeout = "6h"
	}
	if cfg.CompressType == "" {
		cfg.CompressType = "zstd"
	}
	if cfg.CompressLevel == 0 {
		cfg.CompressLevel = 3
	}
	if cfg.ChecksumType == "" {
		cfg.ChecksumType = "sha256"
	}
	if cfg.RetentionKeepLast == 0 {
		cfg.RetentionKeepLast = 7
	}
	if cfg.RetentionKeepDays == 0 {
		cfg.RetentionKeepDays = 7
	}
	if cfg.RetentionKeepFailedLast == 0 {
		cfg.RetentionKeepFailedLast = 3
	}
	if cfg.RetentionMaxTotalSize == "" {
		cfg.RetentionMaxTotalSize = "300G"
	}
	if cfg.DiskMinFreeSize == "" {
		cfg.DiskMinFreeSize = "20G"
	}
	if cfg.DiskMinFreePercent == 0 {
		cfg.DiskMinFreePercent = 15
	}
	if cfg.DiskEstimateBufferPercent == 0 {
		cfg.DiskEstimateBufferPercent = 20
	}
	if cfg.IndexFile == "" {
		cfg.IndexFile = "/data/dbbackupctl/index/backup_records.jsonl"
	}
	if cfg.CommandLogFile == "" {
		cfg.CommandLogFile = "/data/dbbackupctl/index/command_runs.jsonl"
	}
	if cfg.RestoreLogFile == "" {
		cfg.RestoreLogFile = "/data/dbbackupctl/index/restore_records.jsonl"
	}
}

// parseInt parses a string to int with default value
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return defaultVal
	}
	return val
}

// parseStringList parses a comma-separated string to a slice
func parseStringList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}