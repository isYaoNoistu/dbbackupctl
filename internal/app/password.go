package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/isYaoNoistu/dbbackupctl/internal/configenv"
)

// ResolvePassword resolves password from environment or secret config
// Priority: 1. System env var 2. secret.env 3. password file
func ResolvePassword(secret configenv.SecretConfig, envName, filePath string) (string, error) {
	// Try system environment variable first
	if envName != "" {
		if v := os.Getenv(envName); v != "" {
			return v, nil
		}

		// Try secret.env
		if v, ok := secret.GetPassword(envName); ok && v != "" {
			return v, nil
		}
	}

	// Try password file
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading password file %s: %w", filePath, err)
		}
		return strings.TrimSpace(string(b)), nil
	}

	return "", fmt.Errorf("password not found: env=%s file=%s", envName, filePath)
}

// ResolveMySQLPassword resolves MySQL password for a job
func ResolveMySQLPassword(cfg *configenv.Config, jobName string, isRestore bool) (string, error) {
	job, ok := cfg.MySQL.JobConfigs[jobName]
	if !ok {
		return "", fmt.Errorf("mysql job %s not found", jobName)
	}

	envName := job.PasswordEnv
	if isRestore && job.RestorePasswordEnv != "" {
		envName = job.RestorePasswordEnv
	}

	return ResolvePassword(cfg.Secret, envName, "")
}

// ResolvePostgreSQLPassword resolves PostgreSQL password for a job
func ResolvePostgreSQLPassword(cfg *configenv.Config, jobName string, isRestore bool) (string, error) {
	job, ok := cfg.PostgreSQL.JobConfigs[jobName]
	if !ok {
		return "", fmt.Errorf("postgresql job %s not found", jobName)
	}

	envName := job.PasswordEnv
	if isRestore && job.RestorePasswordEnv != "" {
		envName = job.RestorePasswordEnv
	}

	return ResolvePassword(cfg.Secret, envName, "")
}
