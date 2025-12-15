# NOFX Strategy Module - Technical Documentation

**Language:** [English](STRATEGY_MODULE.md) | [中文](STRATEGY_MODULE.zh-CN.md)

## Overview

This document describes the complete data flow of the NOFX strategy module, including coin selection, data assembly, prompt construction, AI request, response parsing, and decision execution.

---

## Complete Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Trading Cycle (Every N Minutes)              │
└─────────────────────────────────────────────────────────────────┘

1. Coin Selection (GetCandidateCoins)
   ├─ Static (Static list)
   ├─ AI500 Pool (AI rating pool)
   ├─ OI Top (Position growth ranking)
   └─ Mixed (Mixed mode)
        ↓
2. Data Assembly (buildTradingContext)
   ├─ Account balance → equity, available, unrealizedPnL
   ├─ Current positions → symbol, side, entry, mark, qty, leverage
   ├─ K-line data → OHLCV (5m, 15m, 1h, 4h)
   ├─ Technical indicators → EMA, MACD, RSI, ATR, Volume
   ├─ On-chain data → OI, Funding Rate
   ├─ Quant data → Capital flow, OI changes (optional)
   └─ Recent trades → Last 10 closed trades
        ↓
3. System Prompt (BuildSystemPrompt)
   ├─ Role definition
   ├─ Trading mode (aggressive/conservative/scalping)
   ├─ Hard constraints (code enforced)
   ├─ AI guidance (suggested values)
   ├─ Trading frequency
   ├─ Entry standards
   ├─ Decision process
   └─ Output format (XML + JSON)
        ↓
4. User Prompt (BuildUserPrompt)
   ├─ System status (time, cycle number)
   ├─ BTC market overview
   ├─ Account information
   ├─ Current positions (with indicators)
   ├─ Candidate coins (full market data)
   └─ "Please analyze and output decisions..."
        ↓
5. AI Request (CallWithMessages)
   ├─ Select AI model
   ├─ POST: system_prompt + user_prompt
   ├─ Timeout: 120s, Retries: 3
   └─ Return raw response
        ↓
6. AI Parsing (parseFullDecisionResponse)
   ├─ Extract Chain of Thought <reasoning>
   ├─ Extract JSON decision <decision>
   ├─ Fix character encoding
   ├─ Validate JSON format
   ├─ Parse decision array
   └─ Validate risk parameters
        ↓
7. Decision Execution
   ├─ Sort: Close first → Open → hold/wait
   ├─ Risk control enforcement
   ├─ Submit orders
   ├─ Confirm fills
   └─ Record to database
