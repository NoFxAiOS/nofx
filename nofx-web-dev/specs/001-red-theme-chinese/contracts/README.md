# Contracts: Red Theme and Chinese Language

**Feature**: 001-red-theme-chinese  
**Date**: 2025-11-11

## API Contracts

本功能不涉及 API 修改或新增端点。所有修改仅限于前端配置。

## Contract Summary

### No Backend Changes

- ❌ 无新增 API 端点
- ❌ 无修改现有 API 端点
- ❌ 无数据库 schema 变更
- ❌ 无后端配置修改

### Frontend-Only Changes

- ✅ CSS 变量修改 (`src/index.css`)
- ✅ React Context 修改 (`src/contexts/LanguageContext.tsx`)
- ✅ 组件自动应用新主题（无需修改组件代码）

## Configuration Contracts

虽然不涉及 API，但前端配置需要遵循以下契约：

### Theme Configuration Contract

**Location**: `src/index.css` `:root`

**Required Variables**:
```css
:root {
  /* 必需的主题变量 */
  --brand-red: #E50012;
  --background: #FAFAFA;
  --foreground: #1A1A1A;
  --panel-bg: #FFFFFF;
  --panel-border: #E0E0E0;
  --text-primary: #1A1A1A;
  --text-secondary: #616161;
  
  /* 保留的状态颜色 */
  --binance-green: #0ecb81;
  --binance-red: #E50012;
}
```

**Contract**:
- 所有组件必须通过 `var(--variable-name)` 引用颜色
- 不允许 inline 颜色硬编码（除非有特殊原因）
- 对比度必须符合 WCAG 2.1 AA 标准

### Language Contract

**Location**: `src/contexts/LanguageContext.tsx`

**Required Interface**:
```typescript
interface LanguageContextType {
  language: 'zh'  // 固定为中文
  setLanguage?: () => void  // 可选，no-op 或不暴露
}
```

**Contract**:
- `language` 必须始终为 `'zh'`
- 所有组件通过 `useLanguage()` 获取语言
- 翻译 keys 必须在 `src/i18n/translations.ts` 中存在

## Testing Contracts

### Visual Regression

**Viewport sizes to test**:
- Desktop: 1920x1080
- Tablet: 768x1024
- Mobile: 375x667

**Pages to capture**:
- Landing Page
- Dashboard
- Login/Register
- All major components

### Accessibility

**Required Checks**:
- 对比度 >= 4.5:1 (普通文本)
- 对比度 >= 3:1 (大文本 18pt+)
- 焦点指示器可见
- 无仅靠颜色传达的信息

## Backward Compatibility

### Breaking Changes

- ⚠️ 用户语言偏好将被强制覆盖为中文
- ⚠️ localStorage 中的 `language` 设置将被忽略
- ⚠️ 视觉风格完全改变（非向后兼容）

### Migration Strategy

无需数据迁移。用户刷新页面即可看到新主题。

## Future Extensions

如果未来需要恢复多语言支持：

1. 恢复 `LanguageContext` 中的 `setLanguage` 逻辑
2. 在 Header 中添加语言选择器
3. 恢复 localStorage 持久化

```typescript
// Future extension
const [language, setLanguage] = useState<Language>(() => {
  const saved = localStorage.getItem('language')
  return saved === 'en' || saved === 'zh' ? saved : 'zh' // default 'zh'
})
```

## Notes

- 本功能完全在客户端实现
- 无需后端配合或部署
- 可独立测试和发布
