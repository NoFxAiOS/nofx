## Why

用户登录 https://www.agentrade.xyz 后，在右上角导航栏（语言切换按钮左侧）发现没有显示可用积分信息。这是一个影响核心用户体验的Bug，用户无法看到他们的账户余额。

## What Changes

修复积分显示系统中导致右上角积分组件无法显示的三个关键问题：

1. **Hook加载状态管理不完整** - useUserCredits Hook在认证失败时未正确设置加载状态，导致持续显示骨架屏
2. **API响应数据格式验证缺失** - 未验证API返回的数据结构，可能因数据格式不符而导致显示失败
3. **错误处理缺乏重试机制** - 当API调用失败时，组件显示占位符 "-" 而不能自动重试或显示错误状态

## Impact

- **Affected specs**: `credits-display` (用户界面), `user-credits-api` (数据获取)
- **Affected code**:
  - `/web/src/hooks/useUserCredits.ts` - Hook核心逻辑
  - `/web/src/components/CreditsDisplay/CreditsDisplay.tsx` - UI组件
  - `/web/src/components/Header.tsx` - Header集成
- **User Impact**: P0级别 - 影响用户核心功能（查看账户余额）
- **Breaking Changes**: 无 - 仅修复现有功能
