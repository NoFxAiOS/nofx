# NOFX Trading Engine & Risk Specification

This document specifies the behaviour and constraints of the NOFX trading engine as implemented across:

- `trader/` (AutoTrader and exchange clients)
- `decision/` (decision engine + prompt manager)
- `market/` (market data and indicators)
- `manager/` (TraderManager orchestration)

It is written in a behaviour-first style and aims to be testable.

## 1. Core Concepts

### 1.1 Trader

- A **Trader** is a running instance of `trader.AutoTrader` bound to:
  - A user (`user_id`).
  - One AI model (`ai_model_id` / provider).
  - One exchange (`exchange_id`).
  - A configuration snapshot (`TraderRecord` from DB plus risk/system config).
- Each trader has:
  - An initial balance (`InitialBalance`) used as baseline for PnL/ROI.
  - Separate maximum leverage for BTC/ETH vs altcoins.
  - References to system-level risk limits (`max_daily_loss`, `max_drawdown`, `stop_trading_minutes`).
  - A scan interval in minutes.

### 1.2 Decision Loop

Each trader runs an independent, periodic decision loop that:

1. Gathers current account state and positions from the exchange.
2. Builds a context object with market data and historical performance.
3. Calls the AI decision engine to obtain proposed actions.
4. Validates and filters decisions via risk constraints.
5. Translates approved decisions into orders via the exchange client.
6. Logs all inputs, outputs, and execution results for auditability.

## 2. Context & Decisions

### 2.1 Context Schema

The decision engine uses a `Context` struct (`decision/engine.go`) that includes:

- `CurrentTime` (string): Human-readable current time.
- `RuntimeMinutes` (int): Elapsed runtime since trader start.
- `CallCount` (int): Number of decision cycles executed.
- `Account` (`AccountInfo`):
  - `TotalEquity`, `AvailableBalance`, `UnrealizedPnL`, `TotalPnL`, `TotalPnLPct`.
  - `MarginUsed`, `MarginUsedPct`, `PositionCount`.
- `Positions` (list of `PositionInfo`):
  - `Symbol`, `Side` (`"long"` / `"short"`).
  - `EntryPrice`, `MarkPrice`, `Quantity`, `Leverage`.
  - `UnrealizedPnL`, `UnrealizedPnLPct`, `PeakPnLPct`.
  - `LiquidationPrice`, `MarginUsed`, `UpdateTime`.
- `CandidateCoins` (list of `CandidateCoin`):
  - `Symbol` (e.g. `"BTCUSDT"`).
  - `Sources` (e.g. `["ai500","oi_top"]`).
- `MarketDataMap` (internal, `map[string]*market.Data`):
  - 3m/4h klines, price changes (1h, 4h), EMA20, MACD, RSI7/14, ATR-like volatility, funding rate, OI series.
- `OITopDataMap`: per-symbol OI ranking/changes.
- `Performance`: aggregated historical performance metrics (P/L, win rate, etc.).
- `BTCETHLeverage` and `AltcoinLeverage`: risk parameters used in prompts and validation.

### 2.2 Decision Schema

AI responses are parsed into `Decision` objects with the following fields:

- `Symbol` (string) – must be a normalised symbol supported by the exchange.
- `Action` (string enum):
  - `"open_long"`, `"open_short"` – open a new position.
  - `"close_long"`, `"close_short"` – fully close an existing position.
  - `"update_stop_loss"`, `"update_take_profit"` – modify risk parameters.
  - `"partial_close"` – partially close a position (`ClosePercentage`).
  - `"hold"`, `"wait"` – no trade actions.
- For opening positions:
  - `Leverage` (int).
  - `PositionSizeUSD` (float64) – notional size in USDT.
  - `StopLoss` (float64).
  - `TakeProfit` (float64).
- For adjustments:
  - `NewStopLoss` (float64) – for `update_stop_loss`.
  - `NewTakeProfit` (float64) – for `update_take_profit`.
  - `ClosePercentage` (float64, 0–100) – for `partial_close`.
