# PostgreSQL 备份

先检查环境：

```bash
dbbackupctl check --postgresql --job prod
```

预演：

```bash
dbbackupctl postgresql backup --job prod --dry-run
```

执行：

```bash
dbbackupctl postgresql backup --job prod
```

`--job` 对应 `postgresql.env` 中的环境名，例如 `dev` 或 `prod`。备份使用 `pg_dump`。只有在配置中启用 `POSTGRES_<JOB>_INCLUDE_GLOBALS=true` 时才会备份全局对象。

v1 支持的 dump 格式：

```text
custom
plain
tar
```
