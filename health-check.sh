#!/bin/bash

# NOFX 交易系统 - 服务器健康检查脚本
# 用于监控系统状态并自动处理异常情况

set -e

# 配置变量
BACKEND_URL="http://localhost:8080"
FRONTEND_URL="http://localhost:3000"
LOG_FILE="/var/log/nofx-health.log"
TELEGRAM_BOT_TOKEN=""  # 可选：填入你的 Telegram Bot Token
TELEGRAM_CHAT_ID=""    # 可选：填入你的 Telegram Chat ID

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 发送 Telegram 通知（可选）
send_telegram() {
    if [[ -n "$TELEGRAM_BOT_TOKEN" && -n "$TELEGRAM_CHAT_ID" ]]; then
        curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
            -d chat_id="$TELEGRAM_CHAT_ID" \
            -d text="🚨 NOFX Alert: $1" > /dev/null
    fi
}

# 检查容器状态
check_containers() {
    log "检查 Docker 容器状态..."
    
    # 检查后端容器
    if ! docker ps | grep -q "nofx-trading-prod"; then
        log "❌ 后端容器未运行，尝试重启..."
        docker-compose -f /opt/nofx-trading/docker-compose.prod.yml up -d nofx-trading
        send_telegram "后端容器已重启"
    else
        log "✅ 后端容器运行正常"
    fi
    
    # 检查前端容器
    if ! docker ps | grep -q "nofx-frontend-prod"; then
        log "❌ 前端容器未运行，尝试重启..."
        docker-compose -f /opt/nofx-trading/docker-compose.prod.yml up -d nofx-frontend
        send_telegram "前端容器已重启"
    else
        log "✅ 前端容器运行正常"
    fi
}

# 检查服务健康状态
check_health() {
    log "检查服务健康状态..."
    
    # 检查后端 API
    if ! curl -f -s "$BACKEND_URL/api/status" > /dev/null; then
        log "❌ 后端 API 无响应"
        send_telegram "后端 API 无响应，可能需要人工检查"
        return 1
    else
        log "✅ 后端 API 响应正常"
    fi
    
    # 检查前端
    if ! curl -f -s "$FRONTEND_URL" > /dev/null; then
        log "❌ 前端无响应"
        send_telegram "前端无响应，可能需要人工检查"
        return 1
    else
        log "✅ 前端响应正常"
    fi
    
    return 0
}

# 检查系统资源
check_resources() {
    log "检查系统资源..."
    
    # 检查磁盘空间
    DISK_USAGE=$(df / | awk 'NR==2 {print $5}' | sed 's/%//')
    if [ "$DISK_USAGE" -gt 85 ]; then
        log "⚠️  磁盘使用率过高: ${DISK_USAGE}%"
        send_telegram "磁盘使用率过高: ${DISK_USAGE}%"
    else
        log "✅ 磁盘使用率正常: ${DISK_USAGE}%"
    fi
    
    # 检查内存使用
    MEMORY_USAGE=$(free | awk 'NR==2{printf "%.1f", $3*100/$2}')
    if (( $(echo "$MEMORY_USAGE > 85" | bc -l) )); then
        log "⚠️  内存使用率过高: ${MEMORY_USAGE}%"
        send_telegram "内存使用率过高: ${MEMORY_USAGE}%"
    else
        log "✅ 内存使用率正常: ${MEMORY_USAGE}%"
    fi
}

# 检查交易决策日志
check_trading_logs() {
    log "检查交易决策日志..."
    
    LOGS_DIR="/opt/nofx-trading/decision_logs"
    LATEST_LOG=$(find "$LOGS_DIR" -name "*.json" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
    
    if [[ -z "$LATEST_LOG" ]]; then
        log "⚠️  未找到决策日志文件"
        send_telegram "未找到决策日志文件，交易可能已停止"
        return 1
    fi
    
    # 检查最新日志的时间
    LAST_MODIFIED=$(stat -c %Y "$LATEST_LOG")
    CURRENT_TIME=$(date +%s)
    TIME_DIFF=$((CURRENT_TIME - LAST_MODIFIED))
    
    # 如果超过 30 分钟没有新的决策日志，发出警告
    if [ "$TIME_DIFF" -gt 1800 ]; then
        log "⚠️  交易决策日志超过 30 分钟未更新"
        send_telegram "交易决策日志超过 30 分钟未更新，可能存在问题"
        return 1
    else
        log "✅ 交易决策日志更新正常"
    fi
    
    return 0
}

# 清理旧日志
cleanup_logs() {
    log "清理旧日志文件..."
    
    # 清理超过 7 天的决策日志
    find /opt/nofx-trading/decision_logs -name "*.json" -type f -mtime +7 -delete
    
    # 清理超过 30 天的健康检查日志
    if [[ -f "$LOG_FILE" ]]; then
        find "$(dirname "$LOG_FILE")" -name "$(basename "$LOG_FILE")" -type f -mtime +30 -delete
    fi
    
    log "日志清理完成"
}

# 主函数
main() {
    log "==============================================="
    log "开始 NOFX 系统健康检查"
    log "==============================================="
    
    # 确保日志目录存在
    mkdir -p "$(dirname "$LOG_FILE")"
    
    # 执行各项检查
    check_containers
    sleep 5  # 等待容器启动
    
    if check_health && check_trading_logs; then
        log "✅ 所有检查通过，系统运行正常"
    else
        log "❌ 发现问题，已记录并通知"
    fi
    
    check_resources
    cleanup_logs
    
    log "健康检查完成"
    log "==============================================="
}

# 如果脚本被直接执行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi