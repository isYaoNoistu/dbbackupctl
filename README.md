# dbbackupctl

`dbbackupctl` 是一个运行在数据库服务器上的轻量型数据库备份工具，面向 MySQL 和 PostgreSQL 的日常逻辑备份、恢复预案、本地索引、保留策略和上线前自检。

它的定位很明确：不引入复杂平台，不依赖后台服务，不要求额外元数据库。把一个二进制文件、一组 env 配置和系统定时任务放到服务器上，就可以完成可审计、可校验、可清理的数据库备份流程。

## 产品特点

- **轻量部署**：单二进制运行，无 Web 服务、无常驻进程、无外部调度中心依赖。
- **贴近服务器运维习惯**：使用 env 文件配置，适合配合 crontab、systemd timer、堡垒机和现有发布流程。
- **多环境统一管理**：一份配置可同时管理 `dev`、`uat`、`prod` 等多套环境，执行时通过 `--job` 精确选择。
- **备份结果可追踪**：每次备份生成 `manifest.json`、checksum 和本地 JSONL 索引，方便查询、核对和问题追溯。
- **恢复更谨慎**：恢复命令默认只输出计划，必须显式增加 `--execute` 才会真正执行。
- **密码不进命令行**：密码从进程环境变量、`secret.env` 或 password file 读取，避免出现在 shell history 中。
- **内置安全检查**：`check` 会检查配置、权限、依赖工具、磁盘空间、密码解析和数据库连通性。
- **自动保留策略**：支持按数量、天数、失败备份数量和总容量清理旧备份。
- **可脚本化**：所有能力都通过 CLI 暴露，便于放入自动化部署和巡检系统。

## 适用场景

- 单台或少量服务器上的 MySQL/PostgreSQL 日常备份。
- 项目需要一套简单、透明、可交付的备份工具，而不是一整套备份平台。
- 运维希望备份文件、索引、日志都留在服务器本地，便于审计和排查。
- 同一台机器需要管理多套环境，例如 `dev`、`uat`、`prod`。
- 恢复操作需要先生成计划，再由人工确认后执行。

## 核心概念：job 就是环境

`--job` 表示一套数据库环境配置。配置文件中可以写多套环境：

```env
MYSQL_JOBS=dev,prod
MYSQL_DEV_HOST=127.0.0.1
MYSQL_PROD_HOST=10.0.0.10

POSTGRES_JOBS=dev,prod
POSTGRES_DEV_HOST=127.0.0.1
POSTGRES_PROD_HOST=10.0.0.20
```

执行时通过 `--job` 选择具体环境：

```bash
dbbackupctl check --postgresql --job dev
dbbackupctl check --postgresql --job prod
dbbackupctl mysql backup --job dev
dbbackupctl postgresql backup --job prod
```

这种方式的好处是配置集中、命令清晰，不需要为每套环境维护一份完全独立的工具目录。

## 从源码构建并初始化

Linux 服务器上推荐使用源码构建初始化脚本：

```bash
sudo scripts/build-init-linux.sh
```

脚本会完成：

1. 使用 `go build` 构建 `bin/dbbackupctl`。
2. 安装到 `/usr/local/bin/dbbackupctl`。
3. 执行 `dbbackupctl init --with-default-config`。
4. 创建 `/etc/dbbackupctl/*.env.example` 和 `/etc/dbbackupctl/*.env`。
5. 创建 `/data/dbbackupctl`、`/data/backup`、`/var/log/dbbackupctl`、`/var/lock/dbbackupctl` 等目录。

常用参数：

```bash
sudo scripts/build-init-linux.sh --config-dir /etc/dbbackupctl
sudo scripts/build-init-linux.sh --install-dir /usr/local/bin
sudo scripts/build-init-linux.sh --templates-only
sudo scripts/build-init-linux.sh --force
```

只构建并初始化、不安装到系统目录：

```bash
sudo scripts/build-init-linux.sh --skip-install --config-dir /etc/dbbackupctl
```

## 手工构建

```bash
go build -o bin/dbbackupctl ./cmd/dbbackupctl
sudo install -m 0755 bin/dbbackupctl /usr/local/bin/dbbackupctl
sudo dbbackupctl init --with-default-config
```

也可以使用 Makefile：

```bash
make check-build
sudo scripts/install.sh --with-default-config
```

## 配置文件

默认配置目录是：

