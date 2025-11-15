#!/usr/bin/env bash

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="${PROJECT_ROOT}/web"
NODE_REQUIRED_MAJOR=18
GO_REQUIRED_VERSION="1.21"
GO_INSTALL_VERSION="1.22.5"
GO_URL_BASE="https://go.dev/dl"
SUDO_BIN=""
PKG_MANAGER=""
PKG_INDEX_UPDATED="false"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf "%b\n" "$1"; }
info() { log "${BLUE}[INFO]${NC} $1"; }
success() { log "${GREEN}[SUCCESS]${NC} $1"; }
warn() { log "${YELLOW}[WARNING]${NC} $1"; }
err() { log "${RED}[ERROR]${NC} $1"; }

ensure_sudo() {
    if command -v sudo >/dev/null 2>&1; then
        SUDO_BIN="sudo"
    elif [ "${EUID}" -ne 0 ]; then
        err "Please run as root or install sudo."
        exit 1
    fi
}

detect_platform() {
    local uname_s
    uname_s=$(uname -s)
    if [ "${uname_s}" != "Linux" ]; then
        err "setup.sh is intended for Linux servers."
        exit 1
    fi
}

detect_package_manager() {
    if command -v apt-get >/dev/null 2>&1; then
        PKG_MANAGER="apt"
    elif command -v dnf >/dev/null 2>&1; then
        PKG_MANAGER="dnf"
    elif command -v yum >/dev/null 2>&1; then
        PKG_MANAGER="yum"
    elif command -v pacman >/dev/null 2>&1; then
        PKG_MANAGER="pacman"
    elif command -v apk >/dev/null 2>&1; then
        PKG_MANAGER="apk"
    else
        err "Unsupported package manager. Install dependencies manually."
        exit 1
    fi
}

update_pkg_index() {
    if [ "${PKG_INDEX_UPDATED}" = "true" ]; then
        return
    fi
    case "${PKG_MANAGER}" in
        apt)
            ${SUDO_BIN} apt-get update
            ;;
        dnf)
            ${SUDO_BIN} dnf makecache -y
            ;;
        yum)
            ${SUDO_BIN} yum makecache -y
            ;;
        pacman)
            ${SUDO_BIN} pacman -Sy --noconfirm
            ;;
        apk)
            ${SUDO_BIN} apk update
            ;;
    esac
    PKG_INDEX_UPDATED="true"
}

pkg_install() {
    update_pkg_index
    case "${PKG_MANAGER}" in
        apt)
            ${SUDO_BIN} apt-get install -y "$@"
            ;;
        dnf)
            ${SUDO_BIN} dnf install -y "$@"
            ;;
        yum)
            ${SUDO_BIN} yum install -y "$@"
            ;;
        pacman)
            ${SUDO_BIN} pacman -S --needed --noconfirm "$@"
            ;;
        apk)
            ${SUDO_BIN} apk add --no-cache "$@"
            ;;
    esac
}

version_ge() {
    # usage: version_ge "${installed}" "${required}"
    [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

install_base_packages() {
    info "Installing base system packages..."
    case "${PKG_MANAGER}" in
        apt)
            pkg_install build-essential git curl wget ca-certificates pkg-config libssl-dev unzip tar sqlite3 libsqlite3-dev openssl lsof net-tools
            ;;
        dnf|yum)
            pkg_install gcc gcc-c++ make git curl wget ca-certificates pkgconfig openssl openssl-devel sqlite sqlite-devel unzip tar lsof net-tools
            ;;
        pacman)
            pkg_install base-devel git curl wget ca-certificates pkgconf openssl sqlite unzip tar lsof net-tools
            ;;
        apk)
            pkg_install build-base git curl wget ca-certificates openssl sqlite sqlite-dev unzip tar lsof net-tools
            ;;
    esac
}

install_talib() {
    info "Ensuring TA-Lib native libraries are present..."
    case "${PKG_MANAGER}" in
        apt)
            pkg_install libta-lib0 libta-lib0-dev || warn "TA-Lib packages unavailable in this apt repo; please install manually if indicators fail."
            ;;
        dnf|yum)
            pkg_install ta-lib || warn "TA-Lib package unavailable; install from source if needed."
            ;;
        pacman)
            pkg_install ta-lib || warn "TA-Lib package unavailable; install from source if needed."
            ;;
        apk)
            warn "TA-Lib packages are not available via apk. Please compile manually if required."
            ;;
    esac
}

