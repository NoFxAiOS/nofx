#!/bin/bash

# Mars AI交易系统加密环境设置脚本
# 一键生成RSA密钥对和数据加密密钥，完整设置加密环境

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${PURPLE}╔════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${PURPLE}║                          Mars AI交易系统                              ║${NC}"
echo -e "${PURPLE}║                        🔐 加密环境一键设置工具                        ║${NC}"
echo -e "${PURPLE}║                                                                        ║${NC}"
echo -e "${PURPLE}║  功能: 生成RSA密钥对 + 数据加密密钥 + 配置环境变量                    ║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"
echo

# 检查依赖
echo -e "${CYAN}🔍 检查系统依赖...${NC}"

# 检查 OpenSSL
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}❌ 错误: 系统中未安装 OpenSSL${NC}"
    echo -e "请安装 OpenSSL:"
    echo -e "  macOS: ${YELLOW}brew install openssl${NC}"
    echo -e "  Ubuntu/Debian: ${YELLOW}sudo apt-get install openssl${NC}"
    echo -e "  CentOS/RHEL: ${YELLOW}sudo yum install openssl${NC}"
    exit 1
fi

echo -e "${GREEN}✓ OpenSSL: $(openssl version)${NC}"

# 进入项目根目录
cd "$PROJECT_ROOT"
echo -e "${GREEN}✓ 工作目录: $(pwd)${NC}"

# 配置参数
RSA_KEY_SIZE=2048
SECRETS_DIR="secrets"
PRIVATE_KEY_FILE="$SECRETS_DIR/rsa_key"
PUBLIC_KEY_FILE="$SECRETS_DIR/rsa_key.pub"

echo
echo -e "${BLUE}📋 配置参数:${NC}"
echo -e "  • RSA密钥大小: ${YELLOW}$RSA_KEY_SIZE bits${NC}"
echo -e "  • 私钥文件: ${YELLOW}$PRIVATE_KEY_FILE${NC}"
echo -e "  • 公钥文件: ${YELLOW}$PUBLIC_KEY_FILE${NC}"
echo -e "  • AES密钥: ${YELLOW}256 bits (自动生成)${NC}"

# 询问用户确认
echo
read -p "是否继续设置加密环境? [Y/n]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${BLUE}ℹ️  操作已取消${NC}"
    exit 0
fi

echo
echo -e "${CYAN}🚀 开始设置加密环境...${NC}"

# ============= 步骤1: 创建目录 =============
echo
echo -e "${YELLOW}📁 步骤 1/4: 创建必要目录...${NC}"

if [ ! -d "$SECRETS_DIR" ]; then
    mkdir -p "$SECRETS_DIR"
    chmod 700 "$SECRETS_DIR"
    echo -e "${GREEN}✓ 创建 $SECRETS_DIR 目录${NC}"
else
    echo -e "${GREEN}✓ $SECRETS_DIR 目录已存在${NC}"
fi

if [ ! -d "scripts" ]; then
    mkdir -p "scripts"
    echo -e "${GREEN}✓ 创建 scripts 目录${NC}"
else
    echo -e "${GREEN}✓ scripts 目录已存在${NC}"
fi

# ============= 步骤2: 生成RSA密钥对 =============
echo
echo -e "${YELLOW}🔐 步骤 2/4: 生成 RSA-$RSA_KEY_SIZE 密钥对...${NC}"

# 检查现有RSA密钥
if [ -f "$PRIVATE_KEY_FILE" ] || [ -f "$PUBLIC_KEY_FILE" ]; then
    echo -e "${YELLOW}⚠️  检测到现有的RSA密钥文件${NC}"
    read -p "是否重新生成RSA密钥? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f "$PRIVATE_KEY_FILE" "$PUBLIC_KEY_FILE"
        echo -e "${YELLOW}🗑️  删除旧密钥${NC}"
    else
        echo -e "${BLUE}ℹ️  保持现有RSA密钥${NC}"
        RSA_SKIPPED=true
    fi
fi

if [ "$RSA_SKIPPED" != "true" ]; then
    # 生成私钥
    echo -e "  ${CYAN}生成RSA私钥...${NC}"
    openssl genrsa -out "$PRIVATE_KEY_FILE" $RSA_KEY_SIZE 2>/dev/null
    chmod 600 "$PRIVATE_KEY_FILE"
    echo -e "${GREEN}  ✓ 私钥生成完成${NC}"
    
    # 生成公钥
    echo -e "  ${CYAN}提取RSA公钥...${NC}"
    openssl rsa -in "$PRIVATE_KEY_FILE" -pubout -out "$PUBLIC_KEY_FILE" 2>/dev/null
    chmod 644 "$PUBLIC_KEY_FILE"
    echo -e "${GREEN}  ✓ 公钥生成完成${NC}"
    
    # 验证密钥
    echo -e "  ${CYAN}验证密钥对...${NC}"
    openssl rsa -in "$PRIVATE_KEY_FILE" -check -noout 2>/dev/null
    echo -e "${GREEN}  ✓ 密钥验证通过${NC}"
