#!/bin/bash
#
# NOFX Stable Release Installation Script
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/release/stable/install-stable.sh | bash
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="${1:-$HOME/nofx}"
COMPOSE_FILE="docker-compose.stable.yml"
GITHUB_RAW="https://raw.githubusercontent.com/NoFxAiOS/nofx/release/stable"

echo -e "${BLUE}"
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                 NOFX Stable Release                        ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

check_docker() {
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Error: Docker is not installed.${NC}"
        exit 1
    fi
    if ! docker info &> /dev/null; then
        echo -e "${RED}Error: Docker daemon is not running.${NC}"
        exit 1
    fi
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo -e "${RED}Error: Docker Compose is not available.${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Docker ready${NC}"
}

setup_directory() {
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    echo -e "${GREEN}✓ Directory: $INSTALL_DIR${NC}"
}

download_files() {
    curl -fsSL "$GITHUB_RAW/$COMPOSE_FILE" -o docker-compose.yml
    echo -e "${GREEN}✓ Config downloaded${NC}"
}

generate_env() {
    if [ -f ".env" ]; then
        echo -e "${GREEN}✓ .env exists${NC}"
        return
    fi
    JWT_SECRET=$(openssl rand -base64 32)
    DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
    RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null | tr '\n' '\\' | sed 's/\\/\\n/g' | sed 's/\\n$//')
    cat > .env << EOF
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000
TZ=Asia/Shanghai
JWT_SECRET=${JWT_SECRET}
DATA_ENCRYPTION_KEY=${DATA_ENCRYPTION_KEY}
RSA_PRIVATE_KEY=${RSA_PRIVATE_KEY}
EOF
    echo -e "${GREEN}✓ Keys generated${NC}"
}

start_services() {
    $COMPOSE_CMD pull
    $COMPOSE_CMD up -d
    echo -e "${GREEN}✓ Services started${NC}"
}

get_server_ip() {
    local ip=$(curl -s --max-time 3 ifconfig.me 2>/dev/null || echo "")
    echo "${ip:-127.0.0.1}"
}

print_success() {
    local IP=$(get_server_ip)
    echo ""
    echo -e "${GREEN}Installation Complete!${NC}"
    echo -e "  Web: http://${IP}:3000"
    echo -e "  API: http://${IP}:8080"
    echo ""
}

main() {
    check_docker
    setup_directory
    download_files
    generate_env
    start_services
    print_success
}

main
