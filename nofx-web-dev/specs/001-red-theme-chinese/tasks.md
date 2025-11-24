# Task Breakdown: Red Theme and Chinese Language

**Feature**: 001-red-theme-chinese  
**Branch**: `001-red-theme-chinese`  
**Generated**: 2025-11-11  
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

## Overview

将应用主题从黑色背景+黄色强调色改为浅灰色背景+红色强调色（#E50012），并强制所有用户使用中文界面。

**Implementation Strategy**: MVP-first, incremental delivery
- **MVP Scope**: User Story 1 (Theme Color Update) - 提供立即可见的视觉变化
- **Next Increment**: User Story 2 (Chinese Language) - 独立于主题，可单独部署

**Total Tasks**: 15
- Setup: 2 tasks
- Foundational: 3 tasks
- User Story 1: 6 tasks
- User Story 2: 3 tasks
- Polish: 1 task

**Parallel Opportunities**: 8 tasks marked with [P] can be executed in parallel

---

## Phase 1: Setup

准备开发环境和工具。

- [x] T001 备份当前主题配置文件 src/index.css
- [x] T002 创建色值对比度验证脚本或准备在线工具 (WebAIM Contrast Checker)

**Completion Criteria**: 
- ✅ 备份文件已创建 (src/index.css.backup)
- ✅ 对比度验证工具已就绪

---

## Phase 2: Foundational Tasks

阻塞性前置任务，必须在实现用户故事前完成。

