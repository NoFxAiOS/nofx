# 修复报告: /api/exchanges 间歇性 500 错误

## 问题描述

用户刚登录后，访问 `/api/exchanges` 端点会返回 500 错误，但过一段时间后又能正常工作。

**错误信息**:
```
Failed to load resource: the server responded with a status of 500 ()
nofx-gyc567.replit.app/api/exchanges:1
Failed to load configs: Error: 获取交易所配置失败
```

## 根本原因分析

### 调查的三个可能原因

#### 原因 1: GetExchangeConfigs 缺少重试逻辑 ❌
**检查结果**: `GetExchanges` 函数已经有 `withRetry` 包装，不是根本原因。

#### 原因 2: GetSystemConfig 缺少重试逻辑 ✅ (根本原因)
**检查结果**: `GetSystemConfig` 函数在 `authMiddleware` 中被调用，但**没有** `withRetry` 包装。

**问题路径**:
```
用户请求 /api/exchanges
    ↓
authMiddleware 被调用
    ↓
GetSystemConfig("admin_mode") 查询数据库  ← 没有重试！
    ↓
Neon 冷启动时第一次查询失败
    ↓
500 错误返回给用户
```

#### 原因 3: isTransientError 缺少 Neon 特有错误 ⚠️
**检查结果**: `isTransientError` 函数缺少一些 Neon 特有的错误类型，需要补充。

## 修复内容

### 1. 为 GetSystemConfig 添加 withRetry

```go
// 修复前
func (d *Database) GetSystemConfig(key string) (string, error) {
    var value string
    err := d.queryRow(`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
    // ...
}

// 修复后
func (d *Database) GetSystemConfig(key string) (string, error) {
    return withRetry(func() (string, error) {
        var value string
        err := d.queryRow(`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
        // ...
    })
}
```

### 2. 为 GetTraders 添加 withRetry

GetTraders 在用户登录后也会被频繁调用，添加了重试逻辑。

### 3. 为 GetTraderConfig 添加 withRetry

GetTraderConfig 用于获取交易员完整配置，添加了重试逻辑。

### 4. 扩展 isTransientError 错误检测

添加了更多 Neon PostgreSQL 特有的错误类型:

```go
transientErrors := []string{
    // 原有错误...
    "driver: bad connection",
    
    // 新增 Neon 特有的冷启动错误
    "terminating connection due to administrator command",
    "can't reach database server",
    "the database system is starting up",
    "connection timed out",
    "network is unreachable",
    "i/o timeout",
    "connection reset by peer",
    "no such host",
}
```

## 修复后的行为

| 场景 | 修复前 | 修复后 |
|------|--------|--------|
| Neon 冷启动时首次请求 | 500 错误 | 自动重试 3 次，成功返回 |
| 连接池死连接 | 500 错误 | 自动获取新连接，成功返回 |
| 临时网络故障 | 500 错误 | 指数退避重试后成功 |

## 重试策略

```
第1次尝试: 立即执行
    ↓ 失败
等待 100ms
    ↓
第2次尝试
    ↓ 失败
等待 200ms
    ↓
第3次尝试
    ↓ 失败
返回错误 (总共耗时 ~700ms)
```

## 涉及的函数

| 函数名 | 修复前 | 修复后 |
|--------|--------|--------|
| GetSystemConfig | 无重试 | ✅ withRetry |
| GetTraders | 无重试 | ✅ withRetry |
| GetTraderConfig | 无重试 | ✅ withRetry |
| GetExchanges | ✅ withRetry | ✅ withRetry |
| GetAIModels | ✅ withRetry | ✅ withRetry |
| GetUserByID | ✅ withRetry | ✅ withRetry |

## 部署说明

修复已完成，需要点击 **Publish** 按钮将更改部署到生产环境。

## 验证方法

1. 等待 Neon 数据库进入休眠状态（约 5 分钟无活动）
2. 访问 https://nofx-gyc567.replit.app/
3. 登录并访问交易所配置页面
4. 确认不再出现 500 错误

## 日志监控

修复后，如果发生重试，会在日志中看到：
```
⚠️ 数据库操作失败 (尝试 1/3): driver: bad connection, 100ms后重试...
⚠️ 数据库操作失败 (尝试 2/3): driver: bad connection, 200ms后重试...
✅ 重试成功
```

## 参考资料

- [Neon Connection Errors](https://neon.com/docs/connect/connection-errors)
- [Neon Connection Latency](https://neon.com/docs/connect/connection-latency)
