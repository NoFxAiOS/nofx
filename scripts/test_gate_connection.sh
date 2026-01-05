#!/bin/bash

# Gate.io API 连接测试脚本
# 用于诊断网络连接问题

echo "=========================================="
echo "Gate.io API 连接诊断工具"
echo "=========================================="
echo ""

# 测试 1: DNS 解析
echo "📍 测试 1: DNS 解析"
echo "正在解析 api.gateio.ws..."
if host api.gateio.ws > /dev/null 2>&1; then
    echo "✅ DNS 解析成功"
    host api.gateio.ws | head -5
else
    echo "❌ DNS 解析失败"
    echo "建议: 检查 DNS 设置或使用代理"
fi
echo ""

# 测试 2: Ping 测试
echo "📍 测试 2: Ping 测试"
echo "正在 ping api.gateio.ws (4次)..."
if ping -c 4 api.gateio.ws > /dev/null 2>&1; then
    echo "✅ Ping 成功"
    ping -c 4 api.gateio.ws | tail -2
else
    echo "⚠️  Ping 失败 (可能被防火墙拦截，但不影响 HTTPS 连接)"
fi
echo ""

# 测试 3: TCP 连接
echo "📍 测试 3: TCP 443 端口连接"
echo "正在测试 HTTPS 连接..."
if timeout 10 bash -c "cat < /dev/null > /dev/tcp/api.gateio.ws/443" 2>/dev/null; then
    echo "✅ TCP 连接成功"
else
    echo "❌ TCP 连接失败"
    echo "建议: 使用代理或检查防火墙设置"
fi
echo ""

# 测试 4: HTTPS 请求
echo "📍 测试 4: HTTPS API 请求"
echo "正在测试 Gate.io API 端点..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 10 --max-time 30 https://api.gateio.ws/api/v4/futures/usdt/contracts 2>/dev/null)

if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ API 请求成功 (HTTP $HTTP_CODE)"
    echo "响应时间:"
    curl -s -o /dev/null -w "  连接: %{time_connect}s\n  总计: %{time_total}s\n" --connect-timeout 10 --max-time 30 https://api.gateio.ws/api/v4/futures/usdt/contracts
elif [ "$HTTP_CODE" = "000" ]; then
    echo "❌ API 请求失败 (连接超时或被拒绝)"
    echo "建议: 配置系统代理"
else
    echo "⚠️  API 请求返回 HTTP $HTTP_CODE"
fi
echo ""

# 测试 5: 代理检测
echo "📍 测试 5: 代理设置检测"
if [ -n "$HTTP_PROXY" ] || [ -n "$HTTPS_PROXY" ] || [ -n "$http_proxy" ] || [ -n "$https_proxy" ]; then
    echo "✅ 检测到系统代理设置:"
    [ -n "$HTTP_PROXY" ] && echo "  HTTP_PROXY=$HTTP_PROXY"
    [ -n "$HTTPS_PROXY" ] && echo "  HTTPS_PROXY=$HTTPS_PROXY"
    [ -n "$http_proxy" ] && echo "  http_proxy=$http_proxy"
    [ -n "$https_proxy" ] && echo "  https_proxy=$https_proxy"
else
    echo "⚠️  未检测到系统代理设置"
    echo "如果在中国大陆，建议配置代理"
fi
echo ""

# 诊断结果
echo "=========================================="
echo "📊 诊断结果"
echo "=========================================="

if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ Gate.io API 连接正常"
    echo ""
    echo "建议:"
    echo "  1. 如果应用仍然超时，请重启应用"
    echo "  2. 检查应用日志中的详细错误信息"
else
    echo "❌ Gate.io API 连接存在问题"
    echo ""
    echo "解决方案:"
    echo ""
    echo "方案 1: 配置系统代理（推荐）"
    echo "  export HTTP_PROXY=http://127.0.0.1:7890"
    echo "  export HTTPS_PROXY=http://127.0.0.1:7890"
    echo "  # 或在 ~/.bashrc 或 ~/.zshrc 中添加"
    echo ""
    echo "方案 2: 使用 Clash 或其他代理工具"
    echo "  1. 启动 Clash/V2Ray 等代理软件"
    echo "  2. 确保系统代理已配置"
    echo "  3. 重启终端和应用"
    echo ""
    echo "方案 3: 修改 DNS 设置"
    echo "  1. 使用 8.8.8.8 或 1.1.1.1"
    echo "  2. 或使用国内 DNS (114.114.114.114)"
    echo ""
    echo "方案 4: 临时使用 HTX 或币安（国内访问更稳定）"
    echo "  HTX: api.hbdm.com (国内服务器)"
    echo "  币安: api.binance.com (全球CDN)"
fi
echo ""

# HTX 测试（对比）
echo "=========================================="
echo "📍 对比测试: HTX API (国内服务器)"
echo "=========================================="
HTX_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 10 --max-time 30 https://api.hbdm.com/linear-swap-api/v1/swap_contract_info 2>/dev/null)

if [ "$HTX_CODE" = "200" ]; then
    echo "✅ HTX API 连接正常 (HTTP $HTX_CODE)"
    echo "响应时间:"
    curl -s -o /dev/null -w "  连接: %{time_connect}s\n  总计: %{time_total}s\n" --connect-timeout 10 --max-time 30 https://api.hbdm.com/linear-swap-api/v1/swap_contract_info
    echo ""
    echo "💡 如果 HTX 连接正常但 Gate.io 失败，说明是网络限制问题"
else
    echo "❌ HTX API 连接失败 (HTTP $HTX_CODE)"
    echo "💡 如果两个都失败，说明是整体网络问题"
fi
echo ""

echo "=========================================="
echo "测试完成"
echo "=========================================="
