# 清理

预演：

```bash
dbbackupctl prune --mysql --job dev --dry-run
```

执行：

```bash
dbbackupctl prune --postgresql --job prod
```

清理会应用 keep-last、keep-days、keep-failed-last 和 max-total-size 策略。删除前会确认目录位于 `DBB_BACKUP_ROOT` 内，并要求备份目录存在 `manifest.json`。
