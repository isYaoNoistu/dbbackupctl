# 恢复

恢复命令默认只输出计划，不执行真实恢复。

MySQL：

```bash
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore
dbbackupctl mysql restore --id mysql-dev-20260616-020000 --database app --target-db app_restore --execute
```

PostgreSQL：

```bash
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore
dbbackupctl postgresql restore --id postgresql-prod-20260616-020000 --database app --target-db app_restore --execute
```

安全规则：

```text
恢复前校验 checksum
artifact 路径必须位于备份目录内
恢复到源库必须显式加 --allow-overwrite
PostgreSQL 全局对象必须显式加 --include-globals
```
