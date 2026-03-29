<h1 align="center">NOFX</h1>

<p align="center">
  <strong>你的个人 AI 交易助手。</strong><br/>
  <strong>任何市场。任何模型。用 USDC 付费，无需 API Key。</strong>
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
  <a href="https://x402.org"><img src="https://img.shields.io/badge/x402-USDC%20Payments-2775CA?style=flat" alt="x402"></a>
  <a href="https://claw402.ai"><img src="https://img.shields.io/badge/Claw402-AI%20Gateway-FF6B35?style=flat" alt="Claw402"></a>
  <a href="https://blockrun.ai"><img src="https://img.shields.io/badge/BlockRun-x402%20Provider-8B5CF6?style=flat" alt="BlockRun"></a>
</p>

<p align="center">
  <a href="../../../README.md">English</a> ·
  <a href="README.md">中文</a> ·
  <a href="../ja/README.md">日本語</a> ·
  <a href="../ko/README.md">한국어</a> ·
  <a href="../ru/README.md">Русский</a> ·
  <a href="../uk/README.md">Українська</a> ·
  <a href="../vi/README.md">Tiếng Việt</a>
</p>

> **语言声明：** 本中文版本文档仅为方便海外华人社区阅读而提供，不代表本软件面向中国大陆、香港、澳门或台湾地区用户开放。如您位于上述地区，请勿使用本软件。

---

NOFX 是一个开源的**自主式** AI 交易助手。与需要手动配置模型、管理 API Key、接入数据源的传统 AI 工具不同 —— NOFX 的 AI **自主感知市场、自选模型、自动获取数据**。零人工干预。你只需设定策略，AI 负责一切。

**完全自主**：AI 自行决定使用哪个模型、获取什么市场数据、何时交易。无需手动配置模型，无需管理各种服务的 API Key。只需充值 USDC 钱包，一键启动。

