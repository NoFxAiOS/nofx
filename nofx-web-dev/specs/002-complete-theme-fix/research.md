# Research: Complete Red Theme Conversion

**Feature**: 002-complete-theme-fix  
**Date**: 2025-01-11  
**Status**: N/A - No Research Required

## Summary

此功能不需要技术研究，因为：

1. **明确的技术方案**：使用现有的 CSS 变量系统，仅需要查找和替换颜色值
2. **无新技术引入**：不涉及新的库、框架或工具
3. **已有先例**：之前已经完成了部分主题转换（登录页、注册页、首页），技术路径已验证

## Technical Decisions

### Decision 1: 颜色替换策略

**Decision**: 使用批量查找替换，按优先级处理文件

**Rationale**: 
- 效率最高：可以快速处理大量相似的修改
- 风险可控：每个文件独立修改，易于回滚
- 可验证：每次修改后可以立即通过 Playwright 验证

**Alternatives Considered**:
- 手动逐个修改：太慢，容易遗漏
- 创建新的 CSS 类：增加复杂性，不符合现有代码风格

### Decision 2: 文本颜色对比度标准

**Decision**: 使用 `var(--text-primary)` 替换所有主要文本的灰色

**Rationale**:
- 符合 WCAG 2.1 AA 标准（对比度 ≥ 4.5:1）
- 与现有主题系统一致
- 已在其他页面验证可行

**Alternatives Considered**:
- 使用中间灰度：对比度不足，可读性差
- 保持原有灰色：不符合用户需求

### Decision 3: 修改顺序

**Decision**: 按用户访问频率和影响范围排序

**Rationale**:
- 优先修复用户最常访问的页面（/traders, /competition）
- 最大化用户体验改善的即时效果
- 降低批量修改的风险

**Alternatives Considered**:
- 按文件名字母顺序：不考虑用户影响
- 按代码行数：不反映实际重要性

## Implementation Notes

- 所有修改都是样式层面的，不涉及逻辑变更
- 使用 `edit` 和 `multi_edit` 工具进行批量替换
- 每个文件修改后运行 `npm run lint:fix`
- 使用 Playwright MCP 进行视觉验证

## No Further Research Required

此功能的技术方案已经明确，可以直接进入实施阶段。
