# 🎉 NOFX项目Vercel部署完整方案

> **Linus Torvalds出品 | 简洁高效的云端部署方案**

---

## 📖 项目概述

**NOFX** 是一个基于 **Go + React** 的AI自动交易竞赛系统，支持多交易所（Hyperliquid、Binance、Aster）和多AI模型。

**架构特点**：
- ⚡ Go 1.25 高性能后端（Gin框架）
- 🎨 React 18 + TypeScript 前端（Vite构建）
- 📊 实时数据可视化（Recharts）
- 🌍 国际化支持（4种语言）

---

## 📦 部署包内容

本次交付包含**11个核心文件**，总计 **~30KB** 的部署资料：

### 📄 文档类（3个）
1. **QUICK_START.md** - 30分钟快速部署指南 ⭐
2. **VERCEL_DEPLOYMENT_GUIDE.md** - 完整详细教程
3. **README_DEPLOYMENT.md** - 综合部署参考手册

### ⚙️ 配置类（6个）
4. **vercel.json** - Vercel部署配置
5. **railway.toml** - Railway后端配置
6. **web/vite.config.ts** - Vite构建优化
7. **web/src/lib/api.ts** - 动态API路由
8. **web/.env.example** - 前端环境变量模板
9. **.env.example** - 后端环境变量模板

### 🛠️ 工具类（2个）
10. **web/public/_redirects** - SPA路由支持
11. **scripts/deploy-check.sh** - 自动化检查脚本（可执行）

---

## 🚀 部署方案

### 推荐方案：Vercel + Railway

```
┌────────────────────┐
│    Vercel CDN      │  ← 前端React应用
│   (全球加速)        │     静态资源托管
└────────┬───────────┘
         │ HTTPS
         ↓ API调用
┌────────────────────┐
│    Railway         │  ← 后端Go API
│   (容器平台)        │     自动扩缩容
└────────────────────┘
```

**优势**：
- ✅ **零配置** - 自动检测项目类型
- ✅ **免费额度** - 足够个人项目使用
- ✅ **全球CDN** - 访问速度快
- ✅ **自动HTTPS** - 无需手动配置
- ✅ **一键部署** - 推送到GitHub自动触发

### 成本估算

| 平台 | 套餐 | 价格 | 额度 |
|------|------|------|------|
| **Vercel** | Hobby | 免费 | 100GB带宽/月 |
| **Railway** | Starter | $5/月 | $5信用额度/月 |

**总计：约$5/月**，适合个人和小团队项目。

---

## 📋 详细部署流程

### 阶段1️⃣：环境准备（5分钟）

```bash
# 1. 克隆项目
git clone <your-repo-url>
cd nofx

# 2. 复制配置文件
cp config.json.example config.json
cp .env.example .env
cp web/.env.example web/.env.local

# 3. 编辑配置文件（填入真实API密钥）
#    - config.json: 交易配置
#    - .env: 后端环境变量
#    - web/.env.local: 前端环境变量

# 4. 运行检查脚本
chmod +x scripts/deploy-check.sh
./scripts/deploy-check.sh
```

### 阶段2️⃣：后端部署 - Railway（10分钟）

