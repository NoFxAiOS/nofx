# Frontend Routing Bug Fix

## 问题描述

用户报告点击 dashboard 和其他页面时出现黑屏问题。

## 根本原因

### 第一个问题: React Router 依赖冲突
- `LoginPage.tsx` 和 `RegisterPage.tsx` 使用了 `react-router-dom` 的 `useNavigate` hook
- `App.tsx` 使用自定义状态路由,没有提供 `<Router>` 上下文
- 导致错误: "useNavigate() may be used only in the context of a <Router> component"

### 第二个问题: 缺少显式路由处理
- `App.tsx` 中缺少 `/dashboard` 和 `/traders` 路由的显式处理
- 这些路由会 fall-through 到默认渲染逻辑
- 默认逻辑在某些情况下无法正确渲染认证页面

## 解决方案

### 1. 移除 React Router 依赖 (commit 4347833)

**修改文件:**
- `web/src/components/LoginPage.tsx`
- `web/src/components/RegisterPage.tsx`

**变更:**
```typescript
// 之前
import { useNavigate } from 'react-router-dom';
const navigate = useNavigate();
navigate('/path');

// 之后  
window.location.href = '/path';
```

### 2. 添加显式路由处理 (commit 530f3bf)

**修改文件:**
- `web/src/App.tsx`

**新增路由处理:**

#### `/dashboard` 路由
```typescript
if (route === '/dashboard') {
  if (!user || !token) {
    window.location.href = '/login'
    return null
  }
  return (
    <div className="min-h-screen" style={{ background: '#0B0E11', color: '#EAECEF' }}>
      <HeaderBar {...props} currentPage="trader" />
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
        <TraderDetailsPage
          {...allProps}
          isRefreshingAccount={isRefreshingAccount}
          onRefreshAccount={handleRefreshAccount}
        />
      </main>
    </div>
  )
}
```

#### `/traders` 路由
```typescript
if (route === '/traders') {
  if (!user || !token) {
    window.location.href = '/login'
    return null
  }
  return (
    <div className="min-h-screen" style={{ background: '#0B0E11', color: '#EAECEF' }}>
      <HeaderBar {...props} currentPage="traders" />
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
        <AITradersPage onTraderSelect={...} />
      </main>
    </div>
  )
}
```

### 3. 修复账户刷新功能集成

**更新 TraderDetailsPage 类型定义:**
```typescript
function TraderDetailsPage({
  // ...existing props
  isRefreshingAccount,
  onRefreshAccount,
}: {
  // ...existing types
  isRefreshingAccount?: boolean
  onRefreshAccount?: () => void
}) {
```

**条件渲染刷新按钮:**
```typescript
{onRefreshAccount && (
  <button onClick={onRefreshAccount} disabled={isRefreshingAccount}>
    {/* 刷新按钮 UI */}
  </button>
)}
```

## 测试结果

### 功能测试
- ✅ 登录页面正常显示
- ✅ 注册页面正常显示
- ✅ 重置密码页面正常显示
- ✅ Dashboard 页面正常显示 (之前黑屏)
- ✅ Traders 页面正常显示 (之前黑屏)
- ✅ 所有导航链接正常工作
- ✅ 账户余额刷新按钮正常工作

### 控制台检查
- ✅ 无 React Router 错误
- ✅ 无 useNavigate 相关错误
- ✅ 无其他 JavaScript 错误

### 代码扫描
- ✅ 整个代码库已无 react-router-dom 引用
- ✅ 所有路由都有显式处理器
- ✅ 认证检查正确实现

## 架构说明

### 自定义路由架构
项目使用**自定义状态路由**而非 react-router-dom:

1. **路由状态管理:**
   - `const [route, setRoute] = useState(window.location.pathname)`
   - 监听 `popstate` 和 `hashchange` 事件

2. **导航方法:**
   - 使用 `window.history.pushState()` + `setRoute()`
   - 或使用 `window.location.href` 进行完整页面导航

3. **页面渲染:**
   - 基于 `route` 状态的条件渲染
   - 每个路由都有显式的 `if (route === '/path')` 处理

### 最佳实践
- ✅ 所有组件应使用 `window.location.href` 或 `window.history.pushState` + `setRoute`
- ✅ 避免引入 react-router-dom
- ✅ 认证页面必须检查 `user` 和 `token`
- ✅ 未认证用户重定向到 `/login`

## 相关提交

1. `4347833` - fix(ui): remove react-router-dom dependency from auth pages
2. `530f3bf` - fix(ui): add explicit route handling for /dashboard and /traders
3. `e071bed` - docs: update CHANGELOG for account balance refresh and bug fix
4. `002745c` - docs: update CHANGELOG for dashboard/traders routing fix

## 影响范围

- **前端组件:** LoginPage, RegisterPage, App.tsx
- **路由:** /login, /register, /dashboard, /traders, /reset-password
- **功能:** 账户余额刷新按钮集成
- **用户体验:** 修复所有黑屏问题,确保流畅导航

## 预防措施

为防止类似问题再次发生:

1. **代码审查检查清单:**
   - 确认新组件不使用 react-router-dom
   - 验证所有路由都有显式处理
   - 测试认证和未认证状态

2. **测试要求:**
   - 手动测试所有页面导航流程
   - 检查控制台是否有错误
   - 验证认证重定向逻辑

3. **文档更新:**
   - 在 CONTRIBUTING.md 中说明路由架构
   - 添加导航最佳实践指南
