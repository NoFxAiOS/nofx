# ENHANCE PROPOSAL: Comprehensive Performance Metrics & Real-time Feedback System

## 1. Problem Statement

### 1.1 Current Limitations

The trading system TopTrader has a **disabled feedback loop**:

```
AI Decision Generation
  ↓
Execute Trade
  ↓
Record Result
  ↓
Analyze Stats (but NOT injected back to AI)
```

### 1.2 Specific Deficiencies

| Issue | Impact | Severity |
|-------|--------|----------|
| **No Sharpe Ratio calculation** | Cannot measure risk-adjusted returns | HIGH |
| **No real-time perf injection** | AI generates decisions without current stats context | CRITICAL |
| **No symbol-specific params** | BTC/ALT treated identically despite different risk profiles | HIGH |
| **No hard loss limits** | 5+ consecutive losses not prevented system-wide | HIGH |
| **No perf snapshots** | Historical performance trends not tracked | MEDIUM |
| **No volatility-aware sizing** | Position sizing ignores market conditions | MEDIUM |

---

## 2. Root Cause Analysis (Three-Layer)

### Layer 1: Phenomenon (What users observe)
- AI makes decisions with 30% win rate despite having data showing 25% win rate
- No account equity or drawdown shown in decision logs
- Same coin size used for BTC and DOGE despite vastly different volatilities
- 5+ consecutive losses still allowed to execute

### Layer 2: Essence (System architecture problem)
- **Disconnected Pipeline**: Stats calculated but never fed back to Decision Engine
- **Missing Lookups**: No query to get current metrics for this specific trader
- **Static Prompts**: System Prompt never includes real metrics
- **No Constraints**: Consecutive loss tracking not implemented as hard stop

### Layer 3: Philosophy (Design principle violation)
**Good Design**: Complete feedback loop - Measure → Learn → Adapt → Measure
**Bad Design**: Linear pipeline - Generate → Execute → Measure (forgotten)

This violates the principle: **Information should flow bidirectionally**

---

## 3. Proposed Comprehensive Solution

### 3.1 Architecture Changes

```
Current (Broken):
┌─────────────────┐
│ AI Decision Gen │ ← Static Prompt (no context)
└────────┬────────┘
         ↓
┌─────────────────┐
│ Execute Trade   │
└────────┬────────┘
         ↓
┌─────────────────┐
│ Record Result   │
└────────┬────────┘
         ↓
┌─────────────────┐
│ Analyze Stats   │ ← Calculated but forgotten
└─────────────────┘


Proposed (Complete Loop):
┌─────────────────────────────────┐
│ Query Recent Metrics            │ ← NEW!
│ (Win rate, Sharpe, Max DD, etc) │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ AI Decision Gen                 │ ← Dynamic Prompt with metrics
│ (With real-time context)        │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Execute Trade (with constraints)│ ← NEW: Hard loss limits
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Record Result                   │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Analyze & Store Metrics         │ ← NEW: Store snapshots
│ (Sharpe, DD, Perf by symbol)    │
└─────────────────────────────────┘
```

### 3.2 Four-Layer Implementation

#### Layer 1: Metrics Calculation Enhancement
**File**: `decision/analysis/trade_analyzer.go`

Add missing metrics:
```go
// Sharpe Ratio (risk-adjusted return)
func (ta *TradeAnalyzer) CalculateSharpeRatio(trades []database.TradeRecord) float64
// Returns (AvgReturn - RiskFree) / StdDev * sqrt(252)

// Consecutive Losses
func (ta *TradeAnalyzer) GetConsecutiveLosses(trades []database.TradeRecord) int
// Count from most recent trade backwards

// Max Drawdown Enhancement
func (ta *TradeAnalyzer) CalculateMaxDrawdownPercent(trades []database.TradeRecord) float64
// More accurate: peak-to-trough

// Symbol-specific stats
func (ta *TradeAnalyzer) GetSymbolStats(trades []database.TradeRecord, symbol string) *SymbolStats
// Win rate, avg profit, volatility per symbol
```

---

#### Layer 2: Dynamic Prompt Injection
**File**: `decision/engine.go` - Modify `buildUserPrompt()`

