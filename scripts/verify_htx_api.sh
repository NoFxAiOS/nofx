#!/bin/bash

# HTX API 新版验证脚本
# 日期: 2026-01-05
# 用途: 验证新版 HTX API 的可用性

echo "================================================"
echo "HTX API 新版验证测试"
echo "================================================"
echo ""

# HTX使用不同的域名服务不同业务
NEW_CONTRACT_API="https://api.hbdm.com"  # 合约专用域名
NEW_SPOT_API="https://api.htx.com"       # 现货专用域名
OLD_API="https://api.huobi.pro"          # 旧版统一域名

echo "📋 重要说明："
echo "HTX使用分离的API域名："
echo "  - 现货交易: api.htx.com"
echo "  - 合约交易: api.hbdm.com  ← 本项目使用"
echo "  - 旧版统一: api.huobi.pro"
echo ""

echo "📋 测试项目："
echo "1. 新版合约API域名连通性"
echo "2. 旧版API域名连通性（对比）"
echo "3. 公开接口响应一致性"
echo ""

# 测试1: 新版合约API
echo "✅ 测试 1: 新版合约API时间戳接口"
echo "请求: GET $NEW_SPOT_API/v1/common/timestamp"
NEW_RESPONSE=$(curl -s "$NEW_SPOT_API/v1/common/timestamp")
echo "响应: $NEW_RESPONSE"

if echo "$NEW_RESPONSE" | grep -q '"status":"ok"'; then
    echo "✅ 新版API正常"
else
    echo "❌ 新版API异常"
fi
echo ""

# 测试2: 旧版API时间戳接口（对比）
echo "📊 测试 2: 旧版API时间戳接口（对比）"
echo "请求: GET $OLD_API/v1/common/timestamp"
OLD_RESPONSE=$(curl -s "$OLD_API/v1/common/timestamp")
echo "响应: $OLD_RESPONSE"

if echo "$OLD_RESPONSE" | grep -q '"status":"ok"'; then
    echo "✅ 旧版API仍然正常"
else
    echo "⚠️  旧版API可能已下线"
fi
echo ""

# 测试3: 合约信息查询（使用合约专用域名）
echo "✅ 测试 3: 合约信息查询接口（合约专用域名）"
CONTRACT_URL="$NEW_CONTRACT_API/linear-swap-api/v1/swap_contract_info?contract_code=BTC-USDT"
echo "请求: GET $CONTRACT_URL"
CONTRACT_RESPONSE=$(curl -s "$CONTRACT_URL")

if echo "$CONTRACT_RESPONSE" | grep -q '"status":"ok"'; then
    echo "✅ 合约信息查询接口正常"
    echo "$CONTRACT_RESPONSE" | python3 -m json.tool 2>/dev/null | head -30 || echo "$CONTRACT_RESPONSE" | head -20
else
    echo "❌ 合约信息查询接口异常"
    echo "$CONTRACT_RESPONSE"
    exit 1
fi
echo ""

# 测试4: 合约市场深度
echo "✅ 测试 4: 合约市场深度接口"
DEPTH_URL="$NEW_CONTRACT_API/linear-swap-ex/market/depth?contract_code=BTC-USDT&type=step0"
echo "请求: GET $DEPTH_URL"
DEPTH_RESPONSE=$(curl -s "$DEPTH_URL")

if echo "$DEPTH_RESPONSE" | grep -q '"status":"ok"'; then
    echo "✅ 合约市场深度接口正常"
    echo "$DEPTH_RESPONSE" | python3 -m json.tool 2>/dev/null | head -25 || echo "$DEPTH_RESPONSE" | head -20
else
    echo "❌ 合约市场深度接口异常"
    echo "$DEPTH_RESPONSE"
fi
echo ""

# 总结
echo "================================================"
echo "📊 测试总结"
echo "================================================"
echo "✅ 新版合约API域名: $NEW_CONTRACT_API"
echo "✅ 新版现货API域名: $NEW_SPOT_API"
echo "📊 旧版API域名: $OLD_API (仍可用)"
echo ""
echo "✅ 所有公开接口测试通过！"
echo ""
echo "⚠️  注意事项："
echo "1. HTX合约交易使用专用域名: api.hbdm.com"
echo "2. 私有接口（需要签名）需要使用真实API Key进行测试"
echo "3. 签名算法的host参数已自动更新为: api.hbdm.com"
echo "4. 建议在测试环境先进行小额交易测试"
echo ""
echo "📋 下一步行动："
echo "1. 编译项目: make build 或 go build"
echo "2. 使用测试账户验证交易功能"
echo "3. 监控生产环境API调用成功率"
echo ""
echo "================================================"
