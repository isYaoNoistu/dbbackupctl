package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	version    string
	commit     string
	date       string
	configDir  string
	logLevel   string
	jsonOutput bool
)

// Run initializes and executes the root command
func Run(ver, com, dat string) error {
	version = ver
	commit = com
	date = dat

	cobra.AddTemplateFunc("flagUsagesCN", flagUsagesCN)

	rootCmd := &cobra.Command{
		Use:   "dbbackupctl",
		Short: "MySQL/PostgreSQL 本地备份命令行工具",
		Long: `dbbackupctl 是一个运行在数据库服务器本地的 MySQL/PostgreSQL 备份工具。

它不提供 Web UI、Docker 平台、内部元数据库或后台 API 服务。

主要能力：
  - MySQL 和 PostgreSQL 逻辑备份
  - zstd/gzip/none 压缩
  - checksum 校验和 manifest
  - 本地 JSONL 备份索引
  - 按环境名选择任务，例如 --job dev 或 --job prod
  - 保留策略、磁盘保护和任务锁`,
		Example: `  dbbackupctl init
  dbbackupctl check --postgresql --job dev
  dbbackupctl check --postgresql --job prod
  dbbackupctl mysql backup --job dev
  dbbackupctl postgresql backup --job prod
  dbbackupctl show mysql --job dev
  dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute`,
		SilenceUsage: true,
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(newHelpCmd())

	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "/etc/dbbackupctl", "配置目录路径")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "日志级别：debug、info、warn、error")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "以 JSON 格式输出")

	rootCmd.AddCommand(
		newInitCmd(),
		newCheckCmd(),
		newMySQLCmd(),
		newPostgreSQLCmd(),
		newShowCmd(),
		newPruneCmd(),
		newIndexCmd(),
		newVersionCmd(),
	)
	applyChineseTemplates(rootCmd)

	return rootCmd.Execute()
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("dbbackupctl %s\n", version)
			fmt.Printf("  提交: %s\n", commit)
			fmt.Printf("  日期: %s\n", date)
		},
	}
}

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [命令]",
		Short: "显示命令帮助",
		Long:  "显示 dbbackupctl 或指定子命令的帮助信息。",
		RunE: func(cmd *cobra.Command, args []string) error {
			target := cmd.Root()
			if len(args) > 0 {
				found, _, err := target.Find(args)
				if err != nil {
					return err
				}
				target = found
			}
			return target.Help()
		},
	}
}

// GetConfigDir returns the global config directory
func GetConfigDir() string {
	if configDir == "" {
		return "/etc/dbbackupctl"
	}
	return configDir
}

// GetLogLevel returns the global log level
func GetLogLevel() string {
	if logLevel == "" {
		return "info"
	}
	return logLevel
}

// IsJSONOutput returns whether JSON output is enabled
func IsJSONOutput() bool {
	return jsonOutput
}

const chineseUsageTemplate = `用法:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

可用命令:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

参数:
{{flagUsagesCN .LocalFlags}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数:
{{flagUsagesCN .InheritedFlags}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}
`

const chineseHelpTemplate = `{{with .Long}}{{.}}{{else}}{{.Short}}{{end}}

{{.UsageString}}`

func applyChineseTemplates(cmd *cobra.Command) {
	cmd.SetUsageTemplate(chineseUsageTemplate)
	cmd.SetHelpTemplate(chineseHelpTemplate)
	cmd.InitDefaultHelpFlag()
	if helpFlag := cmd.Flags().Lookup("help"); helpFlag != nil {
		helpFlag.Usage = "显示帮助"
	}
	for _, child := range cmd.Commands() {
		applyChineseTemplates(child)
	}
}

func flagUsagesCN(flags *pflag.FlagSet) string {
	if flags == nil || !flags.HasAvailableFlags() {
		return ""
	}

	lines := make([]string, 0)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		name := "      --" + flag.Name
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			name = fmt.Sprintf("  -%s, --%s", flag.Shorthand, flag.Name)
		}

		valueType := chineseFlagType(flag.Value.Type())
		if valueType != "" {
			name += " " + valueType
		}

		usage := flag.Usage
		if shouldShowDefault(flag) {
			usage += fmt.Sprintf("（默认: %s）", formatFlagDefault(flag))
		}

		lines = append(lines, fmt.Sprintf("%-28s %s", name, usage))
	})

	return strings.Join(lines, "\n")
}

func chineseFlagType(valueType string) string {
	switch valueType {
	case "string":
		return "字符串"
	case "int", "int32", "int64":
		return "整数"
	case "uint", "uint32", "uint64":
		return "正整数"
	case "bool":
		return ""
	default:
		return valueType
	}
}

func shouldShowDefault(flag *pflag.Flag) bool {
	if flag.DefValue == "" || flag.DefValue == "false" {
		return false
	}
	return true
}

func formatFlagDefault(flag *pflag.Flag) string {
	switch flag.Value.Type() {
	case "string":
		return fmt.Sprintf("%q", flag.DefValue)
	default:
		return flag.DefValue
	}
}
