# WxPusher 消息推送功能

## 概述

NoFx Trading 集成了 WxPusher 服务，用于在 AI 交易决策和订单执行时向用户发送实时微信通知。

## 功能特点

- ✅ **AI决策通知** - 当AI做出交易决策时发送通知
- ✅ **订单通知** - 订单开仓和平仓时的实时通知
- ✅ **HTML富文本** - 支持富文本格式的消息内容
- ✅ **按交易员配置** - 每个交易员可独立启用/禁用通知
- ✅ **多用户支持** - 支持向多个微信用户ID发送通知

## 设置步骤

### 1. 获取 WxPusher Token

1. 访问 [WxPusher 官网](https://wxpusher.zjiecode.com/)
2. 注册账号或登录
3. 创建应用，获取 `appToken` 
4. 在微信端扫码关注或绑定用户，获取用户 `UID`

### 2. 在系统中配置

#### 配置通知和 Token (一步完成)

**请求:**
```http
POST /api/notifications/config?trader_id=TRADER_ID
Content-Type: application/json
Authorization: Bearer YOUR_JWT_TOKEN

{
  "is_enabled": true,
  "wx_pusher_token": "AT_xxxxxxxxxxxxx",
  "wx_pusher_uids": "[\"UID_user1\", \"UID_user2\"]"
}
```

**参数说明:**
- `trader_id` - 交易员ID (必填,URL参数)
- `is_enabled` - 是否启用通知 (必填)
- `wx_pusher_token` - WxPusher appToken (启用时必填,会自动加密存储到数据库)
- `wx_pusher_uids` - 接收通知的微信用户ID数组,JSON格式字符串 (必填,当 is_enabled=true 时)

**响应:**
```json
{
  "message": "Notification config updated",
  "data": {
    "id": "config_id",
    "user_id": "user_id",
    "trader_id": "trader_id",
    "wx_pusher_token": "***",
    "wx_pusher_uids": "[\"UID_user1\", \"UID_user2\"]",
    "is_enabled": true,
    "created_at": "2024-01-15T10:00:00Z",
    "updated_at": "2024-01-15T10:00:00Z"
  }
}
```

**安全说明:**
- ✅ Token 自动使用 AES 加密存储在数据库中
- ✅ 返回的配置中 Token 显示为 `***` 以保护安全
- ✅ 加密密钥来自环境变量 `DATA_ENCRYPTION_KEY`

#### 查看配置

**请求:**
```http
GET /api/notifications/config?trader_id=TRADER_ID
Authorization: Bearer YOUR_JWT_TOKEN
```

#### 发送测试通知

**请求:**
```http
POST /api/notifications/test?trader_id=TRADER_ID
Authorization: Bearer YOUR_JWT_TOKEN
```

#### 禁用通知

**请求:**
```http
DELETE /api/notifications/config?trader_id=TRADER_ID
Authorization: Bearer YOUR_JWT_TOKEN
```

## API 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/notifications/config` | 获取通知配置 |
| POST | `/api/notifications/config` | 创建/更新配置(含加密Token) |
| POST | `/api/notifications/test` | 发送测试通知 |
| DELETE | `/api/notifications/config` | 禁用通知 |

## 代码集成

### 在交易代码中发送通知

```go
// 发送AI决策通知
decision := map[string]interface{}{
    "symbol": "BTCUSDT",
    "action": "BUY",
    "confidence": 0.85,
}
err := s.notificationManager.NotifyDecision(
    traderID,
    userID,
    decision,
)

// 发送订单通知
err := s.notificationManager.NotifyTradeOpened(
    traderID,
    userID,
    "BTCUSDT",
    "long",
    0.1,
    50000.0,
)

err := s.notificationManager.NotifyTradeClosed(
    traderID,
    userID,
    "BTCUSDT",
    "long",
    0.1,
    51000.0,
    500.0, // PnL
)
```

## 数据存储

通知配置存储在 `notification_configs` 表中:

```sql
CREATE TABLE notification_configs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    trader_id TEXT NOT NULL,
    wx_pusher_token TEXT, -- AES 加密存储的 WxPusher Token
    wx_pusher_uids TEXT, -- JSON array of UIDs
    is_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**加密机制:**
- Token 使用 `crypto.EncryptedString` 类型自动加密/解密
- 采用 AES-256-GCM 算法加密,密钥由 `DATA_ENCRYPTION_KEY` 环境变量提供
- 与交易所 API 密钥使用相同的加密方式,确保安全性

## 安全性说明

- ✅ WxPusher Token 使用 AES-256 加密存储在数据库中
- ✅ Token 在 API 响应中显示为 `***` 防止泄露
- ✅ 所有通知操作都需要用户认证
- ✅ 用户只能管理自己的通知配置
- ✅ 加密密钥通过环境变量配置,不硬编码在代码中

## 故障排除

### 测试通知失败

1. 检查 Token 是否正确设置
2. 检查 UIDs 是否为有效的 WxPusher 用户ID
3. 确认用户已在 WxPusher 中关注应用

### 没有收到通知

1. 检查通知是否启用: `GET /api/notifications/config?trader_id=xxx`
2. 验证微信用户ID是否正确
3. 检查服务日志中是否有错误信息

## 支持的消息类型

- **HTML 富文本** (contentType=2) - 推荐
- **纯文本** (contentType=1)
- **Markdown** (contentType=3)

## 价格和限制

- WxPusher 免费版：10 app token，每个 app 最多 100 个用户
- 频率限制：10 次/秒/UID
- 详见 [WxPusher 文档](https://wxpusher.zjiecode.com/)
