# Color Mapping Reference

**Feature**: 002-complete-theme-fix  
**Date**: 2025-01-11

## 颜色替换规则

### 黄色 → 红色

| 旧颜色值 | 新颜色值 | 用途 |
|---------|---------|------|
| `#F0B90B` | `var(--brand-red)` 或 `#E50012` | 主题色、按钮、图标 |
| `var(--brand-yellow)` | `var(--brand-red)` | 品牌黄色变量 |
| `var(--binance-yellow)` | `var(--brand-red)` | Binance 黄色变量 |
| `rgba(240, 185, 11, 0.1)` | `rgba(229, 0, 18, 0.1)` | 黄色半透明背景 |
| `rgba(240, 185, 11, 0.2)` | `rgba(229, 0, 18, 0.2)` | 黄色半透明边框 |
| `rgba(240, 185, 11, 0.3)` | `rgba(229, 0, 18, 0.3)` | 黄色半透明强调 |
| `rgba(240, 185, 11, 0.4)` | `rgba(229, 0, 18, 0.4)` | 黄色半透明阴影 |

### 灰色 → 深色（用于文本）

| 旧颜色值 | 新颜色值 | 用途 | 何时替换 |
|---------|---------|------|---------|
| `#848E9C` | `var(--text-primary)` | 主要文本 | 当用于标题、主要内容时 |
| `#848E9C` | `var(--text-secondary)` | 次要文本 | 当用于辅助信息时 |
| `#EAECEF` | `var(--text-primary)` | 浅灰文本 | 当用于标题、主要内容时 |
| `var(--brand-light-gray)` | `var(--text-primary)` | 品牌浅灰 | 仅当用于文本时（背景色保留） |

### 背景色（确保使用浅色）

| 应该使用的颜色 | 用途 |
|--------------|------|
| `var(--background)` (#FAFAFA) | 页面主背景 |
| `var(--panel-bg)` (#FFFFFF) | 面板/卡片背景 |
| `var(--background-elevated)` (#FFFFFF) | 提升的背景（模态框、下拉菜单） |
| `var(--panel-bg-hover)` (#F5F5F5) | 悬停状态背景 |

### 不应该使用的颜色

❌ **避免使用**：
- `var(--brand-black)` (#000000) - 除非用于特定的深色元素（如代码块）
- `var(--brand-dark-gray)` (#1A1A1A) - 仅用于文本，不用于背景
- 任何深色背景 - 应该使用浅色主题

## CSS 变量参考

### 红色主题变量

```css
--brand-red: #E50012;              /* 主红色 */
--brand-red-dark: #C40010;         /* 深红色 */
--brand-red-light: #FF1A2E;        /* 浅红色 */
--accent-red: #E50012;             /* 强调红色 */
--accent-red-glow: rgba(229, 0, 18, 0.2);  /* 红色光晕 */
```

### 文本颜色变量

```css
--text-primary: #1A1A1A;           /* 主要文本（深色） */
--text-secondary: #616161;         /* 次要文本（中灰） */
--text-tertiary: #9E9E9E;          /* 三级文本（浅灰） */
--text-disabled: #BDBDBD;          /* 禁用文本 */
```

### 背景颜色变量

```css
--background: #FAFAFA;             /* 主背景（浅灰） */
--background-elevated: #FFFFFF;    /* 提升背景（白色） */
--panel-bg: #FFFFFF;               /* 面板背景 */
--panel-bg-hover: #F5F5F5;         /* 悬停背景 */
```

## 替换示例

### 示例 1: 黄色按钮 → 红色按钮

```tsx
// 旧代码
<button style={{ background: '#F0B90B', color: '#000' }}>
  点击
</button>

// 新代码
<button style={{ background: 'var(--brand-red)', color: '#FFFFFF' }}>
  点击
</button>
```

### 示例 2: 灰色标题 → 深色标题

```tsx
// 旧代码
<h2 style={{ color: '#848E9C' }}>
  AI交易员
</h2>

// 新代码
<h2 style={{ color: 'var(--text-primary)' }}>
  AI交易员
</h2>
```

### 示例 3: 黄色徽章 → 红色徽章

```tsx
// 旧代码
<span style={{
  background: 'rgba(240, 185, 11, 0.1)',
  border: '1px solid rgba(240, 185, 11, 0.2)',
  color: 'var(--brand-yellow)'
}}>
  0 活跃
</span>

// 新代码
<span style={{
  background: 'rgba(229, 0, 18, 0.1)',
  border: '1px solid rgba(229, 0, 18, 0.2)',
  color: 'var(--brand-red)'
}}>
  0 活跃
</span>
```

## WCAG 2.1 AA 对比度标准

确保文本与背景的对比度至少为 4.5:1：

- ✅ `#1A1A1A` (text-primary) 在 `#FAFAFA` (background) 上 = 12.6:1
- ✅ `#616161` (text-secondary) 在 `#FAFAFA` (background) 上 = 5.9:1
- ✅ `#E50012` (brand-red) 在 `#FFFFFF` (white) 上 = 5.4:1

## 验证清单

修改每个文件后，检查：

- [ ] 所有黄色颜色值已替换为红色
- [ ] 主要文本使用 `var(--text-primary)`
- [ ] 次要文本使用 `var(--text-secondary)`
- [ ] 背景使用浅色变量
- [ ] 没有硬编码的黑色背景（除非是特定元素如代码块）
- [ ] 运行 `npm run lint:fix` 无错误
- [ ] 浏览器中视觉验证通过
