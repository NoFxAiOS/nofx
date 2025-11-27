# ai_models 表设计审计报告

**审计日期**: 2025-11-27  
**审计范围**: `ai_models` 表结构、数据模型、查询逻辑  
**审计结论**: ⚠️ **存在设计缺陷，需要重构**

---

## 1. 当前设计分析

### 1.1 表结构

```sql
CREATE TABLE IF NOT EXISTS ai_models (
    id TEXT PRIMARY KEY,           -- 主键，格式不统一
    user_id TEXT NOT NULL DEFAULT 'default',  -- 用户ID
    name TEXT NOT NULL,            -- 模型名称
    provider TEXT NOT NULL,        -- 提供商 (deepseek/qwen)
    enabled BOOLEAN DEFAULT false, -- 是否启用
    api_key TEXT DEFAULT '',       -- API密钥
    custom_api_url TEXT DEFAULT '',     -- 自定义API地址
    custom_model_name TEXT DEFAULT '',  -- 自定义模型名称
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

### 1.2 数据存储模式

当前设计将**系统配置**和**用户配置**混合存储在同一张表中：

| 数据类型 | user_id | 示例 ID | 说明 |
|----------|---------|---------|------|
| 系统默认配置 | `default` | `deepseek`, `qwen` | 系统初始化时创建 |
| 用户自定义配置 | `user_xxx` | `user_xxx_deepseek` | 用户修改时创建 |

### 1.3 查询逻辑分析

```go
// GetAIModels - 只查询用户自己的配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
    rows, err := d.query(`
        SELECT ... FROM ai_models WHERE user_id = ? ORDER BY id
    `, userID)
    // ...
}
```

**问题**: 如果用户没有自定义配置，返回空列表，无法获取系统默认配置。

---

## 2. 设计问题诊断

### 2.1 🔴 严重问题

#### 问题1: 主键设计混乱

```
系统配置: id = "deepseek" (简单字符串)
用户配置: id = "user_xxx_deepseek" (复合格式)
```

**影响**:
- 主键格式不统一，难以维护
- 无法通过主键快速定位用户配置
- `ON CONFLICT (id)` 逻辑复杂

#### 问题2: 缺少复合唯一约束

当前只有 `id` 作为主键，没有 `(user_id, provider)` 的唯一约束。

**影响**:
- 同一用户可能创建多个相同 provider 的配置
- 数据一致性难以保证

#### 问题3: 系统配置与用户配置耦合

```go
// 初始化系统默认配置
INSERT INTO ai_models (id, user_id, name, provider, enabled)
VALUES ('deepseek', 'default', 'DeepSeek', 'deepseek', false)

// 用户配置也在同一张表
INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key)
VALUES ('user_xxx_deepseek', 'user_xxx', 'DeepSeek', 'deepseek', true, 'sk-xxx')
```

**影响**:
- 系统配置变更需要考虑用户数据
- 无法独立管理系统支持的模型列表
- 查询逻辑复杂（需要 fallback 到 default）

### 2.2 🟡 中等问题

#### 问题4: 查询效率低下

```go
// 当前查询只按 user_id 过滤
SELECT ... FROM ai_models WHERE user_id = ?
```

**1万用户场景分析**:
- 假设每用户平均 2 个模型配置
- 总记录数: 2 + 20,000 = 20,002 条
- 每次查询需要扫描 user_id 索引

**缺少的索引**:
```sql
-- 当前没有这些索引
CREATE INDEX idx_ai_models_user_id ON ai_models(user_id);
CREATE INDEX idx_ai_models_provider ON ai_models(provider);
```

#### 问题5: UpdateAIModel 逻辑过于复杂

```go
func (d *Database) UpdateAIModel(userID, id string, ...) error {
    // 1. 先尝试精确匹配 ID
    // 2. 如果失败，尝试用 provider 匹配（兼容旧版）
    // 3. 如果还失败，创建新记录
    // 4. 创建时需要推断 provider 和 name
}
```

**影响**:
- 代码复杂度高，容易出错
- 多次数据库查询，性能差
- 兼容逻辑难以维护

### 2.3 🟢 轻微问题

#### 问题6: 敏感数据未加密

`api_key` 字段以明文存储，存在安全风险。

#### 问题7: 缺少软删除机制

删除配置是物理删除，无法恢复。

---

## 3. 1万用户规模评估

### 3.1 数据量预估

| 场景 | 记录数 | 说明 |
|------|--------|------|
| 系统配置 | 2 | deepseek, qwen |
| 用户配置 (保守) | 10,000 | 每用户1个 |
| 用户配置 (中等) | 20,000 | 每用户2个 |
| 用户配置 (激进) | 50,000 | 每用户5个 |

### 3.2 性能瓶颈分析

#### 查询性能

```sql
-- 当前查询 (无索引)
SELECT * FROM ai_models WHERE user_id = 'user_xxx';
-- 预计耗时: 5-20ms (全表扫描)

