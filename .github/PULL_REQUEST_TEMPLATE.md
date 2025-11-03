# Pull Request | PR 提交

> **💡 提示 Tip:** 推荐 PR 标题格式 Recommended PR title format: `type(scope): description`
> 例如 Examples: `feat(trader): add new strategy` | `fix(api): resolve auth issue`
> 详情 Details: [PR Title Guide](./PR_TITLE_GUIDE.md)

---

## 📝 Description | 描述

<!-- Provide a brief summary of your changes -->
<!-- 简要描述你的变更 -->

**English:**

**中文：**

---

## 🎯 Type of Change | 变更类型

<!-- Mark the relevant option with an "x" -->
<!-- 在相关选项上打"x" -->

- [ ] 🐛 Bug fix | 修复 Bug（不影响现有功能的修复）
- [ ] ✨ New feature | 新功能（不影响现有功能的新增）
- [ ] 💥 Breaking change | 破坏性变更（会导致现有功能无法正常工作的修复或功能）
- [ ] 📝 Documentation update | 文档更新
- [ ] 🎨 Code style update | 代码样式更新（格式化、重命名等）
- [ ] ♻️ Refactoring | 重构（无功能变更）
- [ ] ⚡ Performance improvement | 性能优化
- [ ] ✅ Test update | 测试更新
- [ ] 🔧 Build/config change | 构建/配置变更
- [ ] 🔒 Security fix | 安全修复

---

## 🔗 Related Issues | 相关 Issue

<!-- Link related issues below. Use "Closes #123" to auto-close issues when PR is merged -->
<!-- 在下方关联相关 issue。使用 "Closes #123" 可以在 PR 合并时自动关闭 issue -->

- Closes # | 关闭 #
- Related to # | 相关 #

---

## 📋 Changes Made | 具体变更

<!-- List the specific changes you made -->
<!-- 列出你做的具体变更 -->

**English:**
- Change 1
- Change 2
- Change 3

**中文：**
- 变更 1
- 变更 2
- 变更 3

---

## 🧪 Testing | 测试

### Manual Testing | 手动测试

<!-- Describe how you tested your changes -->
<!-- 描述你如何测试你的变更 -->

- [ ] Tested locally | 本地测试通过
- [ ] Tested on testnet | 测试网测试通过（交易所集成相关）
- [ ] Tested with different configurations | 测试了不同配置
- [ ] Verified no existing functionality broke | 确认没有破坏现有功能

### Test Environment | 测试环境

- **OS | 操作系统:** [e.g. macOS, Ubuntu, Windows]
- **Go Version | Go 版本:** [e.g. 1.21.5]
- **Node Version | Node 版本:** [e.g. 18.x] (if applicable | 如适用)
- **Exchange | 交易所:** [if applicable | 如适用]

### Test Results | 测试结果

<!-- Paste relevant test output or describe results -->
<!-- 粘贴相关测试输出或描述结果 -->

```
Test output here | 测试输出
```

---

## 📸 Screenshots / Demo | 截图/演示

<!-- If applicable, add screenshots or video demo -->
<!-- 如适用，添加截图或视频演示 -->

<!-- For UI changes, include before/after screenshots -->
<!-- 对于 UI 变更，包含变更前后的截图 -->

**Before | 变更前:**


**After | 变更后:**


---

## ✅ Checklist | 检查清单

<!-- Mark completed items with an "x" -->
<!-- 在已完成的项目上打"x" -->

### Code Quality | 代码质量

- [ ] My code follows the project's code style | 我的代码遵循项目代码风格 ([Contributing Guide](../CONTRIBUTING.md))
- [ ] I have performed a self-review of my code | 我已进行代码自查
- [ ] I have commented my code, particularly in hard-to-understand areas | 我已添加代码注释，特别是难以理解的部分
- [ ] My changes generate no new warnings or errors | 我的变更没有产生新的警告或错误
- [ ] Code compiles successfully | 代码编译成功 (`go build` / `npm run build`)
- [ ] I have run `go fmt` (for Go code) | 我已运行 `go fmt`（Go 代码）
- [ ] I have run `npm run lint` (for frontend code) | 我已运行 `npm run lint`（前端代码）

### Testing | 测试

- [ ] I have added tests that prove my fix is effective or that my feature works | 我已添加证明修复有效或功能正常的测试
- [ ] New and existing unit tests pass locally | 新旧单元测试在本地通过
- [ ] I have tested on testnet (for trading/exchange changes) | 我已在测试网测试（交易/交易所变更）
- [ ] Integration tests pass | 集成测试通过

### Documentation | 文档

- [ ] I have updated the documentation accordingly | 我已相应更新文档
- [ ] I have updated the README if needed | 我已更新 README（如需要）
- [ ] I have added inline code comments where necessary | 我已在必要处添加代码注释
- [ ] I have updated type definitions (for TypeScript changes) | 我已更新类型定义（TypeScript 变更）
- [ ] I have updated API documentation (if applicable) | 我已更新 API 文档（如适用）

### Git

