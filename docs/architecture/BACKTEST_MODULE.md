# NOFX Backtest Module - Technical Documentation

**Language:** [English](BACKTEST_MODULE.md) | [中文](BACKTEST_MODULE.zh-CN.md)

## Overview

This document describes the complete technical implementation of the NOFX backtest module, including configuration, historical data loading, simulation engine, AI decision making, performance metrics calculation, and result storage.

---

## Complete Backtest Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Backtest Execution Flow                       │
└─────────────────────────────────────────────────────────────────┘

1. API Request: /backtest/start
   ↓
2. Manager.Start()
   ├─ Validate config
   ├─ Parse AI model
   ├─ Create Runner instance
   └─ Start runner.Start() (goroutine)
   ↓
3. Runner.Start() → Runner.loop()
   └─ Iterate each decision time point:
      ├─ DataFeed.BuildMarketData()      [Build market data]
      ├─ Check decision trigger           [Every N bars]
      ├─ buildDecisionContext()           [Build decision context]
      ├─ invokeAIWithRetry()              [Call AI + cache]
      ├─ executeDecision()                [Execute trades]
      ├─ checkLiquidation()               [Check liquidation]
      ├─ updateState()                    [Update state]
      ├─ appendEquityPoint()              [Record equity]
      ├─ appendTradeEvent()               [Record trades]
      ├─ maybeCheckpoint()                [Save checkpoint]
      └─ persistMetrics()                 [Persist metrics]
   ↓
4. Complete/Failed
   ├─ Calculate final metrics
   ├─ Persist all results
   └─ Release lock
   ↓
5. API Query: /backtest/metrics, /backtest/equity, /backtest/trades
   └─ Load and return results
```

---

## 1. Configuration

**Core File:** `backtest/config.go`

### 1.1 Config Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `RunID` | string | (required) | Unique backtest run ID |
| `UserID` | string | "default" | User ID |
| `Symbols` | []string | (required) | Trading symbols list |
| `Timeframes` | []string | ["3m", "15m", "4h"] | K-line timeframes |
| `DecisionTimeframe` | string | Symbols[0] | Primary decision timeframe |
| `DecisionCadenceNBars` | int | 20 | Trigger decision every N bars |
| `StartTS`, `EndTS` | int64 | (required) | Backtest time range (Unix timestamp) |
| `InitialBalance` | float64 | 1000 | Initial balance (USD) |
| `FeeBps` | float64 | 5 | Trading fee (basis points) |
| `SlippageBps` | float64 | 2 | Slippage (basis points) |
| `FillPolicy` | string | "next_open" | Fill policy |
| `PromptVariant` | string | "baseline" | AI prompt variant |
| `CacheAI` | bool | false | Cache AI decisions |
| `Leverage` | LeverageConfig | BTC/ETH:5, Altcoin:5 | Leverage settings |

### 1.2 Fill Policy

```go
// backtest/config.go:163-179
switch fillPolicy {
case "next_open":  // Next bar open price
case "bar_vwap":   // Current bar VWAP
case "mid":        // Current bar (High+Low)/2
default:           // Mark Price
}
```

### 1.3 Config Example

```go
cfg := backtest.BacktestConfig{
    RunID:                "bt_20231215_150405",
    Symbols:              []string{"BTCUSDT", "ETHUSDT"},
    Timeframes:           []string{"3m", "15m", "4h"},
    DecisionTimeframe:    "3m",
    DecisionCadenceNBars: 20,
    StartTS:              1702566000,
    EndTS:                1702652400,
    InitialBalance:       10000,
    FeeBps:               5,
    SlippageBps:          2,
    FillPolicy:           "next_open",
}
```

---

## 2. Data Loading

**Core File:** `backtest/datafeed.go`

### 2.1 Data Loading Flow

```
1. NewDataFeed() - Initialize
   ↓
2. loadAll() - Load all historical data
   ├─ Calculate buffer (200 bars before StartTS)
   ├─ Call market.GetKlinesRange() to fetch data
   ├─ Store in symbolSeries map
   └─ Build decision timeline from primary timeframe
   ↓
3. BuildMarketData() - Build market data snapshot
   ├─ Slice K-line data to current timestamp
   ├─ Calculate technical indicators (EMA, MACD, RSI, ATR)
   └─ Return market.Data structure
```

### 2.2 Data Structure

```go
// DataFeed core structure
type DataFeed struct {
    decisionTimes []int64                    // Decision time points list
    symbolSeries  map[string]*symbolSeries   // Data stored by symbol
}

