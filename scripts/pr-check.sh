#!/bin/bash

# 🔍 PR Health Check Script
# Analyzes your PR and gives suggestions on how to meet the new standards
# This script only analyzes and suggests - it won't modify your code

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Counters
ISSUES_FOUND=0
WARNINGS_FOUND=0
PASSED_CHECKS=0

# Helper functions
log_section() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════${NC}"
}

log_check() {
    echo -e "${BLUE}🔍 Checking: $1${NC}"
}

log_pass() {
    echo -e "${GREEN}✅ PASS: $1${NC}"
    ((PASSED_CHECKS++))
}

log_warning() {
    echo -e "${YELLOW}⚠️  WARNING: $1${NC}"
    ((WARNINGS_FOUND++))
}

log_error() {
    echo -e "${RED}❌ ISSUE: $1${NC}"
    ((ISSUES_FOUND++))
}

log_suggestion() {
    echo -e "${CYAN}💡 Suggestion: $1${NC}"
}

log_command() {
    echo -e "${GREEN}   Run: ${NC}$1"
}

# Welcome
echo ""
echo "╔═══════════════════════════════════════════╗"
echo "║  NOFX PR Health Check                     ║"
echo "║  Analyze your PR and get suggestions      ║"
echo "╚═══════════════════════════════════════════╝"
echo ""

# Check if we're in a git repo
if ! git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
    log_error "Not a git repository"
    exit 1
fi

# Get current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo -e "${BLUE}Current branch: ${GREEN}$CURRENT_BRANCH${NC}"

if [ "$CURRENT_BRANCH" = "main" ] || [ "$CURRENT_BRANCH" = "dev" ]; then
    log_error "You're on the $CURRENT_BRANCH branch. Please switch to your PR branch."
    exit 1
fi

# Check if upstream exists
if ! git remote | grep -q "^upstream$"; then
    log_warning "Upstream remote not found"
    log_suggestion "Add upstream remote:"
    log_command "git remote add upstream https://github.com/tinkle-community/nofx.git"
    echo ""
fi

# ═══════════════════════════════════════════
# 1. GIT BRANCH CHECKS
# ═══════════════════════════════════════════
log_section "1. Git Branch Status"

# Check if branch is up to date with upstream
log_check "Is branch based on latest upstream/dev?"
if git remote | grep -q "^upstream$"; then
    git fetch upstream -q 2>/dev/null || true

    if git merge-base --is-ancestor upstream/dev HEAD 2>/dev/null; then
        log_pass "Branch is up to date with upstream/dev"
    else
        log_error "Branch is not based on latest upstream/dev"
        log_suggestion "Rebase your branch:"
        log_command "git fetch upstream && git rebase upstream/dev"
        echo ""
    fi
else
    log_warning "Cannot check - upstream remote not configured"
fi

# Check for merge conflicts
log_check "Any merge conflicts?"
if git diff --check > /dev/null 2>&1; then
    log_pass "No merge conflicts detected"
else
    log_error "Merge conflicts detected"
    log_suggestion "Resolve conflicts and commit"
fi

# ═══════════════════════════════════════════
# 2. COMMIT MESSAGE CHECKS
# ═══════════════════════════════════════════
log_section "2. Commit Messages"

# Get commits in this branch (not in upstream/dev)
if git remote | grep -q "^upstream$"; then
    COMMITS=$(git log upstream/dev..HEAD --oneline 2>/dev/null || git log --oneline -10)
else
    COMMITS=$(git log --oneline -10)
fi

COMMIT_COUNT=$(echo "$COMMITS" | wc -l | tr -d ' ')
echo -e "${BLUE}Found $COMMIT_COUNT commit(s) in your branch${NC}"
echo ""

# Check each commit message
echo "$COMMITS" | while read -r line; do
    COMMIT_MSG=$(echo "$line" | cut -d' ' -f2-)

    # Check if follows conventional commits
    if echo "$COMMIT_MSG" | grep -qE "^(feat|fix|docs|style|refactor|perf|test|chore|ci|security)(\(.+\))?: .+"; then
        log_pass "\"$COMMIT_MSG\""
    else
        log_warning "\"$COMMIT_MSG\""
        log_suggestion "Should follow format: type(scope): description"
        echo "   Examples:"
        echo "   - feat(exchange): add OKX integration"
        echo "   - fix(trader): resolve position bug"
        echo ""
    fi
done

# Suggest PR title based on commits
echo ""
log_check "Suggested PR title:"
SUGGESTED_TITLE=$(git log --pretty=%s upstream/dev..HEAD 2>/dev/null | head -1 || git log --pretty=%s -1)
echo -e "${GREEN}   \"$SUGGESTED_TITLE\"${NC}"
echo ""

