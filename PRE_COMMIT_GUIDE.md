# Pre-commit Setup Guide

Pre-commit has been successfully installed and configured for the NOFX project.

## What was installed:

1. **Pre-commit** - Runs quality checks before each commit
2. **Python virtual environment** - Located at `.venv/` for isolated Python dependencies
3. **Pre-commit hooks configuration** - Located at `.pre-commit-config.yaml`

## Active hooks:

### General Checks
- **trailing-whitespace** - Removes trailing whitespace
- **end-of-file-fixer** - Ensures files end with newlines
- **check-yaml** - Validates YAML syntax
- **check-json** - Validates JSON syntax (excludes tsconfig.json with comments)
- **check-toml** - Validates TOML syntax
- **check-added-large-files** - Prevents large files from being committed
- **check-merge-conflict** - Detects merge conflict markers

### Go Code Quality
- **go fmt** - Formats Go code automatically
- **go vet** - Runs Go static analysis (excludes scripts/ directory)
- **go mod tidy** - Cleans up go.mod and go.sum files

## How it works:

When you run `git commit`, pre-commit will automatically:
1. Check all staged files
2. Apply formatting fixes where possible
3. Report any issues that need manual fixes
4. Only allow commit if all checks pass

## Manual usage:

Run all hooks on all files:
```bash
/home/jeffee/Desktop/nofx/.venv/bin/pre-commit run --all-files
```

Run hooks on staged files only:
```bash
/home/jeffee/Desktop/nofx/.venv/bin/pre-commit run
```

Update hook versions:
```bash
/home/jeffee/Desktop/nofx/.venv/bin/pre-commit autoupdate
```

## Frontend code quality:

The frontend (web/) directory already has its own quality tools via Husky and lint-staged:
- ESLint for code linting
- Prettier for code formatting
- These run automatically via the existing `web/.husky` Git hooks

## Bypassing hooks (use sparingly):

If you need to commit without running hooks:
```bash
git commit --no-verify -m "Your commit message"
```

## Benefits:

✅ Consistent code formatting across the team
✅ Early detection of syntax errors and issues
✅ Automated cleanup of common issues
✅ Faster CI/CD pipeline due to pre-validated code
✅ Better code quality and maintainability

The setup aligns with the project's existing code quality standards mentioned in CONTRIBUTING.md and integrates well with the existing Husky+lint-staged setup in the web/ directory.
