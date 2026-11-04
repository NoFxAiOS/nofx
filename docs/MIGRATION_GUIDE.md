# 📦 Documentation Migration Guide

## What Changed?

NOFX documentation has been reorganized into a structured `docs/` directory for better organization and navigation.

## 🗺️ File Locations (Old → New)

### Deployment Guides
- `DOCKER_DEPLOY.en.md` → `docs/getting-started/docker-deploy.en.md`
- `DOCKER_DEPLOY.md` → `docs/getting-started/docker-deploy.zh-CN.md`
- `CUSTOM_API.md` → `docs/getting-started/custom-api.md`

### Community Docs
- `HOW_TO_POST_BOUNTY.md` → `docs/community/bounty-guide.md`
- `INTEGRATION_BOUNTY_HYPERLIQUID.md` → `docs/community/bounty-hyperliquid.md`
- `INTEGRATION_BOUNTY_ASTER.md` → `docs/community/bounty-aster.md`

### Internationalization
- `README.zh-CN.md` → `docs/i18n/zh-CN/README.md`
- `README.ru.md` → `docs/i18n/ru/README.md`
- `README.uk.md` → `docs/i18n/uk/README.md`
- `常见问题.md` → `docs/guides/faq.zh-CN.md`

### Root Directory (Unchanged)
These stay in the root for GitHub recognition:
- `README.md` ✅ (stays in root)
- `LICENSE` ✅ (stays in root)
- `CONTRIBUTING.md` ✅ (stays in root)
- `CODE_OF_CONDUCT.md` ✅ (stays in root)
- `SECURITY.md` ✅ (stays in root)

## 🎯 Why This Change?

### Before (❌ Problems)
```
nofx/
├── README.md
├── README.zh-CN.md
├── README.ru.md
├── README.uk.md
├── DOCKER_DEPLOY.md
├── DOCKER_DEPLOY.en.md
├── CUSTOM_API.md
├── HOW_TO_POST_BOUNTY.md
├── INTEGRATION_BOUNTY_HYPERLIQUID.md
├── INTEGRATION_BOUNTY_ASTER.md
├── 常见问题.md
└── ... (15+ markdown files in root!)
```

**Issues:**
- 😵 Too cluttered (15+ files in root)
- 🔍 Hard to find specific docs
- 🌍 Mixed languages
- 📚 No clear organization

### After (✅ Benefits)
```
nofx/
├── README.md              # Project homepage
├── LICENSE                # Legal (GitHub needs it here)
├── CONTRIBUTING.md        # GitHub auto-links
├── CODE_OF_CONDUCT.md     # GitHub auto-links
├── SECURITY.md            # GitHub auto-links
│
└── docs/                  # 📚 Documentation hub
    ├── README.md          # Documentation home
    ├── getting-started/   # 🚀 Setup guides
    ├── guides/            # 📘 User guides
    ├── community/         # 👥 Contribution docs
    ├── i18n/              # 🌍 Translations
    └── architecture/      # 🏗️ Technical docs
```

**Benefits:**
- ✅ Clean root directory
- ✅ Logical categorization
- ✅ Easy navigation
- ✅ Scalable structure
- ✅ Professional appearance

## 📚 New Documentation Structure

### Root Level
Files GitHub needs to see:
- `README.md` - Main project page
- `LICENSE` - Open source license
- `CONTRIBUTING.md` - Contributor guide
- `CODE_OF_CONDUCT.md` - Community standards
- `SECURITY.md` - Security policy

### docs/ Level

**Navigation:**
- `docs/README.md` - **Start here!** Main documentation hub

**Categories:**

1. **`getting-started/`** - Deployment and setup
   - Docker deployment (EN/中文)
   - Custom API configuration

2. **`guides/`** - Usage guides and tutorials
   - FAQ (中文)
   - Troubleshooting (planned)
   - Configuration examples (planned)

3. **`community/`** - Contribution and bounties
   - Bounty guide
   - Active bounty tasks
   - Contributor recognition

4. **`i18n/`** - International translations
   - `zh-CN/` - Simplified Chinese
   - `ru/` - Russian
   - `uk/` - Ukrainian

5. **`architecture/`** - Technical documentation
   - System design (planned)
   - API reference (planned)
   - Database schema (planned)

## 🔗 Updating Your Links

### If you bookmarked old links:

| Old Link | New Link |
|----------|----------|
| `DOCKER_DEPLOY.en.md` | `docs/getting-started/docker-deploy.en.md` |
| `README.zh-CN.md` | `docs/i18n/zh-CN/README.md` |
| `HOW_TO_POST_BOUNTY.md` | `docs/community/bounty-guide.md` |

### If you linked in your own docs:

**Update relative links:**
```markdown
<!-- Old -->
[Docker Deployment](DOCKER_DEPLOY.en.md)

<!-- New -->
[Docker Deployment](docs/getting-started/docker-deploy.en.md)
```

**GitHub URLs automatically redirect!**
- Old: `github.com/NoFxAiOS/nofx/blob/main/DOCKER_DEPLOY.en.md`
- Will redirect to: `github.com/.../docs/getting-started/docker-deploy.en.md`

## 🛠️ For Contributors

### Cloning/Pulling Latest

```bash
# Pull latest changes
git pull origin dev

# Your old bookmarks still work!
# Git tracked the file moves (git mv)
```

### Finding Documentation

**Use the navigation hub:**
1. Start at [docs/README.md](README.md)
2. Browse by category
3. Use the quick navigation section

**Or search:**
```bash
# Find all markdown docs
find docs -name "*.md"

# Search content
grep -r "keyword" docs/
```

### Adding New Documentation

**Follow the structure:**

```bash
# Getting started guides
docs/getting-started/your-guide.md

# User guides
docs/guides/your-tutorial.md

# Community docs
docs/community/your-doc.md

# Translations
docs/i18n/ja/README.md  # Japanese example
```

**Update navigation:**
- Add link in relevant category README
- Add to `docs/README.md` main hub

## 📝 Commit Messages

This reorganization was committed as:

```
docs: reorganize documentation into structured docs/ directory

- Move deployment guides to docs/getting-started/
- Move community docs to docs/community/
- Move translations to docs/i18n/
- Create navigation hub at docs/README.md
- Update all internal links in README.md
- Add GitHub issue/PR templates

BREAKING CHANGE: Direct links to moved files will need updating
(though GitHub redirects should work)

Closes #XXX
```

## 🆘 Need Help?

**Can't find a document?**
1. Check [docs/README.md](README.md) navigation hub
2. Search GitHub repo


**Link broken?**
- Report in [GitHub Issues](https://github.com/NoFxAiOS/nofx/issues)
- We'll fix it ASAP!

**Want to contribute docs?**
- See [Contributing Guide](../CONTRIBUTING.md)
- Check [docs/community/](community/README.md)

---

**Migration Date:** 2025-11-01
**Maintainers:** Tinkle Community

[← Back to Documentation Home](README.md)
