<h1 align="center">NOFX — 开源 AI 交易操作系统</h1>

<p align="center">
  <strong>AI 驱动金融交易的基础设施层</strong>
</p>

<p align="center">
  <a href="https://github.com/NoFxAiOS/nofx/stargazers"><img src="https://img.shields.io/github/stars/NoFxAiOS/nofx?style=for-the-badge" alt="Stars"></a>
  <a href="https://github.com/NoFxAiOS/nofx/releases"><img src="https://img.shields.io/github/v/release/NoFxAiOS/nofx?style=for-the-badge" alt="Release"></a>
  <a href="https://github.com/NoFxAiOS/nofx/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg?style=for-the-badge" alt="License"></a>
  <a href="https://t.me/nofx_dev_community"><img src="https://img.shields.io/badge/Telegram-Community-blue?style=for-the-badge&logo=telegram" alt="Telegram"></a>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go"></a>
  <a href="https://reactjs.org/"><img src="https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react" alt="React"></a>
  <a href="https://www.typescriptlang.org/"><img src="https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript" alt="TypeScript"></a>
</p>

> **语言声明：** 本中文版本文档仅为方便海外华人社区阅读而提供，不代表本软件面向中国大陆、香港、澳门或台湾地区用户开放。如您位于上述地区，请勿使用本软件。

