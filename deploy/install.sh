#!/usr/bin/env bash
set -e

# ============================================================================
#  Easy-Pay 一键部署脚本
#  用法: curl -sSL https://raw.githubusercontent.com/DouDOU-start/easy-pay/master/deploy/install.sh | sudo bash
#
#  参数:
#    -y, --yes             跳过交互确认，使用默认值
#    -p, --port PORT       监听端口 (默认 8080)
#    -v, --version VER     安装指定版本 (如 v0.1.0)，默认最新
#    --install-dir DIR     安装目录 (默认 /opt/easypay)
#    --uninstall           卸载
# ============================================================================

GITHUB_REPO="DouDOU-start/easy-pay"
GITHUB_API="https://api.github.com/repos/${GITHUB_REPO}"
APP_NAME="easypay"
SERVICE_NAME="easypay"
DEFAULT_INSTALL_DIR="/opt/easypay"
DEFAULT_PORT=8080
DEFAULT_USER="easypay"

# -------------------------------- Colors ------------------------------------

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# -------------------------------- Helpers -----------------------------------

info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; }
fatal()   { error "$*"; exit 1; }

separator() {
    echo -e "${DIM}────────────────────────────────────────────────────────────${NC}"
}

banner() {
    echo ""
    echo -e "${BOLD}${CYAN}"
    echo "  ┌──────────────────────────────────────────┐"
    echo "  │           Easy-Pay 部署脚本               │"
    echo "  │         统一支付网关 · v0.1               │"
    echo "  └──────────────────────────────────────────┘"
    echo -e "${NC}"
}

is_interactive() {
    [ -e /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]
}

ask() {
    local prompt="$1" default="$2" var="$3"
    if [ "$FORCE_YES" = "true" ] || ! is_interactive; then
        eval "$var='$default'"
        return
    fi
    read -r -p "$(echo -e "${CYAN}?${NC} ${prompt} [${default}]: ")" input < /dev/tty
    eval "$var='${input:-$default}'"
}

confirm() {
    local prompt="$1"
    if [ "$FORCE_YES" = "true" ] || ! is_interactive; then
        return 0
    fi
    read -r -p "$(echo -e "${CYAN}?${NC} ${prompt} (y/N): ")" answer < /dev/tty
    case "$answer" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

get_public_ip() {
    local ip=""
    for url in "https://ipinfo.io/ip" "https://ifconfig.me" "https://icanhazip.com"; do
        ip=$(curl -s --noproxy '*' --connect-timeout 3 "$url" 2>/dev/null | tr -d '[:space:]')
        if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return
        fi
    done
    hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost"
}

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) fatal "不支持的架构: $arch (仅支持 amd64/arm64)" ;;
    esac
}

get_latest_version() {
    local version
    version=$(curl -sSL --connect-timeout 10 --max-time 15 "${GITHUB_API}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "$version" ]; then
        fatal "无法获取最新版本号，请检查网络或使用 --version 指定版本"
    fi
    echo "$version"
}

validate_version() {
    local version="$1"
    local http_code
    http_code=$(curl -s --connect-timeout 10 --max-time 15 -o /dev/null -w "%{http_code}" "${GITHUB_API}/releases/tags/${version}")
    if [ "$http_code" != "200" ]; then
        fatal "版本 ${version} 不存在，请检查: https://github.com/${GITHUB_REPO}/releases"
    fi
}

# -------------------------------- Checks ------------------------------------

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        fatal "请以 root 权限运行此脚本，或使用 sudo"
    fi
}

check_os() {
    if [[ "$(uname)" != "Linux" ]]; then
        fatal "此脚本仅支持 Linux 系统"
    fi
    success "操作系统: $(. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME" || uname -s)"
}

