package postgresql

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
	"github.com/jackc/pgx/v5"
)

// Inspector handles PostgreSQL inspection operations
type Inspector struct{}

// NewInspector creates a new PostgreSQL inspector
func NewInspector() *Inspector {
	return &Inspector{}
}

// CheckDependency checks if pg_dump, pg_restore, pg_dumpall, psql are available
func (i *Inspector) CheckDependency(ctx context.Context) error {
	// Check pg_dump
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("PATH 中未找到 pg_dump: %w", err)
	}

	// Check pg_restore
	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("PATH 中未找到 pg_restore: %w", err)
	}

	// Check pg_dumpall
	if _, err := exec.LookPath("pg_dumpall"); err != nil {
		return fmt.Errorf("PATH 中未找到 pg_dumpall: %w", err)
	}

	// Check psql
	if _, err := exec.LookPath("psql"); err != nil {
		return fmt.Errorf("PATH 中未找到 psql: %w", err)
	}

	return nil
}

// CheckConnection checks if PostgreSQL connection is working
func (i *Inspector) CheckConnection(ctx context.Context, job engine.JobConfig) error {
	connStr := buildConnStr(job)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL Ping 失败: %w", err)
	}

	return nil
}

// GetDatabases returns a list of databases
func (i *Inspector) GetDatabases(ctx context.Context, job engine.JobConfig, includeTemplate, includePostgres bool) ([]string, error) {
	connStr := buildConnStr(job)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	defer conn.Close(ctx)

	query := `
		SELECT datname 
		FROM pg_database 
		WHERE datistemplate = false
		AND datname != 'postgres'
	`
	if includeTemplate {
		query = `
			SELECT datname 
			FROM pg_database 
			WHERE datname != 'postgres'
		`
	}
	if includePostgres {
		query = "SELECT datname FROM pg_database"
	}

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询数据库列表失败: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			return nil, fmt.Errorf("读取数据库列表失败: %w", err)
		}
		databases = append(databases, db)
	}

	return databases, nil
}

// EstimateSize estimates the backup size in bytes
func (i *Inspector) EstimateSize(ctx context.Context, job engine.JobConfig, databases []string) (int64, error) {
	connStr := buildConnStr(job)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return 0, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	defer conn.Close(ctx)

	var totalSize int64
	for _, db := range databases {
		var size int64
		err := conn.QueryRow(ctx, "SELECT pg_database_size($1)", db).Scan(&size)
		if err != nil {
			return 0, fmt.Errorf("估算 %s 大小失败: %w", db, err)
		}
		totalSize += size
	}

	return totalSize, nil
}

// buildConnStr builds PostgreSQL connection string from job config
func buildConnStr(job engine.JobConfig) string {
	sslMode := "disable"
	if opts, ok := job.Options["sslmode"]; ok {
		sslMode = opts.(string)
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		job.Host, job.Port, job.User, job.Password, sslMode)
}
