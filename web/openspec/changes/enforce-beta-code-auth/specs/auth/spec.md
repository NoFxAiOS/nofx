## MODIFIED Requirements

### Requirement: User Login
系统 SHALL 验证用户登录凭证。如果系统开启内测模式，系统 SHALL 验证用户是否有关联的有效内测码。

**Previous Behavior**:
用户只需提供邮箱和密码即可登录，不检查内测模式状态。

**New Behavior**:
- 用户提供邮箱和密码
- 系统验证邮箱和密码正确性
- 如果系统开启内测模式，系统检查用户是否有有效的内测码
- 如果内测码无效或不存在，返回错误"内测码无效，请联系管理员"
- 如果内测码有效，返回 JWT token

#### Scenario: 登录成功（非内测模式）
- **GIVEN** 系统未开启内测模式
- **WHEN** 用户提供正确的邮箱和密码
- **THEN** 系统返回 JWT token 和用户信息

#### Scenario: 登录成功（内测模式）
- **GIVEN** 系统开启内测模式
- **AND** 用户有关联的有效内测码
- **WHEN** 用户提供正确的邮箱和密码
- **THEN** 系统返回 JWT token 和用户信息

#### Scenario: 内测模式下登录失败（无内测码）
- **GIVEN** 系统开启内测模式
- **AND** 用户没有关联的内测码
- **WHEN** 用户提供正确的邮箱和密码
- **THEN** 系统返回 401 错误，错误信息为"内测码无效，请联系管理员"

#### Scenario: 内测码无效
- **GIVEN** 系统开启内测模式
- **AND** 用户关联的内测码已被标记为无效
- **WHEN** 用户提供正确的邮箱和密码
- **THEN** 系统返回 401 错误，错误信息为"内测码无效，请联系管理员"

#### Scenario: 密码错误
- **GIVEN** 任何模式
- **WHEN** 用户提供错误的密码
- **THEN** 系统返回 401 错误，错误信息为"邮箱或密码错误"

#### Scenario: 用户不存在
- **GIVEN** 任何模式
- **WHEN** 用户提供不存在的邮箱
- **THEN** 系统返回 401 错误，错误信息为"邮箱或密码错误"

### Requirement: User Registration
系统 SHALL 验证用户注册信息。如果系统开启内测模式，系统 SHALL 验证内测码有效性。注册成功后，系统 SHALL 将内测码关联到用户账户。

**Previous Behavior**:
用户注册时验证内测码，但不将内测码关联到用户账户。

**New Behavior**:
- 用户提供邮箱、密码和内测码（如果需要）
- 系统验证邮箱格式和密码强度
- 如果系统开启内测模式，验证内测码有效且未使用
- 验证通过后，创建用户账户并将内测码关联到该账户
- 标记内测码为"已使用"状态
- 返回注册成功信息和登录 token

#### Scenario: 注册成功（非内测模式）
- **GIVEN** 系统未开启内测模式
- **WHEN** 用户提供有效的邮箱和密码
- **THEN** 系统创建用户账户，返回成功信息

#### Scenario: 注册成功（内测模式）
- **GIVEN** 系统开启内测模式
- **AND** 用户提供有效的内测码
- **WHEN** 用户提供有效的邮箱和密码
- **THEN** 系统创建用户账户，关联内测码，返回成功信息

#### Scenario: 内测模式注册失败（无内测码）
- **GIVEN** 系统开启内测模式
- **WHEN** 用户未提供内测码
- **THEN** 系统返回 400 错误，错误信息为"内测码不能为空"

#### Scenario: 内测码无效
- **GIVEN** 系统开启内测模式
- **WHEN** 用户提供无效的内测码
- **THEN** 系统返回 400 错误，错误信息为"内测码无效或已被使用"

### Requirement: Admin Mode Removal
系统 SHALL NOT 支持 admin@localhost 自动登录模式。所有用户 SHALL 通过注册和登录流程访问系统。

**Previous Behavior**:
当 `admin_mode` 配置启用时，前端自动创建 admin@localhost 用户并登录。

**New Behavior**:
- 移除 `admin_mode` 配置项
- 移除前端对 `admin_mode` 的检查
- 移除 admin@localhost 自动登录逻辑
- 所有用户必须通过注册和登录流程

#### Scenario: 访问需要认证的页面（未登录）
- **GIVEN** 用户未登录
- **WHEN** 用户访问需要认证的页面
- **THEN** 系统重定向到登录页面

#### Scenario: 访问需要认证的页面（已登录）
- **GIVEN** 用户已登录（通过正常流程）
- **WHEN** 用户访问需要认证的页面
- **THEN** 系统显示页面内容

## REMOVED Requirements

### Requirement: Admin Mode Auto Login
**Reason**: 绕过认证机制的安全漏洞，违背了内测期间的访问控制目标
**Migration**: 管理员用户需通过正常注册流程创建账户，或在数据库中直接创建管理员用户记录

#### Previous Behavior:
当 `admin_mode` 启用时，前端自动模拟 admin@localhost 用户登录。

#### Impact:
- 前端代码中所有 `admin_mode` 检查需要移除
- 后端 `auth.go` 中的 `AdminMode` 变量和相关函数需要移除
- API 响应中的 `admin_mode` 字段需要移除

## ADDED Requirements

### Requirement: Beta Code Association
系统 SHALL 将用户注册时使用的内测码关联到用户账户。登录时，系统 SHALL 验证用户的内测码有效性。

#### Database Schema:
用户表新增字段：
- `beta_code` (TEXT) - 用户关联的内测码

#### Scenario: 内测码关联到用户
- **WHEN** 用户成功注册（使用内测码）
- **THEN** 系统将内测码存储到用户的 `beta_code` 字段
- **AND** 标记该内测码为"已使用"状态

#### Scenario: 登录时验证内测码
- **WHEN** 用户尝试登录（内测模式开启）
- **THEN** 系统检查用户的 `beta_code` 字段
- **AND** 验证该内测码在 `beta_codes` 表中是否仍然有效
- **AND** 只有验证通过才允许登录

### Requirement: Enhanced Error Messages
系统 SHALL 提供清晰的错误消息，帮助用户理解登录失败的原因。

#### Scenario: 内测码相关错误
- **WHEN** 用户因内测码问题无法登录
- **THEN** 错误消息明确指出"内测码无效，请联系管理员"
- **AND** 不透露具体的失败原因（如内测码不存在或已过期）

#### Scenario: 密码错误
- **WHEN** 用户因密码错误无法登录
- **THEN** 错误消息显示"邮箱或密码错误"
- **AND** 不透露具体是邮箱错误还是密码错误
