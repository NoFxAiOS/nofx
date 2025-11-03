# PR 标题指南

## 📋 概述

我们使用 **Conventional Commits** 格式来保持 PR 标题的一致性，但这是**建议性的**，不会阻止你的 PR 被合并。

## ✅ 推荐格式

```
type(scope): description
```

### 示例

```
feat(trader): add new trading strategy
fix(api): resolve authentication issue
docs: update README
chore(deps): update dependencies
ci(workflow): improve GitHub Actions
```

---

## 📖 详细说明

### Type（类型）- 必需

描述这次变更的类型：

| Type | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat(trader): add stop-loss feature` |
| `fix` | Bug 修复 | `fix(api): handle null response` |
| `docs` | 文档变更 | `docs: update installation guide` |
| `style` | 代码格式（不影响代码运行） | `style: format code with prettier` |
| `refactor` | 重构（既不是新功能也不是修复） | `refactor(exchange): simplify connection logic` |
| `perf` | 性能优化 | `perf(ai): optimize prompt processing` |
| `test` | 添加或修改测试 | `test(trader): add unit tests` |
| `chore` | 构建过程或辅助工具的变动 | `chore: update dependencies` |
| `ci` | CI/CD 相关变更 | `ci: add test coverage report` |
| `security` | 安全相关修复 | `security: update vulnerable dependencies` |
| `build` | 构建系统或外部依赖项变更 | `build: upgrade webpack to v5` |

### Scope（范围）- 可选

描述这次变更影响的范围：

| Scope | 说明 |
|-------|------|
| `exchange` | 交易所相关 |
| `trader` | 交易员/交易策略 |
| `ai` | AI 模型相关 |
| `api` | API 接口 |
| `ui` | 用户界面 |
| `frontend` | 前端代码 |
| `backend` | 后端代码 |
| `security` | 安全相关 |
| `deps` | 依赖项 |
| `workflow` | GitHub Actions workflows |
| `github` | GitHub 配置 |
| `actions` | GitHub Actions |
| `config` | 配置文件 |
| `docker` | Docker 相关 |
| `build` | 构建相关 |
| `release` | 发布相关 |

**注意：** 如果变更影响多个范围，可以省略 scope 或选择最主要的。

### Description（描述）- 必需

- 使用现在时态（"add" 而不是 "added"）
- 首字母小写
- 结尾不加句号
- 简洁明了地描述变更内容

---

## 🎯 完整示例

### ✅ 好的 PR 标题

```
feat(trader): add risk management system
fix(exchange): resolve connection timeout issue
docs: add API documentation for trading endpoints
style: apply consistent code formatting
refactor(ai): simplify prompt processing logic
perf(backend): optimize database queries
test(api): add integration tests for auth
chore(deps): update TypeScript to 5.0
ci(workflow): add automated security scanning
security(api): fix SQL injection vulnerability
build(docker): optimize Docker image size
```

### ⚠️ 需要改进的标题

| 不好的标题 | 问题 | 改进后 |
|-----------|------|--------|
| `update code` | 太模糊 | `refactor(trader): simplify order execution logic` |
| `Fixed bug` | 首字母大写，不够具体 | `fix(api): handle edge case in login` |
| `Add new feature.` | 有句号，不够具体 | `feat(ui): add dark mode toggle` |
| `changes` | 完全不符合格式 | `chore: update dependencies` |
| `feat: Added new trading algo` | 时态错误 | `feat(trader): add new trading algorithm` |

---

## 🤖 自动检查行为

### 当 PR 标题不符合格式时

1. **不会阻止合并** ✅
   - 检查会标记为"建议"
   - PR 仍然可以被审查和合并

2. **会收到友好提示** 💬
   - 机器人会在 PR 中留言
   - 提供格式说明和示例
   - 建议如何改进标题

3. **可以随时更新** 🔄
   - 更新 PR 标题后会重新检查
   - 无需关闭和重新打开 PR

### 示例评论

如果你的 PR 标题是 `update workflow`，你会收到这样的评论：

```markdown
## ⚠️ PR Title Format Suggestion

Your PR title doesn't follow the Conventional Commits format,
but this won't block your PR from being merged.

**Current title:** `update workflow`

**Recommended format:** `type(scope): description`

### Valid types:
feat, fix, docs, style, refactor, perf, test, chore, ci, security, build

