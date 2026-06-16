package configenv

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExampleConfigsLoadMultipleJobs(t *testing.T) {
	configDir := t.TempDir()
	copyExample(t, "core.env.example", filepath.Join(configDir, "core.env"))
	copyExample(t, "mysql.env.example", filepath.Join(configDir, "mysql.env"))
	copyExample(t, "postgresql.env.example", filepath.Join(configDir, "postgresql.env"))
	copyExample(t, "secret.env.example", filepath.Join(configDir, "secret.env"))

	cfg, err := NewLoader(configDir).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := NewValidator().Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if want := []string{"dev", "prod"}; !reflect.DeepEqual(cfg.MySQL.Jobs, want) {
		t.Fatalf("MySQL jobs = %v, want %v", cfg.MySQL.Jobs, want)
	}
	if want := []string{"dev", "prod"}; !reflect.DeepEqual(cfg.PostgreSQL.Jobs, want) {
		t.Fatalf("PostgreSQL jobs = %v, want %v", cfg.PostgreSQL.Jobs, want)
	}

	mysqlDev := cfg.MySQL.JobConfigs["dev"]
	if mysqlDev.Host != "127.0.0.1" || mysqlDev.PasswordEnv != "MYSQL_DEV_PASSWORD" {
		t.Fatalf("unexpected mysql dev config: %+v", mysqlDev)
	}
	mysqlProd := cfg.MySQL.JobConfigs["prod"]
	if mysqlProd.BackupDir != "/data/backup/mysql/prod" || mysqlProd.RestorePasswordEnv != "MYSQL_PROD_RESTORE_PASSWORD" {
		t.Fatalf("unexpected mysql prod config: %+v", mysqlProd)
	}

	postgresDev := cfg.PostgreSQL.JobConfigs["dev"]
	if postgresDev.DumpFormat != "custom" || !postgresDev.IncludeGlobals {
		t.Fatalf("unexpected postgresql dev config: %+v", postgresDev)
	}
	postgresProd := cfg.PostgreSQL.JobConfigs["prod"]
	if postgresProd.BackupDir != "/data/backup/postgresql/prod" || postgresProd.RestoreSSLMode != "disable" {
		t.Fatalf("unexpected postgresql prod config: %+v", postgresProd)
	}

	if got := cfg.Secret.Passwords["MYSQL_DEV_PASSWORD"]; got == "" {
		t.Fatalf("MYSQL_DEV_PASSWORD was not loaded from secret.env")
	}
	if got := cfg.Secret.Passwords["POSTGRES_PROD_RESTORE_PASSWORD"]; got == "" {
		t.Fatalf("POSTGRES_PROD_RESTORE_PASSWORD was not loaded from secret.env")
	}
}

func copyExample(t *testing.T, name, dest string) {
	t.Helper()

	src := filepath.Join("..", "..", "configs", name)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dest, data, 0600); err != nil {
		t.Fatalf("write %s: %v", dest, err)
	}
}
