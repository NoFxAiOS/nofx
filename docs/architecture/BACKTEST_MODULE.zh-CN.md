# NOFX 回测模块技术文档

**语言:** [English](BACKTEST_MODULE.md) | [中文](BACKTEST_MODULE.zh-CN.md)

## 概述

本文档详细描述 NOFX 回测模块的完整技术实现，包括配置、历史数据加载、模拟引擎、AI 决策、性能指标计算和结果存储。

---

## 完整回测流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                    回测执行流程                                   │
└─────────────────────────────────────────────────────────────────┘

1. API 请求: /backtest/start
   ↓
2. Manager.Start()
   ├─ 验证配置
   ├─ 解析 AI 模型
   ├─ 创建 Runner 实例
   └─ 启动 runner.Start() (goroutine)
   ↓
3. Runner.Start() → Runner.loop()
   └─ 遍历每个决策时间点:
      ├─ DataFeed.BuildMarketData()      [构建市场数据]
      ├─ 检查决策触发条件                  [每 N 根 K 线]
      ├─ buildDecisionContext()           [构建决策上下文]
      ├─ invokeAIWithRetry()              [调用 AI + 缓存]
      ├─ executeDecision()                [执行交易]
      ├─ checkLiquidation()               [检查爆仓]
      ├─ updateState()                    [更新状态]
      ├─ appendEquityPoint()              [记录权益]
      ├─ appendTradeEvent()               [记录交易]
      ├─ maybeCheckpoint()                [保存检查点]
      └─ persistMetrics()                 [持久化指标]
   ↓
4. 完成/失败
   ├─ 计算最终指标
   ├─ 持久化所有结果
   └─ 释放锁
   ↓
5. API 查询: /backtest/metrics, /backtest/equity, /backtest/trades
   └─ 加载并返回结果
```

---

## 1. 回测配置 (Configuration)

**核心文件:** `backtest/config.go`

### 1.1 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `RunID` | string | (必填) | 回测运行唯一标识 |
| `UserID` | string | "default" | 用户 ID |
| `Symbols` | []string | (必填) | 交易币种列表 |
| `Timeframes` | []string | ["3m", "15m", "4h"] | K 线周期 |
| `DecisionTimeframe` | string | Symbols[0] | 主决策周期 |
| `DecisionCadenceNBars` | int | 20 | 每 N 根 K 线触发一次决策 |
| `StartTS`, `EndTS` | int64 | (必填) | 回测时间范围 (Unix 时间戳) |
| `InitialBalance` | float64 | 1000 | 初始资金 (USD) |
| `FeeBps` | float64 | 5 | 手续费 (基点) |
| `SlippageBps` | float64 | 2 | 滑点 (基点) |
| `FillPolicy` | string | "next_open" | 成交策略 |
| `PromptVariant` | string | "baseline" | AI 提示词变体 |
| `CacheAI` | bool | false | 是否缓存 AI 决策 |
| `Leverage` | LeverageConfig | BTC/ETH:5, Altcoin:5 | 杠杆设置 |

### 1.2 成交策略 (Fill Policy)

```go
// backtest/config.go:163-179
switch fillPolicy {
case "next_open":  // 下一根 K 线开盘价
case "bar_vwap":   // 当前 K 线 VWAP
case "mid":        // 当前 K 线 (High+Low)/2
default:           // Mark Price
}
```

### 1.3 配置示例

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

## 2. 历史数据加载 (Data Loading)

**核心文件:** `backtest/datafeed.go`

### 2.1 数据加载流程

```
1. NewDataFeed() - 初始化
   ↓
2. loadAll() - 加载所有历史数据
   ├─ 计算缓冲区 (StartTS 前 200 根 K 线)
   ├─ 调用 market.GetKlinesRange() 获取数据
   ├─ 存储到 symbolSeries map
   └─ 从主周期构建决策时间线
   ↓
3. BuildMarketData() - 构建市场数据快照
   ├─ 切片 K 线数据到当前时间戳
   ├─ 计算技术指标 (EMA, MACD, RSI, ATR)
   └─ 返回 market.Data 结构
```

### 2.2 数据结构

```go
// DataFeed 核心结构
type DataFeed struct {
    decisionTimes []int64                    // 决策时间点列表
    symbolSeries  map[string]*symbolSeries   // 按币种存储的数据
}

// 单币种时间序列
type symbolSeries struct {
    timeframes map[string]*timeframeSeries   // 按周期存储
}

