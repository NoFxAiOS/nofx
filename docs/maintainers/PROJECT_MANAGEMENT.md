# 📊 Project Management Guide

**Language:** [English](PROJECT_MANAGEMENT.md) | [中文](PROJECT_MANAGEMENT.zh-CN.md)

This guide explains how we manage the NOFX project, track progress, and prioritize work.

---

## 🎯 Project Structure

### GitHub Projects

We use **GitHub Projects (Beta)** with these boards:

#### 1. **NOFX Development Board**

**Columns:**
```
Backlog → Triaged → In Progress → In Review → Done
```

**Views:**
- 📋 **All Issues** - Kanban view of all work items
- 🏃 **Sprint** - Current sprint items (2-week sprints)
- 🗺️ **Roadmap** - Timeline view by roadmap phase
- 🏷️ **By Area** - Grouped by area labels
- 🔥 **Priority** - Sorted by priority (critical/high/medium/low)
- 👥 **By Assignee** - Grouped by assigned maintainer

#### 2. **Bounty Program Board**

**Columns:**
```
Available → Claimed → In Progress → Under Review → Paid
```

---

## 📅 Sprint Planning (Bi-weekly)

### Sprint Schedule

**Sprint Duration:** 2 weeks
**Sprint Planning:** Every other Monday
**Sprint Review:** Every other Friday

### Planning Process

**Monday - Sprint Planning (1 hour):**

1. **Review previous sprint** (15 min)
   - What was completed?
   - What was not completed and why?
   - Metrics review

2. **Prioritize backlog** (20 min)
   - Review new issues/PRs
   - Update priorities based on roadmap
   - Assign labels

3. **Plan next sprint** (25 min)
   - Select items for next sprint
   - Assign to maintainers
   - Set clear acceptance criteria
   - Estimate effort (S/M/L)

**Friday - Sprint Review (30 min):**

1. **Demo completed work** (15 min)
   - Show merged PRs
   - Demonstrate new features

2. **Retrospective** (15 min)
   - What went well?
   - What can improve?
   - Action items for next sprint

---

## 🏷️ Issue Triage Process

### Daily Triage (Mon-Fri, 15 min)

Review new issues and PRs:

1. **Verify completeness**
   - Template filled properly?
   - Reproduction steps clear (for bugs)?
   - Use case explained (for features)?

2. **Apply labels**
   ```yaml
   Priority:
     - priority: critical  # Security, data loss, production down
     - priority: high      # Major bugs, high-value features
     - priority: medium    # Regular bugs, standard features
     - priority: low       # Nice-to-have, minor improvements

   Type:
     - type: bug
     - type: feature
     - type: enhancement
     - type: documentation
     - type: security

   Area:
     - area: exchange
     - area: ai
     - area: frontend
     - area: backend
     - area: security
     - area: ui/ux

   Roadmap:
     - roadmap: phase-1  # Core Infrastructure
     - roadmap: phase-2  # Testing & Stability
     - roadmap: phase-3  # Universal Markets
     ```

3. **Assign or tag for discussion**
   - Can handle immediately? Assign to maintainer
   - Needs discussion? Tag for next planning session
   - Needs more info? Request from author

4. **Close if needed**
   - Duplicate? Close with link to original
   - Invalid? Close with explanation
   - Out of scope? Close politely with reasoning

---

## 🎯 Priority Decision Matrix

Use this matrix to decide priority:

| Impact / Urgency | High Urgency | Medium Urgency | Low Urgency |
|------------------|--------------|----------------|-------------|
| **High Impact** | 🔴 Critical | 🔴 Critical | 🟡 High |
| **Medium Impact** | 🔴 Critical | 🟡 High | 🟢 Medium |
| **Low Impact** | 🟡 High | 🟢 Medium | ⚪ Low |

**Impact:**
- High: Affects core functionality, security, or many users
- Medium: Affects specific features or moderate users
- Low: Nice-to-have, minor improvements

**Urgency:**
- High: Needs immediate attention
- Medium: Should be addressed soon
- Low: Can wait for natural inclusion

---

## 📊 Roadmap Alignment

All work should align with our [roadmap](../roadmap/README.md):

### Phase 1: Core Infrastructure (Current Focus)

**Must Accept:**
- Security enhancements
- AI model integrations
- Exchange integrations (OKX, Bybit, Lighter, EdgeX)
- Project structure refactoring
- UI/UX improvements

**Can Accept:**
- Related bug fixes
- Documentation improvements
- Performance optimizations

**Should Defer:**
- Universal market expansion (stocks, futures)
- Advanced AI features (RL, multi-agent)
- Enterprise features

### Phase 2-5: Future Work

Mark with appropriate `roadmap: phase-X` label and add to backlog.

---

## 🎫 Issue Templates

We have these issue templates:

### 1. Bug Report
- Use for bugs and errors
- Must include reproduction steps
- Label: `type: bug`

### 2. Feature Request
- Use for new features
- Must include use case and benefits
- Label: `type: feature`

### 3. Bounty Claim
- Use when claiming a bounty
- Must reference bounty issue
- Label: `bounty: claimed`

### 4. Security Vulnerability
- Use for security issues (private)
- Follow responsible disclosure
- Label: `type: security`

**Missing a template?**
- Use blank issue
- Maintainers will convert to appropriate template

---

## 📈 Metrics We Track

### Weekly Metrics

- **PR Metrics:**
  - Number of PRs opened
  - Number of PRs merged
  - Average time to first review
  - Average time to merge

- **Issue Metrics:**
  - Number of issues opened
  - Number of issues closed
  - Issue backlog size
  - Issues by priority/type/area

- **Community Metrics:**
  - New contributors
  - Active contributors
  - Community engagement (comments, reactions)

### Monthly Metrics

- **Roadmap Progress:**
  - % completion per phase
  - Items completed vs planned
  - Blockers and risks

- **Code Quality:**
  - Test coverage
  - Code review comments per PR
  - Bug fix vs feature ratio

- **Bounty Program:**
  - Bounties created
  - Bounties claimed
  - Bounties paid
  - Average completion time

---

## 🤖 Automation

We use GitHub Actions for automation:

### PR Automation

- **Automatic labeling** based on files changed
- **PR size labeling** (small/medium/large)
- **CI checks** (tests, linting, build)
- **Security scans** (Trivy, Gitleaks)
- **Conventional commit validation**

### Issue Automation

- **Stale issue detection** (closes after 30 days inactive)
- **Automatic bounty labeling** when "bounty" keyword used
- **Duplicate detection** using issue similarity

### Release Automation

- **Changelog generation** from conventional commits
- **Version bumping** based on commit types
- **Release notes** auto-generated
- **Deployment** to staging/production

---

## 🔄 Regular Tasks

### Daily
- ✅ Triage new issues/PRs
- ✅ Review urgent PRs
- ✅ Respond to community questions

### Weekly
- ✅ Sprint planning (Monday)
- ✅ Sprint review (Friday)
- ✅ Review metrics dashboard
- ✅ Update project boards

### Monthly
- ✅ Roadmap progress review
- ✅ Community update post
- ✅ Bounty program review
- ✅ Dependency updates
- ✅ Security audit

### Quarterly
- ✅ Roadmap update
- ✅ Major release planning
- ✅ Contributor recognition
- ✅ Documentation audit

---

## 📞 Communication Channels

### Internal (Maintainers)

- **GitHub Discussions:** Architecture decisions, RFC
- **Private channel:** Sensitive discussions, bounty payments
- **Weekly sync:** Sprint planning and review

### External (Community)

- **Telegram:** [@nofx_dev_community](https://t.me/nofx_dev_community)
- **GitHub Issues:** Bug reports, feature requests
- **GitHub Discussions:** General questions, ideas
- **Twitter:** [@nofx_official](https://x.com/nofx_official) - Announcements

---

## 🎓 Onboarding New Maintainers

### Checklist for New Maintainers

- [ ] Add to GitHub organization
- [ ] Grant write access to repository
- [ ] Add to private maintainer channel
- [ ] Introduce to the team
- [ ] Read all docs in `/docs/maintainers/`
- [ ] Shadow experienced maintainer for 1 sprint
- [ ] First solo PR review (with backup reviewer)
- [ ] First solo issue triage
- [ ] First sprint planning participation

### Expectations

**Time Commitment:**
- ~5-10 hours per week
- Participate in sprint planning/review
- Respond to assigned issues/PRs within SLA

**Responsibilities:**
- Code review
- Issue triage
- Community support
- Documentation maintenance

---

## 🏆 Contributor Recognition

### Monthly Recognition

**Spotlight in Community Update:**
- Top contributor
- Best PR of the month
- Most helpful community member

### Quarterly Recognition

**Contributor Tier System:**
- 🥇 **Core Contributor** - 20+ merged PRs
- 🥈 **Active Contributor** - 10+ merged PRs
- 🥉 **Contributor** - 5+ merged PRs
- ⭐ **First Timer** - 1+ merged PR

**Benefits:**
- Recognition in README
- Invitation to private Discord
- Early access to features
- Swag (for Core Contributors)

---

## 📚 Resources

### Internal Docs
- [PR Review Guide](PR_REVIEW_GUIDE.md)
- [Security Policy](../../SECURITY.md)
- [Code of Conduct](../../CODE_OF_CONDUCT.md)

### External Resources
- [GitHub Project Management](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)

---

## 🤔 Questions?

Reach out in the maintainer channel or open a discussion.

**Let's build something amazing together! 🚀**