-- 添加索引后
SELECT * FROM ai_models WHERE user_id = 'user_xxx';
-- 预计耗时: 0.5-2ms (索引查询)
```

#### 写入性能

```sql
-- 当前写入 (需要检查主键冲突)
INSERT INTO ai_models (...) ON CONFLICT (id) DO NOTHING;
-- 预计耗时: 2-5ms

-- 如果有复合唯一约束
INSERT INTO ai_models (...) ON CONFLICT (user_id, provider) DO UPDATE ...;
-- 预计耗时: 1-3ms (更高效)
```

### 3.3 结论

| 指标 | 当前设计 | 1万用户后 | 评估 |
|------|----------|-----------|------|
| 查询延迟 | 1-5ms | 5-20ms | ⚠️ 需要索引 |
| 写入延迟 | 2-5ms | 3-8ms | ✅ 可接受 |
| 存储空间 | ~100KB | ~5MB | ✅ 可接受 |
| 代码复杂度 | 高 | 更高 | 🔴 需要重构 |

---

## 4. 推荐重构方案

### 4.1 方案A: 分表设计（推荐）

将系统配置和用户配置分离到两张表：

```sql
-- 系统支持的AI模型（只读，管理员维护）
CREATE TABLE ai_model_providers (
    id TEXT PRIMARY KEY,           -- 'deepseek', 'qwen'
    name TEXT NOT NULL,            -- 'DeepSeek AI'
    provider TEXT NOT NULL UNIQUE, -- 'deepseek'
    default_api_url TEXT,          -- 默认API地址
    default_model_name TEXT,       -- 默认模型名称
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 用户的AI模型配置（用户数据）
CREATE TABLE user_ai_configs (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES users(id),
    provider_id TEXT NOT NULL REFERENCES ai_model_providers(id),
    enabled BOOLEAN DEFAULT false,
    api_key TEXT DEFAULT '',       -- 建议加密存储
    custom_api_url TEXT DEFAULT '',
    custom_model_name TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 复合唯一约束：每用户每provider只能有一个配置
    UNIQUE (user_id, provider_id)
);

-- 索引
CREATE INDEX idx_user_ai_configs_user_id ON user_ai_configs(user_id);
CREATE INDEX idx_user_ai_configs_provider ON user_ai_configs(provider_id);
```

**优点**:
- 系统配置和用户配置完全分离
- 主键设计清晰（UUID）
- 复合唯一约束保证数据一致性
- 查询逻辑简单

**查询示例**:
```go
// 获取用户的AI模型配置（带系统默认值）
func (d *Database) GetUserAIConfigs(userID string) ([]*AIModelConfig, error) {
    rows, err := d.query(`
        SELECT 
            p.id as provider_id,
            p.name,
            p.provider,
            COALESCE(u.enabled, false) as enabled,
            COALESCE(u.api_key, '') as api_key,
            COALESCE(u.custom_api_url, p.default_api_url) as api_url,
            COALESCE(u.custom_model_name, p.default_model_name) as model_name
        FROM ai_model_providers p
        LEFT JOIN user_ai_configs u ON p.id = u.provider_id AND u.user_id = $1
        WHERE p.is_active = true
        ORDER BY p.sort_order
    `, userID)
    // ...
}
```

### 4.2 方案B: 优化现有表（最小改动）

如果不想大改，可以优化现有设计：

```sql
-- 1. 添加索引
CREATE INDEX idx_ai_models_user_id ON ai_models(user_id);
CREATE INDEX idx_ai_models_user_provider ON ai_models(user_id, provider);

-- 2. 添加复合唯一约束（需要先清理重复数据）
ALTER TABLE ai_models ADD CONSTRAINT uq_ai_models_user_provider 
    UNIQUE (user_id, provider);

-- 3. 标准化主键格式
-- 系统配置: id = 'system_deepseek'
-- 用户配置: id = 'user_{user_id}_{provider}'
```

**优化后的查询**:
```go
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
    // 使用 UNION 合并系统配置和用户配置
    rows, err := d.query(`
        SELECT id, user_id, name, provider, enabled, api_key, 
               custom_api_url, custom_model_name
        FROM ai_models 
        WHERE user_id = $1
        
        UNION ALL
        
        SELECT id, 'default' as user_id, name, provider, false as enabled, 
               '' as api_key, '' as custom_api_url, '' as custom_model_name
        FROM ai_models 
        WHERE user_id = 'default' 
          AND provider NOT IN (SELECT provider FROM ai_models WHERE user_id = $1)
        
        ORDER BY provider
    `, userID)
    // ...
}
```

### 4.3 方案对比

| 维度 | 方案A (分表) | 方案B (优化) |
|------|-------------|-------------|
| 改动量 | 大 | 小 |
| 代码清晰度 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| 查询性能 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| 扩展性 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| 数据一致性 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| 迁移风险 | 中 | 低 |

---

## 5. 迁移建议

### 5.1 短期优化（立即执行）

```sql
-- 1. 添加索引（无风险）
CREATE INDEX IF NOT EXISTS idx_ai_models_user_id ON ai_models(user_id);

