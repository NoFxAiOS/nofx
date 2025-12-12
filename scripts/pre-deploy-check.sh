#!/bin/bash

# 🚀 邮件系统部署检查清单 - 快速参考
# 用途: 部署前的 5 分钟快速验证
# 时间: 2025-12-12

echo "🚀 邮件系统部署前检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

ISSUES=0

# 1️⃣ 检查 Go 编译
echo "✓ 检查 Go 编译..."
if go build -o /tmp/test-build 2>&1 | grep -q "error"; then
    echo "  ❌ 编译失败"
    ISSUES=$((ISSUES+1))
else
    echo "  ✅ 编译成功"
    rm -f /tmp/test-build
fi

# 2️⃣ 检查环境变量
echo ""
echo "✓ 检查必要的环境变量..."
if [ -z "$RESEND_API_KEY" ]; then
    echo "  ⚠️  RESEND_API_KEY 未设置"
    echo "     export RESEND_API_KEY='re_xxxxx'"
else
    echo "  ✅ RESEND_API_KEY 已设置"
fi

# 3️⃣ 检查关键函数
echo ""
echo "✓ 检查关键函数是否存在..."

grep -q "SendPasswordResetEmailWithRetry" email/email.go && echo "  ✅ SendPasswordResetEmailWithRetry" || (echo "  ❌ SendPasswordResetEmailWithRetry 未找到"; ISSUES=$((ISSUES+1)))
grep -q "SendEmailWithRetry" email/email.go && echo "  ✅ SendEmailWithRetry" || (echo "  ❌ SendEmailWithRetry 未找到"; ISSUES=$((ISSUES+1)))
grep -q "handleEmailHealthCheck" api/server.go && echo "  ✅ handleEmailHealthCheck" || (echo "  ❌ handleEmailHealthCheck 未找到"; ISSUES=$((ISSUES+1)))
grep -q "/health/email" api/server.go && echo "  ✅ /health/email 路由" || (echo "  ❌ /health/email 路由未找到"; ISSUES=$((ISSUES+1)))

# 4️⃣ 验证日志标记
echo ""
echo "✓ 检查日志标记..."

grep -q "PASSWORD_RESET_FAILED" api/server.go && echo "  ✅ PASSWORD_RESET_FAILED 标记" || echo "  ⚠️  缺少标记"
grep -q "EMAIL_RETRY" email/email.go && echo "  ✅ EMAIL_RETRY 标记" || echo "  ⚠️  缺少标记"
grep -q "EMAIL_HEALTH_CHECK" api/server.go && echo "  ✅ EMAIL_HEALTH_CHECK 标记" || echo "  ⚠️  缺少标记"

# 5️⃣ 检查诊断脚本
echo ""
echo "✓ 检查诊断脚本..."
if [ -f "scripts/email-diagnostics.sh" ]; then
    echo "  ✅ email-diagnostics.sh 存在"
    chmod +x scripts/email-diagnostics.sh
else
    echo "  ❌ email-diagnostics.sh 未找到"
    ISSUES=$((ISSUES+1))
fi

# 总结
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $ISSUES -eq 0 ]; then
    echo "✅ 所有检查通过！"
    echo ""
    echo "📋 部署步骤:"
    echo "  1. 确保环境变量已设置: export RESEND_API_KEY='...'"
    echo "  2. 启动应用: ./app"
    echo "  3. 测试健康检查: curl http://localhost:8080/api/health/email"
    echo "  4. 运行诊断: bash scripts/email-diagnostics.sh"
    exit 0
else
    echo "⚠️  发现 $ISSUES 个问题，请修复后再部署"
    exit 1
fi