- [ ] My commits follow the conventional commits format | 我的提交遵循 Conventional Commits 格式 (`feat:`, `fix:`, etc.)
- [ ] I have rebased my branch on the latest `dev` branch | 我已将分支 rebase 到最新的 `dev` 分支
- [ ] There are no merge conflicts | 没有合并冲突
- [ ] Commit messages are clear and descriptive | 提交信息清晰明确

---

## 🔒 Security Considerations | 安全考虑

<!-- Answer these questions for security-sensitive changes -->
<!-- 对于安全敏感的变更，请回答以下问题 -->

- [ ] No API keys or secrets are hardcoded | 没有硬编码 API 密钥或密钥
- [ ] User inputs are properly validated | 用户输入已正确验证
- [ ] No SQL injection vulnerabilities introduced | 未引入 SQL 注入漏洞
- [ ] No XSS vulnerabilities introduced | 未引入 XSS 漏洞
- [ ] Authentication/authorization properly handled | 认证/授权已正确处理
- [ ] Sensitive data is encrypted | 敏感数据已加密
- [ ] N/A (not security-related) | 不适用（非安全相关）

---

## ⚡ Performance Impact | 性能影响

<!-- Describe any performance implications -->
<!-- 描述任何性能影响 -->

- [ ] No significant performance impact | 无显著性能影响
- [ ] Performance improved | 性能提升
- [ ] Performance may be impacted (explain below) | 性能可能受影响（请在下方说明）

<!-- If performance impacted, explain: -->
<!-- 如果性能受影响，请说明： -->

**English:**

**中文：**

---

## 🌐 Internationalization | 国际化

<!-- For UI/documentation changes -->
<!-- 对于 UI/文档变更 -->

- [ ] All user-facing text supports i18n | 所有面向用户的文本支持国际化
- [ ] Both English and Chinese versions provided | 提供了中英文版本
- [ ] N/A | 不适用

---

## 📚 Additional Notes | 补充说明

<!-- Any additional information for reviewers -->
<!-- 给审查者的任何补充信息 -->

**English:**

**中文：**

---

## 💰 For Bounty Claims | 赏金申请

<!-- Fill this section only if claiming a bounty -->
<!-- 仅在申请赏金时填写此部分 -->

- [ ] This PR is for bounty issue # | 此 PR 用于赏金 issue #
- [ ] All acceptance criteria from the bounty issue are met | 满足赏金 issue 的所有验收标准
- [ ] I have included a demo video/screenshots | 我已包含演示视频/截图
- [ ] I am ready for payment upon merge | 我准备好在合并后接收付款

**Payment Details | 付款详情:** <!-- Discuss privately with maintainers | 与维护者私下讨论 -->

---

## 🙏 Reviewer Notes | 审查者注意事项

<!-- Optional: anything specific you want reviewers to focus on? -->
<!-- 可选：你希望审查者关注的特定内容？ -->

**English:**

**中文：**

---

## 📋 PR Size Estimate | PR 大小估计

<!-- This helps reviewers plan their time -->
<!-- 这有助于审查者安排时间 -->

- [ ] 🟢 Small (< 100 lines) | 小（< 100 行）
- [ ] 🟡 Medium (100-500 lines) | 中（100-500 行）
- [ ] 🔴 Large (> 500 lines) | 大（> 500 行）

<!-- For large PRs, consider: -->
<!-- 对于大型 PR，考虑： -->
<!-- - Breaking into smaller, focused PRs | 拆分为更小、更专注的 PR -->
<!-- - Providing a detailed explanation | 提供详细说明 -->
<!-- - Highlighting the most important changes | 突出最重要的变更 -->

---

## 🎯 Review Focus Areas | 审查重点

<!-- Help reviewers know where to focus their attention -->
<!-- 帮助审查者了解重点关注的地方 -->

Please pay special attention to:
请特别注意：

- [ ] Logic changes | 逻辑变更
- [ ] Security implications | 安全影响
- [ ] Performance optimization | 性能优化
- [ ] API changes | API 变更
- [ ] Database schema changes | 数据库架构变更
- [ ] UI/UX changes | UI/UX 变更

---

**By submitting this PR, I confirm that:**
**提交此 PR，我确认：**

- [ ] I have read the [Contributing Guidelines](../CONTRIBUTING.md) | 我已阅读[贡献指南](../CONTRIBUTING.md)
- [ ] I agree to the [Code of Conduct](../CODE_OF_CONDUCT.md) | 我同意[行为准则](../CODE_OF_CONDUCT.md)
- [ ] My contribution is licensed under the MIT License | 我的贡献遵循 MIT 许可证
- [ ] I understand this is a voluntary contribution | 我理解这是自愿贡献
- [ ] I have the right to submit this code | 我有权提交此代码

---

<!--
🌟 感谢你的贡献！Thank you for your contribution!

贡献者来自世界各地，我们重视每一份贡献。
Contributors come from all around the world, and we value every contribution.

如果你是首次贡献，欢迎加入我们的社区！
If this is your first contribution, welcome to our community!

💬 需要帮助？Feel free to ask questions in:
- GitHub Discussions
- Discord: [链接 Link]
- Telegram: [链接 Link]
-->
