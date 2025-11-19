# 📢 PR 管理系统更新 - 贡献者须知

**语言：** [English](MIGRATION_ANNOUNCEMENT.md) | [中文](MIGRATION_ANNOUNCEMENT.zh-CN.md)

我们正在引入新的 PR 管理系统，以提高代码质量并让贡献变得更容易！本指南解释了变化内容以及你需要做什么。

---

## 🎯 有什么变化？

我们正在引入：

✅ **清晰的贡献指南** 与我们的[路线图](../roadmap/README.zh-CN.md)对齐
✅ **自动化检查**（测试、linting、安全扫描）
✅ **更好的标签** 用于组织和优先级排序
✅ **更快的审核周转** 通过预检查
✅ **透明的流程** 让你准确知道期望什么

---

## 📅 时间表

```
第 1-2 周：现有 PR 审核期
第 3 周：  软启动（检查仅是建议性的）
第 4 周+： 完全启动（检查是必需的）
```

**重要：** 这个推出是渐进式的。你将有时间适应！

---

## 🤔 这对你意味着什么

### 如果你有现有的打开的 PR

**好消息：** 你的 PR 不会被新规则阻塞！

- ✅ 你的 PR 将按照当前（宽松）标准进行审核
- ✅ 我们将在 1-2 周内审核并提供反馈
- ✅ 一些 PR 可能需要快速 rebase 或次要更新

**你可能需要做什么：**
1. **基于最新 `dev` 分支 rebase** 如果有冲突
2. **在 1 周内回应审核评论**
3. **保持耐心** 我们正在处理积压

**如果我不回应会怎样？**
- 我们可能会在 2 周不活动后关闭你的 PR
- 你随时可以稍后重新打开并更新！
- 没有恶意 - 我们只是在清理积压

### 🚀 想要检查你的 PR？（可选）

我们创建了一个 **PR 健康检查工具**来帮助你看 PR 是否符合新标准！

**在你的本地 fork 中运行：**
```bash
./scripts/pr-check.sh
```

**它做什么：**
- 🔍 分析你的 PR（不修改任何内容）
- ✅ 显示什么是好的
- ⚠️ 指出问题
- 💡 给你具体的修复建议
- 📊 整体健康评分

**然后修复问题并推送：**
```bash
# 修复问题（查看脚本的建议）
# 再次运行检查
./scripts/pr-check.sh

# 准备好后推送
git push -f origin <your-branch>
```

**📖 完整指南：** [如何迁移你的 PR](HOW_TO_MIGRATE_YOUR_PR.zh-CN.md)

**记住：** 对于现有 PR，这是完全**可选的**！

---

### 如果你要提交新的 PR

**时间很重要：**

#### 第 3 周（软启动）：
- ✅ 自动化检查将运行（测试、linting、安全性）
- ⚠️ **检查仅是建议性的** - 不会阻塞你的 PR
- ✅ 这是一个学习期 - 我们在这里帮助！
- ✅ 熟悉新的[贡献指南](../../docs/i18n/zh-CN/CONTRIBUTING.md)