// Single symbol time series
type symbolSeries struct {
    timeframes map[string]*timeframeSeries   // Stored by timeframe
}

// Single timeframe data
type timeframeSeries struct {
    klines     []market.Kline               // K-line data
    closeTimes []int64                      // Close time index
}
```

### 2.3 Key Code References

- Data fetching: `backtest/datafeed.go:48-93`
- Timeline generation: `backtest/datafeed.go:96-115`
- Market data assembly: `backtest/datafeed.go:141-171`

---

## 3. Simulation Engine

**Core File:** `backtest/runner.go`

### 3.1 Main Loop

```go
// backtest/runner.go:232-264
func (r *Runner) loop() {
    for _, ts := range r.feed.DecisionTimes() {
        if r.isPaused() {
            break
        }
        r.stepOnce(ts)
    }
}
```

### 3.2 Single Step Execution

```go
// backtest/runner.go:266-471
func (r *Runner) stepOnce(ts int64) {
    // 1. Get current bar timestamp
    // 2. Build market data
    // 3. Check decision trigger (every N bars)
    // 4. Execute decision cycle (if triggered)
    // 5. Check liquidation
    // 6. Update state and record
}
```

### 3.3 State Management

```go
// backtest/types.go:31-47
type BacktestState struct {
    BarIndex       int                      // Current bar index
    Cash           float64                  // Available balance
    Equity         float64                  // Total equity
    UnrealizedPnL  float64                  // Unrealized PnL
    RealizedPnL    float64                  // Realized PnL
    MaxEquity      float64                  // Peak equity
    MinEquity      float64                  // Trough equity
    MaxDrawdownPct float64                  // Max drawdown
    Positions      map[string]*position     // Positions
}
```

---

## 4. AI Decision Making

**Core File:** `backtest/runner.go`

### 4.1 Decision Context Building

```go
// backtest/runner.go:473-532
func (r *Runner) buildDecisionContext() *decision.Context {
    return &decision.Context{
        CurrentTime:    "2023-12-15 10:30:00 UTC",
        RuntimeMinutes: elapsed,
        CallCount:      cycleNumber,
        Account: {
            TotalEquity, AvailableBalance, TotalPnL, MarginUsedPct
        },
        Positions:       []PositionInfo{...},
        CandidateCoins:  []string{symbols...},
        MarketDataMap:   map[symbol]*market.Data{...},
        MultiTFMarket:   map[symbol]map[timeframe]*market.Data{...},
    }
}
```

### 4.2 AI Invocation

```go
// backtest/runner.go:544-563
func (r *Runner) invokeAIWithRetry() (*decision.FullDecision, error) {
    // Max 3 retries
    // Exponential backoff: 500ms, 1000ms, 1500ms
    // Uses decision.GetFullDecisionWithStrategy() for unified prompt generation
}
```

### 4.3 AI Cache

```go
// backtest/aicache.go:127-168
// Cache key: SHA256(context payload)
// Contains: variant, timestamp, account, positions, market data
```

### 4.4 Supported AI Models

| Model | Client File |
|-------|-------------|
| DeepSeek | `mcp/deepseek_client.go` |
| Qwen | `mcp/qwen_client.go` |
| Claude | `mcp/claude_client.go` |
| Gemini | `mcp/gemini_client.go` |
| Grok | `mcp/grok_client.go` |
| OpenAI | `mcp/openai_client.go` |
| Kimi | `mcp/kimi_client.go` |

---

## 5. Performance Metrics

**Core File:** `backtest/metrics.go`

### 5.1 Metrics Calculation

| Metric | Formula | Code Location |
|--------|---------|---------------|
| **Total Return** | (Final Equity - Initial) / Initial × 100 | metrics.go:36-42 |
| **Max Drawdown** | max((Peak - Current) / Peak × 100) | metrics.go:64-91 |
| **Sharpe Ratio** | Avg Return / Return StdDev | metrics.go:94-138 |
| **Win Rate** | Winning Trades / Total Trades × 100 | metrics.go:180-181 |
| **Profit Factor** | Total Profit / Total Loss | metrics.go:189-193 |

### 5.2 Trade Statistics

```go
// backtest/metrics.go:141-225
type TradeMetrics struct {
    TotalTrades    int
    WinningTrades  int
    LosingTrades   int
    AvgWin         float64
    AvgLoss        float64
    BestSymbol     string
    WorstSymbol    string
    SymbolStats    map[string]*SymbolStat
}
```

---

## 6. Equity Curve

**Core File:** `backtest/equity.go`

### 6.1 Equity Point Structure

```json
{
  "ts": 1702566000000,
  "equity": 10500.50,
  "available": 8000.00,
  "pnl": 500.50,
  "pnl_pct": 5.005,
  "dd_pct": 2.34,
  "cycle": 42
}
```

### 6.2 Equity Update

```go
// backtest/runner.go:829-872
func (r *Runner) updateState() {
    // 1. Calculate total equity: cash + margin + unrealized PnL
    // 2. Track peak (MaxEquity)
    // 3. Track trough (MinEquity)
    // 4. Recalculate drawdown: (MaxEquity - Equity) / MaxEquity × 100
}
```

### 6.3 Data Resampling

```go
// backtest/equity.go:10-50
func ResampleEquity(points []EquityPoint, timeframe string) []EquityPoint {
    // Bucket by timeframe
    // Keep last point in each bucket
}
```

---

## 7. Result Storage

**Core Files:** `backtest/storage.go`, `store/backtest.go`

### 7.1 File Storage Structure

```
backtests/
├── <run_id>/
│   ├── run.json              # Run metadata
│   ├── checkpoint.json       # Checkpoint (for resume)
│   ├── equity.jsonl          # Equity curve (line-delimited JSON)
│   ├── trades.jsonl          # Trade records (line-delimited JSON)
│   ├── metrics.json          # Performance metrics
│   ├── progress.json         # Progress info
│   ├── ai_cache.json         # AI decision cache
│   └── decision_logs/        # Decision logs
│       ├── 0.json
│       ├── 1.json
│       └── ...
```

### 7.2 Database Schema

```sql
-- Backtest run metadata
CREATE TABLE backtest_runs (
  run_id TEXT PRIMARY KEY,
  user_id TEXT,
  config_json TEXT,
  state TEXT,                    -- pending, running, completed, failed
  processed_bars INTEGER,
  progress_pct REAL,
  equity_last REAL,
  max_drawdown_pct REAL,
  liquidated BOOLEAN,
  ai_provider TEXT,
  ai_model TEXT,
  created_at DATETIME,
  updated_at DATETIME
);

