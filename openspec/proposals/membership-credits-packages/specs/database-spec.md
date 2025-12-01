# 数据库规格说明

## 表关系图

```
┌─────────────────────┐
│       users         │
│  (现有表，不修改)    │
│                     │
│  id (PK)            │◄────────────────────┐
│  email              │                     │
│  ...                │                     │
└─────────────────────┘                     │
                                            │
                                            │ FK (user_id)
                                            │
┌─────────────────────┐     ┌─────────────────────────────┐
│  credit_packages    │     │        user_credits         │
│  (套餐配置表)        │     │      (用户积分账户)          │
│                     │     │                             │
│  id (PK)            │     │  id (PK)                    │
│  name               │     │  user_id (FK, UNIQUE)───────┘
│  price_usdt         │     │  available_credits          │
│  credits            │     │  total_credits              │
│  bonus_credits      │     │  used_credits               │
│  is_active          │     │  created_at                 │
│  is_recommended     │     │  updated_at                 │
│  sort_order         │     └─────────────────────────────┘
└─────────────────────┘                     │
        │                                   │
        │                                   │ FK (user_id)
        │ 业务关联                           │
        │ (购买时记录)                       │
        ▼                                   ▼
┌─────────────────────────────────────────────────────────┐
│                   credit_transactions                    │
│                     (积分流水表)                          │
│                                                          │
│  id (PK)                                                 │
│  user_id (FK)                                            │
│  type (credit/debit)                                     │
│  amount                                                  │
│  balance_before                                          │
│  balance_after                                           │
│  category (purchase/consume/gift/refund)                 │
│  description                                             │
│  reference_id (关联套餐ID/订单ID等)                       │
│  created_at                                              │
└─────────────────────────────────────────────────────────┘
```

## 设计原则

### 1. 不修改现有表

- `users` 表结构保持不变
- 通过外键关联实现扩展
- 避免影响现有业务逻辑

### 2. 单一职责

| 表名 | 职责 |
|------|------|
| `credit_packages` | 存储套餐配置，与用户无关 |
| `user_credits` | 用户积分汇总，一对一关系 |
| `credit_transactions` | 积分变动流水，可审计追溯 |

### 3. 数据一致性

- `user_credits` 的 `user_id` 设置 UNIQUE 约束，保证一对一
- 所有积分操作必须同时写入 `user_credits` 和 `credit_transactions`
- 使用数据库事务保证原子性

---

## 完整迁移脚本

```sql
-- ============================================================
-- 积分系统 - 数据库迁移脚本
-- 版本: 2025-12-01
-- ============================================================

-- 1. 创建积分套餐配置表
CREATE TABLE IF NOT EXISTS credit_packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    name_en TEXT,
    description TEXT,
    price_usdt DECIMAL(10,2) NOT NULL,
    credits INTEGER NOT NULL,
    bonus_credits INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    is_recommended BOOLEAN DEFAULT FALSE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. 创建用户积分账户表
CREATE TABLE IF NOT EXISTS user_credits (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    available_credits INTEGER DEFAULT 0,
    total_credits INTEGER DEFAULT 0,
    used_credits INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT fk_user_credits_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

-- 3. 创建积分流水表
CREATE TABLE IF NOT EXISTS credit_transactions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('credit', 'debit')),
    amount INTEGER NOT NULL CHECK (amount > 0),
    balance_before INTEGER NOT NULL CHECK (balance_before >= 0),
    balance_after INTEGER NOT NULL CHECK (balance_after >= 0),
    category TEXT NOT NULL CHECK (category IN ('purchase', 'consume', 'gift', 'refund', 'admin')),
    description TEXT,
    reference_id TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT fk_credit_transactions_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

-- 4. 创建索引
CREATE INDEX IF NOT EXISTS idx_credit_packages_active ON credit_packages(is_active);
CREATE INDEX IF NOT EXISTS idx_credit_packages_sort ON credit_packages(sort_order);
CREATE INDEX IF NOT EXISTS idx_user_credits_user_id ON user_credits(user_id);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_user_id ON credit_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_created_at ON credit_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_category ON credit_transactions(category);

-- 5. 创建触发器
DROP TRIGGER IF EXISTS update_credit_packages_updated_at ON credit_packages;
DROP TRIGGER IF EXISTS update_user_credits_updated_at ON user_credits;

CREATE TRIGGER update_credit_packages_updated_at
    BEFORE UPDATE ON credit_packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_credits_updated_at
    BEFORE UPDATE ON user_credits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 6. 插入默认套餐数据
INSERT INTO credit_packages (id, name, name_en, description, price_usdt, credits, bonus_credits, is_active, is_recommended, sort_order)
VALUES
    ('pkg_starter', '入门套餐', 'Starter Pack', '适合新用户体验', 5.00, 200, 0, TRUE, FALSE, 1),
    ('pkg_standard', '标准套餐', 'Standard Pack', '10U购买500积分', 10.00, 500, 0, TRUE, TRUE, 2),
    ('pkg_premium', '高级套餐', 'Premium Pack', '20U购买1200积分（超值）', 20.00, 1200, 0, TRUE, FALSE, 3),
    ('pkg_pro', '专业套餐', 'Pro Pack', '专业用户首选', 50.00, 3500, 0, TRUE, FALSE, 4)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    name_en = EXCLUDED.name_en,
    description = EXCLUDED.description,
    price_usdt = EXCLUDED.price_usdt,
    credits = EXCLUDED.credits,
    bonus_credits = EXCLUDED.bonus_credits,
    is_active = EXCLUDED.is_active,
    is_recommended = EXCLUDED.is_recommended,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

-- 7. 验证
DO $$
DECLARE
    pkg_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO pkg_count FROM credit_packages WHERE is_active = TRUE;
    IF pkg_count < 2 THEN
        RAISE EXCEPTION '套餐数量不足: %', pkg_count;
    END IF;
    RAISE NOTICE '积分系统迁移完成，套餐数量: %', pkg_count;
END $$;
```

