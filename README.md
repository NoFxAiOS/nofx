<h1 align="center">NOFX</h1>

<p align="center">
  <strong>Your personal AI trading assistant.</strong><br/>
  <strong>Any market. Any model. Pay with USDC, not API keys.</strong>
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
  <a href="README.md">English</a> ·
  <a href="docs/i18n/zh-CN/README.md">中文</a> ·
  <a href="docs/i18n/ja/README.md">日本語</a> ·
  <a href="docs/i18n/ko/README.md">한국어</a> ·
  <a href="docs/i18n/ru/README.md">Русский</a> ·
  <a href="docs/i18n/uk/README.md">Українська</a> ·
  <a href="docs/i18n/vi/README.md">Tiếng Việt</a>
</p>

---

NOFX is an open-source **autonomous** AI trading assistant. Unlike traditional AI tools that require you to manually configure models, manage API keys, and wire up data sources — NOFX's AI **perceives markets, selects models, and fetches data entirely on its own**. Zero human intervention. You set the strategy, the AI handles everything else.

**Fully autonomous**: The AI decides which model to use, what market data to pull, when to trade — all by itself. No manual model configuration. No juggling API keys for different services. Just fund a USDC wallet and let it run.

What makes it different: **built-in [x402](https://x402.org) micropayments**. No API keys. Fund a USDC wallet and pay per request. Your wallet is your identity.

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

Open **http://127.0.0.1:3000**. Done.

---

## How x402 Works

Traditional flow: register account → buy credits → get API key → manage quota → rotate keys.

x402 flow:

```
Request → 402 (here's the price) → wallet signs USDC → retry → done
```

No accounts. No API keys. No prepaid credits. One wallet, every model.

### Built-in x402 Providers

| Provider | Chain | Models |
|:---------|:------|:-------|
| <img src="web/public/icons/claw402.png" width="20" height="20" style="vertical-align: middle;"/> **[Claw402](https://claw402.ai)** | Base | GPT-5.4, Claude Opus, DeepSeek, Qwen, Grok, Gemini, Kimi — 15+ models |
| **[BlockRun](https://blockrun.ai)** | Base | Configurable |
| **[BlockRun Sol](https://sol.blockrun.ai)** | Solana | Configurable |

Also compatible with **[ClawRouter](https://github.com/BlockRunAI/ClawRouter)** — an intelligent LLM router that picks the cheapest capable model per request (41+ models, 74-100% savings, <1ms routing).

---

## What It Does

| Feature | Description |
|:--------|:------------|
| **Multi-AI** | DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi, MiniMax — switch anytime |
| **Multi-Exchange** | Binance, Bybit, OKX, Bitget, KuCoin, Gate, Hyperliquid, Aster, Lighter |
| **Strategy Studio** | Visual builder — coin sources, indicators, risk controls |
| **AI Competition** | AIs compete in real-time, leaderboard ranks performance |
| **Telegram Agent** | Chat with your trading assistant — streaming, tool calling, memory |
| **Backtest Lab** | Historical simulation with equity curves and performance metrics |
| **Dashboard** | Live positions, P/L, AI decision logs with Chain of Thought |

### Markets

Crypto · US Stocks · Forex · Metals

### Exchanges (CEX)

| Exchange | Status | Register (Fee Discount) |
|:---------|:------:|:------------------------|
| <img src="web/public/exchange-icons/binance.jpg" width="20" height="20" style="vertical-align: middle;"/> **Binance** | ✅ | [Register](https://www.binance.com/join?ref=NOFXENG) |
| <img src="web/public/exchange-icons/bybit.png" width="20" height="20" style="vertical-align: middle;"/> **Bybit** | ✅ | [Register](https://partner.bybit.com/b/83856) |
| <img src="web/public/exchange-icons/okx.svg" width="20" height="20" style="vertical-align: middle;"/> **OKX** | ✅ | [Register](https://www.okx.com/join/1865360) |
| <img src="web/public/exchange-icons/bitget.svg" width="20" height="20" style="vertical-align: middle;"/> **Bitget** | ✅ | [Register](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |
| <img src="web/public/exchange-icons/kucoin.svg" width="20" height="20" style="vertical-align: middle;"/> **KuCoin** | ✅ | [Register](https://www.kucoin.com/r/broker/CXEV7XKK) |
| <img src="web/public/exchange-icons/gate.svg" width="20" height="20" style="vertical-align: middle;"/> **Gate** | ✅ | [Register](https://www.gatenode.xyz/share/VQBGUAxY) |

### Exchanges (Perp-DEX)

| Exchange | Status | Register (Fee Discount) |
|:---------|:------:|:------------------------|
| <img src="web/public/exchange-icons/hyperliquid.png" width="20" height="20" style="vertical-align: middle;"/> **Hyperliquid** | ✅ | [Register](https://app.hyperliquid.xyz/join/AITRADING) |
| <img src="web/public/exchange-icons/aster.svg" width="20" height="20" style="vertical-align: middle;"/> **Aster DEX** | ✅ | [Register](https://www.asterdex.com/en/referral/fdfc0e) |
| <img src="web/public/exchange-icons/lighter.png" width="20" height="20" style="vertical-align: middle;"/> **Lighter** | ✅ | [Register](https://app.lighter.xyz/?referral=68151432) |

### AI Models (API Key Mode)

| AI Model | Status | Get API Key |
|:---------|:------:|:------------|
| <img src="web/public/icons/deepseek.svg" width="20" height="20" style="vertical-align: middle;"/> **DeepSeek** | ✅ | [Get API Key](https://platform.deepseek.com) |
| <img src="web/public/icons/qwen.svg" width="20" height="20" style="vertical-align: middle;"/> **Qwen** | ✅ | [Get API Key](https://dashscope.console.aliyun.com) |
| <img src="web/public/icons/openai.svg" width="20" height="20" style="vertical-align: middle;"/> **OpenAI (GPT)** | ✅ | [Get API Key](https://platform.openai.com) |
| <img src="web/public/icons/claude.svg" width="20" height="20" style="vertical-align: middle;"/> **Claude** | ✅ | [Get API Key](https://console.anthropic.com) |
| <img src="web/public/icons/gemini.svg" width="20" height="20" style="vertical-align: middle;"/> **Gemini** | ✅ | [Get API Key](https://aistudio.google.com) |
| <img src="web/public/icons/grok.svg" width="20" height="20" style="vertical-align: middle;"/> **Grok** | ✅ | [Get API Key](https://console.x.ai) |
| <img src="web/public/icons/kimi.svg" width="20" height="20" style="vertical-align: middle;"/> **Kimi** | ✅ | [Get API Key](https://platform.moonshot.cn) |
| <img src="web/public/icons/minimax.svg" width="20" height="20" style="vertical-align: middle;"/> **MiniMax** | ✅ | [Get API Key](https://platform.minimax.io) |

### AI Models (x402 Mode — No API Key)

15+ models via [Claw402](https://claw402.ai) or [BlockRun](https://blockrun.ai) — just a USDC wallet

---

## Screenshots

<details>
<summary><b>Config Page</b></summary>

| AI Models & Exchanges | Traders List |
|:---:|:---:|
| <img src="screenshots/config-ai-exchanges.png" width="400"/> | <img src="screenshots/config-traders-list.png" width="400"/> |
</details>

<details>
<summary><b>Dashboard</b></summary>

| Overview | Market Chart |
|:---:|:---:|
| <img src="screenshots/dashboard-page.png" width="400"/> | <img src="screenshots/dashboard-market-chart.png" width="400"/> |

| Trading Stats | Position History |
|:---:|:---:|
| <img src="screenshots/dashboard-trading-stats.png" width="400"/> | <img src="screenshots/dashboard-position-history.png" width="400"/> |

| Positions | Trader Details |
|:---:|:---:|
| <img src="screenshots/dashboard-positions.png" width="400"/> | <img src="screenshots/details-page.png" width="400"/> |
</details>

<details>
<summary><b>Strategy Studio</b></summary>

| Strategy Editor | Indicators Config |
|:---:|:---:|
| <img src="screenshots/strategy-studio.png" width="400"/> | <img src="screenshots/strategy-indicators.png" width="400"/> |
</details>

<details>
<summary><b>Competition & Backtest</b></summary>

| Competition Mode | Backtest Lab |
|:---:|:---:|
| <img src="screenshots/competition-page.png" width="400"/> | <img src="screenshots/backtest-lab.png" width="400"/> |
</details>

---

## Install

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

### Railway (Cloud)

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/nofx?referralCode=nofx)

### Docker

```bash
curl -O https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### Windows

Install [Docker Desktop](https://www.docker.com/products/docker-desktop/), then:

```powershell
curl -o docker-compose.prod.yml https://raw.githubusercontent.com/NoFxAiOS/nofx/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

### From Source

```bash
# Prerequisites: Go 1.21+, Node.js 18+, TA-Lib
# macOS: brew install ta-lib
# Ubuntu: sudo apt-get install libta-lib0-dev

git clone https://github.com/NoFxAiOS/nofx.git && cd nofx
go build -o nofx && ./nofx          # backend
cd web && npm install && npm run dev  # frontend (new terminal)
```

### Update

```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
```

---

## Setup

1. **AI** — Add API keys or configure x402 wallet
2. **Exchange** — Connect exchange API credentials
3. **Strategy** — Build in Strategy Studio
4. **Trader** — Combine AI + Exchange + Strategy
5. **Trade** — Launch from the dashboard

Everything through the web UI at **http://127.0.0.1:3000**.

---

## Deploy to Server

**HTTP (quick):**
```bash
curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash
# Access via http://YOUR_IP:3000
```

**HTTPS (Cloudflare):**
1. Add domain to [Cloudflare](https://dash.cloudflare.com) (free plan)
2. A record → your server IP (Proxied)
3. SSL/TLS → Flexible
4. Set `TRANSPORT_ENCRYPTION=true` in `.env`

---

## Architecture

```
                              NOFX
    ┌─────────────────────────────────────────────────┐
    │                 Web Dashboard                     │
    │           React + TypeScript + TradingView        │
    ├─────────────────────────────────────────────────┤
    │                  API Server (Go)                  │
    ├──────────┬──────────┬──────────┬────────────────┤
    │ Strategy │ Backtest │    Telegram     │
    │  Engine  │   Lab    │     Agent       │
    ├──────────┴──────────┴──────────┴────────────────┤
    │               MCP AI Client Layer                │
    │    ┌───────────┐  ┌───────────┐  ┌───────────┐  │
    │    │  API Key   │  │   x402    │  │ ClawRouter│  │
    │    │ DeepSeek   │  │ Claw402   │  │ 41+ models│  │
    │    │ GPT,Claude │  │ BlockRun  │  │ auto-route│  │
    │    └───────────┘  └───────────┘  └───────────┘  │
    ├─────────────────────────────────────────────────┤
    │             Exchange Connectors                   │
    │  Binance · Bybit · OKX · Bitget · KuCoin · Gate  │
    │      Hyperliquid · Aster DEX · Lighter            │
    └─────────────────────────────────────────────────┘
```

---

## Docs

| | |
|:--|:--|
| [Architecture](docs/architecture/README.md) | System design and module index |
| [Strategy Module](docs/architecture/STRATEGY_MODULE.md) | Coin selection, AI prompts, execution |
| [Backtest Module](docs/architecture/BACKTEST_MODULE.md) | Historical simulation, metrics |
| [FAQ](docs/faq/README.md) | Common questions |
| [Getting Started](docs/getting-started/README.md) | Deployment guide |

---

## Contributing

See [Contributing Guide](CONTRIBUTING.md) · [Code of Conduct](CODE_OF_CONDUCT.md) · [Security Policy](SECURITY.md)

### Contributor Airdrop Program

All contributions are tracked. When NOFX generates revenue, contributors receive airdrops.

**[Pinned Issues](https://github.com/NoFxAiOS/nofx/issues) get the highest rewards.**

| Contribution | Weight |
|:-------------|:------:|
| Pinned Issue PRs | ★★★★★★ |
| Code (Merged PRs) | ★★★★★ |
| Bug Fixes | ★★★★ |
| Feature Ideas | ★★★ |
| Bug Reports | ★★ |
| Documentation | ★★ |

---

## Links

| | |
|:--|:--|
| Website | [nofxai.com](https://nofxai.com) |
| Dashboard | [nofxos.ai/dashboard](https://nofxos.ai/dashboard) |
| API Docs | [nofxos.ai/api-docs](https://nofxos.ai/api-docs) |
| Telegram | [nofx_dev_community](https://t.me/nofx_dev_community) |
| Twitter | [@nofx_official](https://x.com/nofx_official) |

> **Risk Warning**: AI auto-trading carries significant risks. Recommended for learning/research or small amounts only.

---

## Sponsors

<a href="https://github.com/pjl914335852-ux"><img src="https://github.com/pjl914335852-ux.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cat9999aaa"><img src="https://github.com/cat9999aaa.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/1733055465"><img src="https://github.com/1733055465.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/kolal2020"><img src="https://github.com/kolal2020.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/CyberFFarm"><img src="https://github.com/CyberFFarm.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/vip3001003"><img src="https://github.com/vip3001003.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/mrtluh"><img src="https://github.com/mrtluh.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/cpcp1117-source"><img src="https://github.com/cpcp1117-source.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/match-007"><img src="https://github.com/match-007.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/leiwuhen1715"><img src="https://github.com/leiwuhen1715.png" width="50" height="50" style="border-radius:50%"/></a>
<a href="https://github.com/SHAOXIA1991"><img src="https://github.com/SHAOXIA1991.png" width="50" height="50" style="border-radius:50%"/></a>

[Become a sponsor](https://github.com/sponsors/NoFxAiOS)

## License

[AGPL-3.0](LICENSE)

[![Star History Chart](https://api.star-history.com/svg?repos=NoFxAiOS/nofx&type=Date)](https://star-history.com/#NoFxAiOS/nofx&Date)
