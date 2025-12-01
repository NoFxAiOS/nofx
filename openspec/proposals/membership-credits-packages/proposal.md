# 会员积分套餐系统 - 精简提案

## 提案概述

| 属性 | 值 |
|------|-----|
| **提案标题** | 会员积分套餐系统 |
| **提案类型** | 核心商业功能 |
| **优先级** | P0 |
| **预计工作量** | 3-5天 |

## 核心需求

1. **积分购买**: 用户用 10 USDT/USDC 可买 500 积分
2. **多套餐支持**: 支持配置多种套餐（如 20U 买 1200 积分）
3. **用户积分展示**: 用户信息页面显示积分余额
4. **高内聚低耦合**: 不影响现有功能
5. **100%测试覆盖**: 所有新增代码需要单元测试

---

## 设计原则

### 三层架构理念

```
┌──────────────────────────────────────────┐
│  现象层: 用户看到积分余额、购买套餐界面    │
├──────────────────────────────────────────┤
│  本质层: 套餐配置表 + 用户积分账户表       │
├──────────────────────────────────────────┤
│  哲学层: 单一职责、最小依赖、可扩展性      │
└──────────────────────────────────────────┘
```

### 高内聚低耦合实现策略

1. **独立模块**: 积分系统作为独立模块，不修改现有 `users` 表结构
2. **外键关联**: 通过 `user_id` 外键关联，保持数据一致性
3. **接口隔离**: 新增独立 API 端点，不影响现有接口
4. **服务分层**: Repository -> Service -> Handler 三层分离

---

## 数据库设计

### 表结构设计

#### 1. 积分套餐配置表 `credit_packages`

```sql
-- 积分套餐配置表
-- 职责：存储可购买的套餐配置，与用户表完全解耦
CREATE TABLE IF NOT EXISTS credit_packages (
    id TEXT PRIMARY KEY,                    -- UUID
    name TEXT NOT NULL,                     -- 套餐名称（中文）
    name_en TEXT,                           -- 套餐名称（英文）
    description TEXT,                       -- 套餐描述
    price_usdt DECIMAL(10,2) NOT NULL,      -- 价格（USDT/USDC）
    credits INTEGER NOT NULL,               -- 积分数量
    bonus_credits INTEGER DEFAULT 0,        -- 赠送积分
    is_active BOOLEAN DEFAULT TRUE,         -- 是否启用
    is_recommended BOOLEAN DEFAULT FALSE,   -- 是否推荐
    sort_order INTEGER DEFAULT 0,           -- 排序
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_credit_packages_active ON credit_packages(is_active);
CREATE INDEX IF NOT EXISTS idx_credit_packages_sort ON credit_packages(sort_order);
```

#### 2. 用户积分账户表 `user_credits`

```sql
-- 用户积分账户表
-- 职责：记录用户的积分余额，一对一关联 users 表
CREATE TABLE IF NOT EXISTS user_credits (
    id TEXT PRIMARY KEY,                    -- UUID
    user_id TEXT NOT NULL UNIQUE,           -- 用户ID（唯一约束）
    available_credits INTEGER DEFAULT 0,    -- 可用积分
    total_credits INTEGER DEFAULT 0,        -- 累计获得积分
    used_credits INTEGER DEFAULT 0,         -- 已使用积分
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_user_credits_user_id ON user_credits(user_id);
```

#### 3. 积分流水表 `credit_transactions`

```sql
-- 积分流水表
-- 职责：记录所有积分变动，可追溯、可审计
CREATE TABLE IF NOT EXISTS credit_transactions (
    id TEXT PRIMARY KEY,                    -- UUID
    user_id TEXT NOT NULL,                  -- 用户ID
    type TEXT NOT NULL,                     -- 类型: credit(增加)/debit(扣减)
    amount INTEGER NOT NULL,                -- 变动数量
    balance_before INTEGER NOT NULL,        -- 变动前余额
    balance_after INTEGER NOT NULL,         -- 变动后余额
    category TEXT NOT NULL,                 -- 分类: purchase/consume/gift/refund
    description TEXT,                       -- 描述
    reference_id TEXT,                      -- 关联ID（订单ID等）
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_credit_transactions_user_id ON credit_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_created_at ON credit_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_transactions_type ON credit_transactions(type);
```

