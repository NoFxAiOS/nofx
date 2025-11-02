# 🚀 PR Management System Setup Guide

**Language:** [English](SETUP_GUIDE.md) | [中文](SETUP_GUIDE.zh-CN.md)

This guide will help you set up and activate the complete PR management system for NOFX.

---

## 📦 What's Included

The PR management system includes:

### 1. **Documentation**
- ✅ `CONTRIBUTING.md` - Contributor guidelines
- ✅ `docs/maintainers/PR_REVIEW_GUIDE.md` - Reviewer guidelines
- ✅ `docs/maintainers/PROJECT_MANAGEMENT.md` - Project management workflow
- ✅ `docs/maintainers/SETUP_GUIDE.md` - This file

### 2. **GitHub Configuration**
- ✅ `.github/PULL_REQUEST_TEMPLATE.md` - PR template (already exists)
- ✅ `.github/labels.yml` - Label definitions
- ✅ `.github/labeler.yml` - Auto-labeling rules
- ✅ `.github/workflows/pr-checks.yml` - Automated PR checks

### 3. **Automation**
- ✅ Automatic PR labeling
- ✅ PR size checking
- ✅ CI/CD tests
- ✅ Security scanning
- ✅ Commit message validation

---

## 🔧 Setup Steps

### Step 1: Sync GitHub Labels

Create the labels defined in `.github/labels.yml`:

```bash
# Option 1: Using gh CLI (recommended)
gh label list  # See current labels
gh label delete <label-name>  # Remove old labels if needed
gh label create "priority: critical" --color "d73a4a" --description "Critical priority"
# ... repeat for all labels in labels.yml

# Option 2: Use GitHub Labeler Action (automated)
# The workflow will sync labels automatically on push
```

**Or use the GitHub Labeler Action** (add to `.github/workflows/sync-labels.yml`):

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

### Step 2: Enable GitHub Actions

1. Go to **Settings → Actions → General**
2. Enable **"Allow all actions and reusable workflows"**
3. Set **Workflow permissions** to **"Read and write permissions"**
4. Check **"Allow GitHub Actions to create and approve pull requests"**

### Step 3: Set Up Branch Protection Rules

**For `main` branch:**

1. Go to **Settings → Branches → Add rule**
2. Branch name pattern: `main`
3. Configure:
   - ✅ Require a pull request before merging
   - ✅ Require approvals: **1**
   - ✅ Require status checks to pass before merging
     - Select: `Backend Tests (Go)`
     - Select: `Frontend Tests (React/TypeScript)`
     - Select: `Security Scan`
   - ✅ Require conversation resolution before merging
   - ✅ Do not allow bypassing the above settings
   - ❌ Allow force pushes (disabled)
   - ❌ Allow deletions (disabled)

**For `dev` branch:**

1. Same as above, but with:
   - Require approvals: **1**
   - Less strict (allow maintainers to bypass if needed)

### Step 4: Create GitHub Projects

1. Go to **Projects → New project**
2. Create **"NOFX Development"** board
   - Template: Board
   - Add columns: `Backlog`, `Triaged`, `In Progress`, `In Review`, `Done`
   - Add views: Sprint, Roadmap, By Area, Priority

3. Create **"Bounty Program"** board
   - Template: Board
   - Add columns: `Available`, `Claimed`, `In Progress`, `Under Review`, `Paid`

### Step 5: Enable Discussions (Optional but Recommended)

1. Go to **Settings → General → Features**
2. Enable **"Discussions"**
3. Create categories:
   - 💬 **General** - General discussions
   - 💡 **Ideas** - Feature ideas and suggestions
   - 🙏 **Q&A** - Questions and answers
   - 📢 **Announcements** - Important updates
   - 🗳️ **Polls** - Community polls

### Step 6: Configure Issue Templates

The templates already exist in `.github/ISSUE_TEMPLATE/`. Verify they're working:

1. Go to **Issues → New issue**
2. You should see:
   - 🐛 Bug Report
   - ✨ Feature Request
   - 💰 Bounty Claim

If not showing, check files are properly formatted YAML with frontmatter.

### Step 7: Set Up Code Owners (Optional)

Create `.github/CODEOWNERS`:

```
# Global owners
* @tinkle @zack

# Frontend
/web/ @frontend-lead

# Exchange integrations
/internal/exchange/ @exchange-lead

# AI components
/internal/ai/ @ai-lead

# Documentation
/docs/ @tinkle @zack
*.md @tinkle @zack
```

### Step 8: Configure Notifications

**For Maintainers:**

1. Go to **Settings → Notifications**
2. Enable:
   - ✅ Pull request reviews
   - ✅ Pull request pushes
   - ✅ Comments on issues and PRs
   - ✅ New issues
   - ✅ Security alerts

3. Set up email filters to organize notifications

**For Repository:**

1. Go to **Settings → Webhooks** (if integrating with Slack/Discord)
2. Add webhook for notifications