---

## 回滚脚本

```sql
-- ============================================================
-- 积分系统 - 回滚脚本
-- ============================================================

-- 按依赖顺序删除
DROP TABLE IF EXISTS credit_transactions;
DROP TABLE IF EXISTS user_credits;
DROP TABLE IF EXISTS credit_packages;

-- 清理索引（如果表已删除，索引会自动删除）
```

---

## 字段约束说明

### credit_packages

| 字段 | 约束 | 说明 |
|------|------|------|
| id | PRIMARY KEY | UUID 格式 |
| name | NOT NULL | 套餐名称必填 |
| price_usdt | NOT NULL, DECIMAL(10,2) | 精确到分 |
| credits | NOT NULL | 积分数量 |
| bonus_credits | DEFAULT 0 | 赠送积分 |
| is_active | DEFAULT TRUE | 软删除标识 |

### user_credits

| 字段 | 约束 | 说明 |
|------|------|------|
| id | PRIMARY KEY | UUID 格式 |
| user_id | NOT NULL, UNIQUE, FK | 一对一关系 |
| available_credits | DEFAULT 0 | 可用积分 |
| total_credits | DEFAULT 0 | 累计积分 |
| used_credits | DEFAULT 0 | 已使用积分 |

### credit_transactions

| 字段 | 约束 | 说明 |
|------|------|------|
| id | PRIMARY KEY | UUID 格式 |
| user_id | NOT NULL, FK | 关联用户 |
| type | CHECK IN ('credit', 'debit') | 增加/扣减 |
| amount | CHECK > 0 | 正整数 |
| balance_before | CHECK >= 0 | 非负整数 |
| balance_after | CHECK >= 0 | 非负整数 |
| category | CHECK IN (...) | 枚举值约束 |

---

## 查询示例

### 获取启用的套餐列表

```sql
SELECT id, name, name_en, description, price_usdt, credits, bonus_credits, is_recommended
FROM credit_packages
WHERE is_active = TRUE
ORDER BY sort_order ASC;
```

### 获取用户积分（自动创建）

```sql
-- 先尝试获取
SELECT * FROM user_credits WHERE user_id = $1;

-- 如果不存在，插入并返回
INSERT INTO user_credits (id, user_id, available_credits, total_credits, used_credits)
VALUES (gen_random_uuid()::text, $1, 0, 0, 0)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;
```

### 增加积分（事务）

```sql
BEGIN;

-- 1. 获取当前余额（加行锁）
SELECT available_credits FROM user_credits WHERE user_id = $1 FOR UPDATE;

-- 2. 更新余额
UPDATE user_credits
SET available_credits = available_credits + $2,
    total_credits = total_credits + $2,
    updated_at = NOW()
WHERE user_id = $1;

-- 3. 记录流水
INSERT INTO credit_transactions (id, user_id, type, amount, balance_before, balance_after, category, description, reference_id)
VALUES ($3, $1, 'credit', $2, $4, $5, $6, $7, $8);

COMMIT;
```

### 扣减积分（事务）

```sql
BEGIN;

-- 1. 获取当前余额（加行锁）
SELECT available_credits FROM user_credits WHERE user_id = $1 FOR UPDATE;

-- 2. 检查余额是否充足
-- (在应用层判断)

-- 3. 更新余额
UPDATE user_credits
SET available_credits = available_credits - $2,
    used_credits = used_credits + $2,
    updated_at = NOW()
WHERE user_id = $1 AND available_credits >= $2;

-- 4. 记录流水
INSERT INTO credit_transactions (id, user_id, type, amount, balance_before, balance_after, category, description, reference_id)
VALUES ($3, $1, 'debit', $2, $4, $5, $6, $7, $8);

COMMIT;
```

### 获取积分流水（分页）

```sql
SELECT id, type, amount, balance_before, balance_after, category, description, reference_id, created_at
FROM credit_transactions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```
