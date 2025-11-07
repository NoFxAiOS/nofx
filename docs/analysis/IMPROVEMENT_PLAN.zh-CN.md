# 🚀 NOFX 系統改進計劃 - 分階段執行方案

**制定日期**: 2025-11-06
**目標版本**: v4.0.0（安全加固） → v5.0.0（穩定增強） → v6.0.0（功能擴展）
**預估總工期**: 6-8 個月

---

## 📋 目錄

1. [概述](#概述)
2. [階段 0：緊急修復（1 週）](#階段-0緊急修復1-週)
3. [階段 1：安全加固（2-3 週）](#階段-1安全加固2-3-週)
4. [階段 2：穩定性提升（4-6 週）](#階段-2穩定性提升4-6-週)
5. [階段 3：性能優化（3-4 週）](#階段-3性能優化3-4-週)
6. [階段 4：功能擴展（8-12 週）](#階段-4功能擴展8-12-週)
7. [階段 5：企業級準備（持續）](#階段-5企業級準備持續)
8. [資源需求](#資源需求)
9. [風險評估](#風險評估)
10. [成功指標](#成功指標)

---

## 概述

### 改進策略

本改進計劃遵循「**安全優先、穩定為本、循序漸進**」的原則，分階段推進系統從當前的 **6.5/10** 提升至 **9/10** 的生產級質量。

### 核心目標

| 階段 | 重點 | 目標評分 | 工期 |
|------|------|----------|------|
| 階段 0 | 🚨 緊急安全修復 | 7.0/10 | 1 週 |
| 階段 1 | 🔒 全面安全加固 | 7.5/10 | 2-3 週 |
| 階段 2 | 🛡️ 穩定性提升 | 8.0/10 | 4-6 週 |
| 階段 3 | ⚡ 性能優化 | 8.5/10 | 3-4 週 |
| 階段 4 | 🎯 功能擴展 | 9.0/10 | 8-12 週 |
| 階段 5 | 🏢 企業級準備 | 9.5/10 | 持續 |

---

## 階段 0：緊急修復（1 週）

### 🎯 目標
**立即消除生產環境阻斷性安全漏洞**

### 優先級：🔴 P0 - 阻斷發布

### 任務清單

#### 1. API 密鑰洩漏修復（1 天）

**問題：** API 響應返回完整密鑰

**解決方案：**
```go
// api/server.go - 修改 GetModels 和 GetExchanges 端點

func maskAPIKey(key string) string {
    if len(key) <= 8 {
        return "****"
    }
    return key[:4] + "..." + key[len(key)-4:]
}

// 在返回前遮罩
modelConfig.APIKey = maskAPIKey(modelConfig.APIKey)
```

**驗收標準：**
- [ ] 所有 API 響應僅返回遮罩後的密鑰
- [ ] 前端顯示 `sk-xx...xxxx` 格式
- [ ] 現有功能不受影響

**工作量：** 4 小時

---

#### 2. 禁用默認 Admin Mode（30 分鐘）

**問題：** `admin_mode` 默認為 `true`

**解決方案：**
```go
// config/database.go

// 修改默認值
configs := map[string]string{
    "admin_mode": "false",  // 改為 false
    "beta_mode": "false",
    // ...
}

// 添加警告日誌
if adminMode {
    log.Println("⚠️⚠️⚠️ 警告：Admin Mode 已啟用，所有認證已繞過！")
    log.Println("⚠️⚠️⚠️ 僅在開發環境使用，生產環境必須禁用！")
}
```

**驗收標準：**
- [ ] 新安裝默認 `admin_mode = false`
- [ ] 啟用時顯示醒目警告
- [ ] 文檔更新說明風險

**工作量：** 30 分鐘

---

#### 3. CORS 白名單配置（1 小時）

**問題：** `AllowOrigins: ["*"]` 允許任何來源

**解決方案：**
```go
// api/server.go

// 從配置或環境變量讀取
allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
if allowedOrigins == "" {
    allowedOrigins = "http://localhost:3000,http://localhost:5173"
}

origins := strings.Split(allowedOrigins, ",")

router.Use(cors.New(cors.Config{
    AllowOrigins:     origins,
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}))
```

**環境變量：**
```bash
# .env
ALLOWED_ORIGINS=http://localhost:3000,https://yourdomain.com
```

**驗收標準：**
- [ ] 僅白名單域名可訪問
- [ ] 配置靈活（環境變量）
- [ ] 預設安全默認值

**工作量：** 1 小時

---

#### 4. 強制 JWT 密鑰設置（30 分鐘）

**問題：** 可使用弱默認密鑰

**解決方案：**
```go
// config/database.go

jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    jwtSecret = getSysConfig("jwt_secret")
}

if jwtSecret == "" || jwtSecret == "nofx-default-secret-key-change-me" {
    log.Fatal("❌ 安全錯誤：必須設置強 JWT_SECRET！\n" +
              "   請在環境變量或 config.json 中設置至少 32 字符的隨機密鑰\n" +
              "   生成方法：openssl rand -base64 32")
}

if len(jwtSecret) < 32 {
    log.Fatal("❌ 安全錯誤：JWT_SECRET 必須至少 32 字符")
}
```

**驗收標準：**
- [ ] 啟動時強制檢查
- [ ] 拒絕弱密鑰
- [ ] 提供生成方法提示

**工作量：** 30 分鐘

---

#### 5. 基礎速率限制（2 小時）

**問題：** 無 API 速率限制

**解決方案：**
```go
// 安裝依賴
go get github.com/ulule/limiter/v3
go get github.com/ulule/limiter/v3/drivers/store/memory

// api/server.go
import (
    "github.com/ulule/limiter/v3"
    mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
    "github.com/ulule/limiter/v3/drivers/store/memory"
)

// 創建速率限制器
rate := limiter.Rate{
    Period: 1 * time.Minute,
    Limit:  60,  // 每分鐘 60 次請求
}
store := memory.NewStore()
middleware := mgin.NewMiddleware(limiter.New(store, rate))

// 應用到路由
router.Use(middleware)

// 敏感端點更嚴格限制
authRate := limiter.Rate{
    Period: 1 * time.Minute,
    Limit:  5,  // 登錄每分鐘 5 次
}
authLimiter := mgin.NewMiddleware(limiter.New(store, authRate))

router.POST("/api/login", authLimiter, handleLogin)
router.POST("/api/register", authLimiter, handleRegister)
```

**驗收標準：**
- [ ] 全局 60 req/min 限制
- [ ] 登錄端點 5 req/min
- [ ] 超限返回 429 Too Many Requests
- [ ] 響應頭包含限制信息

**工作量：** 2 小時

---

### 階段 0 驗收標準

- [ ] 所有 P0 任務完成
- [ ] 安全掃描無嚴重漏洞
- [ ] 通過人工滲透測試
- [ ] 文檔更新

### 發布產出

**版本：** v3.0.1（安全熱修復）

**變更日誌：**
```markdown
## [3.0.1] - 2025-XX-XX

### 🔒 安全修復（嚴重）
- 修復 API 響應中暴露完整密鑰的問題
- 禁用默認 Admin Mode
- 實現 CORS 白名單
- 強制設置強 JWT 密鑰
- 添加基礎速率限制

### ⚠️ 破壞性變更
- `admin_mode` 默認值改為 `false`
- 必須設置自定義 `JWT_SECRET`（至少 32 字符）
```

---

## 階段 1：安全加固（2-3 週）

### 🎯 目標
**實現生產級安全標準**

### 優先級：🟡 P1 - 高優先級

### 1.1 憑證加密存儲（1 週）

#### 任務 1.1.1：實現加密工具包（2 天）

**實現：**
```go
// crypto/aes.go - 新建套件

package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

var encryptionKey []byte

// InitEncryption 從環境變量初始化
func InitEncryption() error {
    keyStr := os.Getenv("ENCRYPTION_KEY")
    if keyStr == "" {
        return errors.New("必須設置 ENCRYPTION_KEY 環境變量")
    }

    key, err := base64.StdEncoding.DecodeString(keyStr)
    if err != nil || len(key) != 32 {
        return errors.New("ENCRYPTION_KEY 必須是 32 字節的 base64 字符串")
    }

    encryptionKey = key
    return nil
}

// Encrypt AES-256-GCM 加密
func Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt AES-256-GCM 解密
func Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(encryptionKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("密文太短")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

**密鑰生成工具：**
```bash
# scripts/generate_encryption_key.sh
#!/bin/bash
echo "生成 32 字節 AES-256 加密密鑰..."
openssl rand -base64 32
echo ""
echo "請將上述密鑰設置到環境變量："
echo "export ENCRYPTION_KEY='<上述密鑰>'"
```

**驗收標準：**
- [ ] AES-256-GCM 加密實現
- [ ] 密鑰從環境變量載入
- [ ] 單元測試覆蓋率 100%
- [ ] 性能測試（加密/解密 < 1ms）

**工作量：** 16 小時

---

#### 任務 1.1.2：數據庫遷移腳本（1 天）

**實現：**
```go
// scripts/migrate_encrypt_credentials.go

package main

import (
    "database/sql"
    "fmt"
    "log"
    "nofx/crypto"

    _ "github.com/mattn/go-sqlite3"
)

func main() {
    // 初始化加密
    if err := crypto.InitEncryption(); err != nil {
        log.Fatal(err)
    }

    db, err := sql.Open("sqlite3", "./config.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 遷移 AI Models
    log.Println("🔄 遷移 AI Models 表...")
    migrateAIModels(db)

    // 遷移 Exchanges
    log.Println("🔄 遷移 Exchanges 表...")
    migrateExchanges(db)

    log.Println("✅ 遷移完成！")
}

func migrateAIModels(db *sql.DB) {
    rows, _ := db.Query("SELECT id, api_key FROM ai_models WHERE api_key != ''")
    defer rows.Close()

    for rows.Next() {
        var id, apiKey string
        rows.Scan(&id, &apiKey)

        // 檢查是否已加密
        if _, err := crypto.Decrypt(apiKey); err == nil {
            log.Printf("  跳過 %s (已加密)", id)
            continue
        }

        // 加密
        encrypted, err := crypto.Encrypt(apiKey)
        if err != nil {
            log.Printf("  ❌ 加密失敗 %s: %v", id, err)
            continue
        }

        // 更新
        db.Exec("UPDATE ai_models SET api_key = ? WHERE id = ?", encrypted, id)
        log.Printf("  ✅ 已加密 %s", id)
    }
}

func migrateExchanges(db *sql.DB) {
    // 類似實現...
}
```

**使用方法：**
```bash
# 1. 設置加密密鑰
export ENCRYPTION_KEY=$(openssl rand -base64 32)

# 2. 備份數據庫
cp config.db config.db.backup

# 3. 執行遷移
go run scripts/migrate_encrypt_credentials.go

# 4. 驗證
sqlite3 config.db "SELECT id, api_key FROM ai_models LIMIT 1"
```

**驗收標準：**
- [ ] 自動檢測並遷移明文憑證
- [ ] 冪等性（可重複執行）
- [ ] 完整的錯誤處理
- [ ] 遷移日誌記錄

**工作量：** 8 小時

---

#### 任務 1.1.3：應用層集成（2 天）

**修改：**
```go
// config/database.go

import "nofx/crypto"

func SaveAIModel(model *AIModel) error {
    // 加密 API 密鑰
    if model.APIKey != "" {
        encrypted, err := crypto.Encrypt(model.APIKey)
        if err != nil {
            return fmt.Errorf("加密 API 密鑰失敗: %w", err)
        }
        model.APIKey = encrypted
    }

    // 保存到數據庫...
}

func GetAIModel(id string) (*AIModel, error) {
    // 從數據庫讀取...

    // 解密 API 密鑰
    if model.APIKey != "" {
        decrypted, err := crypto.Decrypt(model.APIKey)
        if err != nil {
            return nil, fmt.Errorf("解密 API 密鑰失敗: %w", err)
        }
        model.APIKey = decrypted
    }

    return model, nil
}
```

**API 層修改：**
```go
// api/server.go

func handleGetModels(c *gin.Context) {
    models, err := config.GetAIModels(userID)
    if err != nil {
        c.JSON(500, gin.H{"error": "獲取模型失敗"})
        return
    }

    // 遮罩密鑰後返回
    for i := range models {
        models[i].APIKey = maskAPIKey(models[i].APIKey)
    }

    c.JSON(200, models)
}
```

**驗收標準：**
- [ ] 所有憑證讀寫自動加密/解密
- [ ] API 響應遮罩密鑰
- [ ] 不影響現有功能
- [ ] 性能無明顯下降

**工作量：** 16 小時

---

### 1.2 審計日誌系統（3 天）

#### 實現：

**數據庫表：**
```sql
-- 審計日誌表
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    changes TEXT,  -- JSON 格式
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_action (action),
    INDEX idx_timestamp (timestamp)
);
```

**日誌記錄器：**
```go
// logger/audit.go

package logger

type AuditLog struct {
    UserID       string
    Action       string  // "create", "update", "delete", "login", "logout"
    ResourceType string  // "trader", "ai_model", "exchange"
    ResourceID   string
    IPAddress    string
    UserAgent    string
    Changes      map[string]interface{}
}

func LogAudit(log *AuditLog) error {
    changesJSON, _ := json.Marshal(log.Changes)

    query := `INSERT INTO audit_logs
              (user_id, action, resource_type, resource_id, ip_address, user_agent, changes)
              VALUES (?, ?, ?, ?, ?, ?, ?)`

    _, err := db.Exec(query,
        log.UserID, log.Action, log.ResourceType, log.ResourceID,
        log.IPAddress, log.UserAgent, string(changesJSON))

    return err
}

// 查詢審計日誌
func GetAuditLogs(userID string, limit int) ([]AuditLog, error) {
    // 實現查詢邏輯...
}
```

**API 集成：**
```go
// api/server.go

func handleDeleteTrader(c *gin.Context) {
    traderID := c.Param("id")
    userID := c.GetString("user_id")

    // 執行刪除
    if err := config.DeleteTrader(traderID); err != nil {
        c.JSON(500, gin.H{"error": "刪除失敗"})
        return
    }

    // 記錄審計日誌
    logger.LogAudit(&logger.AuditLog{
        UserID:       userID,
        Action:       "delete",
        ResourceType: "trader",
        ResourceID:   traderID,
        IPAddress:    c.ClientIP(),
        UserAgent:    c.Request.UserAgent(),
    })

    c.JSON(200, gin.H{"success": true})
}
```

**查詢 API：**
```go
// 新增端點
GET /api/audit-logs?limit=50&resource_type=trader
```

**驗收標準：**
- [ ] 記錄所有敏感操作
- [ ] IP 和 User-Agent 記錄
- [ ] 變更內容 JSON 存儲
- [ ] 查詢 API 實現
- [ ] 性能影響 < 5ms

**工作量：** 24 小時

---

### 1.3 增強密碼策略（1 天）

**後端驗證：**
```go
// auth/password.go

import "unicode"

type PasswordPolicy struct {
    MinLength      int
    RequireUpper   bool
    RequireLower   bool
    RequireNumber  bool
    RequireSpecial bool
}

var DefaultPolicy = PasswordPolicy{
    MinLength:      12,
    RequireUpper:   true,
    RequireLower:   true,
    RequireNumber:  true,
    RequireSpecial: true,
}

func ValidatePassword(password string, policy PasswordPolicy) error {
    if len(password) < policy.MinLength {
        return fmt.Errorf("密碼至少需要 %d 字符", policy.MinLength)
    }

    var hasUpper, hasLower, hasNumber, hasSpecial bool

    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsDigit(char):
            hasNumber = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSpecial = true
        }
    }

    if policy.RequireUpper && !hasUpper {
        return errors.New("密碼必須包含大寫字母")
    }
    if policy.RequireLower && !hasLower {
        return errors.New("密碼必須包含小寫字母")
    }
    if policy.RequireNumber && !hasNumber {
        return errors.New("密碼必須包含數字")
    }
    if policy.RequireSpecial && !hasSpecial {
        return errors.New("密碼必須包含特殊字符")
    }

    return nil
}

// 檢查常見密碼
var commonPasswords = []string{
    "password", "123456", "qwerty", "admin", "letmein",
    // 從常見密碼列表載入...
}

func IsCommonPassword(password string) bool {
    lower := strings.ToLower(password)
    for _, common := range commonPasswords {
        if lower == common {
            return true
        }
    }
    return false
}
```

**前端驗證：**
```typescript
// web/src/lib/passwordValidator.ts

export interface PasswordStrength {
  score: number;  // 0-4
  feedback: string[];
  isValid: boolean;
}

export function validatePassword(password: string): PasswordStrength {
  const feedback: string[] = [];
  let score = 0;

  // 長度檢查
  if (password.length < 12) {
    feedback.push('密碼至少需要 12 字符');
  } else {
    score++;
  }

  // 複雜度檢查
  if (!/[A-Z]/.test(password)) {
    feedback.push('需要至少一個大寫字母');
  } else {
    score++;
  }

  if (!/[a-z]/.test(password)) {
    feedback.push('需要至少一個小寫字母');
  } else {
    score++;
  }

  if (!/[0-9]/.test(password)) {
    feedback.push('需要至少一個數字');
  } else {
    score++;
  }

  if (!/[^A-Za-z0-9]/.test(password)) {
    feedback.push('需要至少一個特殊字符');
  } else {
    score++;
  }

  return {
    score: Math.min(score, 4),
    feedback,
    isValid: score >= 4 && password.length >= 12
  };
}
```

**UI 強度指示器：**
```tsx
// web/src/components/PasswordStrengthIndicator.tsx

const PasswordStrengthIndicator = ({ password }: { password: string }) => {
  const strength = validatePassword(password);

  const colors = ['red', 'orange', 'yellow', 'lightgreen', 'green'];
  const labels = ['非常弱', '弱', '一般', '強', '非常強'];

  return (
    <div>
      <div className="flex gap-1">
        {[0, 1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className={`h-2 flex-1 rounded ${
              i <= strength.score ? `bg-${colors[strength.score]}` : 'bg-gray-300'
            }`}
          />
        ))}
      </div>
      <p className="text-sm mt-1">{labels[strength.score]}</p>
      {strength.feedback.length > 0 && (
        <ul className="text-xs text-red-500 mt-2">
          {strength.feedback.map((msg, i) => (
            <li key={i}>• {msg}</li>
          ))}
        </ul>
      )}
    </div>
  );
};
```

**驗收標準：**
- [ ] 前後端雙重驗證
- [ ] 即時強度指示器
- [ ] 拒絕常見密碼
- [ ] 清晰的錯誤提示

**工作量：** 8 小時

---

### 階段 1 驗收標準

- [ ] 所有憑證 AES-256 加密
- [ ] 審計日誌完整記錄
- [ ] 密碼策略強制執行
- [ ] 安全掃描無高危漏洞
- [ ] 性能測試通過

### 發布產出

**版本：** v4.0.0（安全加固版）

**變更日誌：**
```markdown
## [4.0.0] - 2025-XX-XX

### 🔒 安全增強
- 實現 AES-256-GCM 憑證加密存儲
- 添加完整審計日誌系統
- 增強密碼策略（最少 12 字符 + 複雜度要求）
- 數據庫自動遷移腳本

### 🆕 新功能
- 審計日誌查詢 API
- 密碼強度即時指示器
- 加密密鑰管理工具

### ⚠️ 破壞性變更
- 必須設置 `ENCRYPTION_KEY` 環境變量
- 密碼要求從 6 字符提升至 12 字符
- 需要運行數據庫遷移腳本
```

---

## 階段 2：穩定性提升（4-6 週）

### 🎯 目標
**建立完整的測試體系和監控系統**

### 2.1 測試框架建設（3 週）

#### 任務 2.1.1：單元測試（2 週）

**目標覆蓋率：** 60%

**優先測試模塊：**

1. **認證模塊** (`auth/`)
```go
// auth/auth_test.go

func TestGenerateJWT(t *testing.T) {
    user := &User{ID: "test123", Email: "test@example.com"}
    token, err := GenerateJWT(user, "test-secret")

    assert.NoError(t, err)
    assert.NotEmpty(t, token)

    // 驗證 token
    claims, err := ValidateJWT(token, "test-secret")
    assert.NoError(t, err)
    assert.Equal(t, "test123", claims.UserID)
}

func TestPasswordHashing(t *testing.T) {
    password := "MySecureP@ssw0rd123"

    hash, err := HashPassword(password)
    assert.NoError(t, err)

    // 驗證正確密碼
    assert.True(t, CheckPassword(password, hash))

    // 驗證錯誤密碼
    assert.False(t, CheckPassword("WrongPassword", hash))
}

func TestTOTPGeneration(t *testing.T) {
    secret, qrURL, err := GenerateTOTP("test@example.com")

    assert.NoError(t, err)
    assert.NotEmpty(t, secret)
    assert.Contains(t, qrURL, "otpauth://")
}
```

2. **加密模塊** (`crypto/`)
```go
// crypto/aes_test.go

func TestEncryptDecrypt(t *testing.T) {
    // 設置測試密鑰
    os.Setenv("ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
    InitEncryption()

    plaintext := "sk-1234567890abcdefghijklmnopqrstuvwxyz"

    encrypted, err := Encrypt(plaintext)
    assert.NoError(t, err)
    assert.NotEqual(t, plaintext, encrypted)

    decrypted, err := Decrypt(encrypted)
    assert.NoError(t, err)
    assert.Equal(t, plaintext, decrypted)
}

func TestEncryptionIdempotence(t *testing.T) {
    plaintext := "test-secret"

    encrypted1, _ := Encrypt(plaintext)
    encrypted2, _ := Encrypt(plaintext)

    // 每次加密結果應不同（因為隨機 nonce）
    assert.NotEqual(t, encrypted1, encrypted2)

    // 但解密結果應相同
    decrypted1, _ := Decrypt(encrypted1)
    decrypted2, _ := Decrypt(encrypted2)
    assert.Equal(t, decrypted1, decrypted2)
}
```

3. **交易邏輯** (`trader/`)
```go
// trader/auto_trader_test.go

func TestRiskControl(t *testing.T) {
    trader := &AutoTrader{
        InitialBalance: 1000.0,
        Config: TraderConfig{
            BTCETHLeverage:  5,
            AltcoinLeverage: 5,
        },
    }

    // 測試槓桿限制
    decision := &Decision{
        Symbol:   "BTCUSDT",
        Leverage: 10,
    }

    err := trader.ValidateDecision(decision)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "槓桿超出限制")
}

func TestPositionSizeLimit(t *testing.T) {
    // 測試倉位大小限制...
}
```

**CI/CD 集成：**
```yaml
# .github/workflows/test.yml

name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25'

      - name: Install dependencies
        run: go mod download

      - name: Run tests
        run: go test -v -cover -coverprofile=coverage.out ./...

      - name: Coverage report
        run: go tool cover -func=coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

**驗收標準：**
- [ ] 核心模塊覆蓋率 ≥ 60%
- [ ] 所有測試通過
- [ ] CI 自動運行
- [ ] 覆蓋率報告可視化

**工作量：** 80 小時

---

#### 任務 2.1.2：集成測試（1 週）

**測試範圍：**

1. **數據庫集成測試**
```go
// config/database_integration_test.go

func TestDatabaseCRUD(t *testing.T) {
    // 使用臨時數據庫
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    // 測試創建
    trader := &Trader{
        ID:     "test-trader",
        UserID: "test-user",
        Name:   "Test Trader",
    }
    err := SaveTrader(db, trader)
    assert.NoError(t, err)

    // 測試讀取
    loaded, err := GetTrader(db, "test-trader")
    assert.NoError(t, err)
    assert.Equal(t, trader.Name, loaded.Name)

    // 測試更新
    trader.Name = "Updated Name"
    err = UpdateTrader(db, trader)
    assert.NoError(t, err)

    // 測試刪除
    err = DeleteTrader(db, "test-trader")
    assert.NoError(t, err)
}
```

2. **API 端點測試**
```go
// api/server_integration_test.go

func TestAPIEndpoints(t *testing.T) {
    router := setupTestRouter(t)

    // 測試註冊
    t.Run("Register", func(t *testing.T) {
        body := `{"email":"test@example.com","password":"SecureP@ssw0rd123"}`
        req := httptest.NewRequest("POST", "/api/register", strings.NewReader(body))
        w := httptest.NewRecorder()

        router.ServeHTTP(w, req)

        assert.Equal(t, 200, w.Code)
        // 驗證響應...
    })

    // 測試登錄
    t.Run("Login", func(t *testing.T) {
        // 測試邏輯...
    })
}
```

3. **交易所集成測試**（使用 mock）
```go
// trader/binance_integration_test.go

type MockBinanceClient struct {
    mock.Mock
}

func (m *MockBinanceClient) GetBalance() (map[string]interface{}, error) {
    args := m.Called()
    return args.Get(0).(map[string]interface{}), args.Error(1)
}

func TestBinanceTr的ader(t *testing.T) {
    mockClient := new(MockBinanceClient)
    mockClient.On("GetBalance").Return(map[string]interface{}{
        "totalBalance": 1000.0,
    }, nil)

    trader := &BinanceTrader{client: mockClient}
    balance, err := trader.GetBalance()

    assert.NoError(t, err)
    assert.Equal(t, 1000.0, balance["totalBalance"])
    mockClient.AssertExpectations(t)
}
```

**驗收標準：**
- [ ] API 端點全覆蓋
- [ ] 數據庫操作測試
- [ ] 交易所接口 mock 測試
- [ ] 所有測試通過

**工作量：** 40 小時

---

### 2.2 監控系統（2 週）

#### 任務 2.2.1：Prometheus 集成（1 週）

**依賴安裝：**
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
```

**指標定義：**
```go
// monitoring/metrics.go

package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // HTTP 請求指標
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nofx_http_requests_total",
            Help: "HTTP 請求總數",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "nofx_http_request_duration_seconds",
            Help:    "HTTP 請求延遲",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    // 交易指標
    tradesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nofx_trades_total",
            Help: "交易總數",
        },
        []string{"trader_id", "symbol", "side", "result"},
    )

    tradeProfit = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nofx_trade_profit_usdt",
            Help: "交易盈虧（USDT）",
        },
        []string{"trader_id"},
    )

    // AI 決策指標
    aiDecisionsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nofx_ai_decisions_total",
            Help: "AI 決策總數",
        },
        []string{"trader_id", "ai_model", "action"},
    )

    aiResponseTime = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "nofx_ai_response_time_seconds",
            Help:    "AI API 響應時間",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"ai_model"},
    )

    // 系統指標
    activeTraders = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "nofx_active_traders",
            Help: "活躍交易員數量",
        },
    )

    databaseQueries = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nofx_database_queries_total",
            Help: "數據庫查詢總數",
        },
        []string{"operation", "table"},
    )
)

