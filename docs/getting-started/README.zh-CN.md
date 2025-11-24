# 🚀 NOFX 快速开始

本节包含让 NOFX 运行起来所需的所有文档。

## 📋 部署选项

选择最适合您的方式：

### 🐳 Docker 部署（推荐）

**适合：** 初学者、快速部署、生产环境

- **中文文档：** [docker-deploy.zh-CN.md](docker-deploy.zh-CN.md)
- **English:** [docker-deploy.en.md](docker-deploy.en.md)

**优势：**
- ✅ 一键启动
- ✅ 包含所有依赖
- ✅ 易于更新和管理
- ✅ 隔离环境

**快速开始：**
```bash
cp config.json.example config.json
./scripts/start.sh start --build
```

---


## 🤖 AI 配置

### 自定义 AI 提供商

- **中文文档：** [custom-api.md](custom-api.md)
- **English:** [custom-api.en.md](custom-api.en.md)

使用自定义 AI 模型或第三方 OpenAI 兼容 API：
- 自定义 DeepSeek 端点
- 本地部署的模型
- 其他 LLM 提供商

---

## 🔑 环境要求

开始之前，请确保已安装：

### Docker 方式：
- ✅ Docker 20.10+
- ✅ Docker Compose V2

### 手动部署方式：
- ✅ Go 1.21+
- ✅ Node.js 18+
- ✅ TA-Lib 库

---

## 📚 下一步

部署完成后：

1. **配置 AI 模型** → 访问 Web 界面 http://localhost:3000
2. **设置交易所** → 添加 Binance/Hyperliquid 凭证
3. **创建交易员** → 将 AI 模型与交易所结合
4. **开始交易** → 在仪表板中监控表现

---

## ⚠️ 重要提示

**交易前：**
- ⚠️ 先在测试网测试
- ⚠️ 从小金额开始
- ⚠️ 了解风险
- ⚠️ 阅读[安全策略](../../SECURITY.md)

**API 密钥：**
- 🔑 永远不要提交 API 密钥到 git
- 🔑 使用环境变量
- 🔑 限制 IP 访问
- 🔑 在交易所启用 2FA

---

## 🆘 故障排除

**常见问题：**

1. **Docker 构建失败** → 检查 Docker 版本，更新到 20.10+
2. **找不到 TA-Lib** → `brew install ta-lib` (macOS) 或 `apt-get install libta-lib0-dev` (Ubuntu)
3. **端口 8080 被占用** → 在 .env 文件中更改 `API_PORT`
4. **前端无法连接** → 检查后端是否在端口 8080 上运行

**需要更多帮助？**
- 📖 [常见问题](../guides/faq.zh-CN.md)
- 💬 [Telegram 社区](https://t.me/nofx_dev_community)
- 🐛 [GitHub Issues](https://github.com/tinkle-community/nofx/issues)

---

[← 返回文档首页](../README.md)