Inject real metrics before sending to AI:
```
## 📊 实时交易表现 (系统自动注入)

### 账户状态
- 当前净值: 95.23 USDT
- 初始资金: 100 USDT
- 今日PnL: -2.1%
- 周PnL: +1.5%
- 月PnL: -4.77%

### 策略表现
- 总交易笔数: 47
- 胜率: 31.9%
- 加权胜率(衰减): 33.2%
- 盈亏比: 1.32
- 利润因子: 0.89
- 夏普比率: 0.32
- 最大回撤: 14.54%
- 连续亏损: 2笔

### 品种表现 (按胜率排序)
1. BTCUSDT: 胜率 35%, 平均盈利 0.45%, 最大亏损 -2.1%
2. ETHUSDT: 胜率 28%, 平均盈利 0.32%, 最大亏损 -1.8%
3. SOLUSDT: 胜率 25%, 平均盈利 0.28%, 最大亏损 -3.2%
```

---

#### Layer 3: Hard Loss Constraints
**File**: New file `trader/loss_circuit_breaker.go`

Implement hard stops:
```go
type LossCircuitBreaker struct {
    traderID string
    
    // Hard stops
    MaxConsecutiveLosses   int     // Default: 5
    MaxDailyLossPercent    float64 // Default: 12%
    MaxWeeklyLossPercent   float64 // Default: 20%
    MaxDrawdownPercent     float64 // Default: 15%
    
    // Tracking
    currentConsecutiveLosses int
    todayPnLPercent         float64
    currentDrawdownPercent  float64
}

// CanTrade() checks all hard limits
// Returns (allowed bool, reason string)
```

---

#### Layer 4: Performance Snapshots
**File**: `database/` - New migrations

Create tables for historical tracking:
```sql
-- Performance snapshots (daily)
CREATE TABLE performance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    trader_id TEXT NOT NULL,
    snapshot_date DATE NOT NULL,
    total_trades INT,
    win_rate NUMERIC(5,2),
    weighted_win_rate NUMERIC(5,2),
    profit_factor NUMERIC(6,2),
    sharpe_ratio NUMERIC(5,2),
    max_drawdown NUMERIC(5,2),
    daily_pnl NUMERIC(10,4),
    weekly_pnl NUMERIC(10,4),
    monthly_pnl NUMERIC(10,4),
    account_value NUMERIC(15,8),
    UNIQUE(trader_id, snapshot_date)
);

-- Symbol-level performance
CREATE TABLE symbol_performance (
    id BIGSERIAL PRIMARY KEY,
    trader_id TEXT NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    trades_count INT,
    win_rate NUMERIC(5,2),
    avg_profit_pct NUMERIC(6,4),
    avg_loss_pct NUMERIC(6,4),
    best_trade_pct NUMERIC(8,4),
    worst_trade_pct NUMERIC(8,4),
    volatility NUMERIC(6,4),
    UNIQUE(trader_id, symbol)
);

-- Loss event log (for circuit breaker)
CREATE TABLE loss_events (
    id BIGSERIAL PRIMARY KEY,
    trader_id TEXT NOT NULL,
    event_type VARCHAR(50), -- 'consecutive_loss', 'daily_limit', 'weekly_limit', 'drawdown'
    trades_count INT,
    pnl_percent NUMERIC(10,4),
    triggered_at TIMESTAMP DEFAULT NOW()
);
```

---

## 4. Detailed Implementation Plan

### Step 1: Metrics Enhancement (decision/analysis/trade_analyzer.go)

