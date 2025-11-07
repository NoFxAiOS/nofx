#!/bin/bash
# NOFX 加密系統一鍵部署腳本
# 使用方式: chmod +x deploy_encryption.sh && ./deploy_encryption.sh

set -e  # 遇到錯誤立即退出

# 顏色定義
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 輔助函數
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 檢查必要工具
check_dependencies() {
    log_info "檢查依賴工具..."

    if ! command -v go &> /dev/null; then
        log_error "Go 未安裝，請先安裝 Go 1.21+"
        exit 1
    fi

    if ! command -v npm &> /dev/null; then
        log_error "npm 未安裝，請先安裝 Node.js 18+"
        exit 1
    fi

    if ! command -v sqlite3 &> /dev/null; then
        log_warning "sqlite3 未安裝，部分驗證功能不可用"
    fi

    log_success "依賴檢查通過"
}

# 備份數據庫
backup_database() {
    log_info "備份現有數據庫..."

    if [ -f "config.db" ]; then
        BACKUP_FILE="config.db.pre_encryption.$(date +%Y%m%d_%H%M%S).backup"
        cp config.db "$BACKUP_FILE"
        log_success "數據庫已備份到: $BACKUP_FILE"
    else
        log_warning "未找到 config.db，跳過備份（首次安裝）"
    fi
}

# 創建密鑰目錄
setup_secrets_dir() {
    log_info "設置密鑰目錄..."

    if [ ! -d ".secrets" ]; then
        mkdir -p .secrets
        chmod 700 .secrets
        log_success "密鑰目錄已創建: .secrets/"
    else
        log_warning "密鑰目錄已存在，跳過創建"
    fi
}

# 更新 .gitignore
update_gitignore() {
    log_info "更新 .gitignore..."

    if ! grep -q ".secrets/" .gitignore 2>/dev/null; then
        echo ".secrets/" >> .gitignore
        log_success "已添加 .secrets/ 到 .gitignore"
    fi

    if ! grep -q "config.db.backup" .gitignore 2>/dev/null; then
        echo "config.db.*.backup" >> .gitignore
        log_success "已添加備份檔案規則到 .gitignore"
    fi
}

# 安裝依賴
install_dependencies() {
    log_info "安裝 Go 依賴..."
    go mod tidy
    log_success "Go 依賴已更新"

    log_info "安裝前端依賴..."
    cd web
    if [ ! -d "node_modules" ]; then
        npm install
    fi
    npm install tweetnacl tweetnacl-util @noble/secp256k1 --save
    cd ..
    log_success "前端依賴已安裝"
}

# 運行測試
run_tests() {
    log_info "運行加密系統測試..."

    if go test ./crypto -v > /tmp/nofx_test.log 2>&1; then
        log_success "加密系統測試通過"
        cat /tmp/nofx_test.log | grep "✅"
    else
        log_error "加密系統測試失敗，詳情:"
        cat /tmp/nofx_test.log
        exit 1
    fi
}

# 遷移數據
migrate_data() {
    log_info "遷移現有數據到加密格式..."

    if [ -f "config.db" ]; then
        # 檢查是否已經加密過
        if sqlite3 config.db "SELECT api_key FROM exchanges LIMIT 1;" 2>/dev/null | grep -q "=="; then
            log_warning "數據庫似乎已經加密過，跳過遷移"
            read -p "是否強制重新遷移？(y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                return
            fi
        fi

        if go run scripts/migrate_encryption.go; then
            log_success "數據遷移完成"
        else
            log_error "數據遷移失敗"
            exit 1
        fi
    else
        log_warning "未找到數據庫，跳過遷移"
    fi
}

