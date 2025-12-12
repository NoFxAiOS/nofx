#!/bin/bash

# 邮件系统快速诊断脚本
# 用途: 5分钟内快速诊断邮件系统故障
# 作者: System
# 日期: 2025-12-12

set -e

echo "🔍 [邮件系统诊断] 开始检查..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 1️⃣ 检查环境变量
echo ""
echo "📋 步骤1: 检查环境变量配置"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -z "$RESEND_API_KEY" ]; then
    echo "❌ RESEND_API_KEY: 未配置"
    echo "   💡 修复: export RESEND_API_KEY='your-key'"
else
    API_KEY_SHORT="${RESEND_API_KEY:0:10}...${RESEND_API_KEY: -5}"
    echo "✅ RESEND_API_KEY: 已配置 ($API_KEY_SHORT)"
fi

if [ -z "$RESEND_FROM_EMAIL" ]; then
    echo "⚠️  RESEND_FROM_EMAIL: 未配置，使用默认值"
else
    echo "✅ RESEND_FROM_EMAIL: $RESEND_FROM_EMAIL"
fi

if [ -z "$FRONTEND_URL" ]; then
    echo "⚠️  FRONTEND_URL: 未配置，使用默认值"
else
    echo "✅ FRONTEND_URL: $FRONTEND_URL"
fi

# 2️⃣ 测试健康检查端点
echo ""
echo "🏥 步骤2: 测试邮件健康检查端点"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

HEALTH_CHECK_URL="${API_URL:-http://localhost:8080}/api/health/email"
echo "检查端点: $HEALTH_CHECK_URL"

if command -v curl &> /dev/null; then
    HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "$HEALTH_CHECK_URL")
    HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -n1)
    BODY=$(echo "$HEALTH_RESPONSE" | head -n-1)

    if [ "$HTTP_CODE" == "200" ]; then
        echo "✅ 健康检查通过 (HTTP $HTTP_CODE)"
        echo "   响应: $BODY"
    else
        echo "❌ 健康检查失败 (HTTP $HTTP_CODE)"
        echo "   响应: $BODY"
    fi
else
    echo "⚠️  curl 未安装，跳过网络测试"
fi

# 3️⃣ 测试Resend API
echo ""
echo "🚀 步骤3: 测试Resend API连接"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -z "$RESEND_API_KEY" ]; then
    echo "⚠️  跳过API测试（API Key未配置）"
else
    if command -v curl &> /dev/null; then
        API_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST https://api.resend.com/emails \
            -H "Authorization: Bearer $RESEND_API_KEY" \
            -H "Content-Type: application/json" \
            -d '{
                "from":"test@yourdomain.com",
                "to":"test@example.com",
                "subject":"Test",
                "html":"<p>Test message</p>"
            }')

        HTTP_CODE=$(echo "$API_RESPONSE" | tail -n1)
        BODY=$(echo "$API_RESPONSE" | head -n-1)

        if [ "$HTTP_CODE" == "200" ] || [ "$HTTP_CODE" == "422" ]; then
            echo "✅ Resend API 连接正常 (HTTP $HTTP_CODE)"
        else
            echo "❌ Resend API 连接失败 (HTTP $HTTP_CODE)"
            echo "   响应: $BODY"
        fi
    else
        echo "⚠️  curl 未安装，无法测试API"
    fi
fi

# 4️⃣ 查看最近的错误日志
echo ""
echo "📖 步骤4: 查看最近的邮件错误日志"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -f "/var/log/app.log" ]; then
    echo "最近的邮件相关日志："
    grep -E "PASSWORD_RESET|EMAIL|email|邮件|Resend" /var/log/app.log | tail -20 || echo "  (无相关日志)"
elif [ -f "logs/app.log" ]; then
    echo "最近的邮件相关日志："
    grep -E "PASSWORD_RESET|EMAIL|email|邮件|Resend" logs/app.log | tail -20 || echo "  (无相关日志)"
else
    echo "⚠️  未找到日志文件 (/var/log/app.log 或 logs/app.log)"
fi

# 5️⃣ 输出诊断总结
echo ""
echo "📊 诊断总结"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -z "$RESEND_API_KEY" ]; then
    echo "🔴 问题严重等级: 严重"
    echo "   根本原因: RESEND_API_KEY 未配置"
    echo ""
    echo "立即修复步骤:"
    echo "  1. 获取 Resend API Key: https://resend.com/api-keys"
    echo "  2. 导出环境变量: export RESEND_API_KEY='re_xxx'"
    echo "  3. 重新启动应用"
else
    echo "🟡 问题分析: 配置已就绪，检查以下几点:"
    echo "  1. 验证 RESEND_FROM_EMAIL 在 Resend 中是否已验证"
    echo "  2. 检查 Resend 账户是否有充足的配额"
    echo "  3. 查看应用日志中是否有具体错误信息"
    echo "  4. 尝试手动发送测试邮件"
fi

echo ""
echo "✅ 诊断完成！"
echo ""
