# OpenSpec Bug 提案: 创建交易员时选择AI模型导致名称重置

## 📋 Bug概述

**Bug类型**: 🟠 状态管理错误
**优先级**: P1 (高)
**影响范围**: 交易员创建流程
**发现时间**: 2025-12-27
**状态**: 🔍 已定位根本原因，准备修复

---

## 🚨 问题描述

### 现象
用户在创建新交易员时，输入交易员名称后，如果选择AI模型下拉菜单中的其他模型，已输入的交易员名称会立即被清除为空，需要重新输入。

### 复现步骤
1. 点击 "创建交易员" 按钮打开创建模态框
2. 在 "交易员名称" 字段中输入名称（如："Test Trader"）
3. 点击 "AI模型" 下拉菜单
4. 选择一个不同的AI模型
5. **预期**: 交易员名称应该保持不变
6. **实际**: 交易员名称被清空为空字符串

### 用户影响
- 创建流程被打断
- 用户体验差（需要重新输入）
- 可能导致用户放弃创建交易员

---

## 🔍 深度根本原因分析

### 原因1️⃣：useEffect依赖数组包含不稳定的props引用

**文件位置**: `web/src/components/TraderConfigModal.tsx:73-106`

**问题代码**:
```typescript
useEffect(() => {
  if (traderData) {
    setFormData(traderData);
    // ...
  } else if (!isEditMode) {
    setFormData({
      trader_name: '',  // ❌ 无条件重置为空
      ai_model: availableModels[0]?.id || '',
      exchange_id: availableExchanges[0]?.id || '',
      // ...
    });
  }
}, [traderData, isEditMode, availableModels, availableExchanges]); // ❌ 关键问题在此
```

**根本原因链条**:
1. 用户在ModelConfigModal中配置新AI模型
2. `handleSaveModelConfig()` (AITradersPage.tsx:395) 调用 `api.updateModelConfigs()`
3. 重新获取模型列表: `setAllModels(refreshedModels)` (第399行)
4. 重新计算 `enabledModels` (第115行): `allModels?.filter(m => m.enabled && m.apiKey) || []`
5. **新的数组引用**被传给TraderConfigModal作为 `availableModels`
6. useEffect因为依赖改变（availableModels是新引用）而**重新执行**
7. 条件 `else if (!isEditMode)` 触发，**无条件重置trader_name为空**

**影响**: 每当用户与AI模型交互时，useEffect都会因为availableModels变化而触发，导致表单重置。

---

### 原因2️⃣：初始化逻辑缺乏用户输入保护

**文件位置**: `web/src/components/TraderConfigModal.tsx:81-98`

**问题代码**:
```typescript
} else if (!isEditMode) {
  setFormData({
    trader_name: '',  // ❌ 无条件重置，不检查用户输入
    ai_model: availableModels[0]?.id || '',
    exchange_id: availableExchanges[0]?.id || '',
    btc_eth_leverage: 5,
    altcoin_leverage: 3,
    trading_symbols: '',
    // ... 其他字段
  });
}
```

**问题分析**:
- 代码没有检查 `formData` 是否已包含用户输入
- 没有区分"初次打开模态框"和"模态框已打开中"的场景
- 依赖改变时无条件调用setFormData，破坏用户数据

**影响**: 任何时候依赖改变，用户的输入都会被无情地覆盖。

---

### 原因3️⃣：多个useEffect同时修改formData导致状态竞争

**文件位置**: `web/src/components/TraderConfigModal.tsx:145-148`

**问题代码**:
```typescript
// useEffect #1 (第73行)
useEffect(() => {
  // ... 重置formData
  setFormData(prev => ({ ...prev, ... }));
}, [traderData, isEditMode, availableModels, availableExchanges]);

// useEffect #2 (第145行)
useEffect(() => {
  const symbolsString = selectedCoins.join(',');
  setFormData(prev => ({ ...prev, trading_symbols: symbolsString }));
}, [selectedCoins]);
```

