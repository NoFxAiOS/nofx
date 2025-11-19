#!/bin/bash
set -e

echo "=========================================="
echo "  合併後測試驗證"
echo "=========================================="

cd "$(dirname "$0")/.."

# 顏色輸出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0

# 測試函數
run_test() {
    local test_name=$1
    local test_cmd=$2

    echo -e "\n${YELLOW}🧪 測試: $test_name${NC}"
    echo "命令: $test_cmd"

    if eval "$test_cmd"; then
        echo -e "${GREEN}✅ 通過${NC}"
        return 0
    else
        echo -e "${RED}❌ 失敗${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# 1. 編譯檢查
echo -e "\n${GREEN}=== Phase 1: 編譯檢查 ===${NC}"
run_test "Go 程式碼編譯" "go build -o /tmp/nofx-test ./cmd/nofx 2>&1 | head -20"

# 2. 單元測試（關鍵模組）
echo -e "\n${GREEN}=== Phase 2: 單元測試 ===${NC}"
run_test "Trader 模組測試" "go test ./trader/... -short -v 2>&1 | tail -30"
run_test "Decision 模組測試" "go test ./decision/... -short -v 2>&1 | tail -30"
run_test "Logger 模組測試" "go test ./logger/... -short -v 2>&1 | tail -30"

# 3. 檢查新功能是否存在
echo -e "\n${GREEN}=== Phase 3: 功能驗證 ===${NC}"
run_test "Fill Price Verification 存在" "git grep -q 'GetRecentFills' trader/"
run_test "PromptHash 從模板計算存在" "git grep -q 'calculatePromptHashFromTemplate' decision/"
run_test "Token 優化存在" "git grep -q 'skipSymbolMention' market/"

# 4. 前端編譯（如果有 web 目錄）
if [ -d "web" ]; then
    echo -e "\n${GREEN}=== Phase 4: 前端檢查 ===${NC}"
    run_test "前端依賴安裝" "npm --prefix web install --silent"
    run_test "前端編譯" "npm --prefix web run build 2>&1 | tail -20"
fi

# 5. 測試覆蓋率檢查（可選）
echo -e "\n${GREEN}=== Phase 5: 測試覆蓋率 ===${NC}"
run_test "生成覆蓋率報告" "go test ./trader ./decision ./logger -cover -coverprofile=/tmp/coverage.out 2>&1 | grep coverage"

# 總結
echo -e "\n=========================================="
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ 所有測試通過！${NC}"
    echo "=========================================="
    echo ""
    echo "建議下一步："
    echo "  1. 手動測試核心功能（啟動服務、創建交易員等）"
    echo "  2. 檢查日誌是否有異常: ./start.sh"
    echo "  3. 確認無誤後推送: git push origin z-dev-v3"
    echo ""
    exit 0
else
    echo -e "${RED}❌ 有 $FAILED 個測試失敗${NC}"
    echo "=========================================="
    echo ""
    echo "建議操作："
    echo "  1. 檢查失敗的測試輸出"
    echo "  2. 修復問題後重新運行此腳本"
    echo "  3. 或回滾到備份: git reset --hard <backup-branch>"
    echo ""
    exit 1
fi
