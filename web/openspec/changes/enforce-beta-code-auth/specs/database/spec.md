## MODIFIED Requirements

### Requirement: User Table Schema
用户表 SHALL 包含 beta_code 字段，用于存储用户关联的内测码。

**Previous Schema**:
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    -- 其他字段...
);
```

**New Schema**:
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    beta_code TEXT,  -- 新增：关联的内测码
    -- 其他字段...
);
```

#### Migration:
- 为现有用户表的 `beta_code` 字段设置默认值 NULL
- 从 `beta_codes` 表的 `used_by` 字段回填现有用户的内测码

#### Scenario: 创建新用户（内测模式）
- **WHEN** 用户注册（内测码有效）
- **THEN** 系统将内测码存储到用户的 `beta_code` 字段

#### Scenario: 创建新用户（非内测模式）
- **WHEN** 用户注册（无内测码要求）
- **THEN** 用户的 `beta_code` 字段设置为 NULL

#### Scenario: 登录验证内测码
- **WHEN** 用户登录（内测模式开启）
- **THEN** 系统查询用户的 `beta_code` 字段
- **AND** 验证该内测码在 `beta_codes` 表中是否有效

### Requirement: Beta Codes Table
系统 SHALL 支持查询内测码的当前状态，包括是否有效和已使用情况。

**Current Schema**:
```sql
CREATE TABLE beta_codes (
    code TEXT PRIMARY KEY,
    used INTEGER DEFAULT 0,
    used_by TEXT,
    used_at DATETIME
);
```

**No Schema Change Required**

#### Scenario: 验证内测码有效性
- **WHEN** 查询内测码状态
- **THEN** 返回 `used` 字段（0=有效，1=已使用）
- **AND** 返回 `used_by` 和 `used_at` 字段（如果已使用）

## ADDED Requirements

### Requirement: User Beta Code Retrieval
数据库 SHALL 提供查询用户关联内测码的功能。

#### Scenario: 获取用户的内测码
- **WHEN** 查询用户信息
- **THEN** 返回用户的 `beta_code` 字段值
- **AND** 如果用户无内测码，返回 NULL

### Requirement: Beta Code Validation
数据库 SHALL 提供验证内测码是否仍然有效的功能。

**Definition**: 内测码有效 = 内测码存在 AND 未被标记为已使用

#### Scenario: 内测码有效
- **GIVEN** 内测码存在
- **AND** `used` 字段为 0
- **WHEN** 验证内测码
- **THEN** 返回 true

#### Scenario: 内测码已使用
- **GIVEN** 内测码存在
- **AND** `used` 字段为 1
- **WHEN** 验证内测码
- **THEN** 返回 false

#### Scenario: 内测码不存在
- **GIVEN** 内测码不存在于表中
- **WHEN** 验证内测码
- **THEN** 返回 false

## REMOVED Requirements

### Requirement: System Config Admin Mode
**Reason**: 移除 `admin_mode` 配置项，简化系统配置
**Migration**: 管理员用户需通过正常注册流程或直接数据库插入创建

#### Previous Behavior:
系统配置表存储 `admin_mode` 键值，控制是否启用管理员模式。

#### Impact:
- `GetSystemConfig("admin_mode")` 调用需要移除
- 所有依赖 `admin_mode` 的逻辑需要重构
