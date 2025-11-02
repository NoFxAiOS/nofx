# GitHub Actions Workflows

This directory contains automated workflows for the NOFX project.

## Workflows

### `translate-issues.yml` - Auto Translate Issues

Automatically translates GitHub issues between Chinese and English using DeepSeek API.

**Features:**
- ✅ Detects issue language (Chinese/English)
- ✅ Translates to the opposite language
- ✅ Adds translation as a comment
- ✅ Preserves markdown formatting, code blocks, and links
- ✅ Cost-effective (uses DeepSeek API)

**How it works:**
1. Triggered when an issue is opened or edited
2. Detects if the issue is primarily in Chinese or English
3. Translates to the opposite language using DeepSeek API
4. Adds the translation as a comment

**Setup:**

This workflow requires a DeepSeek API key to be configured as a GitHub secret:

1. Get your DeepSeek API key from https://platform.deepseek.com/
2. Go to repository Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Name: `DEEPSEEK_API_KEY`
5. Value: Your DeepSeek API key
6. Click "Add secret"

**Cost:**
- DeepSeek API is very affordable (~$0.001 per issue translation)
- Much cheaper than GPT-4 or Claude

**Skipping translation:**
If `DEEPSEEK_API_KEY` is not set, the workflow will skip translation gracefully.

---

### `test.yml` - Test Workflow

Runs backend and frontend tests on push/PR.

**Features:**
- ✅ Backend tests (Go)
- ✅ Frontend tests (Vitest)
- ✅ Non-blocking (won't prevent PR merges)

See [PR #229](https://github.com/tinkle-community/nofx/pull/229) for details.

---

## Contributing

When adding new workflows:
1. Test locally first if possible
2. Document the workflow in this README
3. Add any required secrets to the setup instructions
4. Make workflows non-blocking unless critical
