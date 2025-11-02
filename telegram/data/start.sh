#!/bin/bash

# Unified bootstrap + lifecycle manager for JT-Bot
# Usage: ./start.sh [start|stop|status|setup|auth|clean|restart] [--no-follow] [--yes]

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${PROJECT_DIR}/.venv"
PYTHON_BIN="${VENV_DIR}/bin/python3"
PIP_BIN="${VENV_DIR}/bin/pip"
LOG_DIR="${PROJECT_DIR}/logs"
DATA_DIR="${PROJECT_DIR}/data"
SESSION_DIR="${PROJECT_DIR}/telegram_collector/data/sessions"
PID_DIR="${PROJECT_DIR}/.runtime"
MONITOR_PID_FILE="${PID_DIR}/monitor.pid"

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
GRAY='\033[0;37m'
NC='\033[0m'

AUTO_APPROVE=0
FOLLOW_LOGS=1
COMMAND="start"

usage() {
    cat <<'EOF'
用法: ./start.sh [命令] [选项]

命令:
  start      默认命令。执行环境检查后启动服务
  stop       停止所有后台服务
  status     查看当前运行状态
  setup      仅执行环境/依赖检查，不启动服务
  auth       进入 Telegram 登录向导
  clean      清理日志和运行时文件
  restart    等价于 stop 后 start

选项:
  --no-follow    启动后不跟随日志，直接返回
  --follow       启动后持续跟随日志 (默认行为)
  --yes, -y      遇到交互提示时默认选择"是"
EOF
}

log_info()    { echo -e "${CYAN}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[OK]${NC}   $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error()   { echo -e "${RED}[ERR]${NC}  $*"; }

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            start|stop|status|setup|auth|clean|restart)
                COMMAND="$1"
                shift
                ;;
            --no-follow)
                FOLLOW_LOGS=0
                shift
                ;;
            --follow)
                FOLLOW_LOGS=1
                shift
                ;;
            --yes|-y)
                AUTO_APPROVE=1
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "未知参数: $1"
                usage
                exit 1
                ;;
        esac
    done
}

ensure_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        log_error "未检测到 python3，请先安装 Python 3.8+ (含 venv 模块)。"
        exit 1
    fi
}

ensure_venv() {
    if [[ ! -d "$VENV_DIR" ]]; then
        log_info "创建虚拟环境 (.venv)..."
        if ! python3 -m venv "$VENV_DIR"; then
            log_error "虚拟环境创建失败，请确认 python3-venv 已安装。"
            exit 1
        fi
        VENV_CREATED=1
    else
        VENV_CREATED=0
    fi
}

activate_venv() {
    # shellcheck disable=SC1090
    if [[ "${VENV_ACTIVE:-0}" -eq 0 ]]; then
        source "${VENV_DIR}/bin/activate"
        VENV_ACTIVE=1
    fi
}

install_dependencies() {
    log_info "检查依赖..."
    if "$PIP_BIN" show telethon >/dev/null 2>&1; then
        log_success "依赖已满足，跳过安装。"
        return
    fi

    log_warn "检测到依赖缺失，开始安装..."
    "$PIP_BIN" install --upgrade pip >/dev/null
    "$PIP_BIN" install -r "${PROJECT_DIR}/requirements.txt"
}

ensure_env_file() {
    ENV_CREATED=0
    if [[ ! -f "${PROJECT_DIR}/.env" ]]; then
        if [[ -f "${PROJECT_DIR}/.env.example" ]]; then
            cp "${PROJECT_DIR}/.env.example" "${PROJECT_DIR}/.env"
            ENV_CREATED=1
            log_warn "已根据模板创建 .env，请填写必需配置后重新运行。"
        else
            cat > "${PROJECT_DIR}/.env" <<'EOF'
TELEGRAM_API_ID=
TELEGRAM_API_HASH=
TELEGRAM_PHONE_NUMBER=
TELEGRAM_PASSWORD=
TELEGRAM_SESSION_NAME=telegram_monitor_optimized
LISTEN_ALL_SUBSCRIBED_CHANNELS=false
BLOCK_PRIVATE_MESSAGES=false
BLOCKED_SENDER_IDS=777000
CHANNEL_ALLOWLIST=
ENABLE_SENDER_WHITELIST=false
DATABASE_PATH=./data/jtbot.db
USE_PROXY=true
HTTP_PROXY=http://127.0.0.1:9910
HTTPS_PROXY=http://127.0.0.1:9910
EOF
            ENV_CREATED=1
            log_warn "已生成空的 .env，请填写配置后重新运行。"
        fi
    fi
}

