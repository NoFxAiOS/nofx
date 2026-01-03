# NOFX 策略模块技术文档

**语言:** [English](STRATEGY_MODULE.md) | [中文](STRATEGY_MODULE.zh-CN.md)

## 概述

本文档详细描述 NOFX 策略模块的完整数据流程，包括币种选择、数据组装、提示词构建、AI 请求、响应解析和决策执行。

---

## 完整数据流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    交易周期 (每 N 分钟)                          │
└─────────────────────────────────────────────────────────────────┘

1. 币种选择 (GetCandidateCoins)
   ├─ Static (静态列表)
   ├─ AI500 Pool (AI评分池)
   ├─ OI Top (持仓增长榜)
   └─ Mixed (混合模式)
        ↓
2. 数据组装 (buildTradingContext)
   ├─ 账户余额 → equity, available, unrealizedPnL
   ├─ 当前持仓 → symbol, side, entry, mark, qty, leverage
   ├─ K线数据 → OHLCV (5m, 15m, 1h, 4h)
   ├─ 技术指标 → EMA, MACD, RSI, ATR, Volume
   ├─ 链上数据 → OI, Funding Rate
   ├─ 量化数据 → 资金流向, OI变化 (可选)
   └─ 最近交易 → 最近10笔已平仓
        ↓
3. 系统提示词 (BuildSystemPrompt)
   ├─ 角色定义
   ├─ 交易模式 (aggressive/conservative/scalping)
   ├─ 硬性约束 (代码强制执行)
   ├─ AI引导 (建议值)
   ├─ 交易频率
   ├─ 入场标准
   ├─ 决策流程
   └─ 输出格式 (XML + JSON)
        ↓
4. 用户提示词 (BuildUserPrompt)
   ├─ 系统状态 (时间, 周期号)
   ├─ BTC市场概览
   ├─ 账户信息
   ├─ 当前持仓 (含技术指标)
   ├─ 候选币种 (完整市场数据)
   └─ "请分析并输出决策..."
        ↓
5. AI请求 (CallWithMessages)
   ├─ 选择AI模型
   ├─ POST: system_prompt + user_prompt
   ├─ 超时: 120秒, 重试: 3次
   └─ 返回原始响应
        ↓
6. AI解析 (parseFullDecisionResponse)
   ├─ 提取思维链 <reasoning>
   ├─ 提取JSON决策 <decision>
   ├─ 修复字符编码
   ├─ 验证JSON格式
   ├─ 解析决策数组
   └─ 验证风控参数
        ↓
7. 决策执行
   ├─ 排序: 平仓优先 → 开仓 → hold/wait
   ├─ 风控强制执行
   ├─ 提交订单
   ├─ 确认成交
   └─ 记录到数据库
```

---

## 1. 币种选择 (Coin Selection)

**核心文件:** `decision/engine.go:380-454`

**入口方法:** `StrategyEngine.GetCandidateCoins()`

### 1.1 静态币种列表 (Static)

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

- **配置:** `StrategyConfig.CoinSource.StaticCoins`
- **用途:** 手动指定交易币种
- **标签:** `["static"]`

### 1.2 AI500 币种池 (CoinPool)

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

- **API:** `config.CoinSource.CoinPoolAPIURL` (默认: `https://nofxos.ai/api/ai500/list`)
- **用途:** 获取 AI 评分最高的 N 个币种
- **标签:** `["ai500"]`

### 1.3 OI Top 币种 (持仓增长榜)

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
- **用途:** 获取持仓量增长最快的币种
- **标签:** `["oi_top"]`

### 1.4 混合模式 (Mixed)

```go
// decision/engine.go:411-449
if config.CoinSource.SourceType == "mixed" {
    if config.CoinSource.UseCoinPool {
        // 添加 AI500 币种
    }
    if config.CoinSource.UseOITop {
        // 添加 OI Top 币种
    }
    if len(config.CoinSource.StaticCoins) > 0 {
        // 添加静态币种
    }
    // 去重合并，保留多来源标签
}
```