### 初始数据

```sql
-- 插入默认套餐配置
INSERT INTO credit_packages (id, name, name_en, description, price_usdt, credits, bonus_credits, is_active, is_recommended, sort_order)
VALUES
    ('pkg_starter', '入门套餐', 'Starter Pack', '适合新用户体验', 5.00, 200, 0, TRUE, FALSE, 1),
    ('pkg_standard', '标准套餐', 'Standard Pack', '最受欢迎的选择', 10.00, 500, 0, TRUE, TRUE, 2),
    ('pkg_premium', '高级套餐', 'Premium Pack', '超值大礼包', 20.00, 1200, 0, TRUE, FALSE, 3),
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
    sort_order = EXCLUDED.sort_order;
```

---

## API 设计

### 套餐相关 API

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/credit-packages` | 获取所有启用套餐 | 可选 |
| GET | `/api/v1/credit-packages/:id` | 获取套餐详情 | 可选 |

### 用户积分 API

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/api/v1/user/credits` | 获取用户积分余额 | 必需 |
| GET | `/api/v1/user/credits/transactions` | 获取积分流水（分页） | 必需 |

### 管理员 API

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| POST | `/api/v1/admin/credit-packages` | 创建套餐 | Admin |
| PUT | `/api/v1/admin/credit-packages/:id` | 更新套餐 | Admin |
| DELETE | `/api/v1/admin/credit-packages/:id` | 删除套餐（软删除） | Admin |
| POST | `/api/v1/admin/users/:id/credits/adjust` | 调整用户积分 | Admin |

### API 响应格式

#### 获取套餐列表响应
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "packages": [
            {
                "id": "pkg_standard",
                "name": "标准套餐",
                "name_en": "Standard Pack",
                "description": "最受欢迎的选择",
                "price_usdt": 10.00,
                "credits": 500,
                "bonus_credits": 0,
                "total_credits": 500,
                "is_recommended": true
            }
        ]
    }
}
```

#### 获取用户积分响应
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "available_credits": 450,
        "total_credits": 500,
        "used_credits": 50
    }
}
```

---

## 代码架构

### 目录结构

```
backend/
├── internal/
│   ├── credits/                    # 积分模块（独立）
│   │   ├── repository.go           # 数据访问层
│   │   ├── repository_test.go      # Repository 测试
│   │   ├── service.go              # 业务逻辑层
│   │   ├── service_test.go         # Service 测试
│   │   ├── handler.go              # HTTP 处理层
│   │   ├── handler_test.go         # Handler 测试
│   │   └── models.go               # 数据模型
│   └── ...
├── database/
│   └── migrations/
│       └── 20251201_credits/       # 积分系统迁移脚本
│           └── 001_create_tables.sql
└── ...
```

### 核心接口定义

```go
// models.go - 数据模型
type CreditPackage struct {
    ID            string    `json:"id"`
    Name          string    `json:"name"`
    NameEN        string    `json:"name_en"`
    Description   string    `json:"description"`
    PriceUSDT     float64   `json:"price_usdt"`
    Credits       int       `json:"credits"`
    BonusCredits  int       `json:"bonus_credits"`
    IsActive      bool      `json:"is_active"`
    IsRecommended bool      `json:"is_recommended"`
    SortOrder     int       `json:"sort_order"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}