// 記錄 HTTP 請求
func RecordHTTPRequest(method, path string, status int, duration float64) {
    httpRequestsTotal.WithLabelValues(method, path, fmt.Sprintf("%d", status)).Inc()
    httpRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// 記錄交易
func RecordTrade(traderID, symbol, side, result string, profit float64) {
    tradesTotal.WithLabelValues(traderID, symbol, side, result).Inc()
    tradeProfit.WithLabelValues(traderID).Set(profit)
}

// 記錄 AI 決策
func RecordAIDecision(traderID, aiModel, action string, responseTime float64) {
    aiDecisionsTotal.WithLabelValues(traderID, aiModel, action).Inc()
    aiResponseTime.WithLabelValues(aiModel).Observe(responseTime)
}
```

**Gin 中間件：**
```go
// api/middleware/metrics.go

func MetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()

        c.Next()

        duration := time.Since(start).Seconds()
        monitoring.RecordHTTPRequest(
            c.Request.Method,
            c.FullPath(),
            c.Writer.Status(),
            duration,
        )
    }
}

// 應用到路由器
router.Use(MetricsMiddleware())

// 暴露 metrics 端點
router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

**docker-compose 配置：**
```yaml
# docker-compose.yml

services:
  nofx:
    # ... existing config
    ports:
      - "8080:8080"
      - "9090:9090"  # Prometheus metrics

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

volumes:
  prometheus_data:
```

