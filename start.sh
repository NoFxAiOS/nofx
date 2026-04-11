#!/bin/bash
# NOFX Startup Script - launches backend + frontend together
# Usage: ./start.sh [--restart]

cd "$(dirname "$0")"

# Kill existing instances
echo "🛑 Stopping existing instances..."
pkill -f './nofx' 2>/dev/null
pkill -f 'vite.*--port 3000' 2>/dev/null
sleep 2

# Verify ports are free
fuser -k 8080/tcp 2>/dev/null
fuser -k 3000/tcp 2>/dev/null
sleep 1

# Start backend
echo "🚀 Starting backend..."
nohup ./nofx >> data/nofx_stdout.log 2>&1 &
BACKEND_PID=$!
echo "   Backend PID: $BACKEND_PID"

# Wait for backend health
for i in {1..15}; do
    if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
        echo "   ✅ Backend healthy"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "   ❌ Backend failed to start"
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