核心差异：**内置 [x402](https://x402.org) 微支付协议**。无需 API Key，充值 USDC 钱包即可按需付费。钱包就是你的身份。

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

打开 **http://127.0.0.1:3000**，完成。

---

## x402 如何工作

传统流程：注册账号 → 购买额度 → 获取 API Key → 管理配额 → 轮换密钥。

x402 流程：

```
请求 → 402（返回价格）→ 钱包签名 USDC → 重试 → 完成
```

无需注册。无需 API Key。无需预付费。一个钱包，所有模型。

### 内置 x402 提供商

| 提供商 | 链 | 模型 |
|:---------|:------|:-------|
| <img src="../../../web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4、Claude Opus、DeepSeek、Qwen、Grok、Gemini、Kimi — 15+ 模型 |
| **[BlockRun](https://blockrun.ai)** | Base | 可配置 |
| **[BlockRun Sol](https://sol.blockrun.ai)** | Solana | 可配置 |

同时兼容 **[ClawRouter](https://github.com/BlockRunAI/ClawRouter)** —— 智能 LLM 路由，自动选择每次请求最便宜的模型（41+ 模型，节省 74-100%，<1ms 路由）。

---

## 功能概览

| 功能 | 描述 |
|:--------|:------------|
| **多 AI** | DeepSeek、Qwen、GPT、Claude、Gemini、Grok、Kimi、MiniMax — 随时切换 |
| **多交易所** | Binance、Bybit、OKX、Bitget、KuCoin、Gate、Hyperliquid、Aster、Lighter |
| **策略工作室** | 可视化构建器 — 币种来源、指标、风控 |
| **AI 竞赛** | AI 实时竞争，排行榜排名 |
| **Telegram Agent** | 与交易助手对话 — 流式输出、工具调用、记忆 |
| **回测实验室** | 历史模拟，权益曲线和性能指标 |
| **仪表板** | 实时持仓、盈亏、AI 决策日志与思维链 |

### 市场

加密货币 · 美股 · 外汇 · 贵金属

### 交易所 (CEX)

| 交易所 | 状态 | 注册 (手续费折扣) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [注册](https://www.binance.com/join?ref=NOFXENG) |
| <img src="../../../web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [注册](https://partner.bybit.com/b/83856) |
| <img src="../../../web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [注册](https://www.okx.com/join/1865360) |
| <img src="../../../web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [注册](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="../../../web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [注册](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="../../../web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [注册](https://www.gatenode.xyz/share/VQBGUAxY) |

### 交易所 (Perp-DEX)

| 交易所 | 状态 | 注册 (手续费折扣) |
|:---------|:------:|:------------------------|
| <img src="../../../web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [注册](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="../../../web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [注册](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="../../../web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [注册](https://app.lighter.xyz/?referral=68151432) |

### AI 模型 (API Key 模式)

| AI 模型 | 状态 | 获取 API Key |
|:---------|:------:|:------------|
| <img src="../../../web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [获取 API Key](https://platform.deepseek.com) |
| <img src="../../../web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **通义千问** | ✅ | [获取 API Key](https://dashscope.console.aliyun.com) |
| <img src="../../../web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [获取 API Key](https://platform.openai.com) |
| <img src="../../../web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [获取 API Key](https://console.anthropic.com) |
| <img src="../../../web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [获取 API Key](https://aistudio.google.com) |
| <img src="../../../web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [获取 API Key](https://console.x.ai) |
| <img src="../../../web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [获取 API Key](https://platform.moonshot.cn) |
| <img src="../../../web/public/icons/minimax.svg" width="20" height="20" style="vertical-align: middle;"/> **MiniMax** | ✅ | [获取 API Key](https://platform.minimax.io) |

### AI 模型 (x402 模式 — 无需 API Key)

15+ 模型通过 [Claw402](https://claw402.ai) 或 [BlockRun](https://blockrun.ai) 接入 — 只需一个 USDC 钱包

---

## 安装

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (云部署)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### Windows

安装 [Docker Desktop](https://www.docker.com/products/docker-desktop/)，然后：

```powershell
curl -o docker-compose.prod.yml https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### 从源码构建

```bash
# 前置条件: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib
# Ubuntu: sudo apt-get install libta-lib0-dev

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # 后端
cd web && npm install && npm run dev  # 前端 (新终端)
```

### 更新

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

---

## 配置

1. **AI** — 添加 API Key 或配置 x402 钱包
2. **交易所** — 连接交易所 API 凭证
3. **策略** — 在策略工作室构建
4. **交易员** — 组合 AI + 交易所 + 策略
5. **交易** — 从仪表板启动

所有操作通过 Web 界面完成：**http://127.0.0.1:3000**

---

## 文档

| | |
|:--|:--|
| [架构概览](../../architecture/README.md) | 系统设计和模块索引 |
| [策略模块](../../architecture/STRATEGY_MODULE.md) | 币种选择、AI 提示词、执行 |
| [回测模块](../../architecture/BACKTEST_MODULE.md) | 历史模拟、指标计算 |
| [常见问题](../../faq/README.md) | FAQ |
| [快速开始](../../getting-started/README.md) | 部署指南 |

---

## 贡献

查看 [贡献指南](../../../CONTRIBUTING.md) · [行为准则](../../../CODE_OF_CONDUCT.md) · [安全政策](../../../SECURITY.md)

### 贡献者空投计划

所有贡献在 GitHub 上追踪。当 NOFX 产生收入时，贡献者将获得空投。

**解决 [置顶 Issue](https://github.com/NoFxAiOS/nofx/issues) 的 PR 获得最高奖励！**

| 贡献类型 | 权重 |
|:-------------|:------:|
| 置顶 Issue PR | ★★★★★★ |
| 代码提交 (合并的 PR) | ★★★★★ |
| Bug 修复 | ★★★★ |
| 功能建议 | ★★★ |
| Bug 报告 | ★★ |
| 文档 | ★★ |

---

## 链接

| | |
|:--|:--|
| 官网 | [nofxai.com](https://nofxai.com) |
| 数据面板 | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API 文档 | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **风险提示**: AI 自动交易存在重大风险。建议仅用于学习/研究或小额测试。

---

## License

[AGPL-3.0](../../../LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