### Common scopes (optional):
exchange, trader, ai, api, ui, frontend, backend, security, deps,
workflow, github, actions, config, docker, build, release

### Examples:
- feat(trader): add new trading strategy
- fix(api): resolve authentication issue
- docs: update README
- chore(deps): update dependencies
- ci(workflow): improve GitHub Actions

**Note:** This is a suggestion to improve consistency.
Your PR can still be reviewed and merged.
```

---

## 🔧 配置详情

### 支持的 Types

在 `.github/workflows/pr-checks.yml` 中配置：

```yaml
types: |
  feat
  fix
  docs
  style
  refactor
  perf
  test
  chore
  ci
  security
  build
```

### 支持的 Scopes

```yaml
scopes: |
  exchange
  trader
  ai
  api
  ui
  frontend
  backend
  security
  deps
  workflow
  github
  actions
  config
  docker
  build
  release
```

### 添加新的 Scope

如果你需要添加新的 scope，请：

1. 在 `.github/workflows/pr-checks.yml` 的 `scopes` 部分添加
2. 在 `.github/workflows/pr-checks-run.yml` 更新正则表达式（可选）
3. 更新本文档

---

## 📚 为什么使用 Conventional Commits？

### 优点

1. **自动化 Changelog** 📝
   - 可以自动生成版本更新日志
   - 清晰地分类各种变更

2. **语义化版本** 🔢
   - `feat` → MINOR 版本（1.1.0）
   - `fix` → PATCH 版本（1.0.1）
   - `BREAKING CHANGE` → MAJOR 版本（2.0.0）

3. **更好的可读性** 👀
   - 一眼看出 PR 的目的
   - 更容易浏览 Git 历史

4. **团队协作** 🤝
   - 统一的提交风格
   - 降低沟通成本

### 示例：自动生成的 Changelog

```markdown
## v1.2.0 (2025-11-02)

### Features
- **trader**: add risk management system (#123)
- **ui**: add dark mode toggle (#125)

### Bug Fixes
- **api**: resolve authentication issue (#124)
- **exchange**: fix connection timeout (#126)

### Documentation
- update API documentation (#127)
```

---

## 🎓 学习资源

- **Conventional Commits 官网:** https://www.conventionalcommits.org/
- **Angular Commit Guidelines:** https://github.com/angular/angular/blob/main/CONTRIBUTING.md#commit
- **Semantic Versioning:** https://semver.org/

---

## ❓ FAQ

### Q: 我必须遵循这个格式吗？

**A:** 不必须。这是建议性的，不会阻止你的 PR 被合并。但遵循格式可以提高项目的可维护性。

### Q: 如果我忘记了怎么办？

**A:** 机器人会在 PR 中提醒你，你可以随时更新标题。

### Q: 我可以在一个 PR 中做多种类型的变更吗？

**A:** 可以，但建议：
- 选择最主要的类型
- 或者考虑拆分成多个 PR（更易于审查）

### Q: Scope 可以省略吗？

**A:** 可以。`requireScope: false` 表示 scope 是可选的。

示例：`docs: update README` （没有 scope 也可以）

### Q: 我想添加新的 type 或 scope，怎么做？

**A:** 提一个 PR 修改 `.github/workflows/pr-checks.yml`，并在本文档中说明新增项的用途。

### Q: Breaking Changes 怎么表示？

**A:** 在描述中添加 `BREAKING CHANGE:` 或在 type 后加 `!`：

```
feat!: remove deprecated API
feat(api)!: change authentication method

BREAKING CHANGE: The old /auth endpoint is removed
```

---

## 📊 统计

想看项目的 commit 类型分布？运行：

```bash
git log --oneline --no-merges | \
  grep -oE '^[a-f0-9]+ (feat|fix|docs|style|refactor|perf|test|chore|ci|security|build)' | \
  cut -d' ' -f2 | sort | uniq -c | sort -rn
```

---

## ✅ 快速检查清单

在提交 PR 前，检查你的标题是否：

- [ ] 包含有效的 type（feat, fix, docs 等）
- [ ] 使用小写字母开头
- [ ] 使用现在时态（"add" 而不是 "added"）
- [ ] 简洁明了（最好在 50 字符内）
- [ ] 准确描述了变更内容

**记住：** 这些都是建议，不是强制要求！
