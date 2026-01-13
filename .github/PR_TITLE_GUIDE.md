# PR Title Guide

## 📋 Overview

We use the **Conventional Commits** format to maintain consistency in PR titles, but this is **recommended**, not mandatory. It will not prevent your PR from being merged.

## ✅ Recommended Format

```
type(scope): description
```

### Examples

```
feat(trader): add new trading strategy
fix(api): resolve authentication issue
docs: update README
chore(deps): update dependencies
ci(workflow): improve GitHub Actions
```

---

## 📖 Detailed Guide

### Type - Required

Describes the type of change:

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat(trader): add stop-loss feature` |
| `fix` | Bug fix | `fix(api): handle null response` |
| `docs` | Documentation change | `docs: update installation guide` |
| `style` | Code formatting (no functional change) | `style: format code with prettier` |
| `refactor` | Code refactoring (neither feature nor fix) | `refactor(exchange): simplify connection logic` |
| `perf` | Performance optimization | `perf(ai): optimize prompt processing` |
| `test` | Add or modify tests | `test(trader): add unit tests` |
| `chore` | Build process or auxiliary tool changes | `chore: update dependencies` |
| `ci` | CI/CD related changes | `ci: add test coverage report` |
| `security` | Security fixes | `security: update vulnerable dependencies` |
| `build` | Build system or external dependency changes | `build: upgrade webpack to v5` |

### Scope - Optional

Describes the area affected by the change:

| Scope | Description |
|-------|-------------|
| `exchange` | Exchange-related |
| `trader` | Trader/trading strategy |
| `ai` | AI model related |
| `api` | API interface |
| `ui` | User interface |
| `frontend` | Frontend code |
| `backend` | Backend code |
| `security` | Security related |
| `deps` | Dependencies |
| `workflow` | GitHub Actions workflows |
| `github` | GitHub configuration |
| `actions` | GitHub Actions |
| `config` | Configuration files |
| `docker` | Docker related |
| `build` | Build related |
| `release` | Release related |

**Note:** If the change affects multiple scopes, you can omit the scope or choose the most relevant one.

### Description - Required

- Use present tense ("add" not "added")
- Start with lowercase
- No period at the end
- Concisely describe what changed

---

## 🎯 Complete Examples

### ✅ Good PR Titles

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

### ⚠️ Titles That Need Improvement

| Poor Title | Issue | Improved |
|-----------|-------|----------|
| `update code` | Too vague | `refactor(trader): simplify order execution logic` |
| `Fixed bug` | Capitalized, not specific | `fix(api): handle edge case in login` |
| `Add new feature.` | Has period, not specific | `feat(ui): add dark mode toggle` |
| `changes` | Doesn't follow format | `chore: update dependencies` |
| `feat: Added new trading algo` | Wrong tense | `feat(trader): add new trading algorithm` |

---

## 🤖 Automated Check Behavior

### When PR Title Doesn't Follow Format

1. **Won't block merging** ✅
   - Check is marked as "advisory"
   - PR can still be reviewed and merged

2. **Provides friendly reminder** 💬
   - Bot will comment on the PR
   - Provides format guidance and examples
   - Suggests how to improve the title

3. **Can be updated anytime** 🔄
   - Re-checks after updating PR title
   - No need to close and reopen PR

### Example Comment

If your PR title is `update workflow`, you'll receive a comment like this:

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

## 🔧 Configuration Details

### Supported Types

Configured in `.github/workflows/pr-checks.yml`:

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

### Supported Scopes

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

### Adding New Scopes

If you need to add a new scope:

1. Add it to the `scopes` section in `.github/workflows/pr-checks.yml`
2. Update the regex in `.github/workflows/pr-checks-run.yml` (optional)
3. Update this documentation

---

## 📚 Why Use Conventional Commits?

### Benefits

1. **Automated Changelog** 📝
   - Automatically generate version changelogs
   - Clearly categorize different types of changes

2. **Semantic Versioning** 🔢
   - `feat` → MINOR version (1.1.0)
   - `fix` → PATCH version (1.0.1)
   - `BREAKING CHANGE` → MAJOR version (2.0.0)

3. **Better Readability** 👀
   - Understand PR purpose at a glance
   - Easier to browse Git history

4. **Team Collaboration** 🤝
   - Unified commit style
   - Reduces communication overhead

### Example: Auto-generated Changelog

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

## 🎓 Learning Resources

- **Conventional Commits:** https://www.conventionalcommits.org/
- **Angular Commit Guidelines:** https://github.com/angular/angular/blob/main/CONTRIBUTING.md#commit
- **Semantic Versioning:** https://semver.org/

---

## ❓ FAQ

### Q: Must I follow this format?

**A:** No. This is recommended but not mandatory. It won't block your PR from being merged. However, following the format improves project maintainability.

### Q: What if I forget?

**A:** The bot will remind you in the PR comments. You can update the title anytime.

### Q: Can I make multiple types of changes in one PR?

**A:** Yes, but it's recommended to:
- Choose the most significant type
- Or consider splitting into multiple PRs (easier to review)

### Q: Can I omit the scope?

**A:** Yes. `requireScope: false` means scope is optional.

Example: `docs: update README` (no scope is fine)

### Q: How do I add a new type or scope?

**A:** Submit a PR to modify `.github/workflows/pr-checks.yml` and document the purpose of the new item in this guide.

### Q: How do I indicate Breaking Changes?

**A:** Add `BREAKING CHANGE:` in the description or add `!` after the type:

```
feat!: remove deprecated API
feat(api)!: change authentication method

BREAKING CHANGE: The old /auth endpoint is removed
```

---

## 📊 Statistics

Want to see the commit type distribution in your project? Run:

```bash
git log --oneline --no-merges | \
  grep -oE '^[a-f0-9]+ (feat|fix|docs|style|refactor|perf|test|chore|ci|security|build)' | \
  cut -d' ' -f2 | sort | uniq -c | sort -rn
```

---

## ✅ Quick Checklist

Before submitting a PR, check if your title:

- [ ] Contains a valid type (feat, fix, docs, etc.)
- [ ] Starts with lowercase
- [ ] Uses present tense ("add" not "added")
- [ ] Is concise (preferably under 50 characters)
- [ ] Accurately describes the change

**Remember:** These are recommendations, not requirements!
