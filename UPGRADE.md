# 📦 升級指南 (Upgrade Guide)

## 升級到 v2.x.x - 數據庫結構統一方案

### ⚠️ 重要提示

本次升級包含數據庫結構遷移，首次啟動時會自動執行遷移程序（約 1-5 秒）。

**升級前必讀：**
- ✅ 系統會**自動創建備份**，但仍建議手動備份數據庫
- ✅ 遷移過程中**請勿關閉程序**
- ✅ 新用戶直接使用新結構，**零遷移成本**
- ✅ 老用戶自動遷移，**一次性完成**（< 5 秒）

---

## 🎯 本次升級解決的問題

### 1. 新用戶無法創建交易員 ❌ → ✅
**問題**: 新安裝的用戶在創建交易員時遇到 500 錯誤
**原因**: 數據庫初始結構與查詢邏輯不匹配
**解決**: 統一使用 INTEGER AUTOINCREMENT 結構

### 2. 無法添加多個相同類型配置 ❌ → ✅
**問題**: 只能有一個 DeepSeek 模型、一個 Binance 配置
**原因**: 使用 TEXT PRIMARY KEY 限制了唯一性
**解決**: 使用自增 ID + 類型欄位，支持多配置

### 3. 老用戶升級失敗 ❌ → ✅
**問題**: 升級時遷移邏輯失敗導致無法啟動
**原因**: 遷移檢測不完整，欄位對應錯誤
**解決**: 智能檢測表結構，跳過已遷移表

---

## 📋 升級步驟

### 方案 A：自動升級（推薦）

```bash
# 1. 停止服務
pkill -f nofx

# 2. 手動備份數據庫（可選但推薦）
cp nofx.db nofx.db.backup.$(date +%Y%m%d_%H%M%S)

# 3. 拉取新代碼
git pull origin dev

# 4. 啟動服務
./start.sh

# 5. 觀察日誌
tail -f logs/nofx.log
```

**預期日誌輸出**:
```
🔄 开始迁移exchanges表...
✅ exchanges表迁移完成
🔄 开始迁移到自增ID结构（支持多配置）...
✅ 自动备份已创建: nofx.db.backup.pre-autoincrement-migration.20250114_120000
  🔄 迁移 ai_models 表...
  ✅ ai_models 表迁移完成
  🔄 迁移 exchanges 表...
  ✅ exchanges 表迁移完成
  🔄 更新 traders 表外键...
  ✅ traders 表更新完成
🔍 验证迁移数据完整性...
📊 数据统计: ai_models=2, exchanges=2, traders=3
✅ 迁移验证通过
✅ 自增ID结构迁移完成
```

---

### 方案 B：測試環境驗證（謹慎用戶）

如果你想在升級前測試遷移過程：

```bash
# 1. 運行測試腳本
./scripts/test-migration.sh nofx.db

# 2. 查看測試報告
cat test-migration-*/migration_test_report.md

# 3. 確認無誤後正式升級
git pull origin dev
./start.sh
```

---

## ⏱️ 遷移時間預估

| 數據規模 | 預估時間 |
|---------|---------|
| < 10 個配置 | < 1 秒 |
| 10-50 個配置 | 1-3 秒 |
| 50-100 個配置 | 3-5 秒 |
| > 100 個配置 | 5-10 秒 |

**註**: 配置數據通常很小（< 50 MB），即使有 1000 個配置也能在 10 秒內完成。

---

## 🔍 遷移驗證

升級後驗證系統正常工作：

```bash
# 1. 檢查服務狀態
curl http://localhost:8080/health

# 2. 登錄前端
open http://localhost:3000

# 3. 嘗試創建新交易員
# 前端: 配置 → 創建交易員

# 4. 驗證數據完整性
sqlite3 nofx.db "SELECT COUNT(*) FROM ai_models WHERE model_id != '';"
sqlite3 nofx.db "SELECT COUNT(*) FROM exchanges WHERE exchange_id != '';"
sqlite3 nofx.db "SELECT COUNT(*) FROM traders WHERE ai_model_id > 0 AND exchange_id > 0;"
```

---

## 🛠️ 遷移失敗處理

### 場景 1：遷移過程中斷電或強制終止

**症狀**:
```
❌ 数据库已损坏: database disk image is malformed
```

**解決方法**:
```bash
# 使用自動備份恢復
cp nofx.db.backup.pre-autoincrement-migration.* nofx.db

# 或使用手動備份
cp nofx.db.backup.20250114_120000 nofx.db

# 重新啟動
./start.sh
```

---

### 場景 2：數據庫文件鎖定

**症狀**:
```
❌ 启用WAL模式失败: database is locked
❌ 迁移失败: database is locked
```

**解決方法**:
```bash
# 1. 關閉所有實例
pkill -f nofx

# 2. 檢查並刪除鎖定文件
ls -la nofx.db*
rm nofx.db-shm nofx.db-wal

# 3. 重新啟動
./start.sh
```

---