```go
// Add these methods to TradeAnalyzer

func (ta *TradeAnalyzer) CalculateSharpeRatio(trades []database.TradeRecord, riskFreeRate float64) float64 {
    if len(trades) < 2 {
        return 0
    }
    
    // Calculate daily returns
    var returns []float64
    for _, t := range trades {
        returns = append(returns, t.ProfitPct)
    }
    
    // Mean return
    var sum float64
    for _, r := range returns {
        sum += r
    }
    mean := sum / float64(len(returns))
    
    // Standard deviation
    var variance float64
    for _, r := range returns {
        variance += math.Pow(r-mean, 2)
    }
    stdDev := math.Sqrt(variance / float64(len(returns)))
    
    if stdDev == 0 {
        return 0
    }
    
    // Annualize: (mean - rf) / stddev * sqrt(252)
    return (mean - riskFreeRate) / stdDev * math.Sqrt(252)
}

func (ta *TradeAnalyzer) GetConsecutiveLosses(trades []database.TradeRecord) int {
    if len(trades) == 0 {
        return 0
    }
    
    count := 0
    for i := len(trades) - 1; i >= 0; i-- {
        if trades[i].ProfitPct < 0 {
            count++
        } else {
            break
        }
    }
    return count
}

func (ta *TradeAnalyzer) GetSymbolStats(trades []database.TradeRecord, symbol string) *SymbolStats {
    stats := &SymbolStats{Symbol: symbol}
    
    var symbolTrades []database.TradeRecord
    for _, t := range trades {
        if t.Symbol == symbol {
            symbolTrades = append(symbolTrades, t)
        }
    }
    
    if len(symbolTrades) == 0 {
        return stats
    }
    
    // Calculate stats
    stats.TradesCount = len(symbolTrades)
    
    var wins, totalProfit, totalLoss, bestTrade, worstTrade float64
    bestTrade = -999999
    worstTrade = 999999
    
    for _, t := range symbolTrades {
        if t.ProfitPct > 0 {
            wins++
            totalProfit += t.ProfitPct
        } else {
            totalLoss += math.Abs(t.ProfitPct)
        }
        
        if t.ProfitPct > bestTrade {
            bestTrade = t.ProfitPct
        }
        if t.ProfitPct < worstTrade {
            worstTrade = t.ProfitPct
        }
    }
    
    stats.WinRate = (wins / float64(len(symbolTrades))) * 100
    stats.AvgProfitPct = totalProfit / wins if wins > 0
    stats.AvgLossPct = totalLoss / (float64(len(symbolTrades)) - wins)
    stats.BestTradePct = bestTrade
    stats.WorstTradePct = worstTrade
    
    // Calculate volatility (for symbol-specific sizing)
    var variance float64
    mean := (totalProfit - totalLoss) / float64(len(symbolTrades))
    for _, t := range symbolTrades {
        variance += math.Pow(t.ProfitPct-mean, 2)
    }
    stats.Volatility = math.Sqrt(variance / float64(len(symbolTrades)))
    
    return stats
}
```

---

### Step 2: Circuit Breaker (trader/loss_circuit_breaker.go)

```go
package trader

import (
    "fmt"
    "log"
    "nofx/database"
    "time"
)

type LossCircuitBreaker struct {
    traderID string
    db       *database.Database
    
    // Hard limits
    MaxConsecutiveLosses   int
    MaxDailyLossPercent    float64
    MaxWeeklyLossPercent   float64
    MaxDrawdownPercent     float64
    
    // Current state
    consecutiveLosses      int
    todayPnLPercent        float64
    weeklyPnLPercent       float64
    currentDrawdownPercent float64
    accountPeak            float64
}

func NewLossCircuitBreaker(traderID string, db *database.Database) *LossCircuitBreaker {
    return &LossCircuitBreaker{
        traderID:               traderID,
        db:                     db,
        MaxConsecutiveLosses:   5,
        MaxDailyLossPercent:    12.0,
        MaxWeeklyLossPercent:   20.0,
        MaxDrawdownPercent:     15.0,
    }
}

// CanTrade checks all hard limits and returns permission + reason
func (lcb *LossCircuitBreaker) CanTrade() (bool, string) {
    // Check consecutive losses
    if lcb.consecutiveLosses >= lcb.MaxConsecutiveLosses {
        return false, fmt.Sprintf(
            "🚨 Hard stop: %d consecutive losses (limit: %d)",
            lcb.consecutiveLosses, lcb.MaxConsecutiveLosses)
    }
    
    // Check daily loss limit
    if lcb.todayPnLPercent < -lcb.MaxDailyLossPercent {
        return false, fmt.Sprintf(
            "🚨 Hard stop: Daily loss %.2f%% exceeds limit %.2f%%",
            lcb.todayPnLPercent, lcb.MaxDailyLossPercent)
    }
    
    // Check weekly loss limit
    if lcb.weeklyPnLPercent < -lcb.MaxWeeklyLossPercent {
        return false, fmt.Sprintf(
            "🚨 Hard stop: Weekly loss %.2f%% exceeds limit %.2f%%",
            lcb.weeklyPnLPercent, lcb.MaxWeeklyLossPercent)
    }
    
    // Check drawdown limit
    if lcb.currentDrawdownPercent > lcb.MaxDrawdownPercent {
        return false, fmt.Sprintf(
            "🚨 Hard stop: Drawdown %.2f%% exceeds limit %.2f%%",
            lcb.currentDrawdownPercent, lcb.MaxDrawdownPercent)
    }
    
    return true, ""
}

// UpdateAfterTrade updates circuit breaker after trade execution
func (lcb *LossCircuitBreaker) UpdateAfterTrade(trade *database.TradeRecord, currentAccountValue float64) {
    // Update consecutive losses
    if trade.ProfitPct < 0 {
        lcb.consecutiveLosses++
        
        if lcb.consecutiveLosses >= lcb.MaxConsecutiveLosses {
            log.Printf("⛔ Circuit breaker triggered: %d consecutive losses", lcb.consecutiveLosses)
            lcb.logLossEvent("consecutive_loss", lcb.consecutiveLosses, trade.ProfitPct)
        }
    } else {
        lcb.consecutiveLosses = 0
    }
    
    // Update daily/weekly PnL
    lcb.updatePeriodPnL()
    
    // Update drawdown
    if currentAccountValue > lcb.accountPeak {
        lcb.accountPeak = currentAccountValue
    }
    lcb.currentDrawdownPercent = ((lcb.accountPeak - currentAccountValue) / lcb.accountPeak) * 100
}

func (lcb *LossCircuitBreaker) updatePeriodPnL() {
    // Query today's trades and calculate PnL
    // Query this week's trades and calculate PnL
    // (Implementation depends on database structure)
}

func (lcb *LossCircuitBreaker) logLossEvent(eventType string, count int, pnlPct float64) {
    // Insert into loss_events table
    // log to database for monitoring
}
```

