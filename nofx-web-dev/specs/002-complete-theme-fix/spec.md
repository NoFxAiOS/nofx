# Feature Specification: Complete Red Theme Conversion

**Feature Branch**: `002-complete-theme-fix`  
**Created**: 2025-01-11  
**Status**: Draft  
**Input**: User description: "完成红色主题转换：修复所有剩余的黄色元素和灰色文本可读性问题"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Traders Page with Clear Text (Priority: P1)

用户访问 `/traders` 页面时，所有文本（包括"AI交易员"、"AI模型"、"交易所"、"当前交易员"等标题）都应该清晰可读，使用深色文本而非灰色。所有黄色主题元素（如"0 活跃"徽章、图标）应该改为红色主题。

**Why this priority**: 这是用户登录后的主要页面，文本可读性直接影响用户体验和功能使用。

**Independent Test**: 访问 `/traders` 页面，检查所有标题和文本是否使用深色（`var(--text-primary)`），所有黄色元素是否已改为红色。

**Acceptance Scenarios**:

1. **Given** 用户已登录系统，**When** 访问 `/traders` 页面，**Then** 所有页面标题（"AI交易员"、"AI模型"、"交易所"、"当前交易员"）显示为深色文本，易于阅读
2. **Given** 用户在 `/traders` 页面，**When** 查看状态徽章和图标，**Then** 所有黄色元素已改为红色主题
3. **Given** 用户在浅色背景下，**When** 阅读任何文本，**Then** 文本与背景对比度符合 WCAG 2.1 AA 标准

---

### User Story 2 - Navigate All Pages with Consistent Theme (Priority: P2)

用户在所有页面（实时、配置、看板、常见问题等）之间导航时，应该看到一致的红色主题，没有黄色残留元素，所有文本都清晰可读。

**Why this priority**: 确保整个应用的视觉一致性和品牌统一性。

**Independent Test**: 依次访问所有主要页面，验证主题色一致性和文本可读性。

**Acceptance Scenarios**:

1. **Given** 用户在任何页面，**When** 查看按钮、链接、徽章，**Then** 所有交互元素使用红色主题而非黄色
2. **Given** 用户浏览不同页面，**When** 阅读各种文本内容，**Then** 所有文本使用适当的深色/浅色对比，无灰色文本可读性问题
3. **Given** 用户查看图表和数据可视化，**When** 观察颜色使用，**Then** 主题色统一为红色系

---

### User Story 3 - Configure Traders with Clear UI (Priority: P2)

用户在配置交易员、AI模型、交易所时，所有配置界面的文本、标签、按钮都应该清晰可读，使用红色主题。

**Why this priority**: 配置界面是关键功能，需要确保用户能够清楚地理解和操作所有选项。

**Independent Test**: 打开各种配置模态框和表单，验证文本可读性和主题一致性。

**Acceptance Scenarios**:

1. **Given** 用户打开配置模态框，**When** 查看表单标签和说明文本，**Then** 所有文本使用深色，易于阅读
2. **Given** 用户在配置界面，**When** 与按钮和链接交互，**Then** 所有交互元素使用红色主题
3. **Given** 用户查看配置选项，**When** 阅读帮助文本和提示，**Then** 次要文本使用适当的灰度，但仍然清晰可读

---

### Edge Cases

- 当用户使用高对比度显示设置时，文本仍然保持可读性
- 在不同屏幕尺寸和分辨率下，颜色对比度保持一致
- 暗色模式切换时（如果将来支持），主题色正确转换

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统必须将所有硬编码的黄色颜色值（`#F0B90B`、`var(--brand-yellow)`、`var(--binance-yellow)`）替换为对应的红色值（`var(--brand-red)`）
- **FR-002**: 系统必须将所有灰色文本颜色（`#848E9C`、`#EAECEF`、`var(--brand-light-gray)` 用于文本时）替换为深色文本颜色（`var(--text-primary)`）
- **FR-003**: 系统必须确保所有背景色使用浅色主题变量（`var(--background)`、`var(--panel-bg)`、`var(--background-elevated)`）而非黑色或深色
- **FR-004**: 系统必须在以下页面应用修复：
  - `/traders` (AITradersPage.tsx)
  - `/competition` (CompetitionPage.tsx)
  - `/dashboard` (已完成)
  - FAQ 相关页面 (FAQContent.tsx, FAQSidebar.tsx, FAQLayout.tsx, FAQSearchBar.tsx)
  - 配置模态框 (TraderConfigModal.tsx, TraderConfigViewModal.tsx)
  - 其他包含黄色/灰色问题的组件
- **FR-005**: 系统必须保持图表组件（EquityChart.tsx、ComparisonChart.tsx）的颜色方案与红色主题一致
- **FR-006**: 系统必须确保所有文本与背景的对比度符合 WCAG 2.1 AA 标准（至少 4.5:1）
- **FR-007**: 系统必须保持现有功能不受影响，仅进行视觉样式修改

### Affected Files

根据代码搜索结果，需要修改的主要文件：

**高优先级（黄色元素最多）**:
- `src/components/AITradersPage.tsx` (25处黄色)
- `src/components/TraderConfigModal.tsx` (22处黄色)
- `src/components/faq/FAQContent.tsx` (17处黄色)
- `src/components/AILearning.tsx` (9处黄色)
- `src/components/CompetitionPage.tsx` (7处黄色)

**中优先级（灰色文本最多）**:
- `src/components/AITradersPage.tsx` (83处灰色)
- `src/components/TraderConfigModal.tsx` (41处灰色)
- `src/components/EquityChart.tsx` (22处灰色)
- `src/components/CompetitionPage.tsx` (21处灰色)
- `src/components/ComparisonChart.tsx` (15处灰色)

**其他需要修复的文件**:
- `src/components/EquityChart.tsx`
- `src/components/faq/FAQSidebar.tsx`
- `src/components/ResetPasswordPage.tsx`
- `src/components/TraderConfigViewModal.tsx`
- `src/components/faq/FAQLayout.tsx`
- `src/components/Header.tsx`
- `src/components/faq/FAQSearchBar.tsx`

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 所有页面的文本对比度测试通过 WCAG 2.1 AA 标准（使用浏览器开发工具或在线对比度检查器验证）
- **SC-002**: 使用 `grep` 搜索代码库，确认没有剩余的黄色颜色值（`#F0B90B`、`brand-yellow`、`binance-yellow`）用于主题元素
- **SC-003**: 使用 `grep` 搜索代码库，确认灰色文本颜色（`#848E9C`、`#EAECEF`）仅用于次要信息，主要文本使用深色
- **SC-004**: 通过 Playwright 自动化测试或手动测试，验证所有主要页面（/traders, /competition, /dashboard, /faq）的视觉一致性
- **SC-005**: 用户反馈确认文本可读性问题已解决，没有"看不清楚"的抱怨
- **SC-006**: 所有交互元素（按钮、链接、徽章）统一使用红色主题，视觉上与品牌一致
