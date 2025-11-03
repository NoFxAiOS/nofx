#!/bin/bash

# NOFX 交易系统 - 一键服务器部署脚本
# 适用于 Ubuntu 20.04+ / Debian 11+ / CentOS 8+

set -e

# 颜色代码
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置变量
INSTALL_DIR="/opt/nofx-trading"
SERVICE_USER="nofx"
BACKUP_DIR="/opt/nofx-backups"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查系统要求
check_system() {
    log_step "检查系统要求..."
    
    # 检查是否为 root 用户
    if [[ $EUID -ne 0 ]]; then
        log_error "此脚本需要 root 权限运行"
        echo "请使用: sudo $0"
        exit 1
    fi
    
    # 检查系统类型
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$NAME
        VER=$VERSION_ID
        log_info "检测到系统: $OS $VER"
    else
        log_error "无法检测系统类型"
        exit 1
    fi
    
    # 检查内存
    local memory_mb=$(free -m | awk 'NR==2{print $2}')
    if [[ $memory_mb -lt 1024 ]]; then
        log_warn "内存不足 1GB，可能影响性能"
    else
        log_info "内存检查通过: ${memory_mb}MB"
    fi
    
    # 检查磁盘空间
    local disk_gb=$(df / | awk 'NR==2{print int($4/1024/1024)}')
    if [[ $disk_gb -lt 5 ]]; then
        log_error "磁盘空间不足 5GB"
        exit 1
    else
        log_info "磁盘空间检查通过: ${disk_gb}GB"
    fi
}

# 安装依赖
install_dependencies() {
    log_step "安装系统依赖..."
    
    # 更新包管理器
    if command -v apt-get &> /dev/null; then
        apt-get update
        apt-get install -y curl wget git unzip vim cron bc
    elif command -v yum &> /dev/null; then
        yum update -y
        yum install -y curl wget git unzip vim crontabs bc
    else
        log_error "不支持的包管理器"
        exit 1
    fi
    
    log_info "系统依赖安装完成"
}

# 安装 Docker
install_docker() {
    log_step "安装 Docker..."
    
    if command -v docker &> /dev/null; then
        log_info "Docker 已安装，跳过"
        return
    fi
    
    # 下载 Docker 安装脚本
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    
    # 启动 Docker 服务
    systemctl start docker
    systemctl enable docker
    
    # 安装 Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        log_info "安装 Docker Compose..."
        curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        chmod +x /usr/local/bin/docker-compose
    fi
    
    log_info "Docker 安装完成"
}

# 创建系统用户
create_user() {
    log_step "创建系统用户..."
    
    if id "$SERVICE_USER" &>/dev/null; then
        log_info "用户 $SERVICE_USER 已存在，跳过"
    else
        useradd -r -s /bin/bash -d "$INSTALL_DIR" "$SERVICE_USER"
        usermod -aG docker "$SERVICE_USER"
        log_info "用户 $SERVICE_USER 创建完成"
    fi
}

# 部署应用
deploy_application() {
    log_step "部署 NOFX 应用..."
    
    # 创建安装目录
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$BACKUP_DIR"
    
    # 复制文件到安装目录
    local current_dir=$(pwd)
    log_info "从 $current_dir 复制文件到 $INSTALL_DIR"
    
    # 复制应用文件
    cp -r "$current_dir"/* "$INSTALL_DIR/"
    
    # 设置权限
    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
    chown -R "$SERVICE_USER:$SERVICE_USER" "$BACKUP_DIR"
    
    # 创建环境变量文件
    if [[ ! -f "$INSTALL_DIR/.env" ]]; then
        cp "$INSTALL_DIR/env.server.example" "$INSTALL_DIR/.env"
        log_warn "请编辑 $INSTALL_DIR/.env 文件并填入API密钥"
    fi
    
    # 设置脚本执行权限
    chmod +x "$INSTALL_DIR"/*.sh
    
    log_info "应用部署完成"
}

# 配置服务
configure_services() {
    log_step "配置系统服务..."
    
    # 创建 systemd 服务文件
    cat > /etc/systemd/system/nofx-trading.service << EOF
[Unit]
Description=NOFX Trading System
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=true
WorkingDirectory=$INSTALL_DIR
ExecStart=/usr/local/bin/docker-compose -f docker-compose.prod.yml up -d
ExecStop=/usr/local/bin/docker-compose -f docker-compose.prod.yml down
User=$SERVICE_USER
Group=$SERVICE_USER

[Install]
WantedBy=multi-user.target
EOF
    
    # 重载 systemd
    systemctl daemon-reload
    systemctl enable nofx-trading.service
    
    log_info "系统服务配置完成"
}

# 配置定时任务
configure_cron() {
    log_step "配置定时任务..."
    
    # 健康检查 - 每5分钟
    echo "*/5 * * * * root $INSTALL_DIR/health-check.sh" >> /etc/crontab
    
    # 自动备份 - 每天凌晨2点
    echo "0 2 * * * root $INSTALL_DIR/backup-restore.sh backup" >> /etc/crontab
    
    # 重启 cron 服务
    systemctl restart cron || systemctl restart crond
    
    log_info "定时任务配置完成"
}

