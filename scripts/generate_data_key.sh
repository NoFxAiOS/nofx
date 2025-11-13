#!/bin/bash

# 数据加密密钥生成脚本 - 用于Mars AI交易系统数据库加密
# 生成用于AES-256-GCM数据库加密的随机密钥

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                   Mars AI交易系统 安全密钥生成器                 ║${NC}"
echo -e "${BLUE}║                 AES-256-GCM数据密钥 + JWT认证密钥                ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
echo

# 检查是否安装了 OpenSSL
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}❌ 错误: 系统中未安装 OpenSSL${NC}"
    echo -e "请安装 OpenSSL:"
    echo -e "  macOS: ${YELLOW}brew install openssl${NC}"
    echo -e "  Ubuntu/Debian: ${YELLOW}sudo apt-get install openssl${NC}"
    echo -e "  CentOS/RHEL: ${YELLOW}sudo yum install openssl${NC}"
    exit 1
fi

echo -e "${GREEN}✓ OpenSSL 已安装: $(openssl version)${NC}"

# 生成安全密钥
echo -e "${BLUE}🔐 生成安全密钥...${NC}"
echo

# 生成 AES-256 数据加密密钥
echo -e "${YELLOW}1/2: 生成 AES-256 数据加密密钥...${NC}"
DATA_KEY=$(openssl rand -base64 32)
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ 数据加密密钥生成成功${NC}"
else
    echo -e "${RED}  ❌ 数据加密密钥生成失败${NC}"
    exit 1
fi

# 生成 JWT 认证密钥
echo -e "${YELLOW}2/2: 生成 JWT 认证密钥...${NC}"
JWT_KEY=$(openssl rand -base64 64)
if [ $? -eq 0 ]; then
    echo -e "${GREEN}  ✓ JWT认证密钥生成成功${NC}"
else
    echo -e "${RED}  ❌ JWT认证密钥生成失败${NC}"
    exit 1
fi

# 显示密钥
echo
echo -e "${GREEN}🎉 安全密钥生成完成!${NC}"
echo
echo -e "${BLUE}📋 生成的密钥:${NC}"
echo -e "${PURPLE}1. 数据加密密钥 (AES-256):${NC}"
echo -e "${YELLOW}$DATA_KEY${NC}"
echo
echo -e "${PURPLE}2. JWT认证密钥 (512-bit):${NC}"
echo -e "${YELLOW}$JWT_KEY${NC}"
echo

# 显示使用方法
echo -e "${YELLOW}📋 使用方法:${NC}"
echo
echo -e "${BLUE}1. 环境变量设置:${NC}"
echo -e "   export DATA_ENCRYPTION_KEY=\"$DATA_KEY\""
echo -e "   export JWT_SECRET=\"$JWT_KEY\""
echo
echo -e "${BLUE}2. .env 文件设置:${NC}"
echo -e "   DATA_ENCRYPTION_KEY=$DATA_KEY"
echo -e "   JWT_SECRET=$JWT_KEY"
echo
echo -e "${BLUE}3. Docker环境设置:${NC}"
echo -e "   docker run -e DATA_ENCRYPTION_KEY=\"$DATA_KEY\" -e JWT_SECRET=\"$JWT_KEY\" ..."
echo
echo -e "${BLUE}4. Kubernetes Secret:${NC}"
echo -e "   kubectl create secret generic mars-crypto-key \\"
echo -e "     --from-literal=DATA_ENCRYPTION_KEY=\"$DATA_KEY\" \\"
echo -e "     --from-literal=JWT_SECRET=\"$JWT_KEY\""
echo

# 显示密钥特性
echo -e "${BLUE}🔍 密钥特性:${NC}"
echo -e "  • 数据加密: ${YELLOW}AES-256-GCM (256 bits)${NC}"
echo -e "  • JWT认证: ${YELLOW}HS256 (512 bits)${NC}"
echo -e "  • 格式: ${YELLOW}Base64 编码${NC}"
echo -e "  • 用途: ${YELLOW}数据库加密 + 用户认证${NC}"

# 安全提醒
echo
echo -e "${RED}⚠️  安全提醒:${NC}"
echo -e "  • 请妥善保管此密钥，丢失后无法恢复加密的数据"
echo -e "  • 不要将密钥提交到版本控制系统"
echo -e "  • 建议在不同环境使用不同的密钥"
echo -e "  • 定期更换密钥并重新加密数据"
echo -e "  • 在生产环境中，建议使用密钥管理服务"

echo
echo -e "${GREEN}✅ 数据加密密钥生成完成!${NC}"

# 可选：保存到 .env 文件
echo
read -p "是否将密钥保存到 .env 文件? [y/N]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [ -f ".env" ]; then
        # 检查是否已存在 DATA_ENCRYPTION_KEY
        if grep -q "^DATA_ENCRYPTION_KEY=" .env; then
            echo -e "${YELLOW}⚠️  .env 文件中已存在 DATA_ENCRYPTION_KEY${NC}"
            read -p "是否覆盖现有密钥? [y/N]: " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                # 替换现有密钥
                if [[ "$OSTYPE" == "darwin"* ]]; then
                    # macOS
                    sed -i '' "s/^DATA_ENCRYPTION_KEY=.*/DATA_ENCRYPTION_KEY=$RAW_KEY/" .env
                else
                    # Linux
                    sed -i "s/^DATA_ENCRYPTION_KEY=.*/DATA_ENCRYPTION_KEY=$RAW_KEY/" .env
                fi
                echo -e "${GREEN}✓ .env 文件中的密钥已更新${NC}"
            else
                echo -e "${BLUE}ℹ️  保持现有密钥不变${NC}"
            fi
        else
            # 追加新密钥
            echo "DATA_ENCRYPTION_KEY=$RAW_KEY" >> .env
            echo -e "${GREEN}✓ 密钥已保存到 .env 文件${NC}"
        fi
    else
        # 创建新的 .env 文件
        echo "DATA_ENCRYPTION_KEY=$RAW_KEY" > .env
        echo -e "${GREEN}✓ 密钥已保存到 .env 文件${NC}"
    fi
fi