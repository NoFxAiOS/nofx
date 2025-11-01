#!/bin/bash

# 🔄 PR Migration Script for Contributors
# This script helps you migrate your PR to the new format
# Run this in your local fork to update your PR automatically

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

confirm() {
    read -p "$(echo -e ${YELLOW}"$1 (y/N): "${NC})" -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]]
}

# Welcome message
echo ""
echo "╔═══════════════════════════════════════════╗"
echo "║  NOFX PR Migration Tool                   ║"
echo "║  Migrate your PR to the new format        ║"
echo "╚═══════════════════════════════════════════╝"
echo ""

# Check if we're in a git repo
if ! git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
    log_error "Not a git repository. Please run this from your NOFX fork."
    exit 1
fi

# Check current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
log_info "Current branch: $CURRENT_BRANCH"

if [ "$CURRENT_BRANCH" = "main" ] || [ "$CURRENT_BRANCH" = "dev" ]; then
    log_warning "You're on the $CURRENT_BRANCH branch."
    log_info "This script should be run on your PR branch."

    # List branches
    log_info "Your branches:"
    git branch

    echo ""
    read -p "Enter your PR branch name: " PR_BRANCH

    if [ -z "$PR_BRANCH" ]; then
        log_error "No branch specified. Exiting."
        exit 1
    fi

    git checkout "$PR_BRANCH" || {
        log_error "Failed to checkout branch $PR_BRANCH"
        exit 1
    }

    CURRENT_BRANCH="$PR_BRANCH"
fi

log_success "Working on branch: $CURRENT_BRANCH"

echo ""
log_info "What this script will do:"
echo "  1. ✅ Verify you're rebased on latest upstream/dev"
echo "  2. ✅ Check and format Go code (go fmt)"
echo "  3. ✅ Run Go linting (go vet)"
echo "  4. ✅ Run Go tests"
echo "  5. ✅ Check frontend code (if modified)"
echo "  6. ✅ Give you feedback and suggestions"
echo ""
log_warning "Make sure you've already run: git fetch upstream && git rebase upstream/dev"
echo ""

if ! confirm "Continue with migration?"; then
    log_info "Migration cancelled"
    exit 0
fi

# Step 1: Verify upstream sync
echo ""
log_info "Step 1: Verifying upstream sync..."

# Check if upstream remote exists
if ! git remote | grep -q "^upstream$"; then
    log_warning "Upstream remote not found. Adding it..."
    git remote add upstream https://github.com/tinkle-community/nofx.git
    git fetch upstream
    log_success "Added upstream remote"
fi

# Check if we're up to date with upstream/dev
if git merge-base --is-ancestor upstream/dev HEAD; then
    log_success "Your branch is up to date with upstream/dev"
else
    log_warning "Your branch is not based on latest upstream/dev"
    log_info "Please run first: git fetch upstream && git rebase upstream/dev"

    if confirm "Try to rebase now?"; then
        git fetch upstream
        if git rebase upstream/dev; then
            log_success "Successfully rebased on upstream/dev"
        else
            log_error "Rebase failed. Please resolve conflicts manually."
            exit 1
        fi
    else
        log_warning "Skipping rebase. Results may not be accurate."
    fi
fi

# Step 2: Backend checks (if Go files exist)
if find . -name "*.go" -not -path "./vendor/*" | grep -q .; then
    echo ""
    log_info "Step 2: Running backend checks..."

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_warning "Go not found. Skipping backend checks."
        log_info "Install Go: https://go.dev/doc/install"
    else
        # Format Go code
        log_info "Formatting Go code..."
        if go fmt ./...; then
            log_success "Go code formatted"

            # Check if there are changes
            if ! git diff --quiet; then
                log_info "Formatting created changes. Committing..."
                git add .
                git commit -m "chore: format Go code with go fmt" || true
            fi
        else
            log_warning "Go formatting had issues (non-critical)"
        fi

        # Run go vet
        log_info "Running go vet..."
        if go vet ./...; then
            log_success "Go vet passed"
        else
            log_warning "Go vet found issues. Please review them."
            if confirm "Continue anyway?"; then
                log_info "Continuing..."
            else
                exit 1
            fi
        fi

        # Run tests
        log_info "Running Go tests..."
        if go test ./...; then
            log_success "All Go tests passed"
        else
            log_warning "Some tests failed. Please fix them before pushing."
            if confirm "Continue anyway?"; then
                log_info "Continuing..."
            else
                exit 1
            fi
        fi
    fi
else
    log_info "Step 2: No Go files found, skipping backend checks"
