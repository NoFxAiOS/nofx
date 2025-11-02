# 🚀 PR 管理系统设置指南

**语言：** [English](SETUP_GUIDE.md) | [中文](SETUP_GUIDE.zh-CN.md)

本指南将帮助你为 NOFX 设置和激活完整的 PR 管理系统。

---

## 📦 包含内容

PR 管理系统包括：

### 1. **文档**
- ✅ `CONTRIBUTING.md` - 贡献者指南
- ✅ `docs/maintainers/PR_REVIEW_GUIDE.md` - 审核者指南
- ✅ `docs/maintainers/PROJECT_MANAGEMENT.md` - 项目管理工作流程
- ✅ `docs/maintainers/SETUP_GUIDE.md` - 本文件

### 2. **GitHub 配置**
- ✅ `.github/PULL_REQUEST_TEMPLATE.md` - PR 模板（已存在）
- ✅ `.github/labels.yml` - 标签定义
- ✅ `.github/labeler.yml` - 自动标签规则
- ✅ `.github/workflows/pr-checks.yml` - 自动化 PR 检查

### 3. **自动化**
- ✅ 自动 PR 标签
- ✅ PR 大小检查
- ✅ CI/CD 测试
- ✅ 安全扫描
- ✅ Commit 信息验证

---

## 🔧 设置步骤

### 步骤 1：同步 GitHub 标签

创建 `.github/labels.yml` 中定义的标签：

```bash
# 选项 1：使用 gh CLI（推荐）
gh label list  # 查看当前标签
gh label delete <label-name>  # 如需要，删除旧标签
gh label create "priority: critical" --color "d73a4a" --description "Critical priority"
# ... 为 labels.yml 中的所有标签重复

# 选项 2：使用 GitHub Labeler Action（自动化）
# 工作流将在推送时自动同步标签
```

**或使用 GitHub Labeler Action**（添加到 `.github/workflows/sync-labels.yml`）：

```yaml
name: Sync Labels
on:
  push:
    branches: [main, dev]
    paths:
      - '.github/labels.yml'

jobs:
  labels:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: crazy-max/ghaction-github-labeler@v5
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          yaml-file: .github/labels.yml
```

### 步骤 2：启用 GitHub Actions

1. 前往 **Settings → Actions → General**
2. 启用 **"Allow all actions and reusable workflows"**
3. 设置 **Workflow permissions** 为 **"Read and write permissions"**
4. 勾选 **"Allow GitHub Actions to create and approve pull requests"**

### 步骤 3：设置分支保护规则

**对于 `main` 分支：**

1. 前往 **Settings → Branches → Add rule**
2. 分支名称模式：`main`
3. 配置：
   - ✅ Require a pull request before merging
   - ✅ Require approvals: **1**
   - ✅ Require status checks to pass before merging
     - 选择：`Backend Tests (Go)`
     - 选择：`Frontend Tests (React/TypeScript)`
     - 选择：`Security Scan`
   - ✅ Require conversation resolution before merging
   - ✅ Do not allow bypassing the above settings
   - ❌ Allow force pushes（禁用）
   - ❌ Allow deletions（禁用）

**对于 `dev` 分支：**

1. 与上面相同，但：
   - Require approvals: **1**
   - 宽松一些（如需要允许维护者绕过）

### 步骤 4：创建 GitHub Projects

1. 前往 **Projects → New project**
2. 创建 **"NOFX Development"** 看板
   - 模板：Board
   - 添加列：`Backlog`、`Triaged`、`In Progress`、`In Review`、`Done`
   - 添加视图：Sprint、Roadmap、By Area、Priority

3. 创建 **"Bounty Program"** 看板
   - 模板：Board
   - 添加列：`Available`、`Claimed`、`In Progress`、`Under Review`、`Paid`

### 步骤 5：启用 Discussions（可选但推荐）

1. 前往 **Settings → General → Features**
2. 启用 **"Discussions"**
3. 创建分类：
   - 💬 **General** - 一般讨论
   - 💡 **Ideas** - 功能想法和建议
   - 🙏 **Q&A** - 问答
   - 📢 **Announcements** - 重要更新
   - 🗳️ **Polls** - 社区投票

### 步骤 6：配置 Issue 模板

模板已存在于 `.github/ISSUE_TEMPLATE/` 中。验证它们是否正常工作：

1. 前往 **Issues → New issue**
2. 你应该看到：
   - 🐛 Bug Report
   - ✨ Feature Request
   - 💰 Bounty Claim

如果没有显示，检查文件是否为正确格式的 YAML 和 frontmatter。

### 步骤 7：设置 Code Owners（可选）

创建 `.github/CODEOWNERS`：

```
# 全局所有者
* @tinkle @zack

# 前端
/web/ @frontend-lead

# 交易所集成
/internal/exchange/ @exchange-lead

# AI 组件
/internal/ai/ @ai-lead

# 文档
/docs/ @tinkle @zack
*.md @tinkle @zack
```

### 步骤 8：配置通知

**对于维护者：**

1. 前往 **Settings → Notifications**
2. 启用：
   - ✅ Pull request reviews
   - ✅ Pull request pushes
   - ✅ Comments on issues and PRs
   - ✅ New issues
   - ✅ Security alerts

3. 设置电子邮件过滤器来组织通知

**对于仓库：**

