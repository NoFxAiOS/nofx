#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# NOFX AI Trading System - Local Development Start Script
# 本地开发启动脚本（不使用 Docker）
# Usage: ./start_local.sh [start|stop|restart|status|logs] [--dev]
# ═══════════════════════════════════════════════════════════════

set -e

# ------------------------------------------------------------------------
# Color Definitions
# ------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ------------------------------------------------------------------------
# Utility Functions
# ------------------------------------------------------------------------
print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# ------------------------------------------------------------------------
# Environment Setup
# ------------------------------------------------------------------------
setup_environment() {
    print_info "检查运行环境..."
    
    # 检查必要工具
    if ! command -v node &> /dev/null; then
        print_error "Node.js 未安装！请先安装 Node.js 18+"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装！请先安装 Go 1.21+"
        exit 1
    fi
    
    # 检查 .env 文件
    if [ ! -f ".env" ]; then
        print_warning ".env 不存在，从模板复制..."
        if [ -f ".env.example" ]; then
            cp .env.example .env
        else
            cat > .env << EOF
NOFX_FRONTEND_PORT=3000
NOFX_BACKEND_PORT=8080
DATA_ENCRYPTION_KEY=your_data_encryption_key_here_change_me
JWT_SECRET=your_jwt_secret_here_change_me
NODE_ENV=development
GO_ENV=development
EOF
        fi
        print_info "已创建 .env 文件"
    fi
    
    # 检查加密环境
    if [ ! -f "secrets/rsa_key" ] || ! grep -q "^DATA_ENCRYPTION_KEY=" .env; then
        print_warning "加密环境未配置，正在自动设置..."
        if [ -f "scripts/setup_encryption.sh" ]; then
            echo -e "Y\nn\nn" | bash scripts/setup_encryption.sh
            print_success "加密环境设置完成"
        fi
    fi
    
    # 检查 config.json
    if [ ! -f "config.json" ]; then
        if [ -f "config.json.example" ]; then
            cp config.json.example config.json
            print_info "已从示例复制 config.json"
        fi
    fi
    
    # 确保目录存在
    mkdir -p secrets logs decision_logs temp database_backups
    chmod 700 secrets
    
    print_success "环境检查完成"
}

# ------------------------------------------------------------------------
# Database Management
# ------------------------------------------------------------------------
check_database() {
    print_info "检查数据库..."
    
    # 如果数据库不存在，会在启动时自动创建并包含 paper_trading
    if [ ! -f "config.db" ]; then
        print_info "数据库不存在，首次启动时会自动创建"
        print_info "将包含以下交易所: Binance, Hyperliquid, Aster, Paper Trading"
    else
        # 备份现有数据库
        local backup_dir="database_backups"
        local timestamp=$(date +%Y%m%d_%H%M%S)
        local backup_file="$backup_dir/config.db.$timestamp"
        
        cp config.db "$backup_file"
        chmod 600 "$backup_file"
        print_success "数据库已备份: $backup_file"
        
        # 清理旧备份（保留最近10个）
        ls -t $backup_dir/config.db.* 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null || true
    fi
}

# ------------------------------------------------------------------------
# Start Services
# ------------------------------------------------------------------------
start_services() {
    local dev_mode=$1
    
    print_info "启动 NOFX AI Trading System (本地模式)..."
    
    # 检查端口
    local backend_port=${NOFX_BACKEND_PORT:-8080}
    local frontend_port=${NOFX_FRONTEND_PORT:-3000}
    
    if lsof -Pi :$backend_port -sTCP:LISTEN -t >/dev/null 2>&1; then
        print_error "端口 $backend_port 已被占用"
        exit 1
    fi
    
    # 清理旧的PID文件
    rm -f nofx.pid frontend.pid
    
    # 启动后端
    print_info "启动后端服务..."
    
    if [ "$dev_mode" == "--dev" ]; then
        export DISABLE_OTP=true
        print_info "开发模式：已禁用2FA验证"
    fi
    
    # 使用源码运行
    nohup go run . > nofx.log 2>&1 &
    BACKEND_PID=$!
    echo $BACKEND_PID > nofx.pid
    
    # 等待后端启动
    sleep 3
    if ! kill -0 $BACKEND_PID 2>/dev/null; then
        print_error "后端启动失败，查看日志: tail -f nofx.log"
        rm -f nofx.pid
        exit 1
    fi
    print_success "后端服务已启动 (PID: $BACKEND_PID, Port: $backend_port)"
    
    # 启动前端
    if [ "$dev_mode" == "--dev" ]; then
        print_info "启动前端开发服务器..."
        cd web
        
        if [ ! -d "node_modules" ]; then
            print_info "安装前端依赖..."
            npm install
        fi
        
        export VITE_API_URL="http://localhost:$backend_port"
        nohup npm run dev > ../frontend.log 2>&1 &
        FRONTEND_PID=$!
        echo $FRONTEND_PID > ../frontend.pid
        cd ..
        
        sleep 5
        if ! kill -0 $FRONTEND_PID 2>/dev/null; then
            print_error "前端启动失败，查看日志: tail -f frontend.log"
            kill $BACKEND_PID 2>/dev/null
            rm -f nofx.pid frontend.pid
            exit 1
        fi
        print_success "前端开发服务器已启动 (PID: $FRONTEND_PID, Port: $frontend_port)"
    else
        # 生产模式：构建前端
        print_info "构建前端生产版本..."
        cd web
        
        if [ ! -d "node_modules" ]; then
            npm install
        fi
        
        npm run build
        cd ..
        print_success "前端已构建（通过后端 :$backend_port 提供服务）"
    fi
    
    # 显示启动信息
    echo ""
    print_success "🎯 NOFX AI Trading System 启动完成！"
    echo ""
    if [ "$dev_mode" == "--dev" ]; then
        echo "📱 前端开发服务器: http://localhost:$frontend_port"
    else
        echo "📱 Web 界面: http://localhost:$backend_port"
    fi
    echo "🔗 API 端点: http://localhost:$backend_port"
    echo ""
    echo "📊 服务状态:"
    echo "  ✅ 后端服务运行中 (PID: $BACKEND_PID)"
    if [ "$dev_mode" == "--dev" ]; then
        echo "  ✅ 前端开发服务器运行中 (PID: $FRONTEND_PID)"
    fi
    echo ""
    echo "📋 常用命令:"
    echo "  查看服务状态: ./start_local.sh status"
    echo "  查看后端日志: tail -f nofx.log"
    if [ "$dev_mode" == "--dev" ]; then
        echo "  查看前端日志: tail -f frontend.log"
    fi
    echo "  停止服务: ./start_local.sh stop"
    echo "  重启服务: ./start_local.sh restart $dev_mode"
    echo ""
    echo "💡 Paper Trading 已启用！"
    echo "   登录后在交易所配置中可以看到 'Paper Trading (Binance Testnet)'"
    echo ""
}

