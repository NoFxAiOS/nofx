# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> **Note:** For agent-specific workflows and specialized development patterns, see [AGENTS.md](AGENTS.md).

## Project Overview

NOFX is an agentic AI trading operating system that combines multi-agent decision-making, unified risk control, and low-latency execution across multiple cryptocurrency exchanges (Binance, Hyperliquid, Aster DEX). The system features a Go backend with a React/TypeScript frontend, enabling real-time AI-powered trading with multiple AI models (DeepSeek, Qwen, custom OpenAI-compatible APIs).

## Architecture

**Backend Stack:**
- Go 1.21+ with Gin framework
- **SQLite database** (config.db, trading.db) - Deliberately chosen for zero-config deployment. No database server setup required - just run and trade. Perfect for individual traders and small teams.
- RESTful API server (default port 8080)
- Concurrent goroutine-based trader execution
- TA-Lib for technical indicator calculations

**Frontend Stack:**
- React 18 + TypeScript 5.8+
- Vite 6.0+ build tool
- TailwindCSS for styling
- Recharts for data visualization
- SWR for data fetching with 5-10s polling
- Zustand for state management

**Key Architectural Principles:**
- **Zero-config philosophy**: SQLite chosen intentionally so traders can run the system without managing database servers
- Interface-based abstraction for exchange adapters (Strategy pattern)
- Database-driven configuration (moved away from JSON config files in v3.0.0)
- Each trader runs in a separate goroutine managed by TraderManager
- AI decision engine with historical feedback loop
- Multi-exchange support via unified trader interface

## Module Structure

```
trader/          - Exchange adapters and execution layer
  - auto_trader.go         (main trading orchestrator)
  - interface.go           (unified trader interface)
  - binance_futures.go     (Binance API wrapper)
  - hyperliquid_trader.go  (Hyperliquid DEX wrapper)
  - aster_trader.go        (Aster DEX wrapper)

decision/        - AI decision engine
  - engine.go              (decision logic with historical feedback)
  - prompt_manager.go      (AI prompt template system)

manager/         - Multi-trader orchestration
  - trader_manager.go      (lifecycle management, concurrent execution)

config/          - Database layer
  - database.go            (SQLite operations, schema management)

auth/            - Authentication
  - auth.go                (JWT + 2FA/TOTP support)

mcp/             - AI communication
  - client.go              (AI API client for DeepSeek/Qwen/Custom)

market/          - Market data
  - data.go                (K-lines, technical indicators via TA-Lib)

pool/            - Coin pool management
  - coin_pool.go           (AI500 + OI Top merged pool)

logger/          - Decision logging
  - decision_logger.go     (JSON logs + performance analysis)

api/             - HTTP API server
  - server.go              (Gin framework, RESTful endpoints)

web/             - React frontend
  src/components/          (React components)
  src/pages/               (Page components)
  src/lib/api.ts           (API wrapper)
  src/types/               (TypeScript types)
```

## Development Commands

### Backend (Go)

```bash
# Install dependencies
go mod download

# Install TA-Lib (required)
# macOS:
brew install ta-lib
# Ubuntu/Debian:
sudo apt-get install libta-lib0-dev

# Build
go build -o nofx

# Run backend (development)
./nofx

# Run backend with custom database path
./nofx path/to/config.db

# Format code
go fmt ./...

# Run tests
go test ./...

# Run linter (if configured)
golangci-lint run
```

### Frontend (React/TypeScript)

```bash
# Navigate to frontend directory
cd web

# Install dependencies
npm install

# Development server (port 3000)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Type check
tsc --noEmit
```

### Docker Deployment

```bash
# Start all services (recommended)
./start.sh start --build

# View logs
./start.sh logs

# Check status
./start.sh status

# Stop services
./start.sh stop

# Restart services
./start.sh restart

# Or use docker compose directly
docker compose up -d --build
docker compose logs -f
docker compose down
```

## Database Schema

**Core Tables:**
- `users` - User accounts with 2FA support
- `ai_models` - AI model configurations (DeepSeek, Qwen, custom)
- `exchanges` - Exchange credentials (Binance, Hyperliquid, Aster)
- `traders` - Trader instance configurations
- `equity_history` - Performance tracking over time
- `system_config` - Application-wide settings (key-value pairs)

