# 🔄 How to Migrate Your PR to the New Format

**Language:** [English](HOW_TO_MIGRATE_YOUR_PR.md) | [中文](HOW_TO_MIGRATE_YOUR_PR.zh-CN.md)

This guide helps you migrate your existing PR to meet the new PR management system requirements.

---

## 🎯 Why Migrate?

While your existing PR **will still be reviewed and merged** under current standards, migrating it to the new format gives you:

✅ **Faster reviews** - Automated checks catch issues early
✅ **Better feedback** - Clear, actionable feedback from CI
✅ **Higher quality** - Consistent code standards
✅ **Learning** - Understand our new contribution workflow

---

## ⚡ Quick Check (Recommended)

### Step 1: Analyze Your PR

```bash
# Run the PR health check (reads only, doesn't modify anything)
./scripts/pr-check.sh
```

This will analyze your PR and tell you:
- ✅ What's good
- ⚠️ What needs attention
- 💡 How to fix issues
- 📊 Overall health score

### Step 2: Fix Issues

Based on the suggestions, fix the issues manually. Common fixes:

```bash
# Rebase on latest dev
git fetch upstream && git rebase upstream/dev

# Format Go code
go fmt ./...

# Run tests
go test ./...

# Format frontend code
cd web && npm run lint -- --fix
```

### Step 3: Run Check Again

```bash
# Verify all issues are fixed
./scripts/pr-check.sh
```

### Step 4: Push Changes

```bash
git push -f origin <your-pr-branch>
```

### What the Script Does

1. ✅ Syncs with latest `upstream/dev`
2. ✅ Rebases your changes
3. ✅ Formats Go code (`go fmt`)
4. ✅ Runs Go linting (`go vet`)
5. ✅ Runs tests
6. ✅ Formats frontend code (if applicable)
7. ✅ Pushes changes to your PR

---

## 🛠️ Manual Migration (Step by Step)

If you prefer to do it manually:

### Step 1: Sync with Upstream

```bash
# Add upstream if not already added
git remote add upstream https://github.com/NoFxAiOS/nofx.git

# Fetch latest changes
git fetch upstream

# Rebase your branch
git checkout <your-pr-branch>
git rebase upstream/dev
```

### Step 2: Backend Checks (Go)

```bash
# Format Go code
go fmt ./...

# Run linting
go vet ./...

# Run tests
go test ./...

# If you made changes, commit them
git add .
git commit -m "chore: format and fix backend issues"
```

### Step 3: Frontend Checks (if applicable)

```bash
cd web

# Install dependencies
npm install

# Fix linting issues
npm run lint -- --fix

# Check types
npm run type-check

# Test build
npm run build

cd ..

# Commit any fixes
git add .
git commit -m "chore: fix frontend issues"
```

### Step 4: Update PR Title (if needed)

Ensure your PR title follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

Examples:
feat(exchange): add OKX integration
fix(trader): resolve position tracking bug
docs(readme): update installation guide
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation
- `refactor` - Code refactoring
- `perf` - Performance improvement
- `test` - Test updates
- `chore` - Build/config changes
- `security` - Security improvements

### Step 5: Push Changes

```bash
git push -f origin <your-pr-branch>
```

---

## 📋 Checklist

After migrating, verify:

- [ ] PR is rebased on latest `dev`
- [ ] No merge conflicts
- [ ] Backend tests pass locally
- [ ] Frontend builds successfully
- [ ] PR title follows Conventional Commits format
- [ ] All commits are meaningful
- [ ] Changes pushed to GitHub

---

## 🤖 What Happens After Migration?

After you push your changes:

1. **Automated checks will run** (they won't block merging, just provide feedback)
2. **You'll get a comment** with check results and suggestions
3. **Maintainers will review** your PR with the new context
4. **Faster review** thanks to pre-checks

---

## ❓ Troubleshooting

### "Rebase conflicts"

If you get conflicts during rebase:

```bash
# Fix conflicts in your editor
# Then:
git add <fixed-files>
git rebase --continue

# Or abort and ask for help:
git rebase --abort
```

**Need help?** Just comment on your PR and we'll assist!

### "Tests failing"

If tests fail:

```bash
# Run tests to see the error
go test ./...

# Fix the issue
# Then commit and push
git add .
git commit -m "fix: resolve test failures"
git push -f origin <your-pr-branch>
```

### "Script not working"

If the migration script doesn't work:

1. Check you have Go and Node.js installed
2. Try manual migration (steps above)
3. Ask for help in your PR comments

---

## 💡 Tips

**Don't want to migrate?**
- That's okay! Your PR will still be reviewed and merged
- Migration is optional but recommended

**First time using Git rebase?**
- Check our [Git guide](https://git-scm.com/book/en/v2/Git-Branching-Rebasing)
- Ask questions in your PR - we're here to help!

**Want to learn more?**
- [Contributing Guidelines](../../CONTRIBUTING.md)
- [Migration Announcement](MIGRATION_ANNOUNCEMENT.md)
- [PR Review Guide](../maintainers/PR_REVIEW_GUIDE.md)

---

## 📞 Need Help?

**Stuck on migration?**
- Comment on your PR

- Open a [Discussion](https://github.com/NoFxAiOS/nofx/discussions)

**We're here to help you succeed!** 🚀

---

## 🎉 After Migration

Once migrated:
1. ✅ Wait for automated checks to run
2. ✅ Address any feedback in comments
3. ✅ Wait for maintainer review
4. ✅ Celebrate when merged! 🎉

**Thank you for contributing to NOFX!**
