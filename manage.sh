#!/bin/bash

# =================================================================
# Nofx 项目服务管理脚本 (纯 Shell 版本)
#
# 使用方法:
#   ./manage.sh start   - 启动后端和前端服务（自动初始化环境配置）
#   ./manage.sh stop    - 停止后端和前端服务
#   ./manage.sh restart - 重启所有服务
#   ./manage.sh status  - 查看所有服务状态
#
# 自动化功能:
#   首次启动时，如果检测到 .env 文件不存在，将自动：
#   1. 复制 .env.example 为 .env
#   2. 生成所有必需的密钥并填充到 .env 文件中
# =================================================================

# --- 配置区 ---
# 项目根目录 - 自动获取脚本所在目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# 初始化环境配置文件
init_env_file() {
    local env_file="$PROJECT_ROOT/.env"
    local env_example="$PROJECT_ROOT/.env.example"
    
    # 检查 .env 文件是否已存在
    if [ -f "$env_file" ]; then
        echo "-> 环境配置文件 .env 已存在，跳过初始化。"
        return 0
    fi
    
    echo "-> 环境配置文件 .env 不存在，正在自动初始化..."
    
    # 检查 .env.example 文件是否存在
    if [ ! -f "$env_example" ]; then
        echo "=> [错误] 模板文件 .env.example 不存在，无法初始化环境配置。"
        return 1
    fi
    
    # 复制模板文件
    cp "$env_example" "$env_file"
    echo "=> 已复制 .env.example 到 .env"
    
    # 生成 JWT_SECRET
    local jwt_secret=$(openssl rand -base64 32 2>/dev/null)
    if [ -z "$jwt_secret" ]; then
        # 如果 openssl 不可用，使用随机字符串
        jwt_secret=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    fi
    # 使用 # 作为分隔符避免 base64 中的 / 字符导致问题
    sed -i "s#JWT_SECRET=your-jwt-secret-change-this-in-production#JWT_SECRET=$jwt_secret#" "$env_file"
    echo "=> 已生成 JWT 签名密钥"
    
    # 生成 DATA_ENCRYPTION_KEY
    local data_key=$(openssl rand -base64 32 2>/dev/null)
    if [ -z "$data_key" ]; then
        # 如果 openssl 不可用，使用随机字符串
        data_key=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
    fi
    # 使用 # 作为分隔符避免 base64 中的 / 字符导致问题
    sed -i "s#DATA_ENCRYPTION_KEY=your-base64-encoded-32-byte-key#DATA_ENCRYPTION_KEY=$data_key#" "$env_file"
    echo "=> 已生成 AES-256 数据加密密钥"
    
    # 生成 RSA_PRIVATE_KEY
    local rsa_key_file="/tmp/nofx_rsa_key_$$"
    if openssl genrsa -out "$rsa_key_file" 2048 2>/dev/null; then
        # 将多行 RSA 密钥转换为单行，用 \n 替换换行符
        local rsa_key=$(awk '{printf "%s\\n", $0}' "$rsa_key_file" | sed 's/\\n$//')
        # 使用 # 作为分隔符，并转义密钥中的 # 字符
        local escaped_rsa_key="${rsa_key//#/\#}"
        sed -i "s#RSA_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----\\\\nYOUR_KEY_HERE\\\\n-----END RSA PRIVATE KEY-----|RSA_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----\\\\n${escaped_rsa_key}\\\\n-----END RSA PRIVATE KEY-----|#" "$env_file"
        rm -f "$rsa_key_file"
        echo "=> 已生成 RSA 私钥"
    else
        echo "=> [警告] 无法生成 RSA 私钥，请手动配置 RSA_PRIVATE_KEY"
    fi
    
    echo "=> 环境配置文件初始化完成！"
    return 0
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
        # 初始化环境配置文件
        init_env_file
        if [ $? -ne 0 ]; then
            echo "=> [错误] 环境配置初始化失败，服务启动中止。"
            exit 1
        fi
        
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