### 場景 3：外鍵約束失敗

**症狀**:
```
❌ 发现 X 个孤立的 trader 记录（外键引用不存在）
```

**解決方法**:
```bash
# 1. 使用備份回滾
cp nofx.db.backup.pre-autoincrement-migration.* nofx.db

# 2. 聯繫支持並提供日誌
cat logs/nofx.log > migration_error.log

# 3. 提交 Issue
# https://github.com/NoFxAiOS/nofx/issues
```

---

## 🔄 回滾到舊版本

**⚠️ 重要**: 遷移後的數據庫**無法直接回滾**到舊版本。

如需回滾：

### 選項 1：使用備份
```bash
# 停止服務
pkill -f nofx

# 恢復備份
cp nofx.db.backup.20250114_120000 nofx.db

# 切換到舊版本
git checkout v1.x.x

# 啟動舊版本
./start.sh
```

### 選項 2：數據庫結構回滾（高級用戶）
```bash
# 將 INTEGER AUTOINCREMENT 結構轉回 TEXT PRIMARY KEY
sqlite3 nofx.db <<EOF
BEGIN TRANSACTION;

-- 創建舊結構表
CREATE TABLE ai_models_rollback (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 0,
    api_key TEXT DEFAULT '',
    custom_api_url TEXT DEFAULT '',
    custom_model_name TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 複製數據（使用 model_id 作為 id）
INSERT INTO ai_models_rollback
SELECT model_id, user_id, name, provider, enabled, api_key,
       custom_api_url, custom_model_name, created_at, updated_at
FROM ai_models;

-- 更新 traders 表（將 integer ID 轉回 text ID）
UPDATE traders SET ai_model_id = (
    SELECT model_id FROM ai_models WHERE id = traders.ai_model_id
);

-- 替換表
DROP TABLE ai_models;
ALTER TABLE ai_models_rollback RENAME TO ai_models;

COMMIT;
EOF

# 對 exchanges 表執行類似操作
# ...

# 切換到舊版本
git checkout v1.x.x
./start.sh
```

---

## 📊 新功能：多配置支持

升級後，你可以添加多個相同類型的配置：

### 添加多個 AI 模型
```bash
# 第一個 DeepSeek (API 1)
PUT /api/models
{
  "models": {
    "deepseek": {
      "enabled": true,
      "api_key": "API_KEY_1",
      "display_name": "DeepSeek - Production"
    }
  }
}

# 第二個 DeepSeek (API 2)
PUT /api/models
{
  "models": {
    "deepseek": {
      "enabled": true,
      "api_key": "API_KEY_2",
      "display_name": "DeepSeek - Testing"
    }
  }
}
```

**前端區分**: 使用 `display_name` 字段來區分不同配置（需前端支持）

---

## 🐛 常見問題 (FAQ)

### Q1: 遷移需要多久？
**A**: 通常 < 5 秒。配置數據量很小（< 50 MB），即使有大量配置也能快速完成。

### Q2: 遷移會丟失數據嗎？
**A**: 不會。遷移過程使用 `CREATE + COPY + RENAME` 模式，原始數據保留直到驗證通過。系統會自動創建備份。

### Q3: 可以跳過遷移嗎？
**A**: 不能。如果是老用戶，必須遷移才能使用新版本。新用戶直接使用新結構，無需遷移。

### Q4: 遷移失敗怎麼辦？
**A**:
1. 查看日誌: `cat logs/nofx.log`
2. 使用備份恢復: `cp nofx.db.backup.* nofx.db`
3. 提交 Issue 並附上日誌

### Q5: 如何確認遷移成功？
**A**: 查看日誌中的 `✅ 自增ID结构迁移完成` 和 `✅ 迁移验证通过`。

### Q6: 新版本有哪些改進？
**A**:
- ✅ 修復新用戶無法創建交易員的 bug
- ✅ 支持多個相同類型的 AI 模型/交易所配置
- ✅ 統一新老用戶數據庫結構
- ✅ 自動備份和驗證機制

---

## 📞 技術支持

如遇到問題，請：

1. **查看日誌**: `cat logs/nofx.log`
2. **查看測試報告**: 運行 `./scripts/test-migration.sh nofx.db`
3. **提交 Issue**: https://github.com/NoFxAiOS/nofx/issues
4. **附上信息**:
   - 日誌文件 (`logs/nofx.log`)
   - 測試報告 (`migration_test_report.md`)
   - 數據庫大小 (`ls -lh nofx.db`)
   - 系統信息 (`uname -a`)

---

## 🔐 安全建議

1. ✅ **定期備份**: 建議每週備份一次 `nofx.db`
2. ✅ **測試環境驗證**: 先在測試環境升級，確認無誤後再升級生產環境
3. ✅ **監控日誌**: 升級後密切關注日誌，確保無異常
4. ✅ **保留備份**: 至少保留最近 3 次的備份文件

---

**升級愉快！** 🚀

如有任何問題，歡迎在 GitHub Issues 提問。