---

### Step 3: Performance Snapshot Manager

**File**: New `logger/performance_snapshot_manager.go`

```go
package logger

import (
    "log"
    "nofx/database"
    "nofx/decision/analysis"
    "time"
)

type PerformanceSnapshotManager struct {
    db       *database.Database
    traderID string
}

func NewPerformanceSnapshotManager(db *database.Database, traderID string) *PerformanceSnapshotManager {
    return &PerformanceSnapshotManager{
        db:       db,
        traderID: traderID,
    }
}

// TakeDailySnapshot captures current performance metrics
func (psm *PerformanceSnapshotManager) TakeDailySnapshot(stats *analysis.TradeAnalysisResult) error {
    snapshot := map[string]interface{}{
        "trader_id":         psm.traderID,
        "snapshot_date":     time.Now().Format("2006-01-02"),
        "total_trades":      stats.TotalTrades,
        "win_rate":          stats.WinRate,
        "profit_factor":     stats.ProfitFactor,
        "sharpe_ratio":      stats.SharpeRatio, // NEW
        "max_drawdown":      stats.MaxDrawdownPercent, // NEW
        "daily_pnl":         stats.DailyPnL,
        "weekly_pnl":        stats.WeeklyPnL,
        "monthly_pnl":       stats.MonthlyPnL,
        "account_value":     stats.CurrentAccountValue,
    }
    
    // Insert into performance_snapshots table
    return psm.db.InsertPerformanceSnapshot(snapshot)
}

// UpdateSymbolStats updates per-symbol performance
func (psm *PerformanceSnapshotManager) UpdateSymbolStats(symbolStats map[string]*analysis.SymbolStats) error {
    for symbol, stats := range symbolStats {
        record := map[string]interface{}{
            "trader_id":       psm.traderID,
            "symbol":          symbol,
            "trades_count":    stats.TradesCount,
            "win_rate":        stats.WinRate,
            "avg_profit_pct":  stats.AvgProfitPct,
            "avg_loss_pct":    stats.AvgLossPct,
            "best_trade_pct":  stats.BestTradePct,
            "worst_trade_pct": stats.WorstTradePct,
            "volatility":      stats.Volatility,
        }
        
        if err := psm.db.UpsertSymbolPerformance(record); err != nil {
            log.Printf("Failed to upsert symbol stats for %s: %v", symbol, err)
        }
    }
    return nil
}
```

---

### Step 4: Dynamic Prompt Injection

**File**: Modify `decision/engine.go` - `buildUserPrompt()`

```go
// In buildUserPrompt(), after existing account info, add:

// 📊 实时交易表现注入
if ctx.Performance != nil {
    sb.WriteString("## 📊 实时交易表现\n\n")
    
    perf := ctx.Performance.(map[string]interface{})
    
    // Account metrics
    sb.WriteString(fmt.Sprintf("### 账户状态\n"))
    sb.WriteString(fmt.Sprintf("- 当前净值: %.2f USDT\n", perf["account_value"]))
    sb.WriteString(fmt.Sprintf("- 初始资金: 100 USDT\n"))
    sb.WriteString(fmt.Sprintf("- 今日PnL: %+.2f%%\n", perf["daily_pnl"]))
    sb.WriteString(fmt.Sprintf("- 周PnL: %+.2f%%\n", perf["weekly_pnl"]))
    sb.WriteString(fmt.Sprintf("- 月PnL: %+.2f%%\n\n", perf["monthly_pnl"]))
    
    // Strategy performance
    sb.WriteString(fmt.Sprintf("### 策略表现\n"))
    sb.WriteString(fmt.Sprintf("- 总交易笔数: %d\n", perf["total_trades"]))
    sb.WriteString(fmt.Sprintf("- 胜率: %.1f%%\n", perf["win_rate"]))
    sb.WriteString(fmt.Sprintf("- 加权胜率: %.1f%%\n", perf["weighted_win_rate"]))
    sb.WriteString(fmt.Sprintf("- 盈亏比: %.2f\n", perf["profit_ratio"]))
    sb.WriteString(fmt.Sprintf("- 利润因子: %.2f\n", perf["profit_factor"]))
    sb.WriteString(fmt.Sprintf("- 夏普比率: %.2f\n", perf["sharpe_ratio"]))
    sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n\n", perf["max_drawdown"]))
    
    // Symbol-specific metrics (if available)
    // ... (format symbol stats)
}
```

