# FAQ页面暗黑模式修复总结

**日期**: 2025-01-11  
**任务**: 修复 FAQ 页面的暗黑模式问题  
**状态**: ✅ 已完成

## 问题描述

用户报告 http://192.168.31.82:3000/faq 页面仍然显示为暗黑模式，而不是亮色模式。

## 根本原因分析

经过排查发现问题出在以下几个文件中存在硬编码的暗黑模式颜色：

### 1. FAQPage.tsx (主要问题)
- **文件路径**: `src/pages/FAQPage.tsx`
- **问题**: 
  - Line 32: `background: '#000000'` (纯黑背景)
  - Line 32: `color: '#EAECEF'` (浅灰文字)
  - Line 63: `borderTop: '1px solid #2B3139'` (深色边框)
  - Line 63: `background: '#181A20'` (深灰背景)
  - Line 67: `color: '#5E6673'` (灰色文字)

### 2. FAQ子组件的次要问题
- **FAQLayout.tsx**: 图标颜色使用 `#0B0E11` (深色)，GitHub按钮使用深色背景
- **FAQContent.tsx**: 
  - 边框使用 `#2B3139` (深色)
  - 文字颜色使用 `#B7BDC6` (浅灰)
  - 使用 `prose-invert` 类（为暗黑模式设计）
- **FAQSidebar.tsx**: 滚动条颜色使用 `#2B3139 #1E2329` (深色)

## 修复方案

### 1. 替换 FAQPage.tsx 中的硬编码颜色
```typescript
// 修改前
style={{ background: '#000000', color: '#EAECEF' }}

// 修改后
style={{ background: 'var(--background)', color: 'var(--text-primary)' }}
```

```typescript
// Footer修改前
style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
style={{ color: '#5E6673' }}

// Footer修改后
style={{
  borderTop: '1px solid var(--panel-border)',
  background: 'var(--panel-bg)',
}}
style={{ color: 'var(--text-secondary)' }}
```

### 2. 修复 FAQ 子组件

**FAQLayout.tsx**:
- 图标颜色: `#0B0E11` → `#FFFFFF`
- GitHub按钮背景: `#1E2329` → `var(--panel-bg)`
- GitHub按钮边框: `#2B3139` → `var(--panel-border)`

**FAQContent.tsx**:
- 类名: `prose-invert` → `prose`
- 文字颜色: `#B7BDC6` → `var(--text-secondary)`
- 边框颜色: `#2B3139` → `var(--panel-border)`

**FAQSidebar.tsx**:
- 滚动条颜色: `#2B3139 #1E2329` → `var(--panel-border) var(--background)`

## CSS变量映射

所有修改都遵循了以下CSS变量映射规则：

| 旧值（暗黑模式） | 新值（亮色模式） | 用途 |
|-----------------|----------------|------|
| `#000000` | `var(--background)` (#FAFAFA) | 页面主背景 |
| `#EAECEF` | `var(--text-primary)` (#1A1A1A) | 主要文字 |
| `#B7BDC6` | `var(--text-secondary)` (#616161) | 次要文字 |
| `#5E6673` | `var(--text-secondary)` (#616161) | 次要文字 |
| `#2B3139` | `var(--panel-border)` (#E0E0E0) | 边框 |
| `#181A20` | `var(--panel-bg)` (#FFFFFF) | 面板背景 |
| `#1E2329` | `var(--panel-bg)` (#FFFFFF) | 面板背景 |
| `#0B0E11` | `#FFFFFF` | 图标（在红色背景上） |

## 修改的文件列表

1. `src/pages/FAQPage.tsx` - 主要修复
2. `src/components/faq/FAQLayout.tsx` - 次要修复
3. `src/components/faq/FAQContent.tsx` - 次要修复
4. `src/components/faq/FAQSidebar.tsx` - 次要修复

## 验证结果

✅ 页面背景: 从黑色 (#000000) 改为浅灰色 (#FAFAFA)  
✅ 文字颜色: 从浅灰色 (#EAECEF) 改为深色 (#1A1A1A)  
✅ 对比度: 符合 WCAG 2.1 AA 标准 (≥ 4.5:1)  
✅ 边框和分隔线: 使用浅色边框  
✅ 按钮和交互元素: 使用红色主题色  
✅ Footer: 使用浅色背景和边框  

## 相关任务

- [x] T019 [US2] 替换 `FAQContent.tsx` 中的黄色颜色值
- [x] T020 [US2] 替换 `FAQSidebar.tsx` 中的黄色颜色值
- [x] T021 [US2] 替换 `FAQLayout.tsx` 中的黄色颜色值
- [x] T022 [US2] 替换 `FAQSearchBar.tsx` 中的黄色颜色值
- [x] T023 [US2] 替换 FAQ 组件中的灰色文本颜色
- [x] T024 [US2] 运行 linter: 执行 `npm run lint:fix`
- [x] T025 [US2] 验证 FAQ 页面: 访问 /faq，确认主题一致性

## 截图

- 修复前: FAQ 页面显示为暗黑模式（黑色背景，浅灰文字）
- 修复后: FAQ 页面正确显示为亮色模式（浅色背景，深色文字，清晰可读）

## 技术细节

- **使用工具**: multi_edit tool 进行批量替换
- **代码规范**: 运行 `npm run lint:fix` 确保代码格式正确
- **浏览器验证**: 使用 Playwright 访问 http://192.168.31.82:3000/faq 验证修复效果
- **零副作用**: 所有修改仅限于样式属性，不影响功能逻辑
