# MySQL 备份

先检查环境：

```bash
dbbackupctl check --mysql --job dev
```

预演：

```bash
dbbackupctl mysql backup --job dev --dry-run
```

执行：

```bash
dbbackupctl mysql backup --job dev
```

`--job` 对应 `mysql.env` 中的环境名，例如 `dev` 或 `prod`。备份使用 `mysqldump`，并生成压缩文件、SHA-256 checksum、`manifest.json` 和本地索引记录。