```text
/etc/dbbackupctl
```

主要文件：

```text
core.env          通用目录、压缩、checksum、保留策略、磁盘保护和索引配置
mysql.env         MySQL 多环境配置
postgresql.env    PostgreSQL 多环境配置
secret.env        密码和密钥类敏感配置
```

初始化后先修改：

```bash
sudo vi /etc/dbbackupctl/mysql.env
sudo vi /etc/dbbackupctl/postgresql.env
sudo vi /etc/dbbackupctl/secret.env
sudo chmod 600 /etc/dbbackupctl/secret.env
```

密码读取顺序：

```text
当前进程环境变量 -> secret.env -> password_file
```

## 检查配置是否生效

修改配置后先执行检查：

```bash
dbbackupctl check
dbbackupctl check --mysql --job dev
dbbackupctl check --mysql --job prod
dbbackupctl check --postgresql --job dev
dbbackupctl check --postgresql --job prod
```

`check` 会验证：

- 配置文件是否能加载和校验。
- `--job` 指定的环境是否存在。
- 密码是否能解析。
- 备份目录、日志目录、锁目录权限是否合适。
- `mysqldump`、`mysql`、`pg_dump`、`pg_restore`、`pg_dumpall`、`psql`、`zstd` 等依赖是否可用。
- 数据库连接是否正常。
- 磁盘空间是否满足阈值。

## 执行备份

先预演：

```bash
dbbackupctl mysql backup --job dev --dry-run
dbbackupctl postgresql backup --job prod --dry-run
```

确认无误后执行：

```bash
dbbackupctl mysql backup --job dev
dbbackupctl postgresql backup --job prod
```

备份所有已启用环境：

```bash
dbbackupctl mysql backup --all
dbbackupctl postgresql backup --all
```

每次备份会生成：

- 备份文件。
- `manifest.json`。
- checksum。
- 本地索引记录。
- 命令运行记录。

## 查询备份

```bash
dbbackupctl show mysql
dbbackupctl show mysql --job dev
dbbackupctl show postgresql --job prod --last 10
dbbackupctl show all --json
```

如果索引损坏或迁移过备份目录，可以重建索引：

```bash
dbbackupctl index rebuild
dbbackupctl index verify
```

## 恢复流程

恢复默认只输出计划，不会真正执行：

```bash
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
```

确认计划后，显式增加 `--execute`：

```bash
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute
```

安全规则：

- 恢复到原库必须增加 `--allow-overwrite`。
- PostgreSQL 全局对象默认不恢复，必须增加 `--include-globals`。
- 备份包含多个数据库时必须指定 `--database` 或 `--source-db`。

## 清理旧备份

预演清理：

```bash
dbbackupctl prune --mysql --job dev --dry-run
dbbackupctl prune --postgresql --job prod --dry-run
```

执行清理：

```bash
dbbackupctl prune --mysql --job dev
dbbackupctl prune --postgresql --job prod
```

清理会遵守 `keep-last`、`keep-days`、`keep-failed-last` 和 `max-total-size` 策略。删除前会校验目录必须位于 `DBB_BACKUP_ROOT` 下，并要求备份目录存在 `manifest.json`。

## 定时任务示例

crontab：

```cron
0 2 * * * /usr/local/bin/dbbackupctl mysql backup --job prod >> /var/log/dbbackupctl/cron.log 2>&1
30 2 * * * /usr/local/bin/dbbackupctl postgresql backup --job prod >> /var/log/dbbackupctl/cron.log 2>&1
```

systemd timer 可参考 `docs/install.md`。

## 运维建议

- 生产环境第一次执行前，先跑 `check` 和 `--dry-run`。
- 不要把密码写在命令行参数中。
- `secret.env` 权限建议为 `600`。
- 备份目录建议挂载到独立磁盘或独立分区。
- 定期执行 `dbbackupctl index verify`。
- 定期做恢复演练，避免只验证“能备份”，不验证“能恢复”。
- PostgreSQL v1 流式模式支持 `custom`、`plain`、`tar`，不支持 `directory`。

## 项目文档

更多专题说明见 `docs/`：

- `docs/config.md`
- `docs/install.md`
- `docs/mysql-backup.md`
- `docs/postgresql-backup.md`
- `docs/restore.md`
- `docs/prune.md`
- `docs/test-plan.md`
- `docs/production-checklist.md`