# ═══════════════════════════════════════════
# 3. CODE QUALITY - BACKEND (Go)
# ═══════════════════════════════════════════
if find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | grep -q .; then
    log_section "3. Backend Code Quality (Go)"

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_warning "Go not installed - skipping backend checks"
        log_suggestion "Install Go: https://go.dev/doc/install"
    else
        # Check go fmt
        log_check "Go code formatting (go fmt)"
        UNFORMATTED=$(gofmt -l . 2>/dev/null | grep -v vendor || true)
        if [ -z "$UNFORMATTED" ]; then
            log_pass "All Go files are formatted"
        else
            log_error "Some files need formatting:"
            echo "$UNFORMATTED" | head -5 | while read -r file; do
                echo "   - $file"
            done
            log_suggestion "Format your code:"
            log_command "go fmt ./..."
            echo ""
        fi

        # Check go vet
        log_check "Go static analysis (go vet)"
        if go vet ./... > /tmp/vet-output.txt 2>&1; then
            log_pass "No issues found by go vet"
        else
            log_error "Go vet found issues:"
            head -10 /tmp/vet-output.txt | sed 's/^/   /'
            log_suggestion "Fix the issues above"
            echo ""
        fi

        # Check tests exist
        log_check "Do tests exist?"
        TEST_FILES=$(find . -name "*_test.go" -not -path "./vendor/*" | wc -l)
        if [ "$TEST_FILES" -gt 0 ]; then
            log_pass "Found $TEST_FILES test file(s)"
        else
            log_warning "No test files found"
            log_suggestion "Add tests for your changes"
            echo ""
        fi

        # Run tests
        log_check "Running Go tests..."
        if go test ./... -v > /tmp/test-output.txt 2>&1; then
            log_pass "All tests passed"
        else
            log_error "Some tests failed:"
            grep -E "FAIL|ERROR" /tmp/test-output.txt | head -10 | sed 's/^/   /' || true
            log_suggestion "Fix failing tests:"
            log_command "go test ./... -v"
            echo ""
        fi
    fi
fi

# ═══════════════════════════════════════════
# 4. CODE QUALITY - FRONTEND
# ═══════════════════════════════════════════
if [ -d "web" ]; then
    log_section "4. Frontend Code Quality"

    # Check if npm is installed
    if ! command -v npm &> /dev/null; then
        log_warning "npm not installed - skipping frontend checks"
        log_suggestion "Install Node.js: https://nodejs.org/"
    else
        cd web

        # Check if node_modules exists
        if [ ! -d "node_modules" ]; then
            log_warning "Dependencies not installed"
            log_suggestion "Install dependencies:"
            log_command "cd web && npm install"
            cd ..
        else
            # Check linting
            log_check "Frontend linting"
            if npm run lint > /tmp/lint-output.txt 2>&1; then
                log_pass "No linting issues"
            else
                log_error "Linting issues found:"
                tail -20 /tmp/lint-output.txt | sed 's/^/   /' || true
                log_suggestion "Fix linting issues:"
                log_command "cd web && npm run lint -- --fix"
                echo ""
            fi

            # Check type errors
            log_check "TypeScript type checking"
            if npm run type-check > /tmp/typecheck-output.txt 2>&1; then
                log_pass "No type errors"
            else
                log_error "Type errors found:"
                tail -20 /tmp/typecheck-output.txt | sed 's/^/   /' || true
                log_suggestion "Fix type errors in your code"
                echo ""
            fi

            # Check build
            log_check "Frontend build"
            if npm run build > /tmp/build-output.txt 2>&1; then
                log_pass "Build successful"
            else
                log_error "Build failed:"
                tail -20 /tmp/build-output.txt | sed 's/^/   /' || true
                log_suggestion "Fix build errors"
                echo ""
            fi
        fi

        cd ..
    fi
fi

# ═══════════════════════════════════════════
# 5. PR SIZE CHECK
# ═══════════════════════════════════════════
log_section "5. PR Size"

if git remote | grep -q "^upstream$"; then
    ADDED=$(git diff --numstat upstream/dev...HEAD | awk '{sum+=$1} END {print sum+0}')
    DELETED=$(git diff --numstat upstream/dev...HEAD | awk '{sum+=$2} END {print sum+0}')
    TOTAL=$((ADDED + DELETED))
    FILES_CHANGED=$(git diff --name-only upstream/dev...HEAD | wc -l)

    echo -e "${BLUE}Lines changed: ${GREEN}+$ADDED ${RED}-$DELETED ${NC}(total: $TOTAL)"
    echo -e "${BLUE}Files changed: ${GREEN}$FILES_CHANGED${NC}"
    echo ""

    if [ "$TOTAL" -lt 100 ]; then
        log_pass "Small PR (<100 lines) - ideal for quick review"
    elif [ "$TOTAL" -lt 500 ]; then
        log_pass "Medium PR (100-500 lines) - reasonable size"
    elif [ "$TOTAL" -lt 1000 ]; then
        log_warning "Large PR (500-1000 lines) - consider splitting"
        log_suggestion "Breaking into smaller PRs makes review faster"
    else
        log_error "Very large PR (>1000 lines) - strongly consider splitting"
        log_suggestion "Split into multiple smaller PRs, each with a focused change"
        echo ""
    fi
