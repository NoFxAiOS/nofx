# Tasks: Complete Red Theme Conversion

**Input**: Design documents from `/specs/002-complete-theme-fix/`
**Prerequisites**: plan.md, spec.md, research.md, quickstart.md

**Tests**: 不包含测试任务（纯样式修改，通过浏览器手动验证和 Playwright 截图验证）

**Organization**: 任务按用户故事分组，每个故事可以独立实施和测试

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可以并行运行（不同文件，无依赖）
- **[Story]**: 任务所属的用户故事（US1, US2, US3）
- 包含精确的文件路径

## Phase 1: Setup (共享基础设施)

**Purpose**: 项目初始化和验证

- [x] T001 验证开发环境：确认 Node.js 18+, npm 已安装，项目依赖已安装
- [x] T002 启动开发服务器：运行 `npm run dev`，确认 http://localhost:3000 可访问
- [x] T003 [P] 使用 Playwright 截图记录当前状态：访问 /traders, /competition, /faq 页面并截图

---

## Phase 2: Foundational (阻塞性前置条件)

**Purpose**: 核心基础设施，必须在任何用户故事之前完成

**⚠️ CRITICAL**: 在此阶段完成之前，不能开始任何用户故事工作

- [x] T004 验证 CSS 变量定义：检查 `src/index.css` 中 `--brand-red`, `--text-primary`, `--background` 等变量已正确定义
- [x] T005 创建颜色替换参考文档：在 `specs/002-complete-theme-fix/` 创建 `color-mapping.md`，列出所有颜色映射规则

**Checkpoint**: 基础准备就绪 - 用户故事实施现在可以并行开始

---

## Phase 3: User Story 1 - View Traders Page with Clear Text (Priority: P1) 🎯 MVP

**Goal**: 修复 `/traders` 页面的所有黄色元素和灰色文本可读性问题，这是用户登录后的主要页面

**Independent Test**: 访问 http://localhost:3000/traders，验证：
1. 所有标题（"AI交易员"、"AI模型"、"交易所"、"当前交易员"）使用深色文本
2. 所有黄色元素（徽章、图标）已改为红色
3. 文本在浅色背景上清晰可读

### Implementation for User Story 1

- [x] T006 [US1] 替换 `AITradersPage.tsx` 中的黄色颜色值：将所有 `#F0B90B`, `var(--brand-yellow)`, `var(--binance-yellow)` 替换为 `var(--brand-red)` 在 `src/components/AITradersPage.tsx`
- [x] T007 [US1] 替换 `AITradersPage.tsx` 中的灰色文本颜色：将所有 `#848E9C`, `#EAECEF` 用于文本时替换为 `var(--text-primary)` 在 `src/components/AITradersPage.tsx`
- [x] T008 [US1] 运行 linter：执行 `npm run lint:fix` 修复格式问题
- [x] T009 [US1] 浏览器验证：访问 /traders 页面，确认所有文本清晰可读，所有黄色元素已改为红色（需要清除缓存或重启 Vite）
- [x] T010 [US1] Playwright 截图验证：使用 Playwright MCP 截图 /traders 页面，对比修改前后（检测到缓存问题）

**Checkpoint**: 此时，User Story 1 应该完全功能正常且可独立测试

---

## Phase 4: User Story 2 - Navigate All Pages with Consistent Theme (Priority: P2)

**Goal**: 确保所有页面（实时、配置、看板、常见问题等）的红色主题一致性和文本可读性

**Independent Test**: 依次访问所有主要页面，验证主题色一致性和文本可读性

### Implementation for User Story 2 - Part A: 配置模态框

- [x] T011 [P] [US2] 替换 `TraderConfigModal.tsx` 中的黄色颜色值：将所有黄色替换为红色在 `src/components/TraderConfigModal.tsx`
- [x] T012 [P] [US2] 替换 `TraderConfigModal.tsx` 中的灰色文本颜色：将灰色文本替换为深色在 `src/components/TraderConfigModal.tsx`
- [x] T013 [US2] 运行 linter：执行 `npm run lint:fix`
- [ ] T014 [US2] 验证配置模态框：打开各种配置对话框，确认主题一致性

### Implementation for User Story 2 - Part B: 竞赛页面

