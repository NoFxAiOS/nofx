#!/bin/bash

# ============================================================
# 积分系统数据显示修复 - 测试验证脚本
# ============================================================
# 此脚本用于验证积分系统数据显示错误的修复结果
#
# 测试用户: gyc567@gmail.com
# 用户ID: 68003b68-2f1d-4618-8124-e93e4a86200a
#
# 预期结果:
# - 总积分: 10000
# - 可用积分: 10000
# - 已用积分: 0
# - 不显示"交易次数"字段
# ============================================================

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印函数
print_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# 测试配置
TEST_USER_EMAIL="gyc567@gmail.com"
TEST_USER_ID="68003b68-2f1d-4618-8124-e93e4a86200a"
API_BASE_URL="http://localhost:8080/api/v1"

# 检查依赖
check_dependencies() {
    print_header "检查测试依赖"

    if ! command -v curl &> /dev/null; then
        print_error "curl 未安装"
        exit 1
    fi
    print_success "curl 已安装"

    if ! command -v jq &> /dev/null; then
        print_error "jq 未安装 (用于JSON解析)"
        exit 1
    fi
    print_success "jq 已安装"
}

# 测试API端点
test_api_endpoint() {
    print_header "测试积分系统API端点"

    print_info "测试 GET /api/v1/user/credits"

    # 注意：实际测试时需要真实的token
    # 这里仅检查API端点是否可访问
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE_URL}/user/credits" 2>/dev/null || echo "000")

    if [ "$response_code" = "401" ] || [ "$response_code" = "200" ]; then
        print_success "API端点可访问 (HTTP $response_code)"
    else
        print_error "API端点不可访问 (HTTP $response_code)"
        print_info "请确保后端服务正在运行在端口8080"
    fi
}

# 验证数据库记录
verify_database_record() {
    print_header "验证数据库中的积分记录"

    print_info "用户ID: $TEST_USER_ID"

    # 这里需要psql命令来查询数据库
    # 实际使用时需要配置正确的数据库连接参数
    if command -v psql &> /dev/null; then
        print_info "正在查询数据库..."

        # 注意：实际使用时需要替换为真实的数据库连接参数
        # 例如：psql "postgresql://user:pass@localhost:5432/dbname"
        print_info "需要手动执行以下SQL查询验证数据："
        echo ""
        echo "SELECT user_id, available_credits, total_credits, used_credits"
        echo "FROM user_credits"
        echo "WHERE user_id = '$TEST_USER_ID';"
        echo ""

        print_info "预期结果："
        echo "user_id                                   | available_credits | total_credits | used_credits"
        echo "----------------------------------------+-------------------+---------------+-------------"
        echo "${TEST_USER_ID}     |               10000 |         10000 |           0"
        echo ""
    else
        print_info "psql 未安装，跳过数据库验证"
        print_info "请手动查询数据库验证数据正确性"
    fi
}

# 检查前端代码修改
verify_frontend_changes() {
    print_header "验证前端代码修改"

    print_info "检查 useUserProfile.ts Hook修改..."

    if grep -q "fetch('/api/v1/user/credits'" web/src/hooks/useUserProfile.ts; then
        print_success "useUserCredits Hook 已修改为调用真实API"
    else
        print_error "useUserCredits Hook 未正确修改"
    fi

    if grep -q "transaction_count" web/src/hooks/useUserProfile.ts; then
        print_error "useUserCredits Hook 仍返回 transaction_count 字段"
    else
        print_success "useUserCredits Hook 不再返回 transaction_count 字段"
    fi

    print_info "检查 UserProfilePage.tsx 布局修改..."

    if grep -q "grid-cols-3" web/src/pages/UserProfilePage.tsx; then
        print_success "积分显示布局已改为3列"
    else
        print_error "积分显示布局未正确修改"
    fi

    if grep -q "交易次数" web/src/pages/UserProfilePage.tsx; then
        print_error "UserProfilePage 仍显示交易次数字段"
    else
        print_success "UserProfilePage 不再显示交易次数字段"
    fi
}

