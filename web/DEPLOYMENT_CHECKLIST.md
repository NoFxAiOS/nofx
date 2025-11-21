# 部署验证清单 - 内测码认证机制重构

## 📋 快速验证步骤

### 1. 编译验证 ✅
```bash
cd /Users/guoyingcheng/dreame/code/nofx/web
npm run build
```
**结果**: ✅ 成功 (30.30s, 2743 模块)

### 2. 数据库迁移
```bash
# 检查 beta_code 字段是否存在
sqlite3 database.db ".schema users"

# 回填已有用户数据
# (在代码中调用 database.MigrateUserBetaCodes())
```

### 3. API 测试

#### 3.1 注册测试（内测模式）
```bash
# 开启内测模式
curl -X POST http://localhost:8080/api/config \
  -H "Content-Type: application/json" \
  -d '{"beta_mode": "true"}'

# 注册（需要内测码）
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "password123", "beta_code": "abc123"}'
```

#### 3.2 登录测试
```bash
# 正确凭据登录
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "password123"}'

# 期望响应:
# {
#   "token": "...",
#   "user_id": "...",
#   "email": "test@example.com",
#   "message": "登录成功"
# }
```

#### 3.3 错误测试
```bash
# 无效内测码登录（内测模式下）
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "wrong"}'

# 期望响应:
# {
#   "error": "邮箱或密码错误"
# }

# 有效密码但内测码无效
# (需要在数据库中标记内测码为已使用)
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "password123"}'

# 期望响应:
# {
#   "error": "内测码无效，请联系管理员"
# }
```

### 4. 前端验证

#### 4.1 登录页面
- [ ] 访问 `/login`
- [ ] 使用正确凭据登录 ✅
- [ ] 使用错误密码 → 显示"邮箱或密码错误" ✅

#### 4.2 注册页面
- [ ] 访问 `/register`
- [ ] 填写邮箱、密码、内测码
- [ ] 注册成功并自动登录 ✅

#### 4.3 admin@localhost 验证
- [ ] 确认无法通过 `admin@localhost` 自动登录 ✅
- [ ] 确认无 `admin_mode` 配置项 ✅

---

## 🔍 关键检查点

### ✅ 已移除
- [ ] `auth.AdminMode` 变量
- [ ] `auth.IsAdminMode()` 函数
- [ ] API 响应中的 `admin_mode` 字段
- [ ] 前端 `isAdminMode` 参数
- [ ] AdminMode 自动登录逻辑

### ✅ 已添加
- [ ] 登录时内测码验证（内测模式下）
- [ ] 用户表 `beta_code` 字段
- [ ] `GetUserBetaCode()` 函数
- [ ] `MigrateUserBetaCodes()` 函数

### ✅ 验证逻辑
```
内测模式开启时:
  用户登录 → 验证邮箱/密码 → 验证内测码 → 允许/拒绝
内测模式关闭时:
  用户登录 → 验证邮箱/密码 → 允许登录
```

---

## 🚨 回滚计划

如果发现问题，可以快速回滚：

### 1. 恢复 AdminMode
```go
// auth.go
var AdminMode bool = false

// api/server.go handleGetSystemConfig
"admin_mode": auth.IsAdminMode(),
```

### 2. 移除内测码验证
```go
// api/server.go handleLogin
// 注释掉内测码验证部分 (1515-1545行)
```

### 3. 数据库回滚
```sql
ALTER TABLE users DROP COLUMN beta_code;
```

---

## 📊 测试矩阵

| 场景 | beta_mode | 内测码 | 期望结果 |
|------|-----------|--------|----------|
| 正常登录 | false | 无 | ✅ 成功 |
| 正常登录 | true | 有效 | ✅ 成功 |
| 错误密码 | 任意 | 任意 | ❌ 邮箱或密码错误 |
| 无内测码 | true | 无 | ❌ 内测码无效 |
| 内测码已用 | true | 已使用 | ❌ 内测码无效 |

---

## 📝 验证完成标记

- [ ] 编译验证通过
- [ ] 数据库迁移完成
- [ ] API 测试通过
- [ ] 前端测试通过
- [ ] 无 AdminMode 残留
- [ ] 错误提示正确

**全部完成后签名**: _________________ **日期**: _________