**Prometheus 配置：**
```yaml
# prometheus.yml

global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'nofx'
    static_configs:
      - targets: ['nofx:9090']
```

**驗收標準：**
- [ ] 所有關鍵指標已記錄
- [ ] Prometheus 正常抓取
- [ ] 指標可在 /metrics 查詢
- [ ] Docker 部署測試通過

**工作量：** 40 小時

---

#### 任務 2.2.2：Grafana 儀表板（1 週）

**docker-compose 添加：**
```yaml
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources

volumes:
  grafana_data:
```

**自動配置數據源：**
```yaml
# grafana/datasources/prometheus.yml

apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

**儀表板配置：**

創建 3 個核心儀表板：

1. **系統概覽儀表板**
   - HTTP 請求率（QPS）
   - HTTP 請求延遲（P50/P90/P99）
   - 錯誤率
   - 活躍交易員數量
   - 數據庫查詢率

2. **交易監控儀表板**
   - 每小時交易量
   - 勝率趨勢
   - 累計盈虧
   - 各交易員性能對比
   - 各幣種交易分佈

3. **AI 性能儀表板**
   - AI API 響應時間
   - 決策類型分佈
   - AI 調用成功率
   - 各模型性能對比

**示例儀表板 JSON：**
```json
// grafana/dashboards/system-overview.json
{
  "dashboard": {
    "title": "NOFX 系統概覽",
    "panels": [
      {
        "title": "HTTP 請求率",
        "targets": [
          {
            "expr": "rate(nofx_http_requests_total[5m])"
          }
        ],
        "type": "graph"
      },
      {
        "title": "HTTP 請求延遲（P90）",
        "targets": [
          {
            "expr": "histogram_quantile(0.9, rate(nofx_http_request_duration_seconds_bucket[5m]))"
          }
        ],
        "type": "graph"
      }
      // ... 更多面板
    ]
  }
}
```

**驗收標準：**
- [ ] 3 個儀表板配置完成
- [ ] 自動配置數據源
- [ ] 所有面板數據正常顯示
- [ ] 文檔說明如何自定義

**工作量：** 40 小時

---

### 2.3 結構化日誌（3 天）

**實現：**
```go
// logger/structured.go

