# Neon PostgreSQL 冷启动问题修复报告

**日期**: 2025-11-27  
**问题**: 间歇性 401/500 错误 - `/api/models` 和 `/api/supported-exchanges`  
**状态**: 已修复

---

## 问题描述

前端偶尔报错：
```
GET https://nofx-gyc567.replit.app/api/models 401 (Unauthorized)
GET https://nofx-gyc567.replit.app/api/supported-exchanges 500 (Internal Server Error)
```

导致 AI 模型配置和交易所配置显示为空。问题间歇性出现，过段时间自动恢复。

## 根本原因分析

| # | 问题 | 说明 |
|---|------|------|
| 1 | **Neon PostgreSQL 冷启动** | 数据库空闲后连接被断开，首次请求失败 |
| 2 | **无连接池配置** | 默认配置下 sql.DB 立即关闭空闲连接 |
| 3 | **无重试逻辑** | 临时性连接错误直接返回给用户 |
| 4 | **无连接保活** | 无后台任务维持连接活跃状态 |

### 错误传播路径

```
Neon连接断开
    ↓
authMiddleware 调用 GetUserByID("admin") → 失败
    ↓
返回 401 Unauthorized
```

```
Neon连接断开
    ↓
handleGetSupportedExchanges 调用 GetExchanges("default") → 失败
    ↓
返回 500 Internal Server Error
```

## 修复方案

### 1. 添加数据库连接池配置

```go
// config/database.go - NewDatabase()
db.SetMaxOpenConns(10)                  // 最大打开连接数
db.SetMaxIdleConns(5)                   // 最大空闲连接数
db.SetConnMaxIdleTime(30 * time.Second) // 空闲连接最大存活时间
db.SetConnMaxLifetime(5 * time.Minute)  // 连接最大生命周期
```

### 2. 添加重试辅助函数

```go
// 检测可重试的临时错误
func isTransientError(err error) bool {
    transientErrors := []string{
        "connection not available",
        "connection reset",
        "connection refused",
        "broken pipe",
        "EOF",
        "timeout",
        // ...
    }
    // 匹配返回 true
}

// 带指数退避的重试逻辑
func withRetry[T any](operation func() (T, error)) (T, error) {
    // 最多重试3次
    // 退避时间: 100ms, 200ms, 400ms
}
```

### 3. 关键查询添加重试

修改以下函数使用 `withRetry()` 包装：
- `GetUserByID()` - 认证中间件依赖
- `GetExchanges()` - `/api/supported-exchanges` 依赖
- `GetAIModels()` - `/api/models` 依赖

### 4. 添加后台连接保活

```go
// 每5分钟ping一次数据库，防止连接被断开
func (d *Database) StartKeepAlive() {
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        for range ticker.C {
            d.db.Ping()
        }
    }()
}
```

## 修改的文件

1. **config/database.go**
   - 添加 `isTransientError()` 函数
   - 添加 `withRetry()` 泛型重试函数
   - 添加 `StartKeepAlive()` 保活协程
   - 修改 `NewDatabase()` 添加连接池配置
   - 修改 `GetUserByID()` 使用重试
   - 修改 `GetExchanges()` 使用重试
   - 修改 `GetAIModels()` 使用重试

2. **main.go**
   - 添加 `database.StartKeepAlive()` 调用

## 验证结果

启动日志显示：
```
📋 数据库连接池配置: MaxOpen=10, MaxIdle=5, IdleTime=30s, Lifetime=5m
🔄 数据库连接保活协程已启动 (每5分钟ping一次)
```

API 端点测试：
```bash
# /api/supported-exchanges - 200 OK
curl http://localhost:8080/api/supported-exchanges
# 返回交易所列表

# /api/models - 200 OK  
curl http://localhost:8080/api/models
# 返回模型配置列表
```

## 连接池参数说明

| 参数 | 值 | 说明 |
|------|-----|------|
| MaxOpenConns | 10 | 适合 Neon serverless，不宜过高 |
| MaxIdleConns | 5 | 保持适量空闲连接备用 |
| ConnMaxIdleTime | 30s | 低于 Neon 默认超时 |
| ConnMaxLifetime | 5m | 定期刷新连接 |

## 部署说明

修复已应用到开发环境。要部署到生产环境：

1. 点击 Replit 的 **Publish** 按钮
2. 选择 **Reserved VM** 部署类型
3. 点击 **Publish** 开始部署

---

**修复人**: AI Agent  
**审核状态**: 待用户验证