```

---

## 1. Coin Selection

**Core File:** `decision/engine.go:380-454`

**Entry Method:** `StrategyEngine.GetCandidateCoins()`

### 1.1 Static Coin List

```go
// decision/engine.go:395-403
if config.CoinSource.SourceType == "static" {
    for _, symbol := range config.CoinSource.StaticCoins {
        coins = append(coins, CandidateCoin{
            Symbol:  market.Normalize(symbol),
            Sources: []string{"static"},
        })
    }
}
```

- **Config:** `StrategyConfig.CoinSource.StaticCoins`
- **Usage:** Manually specify trading coins
- **Tag:** `["static"]`

### 1.2 AI500 Coin Pool

```go
// decision/engine.go:405-406, 456-474
func (e *StrategyEngine) getCoinPoolCoins(limit int) []CandidateCoin {
    coins, err := e.provider.GetTopRatedCoins(limit)
    // ...
    for _, coin := range coins {
        result = append(result, CandidateCoin{
            Symbol:  coin.Symbol,
            Sources: []string{"ai500"},
        })
    }
}
```

- **API:** `config.CoinSource.CoinPoolAPIURL`
- **Usage:** Get top N coins by AI rating
- **Tag:** `["ai500"]`

### 1.3 OI Top Coins (Position Growth Ranking)

```go
// decision/engine.go:408-409, 476-498
func (e *StrategyEngine) getOITopCoins() []CandidateCoin {
    positions, err := e.provider.GetOITopPositions()
    // ...
    for _, pos := range positions {
        result = append(result, CandidateCoin{
            Symbol:  pos.Symbol,
            Sources: []string{"oi_top"},
        })
    }
}
```

- **API:** `config.CoinSource.OITopAPIURL`
- **Usage:** Get coins with fastest OI growth
- **Tag:** `["oi_top"]`

### 1.4 Mixed Mode

```go
// decision/engine.go:411-449
if config.CoinSource.SourceType == "mixed" {
    if config.CoinSource.UseCoinPool {
        // Add AI500 coins
    }
    if config.CoinSource.UseOITop {
        // Add OI Top coins
    }
    if len(config.CoinSource.StaticCoins) > 0 {
        // Add static coins
    }
    // Deduplicate and merge, keep multi-source tags
}
```

- **Feature:** Use multiple data sources simultaneously
- **Tag Example:** `["ai500", "oi_top"]` (dual signal coin)

---

## 2. Data Assembly

**Core File:** `trader/auto_trader.go:562-791`, `decision/engine.go:299-374`

**Entry Method:** `AutoTrader.buildTradingContext()`

### 2.1 Account Data

```go
// trader/auto_trader.go:565-583
balance, err := at.trader.GetBalance()
equity := balance["total_equity"].(float64)
available := balance["available_balance"].(float64)
unrealizedPnL := balance["total_pnl"].(float64)
```

**Extracted Fields:**
- `total_equity` - Total account equity
- `available_balance` - Available balance
- `total_pnl` - Unrealized PnL

### 2.2 Position Data

```go
// trader/auto_trader.go:588-682
positions, err := at.trader.GetPositions()
for _, pos := range positions {
    position := decision.Position{
        Symbol:           pos.Symbol,
        Side:             pos.Side,          // "long" / "short"
        EntryPrice:       pos.EntryPrice,
        MarkPrice:        pos.MarkPrice,
        Quantity:         pos.Quantity,
        Leverage:         pos.Leverage,
        UnrealizedPnL:    pos.UnrealizedPnL,
        LiquidationPrice: pos.LiquidationPrice,
    }
}
```

### 2.3 Market Data Fetching

```go
// decision/engine.go:299-374
func (e *StrategyEngine) fetchMarketDataWithStrategy(symbols []string) map[string]*market.Data {
    timeframes := config.Indicators.Klines.SelectedTimeframes  // ["5m", "15m", "1h", "4h"]
    primaryTF := config.Indicators.Klines.PrimaryTimeframe     // "5m"
    count := config.Indicators.Klines.PrimaryCount             // 30

    for _, symbol := range symbols {
        data := market.GetWithTimeframes(symbol, timeframes, primaryTF, count)
        result[symbol] = data
    }
}
```

### 2.4 Technical Indicator Calculation

**File:** `market/data.go:59-98`

| Indicator | Config | Calculation |
|-----------|--------|-------------|
| **EMA** | `EnableEMA`, `EMAPeriods` | `calculateEMA(klines, period)` |
| **MACD** | `EnableMACD` | `calculateMACD(klines)` - 12/26/9 |
| **RSI** | `EnableRSI`, `RSIPeriods` | `calculateRSI(klines, period)` |
| **ATR** | `EnableATR`, `ATRPeriods` | `calculateATR(klines, period)` |
| **Volume** | `EnableVolume` | Raw volume data |
| **OI** | `EnableOI` | Open interest data |
| **Funding Rate** | `EnableFundingRate` | Funding rate |

### 2.5 Quant Data (Optional)

```go
// trader/auto_trader.go:759-778
if config.Indicators.EnableQuantData {
    quantData := provider.GetQuantData(symbol)
    // Contains: Capital flow, OI changes, Price changes
}
```

**Data Structure:**
```go
QuantData {
    Netflow {
        Institution: {Future, Spot},  // Institutional flow
        Personal: {Future, Spot}      // Retail flow
    },
    OI {
        CurrentOI: float64,
        Delta: {1h, 4h, 24h}          // OI changes
    },
    PriceChange {
        "1h", "4h", "24h": float64    // Price change %
    }
}
```

---

## 3. System Prompt

**Core File:** `decision/engine.go:700-818`

**Entry Method:** `StrategyEngine.BuildSystemPrompt(accountEquity, variant)`

### 3.1 Prompt Structure (8 Sections)

```
1. Role Definition          [Editable]
2. Trading Mode Variant     [Runtime determined]
3. Hard Constraints         [Code enforced + AI guided]
4. Trading Frequency        [Editable]
5. Entry Standards          [Editable]
6. Decision Process         [Editable]
7. Output Format            [Fixed XML + JSON structure]
8. Custom Prompt            [Optional]
```

### 3.2 Role Definition

```go
// decision/engine.go:706-713
roleDefinition := config.PromptSections.RoleDefinition
if roleDefinition == "" {
    roleDefinition = "You are a professional cryptocurrency trading AI..."
}
```

### 3.3 Trading Mode Variants

| Mode | Characteristics |
|------|-----------------|
| `aggressive` | Trend breakout, higher position tolerance |
| `conservative` | Multi-signal confirmation, conservative money management |
| `scalping` | Short-term momentum, tight take-profit |

### 3.4 Hard Constraints

**Code Enforced:**

```go
// decision/engine.go:725-749
maxPositions := config.RiskControl.MaxPositions           // Default: 3
altcoinMaxRatio := config.RiskControl.AltcoinMaxPositionValueRatio  // Default: 1.0
btcethMaxRatio := config.RiskControl.BTCETHMaxPositionValueRatio    // Default: 5.0
maxMarginUsage := config.RiskControl.MaxMarginUsage       // Default: 90%
minPositionSize := config.RiskControl.MinPositionSize     // Default: 12 USDT
```

**AI Guided (Suggested Values):**

```go
altcoinMaxLeverage := config.RiskControl.AltcoinMaxLeverage  // Default: 5x
btcethMaxLeverage := config.RiskControl.BTCETHMaxLeverage    // Default: 5x
minRiskRewardRatio := config.RiskControl.MinRiskRewardRatio  // Default: 1:3
minConfidence := config.RiskControl.MinConfidence            // Default: 75
```

### 3.5 Output Format Requirements

```xml
<reasoning>
[Chain of Thought analysis process]
</reasoning>

