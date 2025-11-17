# NOFX Requirements Specification

## 1. Scope and Goals

NOFX is an agentic trading OS that orchestrates multiple AI trading agents across supported exchanges (Binance Futures, Hyperliquid, Aster DEX). This document captures the current, code-backed behaviour of the system as shipped in this repository, in terms of user-facing and system-level requirements.

This is a living document and should be updated whenever behaviour changes.

## 2. Actors

- **Admin / Operator**
  - Installs and runs NOFX (typically self-hosted).
  - Manages system configuration, AI models, exchanges, and trader instances.
  - Monitors competition and per-trader performance.
- **Authenticated User**
  - Registers (if registration is enabled) and signs in to the web UI.
  - Configures their own AI models, exchanges, signal sources, and traders.
  - Starts/stops their own traders and inspects performance and decisions.
- **Public Visitor**
  - Accesses public competition and leaderboard data.
  - Views limited public trader information and equity history.
- **External Services**
  - Spot/derivatives exchanges (Binance, Hyperliquid, Aster).
  - AI model APIs (DeepSeek, Qwen, custom OpenAI-compatible endpoints).
  - Coin pool / OI Top data providers.

## 3. User Journeys

### 3.1 System Setup (Admin / Operator)

1. **Install & start**
   - The operator deploys NOFX (e.g. via Docker or manual Go/Node installation).
   - On first run, NOFX creates and migrates the SQLite database (`config.db`) and initialises default data (AI models, exchanges, system config) as in `config.NewDatabase` and `initDefaultData`.
2. **Configure system defaults**
   - The operator optionally provides a `config.json` and environment variables.
   - On startup `main.go`:
     - Reads `config.json` (if present) via `loadConfigFile`.
     - Synchronises values such as `beta_mode`, `api_server_port`, `default_coins`, risk parameters, and `jwt_secret` into the `system_config` table via `syncConfigToDatabase`.
     - Loads beta codes from `beta_codes.txt` if present.
3. **Start backend**
   - Backend starts HTTP API server on port decided by:
     - `NOFX_BACKEND_PORT` env var (highest priority).
     - `api_server_port` from `system_config` (if set).
     - Default `8080`.
   - Market data websocket monitor starts (`market.NewWSMonitor(150).Start(...)`).

### 3.2 Authentication and Registration (User)

1. **Registration**
   - When registration is enabled (`registration_enabled` in `system_config` != `"false"`), a visitor can:
     - Call `/api/register` to create a pending user.
     - Use `/api/verify-otp` to verify one-time code (TOTP/email, depending on current flow).
     - Call `/api/complete-registration` to finalise registration.
   - When registration is disabled, `/api/register` and related flows respond with an error and frontend should display `RegistrationDisabled` UI.
2. **Login**
   - User sends credentials and optional TOTP to `/api/login`.
   - On success, backend returns a JWT token; frontend stores it (e.g. `localStorage.auth_token`) and uses it in `Authorization: Bearer <token>` for protected endpoints.
3. **Logout**
   - Authenticated user calls `/api/logout` to invalidate current token (blacklist semantics).

### 3.3 Model & Exchange Configuration (User)

1. **View system-supported models and exchanges (public)**
   - Any visitor can call:
     - `GET /api/supported-models` to list supported AI model providers.
     - `GET /api/supported-exchanges` to list supported exchanges.
2. **Configure AI models (authenticated)**
   - User calls `GET /api/models` to fetch their AI model configs (per-user rows in `ai_models`).
   - User updates configs via `PUT /api/models`:
     - May be sent plaintext or encrypted using `/api/crypto/public-key` and `/api/crypto/decrypt` depending on frontend usage.
     - Backend stores API keys / custom API URLs in encrypted form via `crypto.CryptoService` when configured.
3. **Configure exchanges (authenticated)**
   - User calls `GET /api/exchanges` to fetch their exchange configurations (`exchanges` table).
   - User updates credentials via `PUT /api/exchanges`:
     - Sensitive fields (API keys, secrets, private keys) can be encrypted via public key prior to submission.
   - Separate per-user entries exist for each supported exchange (`binance`, `hyperliquid`, `aster`), with enable/disable flags and testnet toggles.

### 3.4 User Signal Source Configuration (User)

1. **View user signal sources**
   - Authenticated user calls `GET /api/user/signal-sources` to retrieve `coin_pool_url` and `oi_top_url` stored in `user_signal_sources`.
