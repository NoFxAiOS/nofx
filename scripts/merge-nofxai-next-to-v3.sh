#!/bin/bash
set -e  # 遇到錯誤立即停止

echo "=========================================="
echo "  合併 nofxai/next 改進到 z-dev-v3"
echo "=========================================="

cd "$(dirname "$0")/.."

# 顏色輸出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 確認當前在 z-dev-v3
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "z-dev-v3" ]; then
    echo -e "${RED}錯誤：當前不在 z-dev-v3 分支${NC}"
    echo "請先執行: git checkout z-dev-v3"
    exit 1
fi

# 2. 檢查是否有未提交的修改
if ! git diff-index --quiet HEAD --; then
    echo -e "${RED}錯誤：有未提交的修改${NC}"
    echo "請先提交或 stash 你的修改"
    git status --short
    exit 1
fi

# 3. 創建備份分支
BACKUP_BRANCH="backup-z-dev-v3-before-nofxai-merge-$(date +%Y%m%d-%H%M%S)"
echo -e "${YELLOW}創建備份分支: $BACKUP_BRANCH${NC}"
git branch "$BACKUP_BRANCH"
git push origin "$BACKUP_BRANCH" || echo "提示：無法推送備份分支到遠端（可能沒有配置 origin）"

# 4. 更新 nofxai remote
echo -e "${YELLOW}更新 nofxai remote...${NC}"
git fetch nofxai next

# 5. 定義要合併的 commits（按依賴順序）
# 分組方便管理和出錯時定位
declare -a PHASE1_COMMITS=(
    "aa9fabda"  # Slippage Protection（基礎）
    "5140ee32"  # Fill Price Verification（依賴 aa9fabda）
    "a3afaf98"  # Token 優化（獨立）
)

declare -a PHASE2_COMMITS=(
    "9b08d2a9"  # Cache Recovery（獨立）
    "50ca9293"  # PromptHash 過濾（基礎）
    "5d166f41"  # PromptHash 從模板計算（依賴 50ca9293）
    "b07133a8"  # PromptHash 測試
)

declare -a PHASE3_COMMITS=(
    "46facaf2"  # SL/TP 可見性
    "15d82dcb"  # Decision Actions 詳細字段
    "04b1ffa1"  # 顯示實際平倉價
    "6b6a39a4"  # 移動端 Overflow
    "29745a20"  # SharpeRatio 提示
)

declare -a PHASE4_COMMITS=(
    "96f775b8"  # InitialScanCycles 10000
    "1e2371ef"  # KISS 重構
)

# 6. 合併函數
merge_commits() {
    local phase_name=$1
    shift
    local commits=("$@")

    echo -e "\n${GREEN}=== $phase_name ===${NC}"

    for commit in "${commits[@]}"; do
        echo -e "${YELLOW}Cherry-picking $commit...${NC}"

        # 獲取 commit 訊息
        COMMIT_MSG=$(git log nofxai/next --oneline --grep="$commit" -1 2>/dev/null || git log nofxai/next --oneline | grep "$commit" | head -1)
        echo "  $COMMIT_MSG"

        if git cherry-pick "$commit" 2>&1; then
            echo -e "${GREEN}  ✅ 成功${NC}"
        else
            echo -e "${RED}  ❌ 衝突！${NC}"
            echo -e "${YELLOW}請手動解決衝突，然後：${NC}"
            echo "  1. 解決衝突後: git add <files>"
            echo "  2. 繼續: git cherry-pick --continue"
            echo "  3. 或放棄: git cherry-pick --abort"
            echo ""
            echo -e "${YELLOW}解決後重新執行此腳本會跳過已合併的 commits${NC}"
            exit 1
        fi
    done
}

# 7. 執行合併
echo -e "\n${GREEN}開始合併流程...${NC}"

merge_commits "Phase 1: 交易準確性修復" "${PHASE1_COMMITS[@]}"
merge_commits "Phase 2: AI 決策系統改進" "${PHASE2_COMMITS[@]}"
merge_commits "Phase 3: UI/UX 改進" "${PHASE3_COMMITS[@]}"
merge_commits "Phase 4: 配置優化" "${PHASE4_COMMITS[@]}"

# 8. 完成提示
echo -e "\n${GREEN}=========================================="
echo "  ✅ 合併完成！"
echo "==========================================${NC}"
echo ""
echo "下一步操作："
echo "  1. 運行測試: ./scripts/test-after-merge.sh"
echo "  2. 如果測試通過: git push origin z-dev-v3"
echo "  3. 如果有問題，恢復備份: git reset --hard $BACKUP_BRANCH"
echo ""
echo "備份分支: $BACKUP_BRANCH"