<decision>
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "open_long",
    "leverage": 5,
    "position_size_usd": 100.00,
    "stop_loss": 65000.00,
    "take_profit": 72000.00,
    "confidence": 85,
    "risk_usd": 20.00,
    "reasoning": "..."
  }
]
```
</decision>
```

---

## 4. User Prompt

**Core File:** `decision/engine.go:884-1007`

**Entry Method:** `StrategyEngine.BuildUserPrompt(ctx)`

### 4.1 Prompt Content Structure

```
1. System Status           [Time, cycle number, runtime]
2. BTC Market Overview     [Price, change%, MACD, RSI]
3. Account Info            [Equity, balance%, PnL%, margin%, positions]
4. Recent Trades           [Last 10 closed trades]
5. Current Positions       [Detailed position data + indicators]
6. Candidate Coins         [Full market data]
7. Quant Data              [Capital flow, OI data] (optional)
8. OI Ranking Data         [Market OI change ranking] (optional)
```

### 4.2 Account Info Format

```
Account: Equity 1000.00 | Balance 800.00 (80.0%) | PnL +5.5% | Margin 20.0% | Positions 2
```

### 4.3 Position Info Format

```
1. BTCUSDT LONG | Entry 68000.0000 Current 69500.0000
   Qty 0.0100 | Position Value $695.00
   PnL +2.21% | Amount +$15.00
   Peak PnL +3.50% | Leverage 5x
   Margin $139.00 | Liquidation Price 55000.0000
   Holding Duration 2 hours 30 minutes

   Market: price=69500, ema20=68800, macd=150.5, rsi7=62.3
   OI: Latest=15000000, Avg=14500000
   Funding Rate: 0.0100%
```

