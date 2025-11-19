# 📢 PR Management System Update - What Contributors Need to Know

**Language:** [English](MIGRATION_ANNOUNCEMENT.md) | [中文](MIGRATION_ANNOUNCEMENT.zh-CN.md)

We're introducing a new PR management system to improve code quality and make contributing easier! This guide explains what's changing and what you need to do.

---

## 🎯 What's Changing?

We're introducing:

✅ **Clear contribution guidelines** aligned with our [roadmap](../roadmap/README.md)
✅ **Automated checks** (tests, linting, security scans)
✅ **Better labeling** for organization and prioritization
✅ **Faster review turnaround** with pre-checks
✅ **Transparent process** so you know exactly what to expect

---

## 📅 Timeline

```
Week 1-2: Existing PR Review Period
Week 3:   Soft Launch (checks are advisory only)
Week 4+:  Full Launch (checks are required)
```

**Important:** This rollout is gradual. You'll have time to adapt!

---

## 🤔 What This Means for YOU

### If You Have an Existing Open PR

**Good news:** Your PR will NOT be blocked by new rules!

- ✅ Your PR will be reviewed under current (relaxed) standards
- ✅ We'll review and provide feedback within 1-2 weeks
- ✅ Some PRs may need a quick rebase or minor updates

**What you might need to do:**
1. **Rebase on latest `dev` branch** if there are conflicts
2. **Respond to review comments** within 1 week
3. **Be patient** as we work through the backlog

**What happens if I don't respond?**
- We may close your PR after 2 weeks of inactivity
- You can always reopen it later with updates!
- No hard feelings - we're just cleaning up the backlog

### 🚀 Want to Check Your PR? (Optional)

We've created a **PR health check tool** to help you see if your PR meets the new standards!

**Run this in your local fork:**
```bash
./scripts/pr-check.sh
```