2. **Update user signal sources**
   - Authenticated user calls `POST /api/user/signal-sources` to set/update URLs.
   - When a trader is configured with `UseCoinPool`/`UseOITop`, the manager uses these URLs to fetch candidate coins.

### 3.5 Trader Lifecycle (User)

1. **List traders**
   - Authenticated user calls `GET /api/my-traders` to list their own traders, including status and key fields from `traders` table.
   - Public visitors can call `GET /api/traders` to retrieve an anonymised list and competition-related info.
2. **Create trader**
   - Authenticated user sends a `CreateTraderRequest` to `POST /api/traders` with:
     - `name`, `ai_model_id`, `exchange_id`, optional `trading_symbols`, leverage overrides, and scan interval.
   - Backend:
     - Validates model and exchange configs for the user.
     - Optionally queries the selected exchange via temporary `trader.Trader` to obtain actual account equity and overrides `initial_balance`.
     - Persists a `TraderRecord` with computed values.
3. **Update trader**
   - Authenticated user sends `PUT /api/traders/:id` with updated fields (same shape as create).
   - Backend updates DB record, possibly recalculating initial balance in line with exchange equity.
4. **Delete trader**
   - Authenticated user calls `DELETE /api/traders/:id`.
   - Backend removes trader from DB and from `TraderManager` if loaded.
5. **Start/stop trader**
   - Authenticated user calls:
     - `POST /api/traders/:id/start`
     - `POST /api/traders/:id/stop`
   - Manager:
     - Loads trader config from DB (if needed).
     - Starts/stops the corresponding `AutoTrader` goroutine for that user.
6. **Custom prompts**
   - Authenticated user can set or override system prompts via:
     - `PUT /api/traders/:id/prompt` with `custom_prompt` and `override_base` semantics.
   - At runtime, `TraderManager` propagates custom prompts into `AutoTrader`, which uses `decision.buildSystemPromptWithCustom` and `prompt_manager` to create composite prompts.

### 3.6 Monitoring & Analysis (User)

1. **Status and account**
   - Authenticated user requests `/api/status`, `/api/account`, `/api/positions` with optional `trader_id`.
   - Backend selects the trader (defaulting to first available if `trader_id` omitted) via `getTraderFromQuery`, then:
     - Exposes connection and running status.
     - Returns account equity, PnL, margin usage, and open positions.
2. **Decisions & performance**
   - Authenticated user:
     - Calls `/api/decisions` for decision history.
     - Calls `/api/decisions/latest` with optional `trader_id` and `limit` to obtain recent decisions.
     - Calls `/api/statistics` and `/api/performance` for aggregated performance metrics and AI learning analytics.
   - Decision logs are backed by JSON files in `decision_logs/{trader_id}/` and DB aggregates via `logger` (see design docs).
3. **Equity history**
   - Authenticated user calls `/api/equity-history` (optionally with `trader_id`) to retrieve equity curves.
   - Public visitors use `/api/equity-history` or `/api/equity-history-batch` for competition display.

### 3.7 Competition & Public Views (Public Visitor)

1. **Competition leaderboard**
   - Visitor calls `GET /api/competition` to retrieve aggregated competition data:
     - `TraderManager` collects per-trader data concurrently (`getConcurrentTraderData`), sorts by PnL percentage, and caches the result.
   - Frontend renders `CompetitionPage` using this data, including ROI, rankings, and AI model/exchange labels.
2. **Top traders**
   - Visitor uses `GET /api/top-traders` to fetch top-N traders (e.g. first 5–50), used in landing/marketing sections.
3. **Public trader configs**
   - Visitor calls `GET /api/traders/:id/public-config` (and `web` sometimes `GET /api/trader/:id/config`; see API spec for final canonical path) to display limited, non-sensitive profile details for a trader.

### 3.8 Server IP Utility (Authenticated User)

1. **Fetch server IP**
   - Authenticated user calls `GET /api/server-ip`.
   - Backend:
     - Optionally delegates to hook `hook.GETIP` to retrieve a per-user IP.
     - Falls back to external IP lookup services and/or network interfaces.
   - Used to configure IP whitelisting on exchanges.

## 4. System Behavioural Requirements

### 4.1 Risk Management & Constraints

At minimum, the system must enforce the following risk-related behaviours (actual values may be configurable; see trading-engine spec for details):

