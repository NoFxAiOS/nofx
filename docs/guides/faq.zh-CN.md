# 常见问题（FAQ）

快速解答常见问题。详细故障排查请参考[故障排查指南](TROUBLESHOOTING.zh-CN.md)。

---

## 基础问题

### NOFX 是什么？
NOFX 是一个 AI 驱动的加密货币交易机器人，使用大语言模型（LLM）在期货市场进行交易决策。

### 支持哪些交易所？
- ✅ 币安合约（Binance Futures）
- ✅ Hyperliquid
- 🚧 更多交易所开发中

### NOFX 能盈利吗？
AI 交易是**实验性**的，**不保证盈利**。请始终用小额资金测试，不要投入超过您承受能力的资金。

### 可以同时运行多个交易员吗？
可以！NOFX 支持运行多个交易员，每个可配置不同的 AI 模型和交易策略。

---

## 安装与配置

### 系统要求是什么？
- **操作系统**：Linux、macOS 或 Windows（推荐 Docker）
- **内存**：最低 2GB，推荐 4GB
- **硬盘**：应用 + 日志需要 1GB
- **网络**：稳定的互联网连接

### 需要编程经验吗？
不需要！NOFX 有 Web 界面进行所有配置。但基础的命令行知识有助于安装和故障排查。

### 如何获取 API 密钥？
1. **币安**：账户 → API 管理 → 创建 API → 启用合约
2. **Hyperliquid**：访问 [Hyperliquid App](https://app.hyperliquid.xyz/) → API 设置

### 应该使用子账户吗？
**推荐**：是的，使用专门的子账户运行 NOFX 可以更好地隔离风险。但请注意，某些子账户有限制（例如币安子账户最高 5 倍杠杆）。

---

## 交易问题

### 为什么我的交易员不开仓？
常见原因：
- AI 根据市场情况决定"等待"
- 余额或保证金不足
- 达到持仓上限（默认最多 3 个仓位）
- 详细诊断请查看[故障排查指南](TROUBLESHOOTING.zh-CN.md#-ai-总是说等待持有)

### AI 多久做一次决策？
可配置！默认是每 **3-5 分钟**。太频繁 = 过度交易，太慢 = 错过机会。

### 可以自定义交易策略吗？
可以！您可以：
- 调整杠杆设置
- 修改币种选择池
- 更改决策间隔
- 自定义系统提示词（高级）

### 最多可以同时持有多少个仓位？
默认：**3 个仓位**。这是 AI 提示词中的软限制，不是硬编码。参见 `decision/engine.go:266`。

---

## 技术问题

### 币安持仓模式错误 (code=-4061)

**错误信息**：`Order's position side does not match user's setting`

**解决方法**：切换为**双向持仓**模式
1. 登录[币安合约](https://www.binance.com/zh-CN/futures/BTCUSDT)
2. 点击右上角 **⚙️ 偏好设置**
3. 选择 **持仓模式** → **双向持仓**
4. ⚠️ 先平掉所有持仓

**原因**：NOFX 使用 `PositionSide(LONG/SHORT)`，需要双向持仓模式。

参见 [Issue #202](https://github.com/tinkle-community/nofx/issues/202) 和[故障排查指南](TROUBLESHOOTING.zh-CN.md#-只开空单-issue-202)。

---

### 后端无法启动 / 端口被占用

**解决方法**：
```bash
# 查看占用端口的进程
lsof -i :8080

# 修改 .env 中的端口
NOFX_BACKEND_PORT=8081
```

---

### 前端一直显示"加载中..."

**快速检查**：
```bash
# 后端是否运行？
curl http://localhost:8080/api/health

# 应该返回：{"status":"ok"}
```

如果不是，查看[故障排查指南](TROUBLESHOOTING.zh-CN.md#-前端无法连接后端)。

---

### 数据库锁定错误

**解决方法**：
```bash
# 停止所有 NOFX 进程
docker compose down
# 或
pkill nofx

# 重启
docker compose up -d
```

---

## AI 与模型问题

### 支持哪些 AI 模型？
- **DeepSeek**（推荐性价比）
- **Qwen**（阿里云通义千问）
- **自定义 OpenAI 兼容 API**（可用于 OpenAI、通过代理的 Claude 或其他提供商）

### API 调用成本是多少？
取决于您的模型和决策频率：
- **DeepSeek**：每天约 $0.10-0.50（1 个交易员，5 分钟间隔）
- **Qwen**：每天约 $0.20-0.80
- **自定义 API**（例如 OpenAI GPT-4）：每天约 $2-5

*基于典型使用的估算。实际成本因提供商和使用量而异。*

### 可以使用多个 AI 模型吗？
可以！每个交易员可以使用不同的 AI 模型。您甚至可以 A/B 测试不同模型。

### AI 会从错误中学习吗？
会的，在一定程度上。NOFX 在每次决策提示中提供历史表现反馈，允许 AI 调整策略。

---

## 数据与隐私

### 我的数据存储在哪里？
所有数据都**本地存储**在 PostgreSQL（Docker 卷 `postgres_data`）中，另有：
- `decision_logs/` - AI 决策记录

### API 密钥安全吗？
API 密钥存储在本地数据库中。永远不要分享您的数据库或 `.env` 文件。我们建议使用带 IP 白名单限制的 API 密钥。

### 可以导出交易历史吗？
可以！使用 `pg_dump` 或 `psql` 导出数据：
```bash
docker compose exec postgres \
  psql -U nofx -d nofx -c "SELECT * FROM trades;"
```

---

## 故障排查

### 在哪里可以找到详细的故障排查？
查看全面的[故障排查指南](TROUBLESHOOTING.zh-CN.md)，包含：
- 分步诊断方法
- 日志收集方法
- 常见错误解决方案
- 紧急重置步骤

### 如何报告 Bug？
1. 先查看[故障排查指南](TROUBLESHOOTING.zh-CN.md)
2. 搜索[现有 Issues](https://github.com/tinkle-community/nofx/issues)
3. 如果没找到，使用我们的 [Bug 报告模板](../../.github/ISSUE_TEMPLATE/bug_report.md)

### 在哪里可以获得帮助？
- [GitHub Discussions](https://github.com/tinkle-community/nofx/discussions)
- [Telegram 社区](https://t.me/nofx_dev_community)
- [GitHub Issues](https://github.com/tinkle-community/nofx/issues)

---

## 贡献

### 可以为 NOFX 贡献代码吗？
可以！我们欢迎贡献：
- Bug 修复和新功能
- 文档改进
- 翻译
- 查看[贡献指南](../CONTRIBUTING.md)

### 如何建议新功能？
提交 [Feature Request](https://github.com/tinkle-community/nofx/issues/new/choose) 说明您的想法！

---

**最后更新：** 2025-11-02
