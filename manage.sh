#!/bin/bash

# =================================================================
# Nofx 项目服务管理脚本 (纯 Shell 版本)
#
# 使用方法:
#   ./manage.sh start   - 启动后端和前端服务
#   ./manage.sh stop    - 停止后端和前端服务
#   ./manage.sh restart - 重启所有服务
#   ./manage.sh status  - 查看所有服务状态
# =================================================================

# --- 配置区 ---
# 项目根目录
PROJECT_ROOT="/root/nofx"

# 后端服务配置
BACKEND_NAME="nofx-backend"
BACKEND_CMD="./nofx"  # 在项目根目录下执行
BACKEND_LOG="$PROJECT_ROOT/logs/nofx.log"
BACKEND_PID_FILE="$PROJECT_ROOT/$BACKEND_NAME.pid"

# 前端服务配置
FRONTEND_NAME="nofx-frontend"
FRONTEND_CMD="npm run dev"  # 在 web 目录下执行
FRONTEND_LOG="$PROJECT_ROOT/logs/web.log"
FRONTEND_PID_FILE="$PROJECT_ROOT/$FRONTEND_NAME.pid"
FRONTEND_WORKING_DIR="$PROJECT_ROOT/web"
# --- 配置区结束 ---


# 检查日志目录是否存在，不存在则创建
check_log_dir() {
    local log_dir=$(dirname "$BACKEND_LOG")
    if [ ! -d "$log_dir" ]; then
        echo "日志目录 $log_dir 不存在，正在创建..."
        mkdir -p "$log_dir"
    fi
}

# 启动服务的通用函数
start_service() {
    local name=$1
    local cmd=$2
    local log=$3
    local pid_file=$4
    local working_dir=$5 # 新增：工作目录

    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p $pid > /dev/null; then
            echo "-> 服务 '$name' 已经在运行中 (PID: $pid)。"
            return
        else
            echo "-> 发现 '$name' 的旧 PID 文件，但进程不存在，正在清理..."
            rm "$pid_file"
        fi
    fi

    echo "=> 正在启动服务 '$name'..."
    check_log_dir
    
    # 使用 nohup 在指定的工作目录中启动命令
    # $! 会获取最后一个后台进程的 PID
    nohup bash -c "cd $working_dir && $cmd" > "$log" 2>&1 &
    local pid=$!
    
    # 将 PID 写入 PID 文件
    echo $pid > "$pid_file"
    
    # 短暂等待，检查进程是否成功启动
    sleep 2
    if ps -p $pid > /dev/null; then
        echo "=> 服务 '$name' 启动成功！PID: $pid, 日志: $log"
    else
        echo "=> [错误] 服务 '$name' 启动失败。请检查日志文件: $log"
        rm "$pid_file" # 启动失败，删除无效的 PID 文件
    fi
}

# 停止服务的通用函数
stop_service() {
    local name=$1
    local pid_file=$2

    if [ ! -f "$pid_file" ]; then
        echo "-> 服务 '$name' 未运行 (未找到 PID 文件)。"
        return
    fi

    local pid=$(cat "$pid_file")
    if ps -p $pid > /dev/null; then
        echo "<= 正在停止服务 '$name' (PID: $pid)..."
        kill $pid
        
        # 等待进程结束，最多等待10秒
        local count=0
        while ps -p $pid > /dev/null; do
            if [ $count -ge 10 ]; then
                echo "-> 进程 $pid 未能正常退出，强制杀死 (kill -9)..."
                kill -9 $pid
                break
            fi
            echo "-> 等待进程 $pid 结束... ($count/10)"
            sleep 1
            ((count++))
        done
        
        echo "<= 服务 '$name' 已停止。"
    else
        echo "-> 服务 '$name' 的 PID 文件存在，但进程 $pid 未在运行。正在清理 PID 文件。"
    fi
    
    rm "$pid_file"
}

# 查看服务状态的通用函数
status_service() {
    local name=$1
    local pid_file=$2

    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p $pid > /dev/null; then
            echo "服务 '$name': 正在运行 (PID: $pid)"
        else
            echo "服务 '$name': 已停止 (存在无效的 PID 文件)"
        fi
    else
        echo "服务 '$name': 已停止"
    fi
}

# 显示使用帮助
usage() {
    echo "使用方法: $0 {start|stop|restart|status}"
    exit 1
}

# --- 主逻辑 ---
case "$1" in
    start)
        # 启动后端 (工作目录为项目根目录)
        start_service "$BACKEND_NAME" "$BACKEND_CMD" "$BACKEND_LOG" "$BACKEND_PID_FILE" "$PROJECT_ROOT"
        # 启动前端 (工作目录为 web 目录)
        start_service "$FRONTEND_NAME" "$FRONTEND_CMD" "$FRONTEND_LOG" "$FRONTEND_PID_FILE" "$FRONTEND_WORKING_DIR"
        echo ""
        echo "所有服务启动命令已发送。使用 './manage.sh status' 查看状态。"
        ;;
    stop)
        stop_service "$FRONTEND_NAME" "$FRONTEND_PID_FILE"
        stop_service "$BACKEND_NAME" "$BACKEND_PID_FILE"
        ;;
    restart)
        echo "--- 正在重启所有服务 ---"
        $0 stop
        echo "--- 等待 3 秒 ---"
        sleep 3
        $0 start
        ;;
    status)
        echo "--- Nofx 服务状态 ---"
        status_service "$BACKEND_NAME" "$BACKEND_PID_FILE"
        status_service "$FRONTEND_NAME" "$FRONTEND_PID_FILE"
        ;;
    *)
        usage
        ;;
esac

exit 0