#### 第 4 周+（完全启动）：
- ✅ 所有自动化检查必须通过才能合并
- ✅ PR 必须遵循 [Conventional Commits](https://www.conventionalcommits.org/) 格式
- ✅ 必须填写 PR 模板
- ✅ 必须与[路线图](../roadmap/README.zh-CN.md)优先级对齐

---

## ✅ 如何为新系统做准备

### 1. 阅读贡献指南

📖 [CONTRIBUTING.md](../../docs/i18n/zh-CN/CONTRIBUTING.md)

**关键点：**
- 我们接受与路线图对齐的 PR（安全性、AI、交易所、UI/UX）
- PR 应该集中且小型（<300 行优先）
- 使用 Conventional Commits 格式：`feat(area): description`
- 为新功能包含测试

### 2. 查看路线图

🗺️ [路线图](../roadmap/README.zh-CN.md)

**当前优先级（Phase 1）：**
- 🔒 安全增强
- 🧠 AI 模型集成
- 🔗 交易所集成（OKX、Bybit、Lighter、EdgeX）
- 🎨 UI/UX 改进
- ⚡ 性能优化
- 🐛 Bug 修复

**较低优先级（Phase 2+）：**
- 通用市场扩展（股票、期货）
- 高级 AI 功能
- 企业功能

💡 **专业提示：** 如果你的 PR 与 Phase 1 对齐，它会被更快审核！

### 3. 设置本地测试

提交 PR 前，在本地测试：

```bash
# 后端测试
go test ./...
go fmt ./...
go vet ./...

# 前端测试
cd web
npm run lint
npm run type-check
npm run build
```

这有助于你的 PR 第一次就通过自动化检查！

---

## 📝 PR 标题格式

使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```
<type>(<scope>): <description>

示例：
feat(exchange): add OKX futures support
fix(trader): resolve position tracking bug
docs(readme): update installation instructions
perf(ai): optimize prompt generation
```

**类型：**
- `feat` - 新功能
- `fix` - Bug 修复
- `docs` - 文档
- `refactor` - 代码重构
- `perf` - 性能改进
- `test` - 测试更新
- `chore` - 构建/配置变更
- `security` - 安全改进

---

## 🎯 什么是好的 PR？

### ✅ 好的 PR 示例

```
标题：feat(exchange): add OKX exchange integration

描述：
使用以下功能实现 OKX 交易所支持：
- 订单下达和取消
- 余额和仓位检索
- 杠杆配置
- 错误处理和重试逻辑

关闭 #123

测试：
- [x] 单元测试已添加并通过
- [x] 使用真实 API 手动测试
- [x] 文档已更新
```

**为什么好：**
- ✅ 清晰、描述性标题
- ✅ 解释了什么和为什么
- ✅ 链接到 issue
- ✅ 包含测试详情
- ✅ 小型、集中的变更

### ❌ 避免这些

**太模糊：**
```
标题：update code
描述：made some changes
```

**太大：**
```
标题：feat: complete rewrite of entire trading system
文件变更：2,500+
```

**不在路线图上：**
```
标题：feat: add support for stock trading
（这是 Phase 3，不是当前优先级）
```

---

## 🐛 如果你的 PR 检查失败

不要恐慌！我们在这里帮助。

**第 3 周（软启动）：**
- 检查是建议性的 - 我们会帮你解决问题
- 在你的 PR 评论中提问
- 我们可以指导你进行调试

**第 4 周+（完全启动）：**
- 检查必须通过，但我们仍然会帮助！
- 常见问题：
  - 测试失败 → 在本地运行 `go test ./...`
  - Linting 错误 → 运行 `go fmt` 和 `npm run lint`
  - 合并冲突 → 基于最新 `dev` rebase

**需要帮助？** 只管问！在你的 PR 中评论或联系：
- [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions)
- [Telegram 社区](https://t.me/nofx_dev_community)

---

## 💰 悬赏贡献者特别说明

如果你正在做悬赏任务：

✅ **你的 PR 获得优先审核**（24-48 小时）
✅ **额外支持** 以满足要求
✅ **过渡期间灵活** - 我们会与你合作

只需确保：
- 引用悬赏 issue 编号
- 满足所有验收标准
- 包含演示视频/截图

---

## ❓ 常见问题

### Q：我的现有 PR 会被拒绝吗？

**A：** 不会！现有 PR 使用宽松标准。我们可能会要求次要更新（rebase、小修复），但你不会被要求满足新的严格要求。

### Q：如果我无法通过新的 CI 检查怎么办？

**A：** 第 3 周是学习期。我们会帮你理解和修复问题。到第 4 周，你将熟悉这个流程！

### Q：这会减慢贡献速度吗？

**A：** 实际上不会！自动化检查尽早捕获问题，使审核更快。清晰的指南帮助你第一次就提交更好的 PR。

### Q：如果我是初学者，我还能贡献吗？

**A：** 绝对可以！查找标记为 `good first issue` 的 issue。我们在这里指导并帮助你成功。

### Q：我的 PR 很大（>1000 行）。我应该怎么做？

**A：** 考虑将其拆分为更小的 PR。这让你获得：
- ✅ 更快的审核
- ✅ 更容易的测试
- ✅ 更高的快速合并机会

需要帮助规划？在你的 PR 中提问即可！

### Q：如果我的功能不在路线图上怎么办？

**A：** 先开一个 issue 讨论！我们对好想法持开放态度，但在你花时间编码之前想确保对齐。

### Q：这将何时完全激活？

**A：** 第 4 周+（从公告日期起大约 4 周）。查看置顶的 Discussion 帖子了解确切日期。

---

## 🎉 对贡献者的好处

这个新系统通过以下方式帮助你：

✅ **更快的审核** - 自动化预检查减少审核时间
✅ **清晰的期望** - 你准确知道需要什么
✅ **更好的反馈** - 自动化检查尽早捕获问题
✅ **公平的优先级排序** - 路线图对齐的 PR 审核更快
✅ **表彰** - 贡献者等级和表彰计划

---

## 📚 资源

### 必读
- [贡献指南](../../docs/i18n/zh-CN/CONTRIBUTING.md) - 完整指南
- [路线图](../roadmap/README.zh-CN.md) - 当前优先级

### 有用链接
- [Conventional Commits](https://www.conventionalcommits.org/) - Commit 格式
- [Good First Issues](https://github.com/tinkle-community/nofx/labels/good%20first%20issue) - 适合初学者的任务
- [悬赏计划](../bounty-guide.md) - 获得报酬来贡献

### 获取帮助
- [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions) - 提问
- [Telegram](https://t.me/nofx_dev_community) - 社区聊天
- [Twitter](https://x.com/nofx_official) - 更新和公告

---

## 💬 欢迎反馈！

这是一个新系统，我们想要你的意见：

- 📝 什么不清楚？
- 🤔 你有什么顾虑？
- 💡 我们如何改进？

在[迁移反馈讨论](https://github.com/tinkle-community/nofx/discussions)中分享（链接待定）

---

## 🙏 谢谢你！

我们感谢你的贡献和在这次过渡期间的耐心。我们一起正在构建令人惊叹的东西！

**问题？** 不要犹豫提问。我们在这里帮助！🚀

---

**最后更新：** 2025-01-XX
**状态：** 公告（第 0 周）
**完全启动：** 第 4 周+（待定）