1. **注册Railway账户**
   - 访问 [https://railway.app](https://railway.app)
   - 使用GitHub登录

2. **创建新项目**
   - 点击 "New Project"
   - 选择 "Deploy from GitHub repo"
   - 选择你的NOFX仓库

3. **配置环境变量**
   - 在项目设置中添加：
     ```
     NOFX_BACKEND_PORT=8080
     NOFX_TIMEZONE=Asia/Shanghai
     MAX_DAILY_LOSS=10.0
     MAX_DRAWDOWN=20.0
     ```

4. **上传config.json**
   - 方式1：在Railway设置中添加 `CONFIG_FILE` 环境变量，值为完整config.json内容
   - 方式2：将config.json推送到GitHub根目录

5. **等待构建完成**
   - Railway自动检测Go项目
   - 构建时间约3-5分钟
   - 完成后记录URL：`https://xxxx.railway.app`

### 阶段3️⃣：前端部署 - Vercel（10分钟）

1. **注册Vercel账户**
   - 访问 [https://vercel.com](https://vercel.com)
   - 使用GitHub登录

2. **导入GitHub项目**
   - 点击 "New Project"
   - 选择你的NOFX仓库

3. **配置构建设置**
   - Framework Preset: `Vite`
   - Root Directory: `web` ⭐ 重要
   - Build Command: `npm run build`
   - Output Directory: `dist`

4. **添加环境变量**
   ```
   VITE_API_URL=https://xxxx.railway.app
   VITE_APP_TITLE=NOFX AI交易竞赛平台
   VITE_APP_VERSION=1.0.0
   ```

5. **部署**
   - 点击 "Deploy" 按钮
   - 构建时间约2-3分钟
   - 完成后获得URL：`https://xxxx.vercel.app`

### 阶段4️⃣：联调测试（5分钟）

1. **测试后端API**
   ```bash
   curl https://xxxx.railway.app/health
   # 应返回: {"status":"ok"}
   ```

2. **测试前端应用**
   - 打开 `https://xxxx.vercel.app`
   - 检查页面是否正常加载
   - 打开浏览器控制台（F12）
   - 确认无错误信息

3. **验证数据流**
   - 前端应该能成功调用后端API
   - 图表应该正常渲染
   - 交易数据应该正常显示

---

## 🔧 环境变量详解

### 前端环境变量（Vercel）

| 变量名 | 描述 | 示例值 | 必填 |
|--------|------|--------|------|
| `VITE_API_URL` | 后端API地址 | `https://xxx.railway.app` | ✅ |
| `VITE_APP_TITLE` | 应用标题 | `NOFX AI交易竞赛平台` | ❌ |
| `VITE_APP_VERSION` | 版本号 | `1.0.0` | ❌ |

### 后端环境变量（Railway）

| 变量名 | 描述 | 示例值 | 必填 |
|--------|------|--------|------|
| `NOFX_BACKEND_PORT` | 后端端口 | `8080` | ✅ |
| `NOFX_TIMEZONE` | 时区设置 | `Asia/Shanghai` | ✅ |
| `BINANCE_API_KEY` | 币安API Key | `你的密钥` | ❌ |
| `BINANCE_SECRET_KEY` | 币安Secret | `你的密钥` | ❌ |
| `HYPERLIQUID_PRIVATE_KEY` | Hyperliquid私钥 | `你的密钥` | ❌ |
| `DEEPSEEK_KEY` | DeepSeek API Key | `你的密钥` | ❌ |
| `MAX_DAILY_LOSS` | 最大日亏损 | `10.0` | ✅ |
| `MAX_DRAWDOWN` | 最大回撤 | `20.0` | ✅ |

### 配置文件（config.json）

```json
{
  "traders": [
    {
      "id": "hyperliquid_deepseek",
      "name": "Hyperliquid DeepSeek Trader",
      "enabled": true,
      "ai_model": "deepseek",
      "exchange": "hyperliquid",
      "hyperliquid_private_key": "your_private_key_here",
      "deepseek_key": "your_deepseek_key_here",
      "initial_balance": 1000
    }
  ],
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "api_server_port": 8080,
  "max_daily_loss": 10.0,
  "max_drawdown": 20.0,
  "cors": {
    "allowed_origins": [
      "https://your-app.vercel.app"
    ]
  }
}
```

---

## 🛡️ 安全最佳实践

### 1. API密钥管理

```bash
# ✅ 推荐做法
- 使用部署平台的环境变量功能
- 定期轮换API密钥（每3个月）
- 限制API权限（只启用必要功能）
- 监控API使用情况

# ❌ 避免做法
- 在代码中硬编码密钥
- 提交密钥到Git仓库
- 使用权限过大的密钥
- 在聊天工具中分享密钥
```

### 2. 访问控制

```json
// CORS配置 - 限制允许的域名
"cors": {
  "allowed_origins": [
    "https://your-app.vercel.app",
    "http://localhost:3000"
  ]
}
```

### 3. 监控和告警

**推荐监控指标**：
- API响应时间（应 < 500ms）
- 错误率（应 < 1%）
- 内存使用率（应 < 80%）
- CPU使用率（应 < 70%）

**设置告警**：
- 错误率 > 5% - 立即通知
- 响应时间 > 2s - 性能告警
- 服务不可用 - 紧急通知

---

## 🎯 功能验证清单

部署完成后，请逐项验证：

### ✅ 前端功能
- [ ] 页面正常加载
- [ ] 导航菜单工作正常
- [ ] 图表正常渲染
- [ ] 多语言切换正常
- [ ] 移动端适配良好
- [ ] 无控制台错误

### ✅ 后端功能
- [ ] `/health` 端点返回正常
- [ ] `/api/competition` 返回竞赛数据
- [ ] `/api/traders` 返回交易员列表
- [ ] WebSocket连接正常
- [ ] 日志输出正常

### ✅ 整体测试
- [ ] 前端能成功调用后端API
- [ ] 数据实时更新
- [ ] 页面刷新后状态保持
- [ ] 错误处理机制正常
- [ ] 性能表现良好（加载 < 3s）

---

## 🐛 常见问题与解决方案

### Q1: 页面显示空白

**原因**：前端构建失败或环境变量未设置

**解决方案**：
```bash
# 1. 检查Vercel构建日志
# 2. 确认 VITE_API_URL 环境变量已设置
# 3. 本地测试构建
cd web && npm run build
```

### Q2: API调用返回404

**原因**：代理配置错误或后端未启动

**解决方案**：
```bash
# 1. 检查后端是否正常
curl https://xxxx.railway.app/health

# 2. 确认 VITE_API_URL 格式正确
# 3. 检查 vercel.json 配置
```

### Q3: CORS跨域错误

**原因**：后端CORS配置未包含前端域名

**解决方案**：
在config.json中添加：
```json
"cors": {
  "allowed_origins": [
    "https://your-app.vercel.app"
  ]
}
```

### Q4: 构建失败

**前端失败**：
```bash
cd web
npm install
npm run build  # 查看错误信息
```

**后端失败**：
```bash
go mod tidy
go build -o nofx .  # 查看错误信息
```

### Q5: 交易功能异常

**检查清单**：
- [ ] API密钥是否有效
- [ ] 余额是否充足
- [ ] 网络连接是否正常
- [ ] 交易所API状态是否正常

---

## 📊 性能优化建议

### 前端优化（Vercel）

1. **启用图片优化**
   ```json
   // vercel.json
   {
     "images": {
       "domains": ["your-cdn.com"]
     }
   }
   ```

2. **配置缓存策略**
   - 静态资源：1年缓存
   - HTML：1小时缓存
   - API响应：不缓存

3. **代码分割**
   - Vite已配置按需加载
   - 图表库独立chunk
   - 工具库独立chunk

### 后端优化（Railway）

1. **选择合适实例**
   - 个人项目：Starter
   - 小团队：Professional
   - 企业级：Team

2. **配置健康检查**
   ```toml
   [healthcheck]
   path = "/health"
   interval = 30
   ```

3. **启用自动扩缩容**
   - Railway Pro套餐支持
   - 根据负载自动调整实例

---

## 🔄 持续集成/持续部署

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
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - run: |
          cd web && npm install && npm test
          go test ./...
```

---

## 📈 监控和分析

### 推荐监控工具

1. **Sentry** - 错误追踪
   - 自动收集前端和后端错误
   - 提供详细错误堆栈

2. **DataDog** - 应用性能监控
   - 监控API响应时间
   - 跟踪数据库性能

3. **LogRocket** - 用户行为分析
   - 录制用户会话
   - 重现用户问题

4. **Pingdom** - 站点可用性
   - 定期检查网站可访问性
   - 响应时间监控

### 关键指标仪表盘

创建监控仪表盘，关注以下指标：

- **响应时间**：API请求耗时（目标 < 500ms）
- **错误率**：5xx错误占比（目标 < 1%）
- **吞吐量**：QPS和并发数
- **可用性**：正常运行时间（目标 > 99.9%）
- **用户活跃度**：DAU/MAU
- **交易成功率**：API调用成功率

---

## 🎓 进阶功能

### 自定义域名

**Vercel前端**：
1. Vercel项目 → Settings → Domains
2. 添加域名：`nofx.yourdomain.com`
3. 配置DNS CNAME记录指向Vercel

**Railway后端**：
1. Railway项目 → Settings → Domains
2. 添加域名：`api.yourdomain.com`
3. 配置DNS CNAME记录指向Railway

### HTTPS证书

**好消息**：Vercel和Railway都**自动提供Let's Encrypt证书**，无需手动配置！

### CDN加速

**Vercel Edge Network**：
- 全球200+边缘节点
- 智能路由选择
- 自动压缩和优化

**Railway CDN**：
- 支持自定义CDN
- 集成Cloudflare

### 数据库集成

如需持久化存储数据，推荐：

1. **PostgreSQL** - 关系型数据库
   - Railway内置支持
   - 或使用Supabase/PlanetScale

2. **Redis** - 缓存和会话存储
   - Railway Redis插件
   - 或使用Upstash

3. **InfluxDB** - 时间序列数据
   - 适合存储交易历史
   - 完美配合Grafana

---

## 💰 成本优化

### 减少费用技巧

1. **利用免费额度**
   - Vercel Hobby套餐：100GB/月
   - Railway Starter：$5信用额度
   - 合理使用足够个人项目

2. **优化带宽**
   - 压缩图片和静态资源
   - 启用Gzip/Brotli压缩
   - 使用WebP图片格式

3. **减少请求**
   - 启用客户端缓存
   - 合并API请求
   - 使用GraphQL（可选）

4. **按需付费**
   - Railway Pro按实例小时计费
   - 关闭空闲实例（Railway Pro功能）

### 升级建议

**当流量增长时**：
- 从Hobby升级到Pro（$20/月）
- 启用Vercel Analytics
- 添加CDN（如Cloudflare）

**当用户增长时**：
- Railway从Starter升级到Professional
- 配置多实例部署
- 添加负载均衡器

---

## 📞 获取帮助

### 官方资源

- **Vercel文档**: [https://vercel.com/docs](https://vercel.com/docs)
- **Railway文档**: [https://docs.railway.app](https://docs.railway.app)
- **Go文档**: [https://golang.org/doc](https://golang.org/doc)
- **React文档**: [https://react.dev](https://react.dev)

### 社区支持

- **Vercel Discord**: [https://vercel.com/discord](https://vercel.com/discord)
- **Railway Discord**: [https://railway.app/discord](https://railway.app/discord)
- **Go中文社区**: [https://studygolang.com](https://studygolang.com)
- **React中文社区**: [https://react.docschina.org](https://react.docschina.org)

### 技术支持

- **邮件**: support@example.com
- **GitHub Issues**: 在项目仓库提交Issue
- **Stack Overflow**: 搜索相关问题标签

---

## 📚 进一步学习

### 部署相关

- [《Docker容器化实战》](https://example.com/docker)
- [《Kubernetes云原生架构》](https://example.com/kubernetes)
- [《GitHub Actions CI/CD》](https://example.com/github-actions)

### Go后端

- [《Go Web编程》](https://example.com/go-web)
- [《Gin框架实战》](https://example.com/gin)
- [《Go性能优化》](https://example.com/go-performance)

### React前端

- [《React最佳实践》](https://example.com/react-best)
- [《TypeScript实战》](https://example.com/typescript)
- [《性能优化指南》](https://example.com/frontend-performance)

---

## 🏆 致谢

感谢Linus Torvalds的"好品味"哲学指导本次部署方案设计：

> "简单就是终极的复杂"
> 
> "好程序员编写简单代码，伟大的程序员编写可读代码"

本部署方案遵循以下原则：
- ✅ **简单性** - 最小化配置，最大化效果
- ✅ **可维护性** - 清晰的文档和自动化工具
- ✅ **可扩展性** - 易于升级和扩展
- ✅ **可观测性** - 完整的监控和日志

---

## 📄 许可证

本部署方案遵循项目原有许可证。

---

## 🔔 更新日志

### v1.0.0 (2025-11-10)
- ✅ 初始版本发布
- ✅ 支持Vercel + Railway部署
- ✅ 完整的文档和工具
- ✅ 自动化检查脚本
- ✅ 环境变量模板

---

**© 2025 NOFX项目 | 祝部署顺利！ 🚀**

**"Talk is cheap. Show me the code." - Linus Torvalds**