| 贡献者空投计划 |
|:----------------------------------:|
| 代码 · Bug修复 · Issue → 空投奖励 |
| [了解更多](#贡献者空投计划) |

**语言:** [English](../../../README.md) | [中文](README.md) | [日本語](../ja/README.md) | [한국어](../ko/README.md) | [Русский](../ru/README.md) | [Українська](../uk/README.md) | [Tiếng Việt](../vi/README.md)

---

### 核心功能

- **多 AI 支持**: 运行 DeepSeek、通义千问、GPT、Claude、Gemini、Grok、Kimi - 随时切换模型
- **多交易所**: 在 Binance、Bybit、OKX、Bitget、KuCoin、Gate、Hyperliquid、Aster DEX、Lighter 统一交易
- **策略工作室**: 可视化策略构建器，配置币种来源、指标和风控参数
- **AI 竞赛模式**: 多个 AI 交易员实时竞争，并排追踪表现
- **Web 配置**: 无需编辑 JSON - 通过 Web 界面完成所有配置
- **实时仪表板**: 实时持仓、盈亏追踪、AI 决策日志与思维链

### 核心团队

- **Tinkle** - [@Web3Tinkle](https://x.com/Web3Tinkle)
- **官方 Twitter** - [@nofx_official](https://x.com/nofx_official)

### 官方链接

- **官网**: [https://nofxai.com](https://nofxai.com)
- **数据站点**: [https://nofxos.ai/dashboard](https://nofxos.ai/dashboard)
- **API 文档**: [https://nofxos.ai/api-docs](https://nofxos.ai/api-docs)

> **风险提示**: 本系统为实验性质。AI 自动交易存在重大风险。强烈建议仅用于学习/研究目的或小额测试！

## 开发者社区

加入我们的 Telegram 开发者社区: **[NOFX 开发者社区](https://t.me/nofx_dev_community)**

---

## 开始之前

使用 NOFX 你需要准备:

1. **交易所账户** - 在任意支持的交易所注册并创建具有交易权限的 API 凭证
2. **AI 模型 API Key** - 从任意支持的提供商获取（推荐 DeepSeek，性价比最高）

---

## 支持的交易所

### CEX (中心化交易所)

| 交易所 | 状态 | 注册 (手续费折扣) |
|----------|--------|-------------------------|
| **Binance** | ✅ 已支持 | [注册](https://www.binance.com/join?ref=NOFXENG) |
| **Bybit** | ✅ 已支持 | [注册](https://partner.bybit.com/b/83856) |
| **OKX** | ✅ 已支持 | [注册](https://www.okx.com/join/1865360) |
| **Bitget** | ✅ 已支持 | [注册](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| **KuCoin** | ✅ 已支持 | [注册](https://www.kucoin.com/r/broker/CXEV7XKK) |
| **Gate** | ✅ 已支持 | [注册](https://www.gatenode.xyz/share/VQBGUAxY) |

### Perp-DEX (去中心化永续交易所)

| 交易所 | 状态 | 注册 (手续费折扣) |
|----------|--------|-------------------------|
| **Hyperliquid** | ✅ 已支持 | [注册](https://app.hyperliquid.xyz/join/AITRADING) |
| **Aster DEX** | ✅ 已支持 | [注册](https://www.asterdex.com/en/referral/fdfc0e) |
| **Lighter** | ✅ 已支持 | [注册](https://app.lighter.xyz/?referral=68151432) |

---

## 支持的 AI 模型

| AI 模型 | 状态 | 获取 API Key |
|----------|--------|-------------|
| **DeepSeek** | ✅ 已支持 | [获取 API Key](https://platform.deepseek.com) |
| **通义千问** | ✅ 已支持 | [获取 API Key](https://dashscope.console.aliyun.com) |
| **OpenAI (GPT)** | ✅ 已支持 | [获取 API Key](https://platform.openai.com) |
| **Claude** | ✅ 已支持 | [获取 API Key](https://console.anthropic.com) |
| **Gemini** | ✅ 已支持 | [获取 API Key](https://aistudio.google.com) |
| **Grok** | ✅ 已支持 | [获取 API Key](https://console.x.ai) |
| **Kimi** | ✅ 已支持 | [获取 API Key](https://platform.moonshot.cn) |

---

## 截图

### 竞赛模式 - 实时 AI 对战
![竞赛页面](../../../screenshots/competition-page.png)
*多 AI 排行榜，实时性能对比*

### 仪表板 - 市场图表视图
![仪表板市场图表](../../../screenshots/dashboard-market-chart.png)
*专业交易仪表板，TradingView 风格图表*

### 策略工作室
![策略工作室](../../../screenshots/strategy-studio.png)
*多数据源策略配置与 AI 测试*

---

## 快速开始

### 一键安装 (本地/服务器)

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

完成！打开浏览器访问 **http://127.0.0.1:3000**

### 一键云部署 (Railway)

一键部署到 Railway - 无需自己搭建服务器：

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

部署后，Railway 会提供一个公网 URL 访问你的 NOFX 实例。

### Docker Compose (手动)

```bash
# 下载并启动
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

访问 Web 界面: **http://127.0.0.1:3000**

```bash
# 管理命令
docker compose -f docker-compose.prod.yml logs -f    # 查看日志
docker compose -f docker-compose.prod.yml restart    # 重启
docker compose -f docker-compose.prod.yml down       # 停止
docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d  # 更新
```

### 保持更新

> **💡 更新频繁。** 每天运行以下命令以获取最新功能和修复：

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

此命令会拉取最新官方镜像并自动重启服务。

### 手动安装 (开发者)

#### 前置条件

- **Go 1.21+**
- **Node.js 18+**
- **TA-Lib** (技术指标库)

```bash
# 安装 TA-Lib
# macOS
brew install ta-lib

# Ubuntu/Debian
sudo apt-get install libta-lib0-dev
```

#### 安装步骤

```bash
# 1. 克隆仓库
git clone https://github.com/NoFxAiOS/nofx.git
cd nofx

# 2. 安装后端依赖
go mod download

# 3. 安装前端依赖
cd web
npm install
cd ..

# 4. 构建并启动后端
go build -o nofx
./nofx

# 5. 启动前端 (新终端)
cd web
npm run dev
```

访问 Web 界面: **http://127.0.0.1:3000**

---

## Windows 安装

### 方法一：Docker Desktop（推荐）

1. **安装 Docker Desktop**
   - 从 [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop/) 下载
   - 运行安装程序并重启电脑
   - 启动 Docker Desktop 并等待就绪

2. **运行 NOFX**
   ```powershell
   # 打开 PowerShell 运行：
   curl -o docker-compose.prod.yml https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
   docker compose -f docker-compose.prod.yml up -d
   ```

3. **访问**：在浏览器打开 **http://127.0.0.1:3000**

### 方法二：WSL2（适合开发）

1. **安装 WSL2**
   ```powershell
   # 以管理员身份打开 PowerShell
   wsl --install
   ```
   安装完成后重启电脑。

2. **从 Microsoft Store 安装 Ubuntu**
   - 打开 Microsoft Store
   - 搜索 "Ubuntu 22.04" 并安装
   - 启动 Ubuntu 并设置用户名/密码

3. **在 WSL2 中安装依赖**
   ```bash
   # 更新系统
   sudo apt update && sudo apt upgrade -y

   # 安装 Go
   wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   source ~/.bashrc

   # 安装 Node.js
   curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
   sudo apt-get install -y nodejs

   # 安装 TA-Lib
   sudo apt-get install -y libta-lib0-dev

   # 安装 Git
   sudo apt-get install -y git
   ```

4. **克隆并运行 NOFX**
   ```bash
   git clone https://github.com/NoFxAiOS/nofx.git
   cd nofx

   # 构建并运行后端
   go build -o nofx && ./nofx

   # 在另一个终端运行前端
   cd web && npm install && npm run dev
   ```

5. **访问**：在 Windows 浏览器打开 **http://127.0.0.1:3000**

### 方法三：WSL2 + Docker（两全其美）

1. **安装 Docker Desktop 并启用 WSL2 后端**
   - Docker Desktop 安装时勾选 "Use WSL 2 based engine"
   - 在 Docker Desktop 设置 → Resources → WSL Integration 中启用你的 Linux 发行版

2. **在 WSL2 终端运行**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
   ```

---

## 服务器部署

### 快速部署 (HTTP/IP 访问)

默认情况下，传输加密已**禁用**，可直接通过 IP 地址访问 NOFX：

```bash
# 部署到你的服务器
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

通过 `http://你的服务器IP:3000` 访问 - 立即可用。

### 增强安全 (HTTPS)

如需增强安全性，在 `.env` 中启用传输加密：

```bash
TRANSPORT_ENCRYPTION=true
```

启用后，浏览器会使用 Web Crypto API 在传输前加密 API 密钥。此功能需要：
- `https://` - 任何有 SSL 证书的域名
- `http://localhost` - 本地开发

### Cloudflare 快速配置 HTTPS

1. **添加域名到 Cloudflare** (免费计划即可)
   - 访问 [dash.cloudflare.com](https://dash.cloudflare.com)
   - 添加域名并更新 DNS 服务器

2. **创建 DNS 记录**
   - 类型: `A`
   - 名称: `nofx` (或你的子域名)
   - 内容: 你的服务器 IP
   - 代理状态: **已代理** (橙色云朵)

3. **配置 SSL/TLS**
   - 进入 SSL/TLS 设置
   - 加密模式选择 **灵活**

   ```
   用户 ──[HTTPS]──→ Cloudflare ──[HTTP]──→ 你的服务器:3000
   ```

4. **启用传输加密**
   ```bash
   # 编辑 .env 并设置
   TRANSPORT_ENCRYPTION=true
   ```

5. **完成！** 通过 `https://nofx.你的域名.com` 访问

---

## 初始配置 (Web 界面)

启动系统后，通过 Web 界面进行配置:

1. **配置 AI 模型** - 添加你的 AI API 密钥 (DeepSeek, OpenAI 等)
2. **配置交易所** - 设置交易所 API 凭证
3. **创建策略** - 在策略工作室配置交易策略
4. **创建交易员** - 组合 AI 模型 + 交易所 + 策略
5. **开始交易** - 启动你配置的交易员

所有配置都通过 Web 界面完成 - 无需编辑 JSON 文件。

---

## Web 界面功能

### 竞赛页面
- 实时 ROI 排行榜
- 多 AI 性能对比图表
- 实时盈亏追踪和排名

### 仪表板
- TradingView 风格 K 线图
- 实时持仓管理
- AI 决策日志与思维链推理
- 权益曲线追踪

### 策略工作室
- 币种来源配置 (静态列表、AI500 池、OI Top)
- 技术指标 (EMA, MACD, RSI, ATR, 成交量, OI, 资金费率)
- 风控设置 (杠杆、仓位限制、保证金使用率)
- AI 测试与实时提示词预览

---

## 常见问题

### TA-Lib 未找到
```bash
# macOS
brew install ta-lib

# Ubuntu
sudo apt-get install libta-lib0-dev
```

### AI API 超时
- 检查 API 密钥是否正确
- 检查网络连接
- 系统超时时间为 120 秒

### 前端无法连接后端
- 确保后端运行在 http://localhost:8080
- 检查端口是否被占用

---

## 文档

| 文档 | 描述 |
|------|------|
| **[架构概览](../../architecture/README.zh-CN.md)** | 系统设计和模块索引 |
| **[策略模块](../../architecture/STRATEGY_MODULE.md)** | 币种选择、数据组装、AI 提示词、执行 |
| **[回测模块](../../architecture/BACKTEST_MODULE.md)** | 历史模拟、指标计算、断点续测 |
| **[辩论模块](../../architecture/DEBATE_MODULE.md)** | 多 AI 辩论、投票共识、自动执行 |
| **[常见问题](../../faq/README.md)** | FAQ |
| **[快速开始](../../getting-started/README.zh-CN.md)** | 部署指南 |

---

## 许可证

本项目采用 **GNU Affero General Public License v3.0 (AGPL-3.0)** 许可 - 详见 [LICENSE](../../../LICENSE) 文件。

---

## 贡献

欢迎贡献！查看:
- **[贡献指南](../../../CONTRIBUTING.md)** - 开发流程和 PR 流程
- **[行为准则](../../../CODE_OF_CONDUCT.md)** - 社区准则
- **[安全政策](../../../SECURITY.md)** - 报告漏洞

---

## 贡献者空投计划

所有贡献都在 GitHub 上追踪。当 NOFX 产生收入时，贡献者将根据其贡献获得空投。

**解决 [置顶 Issue](https://github.com/NoFxAiOS/nofx/issues) 的 PR 获得最高奖励！**

| 贡献类型 | 权重 |
|------------------|:------:|
| **置顶 Issue PR** | ⭐⭐⭐⭐⭐⭐ |
| **代码提交** (合并的 PR) | ⭐⭐⭐⭐⭐ |
| **Bug 修复** | ⭐⭐⭐⭐ |
| **功能建议** | ⭐⭐⭐ |
| **Bug 报告** | ⭐⭐ |
| **文档** | ⭐⭐ |

---

## 联系方式

- **GitHub Issues**: [提交 Issue](https://github.com/NoFxAiOS/nofx/issues)
- **开发者社区**: [Telegram 群组](https://t.me/nofx_dev_community)

---

## Star 历史

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