-- Equity curve
CREATE TABLE backtest_equity (
  id INTEGER PRIMARY KEY,
  run_id TEXT,
  ts INTEGER,
  equity REAL,
  available REAL,
  pnl REAL,
  pnl_pct REAL,
  dd_pct REAL,
  cycle INTEGER
);

-- Trade records
CREATE TABLE backtest_trades (
  id INTEGER PRIMARY KEY,
  run_id TEXT,
  ts INTEGER,
  symbol TEXT,
  action TEXT,
  side TEXT,
  qty REAL,
  price REAL,
  fee REAL,
  slippage REAL,
  realized_pnl REAL,
  leverage INTEGER,
  liquidation BOOLEAN
);

-- Performance metrics
CREATE TABLE backtest_metrics (
  run_id TEXT PRIMARY KEY,
  payload BLOB,
  updated_at DATETIME
);

-- Checkpoints (pause/resume)
CREATE TABLE backtest_checkpoints (
  run_id TEXT PRIMARY KEY,
  payload BLOB,
  updated_at DATETIME
);
```

---

## 8. API Endpoints

**Core File:** `api/backtest.go`

### 8.1 Endpoint List

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/backtest/start` | POST | Start backtest |
| `/backtest/pause` | POST | Pause backtest |
| `/backtest/resume` | POST | Resume backtest |
| `/backtest/stop` | POST | Stop backtest |
| `/backtest/status` | GET | Get status |
| `/backtest/runs` | GET | List all backtests |
| `/backtest/equity` | GET | Get equity curve |
| `/backtest/trades` | GET | Get trade records |
| `/backtest/metrics` | GET | Get performance metrics |
| `/backtest/trace` | GET | Get decision logs |
| `/backtest/export` | GET | Export ZIP |
| `/backtest/delete` | POST | Delete backtest |

### 8.2 Request Examples

```bash
# Start backtest
POST /backtest/start
{
  "config": {
    "run_id": "bt_20231215",
    "symbols": ["BTCUSDT", "ETHUSDT"],
    "timeframes": ["3m", "15m", "4h"],
    "start_ts": 1702566000,
    "end_ts": 1702652400,
    "initial_balance": 10000,
    "ai_model_id": "model_001"
  }
}

# Get equity curve
GET /backtest/equity?run_id=bt_20231215&tf=1h&limit=1000

# Get metrics
GET /backtest/metrics?run_id=bt_20231215
```

