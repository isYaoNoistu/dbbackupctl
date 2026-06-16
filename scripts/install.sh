#!/bin/bash
# dbbackupctl 安装脚本

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
BACKUP_ROOT="/data/backup"
LOG_DIR="/var/log/dbbackupctl"
LOCK_DIR="/var/lock/dbbackupctl"
WITH_DEFAULT_CONFIG=false

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

check_binary() {
    if [ ! -f "./bin/$BINARY_NAME" ]; then
        log_error "未找到二进制文件。请先执行 'make build'。"
        exit 1
    fi
}

create_directories() {
    log_info "正在创建目录..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR/index"
    mkdir -p "$DATA_DIR/tmp"
    mkdir -p "$BACKUP_ROOT"
    mkdir -p "$LOG_DIR"
    mkdir -p "$LOCK_DIR"
    
    log_info "目录创建完成。"
}

install_binary() {
    log_info "正在安装二进制文件到 $INSTALL_DIR..."
    
    cp "./bin/$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    log_info "二进制文件安装完成。"
}

install_config() {
    log_info "正在安装配置模板..."

    install -m 0640 "./configs/core.env.example" "$CONFIG_DIR/core.env.example"
    install -m 0640 "./configs/mysql.env.example" "$CONFIG_DIR/mysql.env.example"
    install -m 0640 "./configs/postgresql.env.example" "$CONFIG_DIR/postgresql.env.example"
    install -m 0600 "./configs/secret.env.example" "$CONFIG_DIR/secret.env.example"

    if [ "$WITH_DEFAULT_CONFIG" = "true" ]; then
        install_default_config "core.env" 0640
        install_default_config "mysql.env" 0640
        install_default_config "postgresql.env" 0640
        install_default_config "secret.env" 0600
    else
        log_warn "未创建默认 .env 文件。可使用 --with-default-config 重新运行，或手工复制 example 文件。"
    fi

    log_info "配置模板安装完成。"
}

install_default_config() {
    local target="$1"
    local mode="$2"
    if [ -f "$CONFIG_DIR/$target" ]; then
        log_warn "$CONFIG_DIR/$target 已存在，跳过。"
        return
    fi
    install -m "$mode" "$CONFIG_DIR/$target.example" "$CONFIG_DIR/$target"
}

set_permissions() {
    log_info "正在设置权限..."
    
    # 不存在时创建 dbbackup 用户组
    if ! getent group dbbackup > /dev/null 2>&1; then
        groupadd dbbackup
        log_info "已创建用户组：dbbackup"
    fi
    
    # 设置目录权限
    chown -R root:dbbackup "$CONFIG_DIR"
    chown -R root:dbbackup "$DATA_DIR"
    chown -R root:dbbackup "$BACKUP_ROOT"
    chown -R root:dbbackup "$LOG_DIR"
    chown -R root:dbbackup "$LOCK_DIR"
    
    chmod 750 "$CONFIG_DIR"
    chmod 750 "$DATA_DIR"
    chmod 750 "$BACKUP_ROOT"
    chmod 750 "$LOG_DIR"
    chmod 750 "$LOCK_DIR"
    chmod 0640 "$CONFIG_DIR"/*.env.example
    chmod 0600 "$CONFIG_DIR"/secret.env.example
    if [ -f "$CONFIG_DIR/secret.env" ]; then
        chmod 0600 "$CONFIG_DIR/secret.env"
    fi
    
    log_info "权限设置完成。"
}

print_next_steps() {
    echo ""
    echo "=========================================="
    echo " 安装完成！"
    echo "=========================================="
    echo ""
    echo "后续步骤："
    echo ""
    echo "1. 编辑配置文件："
    echo "   $CONFIG_DIR/core.env"
    echo "   $CONFIG_DIR/mysql.env"
    echo "   $CONFIG_DIR/postgresql.env"
    echo "   $CONFIG_DIR/secret.env"
    echo ""
    echo "2. 设置密码文件权限："
    echo "   chmod 600 $CONFIG_DIR/secret.env"
    echo ""
    echo "3. 验证安装："
    echo "   $BINARY_NAME --version"
    echo "   $BINARY_NAME check"
    echo ""
    echo "4. 执行第一次备份："
    echo "   $BINARY_NAME mysql backup --job prod"
    echo "   $BINARY_NAME postgresql backup --job prod"
    echo ""
}

# Main
main() {
    for arg in "$@"; do
        case "$arg" in
            --with-default-config)
                WITH_DEFAULT_CONFIG=true
                ;;
            -h|--help)
                echo "用法：scripts/install.sh [--with-default-config]"
                exit 0
                ;;
            *)
                log_error "未知参数：$arg"
                exit 1
                ;;
        esac
    done

    log_info "正在安装 $BINARY_NAME..."
    
    check_root
    check_binary
    create_directories
    install_binary
    install_config
    set_permissions
    print_next_steps
}

main "$@"