install_node() {
    if command -v node >/dev/null 2>&1; then
        local current_major
        current_major=$(node --version | sed 's/^v//' | cut -d'.' -f1)
        if [ "$current_major" -ge "${NODE_REQUIRED_MAJOR}" ]; then
            success "Node.js $(node --version) already satisfies requirements."
            return
        fi
        warn "Node.js version $(node --version) is below v${NODE_REQUIRED_MAJOR}."
    else
        warn "Node.js not found."
    fi

    info "Installing Node.js v${NODE_REQUIRED_MAJOR} LTS..."
    case "${PKG_MANAGER}" in
        apt)
            if [ -n "${SUDO_BIN}" ]; then
                curl -fsSL https://deb.nodesource.com/setup_${NODE_REQUIRED_MAJOR}.x | ${SUDO_BIN} -E bash -
            else
                curl -fsSL https://deb.nodesource.com/setup_${NODE_REQUIRED_MAJOR}.x | bash -
            fi
            pkg_install nodejs
            ;;
        dnf|yum)
            if [ -n "${SUDO_BIN}" ]; then
                curl -fsSL https://rpm.nodesource.com/setup_${NODE_REQUIRED_MAJOR}.x | ${SUDO_BIN} -E bash -
            else
                curl -fsSL https://rpm.nodesource.com/setup_${NODE_REQUIRED_MAJOR}.x | bash -
            fi
            pkg_install nodejs
            ;;
        pacman)
            pkg_install nodejs npm
            ;;
        apk)
            pkg_install nodejs npm
            ;;
    esac
    success "Node.js $(node --version) installed."
}

install_go() {
    if command -v go >/dev/null 2>&1; then
        local go_version
        go_version=$(go version | awk '{print $3}' | sed 's/go//')
        if version_ge "$go_version" "${GO_REQUIRED_VERSION}"; then
            success "Go ${go_version} already satisfies requirements."
            return
        fi
        warn "Existing Go ${go_version} is older than ${GO_REQUIRED_VERSION}."
    fi

    local arch tar_arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            tar_arch="amd64"
            ;;
        arm64|aarch64)
            tar_arch="arm64"
            ;;
        *)
            err "Unsupported architecture: ${arch}. Install Go manually."
            exit 1
            ;;
    esac

    local tarball="go${GO_INSTALL_VERSION}.linux-${tar_arch}.tar.gz"
    local url="${GO_URL_BASE}/${tarball}"
    local tmp_tar="/tmp/${tarball}"

    info "Downloading Go ${GO_INSTALL_VERSION} (${tar_arch})..."
    curl -fsSL "$url" -o "$tmp_tar"
    info "Installing Go to /usr/local/go..."
    ${SUDO_BIN} rm -rf /usr/local/go
    ${SUDO_BIN} tar -C /usr/local -xzf "$tmp_tar"
    rm -f "$tmp_tar"

    export PATH="/usr/local/go/bin:${PATH}"
    
    # 立即持久化 PATH 配置，确保后续 go 命令可用
    local profile="$HOME/.profile"
    touch "$profile"
    if ! grep -F '/usr/local/go/bin' "$profile" >/dev/null 2>&1; then
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$profile"
    fi
    if ! grep -F '$HOME/go/bin' "$profile" >/dev/null 2>&1; then
        echo 'export PATH="$HOME/go/bin:$PATH"' >> "$profile"
    fi
    
    success "Go $(go version | awk '{print $3}') installed."
}

prepare_env_files() {
    cd "$PROJECT_ROOT"
    if [ ! -f .env ]; then
        if [ -f .env.example ]; then
            cp .env.example .env
        else
            cat > .env <<'EOF'
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
DATA_ENCRYPTION_KEY=your_data_encryption_key_here_change_me
JWT_SECRET=your_jwt_secret_here_change_me
NODE_ENV=development
GO_ENV=development
EOF
        fi
        warn "Created default .env. Update secrets before going live."
        chmod 600 .env || true
    fi

    if [ -f .env ]; then
        chmod 600 .env || true
    fi

    if [ ! -f config.json ]; then
        if [ -f config.json.example ]; then
            cp config.json.example config.json
        else
            cat > config.json <<'EOF'
{
  "system": {
    "lever_rate": 10,
    "leverage_enabled": true,
    "admin_mode": false
  }
}
EOF
        fi
        warn "Created config.json from defaults."
    fi

    mkdir -p secrets logs decision_logs temp database_backups
    chmod 700 secrets

    if [ -f start.sh ]; then
        chmod +x start.sh || true
    fi
    if [ -f scripts/setup_encryption.sh ]; then
        chmod +x scripts/setup_encryption.sh || true
    fi
}

bootstrap_dependencies() {
    cd "$PROJECT_ROOT"
    info "Fetching Go modules..."
    go mod download
    info "Installing frontend dependencies..."
    if [ -d "$WEB_DIR" ]; then
        pushd "$WEB_DIR" >/dev/null
        npm install
        popd >/dev/null
        success "Frontend dependencies installed."
    else
        warn "web directory missing; skipped npm install."
    fi
}

show_summary() {
    cat <<'EOM'

============================================================
Setup complete!
Next steps:
  1. (Recommended) Run: ./scripts/setup_encryption.sh
  2. Start the platform: ./start.sh start --build   # or ./start.sh --dev
  3. Access the web UI at http://localhost:3000 once services are running.

If you installed Go via this script, log out/in or run 'source ~/.profile'
so that /usr/local/go/bin is available in new shells.
============================================================
EOM
}

main() {
    detect_platform
    ensure_sudo
    detect_package_manager
    info "Detected package manager: ${PKG_MANAGER}"
    install_base_packages
    install_talib
    install_node
    install_go
    prepare_env_files
    bootstrap_dependencies
    show_summary
    success "Environment is ready for start.sh"
}

main "$@"
