# 用户积分管理工具

## 概述

提供多个工具用于查询、更新和验证用户积分。所有工具都使用事务确保数据一致性，并记录完整的审计日志。

## 工具列表

### 1. update_credits.go - 积分更新工具

**功能**: 将指定用户的积分更新为目标值

**用法**:
```bash
# 设置环境变量
export DATABASE_URL="postgresql://neondb_owner:npg_i1TxKk6bzZgw@ep-green-poetry-adtqfubw-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require"

# 运行程序
go run scripts/update_credits.go
```

**配置**: 修改程序中的变量：
- `userID`: 目标用户ID
- `targetCredits`: 目标积分值
- `reason`: 更新原因

**特性**:
- ✅ 自动创建用户积分账户（如果不存在）
- ✅ 使用事务确保原子性
- ✅ 记录完整积分流水
- ✅ 自动验证更新结果

---

### 2. verify_credits.go - 积分验证工具

**功能**: 查询和验证用户积分信息

**用法**:
```bash
export DATABASE_URL="postgresql://neondb_owner:npg_i1TxKk6bzZgw@ep-green-poetry-adtqfubw-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require"
go run scripts/verify_credits.go
```

**输出**:
- 用户积分信息（可用、总计、已使用）
- 最近积分流水记录
- 审计日志记录

---

### 3. update_user_credits.sh - Shell脚本工具

**功能**: 命令行积分管理工具，支持多种操作模式

**用法**:
```bash
# 显示帮助
./scripts/update_user_credits.sh --help

# 检查模式（仅查询）
./scripts/update_user_credits.sh -c USER_ID TARGET_CREDITS

# 干运行模式（模拟执行）
./scripts/update_user_credits.sh --dry-run USER_ID TARGET_CREDITS

# 实际更新
./scripts/update_user_credits.sh USER_ID TARGET_CREDITS

# 详细输出
./scripts/update_user_credits.sh -v USER_ID TARGET_CREDITS

# 强制模式（跳过安全检查）
./scripts/update_user_credits.sh --force USER_ID TARGET_CREDITS
```

**选项**:
- `-c, --check`: 仅查询当前积分，不更新
- `--dry-run`: 模拟运行，显示将要执行的操作
- `-v, --verbose`: 详细输出
- `--force`: 强制更新（跳过余额检查）
- `-h, --help`: 显示帮助信息

---

## 数据库连接

所有工具都需要设置 `DATABASE_URL` 环境变量：

```bash
export DATABASE_URL="postgresql://neondb_owner:npg_i1TxKk6bzZgw@ep-green-poetry-adtqfubw-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require"
```

## 使用场景

### 场景1：更新特定用户积分

```bash
# 1. 检查用户当前积分
export DATABASE_URL="..."
go run scripts/verify_credits.go

# 2. 更新积分（编辑脚本中的变量）
#    userID = "68003b68-2f1d-4618-8124-e93e4a86200a"
#    targetCredits = 100000
go run scripts/update_credits.go

# 3. 验证结果
go run scripts/verify_credits.go
```

### 场景2：批量调整积分

```bash
# 使用Shell脚本
for user_id in user1 user2 user3; do
    ./scripts/update_user_credits.sh $user_id 100000
done
```

### 场景3：生产环境紧急修复

```bash
# 1. 检查用户积分
./scripts/update_user_credits.sh -c USER_ID

# 2. 干运行查看操作计划
./scripts/update_user_credits.sh --dry-run USER_ID 100000

# 3. 确认后执行
./scripts/update_user_credits.sh USER_ID 100000

# 4. 验证
go run scripts/verify_credits.go
```

## 重要提醒

### 安全操作

1. **生产环境操作前先测试**
   ```bash
   # 在测试环境验证
   export TEST_DATABASE_URL="..."
   # 执行操作
   ```

2. **使用干运行模式预览**
   ```bash
   ./scripts/update_user_credits.sh --dry-run USER_ID 100000
   ```

3. **操作后验证结果**
   ```bash
   go run scripts/verify_credits.go
   ```

### 数据一致性

- ✅ 所有操作使用事务确保原子性
- ✅ 出错时自动回滚
- ✅ 完整的审计日志记录
- ✅ 积分流水记录可追溯

### Neon 控制台查询

操作完成后，可在 Neon 控制台验证：

```sql
-- 查询用户积分
SELECT * FROM user_credits WHERE user_id = 'USER_ID';

-- 查询积分流水
SELECT * FROM credit_transactions
WHERE user_id = 'USER_ID'
ORDER BY created_at DESC LIMIT 10;

-- 查询审计日志
SELECT * FROM audit_logs
WHERE user_id = 'USER_ID'
ORDER BY created_at DESC LIMIT 10;
```

## 故障排除

### 数据库连接失败

```bash
# 检查环境变量
echo $DATABASE_URL

# 测试连接
go run scripts/verify_credits.go
```

### 用户不存在

```bash
# 确认用户ID正确
# 脚本会自动创建积分账户，但用户必须在users表中存在
```

### 积分不足

```bash
# 扣减积分时余额不足，使用强制模式
./scripts/update_user_credits.sh --force USER_ID TARGET
```

## 技术细节

### 数据库事务

所有更新操作都在事务中执行：

```
BEGIN → SELECT FOR UPDATE → UPDATE/INSERT → INSERT流水 → COMMIT
                                  ↓
                              出错时回滚
```

### 并发控制

使用 `FOR UPDATE` 锁定行，防止并发修改：

```sql
SELECT id, available_credits FROM user_credits
WHERE user_id = $1 FOR UPDATE
```

### 错误处理

- **输入验证**: 检查用户ID和积分值格式
- **数据库错误**: 自动重试（处理Neon冷启动）
- **余额检查**: 扣减时验证余额充足
- **事务回滚**: 任何步骤失败都回滚所有更改

## 相关文档

- **OpenSpec提案**: `../openspec/proposals/user-credits-management-tool/`
- **实现报告**: `../openspec/proposals/user-credits-management-tool/IMPLEMENTATION_REPORT.md`
- **技术规范**: `../openspec/proposals/user-credits-management-tool/specs/script-spec.md`

## 许可证

本工具遵循项目许可证条款。

## 联系方式

如有疑问，请联系 DevOps Team。

---

**版本**: v1.0
**最后更新**: 2025-12-02