fi

# ═══════════════════════════════════════════
# 6. DOCUMENTATION CHECK
# ═══════════════════════════════════════════
log_section "6. Documentation"

# Check if README or docs were updated
log_check "Documentation updates"
if git remote | grep -q "^upstream$"; then
    DOC_CHANGES=$(git diff --name-only upstream/dev...HEAD | grep -E "\.(md|txt)$" || true)

    if [ -n "$DOC_CHANGES" ]; then
        log_pass "Documentation files updated"
        echo "$DOC_CHANGES" | sed 's/^/   - /'
    else
        # Check if this is a feature/fix that might need docs
        COMMIT_TYPES=$(git log --pretty=%s upstream/dev..HEAD | grep -oE "^(feat|fix)" || true)
        if [ -n "$COMMIT_TYPES" ]; then
            log_warning "No documentation updates found"
            log_suggestion "Consider updating docs if your changes affect usage"
            echo ""
        else
            log_pass "No documentation update needed"
        fi
    fi
fi

# ═══════════════════════════════════════════
# 7. ROADMAP ALIGNMENT
# ═══════════════════════════════════════════
log_section "7. Roadmap Alignment"

log_check "Does your PR align with the roadmap?"
echo ""
echo "Current priorities (Phase 1):"
echo "  ✅ Security enhancements"
echo "  ✅ AI model integrations"
echo "  ✅ Exchange integrations (OKX, Bybit, Lighter, EdgeX)"
echo "  ✅ UI/UX improvements"
echo "  ✅ Performance optimizations"
echo "  ✅ Bug fixes"
echo ""
log_suggestion "Check roadmap: https://github.com/tinkle-community/nofx/blob/dev/docs/roadmap/README.md"
echo ""

# ═══════════════════════════════════════════
# FINAL REPORT
# ═══════════════════════════════════════════
log_section "Summary Report"

echo ""
echo -e "${GREEN}✅ Passed checks: $PASSED_CHECKS${NC}"
echo -e "${YELLOW}⚠️  Warnings: $WARNINGS_FOUND${NC}"
echo -e "${RED}❌ Issues found: $ISSUES_FOUND${NC}"
echo ""

# Overall assessment
if [ "$ISSUES_FOUND" -eq 0 ] && [ "$WARNINGS_FOUND" -eq 0 ]; then
    echo "╔═══════════════════════════════════════════╗"
    echo "║  🎉 Excellent! Your PR looks great!      ║"
    echo "║  Ready to submit or update your PR       ║"
    echo "╚═══════════════════════════════════════════╝"
elif [ "$ISSUES_FOUND" -eq 0 ]; then
    echo "╔═══════════════════════════════════════════╗"
    echo "║  👍 Good! Minor warnings found            ║"
    echo "║  Consider addressing warnings             ║"
    echo "╚═══════════════════════════════════════════╝"
elif [ "$ISSUES_FOUND" -le 3 ]; then
    echo "╔═══════════════════════════════════════════╗"
    echo "║  ⚠️  Issues found - Please fix            ║"
    echo "║  See suggestions above                    ║"
    echo "╚═══════════════════════════════════════════╝"
else
    echo "╔═══════════════════════════════════════════╗"
    echo "║  ❌ Multiple issues found                 ║"
    echo "║  Please address issues before submitting  ║"
    echo "╚═══════════════════════════════════════════╝"
fi

echo ""
echo "📖 Next steps:"
echo ""

if [ "$ISSUES_FOUND" -gt 0 ] || [ "$WARNINGS_FOUND" -gt 0 ]; then
    echo "1. Fix the issues and warnings listed above"
    echo "2. Run this script again to verify: ./scripts/pr-check.sh"
    echo "3. Commit your fixes"
    echo "4. Push to your PR: git push origin $CURRENT_BRANCH"
else
    echo "1. Push your changes: git push origin $CURRENT_BRANCH"
    echo "2. Create or update your PR on GitHub"
    echo "3. Wait for automated CI checks"
    echo "4. Address reviewer feedback"
fi

echo ""
echo "📚 Resources:"
echo "  - Contributing Guide: https://github.com/tinkle-community/nofx/blob/dev/CONTRIBUTING.md"
echo "  - Migration Guide: https://github.com/tinkle-community/nofx/blob/dev/docs/community/MIGRATION_ANNOUNCEMENT.md"
echo ""

# Cleanup temp files
rm -f /tmp/vet-output.txt /tmp/test-output.txt /tmp/lint-output.txt /tmp/typecheck-output.txt /tmp/build-output.txt

echo "✨ Analysis complete! Good luck with your PR! 🚀"
echo ""