**Configuration Flow:**
- System settings stored in `system_config` table
- Legacy `config.json` synced to database on startup (main.go lines 40-106)
- All trader management via web interface → database → TraderManager

## AI Decision Flow

Each trading cycle (default 3-5 minutes):

1. **Analyze Historical Performance** - Last 20 cycles, win rate, P/L ratio, per-coin stats
2. **Get Account Status** - Equity, available balance, margin usage, positions
3. **Analyze Existing Positions** - Fetch latest market data, calculate indicators (RSI, MACD, EMA, ATR)
4. **Evaluate New Opportunities** - Coin pool (default or API), liquidity filtering, batch market data
5. **AI Decision** - DeepSeek/Qwen with Chain-of-Thought reasoning, structured output (action, symbol, quantity, leverage, SL/TP)
6. **Execute Trades** - Priority: close existing → open new, with risk checks (position limits, margin usage, no duplicates)
7. **Record Logs** - Decision logs as JSON files, performance database updates

**Risk Controls:**
- Position limits: Altcoins ≤1.5x equity, BTC/ETH ≤10x equity
- Leverage: Configurable per asset class (default 5x for safety, up to 50x BTC/ETH on main accounts)
- Margin: Total usage ≤90%
- Risk-reward ratio: Mandatory ≥1:2 SL/TP
- Anti-stacking: No duplicate positions in same symbol+direction

## API Endpoints

### Configuration
- `GET /api/models` - Get AI model configurations
- `PUT /api/models` - Update AI models
- `GET /api/exchanges` - Get exchange configurations
- `PUT /api/exchanges` - Update exchanges

### Trader Management
- `GET /api/traders` - List all traders
- `POST /api/traders` - Create new trader
- `DELETE /api/traders/:id` - Delete trader
- `POST /api/traders/:id/start` - Start trader
- `POST /api/traders/:id/stop` - Stop trader

### Trading Data
- `GET /api/status?trader_id=xxx` - System status
- `GET /api/account?trader_id=xxx` - Account info
- `GET /api/positions?trader_id=xxx` - Position list
- `GET /api/equity-history?trader_id=xxx` - Equity chart data
- `GET /api/decisions/latest?trader_id=xxx` - Latest 5 decisions
- `GET /api/statistics?trader_id=xxx` - Performance stats
- `GET /api/performance?trader_id=xxx` - AI performance analysis

### System
- `GET /api/health` - Health check

## Key Implementation Details

### Exchange Adapter Pattern
All exchange implementations must satisfy the `ExchangeClient` interface (trader/interface.go):
- `GetAccount()` - Fetch account balance and margin info
- `GetPositions()` - Fetch current open positions
- `CreateOrder()` - Place new orders
- Automatic precision handling per exchange
- Unified error handling and response normalization

### Multi-Trader Coordination
- TraderManager (manager/trader_manager.go) manages multiple concurrent traders
- Each trader runs in its own goroutine with independent decision cycles
- Database used for configuration persistence and cross-trader state
- API server provides real-time access to all trader states

### Decision Logging
- Every decision saved to `decision_logs/{trader_id}/{timestamp}.json`
- Includes: full Chain-of-Thought, input prompt, structured decision, account snapshot, execution results
- Performance data tracked: win rate, profit factor, Sharpe ratio, per-coin statistics
- Historical feedback loop: last 20 trades analyzed before each new decision

### Frontend Data Flow
- SWR with 5-10s polling for real-time updates
- Zustand for client-side state management
- Recharts for equity curves and comparison charts
- Dark theme styled like Binance interface

## Common Development Tasks

### Adding a New Exchange
1. Create new file in `trader/` (e.g., `okx_trader.go`)
2. Implement `ExchangeClient` interface
3. Add exchange configuration to database schema (config/database.go)
4. Update frontend exchange selection UI (web/src/components/)
5. Add exchange-specific API client logic
6. Test with small amounts on testnet/mainnet

