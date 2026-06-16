package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/isYaoNoistu/dbbackupctl/internal/engine"
)

// Inspector handles MySQL inspection operations
type Inspector struct{}

// NewInspector creates a new MySQL inspector
func NewInspector() *Inspector {
	return &Inspector{}
}

// CheckDependency checks if mysqldump and mysql are available
func (i *Inspector) CheckDependency(ctx context.Context) error {
	// Check mysqldump
	if _, err := exec.LookPath("mysqldump"); err != nil {
		return fmt.Errorf("PATH 中未找到 mysqldump: %w", err)
	}

	// Check mysql
	if _, err := exec.LookPath("mysql"); err != nil {
		return fmt.Errorf("PATH 中未找到 mysql: %w", err)
	}

	return nil
}

// CheckConnection checks if MySQL connection is working
func (i *Inspector) CheckConnection(ctx context.Context, job engine.JobConfig) error {
	dsn := buildDSN(job)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("MySQL Ping 失败: %w", err)
	}

	return nil
}

// GetDatabases returns a list of databases
func (i *Inspector) GetDatabases(ctx context.Context, job engine.JobConfig, includeSystem bool) ([]string, error) {
	dsn := buildDSN(job)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}
	defer db.Close()

	query := "SHOW DATABASES"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询数据库列表失败: %w", err)
	}
	defer rows.Close()

	var databases []string
	systemDatabases := map[string]bool{
		"information_schema": true,
		"mysql":              true,
		"performance_schema": true,
		"sys":                true,
	}

	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			return nil, fmt.Errorf("读取数据库列表失败: %w", err)
		}

		if !includeSystem && systemDatabases[db] {
			continue
		}

		databases = append(databases, db)
	}

	return databases, nil
}

// EstimateSize estimates the backup size in bytes
func (i *Inspector) EstimateSize(ctx context.Context, job engine.JobConfig, databases []string) (int64, error) {
	dsn := buildDSN(job)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return 0, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}
	defer db.Close()

	placeholders := make([]string, len(databases))
	args := make([]interface{}, len(databases))
	for i, d := range databases {
		placeholders[i] = "?"
		args[i] = d
	}

	query := fmt.Sprintf(`
		SELECT table_schema, SUM(data_length + index_length) as size
		FROM information_schema.tables
		WHERE table_schema IN (%s)
		GROUP BY table_schema
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("估算大小失败: %w", err)
	}
	defer rows.Close()

	var totalSize int64
	for rows.Next() {
		var schema string
		var size int64
		if err := rows.Scan(&schema, &size); err != nil {
			return 0, fmt.Errorf("读取大小结果失败: %w", err)
		}
		totalSize += size
	}

	return totalSize, nil
}

// buildDSN builds MySQL DSN from job config
func buildDSN(job engine.JobConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=10s",
		job.User, job.Password, job.Host, job.Port)
}