- [x] T003 [P] 分析并记录所有使用黄色 (#f0b90b, yellow) 的组件和文件
- [x] T004 [P] 分析并记录所有使用黑色背景 (#000000, #0a0a0a) 的组件和文件
- [x] T005 验证新红色 (#E50012) 和浅灰色背景 (#FAFAFA) 的 WCAG 2.1 AA 对比度

**Completion Criteria**: 
- ✅ 所有颜色引用已记录在文档中
- ✅ 对比度验证通过 (>=4.5:1 for normal text, >=3:1 for large text)

---

## Phase 3: User Story 1 - Theme Color Update (P1)

**Goal**: 用户打开应用程序时，看到全新的视觉设计，采用浅色背景配红色强调色方案。

**Independent Test**: 
```bash
# 启动开发服务器
npm run dev

# 访问 http://localhost:3000
# 验证：
# 1. 页面背景为浅灰色 (#FAFAFA)
# 2. 按钮、链接为红色 (#E50012)
# 3. 文本可读性良好
# 4. 无黑色背景或黄色强调色残留
```

### CSS 变量更新

- [x] T006 [US1] 更新 src/index.css 中的品牌颜色变量 (--brand-yellow → --brand-red: #E50012)
- [x] T007 [US1] 更新 src/index.css 中的背景颜色变量 (--background: #000000 → #FAFAFA)
- [x] T008 [US1] 更新 src/index.css 中的前景/文本颜色变量 (--foreground: #eaecef → #1A1A1A)
- [x] T009 [US1] 更新 src/index.css 中的面板和边框颜色变量 (适配浅色主题)
- [x] T010 [US1] 更新 src/index.css 中的阴影样式 (从深色阴影改为浅色阴影)

### 硬编码颜色替换

- [x] T011 [P] [US1] 查找并替换 src/App.tsx 中的所有硬编码颜色 (inline styles)

**Acceptance Criteria** (from spec.md):
- ✅ 应用正在运行时，所有页面背景为浅灰色
- ✅ 所有交互元素（按钮、链接）使用红色强调色
- ✅ 高亮和选中元素为红色
- ✅ 配色方案在所有页面保持一致

**Parallel Execution Example**:
```bash
# T006-T010 可以在同一个编辑会话中完成（修改同一文件）
# T011 可以并行处理（不同文件）
```

---

## Phase 4: User Story 2 - Chinese Language as Default (P2)

**Goal**: 用户首次打开应用程序时，看到所有界面文本以中文显示。

**Independent Test**: 
```bash
# 清除浏览器 localStorage
localStorage.clear()

# 刷新页面
# 验证：
# 1. 所有导航菜单显示为中文
# 2. 所有按钮和标签显示为中文
# 3. 无语言选择器可见
```

### 语言上下文修改

- [x] T012 [US2] 修改 src/contexts/LanguageContext.tsx 强制默认语言为 'zh'
- [x] T013 [US2] 移除 src/contexts/LanguageContext.tsx 中的 localStorage 读写逻辑
- [x] T014 [P] [US2] 在 src/components/Header.tsx 或 HeaderBar.tsx 中隐藏语言选择器 UI (如果存在)

**Acceptance Criteria** (from spec.md):
- ✅ 应用首次加载时所有文本显示为中文
- ✅ 错误消息和通知显示为中文
- ✅ 中文为默认语言，无需手动选择
- ✅ 表单标签和占位符显示为中文

**Parallel Execution Example**:
```bash
# T012-T013 必须顺序执行（修改同一文件）
# T014 可以并行处理（不同文件）
```

---

## Phase 5: Polish & Cross-Cutting Concerns

最终优化和验证。

- [x] T015 在浏览器中手动测试所有主要页面和组件，验证主题和语言一致性

**Final Validation Checklist**:
- [ ] 所有页面背景为浅灰色
- [ ] 所有按钮和链接为红色
- [ ] 文本对比度符合 WCAG AA 标准
- [ ] 界面语言强制为中文
- [ ] 无语言选择选项可见
- [ ] 未翻译内容正确显示原文
- [ ] 跨浏览器测试通过 (Chrome, Firefox, Safari, Edge)

---

## Task Dependencies

### Dependency Graph

```
Setup (T001-T002)
    ↓
Foundational (T003-T005)
    ↓
    ├─→ User Story 1 (T006-T011) [可独立完成和测试]
    │       ↓
    └─→ User Story 2 (T012-T014) [可独立完成和测试]
            ↓
        Polish (T015)
```

### Story Completion Order

1. **Must Complete First**: Setup + Foundational (T001-T005)
2. **Independent Stories** (can be done in any order):
   - User Story 1 (T006-T011) - **推荐先做** (视觉变化更明显)
   - User Story 2 (T012-T014) - 可以后做或并行
3. **Must Complete Last**: Polish (T015)

### Blocking Relationships

- T006-T011 **require** T003-T005 (需要知道哪些文件要修改)
- T012-T014 **independent** of T006-T011 (可以并行)
- T015 **requires** all previous tasks

---

## Parallel Execution Opportunities

### User Story 1 Parallelization

**Batch 1** (same file - sequential):
```bash
# T006-T010: 修改 src/index.css
# 必须顺序执行，但可以在一次编辑会话中完成
```

**Batch 2** (different file - parallel):
```bash
# T011: 修改 src/App.tsx
# 可以与 T006-T010 并行准备
```

### User Story 2 Parallelization

**Batch 1** (same file - sequential):
```bash
# T012-T013: 修改 src/contexts/LanguageContext.tsx
```

**Batch 2** (different file - parallel):
```bash
# T014: 修改 src/components/Header.tsx
# 可以与 T012-T013 并行
```

---

## Implementation Notes

### CSS Variable Changes (T006-T010)

**From** (Dark Theme + Yellow):
```css
:root {
  --brand-yellow: #f0b90b;
  --binance-yellow: #f0b90b;
  --binance-yellow-dark: #c99400;
  --binance-yellow-light: #fcd535;
  --background: #000000;
  --panel-bg: #0a0a0a;
  --foreground: #eaecef;
  --text-primary: #eaecef;
  --text-secondary: #848e9c;
}
```

**To** (Light Theme + Red):
```css
:root {
  --brand-red: #E50012;
  --accent-red: #E50012;
  --brand-red-dark: #C40010;
  --brand-red-light: #FF1A2E;
  --background: #FAFAFA;
  --panel-bg: #FFFFFF;
  --foreground: #1A1A1A;
  --text-primary: #1A1A1A;
  --text-secondary: #616161;
}
```

### Language Context Changes (T012-T013)

**From**:
```typescript
const [language, setLanguage] = useState<Language>(() => {
  const saved = localStorage.getItem('language')
  return saved === 'en' || saved === 'zh' ? saved : 'en'
})

const handleSetLanguage = (lang: Language) => {
  setLanguage(lang)
  localStorage.setItem('language', lang)
}
```

**To**:
```typescript
const [language] = useState<Language>('zh') // Fixed to Chinese

const handleSetLanguage = () => {
  // No-op or not exposed
}
```

---

## Testing Strategy

### Manual Testing Checklist

**Theme (User Story 1)**:
- [ ] Landing page 背景为浅灰色
- [ ] Dashboard 背景为浅灰色
- [ ] Login/Register 页面背景为浅灰色
- [ ] 所有按钮为红色 (#E50012)
- [ ] 所有链接为红色
- [ ] Hover 状态正确（红色加深）
- [ ] 图表颜色协调（绿色盈利、红色亏损保持）

**Language (User Story 2)**:
- [ ] Header 导航为中文
- [ ] 所有按钮标签为中文
- [ ] 表单字段标签为中文
- [ ] 错误消息为中文
- [ ] 无语言选择器可见

**Cross-Browser**:
- [ ] Chrome
- [ ] Firefox
- [ ] Safari
- [ ] Edge

### Automated Testing (Optional)

如果需要自动化测试，可以添加：
```typescript
// tests/theme.test.ts
describe('Theme Colors', () => {
  it('should use red accent color', () => {
    const root = document.documentElement
    const red = getComputedStyle(root).getPropertyValue('--brand-red')
    expect(red.trim()).toBe('#E50012')
  })
})

// tests/language.test.tsx
describe('Language Context', () => {
  it('should default to Chinese', () => {
    const { result } = renderHook(() => useLanguage())
    expect(result.current.language).toBe('zh')
  })
})
```

---

## Rollback Plan

如果需要回滚：

```bash
# 1. 恢复 CSS 配置
cp src/index.css.backup src/index.css

# 2. 恢复 Git 提交
git revert <commit-hash>

# 或直接 checkout 文件
git checkout HEAD~1 -- src/index.css src/contexts/LanguageContext.tsx
```

---

## Success Metrics

From spec.md Success Criteria:

- ✅ **SC-001**: 所有页面显示浅灰色背景 (#FAFAFA)
- ✅ **SC-002**: 所有主要交互元素使用 #E50012 红色
- ✅ **SC-003**: 所有用户界面语言强制为中文
- ✅ **SC-004**: 未翻译内容正确显示原文
- ✅ **SC-005**: 对比度符合 WCAG 2.1 AA 标准
- ✅ **SC-006**: 用户能在 3 秒内识别新主题

---

## File Changes Summary

**Modified Files** (2):
1. `src/index.css` - CSS 变量更新 (T006-T010)
2. `src/contexts/LanguageContext.tsx` - 语言默认值和逻辑 (T012-T013)

**Potentially Modified** (2):
3. `src/App.tsx` - 硬编码颜色替换 (T011)
4. `src/components/Header.tsx` 或 `HeaderBar.tsx` - 隐藏语言选择器 (T014)

**No Changes Needed**:
- `src/i18n/translations.ts` - 翻译字典已有中文
- `src/components/**` - 通过 CSS 变量自动应用
- `src/pages/**` - 通过 CSS 变量自动应用
- 所有其他文件 - 无需修改

---

**Ready to implement**: 所有任务已定义，可以开始执行 `/speckit.implement` 或手动按任务列表实施。