// 单周期数据
type timeframeSeries struct {
    klines     []market.Kline               // K 线数据
    closeTimes []int64                      // 收盘时间索引
}
```

### 2.3 关键代码引用

- 数据获取: `backtest/datafeed.go:48-93`
- 时间线生成: `backtest/datafeed.go:96-115`
- 市场数据组装: `backtest/datafeed.go:141-171`

---

## 3. 模拟引擎 (Simulation Engine)

**核心文件:** `backtest/runner.go`

### 3.1 主循环

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

### 3.2 单步执行

```go
// backtest/runner.go:266-471
func (r *Runner) stepOnce(ts int64) {
    // 1. 获取当前 K 线时间戳
    // 2. 构建市场数据
    // 3. 检查决策触发条件 (每 N 根 K 线)
    // 4. 执行决策周期 (如果触发)
    // 5. 检查爆仓
    // 6. 更新状态并记录
}
```

### 3.3 状态管理

```go
// backtest/types.go:31-47
type BacktestState struct {
    BarIndex       int                      // 当前 K 线索引
    Cash           float64                  // 可用余额
    Equity         float64                  // 总权益
    UnrealizedPnL  float64                  // 未实现盈亏
    RealizedPnL    float64                  // 已实现盈亏
    MaxEquity      float64                  // 最高权益
    MinEquity      float64                  // 最低权益
    MaxDrawdownPct float64                  // 最大回撤
    Positions      map[string]*position     // 持仓
}
```

---

## 4. AI 决策 (AI Decision Making)

**核心文件:** `backtest/runner.go`

### 4.1 决策上下文构建

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

### 4.2 AI 调用

```go
// backtest/runner.go:544-563
func (r *Runner) invokeAIWithRetry() (*decision.FullDecision, error) {
    // 最多重试 3 次
    // 指数退避: 500ms, 1000ms, 1500ms
    // 使用 decision.GetFullDecisionWithStrategy() 统一提示词生成
}
```

### 4.3 AI 缓存

```go
// backtest/aicache.go:127-168
// 缓存键: SHA256(context payload)
// 包含: variant, timestamp, account, positions, market data
```

### 4.4 支持的 AI 模型

| 模型 | 客户端文件 |
|------|-----------|
| DeepSeek | `mcp/deepseek_client.go` |
| Qwen | `mcp/qwen_client.go` |
| Claude | `mcp/claude_client.go` |
| Gemini | `mcp/gemini_client.go` |
| Grok | `mcp/grok_client.go` |
| OpenAI | `mcp/openai_client.go` |
| Kimi | `mcp/kimi_client.go` |

---

## 5. 性能指标 (Performance Metrics)

**核心文件:** `backtest/metrics.go`

### 5.1 指标计算

| 指标 | 公式 | 代码位置 |
|------|------|----------|
| **总收益率** | (最终权益 - 初始资金) / 初始资金 × 100 | metrics.go:36-42 |
| **最大回撤** | max((峰值 - 当前) / 峰值 × 100) | metrics.go:64-91 |
| **夏普比率** | 平均收益 / 收益标准差 | metrics.go:94-138 |
| **胜率** | 盈利交易数 / 总交易数 × 100 | metrics.go:180-181 |
| **盈亏比** | 总盈利 / 总亏损 | metrics.go:189-193 |

### 5.2 交易统计

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

## 6. 权益曲线 (Equity Curve)

**核心文件:** `backtest/equity.go`

### 6.1 权益点结构

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

### 6.2 权益更新

```go
// backtest/runner.go:829-872
func (r *Runner) updateState() {
    // 1. 计算总权益: cash + margin + 未实现盈亏
    // 2. 追踪峰值 (MaxEquity)
    // 3. 追踪谷值 (MinEquity)
    // 4. 重新计算回撤: (MaxEquity - Equity) / MaxEquity × 100
}
```

### 6.3 数据重采样

```go
// backtest/equity.go:10-50
func ResampleEquity(points []EquityPoint, timeframe string) []EquityPoint {
    // 按时间周期分桶
    // 保留每个桶的最后一个点
}
```

---

## 7. 结果存储 (Result Storage)

**核心文件:** `backtest/storage.go`, `store/backtest.go`

### 7.1 文件存储结构

```
backtests/
├── <run_id>/
│   ├── run.json              # 运行元数据
│   ├── checkpoint.json       # 检查点 (用于恢复)
│   ├── equity.jsonl          # 权益曲线 (逐行 JSON)
│   ├── trades.jsonl          # 交易记录 (逐行 JSON)
│   ├── metrics.json          # 性能指标
│   ├── progress.json         # 进度信息
│   ├── ai_cache.json         # AI 决策缓存
│   └── decision_logs/        # 决策日志
│       ├── 0.json
│       ├── 1.json
│       └── ...
```

### 7.2 数据库表结构

```sql
-- 回测运行元数据
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

-- 权益曲线
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

-- 交易记录
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

-- 性能指标
CREATE TABLE backtest_metrics (
  run_id TEXT PRIMARY KEY,
  payload BLOB,
  updated_at DATETIME
);

