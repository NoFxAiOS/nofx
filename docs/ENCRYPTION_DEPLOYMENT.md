# 🔐 加密系统部署指南

## 架構概述

```
┌─────────────────────────────────────────────────────────────────┐
│                   三层加密安全架構                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  前端 (Browser)                                                   │
│  ├─ 二階段输入（分段 + 剪贴簿混淆）                              │
│  ├─ RSA-4096 混合加密                                             │
│  └─ Base64 传输                                                   │
│                        ↓ HTTPS                                    │
│  后端 (Go Server)                                                 │
│  ├─ RSA 私鑰解密                                                  │
│  ├─ AES-256-GCM 数据庫加密                                        │
│  ├─ 密钥轮换机制                                                  │
│  └─ 审计日誌记录                                                  │
│                        ↓                                          │
│  数据庫 (SQLite)                                                  │
│  └─ 所有敏感字段加密存储                                          │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 第一步：初始化加密系统

### 1.1 生成密钥對

首次启动时，系统会自動生成：
- RSA-4096 密钥對（用于前后端通信加密）
- AES-256 主密钥（用于数据庫加密）

```bash
cd /Users/sotadic/Documents/GitHub/nofx

# 启动系统，自動生成密钥
go run main.go

# 密钥会保存在 .secrets/ 目錄
# ⚠️ 确保此目錄不会被 Git 追蹤
echo ".secrets/" >> .gitignore
```

**生成的文件**:
```
.secrets/
├── rsa_private.pem    # RSA 私鑰 (4096-bit)
├── rsa_public.pem     # RSA 公鑰
└── master.key         # 数据庫加密主密钥 (Base64)
```

### 1.2 設置环境变量（生产环境必须）

```bash
# 读取生成的主密钥
MASTER_KEY=$(cat .secrets/master.key)

# 添加到环境变量
export NOFX_MASTER_KEY="$MASTER_KEY"

# 或添加到 .env 文件（⚠️ 确保 .env 不会提交到 Git）
echo "NOFX_MASTER_KEY=$MASTER_KEY" >> .env
```

**⚠️ 生产环境安全建議**:
```bash
# 使用 systemd 服務管理环境变量
sudo nano /etc/systemd/system/nofx.service

[Service]
Environment="NOFX_MASTER_KEY=<your_key_here>"
EnvironmentFile=/opt/nofx/.env
```

---

## 第二步：迁移現有数据

如果你已有明文密钥数据，需要迁移到加密格式：

### 2.1 备份数据庫

```bash
# 备份原始数据庫
cp config.db config.db.backup.$(date +%Y%m%d_%H%M%S)

# 验证备份
sqlite3 config.db.backup.* "SELECT COUNT(*) FROM exchanges;"
```

### 2.2 执行迁移

```bash
# 方式 1: 使用 Go 程式迁移
go run scripts/migrate_encryption.go

# 方式 2: 使用 SQL 腳本
sqlite3 config.db < scripts/migrate_to_encrypted.sql
```

### 2.3 验证迁移結果

```bash
# 检查加密後的数据（應該看到 Base64 字串）
sqlite3 config.db "SELECT id, substr(api_key, 1, 20) FROM exchanges LIMIT 3;"

# 输出示例:
# binance|J8K9L0M1N2O3P4Q5R6S7==
# hyperliquid|X9Y8Z7A6B5C4D3E2F1G0==
```

---

## 第三步：更新 main.go

### 3.1 引入加密模组

在 `main.go` 中添加初始化代碼：

```go
package main

import (
    "log"
    "nofx/api"
    "nofx/config"
    "nofx/crypto"
    "net/http"
)

