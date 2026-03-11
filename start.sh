#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - Docker Management Script
# Usage: ./start.sh [command]
# ═══════════════════════════════════════════════════════════════

set -e

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ------------------------------------------------------------------------
# Utility Functions: Colored Output
# ------------------------------------------------------------------------
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ------------------------------------------------------------------------
# Detection: Docker Compose Command (Backward Compatible)
# ------------------------------------------------------------------------
detect_compose_cmd() {
    if command -v docker compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        print_error "Docker Compose not found. Please install Docker Compose first."
        exit 1
    fi
    print_info "Using Docker Compose: $COMPOSE_CMD"
}

# ------------------------------------------------------------------------
# Validation: Docker Installation
# ------------------------------------------------------------------------
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker not found. Please install Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    detect_compose_cmd
    print_success "Docker and Docker Compose are installed"
}

# ------------------------------------------------------------------------
# Validation: Environment File (.env)
# ------------------------------------------------------------------------
check_env() {
    if [ ! -f ".env" ]; then
        print_warning ".env not found, copying from template..."
        cp .env.example .env
        print_info ".env file created"
    fi
    print_success "Environment file exists"
}

# ------------------------------------------------------------------------
# Helper: Check if env var is set and not placeholder
# ------------------------------------------------------------------------
is_env_configured() {
    local var_name="$1"
    local value=$(grep "^${var_name}=" .env 2>/dev/null | cut -d'=' -f2-)

    # Strip quotes
    value=$(echo "$value" | tr -d '"'"'")

    # Check empty
    if [ -z "$value" ]; then
        return 1
    fi

    # Check placeholder values
    case "$value" in
        *your-*|*YOUR_*|*change-this*|*CHANGE_THIS*|*example*|*EXAMPLE*)
            return 1
            ;;
    esac

    return 0
}

# ------------------------------------------------------------------------
# Helper: Set env var in .env file
# ------------------------------------------------------------------------
set_env_var() {
    local var_name="$1"
    local var_value="$2"

    if grep -q "^${var_name}=" .env 2>/dev/null; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|^${var_name}=.*|${var_name}=${var_value}|" .env
        else
            sed -i "s|^${var_name}=.*|${var_name}=${var_value}|" .env
        fi
    else
        # Ensure .env ends with a newline before appending
        if [ -s ".env" ] && [ "$(tail -c1 .env | wc -l)" -eq 0 ]; then
            echo "" >> .env
        fi
        echo "${var_name}=${var_value}" >> .env
    fi
}

# ------------------------------------------------------------------------
# Validation: Encryption Keys in .env
# ------------------------------------------------------------------------
check_encryption() {
    print_info "Checking encryption keys..."

    local generated=false

    if ! is_env_configured "JWT_SECRET"; then
        print_warning "JWT_SECRET not set, generating..."
        local jwt_secret=$(openssl rand -base64 32)
        set_env_var "JWT_SECRET" "$jwt_secret"
        print_success "JWT_SECRET generated"
        generated=true
    fi

    if ! is_env_configured "DATA_ENCRYPTION_KEY"; then
        print_warning "DATA_ENCRYPTION_KEY not set, generating..."
        local data_key=$(openssl rand -base64 32)
        set_env_var "DATA_ENCRYPTION_KEY" "$data_key"
        print_success "DATA_ENCRYPTION_KEY generated"
        generated=true
    fi

    if ! is_env_configured "RSA_PRIVATE_KEY"; then
        print_warning "RSA_PRIVATE_KEY not set, generating..."
        local rsa_key=$(openssl genrsa 2048 2>/dev/null | awk '{printf "%s\\n", $0}')
        set_env_var "RSA_PRIVATE_KEY" "\"$rsa_key\""
        print_success "RSA_PRIVATE_KEY generated"
        generated=true
    fi

    if [ "$generated" = true ]; then
        echo ""
        print_success "Missing keys generated and saved to .env"
        print_warning "Keep .env safe — do not commit it to version control"
        echo ""
    fi

    print_success "Encryption keys OK"
    print_info "  • JWT_SECRET: OK"
    print_info "  • DATA_ENCRYPTION_KEY: OK"
    print_info "  • RSA_PRIVATE_KEY: OK"

    chmod 600 .env 2>/dev/null || true
}

# ------------------------------------------------------------------------
# Utility: Read Environment Variables
# ------------------------------------------------------------------------
read_env_vars() {
    if [ -f ".env" ]; then
        NOFX_FRONTEND_PORT=$(grep "^NOFX_FRONTEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "3000")
        NOFX_BACKEND_PORT=$(grep "^NOFX_BACKEND_PORT=" .env 2>/dev/null | cut -d'=' -f2 || echo "8080")

        NOFX_FRONTEND_PORT=$(echo "$NOFX_FRONTEND_PORT" | tr -d '"'"'" | tr -d ' ')
        NOFX_BACKEND_PORT=$(echo "$NOFX_BACKEND_PORT" | tr -d '"'"'" | tr -d ' ')

        NOFX_FRONTEND_PORT=${NOFX_FRONTEND_PORT:-3000}
        NOFX_BACKEND_PORT=${NOFX_BACKEND_PORT:-8080}
    else
        NOFX_FRONTEND_PORT=3000
        NOFX_BACKEND_PORT=8080
    fi
}

# ------------------------------------------------------------------------
# Validation: Database Directory (data/)
# ------------------------------------------------------------------------
check_database() {
    if [ ! -d "data" ]; then
        print_warning "Data directory missing, creating data/..."
        install -m 700 -d data
        print_success "data/ directory created"
    else
        print_success "Data directory exists"
    fi
}

# ------------------------------------------------------------------------
# Service Management: Start
# ------------------------------------------------------------------------
start() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║         🚀 NOFX AI Trading Bot — Startup             ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""

    read_env_vars

    if [ ! -d "data" ]; then
        install -m 700 -d data
    fi

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    print_info "Starting services..."

    if [ "$1" == "--build" ]; then
        $COMPOSE_CMD up -d --build
    else
        $COMPOSE_CMD up -d
    fi

    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ✅ Started! Next steps:                             ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "  1. Open the web dashboard to register and configure"
    echo "  2. Add an AI model and exchange in Settings"
    echo "  3. (Optional) Add a Telegram bot token in Settings → Telegram"
    echo ""
    echo -e "  Web dashboard: ${BLUE}http://localhost:${NOFX_FRONTEND_PORT}${NC}"
    echo -e "  View logs:     ${YELLOW}./start.sh logs${NC}"
    echo -e "  Stop:          ${YELLOW}./start.sh stop${NC}"
    echo ""
}

