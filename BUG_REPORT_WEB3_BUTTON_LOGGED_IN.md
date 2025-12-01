# OpenSpec Bug Report: Web3 钱包按钮登录后不显示

## Bug ID
`BUG-2025-12-01-WEB3-BUTTON-LOGGED-IN`

## 问题描述
用户登录后，在 https://www.agentrade.xyz/ 页面左上角的 Web3 钱包连接按钮消失。按钮应该始终显示（除了在登录/注册页面），无论用户是否已登录。

## 影响范围
- **环境**: 生产环境 (Vercel 部署) + 开发环境 (Replit)
- **页面**: 首页 (LandingPage)、竞赛页面等
- **触发条件**: 用户登录后
- **用户影响**: 登录用户无法使用 Web3 钱包连接功能

---

## 调查的 3 个可能原因

### 可能原因 1: 桌面端 HeaderBar 条件渲染逻辑错误 ✅ 已确认

**代码分析** (HeaderBar.tsx 第245-306行):

```tsx
// 修复前的代码
{isLoggedIn && user ? (
  // 只显示用户下拉菜单，没有 Web3ConnectButton
  <div className='flex items-center gap-3'>
    <div className='relative' ref={userDropdownRef}>
      {/* 用户下拉菜单 */}
    </div>
  </div>
) : (
  // Web3ConnectButton 只在这里，即只有未登录时显示
  !['login', 'register'].includes(currentPage || '') && (
    <div className='flex items-center gap-3'>
      <Web3ConnectButton size="small" variant="secondary" />
      {/* 登录/注册按钮 */}
    </div>
  )
)}
```

**问题**: 三元表达式将 Web3ConnectButton 放在了 `else` 分支中，导致登录后不显示。

---

### 可能原因 2: 移动端菜单同样的条件逻辑错误 ✅ 已确认

**代码分析** (HeaderBar.tsx 第590-611行):

```tsx
// 修复前的代码
{!isLoggedIn && !['login', 'register'].includes(currentPage || '') && (
  <div className='space-y-2 mt-2'>
    <Web3ConnectButton size="small" variant="secondary" />
    {/* 登录/注册按钮 */}
  </div>
)}
```

**问题**: 条件 `!isLoggedIn` 明确排除了已登录用户。

---

### 可能原因 3: 生产环境需要重新部署 ✅ 需要操作

即使代码修复后，Vercel 上的生产版本仍然是旧代码，需要重新部署才能生效。

---

## 修复方案

### 修复 1: 重构桌面端条件逻辑

将 Web3ConnectButton 移到外层，确保登录和未登录状态都显示：

```tsx
// 修复后的代码
{!['login', 'register'].includes(currentPage || '') && (
  <div className='flex items-center gap-3'>
    {/* Web3 Connect Button - 始终显示 */}
    <Web3ConnectButton size="small" variant="secondary" />
    
    {isLoggedIn && user ? (
      /* 用户下拉菜单 */
    ) : (
      /* 登录/注册按钮 */
    )}
  </div>
)}
```

### 修复 2: 修复移动端菜单

将 Web3ConnectButton 从登录条件中分离出来：

```tsx
// 修复后的代码
{/* Web3 wallet button - 始终显示 */}
{!['login', 'register'].includes(currentPage || '') && (
  <div className='mt-4 pt-4' style={{ borderTop: '1px solid var(--panel-border)' }}>
    <Web3ConnectButton size="small" variant="secondary" />
  </div>
)}

{/* 用户信息 - 仅登录后显示 */}
{isLoggedIn && user && ( ... )}

{/* 登录/注册按钮 - 仅未登录时显示 */}
{!isLoggedIn && !['login', 'register'].includes(currentPage || '') && ( ... )}
```

### 修复 3: 重新部署到 Vercel

```bash
git add web/src/components/landing/HeaderBar.tsx
git commit -m "fix: show Web3 wallet button for both logged-in and logged-out users"
git push origin main
# Vercel 会自动重新部署
```

---

## 验证清单

- [x] 桌面端：未登录时显示 Web3 按钮
- [x] 桌面端：登录后显示 Web3 按钮
- [x] 移动端：未登录时显示 Web3 按钮
- [x] 移动端：登录后显示 Web3 按钮
- [ ] 中英文翻译正确
- [ ] Vercel 部署成功

---

## 文件变更

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `web/src/components/landing/HeaderBar.tsx` | 修改 | 重构条件渲染逻辑，确保 Web3 按钮始终显示 |

---

## 日期
2025-12-01

## 状态
✅ 代码修复完成，等待部署到 Vercel
