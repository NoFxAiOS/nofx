# Trader Manager Design

This document describes the internal design of the NOFX `TraderManager` and how it coordinates multiple `AutoTrader` instances.

## 1. Responsibilities

`TraderManager` (`manager/trader_manager.go`) is responsible for:

- Loading trader configurations from the database at startup.
- Creating and managing `trader.AutoTrader` instances.
- Starting and stopping traders on demand.
- Providing aggregated data for competition and monitoring endpoints.
- Acting as a central registry of traders for API handlers.

## 2. Data Structures

```go
type TraderManager struct {
    traders          map[string]*trader.AutoTrader
    competitionCache *CompetitionCache
    mu               sync.RWMutex
}

type CompetitionCache struct {
    data      map[string]interface{}
    timestamp time.Time
    mu        sync.RWMutex
}
```

- `traders` maps `TraderRecord.ID` → `*AutoTrader`.
- `competitionCache` stores the last computed competition dataset to avoid redundant recomputation.
- `mu` protects concurrent access to the `traders` map.

## 3. Startup Flow

On application start, `main.go` calls:

```go
traderManager := manager.NewTraderManager()
err := traderManager.LoadTradersFromDatabase(database)
```

`LoadTradersFromDatabase`:

1. Fetches all user IDs via `database.GetAllUsers()`.
2. For each user, loads their `TraderRecord`s with `GetTraders(userID)`.
3. Fetches system-level risk configuration from `system_config`:
   - `max_daily_loss`, `max_drawdown`, `stop_trading_minutes`, `default_coins`.
4. For each trader:
   - Resolves `AIModelConfig` using `GetAIModels(traderCfg.UserID)`.
   - Resolves `ExchangeConfig` using `GetExchanges(traderCfg.UserID)`.
   - Fetches per-user signal sources via `GetUserSignalSource`.
   - Calls `addTraderFromDB` to instantiate and register an `AutoTrader`.
5. Logs counts and any skipped traders (e.g. missing model or disabled exchange).

This process populates `traders` with all ready-to-run trader instances at startup, but does not automatically start them; API or CLI must call start operations explicitly.

## 4. Creating AutoTrader Instances

`addTraderFromDB` (internal) and `AddTraderFromDB` (exported) create `AutoTrader` instances from DB records:

- Resolve trading symbols:
  - If `TraderRecord.TradingSymbols` is non-empty, parse comma-separated list.
  - Otherwise, fall back to `defaultCoins` from `system_config`.
- Apply user signal sources:
  - If `TraderRecord.UseCoinPool` is true and user-level `CoinPoolURL` is set, use it.
- Build `trader.AutoTraderConfig`:
  - IDs and names.
  - AI model provider and associated API keys.
  - Exchange ID and API credentials (Binance, Hyperliquid, or Aster-specific fields).
  - Risk parameters: `InitialBalance`, leverage settings, `MaxDailyLoss`, `MaxDrawdown`, `StopTradingTime`.
  - Coin lists and prompt template selection.
- Call `trader.NewAutoTrader(traderConfig, database, userID)` to construct the instance.
- Apply optional custom prompt:
  - If `TraderRecord.CustomPrompt` present, call `SetCustomPrompt` and `SetOverrideBasePrompt` to control base prompt behaviour.
- Register instance:
  - Store in `traders[TraderRecord.ID]`.

## 5. Concurrency Model

- All modifications to `traders` are guarded by `mu` (write lock).
- Read operations (e.g. fetching trader list, competition data) use `RLock` / `RUnlock`.
- Individual `AutoTrader` instances manage their own goroutines and internal synchronisation; `TraderManager` interacts with them through a stable interface:
  - `GetID`, `GetName`, `GetAIModel`, `GetExchange`.
  - `Start`, `Stop`, `GetStatus`, `GetAccountInfo`, and other helper methods.

## 6. Lifecycle Operations

Typical operations exposed to API handlers:

- **StartTrader(id)** (via handler):
  - Look up `*AutoTrader` from `traders`.
  - Start its decision loop (if not already running).
  - Update DB status via `UpdateTraderStatus(userID, id, true)`.
- **StopTrader(id)**:
  - Signal `AutoTrader` to stop (e.g. via context cancellation).
  - Update DB status to stopped.
- **StopAll()**:
  - Iterate all `traders` and stop each.
  - Used during graceful shutdown in `main.go`.

## 7. Competition & Aggregated Data

`TraderManager` provides competition data for:

- `GET /api/competition`
- `GET /api/top-traders`
- Public dashboard UI.

Key method: `getConcurrentTraderData`:

- Accepts a slice of `*AutoTrader`.
- For each trader, spawns a goroutine that:
  - Fetches account info via `GetAccountInfo()` with a 3-second timeout.
  - Reads trader status via `GetStatus()`.
  - Normalises errors/timeouts into a standard result with zeroed metrics and an `error` field.
- Receives results via channel and aggregates into a slice of `map[string]interface{}` with:
  - `trader_id`, `trader_name`, `ai_model`, `exchange`.
  - `total_equity`, `total_pnl`, `total_pnl_pct`.
  - `position_count`, `margin_used_pct`, `is_running`.
  - `system_prompt_template`.
- Sorting and limiting:
  - Sorts by `total_pnl_pct` descending.
  - Truncates to a configured limit (e.g. top 50).
- Caching:
  - Stores the aggregated result and timestamp in `competitionCache`.
  - Subsequent calls may serve cached data based on staleness policy.

## 8. Error Handling & Resilience

- Missing or disabled AI models and exchanges:
  - Traders with unresolved dependencies are skipped with log warnings.
- Per-trader account data errors:
  - Timeouts and API errors are logged and surfaced in competition data via an `error` field, but do not break overall aggregation.
- DB migration:
  - `Database.migrateExchangesTable` and other migrations ensure schema compatibility before `TraderManager` uses the data.

## 9. Extension Points

Potential extension areas:

- Support for additional exchanges:
  - Extend `ExchangeConfig` and `AutoTraderConfig`.
  - Implement new `trader.Trader` implementations and wire them into `addTraderFromDB`.
- More granular competition metrics:
  - Add fields to account info and competition maps.
  - Update frontend to visualise new metrics.
- Multi-process or microservice split:
  - Replace in-process `TraderManager` with RPC boundaries (e.g. gRPC), keeping this design document as the logical contract.

When extending `TraderManager`, update this document to keep it aligned with implementation.