type UserCredits struct {
    ID               string    `json:"id"`
    UserID           string    `json:"user_id"`
    AvailableCredits int       `json:"available_credits"`
    TotalCredits     int       `json:"total_credits"`
    UsedCredits      int       `json:"used_credits"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}

type CreditTransaction struct {
    ID            string    `json:"id"`
    UserID        string    `json:"user_id"`
    Type          string    `json:"type"`          // credit/debit
    Amount        int       `json:"amount"`
    BalanceBefore int       `json:"balance_before"`
    BalanceAfter  int       `json:"balance_after"`
    Category      string    `json:"category"`      // purchase/consume/gift/refund
    Description   string    `json:"description"`
    ReferenceID   string    `json:"reference_id"`
    CreatedAt     time.Time `json:"created_at"`
}
```

```go
// repository.go - 数据访问接口
type CreditRepository interface {
    // 套餐
    GetActivePackages(ctx context.Context) ([]CreditPackage, error)
    GetPackageByID(ctx context.Context, id string) (*CreditPackage, error)
    CreatePackage(ctx context.Context, pkg *CreditPackage) error
    UpdatePackage(ctx context.Context, pkg *CreditPackage) error
    DeletePackage(ctx context.Context, id string) error

    // 用户积分
    GetUserCredits(ctx context.Context, userID string) (*UserCredits, error)
    CreateUserCredits(ctx context.Context, credits *UserCredits) error
    UpdateUserCredits(ctx context.Context, credits *UserCredits) error

    // 流水
    CreateTransaction(ctx context.Context, tx *CreditTransaction) error
    GetUserTransactions(ctx context.Context, userID string, page, limit int) ([]CreditTransaction, int, error)
}
```

```go
// service.go - 业务逻辑接口
type CreditService interface {
    // 套餐查询
    GetActivePackages(ctx context.Context) ([]CreditPackage, error)
    GetPackageByID(ctx context.Context, id string) (*CreditPackage, error)

    // 用户积分
    GetUserCredits(ctx context.Context, userID string) (*UserCredits, error)
    AddCredits(ctx context.Context, userID string, amount int, category, description, refID string) error
    DeductCredits(ctx context.Context, userID string, amount int, category, description, refID string) error
    HasEnoughCredits(ctx context.Context, userID string, amount int) (bool, error)

    // 流水
    GetUserTransactions(ctx context.Context, userID string, page, limit int) ([]CreditTransaction, int, error)

    // 管理
    CreatePackage(ctx context.Context, pkg *CreditPackage) error
    UpdatePackage(ctx context.Context, pkg *CreditPackage) error
    DeletePackage(ctx context.Context, id string) error
    AdjustUserCredits(ctx context.Context, userID string, amount int, reason string) error
}
```

---

## 测试策略

### 测试覆盖率目标

| 模块 | 目标覆盖率 |
|------|-----------|
| Repository | >= 90% |
| Service | >= 95% |
| Handler | >= 85% |
| **总体** | **>= 90%** |

### 测试用例设计

#### Repository 层测试

```go
// repository_test.go

func TestGetActivePackages(t *testing.T) {
    // 测试：获取所有启用的套餐
    // 预期：返回所有 is_active=true 的套餐，按 sort_order 排序
}

func TestGetPackageByID(t *testing.T) {
    // 测试：根据ID获取套餐
    // 场景1：存在的ID -> 返回套餐
    // 场景2：不存在的ID -> 返回 nil, error
}

func TestGetUserCredits(t *testing.T) {
    // 测试：获取用户积分
    // 场景1：存在记录 -> 返回积分信息
    // 场景2：不存在记录 -> 自动创建并返回
}

func TestUpdateUserCredits(t *testing.T) {
    // 测试：更新用户积分
    // 验证：并发更新时的数据一致性
}
```

#### Service 层测试

```go
// service_test.go

func TestAddCredits(t *testing.T) {
    // 测试：增加积分
    // 验证：
    // 1. available_credits 正确增加
    // 2. total_credits 正确增加
    // 3. 流水记录正确创建
}

func TestDeductCredits(t *testing.T) {
    // 测试：扣减积分
    // 场景1：余额充足 -> 扣减成功
    // 场景2：余额不足 -> 返回错误
    // 验证：
    // 1. available_credits 正确减少
    // 2. used_credits 正确增加
    // 3. 流水记录正确创建
}

func TestHasEnoughCredits(t *testing.T) {
    // 测试：检查积分是否充足
    // 场景1：余额 >= 需求 -> true
    // 场景2：余额 < 需求 -> false
}

func TestConcurrentDeduction(t *testing.T) {
    // 测试：并发扣减
    // 验证：100并发扣减不会出现数据不一致
}
```

#### Handler 层测试

```go
// handler_test.go

func TestGetPackagesHandler(t *testing.T) {
    // 测试：GET /api/v1/credit-packages
    // 验证：返回正确的套餐列表和状态码
}

func TestGetUserCreditsHandler(t *testing.T) {
    // 测试：GET /api/v1/user/credits
    // 场景1：已认证用户 -> 返回积分信息
    // 场景2：未认证用户 -> 401
}

func TestAdjustCreditsHandler(t *testing.T) {
    // 测试：POST /api/v1/admin/users/:id/credits/adjust
    // 场景1：管理员操作 -> 成功
    // 场景2：非管理员操作 -> 403
}
```

---

## 前端集成

### 用户信息展示

在用户信息区域添加积分显示：

```tsx
// 用户信息组件中添加积分展示
<div className="user-credits">
    <span className="credits-label">{t('credits.balance')}</span>
    <span className="credits-value">{userCredits.available_credits}</span>
</div>
```

### i18n 翻译

```json
// zh-CN.json
{
    "credits": {
        "balance": "积分余额",
        "available": "可用积分",
        "total": "累计积分",
        "used": "已使用",
        "packages": "积分套餐",
        "buy": "购买",
        "price": "价格",
        "get": "获得"
    }
}

// en.json
{
    "credits": {
        "balance": "Credit Balance",
        "available": "Available Credits",
        "total": "Total Credits",
        "used": "Used",
        "packages": "Credit Packages",
        "buy": "Buy",
        "price": "Price",
        "get": "Get"
    }
}
```

---

## 实施计划

### 阶段划分

| 阶段 | 时间 | 内容 |
|------|------|------|
| **Phase 1** | Day 1 | 数据库设计 + 迁移脚本 |
| **Phase 2** | Day 2 | Repository 层 + 测试 |
| **Phase 3** | Day 3 | Service 层 + 测试 |
| **Phase 4** | Day 4 | Handler 层 + API + 测试 |
| **Phase 5** | Day 5 | 前端集成 + 联调 + 文档 |

### 验收标准

1. **功能验收**
   - [ ] 套餐列表正确返回
   - [ ] 用户积分余额正确显示
   - [ ] 积分增减操作正确
   - [ ] 流水记录完整

2. **质量验收**
   - [ ] 单元测试覆盖率 >= 90%
   - [ ] 无 P0/P1 级别 Bug
   - [ ] API 响应时间 < 100ms

3. **兼容性验收**
   - [ ] 不影响现有用户功能
   - [ ] 数据库迁移可回滚
   - [ ] 现有测试全部通过

---

## 风险与应对

| 风险 | 概率 | 影响 | 应对策略 |
|------|------|------|----------|
| 并发积分操作数据不一致 | 中 | 高 | 使用数据库事务 + 行级锁 |
| 现有功能受影响 | 低 | 高 | 独立模块设计 + 充分测试 |
| 迁移脚本执行失败 | 低 | 中 | 提供回滚脚本 + 分步执行 |

---

## 未来扩展

本设计预留了以下扩展点：

1. **支付系统**: 添加 `credit_orders` 表支持在线支付
2. **积分消费**: 添加 `credit_consumption_items` 表配置消费项目
3. **会员等级**: 基于累计消费实现等级体系
4. **有效期管理**: 扩展 `user_credits` 支持积分有效期

---

## 总结

本提案采用**最小可行设计**，聚焦核心需求：

- 3张表，清晰职责分离
- 独立模块，不影响现有功能
- 100%测试覆盖，保证质量
- 5天完成，快速交付

**设计哲学**：
> "简单即是可靠。三张表解决核心问题，为未来扩展留足空间，却不预设复杂度。"