# ------------------------------------------------------------------------
# Service Management: Stop
# ------------------------------------------------------------------------
stop() {
    print_info "Stopping services..."
    $COMPOSE_CMD stop
    print_success "Services stopped"
}

# ------------------------------------------------------------------------
# Service Management: Restart
# ------------------------------------------------------------------------
restart() {
    print_info "Restarting services..."
    $COMPOSE_CMD restart
    print_success "Services restarted"
}

# ------------------------------------------------------------------------
# Monitoring: Logs
# ------------------------------------------------------------------------
logs() {
    if [ -z "$2" ]; then
        $COMPOSE_CMD logs -f
    else
        $COMPOSE_CMD logs -f "$2"
    fi
}

# ------------------------------------------------------------------------
# Monitoring: Status
# ------------------------------------------------------------------------
status() {
    read_env_vars

    print_info "Service status:"
    $COMPOSE_CMD ps
    echo ""
    print_info "Health check:"
    curl -s "http://localhost:${NOFX_BACKEND_PORT}/api/health" | jq '.' || echo "Backend not responding"
}

# ------------------------------------------------------------------------
# Maintenance: Clean (Destructive)
# ------------------------------------------------------------------------
clean() {
    print_warning "This will delete all containers and data!"
    read -p "Confirm? (yes/no): " confirm
    if [ "$confirm" == "yes" ]; then
        print_info "Cleaning up..."
        $COMPOSE_CMD down -v
        print_success "Cleanup complete"
    else
        print_info "Cancelled"
    fi
}

# ------------------------------------------------------------------------
# Maintenance: Update
# ------------------------------------------------------------------------
update() {
    print_info "Updating..."
    git pull
    $COMPOSE_CMD up -d --build
    print_success "Update complete"
}

# ------------------------------------------------------------------------
# Command: Regenerate all keys (force)
# ------------------------------------------------------------------------
regenerate_keys() {
    print_warning "This will regenerate ALL encryption keys!"
    print_warning "Any existing encrypted data will become unreadable!"
    echo ""
    read -p "Confirm? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        print_info "Cancelled"
        return
    fi

    check_env

    print_info "Generating new keys..."

    local jwt_secret=$(openssl rand -base64 32)
    set_env_var "JWT_SECRET" "$jwt_secret"
    print_success "JWT_SECRET generated"

    local data_key=$(openssl rand -base64 32)
    set_env_var "DATA_ENCRYPTION_KEY" "$data_key"
    print_success "DATA_ENCRYPTION_KEY generated"

    local rsa_key=$(openssl genrsa 2048 2>/dev/null | awk '{printf "%s\\n", $0}')
    set_env_var "RSA_PRIVATE_KEY" "\"$rsa_key\""
    print_success "RSA_PRIVATE_KEY generated"

    chmod 600 .env 2>/dev/null || true

    echo ""
    print_success "All keys regenerated and saved to .env"
    print_warning "Keep .env safe"
}

# ------------------------------------------------------------------------
# Help: Usage Information
# ------------------------------------------------------------------------
show_help() {
    echo "NOFX AI Trading System - Docker Management Script"
    echo ""
    echo "Usage: ./start.sh [command] [options]"
    echo ""
    echo "Commands:"
    echo "  start [--build]    Start services (optional: rebuild images)"
    echo "  stop               Stop services"
    echo "  restart            Restart services"
    echo "  logs [service]     View logs (optional: backend / frontend)"
    echo "  status             Show service status"
    echo "  clean              Remove all containers and data"
    echo "  update             Pull latest code and rebuild"
    echo "  regenerate-keys    Regenerate all encryption keys (destructive)"
    echo "  help               Show this help"
    echo ""
    echo "Examples:"
    echo "  ./start.sh start --build    # Build and start"
    echo "  ./start.sh logs backend     # View backend logs"
    echo "  ./start.sh status           # Check status"
    echo ""
    echo "First time:"
    echo "  Just run ./start.sh — missing keys are generated automatically"
}

# ------------------------------------------------------------------------
# Main: Command Dispatcher
# ------------------------------------------------------------------------
main() {
    check_docker

    case "${1:-start}" in
        start)
            check_env
            check_encryption
            check_database
            start "$2"
            ;;
        stop)
            stop
            ;;
        restart)
            restart
            ;;
        logs)
            logs "$@"
            ;;
        status)
            status
            ;;
        clean)
            clean
            ;;
        update)
            update
            ;;
        regenerate-keys)
            regenerate_keys
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Execute Main
main "$@"
