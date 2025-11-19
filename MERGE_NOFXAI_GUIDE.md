# 🚀 合併 nofxai/next 改進到 z-dev-v3

## ⚡ 快速開始（3 步驟）

### 1️⃣ 執行合併
```bash
cd ~/Documents/GitHub/nofx
git checkout z-dev-v3
./scripts/merge-nofxai-next-to-v3.sh
```

### 2️⃣ 運行測試
```bash
./scripts/test-after-merge.sh
```

### 3️⃣ 推送到遠端
```bash
git push origin z-dev-v3
```

**完成！整個流程 < 10 分鐘（如果無衝突）**

---

## 📦 將合併的改進（15 個 commits）

### Phase 1: 交易準確性修復（3 個）
- ✅ `aa9fabda` - Slippage Protection
- ✅ `5140ee32` - Fill Price Verification（100% 準確成交價）
- ✅ `a3afaf98` - Token 優化（節省 5-15% AI 成本）

### Phase 2: AI 決策系統改進（4 個）
- ✅ `9b08d2a9` - Cache Recovery（服務重啟恢復）
- ✅ `50ca9293` - PromptHash 可切換過濾
- ✅ `5d166f41` - PromptHash 從模板計算
- ✅ `b07133a8` - PromptHash 測試

### Phase 3: UI/UX 改進（5 個）
- ✅ `46facaf2` - Stop Loss/Take Profit 在 AI prompt 可見
- ✅ `15d82dcb` - Decision Actions 詳細字段
- ✅ `04b1ffa1` - 顯示實際平倉價
- ✅ `6b6a39a4` - 移動端 Overflow 修復
- ✅ `29745a20` - SharpeRatio 數據充足性提示

### Phase 4: 配置優化（2 個）
- ✅ `96f775b8` - InitialScanCycles 增加到 10000
- ✅ `1e2371ef` - KISS 原則重構

---

## 🛠️ 如果遇到衝突

### 衝突處理流程
```bash
# 1. 查看衝突文件
git status

# 2. 手動解決衝突（編輯文件）
code <conflict-file>

# 3. 標記為已解決
git add <conflict-file>

# 4. 繼續 cherry-pick
git cherry-pick --continue

# 5. 重新運行合併腳本
./scripts/merge-nofxai-next-to-v3.sh
```

### 常見衝突文件
- `trader/auto_trader.go` - 平倉邏輯可能有差異
- `decision/engine.go` - AI prompt 生成邏輯
- `logger/decision_logger.go` - 日誌結構

---

## 🧪 測試驗證清單

### 自動測試（test-after-merge.sh）
- [x] Go 編譯檢查
- [x] Trader 模組測試
- [x] Decision 模組測試
- [x] Logger 模組測試
- [x] 前端編譯
- [x] 測試覆蓋率

### 手動測試（建議）
```bash
# 1. 啟動服務
./start.sh

# 2. 創建測試交易員
curl http://localhost:8080/api/traders -X POST -d '{"name":"test",...}'

# 3. 檢查新功能
# - Fill Price Verification: 平倉後檢查是否記錄實際成交價
# - PromptHash Filtering: 前端 AI Learning 頁面是否有過濾選項
# - Token 優化: 檢查 AI prompt 是否無重複 symbol

# 4. 檢查日誌
tail -f logs/nofx.log | grep -E "fill price|PromptHash|token"
```

---

## 🔄 回滾操作

### 如果測試失敗，回滾到備份
```bash
# 1. 查找備份分支
git branch | grep backup-z-dev-v3

# 2. 回滾到備份
git reset --hard backup-z-dev-v3-before-nofxai-merge-<timestamp>

# 3. 強制推送（謹慎！）
git push origin z-dev-v3 --force-with-lease
```

---

## 📊 預期效果

### 交易準確性
- ✅ 平倉價格 100% 準確（不再依賴市場快照）
- ✅ 滑點正確計算
- ✅ 風險管理更精準

### AI 決策改進
- ✅ 策略版本追蹤準確
- ✅ AI prompt 無重複信息
- ✅ Stop Loss/Take Profit 完整可見

### 系統穩定性
- ✅ 服務重啟後自動恢復交易緩存
- ✅ 開倉位置不丟失

### 成本優化
- ✅ AI token 使用減少 5-15%
- ✅ 月省 $50-150（假設日均 1000 次調用）

---

## ❓ 常見問題

### Q: 合併後服務啟動變慢了？
**A**: 因為 `InitialScanCycles` 從 1000 增加到 10000。可以調整 `logger/decision_logger.go`：
```go
const InitialScanCycles = 5000  // 改為中間值
```

### Q: 如何只合併部分改進？
**A**: 編輯 `scripts/merge-nofxai-next-to-v3.sh`，註釋掉不需要的 commits：
```bash
declare -a PHASE4_COMMITS=(
    # "96f775b8"  # 不要 InitialScanCycles 增加
    "1e2371ef"   # 保留 KISS 重構
)
```

### Q: 合併後如何同步到 v2？
**A**: 在 v3 充分測試後，選擇性 cherry-pick 到 v2：
```bash
git checkout z-dev-v2
git cherry-pick aa9fabda 5140ee32 a3afaf98  # 只合併關鍵修復
```

---

## 📝 合併日誌

記錄每次合併的結果：

| 日期 | 操作人 | Commits 數量 | 測試結果 | 備註 |
|------|--------|-------------|---------|------|
| 2025-11-19 | - | 15 | ⏳ 待測試 | 初次合併 |

---

## 🔗 相關資源

- [nofxai/nofx 原始 repo](https://github.com/nofxai/nofx)
- [詳細改動分析](./NOFXAI_CHANGES_ANALYSIS.md) - 如需創建
- [測試報告模板](./TEST_REPORT_TEMPLATE.md) - 如需創建

---

**最後更新**: 2025-11-19
**維護者**: @sotadic
