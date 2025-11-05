# PR Templates | PR 模板

## 📋 模板概述 | Template Overview

我们提供了4种针对不同类型PR的专用模板，帮助贡献者快速填写PR信息：
We offer 4 specialized templates for different types of PRs to help contributors quickly fill out PR information:

### 1. 🔧 Backend Template | 后端模板
**文件:** `backend.md`

**适用于 | Use for:**
- Go代码变更 | Go code changes
- API端点开发 | API endpoint development
- 交易逻辑实现 | Trading logic implementation
- 后端性能优化 | Backend performance optimization
- 数据库相关改动 | Database-related changes

**包含 | Includes:**
- Go测试环境配置 | Go test environment
- 安全考虑检查 | Security considerations
- 性能影响评估 | Performance impact assessment
- `go fmt` 和 `go build` 检查 | `go fmt` and `go build` checks

### 2. 🎨 Frontend Template | 前端模板
**文件:** `frontend.md`

**适用于 | Use for:**
- UI/UX变更 | UI/UX changes
- React/Vue组件开发 | React/Vue component development
- 前端样式更新 | Frontend styling updates
- 浏览器兼容性修复 | Browser compatibility fixes
- 前端性能优化 | Frontend performance optimization

**包含 | Includes:**
- 截图/演示要求 | Screenshots/demo requirements
- 浏览器测试清单 | Browser testing checklist
- 国际化检查 | Internationalization checks
- 响应式设计验证 | Responsive design verification
- `npm run lint` 和 `npm run build` 检查 | Linting and build checks

### 3. 📝 Documentation Template | 文档模板
**文件:** `docs.md`

**适用于 | Use for:**
- README更新 | README updates
- API文档编写 | API documentation
- 教程和指南 | Tutorials and guides
- 代码注释改进 | Code comment improvements
- 翻译工作 | Translation work

**包含 | Includes:**
- 文档类型分类 | Documentation type classification
- 内容质量检查 | Content quality checks
- 双语要求（中英文）| Bilingual requirements (EN/CN)
- 链接有效性验证 | Link validity verification

### 4. 📦 General Template | 通用模板
**文件:** `general.md`

**适用于 | Use for:**
- 混合类型变更 | Mixed-type changes
- 跨多个领域的PR | Cross-domain PRs
- 构建配置变更 | Build configuration changes
- 依赖更新 | Dependency updates
- 不确定使用哪个模板时 | When unsure which template to use

## 🤖 自动模板建议 | Automatic Template Suggestion

我们的GitHub Action会自动分析你的PR并建议最合适的模板：
Our GitHub Action automatically analyzes your PR and suggests the most suitable template:

### 工作原理 | How it works:

1. **文件分析 | File Analysis**
   - 检测PR中所有变更的文件类型
   - Detects all changed file types in the PR

2. **智能判断 | Smart Detection**
   - 如果 >50% 是 `.go` 文件 → 建议**后端模板**
   - If >50% are `.go` files → Suggests **Backend template**
   - 如果 >50% 是 `.js/.ts/.tsx/.vue` 文件 → 建议**前端模板**
   - If >50% are `.js/.ts/.tsx/.vue` files → Suggests **Frontend template**
   - 如果 >70% 是 `.md` 文件 → 建议**文档模板**
   - If >70% are `.md` files → Suggests **Documentation template**

3. **自动评论 | Auto-comment**
   - 如果检测到你使用了默认模板，但应该用专用模板
   - If it detects you're using the default template but should use a specialized one
   - 会自动添加友好的评论建议
   - It will automatically add a friendly comment suggestion

4. **自动标签 | Auto-labeling**
   - 自动添加对应的标签：`backend`、`frontend`、`documentation`
   - Automatically adds corresponding labels: `backend`, `frontend`, `documentation`

## 📖 使用方法 | How to Use

### 方法1: URL参数（推荐） | Method 1: URL Parameter (Recommended)

创建PR时，在URL末尾添加模板参数：
When creating a PR, add the template parameter to the URL:

```
https://github.com/YOUR_ORG/nofx/compare/dev...YOUR_BRANCH?template=backend.md
```

替换 `backend.md` 为：
Replace `backend.md` with:
- `backend.md` - 后端模板 | Backend template
- `frontend.md` - 前端模板 | Frontend template
- `docs.md` - 文档模板 | Documentation template
- `general.md` - 通用模板 | General template

### 方法2: 手动选择 | Method 2: Manual Selection

1. 创建PR时，默认模板会显示
   When creating a PR, the default template will be shown

2. 根据顶部的指引链接，点击查看对应的模板
   Follow the guidance links at the top to view the corresponding template

3. 复制模板内容到PR描述中
   Copy the template content into the PR description

### 方法3: 跟随自动建议 | Method 3: Follow Auto-suggestion

1. 使用任何模板创建PR
   Create a PR with any template

2. GitHub Action会自动分析并评论建议
   GitHub Action will automatically analyze and comment with a suggestion

3. 根据建议更新PR描述
   Update the PR description based on the suggestion

## 🎯 最佳实践 | Best Practices

1. **提前选择 | Choose in Advance**
   - 在创建PR前确定变更类型
   - Determine the change type before creating the PR

2. **完整填写 | Complete Filling**
   - 不要跳过必填项（标记为 required）
   - Don't skip required items

3. **保持简洁 | Keep it Concise**
   - 描述清晰但简洁
   - Keep descriptions clear but concise

4. **添加截图 | Add Screenshots**
   - 对于UI变更，务必添加截图
   - For UI changes, always add screenshots

5. **测试证明 | Test Evidence**
   - 提供测试通过的证据
   - Provide evidence that tests pass

## 🔧 自定义 | Customization

如果需要修改模板或自动检测逻辑：
If you need to modify templates or auto-detection logic:

1. **修改模板** | **Modify Templates**
   - 编辑 `.github/PULL_REQUEST_TEMPLATE/*.md` 文件
   - Edit `.github/PULL_REQUEST_TEMPLATE/*.md` files

2. **调整检测阈值** | **Adjust Detection Threshold**
   - 编辑 `.github/workflows/pr-template-suggester.yml`
   - Edit `.github/workflows/pr-template-suggester.yml`
   - 修改文件类型占比阈值（当前：50%后端，50%前端，70%文档）
   - Modify file type percentage thresholds (current: 50% backend, 50% frontend, 70% docs)

3. **添加新模板** | **Add New Template**
   - 在 `PULL_REQUEST_TEMPLATE/` 目录创建新的 `.md` 文件
   - Create a new `.md` file in the `PULL_REQUEST_TEMPLATE/` directory
   - 更新工作流以支持新的文件类型检测
   - Update the workflow to support new file type detection

## ❓ FAQ

**Q: 我的PR既有前端又有后端代码，用哪个模板？**
**Q: My PR has both frontend and backend code, which template should I use?**

A: 使用**通用模板**（`general.md`），或选择主要变更类型的模板。
A: Use the **General template** (`general.md`), or choose the template for the primary change type.

---

**Q: 自动建议的模板不合适怎么办？**
**Q: What if the automatically suggested template is not suitable?**

A: 你可以忽略建议，继续使用当前模板。自动建议仅供参考。
A: You can ignore the suggestion and continue using the current template. Auto-suggestions are for reference only.

---

**Q: 可以不使用任何模板吗？**
**Q: Can I not use any template?**

A: 不推荐。模板帮助确保PR包含必要信息，加快审查速度。
A: Not recommended. Templates help ensure PRs contain necessary information and speed up reviews.

---

**Q: 如何禁用自动模板建议？**
**Q: How to disable automatic template suggestions?**

A: 删除或禁用 `.github/workflows/pr-template-suggester.yml` 文件。
A: Delete or disable the `.github/workflows/pr-template-suggester.yml` file.

---

🌟 **感谢使用我们的PR模板系统！| Thank you for using our PR template system!**
