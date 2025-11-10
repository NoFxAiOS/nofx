# NOFX 部署文档

> **一键部署NOFX AI交易竞赛系统到云端** ☁️

---

## 📚 文档目录

1. [快速开始](#-快速开始) - 30分钟完成部署
2. [详细指南](./VERCEL_DEPLOYMENT_GUIDE.md) - 完整部署教程
3. [环境变量配置](#-环境变量配置)
4. [部署架构](#-部署架构)
5. [故障排除](#-故障排除)

---

## ⚡ 快速开始

### 前置要求

- ✅ GitHub账户
- ✅ Node.js ≥ 18
- ✅ Go ≥ 1.25

### 一键检查

运行部署检查脚本，确保环境准备就绪：

```bash
# 给脚本执行权限（仅首次需要）
chmod +x scripts/deploy-check.sh

# 运行检查
./scripts/deploy-check.sh
```

### 快速部署（3步）

```bash
# 1️⃣ 配置项目
cp config.json.example config.json
# 编辑 config.json，填入API密钥

# 2️⃣ 推送到GitHub
git init
git add .
git commit -m "init: nofx"
git remote add origin <your-repo-url>
git push -u origin main

# 3️⃣ 部署
# - 后端：railway.app（自动检测Go项目）
# - 前端：vercel.com（选择Vite框架，Root Directory设为 web/）
```

🎉 **部署完成！** 前端Vercel + 后端Railway，全球访问无压力！

---

## 📋 部署架构

```
┌──────────────────────┐
│   ┌──────────────┐   │
│   │  Vercel CDN  │   │  ← 全球CDN加速
│   └──────────────┘   │     静态资源 + 前端路由
│          │           │
│   ┌──────────────┐   │
│   │  React 18    │   │  ← 前端SPA应用
│   │  + Vite 6    │   │     TypeScript + TailwindCSS
│   └──────────────┘   │
└──────────┬───────────┘
           │ HTTPS
           ↓
┌──────────────────────┐
│   ┌──────────────┐   │
│   │  Railway     │   │  ← Go后端服务
│   └──────────────┘   │     Gin框架 + 实时WebSocket
│          │           │
│   ┌──────────────┐   │
│   │  Go 1.25     │   │  ← API服务
│   │  + Binance   │   │     交易API + AI模型
│   │  + Hyperliquid│   │
│   └──────────────┘   │
└──────────────────────┘
```

### 数据流

```
用户浏览器
    ↓ HTTPS
Vercel (React前端)
    ↓ API请求
Railway (Go后端)
    ↓ 交易API
Binance/Hyperliquid
    ↓ WebSocket
实时数据推送
    ↓
前端图表更新
```

---

## 🔧 环境变量配置

### 前端环境变量（Vercel）

在Vercel项目设置中添加：

| 变量名 | 描述 | 示例值 |
|--------|------|--------|
| `VITE_API_URL` | 后端API地址 | `https://xxx.railway.app` |
| `VITE_APP_TITLE` | 应用标题 | `NOFX AI交易竞赛平台` |
| `VITE_APP_VERSION` | 版本号 | `1.0.0` |

### 后端环境变量（Railway）

在Railway项目设置中添加：

| 变量名 | 描述 | 示例值 |
|--------|------|--------|
| `NOFX_BACKEND_PORT` | 后端端口 | `8080` |
| `NOFX_TIMEZONE` | 时区 | `Asia/Shanghai` |
| `BINANCE_API_KEY` | 币安API Key | `你的密钥` |
| `BINANCE_SECRET_KEY` | 币安Secret | `你的密钥` |
| `HYPERLIQUID_PRIVATE_KEY` | Hyperliquid私钥 | `你的密钥` |
| `DEEPSEEK_KEY` | DeepSeek API Key | `你的密钥` |

### 配置文件（config.json）

创建一个 `config.json` 文件，或在Railway中设置为环境变量 `CONFIG_FILE`：

```json
{
  "traders": [
    {
      "id": "hyperliquid_deepseek",
      "name": "Hyperliquid DeepSeek Trader",
      "enabled": true,
      "ai_model": "deepseek",
      "exchange": "hyperliquid",
      "hyperliquid_private_key": "your_key_here",
      "deepseek_key": "your_key_here",
      "initial_balance": 1000
    }
  ],
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0
}
```

---

## 🔐 安全最佳实践

### 1. API密钥管理

```bash
# ✅ 正确做法
- 使用环境变量存储API密钥
- 定期轮换密钥
- 限制API权限

# ❌ 错误做法
- 在代码中硬编码密钥
- 提交密钥到Git
- 使用权限过大的密钥
```

### 2. 访问控制

```bash
# 配置允许的域名（CORS）
"cors": {
  "allowed_origins": [
    "https://your-app.vercel.app",
    "http://localhost:3000"
  ]
}
```

### 3. 限流和监控

- ✅ 启用Railway监控
- ✅ 设置API限率
- ✅ 定期检查日志
- ✅ 配置告警通知

---

## 🚀 高级功能

### 自定义域名

**Vercel前端**：
```bash
# 1. Vercel项目 → Settings → Domains
# 2. 添加域名：nofx.yourdomain.com
# 3. 配置DNS CNAME记录指向Vercel
```

**Railway后端**：
```bash
# 1. Railway项目 → Settings → Domains
# 2. 添加域名：api.yourdomain.com
# 3. 配置DNS CNAME记录指向Railway
```

### 性能优化

**Vercel优化**：
- 启用图片优化
- 配置缓存策略
- 开启Gzip压缩

**Railway优化**：
- 选择合适实例大小
- 配置健康检查
- 设置自动扩容

### 监控告警

集成以下监控服务：

- **Sentry** - 错误追踪
- **LogRocket** - 用户行为分析
- **DataDog** - 应用性能监控
- **Pingdom** - 站点可用性监控

---

## 🐛 故障排除

### 前端问题

**页面空白**：
```bash
# 检查1：环境变量VITE_API_URL是否设置
# 检查2：后端是否正常响应 /health
# 检查3：浏览器控制台是否有错误
```

**API调用404**：
```bash
# 检查1：Vite代理配置
# 检查2：后端路由是否正确
# 检查3：CORS设置
```

**构建失败**：
```bash
# 本地测试构建
cd web
npm run build

# 查看详细错误
npm run build -- --debug
```

### 后端问题

**启动失败**：
```bash
# 检查1：环境变量是否配置
# 检查2：config.json格式是否正确
# 检查3：Go版本是否≥1.25
```

**API错误**：
```bash
# 测试API
curl https://your-app.railway.app/health

# 查看日志
# Railway项目 → Deploy → Logs
```

**交易失败**：
```bash
# 检查1：API密钥是否有效
# 检查2：余额是否充足
# 检查3：网络连接是否正常
```

### 通用问题

**部署失败**：
```bash
# 解决方案
1. 查看部署日志
2. 检查环境变量
3. 确认文件结构正确
4. 尝试重新部署
```

**性能问题**：
```bash
# 优化建议
1. 启用CDN
2. 配置缓存
3. 压缩静态资源
4. 减少API调用频率
```

---

## 📊 监控和日志

### 查看日志

**Vercel前端日志**：
```bash
# Vercel项目 → Functions → 选择函数 → 查看日志
```

**Railway后端日志**：
```bash
# Railway项目 → Deploy → 选择部署 → 查看日志
```

### 性能指标

监控以下关键指标：

- **响应时间** - API请求耗时
- **错误率** - 5xx错误占比
- **吞吐量** - QPS和并发数
- **可用性** - 99.9%+正常运行时间

### 告警设置

推荐设置告警：

- **错误率 > 5%** - 立即通知
- **响应时间 > 2s** - 性能告警
- **内存使用率 > 80%** - 资源告警
- **服务不可用** - 紧急通知

---

## 🔄 持续集成/持续部署（CI/CD）

### GitHub Actions自动部署

创建 `.github/workflows/deploy.yml`：

```yaml
name: Deploy NOFX

on:
  push:
    branches: [ main ]

jobs:
  deploy-railway:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Deploy to Railway
        uses: railway/deploy@main
        with:
          token: ${{ secrets.RAILWAY_TOKEN }}
          environment: production

  deploy-vercel:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Deploy to Vercel
        uses: amondnet/vercel-action@v20
        with:
          vercel-token: ${{ secrets.VERCEL_TOKEN }}
          vercel-args: '--prod'
```

### 自动测试

在部署前运行测试：

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-node@v2
        with:
          node-version: '18'
      - run: cd web && npm install && npm test

  test-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.25'
      - run: go test ./...
```

---

## 📞 支持和帮助

### 获取帮助

- 📖 **文档** - 查看详细的 [VERCEL_DEPLOYMENT_GUIDE.md](./VERCEL_DEPLOYMENT_GUIDE.md)
- 💬 **社区** - 加入我们的Discord服务器
- 🐛 **Bug报告** - 在GitHub创建Issue
- 📧 **邮件** - 发送邮件至 support@example.com

### 常见资源

- **Vercel文档**: [https://vercel.com/docs](https://vercel.com/docs)
- **Railway文档**: [https://docs.railway.app](https://docs.railway.app)
- **Go文档**: [https://golang.org/doc](https://golang.org/doc)
- **React文档**: [https://react.dev](https://react.dev)

### 反馈和建议

我们重视你的反馈！

- ⭐ 给项目点个Star
- 🐛 报告Bug和问题
- 💡 提出新功能建议
- 🤝 贡献代码

---

**© 2025 NOFX项目 | 祝部署顺利！ 🚀**

最后更新：2025-11-10