---

## 📋 Post-Setup Checklist

After setup, verify:

- [ ] Labels are created and visible
- [ ] Branch protection rules are active
- [ ] GitHub Actions workflows run on new PR
- [ ] Auto-labeling works (create a test PR)
- [ ] PR template shows when creating PR
- [ ] Issue templates show when creating issue
- [ ] Projects boards are accessible
- [ ] CONTRIBUTING.md is linked in README

---

## 🎯 How to Use the System

### For Contributors

1. **Read** [CONTRIBUTING.md](../../../CONTRIBUTING.md)
2. **Check** [Roadmap](../../roadmap/README.md) for priorities
3. **Open issue** or find existing one
4. **Create PR** using the template
5. **Address review feedback**
6. **Celebrate** when merged! 🎉

### For Maintainers

1. **Daily:** Triage new issues/PRs (15 min)
2. **Daily:** Review assigned PRs
3. **Weekly:** Sprint planning (Monday) and review (Friday)
4. **Follow:** [PR Review Guide](PR_REVIEW_GUIDE.md)
5. **Follow:** [Project Management Guide](PROJECT_MANAGEMENT.md)

### For Bounty Hunters

1. **Check** bounty issues with `bounty` label
2. **Claim** by commenting on issue
3. **Complete** within deadline
4. **Submit PR** with bounty claim section filled
5. **Get paid** after merge

---

## 🔍 Testing the System

### Test 1: Create a Test PR

```bash
# Create a test branch
git checkout -b test/pr-system-check

# Make a small change
echo "# Test" >> TEST.md

# Commit and push
git add TEST.md
git commit -m "test: verify PR automation system"
git push origin test/pr-system-check

# Create PR on GitHub
# Verify:
# - PR template loads
# - Auto-labels are applied
# - CI checks run
# - Size label is added
```

### Test 2: Create a Test Issue

1. Go to **Issues → New issue**
2. Select **Bug Report**
3. Fill in template
4. Submit
5. Verify:
   - Template renders correctly
   - Issue can be labeled
   - Issue appears in project board

### Test 3: Test Auto-Labeling

Create PRs that change files in different areas:

```bash
# Test 1: Frontend changes
git checkout -b test/frontend-label
touch web/src/test.tsx
git add . && git commit -m "test: frontend labeling"
git push origin test/frontend-label
# Should get "area: frontend" label

# Test 2: Backend changes
git checkout -b test/backend-label
touch internal/test.go
git add . && git commit -m "test: backend labeling"
git push origin test/backend-label
# Should get "area: backend" label
```

---

## 🐛 Troubleshooting

### Issue: Labels not syncing

**Solution:**
```bash
# Delete all existing labels first
gh label list --json name --jq '.[].name' | xargs -I {} gh label delete "{}" --yes

# Then create from labels.yml manually or via action
```

### Issue: GitHub Actions not running

**Check:**
1. Actions are enabled in repository settings
2. Workflow files are in `.github/workflows/`
3. YAML syntax is valid
4. Permissions are set correctly

**Debug:**
```bash
# Validate workflow locally
act pull_request  # Using 'act' tool
```

### Issue: Branch protection blocking PRs

**Check:**
1. Required checks are defined in workflow
2. Check names match exactly
3. Checks are completing (not stuck)

**Temporary fix:**
- Maintainers can bypass if urgent
- Adjust protection rules if too strict

### Issue: Auto-labeler not working

**Check:**
1. `.github/labeler.yml` exists and valid YAML
2. Labels defined in labeler.yml exist in repository
3. Workflow has `pull-requests: write` permission

---

## 📊 Monitoring and Maintenance

### Weekly Review

Check these metrics every week:

```bash
# Using gh CLI
gh pr list --state all --json number,createdAt,closedAt
gh issue list --state all --json number,createdAt,closedAt

# Or use GitHub Insights
# Repository → Insights → Pulse, Contributors, Traffic
```

### Monthly Maintenance

- [ ] Review and update labels if needed
- [ ] Check for outdated dependencies in workflows
- [ ] Update CONTRIBUTING.md if processes change
- [ ] Review automation effectiveness
- [ ] Gather community feedback

---

## 🎓 Training Resources

### For New Contributors

- [First Contributions Guide](https://github.com/firstcontributions/first-contributions)
- [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/)
- [Conventional Commits](https://www.conventionalcommits.org/)

### For Maintainers

- [The Art of Code Review](https://google.github.io/eng-practices/review/)
- [GitHub Project Management](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [Maintainer Community](https://maintainers.github.com/)

---

## 🎉 You're All Set!

The PR management system is now ready to:

✅ Guide contributors with clear guidelines
✅ Automate repetitive tasks
✅ Maintain code quality
✅ Track progress systematically
✅ Scale the community

**Questions?** Reach out in the maintainer channel or open a discussion.

**Let's build an amazing community! 🚀**