# 配置防火墙
configure_firewall() {
    log_step "配置防火墙..."
    
    if command -v ufw &> /dev/null; then
        ufw --force reset
        ufw default deny incoming
        ufw default allow outgoing
        ufw allow ssh
        ufw allow 8080/tcp comment 'NOFX API'
        ufw allow 3000/tcp comment 'NOFX Frontend'
        ufw --force enable
        log_info "UFW 防火墙配置完成"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-service=ssh
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --permanent --add-port=3000/tcp
        firewall-cmd --reload
        log_info "FirewallD 配置完成"
    else
        log_warn "未检测到防火墙，请手动配置"
    fi
}

# 启动服务
start_services() {
    log_step "启动 NOFX 服务..."
    
    cd "$INSTALL_DIR"
    
    # 检查环境变量文件
    if [[ ! -f ".env" ]] || ! grep -q "DEEPSEEK_API_KEY=your_deepseek_api_key_here" .env; then
        log_error "请先配置 .env 文件中的 API 密钥"
        log_info "编辑文件: nano $INSTALL_DIR/.env"
        return 1
    fi
    
    # 启动服务
    systemctl start nofx-trading
    
    # 等待服务启动
    sleep 10
    
    # 检查服务状态
    if docker ps | grep -q "nofx-trading-prod"; then
        log_info "NOFX 服务启动成功"
    else
        log_error "NOFX 服务启动失败，请检查日志"
        docker-compose -f docker-compose.prod.yml logs
        return 1
    fi
}

# 显示部署结果
show_results() {
    log_step "部署完成！"
    
    echo "=================================="
    echo "🎉 NOFX 交易系统部署成功！"
    echo "=================================="
    echo ""
    echo "📁 安装目录: $INSTALL_DIR"
    echo "👤 系统用户: $SERVICE_USER"
    echo "💾 备份目录: $BACKUP_DIR"
    echo ""
    echo "🌐 访问地址:"
    echo "   前端: http://$(hostname -I | awk '{print $1}'):3000"
    echo "   API:  http://$(hostname -I | awk '{print $1}'):8080"
    echo ""
    echo "🔧 管理命令:"
    echo "   启动服务: sudo systemctl start nofx-trading"
    echo "   停止服务: sudo systemctl stop nofx-trading"
    echo "   查看状态: sudo systemctl status nofx-trading"
    echo "   查看日志: sudo docker-compose -f $INSTALL_DIR/docker-compose.prod.yml logs -f"
    echo ""
    echo "🛡️ 运维工具:"
    echo "   健康检查: sudo $INSTALL_DIR/health-check.sh"
    echo "   创建备份: sudo $INSTALL_DIR/backup-restore.sh backup"
    echo "   查看备份: sudo $INSTALL_DIR/backup-restore.sh list"
    echo ""
    echo "⚠️  重要提醒:"
    echo "   1. 请编辑 $INSTALL_DIR/.env 文件配置API密钥"
    echo "   2. 首次启动前务必填入正确的密钥信息"
    echo "   3. 定期检查备份和监控日志"
    echo ""
    echo "🚀 开始使用: sudo systemctl start nofx-trading"
}

# 交互配置
interactive_config() {
    echo "=================================="
    echo "🔧 NOFX 交易系统配置向导"
    echo "=================================="
    echo ""
    
    read -p "请输入 DeepSeek API Key: " deepseek_key
    read -p "请输入 Aster Private Key: " aster_key
    
    # 更新环境变量文件
    sed -i "s/DEEPSEEK_API_KEY=your_deepseek_api_key_here/DEEPSEEK_API_KEY=$deepseek_key/g" "$INSTALL_DIR/.env"
    sed -i "s/ASTER_PRIVATE_KEY=your_aster_private_key_here/ASTER_PRIVATE_KEY=$aster_key/g" "$INSTALL_DIR/.env"
    
    chmod 600 "$INSTALL_DIR/.env"
    chown "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/.env"
    
    log_info "配置已保存"
}

# 主安装流程
main() {
    echo "=================================="
    echo "🚀 NOFX 交易系统一键部署脚本"
    echo "=================================="
    echo ""
    
    check_system
    install_dependencies
    install_docker
    create_user
    deploy_application
    configure_services
    configure_cron
    configure_firewall
    
    # 询问是否现在配置
    read -p "是否现在配置 API 密钥？(y/n): " configure_now
    if [[ $configure_now =~ ^[Yy]$ ]]; then
        interactive_config
        start_services
    else
        log_warn "请稍后手动配置 $INSTALL_DIR/.env 文件"
    fi
    
    show_results
    
    log_info "部署脚本执行完成！"
}

# 执行主函数
main "$@"