### Adding a New AI Model
1. Add model configuration to database schema (config/database.go)
2. Update MCP client to support new API (mcp/client.go)
3. Add model selection to frontend (web/src/components/)
4. Test decision generation and parsing
5. Document API key setup in README

### Modifying Decision Logic
- Main decision engine: `decision/engine.go`
- Prompt templates: `decision/prompt_manager.go`
- Always maintain Chain-of-Thought format for explainability
- Update historical feedback system if changing performance metrics
- Test with small position sizes first

### Adding API Endpoints
1. Define route in `api/server.go`
2. Add handler function with proper authentication middleware
3. Update frontend API client (web/src/lib/api.ts)
4. Add TypeScript types (web/src/types/)
5. Update UI components to consume new endpoint

## Testing Strategy

**Current State:**
- No formal unit tests yet (manual testing + testnet verification)
- Frontend: No test suite configured

**Priority Testing Areas:**
1. Exchange precision handling (critical for order execution)
2. Risk control validation (position limits, margin checks)
3. AI decision parsing (structured output validation)
4. Database operations (CRUD for traders, models, exchanges)
5. Frontend component testing (Vitest + React Testing Library recommended)

## Environment Variables

From `.env.example`:
- `NOFX_BACKEND_PORT` - Backend API port (default: 8080)
- `NOFX_FRONTEND_PORT` - Frontend port (default: 3000)
- `NOFX_TIMEZONE` - Container timezone (default: Asia/Shanghai)

## Configuration Migration (v3.0.0+)

**Important:** Configuration moved from `config.json` to database in v3.0.0:
- Legacy `config.json` still supported (synced to DB on startup)
- All new configuration via web interface
- Traders, exchanges, AI models managed through API/UI
- `main.go` handles sync from config.json to database (lines 40-106)

## Security Considerations

- API keys encrypted in database
- JWT authentication with optional 2FA (TOTP)
- Admin mode available for single-user deployments (auth.SetAdminMode)
- Never commit real API keys or secrets
- Use `.env` files for sensitive environment variables
- Whitelist IP addresses for exchange API access

## Performance Characteristics

- Backend memory: ~50-100MB per trader
- Database: **SQLite by design** - Zero-configuration, file-based, perfect for traders who don't want to manage database servers. Handles 100+ concurrent traders easily. The simplicity is intentional, not a limitation.
- API rate limits handled per exchange
- Frontend bundle: ~500KB gzipped
- Trader decision cycle: 3-5 minutes (configurable)

## Branching Strategy

- `main` - Production-ready stable branch
- `dev` - Development branch (default PR target)
- `feature/*` - New features
- `fix/*` - Bug fixes

**Always create PRs to `dev` branch, not `main`.**

## Debugging Tips

### Backend Issues
- Check trader logs: Look for "❌" prefixed errors in console output
- Database errors: Verify config.db permissions and schema
- Exchange API errors: Check API key validity, IP whitelist, rate limits
- TA-Lib errors: Ensure library installed (`brew install ta-lib` on macOS)

### Frontend Issues
- Check browser console for API errors
- Verify backend is running on correct port (default 8080)
- Check CORS settings if API calls fail
- SWR caching: Use React DevTools to inspect SWR state

### Common Errors
- "Precision is over the maximum" → Binance LOT_SIZE precision handling (auto-fixed in code)
- "Subaccounts restricted to 5x leverage" → Lower leverage config for Binance subaccounts
- "Port 8080 already in use" → Change `NOFX_BACKEND_PORT` in .env
- "TA-Lib not found" → Install system library first before running Go build

## Related Documentation

- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines and PR process
- **[README.md](README.md)** - User-facing documentation and quick start
- **[AGENTS.md](AGENTS.md)** - Agent workflows and specialized development patterns (symlink to this file)
- **[Architecture Docs](docs/architecture/README.md)** - Detailed technical architecture
- **[Roadmap](docs/roadmap/README.md)** - Future features and expansion plans

## Contributing

See CONTRIBUTING.md for detailed guidelines:
- Use conventional commits format (`feat:`, `fix:`, `docs:`, etc.)
- Keep PRs focused and <1000 lines when possible
- Update documentation for new features
- Follow Go and TypeScript/React coding standards
- Run tests and linters before submitting