package logger

import (
    "os"
    "github.com/sirupsen/logrus"
)

var log = logrus.New()

func Init() {
    // JSON 格式
    log.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: "2006-01-02 15:04:05",
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "level",
            logrus.FieldKeyMsg:   "message",
        },
    })

    // 輸出到標準輸出
    log.SetOutput(os.Stdout)

    // 日誌級別
    level := os.Getenv("LOG_LEVEL")
    if level == "" {
        level = "info"
    }

    logLevel, _ := logrus.ParseLevel(level)
    log.SetLevel(logLevel)
}

// 結構化日誌方法
func Info(msg string, fields map[string]interface{}) {
    log.WithFields(fields).Info(msg)
}

func Error(msg string, err error, fields map[string]interface{}) {
    if fields == nil {
        fields = make(map[string]interface{})
    }
    if err != nil {
        fields["error"] = err.Error()
    }
    log.WithFields(fields).Error(msg)
}

func Warn(msg string, fields map[string]interface{}) {
    log.WithFields(fields).Warn(msg)
}

func Debug(msg string, fields map[string]interface{}) {
    log.WithFields(fields).Debug(msg)
}
```

**使用示例：**
```go
// trader/auto_trader.go

logger.Info("開始交易決策週期", map[string]interface{}{
    "trader_id":    trader.ID,
    "cycle_number": cycleCount,
    "balance":      account.TotalBalance,
})

