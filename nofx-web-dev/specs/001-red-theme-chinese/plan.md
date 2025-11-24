# Implementation Plan: Red Theme and Chinese Language

**Branch**: `001-red-theme-chinese` | **Date**: 2025-11-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-red-theme-chinese/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

将应用主题从黑色背景+黄色强调色改为浅灰色背景+红色强调色（#E50012），并强制所有用户使用中文界面。主要通过修改 CSS 变量配置（`src/index.css`）和语言上下文（`src/contexts/LanguageContext.tsx`）实现，无需创建新组件或修改业务逻辑代码。

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: TypeScript 5.8, React 18.3  
**Primary Dependencies**: Vite 6.0, Tailwind CSS 3.4, React Context API  
**Storage**: localStorage (for language preference - to be removed)  
**Testing**: Vitest 2.1, Testing Library  
**Target Platform**: Modern browsers (Chrome, Firefox, Safari, Edge)
**Project Type**: Web application (single-page React app)  
**Performance Goals**: 主题切换无闪烁，语言加载 <100ms  
**Constraints**: 必须符合 WCAG 2.1 AA 标准，主题配置集中在 CSS 变量中  
**Scale/Scope**: 约 30+ 组件文件，所有页面和 UI 元素需要主题更新

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Reference: `.specify/memory/constitution.md`

### I. Component Reuse First
- [x] Verified existing components in `src/components/`, `src/hooks/`, `src/lib/`, `src/utils/`
- [x] Reviewed `package.json` dependencies for available libraries
- [x] Documented search results and justification for any new components
- [x] **GATE**: 不需要创建新组件，仅修改现有配置文件 (`src/index.css`, `src/contexts/LanguageContext.tsx`)

### II. Documentation Language Protocol
- [x] Document structure (headings, keywords) uses English
- [x] Human-readable content written in 中文 (Chinese)
- [x] Agent-readable content (code, APIs, paths) uses English
- [x] Code comments: 中文 for business logic, English for technical details

### III. Type Safety & Linting
- [x] All types defined in `src/types/` or inline (language type already exists)
- [x] No `any` types without justification
- [x] ESLint configuration followed
- [x] Prettier formatting applied

### IV. UI/UX Standards
- [x] Uses existing UI stack: Radix UI, Tailwind CSS, Lucide React, Framer Motion (no changes needed)
- [x] Responsive design (mobile-first) - maintained through CSS variables
- [x] Accessibility: WCAG 2.1 AA contrast ratios verified for red on light gray
- [x] Loading states and error boundaries - no changes needed

### V. State Management & Data Fetching
- [x] State approach: Context API for language (existing `LanguageContext`)
- [x] API calls use `httpClient` - no changes needed
- [x] Endpoints centralized - no changes needed
- [x] Types defined - `Language` type already exists in `src/i18n/translations.ts`

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
src/
├── index.css              # 主题颜色配置 (主要修改文件)
├── contexts/
│   └── LanguageContext.tsx  # 语言上下文 (主要修改文件)
├── i18n/
│   └── translations.ts      # 翻译定义 (无需修改，已有中文翻译)
├── components/              # UI 组件 (无需修改，使用 CSS 变量)
│   ├── ui/                  # Radix UI 组件
│   ├── Header.tsx
│   ├── LoginPage.tsx
│   └── ...
├── pages/                   # 页面组件 (无需修改)
├── hooks/                   # 自定义 hooks (无需修改)
├── lib/                     # 核心服务 (无需修改)
└── types/                   # TypeScript 类型 (无需修改)

tests/
└── (no test files exist yet, but can be added)
```

**Structure Decision**: 这是一个 React 单页应用，使用 CSS 变量进行主题管理。所有颜色定义集中在 `src/index.css` 的 `:root` 选择器中，组件通过 `var(--variable-name)` 引用这些变量。语言管理通过 React Context API 实现。本次修改仅涉及配置文件，不需要修改组件代码。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
