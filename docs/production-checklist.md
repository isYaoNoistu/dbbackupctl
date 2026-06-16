# 生产检查清单

- `make check-build` 通过。
- `/etc/dbbackupctl/secret.env` 权限为 `600`。
- `dbbackupctl check --mysql --job prod` 无失败项。
- `dbbackupctl check --postgresql --job prod` 无失败项。
- `--dry-run` 输出的环境、数据库和备份目录符合预期。
- 真实备份能生成 artifact、`manifest.json` 和索引记录。
- 恢复先恢复到新库验证。
- 恢复到原库必须要求 `--allow-overwrite`。
- `prune --dry-run` 结果确认后再执行。
- 备份目录和索引文件纳入监控。