- [x] T015 [P] [US2] 替换 `CompetitionPage.tsx` 中的黄色颜色值：在 `src/components/CompetitionPage.tsx`
- [x] T016 [P] [US2] 替换 `CompetitionPage.tsx` 中的灰色文本颜色：在 `src/components/CompetitionPage.tsx`
- [x] T017 [US2] 运行 linter：执行 `npm run lint:fix`
- [ ] T018 [US2] 验证竞赛页面：访问 /competition，确认主题一致性

### Implementation for User Story 2 - Part C: FAQ 页面

- [x] T019 [P] [US2] 替换 `FAQContent.tsx` 中的黄色颜色值：在 `src/components/faq/FAQContent.tsx`
- [x] T020 [P] [US2] 替换 `FAQSidebar.tsx` 中的黄色颜色值：在 `src/components/faq/FAQSidebar.tsx`
- [x] T021 [P] [US2] 替换 `FAQLayout.tsx` 中的黄色颜色值：在 `src/components/faq/FAQLayout.tsx`
- [x] T022 [P] [US2] 替换 `FAQSearchBar.tsx` 中的黄色颜色值：在 `src/components/faq/FAQSearchBar.tsx`
- [x] T023 [US2] 替换 FAQ 组件中的灰色文本颜色：在所有 FAQ 相关文件中
- [x] T024 [US2] 运行 linter：执行 `npm run lint:fix`
- [x] T025 [US2] 验证 FAQ 页面：访问 /faq，确认主题一致性

### Implementation for User Story 2 - Part D: 其他组件

- [ ] T026 [P] [US2] 替换 `AILearning.tsx` 中的黄色颜色值：在 `src/components/AILearning.tsx`
- [ ] T027 [P] [US2] 替换 `EquityChart.tsx` 中的黄色和灰色：在 `src/components/EquityChart.tsx`
- [ ] T028 [P] [US2] 替换 `ComparisonChart.tsx` 中的灰色：在 `src/components/ComparisonChart.tsx`
- [ ] T029 [P] [US2] 替换 `TraderConfigViewModal.tsx` 中的黄色和灰色：在 `src/components/TraderConfigViewModal.tsx`
- [ ] T030 [P] [US2] 替换 `ResetPasswordPage.tsx` 中的黄色和灰色：在 `src/components/ResetPasswordPage.tsx`
- [ ] T031 [P] [US2] 替换 `Header.tsx` 中的黄色和灰色：在 `src/components/Header.tsx`
- [ ] T032 [P] [US2] 替换 `FAQPage.tsx` 中的灰色：在 `src/pages/FAQPage.tsx`
- [ ] T033 [P] [US2] 替换 `httpClient.ts` 中的黄色（如果有）：在 `src/lib/httpClient.ts`
- [ ] T034 [US2] 运行 linter：执行 `npm run lint:fix`
- [ ] T035 [US2] 全面浏览器验证：访问所有主要页面，确认主题一致性

**Checkpoint**: 此时，User Stories 1 和 2 都应该独立工作

---

## Phase 5: User Story 3 - Configure Traders with Clear UI (Priority: P2)

**Goal**: 确保配置交易员、AI模型、交易所时，所有配置界面的文本、标签、按钮都清晰可读，使用红色主题

**Independent Test**: 打开各种配置模态框和表单，验证文本可读性和主题一致性

### Implementation for User Story 3

**Note**: 此用户故事的大部分工作已在 Phase 4 (US2) 中完成（TraderConfigModal.tsx, TraderConfigViewModal.tsx）

- [ ] T036 [US3] 验证所有配置界面：逐个打开并测试所有配置相关的模态框和表单
- [ ] T037 [US3] 检查配置界面的辅助文本：确保帮助文本和提示使用适当的灰度但仍清晰可读
- [ ] T038 [US3] 验证配置界面的交互元素：确认所有按钮、链接使用红色主题

**Checkpoint**: 所有用户故事现在都应该独立功能正常

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 影响多个用户故事的改进