logger.Error("AI 決策失敗", err, map[string]interface{}{
    "trader_id": trader.ID,
    "ai_model":  trader.AIModel,
    "attempt":   retryCount,
})
```

**ELK Stack 集成（可選）：**
```yaml
# docker-compose.yml

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.8.0
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    volumes:
      - es_data:/usr/share/elasticsearch/data

  kibana:
    image: docker.elastic.co/kibana/kibana:8.8.0
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch

volumes:
  es_data:
```

**驗收標準：**
- [ ] 所有日誌 JSON 格式
- [ ] 包含上下文字段
- [ ] 可配置日誌級別
- [ ] ELK 集成（可選）

**工作量：** 24 小時

---

### 階段 2 驗收標準

- [ ] 測試覆蓋率 ≥ 60%
- [ ] CI/CD 流程完整
- [ ] Prometheus + Grafana 運行正常
- [ ] 3 個核心儀表板配置
- [ ] 結構化日誌實現
- [ ] 文檔完整

### 發布產出

**版本：** v5.0.0（穩定增強版）

**變更日誌：**
```markdown
## [5.0.0] - 2025-XX-XX

### 🧪 測試與質量
- 建立完整單元測試框架（覆蓋率 60%）
- 添加集成測試套件
- CI/CD 自動化測試

### 📊 監控與可觀測性
- Prometheus 指標集成
- Grafana 儀表板（系統、交易、AI）
- 結構化 JSON 日誌
- ELK Stack 支持（可選）

### 🐛 修復
- 提升系統穩定性
- 修復測試發現的邊緣情況
```

---

## 階段 3：性能優化（3-4 週）

### 🎯 目標
**實現 WebSocket 即時通信，優化系統性能**

### 3.1 WebSocket 實現（2 週）

#### 任務 3.1.1：後端 WebSocket 服務器（1 週）

**依賴：**
```go
// 已有 gorilla/websocket
```

**WebSocket Hub 實現：**
```go
// websocket/hub.go

package websocket

import (
    "encoding/json"
    "sync"
)

type Hub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
    mu         sync.RWMutex
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[*Client]bool),
        broadcast:  make(chan []byte, 256),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()

        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
            h.mu.Unlock()

        case message := <-h.broadcast:
            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
            h.mu.RUnlock()
        }
    }
}

// 廣播消息
func (h *Hub) Broadcast(messageType string, data interface{}) {
    message := map[string]interface{}{
        "type":      messageType,
        "data":      data,
        "timestamp": time.Now().Unix(),
    }

    jsonData, _ := json.Marshal(message)
    h.broadcast <- jsonData
}

// 按交易員 ID 過濾廣播
func (h *Hub) BroadcastToTrader(traderID string, messageType string, data interface{}) {
    message := map[string]interface{}{
        "type":      messageType,
        "trader_id": traderID,
        "data":      data,
        "timestamp": time.Now().Unix(),
    }

    jsonData, _ := json.Marshal(message)

    h.mu.RLock()
    defer h.mu.RUnlock()

    for client := range h.clients {
        if client.traderID == traderID || client.traderID == "" {
            select {
            case client.send <- jsonData:
            default:
                close(client.send)
                delete(h.clients, client)
            }
        }
    }
}
```

**Client 實現：**
```go
// websocket/client.go

type Client struct {
    hub      *Hub
    conn     *websocket.Conn
    send     chan []byte
    userID   string
    traderID string  // 可選，訂閱特定交易員
}

func (c *Client) ReadPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }

        // 處理客戶端消息（如訂閱特定交易員）
        c.handleMessage(message)
    }
}

func (c *Client) WritePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }

        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}

func (c *Client) handleMessage(message []byte) {
    var msg map[string]interface{}
    if err := json.Unmarshal(message, &msg); err != nil {
        return
    }

    switch msg["type"] {
    case "subscribe":
        c.traderID = msg["trader_id"].(string)
    case "unsubscribe":
        c.traderID = ""
    }
}
```

**API 端點：**
```go
// api/websocket.go

var wsHub = websocket.NewHub()

func init() {
    go wsHub.Run()
}

func handleWebSocket(c *gin.Context) {
    userID := c.GetString("user_id")

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }

    client := &websocket.Client{
        hub:    wsHub,
        conn:   conn,
        send:   make(chan []byte, 256),
        userID: userID,
    }

    client.hub.register <- client

    go client.WritePump()
    go client.ReadPump()
}