**问题分析**:
- 两个独立的useEffect在修改同一个状态
- 第一个useEffect会无条件重置整个formData
- 第二个useEffect试图更新trading_symbols
- React没有保证执行顺序，导致不可预测的最终状态

**影响**: 状态变成"脆弱的"，难以调试和维护。

---

## 💡 修复方案

### Primary Fix: 移除不稳定的props依赖

**从useEffect依赖数组中移除 availableModels 和 availableExchanges**:
```typescript
// 修复前
useEffect(() => { ... }, [traderData, isEditMode, availableModels, availableExchanges]);

// 修复后
useEffect(() => { ... }, [traderData, isEditMode, isOpen]);
```

**理由**:
- `availableModels` 和 `availableExchanges` 是动态数组，每次父组件更新时都会改变引用
- 这些props只用于初始化，不需要作为依赖监听
- 使用 `isOpen` 作为显式信号来表示"模态框打开"事件

### Secondary Fix: 添加初始化保护

**添加条件逻辑**:
```typescript
const [hasInitialized, setHasInitialized] = useState(false);

useEffect(() => {
  if (traderData) {
    setFormData(traderData);
    setHasInitialized(true);
  } else if (!isEditMode && !hasInitialized) {
    // ✅ 只在首次打开模态框时初始化
    setFormData({
      trader_name: '',
      ai_model: availableModels[0]?.id || '',
      // ...
    });
    setHasInitialized(true);
  }
}, [traderData, isEditMode, isOpen]);

// 当模态框关闭时重置标志
useEffect(() => {
  if (!isOpen) {
    setHasInitialized(false);
  }
}, [isOpen]);
```

### Tertiary Fix: 整合状态更新

**统一在单一初始化点处理所有字段**:
- 将 `selectedCoins` 的初始化移到主useEffect中
- 避免多个useEffect同时修改formData
- 保持状态管理的可预测性

---

## 📝 修复前后对比

| 场景 | 修复前 | 修复后 |
|------|--------|--------|
| 用户输入名称 | ✅ 可输入 | ✅ 可输入 |
| 选择AI模型 | ❌ 名称被清空 | ✅ 名称保留 |
| 选择交易所 | ❌ 数据被重置 | ✅ 数据保留 |
| 新开模态框 | ✅ 表单初始化 | ✅ 表单初始化 |
| 编辑切换 | ❌ 可能丢失数据 | ✅ 正确切换 |

---

## 🧪 测试计划

### 功能测试
- [ ] 打开创建模态框后输入交易员名称
- [ ] 选择不同的AI模型，验证名称保留
- [ ] 选择不同的交易所，验证所有数据保留
- [ ] 刷新页面后再打开模态框，验证正确初始化
- [ ] 在编辑模态框中修改数据，切换到创建模态框，验证数据不混乱

### 边界情况
- [ ] 快速选择多个模型，验证稳定性
- [ ] 模态框打开时父组件更新可用模型列表
- [ ] 模态框打开时其他用户修改配置

### 性能
- [ ] 验证不会有多余的重新渲染
- [ ] 验证useEffect调用次数符合预期

---

## 📋 实现清单

- [ ] 修改 `TraderConfigModal.tsx` 的useEffect依赖
- [ ] 添加初始化状态跟踪
- [ ] 添加isOpen显式依赖
- [ ] 测试所有场景
- [ ] 验证表单提交仍然正确工作
- [ ] 完成代码审查

---

## 📊 影响评估

- **Breaking Changes**: ❌ 无
- **API Changes**: ❌ 无
- **数据模型变化**: ❌ 无
- **性能影响**: ✅ 正面（减少不必要的re-render）
- **向后兼容**: ✅ 完全兼容

---

## 📌 相关代码位置

- 主要文件: `web/src/components/TraderConfigModal.tsx`
- 父组件: `web/src/components/AITradersPage.tsx`
- 问题出发点: `AITradersPage.tsx:398-399` (模型配置更新)

---

## 🔗 参考

- OpenSpec: Bug Fix指南
- 项目规范: `web/openspec/project.md`
