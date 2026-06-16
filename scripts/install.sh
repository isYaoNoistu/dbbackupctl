#!/bin/bash
# dbbackupctl installation script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="dbbackupctl"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/dbbackupctl"
DATA_DIR="/data/dbbackupctl"
BACKUP_ROOT="/data/backup"
LOG_DIR="/var/log/dbbackupctl"
LOCK_DIR="/var/lock/dbbackupctl"

# Functions
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
        log_error "Please run as root"
        exit 1
    fi
}

check_binary() {
    if [ ! -f "./bin/$BINARY_NAME" ]; then
        log_error "Binary not found. Please run 'make build' first."
        exit 1
    fi
}

create_directories() {
    log_info "Creating directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR/index"
    mkdir -p "$DATA_DIR/tmp"
    mkdir -p "$BACKUP_ROOT"
    mkdir -p "$LOG_DIR"
    mkdir -p "$LOCK_DIR"
    
    log_info "Directories created."
}

install_binary() {
    log_info "Installing binary to $INSTALL_DIR..."
    
    cp "./bin/$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    log_info "Binary installed."
}

install_config() {
    log_info "Installing configuration templates..."
    
    if [ ! -f "$CONFIG_DIR/core.env" ]; then
        cp "./configs/core.env.example" "$CONFIG_DIR/core.env.example"
        log_info "Configuration templates installed."
        log_warn "Please edit configuration files in $CONFIG_DIR"
    else
        log_warn "Configuration already exists. Skipping."
    fi
}

set_permissions() {
    log_info "Setting permissions..."
    
    # Create dbbackup group if it doesn't exist
    if ! getent group dbbackup > /dev/null 2>&1; then
        groupadd dbbackup
        log_info "Created group: dbbackup"
    fi
    
    # Set directory permissions
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
    
    log_info "Permissions set."
}

print_next_steps() {
    echo ""
    echo "=========================================="
    echo " Installation Complete!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Edit configuration files:"
    echo "   $CONFIG_DIR/core.env"
    echo "   $CONFIG_DIR/mysql.env"
    echo "   $CONFIG_DIR/postgresql.env"
    echo "   $CONFIG_DIR/secret.env"
    echo ""
    echo "2. Set password permissions:"
    echo "   chmod 600 $CONFIG_DIR/secret.env"
    echo ""
    echo "3. Verify installation:"
    echo "   $BINARY_NAME --version"
    echo "   $BINARY_NAME check"
    echo ""
    echo "4. Run your first backup:"
    echo "   $BINARY_NAME mysql backup --job prod"
    echo "   $BINARY_NAME postgresql backup --job prod"
    echo ""
}

# Main
main() {
    log_info "Installing $BINARY_NAME..."
    
    check_root
    check_binary
    create_directories
    install_binary
    install_config
    set_permissions
    print_next_steps
}

main "$@"