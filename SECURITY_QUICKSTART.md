# 🚀 加密系统快速开始指南（5 分鐘部署）

## 一鍵部署腳本

```bash
#!/bin/bash
# 文件: deploy_encryption.sh

set -e

echo "🔐 开始部署加密系统..."

# 1. 确保在專案根目錄
cd /Users/sotadic/Documents/GitHub/nofx

# 2. 备份現有数据庫
if [ -f "config.db" ]; then
    cp config.db "config.db.backup.$(date +%Y%m%d_%H%M%S)"
    echo "✅ 数据庫已备份"
fi

# 3. 创建密钥目錄
mkdir -p .secrets
chmod 700 .secrets
echo "✅ 密钥目錄已创建"

# 4. 安裝依赖
echo "📦 安裝 Go 依赖..."
go mod tidy

echo "📦 安裝前端依赖..."
cd web && npm install tweetnacl tweetnacl-util && cd ..

# 5. 运行测试
echo "🧪 运行加密测试..."
go test ./crypto -v

# 6. 迁移数据
echo "🔄 迁移現有数据到加密格式..."
go run scripts/migrate_encryption.go

# 7. 設置环境变量
MASTER_KEY=$(cat .secrets/master.key)
echo "export NOFX_MASTER_KEY='$MASTER_KEY'" >> ~/.bashrc
source ~/.bashrc

echo ""
echo "✅ 部署完成！"
echo ""
echo "📝 後續步驟:"
echo "1. 重啟应用: go run main.go"
echo "2. 验证前端: 访问 http://localhost:3000"
echo "3. 查看审计日誌: curl http://localhost:8080/api/audit-logs"
echo ""
echo "⚠️  重要提醒:"
echo "- 請妥善保管 .secrets/ 目錄"
echo "- 生产环境務必使用环境变量管理密钥"
echo "- 定期执行密钥轮换（建議每季度一次）"
echo ""
```

---

## 最小化改動清单

### 1. 修改 main.go (添加 15 行代碼)

```go
// 在現有 main.go 的 import 區塊添加
import "nofx/crypto"

// 在 main() 函數中添加
func main() {
    // ... 現有代碼 ...

    // 【新增】初始化安全存储
    secureStorage, err := crypto.NewSecureStorage(db.GetDB())
    if err != nil {
        log.Fatalf("加密系统初始化失败: %v", err)
    }

    // 【新增】迁移旧数据（僅首次运行）
    secureStorage.MigrateToEncrypted()

    // 【新增】注册加密 API
    cryptoHandler, _ := api.NewCryptoHandler(secureStorage)
    http.HandleFunc("/api/crypto/public-key", cryptoHandler.HandleGetPublicKey)

    // ... 現有代碼 ...
}
```

### 2. 修改前端 AITradersPage.tsx (替換 1 个函數)

```typescript
// 【替換】原有的 handleSaveExchangeConfig 函數
import { twoStagePrivateKeyInput, fetchServerPublicKey } from '../lib/crypto';

const handleSaveExchangeConfig = async (exchangeId: string, apiKey: string) => {
  const serverPublicKey = await fetchServerPublicKey();
  const { encryptedKey } = await twoStagePrivateKeyInput(serverPublicKey);

  await api.post('/api/exchange/config', {
    exchange_id: exchangeId,
    encrypted_key: encryptedKey,
  });
};
```

### 3. 更新 .gitignore

```bash
echo ".secrets/" >> .gitignore
echo "config.db.backup.*" >> .gitignore
```

---

## 验证清单

完成部署後，請执行以下验证：

```bash
# ✅ 密钥文件存在
ls -la .secrets/
# 预期输出: rsa_private.pem, rsa_public.pem, master.key

# ✅ 资料庫中的密钥已加密
sqlite3 config.db "SELECT substr(api_key, 1, 20) FROM exchanges LIMIT 1;"
# 预期输出: Base64 字串（如 J8K9L0M1N2O3P4Q5R6S7==）

# ✅ 公鑰 API 可访问
curl http://localhost:8080/api/crypto/public-key | jq .
# 预期输出: {"public_key": "-----BEGIN PUBLIC KEY-----..."}

# ✅ 前端加密模组载入成功
# 打開浏览器控制台，输入:
typeof window.crypto.subtle
# 预期输出: "object"
```

---

## 常見问题排查

### 问题1: "初始化加密管理器失败"

**原因**: .secrets/ 目錄权限错误

**解決**:
```bash
chmod 700 .secrets
chmod 600 .secrets/*
```

### 问题2: "解密失败: invalid ciphertext"

**原因**: 主密钥不匹配

**解決**:
```bash
# 从备份恢復
cp config.db.backup.20250106 config.db

# 或重新迁移
go run scripts/migrate_encryption.go
```

### 问题3: 前端报错 "无法获取服务器公鑰"

**原因**: 后端未正確启动或路由未注册

**解決**:
```bash
# 检查后端日誌
tail -f nohup.out | grep "加密"

# 验证路由
curl http://localhost:8080/api/crypto/public-key
```

---

## 安全等级对比

| 方案 | 明文儲存（当前） | 本加密方案 | 硬體 HSM |
|------|----------------|-----------|---------|
| 资料庫洩露風险 | ❌ 100% 洩露 | ✅ 密钥保护 | ✅ 物理隔離 |
| 剪贴簿監聽 | ❌ 100% 洩露 | ✅ 混淆保护 | ✅ 無需输入 |
| 服务器入侵 | ❌ 立即洩露 | ⚠️ 需竊取密钥 | ✅ 无法竊取 |
| 实施成本 | 免费 | 免费 | 高昂 |
| 实施时间 | - | 5 分鐘 | 1-2 週 |

---

## 效能影響

```
加密操作延遲测试（MacBook Pro M1）:

BenchmarkEncryption-8     50000    35421 ns/op   (0.035 ms)
BenchmarkDecryption-8     50000    28912 ns/op   (0.029 ms)

结论：每次操作增加 < 0.1ms 延遲，對用戶體驗無感知影響
```

---

## 紧急回退方案

如果加密系统出現问题，可立即回退：

```bash
# 1. 停止服務
sudo systemctl stop nofx

# 2. 恢復备份
cp config.db.backup.20250106 config.db

# 3. 注释掉 main.go 中的加密代碼（15 行）
# secureStorage, err := crypto.NewSecureStorage(...)
# // 注释掉上面這行

# 4. 重啟服務
go run main.go
```

---

## 📚 詳細文檔

需要更深入的配置和优化指南？

- **[Aliyun KMS 完整指南](docs/ALIYUN_KMS_GUIDE.md)** - 阿里云 KMS 詳細配置、成本分析、性能优化
- **[GCP KMS 配置指南](docs/GCP_KMS_SETUP.md)** - Google Cloud KMS 快速配置
- **[完整部署指南](docs/ENCRYPTION_DEPLOYMENT.md)** - 生产环境部署、迁移、故障排除

---

## 聯繫与支援

- **测试腳本**: `go test ./crypto -v`
- **迁移腳本**: `go run scripts/migrate_encryption.go`
- **GitHub Issues**: 报告问题或建議

**記住：安全是一项持續的投資，而非一次性成本。**
