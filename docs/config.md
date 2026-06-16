# 配置

默认配置目录是 `/etc/dbbackupctl`。

文件：

```text
core.env
mysql.env
postgresql.env
secret.env
```

`--job` 表示一套环境配置。示例：

```env
MYSQL_JOBS=dev,prod
MYSQL_DEV_HOST=127.0.0.1
MYSQL_PROD_HOST=10.0.0.10

POSTGRES_JOBS=dev,prod
POSTGRES_DEV_HOST=127.0.0.1
POSTGRES_PROD_HOST=10.0.0.20
```

执行：

```bash
dbbackupctl mysql backup --job dev
dbbackupctl postgresql backup --job prod
dbbackupctl check --postgresql --job dev
```

配置优先级：

```text
默认值 -> env 文件 -> 当前进程环境变量
```

密码读取顺序：

```text
当前进程环境变量 -> secret.env -> *_PASSWORD_FILE
```

压缩类型支持 `zstd`、`gzip`、`none`。PostgreSQL dump 格式支持 `custom`、`plain`、`tar`，v1 流式模式不支持 `directory`。