- **特点:** 同时使用多个数据源
- **标签示例:** `["ai500", "oi_top"]` (双信号币种)

---

## 2. 数据组装 (Data Assembly)

**核心文件:** `trader/auto_trader.go:562-791`, `decision/engine.go:299-374`

**入口方法:** `AutoTrader.buildTradingContext()`

### 2.1 账户数据

```go
// trader/auto_trader.go:565-583
balance, err := at.trader.GetBalance()
equity := balance["total_equity"].(float64)
available := balance["available_balance"].(float64)
unrealizedPnL := balance["total_pnl"].(float64)
```

**提取字段:**
- `total_equity` - 账户总权益
- `available_balance` - 可用余额
- `total_pnl` - 未实现盈亏

### 2.2 持仓数据

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

### 2.3 市场数据获取

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

### 2.4 技术指标计算

**文件:** `market/data.go:59-98`

| 指标 | 配置 | 计算方法 |
|------|------|----------|
| **EMA** | `EnableEMA`, `EMAPeriods` | `calculateEMA(klines, period)` |
| **MACD** | `EnableMACD` | `calculateMACD(klines)` - 12/26/9 |
| **RSI** | `EnableRSI`, `RSIPeriods` | `calculateRSI(klines, period)` |
| **ATR** | `EnableATR`, `ATRPeriods` | `calculateATR(klines, period)` |
| **Volume** | `EnableVolume` | 原始成交量数据 |
| **OI** | `EnableOI` | 持仓量数据 |
| **Funding Rate** | `EnableFundingRate` | 资金费率 |

### 2.5 量化数据 (可选)

```go
// trader/auto_trader.go:759-778
if config.Indicators.EnableQuantData {
    quantData := provider.GetQuantData(symbol)
    // 包含: 资金流向、OI变化、价格变化
}
```

**数据结构:**
```go
QuantData {
    Netflow {
        Institution: {Future, Spot},  // 机构资金流
        Personal: {Future, Spot}      // 散户资金流
    },
    OI {
        CurrentOI: float64,
        Delta: {1h, 4h, 24h}          // OI变化
    },
    PriceChange {
        "1h", "4h", "24h": float64    // 价格变化百分比
    }
}
```

---

## 3. 系统提示词 (System Prompt)

**核心文件:** `decision/engine.go:700-818`

**入口方法:** `StrategyEngine.BuildSystemPrompt(accountEquity, variant)`

### 3.1 提示词结构 (8个部分)

```
1. 角色定义          [可编辑]
2. 交易模式变体      [运行时确定]
3. 硬性约束          [代码强制 + AI引导]
4. 交易频率          [可编辑]
5. 入场标准          [可编辑]
6. 决策流程          [可编辑]
7. 输出格式          [固定XML + JSON结构]
8. 自定义提示词      [可选]
```

### 3.2 角色定义

```go
// decision/engine.go:706-713
roleDefinition := config.PromptSections.RoleDefinition
if roleDefinition == "" {
    roleDefinition = "You are a professional cryptocurrency trading AI..."
}
```

### 3.3 交易模式变体

| 模式 | 特点 |
|------|------|
| `aggressive` | 趋势突破，较高仓位容忍度 |
| `conservative` | 多信号确认，保守资金管理 |
| `scalping` | 短线动量，紧止盈 |

### 3.4 硬性约束

**代码强制执行 (CODE ENFORCED):**

```go
// decision/engine.go:725-749
maxPositions := config.RiskControl.MaxPositions           // 默认: 3
altcoinMaxRatio := config.RiskControl.AltcoinMaxPositionValueRatio  // 默认: 1.0
btcethMaxRatio := config.RiskControl.BTCETHMaxPositionValueRatio    // 默认: 5.0
maxMarginUsage := config.RiskControl.MaxMarginUsage       // 默认: 90%
minPositionSize := config.RiskControl.MinPositionSize     // 默认: 12 USDT
```

