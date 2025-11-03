# GitHub Actions Workflows

This directory contains the GitHub Actions workflows for the NOFX project.

## 📚 Documentation Index

- **[README.md](./README.md)** - This file, overview of all workflows
- **[PERMISSIONS.md](./PERMISSIONS.md)** - Detailed permission analysis and security model
- **[TRIGGERS.md](./TRIGGERS.md)** - Comparison of event triggers (pull_request vs pull_request_target vs workflow_run)
- **[FORK_PR_FLOW.md](./FORK_PR_FLOW.md)** - Complete analysis of what happens when a fork PR is submitted
- **[FLOW_DIAGRAM.md](./FLOW_DIAGRAM.md)** - Visual flow diagrams and quick reference
- **[SECRETS_SCANNING.md](./SECRETS_SCANNING.md)** - Secrets scanning solutions and TruffleHog setup

## 🚀 Quick Start

**Want to understand how fork PRs work?** → Read [FLOW_DIAGRAM.md](./FLOW_DIAGRAM.md)

**Need security details?** → Read [PERMISSIONS.md](./PERMISSIONS.md)

**Confused about triggers?** → Read [TRIGGERS.md](./TRIGGERS.md)

## PR Check Workflows

We use a **two-workflow pattern** to safely handle PR checks from both internal and fork PRs:

### 1. `pr-checks-run.yml` - Execute Checks

**Trigger:** On pull request (opened, synchronize, reopened)

**Permissions:** Read-only

**Purpose:** Executes all PR checks with read-only permissions, making it safe for fork PRs.

**What it does:**
- ✅ Checks PR title format (Conventional Commits)
- ✅ Calculates PR size
- ✅ Runs backend checks (Go formatting, vet, tests)
- ✅ Runs frontend checks (linting, type checking, build)
- ✅ Saves all results as artifacts

**Security:** Safe for fork PRs because it only has read permissions and cannot access secrets or modify the repository.

### 2. `pr-checks-comment.yml` - Post Results

**Trigger:** When `pr-checks-run.yml` completes (workflow_run)

**Permissions:** Write (pull-requests, issues)

**Purpose:** Posts check results as PR comments, running in the main repository context.

**What it does:**
- ✅ Downloads artifacts from `pr-checks-run.yml`
- ✅ Reads check results
- ✅ Posts a comprehensive comment to the PR

**Security:** Safe because:
- Runs in the main repository context (not fork context)
- Has write permissions but doesn't execute untrusted code
- Only reads pre-generated results from artifacts

### 3. `pr-checks.yml` - Strict Checks

**Trigger:** On pull request

**Permissions:** Read + conditional write

**Purpose:** Runs mandatory checks that must pass before PR can be merged.

**What it does:**
- ✅ Validates PR title (blocks merge if invalid)
- ✅ Auto-labels PR based on size and files changed (non-fork only)
- ✅ Runs backend tests (Go)
- ✅ Runs frontend tests (React/TypeScript)
- ✅ Security scanning (Trivy, Gitleaks)

**Security:**
- Fork PRs: Only runs read-only operations (tests, security scans)
- Non-fork PRs: Can add labels and comments
- Uses `continue-on-error` for operations that may fail on forks

## Why Two Workflows for PR Checks?

### The Problem

When a PR comes from a forked repository:
- GitHub restricts `GITHUB_TOKEN` permissions for security
- Fork PRs cannot write comments, add labels, or access secrets
- This prevents malicious contributors from:
  - Stealing repository secrets
  - Modifying workflow files to execute malicious code
  - Spamming issues/PRs with automated comments

### The Solution

**Two-Workflow Pattern:**

```
Fork PR Submitted
       ↓
[pr-checks-run.yml]
  - Runs with read-only permissions
  - Executes all checks safely
  - Saves results to artifacts
       ↓
[pr-checks-comment.yml]
  - Triggered by workflow_run
  - Runs in main repo context (has write permissions)
  - Downloads artifacts
  - Posts comment with results
```

This approach:
- ✅ Allows fork PRs to run checks
- ✅ Safely posts results as comments
- ✅ Prevents security vulnerabilities
- ✅ Follows GitHub's best practices

### Can workflow_run Comment on Fork PRs?

**Yes! ✅ The permissions are sufficient.**

**Key Understanding:**
- `workflow_run` executes in the **base repository** context
- Fork PRs exist in the **base repository** (not in the fork)
- The base repository's `GITHUB_TOKEN` has write permissions
- Therefore, `workflow_run` can comment on fork PRs

**Security:**
- Fork PR code runs in isolated environment (read-only)
- Comment workflow doesn't execute fork code
- Only reads pre-generated artifact data

**For detailed permission analysis, see:** [PERMISSIONS.md](./PERMISSIONS.md)

## Workflow Comparison

| Workflow | Fork PRs | Write Access | Blocks Merge | Purpose |
|----------|----------|--------------|--------------|---------|
| `pr-checks-run.yml` | ✅ Yes | ❌ No | ❌ No | Advisory checks |
| `pr-checks-comment.yml` | ✅ Yes | ✅ Yes* | ❌ No | Post results |
| `pr-checks.yml` | ✅ Yes | ⚠️ Partial | ✅ Yes | Mandatory checks |

\* Write access only in main repo context, not available to fork PR code

## File History

- `pr-checks-advisory.yml.old` - Old advisory workflow that failed on fork PRs (deprecated)
- Now replaced by the two-workflow pattern (`pr-checks-run.yml` + `pr-checks-comment.yml`)

## Testing the Workflows

### Test with a Fork PR

1. Fork the repository
2. Make changes in your fork
3. Create a PR to the main repository
4. Observe:
   - `pr-checks-run.yml` runs successfully with read-only access
   - `pr-checks-comment.yml` posts results as a comment
   - `pr-checks.yml` runs tests but skips labeling

### Test with a Branch PR

1. Create a branch in the main repository
2. Make changes
3. Create a PR
4. Observe:
   - All workflows run with full permissions
   - Labels are added automatically
   - Comments are posted

## References

- [GitHub Actions: Keeping your GitHub Actions and workflows secure Part 1](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/)
- [Safely posting comments from untrusted workflows](https://securitylab.github.com/research/github-actions-building-blocks/)
- [GitHub Actions: workflow_run trigger](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#workflow_run)
