# Vercel 部署指南

## ✅ 已完成的工作

1. **前端代码构建成功**
   - 修复了编码问题
   - 安装了所有依赖包
   - 成功构建生产版本

2. **创建了GitHub Actions工作流**
   - 文件位置：`.github/workflows/vercel-deploy.yml`
   - 会在每次推送时自动部署到Vercel

3. **Vercel项目配置**
   - 项目ID: `prj_xMoVJ4AGtNNIiX6nN9uCgRop6KsP`
   - 组织ID: `team_CrV6muN0s3QNDJ3vrabttjLR`

## 🔑 需要设置的GitHub Secrets

为了让GitHub Actions自动部署到Vercel，需要在GitHub仓库设置中添加以下Secrets：

### 1. VERCEL_TOKEN
**获取方式：**
1. 访问 [Vercel Account Settings](https://vercel.com/account/tokens)
2. 点击 "Create Token"
3. 复制生成的Token

**设置位置：**
- GitHub仓库 → Settings → Secrets and variables → Actions → New repository secret
- Name: `VERCEL_TOKEN`
- Value: [粘贴刚才获取的Token]

### 2. VERCEL_ORG_ID
**值：** `team_CrV6muN0s3QNDJ3vrabttjLR`

**设置位置：**
- Name: `VERCEL_ORG_ID`
- Value: `team_CrV6muN0s3QNDJ3vrabttjLR`

### 3. VERCEL_PROJECT_ID
**值：** `prj_xMoVJ4AGtNNIiX6nN9uCgRop6KsP`

**设置位置：**
- Name: `VERCEL_PROJECT_ID`
- Value: `prj_xMoVJ4AGtNNIiX6nN9uCgRop6KsP`

### 4. VITE_API_URL (可选)
**推荐设置：**
```
https://your-backend-api-url.railway.app
```

**设置位置：**
- Name: `VITE_API_URL`
- Value: `你的后端API URL`

## 🚀 部署步骤

### 方法1：通过GitHub自动部署（推荐）

1. 在GitHub仓库中设置上述Secrets
2. 推送代码到main分支
3. GitHub Actions将自动：
   - 检出代码
   - 安装依赖
   - 构建项目
   - 部署到Vercel

### 方法2：手动通过Vercel CLI部署

如果您想立即部署且有Vercel CLI权限：

```bash
cd /Users/guoyingcheng/dreame/code/nofx/web
vercel --prod --token=$VERCEL_TOKEN
```

## 📝 验证部署

部署成功后，您将看到类似输出：

```
✅  Production: https://web-xxxxx.vercel.app [1m 23s]
📝  Deployed to production. Run `vercel --prod` to overwrite later.
💡  To change the domain, go to https://vercel.com/gyc567s-projects/web
```

## 🔍 访问应用

部署完成后，访问：
- **生产环境**: https://web-xxxxx.vercel.app

## 📚 参考文档

- [Vercel GitHub Action](https://github.com/amondnet/vercel-action)
- [GitHub Actions Secrets](https://docs.github.com/en/actions/security-guides/using-secrets-in-github-actions)
- [Vercel CLI](https://vercel.com/docs/cli)

## ⚠️ 注意事项

1. **环境变量**：确保在Vercel项目设置中也配置了相同的环境变量
2. **API URL**：前端需要正确的API后端URL才能正常工作
3. **域名**：默认部署到Vercel的随机域名，可以绑定自定义域名

---

## 快速设置脚本

如果您需要快速创建Secrets，可以使用以下命令生成说明：

```bash
echo "请在GitHub仓库设置中添加以下Secrets："
echo ""
echo "VERCEL_TOKEN: [从 https://vercel.com/account/tokens 获取]"
echo "VERCEL_ORG_ID: team_CrV6muN0s3QNDJ3vrabttjLR"
echo "VERCEL_PROJECT_ID: prj_xMoVJ4AGtNNIiX6nN9uCgRop6KsP"
echo "VITE_API_URL: https://your-backend-api-url.railway.app"
```
