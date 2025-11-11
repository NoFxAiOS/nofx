#!/bin/bash

# 🔍 NOFX Binance 连接诊断工具
# 用于检测 Binance Testnet 连接问题

echo "🔍 NOFX Binance 连接诊断工具"
echo "====================================="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检测函数
check_dns() {
    echo "📡 检查 DNS 解析..."
    if ping -c 1 fstream.binance.com &> /dev/null; then
        echo -e "${GREEN}✅ fstream.binance.com DNS 解析成功${NC}"
    else
        echo -e "${RED}❌ fstream.binance.com DNS 解析失败${NC}"
        echo -e "${YELLOW}💡 建议：检查网络连接或使用代理${NC}"
    fi
    echo ""
}

check_testnet_api() {
    echo "🌐 检查 Testnet API 连接..."
    response=$(curl -s -o /dev/null -w "%{http_code}" "https://testnet.binance.vision/fapi/v1/time")
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✅ Testnet API 连接成功${NC}"
        
        # 获取服务器时间
        server_time=$(curl -s "https://testnet.binance.vision/fapi/v1/time" | grep -o '"serverTime":[0-9]*' | cut -d':' -f2)
        local_time=$(date +%s)000
        time_diff=$((server_time - local_time))
        
        echo "  服务器时间: $(date -r $((server_time / 1000)) '+%Y-%m-%d %H:%M:%S')"
        echo "  本地时间: $(date '+%Y-%m-%d %H:%M:%S')"
        echo "  时间差: ${time_diff}ms"
        
        if [ ${time_diff#-} -gt 5000 ]; then
            echo -e "${YELLOW}⚠️ 时间差过大，可能导致 API 调用失败${NC}"
            echo -e "${YELLOW}💡 建议：同步系统时间${NC}"
        fi
    else
        echo -e "${RED}❌ Testnet API 连接失败 (HTTP $response)${NC}"
        echo -e "${YELLOW}💡 建议：检查网络连接或使用代理${NC}"
    fi
    echo ""
}

check_websocket() {
    echo "🔌 检查 WebSocket 连接..."
    if command -v wscat &> /dev/null; then
        timeout 3 wscat -c "wss://fstream.binance.com/stream" &> /dev/null
        if [ $? -eq 0 ] || [ $? -eq 124 ]; then
            echo -e "${GREEN}✅ WebSocket 连接成功${NC}"
        else
            echo -e "${RED}❌ WebSocket 连接失败${NC}"
            echo -e "${YELLOW}💡 建议：检查网络连接或使用代理${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️ 未安装 wscat，跳过 WebSocket 测试${NC}"
        echo -e "${YELLOW}💡 安装方法: npm install -g wscat${NC}"
    fi
    echo ""
}

check_proxy() {
    echo "🌍 检查代理设置..."
    if [ -n "$https_proxy" ] || [ -n "$HTTPS_PROXY" ]; then
        echo -e "${GREEN}✅ 已配置 HTTPS 代理: ${https_proxy:-$HTTPS_PROXY}${NC}"
    else
        echo -e "${YELLOW}⚠️ 未配置代理${NC}"
        echo -e "${YELLOW}💡 如果在中国大陆，建议配置代理：${NC}"
        echo "   export https_proxy=http://127.0.0.1:7890"
        echo "   export http_proxy=http://127.0.0.1:7890"
    fi
    echo ""
}

check_config() {
    echo "⚙️ 检查配置文件..."
    if [ -f "config.json" ]; then
        echo -e "${GREEN}✅ config.json 存在${NC}"
        
        # 检查是否配置了 API Key
        if grep -q '"api_key"' config.json 2>/dev/null; then
            echo -e "${GREEN}✅ 已配置 API Key${NC}"
        else
            echo -e "${YELLOW}⚠️ 未配置 API Key${NC}"
            echo -e "${YELLOW}💡 请在 Web 界面配置交易所 API${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️ config.json 不存在${NC}"
        echo -e "${YELLOW}💡 首次运行时会自动创建${NC}"
    fi
    echo ""
}

check_database() {
    echo "💾 检查数据库..."
    if [ -f "config.db" ]; then
        echo -e "${GREEN}✅ config.db 存在${NC}"
        
        # 检查数据库大小
        db_size=$(du -h config.db | cut -f1)
        echo "  数据库大小: $db_size"
    else
        echo -e "${YELLOW}⚠️ config.db 不存在${NC}"
        echo -e "${YELLOW}💡 首次运行时会自动创建${NC}"
    fi
    echo ""
}

print_summary() {
    echo "====================================="
    echo "📋 诊断总结"
    echo "====================================="
    echo ""
    echo "常见问题解决方案："
    echo ""
    echo "1️⃣ 账户未激活："
    echo "   访问 https://testnet.binance.vision/"
    echo "   使用 GitHub 登录并生成 API Key"
    echo ""
    echo "2️⃣ 网络连接问题："
    echo "   配置代理（如果在中国大陆）"
    echo "   export https_proxy=http://127.0.0.1:7890"
    echo ""
    echo "3️⃣ 时间同步问题："
    echo "   sudo sntp -sS time.apple.com  # macOS"
    echo "   sudo ntpdate -s time.nist.gov # Linux"
    echo ""
    echo "4️⃣ 查看详细文档："
    echo "   docs/BINANCE_TESTNET_SETUP.md"
    echo ""
}

# 执行所有检查
check_dns
check_testnet_api
check_websocket
check_proxy
check_config
check_database
print_summary

echo "✨ 诊断完成！"
echo ""