-- 2. 检查重复数据
SELECT user_id, provider, COUNT(*) 
FROM ai_models 
GROUP BY user_id, provider 
HAVING COUNT(*) > 1;
```

### 5.2 中期重构（1-2周）

1. 创建新表结构（方案A）
2. 编写数据迁移脚本
3. 更新 Go 代码中的查询逻辑
4. 并行运行新旧逻辑，验证数据一致性
5. 切换到新表，删除旧表

### 5.3 迁移脚本示例

```sql
-- 迁移数据到新表
INSERT INTO ai_model_providers (id, name, provider, is_active)
SELECT DISTINCT provider, name, provider, true
FROM ai_models
WHERE user_id = 'default';

INSERT INTO user_ai_configs (user_id, provider_id, enabled, api_key, custom_api_url, custom_model_name)
SELECT user_id, provider, enabled, api_key, custom_api_url, custom_model_name
FROM ai_models
WHERE user_id != 'default';
```

---

## 6. 总结

### 6.1 核心问题

1. **设计耦合**: 系统配置和用户配置混在一张表
2. **主键混乱**: ID 格式不统一，难以维护
3. **缺少约束**: 没有复合唯一约束，可能产生重复数据
4. **缺少索引**: 1万用户后查询性能会下降

### 6.2 建议

| 优先级 | 行动 | 预计耗时 |
|--------|------|----------|
| P0 | 添加 `user_id` 索引 | 5分钟 |
| P1 | 清理重复数据 | 30分钟 |
| P2 | 重构为分表设计（方案A） | 1-2周 |
| P3 | API Key 加密存储 | 1天 |

### 6.3 1万用户适应性评估

| 评估项 | 当前状态 | 优化后 |
|--------|----------|--------|
| 能否支持1万用户 | ⚠️ 勉强可以 | ✅ 完全支持 |
| 查询性能 | 5-20ms | <2ms |
| 代码可维护性 | 差 | 好 |
| 数据一致性 | 有风险 | 有保障 |

---

**审计结论**: 当前设计在小规模（<1000用户）下可以工作，但存在明显的设计缺陷。建议在用户量增长前进行重构，采用**方案A（分表设计）**，将系统配置和用户配置分离，提高代码清晰度和查询性能。
