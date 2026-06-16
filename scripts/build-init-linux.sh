#!/usr/bin/env bash
# 从源码构建 dbbackupctl，并在 Linux 服务器上完成本地初始化。

set -euo pipefail

BINARY_NAME="dbbackupctl"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$ROOT_DIR/bin/$BINARY_NAME"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/dbbackupctl"
SKIP_INSTALL=false
FORCE_INIT=false
WITH_DEFAULT_CONFIG=true

log_info() {
  echo "[信息] $1"
}

log_warn() {
  echo "[警告] $1"
}

log_error() {
  echo "[错误] $1" >&2
}

usage() {
  cat <<'EOF'
用法：scripts/build-init-linux.sh [参数]

参数：
  --install-dir PATH       安装目录，默认 /usr/local/bin
  --config-dir PATH        配置目录，默认 /etc/dbbackupctl
  --skip-install           只构建并用 bin/dbbackupctl 初始化，不安装到系统目录
  --templates-only         只生成 .env.example，不生成运行时读取的 .env
  --force                  覆盖已有初始化文件
  -h, --help               显示帮助

默认行为：
  1. 使用 go build 从源码构建 bin/dbbackupctl
  2. 安装到 /usr/local/bin/dbbackupctl
  3. 执行 dbbackupctl init --with-default-config
  4. 生成 /etc/dbbackupctl/*.env.example 和 /etc/dbbackupctl/*.env
EOF
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --install-dir)
        INSTALL_DIR="${2:?缺少 --install-dir 参数值}"
        shift 2
        ;;
      --config-dir)
        CONFIG_DIR="${2:?缺少 --config-dir 参数值}"
        shift 2
        ;;
      --skip-install)
        SKIP_INSTALL=true
        shift
        ;;
      --templates-only)
        WITH_DEFAULT_CONFIG=false
        shift
        ;;
      --force)
        FORCE_INIT=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        log_error "未知参数：$1"
        usage
        exit 2
        ;;
    esac
  done
}

require_linux() {
  if [ "$(uname -s)" != "Linux" ]; then
    log_error "该脚本仅支持 Linux。"
    exit 1
  fi
}

require_go() {
  if ! command -v go >/dev/null 2>&1; then
    log_error "未找到 go，请先安装 Go。"
    exit 1
  fi
}

require_root_for_system_init() {
  if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    log_error "初始化会写入 $CONFIG_DIR、/data 和 /var；未使用 --skip-install 时还会写入 $INSTALL_DIR。请使用 root 或 sudo 运行。"
    exit 1
  fi
}

build_binary() {
  log_info "正在从源码构建 $BINARY_NAME..."
  cd "$ROOT_DIR"
  mkdir -p "$ROOT_DIR/bin"
  go build -o "$BUILD_OUTPUT" ./cmd/dbbackupctl
  log_info "构建完成：$BUILD_OUTPUT"
}

install_binary() {
  if [ "$SKIP_INSTALL" = "true" ]; then
    log_warn "已跳过系统安装，将使用 $BUILD_OUTPUT 执行初始化。"
    return
  fi

  log_info "正在安装到 $INSTALL_DIR/$BINARY_NAME..."
  install -d -m 0755 "$INSTALL_DIR"
  install -m 0755 "$BUILD_OUTPUT" "$INSTALL_DIR/$BINARY_NAME"
  log_info "安装完成。"
}

run_init() {
  local binary="$BUILD_OUTPUT"
  if [ "$SKIP_INSTALL" != "true" ]; then
    binary="$INSTALL_DIR/$BINARY_NAME"
  fi

  local args=("init" "--config-dir" "$CONFIG_DIR")
  if [ "$WITH_DEFAULT_CONFIG" = "true" ]; then
    args+=("--with-default-config")
  fi
  if [ "$FORCE_INIT" = "true" ]; then
    args+=("--force")
  fi

  log_info "正在初始化配置和本地目录..."
  "$binary" "${args[@]}"

  if [ -f "$CONFIG_DIR/secret.env" ]; then
    chmod 0600 "$CONFIG_DIR/secret.env"
  fi
}

print_next_steps() {
  echo
  echo "初始化完成。"
  echo
  echo "下一步："
  echo "  1. 编辑 $CONFIG_DIR/mysql.env、postgresql.env、secret.env"
  echo "  2. 按实际环境修改 dev/prod 主机、用户、数据库和备份目录"
  echo "  3. 执行：$BINARY_NAME check --mysql --job dev"
  echo "  4. 执行：$BINARY_NAME check --postgresql --job prod"
  echo
}

main() {
  parse_args "$@"
  require_linux
  require_go
  require_root_for_system_init
  build_binary
  install_binary
  run_init
  print_next_steps
}

main "$@"