// 註冊路由
router.GET("/ws", AuthMiddleware(), handleWebSocket)
```

**集成到交易員：**
```go
// trader/auto_trader.go

func (at *AutoTrader) notifyUpdate(updateType string, data interface{}) {
    wsHub.BroadcastToTrader(at.ID, updateType, data)
}

// 在關鍵點調用
func (at *AutoTrader) runCycle() {
    // ... 決策邏輯

    // 通知賬戶更新
    at.notifyUpdate("account_update", account)

    // 通知倉位更新
    at.notifyUpdate("positions_update", positions)

    // 通知新決策
    at.notifyUpdate("decision_update", decision)
}
```

**驗收標準：**
- [ ] WebSocket 連接穩定
- [ ] 支持訂閱/取消訂閱
- [ ] 即時推送賬戶/倉位/決策更新
- [ ] 心跳機制正常
- [ ] 斷線自動重連

**工作量：** 40 小時

---

#### 任務 3.1.2：前端 WebSocket 客戶端（1 週）

**WebSocket Hook：**
```typescript
// web/src/hooks/useWebSocket.ts

import { useEffect, useRef, useState } from 'react';

interface WebSocketMessage {
  type: string;
  trader_id?: string;
  data: any;
  timestamp: number;
}

export function useWebSocket(traderId?: string) {
  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<NodeJS.Timeout>();

  const connect = () => {
    const token = localStorage.getItem('auth_token');
    if (!token) return;

    const wsUrl = `ws://localhost:8080/ws?token=${token}`;
    ws.current = new WebSocket(wsUrl);

    ws.current.onopen = () => {
      setIsConnected(true);
      console.log('WebSocket 已連接');

      // 訂閱特定交易員
      if (traderId) {
        ws.current?.send(JSON.stringify({
          type: 'subscribe',
          trader_id: traderId
        }));
      }
    };

    ws.current.onmessage = (event) => {
      const message: WebSocketMessage = JSON.parse(event.data);
      setLastMessage(message);
    };

    ws.current.onclose = () => {
      setIsConnected(false);
      console.log('WebSocket 已斷開，3 秒後重連...');

      // 3 秒後自動重連
      reconnectTimer.current = setTimeout(() => {
        connect();
      }, 3000);
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket 錯誤:', error);
    };
  };

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      ws.current?.close();
    };
  }, [traderId]);

  return { isConnected, lastMessage };
}
```

**在組件中使用：**
```typescript
// web/src/App.tsx

