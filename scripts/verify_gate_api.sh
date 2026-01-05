#!/bin/bash

# Gate.io API 验证脚本
# 测试Gate.io合约API的可用性和正确性

echo "========================================="
echo "Gate.io API 验证测试"
echo "========================================="
echo ""

# 基础信息
echo "API版本: v4.106.9 (2025年最新版本)"
echo "API域名: https://api.gateio.ws"
echo "合约路径: /api/v4/futures/usdt"
echo ""

# 测试计数器
total_tests=0
passed_tests=0

# 测试1: 查询合约列表
echo "测试 1: 查询合约列表"
echo "接口: GET /api/v4/futures/usdt/contracts"
total_tests=$((total_tests+1))
response=$(curl -s "https://api.gateio.ws/api/v4/futures/usdt/contracts")
if echo "$response" | jq -e '.[0].name' >/dev/null 2>&1; then
    contract_name=$(echo "$response" | jq -r '.[0].name')
    echo "✓ 通过 - 成功获取合约列表，首个合约: $contract_name"
    passed_tests=$((passed_tests+1))
else
    echo "✗ 失败 - 无法获取合约列表"
    echo "响应: $response"
fi
echo ""

# 测试2: 查询单个合约信息
echo "测试 2: 查询BTC_USDT合约详情"
echo "接口: GET /api/v4/futures/usdt/contracts/BTC_USDT"
total_tests=$((total_tests+1))
response=$(curl -s "https://api.gateio.ws/api/v4/futures/usdt/contracts/BTC_USDT")
if echo "$response" | jq -e '.name' >/dev/null 2>&1; then
    mark_price=$(echo "$response" | jq -r '.mark_price')
    leverage_max=$(echo "$response" | jq -r '.leverage_max')
    echo "✓ 通过 - 合约信息: 标记价格=$mark_price, 最大杠杆=${leverage_max}x"
    passed_tests=$((passed_tests+1))
else
    echo "✗ 失败 - 无法获取合约详情"
    echo "响应: $response"
fi
echo ""

# 测试3: 查询市场深度
echo "测试 3: 查询BTC_USDT市场深度"
echo "接口: GET /api/v4/futures/usdt/order_book?contract=BTC_USDT"
total_tests=$((total_tests+1))
response=$(curl -s "https://api.gateio.ws/api/v4/futures/usdt/order_book?contract=BTC_USDT&limit=5")
if echo "$response" | jq -e '.asks[0]' >/dev/null 2>&1; then
    ask_price=$(echo "$response" | jq -r '.asks[0].p')
    bid_price=$(echo "$response" | jq -r '.bids[0].p')
    echo "✓ 通过 - 深度信息: 卖一价=$ask_price, 买一价=$bid_price"
    passed_tests=$((passed_tests+1))
else
    echo "✗ 失败 - 无法获取市场深度"
    echo "响应: $response"
fi
echo ""

# 测试4: 查询交易对Ticker
echo "测试 4: 查询BTC_USDT Ticker"
echo "接口: GET /api/v4/futures/usdt/tickers?contract=BTC_USDT"
total_tests=$((total_tests+1))
response=$(curl -s "https://api.gateio.ws/api/v4/futures/usdt/tickers?contract=BTC_USDT")
if echo "$response" | jq -e '.[0].last' >/dev/null 2>&1; then
    last_price=$(echo "$response" | jq -r '.[0].last')
    volume_24h=$(echo "$response" | jq -r '.[0].volume_24h')
    echo "✓ 通过 - Ticker信息: 最新价=$last_price, 24h成交量=$volume_24h"
    passed_tests=$((passed_tests+1))
else
    echo "✗ 失败 - 无法获取Ticker信息"
    echo "响应: $response"
fi
echo ""

# 测试5: 对比域名（www.gate.io vs api.gateio.ws）
echo "测试 5: 域名验证"
echo "测试域名: https://www.gate.io/api/v4/futures/usdt/contracts/BTC_USDT"
total_tests=$((total_tests+1))
response=$(curl -s "https://www.gate.io/api/v4/futures/usdt/contracts/BTC_USDT")
# www.gate.io不提供API服务，应该返回错误或重定向
if echo "$response" | jq -e '.name' >/dev/null 2>&1; then
    echo "⚠ 警告 - www.gate.io域名可用（不推荐使用）"
else
    echo "✓ 通过 - www.gate.io不提供API服务，应使用api.gateio.ws"
    passed_tests=$((passed_tests+1))
fi
echo ""

# 测试6: API文档验证
echo "测试 6: 官方文档可达性"
echo "文档地址: https://www.gate.com/docs/developers/apiv4/zh_CN/"
total_tests=$((total_tests+1))
response=$(curl -s -o /dev/null -w "%{http_code}" "https://www.gate.com/docs/developers/apiv4/zh_CN/")
if [ "$response" = "200" ]; then
    echo "✓ 通过 - 官方文档可访问 (HTTP $response)"
    passed_tests=$((passed_tests+1))
else
    echo "⚠ 警告 - 官方文档返回HTTP $response"
fi
echo ""

# 总结
echo "========================================="
echo "测试总结"
echo "========================================="
echo "总测试数: $total_tests"
echo "通过测试: $passed_tests"
echo "失败测试: $((total_tests - passed_tests))"
if [ $passed_tests -eq $total_tests ]; then
    echo ""
    echo "✓ 所有测试通过! Gate.io API可用且符合最新规范"
else
    echo ""
    echo "⚠ 部分测试失败，请检查具体错误"
fi
echo "========================================="