check_dependencies() {
    local missing=()
    for cmd in curl; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done

    if [ ${#missing[@]} -gt 0 ]; then
        fatal "缺少依赖: ${missing[*]}，请先安装"
    fi
    success "依赖检查通过"

    if ! command -v systemctl &>/dev/null; then
        fatal "需要 systemd，当前系统不支持"
    fi
    success "systemd 可用"
}

# -------------------------------- Uninstall ---------------------------------

do_uninstall() {
    banner
    info "开始卸载 Easy-Pay..."
    separator

    if ! confirm "确认卸载 Easy-Pay？"; then
        info "取消卸载"
        exit 0
    fi

    # Stop service
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "停止服务..."
        systemctl stop "$SERVICE_NAME"
    fi
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    fi

    # Remove service file
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
        success "systemd 服务已移除"
    fi

    # Remove install dir
    local install_dir="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
    if [ -d "$install_dir" ]; then
        if confirm "是否删除安装目录 $install_dir（包含配置文件）"; then
            rm -rf "$install_dir"
            success "安装目录已删除"
        else
            info "保留安装目录 $install_dir"
        fi
    fi

    # Remove user
    if id "$DEFAULT_USER" &>/dev/null; then
        if confirm "是否删除系统用户 $DEFAULT_USER"; then
            userdel "$DEFAULT_USER" 2>/dev/null || true
            success "用户 $DEFAULT_USER 已删除"
        fi
    fi

    separator
    success "Easy-Pay 已卸载"
    echo ""
    exit 0
}

# -------------------------------- Parse Args --------------------------------

FORCE_YES="false"
INSTALL_DIR=""
API_PORT=""
TARGET_VERSION=""
DO_UNINSTALL="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -y|--yes)         FORCE_YES="true"; shift ;;
        -p|--port)        API_PORT="$2"; shift 2 ;;
        --port=*)         API_PORT="${1#*=}"; shift ;;
        -v|--version)     TARGET_VERSION="$2"; shift 2 ;;
        --version=*)      TARGET_VERSION="${1#*=}"; shift ;;
        --install-dir)    INSTALL_DIR="$2"; shift 2 ;;
        --install-dir=*)  INSTALL_DIR="${1#*=}"; shift ;;
        --uninstall)      DO_UNINSTALL="true"; shift ;;
        -h|--help)
            echo "用法: install.sh [选项]"
            echo ""
            echo "选项:"
            echo "  -y, --yes             跳过交互确认"
            echo "  -p, --port PORT       API 端口 (默认 $DEFAULT_PORT)"
            echo "  -v, --version VER     安装指定版本 (默认最新)"
            echo "  --install-dir DIR     安装目录 (默认 $DEFAULT_INSTALL_DIR)"
            echo "  --uninstall           卸载 Easy-Pay"
            echo "  -h, --help            显示帮助"
            exit 0
            ;;
        *)
            warn "未知参数: $1"
            shift
            ;;
    esac
done

# -------------------------------- Main Flow ---------------------------------