fi

# Step 3: Frontend checks (if web directory exists)
if [ -d "web" ]; then
    echo ""
    log_info "Step 3: Running frontend checks..."

    # Check if npm is installed
    if ! command -v npm &> /dev/null; then
        log_warning "npm not found. Skipping frontend checks."
        log_info "Install Node.js: https://nodejs.org/"
    else
        cd web

        # Install dependencies if needed
        if [ ! -d "node_modules" ]; then
            log_info "Installing dependencies..."
            npm install
        fi

        # Run linter
        log_info "Running linter..."
        if npm run lint; then
            log_success "Linting passed"
        else
            log_warning "Linting found issues"
            log_info "Attempting to auto-fix..."
            npm run lint -- --fix || true

            # Commit fixes if any
            if ! git diff --quiet; then
                git add .
                git commit -m "chore: fix linting issues" || true
            fi
        fi

        # Type check
        log_info "Running type check..."
        if npm run type-check; then
            log_success "Type checking passed"
        else
            log_warning "Type checking found issues. Please fix them."
        fi

        # Build
        log_info "Testing build..."
        if npm run build; then
            log_success "Build successful"
        else
            log_error "Build failed. Please fix build errors."
            cd ..
            exit 1
        fi

        cd ..
    fi
else
    log_info "Step 3: No frontend changes, skipping frontend checks"
fi

# Step 4: Check PR title format
echo ""
log_info "Step 4: Checking PR title format..."

# Get the commit messages to suggest a title
COMMITS=$(git log upstream/dev..HEAD --oneline)
COMMIT_COUNT=$(echo "$COMMITS" | wc -l | tr -d ' ')

log_info "Found $COMMIT_COUNT commit(s) in your PR"

if [ "$COMMIT_COUNT" -eq 1 ]; then
    SUGGESTED_TITLE=$(git log -1 --pretty=%s)
else
    SUGGESTED_TITLE=$(git log --pretty=%s upstream/dev..HEAD | head -1)
fi

log_info "Current/suggested title: $SUGGESTED_TITLE"

# Check if it follows conventional commits
if echo "$SUGGESTED_TITLE" | grep -qE "^(feat|fix|docs|style|refactor|perf|test|chore|ci|security)(\(.+\))?: .+"; then
    log_success "Title follows Conventional Commits format"
else
    log_warning "Title doesn't follow Conventional Commits format"
    echo ""
    echo "Conventional Commits format:"
    echo "  <type>(<scope>): <description>"
    echo ""
    echo "Types: feat, fix, docs, style, refactor, perf, test, chore, ci, security"
    echo ""
    echo "Examples:"
    echo "  feat(exchange): add OKX integration"
    echo "  fix(trader): resolve position tracking bug"
    echo "  docs(readme): update installation guide"
    echo ""

    read -p "Enter new title (or press Enter to keep current): " NEW_TITLE

    if [ -n "$NEW_TITLE" ]; then
        log_info "You can update the PR title on GitHub after pushing"
        log_info "Suggested title: $NEW_TITLE"
    fi
fi

# Step 5: Push changes
echo ""
log_info "Step 5: Ready to push changes"

# Check if there are changes to push
if git diff upstream/dev..HEAD --quiet; then
    log_info "No changes to push"
else
    log_info "Changes ready to push to origin/$CURRENT_BRANCH"

    if confirm "Push changes now?"; then
        log_info "Pushing to origin/$CURRENT_BRANCH..."
        if git push -f origin "$CURRENT_BRANCH"; then
            log_success "Successfully pushed changes!"
        else
            log_error "Failed to push. You may need to push manually:"
            echo "  git push -f origin $CURRENT_BRANCH"
            exit 1
        fi
    else
        log_info "Skipped push. You can push manually later:"
        echo "  git push -f origin $CURRENT_BRANCH"
    fi
fi

# Summary
echo ""
echo "╔═══════════════════════════════════════════╗"
echo "║  ✅ Migration Complete!                   ║"
echo "╚═══════════════════════════════════════════╝"
echo ""

log_success "Your PR has been migrated!"

echo ""
log_info "Next steps:"
echo "  1. Check your PR on GitHub"
echo "  2. Update PR title if needed (Conventional Commits format)"
echo "  3. Wait for CI checks to run"
echo "  4. Address any reviewer feedback"
echo ""

log_info "Need help? Ask in the PR comments or Telegram!"
log_info "Telegram: https://t.me/nofx_dev_community"

echo ""
log_success "Thank you for contributing to NOFX! 🚀"
echo ""
