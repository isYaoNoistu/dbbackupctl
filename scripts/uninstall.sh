#!/bin/bash
# dbbackupctl 卸载脚本

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # 无颜色

# 配置
BINARY_NAME="dbbackupctl"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/dbbackupctl"
DATA_DIR="/data/dbbackupctl"
LOG_DIR="/var/log/dbbackupctl"
LOCK_DIR="/var/lock/dbbackupctl"

# 函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 root 用户运行"
        exit 1
    fi
}

confirm() {
    read -p "确认卸载 $BINARY_NAME？(y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "已取消卸载。"
        exit 0
    fi
}

remove_binary() {
    log_info "正在删除二进制文件..."
    
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        rm -f "$INSTALL_DIR/$BINARY_NAME"
        log_info "二进制文件已删除。"
    else
        log_warn "未在 $INSTALL_DIR/$BINARY_NAME 找到二进制文件"
    fi
}

remove_config() {
    read -p "是否删除配置文件？(y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "正在删除配置文件..."
        rm -rf "$CONFIG_DIR"
        log_info "配置文件已删除。"
    else
        log_info "保留配置文件。"
    fi
}

remove_data() {
    read -p "是否删除数据文件（备份记录、索引）？(y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "正在删除数据文件..."
        rm -rf "$DATA_DIR"
        log_info "数据文件已删除。"
    else
        log_info "保留数据文件。"
    fi
}

remove_logs() {
    read -p "是否删除日志文件？(y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "正在删除日志文件..."
        rm -rf "$LOG_DIR"
        log_info "日志文件已删除。"
    else
        log_info "保留日志文件。"
    fi
}

remove_locks() {
    log_info "正在删除锁文件..."
    rm -rf "$LOCK_DIR"
    log_info "锁文件已删除。"
}

print_summary() {
    echo ""
    echo "=========================================="
    echo " 卸载完成！"
    echo "=========================================="
    echo ""
    echo "注意：/data/backup 中的备份文件不会被删除。"
    echo "如需清理，请手工删除。"
    echo ""
}

# Main
main() {
    log_info "正在卸载 $BINARY_NAME..."
    
    check_root
    confirm
    remove_binary
    remove_config
    remove_data
    remove_logs
    remove_locks
    print_summary
}

main "$@"