function TraderDetailsPage() {
  const [account, setAccount] = useState<AccountInfo | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);

  const { isConnected, lastMessage } = useWebSocket(selectedTraderId);

  // 處理 WebSocket 消息
  useEffect(() => {
    if (!lastMessage) return;

    switch (lastMessage.type) {
      case 'account_update':
        setAccount(lastMessage.data);
        break;
      case 'positions_update':
        setPositions(lastMessage.data);
        break;
      case 'decision_update':
        // 添加到決策日誌...
        break;
    }
  }, [lastMessage]);

  return (
    <div>
      <div className="connection-status">
        {isConnected ? '🟢 即時連接' : '🔴 已斷開'}
      </div>

      {/* 其他組件... */}
    </div>
  );
}
```

**Fallback 到輪詢：**
```typescript
// 如果 WebSocket 不可用，自動降級到輪詢
function useRealtimeData(traderId: string) {
  const { isConnected, lastMessage } = useWebSocket(traderId);

  // WebSocket 數據
  const [wsData, setWsData] = useState<any>(null);

  // SWR 輪詢（僅在 WebSocket 斷開時啟用）
  const { data: pollingData } = useSWR(
    !isConnected && traderId ? `account-${traderId}` : null,
    () => api.getAccount(traderId),
    { refreshInterval: 15000 }
  );

  // 優先使用 WebSocket 數據
  return isConnected ? wsData : pollingData;
}
```

**驗收標準：**
- [ ] WebSocket 連接穩定
- [ ] 即時數據顯示
- [ ] 斷線自動重連
- [ ] Fallback 到輪詢
- [ ] 連接狀態指示

**工作量：** 40 小時

---

### 3.2 數據庫優化（1 週）

#### 索引優化：
```sql
-- 添加索引
CREATE INDEX idx_traders_user_id ON traders(user_id);
CREATE INDEX idx_traders_is_running ON traders(is_running);
CREATE INDEX idx_ai_models_user_id ON ai_models(user_id);
CREATE INDEX idx_exchanges_user_id ON exchanges(user_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
```

#### 查詢優化：
```go
// 使用預處理語句
stmt, err := db.Prepare("SELECT * FROM traders WHERE user_id = ? AND is_running = ?")
defer stmt.Close()

// 批量查詢
rows, err := db.Query(`
    SELECT t.*, am.name as ai_model_name, e.name as exchange_name
    FROM traders t
    LEFT JOIN ai_models am ON t.ai_model_id = am.id
    LEFT JOIN exchanges e ON t.exchange_id = e.id
    WHERE t.user_id = ?
`, userID)
```

#### 連接池配置：
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

**驗收標準：**
- [ ] 所有索引創建
- [ ] 查詢性能提升 50%+
- [ ] 連接池優化
- [ ] 慢查詢日誌分析

**工作量：** 40 小時

---

### 3.3 前端性能優化（1 週）

#### React 優化：
```typescript
// 使用 React.memo 防止不必要的重渲染
export const EquityChart = React.memo(({ data }: Props) => {
  // 組件邏輯...
});

// 使用 useMemo 緩存計算結果
const filteredData = useMemo(() => {
  return equityHistory.filter(point => point.equity > 1);
}, [equityHistory]);

// 使用 useCallback 緩存回調函數
const handleTraderSelect = useCallback((traderId: string) => {
  setSelectedTraderId(traderId);
}, []);
```

#### Code Splitting：
```typescript
// 懶加載路由組件
const CompetitionPage = lazy(() => import('./components/CompetitionPage'));
const AITradersPage = lazy(() => import('./components/AITradersPage'));

<Suspense fallback={<Loading />}>
  <CompetitionPage />
</Suspense>
```

#### 圖表優化：
```typescript
// 限制數據點數量
const chartData = useMemo(() => {
  if (equityHistory.length > 2000) {
    // 每 N 個點取樣
    const step = Math.ceil(equityHistory.length / 2000);
    return equityHistory.filter((_, i) => i % step === 0);
  }
  return equityHistory;
}, [equityHistory]);
```

**驗收標準：**
- [ ] 首屏加載時間 < 2 秒
- [ ] Code Splitting 實現
- [ ] React 優化應用
- [ ] Lighthouse 評分 > 90

**工作量：** 40 小時

---

### 階段 3 驗收標準

- [ ] WebSocket 全面替代輪詢
- [ ] 即時更新延遲 < 1 秒
- [ ] 數據庫查詢性能提升 50%
- [ ] 前端首屏加載 < 2 秒
- [ ] Lighthouse 評分 > 90

### 發布產出

**版本：** v5.5.0（性能優化版）

---

## 階段 4：功能擴展（8-12 週）

### 🎯 目標
**擴展交易所、AI 模型、告警系統**

### 4.1 新交易所整合（6 週）

#### OKX（2 週）
#### Bybit（2 週）
#### Bitget（2 週）

每個交易所實現：
- [ ] Trader 接口實現
- [ ] API 客戶端封裝
- [ ] 精度處理
- [ ] 單元測試
- [ ] 集成測試
- [ ] 文檔

**工作量：** 240 小時

---

### 4.2 AI 模型擴展（4 週）

#### GPT-4 Integration（1 週）
#### Claude 3 Integration（1 週）
#### Gemini Pro Integration（1 週）
#### 多模型集成投票（1 週）

**工作量：** 160 小時

---

### 4.3 告警通知系統（2 週）

#### Telegram Bot（1 週）
#### Email 通知（3 天）
#### Webhook（2 天）
#### 告警規則配置（2 天）

**工作量：** 80 小時

---

### 階段 4 驗收標準

- [ ] 3 個新交易所上線
- [ ] 3 個新 AI 模型支持
- [ ] 告警系統完整運行
- [ ] 文檔完整

### 發布產出

**版本：** v6.0.0（功能擴展版）

---

## 階段 5：企業級準備（持續）

### 5.1 高可用部署
- [ ] PostgreSQL 遷移
- [ ] Redis 緩存
- [ ] Kubernetes 部署
- [ ] 負載均衡

### 5.2 安全加固
- [ ] API 密鑰輪換
- [ ] RBAC 完整實現
- [ ] 滲透測試
- [ ] 安全審計

### 5.3 文檔完善
- [ ] API 文檔
- [ ] 用戶手冊
- [ ] 運維手冊
- [ ] 視頻教程

---

## 資源需求

### 人力資源

| 角色 | 人數 | 階段 | 工作量 |
|------|------|------|--------|
| 後端開發 | 2 | 全部 | 全職 |
| 前端開發 | 1 | 全部 | 全職 |
| 測試工程師 | 1 | 階段 2+ | 兼職 |
| DevOps | 1 | 階段 2+ | 兼職 |
| 安全專家 | 1 | 階段 1 | 顧問 |

### 基礎設施

| 資源 | 用途 | 成本（月） |
|------|------|------------|
| 開發服務器 | CI/CD, 測試 | $50 |
| 監控服務 | Prometheus + Grafana | $30 |
| 數據庫備份 | S3/OSS | $10 |
| 域名/SSL | HTTPS | $5 |
| **總計** | | **$95/月** |

---

## 風險評估

### 技術風險

| 風險 | 可能性 | 影響 | 緩解措施 |
|------|--------|------|----------|
| 加密遷移失敗 | 中 | 高 | 完整備份 + 回滾計劃 |
| WebSocket 穩定性 | 中 | 中 | Fallback 到輪詢 |
| 測試覆蓋不足 | 高 | 中 | 階段性目標 |
| 性能下降 | 低 | 中 | 性能基準測試 |

### 業務風險

| 風險 | 可能性 | 影響 | 緩解措施 |
|------|--------|------|----------|
| 資金不足 | 中 | 高 | 分階段執行 |
| 人員流失 | 低 | 高 | 文檔完善 |
| 進度延遲 | 中 | 中 | 彈性時間緩衝 |

---

## 成功指標

### 階段 0（1 週）
- [ ] 安全掃描無嚴重漏洞
- [ ] 系統評分 7.0/10

### 階段 1（2-3 週）
- [ ] 所有憑證已加密
- [ ] 審計日誌運行
- [ ] 系統評分 7.5/10

### 階段 2（4-6 週）
- [ ] 測試覆蓋率 ≥ 60%
- [ ] 監控系統上線
- [ ] 系統評分 8.0/10

### 階段 3（3-4 週）
- [ ] WebSocket 替代輪詢
- [ ] 首屏加載 < 2 秒
- [ ] 系統評分 8.5/10

### 階段 4（8-12 週）
- [ ] 6 個交易所支持
- [ ] 5 個 AI 模型
- [ ] 告警系統運行
- [ ] 系統評分 9.0/10

### 階段 5（持續）
- [ ] 高可用部署
- [ ] 企業級安全
- [ ] 系統評分 9.5/10

---

## 附錄：快速參考

### 環境變量清單
```bash
# 必需
JWT_SECRET=<32字節隨機密鑰>
ENCRYPTION_KEY=<32字節base64編碼>

# 可選
ALLOWED_ORIGINS=http://localhost:3000
LOG_LEVEL=info
DATABASE_PATH=./config.db
```

### 常用命令
```bash
# 生成 JWT 密鑰
openssl rand -base64 32

# 生成加密密鑰
openssl rand -base64 32

# 運行測試
go test -v -cover ./...

# 數據庫遷移
go run scripts/migrate_encrypt_credentials.go

# 啟動監控
docker-compose up -d prometheus grafana
```

---

**計劃制定日期**: 2025-11-06
**最後更新**: 2025-11-06
**版本**: 1.0

*此計劃為動態文檔，將根據實際執行情況調整。*