fi

# ============= 步骤3: 生成数据加密密钥和JWT密钥 =============
echo
echo -e "${YELLOW}🔑 步骤 3/4: 生成 AES-256 数据加密密钥和JWT认证密钥...${NC}"

# 检查现有密钥
DATA_KEY_EXISTS=false
JWT_KEY_EXISTS=false

if [ -f ".env" ]; then
    if grep -q "^DATA_ENCRYPTION_KEY=" .env; then
        DATA_KEY_EXISTS=true
    fi
    if grep -q "^JWT_SECRET=" .env; then
        JWT_KEY_EXISTS=true
    fi
fi

if [ "$DATA_KEY_EXISTS" = "true" ] || [ "$JWT_KEY_EXISTS" = "true" ]; then
    echo -e "${YELLOW}⚠️  检测到现有的密钥配置${NC}"
    if [ "$DATA_KEY_EXISTS" = "true" ]; then
        echo -e "  • 数据加密密钥已存在"
    fi
    if [ "$JWT_KEY_EXISTS" = "true" ]; then
        echo -e "  • JWT认证密钥已存在"
    fi
    read -p "是否重新生成所有密钥? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}ℹ️  保持现有密钥${NC}"
        KEY_SKIPPED=true
        # 读取现有密钥
        if [ "$DATA_KEY_EXISTS" = "true" ]; then
            DATA_KEY=$(grep "^DATA_ENCRYPTION_KEY=" .env | cut -d'=' -f2)
        fi
        if [ "$JWT_KEY_EXISTS" = "true" ]; then
            JWT_KEY=$(grep "^JWT_SECRET=" .env | cut -d'=' -f2)
        fi
    fi
fi

if [ "$KEY_SKIPPED" != "true" ]; then
    # 生成新的密钥
    echo -e "  ${CYAN}生成AES-256数据加密密钥...${NC}"
    DATA_KEY=$(openssl rand -base64 32)
    echo -e "${GREEN}  ✓ 数据加密密钥生成完成${NC}"
    
    echo -e "  ${CYAN}生成JWT认证密钥...${NC}"
    JWT_KEY=$(openssl rand -base64 64)
    echo -e "${GREEN}  ✓ JWT认证密钥生成完成${NC}"
    
    # 保存到.env文件
    if [ -f ".env" ]; then
        # 更新现有文件
        if grep -q "^DATA_ENCRYPTION_KEY=" .env; then
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s/^DATA_ENCRYPTION_KEY=.*/DATA_ENCRYPTION_KEY=$DATA_KEY/" .env
            else
                sed -i "s/^DATA_ENCRYPTION_KEY=.*/DATA_ENCRYPTION_KEY=$DATA_KEY/" .env
            fi
        else
            echo "DATA_ENCRYPTION_KEY=$DATA_KEY" >> .env
        fi
        
        if grep -q "^JWT_SECRET=" .env; then
            # 使用替代分隔符避免 / 字符冲突，并用引号保护值
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=\"$JWT_KEY\"|" .env
            else
                sed -i "s|^JWT_SECRET=.*|JWT_SECRET=\"$JWT_KEY\"|" .env
            fi
        else
            # 使用引号确保值在同一行
            printf "JWT_SECRET=\"%s\"\n" "$JWT_KEY" >> .env
        fi
    else
        # 创建新文件
        echo "DATA_ENCRYPTION_KEY=$DATA_KEY" > .env
        printf "JWT_SECRET=\"%s\"\n" "$JWT_KEY" >> .env
    fi
    chmod 600 .env
    echo -e "${GREEN}  ✓ 密钥已保存到 .env 文件${NC}"
elif [ "$DATA_KEY_EXISTS" != "true" ] || [ "$JWT_KEY_EXISTS" != "true" ]; then
    # 生成缺失的密钥
    if [ "$DATA_KEY_EXISTS" != "true" ]; then
        echo -e "  ${CYAN}生成缺失的AES-256数据加密密钥...${NC}"
        DATA_KEY=$(openssl rand -base64 32)
        echo "DATA_ENCRYPTION_KEY=$DATA_KEY" >> .env
        echo -e "${GREEN}  ✓ 数据加密密钥生成完成${NC}"
    fi
    
    if [ "$JWT_KEY_EXISTS" != "true" ]; then
        echo -e "  ${CYAN}生成缺失的JWT认证密钥...${NC}"
        JWT_KEY=$(openssl rand -base64 64)
        printf "JWT_SECRET=\"%s\"\n" "$JWT_KEY" >> .env
        echo -e "${GREEN}  ✓ JWT认证密钥生成完成${NC}"
    fi
    
    chmod 600 .env
    echo -e "${GREEN}  ✓ 密钥已保存到 .env 文件${NC}"
fi

# ============= 步骤4: 验证和总结 =============
echo
echo -e "${YELLOW}✅ 步骤 4/4: 环境验证和总结...${NC}"