**AI引导 (建议值):**

```go
altcoinMaxLeverage := config.RiskControl.AltcoinMaxLeverage  // 默认: 5x
btcethMaxLeverage := config.RiskControl.BTCETHMaxLeverage    // 默认: 5x
minRiskRewardRatio := config.RiskControl.MinRiskRewardRatio  // 默认: 1:3
minConfidence := config.RiskControl.MinConfidence            // 默认: 75
```

### 3.5 输出格式要求

```xml
<reasoning>
[思维链分析过程]
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

## 4. 用户提示词 (User Prompt)

**核心文件:** `decision/engine.go:884-1007`

**入口方法:** `StrategyEngine.BuildUserPrompt(ctx)`

### 4.1 提示词内容结构

```
1. 系统状态          [时间, 周期号, 运行时长]
2. BTC市场概览      [价格, 涨跌幅, MACD, RSI]
3. 账户信息          [权益, 余额%, 盈亏%, 保证金%, 持仓数]
4. 最近成交          [最近10笔已平仓交易]
5. 当前持仓          [详细持仓数据 + 技术指标]
6. 候选币种          [完整市场数据]
7. 量化数据          [资金流向, OI数据] (可选)
8. OI排行数据        [市场OI变化排行] (可选)
```

### 4.2 账户信息格式

```
Account: Equity 1000.00 | Balance 800.00 (80.0%) | PnL +5.5% | Margin 20.0% | Positions 2
```

### 4.3 持仓信息格式

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

### 4.4 候选币种格式

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

## 5. AI请求 (AI Request)

**核心文件:** `decision/engine.go:222-293`, `mcp/client.go:136-150`

### 5.1 请求流程

```go
// decision/engine.go:263-268
aiCallStart := time.Now()
aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
aiCallDuration := time.Since(aiCallStart)
```

### 5.2 支持的AI模型

| 模型 | 客户端文件 | 默认模型 |
|------|-----------|----------|
| **DeepSeek** | `mcp/deepseek_client.go` | deepseek-chat |
| **Qwen** | `mcp/qwen_client.go` | qwen-max |
| **Claude** | `mcp/claude_client.go` | claude-3-5-sonnet |
| **Gemini** | `mcp/gemini_client.go` | gemini-pro |
| **Grok** | `mcp/grok_client.go` | grok-beta |
| **OpenAI** | `mcp/openai_client.go` | gpt-5.2 |
| **Kimi** | `mcp/kimi_client.go` | moonshot-v1-8k |

### 5.3 请求参数

```go
// mcp/client.go
Timeout: 120 seconds
MaxRetries: 3
RetryDelay: 2 seconds (exponential backoff)
```

---

## 6. AI响应解析 (Response Parsing)

**核心文件:** `decision/engine.go:1303-1604`

**入口方法:** `parseFullDecisionResponse(response, accountEquity, leverage, ratio)`

### 6.1 解析流程

```
原始AI响应 (文本)
    ↓
1. 提取思维链  [extractCoTTrace()]
    ↓
2. 提取JSON决策  [extractDecisions()]
    ↓
3. 验证JSON格式  [validateJSONFormat()]
    ↓
4. 解析JSON  [json.Unmarshal()]
    ↓
5. 验证决策  [validateDecisions()]
    ↓
