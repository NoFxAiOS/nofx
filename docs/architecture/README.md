# NOFX Architecture Documentation

**Language:** [English](README.md) | [中文](README.zh-CN.md)

Technical documentation for developers who want to understand NOFX internals.

---

## Overview

NOFX is a full-stack AI trading platform for cryptocurrency and US stock markets:

- **Backend:** Go (Gin framework, SQLite)
- **Frontend:** React/TypeScript (Vite, TailwindCSS)
- **AI Models:** DeepSeek, Qwen, OpenAI (GPT-5.2), Claude, Gemini, Grok, Kimi
- **Exchanges:** Binance, Bybit, OKX, Hyperliquid, Aster, Lighter

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              NOFX Platform                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐│
│  │  Strategy   │  │  Backtest   │  │   Debate    │  │   Live Trading      ││
│  │   Studio    │  │   Engine    │  │    Arena    │  │   (Auto Trader)     ││
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘│
│         │                │                │                    │           │
│         └────────────────┴────────────────┴────────────────────┘           │
│                                    │                                        │
│                          ┌─────────▼─────────┐                              │
│                          │   Core Services   │                              │
│                          │  - Market Data    │                              │
│                          │  - AI Providers   │                              │
│                          │  - Risk Control   │                              │
│                          └─────────┬─────────┘                              │
│                                    │                                        │
│         ┌──────────────────────────┼──────────────────────────┐            │
│         │                          │                          │            │
│  ┌──────▼──────┐         ┌─────────▼─────────┐      ┌────────▼────────┐   │
│  │  Exchanges  │         │     Database      │      │   Frontend UI   │   │
│  │  (CEX/DEX)  │         │    (SQLite)       │      │   (React SPA)   │   │
│  └─────────────┘         └───────────────────┘      └─────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Module Documentation

### Core Modules

| Module | Description | Documentation |
|--------|-------------|---------------|
| **Strategy Studio** | Strategy configuration, coin selection, data assembly, AI prompts | [STRATEGY_MODULE.md](STRATEGY_MODULE.md) |
| **Backtest Engine** | Historical simulation, performance metrics, AI decision replay | [BACKTEST_MODULE.md](BACKTEST_MODULE.md) |
| **Debate Arena** | Multi-AI collaborative decision making with voting consensus | [DEBATE_MODULE.md](DEBATE_MODULE.md) |

### Module Overview

#### Strategy Module
Complete strategy configuration system including:
- Coin source selection (static list, AI500 pool, OI ranking)
- Market data indicators (K-lines, EMA, MACD, RSI, ATR)
- Prompt construction (system prompt, user prompt, sections)
- AI response parsing and decision execution
- Risk control enforcement

**[Read Full Documentation →](STRATEGY_MODULE.md)**

#### Backtest Module
Historical trading simulation engine:
- Multi-symbol, multi-timeframe backtesting
- AI decision replay with caching
- Performance metrics (Sharpe, drawdown, win rate)
- Real-time progress streaming via SSE
- Checkpoint and resume support

**[Read Full Documentation →](BACKTEST_MODULE.md)**

#### Debate Module
Multi-AI collaborative decision system:
- 5 AI personalities (Bull, Bear, Analyst, Contrarian, Risk Manager)
- Multi-round debate with market context
- Weighted voting and consensus algorithm
- Auto-execution to live trading
- Real-time SSE streaming

**[Read Full Documentation →](DEBATE_MODULE.md)**

---

## Project Structure

```
nofx/
├── main.go                    # Entry point
├── api/                       # HTTP API (Gin framework)
├── trader/                    # Trading execution layer
├── strategy/                  # Strategy engine
├── backtest/                  # Backtest simulation engine
├── debate/                    # Debate arena engine
├── market/                    # Market data service
├── mcp/                       # AI model clients
├── store/                     # Database operations
├── auth/                      # JWT authentication
├── manager/                   # Multi-trader management
└── web/                       # React frontend
    ├── src/pages/             # Page components
    ├── src/components/        # Shared components
    └── src/lib/api.ts         # API client
```

---

## Core Dependencies

### Backend (Go)

| Package | Purpose |
|---------|---------|
| `gin-gonic/gin` | HTTP API framework |
| `adshao/go-binance` | Binance API client |
| `markcheno/go-talib` | Technical indicators |
| `golang-jwt/jwt` | JWT authentication |

### Frontend (React)

| Package | Purpose |
|---------|---------|
| `react` | UI framework |
| `recharts` | Charts and visualizations |
| `swr` | Data fetching |
| `zustand` | State management |
| `tailwindcss` | CSS framework |

---

## Quick Links

- [Strategy Module](STRATEGY_MODULE.md) - How strategies work
- [Backtest Module](BACKTEST_MODULE.md) - How backtesting works
- [Debate Module](DEBATE_MODULE.md) - How AI debates work
- [Getting Started](../getting-started/README.md) - Setup guide
- [FAQ](../faq/README.md) - Frequently asked questions

---

## For Developers

**Want to contribute?**
- Read the module documentation above
- Check [Open Issues](https://github.com/NoFxAiOS/nofx/issues)
- Join our community

**Repository:** https://github.com/NoFxAiOS/nofx

---

[← Back to Documentation](../README.md)