# 验证文件存在性和权限
echo -e "  ${CYAN}验证文件和权限...${NC}"

if [ -f "$PRIVATE_KEY_FILE" ]; then
    PRIVATE_PERM=$(stat -f "%A" "$PRIVATE_KEY_FILE" 2>/dev/null || stat -c "%a" "$PRIVATE_KEY_FILE" 2>/dev/null)
    echo -e "${GREEN}  ✓ 私钥文件: $PRIVATE_KEY_FILE (权限: $PRIVATE_PERM)${NC}"
else
    echo -e "${RED}  ❌ 私钥文件不存在${NC}"
    exit 1
fi

if [ -f "$PUBLIC_KEY_FILE" ]; then
    PUBLIC_PERM=$(stat -f "%A" "$PUBLIC_KEY_FILE" 2>/dev/null || stat -c "%a" "$PUBLIC_KEY_FILE" 2>/dev/null)
    echo -e "${GREEN}  ✓ 公钥文件: $PUBLIC_KEY_FILE (权限: $PUBLIC_PERM)${NC}"
else
    echo -e "${RED}  ❌ 公钥文件不存在${NC}"
    exit 1
fi

if [ -f ".env" ] && grep -q "^DATA_ENCRYPTION_KEY=" .env && grep -q "^JWT_SECRET=" .env; then
    ENV_PERM=$(stat -f "%A" ".env" 2>/dev/null || stat -c "%a" ".env" 2>/dev/null)
    echo -e "${GREEN}  ✓ 环境文件: .env (权限: $ENV_PERM)${NC}"
    echo -e "${GREEN}    包含: DATA_ENCRYPTION_KEY, JWT_SECRET${NC}"
else
    echo -e "${RED}  ❌ 环境文件不存在或缺少必要密钥${NC}"
    exit 1
fi

# 测试密钥功能
echo -e "  ${CYAN}测试密钥功能...${NC}"
TEST_DATA="Hello Mars AI Trading System"
ENCRYPTED=$(echo "$TEST_DATA" | openssl rsautl -encrypt -pubin -inkey "$PUBLIC_KEY_FILE" | base64)
DECRYPTED=$(echo "$ENCRYPTED" | base64 -d | openssl rsautl -decrypt -inkey "$PRIVATE_KEY_FILE")

if [ "$DECRYPTED" = "$TEST_DATA" ]; then
    echo -e "${GREEN}  ✓ RSA加密/解密测试通过${NC}"
else
    echo -e "${RED}  ❌ RSA加密/解密测试失败${NC}"
    exit 1
fi

# 显示最终结果
echo
echo -e "${GREEN}🎉 加密环境设置完成！${NC}"
echo
echo -e "${PURPLE}╔════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${PURPLE}║                            设置完成摘要                               ║${NC}"
echo -e "${PURPLE}╠════════════════════════════════════════════════════════════════════════╣${NC}"
echo -e "${PURPLE}║${NC} ${BLUE}RSA密钥对:${NC}                                                         ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   私钥: ${YELLOW}$PRIVATE_KEY_FILE${NC}                      ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   公钥: ${YELLOW}$PUBLIC_KEY_FILE${NC}                  ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   大小: ${YELLOW}$RSA_KEY_SIZE bits${NC}                                              ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}                                                                        ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC} ${BLUE}安全密钥配置:${NC}                                                     ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   文件: ${YELLOW}.env${NC}                                                      ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   数据加密: ${YELLOW}DATA_ENCRYPTION_KEY (AES-256-GCM)${NC}                   ${PURPLE}║${NC}"
echo -e "${PURPLE}║${NC}   JWT认证: ${YELLOW}JWT_SECRET (HS256)${NC}                                   ${PURPLE}║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"

# 使用指南
echo
echo -e "${BLUE}📋 使用指南:${NC}"
echo
echo -e "${YELLOW}1. 启动Mars AI交易系统:${NC}"
echo -e "   source .env && ./mars"
echo
echo -e "${YELLOW}2. Docker部署:${NC}"
echo -e "   docker run --env-file .env mars-ai-trading"
echo
echo -e "${YELLOW}3. 查看公钥内容:${NC}"
echo -e "   cat $PUBLIC_KEY_FILE"
echo
echo -e "${YELLOW}4. 测试加密API:${NC}"
echo -e "   curl http://localhost:8080/api/crypto/public-key"

# 安全提醒
echo
echo -e "${RED}🔒 安全提醒:${NC}"
echo -e "  • 私钥文件 ($PRIVATE_KEY_FILE) 权限已设置为 600"
echo -e "  • 环境文件 (.env) 权限已设置为 600"
echo -e "  • 请勿将私钥和数据密钥提交到版本控制系统"
echo -e "  • 建议在生产环境中使用密钥管理服务"
echo -e "  • 定期备份密钥文件"

echo
echo -e "${GREEN}✅ Mars AI交易系统加密环境设置完成！${NC}"