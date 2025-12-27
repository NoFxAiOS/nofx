## 问题分析和修复计划

### 三个根本原因分析

#### 原因1: useUserCredits Hook加载状态管理不完整
**位置**: `/web/src/hooks/useUserCredits.ts:44-113`

**问题**:
- 第61行：设置 `setLoading(true)`
- 第72-77行：在401认证失败时只清空数据，**未设置** `setLoading(false)`
- 结果：组件持续显示加载骨架屏，无法显示实际数据

**现象**: 用户登录后右上角显示加载中状态（骨架屏），不显示实际积分数值

**代码证据**:
```typescript
// 第72-77行缺少setLoading(false)
if (!response.ok) {
  if (response.status === 401) {
    // 认证失败，不需要设置错误，直接清空数据
    setCredits(null);
    return;  // ❌ 这里没有设置 setLoading(false)
  }
  throw new Error(`Failed to fetch credits: ${response.statusText}`);
}
```

#### 原因2: API响应数据格式验证缺失
**位置**: `/web/src/hooks/useUserCredits.ts:81-82`

**问题**:
```typescript
const data = await response.json();
setCredits(data as UserCredits);  // ❌ 没有验证data结构
```
- 直接将API返回数据转换为UserCredits接口，未检查必要字段是否存在
- 如果API返回 `{ data: { available: 750 } }`（嵌套）或其他非预期格式，会导致显示失败
- credits.available 可能是 undefined

**现象**: 即使API返回200，但数据格式异常时组件显示"-"

#### 原因3: 错误处理缺乏恢复机制
**位置**: `/web/src/components/CreditsDisplay/CreditsDisplay.tsx:39-44`

**问题**:
```typescript
// 错误或无数据时直接显示占位符，无重试
if (error || !credits) {
  return <div className="credits-error">-</div>;
}
```
- useUserCredits Hook在第86-88行出错时设置 `setError(error)` 和 `setCredits(null)`
- CreditsDisplay没有提供重试按钮或重新加载机制
- 用户无法手动刷新积分数据

**现象**: 网络临时中断后，用户看到"-"且无法恢复，需要刷新页面

---

## 1. 修复任务

### 1.1 修复useUserCredits Hook加载状态
- [ ] **任务**: 完成useUserCredits Hook中的加载状态管理
  - [ ] 在401认证失败处添加 `setLoading(false)`（第76行）
  - [ ] 在catch块中确保 `setLoading(false)` 已设置（第88行已有）
  - [ ] 验证所有执行路径都正确设置加载状态

**修复代码**:
```typescript
// 第72-77行修改
if (!response.ok) {
  if (response.status === 401) {
    setCredits(null);
    setLoading(false);  // ✅ 新增
    return;
  }
  throw new Error(`Failed to fetch credits: ${response.statusText}`);
}
```

### 1.2 添加API响应数据格式验证
- [ ] **任务**: 验证API返回的数据结构
  - [ ] 检查data是否为对象
  - [ ] 检查available、total、used字段是否存在且为数字
  - [ ] 提供有意义的错误提示

**修复代码**:
```typescript
// 第81-82行修改
const data = await response.json();

// ✅ 新增数据验证
if (!data || typeof data !== 'object') {
  throw new Error('API响应数据格式错误: 期望对象');
}

const credits = data as UserCredits;
if (typeof credits.available !== 'number' ||
    typeof credits.total !== 'number' ||
    typeof credits.used !== 'number') {
  throw new Error('API响应数据格式错误: 缺少必要字段');
}

setCredits(credits);
```

### 1.3 改进错误处理和显示
- [ ] **任务**: 改进CreditsDisplay的错误状态显示
  - [ ] 在错误状态下显示更有用的提示（如"加载失败"而非"-"）
  - [ ] Hook中保留refetch方法供手动刷新
  - [ ] 考虑添加自动重试逻辑

**修复代码** (CreditsDisplay.tsx第39-44行):
```typescript
// 错误状态：显示可重试的占位符
if (error) {
  return (
    <div
      className="credits-error"
      data-testid="credits-error"
      title="积分加载失败，请刷新页面"
      role="status"
    >
      ⚠️
    </div>
  );
}

// 无数据但无错误（仍在加载）：显示骨架屏
if (!credits) {
  return <div className="credits-loading" data-testid="credits-loading" />;
}
```

---

## 2. 测试验证

### 2.1 单元测试
- [ ] **任务**: 验证useUserCredits Hook的所有执行路径
  - [ ] 成功获取积分数据
  - [ ] 处理401认证失败（验证setLoading被调用）
  - [ ] 处理数据格式错误
  - [ ] 处理网络错误

### 2.2 集成测试
- [ ] **任务**: 验证Header中的CreditsDisplay集成
  - [ ] 登录后显示正确的积分数值
  - [ ] 30秒自动刷新正常工作
  - [ ] 手动refetch正常工作

### 2.3 E2E测试 (Playwright)
- [ ] **任务**: 验证整个用户流程
  - [ ] 访问 https://www.agentrade.xyz
  - [ ] 用户登录
  - [ ] 右上角正确显示可用积分
  - [ ] 语言切换不影响积分显示

---

## 3. 部署前检查

- [ ] 代码修改通过linter检查
- [ ] 所有单元测试通过
- [ ] E2E测试通过
- [ ] 本地开发环境验证正确
- [ ] 准备好部署更新到生产环境
