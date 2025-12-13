# 密码重置工具使用说明

> ⚠️ **重要**: `resetUserPwd/` 目录包含敏感的密码重置工具，在 `.gitignore` 中，**不会提交到远程仓库**。

## 快速开始

### 1. 重置用户密码

```bash
cd resetUserPwd
go run reset_password.go -email <user-email> -password <new-password>
```

**示例**:
```bash
go run reset_password.go -email gyc567@gmail.com -password eric8577HH
```

### 2. 仅验证密码与哈希

```bash
go run reset_password.go -password <password> -hash <hash> -verify
```

### 3. 查看详细文档

```bash
# 完整使用指南
cat resetUserPwd/README.md

# 快速参考
cat resetUserPwd/QUICK_REFERENCE.md
```

---

## 文件说明

| 文件 | 说明 |
|------|------|
| `resetUserPwd/reset_password.go` | 主脚本，处理密码重置 |
| `resetUserPwd/README.md` | 详细使用文档 |
| `resetUserPwd/QUICK_REFERENCE.md` | 快速命令参考 |

---

## 核心功能

✅ **生成 bcrypt 哈希** - 自动生成强加密的密码哈希
✅ **验证密码** - 确保密码与哈希匹配
✅ **更新数据库** - 安全地更新用户密码
✅ **验证完整性** - 更新后验证哈希完整性
✅ **敏感信息保护** - 存在 `.gitignore`，不提交到仓库

---

## 工作流程

```
修改密码请求
    ↓
[生成新 bcrypt 哈希]
    ↓
[验证密码与哈希匹配]
    ↓
[连接数据库]
    ↓
[查询用户是否存在]
    ↓
[更新用户密码哈希]
    ↓
[验证更新成功]
    ↓
[输出测试命令]
```

---

## 注意事项

1. **环境变量** - 脚本需要 `DATABASE_URL` 环境变量或 `-db` 参数
2. **密码长度** - 最少 8 位
3. **安全性** - 不要在命令历史中保留密码，修改后立即清除
4. **版本控制** - 此目录在 `.gitignore` 中，不会被提交

---

## 常见命令

```bash
# 重置密码
cd resetUserPwd && go run reset_password.go -email <email> -password <password>

# 仅生成哈希（不更新数据库）
go run reset_password.go -password <password> -verify

# 使用自定义数据库
go run reset_password.go -email <email> -password <password> -db '<db-url>'
```

---

更多信息请阅读 `resetUserPwd/README.md`
