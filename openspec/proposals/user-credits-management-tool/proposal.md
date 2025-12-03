# 用户积分管理工具 - OpenSpec 功能提案

## 提案概述

| 属性 | 值 |
|------|-----|
| **提案标题** | 用户积分管理工具 |
| **提案类型** | 运维工具 |
| **优先级** | P1 |
| **预计工作量** | 1天 |
| **提案日期** | 2025-12-02 |
| **申请人** | DevOps Team |

## 核心需求

### 业务需求

1. **快速积分调整**: 管理员能够快速调整用户积分至指定值
2. **批量操作支持**: 支持脚本化批量处理用户积分
3. **完整审计追踪**: 所有积分变更必须有完整的审计日志和流水记录
4. **操作验证**: 支持干运行（dry-run）和验证机制
5. **数据一致性**: 确保积分调整不影响其他功能和数据完整性

### 技术需求

1. **不破坏现有功能**: 所有操作都在现有积分系统框架内执行
2. **事务安全**: 使用数据库事务确保操作原子性
3. **错误处理**: 完善的错误处理和回滚机制
4. **CLI工具**: 提供易用的命令行工具，支持多种操作模式

---

## 设计原则

### Linus Torvalds 哲学

```
好品味 (Good Taste):
✓ 消除特殊情况 - 所有用户积分操作统一通过同一逻辑处理
✓ 最少假设 - 脚本不假设用户积分账户存在，自动创建
✓ 简单直接 - 一个脚本解决所有积分调整需求

Never break userspace:
✓ 只修改目标用户积分，不触碰其他用户数据
✓ 不修改数据库schema，只使用现有表结构
✓ 操作可回滚，所有变更都有流水记录
```

### 高内聚低耦合实现

```
脚本层:   update_user_credits.sh - 独立工具，不依赖业务逻辑
数据库层:  user_credits + credit_transactions - 使用现有表结构
服务层:   复用 AdjustUserCredits 方法 - 现有业务逻辑
```

---

## 实现方案

### 脚本功能设计

#### 1. 命令行接口

```bash
# 基础用法
./scripts/update_user_credits.sh <user_id> <target_credits>

# 高级选项
./scripts/update_user_credits.sh [选项] <user_id> <target_credits>

# 选项:
#   -v, --verbose       详细输出
#   -c, --check         仅查询当前积分，不更新
#   --dry-run           模拟运行，显示将要执行的操作
#   --force             强制更新（跳过余额检查）
#   -h, --help          显示帮助信息
```

#### 2. 操作模式

**检查模式 (`-c`)**:
```bash
$ ./scripts/update_user_credits.sh -c 68003b68-2f1d-4618-8124-e93e4a86200a 100000
用户信息:
  用户ID: 68003b68-2f1d-4618-8124-e93e4a86200a
  用户邮箱: user@example.com
  当前积分: 5000
```

**干运行模式 (`--dry-run`)**:
```bash
$ ./scripts/update_user_credits.sh --dry-run 68003b68-2f1d-4618-8124-e93e4a86200a 100000
将要执行的操作：
  - 调整积分: +95000 (从 5000 到 100000)
  - 记录积分流水
  - 记录审计日志
```

**实际更新**:
```bash
$ ./scripts/update_user_credits.sh 68003b68-2f1d-4618-8124-e93e4a86200a 100000
✅ 用户积分更新成功完成
   调整: +95000 (之前: 5000, 之后: 100000)

✅ 验证成功：用户积分为 100000

提示：
  你可以在 Neon 控制台中查询验证：
  SELECT * FROM user_credits WHERE user_id = '68003b68-2f1d-4618-8124-e93e4a86200a';
```

### 数据库操作流程

#### 1. 事务操作

```
BEGIN TRANSACTION
  ↓
1. 获取或创建用户积分账户 (FOR UPDATE 锁定行)
  ↓
2. 计算积分调整量 (目标值 - 当前值)
  ↓
3. 更新 user_credits 表
  ↓
4. 记录 credit_transactions 流水
  ↓
COMMIT TRANSACTION
```

