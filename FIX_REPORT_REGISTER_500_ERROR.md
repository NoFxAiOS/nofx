# 修复报告：用户注册 500 错误

## 问题描述
用户在生产环境 (nofx-gyc567.replit.app) 注册时收到 500 错误。

## 调查过程

### 调查的 5 个可能原因

| 可能原因 | 调查结果 | 结论 |
|----------|----------|------|
| **1. 数据库连接问题（Neon冷启动）** | `GetUserByEmail` 和 `CreateUser` 没有 `withRetry` 逻辑 | ✅ **根本原因** |
| **2. CORS 配置问题** | 配置正确，白名单包含生产域名 | ❌ 排除 |
| **3. GetSystemConfig 无重试** | 已有 `withRetry` 逻辑 | ❌ 排除 |
| **4. 密码哈希依赖问题** | bcrypt 工作正常 | ❌ 排除 |
| **5. 生产环境代码版本过旧** | 需要重新部署以应用修复 | ⚠️ 需操作 |

### 根本原因分析

注册流程中调用的数据库函数缺少重试逻辑：

```
用户注册请求
    ↓
GetUserByEmail(email) ← 检查邮箱是否已存在 ❌ 无 withRetry
    ↓
CreateUser(user) ← 创建新用户 ❌ 无 withRetry
    ↓
500 错误（Neon冷启动时连接失败）
```

对比其他已修复的函数：

| 函数 | withRetry | 状态 |
|------|-----------|------|
| GetUserByID | ✅ 有 | 正常 |
| GetUserByEmail | ❌ 无 | **问题** |
| CreateUser | ❌ 无 | **问题** |
| GetSystemConfig | ✅ 有 | 正常 |
| GetExchanges | ✅ 有 | 正常 |
| GetAIModels | ✅ 有 | 正常 |

## 修复方案

### 1. 为 GetUserByEmail 添加 withRetry

```go
// 修复前
func (d *Database) GetUserByEmail(email string) (*User, error) {
    var user User
    err := d.queryRow(`SELECT ... FROM users WHERE email = ?`, email).Scan(...)
    return &user, nil
}

// 修复后
func (d *Database) GetUserByEmail(email string) (*User, error) {
    return withRetry(func() (*User, error) {
        var user User
        err := d.queryRow(`SELECT ... FROM users WHERE email = ?`, email).Scan(...)
        return &user, nil
    })
}
```

### 2. 为 CreateUser 添加 withRetry

```go
// 修复前
func (d *Database) CreateUser(user *User) error {
    _, err := d.exec(`INSERT INTO users ...`)
    return err
}

// 修复后
func (d *Database) CreateUser(user *User) error {
    _, err := withRetry(func() (bool, error) {
        _, execErr := d.exec(`INSERT INTO users ...`)
        return true, execErr
    })
    return err
}
```

## 修改的文件

| 文件 | 修改内容 |
|------|----------|
| `config/database.go` | 1. GetUserByEmail 添加 withRetry 包装<br>2. CreateUser 添加 withRetry 包装 |

## withRetry 机制说明

```go
func withRetry[T any](operation func() (T, error)) (T, error) {
    maxRetries := 3
    retryDelay := 500 * time.Millisecond
    
    for attempt := 1; attempt <= maxRetries; attempt++ {
        result, err := operation()
        if err == nil {
            return result, nil
        }
        
        if !isTransientError(err) {
            return result, err  // 非暂时性错误，不重试
        }
        
        if attempt < maxRetries {
            time.Sleep(retryDelay)
            retryDelay *= 2  // 指数退避
        }
    }
    return zero, lastError
}

func isTransientError(err error) bool {
    transientErrors := []string{
        "connection reset by peer",
        "connection refused", 
        "connection timed out",
        "terminating connection",
        "can't reach database server",
        "database system starting up",
        "too many connections",
        "connection pool timeout",
    }
    // 检查错误是否包含这些字符串
}
```

## 验证结果

### 修复前
```
生产环境: POST /api/register → 500 Internal Server Error
```

### 修复后
```bash
$ curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123456"}'

{"success":true,"message":"注册成功，欢迎加入Monnaire Trading Agent OS！","token":"...","user":{"id":"...","email":"test@example.com"}}
```

## 部署说明

修复已在开发环境验证通过。要将修复部署到生产环境：

1. 点击 Replit 的 **"Publish"** 按钮
2. 选择 **Reserved VM** 部署类型
3. 点击 **Publish**

## 相关修复

此修复与之前的 Neon 冷启动问题修复一致：
- `FIX_REPORT_NEON_COLDSTART.md` - 初始连接池和重试逻辑
- `FIX_REPORT_API_500_ERROR.md` - GetSystemConfig 重试逻辑

## 日期
2025-12-01
