# Pull Request - Frontend | 前端 PR

> 💡 提示 Tip: 推荐 PR 标题格式 `type(scope): description`
> 示例 Example: `feat(web): add FAQ page with search & sidebar`

---

## 📝 Description | 描述

**English:**
- Add a new FAQ page with searchable content, categorized sidebar, and smooth in-page navigation.
- Integrate FAQ into header navigation and routing (`/faq`).
- Provide full i18n coverage (English/Chinese) and structured FAQ data source.
- Include a user feedback analysis doc (`web/faq.md`) to inform FAQ content.

**中文：**
- 新增 FAQ 页面：支持搜索、按分类的侧边目录、页面内平滑定位。
- 将 FAQ 集成到导航与路由（路径 `/faq`）。
- 完成中英双语文案与 i18n 键值，FAQ 文案由结构化数据驱动。
- 增加 `web/faq.md` 用户问题分析文档，用于支撑 FAQ 内容。

---

## 🎯 Type of Change | 变更类型

- [x] ✨ New feature | 新功能
- [ ] 🐛 Bug fix | 修复 Bug
- [ ] 💥 Breaking change | 破坏性变更
- [ ] 🎨 Code style update | 代码样式更新
- [ ] ♻️ Refactoring | 重构
- [ ] ⚡ Performance improvement | 性能优化

---

## 🔗 Related Issues | 相关 Issue

- Closes #
- Related to #

---

## 📋 Changes Made | 具体变更

**English:**
- Add `FAQPage` and route handling in `web/src/App.tsx` (mounts at `/faq`).
- Add header navigation entry for FAQ in `web/src/components/landing/HeaderBar.tsx`.
- Add FAQ components: `FAQLayout`, `FAQSidebar`, `FAQContent`, `FAQSearchBar`.
- Add structured FAQ data in `web/src/data/faqData.ts`.
- Add bilingual strings in `web/src/i18n/translations.ts` (English/Chinese).
- Add user feedback analysis doc `web/faq.md`.

**中文：**
- 在 `web/src/App.tsx` 增加 FAQ 页面与路由（`/faq`）。
- 在 `web/src/components/landing/HeaderBar.tsx` 增加 FAQ 导航入口。
- 新增 FAQ 组件：`FAQLayout`、`FAQSidebar`、`FAQContent`、`FAQSearchBar`。
- 新增结构化 FAQ 数据 `web/src/data/faqData.ts`。
- 在 `web/src/i18n/translations.ts` 增加双语文案键值（中/英）。
- 新增用户反馈分析文档 `web/faq.md`。

---

## 📸 Screenshots / Demo | 截图/演示

**Before | 变更前:** N/A

**After | 变更后:**
- Visit `/faq` to view the new FAQ page with search and sidebar.
- 访问 `/faq` 查看带搜索与侧边目录的新 FAQ 页面。

---

## 🧪 Testing | 测试

### Test Environment | 测试环境
- **OS | 操作系统:** macOS 26.x (Sequoia)
- **Node Version | Node 版本:** v22.13.1
- **Browser(s) | 浏览器:** Chrome (latest)

### Manual Testing | 手动测试
- [ ] Tested in development mode | 开发模式测试通过
- [x] Tested production build | 生产构建测试通过（`npm --prefix web run build`）
- [ ] Tested on multiple browsers | 多浏览器测试通过
- [ ] Tested responsive design | 响应式设计测试通过
- [ ] Verified no existing functionality broke | 确认没有破坏现有功能

---

## 🌐 Internationalization | 国际化

- [x] All user-facing text supports i18n | 所有面向用户的文本支持国际化
- [x] Both English and Chinese versions provided | 提供了中英文版本
- [ ] N/A | 不适用

---

## ✅ Checklist | 检查清单

### Code Quality | 代码质量
- [x] Code builds successfully | 代码构建成功（`npm --prefix web run build`）
- [ ] Ran `npm run lint` | 已运行 `npm run lint`
- [ ] Code follows project style | 代码遵循项目风格
- [ ] Self-review completed | 已完成代码自查
- [ ] Comments added for complex logic | 已添加必要注释
- [ ] No console errors or warnings | 无控制台错误或警告

### Testing | 测试
- [ ] Component tests added/updated | 已添加/更新组件测试
- [ ] Tests pass locally | 测试在本地通过

### Documentation | 文档
- [x] Updated relevant documentation | 已更新相关文档（`web/faq.md`）
- [ ] Updated type definitions (TypeScript) | 已更新类型定义
- [ ] Added JSDoc comments where necessary | 已添加 JSDoc 注释

### Git
- [x] Commits follow conventional format | 提交遵循 Conventional Commits 格式
- [ ] Rebased on latest `dev` branch | 已 rebase 到最新 `dev` 分支
- [x] No merge conflicts | 无合并冲突

---

## 📚 Additional Notes | 补充说明

**English:**
- Kept changes scoped to web UI and i18n; no backend impact.
- Large FAQ bundle is data-driven and easy to extend via `web/src/data/faqData.ts` and `translations.ts`.

**中文：**
- 变更仅影响前端与国际化，无后端影响。
- FAQ 内容为数据驱动，后续可在 `web/src/data/faqData.ts` 与 `translations.ts` 中扩展。

---

**By submitting this PR, I confirm | 提交此 PR，我确认：**

- [ ] I have read the [Contributing Guidelines](./CONTRIBUTING.md) | 已阅读贡献指南
- [ ] I agree to the [Code of Conduct](./CODE_OF_CONDUCT.md) | 同意行为准则
- [ ] My contribution is licensed under AGPL-3.0 | 贡献遵循 AGPL-3.0 许可证

---

🌟 Thank you for reviewing! | 感谢审核！