6. 构建FullDecision  [返回结构化结果]
```

### 6.2 思维链提取

```go
// decision/engine.go:1327-1345
func extractCoTTrace(response string) string {
    // 优先级1: <reasoning> XML标签
    if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
        return strings.TrimSpace(match[1])
    }
    // 优先级2: <decision>标签之前的文本
    // 优先级3: JSON [ 之前的文本
    // 优先级4: 完整响应
}
```

### 6.3 JSON决策提取

```go
// decision/engine.go:1347-1408
func extractDecisions(response string) (string, error) {
    // 1. 移除不可见字符
    response = removeInvisibleRunes(response)

    // 2. 修复字符编码
    response = fixMissingQuotes(response)

    // 3. 提取JSON (优先级)
    //    - <decision> XML标签 + ```json
    //    - 独立 ```json 代码块
    //    - 裸JSON数组
}
```

### 6.4 字符编码修复

```go
// decision/engine.go:1410-1432
func fixMissingQuotes(s string) string {
    // 中文引号 → ASCII
    s = strings.ReplaceAll(s, """, "\"")
    s = strings.ReplaceAll(s, """, "\"")

    // 中文括号 → ASCII
    s = strings.ReplaceAll(s, "［", "[")
    s = strings.ReplaceAll(s, "］", "]")
    s = strings.ReplaceAll(s, "｛", "{")
    s = strings.ReplaceAll(s, "｝", "}")

    // 中文标点 → ASCII
    s = strings.ReplaceAll(s, "：", ":")
    s = strings.ReplaceAll(s, "，", ",")
}
```

### 6.5 决策验证

```go
// decision/engine.go:1480-1602
func validateDecisions(decisions []Decision, equity, leverage, ratio float64) error {
    for _, d := range decisions {
        // 1. 验证action类型
        validActions := []string{"open_long", "open_short", "close_long", "close_short", "hold", "wait"}

        // 2. 开仓验证
        if isOpenAction(d.Action) {
            // 杠杆范围检查
            // 仓位大小检查
            // 止损止盈检查
            // 风险回报比检查
            // 置信度检查
        }

        // 3. 平仓验证
        if isCloseAction(d.Action) {
            // Symbol必须存在
        }
    }
}
```

### 6.6 Decision结构体

```go
// decision/engine.go:128-143
type Decision struct {
    Symbol          string   // 交易对: "BTCUSDT"
    Action          string   // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
    Leverage        int      // 杠杆倍数
    PositionSizeUSD float64  // 仓位价值 (USDT)
    StopLoss        float64  // 止损价格
    TakeProfit      float64  // 止盈价格
    Confidence      int      // 置信度 0-100
    RiskUSD         float64  // 最大风险 (USDT)
    Reasoning       string   // 决策理由
}
```

---

## 7. 决策执行 (Execution)

**核心文件:** `trader/auto_trader.go:392-560`

### 7.1 决策排序

```go
// trader/auto_trader.go:519-526
sort.SliceStable(decisions, func(i, j int) bool {
    priority := map[string]int{
        "close_long": 1, "close_short": 1,  // 最高优先级
        "open_long": 2, "open_short": 2,    // 次优先级
        "hold": 3, "wait": 3,               // 最低优先级
    }
    return priority[decisions[i].Action] < priority[decisions[j].Action]
})
```

### 7.2 风控强制执行

**文件:** `trader/auto_trader.go:1769-1851`

| 检查项 | 方法 | 动作 |
|--------|------|------|
| 最大持仓数 | `enforceMaxPositions()` | 拒绝新开仓 |
| 仓位价值上限 | `enforcePositionValueRatio()` | 自动缩减仓位 |
| 最小仓位 | `enforceMinPositionSize()` | 拒绝过小订单 |
| 保证金调整 | 自动计算 | 根据可用余额调整 |

### 7.3 订单执行

```go
// trader/auto_trader.go:1631-1767
func (at *AutoTrader) recordAndConfirmOrder(orderID, symbol, side, action string) {
    // 1. 轮询订单状态 (5次重试, 500ms间隔)
    for i := 0; i < 5; i++ {
        status := at.trader.GetOrderStatus(orderID)
        if status.Status == "FILLED" {
            break
        }
        time.Sleep(500 * time.Millisecond)
    }

    // 2. 提取成交信息
    filledPrice := status.AvgPrice
    filledQty := status.FilledQty
    fee := status.Fee

    // 3. 记录到数据库
    at.store.Position().SaveOrder(...)
}
```

### 7.4 决策日志保存

```go
// trader/auto_trader.go:1235-1256
record := &store.DecisionRecord{
    CycleNumber:    cycleNumber,
    TraderID:       traderID,
    Timestamp:      time.Now(),
    SystemPrompt:   systemPrompt,     // 完整系统提示词
    InputPrompt:    userPrompt,       // 完整用户提示词
    CoTTrace:       cotTrace,         // AI思维链
    DecisionJSON:   decisionsJSON,    // 解析后的决策
    RawResponse:    rawResponse,      // 原始AI响应
    ExecutionLog:   executionResults, // 执行结果
    CandidateCoins: candidateCoins,   // 候选币种
    Success:        success,          // 执行状态
}
at.store.Decision().LogDecision(record)
```

---

## 核心文件索引

| 模块 | 文件 | 关键方法 |
|------|------|----------|
| **主循环** | `trader/auto_trader.go` | `Run()`, `runCycle()`, `buildTradingContext()` |
| **币种选择** | `decision/engine.go:380-454` | `GetCandidateCoins()` |
| **数据获取** | `market/data.go` | `Get()`, `GetWithTimeframes()` |
| **指标计算** | `market/data.go:59-98` | `calculateEMA()`, `calculateMACD()`, `calculateRSI()` |
| **系统提示词** | `decision/engine.go:700-818` | `BuildSystemPrompt()` |
| **用户提示词** | `decision/engine.go:884-1007` | `BuildUserPrompt()` |
| **市场数据格式化** | `decision/engine.go:1029-1099` | `formatMarketData()` |
| **AI请求** | `decision/engine.go:222-293` | `GetFullDecisionWithStrategy()` |
| **MCP客户端** | `mcp/client.go:136-150` | `CallWithMessages()` |
| **响应解析** | `decision/engine.go:1303-1604` | `parseFullDecisionResponse()` |
| **思维链提取** | `decision/engine.go:1327-1345` | `extractCoTTrace()` |
| **JSON提取** | `decision/engine.go:1347-1408` | `extractDecisions()` |
| **决策验证** | `decision/engine.go:1480-1602` | `validateDecisions()` |
| **风控执行** | `trader/auto_trader.go:1769-1851` | `enforceMaxPositions()`, `enforcePositionValueRatio()` |
| **策略配置** | `store/strategy.go` | `StrategyConfig`, `RiskControlConfig` |
| **数据提供者** | `provider/data_provider.go` | `GetAI500Data()`, `GetOITopPositions()` |

---

## 配置参考

### 策略配置结构

```go
// store/strategy.go
type StrategyConfig struct {
    // 币种来源
    CoinSource struct {
        SourceType     string   // "static", "coinpool", "oi_top", "mixed"
        StaticCoins    []string // 静态币种列表
        UseCoinPool    bool     // 是否使用AI500
        UseOITop       bool     // 是否使用OI排行
        CoinPoolLimit  int      // AI500获取数量
        CoinPoolAPIURL string   // AI500 API地址
        OITopAPIURL    string   // OI排行 API地址
    }

    // 技术指标
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

    // 风控配置
    RiskControl struct {
        MaxPositions               int     // 最大持仓数
        BTCETHMaxLeverage          int     // BTC/ETH最大杠杆
        AltcoinMaxLeverage         int     // 山寨币最大杠杆
        BTCETHMaxPositionValueRatio float64 // BTC/ETH仓位比例上限
        AltcoinMaxPositionValueRatio float64 // 山寨币仓位比例上限
        MaxMarginUsage             float64 // 最大保证金使用率
        MinPositionSize            float64 // 最小仓位
        MinRiskRewardRatio         float64 // 最小风险回报比
        MinConfidence              int     // 最小置信度
    }

    // 提示词部分
    PromptSections struct {
        RoleDefinition   string
        TradingFrequency string
        EntryStandards   string
        DecisionProcess  string
    }

    // 自定义提示词
    CustomPrompt string
}
```

---

**文档版本:** 1.0.0
**最后更新:** 2025-01-15
