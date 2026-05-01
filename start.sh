#!/bin/bash
# NOFX Startup Script - launches backend + frontend together
# Usage: ./start.sh [--restart]

set -euo pipefail

cd "$(dirname "$0")"

BACKEND_PORT="${API_SERVER_PORT:-${NOFX_BACKEND_PORT:-8080}}"
FRONTEND_PORT="${NOFX_FRONTEND_PORT:-3000}"
PROTECT_ONLY_BOOT="false"
if [[ "${1:-}" == "--protect-only" || "${2:-}" == "--protect-only" ]]; then
    PROTECT_ONLY_BOOT="true"
    export NOFX_PROTECT_ONLY_BOOT=1
fi
BACKEND_HEALTH_URL="http://localhost:${BACKEND_PORT}/api/health"
FRONTEND_URL="http://localhost:${FRONTEND_PORT}/"
BACKEND_PID_FILE="data/nofx_backend.pid"
LEGACY_BACKEND_PID_FILE="nofx.pid"
FRONTEND_PID_FILE="/tmp/nofx-frontend.pid"

write_backend_pid() {
    local pid="$1"
    echo "$pid" > "$BACKEND_PID_FILE"
    echo "$pid" > "$LEGACY_BACKEND_PID_FILE"
}

write_frontend_pid() {
    local pid="$1"
    echo "$pid" > "$FRONTEND_PID_FILE"
}

pid_is_running_cmd() {
    local pid="$1"
    local pattern="$2"
    [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null && ps -p "$pid" -o args= 2>/dev/null | grep -q "$pattern"
}

cleanup_stale_pid_files() {
    if [[ -f "$BACKEND_PID_FILE" ]]; then
        local pid
        pid="$(cat "$BACKEND_PID_FILE" 2>/dev/null || true)"
        if ! pid_is_running_cmd "$pid" "./nofx"; then
            rm -f "$BACKEND_PID_FILE"
        fi
    fi
    if [[ -f "$LEGACY_BACKEND_PID_FILE" ]]; then
        local pid
        pid="$(cat "$LEGACY_BACKEND_PID_FILE" 2>/dev/null || true)"
        if ! pid_is_running_cmd "$pid" "./nofx"; then
            rm -f "$LEGACY_BACKEND_PID_FILE"
        fi
    fi
    if [[ -f "$FRONTEND_PID_FILE" ]]; then
        local pid
        pid="$(cat "$FRONTEND_PID_FILE" 2>/dev/null || true)"
        if ! pid_is_running_cmd "$pid" "vite.*--port ${FRONTEND_PORT}"; then
            rm -f "$FRONTEND_PID_FILE"
        fi
    fi
}

is_backend_healthy() {
    curl -fsS --max-time 2 "$BACKEND_HEALTH_URL" > /dev/null 2>&1
}

is_frontend_healthy() {
    curl -fsS --max-time 2 "$FRONTEND_URL" > /dev/null 2>&1
}

if [[ "${1:-}" == "--restart" ]]; then
    echo "🛑 Restart requested, stopping existing NOFX instances..."
    pkill -f './nofx' 2>/dev/null || true
    pkill -f "vite.*--port ${FRONTEND_PORT}" 2>/dev/null || true
    fuser -k "${BACKEND_PORT}/tcp" 2>/dev/null || true
    fuser -k "${FRONTEND_PORT}/tcp" 2>/dev/null || true
    sleep 2
    rm -f "$BACKEND_PID_FILE" "$LEGACY_BACKEND_PID_FILE" "$FRONTEND_PID_FILE"
else
    echo "🔎 Checking existing instances..."
    cleanup_stale_pid_files
fi

if is_backend_healthy; then
    echo "   ✅ Backend already healthy (${BACKEND_HEALTH_URL})"
else
    echo "🚀 Starting backend..."
    if [[ "$PROTECT_ONLY_BOOT" == "true" ]]; then
        echo "   🛡 Diagnostic protect-only boot requested (NOFX_PROTECT_ONLY_BOOT=1)"
    else
        sqlite3 data/data.db "UPDATE traders SET is_running=1" 2>/dev/null || true
        echo "   🟢 Normal boot: traders marked running before backend start"
    fi
    mkdir -p data
    nohup ./nofx >> data/nofx_stdout.log 2>&1 &
    BACKEND_PID=$!
    write_backend_pid "$BACKEND_PID"
    echo "   Backend PID: $BACKEND_PID"

    BACKEND_WAIT_SECONDS=90
    for i in $(seq 1 "$BACKEND_WAIT_SECONDS"); do
        if is_backend_healthy; then
            echo "   ✅ Backend healthy"
            break
        fi
        if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
            echo "   ❌ Backend exited before becoming healthy"
            echo "   Last backend log lines:"
            tail -n 80 data/nofx_stdout.log || true
            exit 1
        fi
        if [ "$i" -eq "$BACKEND_WAIT_SECONDS" ]; then
            echo "   ❌ Backend failed to start within ${BACKEND_WAIT_SECONDS}s"
            echo "   Last backend log lines:"
            tail -n 80 data/nofx_stdout.log || true
            exit 1
        fi
        sleep 1
    done
fi

if is_frontend_healthy; then
    echo "   ✅ Frontend already healthy (${FRONTEND_URL})"
else
    echo "🚀 Starting frontend..."
    cd web
    nohup npx vite --host 0.0.0.0 --port "$FRONTEND_PORT" > /tmp/nofx-frontend.log 2>&1 &
    FRONTEND_PID=$!
    write_frontend_pid "$FRONTEND_PID"
    echo "   Frontend PID: $FRONTEND_PID"
    cd ..

    for i in {1..30}; do
        if is_frontend_healthy; then
            echo "   ✅ Frontend healthy"
            break
        fi
        if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
            echo "   ❌ Frontend exited before becoming healthy"
            echo "   Last frontend log lines:"
            tail -n 80 /tmp/nofx-frontend.log || true
            exit 1
        fi
        if [ "$i" -eq 30 ]; then
            echo "   ⚠️ Frontend may still be starting..."
            tail -n 40 /tmp/nofx-frontend.log || true
        fi
        sleep 1
    done
fi

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  🟢 NOFX Trading System Started          ║"
echo "║  Backend:  http://localhost:${BACKEND_PORT}           ║"
echo "║  Frontend: http://localhost:${FRONTEND_PORT}           ║"
echo "║  Logs:     data/nofx_stdout.log          ║"
echo "╚══════════════════════════════════════════╝"
