package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed configs/*.env.example
var configTemplates embed.FS

func newInitCmd() *cobra.Command {
	var (
		configDir string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "生成 env 配置模板",
		Long: `生成 dbbackupctl 配置模板。

该命令会创建必要目录和示例配置文件：
  - /etc/dbbackupctl/core.env.example
  - /etc/dbbackupctl/mysql.env.example
  - /etc/dbbackupctl/postgresql.env.example
  - /etc/dbbackupctl/secret.env.example

同时创建数据、备份、日志和锁目录：
  - /data/dbbackupctl
  - /data/backup
  - /var/log/dbbackupctl
  - /var/lock/dbbackupctl`,
		Example: `  dbbackupctl init
  dbbackupctl init --config-dir /etc/dbbackupctl
  dbbackupctl init --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(configDir, force)
		},
	}

	cmd.Flags().StringVar(&configDir, "config-dir", "/etc/dbbackupctl", "配置目录路径")
	cmd.Flags().BoolVar(&force, "force", false, "覆盖已存在的配置模板")

	return cmd
}

func runInit(configDir string, force bool) error {
	// Define directories to create
	dirs := []string{
		configDir,
		"/data/dbbackupctl",
		"/data/dbbackupctl/index",
		"/data/dbbackupctl/tmp",
		"/data/backup",
		"/var/log/dbbackupctl",
		"/var/lock/dbbackupctl",
	}

	// Create directories
	fmt.Println("正在创建目录...")
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
		fmt.Printf("  已创建: %s\n", dir)
	}

	// Define config files to create
	configFiles := []struct {
		name string
		src  string
	}{
		{"core.env.example", "configs/core.env.example"},
		{"mysql.env.example", "configs/mysql.env.example"},
		{"postgresql.env.example", "configs/postgresql.env.example"},
		{"secret.env.example", "configs/secret.env.example"},
	}

	// Create config files
	fmt.Println("\n正在创建配置模板...")
	for _, cf := range configFiles {
		destPath := filepath.Join(configDir, cf.name)

		// Check if file exists
		if _, err := os.Stat(destPath); err == nil && !force {
			fmt.Printf("  已跳过: %s（已存在，使用 --force 可覆盖）\n", destPath)
			continue
		}

		// Read template content
		content, err := configTemplates.ReadFile(cf.src)
		if err != nil {
			return fmt.Errorf("读取模板 %s 失败: %w", cf.src, err)
		}

		// Write file
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", destPath, err)
		}
		fmt.Printf("  已创建: %s\n", destPath)
	}

	fmt.Println("\n初始化完成。")
	fmt.Println("\n下一步：")
	fmt.Println("  1. 根据 dev/prod 等环境修改配置文件：", configDir)
	fmt.Println("  2. 将密码写入 secret.env 或对应的 password_file")
	fmt.Printf("  3. 设置权限：chmod 600 %s/secret.env\n", configDir)
	fmt.Println("  4. 执行 'dbbackupctl check --mysql --job dev' 或 'dbbackupctl check --postgresql --job prod'")

	return nil
}