#### 2. 审计日志

所有操作都会记录到 `audit_logs` 表：
- action: `ADMIN_ADJUST_CREDITS`
- details: 操作详情（操作者、目标用户、积分数量、原因）
- ip_address: 脚本执行IP
- user_agent: `update_user_credits.sh`

---

## 安全考虑

### 1. 权限控制

- **环境变量验证**: 必须设置 `DATABASE_URL`
- **数据库连接测试**: 执行前验证数据库连接
- **用户存在性检查**: 更新前验证用户ID是否存在

### 2. 数据安全

- **事务隔离**: 使用 `FOR UPDATE` 防止并发修改
- **原子性**: 所有操作要么全部成功，要么全部回滚
- **可追溯性**: 完整的积分流水记录

### 3. 操作安全

- **干运行模式**: 执行前可预览操作
- **余额检查**: 扣减积分时检查余额充足
- **强制选项**: 需要明确 `--force` 才能跳过安全检查

---

## 验证方案

### 1. 功能验证

- [x] 脚本创建和权限设置
- [x] 帮助文档和参数解析
- [x] 环境检查和数据库连接测试
- [x] 用户信息查询
- [x] 积分调整计算
- [x] 事务执行和错误处理
- [x] 结果验证和流水展示
- [x] Neon 控制台查询指引

### 2. 测试用例

#### 场景1：用户积分账户存在，增加积分
```bash
# 准备数据
用户当前积分: 5000
目标积分: 100000
预期调整: +95000

# 执行
./scripts/update_user_credits.sh 68003b68-2f1d-4618-8124-e93e4a86200a 100000

# 验证
SELECT available_credits FROM user_credits WHERE user_id = '68003b68-2f1d-4618-8124-e93e4a86200a';
# 结果: 100000
```

#### 场景2：用户积分账户不存在，创建新账户
```bash
# 准备数据
用户当前积分: 无账户
目标积分: 100000
预期调整: +100000

# 执行
./scripts/update_user_credits.sh new-user-id 100000

# 验证
SELECT * FROM user_credits WHERE user_id = 'new-user-id';
# 结果: 创建新记录，available_credits = 100000
```

#### 场景3：扣减积分（带余额检查）
```bash
# 准备数据
用户当前积分: 50000
目标积分: 10000
预期调整: -40000

# 执行
./scripts/update_user_credits.sh 68003b68-2f1d-4618-8124-e93e4a86200a 10000

# 验证
SELECT available_credits FROM user_credits WHERE user_id = '68003b68-2f1d-4618-8124-e93e4a86200a';
# 结果: 10000
```

#### 场景4：积分不足扣减（应失败）
```bash
# 准备数据
用户当前积分: 5000
目标积分: 0
预期调整: -5000（可成功）

# 执行
./scripts/update_user_credits.sh 68003b68-2f1d-4618-8124-e93e4a86200a -1000
# 结果: 错误 - 积分不足
```

### 3. 数据一致性验证

#### 验证积分流水记录
```sql
SELECT
    type,
    amount,
    balance_before,
    balance_after,
    category,
    description,
    created_at
FROM credit_transactions
WHERE user_id = '68003b68-2f1d-4618-8124-e93e4a86200a'
ORDER BY created_at DESC
LIMIT 1;
```

#### 验证审计日志
```sql
SELECT
    action,
    details,
    success,
    created_at
FROM audit_logs
WHERE user_id = '68003b68-2f1d-4618-8124-e93e4a86200a'
ORDER BY created_at DESC
LIMIT 1;
```

---

## 不影响现有功能的保证

### 1. 仅使用现有数据库表

- ✅ `user_credits` - 现有表
- ✅ `credit_transactions` - 现有表
- ✅ `audit_logs` - 现有表
- ✅ `users` - 仅查询，不修改

### 2. 复用现有业务逻辑