1. 前往 **Settings → Webhooks**（如果与 Slack/Discord 集成）
2. 添加通知 webhook

---

## 📋 设置后检查清单

设置后，验证：

- [ ] 标签已创建并可见
- [ ] 分支保护规则已激活
- [ ] GitHub Actions 工作流在新 PR 上运行
- [ ] 自动标签工作（创建测试 PR）
- [ ] 创建 PR 时显示 PR 模板
- [ ] 创建 issue 时显示 issue 模板
- [ ] Projects 看板可访问
- [ ] CONTRIBUTING.md 在 README 中链接

---

## 🎯 如何使用系统

### 对于贡献者

1. **阅读** [CONTRIBUTING.md](../../../CONTRIBUTING.md)
2. **查看** [路线图](../../roadmap/README.zh-CN.md)了解优先级
3. **开启 issue** 或找到现有的
4. **使用模板创建 PR**
5. **处理审核反馈**
6. **庆祝** 当合并时！🎉

### 对于维护者

1. **每日：** 分类新 issue/PR（15分钟）
2. **每日：** 审查分配的 PR
3. **每周：** Sprint 计划（周一）和回顾（周五）
4. **遵循：** [PR 审核指南](PR_REVIEW_GUIDE.zh-CN.md)
5. **遵循：** [项目管理指南](PROJECT_MANAGEMENT.zh-CN.md)

### 对于悬赏猎人

1. **查看** 带有 `bounty` 标签的悬赏 issue
2. **通过评论认领** issue
3. **在截止日期前完成**
4. **提交 PR** 并填写悬赏认领部分
5. **合并后获得报酬**

---

## 🔍 测试系统

### 测试 1：创建测试 PR

```bash
# 创建测试分支
git checkout -b test/pr-system-check

# 进行小改动
echo "# Test" >> TEST.md

# 提交并推送
git add TEST.md
git commit -m "test: verify PR automation system"
git push origin test/pr-system-check

# 在 GitHub 上创建 PR
# 验证：
# - PR 模板加载
# - 应用了自动标签
# - CI 检查运行
# - 添加了大小标签
```

### 测试 2：创建测试 Issue

1. 前往 **Issues → New issue**
2. 选择 **Bug Report**
3. 填写模板
4. 提交
5. 验证：
   - 模板正确渲染
   - Issue 可以被标签
   - Issue 出现在项目看板中

### 测试 3：测试自动标签

创建改动不同区域文件的 PR：

```bash
# 测试 1：前端变更
git checkout -b test/frontend-label
touch web/src/test.tsx
git add . && git commit -m "test: frontend labeling"
git push origin test/frontend-label
# 应该得到 "area: frontend" 标签

# 测试 2：后端变更
git checkout -b test/backend-label
touch internal/test.go
git add . && git commit -m "test: backend labeling"
git push origin test/backend-label
# 应该得到 "area: backend" 标签
```

---

## 🐛 故障排除

### 问题：标签未同步

**解决方案：**
```bash
# 首先删除所有现有标签
gh label list --json name --jq '.[].name' | xargs -I {} gh label delete "{}" --yes

# 然后从 labels.yml 手动创建或通过 action 创建
```

### 问题：GitHub Actions 未运行

**检查：**
1. 仓库设置中启用了 Actions
2. 工作流文件在 `.github/workflows/` 中
3. YAML 语法有效
4. 权限设置正确

**调试：**
```bash
# 本地验证工作流
act pull_request  # 使用 'act' 工具
```

### 问题：分支保护阻止 PR

**检查：**
1. 必需的检查在工作流中定义
2. 检查名称完全匹配
3. 检查正在完成（没有卡住）

**临时修复：**
- 维护者可以在紧急情况下绕过
- 如果太严格，调整保护规则

### 问题：自动标签器不工作

**检查：**
1. `.github/labeler.yml` 存在且为有效 YAML
2. labeler.yml 中定义的标签在仓库中存在
3. 工作流有 `pull-requests: write` 权限

---

## 📊 监控和维护

### 每周回顾

每周检查这些指标：

```bash
# 使用 gh CLI
gh pr list --state all --json number,createdAt,closedAt
gh issue list --state all --json number,createdAt,closedAt

# 或使用 GitHub Insights
# Repository → Insights → Pulse, Contributors, Traffic
```

### 每月维护

- [ ] 如需要审查和更新标签
- [ ] 检查工作流中的过期依赖
- [ ] 如果流程变更更新 CONTRIBUTING.md
- [ ] 审查自动化效果
- [ ] 收集社区反馈

---

## 🎓 培训资源

### 对于新贡献者

- [首次贡献指南](https://github.com/firstcontributions/first-contributions)
- [如何写 Git Commit 信息](https://chris.beams.io/posts/git-commit/)
- [Conventional Commits](https://www.conventionalcommits.org/)

### 对于维护者

- [代码审核的艺术](https://google.github.io/eng-practices/review/)
- [GitHub 项目管理](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [维护者社区](https://maintainers.github.com/)

---

## 🎉 一切就绪！

PR 管理系统现在已准备好：

✅ 用清晰的指南引导贡献者
✅ 自动化重复任务
✅ 保持代码质量
✅ 系统性地跟踪进度
✅ 扩展社区

**有问题？** 在维护者频道联系我们或开启讨论。

**让我们构建令人惊叹的社区！🚀**
