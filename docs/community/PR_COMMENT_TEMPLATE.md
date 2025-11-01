# 📢 PR Comment Template for Existing PRs

This template is for maintainers to comment on existing PRs to introduce the new system.

---

## Template (English)

```markdown
Hi @{username}! 👋

Thank you for your contribution to NOFX!

## 🚀 New PR Management System

We're introducing a new PR management system to improve code quality and make reviews faster. Your PR will **not be blocked** by these changes - we'll review it under current standards.

### ✨ Optional: Want to check your PR against new standards?

We've created a **PR health check tool** that analyzes your PR and gives you suggestions!

**How to use:**

```bash
# In your local fork, on your PR branch
cd /path/to/your/nofx-fork
git checkout <your-branch-name>

# Run the health check (reads only, doesn't modify)
./scripts/pr-check.sh
```

**What it does:**
- 🔍 Analyzes your PR (doesn't modify anything)
- ✅ Shows what's already good
- ⚠️ Points out issues
- 💡 Gives specific suggestions on how to fix
- 📊 Overall health score

**Then fix and re-check:**
```bash
# Fix the issues based on suggestions
# Run check again to verify
./scripts/pr-check.sh

# Push when everything looks good
git push origin <your-branch-name>
```

### 📖 Learn More

- [Migration Announcement](https://github.com/tinkle-community/nofx/blob/dev/docs/community/MIGRATION_ANNOUNCEMENT.md)
- [Contributing Guidelines](https://github.com/tinkle-community/nofx/blob/dev/CONTRIBUTING.md)

### ❓ Questions?

Just ask here! We're happy to help. 🙏

---

**Note:** This migration is **completely optional** for existing PRs. We'll review and merge your PR either way!
```

---

## Template (Chinese / 中文)

```markdown
嗨 @{username}！👋

感谢你为 NOFX 做出的贡献！

## 🚀 新的 PR 管理系统

我们正在引入新的 PR 管理系统，以提高代码质量并加快审核速度。你的 PR **不会被阻止** - 我们将按照当前标准审核它。

### ✨ 可选：想要检查你的 PR 吗？

我们创建了一个 **PR 健康检查工具**来帮助你看 PR 是否符合新标准！

**在你的本地 fork 中运行：**

```bash
# 在你的本地 fork 中，切换到你的 PR 分支
cd /path/to/your/nofx-fork
git checkout <your-branch-name>

# 运行健康检查（只读，不修改任何内容）
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
git push origin <your-branch-name>
```

### 📖 了解更多

- [迁移公告](https://github.com/tinkle-community/nofx/blob/dev/docs/community/MIGRATION_ANNOUNCEMENT.zh-CN.md)
- [贡献指南](https://github.com/tinkle-community/nofx/blob/dev/docs/i18n/zh-CN/CONTRIBUTING.md)

### ❓ 问题？

在这里提问即可！我们很乐意帮助。🙏

---

**注意：** 对于现有 PR，此迁移是**完全可选的**。无论如何我们都会审核和合并你的 PR！
```

---

## Quick Copy-Paste Template

For quick commenting on multiple PRs:

```markdown
👋 Hi! Thanks for your PR!

We're introducing a new PR system. Your PR won't be blocked - we'll review it normally.

**Want to check your PR?** Run this in your fork:
```bash
./scripts/pr-check.sh
```

[Learn more](https://github.com/tinkle-community/nofx/blob/dev/docs/community/MIGRATION_ANNOUNCEMENT.md) | This is optional!
```

---

## Bulk Comment Script (for maintainers)

```bash
#!/bin/bash

# Comment on all open PRs
gh pr list --state open --json number --jq '.[].number' | while read pr_number; do
  echo "Commenting on PR #$pr_number"

  gh pr comment "$pr_number" --body "👋 Hi! Thanks for your PR!

We're introducing a new PR system. Your PR won't be blocked - we'll review it normally.

**Want to check your PR?** Run this in your fork:
\`\`\`bash
./scripts/pr-check.sh
\`\`\`

[Learn more](https://github.com/tinkle-community/nofx/blob/dev/docs/community/MIGRATION_ANNOUNCEMENT.md) | This is optional!"

  echo "✅ Commented on PR #$pr_number"
  sleep 2  # Be nice to GitHub API
done
```

Save as `comment-all-prs.sh` and run:
```bash
chmod +x comment-all-prs.sh
./comment-all-prs.sh
```
