# 📄 config.json.example 说明文檔

> **注意**: 标准 JSON 不支持 `//` 注释，所以本文檔補充说明配置范例的每个欄位用途。

## 如何使用

```bash
# 1. 复制范例文件
cp config.json.example config.json

# 2. 根据下方说明編輯 config.json
nano config.json

# 3. 启动系统
./nofx
```

---

## 配置欄位说明

### 基本设定

```json
"admin_mode": true
```
- **说明**: 管理員模式，跳過登入验证
- **建議**: 
  - 開發环境: `true` (方便测试)
  - 生产环境: `false` (需要登入)

---

### 杠桿配置

```json
"leverage": {
  "btc_eth_leverage": 5,
  "altcoin_leverage": 5
}
```
- **说明**: 
  - `btc_eth_leverage`: BTC/ETH 的杠桿倍數
  - `altcoin_leverage`: 山寨币的杠桿倍數
- **建議**: 
  - 新手: 3-5x (安全)
  - 有经驗: 5-10x (平衡)
  - ⚠️ 高風险: >10x (容易爆倉)

---

### 交易币種

```json
"use_default_coins": true,
"default_coins": [
  "BTCUSDT",
  "ETHUSDT",
  "SOLUSDT",
  ...
]
```
- **说明**: 
  - `use_default_coins: true` → 使用内建币種列表
  - `use_default_coins: false` → 使用外部 API 获取币種
- **建議**: 
  - 新手: 保持 `true` (使用預設的主流币)
  - 進階: 設为 `false` 并配置 `coin_pool_api_url`

---

### 外部数据源

```json
"coin_pool_api_url": "",
"oi_top_api_url": ""
```
- **说明**: 
  - `coin_pool_api_url`: 自定義币種池 API
  - `oi_top_api_url`: 持倉量排行 API
- **何时使用**: 
  - 空字符串 (`""`) → 使用内建数据
  - 填入 URL → 使用外部 API (進階用戶)

---

### 風险控制

```json
"max_daily_loss": 10.0,
"max_drawdown": 20.0,
"stop_trading_minutes": 60
```
- **说明**: 
  - `max_daily_loss`: 单日最大亏损百分比 (觸發後停止交易)
  - `max_drawdown`: 最大回撤百分比
  - `stop_trading_minutes`: 觸發風控後暫停交易的时间 (分鐘)
- **建議**: 
  - 保守: `max_daily_loss: 5.0`
  - 中等: `max_daily_loss: 10.0` (預設)
  - 激進: `max_daily_loss: 20.0`

---

### JWT 密钥

```json
"jwt_secret": "Qk0kAa+d0iIEzXVHXbNbm+UaN3RNabmWtH8rDWZ5OPf..."
```
- **说明**: 用于用戶認證的密钥
- **⚠️ 安全警告**: 
  - 生产环境必须更換！
  - 使用以下命令生成新密钥:
    ```bash
    openssl rand -base64 64
    ```

---

### 新聞源配置 (可選功能)

```json
"news": [
  {
    "provider": "telegram",
    "telegram": {
      "proxyurl": "http://127.0.0.1:18080"
    },
    "channels": [
      {
        "id": "ChannelPANews",
        "name": "PANews"
      }
    ]
  }
]
```

**⚠️ 重要提示**: 目前新聞源功能还比較初级，建議使用时刪除或保持預設值

#### 欄位说明:

**`proxyurl`**:
- **用途**: Telegram 代理地址
- **何时需要**: 
  - ✅ 中国大陸服务器: 需要配置代理
  - ❌ 国外服务器: 留空或刪除此行

**`channels.id`**:
- **用途**: Telegram 頻道 ID
- **如何获取**: 
  - 例如頻道網址是 `t.me/ChannelPANews`
  - 則 `id` 为 `"ChannelPANews"` (去掉 t.me/)

**`channels.name`**:
- **用途**: 頻道显示名稱 (僅用于识別)
- **建議**: 填入易识別的名稱

#### 示例配置:

```json
// 中国大陸服务器 (需要代理)
"telegram": {
  "proxyurl": "http://127.0.0.1:18080"
}

// 国外服务器 (無需代理)
"telegram": {
  "proxyurl": ""
}

// 或直接刪除 telegram 整个配置塊
"news": []
```

#### 推薦的 Telegram 頻道:

| 頻道 ID | 頻道名稱 | 内容类型 |
|---------|---------|---------|
| `ChannelPANews` | PANews | 加密货币新聞 |
| `cointelegraph` | Cointelegraph | 區塊鏈资讯 |
| `BitcoinMagazine` | Bitcoin Magazine | 比特币專题 |

---

## 完整范例

### 示例 1: 保守型配置 (新手推薦)

```json
{
  "admin_mode": false,
  "leverage": {
    "btc_eth_leverage": 3,
    "altcoin_leverage": 3
  },
  "use_default_coins": true,
  "default_coins": ["BTCUSDT", "ETHUSDT"],
  "coin_pool_api_url": "",
  "oi_top_api_url": "",
  "api_server_port": 8080,
  "max_daily_loss": 5.0,
  "max_drawdown": 10.0,
  "stop_trading_minutes": 120,
  "jwt_secret": "YOUR_NEW_GENERATED_SECRET_HERE",
  "news": []
}
```

### 示例 2: 激進型配置 (经驗用戶)

```json
{
  "admin_mode": true,
  "leverage": {
    "btc_eth_leverage": 10,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "default_coins": [
    "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", 
    "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"
  ],
  "coin_pool_api_url": "",
  "oi_top_api_url": "",
  "api_server_port": 8080,
  "max_daily_loss": 15.0,
  "max_drawdown": 30.0,
  "stop_trading_minutes": 30,
  "jwt_secret": "YOUR_NEW_GENERATED_SECRET_HERE",
  "news": []
}
```

---

## 常見问题

**Q: 为什麼 JSON 文件不能有注释？**
A: 标准 JSON 格式不支持 `//` 或 `/* */` 注释。如果需要注释，請参阅本文檔。

**Q: 如何验证 JSON 格式正確？**
A: 使用以下命令:
```bash
python3 -c "import json; json.load(open('config.json')); print('✅ 格式正確')"
```

**Q: 如果我只想交易 BTC 怎麼辦？**
A: 修改 `default_coins` 为:
```json
"default_coins": ["BTCUSDT"]
```

---

## 相关文檔

- [README.md](README.md) - 完整使用说明
- [CONFIG_SECURITY_GUIDE.md](CONFIG_SECURITY_GUIDE.md) - 安全配置指南
- [ENCRYPTION_DEPLOYMENT.md](docs/ENCRYPTION_DEPLOYMENT.md) - 加密部署

---

**記住**: 配置文件包含敏感信息，請勿提交到 Git！将 `config.json` 加入 `.gitignore`。