main() {
    banner
    check_root
    check_os
    check_dependencies
    separator

    # Detect architecture
    local arch
    arch=$(detect_arch)
    success "系统架构: linux/$arch"

    # Resolve version
    if [ -z "$TARGET_VERSION" ]; then
        info "获取最新版本..."
        TARGET_VERSION=$(get_latest_version)
    else
        validate_version "$TARGET_VERSION"
    fi
    success "目标版本: $TARGET_VERSION"

    # Interactive config
    ask "安装目录" "$DEFAULT_INSTALL_DIR" INSTALL_DIR
    INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

    ask "监听端口" "$DEFAULT_PORT" API_PORT
    API_PORT="${API_PORT:-$DEFAULT_PORT}"

    separator

    # ---------- Create user ----------

    if ! id "$DEFAULT_USER" &>/dev/null; then
        info "创建系统用户: $DEFAULT_USER"
        useradd --system --no-create-home --shell /usr/sbin/nologin "$DEFAULT_USER"
        success "用户 $DEFAULT_USER 已创建"
    else
        success "用户 $DEFAULT_USER 已存在"
    fi

    # ---------- Create directories ----------

    info "创建目录结构..."
    mkdir -p "$INSTALL_DIR"/{bin,data}
    success "目录已创建: $INSTALL_DIR"

    # ---------- Download binary ----------

    local binary_name="easypay-linux-${arch}"
    local github_url="https://github.com/${GITHUB_REPO}/releases/download/${TARGET_VERSION}/${binary_name}"
    local mirrors=(
        "https://gh-proxy.com/${github_url}"
        "https://ghfast.top/${github_url}"
        "https://mirror.ghproxy.com/${github_url}"
        "$github_url"
    )
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT

    info "下载 ${binary_name} (${TARGET_VERSION})..."
    local downloaded=false
    for url in "${mirrors[@]}"; do
        local source
        if [ "$url" = "$github_url" ]; then
            source="GitHub"
        else
            source="镜像"
        fi
        info "尝试 ${source}: ${url}"
        if curl -sSL --fail --connect-timeout 10 --max-time 120 "$url" -o "$temp_dir/$binary_name" 2>/dev/null; then
            local file_size
            file_size=$(stat -c%s "$temp_dir/$binary_name" 2>/dev/null || stat -f%z "$temp_dir/$binary_name" 2>/dev/null || echo "0")
            if [ "$file_size" -gt 1000000 ]; then
                downloaded=true
                success "下载完成 (${source})"
                break
            fi
            warn "文件异常，尝试下一个源..."
        else
            warn "下载失败，尝试下一个源..."
        fi
    done

    if [ "$downloaded" != "true" ]; then
        fatal "所有下载源均失败，请手动下载: ${github_url}"
    fi

    # Checksum verification
    local checksum_url="${github_url}.sha256"
    info "校验文件完整性..."
    if curl -sSL --fail --connect-timeout 5 --max-time 15 "$checksum_url" -o "$temp_dir/checksum.sha256" 2>/dev/null; then
        local expected actual
        expected=$(awk '{print $1}' "$temp_dir/checksum.sha256")
        actual=$(sha256sum "$temp_dir/$binary_name" | awk '{print $1}')
        if [ "$expected" != "$actual" ]; then
            fatal "SHA256 校验失败！文件可能被篡改"
        fi
        success "SHA256 校验通过"
    else
        warn "未找到校验文件，跳过完整性验证"
    fi

    # ---------- Install binary ----------

    # Backup old binary if exists
    if [ -f "$INSTALL_DIR/bin/easypay" ]; then
        local old_version
        old_version=$("$INSTALL_DIR/bin/easypay" --version 2>/dev/null || echo "unknown")
        cp -f "$INSTALL_DIR/bin/easypay" "$INSTALL_DIR/bin/easypay.backup"
        info "已备份旧版本: $old_version"
    fi

    cp -f "$temp_dir/$binary_name" "$INSTALL_DIR/bin/easypay"
    chmod +x "$INSTALL_DIR/bin/easypay"
    success "二进制文件已安装: $INSTALL_DIR/bin/easypay"

    # ---------- Set permissions ----------

    chown -R "$DEFAULT_USER:$DEFAULT_USER" "$INSTALL_DIR"
    chmod 750 "$INSTALL_DIR"
    chmod 750 "$INSTALL_DIR/data"

    # ---------- Write systemd service ----------

    info "配置 systemd 服务..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << SERVICE_EOF
[Unit]
Description=Easy-Pay 统一支付网关
Documentation=https://github.com/${GITHUB_REPO}
After=network.target postgresql.service redis.service
Wants=network-online.target

[Service]
Type=simple
User=${DEFAULT_USER}
Group=${DEFAULT_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/easypay --config ${INSTALL_DIR}/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536

Environment=PORT=${API_PORT}

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${INSTALL_DIR}

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    systemctl daemon-reload
    success "systemd 服务已配置"

    # ---------- Enable and start ----------

    systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
    success "已设置开机自启"

    # Stop if already running
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true

    info "启动服务..."
    systemctl start "$SERVICE_NAME"

    # Wait for port to be ready
    local retries=0
    while [ $retries -lt 15 ]; do
        if curl -s -o /dev/null "http://127.0.0.1:${API_PORT}/setup/status" 2>/dev/null; then
            break
        fi
        retries=$((retries + 1))
        sleep 1
    done

    if [ $retries -ge 15 ]; then
        warn "服务启动中，请稍后检查:"
        echo "  systemctl status $SERVICE_NAME"
        echo "  journalctl -u $SERVICE_NAME -f"
    else
        success "服务已启动"
    fi

    # ---------- Save metadata ----------

    cat > "$INSTALL_DIR/.install-info" << META_EOF
version=${TARGET_VERSION}
installed_at=$(date '+%Y-%m-%d %H:%M:%S')
arch=${arch}
port=${API_PORT}
META_EOF

    # ---------- Print summary ----------

    separator
    echo ""

    local public_ip
    public_ip=$(get_public_ip)

    echo -e "${BOLD}${GREEN}  ┌──────────────────────────────────────────┐${NC}"
    echo -e "${BOLD}${GREEN}  │         Easy-Pay 部署完成!               │${NC}"
    echo -e "${BOLD}${GREEN}  └──────────────────────────────────────────┘${NC}"
    echo ""
    echo -e "  ${BOLD}初始化向导${NC}"
    echo -e "  ${CYAN}http://${public_ip}:${API_PORT}${NC}"
    echo ""
    echo -e "  ${DIM}首次运行会进入初始化向导，请在浏览器中完成以下配置:${NC}"
    echo -e "  ${DIM}  1. 数据库连接 (PostgreSQL)${NC}"
    echo -e "  ${DIM}  2. Redis 连接${NC}"
    echo -e "  ${DIM}  3. 管理员账户${NC}"
    echo ""
    echo -e "  ${BOLD}安装信息${NC}"
    echo -e "  版本:        ${TARGET_VERSION}"
    echo -e "  安装目录:    ${INSTALL_DIR}"
    echo -e "  配置文件:    ${INSTALL_DIR}/config.yaml ${DIM}(向导完成后自动生成)${NC}"
    echo -e "  监听端口:    ${API_PORT}"
    echo -e "  运行用户:    ${DEFAULT_USER}"
    echo ""
    echo -e "  ${BOLD}常用命令${NC}"
    echo -e "  systemctl status ${SERVICE_NAME}       ${DIM}# 查看状态${NC}"
    echo -e "  systemctl restart ${SERVICE_NAME}      ${DIM}# 重启服务${NC}"
    echo -e "  journalctl -u ${SERVICE_NAME} -f       ${DIM}# 实时查看日志${NC}"
    echo ""
    echo -e "  ${BOLD}日志排查${NC}"
    echo -e "  journalctl -u ${SERVICE_NAME} --since '1 hour ago'     ${DIM}# 最近1小时${NC}"
    echo -e "  journalctl -u ${SERVICE_NAME} --since today            ${DIM}# 今天的日志${NC}"
    echo -e "  journalctl -u ${SERVICE_NAME} | grep order_no          ${DIM}# 按单号搜索${NC}"
    echo -e "  journalctl -u ${SERVICE_NAME} | grep '\"level\":\"error\"' ${DIM}# 只看错误${NC}"
    echo ""
    echo -e "  ${DIM}日志配置: 在 config.yaml 中设置 log.level (debug/info/warn/error)${NC}"
    echo -e "  ${DIM}          和 log.format (json/console) 可切换日志格式${NC}"
    echo ""
    echo -e "  ${BOLD}更新版本${NC}"
    echo -e "  ${DIM}curl -sSL https://raw.githubusercontent.com/${GITHUB_REPO}/master/deploy/install.sh | sudo bash${NC}"
    echo ""
    separator
    echo -e "  ${DIM}请确保 PostgreSQL 和 Redis 已安装并运行，然后访问上方地址完成初始化。${NC}"
    echo ""
}

# -------------------------------- Entry Point -------------------------------

if [ "$DO_UNINSTALL" = "true" ]; then
    check_root
    do_uninstall
else
    main
fi
