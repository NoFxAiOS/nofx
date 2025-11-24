# Data Model: Red Theme and Chinese Language

**Date**: 2025-11-11  
**Feature**: 001-red-theme-chinese

## Overview

本功能不涉及数据库模型或 API 数据结构，主要涉及前端配置和状态管理。

## Configuration Entities

### 1. Theme Configuration (CSS Variables)

**Location**: `src/index.css` `:root` selector

**Schema**:
```typescript
interface ThemeVariables {
  // Brand Colors
  '--brand-red': string          // #E50012 (primary accent)
  '--brand-red-dark': string     // #C40010 (hover, dark variant)
  '--brand-red-light': string    // #FF1A2E (highlights)
  '--brand-red-glow': string     // rgba(229, 0, 18, 0.2)
  
  // Background Colors
  '--background': string          // #FAFAFA (main background)
  '--header-bg': string           // #FAFAFA
  '--background-elevated': string // #FFFFFF (cards, panels)
  '--foreground': string          // #1A1A1A (main text)
  
  // Panel Colors
  '--panel-bg': string            // #FFFFFF
  '--panel-bg-hover': string      // #F5F5F5
  '--panel-border': string        // #E0E0E0
  '--panel-border-hover': string  // #BDBDBD
  
  // Status Colors (preserved)
  '--binance-green': string       // #0ecb81 (profit)
  '--binance-red': string         // #E50012 (loss, aligned with theme)
  
  // Text Colors
  '--text-primary': string        // #1A1A1A
  '--text-secondary': string      // #616161
  '--text-tertiary': string       // #9E9E9E
  '--text-disabled': string       // #BDBDBD
  
  // Shadows (light theme adjusted)
  '--shadow-sm': string
  '--shadow-md': string
  '--shadow-lg': string
  '--shadow-xl': string
}
```

**Validation Rules**:
- 所有颜色必须符合 WCAG 2.1 AA 标准
- Primary text (#1A1A1A) vs Background (#FAFAFA): ≥ 4.5:1
- Accent red (#E50012) vs Background (#FAFAFA): ≥ 4.5:1
- Border colors must be visually distinct from backgrounds

**State Transitions**: N/A (static configuration)

**Relationships**: CSS 变量被所有组件引用

---

### 2. Language Settings (React Context)

**Location**: `src/contexts/LanguageContext.tsx`

**Schema**:
```typescript
// src/i18n/translations.ts
export type Language = 'en' | 'zh'

// src/contexts/LanguageContext.tsx
interface LanguageContextType {
  language: Language       // Fixed to 'zh'
  setLanguage?: never      // Removed or no-op
}
```

**Validation Rules**:
- `language` must always be `'zh'`
- Context must be available to all components via `useLanguage()`

**State Transitions**:
```
Initial: language = 'zh' (hardcoded)
No transitions allowed (fixed to Chinese)
```

**Relationships**:
- Used by all components via `useLanguage()` hook
- References `translations` object from `src/i18n/translations.ts`

---

### 3. Translation Dictionary (Existing)

**Location**: `src/i18n/translations.ts`

**Schema**:
```typescript
export const translations = {
  en: { [key: string]: string },
  zh: { [key: string]: string }
}
```

**Validation Rules**:
- All keys in `en` must exist in `zh`
- Missing translations in dynamic content should fallback to original language

**State Transitions**: N/A (static dictionary)

**Relationships**: 
- Consumed by `useLanguage()` hook
- Keys used throughout all components

---

## Data Flow Diagram

```
┌─────────────────────────────────────────┐
│         User Loads Application          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│     LanguageProvider initializes        │
│     language = 'zh' (hardcoded)         │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│   CSS :root variables loaded            │
│   --brand-red, --background, etc.       │
└─────────────────┬───────────────────────┘
                  │
                  ├──────────────────────────┐
                  │                          │
                  ▼                          ▼
     ┌─────────────────────┐    ┌─────────────────────┐
     │   All Components    │    │   All Components    │
     │   use useLanguage() │    │   use CSS variables │
     │   → get 'zh'        │    │   via var(--name)   │
     └─────────────────────┘    └─────────────────────┘
                  │                          │
                  └──────────┬───────────────┘
                             ▼
              ┌─────────────────────────────┐
              │  UI Rendered in Chinese     │
              │  with Red on Light Theme    │
              └─────────────────────────────┘
```

---

## Migration from Old Model

### Before (Black + Yellow + English)
```css
:root {
  --brand-yellow: #f0b90b;
  --background: #000000;
  --foreground: #eaecef;
}
```

```typescript
// LanguageContext
const [language, setLanguage] = useState<Language>(() => {
  const saved = localStorage.getItem('language')
  return saved === 'en' || saved === 'zh' ? saved : 'en' // default 'en'
})
```

### After (Light Gray + Red + Chinese)
```css
:root {
  --brand-red: #E50012;
  --background: #FAFAFA;
  --foreground: #1A1A1A;
}
```

```typescript
// LanguageContext
const [language] = useState<Language>('zh') // fixed to 'zh'
```

---

## Constraints

### Performance
- CSS 变量修改后浏览器立即重绘 (<16ms)
- 语言加载时间 <100ms (已在内存中)

### Accessibility
- 所有颜色对比度 >= 4.5:1 (WCAG AA)
- 焦点指示器清晰可见
- 无颜色依赖的信息传达

### Browser Support
- CSS Custom Properties: Chrome 49+, Firefox 31+, Safari 9.1+, Edge 15+
- React Context: All modern browsers

---

## No Database/API Impact

本功能完全在前端实现，不涉及：
- ❌ 数据库 schema 修改
- ❌ API 端点修改
- ❌ 后端配置修改
- ❌ 数据迁移脚本

所有修改仅限于前端配置文件和 React Context。