load_env() {
    set -a
    # shellcheck disable=SC1091
    source "${PROJECT_DIR}/.env"
    set +a
}

validate_env() {
    local missing=()
    local required=(TELEGRAM_API_ID TELEGRAM_API_HASH TELEGRAM_PHONE_NUMBER)

    for var in "${required[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            missing+=("$var")
        fi
    done

    if ((${#missing[@]} > 0)); then
        log_error "缺少必需的环境变量: ${missing[*]}"
        log_info  "请编辑 ${PROJECT_DIR}/.env 填写上述内容。"
        exit 1
    fi
}

ensure_directories() {
    mkdir -p "$LOG_DIR" "$DATA_DIR" "$SESSION_DIR" "$PID_DIR"
}

check_network() {
    log_info "检查网络连接..."

    # 测试直连 Telegram (使用一个快速的超时)
    if timeout 3 bash -c "cat < /dev/null > /dev/tcp/149.154.167.51/443" 2>/dev/null; then
        log_success "✅ 直连 Telegram 成功,将优先使用直连。"
        return 0
    fi

    log_warn "⚠️  直连 Telegram 失败。"

    # 检查是否配置了代理
    if [[ "${USE_PROXY:-false}" == "true" ]]; then
        local proxy_host="${PROXY_HOST:-127.0.0.1}"
        local proxy_port="${PROXY_PORT:-9910}"

        log_info "检测到代理配置: ${proxy_host}:${proxy_port}"

        # 测试代理是否可用
        if timeout 3 bash -c "cat < /dev/null > /dev/tcp/${proxy_host}/${proxy_port}" 2>/dev/null; then
            log_success "✅ 代理服务可用,将使用代理连接。"
            return 0
        else
            log_error "❌ 代理服务不可用,请检查:"
            log_info "   1. 代理服务是否启动 (${proxy_host}:${proxy_port})"
            log_info "   2. .env 中的代理配置是否正确"
            exit 1
        fi
    else
        log_warn "未配置代理,但直连失败。建议:"
        log_info "   1. 检查网络连接"
        log_info "   2. 或在 .env 中配置代理: USE_PROXY=true"
        log_info "正在继续尝试..."
    fi
}

prompt_for_auth() {
    if [[ $AUTO_APPROVE -eq 1 ]]; then
        RESPONSE="y"
    else
        read -r -p "是否立即运行 Telegram 登录向导? [y/N]: " RESPONSE
    fi

    if [[ "$RESPONSE" =~ ^[Yy]$ ]]; then
        log_info "启动 Telegram 登录向导，请按提示完成验证。"
        "$PYTHON_BIN" -m telegram_collector.jt_bot auth || true
    else
        log_warn "已跳过登录，请稍后手动运行: ${PYTHON_BIN} -m telegram_collector.jt_bot auth"
        return 1
    fi
}

check_session() {
    local session_name="${TELEGRAM_SESSION_NAME:-telegram_monitor_optimized}"
    local status_output
    local status
    # 临时禁用 errexit 以捕获非零退出码
    set +e
    status_output="$("$PYTHON_BIN" -m telegram_collector.jt_bot session-status 2>&1)"
    status=$?
    set -e

    if [[ $status -eq 0 ]]; then
        log_success "检测到有效的 Telegram 会话 (${session_name})."
        return 0
    fi

    if [[ -n "$status_output" ]]; then
        printf '%s\n' "$status_output"
    fi

    if [[ $status -eq 2 ]]; then
        log_warn "未检测到有效的 Telegram 会话 (${session_name})。"
        log_info "正在自动启动 Telegram 登录向导..."

        # 自动运行登录流程，不再询问用户
        if "$PYTHON_BIN" -m telegram_collector.jt_bot auth; then
            # 登录成功后重新检查会话状态
            status_output="$("$PYTHON_BIN" -m telegram_collector.jt_bot session-status 2>&1)"
            status=$?
            if [[ $status -eq 0 ]]; then
                log_success "Telegram 会话创建成功。"
                return 0
            fi
            if [[ -n "$status_output" ]]; then
                printf '%s\n' "$status_output"
            fi
            log_error "登录完成但会话验证失败，请检查配置。"
            exit 1
        else
            log_error "Telegram 登录流程未完成或失败。"
            exit 1
        fi
    fi

    log_error "检查 Telegram 会话状态失败，请运行 ${PYTHON_BIN} -m telegram_collector.jt_bot session-status 查看详细信息。"
    exit 1
}

stop_process_by_pidfile() {
    local file="$1"
    local name="$2"
    local quiet="${3:-0}"

    if [[ -f "$file" ]]; then
        local pid
        pid="$(cat "$file")"
        if kill -0 "$pid" >/dev/null 2>&1; then
            kill "$pid" >/dev/null 2>&1 || true
            sleep 1
            if kill -0 "$pid" >/dev/null 2>&1; then
                kill -9 "$pid" >/dev/null 2>&1 || true
            fi
            if [[ $quiet -eq 0 ]]; then
                log_info "已停止 ${name} (PID: $pid)"
            fi
        fi
        rm -f "$file"
    fi
}

stop_processes() {
    local quiet="${1:-0}"
    stop_process_by_pidfile "$MONITOR_PID_FILE" "Telegram 监听器" "$quiet"
    pkill -f "telegram_collector.jt_bot monitor" >/dev/null 2>&1 || true
}

bootstrap() {
    ensure_python
    ensure_venv
    activate_venv
    install_dependencies
    ensure_env_file
    if [[ ${ENV_CREATED:-0} -eq 1 ]]; then
        log_warn "请编辑 .env 后再次运行本脚本。"
        exit 0
    fi
    load_env
    validate_env
    ensure_directories
    check_network
    check_session
    log_success "环境检查完成。"
}

start_services() {
    local follow="${1:-1}"
    bootstrap
    stop_processes 1

    touch "${LOG_DIR}/jt_bot.log"

    log_info "启动 Telegram 监听器..."
    "$PYTHON_BIN" -m telegram_collector.jt_bot monitor >> "${LOG_DIR}/jt_bot.log" 2>&1 &
    local monitor_pid=$!
    echo "$monitor_pid" > "$MONITOR_PID_FILE"
    log_success "Telegram 监听器已启动 (PID: $monitor_pid)"

    log_info "日志路径:"
    echo -e "  ${GRAY}${LOG_DIR}/jt_bot.log${NC} (Telegram)"

    if [[ "$follow" -eq 1 ]]; then
        log_info "正在跟随日志，按 Ctrl+C 可停止所有服务。"
        trap 'echo; log_info "正在停止服务..."; stop_processes; log_success "服务已停止"; exit 0' INT TERM

        tail -n0 -F "${LOG_DIR}/jt_bot.log"

        stop_processes
        log_success "服务已停止。"
    else
        log_success "服务已在后台运行，可使用 ./start.sh status 查看状态。"
    fi
}

status_services() {
    load_env 2>/dev/null || true

    echo -e "${CYAN}╔════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              JT-Bot 运行状态                ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════╝${NC}"

    if [[ -f "$MONITOR_PID_FILE" ]]; then
        local pid
        pid="$(cat "$MONITOR_PID_FILE")"
        if kill -0 "$pid" >/dev/null 2>&1; then
            log_success "Telegram 监听器运行中 (PID: $pid)"
        else
            log_warn "Telegram 监听器 PID 文件存在但进程未运行。"
        fi
    else
        log_warn "Telegram 监听器未运行。"
    fi

    local db_path="${DATABASE_PATH:-${PROJECT_DIR}/data/jtbot.db}"
    if [[ -f "$db_path" ]]; then
        local size
        size="$(du -h "$db_path" | awk '{print $1}')"
        log_info "数据库文件: $db_path (大小: $size)"
    else
        log_warn "尚未生成数据库文件 (可能尚未收到消息)。"
    fi
}

clean_runtime() {
    stop_processes 1
    rm -f "${LOG_DIR}/"*.log
    rm -f "$MONITOR_PID_FILE"
    log_success "日志与运行时文件已清理。"
}

run_auth() {
    bootstrap
    "$PYTHON_BIN" -m telegram_collector.jt_bot auth
}

main() {
    parse_args "$@"

    case "$COMMAND" in
        start)
            start_services "$FOLLOW_LOGS"
            ;;
        stop)
            stop_processes
            ;;
        status)
            status_services
            ;;
        setup)
            bootstrap
            ;;
        auth)
            run_auth
            ;;
        clean)
            clean_runtime
            ;;
        restart)
            stop_processes
            start_services "$FOLLOW_LOGS"
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

main "$@"