---

## 5. Implementation Priority

| Phase | Task | Effort | Impact | Dependencies |
|-------|------|--------|--------|--------------|
| 1️⃣ | Add Sharpe/Consecutive Loss calcs | 2h | HIGH | None |
| 2️⃣ | Circuit Breaker foundation | 3h | CRITICAL | Phase 1 |
| 3️⃣ | DB tables + migrations | 2h | MEDIUM | None |
| 4️⃣ | Dynamic prompt injection | 2h | CRITICAL | Phase 1 |
| 5️⃣ | Symbol-specific sizing logic | 3h | MEDIUM | Phase 1 |
| 6️⃣ | Snapshot manager | 1h | LOW | Phase 3 |

**Total Effort**: ~13 hours | **Deployable Phase**: After Phase 2

---

## 6. Expected Outcomes

### Before Optimization
- ❌ AI unaware of current performance
- ❌ 5+ consecutive losses still execute
- ❌ No risk-adjusted metrics
- ❌ All symbols sized equally

### After Optimization
- ✅ AI sees real-time metrics in every decision
- ✅ Circuit breaker prevents catastrophic loss streaks
- ✅ Sharpe ratio guides risk-adjusted entries
- ✅ Symbol volatility adjusts position sizing

### Quantified Impact
- **Decision Quality**: AI aware of context → better decisions
- **Risk Control**: 5+ loss streak prevented → preserved capital
- **Performance Tracking**: Daily snapshots → identify trends early
- **Learning**: AI receives performance feedback → can adapt

---

## 7. Testing Strategy

### Unit Tests
```bash
# Test Sharpe Ratio
go test -run TestSharpeRatio decision/analysis/trade_analyzer_test.go

# Test Circuit Breaker
go test -run TestLossCircuitBreaker trader/loss_circuit_breaker_test.go

# Test Symbol Stats
go test -run TestSymbolStats decision/analysis/trade_analyzer_test.go
```

### Integration Tests
```bash
# Run backtest with new metrics injected
go run p0_backtest.go | grep "Sharpe\|Circuit\|Consecutive"

# Verify prompt injection
grep "📊 实时交易表现" decision_logs/*.json
```

---

## 8. Rollout Plan

### Stage 1: Metrics Only (Low Risk)
- Deploy calculation functions
- Enable Sharpe ratio logging
- Monitor for 1 week

### Stage 2: Add Circuit Breaker
- Deploy LossCircuitBreaker
- Start with logging only (non-blocking)
- Monitor rejection rates

### Stage 3: Enable Blocking
- Flip circuit breaker to blocking mode
- Monitor: trade rejection reasons
- Adjust thresholds if needed

### Stage 4: Full Injection
- Enable prompt injection
- Monitor: decision quality changes
- Measure Sharpe ratio improvement

---

## 9. Success Criteria

✅ Sharpe Ratio calculated daily (non-zero for >10 trades)
✅ Consecutive loss limit prevents 5+ loss sequences
✅ AI receives performance metrics in every prompt
✅ Symbol stats show accurate win rates per coin
✅ Performance snapshots stored daily
✅ No increase in trade rejection rate (< 5%)

---

## 10. Known Risks & Mitigations

| Risk | Probability | Mitigation |
|------|-------------|-----------|
| Circuit breaker too strict | MEDIUM | Start with logging, tune after 1 week |
| Sharpe calc errors | LOW | Unit tests cover edge cases |
| Prompt injection too long | MEDIUM | Compress metrics, only include last 10 trades |
| DB migration issues | LOW | Run migration in test first, backup before prod |

---

## Author Notes

This proposal implements the **complete feedback loop** missing from current system:
- Measure performance accurately (including Sharpe)
- Inject real metrics to AI decision engine
- Add hard stops for catastrophic loss events
- Track historical trends for learning

This transforms the system from **linear pipeline** → **closed-loop learning system**.

