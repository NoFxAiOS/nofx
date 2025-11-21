## Why

当前认证系统存在严重安全漏洞：
1. **登录绕过内测验证** - 任意用户注册后，即使系统开启内测模式，仍可直接登录，绕过了内测限制
2. **admin@localhost 自动登录** - 当 `admin_mode` 启用时，系统自动创建管理员账户，完全绕过认证机制
3. **一次性验证缺陷** - 内测码仅在注册时验证一次，登录时不再检查，违反了安全设计的"持续验证"原则

这些漏洞违背了内测期间的访问控制目标，可能导致未授权用户访问系统。

## What Changes

### 1. 登录强制内测码验证
- **MODIFIED**: 登录流程 (`api/server.go:handleLogin`)
  - 登录时检查系统是否开启内测模式
  - 如果开启，验证用户是否有关联的有效内测码
  - 内测码无效或已过期的用户禁止登录，返回明确错误信息

### 2. 移除 admin@localhost 自动登录
- **REMOVED**: `auth.AdminMode` 全局变量和相关逻辑
- **REMOVED**: `AuthContext.tsx` 中的 admin 自动登录代码
- **REMOVED**: `api/server.go` 中的 `handleGetSystemConfig` 返回的 `admin_mode` 字段
- **REMOVED**: 前端对 `admin_mode` 的所有检查和使用

### 3. 数据库 schema 更新
- **ADDED**: 用户表新增 `beta_code` 字段
  - 记录用户注册时使用的内测码
  - 用于登录时验证
- **MODIFIED**: 用户注册流程
  - 注册成功后，将内测码关联到用户账户
  - 标记该内测码为"已使用"状态

### 4. 前端优化
- **MODIFIED**: `AuthContext.tsx`
  - 移除对 `admin_mode` 的检查和自动登录
  - 简化初始化逻辑
- **MODIFIED**: 登录注册页面
  - 保持内测码输入字段（已有）
  - 优化错误提示，明确区分"内测码无效"和"密码错误"

## Impact

### 受影响的能力
- **specs/auth/spec.md** - 用户认证流程完全重构
- **specs/database/spec.md** - 数据库 schema 更新

### 受影响的代码
- **后端**:
  - `api/server.go` - 登录、注册、配置接口
  - `auth/auth.go` - 移除 AdminMode 相关代码
  - `config/database.go` - 新增用户表字段，验证内测码逻辑
- **前端**:
  - `src/contexts/AuthContext.tsx` - 移除 admin 模式
  - `src/components/LoginPage.tsx` - 错误提示优化
  - `src/components/RegisterPage.tsx` - 已实现，无需修改

### Breaking Changes
- **移除 admin@localhost 账户** - 现有管理员用户需使用真实账户登录
- **登录验证增强** - 内测模式下，无效内测码用户无法登录

### 向后兼容性
- ✅ 已注册用户不受影响（内测码已关联到账户）
- ✅ 非内测模式下，所有用户正常登录
- ❌ 依赖 `admin_mode` 的自动化工具需更新

## Migration Plan

### 阶段1: 数据库迁移
1. 为用户表添加 `beta_code` 列
2. 更新已有用户的 `beta_code` 字段（从注册记录中获取）
3. 验证迁移完整性

### 阶段2: 后端代码更新
1. 更新登录验证逻辑
2. 移除 AdminMode 相关代码
3. 更新系统配置接口
4. 测试所有认证流程

### 阶段3: 前端代码更新
1. 移除 admin 自动登录逻辑
2. 优化错误提示
3. 端到端测试

### 阶段4: 验证与清理
1. 验证内测模式下访问控制
2. 确认非内测模式不受影响
3. 清理旧的 AdminMode 配置