### 4.4 Candidate Coin Format

```
### 1. ETHUSDT (AI500+OI_Top dual signal)

current_price = 3500.00, current_ema20 = 3450.00, current_macd = 25.5, current_rsi7 = 58.0

Open Interest: Latest: 8500000.00 Average: 8200000.00
Funding Rate: 0.0050

=== 5M TIMEFRAME (oldest → latest) ===
Prices: [3480, 3485, 3490, 3495, 3500]
Volumes: [1000, 1200, 1100, 1300, 1150]
EMA20: [3470, 3475, 3478, 3482, 3485]
MACD: [20.1, 21.5, 22.8, 24.0, 25.5]
RSI7: [55.0, 56.2, 57.1, 57.8, 58.0]

=== 15M TIMEFRAME ===
...
```

---

## 5. AI Request

**Core File:** `decision/engine.go:222-293`, `mcp/client.go:136-150`

### 5.1 Request Flow

```go
// decision/engine.go:263-268
aiCallStart := time.Now()
aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
aiCallDuration := time.Since(aiCallStart)
```

### 5.2 Supported AI Models

| Model | Client File | Default Model |
|-------|-------------|---------------|
| **DeepSeek** | `mcp/deepseek_client.go` | deepseek-chat |
| **Qwen** | `mcp/qwen_client.go` | qwen-max |
| **Claude** | `mcp/claude_client.go` | claude-3-5-sonnet |
| **Gemini** | `mcp/gemini_client.go` | gemini-pro |
| **Grok** | `mcp/grok_client.go` | grok-beta |
| **OpenAI** | `mcp/openai_client.go` | gpt-5.2 |
| **Kimi** | `mcp/kimi_client.go` | moonshot-v1-8k |

### 5.3 Request Parameters

```go
// mcp/client.go
Timeout: 120 seconds
MaxRetries: 3
RetryDelay: 2 seconds (exponential backoff)
```

---

## 6. AI Response Parsing

**Core File:** `decision/engine.go:1303-1604`

**Entry Method:** `parseFullDecisionResponse(response, accountEquity, leverage, ratio)`

### 6.1 Parsing Flow

```
Raw AI Response (text)
    ↓
1. Extract Chain of Thought  [extractCoTTrace()]
    ↓
2. Extract JSON Decision     [extractDecisions()]
    ↓
3. Validate JSON Format      [validateJSONFormat()]
    ↓
4. Parse JSON                [json.Unmarshal()]
    ↓
5. Validate Decisions        [validateDecisions()]
    ↓
6. Build FullDecision        [Return structured result]
```

### 6.2 Chain of Thought Extraction

```go
// decision/engine.go:1327-1345
func extractCoTTrace(response string) string {
    // Priority 1: <reasoning> XML tag
    if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
        return strings.TrimSpace(match[1])
    }
    // Priority 2: Text before <decision> tag
    // Priority 3: Text before JSON [
    // Priority 4: Full response
}
```

### 6.3 JSON Decision Extraction