-- 检查点 (暂停/恢复)
CREATE TABLE backtest_checkpoints (
  run_id TEXT PRIMARY KEY,
  payload BLOB,
  updated_at DATETIME
);
```

---

## 8. API 接口

**核心文件:** `api/backtest.go`

### 8.1 接口列表

| 接口 | 方法 | 说明 |
|------|------|------|
| `/backtest/start` | POST | 开始回测 |
| `/backtest/pause` | POST | 暂停回测 |
| `/backtest/resume` | POST | 恢复回测 |
| `/backtest/stop` | POST | 停止回测 |
| `/backtest/status` | GET | 获取状态 |
| `/backtest/runs` | GET | 列出所有回测 |
| `/backtest/equity` | GET | 获取权益曲线 |
| `/backtest/trades` | GET | 获取交易记录 |
| `/backtest/metrics` | GET | 获取性能指标 |
| `/backtest/trace` | GET | 获取决策日志 |
| `/backtest/export` | GET | 导出 ZIP |
| `/backtest/delete` | POST | 删除回测 |

### 8.2 请求示例

```bash
# 开始回测
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

# 获取权益曲线
GET /backtest/equity?run_id=bt_20231215&tf=1h&limit=1000

# 获取指标
GET /backtest/metrics?run_id=bt_20231215
```

### 8.3 响应示例

```json
// 状态响应
{
  "run_id": "bt_20231215",
  "state": "running",
  "progress_pct": 45.5,
  "processed_bars": 1234,
  "equity": 10234.50,
  "unrealized_pnl": 234.50
}

// 指标响应
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

## 9. 账户与持仓管理

**核心文件:** `backtest/account.go`

### 9.1 持仓结构

```go
type position struct {
    Symbol           string
    Side             string     // "long" 或 "short"
    Quantity         float64
    EntryPrice       float64
    Leverage         int
    Margin           float64    // 保证金
    Notional         float64    // 名义价值
    LiquidationPrice float64    // 爆仓价格
    OpenTime         int64
}
```

### 9.2 开仓逻辑

```go
// backtest/account.go:61-104
func (a *BacktestAccount) Open(symbol, side string, qty, price float64, leverage int) {
    // 1. 应用滑点
    // 2. 计算名义价值 (qty × price)
    // 3. 计算保证金 (notional / leverage)
    // 4. 扣除保证金 + 手续费
    // 5. 创建/加仓
    // 6. 计算爆仓价格
}
```

### 9.3 平仓逻辑

```go
// backtest/account.go:106-140
func (a *BacktestAccount) Close(symbol, side string, qty, price float64) {
    // 1. 验证持仓存在
    // 2. 应用滑点 (反向)
    // 3. 计算已实现盈亏
    //    long:  (exit - entry) × qty
    //    short: (entry - exit) × qty
    // 4. 返还保证金 + 盈亏 - 手续费
    // 5. 更新/删除持仓
}
```

### 9.4 爆仓价格计算

```go
// backtest/account.go:177-186
func computeLiquidation(entry float64, leverage int, side string) float64 {
    if side == "long" {
        return entry * (1 - 1.0/float64(leverage))  // 做多: 下跌爆仓
    }
    return entry * (1 + 1.0/float64(leverage))      // 做空: 上涨爆仓
}
```

---

## 10. 检查点与恢复

**核心文件:** `backtest/runner.go`

### 10.1 检查点结构

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

### 10.2 检查点触发

```go
// backtest/runner.go:874-898
func (r *Runner) maybeCheckpoint() {
    // 每 N 根 K 线保存
    // 或每 N 秒保存
}
```

### 10.3 恢复流程

```go
func (r *Runner) RestoreFromCheckpoint() {
    // 1. 加载检查点
    // 2. 恢复账户状态
    // 3. 恢复 K 线索引 (从下一根继续)
    // 4. 恢复权益曲线、交易记录
}
```

---

## 核心文件索引

| 模块 | 文件 | 关键方法 |
|------|------|----------|
| **配置** | `backtest/config.go` | `BacktestConfig`, `Validate()` |
| **数据加载** | `backtest/datafeed.go` | `NewDataFeed()`, `loadAll()`, `BuildMarketData()` |
| **模拟引擎** | `backtest/runner.go` | `Start()`, `loop()`, `stepOnce()` |
| **决策** | `backtest/runner.go` | `buildDecisionContext()`, `invokeAIWithRetry()` |
| **执行** | `backtest/runner.go` | `executeDecision()` |
| **账户** | `backtest/account.go` | `Open()`, `Close()`, `TotalEquity()` |
| **指标** | `backtest/metrics.go` | `CalculateMetrics()` |
| **权益** | `backtest/equity.go` | `ResampleEquity()`, `LimitEquityPoints()` |
| **存储** | `backtest/storage.go` | `SaveCheckpoint()`, `appendEquityPoint()` |
| **数据库** | `store/backtest.go` | 表结构和 CRUD 操作 |
| **API** | `api/backtest.go` | HTTP 处理器 |
| **AI 缓存** | `backtest/aicache.go` | `Get()`, `Put()`, `save()` |

---

**文档版本:** 1.0.0
**最后更新:** 2025-01-15
