#!/bin/bash

# NOFX 系统一键启动脚本
# 用途: 检查环境、配置、并启动系统

set -e  # 遇到错误立即退出

echo "╔════════════════════════════════════════════════════════════╗"
echo "║           🚀 NOFX 系统启动脚本 v1.0                       ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# ============================================================================
# 1. 环境检查
# ============================================================================

echo "📋 步骤1: 环境检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 检查Go
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装"
    echo ""
    echo "请安装Go 1.21+:"
    echo "  wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz"
    echo "  sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz"
    echo "  export PATH=\$PATH:/usr/local/go/bin"
    exit 1
fi
echo "✓ Go 已安装: $(go version)"

# 检查Python
if ! command -v python3 &> /dev/null; then
    echo "❌ Python3 未安装"
    exit 1
fi
echo "✓ Python3 已安装: $(python3 --version)"

# 检查经济日历数据库
if [ ! -f "economic_calendar.db" ]; then
    echo "⚠️  经济日历数据库不存在，将在首次运行时创建"
else
    db_size=$(du -h economic_calendar.db | cut -f1)
    echo "✓ 经济日历数据库存在 (大小: $db_size)"
fi

echo ""

# ============================================================================
# 2. 配置检查
# ============================================================================

echo "📋 步骤2: 配置检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 检查config.json
if [ ! -f "config.json" ]; then
    echo "⚠️  config.json 不存在，从模板创建..."
    if [ -f "config.json.example" ]; then
        cp config.json.example config.json
        echo "✓ 已创建 config.json (使用默认配置)"
        echo "  可以稍后通过Web界面修改配置"
    else
        echo "❌ 找不到 config.json.example"
        exit 1
    fi
else
    echo "✓ config.json 存在"
fi

# 检查Python依赖
echo "检查Python依赖..."
cd world/经济日历
if python3 -c "import requests, lxml, pytz" 2>/dev/null; then
    echo "✓ Python依赖已安装"
else
    echo "⚠️  安装Python依赖..."
    pip install -r requirements.txt
    echo "✓ Python依赖安装完成"
fi
cd ../..

echo ""

# ============================================================================
# 3. 编译Go程序
# ============================================================================

echo "📋 步骤3: 编译程序"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ ! -f "nofx" ] || [ "main.go" -nt "nofx" ]; then
    echo "正在编译 NOFX..."

    # 安装依赖
    echo "  → 下载Go依赖..."
    go mod download

    # 编译
    echo "  → 编译程序..."
    go build -o nofx

    if [ -f "nofx" ]; then
        echo "✓ 编译成功"
    else
        echo "❌ 编译失败"
        exit 1
    fi
else
    echo "✓ nofx 可执行文件已存在且为最新"
fi

echo ""

# ============================================================================
# 4. 启动系统
# ============================================================================

echo "📋 步骤4: 启动系统"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "🚀 启动 NOFX 交易系统..."
echo "   (经济日历服务将自动启动)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 启动NOFX
exec ./nofx