func main() {
    // 1. 初始化数据庫
    db, err := config.NewDatabase("config.db")
    if err != nil {
        log.Fatalf("数据庫初始化失败: %v", err)
    }
    defer db.Close()

    // 2. 初始化安全存储层
    secureStorage, err := crypto.NewSecureStorage(db.GetDB())
    if err != nil {
        log.Fatalf("安全存储初始化失败: %v", err)
    }

    // 3. 可選：迁移旧数据
    if err := secureStorage.MigrateToEncrypted(); err != nil {
        log.Printf("⚠️ 数据迁移失败（如果是首次运行可忽略）: %v", err)
    }

    // 4. 创建加密 API 处理器
    cryptoHandler, err := api.NewCryptoHandler(secureStorage)
    if err != nil {
        log.Fatalf("加密处理器初始化失败: %v", err)
    }

    // 5. 注册路由
    http.HandleFunc("/api/crypto/public-key", cryptoHandler.HandleGetPublicKey)
    http.HandleFunc("/api/crypto/decrypt", cryptoHandler.HandleDecryptPrivateKey)
    http.HandleFunc("/api/audit-logs", cryptoHandler.HandleGetAuditLogs)

    log.Println("🔐 加密系统已启用")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## 第四步：前端集成

### 4.1 更新 AITradersPage.tsx

```typescript
import { twoStagePrivateKeyInput, fetchServerPublicKey } from '../lib/crypto';

// 替換原有的私鑰输入邏輯
const handleSaveExchangeConfig = async (
  exchangeId: string,
  apiKey: string,
  secretKey?: string
) => {
  try {
    // 1. 获取服务器公鑰
    const serverPublicKey = await fetchServerPublicKey();

    // 2. 二階段输入并加密私鑰
    const { encryptedKey } = await twoStagePrivateKeyInput(serverPublicKey);

    // 3. 发送加密数据到后端
    await api.post('/api/exchange/config', {
      exchange_id: exchangeId,
      api_key: apiKey,
      secret_key: secretKey,
      encrypted_private_key: encryptedKey, // 加密後的私鑰
    });

    alert('✅ 配置已安全保存');
  } catch (error) {
    console.error('保存失败:', error);
    alert('❌ 保存失败，請重試');
  }
};
```

---

## 第五步：安全加固

### 5.1 文件权限設置

```bash
# 限制密钥文件权限
chmod 700 .secrets
chmod 600 .secrets/*

# 数据庫权限
chmod 600 config.db

# 检查权限
ls -la .secrets/
# 應該显示: drwx------ (僅所有者可讀寫执行)
```

### 5.2 阿里云服务器加固

```bash
# 1. 启用防火牆
sudo ufw enable
sudo ufw allow 8080/tcp   # 后端 API
sudo ufw allow 3000/tcp   # 前端 (生产环境應該用 Nginx 反向代理)
sudo ufw allow 22/tcp     # SSH

# 2. 禁用 root SSH 登入
sudo nano /etc/ssh/sshd_config
# 修改: PermitRootLogin no
sudo systemctl restart sshd

# 3. 安裝 fail2ban（防止暴力破解）
sudo apt install fail2ban
sudo systemctl enable fail2ban
```

### 5.3 监控与告警

创建监控腳本 `scripts/monitor_security.sh`:

```bash
#!/bin/bash

# 监控 .secrets 目錄访问
inotifywait -m .secrets/ -e access -e modify |
while read path action file; do
    echo "⚠️  $(date): $file 被 $action"
    # 发送告警（可接入釘釘/Telegram）
    curl -X POST "https://your-alert-webhook" \
         -d "密钥文件被访问: $file"
done
```

---

## 第六步：密钥轮换計劃

### 6.1 定期轮换主密钥（建議每季度一次）

```bash
# 1. 创建轮换腳本
cat > scripts/rotate_master_key.sh << 'EOF'
#!/bin/bash
set -e

echo "🔄 开始轮换主密钥..."

# 停止服務
sudo systemctl stop nofx

# 备份当前密钥
cp .secrets/master.key .secrets/master.key.old

# 执行轮换
go run scripts/rotate_key.go

# 更新环境变量
NEW_KEY=$(cat .secrets/master.key)
sudo sed -i "s/NOFX_MASTER_KEY=.*/NOFX_MASTER_KEY=$NEW_KEY/" /etc/systemd/system/nofx.service

# 重新加密所有数据
go run scripts/reencrypt_all.go

# 重啟服務
sudo systemctl start nofx

echo "✅ 密钥轮换完成"
EOF

chmod +x scripts/rotate_master_key.sh
```

---

## 第七步：验证与测试

### 7.1 加密测试

```bash
# 测试加密/解密流程
go test ./crypto -v

# 预期输出:
# ✅ RSA 密钥對生成成功
# ✅ AES 加密/解密测试通過
# ✅ 混合加密测试通過
```

### 7.2 端到端测试

```bash
# 1. 启动后端
go run main.go

# 2. 打開前端，测试私鑰输入
# http://localhost:3000

# 3. 验证数据庫中的数据已加密
sqlite3 config.db "SELECT api_key FROM exchanges LIMIT 1;"
# 應該看到 Base64 字串，而非明文
```

### 7.3 安全审计

```bash
# 检查是否有明文密钥外流
grep -r "0x[0-9a-fA-F]{64}" . --exclude-dir=node_modules --exclude-dir=.git
# 應該沒有任何输出

# 检查日誌中是否有敏感信息
grep -i "private.*key\|secret\|api.*key" nohup.out | head
# 應該只看到审计日誌，沒有明文密钥
```

---

## 紧急情況处理

### 情況1：密钥丟失

```bash
# ⚠️ 如果主密钥丟失，所有加密数据将无法恢復
# 恢復方式：
1. 从备份恢復 .secrets/master.key
2. 或使用最近的数据庫备份（未加密版本）
3. 重新生成密钥并提示用戶重新输入
```

### 情況2：懷疑密钥外流

```bash
# 立即执行密钥轮换
./scripts/rotate_master_key.sh

# 撤銷所有交易所 API 权限
# 通知用戶重新配置

# 检查审计日誌
curl http://localhost:8080/api/audit-logs \
  -H "X-User-ID: <user_id>" | jq .
```

---

## 性能与成本

- **加密開銷**: 每次操作增加 ~5ms 延遲（可忽略）
- **存储開銷**: 加密後数据大小增加 ~30%
- **维护成本**: 每季度密钥轮换需要停機 ~5 分鐘

---

## 合規性检查清单

- [x] 私鑰端到端加密（前端 → 后端）
- [x] 数据庫敏感字段加密
- [x] 审计日誌完整记录
- [x] 密钥轮换机制
- [x] 访问控制（文件权限 600）
- [x] 传输层安全（HTTPS）
- [ ] 雙因素認證（2FA）
- [ ] 硬體安全模组（HSM）- 可選

---

## 常見问题

**Q: 为什麼不直接使用 MetaMask 簽名？**
A: MetaMask 簽名适合交易场景，但 AI 自動交易需要服务器端持有私鑰。本方案在此前提下最大化安全性。

**Q: 主密钥存在环境变量是否安全？**
A: 环境变量相比硬編碼更安全，但最佳實踐是使用云端 KMS（如 AWS Secrets Manager）。

**Q: 如何防止内部人員竊取密钥？**
A: 启用审计日誌、最小权限原則、密钥分片（Shamir's Secret Sharing）。

---

## 下一步优化方向

1. **硬體安全模组（HSM）**: 将主密钥存储在专用硬體中
2. **零知识证明**: 实现完全不上傳私鑰的簽名方案
3. **多方計算（MPC）**: 私鑰分片存储，無单點故障
4. **生物识別**: 結合指紋/Face ID 进行二次验证

---

**安全是持續的過程，而非一次性配置。定期審查、更新、演練。**
