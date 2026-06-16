# 安装

构建并检查：

```bash
make check-build
```

安装二进制和配置模板：

```bash
sudo scripts/install.sh
```

需要同时生成可编辑的实际 `.env` 文件时：

```bash
sudo scripts/install.sh --with-default-config
```

默认安装不会覆盖已有生产配置。

systemd service 示例：

```ini
[Unit]
Description=dbbackupctl MySQL 备份

[Service]
Type=oneshot
ExecStart=/usr/local/bin/dbbackupctl mysql backup --job prod
```

systemd timer 示例：

```ini
[Timer]
OnCalendar=*-*-* 02:00:00
Persistent=true

[Install]
WantedBy=timers.target
```
