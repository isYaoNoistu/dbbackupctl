package app

import (
	"fmt"
	"time"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
)

// NewBackupID generates a new backup ID
func NewBackupID(dbType, job string, t time.Time) string {
	return fmt.Sprintf("%s-%s-%s", dbType, job, t.Format("20060102-150405"))
}

// ConvertMySQLJob converts MySQL job config to engine job config
func ConvertMySQLJob(cfg *configenv.Config, jobName string) (engine.JobConfig, engine.BackupTarget, error) {
	job, ok := cfg.MySQL.JobConfigs[jobName]
	if !ok {
		return engine.JobConfig{}, engine.BackupTarget{}, fmt.Errorf("mysql job %s not found", jobName)
	}

	// Resolve password
	password, err := ResolveMySQLPassword(cfg, jobName, false)
	if err != nil {
		return engine.JobConfig{}, engine.BackupTarget{}, fmt.Errorf("resolving password: %w", err)
	}

	// Build engine job config
	engineJob := engine.JobConfig{
		Name:     jobName,
		Host:     job.Host,
		Port:     job.Port,
		User:     job.User,
		Password: password,
		Options: map[string]interface{}{
			"single_transaction":    job.SingleTransaction,
			"quick":                 job.Quick,
			"routines":              job.Routines,
			"events":                job.Events,
			"triggers":              job.Triggers,
			"hex_blob":              job.HexBlob,
			"set_gtid_purged":       job.SetGtidPurged,
			"column_statistics":     job.ColumnStatistics,
			"lock_tables":           job.LockTables,
			"dump_create_database":  job.DumpCreateDatabase,
			"output_mode":           job.OutputMode,
		},
	}

	// Build backup target
	now := time.Now()
	backupID := NewBackupID("mysql", jobName, now)
	backupDir := fmt.Sprintf("%s/%s", job.BackupDir, now.Format("20060102_150405"))

	target := engine.BackupTarget{
		BackupDir: backupDir,
		Databases: job.Databases,
		Timestamp: now.Format("20060102-150405"),
		BackupID:  backupID,
		Compression: engine.CompressionConfig{
			Enabled: cfg.Core.CompressEnabled,
			Type:    cfg.Core.CompressType,
			Level:   cfg.Core.CompressLevel,
			Threads: cfg.Core.CompressThreads,
		},
	}

	return engineJob, target, nil
}

// ConvertPostgreSQLJob converts PostgreSQL job config to engine job config
func ConvertPostgreSQLJob(cfg *configenv.Config, jobName string) (engine.JobConfig, engine.BackupTarget, error) {
	job, ok := cfg.PostgreSQL.JobConfigs[jobName]
	if !ok {
		return engine.JobConfig{}, engine.BackupTarget{}, fmt.Errorf("postgresql job %s not found", jobName)
	}

	// Resolve password
	password, err := ResolvePostgreSQLPassword(cfg, jobName, false)
	if err != nil {
		return engine.JobConfig{}, engine.BackupTarget{}, fmt.Errorf("resolving password: %w", err)
	}

	// Build engine job config
	engineJob := engine.JobConfig{
		Name:     jobName,
		Host:     job.Host,
		Port:     job.Port,
		User:     job.User,
		Password: password,
		Options: map[string]interface{}{
			"sslmode":                job.SSLMode,
			"dump_format":            job.DumpFormat,
			"include_globals":        job.IncludeGlobals,
			"no_owner":               job.NoOwner,
			"no_privileges":          job.NoPrivileges,
			"jobs":                   job.Jobs,
			"include_template_dbs":   job.IncludeTemplateDatabases,
			"include_postgres_db":    job.IncludePostgresDatabase,
		},
	}

	// Build backup target
	now := time.Now()
	backupID := NewBackupID("postgresql", jobName, now)
	backupDir := fmt.Sprintf("%s/%s", job.BackupDir, now.Format("20060102_150405"))

	target := engine.BackupTarget{
		BackupDir: backupDir,
		Databases: job.Databases,
		Timestamp: now.Format("20060102-150405"),
		BackupID:  backupID,
		Compression: engine.CompressionConfig{
			Enabled: cfg.Core.CompressEnabled,
			Type:    cfg.Core.CompressType,
			Level:   cfg.Core.CompressLevel,
			Threads: cfg.Core.CompressThreads,
		},
	}

	return engineJob, target, nil
}

// ConvertMySQLRestoreJob converts MySQL job config for restore
func ConvertMySQLRestoreJob(cfg *configenv.Config, jobName string) (engine.JobConfig, error) {
	job, ok := cfg.MySQL.JobConfigs[jobName]
	if !ok {
		return engine.JobConfig{}, fmt.Errorf("mysql job %s not found", jobName)
	}

	// Resolve restore password
	password, err := ResolveMySQLPassword(cfg, jobName, true)
	if err != nil {
		return engine.JobConfig{}, fmt.Errorf("resolving restore password: %w", err)
	}

	// Use restore connection if configured
	host := job.Host
	port := job.Port
	user := job.User
	if job.RestoreHost != "" {
		host = job.RestoreHost
	}
	if job.RestorePort != 0 {
		port = job.RestorePort
	}
	if job.RestoreUser != "" {
		user = job.RestoreUser
	}

	return engine.JobConfig{
		Name:     jobName,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}, nil
}

// ConvertPostgreSQLRestoreJob converts PostgreSQL job config for restore
func ConvertPostgreSQLRestoreJob(cfg *configenv.Config, jobName string) (engine.JobConfig, error) {
	job, ok := cfg.PostgreSQL.JobConfigs[jobName]
	if !ok {
		return engine.JobConfig{}, fmt.Errorf("postgresql job %s not found", jobName)
	}

	// Resolve restore password
	password, err := ResolvePostgreSQLPassword(cfg, jobName, true)
	if err != nil {
		return engine.JobConfig{}, fmt.Errorf("resolving restore password: %w", err)
	}

	// Use restore connection if configured
	host := job.Host
	port := job.Port
	user := job.User
	sslMode := job.SSLMode
	if job.RestoreHost != "" {
		host = job.RestoreHost
	}
	if job.RestorePort != 0 {
		port = job.RestorePort
	}
	if job.RestoreUser != "" {
		user = job.RestoreUser
	}
	if job.RestoreSSLMode != "" {
		sslMode = job.RestoreSSLMode
	}

	return engine.JobConfig{
		Name:     jobName,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Options: map[string]interface{}{
			"sslmode": sslMode,
		},
	}, nil
}