**What it does:**
- 🔍 Analyzes your PR (doesn't modify anything)
- ✅ Shows what's good
- ⚠️ Points out issues
- 💡 Gives you specific fix suggestions
- 📊 Overall health score

**Then fix issues and push:**
```bash
# Fix the issues (see suggestions from script)
# Run check again
./scripts/pr-check.sh

# Push when ready
git push -f origin <your-branch>
```

**📖 Full Guide:** [How to Migrate Your PR](HOW_TO_MIGRATE_YOUR_PR.md)

**Remember:** This is completely **optional** for existing PRs!

---

### If You're Submitting a NEW PR

**Timeline matters:**

#### Week 3 (Soft Launch):
- ✅ Automated checks will run (tests, linting, security)
- ⚠️ **Checks are advisory only** - they won't block your PR
- ✅ This is a learning period - we're here to help!
- ✅ Get familiar with the new [Contributing Guidelines](../../CONTRIBUTING.md)

#### Week 4+ (Full Launch):
- ✅ All automated checks must pass before merge
- ✅ PR must follow [Conventional Commits](https://www.conventionalcommits.org/) format
- ✅ PR template must be filled out
- ✅ Must align with [roadmap](../roadmap/README.md) priorities

---

## ✅ How to Prepare for New System

### 1. Read the Contributing Guidelines

📖 [CONTRIBUTING.md](../../CONTRIBUTING.md)

**Key points:**
- We accept PRs aligned with our roadmap (security, AI, exchanges, UI/UX)
- PRs should be focused and small (<300 lines preferred)
- Use Conventional Commits format: `feat(area): description`
- Include tests for new features

### 2. Check the Roadmap

🗺️ [Roadmap](../roadmap/README.md)

**Current priorities (Phase 1):**
- 🔒 Security enhancements
- 🧠 AI model integrations
- 🔗 Exchange integrations (OKX, Bybit, Lighter, EdgeX)
- 🎨 UI/UX improvements
- ⚡ Performance optimizations
- 🐛 Bug fixes

**Lower priority (Phase 2+):**
- Universal market expansion (stocks, futures)
- Advanced AI features
- Enterprise features

💡 **Pro tip:** If your PR aligns with Phase 1, it'll be reviewed faster!

### 3. Set Up Local Testing

Before submitting a PR, test locally:

```bash
# Backend tests
go test ./...
go fmt ./...
go vet ./...

# Frontend tests
cd web
npm run lint
npm run type-check
npm run build
```

This helps your PR pass automated checks on first try!

---

## 📝 PR Title Format

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

Examples:
feat(exchange): add OKX futures support
fix(trader): resolve position tracking bug
docs(readme): update installation instructions
perf(ai): optimize prompt generation
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

---

## 🎯 What Makes a Good PR?

### ✅ Good PR Example

```
Title: feat(exchange): add OKX exchange integration

Description:
Implements OKX exchange support with the following features:
- Order placement and cancellation
- Balance and position retrieval
- Leverage configuration
- Error handling and retry logic

Closes #123

Testing:
- [x] Unit tests added and passing
- [x] Manually tested with real API
- [x] Documentation updated
```

**Why it's good:**
- ✅ Clear, descriptive title
- ✅ Explains what and why
- ✅ Links to issue
- ✅ Includes testing details
- ✅ Small, focused change

### ❌ Avoid These

**Too vague:**
```
Title: update code
Description: made some changes
```

**Too large:**
```
Title: feat: complete rewrite of entire trading system
Files changed: 2,500+
```

**Off roadmap:**
```
Title: feat: add support for stock trading
(This is Phase 3, not current priority)
```

---

## 🐛 If Your PR Fails Checks

Don't panic! We're here to help.

**Week 3 (Soft Launch):**
- Checks are advisory - we'll help you fix issues
- Ask questions in your PR comments
- We can guide you through debugging

**Week 4+ (Full Launch):**
- Checks must pass, but we still help!
- Common issues:
  - Test failures → Run `go test ./...` locally
  - Linting errors → Run `go fmt` and `npm run lint`
  - Merge conflicts → Rebase on latest `dev`

**Need help?** Just ask! Comment in your PR or reach out:
- [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions)
- [Telegram Community](https://t.me/nofx_dev_community)

---

## 💰 Special Note for Bounty Contributors

If you're working on a bounty:

✅ **Your PRs get priority review** (24-48 hours)
✅ **Extra support** to meet requirements
✅ **Flexible during transition** - we'll work with you

Just make sure to:
- Reference the bounty issue number
- Meet all acceptance criteria
- Include demo video/screenshots

---

## ❓ FAQ

### Q: Will my existing PR be rejected?

**A:** No! Existing PRs use relaxed standards. We may ask for minor updates (rebase, small fixes), but you won't be held to new strict requirements.

### Q: What if I can't pass the new CI checks?

**A:** Week 3 is a learning period. We'll help you understand and fix issues. By Week 4, you'll be familiar with the process!

### Q: Will this slow down contributions?

**A:** Actually, no! Automated checks catch issues early, making reviews faster. Clear guidelines help you submit better PRs on first try.

### Q: Can I still contribute if I'm a beginner?

**A:** Absolutely! Look for issues labeled `good first issue`. We're here to mentor and help you succeed.

### Q: My PR is large (>1000 lines). What should I do?

**A:** Consider breaking it into smaller PRs. This gets you:
- ✅ Faster reviews
- ✅ Easier testing
- ✅ Higher chance of quick merge

Need help planning? Just ask in your PR!

### Q: What if my feature isn't on the roadmap?

**A:** Open an issue first to discuss! We're open to good ideas, but want to ensure alignment before you spend time coding.

### Q: When will this be fully active?

**A:** Week 4+ (approximately 4 weeks from announcement date). Check the pinned Discussion post for exact dates.

---

## 🎉 Benefits for Contributors

This new system helps YOU by:

✅ **Faster reviews** - Automated pre-checks reduce review time
✅ **Clear expectations** - You know exactly what's required
✅ **Better feedback** - Automated checks catch issues early
✅ **Fair prioritization** - Roadmap-aligned PRs reviewed faster
✅ **Recognition** - Contributor tiers and recognition program

---

## 📚 Resources

### Must Read
- [Contributing Guidelines](../../CONTRIBUTING.md) - Complete guide
- [Roadmap](../roadmap/README.md) - Current priorities

### Helpful Links
- [Conventional Commits](https://www.conventionalcommits.org/) - Commit format
- [Good First Issues](https://github.com/tinkle-community/nofx/labels/good%20first%20issue) - Beginner-friendly tasks
- [Bounty Program](../bounty-guide.md) - Get paid to contribute

### Get Help
- [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions) - Ask questions
- [Telegram](https://t.me/nofx_dev_community) - Community chat
- [Twitter](https://x.com/nofx_official) - Updates and announcements

---

## 💬 Feedback Welcome!

This is a new system and we want YOUR input:

- 📝 What's unclear?
- 🤔 What concerns do you have?
- 💡 How can we improve?

Share in the [Migration Feedback Discussion](https://github.com/tinkle-community/nofx/discussions) (link TBD)

---

## 🙏 Thank You!

We appreciate your contributions and patience during this transition. Together, we're building something amazing!

**Questions?** Don't hesitate to ask. We're here to help! 🚀

---

**Last Updated:** 2025-01-XX
**Status:** Announcement (Week 0)
**Full Launch:** Week 4+ (TBD)