- **Leverage limits**
  - BTC/ETH and altcoins have separate leverage ceilings (`btc_eth_leverage`, `altcoin_leverage`), default 5x each.
  - AI decisions requesting leverage above these limits are clamped or rejected before order placement.
- **Position limits**
  - System restricts the number of simultaneous open positions per trader.
  - Per-symbol notional size is constrained as a function of account equity (e.g. altcoins up to ~1.5× equity, BTC/ETH up to ~10×).
- **Margin/risk usage**
  - Total margin usage must remain below a configured threshold (e.g. 90% of equity).
  - Daily loss and overall drawdown bounds (`max_daily_loss`, `max_drawdown`) govern when trading should be paused (`stop_trading_minutes`).
- **Stop-loss / take-profit requirements**
  - AI must output stop-loss and take-profit prices for all new positions.
  - Enforced minimum risk–reward ratio (e.g. ≥ 1:2 or 1:3) as defined in the prompt and engine.

### 4.2 Decision Loop and Market Data

- **Loop cadence**
  - Each `AutoTrader` periodically runs its decision loop at `ScanIntervalMinutes` (minimum 3 minutes).
- **Context composition**
  - Decision `Context` includes:
    - Account info (equity, PnL, margin usage, position count).
    - Per-position details (symbol, side, entry/mark/liq prices, size, leverage, unrealised PnL, margin used).
    - Candidate coins from pool and OI Top sources, up to a dynamic maximum based on current position count.
    - Market data per symbol (3m/4h klines, price changes, technical indicators, funding rate, OI statistics).
    - Historical performance analysis for the trader.
- **Model interaction**
  - Decision engine calls MCP client with:
    - `system` prompt built via templates and dynamic constraints.
    - `user` content containing JSON representation of `Context`.
  - Responses are parsed for:
    - `<reasoning>...</reasoning>` (ignored by execution, logged for analysis).
    - `<decision>```json [ ... ] ```</decision>` containing list of `Decision` records.
- **Execution**
  - For each parsed decision, engine validates:
    - Symbol validity and inclusion in trading universe.
    - Action type (`open_*`, `close_*`, `update_*`, `partial_close`, `hold`, `wait`).
    - Consistency with risk constraints and account state.
  - Orders are placed via the appropriate exchange client implementation (Binance, Hyperliquid, Aster) with correct precision formatting.

### 4.3 Logging and Observability

- **Decision logs**
  - Every decision cycle produces a JSON log under `decision_logs/{trader_id}/{timestamp}.json` capturing:
    - Input context snapshot.
    - Raw model response and parsed decisions.
    - Execution results and any risk constraint violations.
- **Equity history**
  - Trader equity, ROI, and related metrics are persisted to `equity_history` table at regular intervals for charting.
- **Competition cache**
  - TraderManager maintains an in-memory competition cache, refreshed on demand with:
    - Per-trader total equity, total PnL, ROI, position counts, margin usage, and running status.

## 5. Non-Functional Requirements

- **Performance**
  - Each trader runs in an isolated goroutine; the system should support dozens of concurrent traders on modest hardware, subject to exchange/API limits.
  - Market data is fetched via shared websocket monitor and in-memory cache to avoid redundant API calls.
- **Reliability**
  - Database uses SQLite with WAL and `synchronous=FULL` for durability.
  - On process restart, TraderManager reconstructs in-memory traders for all users from `traders`, `ai_models`, and `exchanges` tables (subject to enabled flags).
- **Security**
  - JWT tokens protect all state-changing and sensitive APIs.
  - Sensitive secrets (API keys, private keys) are encrypted at rest using RSA-based crypto service.
  - Optionally, beta codes may gate registration and/or feature access.
- **Configurability**
  - Risk parameters and defaults (coins, leverage, loss limits, trading stop minutes) are stored in `system_config` and may be updated via config file or admin tooling.
  - Admin mode can be enabled to constrain access to a single admin user and require stronger controls (see getting-started docs).

## 6. Out of Scope / Future Work

The following items are explicitly not guaranteed by the current implementation, but may appear in roadmap documents:

- Full multi-market support (stocks, futures, options, FX) beyond the existing crypto exchanges.
- Reinforcement learning / self-play agent training infrastructure.
- Full multi-tenant SaaS features and billing.

These should not be assumed in downstream specs unless implemented and referenced in code.

