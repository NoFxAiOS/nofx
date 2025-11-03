#!/bin/bash

# NOFX 交易系统 - 备份和恢复脚本
# 用于备份交易数据、配置文件和决策日志

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="/opt/nofx-backups"
NOFX_DIR="/opt/nofx-trading"
DATE_STAMP=$(date '+%Y%m%d_%H%M%S')

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# 备份函数
backup() {
    local backup_name="nofx_backup_${DATE_STAMP}"
    local backup_path="$BACKUP_DIR/$backup_name"
    
    log "开始备份到: $backup_path"
    
    # 创建备份目录
    mkdir -p "$backup_path"
    
    # 备份配置文件
    log "备份配置文件..."
    cp "$NOFX_DIR/config.json" "$backup_path/" 2>/dev/null || log "警告: config.json 不存在"
    cp "$NOFX_DIR/.env" "$backup_path/" 2>/dev/null || log "警告: .env 不存在"
    cp "$NOFX_DIR/docker-compose.prod.yml" "$backup_path/" 2>/dev/null || log "警告: docker-compose.prod.yml 不存在"
    
    # 备份决策日志（最近 30 天）
    log "备份决策日志..."
    if [[ -d "$NOFX_DIR/decision_logs" ]]; then
        mkdir -p "$backup_path/decision_logs"
        find "$NOFX_DIR/decision_logs" -name "*.json" -type f -mtime -30 -exec cp {} "$backup_path/decision_logs/" \;
    fi
    
    # 备份系统状态信息
    log "备份系统状态..."
    echo "备份时间: $(date)" > "$backup_path/backup_info.txt"
    echo "系统信息: $(uname -a)" >> "$backup_path/backup_info.txt"
    echo "Docker 版本: $(docker --version)" >> "$backup_path/backup_info.txt"
    echo "Docker Compose 版本: $(docker-compose --version)" >> "$backup_path/backup_info.txt"
    
    # 记录容器状态
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" > "$backup_path/container_status.txt" 2>/dev/null || true
    
    # 创建压缩包
    log "创建压缩包..."
    cd "$BACKUP_DIR"
    tar -czf "${backup_name}.tar.gz" "$backup_name"
    rm -rf "$backup_name"
    
    log "备份完成: $BACKUP_DIR/${backup_name}.tar.gz"
    
    # 清理超过 30 天的备份
    find "$BACKUP_DIR" -name "nofx_backup_*.tar.gz" -type f -mtime +30 -delete
    
    # 显示备份大小
    local backup_size=$(du -h "$BACKUP_DIR/${backup_name}.tar.gz" | cut -f1)
    log "备份大小: $backup_size"
}

# 恢复函数
restore() {
    local backup_file="$1"
    
    if [[ -z "$backup_file" ]]; then
        log "错误: 请指定备份文件路径"
        echo "用法: $0 restore <backup_file.tar.gz>"
        echo "可用备份:"
        ls -la "$BACKUP_DIR"/*.tar.gz 2>/dev/null || echo "未找到备份文件"
        exit 1
    fi
    
    if [[ ! -f "$backup_file" ]]; then
        log "错误: 备份文件不存在: $backup_file"
        exit 1
    fi
    
    log "开始从备份恢复: $backup_file"
    
    # 停止服务
    log "停止 NOFX 服务..."
    docker-compose -f "$NOFX_DIR/docker-compose.prod.yml" down 2>/dev/null || true
    
    # 备份当前配置（以防万一）
    local current_backup="$BACKUP_DIR/before_restore_$(date '+%Y%m%d_%H%M%S')"
    mkdir -p "$current_backup"
    cp "$NOFX_DIR"/*.json "$current_backup/" 2>/dev/null || true
    cp "$NOFX_DIR/.env" "$current_backup/" 2>/dev/null || true
    log "当前配置已备份到: $current_backup"
    
    # 解压备份文件
    local temp_dir=$(mktemp -d)
    cd "$temp_dir"
    tar -xzf "$backup_file"
    
    local backup_dir=$(find . -maxdepth 1 -type d -name "nofx_backup_*" | head -1)
    if [[ -z "$backup_dir" ]]; then
        log "错误: 备份文件格式不正确"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # 恢复文件
    log "恢复配置文件..."
    cp "$backup_dir"/*.json "$NOFX_DIR/" 2>/dev/null || true
    cp "$backup_dir/.env" "$NOFX_DIR/" 2>/dev/null || true
    cp "$backup_dir/docker-compose.prod.yml" "$NOFX_DIR/" 2>/dev/null || true
    
    # 恢复决策日志
    if [[ -d "$backup_dir/decision_logs" ]]; then
        log "恢复决策日志..."
        mkdir -p "$NOFX_DIR/decision_logs"
        cp -r "$backup_dir/decision_logs"/* "$NOFX_DIR/decision_logs/" 2>/dev/null || true
    fi
    
    # 清理临时文件
    rm -rf "$temp_dir"
    
    # 重启服务
    log "重启 NOFX 服务..."
    cd "$NOFX_DIR"
    docker-compose -f docker-compose.prod.yml up -d
    
    log "恢复完成!"
    log "请检查服务状态: docker ps"
}

# 列出备份
list_backups() {
    log "可用备份列表:"
    echo "=================================================================="
    
    if ls "$BACKUP_DIR"/nofx_backup_*.tar.gz >/dev/null 2>&1; then
        for backup in "$BACKUP_DIR"/nofx_backup_*.tar.gz; do
            local size=$(du -h "$backup" | cut -f1)
            local date=$(stat -c %y "$backup" | cut -d' ' -f1,2 | cut -d'.' -f1)
            printf "%-50s %8s %s\n" "$(basename "$backup")" "$size" "$date"
        done
    else
        echo "未找到备份文件"
    fi
    
    echo "=================================================================="
}

# 显示帮助
show_help() {
    echo "NOFX 交易系统备份和恢复工具"
    echo ""
    echo "用法:"
    echo "  $0 backup                           # 创建新备份"
    echo "  $0 restore <backup_file.tar.gz>    # 从备份恢复"
    echo "  $0 list                            # 列出可用备份"
    echo "  $0 help                            # 显示此帮助"
    echo ""
    echo "示例:"
    echo "  $0 backup"
    echo "  $0 restore /opt/nofx-backups/nofx_backup_20240101_120000.tar.gz"
    echo "  $0 list"
}

# 主函数
main() {
    case "${1:-}" in
        backup)
            backup
            ;;
        restore)
            restore "$2"
            ;;
        list)
            list_backups
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log "错误: 未知命令"
            show_help
            exit 1
            ;;
    esac
}

# 确保以 root 权限运行
if [[ $EUID -ne 0 ]]; then
    echo "此脚本需要 root 权限运行"
    echo "请使用: sudo $0 $*"
    exit 1
fi

# 执行主函数
main "$@"