# ------------------------------------------------------------------------
# Stop Services
# ------------------------------------------------------------------------
stop_services() {
    print_info "停止服务..."
    
    local stopped=0
    
    # 停止前端
    if [ -f "frontend.pid" ]; then
        FRONTEND_PID=$(cat frontend.pid)
        if kill -0 $FRONTEND_PID 2>/dev/null; then
            kill $FRONTEND_PID
            print_success "前端服务已停止"
            stopped=1
        fi
        rm -f frontend.pid
    fi
    
    # 停止后端
    if [ -f "nofx.pid" ]; then
        BACKEND_PID=$(cat nofx.pid)
        if kill -0 $BACKEND_PID 2>/dev/null; then
            kill $BACKEND_PID
            print_success "后端服务已停止"
            stopped=1
        fi
        rm -f nofx.pid
    fi
    
    if [ $stopped -eq 0 ]; then
        print_warning "没有运行中的服务"
    else
        print_success "所有服务已停止"
    fi
}

# ------------------------------------------------------------------------
# Status Check
# ------------------------------------------------------------------------
check_status() {
    print_info "检查服务状态..."
    
    local backend_running=0
    local frontend_running=0
    
    # 检查后端
    if [ -f "nofx.pid" ]; then
        BACKEND_PID=$(cat nofx.pid)
        if kill -0 $BACKEND_PID 2>/dev/null; then
            print_success "✅ 后端服务运行中 (PID: $BACKEND_PID)"
            backend_running=1
        else
            print_warning "❌ 后端服务未运行 (PID文件存在但进程不存在)"
            rm -f nofx.pid
        fi
    else
        print_warning "❌ 后端服务未运行"
    fi
    
    # 检查前端
    if [ -f "frontend.pid" ]; then
        FRONTEND_PID=$(cat frontend.pid)
        if kill -0 $FRONTEND_PID 2>/dev/null; then
            print_success "✅ 前端服务运行中 (PID: $FRONTEND_PID)"
            frontend_running=1
        else
            print_warning "❌ 前端服务未运行 (PID文件存在但进程不存在)"
            rm -f frontend.pid
        fi
    fi
    
    if [ $backend_running -eq 0 ] && [ $frontend_running -eq 0 ]; then
        print_warning "所有服务都未运行"
        return 1
    fi
    
    return 0
}

# ------------------------------------------------------------------------
# View Logs
# ------------------------------------------------------------------------
view_logs() {
    local service=${1:-all}
    
    case "$service" in
        backend)
            if [ -f "nofx.log" ]; then
                tail -f nofx.log
            else
                print_error "后端日志文件不存在"
            fi
            ;;
        frontend)
            if [ -f "frontend.log" ]; then
                tail -f frontend.log
            else
                print_error "前端日志文件不存在"
            fi
            ;;
        all|*)
            if [ -f "nofx.log" ] && [ -f "frontend.log" ]; then
                tail -f nofx.log frontend.log
            elif [ -f "nofx.log" ]; then
                tail -f nofx.log
            else
                print_error "日志文件不存在"
            fi
            ;;
    esac
}

# ------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------
main() {
    local command=${1:-start}
    local mode=${2}
    
    case "$command" in
        start)
            setup_environment
            check_database
            start_services "$mode"
            ;;
        stop)
            stop_services
            ;;
        restart)
            stop_services
            sleep 2
            setup_environment
            check_database
            start_services "$mode"
            ;;
        status)
            check_status
            ;;
        logs)
            view_logs "$mode"
            ;;
        *)
            echo "Usage: $0 {start|stop|restart|status|logs} [--dev]"
            echo ""
            echo "Commands:"
            echo "  start [--dev]   启动服务（默认生产模式，--dev 开发模式）"
            echo "  stop            停止服务"
            echo "  restart [--dev] 重启服务"
            echo "  status          查看状态"
            echo "  logs [service]  查看日志 (backend/frontend/all)"
            echo ""
            echo "Examples:"
            echo "  $0 start --dev          # 开发模式启动"
            echo "  $0 start                # 生产模式启动"
            echo "  $0 logs backend         # 查看后端日志"
            echo "  $0 status               # 查看状态"
            exit 1
            ;;
    esac
}

main "$@"