- [ ] T039 [P] 全局颜色验证：使用 grep 搜索确认没有剩余的黄色颜色值（`#F0B90B`, `brand-yellow`, `binance-yellow`）
- [ ] T040 [P] 全局文本颜色验证：使用 grep 搜索确认灰色文本颜色仅用于次要信息
- [ ] T041 对比度测试：使用浏览器开发工具或在线对比度检查器验证所有文本符合 WCAG 2.1 AA 标准
- [ ] T042 [P] Playwright 全页面截图：为所有主要页面生成最终截图
- [ ] T043 代码清理：移除任何注释掉的旧颜色代码
- [ ] T044 更新文档：在 `specs/002-complete-theme-fix/` 添加完成总结和截图对比
- [ ] T045 最终验证：运行 quickstart.md 中的验证清单

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: 无依赖 - 可以立即开始
- **Foundational (Phase 2)**: 依赖 Setup 完成 - 阻塞所有用户故事
- **User Stories (Phase 3-5)**: 全部依赖 Foundational 阶段完成
  - 用户故事可以并行进行（如果有人力）
  - 或按优先级顺序进行（P1 → P2 → P2）
- **Polish (Phase 6)**: 依赖所有期望的用户故事完成

### User Story Dependencies

- **User Story 1 (P1)**: 可以在 Foundational (Phase 2) 后开始 - 不依赖其他故事
- **User Story 2 (P2)**: 可以在 Foundational (Phase 2) 后开始 - 不依赖 US1，可独立测试
- **User Story 3 (P2)**: 可以在 Foundational (Phase 2) 后开始 - 部分工作与 US2 重叠，但可独立验证

### Within Each User Story

- 按文件优先级顺序修改（高优先级文件先修改）
- 每个文件修改后运行 linter
- 每个部分完成后进行浏览器验证
- 故事完成后再移动到下一个优先级

### Parallel Opportunities

- Phase 1 中所有标记 [P] 的任务可以并行运行
- Phase 2 中所有标记 [P] 的任务可以并行运行
- Phase 2 完成后，所有用户故事可以并行开始（如果团队容量允许）
- User Story 2 中的不同部分（Part A, B, C, D）可以并行工作
- User Story 2 中标记 [P] 的任务可以并行运行
- 不同用户故事可以由不同团队成员并行工作

---

## Parallel Example: User Story 2 - Part D

```bash
# 同时启动 User Story 2 Part D 的所有文件修改：
Task: "替换 AILearning.tsx 中的黄色颜色值"
Task: "替换 EquityChart.tsx 中的黄色和灰色"
Task: "替换 ComparisonChart.tsx 中的灰色"
Task: "替换 TraderConfigViewModal.tsx 中的黄色和灰色"
Task: "替换 ResetPasswordPage.tsx 中的黄色和灰色"
Task: "替换 Header.tsx 中的黄色和灰色"
Task: "替换 FAQPage.tsx 中的灰色"
Task: "替换 httpClient.ts 中的黄色"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (修复 /traders 页面)
4. **STOP and VALIDATE**: 独立测试 User Story 1
5. 如果准备好就部署/演示

### Incremental Delivery

1. Complete Setup + Foundational → 基础准备就绪
2. Add User Story 1 → 独立测试 → 部署/演示（MVP！）
3. Add User Story 2 → 独立测试 → 部署/演示
4. Add User Story 3 → 独立测试 → 部署/演示
5. 每个故事都增加价值而不破坏之前的故事

### Parallel Team Strategy

使用多个开发者：

1. 团队一起完成 Setup + Foundational
2. Foundational 完成后：
   - Developer A: User Story 1 (AITradersPage.tsx)
   - Developer B: User Story 2 Part A+B (配置模态框 + 竞赛页面)
   - Developer C: User Story 2 Part C+D (FAQ + 其他组件)
3. 故事独立完成和集成

---

## Notes

- [P] 任务 = 不同文件，无依赖
- [Story] 标签将任务映射到特定用户故事以便追溯
- 每个用户故事都应该可以独立完成和测试
- 每个任务或逻辑组后提交
- 在任何检查点停止以独立验证故事
- 避免：模糊任务、相同文件冲突、破坏独立性的跨故事依赖

## Task Count Summary

- **Total Tasks**: 45
- **Setup (Phase 1)**: 3 tasks
- **Foundational (Phase 2)**: 2 tasks
- **User Story 1 (Phase 3)**: 5 tasks
- **User Story 2 (Phase 4)**: 25 tasks
- **User Story 3 (Phase 5)**: 3 tasks
- **Polish (Phase 6)**: 7 tasks
- **Parallel Opportunities**: 24 tasks marked [P]
- **Suggested MVP**: Complete through Phase 3 (User Story 1) = 10 tasks