### 8.3 Response Examples

```json
// Status response
{
  "run_id": "bt_20231215",
  "state": "running",
  "progress_pct": 45.5,
  "processed_bars": 1234,
  "equity": 10234.50,
  "unrealized_pnl": 234.50
}

// Metrics response
{
  "total_return_pct": 12.34,
  "max_drawdown_pct": 5.67,
  "sharpe_ratio": 1.89,
  "profit_factor": 2.34,
  "win_rate": 65.5,
  "trades": 123
}
```

---

## 9. Account & Position Management

**Core File:** `backtest/account.go`

### 9.1 Position Structure

```go
type position struct {
    Symbol           string
    Side             string     // "long" or "short"
    Quantity         float64
    EntryPrice       float64
    Leverage         int
    Margin           float64    // Margin
    Notional         float64    // Notional value
    LiquidationPrice float64    // Liquidation price
    OpenTime         int64
}
```

### 9.2 Open Position Logic

```go
// backtest/account.go:61-104
func (a *BacktestAccount) Open(symbol, side string, qty, price float64, leverage int) {
    // 1. Apply slippage
    // 2. Calculate notional value (qty × price)
    // 3. Calculate margin (notional / leverage)
    // 4. Deduct margin + fees
    // 5. Create/add to position
    // 6. Calculate liquidation price
}
```

### 9.3 Close Position Logic

```go
// backtest/account.go:106-140
func (a *BacktestAccount) Close(symbol, side string, qty, price float64) {
    // 1. Verify position exists
    // 2. Apply slippage (reverse direction)
    // 3. Calculate realized PnL
    //    long:  (exit - entry) × qty
    //    short: (entry - exit) × qty
    // 4. Return margin + PnL - fees
    // 5. Update/delete position
}
```

### 9.4 Liquidation Price Calculation

```go
// backtest/account.go:177-186
func computeLiquidation(entry float64, leverage int, side string) float64 {
    if side == "long" {
        return entry * (1 - 1.0/float64(leverage))  // Long: liquidate on drop
    }
    return entry * (1 + 1.0/float64(leverage))      // Short: liquidate on rise
}
```

---

## 10. Checkpoint & Resume

**Core File:** `backtest/runner.go`

### 10.1 Checkpoint Structure

```json
{
  "bar_index": 1234,
  "bar_ts": 1702609200000,
  "cash": 8000.00,
  "equity": 10234.50,
  "max_equity": 10500.00,
  "max_drawdown_pct": 5.67,
  "positions": [...],
  "decision_cycle": 62,
  "liquidated": false
}
```

### 10.2 Checkpoint Trigger

```go
// backtest/runner.go:874-898
func (r *Runner) maybeCheckpoint() {
    // Save every N bars
    // Or save every N seconds
}
```

### 10.3 Resume Flow

```go
func (r *Runner) RestoreFromCheckpoint() {
    // 1. Load checkpoint
    // 2. Restore account state
    // 3. Restore bar index (continue from next bar)
    // 4. Restore equity curve, trade records
}
```

---

## Core File Index

| Module | File | Key Methods |
|--------|------|-------------|
| **Config** | `backtest/config.go` | `BacktestConfig`, `Validate()` |
| **Data Loading** | `backtest/datafeed.go` | `NewDataFeed()`, `loadAll()`, `BuildMarketData()` |
| **Sim Engine** | `backtest/runner.go` | `Start()`, `loop()`, `stepOnce()` |
| **Decision** | `backtest/runner.go` | `buildDecisionContext()`, `invokeAIWithRetry()` |
| **Execution** | `backtest/runner.go` | `executeDecision()` |
| **Account** | `backtest/account.go` | `Open()`, `Close()`, `TotalEquity()` |
| **Metrics** | `backtest/metrics.go` | `CalculateMetrics()` |
| **Equity** | `backtest/equity.go` | `ResampleEquity()`, `LimitEquityPoints()` |
| **Storage** | `backtest/storage.go` | `SaveCheckpoint()`, `appendEquityPoint()` |
| **Database** | `store/backtest.go` | Schema and CRUD operations |
| **API** | `api/backtest.go` | HTTP handlers |
| **AI Cache** | `backtest/aicache.go` | `Get()`, `Put()`, `save()` |

---

**Document Version:** 1.0.0
**Last Updated:** 2025-01-15
