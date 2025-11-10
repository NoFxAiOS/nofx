#!/bin/bash

# NOFX部署验证脚本
# 用于检查部署前后的配置和环境

set -e

echo "=================================="
echo "  NOFX 部署检查工具"
echo "=================================="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查函数
check_command() {
    if command -v "$1" &> /dev/null; then
        echo -e "${GREEN}✓${NC} $1 已安装"
        return 0
    else
        echo -e "${RED}✗${NC} $1 未安装"
        return 1
    fi
}

check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} $1 存在"
        return 0
    else
        echo -e "${RED}✗${NC} $1 不存在"
        return 1
    fi
}

# 1. 检查必要工具
echo "1. 检查必要工具..."
echo "----------------------------"
check_command "git"
check_command "node"
check_command "npm"
check_command "go"
echo ""

# 2. 检查Node版本
echo "2. 检查Node.js版本..."
NODE_VERSION=$(node --version)
echo -e "${GREEN}✓${NC} Node.js版本: $NODE_VERSION"
echo ""

# 3. 检查Go版本
echo "3. 检查Go版本..."
GO_VERSION=$(go version)
echo -e "${GREEN}✓${NC} Go版本: $GO_VERSION"
echo ""

# 4. 检查项目文件
echo "4. 检查项目文件..."
echo "----------------------------"
check_file "config.json"
check_file "web/package.json"
check_file "web/vite.config.ts"
check_file "vercel.json"
check_file "railway.toml"
echo ""

# 5. 检查环境变量
echo "5. 检查环境变量..."
echo "----------------------------"
if [ -f ".env" ]; then
    echo -e "${GREEN}✓${NC} .env 文件存在"
else
    echo -e "${YELLOW}⚠${NC} .env 文件不存在，请参考 .env.example 创建"
fi

if [ -f "web/.env.local" ]; then
    echo -e "${GREEN}✓${NC} web/.env.local 文件存在"
else
    echo -e "${YELLOW}⚠${NC} web/.env.local 文件不存在，请参考 web/.env.example 创建"
fi
echo ""

# 6. 检查依赖
echo "6. 检查依赖..."
echo "----------------------------"
echo "正在检查Go依赖..."
go mod download
echo -e "${GREEN}✓${NC} Go依赖已下载"

echo "正在检查Node依赖..."
cd web
npm install
echo -e "${GREEN}✓${NC} Node依赖已安装"
cd ..
echo ""

# 7. 测试构建
echo "7. 测试构建..."
echo "----------------------------"
echo "正在构建前端..."
cd web
npm run build
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} 前端构建成功"
else
    echo -e "${RED}✗${NC} 前端构建失败"
    exit 1
fi
cd ..

echo "正在构建后端..."
go build -o nofx .
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} 后端构建成功"
else
    echo -e "${RED}✗${NC} 后端构建失败"
    exit 1
fi
echo ""

# 8. 配置检查清单
echo "8. 部署前检查清单..."
echo "----------------------------"
echo -e "${YELLOW}请确认以下项目：${NC}"
echo ""
echo "[ ] 已创建GitHub仓库"
echo "[ ] config.json 中已填入真实API密钥"
echo "[ ] .env 文件已配置（用于Railway）"
echo "[ ] web/.env.local 文件已配置（用于Vercel）"
echo "[ ] 准备使用 Vercel + Railway 部署"
echo ""

# 9. 提供部署命令
echo "9. 下一步操作..."
echo "----------------------------"
echo -e "${GREEN}1. 推送代码到GitHub：${NC}"
echo "   git add ."
echo "   git commit -m 'init: nofx project'"
echo "   git push -u origin main"
echo ""
echo -e "${GREEN}2. 部署到Railway：${NC}"
echo "   - 访问 https://railway.app"
echo "   - 使用GitHub登录"
echo "   - 创建新项目并选择此仓库"
echo "   - 上传 config.json 或设置 CONFIG_FILE 环境变量"
echo "   - 记录后端URL"
echo ""
echo -e "${GREEN}3. 部署到Vercel：${NC}"
echo "   - 访问 https://vercel.com"
echo "   - 使用GitHub登录"
echo "   - 创建新项目并选择此仓库"
echo "   - 设置 Root Directory 为 'web'"
echo "   - 设置 VITE_API_URL 为Railway后端URL"
echo "   - 部署"
echo ""
echo -e "${GREEN}4. 测试部署：${NC}"
echo "   - 访问前端URL"
echo "   - 访问后端URL/health"
echo ""

echo "=================================="
echo -e "${GREEN}  所有检查通过！可以开始部署 🚀${NC}"
echo "=================================="
echo ""
