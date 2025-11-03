#!/bin/bash

# NOFX Trading Bot 服务器部署脚本
# 优化版本 - 包含严格风控和恐惧贪婪指数

echo "🚀 开始部署 NOFX Trading Bot (优化版)..."

# 检查Docker是否安装
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装，正在安装..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker $USER
    echo "✅ Docker 安装完成，请重新登录后继续"
    exit 1
fi

# 检查Docker Compose是否安装
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose 未安装，正在安装..."
    sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose
    echo "✅ Docker Compose 安装完成"
fi

# 创建目录
echo "📁 创建项目目录..."
sudo mkdir -p /opt/nofx-trading
cd /opt/nofx-trading

# 检查配置文件
if [ ! -f "config.json" ]; then
    echo "⚠️  请先配置 config.json 文件"
    echo "参考模板已创建在 config.json.example"
    exit 1
fi

# 停止现有容器
echo "🛑 停止现有容器..."
sudo docker-compose down 2>/dev/null || true

# 构建并启动
echo "🔨 构建并启动容器..."
sudo docker-compose up -d --build

# 检查状态
echo "📊 检查服务状态..."
sleep 5
sudo docker-compose ps

echo ""
echo "🎉 部署完成！"
echo "📱 前端访问: http://您的服务器IP:3000"
echo "🔧 后端API: http://您的服务器IP:8080"
echo "📝 日志查看: sudo docker-compose logs -f"
echo ""
echo "⚠️  重要提醒:"
echo "1. 请确保配置了正确的API密钥"
echo "2. 建议设置防火墙规则"
echo "3. 定期备份决策日志"
echo "4. 监控系统资源使用情况"