# PR Templates

## 📋 Template Overview

We offer 4 specialized templates for different types of PRs to help contributors quickly fill out PR information:

### 1. 🔧 Backend Template
**File:** `backend.md`

**Use for:**
- Go code changes
- API endpoint development
- Trading logic implementation
- Backend performance optimization
- Database-related changes

**Includes:**
- Go test environment
- Security considerations
- Performance impact assessment
- `go fmt` and `go build` checks

### 2. 🎨 Frontend Template
**File:** `frontend.md`

**Use for:**
- UI/UX changes
- React/Vue component development
- Frontend styling updates
- Browser compatibility fixes
- Frontend performance optimization

**Includes:**
- Screenshots/demo requirements
- Browser testing checklist
- Internationalization checks
- Responsive design verification
- `npm run lint` and `npm run build` checks

### 3. 📝 Documentation Template
**File:** `docs.md`

**Use for:**
- README updates
- API documentation
- Tutorials and guides
- Code comment improvements
- Translation work

**Includes:**
- Documentation type classification
- Content quality checks
- Bilingual requirements (EN/CN)
- Link validity verification

### 4. 📦 General Template
**File:** `general.md`

**Use for:**
- Mixed-type changes
- Cross-domain PRs
- Build configuration changes
- Dependency updates
- When unsure which template to use

## 🤖 Automatic Template Suggestion

Our GitHub Action automatically analyzes your PR and suggests the most suitable template:

### How it works:

1. **File Analysis**
   - Detects all changed file types in the PR

2. **Smart Detection**
   - If >50% are `.go` files → Suggests **Backend template**
   - If >50% are `.js/.ts/.tsx/.vue` files → Suggests **Frontend template**
   - If >70% are `.md` files → Suggests **Documentation template**

3. **Auto-comment**
   - If it detects you're using the default template but should use a specialized one
   - It will automatically add a friendly comment suggestion

4. **Auto-labeling**
   - Automatically adds corresponding labels: `backend`, `frontend`, `documentation`

## 📖 How to Use

### Method 1: URL Parameter (Recommended)

When creating a PR, add the template parameter to the URL:

```
https://github.com/YOUR_ORG/nofx/compare/dev...YOUR_BRANCH?template=backend.md
```

Replace `backend.md` with:
- `backend.md` - Backend template
- `frontend.md` - Frontend template
- `docs.md` - Documentation template
- `general.md` - General template

### Method 2: Manual Selection

1. When creating a PR, the default template will be shown

2. Follow the guidance links at the top to view the corresponding template

3. Copy the template content into the PR description

### Method 3: Follow Auto-suggestion

1. Create a PR with any template

2. GitHub Action will automatically analyze and comment with a suggestion

3. Update the PR description based on the suggestion

## 🎯 Best Practices

1. **Choose in Advance**
   - Determine the change type before creating the PR

2. **Complete Filling**
   - Don't skip required items

3. **Keep it Concise**
   - Keep descriptions clear but concise

4. **Add Screenshots**
   - For UI changes, always add screenshots

5. **Test Evidence**
   - Provide evidence that tests pass

## 🔧 Customization

If you need to modify templates or auto-detection logic:

1. **Modify Templates**
   - Edit `.github/PULL_REQUEST_TEMPLATE/*.md` files

2. **Adjust Detection Threshold**
   - Edit `.github/workflows/pr-template-suggester.yml`
   - Modify file type percentage thresholds (current: 50% backend, 50% frontend, 70% docs)

3. **Add New Template**
   - Create a new `.md` file in the `PULL_REQUEST_TEMPLATE/` directory
   - Update the workflow to support new file type detection

## ❓ FAQ

**Q: My PR has both frontend and backend code, which template should I use?**

A: Use the **General template** (`general.md`), or choose the template for the primary change type.

---

**Q: What if the automatically suggested template is not suitable?**

A: You can ignore the suggestion and continue using the current template. Auto-suggestions are for reference only.

---

**Q: Can I not use any template?**

A: Not recommended. Templates help ensure PRs contain necessary information and speed up reviews.

---

**Q: How to disable automatic template suggestions?**

A: Delete or disable the `.github/workflows/pr-template-suggester.yml` file.

---

🌟 **Thank you for using our PR template system!**
