#!/bin/bash

# ============================================
# Vercel 自动化部署脚本
# 项目: nofx-web (React + Vite)
# ============================================\n

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 图标
CHECK_MARK="✅"
CROSS_MARK="❌"
ROCKET="🚀"
GEAR="⚙️"
BOOK="📚"
WARNING="⚠️"

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}${BOOK} $1${NC}"
}

print_success() {
    echo -e "${GREEN}${CHECK_MARK} $1${NC}"
}

print_error() {
    echo -e "${RED}${CROSS_MARK} $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}${WARNING} $1${NC}"
}

print_step() {
    echo -e "\n${GEAR} $1"
}

print_header() {
    echo -e "\n${ROCKET} ========================================"
    echo -e "${ROCKET}  $1"
    echo -e "${ROCKET} ========================================\n"
}

# 检查工具是否安装
check_tool() {
    local tool=$1
    local command=$2

    print_step "检查工具: $tool"

    if command -v $command &> /dev/null; then
        local version=$($command --version 2>&1 | head -n 1)
        print_success "$tool 已安装 (版本: $version)"
        return 0
    else
        print_error "$tool 未安装"
        return 1
    fi
}

# 检查 Node.js 和 npm
check_nodejs() {
    print_step "检查 Node.js 环境"

    if ! command -v node &> /dev/null; then
        print_error "Node.js 未安装"
        print_info "请安装 Node.js: https://nodejs.org/"
        exit 1
    fi

    local node_version=$(node --version)
    local npm_version=$(npm --version)

    print_success "Node.js: $node_version"
    print_success "npm: $npm_version"
}

# 检查 Vercel CLI
check_vercel() {
    if ! command -v vercel &> /dev/null; then
        print_error "Vercel CLI 未安装"
        print_info "正在安装 Vercel CLI..."
        npm install -g vercel
    fi

    local vercel_version=$(vercel --version)
    print_success "Vercel CLI: $vercel_version"
}

# 检查项目文件
check_project() {
    print_step "检查项目文件"

    if [ ! -f "package.json" ]; then
        print_error "package.json 不存在"
        exit 1
    fi
    print_success "package.json 存在"

    if [ ! -f "vercel.json" ]; then
        print_warning "vercel.json 不存在，将使用默认配置"
    else
        print_success "vercel.json 存在"
    fi

    if [ ! -f ".env.local" ]; then
        print_warning ".env.local 不存在"
    else
        print_success ".env.local 存在"
    fi
}

# 安装依赖
install_dependencies() {
    print_step "安装项目依赖"

    if [ -d "node_modules" ]; then
        print_info "依赖已存在，跳过安装"
        return 0
    fi

    npm install
    print_success "依赖安装完成"
}

# 本地构建测试
build_project() {
    print_step "本地构建测试"

    if npm run build; then
        print_success "构建成功 ✅"
        return 0
    else
        print_error "构建失败 ❌"
        print_info "请检查错误信息并修复后重试"
        return 1
    fi
}

# 检查 Vercel 登录状态
check_vercel_login() {
    print_step "检查 Vercel 登录状态"

    if vercel whoami &> /dev/null; then
        local username=$(vercel whoami)
        print_success "已登录 Vercel (用户: $username)"
        return 0
    else
        print_warning "未登录 Vercel"
        print_info "请运行: vercel login"
        return 1
    fi
}

# 登录 Vercel
login_vercel() {
    print_step "登录 Vercel"

    print_info "打开浏览器完成登录..."
    vercel login
}

# 部署到 Vercel
deploy_to_vercel() {
    print_step "部署到 Vercel"

    print_info "部署到生产环境..."
    print_info "URL 将自动生成"
    
    # 直接部署，使用已关联的项目配置
    if vercel --prod --yes --scope gyc567s-projects; then
        print_success "部署成功 🎉"
        return 0
    else
        print_error "部署失败 ❌"
        return 1
    fi
}

# 显示部署结果
show_deployment_info() {
    print_step "部署信息"

    print_info "查看部署历史: vercel ls"
    print_info "查看部署日志: vercel logs <url>"
    print_info "查看部署详情: vercel inspect <url>"
}

# 主函数
main() {
    print_header "Vercel 自动化部署脚本"

    # 检查环境
    check_nodejs
    check_vercel
    check_project

    # 安装依赖和构建
    install_dependencies
    build_project

    # 部署
    if ! check_vercel_login; then
        login_vercel
    fi

    deploy_to_vercel

    # 完成
    print_header "部署完成"
    print_success "应用已成功部署到 Vercel"
    show_deployment_info

    echo -e "\n${GREEN}${ROCKET} 部署流程结束${NC}\n"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            echo "Vercel 自动化部署脚本"
            echo ""
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  -h, --help     显示帮助信息"
            echo "  -s, --skip     跳过本地构建测试"
            echo "  -l, --login    强制重新登录"
            echo ""
            exit 0
            ;;
        -s|--skip)
            SKIP_BUILD=true
            shift
            ;;
        -l|--login)
            FORCE_LOGIN=true
            shift
            ;;
        *)
            echo "未知选项: $1"
            echo "使用 -h 或 --help 查看帮助"
            exit 1
            ;;
    esac
done

# 执行主函数
main
