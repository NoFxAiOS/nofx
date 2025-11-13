#!/bin/bash

# RSA密钥对生成脚本 - 用于Mars AI交易系统加密服务
# 生成用于混合加密的RSA-2048密钥对

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
RSA_KEY_SIZE=2048
SECRETS_DIR="secrets"
PRIVATE_KEY_FILE="$SECRETS_DIR/rsa_key"
PUBLIC_KEY_FILE="$SECRETS_DIR/rsa_key.pub"

echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                   Mars AI交易系统 RSA密钥生成器                 ║${NC}"
echo -e "${BLUE}║                     RSA-2048 混合加密密钥对                     ║${NC}"
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

# 创建 secrets 目录
if [ ! -d "$SECRETS_DIR" ]; then
    echo -e "${YELLOW}📁 创建 $SECRETS_DIR 目录...${NC}"
    mkdir -p "$SECRETS_DIR"
    chmod 700 "$SECRETS_DIR"
    echo -e "${GREEN}✓ 目录创建成功${NC}"
else
    echo -e "${GREEN}✓ $SECRETS_DIR 目录已存在${NC}"
fi

# 检查现有密钥
if [ -f "$PRIVATE_KEY_FILE" ] || [ -f "$PUBLIC_KEY_FILE" ]; then
    echo
    echo -e "${YELLOW}⚠️  检测到现有的RSA密钥文件:${NC}"
    [ -f "$PRIVATE_KEY_FILE" ] && echo -e "  • $PRIVATE_KEY_FILE"
    [ -f "$PUBLIC_KEY_FILE" ] && echo -e "  • $PUBLIC_KEY_FILE"
    echo
    read -p "是否覆盖现有密钥? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}ℹ️  操作已取消${NC}"
        exit 0
    fi
    echo -e "${YELLOW}🗑️  删除现有密钥文件...${NC}"
    rm -f "$PRIVATE_KEY_FILE" "$PUBLIC_KEY_FILE"
fi

echo
echo -e "${BLUE}🔐 开始生成 RSA-$RSA_KEY_SIZE 密钥对...${NC}"

# 生成私钥
echo -e "${YELLOW}📝 步骤 1/3: 生成 RSA 私钥 ($RSA_KEY_SIZE bits)...${NC}"
if openssl genrsa -out "$PRIVATE_KEY_FILE" $RSA_KEY_SIZE 2>/dev/null; then
    echo -e "${GREEN}✓ 私钥生成成功${NC}"
else
    echo -e "${RED}❌ 私钥生成失败${NC}"
    exit 1
fi

# 设置私钥权限
chmod 600 "$PRIVATE_KEY_FILE"
echo -e "${GREEN}✓ 私钥权限设置为 600${NC}"

# 生成公钥
echo -e "${YELLOW}📝 步骤 2/3: 从私钥提取公钥...${NC}"
if openssl rsa -in "$PRIVATE_KEY_FILE" -pubout -out "$PUBLIC_KEY_FILE" 2>/dev/null; then
    echo -e "${GREEN}✓ 公钥生成成功${NC}"
else
    echo -e "${RED}❌ 公钥生成失败${NC}"
    exit 1
fi

# 设置公钥权限
chmod 644 "$PUBLIC_KEY_FILE"
echo -e "${GREEN}✓ 公钥权限设置为 644${NC}"

# 验证密钥
echo -e "${YELLOW}📝 步骤 3/3: 验证密钥对...${NC}"
if openssl rsa -in "$PRIVATE_KEY_FILE" -check -noout 2>/dev/null; then
    echo -e "${GREEN}✓ 私钥验证通过${NC}"
else
    echo -e "${RED}❌ 私钥验证失败${NC}"
    exit 1
fi

if openssl rsa -in "$PUBLIC_KEY_FILE" -pubin -text -noout &>/dev/null; then
    echo -e "${GREEN}✓ 公钥验证通过${NC}"
else
    echo -e "${RED}❌ 公钥验证失败${NC}"
    exit 1
fi

# 显示密钥信息
echo
echo -e "${GREEN}🎉 RSA密钥对生成成功!${NC}"
echo
echo -e "${BLUE}📋 密钥信息:${NC}"
echo -e "  私钥文件: ${YELLOW}$PRIVATE_KEY_FILE${NC}"
echo -e "  公钥文件: ${YELLOW}$PUBLIC_KEY_FILE${NC}"
echo -e "  密钥大小: ${YELLOW}$RSA_KEY_SIZE bits${NC}"
echo

# 显示文件大小
PRIVATE_SIZE=$(stat -f%z "$PRIVATE_KEY_FILE" 2>/dev/null || stat -c%s "$PRIVATE_KEY_FILE" 2>/dev/null || echo "未知")
PUBLIC_SIZE=$(stat -f%z "$PUBLIC_KEY_FILE" 2>/dev/null || stat -c%s "$PUBLIC_KEY_FILE" 2>/dev/null || echo "未知")

echo -e "${BLUE}📏 文件大小:${NC}"
echo -e "  私钥: ${YELLOW}$PRIVATE_SIZE bytes${NC}"
echo -e "  公钥: ${YELLOW}$PUBLIC_SIZE bytes${NC}"

# 显示公钥内容预览
echo
echo -e "${BLUE}🔍 公钥内容预览:${NC}"
head -n 5 "$PUBLIC_KEY_FILE" | sed 's/^/  /'
echo -e "  ${YELLOW}...${NC}"
tail -n 2 "$PUBLIC_KEY_FILE" | sed 's/^/  /'

echo
echo -e "${GREEN}✅ RSA密钥对生成完成!${NC}"
echo
echo -e "${YELLOW}📋 使用说明:${NC}"
echo -e "  1. 私钥文件 ($PRIVATE_KEY_FILE) 用于服务器端解密"
echo -e "  2. 公钥文件 ($PUBLIC_KEY_FILE) 可以分发给客户端用于加密"
echo -e "  3. 确保私钥文件的安全性，不要泄露给第三方"
echo -e "  4. 在生产环境中，建议将私钥存储在安全的密钥管理服务中"
echo
echo -e "${RED}⚠️  安全提醒:${NC}"
echo -e "  • 私钥文件权限已设置为 600 (仅所有者可读写)"
echo -e "  • 请定期备份密钥文件"
echo -e "  • 建议在不同环境使用不同的密钥对"
echo