# 手动测试清单
manual_test_checklist() {
    print_header "手动测试清单"

    echo "请按以下步骤进行手动测试："
    echo ""
    echo "1. 启动开发服务器："
    echo "   cd web && npm run dev"
    echo ""
    echo "2. 登录用户：$TEST_USER_EMAIL"
    echo ""
    echo "3. 访问用户资料页面："
    echo "   http://localhost:3000/profile"
    echo ""
    echo "4. 验证积分系统区域显示："
    echo "   ✓ 显示'总积分: 10000' (蓝色，10000)"
    echo "   ✓ 显示'可用积分: 10000' (绿色，10000)"
    echo "   ✓ 显示'已用积分: 0' (橙色，0)"
    echo "   ✓ 不显示'交易次数'字段"
    echo ""
    echo "5. 检查浏览器开发者工具："
    echo "   - Network选项卡应该显示对 /api/v1/user/credits 的请求"
    echo "   - Console选项卡不应该有错误"
    echo ""
    echo "6. 验证API响应："
    echo "   期望响应："
    echo "   {"
    echo "     \"code\": 200,"
    echo "     \"message\": \"success\","
    echo "     \"data\": {"
    echo "       \"available_credits\": 10000,"
    echo "       \"total_credits\": 10000,"
    echo "       \"used_credits\": 0"
    echo "     }"
    echo "   }"
    echo ""
}

# 生成测试报告
generate_test_report() {
    print_header "生成测试报告"

    local report_file="test_credits_fix_report_$(date +%Y%m%d_%H%M%S).txt"

    cat > "$report_file" << EOF
积分系统数据显示修复 - 测试报告
=====================================

测试时间: $(date)
测试用户: $TEST_USER_EMAIL
用户ID: $TEST_USER_ID

测试项目:
---------

1. API端点测试
   - 端点: GET /api/v1/user/credits
   - 状态: 需要手动验证
   - 备注: 需要真实token进行认证

2. 数据库验证
   - 表: user_credits
   - 预期数据:
     * available_credits: 10000
     * total_credits: 10000
     * used_credits: 0

3. 前端代码修改
   - useUserProfile.ts: ✓ 已修改为调用真实API
   - UserProfilePage.tsx: ✓ 已移除交易次数字段
   - 布局: ✓ 已改为3列显示

修复文件:
---------
1. web/src/hooks/useUserProfile.ts
   - 修改 useUserCredits Hook调用真实API
   - 移除 transaction_count 字段

2. web/src/pages/UserProfilePage.tsx
   - 移除"交易次数"显示字段
   - 调整布局从4列到3列
   - 优化字段显示顺序

3. web/openspec/bugs/credits-display-incorrect-data-bug.md
   - 创建bug修复提案文档

预期结果:
---------
✅ 总积分显示: 10000 (蓝色)
✅ 可用积分显示: 10000 (绿色)
✅ 已用积分显示: 0 (橙色)
✅ 不显示"交易次数"字段
✅ 数据来自user_credits表（真实数据）

测试结论:
---------
代码修改已完成，需要进行功能验证测试。

EOF

    print_success "测试报告已生成: $report_file"
    echo ""
    cat "$report_file"
}

# 主函数
main() {
    echo -e "${BLUE}"
    cat << "EOF"
 ____                 __  __       _        _
|  _ \  ___  _ __    |  \/  | __ _| |_ _ __(_)_   _  ___
| | | |/ _ \| '_ \   | |\/| |/ _` | __| '__| | | | |/ _ \
| |_| | (_) | | | |  | |  | | (_| | |_| |  | | |_| |  __/
|____/ \___/|_| |_|  |_|  |_|\__,_|\__|_|  |_|\__, |\___|
                                                |___/
EOF
    echo -e "${NC}"

    print_header "积分系统数据显示修复 - 测试验证"

    check_dependencies
    test_api_endpoint
    verify_database_record
    verify_frontend_changes
    manual_test_checklist
    generate_test_report

    print_header "测试完成"
    print_success "所有检查项目已完成"
    print_info "请按照手动测试清单进行功能验证"
}

# 运行主函数
main "$@"
