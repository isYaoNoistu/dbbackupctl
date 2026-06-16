# 测试计划

静态检查：

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -o bin/dbbackupctl ./cmd/dbbackupctl
```

功能检查：

```bash
dbbackupctl init --config-dir /etc/dbbackupctl --force
dbbackupctl check --mysql --job dev
dbbackupctl check --postgresql --job prod
dbbackupctl mysql backup --job dev --dry-run
dbbackupctl postgresql backup --job prod --dry-run
dbbackupctl show all
dbbackupctl prune --dry-run
dbbackupctl index verify
```

真实恢复前必须先检查 restore plan。
