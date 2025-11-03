# 🔄 如何将你的 PR 迁移到新格式

**语言：** [English](HOW_TO_MIGRATE_YOUR_PR.md) | [中文](HOW_TO_MIGRATE_YOUR_PR.zh-CN.md)

本指南帮助你将现有 PR 迁移以满足新的 PR 管理系统要求。

---

## 🎯 为什么要迁移？

虽然你的现有 PR **仍将按照当前标准审核和合并**，但将其迁移到新格式可以获得：

✅ **更快的审核** - 自动化检查尽早捕获问题
✅ **更好的反馈** - CI 提供清晰、可操作的反馈
✅ **更高质量** - 一致的代码标准
✅ **学习机会** - 了解我们新的贡献工作流程

---

## ⚡ 快速检查（推荐）

### 步骤 1：分析你的 PR

```bash
# 运行 PR 健康检查（只读，不修改任何内容）
./scripts/pr-check.sh
```

这将分析你的 PR 并告诉你：
- ✅ 什么是好的
- ⚠️ 什么需要注意
- 💡 如何修复问题
- 📊 整体健康评分

### 步骤 2：修复问题

根据建议，手动修复问题。常见修复：

```bash
# Rebase 到最新 dev
git fetch upstream && git rebase upstream/dev

# 格式化 Go 代码
go fmt ./...

# 运行测试
go test ./...

# 格式化前端代码
cd web && npm run lint -- --fix
```

### 步骤 3：再次运行检查

```bash
# 验证所有问题都已修复
./scripts/pr-check.sh
```

### 步骤 4：推送更改

```bash
git push -f origin <your-pr-branch>
```

### 脚本做什么

1. ✅ 与最新的 `upstream/dev` 同步
2. ✅ Rebase 你的更改
3. ✅ 格式化 Go 代码（`go fmt`）
4. ✅ 运行 Go linting（`go vet`）
5. ✅ 运行测试
6. ✅ 格式化前端代码（如果适用）
7. ✅ 推送更改到你的 PR

---

## 🛠️ 手动迁移（逐步指南）

如果你更喜欢手动操作：

### 步骤 1：与 Upstream 同步

```bash
# 如果还没添加 upstream，添加它
git remote add upstream https://github.com/tinkle-community/nofx.git

# 获取最新更改
git fetch upstream

# Rebase 你的分支
git checkout <your-pr-branch>
git rebase upstream/dev
```

### 步骤 2：后端检查（Go）

```bash
# 格式化 Go 代码
go fmt ./...

# 运行 linting
go vet ./...

# 运行测试
go test ./...

# 如果有更改，提交它们
git add .
git commit -m "chore: format and fix backend issues"
```

### 步骤 3：前端检查（如果适用）

```bash
cd web

# 安装依赖
npm install

# 修复 linting 问题
npm run lint -- --fix

# 检查类型
npm run type-check

# 测试构建
npm run build

cd ..

# 提交任何修复
git add .
git commit -m "chore: fix frontend issues"
```

### 步骤 4：更新 PR 标题（如果需要）

确保你的 PR 标题遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

示例：
feat(exchange): add OKX integration
fix(trader): resolve position tracking bug
docs(readme): update installation guide
```

**类型：**
- `feat` - 新功能
- `fix` - Bug 修复
- `docs` - 文档
- `refactor` - 代码重构
- `perf` - 性能改进
- `test` - 测试更新
- `chore` - 构建/配置更改
- `security` - 安全改进

### 步骤 5：推送更改

```bash
git push -f origin <your-pr-branch>
```

---

## 📋 检查清单

迁移后，验证：

- [ ] PR 已基于最新 `dev` rebase
- [ ] 没有合并冲突
- [ ] 后端测试在本地通过
- [ ] 前端构建成功
- [ ] PR 标题遵循 Conventional Commits 格式
- [ ] 所有 commit 都有意义
- [ ] 更改已推送到 GitHub

---

## 🤖 迁移后会发生什么？

推送更改后：

1. **自动化检查将运行**（不会阻止合并，只提供反馈）
2. **你将收到评论**，包含检查结果和建议
3. **维护者将审核** 你的 PR，有了新的上下文
4. **更快的审核** 得益于预检查

---

## ❓ 故障排除

### "Rebase 冲突"

如果在 rebase 期间遇到冲突：

```bash
# 在编辑器中修复冲突
# 然后：
git add <fixed-files>
git rebase --continue

# 或中止并寻求帮助：
git rebase --abort
```

**需要帮助？** 在你的 PR 中评论，我们会协助！

### "测试失败"

如果测试失败：

```bash
# 运行测试查看错误
go test ./...

# 修复问题
# 然后提交并推送
git add .
git commit -m "fix: resolve test failures"
git push -f origin <your-pr-branch>
```

### "脚本不工作"

如果迁移脚本不工作：

1. 检查你是否安装了 Go 和 Node.js
2. 尝试手动迁移（上面的步骤）
3. 在你的 PR 评论中寻求帮助

---

## 💡 提示

**不想迁移？**
- 没关系！你的 PR 仍将被审核和合并
- 迁移是可选的但推荐的

**第一次使用 Git rebase？**
- 查看我们的 [Git 指南](https://git-scm.com/book/zh/v2/Git-%E5%88%86%E6%94%AF-%E5%8F%98%E5%9F%BA)
- 在你的 PR 中提问 - 我们在这里帮助！

**想了解更多？**
- [贡献指南](../../docs/i18n/zh-CN/CONTRIBUTING.md)
- [迁移公告](MIGRATION_ANNOUNCEMENT.zh-CN.md)
- [PR 审核指南](../maintainers/PR_REVIEW_GUIDE.zh-CN.md)

---

## 📞 需要帮助？

**迁移遇到困难？**
- 在你的 PR 中评论
- 在 [Telegram](https://t.me/nofx_dev_community) 提问
- 开启 [Discussion](https://github.com/tinkle-community/nofx/discussions)

**我们在这里帮助你成功！** 🚀

---

## 🎉 迁移后

迁移完成后：
1. ✅ 等待自动化检查运行
2. ✅ 处理评论中的任何反馈
3. ✅ 等待维护者审核
4. ✅ 合并时庆祝！🎉

**感谢你为 NOFX 做出贡献！**