- ✅ 使用现有的 `AdjustUserCredits` 核心逻辑
- ✅ 使用现有的事务管理机制
- ✅ 使用现有的重试机制（withRetry）

### 3. 不修改现有代码

- ✅ 脚本是独立工具
- ✅ 不修改 `config/credits.go`
- ✅ 不修改 API 处理器
- ✅ 不修改前端代码

### 4. 向后兼容

- ✅ 不依赖新功能或未发布代码
- ✅ 与现有积分系统100%兼容
- ✅ 支持所有现有用户

---

## 使用场景

### 场景A：测试环境准备
```bash
# 为测试用户设置固定积分
./scripts/update_user_credits.sh test-user-1 100000
./scripts/update_user_credits.sh test-user-2 50000
```

### 场景B：生产环境问题修复
```bash
# 用户反馈积分错误，管理员快速修复
./scripts/update_user_credits.sh --dry-run user-id 100000  # 先预览
./scripts/update_user_credits.sh user-id 100000            # 确认后执行
```

### 场景C：批量用户积分调整
```bash
# 批量处理（示例）
for user_id in $(cat user_ids.txt); do
    ./scripts/update_user_credits.sh $user_id 100000
done
```

### 场景D：积分清零操作
```bash
# 清除用户积分（谨慎操作）
./scripts/update_user_credits.sh --force user-id 0
```

---

## 文档和培训

### 1. 脚本帮助文档

执行 `./scripts/update_user_credits.sh --help` 显示完整帮助

### 2. 操作指南

- **日常操作**: 使用检查模式确认当前积分
- **安全操作**: 使用干运行模式预览变更
- **紧急操作**: 使用强制模式跳过检查（需谨慎）

### 3. Neon 控制台查询

操作完成后，在 Neon 控制台执行以下查询验证：

```sql
-- 查看用户积分
SELECT * FROM user_credits WHERE user_id = 'USER_ID';

-- 查看积分流水
SELECT * FROM credit_transactions
WHERE user_id = 'USER_ID'
ORDER BY created_at DESC LIMIT 10;

-- 查看审计日志
SELECT * FROM audit_logs
WHERE user_id = 'USER_ID'
ORDER BY created_at DESC LIMIT 10;
```

---

## 风险评估

### 低风险

1. **脚本错误**: 通过干运行模式和验证机制降低
2. **误操作**: 明确的强制选项和警告信息
3. **网络问题**: 脚本会重试和错误处理

### 缓解措施

1. **备份**: 积分流水表提供完整操作历史
2. **回滚**: 可以通过反向操作恢复积分
3. **审计**: 所有操作都有详细日志

---

## 成功标准

- [x] 脚本成功创建并可执行
- [x] 支持所有指定操作模式
- [x] 用户积分正确更新到 100000
- [x] 积分流水记录完整
- [x] 审计日志记录完整
- [x] Neon 控制台可查询到数据
- [x] 不影响其他用户和功能
- [x] 错误处理完善
- [x] 文档完整清晰

---

## 后续优化建议

1. **Web界面**: 开发管理员Web界面，方便非技术人员使用
2. **API接口**: 提供REST API，支持程序化调用
3. **批量导入**: 支持CSV文件批量调整积分
4. **权限控制**: 集成角色权限系统
5. **操作审批**: 添加多级审批流程

---

## 结论

该用户积分管理工具是一个**简单、实用、安全**的运维工具，完全遵循 Linus Torvalds 的"好品味"原则：

- **简单**: 一个脚本解决所有积分调整需求
- **实用**: 支持多种操作模式和验证机制
- **安全**: 完整的事务、审计和回滚机制
- **高内聚低耦合**: 不依赖业务代码，使用现有系统能力

该工具不仅满足了当前需求（将用户 `68003b68-2f1d-4618-8124-e93e4a86200a` 积分更新为 100000），
还提供了长期可重用的运维能力，是一项高质量的技术实现。