- Generic fields:
  - `Confidence` (int, 0–100).
  - `RiskUSD` (float64) – approximate max loss in USD.
  - `Reasoning` (string) – human-readable rationale.

### 2.3 Model I/O Protocol

- System prompt is built using `buildSystemPromptWithCustom` / `buildSystemPrompt` and includes:
  - Static strategy instructions (from prompt templates).
  - Dynamic risk constraints: max leverage, per-symbol position ranges, margin and R:R rules.
  - Output format specification with XML and JSON, e.g.:
    ```xml
    <reasoning>
    ...thought process...
    </reasoning>
    <decision>
    ```json
    [ { ...Decision... }, ... ]
    ```
    </decision>
    ```
- User content is the JSON serialisation of `Context` (with some internal fields omitted).
- Response parsing:
  - Extract `<reasoning>` and `<decision>` blocks via regex.
  - Remove invisible characters and ensure the JSON array is valid.
  - Parse into `[]Decision`.
  - If parsing fails, the cycle may be skipped and error logged.

## 3. Risk Rules (Enforceable Behaviour)

This section defines constraints that the engine must enforce before sending orders to the exchange.

### 3.1 Leverage Limits

- For BTC/ETH symbols:
  - `decision.Leverage` **must not exceed** `BTCETHLeverage` from configuration.
- For all other symbols (altcoins):
  - `decision.Leverage` **must not exceed** `AltcoinLeverage`.
- If AI suggests higher leverage:
  - Either clamp to max or reject the decision (implementation choice should be documented in code comments and logs).

### 3.2 Position Count & Allocation

- Each trader has a maximum allowed number of open positions (as expressed in prompts, e.g. 3).
- If `Account.PositionCount` is already at limit:
  - Engine must reject further `"open_*"` decisions for new symbols.
- Position size constraints:
  - For BTC/ETH: `PositionSizeUSD` should fall within `[min_btceth, max_btceth]`, derived from account equity (e.g. `5x`–`10x` constraints).
  - For altcoins: `PositionSizeUSD` within `[min_alt, max_alt]` (e.g. `0.8x`–`1.5x` equity range).
  - Engine validates ranges before execution; invalid decisions are logged and skipped.

### 3.3 Margin & Drawdown Controls

- Margin usage:
  - After executing proposed changes, estimated `MarginUsedPct` must remain ≤ configured threshold (default 90%).
  - If executing would exceed threshold, engine must scale down or skip orders.
- Daily loss limit:
  - If daily loss exceeds `max_daily_loss` percentage of equity, trader enters “cooldown”:
    - No new opening trades.
    - Closing trades are still allowed.
    - Cooldown duration is at least `stop_trading_minutes`.
- Max drawdown:
  - If running drawdown (from peak equity) exceeds `max_drawdown`, similar protective behaviour applies.

### 3.4 Stop Loss / Take Profit Rules

- For all `open_*` actions:
  - `StopLoss` and `TakeProfit` must be present and valid prices.
  - `TakeProfit` must imply favourable risk–reward vs `StopLoss` (e.g. R:R ≥ 1:2 or 1:3 as configured).
  - Engine checks distance to current price and rejects absurd or exchange-invalid values.
- For updates:
  - `NewStopLoss` / `NewTakeProfit` must move in a direction that does not increase risk beyond configured limits (e.g. no widening SL to increase risk).

### 3.5 Symbol & Side Consistency

- For `close_*`, `update_*`, and `partial_close`:
  - There must be an existing position matching symbol and side.
  - Closing or partial-closing must not push position quantity below zero.

## 4. Market Data & Technical Indicators

### 4.1 Kline Data

- `market.Get(symbol)` must:
  - Normalise symbol via `Normalize`.
  - Fetch recent 3m and 4h klines via `WSMonitorCli.GetCurrentKlines(symbol, "3m"|"4h")`.
  - Return an error if either timeframe has no data.
- Staleness checks:
  - Engine detects freeze issues (e.g. constant prices for many klines) via `isStaleData`.
  - If stale, symbol is skipped and error logged.

### 4.2 Indicators

