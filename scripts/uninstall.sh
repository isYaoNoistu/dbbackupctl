#!/bin/bash
# dbbackupctl uninstallation script

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

confirm() {
    read -p "Are you sure you want to uninstall $BINARY_NAME? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Uninstall cancelled."
        exit 0
    fi
}

remove_binary() {
    log_info "Removing binary..."
    
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        rm -f "$INSTALL_DIR/$BINARY_NAME"
        log_info "Binary removed."
    else
        log_warn "Binary not found at $INSTALL_DIR/$BINARY_NAME"
    fi
}

remove_config() {
    read -p "Remove configuration files? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Removing configuration files..."
        rm -rf "$CONFIG_DIR"
        log_info "Configuration files removed."
    else
        log_info "Keeping configuration files."
    fi
}

remove_data() {
    read -p "Remove data files (backup records, index)? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Removing data files..."
        rm -rf "$DATA_DIR"
        log_info "Data files removed."
    else
        log_info "Keeping data files."
    fi
}

remove_logs() {
    read -p "Remove log files? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Removing log files..."
        rm -rf "$LOG_DIR"
        log_info "Log files removed."
    else
        log_info "Keeping log files."
    fi
}

remove_locks() {
    log_info "Removing lock files..."
    rm -rf "$LOCK_DIR"
    log_info "Lock files removed."
}

print_summary() {
    echo ""
    echo "=========================================="
    echo " Uninstall Complete!"
    echo "=========================================="
    echo ""
    echo "Note: Backup files in /data/backup were NOT removed."
    echo "Please remove them manually if needed."
    echo ""
}

# Main
main() {
    log_info "Uninstalling $BINARY_NAME..."
    
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