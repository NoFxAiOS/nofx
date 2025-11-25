#!/bin/bash

echo "================================"
echo "AI模型配置修复验证测试"
echo "================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查后端进程
echo "1. 检查后端服务状态..."
if pgrep -f "./nofx" > /dev/null; then
    echo -e "${GREEN}✓ 后端服务正在运行${NC}"
else
    echo -e "${YELLOW}⚠ 后端服务未运行，正在启动...${NC}"
    cd /Users/guoyingcheng/dreame/code/nofx
    nohup ./nofx > /dev/null 2>&1 &
    sleep 3
    if pgrep -f "./nofx" > /dev/null; then
        echo -e "${GREEN}✓ 后端服务已启动${NC}"
    else
        echo -e "${RED}✗ 后端服务启动失败${NC}"
        exit 1
    fi
fi

echo ""
echo "2. 登录获取token..."
TOKEN=$(curl -s -X POST http://localhost:8080/api/login \
    -H "Content-Type: application/json" \
    -d '{"email":"test@test.com","password":"password123"}' | \
    grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}⚠ 无法获取token，可能是测试用户不存在，尝试创建用户...${NC}"
    curl -s -X POST http://localhost:8080/api/register \
        -H "Content-Type: application/json" \
        -d '{"email":"test@test.com","password":"password123"}' > /dev/null
    sleep 1
    TOKEN=$(curl -s -X POST http://localhost:8080/api/login \
        -H "Content-Type: application/json" \
        -d '{"email":"test@test.com","password":"password123"}' | \
        grep -o '"token":"[^"]*"' | cut -d'"' -f4)
fi

if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ 无法获取认证token${NC}"
    exit 1
else
    echo -e "${GREEN}✓ 获取token成功${NC}"
fi

echo ""
echo "3. 测试保存AI模型配置..."

# 测试数据
MODEL_DATA='{
    "models": {
        "deepseek": {
            "enabled": true,
            "api_key": "sk-test123456789",
            "custom_api_url": "https://api.deepseek.com/v1",
            "custom_model_name": "deepseek-chat"
        },
        "qwen": {
            "enabled": true,
            "api_key": "sk-qwen987654321",
            "custom_api_url": "",
            "custom_model_name": "qwen-plus"
        }
    }
}'

# 发送PUT请求
RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT http://localhost:8080/api/models \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$MODEL_DATA")

HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS/d')

echo "HTTP状态码: $HTTP_STATUS"
echo "响应内容: $BODY"

if [ "$HTTP_STATUS" = "200" ]; then
    echo -e "${GREEN}✓ 保存AI模型配置成功！${NC}"
else
    echo -e "${RED}✗ 保存AI模型配置失败，状态码: $HTTP_STATUS${NC}"
    echo "错误响应: $BODY"
    exit 1
fi

echo ""
echo "4. 验证数据库中的配置..."

# 检查数据库
DB_PATH="/Users/guoyingcheng/dreame/code/nofx/config.db"

if [ -f "$DB_PATH" ]; then
    echo "数据库中的AI模型配置:"
    sqlite3 "$DB_PATH" "SELECT id, name, provider, enabled, api_key FROM ai_models WHERE user_id = 'test' LIMIT 5;" 2>/dev/null || \
    sqlite3 "$DB_PATH" "SELECT id, name, provider, enabled, api_key FROM ai_models WHERE user_id = 'default' LIMIT 5;" 2>/dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 数据库查询成功${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 数据库文件不存在${NC}"
fi

echo ""
echo "5. 测试获取AI模型配置..."

RESPONSE2=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET http://localhost:8080/api/models \
    -H "Authorization: Bearer $TOKEN")

HTTP_STATUS2=$(echo "$RESPONSE2" | grep "HTTP_STATUS" | cut -d: -f2)

if [ "$HTTP_STATUS2" = "200" ]; then
    echo -e "${GREEN}✓ 获取AI模型配置成功！${NC}"
    echo "配置数量: $(echo "$RESPONSE2" | grep -o '"id"' | wc -l) 个"
else
    echo -e "${RED}✗ 获取AI模型配置失败，状态码: $HTTP_STATUS2${NC}"
fi

echo ""
echo "================================"
echo -e "${GREEN}测试完成！${NC}"
echo "================================"
echo ""
echo "修复总结:"
echo "  问题: UpdateAIModel函数中INSERT语句手动指定时间戳字段"
echo "  解决: 移除created_at和updated_at字段，让数据库自动管理"
echo "  原理: 遵循数据库设计最佳实践，避免触发器冲突"
echo ""
