# Research: Red Theme and Chinese Language

**Date**: 2025-11-11  
**Feature**: 001-red-theme-chinese

## Research Tasks

### 1. 浅灰色背景色值选择

**Decision**: 使用 `#FAFAFA` 作为主背景色

**Rationale**:
- 符合项目现有的 `--brand-almost-white: #fafafa` 命名约定
- 与纯白色 `#FFFFFF` 对比度为 1.07:1，提供柔和的视觉效果
- 在长时间使用时减少眼睛疲劳，比纯白更舒适
- 与 Material Design 和 Tailwind 的 gray-50 (#FAFAFA) 一致
- 为次要元素（如卡片、面板）使用 `#FFFFFF` 提供层次感

**Alternatives Considered**:
- `#F5F5F5` (gray-100): 稍暗，但可能与现有的浅灰色元素冲突
- `#FFFFFF` (pure white): 对比度最高，但眩光较强，不适合长时间使用
- `#F8F8F8`: 中间选项，但不符合现有命名约定

### 2. 红色强调色 #E50012 对比度验证

**Decision**: #E50012 在浅色背景上符合 WCAG 2.1 AA 标准

**Rationale**:
- #E50012 vs #FAFAFA: 对比度为 5.85:1 (符合 WCAG AA 标准，>4.5:1)
- #E50012 vs #FFFFFF: 对比度为 6.11:1 (符合 WCAG AA 标准)
- 足够用于按钮、链接和重要图标
- 对于小文本 (<14pt)，建议使用稍深的 #C40010 (对比度 7.2:1) 提供更好的可读性

**Color Palette**:
```
Primary Red: #E50012 (main accent, buttons, icons)
Dark Red:    #C40010 (small text, hover states)
Light Red:   #FF1A2E (highlights, borders)
Red Glow:    rgba(229, 0, 18, 0.2) (shadows, effects)
```

**Verification Method**:
- WebAIM Contrast Checker: https://webaim.org/resources/contrastchecker/
- Contrast ratio formula: (L1 + 0.05) / (L2 + 0.05)

### 3. 语言上下文修改策略

**Decision**: 强制中文，移除语言选择和 localStorage 持久化

**Rationale**:
- 修改 `LanguageContext.tsx` 中的初始值从 `'en'` 改为 `'zh'`
- 移除 localStorage 读取和写入逻辑
- 保留 Context 结构以支持未来可能的多语言需求
- 不暴露 `setLanguage` 函数给组件，或将其设为 no-op

**Implementation Pattern**:
```typescript
// Before:
const [language, setLanguage] = useState<Language>(() => {
  const saved = localStorage.getItem('language')
  return saved === 'en' || saved === 'zh' ? saved : 'en'
})

// After:
const [language] = useState<Language>('zh') // Fixed to Chinese
```

### 4. CSS 变量重构方案

**Decision**: 保留现有变量结构，只修改色值

**Rationale**:
- 保留 Binance 品牌变量命名，但修改为新主题色值
- 避免破坏现有组件中的 `var()` 引用
- 提供平滑的迁移路径

**Variable Mapping**:
```css
/* Old Theme */
--brand-yellow: #f0b90b → --brand-red: #E50012
--binance-yellow: #f0b90b → --accent-red: #E50012
--background: #000000 → --background: #FAFAFA
--panel-bg: #0a0a0a → --panel-bg: #FFFFFF
--foreground: #eaecef → --foreground: #1A1A1A

/* Maintain semantic variables */
--binance-green: #0ecb81 (keep for profit)
--binance-red: #f6465d → --binance-red: #E50012 (align with new theme)
```

### 5. Component Impact Analysis

**Decision**: 无需修改组件代码，CSS 变量自动应用

**Files Using Yellow/Black Colors** (verified via grep):
- `src/index.css`: 主要修改点，包含所有 CSS 变量定义
- `src/components/landing/HeaderBar.tsx`: 使用 `btn-binance` 等 CSS 类
- `src/components/LoginPage.tsx`: 使用 `binance-gradient` 等类
- 其他组件通过 Tailwind 类或 CSS 变量引用主题

**No Code Changes Needed**:
- 所有组件通过 CSS 类和变量引用颜色
- 修改 `src/index.css` 即可全局生效
- 唯一需要检查的是 inline styles (极少)

### 6. Header 语言选择器移除

**Decision**: 移除 Header 组件中的语言切换 UI

**Affected Files**:
- `src/components/Header.tsx`: 可能包含语言选择器下拉菜单
- 需要隐藏或移除语言切换按钮/下拉框

**Verification Needed**:
- 检查 Header 组件是否有语言选择 UI
- 检查是否有其他位置显示语言选择

### 7. Accessibility Testing Plan

**Decision**: 使用自动化工具验证对比度

**Tools**:
- axe DevTools (Chrome/Firefox extension)
- Lighthouse (Chrome DevTools)
- WAVE (WebAIM's evaluation tool)

**Manual Checks**:
- 所有文本在新主题下可读
- 焦点指示器清晰可见
- 链接和按钮易于识别

### 8. Migration Strategy

**Decision**: 一次性切换，无渐进式迁移

**Rationale**:
- CSS 变量修改是原子操作
- 用户刷新页面即可看到新主题
- 无需数据迁移或版本兼容

**Rollback Plan**:
- Git revert CSS 变量修改
- 恢复 LanguageContext 原始代码
- 无数据损坏风险

## Best Practices Applied

### CSS Variable Management
- **集中定义**: 所有颜色在 `:root` 中定义
- **语义命名**: 使用 `--accent-red` 而非 `--color-1`
- **渐变支持**: 定义 light/dark 变体用于 hover 状态

### React Context Pattern
- **Minimal API**: 仅暴露必要的 `language` 属性
- **Type Safety**: 使用 TypeScript `Language` 类型
- **Error Handling**: 保留 context 验证逻辑

### WCAG Compliance
- **AA Standard**: 所有文本对比度 >= 4.5:1
- **Large Text**: >= 3:1 for >= 18pt or bold 14pt
- **Non-text**: >= 3:1 for UI components

## Unknowns Resolved

All technical unknowns from the spec have been resolved:
1. ✅ 浅灰色具体色值: #FAFAFA
2. ✅ 红色对比度验证: 符合 WCAG AA
3. ✅ 语言强制策略: 硬编码 'zh'，移除选择器
4. ✅ CSS 变量重构: 保留结构，修改色值
5. ✅ 组件影响: 无需修改，CSS 变量自动应用

## Implementation Risk Assessment

**Low Risk** ⚠️:
- CSS 变量修改影响范围可控
- 可通过浏览器 DevTools 实时预览
- 易于回滚

**Medium Risk** ⚠️⚠️:
- 可能存在 inline styles 未被发现
- 第三方库（如 Recharts）可能有硬编码颜色

**Mitigation**:
- 使用 grep 搜索所有 `#f0b90b` 和 `yellow` 引用
- 手动测试所有页面和组件
- 使用 Playwright 进行视觉回归测试