# 設置環境變數
setup_env_vars() {
    log_info "設置環境變數..."

    if [ -f ".secrets/master.key" ]; then
        MASTER_KEY=$(cat .secrets/master.key)

        # 添加到當前 shell 配置
        SHELL_RC="$HOME/.bashrc"
        if [ -f "$HOME/.zshrc" ]; then
            SHELL_RC="$HOME/.zshrc"
        fi

        if ! grep -q "NOFX_MASTER_KEY" "$SHELL_RC" 2>/dev/null; then
            echo "" >> "$SHELL_RC"
            echo "# NOFX 加密系統主密鑰" >> "$SHELL_RC"
            echo "export NOFX_MASTER_KEY='$MASTER_KEY'" >> "$SHELL_RC"
            log_success "主密鑰已添加到 $SHELL_RC"
        else
            log_warning "主密鑰已存在於 $SHELL_RC"
        fi

        # 導出到當前 session
        export NOFX_MASTER_KEY="$MASTER_KEY"
        log_success "主密鑰已導出到當前 session"
    else
        log_warning "主密鑰文件未生成，請先運行應用初始化"
    fi
}

# 驗證部署
verify_deployment() {
    log_info "驗證部署結果..."

    # 1. 檢查密鑰檔案
    if [ -f ".secrets/rsa_private.pem" ] && [ -f ".secrets/rsa_public.pem" ] && [ -f ".secrets/master.key" ]; then
        log_success "密鑰檔案完整"
    else
        log_error "密鑰檔案缺失，請檢查日誌"
        return 1
    fi

    # 2. 檢查檔案權限
    PERM=$(stat -f "%Lp" .secrets 2>/dev/null || stat -c "%a" .secrets 2>/dev/null)
    if [ "$PERM" = "700" ]; then
        log_success "密鑰目錄權限正確 (700)"
    else
        log_warning "密鑰目錄權限為 $PERM，建議修改為 700"
        chmod 700 .secrets
    fi

    # 3. 檢查資料庫加密
    if [ -f "config.db" ] && command -v sqlite3 &> /dev/null; then
        SAMPLE=$(sqlite3 config.db "SELECT api_key FROM exchanges WHERE api_key != '' LIMIT 1;" 2>/dev/null || echo "")
        if echo "$SAMPLE" | grep -q "=="; then
            log_success "數據庫密鑰已加密（Base64 格式）"
        else
            log_warning "數據庫可能未加密或無數據"
        fi
    fi

    log_success "部署驗證通過"
}

# 打印後續步驟
print_next_steps() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${GREEN}🎉 加密系統部署成功！${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "📝 後續步驟:"
    echo ""
    echo "  1️⃣  啟動後端服務:"
    echo "     $ go run main.go"
    echo ""
    echo "  2️⃣  啟動前端服務:"
    echo "     $ cd web && npm run dev"
    echo ""
    echo "  3️⃣  驗證加密功能:"
    echo "     $ curl http://localhost:8080/api/crypto/public-key"
    echo ""
    echo "  4️⃣  查看審計日誌:"
    echo "     $ sqlite3 config.db 'SELECT * FROM audit_logs ORDER BY timestamp DESC LIMIT 10;'"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "⚠️  重要提醒:"
    echo ""
    echo "  • 請妥善保管 .secrets/ 目錄（已設置為 700 權限）"
    echo "  • 生產環境務必使用環境變數管理主密鑰"
    echo "  • 定期執行密鑰輪換（建議每季度一次）"
    echo "  • 數據庫備份已保存，驗證無誤後可手動刪除"
    echo ""
    echo "📚 詳細文檔:"
    echo "  - 快速開始: cat SECURITY_QUICKSTART.md"
    echo "  - 完整指南: cat ENCRYPTION_DEPLOYMENT.md"
    echo ""
}

# 主函數
main() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${BLUE}🔐 NOFX 加密系統部署腳本${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # 確認執行
    log_warning "此腳本將:"
    echo "  1. 備份現有數據庫"
    echo "  2. 生成 RSA-4096 密鑰對"
    echo "  3. 生成 AES-256 主密鑰"
    echo "  4. 遷移現有數據到加密格式"
    echo "  5. 設置環境變數"
    echo ""
    read -p "是否繼續？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "已取消部署"
        exit 0
    fi

    # 執行部署步驟
    check_dependencies
    backup_database
    setup_secrets_dir
    update_gitignore
    install_dependencies
    run_tests
    migrate_data
    setup_env_vars
    verify_deployment
    print_next_steps
}

# 執行主函數
main
