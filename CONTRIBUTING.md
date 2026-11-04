# 🤝 Contributing to NOFX

**Language:** [English](CONTRIBUTING.md) | [中文](docs/i18n/zh-CN/CONTRIBUTING.md)

Thank you for your interest in contributing to NOFX! This document provides guidelines and workflows for contributing to the project.

---

## 📑 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
- [Development Workflow](#development-workflow)
- [PR Submission Guidelines](#pr-submission-guidelines)
- [Coding Standards](#coding-standards)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Review Process](#review-process)
- [Bounty Program](#bounty-program)

---

## 📜 Code of Conduct

This project adheres to the [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

---

## 🎯 How Can I Contribute?

### 1. Report Bugs 🐛

- Use the [Bug Report Template](.github/ISSUE_TEMPLATE/bug_report.md)
- Check if the bug has already been reported
- Include detailed reproduction steps
- Provide environment information (OS, Go version, etc.)

### 2. Suggest Features ✨

- Use the [Feature Request Template](.github/ISSUE_TEMPLATE/feature_request.md)
- Explain the use case and benefits
- Check if it aligns with the [project roadmap](docs/roadmap/README.md)

### 3. Submit Pull Requests 🔧

Before submitting a PR, please check the following:

#### ✅ **Accepted Contributions**

**High Priority** (aligned with roadmap):
- 🔒 Security enhancements (encryption, authentication, RBAC)
- 🧠 AI model integrations (GPT-4, Claude, Gemini Pro)
- 🔗 Exchange integrations (OKX, Bybit, Lighter, EdgeX)
- 📊 Trading data APIs (AI500, OI analysis, NetFlow)
- 🎨 UI/UX improvements (mobile responsiveness, charts)
- ⚡ Performance optimizations
- 🐛 Bug fixes
- 📝 Documentation improvements

**Medium Priority:**
- ✅ Test coverage improvements
- 🌐 Internationalization (new language support)
- 🔧 Build/deployment tooling
- 📈 Monitoring and logging enhancements

#### ❌ **Not Accepted** (without prior discussion)

- Major architectural changes without RFC (Request for Comments)
- Features not aligned with project roadmap
- Breaking changes without migration path
- Code that introduces new dependencies without justification
- Experimental features without opt-in flag

**⚠️ Important:** For major features, please open an issue for discussion **before** starting work.

---

## 🛠️ Development Workflow

### 1. Fork and Clone

```bash
# Fork the repository on GitHub
# Then clone your fork
git clone https://github.com/YOUR_USERNAME/nofx.git
cd nofx

# Add upstream remote
git remote add upstream https://github.com/NoFxAiOS/nofx.git
```

### 2. Create a Feature Branch

```bash
# Update your local dev branch
git checkout dev
git pull upstream dev

# Create a new branch
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

**Branch Naming Convention:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring
- `perf/` - Performance improvements
- `test/` - Test updates
- `chore/` - Build/config changes

### 3. Set Up Development Environment

```bash
# Install Go dependencies
go mod download

# Install frontend dependencies
cd web
npm install
cd ..

# Install TA-Lib (required)
# macOS:
brew install ta-lib

# Ubuntu/Debian:
sudo apt-get install libta-lib0-dev
```

### 4. Make Your Changes

- Follow the [coding standards](#coding-standards)
- Write tests for new features
- Update documentation as needed
- Keep commits focused and atomic

### 5. Test Your Changes

```bash
# Run backend tests
go test ./...

# Build backend
go build -o nofx

# Run frontend in dev mode
cd web
npm run dev

# Build frontend
npm run build
```

### 6. Commit Your Changes

Follow the [commit message guidelines](#commit-message-guidelines):

```bash
git add .
git commit -m "feat: add support for OKX exchange integration"
```

### 7. Push and Create PR

```bash
# Push to your fork
git push origin feature/your-feature-name

# Go to GitHub and create a Pull Request
# Use the PR template and fill in all sections
```

---

## 📝 PR Submission Guidelines

### Before Submitting

- [ ] Code compiles successfully (`go build` and `npm run build`)
- [ ] All tests pass (`go test ./...`)
- [ ] No linting errors (`go fmt`, `go vet`)
- [ ] Documentation is updated
- [ ] Commits follow conventional commits format
- [ ] Branch is rebased on latest `dev`

### PR Title Format

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <subject>

Examples:
feat(exchange): add OKX exchange integration
fix(trader): resolve position tracking bug
docs(readme): update installation instructions
perf(ai): optimize prompt generation
refactor(core): extract common exchange interface
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation
- `style` - Code style (formatting, no logic change)
- `refactor` - Code refactoring
- `perf` - Performance improvement
- `test` - Test updates
- `chore` - Build/config changes
- `ci` - CI/CD changes
- `security` - Security improvements

### PR Description

Use the [PR template](.github/PULL_REQUEST_TEMPLATE.md) and ensure:

1. **Clear description** of what and why
2. **Type of change** is marked
3. **Related issues** are linked
4. **Testing steps** are documented
5. **Screenshots** for UI changes
6. **All checkboxes** are completed

### PR Size

Keep PRs focused and reasonably sized:

- ✅ **Small PR** (< 300 lines): Ideal, fast review
- ⚠️ **Medium PR** (300-1000 lines): Acceptable, may take longer
- ❌ **Large PR** (> 1000 lines): Please break into smaller PRs

---

## 💻 Coding Standards

### Go Code

```go
// ✅ Good: Clear naming, proper error handling
func ConnectToExchange(apiKey, secret string) (*Exchange, error) {
    if apiKey == "" || secret == "" {
        return nil, fmt.Errorf("API credentials are required")
    }

    client, err := createClient(apiKey, secret)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    return &Exchange{client: client}, nil
}

// ❌ Bad: Poor naming, no error handling
func ce(a, s string) *Exchange {
    c := createClient(a, s)
    return &Exchange{client: c}
}
```

**Best Practices:**
- Use meaningful variable names
- Handle all errors explicitly
- Add comments for complex logic
- Follow Go idioms and conventions
- Run `go fmt` before committing
- Use `go vet` and `golangci-lint`

### TypeScript/React Code

```typescript
// ✅ Good: Type-safe, clear naming
interface TraderConfig {
  id: string;
  exchange: 'binance' | 'hyperliquid' | 'aster';
  aiModel: string;
  enabled: boolean;
}

const TraderCard: React.FC<{ trader: TraderConfig }> = ({ trader }) => {
  const [isRunning, setIsRunning] = useState(false);

  const handleStart = async () => {
    try {
      await startTrader(trader.id);
      setIsRunning(true);
    } catch (error) {
      console.error('Failed to start trader:', error);
    }
  };

  return <div>...</div>;
};

// ❌ Bad: No types, unclear naming
const TC = (props) => {
  const [r, setR] = useState(false);
  const h = () => { startTrader(props.t.id); setR(true); };
  return <div>...</div>;
};
```

**Best Practices:**
- Use TypeScript strict mode
- Define interfaces for all data structures
- Avoid `any` type
- Use functional components with hooks
- Follow React best practices
- Run `npm run lint` before committing

### File Structure

```
NOFX/
├── cmd/               # Main applications
├── internal/          # Private code
│   ├── exchange/      # Exchange adapters
│   ├── trader/        # Trading logic
│   ├── ai/           # AI integrations
│   └── api/          # API handlers
├── pkg/              # Public libraries
├── web/              # Frontend
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   └── utils/
│   └── public/
└── docs/             # Documentation
```

---

## 📋 Commit Message Guidelines

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Examples

```
feat(exchange): add OKX futures API integration

- Implement order placement and cancellation
- Add balance and position retrieval
- Support leverage configuration

Closes #123
```

```
fix(trader): prevent duplicate position opening

The trader was opening multiple positions in the same direction
for the same symbol. Added check to prevent this behavior.

Fixes #456
```

```
docs: update Docker deployment guide

- Add troubleshooting section
- Update environment variables
- Add examples for common scenarios
```

### Rules

- Use present tense ("add" not "added")
- Use imperative mood ("move" not "moves")
- First line ≤ 72 characters
- Reference issues and PRs
- Explain "what" and "why", not "how"

---

## 🔍 Review Process

### Timeline

- **Initial review:** Within 2-3 business days
- **Follow-up reviews:** Within 1-2 business days
- **Bounty PRs:** Priority review within 1 business day

### Review Criteria

Reviewers will check:

1. **Functionality**
   - Does it work as intended?
   - Are edge cases handled?
   - No regression in existing features?

2. **Code Quality**
   - Follows coding standards?
   - Well-structured and readable?
   - Proper error handling?

3. **Testing**
   - Adequate test coverage?
   - Tests pass in CI?
   - Manual testing documented?

4. **Documentation**
   - Code comments where needed?
   - README/docs updated?
   - API changes documented?

5. **Security**
   - No hardcoded secrets?
   - Input validation?
   - No known vulnerabilities?

### Response to Feedback

- Address all review comments
- Ask questions if unclear
- Mark conversations as resolved
- Re-request review after changes

### Approval and Merge

- Requires **1 approval** from maintainers
- All CI checks must pass
- No unresolved conversations
- Maintainers will merge (squash merge for small PRs, merge commit for features)

---

## 💰 Bounty Program

### How It Works

1. Check [open bounty issues](https://github.com/NoFxAiOS/nofx/labels/bounty)
2. Comment to claim (first come, first served)
3. Complete work within deadline
4. Submit PR with bounty claim section filled
5. Get paid upon merge

### Guidelines

- Read [Bounty Guide](docs/community/bounty-guide.md)
- Meet all acceptance criteria
- Include demo video/screenshots
- Follow all contribution guidelines
- Payment details discussed privately

---

## ❓ Questions?


- **Technical questions:** Open a [Discussion](https://github.com/NoFxAiOS/nofx/discussions)
- **Security issues:** See [Security Policy](SECURITY.md)
- **Bug reports:** Use [Bug Report Template](.github/ISSUE_TEMPLATE/bug_report.md)

---

## 📚 Additional Resources

- [Project Roadmap](docs/roadmap/README.md)
- [Architecture Documentation](docs/architecture/README.md)
- [Deployment Guide](docs/getting-started/docker-deploy.en.md)

---

## 🙏 Thank You!

Your contributions make NOFX better for everyone. We appreciate your time and effort!

**Happy coding! 🚀**
