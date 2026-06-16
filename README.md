# dbbackupctl

`dbbackupctl` 是一个运行在数据库服务器本地的 MySQL / PostgreSQL 逻辑备份命令行工具。

它不是 Web 平台、不是 Docker 备份平台、不是调度中心、不是多租户系统，也不是 Databasus 的替代品。第一版只聚焦本地备份、恢复、保留策略、索引、checksum 和安全检查。

## 核心概念

`--job` 表示一套环境配置。你可以在配置文件中写多套环境，例如 `dev`、`uat`、`prod`：

```env
MYSQL_JOBS=dev,prod
MYSQL_DEV_HOST=127.0.0.1
MYSQL_PROD_HOST=10.0.0.10

POSTGRES_JOBS=dev,prod
POSTGRES_DEV_HOST=127.0.0.1
POSTGRES_PROD_HOST=10.0.0.20
```

执行时通过 `--job` 选择环境：

```bash
dbbackupctl check --postgresql --job dev
dbbackupctl check --postgresql --job prod
dbbackupctl mysql backup --job dev
dbbackupctl postgresql backup --job prod
```

## 安装

```bash
make check-build
sudo scripts/install.sh
sudo scripts/install.sh --with-default-config
```

默认安装只写入 `.env.example` 模板，不覆盖已有生产配置。需要生成实际 `.env` 文件时再使用 `--with-default-config`。

## 配置文件

默认配置目录：

```text
/etc/dbbackupctl
```

主要文件：

```text
core.env
mysql.env
postgresql.env
secret.env
```

密码读取顺序：

```text
当前进程环境变量 -> secret.env -> password_file
```

`secret.env` 必须限制权限：

```bash
sudo chmod 600 /etc/dbbackupctl/secret.env
```

## 检查

```bash
dbbackupctl check
dbbackupctl check --mysql --job dev
dbbackupctl check --postgresql --job prod
```

`check` 会检查配置、权限、依赖工具、磁盘空间、密码解析和数据库连接。

## 备份

```bash
dbbackupctl mysql backup --job dev --dry-run
dbbackupctl mysql backup --job dev
dbbackupctl postgresql backup --job prod --dry-run
dbbackupctl postgresql backup --job prod
```

每次备份会写入备份文件、`manifest.json`、checksum 信息和本地 JSONL 索引。

## 查询

```bash
dbbackupctl show mysql
dbbackupctl show mysql --job dev
dbbackupctl show postgresql --job prod --last 10
dbbackupctl show all --json
```

## 恢复

恢复默认只输出计划，不真正执行：

```bash
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
```

真正执行必须显式加 `--execute`：

```bash
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute
```

恢复到原库必须加 `--allow-overwrite`。PostgreSQL 全局对象默认不恢复，必须加 `--include-globals`。

## 清理

```bash
dbbackupctl prune --mysql --job dev --dry-run
dbbackupctl prune --postgresql --job prod
dbbackupctl index rebuild
dbbackupctl index verify
```

`prune` 删除前会校验备份目录边界和 `manifest.json`，删除后会重建本地索引。

## 定时执行

crontab 示例：

```cron
0 2 * * * /usr/local/bin/dbbackupctl mysql backup --job prod >> /var/log/dbbackupctl/cron.log 2>&1
30 2 * * * /usr/local/bin/dbbackupctl postgresql backup --job prod >> /var/log/dbbackupctl/cron.log 2>&1
```

## 安全注意

- 不要把密码写在命令行参数中。
- 生产执行前先跑 `--dry-run`。
- 备份目录必须位于 `DBB_BACKUP_ROOT` 下。
- PostgreSQL v1 流式模式支持 `custom`、`plain`、`tar`，不支持 `directory`。

更多说明见 `docs/`。
