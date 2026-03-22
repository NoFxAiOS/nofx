<h1 align="center">NOFXi</h1>

<p align="center">
  <strong>Your AI Trading Agent.</strong><br/>
  <strong>Not a tool. A partner that trades with you.</strong>
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go" alt="Go"></a>
  <a href="https://reactjs.org/"><img src="https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react" alt="React"></a>
  <a href="https://x402.org"><img src="https://img.shields.io/badge/x402-USDC%20Payments-2775CA?style=flat" alt="x402"></a>
</p>

<p align="center">
  <a href="#english">English</a> ·
  <a href="#中文">中文</a>
</p>

---

<a id="english"></a>

## What is NOFXi?

NOFXi is an **AI Trading Agent** built on top of [NOFX](https://github.com/NoFxAiOS/nofx). While NOFX is an open-source AI trading engine, NOFXi elevates it into a fully autonomous agent that perceives, thinks, remembers, and acts.

**NOFX** = Engine. **NOFXi** = Intelligence.

### The Difference

| | NOFX (Engine) | NOFXi (Agent) |
|:--|:--|:--|
| **Interaction** | Configure → Start → Watch Dashboard | "Open a BTC long, 2x leverage, 3% stop loss" |
| **Intelligence** | AI makes trade decisions | AI understands context, learns from history |
| **Memory** | Stateless per session | Remembers your preferences, past trades, lessons |
| **Proactivity** | Executes when triggered | Monitors markets, alerts you, acts autonomously |
| **Communication** | Dashboard & logs | Natural language via Telegram, proactive notifications |

### Core Capabilities

- 🧠 **Agent Core** — Context-aware conversation, intent recognition, memory management
- 👁️ **Perception** — Market monitoring, anomaly detection, position tracking, sentiment analysis
- 💭 **Thinking** — Multi-model AI decisions, strategy matching, risk assessment
- 📝 **Memory** — Trade history, strategy performance, user preferences, market patterns
- ⚡ **Execution** — Multi-exchange trading, position management, x402 payments
- 💬 **Interaction** — Natural language trading, proactive alerts, report generation

### Architecture

```
Interaction Layer (Telegram / Web UI / API)
         │
    Agent Core (Conversation + Intent + Context)
         │
    ┌────┴───────────┬────────────┐
    │                │            │
Perception       Thinking      Memory
(Market Monitor) (AI Engine)   (Experience DB)
    │                │            │
    └────┬───────────┴────────────┘
         │
    Execution Layer (Multi-Exchange + x402 Payment)
```

### Built On

- **[NOFX](https://github.com/NoFxAiOS/nofx)** — Open-source AI trading engine (9 exchanges, 7+ AI models)
- **[Claw402](https://claw402.ai)** — x402 payment gateway (USDC micropayments)
- **[x402](https://x402.org)** — Pay-per-request with USDC, no API keys

---

<a id="中文"></a>

## NOFXi 是什么？

NOFXi 是基于 [NOFX](https://github.com/NoFxAiOS/nofx) 构建的 **AI 交易 Agent**。NOFX 是开源的 AI 交易引擎，而 NOFXi 将其升级为一个能感知、思考、记忆、行动的自主 Agent。

**NOFX** = 引擎。**NOFXi** = 智能。

### 核心区别

| | NOFX（引擎） | NOFXi（Agent） |
|:--|:--|:--|
| **交互方式** | 配置 → 启动 → 看仪表盘 | "帮我开 BTC 多单，2x 杠杆，3% 止损" |
| **智能程度** | AI 做交易决策 | AI 理解上下文，从历史中学习 |
| **记忆** | 每次无状态 | 记住你的偏好、交易历史、教训 |
| **主动性** | 被触发时执行 | 主动监控市场、通知你、自主行动 |
| **沟通方式** | 仪表盘和日志 | Telegram 自然语言对话、主动推送 |

### 核心能力

- 🧠 **Agent 核心** — 上下文对话、意图识别、记忆管理
- 👁️ **感知层** — 市场监控、异动检测、持仓监控、舆情分析
- 💭 **思考层** — 多模型 AI 决策、策略匹配、风险评估
- 📝 **记忆层** — 交易历史、策略效果、用户偏好、市场规律
- ⚡ **执行层** — 多交易所下单、仓位管理、x402 支付
- 💬 **交互层** — 自然语言交易、主动通知、报告生成

### 技术架构

```
交互层（Telegram / Web UI / API）
         │
    Agent Core（对话理解 + 意图识别 + 上下文管理）
         │
    ┌────┴───────────┬────────────┐
    │                │            │
  感知层           思考层        记忆层
（市场监控）    （AI 决策引擎）  （经验库）
    │                │            │
    └────┬───────────┴────────────┘
         │
    执行层（多交易所 + x402 支付）
```

### 构建基础

- **[NOFX](https://github.com/NoFxAiOS/nofx)** — 开源 AI 交易引擎（9 家交易所，7+ AI 模型）
- **[Claw402](https://claw402.ai)** — x402 支付网关（USDC 微支付）
- **[x402](https://x402.org)** — USDC 按量付费，无需 API Key

---

## Development

```bash
# Clone
git clone https://github.com/NoFxAiOS/nofxi.git
cd nofxi

# Build
go build ./...

# Run
go run cmd/nofxi/main.go
```

## License

Proprietary. All rights reserved.

© 2026 NoFxAiOS
