#!/bin/bash
# NOFX Startup Script - launches backend + frontend together
# Usage: ./start.sh [--restart]

set -euo pipefail

cd "$(dirname "$0")"

# Kill existing instances
echo "🛑 Stopping existing instances..."
pkill -f './nofx' 2>/dev/null || true
pkill -f 'vite.*--port 3000' 2>/dev/null || true
sleep 2

# Verify ports are free
fuser -k 8080/tcp 2>/dev/null || true
fuser -k 3000/tcp 2>/dev/null || true
sleep 1

# Start backend
echo "🚀 Starting backend..."
nohup ./nofx >> data/nofx_stdout.log 2>&1 &
BACKEND_PID=$!
echo "   Backend PID: $BACKEND_PID"

# Wait for backend health
BACKEND_WAIT_SECONDS=90
for i in $(seq 1 $BACKEND_WAIT_SECONDS); do
    if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
        echo "   ✅ Backend healthy"
        break
    fi
    if [ $i -eq $BACKEND_WAIT_SECONDS ]; then
        echo "   ❌ Backend failed to start within ${BACKEND_WAIT_SECONDS}s"
        echo "   Last backend log lines:"
        tail -n 40 data/nofx_stdout.log || true
        exit 1
    fi
    sleep 1
done

# Start frontend
echo "🚀 Starting frontend..."
cd web
nohup npx vite --host 0.0.0.0 --port 3000 > /tmp/nofx-frontend.log 2>&1 &
FRONTEND_PID=$!
echo "   Frontend PID: $FRONTEND_PID"
cd ..

# Wait for frontend health
for i in {1..15}; do
    if curl -s http://localhost:3000/ > /dev/null 2>&1; then
        echo "   ✅ Frontend healthy"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "   ⚠️ Frontend may still be starting..."
    fi
    sleep 1
done

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  🟢 NOFX Trading System Started          ║"
echo "║  Backend:  http://localhost:8080           ║"
echo "║  Frontend: http://localhost:3000           ║"
echo "║  Logs:     data/nofx_2026-*.log           ║"
echo "╚══════════════════════════════════════════╝"
