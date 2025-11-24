# Quickstart: Complete Red Theme Conversion

**Feature**: 002-complete-theme-fix  
**Branch**: `002-complete-theme-fix`  
**Date**: 2025-01-11

## 概述

完成 NOFX Web Dashboard 的红色主题转换，修复所有剩余的黄色UI元素和灰色文本可读性问题。

## 前置条件

- Node.js 18+ 已安装
- 项目依赖已安装（`npm install`）
- 开发服务器正在运行（`npm run dev`）
- 已切换到 `002-complete-theme-fix` 分支

## 快速开始

### 1. 验证当前问题

访问以下页面，确认存在黄色元素和灰色文本可读性问题：

```bash
# 启动开发服务器（如果还未启动）
npm run dev

# 访问以下页面：
# - http://localhost:3000/traders (主要问题页面)
# - http://localhost:3000/competition
# - http://localhost:3000/faq
```

### 2. 修改文件

按优先级顺序修改以下文件：

**高优先级**:
1. `src/components/AITradersPage.tsx` (25处黄色 + 83处灰色)
2. `src/components/TraderConfigModal.tsx` (22处黄色 + 41处灰色)

**中优先级**:
3. `src/components/faq/FAQContent.tsx` (17处黄色)
4. `src/components/AILearning.tsx` (9处黄色)
5. `src/components/CompetitionPage.tsx` (7处黄色 + 21处灰色)
6. `src/components/EquityChart.tsx` (5处黄色 + 22处灰色)

**低优先级**:
7. 其他文件（详见 plan.md）

### 3. 颜色替换规则

**黄色 → 红色**:
- `#F0B90B` → `var(--brand-red)` 或 `#E50012`
- `var(--brand-yellow)` → `var(--brand-red)`
- `var(--binance-yellow)` → `var(--brand-red)`
- `rgba(240, 185, 11, ...)` → `rgba(229, 0, 18, ...)`

**灰色 → 深色（用于文本）**:
- `#848E9C` → `var(--text-primary)` 或 `var(--text-secondary)`
- `#EAECEF` → `var(--text-primary)`
- `var(--brand-light-gray)` → `var(--text-primary)` (当用于文本时)

**背景色**:
- 确保使用 `var(--background)`, `var(--panel-bg)`, `var(--background-elevated)`
- 避免使用黑色或深色背景

### 4. 验证修改

每修改一个文件后：

```bash
# 1. 运行 linter
npm run lint:fix

# 2. 检查浏览器中的页面
# 3. 使用 Playwright MCP 截图验证（可选）
```

### 5. 提交代码

```bash
# 查看修改
git status
git diff

# 提交修改
git add .
git commit -m "fix: complete red theme conversion for [component name]"
```

## 验证清单

修改完成后，验证以下内容：

- [ ] 所有页面没有黄色UI元素（除了特定的警告/错误状态）
- [ ] 所有文本在浅色背景上清晰可读
- [ ] 文本对比度符合 WCAG 2.1 AA 标准
- [ ] 所有交互元素（按钮、链接）使用红色主题
- [ ] 图表和数据可视化使用红色系
- [ ] 没有破坏现有功能
- [ ] ESLint 检查通过

## 常见问题

### Q: 如何判断灰色是否需要替换？

A: 如果灰色用于主要文本或标题，应该替换为 `var(--text-primary)`。如果用于次要信息或辅助文本，可以保留或使用 `var(--text-secondary)`。

### Q: 某些黄色元素是否应该保留？

A: 只有在表示警告或特定状态时才保留黄色。所有主题色、品牌色、交互元素都应该使用红色。

### Q: 修改后页面显示异常怎么办？

A: 检查是否有语法错误，运行 `npm run lint:fix`，查看浏览器控制台是否有错误信息。

## 相关文档

- [Feature Specification](./spec.md)
- [Implementation Plan](./plan.md)
- [Research Notes](./research.md)
- [Constitution](./.specify/memory/constitution.md)

## 支持

如有问题，请参考：
- 项目 README: `d:\Projects\nofx\web\README.md`
- 之前的主题修改: `specs/001-red-theme-chinese/`