- For each symbol, engine computes:
  - `EMA20` on 3m klines via `calculateEMA`.
  - `MACD` using 12/26 EMA via `calculateMACD`.
  - `RSI7` and `RSI14` via `calculateRSI`.
  - `ATR` via `calculateATR`.
- Short/long-term price changes:
  - 1h change from 3m klines (20 steps ago).
  - 4h change from previous 4h kline.

### 4.3 Open Interest & Funding

- OI data:
  - Retrieved via `getOpenInterestData`, returning latest and average OI.
  - Engine treats failures as non-fatal and substitutes `0` values.
- Funding rate:
  - Retrieved via `getFundingRate`, cached in `FundingRateCache` (`sync.Map`) with 1-hour TTL.

### 4.4 Intraday & Longer-Term Context

- `calculateIntradaySeries` builds arrays of:
  - Mid prices, EMA20, MACD, RSI7/14, and volumes over the last ~10 Klines.
- `calculateLongerTermData` summarises longer horizon context from 4h klines (trend, volatility).

## 5. TraderManager Orchestration

### 5.1 Startup

- On application start:
  - `TraderManager.LoadTradersFromDatabase`:
    - Fetches all `user_id`s via `GetAllUsers`.
    - For each user, loads `TraderRecord`s via `GetTraders`.
    - For each trader:
      - Resolves `AIModelConfig` and `ExchangeConfig`.
      - Skips traders if:
        - Model not found or disabled.
        - Exchange not found or disabled.
      - Retrieves per-user signal sources (coin pool / OI Top URLs).
      - Calls `addTraderFromDB` to instantiate `AutoTrader` with combined configuration.
  - Each instantiated trader is stored in `traders` map keyed by `TraderRecord.ID`.

### 5.2 Lifecycle Management

- `StartTrader(id)`:
  - Starts the trader’s decision loop in a goroutine (implementation in `trader.AutoTrader`).
  - Updates DB trader status (`is_running`) via `UpdateTraderStatus`.
- `StopTrader(id)`:
  - Stops the loop (e.g. via context cancellation or internal flags).
  - Updates DB status accordingly.
- `StopAll()`:
  - Stops all traders, used on graceful shutdown.

### 5.3 Competition Data

- `GetCompetitionData()`:
  - Collects data from all traders concurrently via `getConcurrentTraderData`.
  - For each trader, attempts to fetch account info with a 3-second timeout.
  - On success, returns:
    - `total_equity`, `total_pnl`, `total_pnl_pct`, `position_count`, `margin_used_pct`, `is_running`, `system_prompt_template`.
  - On failure/timeout, returns default/zero values plus error message.
  - Sorts traders by `total_pnl_pct` descending and truncates to top 50 entries.
  - Caches the result in `competitionCache` with timestamp for subsequent reads.

## 6. Logging & Persistence

### 6.1 Decision Logs

- Every decision cycle writes:
  - A JSON file under `decision_logs/{trader_id}/{timestamp}.json` containing:
    - Context snapshot.
    - Raw AI response.
    - Parsed decisions.
    - Orders sent and exchange responses.
  - DB updates for equity and performance where applicable.

### 6.2 Equity History

- Equity and ROI metrics are periodically stored in `equity_history` table:
  - Used by `/api/equity-history` and `/api/equity-history-batch`.
  - Should include timestamps and PnL percentages.

## 7. Testing Implications

This spec implies the following unit-/integration-test candidates:

- **Decision parsing:**
  - Valid/invalid `<reasoning>` and `<decision>` blocks.
  - Correct handling of invisible characters and malformed JSON.
- **Risk validation:**
  - Leverage clamping or rejection when AI suggests > max.
  - Position count limit enforcement.
  - R:R checks for stop-loss / take-profit.
  - Margin usage and daily loss/drawdown guardrails.
- **Market data:**
  - Indicator correctness (EMA/MACD/RSI/ATR) using known series.
  - Staleness detection behaviour.
- **Manager orchestration:**
  - `LoadTradersFromDatabase` logic with missing/disabled model or exchange.
  - Competition data sorting and truncation.

Whenever the underlying implementation changes, update this document and add/adjust tests to match.