```go
// decision/engine.go:1347-1408
func extractDecisions(response string) (string, error) {
    // 1. Remove invisible characters
    response = removeInvisibleRunes(response)

    // 2. Fix character encoding
    response = fixMissingQuotes(response)

    // 3. Extract JSON (priority)
    //    - <decision> XML tag + ```json
    //    - Standalone ```json code block
    //    - Bare JSON array
}
```

### 6.4 Character Encoding Fix

```go
// decision/engine.go:1410-1432
func fixMissingQuotes(s string) string {
    // Chinese quotes → ASCII
    s = strings.ReplaceAll(s, """, "\"")
    s = strings.ReplaceAll(s, """, "\"")

    // Chinese brackets → ASCII
    s = strings.ReplaceAll(s, "［", "[")
    s = strings.ReplaceAll(s, "］", "]")
    s = strings.ReplaceAll(s, "｛", "{")
    s = strings.ReplaceAll(s, "｝", "}")

    // Chinese punctuation → ASCII
    s = strings.ReplaceAll(s, "：", ":")
    s = strings.ReplaceAll(s, "，", ",")
}
```

### 6.5 Decision Validation

```go
// decision/engine.go:1480-1602
func validateDecisions(decisions []Decision, equity, leverage, ratio float64) error {
    for _, d := range decisions {
        // 1. Validate action type
        validActions := []string{"open_long", "open_short", "close_long", "close_short", "hold", "wait"}

        // 2. Open position validation
        if isOpenAction(d.Action) {
            // Leverage range check
            // Position size check
            // Stop loss/take profit check
            // Risk/reward ratio check
            // Confidence check
        }

        // 3. Close position validation
        if isCloseAction(d.Action) {
            // Symbol must exist
        }
    }
}
```

### 6.6 Decision Structure

```go
// decision/engine.go:128-143
type Decision struct {
    Symbol          string   // Trading pair: "BTCUSDT"
    Action          string   // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
    Leverage        int      // Leverage multiplier
    PositionSizeUSD float64  // Position value (USDT)
    StopLoss        float64  // Stop loss price
    TakeProfit      float64  // Take profit price
    Confidence      int      // Confidence 0-100
    RiskUSD         float64  // Max risk (USDT)
    Reasoning       string   // Decision reasoning
}
```

---

## 7. Decision Execution

**Core File:** `trader/auto_trader.go:392-560`

### 7.1 Decision Sorting

```go
// trader/auto_trader.go:519-526
sort.SliceStable(decisions, func(i, j int) bool {
    priority := map[string]int{
        "close_long": 1, "close_short": 1,  // Highest priority
        "open_long": 2, "open_short": 2,    // Second priority
        "hold": 3, "wait": 3,               // Lowest priority
    }
    return priority[decisions[i].Action] < priority[decisions[j].Action]
})
```

### 7.2 Risk Control Enforcement

**File:** `trader/auto_trader.go:1769-1851`

| Check | Method | Action |
|-------|--------|--------|
| Max positions | `enforceMaxPositions()` | Reject new opens |
| Position value cap | `enforcePositionValueRatio()` | Auto reduce size |
| Min position | `enforceMinPositionSize()` | Reject small orders |
| Margin adjustment | Auto calculate | Adjust by available balance |

### 7.3 Order Execution

```go
// trader/auto_trader.go:1631-1767
func (at *AutoTrader) recordAndConfirmOrder(orderID, symbol, side, action string) {
    // 1. Poll order status (5 retries, 500ms interval)
    for i := 0; i < 5; i++ {
        status := at.trader.GetOrderStatus(orderID)
        if status.Status == "FILLED" {
            break
        }
        time.Sleep(500 * time.Millisecond)
    }

    // 2. Extract fill info
    filledPrice := status.AvgPrice
    filledQty := status.FilledQty
    fee := status.Fee

    // 3. Record to database
    at.store.Position().SaveOrder(...)
}
```

### 7.4 Decision Log Saving

```go
// trader/auto_trader.go:1235-1256
record := &store.DecisionRecord{
    CycleNumber:    cycleNumber,
    TraderID:       traderID,
    Timestamp:      time.Now(),
    SystemPrompt:   systemPrompt,     // Full system prompt
    InputPrompt:    userPrompt,       // Full user prompt
    CoTTrace:       cotTrace,         // AI chain of thought
    DecisionJSON:   decisionsJSON,    // Parsed decisions
    RawResponse:    rawResponse,      // Raw AI response
    ExecutionLog:   executionResults, // Execution results
    CandidateCoins: candidateCoins,   // Candidate coins
    Success:        success,          // Execution status
}
at.store.Decision().LogDecision(record)
```

---

## Core File Index

| Module | File | Key Methods |
|--------|------|-------------|
| **Main Loop** | `trader/auto_trader.go` | `Run()`, `runCycle()`, `buildTradingContext()` |
| **Coin Selection** | `decision/engine.go:380-454` | `GetCandidateCoins()` |
| **Data Fetching** | `market/data.go` | `Get()`, `GetWithTimeframes()` |
| **Indicator Calc** | `market/data.go:59-98` | `calculateEMA()`, `calculateMACD()`, `calculateRSI()` |
| **System Prompt** | `decision/engine.go:700-818` | `BuildSystemPrompt()` |
| **User Prompt** | `decision/engine.go:884-1007` | `BuildUserPrompt()` |
| **Market Format** | `decision/engine.go:1029-1099` | `formatMarketData()` |
| **AI Request** | `decision/engine.go:222-293` | `GetFullDecisionWithStrategy()` |
| **MCP Client** | `mcp/client.go:136-150` | `CallWithMessages()` |
| **Response Parse** | `decision/engine.go:1303-1604` | `parseFullDecisionResponse()` |
| **CoT Extract** | `decision/engine.go:1327-1345` | `extractCoTTrace()` |
| **JSON Extract** | `decision/engine.go:1347-1408` | `extractDecisions()` |
| **Decision Valid** | `decision/engine.go:1480-1602` | `validateDecisions()` |
| **Risk Enforce** | `trader/auto_trader.go:1769-1851` | `enforceMaxPositions()`, `enforcePositionValueRatio()` |
| **Strategy Config** | `store/strategy.go` | `StrategyConfig`, `RiskControlConfig` |
| **Data Provider** | `provider/data_provider.go` | `GetAI500Data()`, `GetOITopPositions()` |

---

## Configuration Reference

### Strategy Config Structure

```go
// store/strategy.go
type StrategyConfig struct {
    // Coin Source
    CoinSource struct {
        SourceType     string   // "static", "coinpool", "oi_top", "mixed"
        StaticCoins    []string // Static coin list
        UseCoinPool    bool     // Use AI500
        UseOITop       bool     // Use OI ranking
        CoinPoolLimit  int      // AI500 fetch limit
        CoinPoolAPIURL string   // AI500 API URL
        OITopAPIURL    string   // OI ranking API URL
    }

    // Technical Indicators
    Indicators struct {
        EnableEMA         bool
        EMAPeriods        []int   // [20, 50]
        EnableMACD        bool
        EnableRSI         bool
        RSIPeriods        []int   // [7, 14]
        EnableATR         bool
        ATRPeriods        []int   // [14]
        EnableVolume      bool
        EnableOI          bool
        EnableFundingRate bool
        EnableQuantData   bool
        EnableOIRanking   bool

        Klines struct {
            PrimaryTimeframe   string   // "5m"
            SelectedTimeframes []string // ["5m", "15m", "1h", "4h"]
            PrimaryCount       int      // 30
        }
    }

    // Risk Control
    RiskControl struct {
        MaxPositions               int     // Max positions
        BTCETHMaxLeverage          int     // BTC/ETH max leverage
        AltcoinMaxLeverage         int     // Altcoin max leverage
        BTCETHMaxPositionValueRatio float64 // BTC/ETH position ratio cap
        AltcoinMaxPositionValueRatio float64 // Altcoin position ratio cap
        MaxMarginUsage             float64 // Max margin usage
        MinPositionSize            float64 // Min position size
        MinRiskRewardRatio         float64 // Min risk/reward ratio
        MinConfidence              int     // Min confidence
    }

    // Prompt Sections
    PromptSections struct {
        RoleDefinition   string
        TradingFrequency string
        EntryStandards   string
        DecisionProcess  string
    }

    // Custom Prompt
    CustomPrompt string
}
```

---

**Document Version:** 1.0.0
**Last Updated:** 2025-01-15
