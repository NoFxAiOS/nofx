# AI 输入数据快速参考

## 🎯 三层数据分类一览表

| 层级 | 名称 | 来源 | 更新频率 | 大小 | 说明 |
|------|------|------|--------|------|------|
| **1** | 行情数据 | WebSocket | 实时 | 2KB/币 | 技术指标、价格、成交量 |
| **2** | 累计数据 | REST API + DB | 每3分钟 | 40KB | 账户、持仓、表现 |
| **3** | 提示词 | 文件 + 代码 | 启动时 | 3-5KB | 规则、约束、格式要求 |

---

## 📊 发送给AI的完整结构

```
Context {
    ├─ CurrentTime: "12:00:00"
    ├─ RuntimeMinutes: 240
    ├─ CallCount: 48
    │
    ├─ Account {
    │   ├─ TotalEquity: 10000.00
    │   ├─ AvailableBalance: 6000.00
    │   ├─ TotalPnL: +500.00
    │   ├─ TotalPnLPct: +5.00%
    │   ├─ MarginUsed: 4000.00
    │   ├─ MarginUsedPct: 40.0%
    │   └─ PositionCount: 2
    │
    ├─ Positions: [
    │   {
    │       "symbol": "BTCUSDT",
    │       "side": "long",
    │       "entry_price": 42000.00,
    │       "mark_price": 42300.00,
    │       "quantity": 0.5,
    │       "leverage": 5,
    │       "unrealized_pnl": +300.00,
    │       "unrealized_pnl_pct": +1.50%,
    │       "liquidation_price": 35700.00,
    │       "margin_used": 2000.00
    │   },
    │   ... 更多持仓
    │ ]
    │
    ├─ CandidateCoins: [
    │   {"symbol": "BNBUSDT", "sources": ["ai500"]},
    │   {"symbol": "SOLUSDT", "sources": ["oi_top"]},
    │   {"symbol": "AVAXUSDT", "sources": ["ai500", "oi_top"]},
    │   ... 更多候选
    │ ]
    │
    └─ MarketDataMap: {
        "BTCUSDT": {
            "current_price": 42300.75,
            "price_change_1h": +0.45%,
            "price_change_4h": +1.23%,
            "current_ema20": 42250.30,
            "current_macd": 12.50,
            "current_rsi7": 68.5,
            "open_interest": {
                "latest": 1234567,
                "average": 1000000
            },
            "funding_rate": 0.00045,
            "intraday_series": {
                "mid_prices": [42100, 42120, 42140, 42165],
                "ema20_values": [42200, 42210, 42230, 42250],
                "macd_values": [10.5, 11.2, 12.0, 12.5],
                "rsi7_values": [65, 66, 67, 68.5],
                "rsi14_values": [60, 61, 62, 63]
            },
            "longer_term_context": {
                "ema20": 42150.30,
                "ema50": 41800.30,
                "atr3": 250.30,
                "atr14": 280.30,
                "current_volume": 5000.30,
                "average_volume": 4500.30,
                "macd_values": [8.0, 9.5, 11.0, 12.5],
                "rsi14_values": [55, 58, 60, 63]
            }
        },
        "ETHUSDT": { ... },
        "BNBUSDT": { ... },
        ... 更多币种
    }
}
```

---

## 🔄 数据流向图

```
┌─────────────────────────────────────────────────────────────┐
│ Binance WebSocket (实时)                                    │
│ ├─ 15m K线 (OHLCV)                                          │
│ └─ 1h K线 (OHLCV)                                           │
└────────────────┬────────────────────────────────────────────┘
                 │
        ┌────────▼────────┐
        │ 市场数据计算    │
        │ (market.Get)    │
        └────────┬────────┘
                 │
        ┌────────▼────────────────────────────┐
        │ 技术指标和序列数据                  │
        │ • CurrentPrice, EMA20, MACD, RSI   │
        │ • IntradaySeries (15m)             │
        │ • LongerTermContext (1h)           │
        └────────┬────────────────────────────┘
                 │
        ┌────────▼────────────────────────────┐
        │ Binance REST API                    │
        │ ├─ 账户信息                        │
        │ ├─ 持仓列表                        │
        │ └─ 持仓量数据 (OI)                 │
        └────────┬────────────────────────────┘
                 │
        ┌────────▼────────────────────────────┐
        │ 本地数据库                          │
        │ ├─ 决策历史                        │
        │ └─ 交易记录                        │
        └────────┬────────────────────────────┘
                 │
        ┌────────▼────────────────────────────┐
        │ 性能分析                            │
        │ (AnalyzePerformance)               │
        │ ├─ 胜率、利润、回撤等              │
        │ └─ 夏普比率                        │
        └────────┬────────────────────────────┘
                 │
        ┌────────▼────────────────────────────┐
        │ 构建 Context                        │
        │ (buildContext)                     │
        └────────┬────────────────────────────┘
                 │
    ┌────────────▼────────────────┐
    │ 构建两个 Prompts            │
    ├────────────────────────────┤
    │ 1. System Prompt (固定)    │
    │    • 策略模板 (3-5KB)      │
    │    • 硬约束 (~500字节)     │
    │    • 格式要求 (~300字节)   │
    ├────────────────────────────┤
    │ 2. User Prompt (动态)      │
    │    • 当前时间、周期        │
    │    • BTC 行情概览          │
    │    • 账户状态              │
    │    • 当前持仓（详细）      │
    │    • 候选币种（详细）      │
    │    • 性能指标              │
    └────────┬───────────────────┘
             │
    ┌────────▼──────────────┐
    │ 调用 AI API           │
    │ • DeepSeek            │
    │ • Qwen                │
    │ • OpenAI兼容API      │
    └────────┬──────────────┘
             │
    ┌────────▼──────────────┐
    │ AI 返回响应           │
    │ • 思维链 (CoT)        │
    │ • JSON 决策列表       │
    └────────┬──────────────┘
             │
    ┌────────▼──────────────┐
    │ 解析和验证            │
    │ • 提取 CoT            │
    │ • 提取 JSON           │
    │ • 验证约束            │
    └────────┬──────────────┘
             │
    ┌────────▼──────────────┐
    │ 执行决策              │
    │ (executeDecision)     │
    └───────────────────────┘
```

---

## 📝 System Prompt 构成

```
3000-5000 字 ={
    ├─ 50% 核心策略模板
    │  ├─ 趋势判断方法
    │  ├─ 入场条件
    │  ├─ 出场条件
    │  ├─ 风险管理
    │  └─ 特殊币种策略 (BTC/ETH vs 山寨币)
    │
    ├─ 30% 硬约束规则
    │  ├─ 风险回报比 ≥ 1:2
    │  ├─ 最多持仓 3个
    │  ├─ 单币仓位范围
    │  ├─ 杠杆限制
    │  ├─ 保证金限制 ≤ 90%
    │  └─ 最小开仓金额 ≥ 12 USDT
    │
    └─ 20% 输出格式要求
       ├─ XML标签 <reasoning> <decision>
       ├─ JSON 数组格式
       ├─ 字段说明
       └─ 示例格式
}
```

---

## 🎲 User Prompt 构成

| 内容 | 大小 | 更新频率 | 例子 |
|------|------|--------|------|
| 时间信息 | ~50字节 | 每3分钟 | "12:00:00 \| 周期: #48" |
| BTC概览 | ~100字节 | 实时 | "42300.75 (1h: +0.45%)" |
| 账户摘要 | ~150字节 | 每3分钟 | "净值10000, 余额6000" |
| 持仓详情 | 500-1500字节 | 每3分钟 | 每个持仓+完整市场数据 |
| 候选币种 | 5-15KB | 每3分钟 | 15-30个币种的完整数据 |
| 性能指标 | ~30字节 | 每3分钟 | "夏普比率: 2.15" |

---

## 🔍 关键数据字段速查

### 行情数据 (Market Data)

| 字段 | 含义 | 来源 | 更新 |
|------|------|------|------|
| CurrentPrice | 当前价格 | 15m K线 Close | 秒级 |
| PriceChange1h | 1小时涨跌% | 15m K线对比 | 15分钟 |
| PriceChange4h | 4小时涨跌% | 1h K线对比 | 1小时 |
| CurrentEMA20 | EMA20指标 | 15m Close序列 | 秒级 |
| CurrentMACD | MACD值 | 15m Close序列 | 秒级 |
| CurrentRSI7 | RSI7指标 | 15m Close序列 | 秒级 |
| IntradaySeries | 15m指标序列 | 15m K线数据 | 15分钟 |
| LongerTermContext | 1h指标数据 | 1h K线数据 | 1小时 |

### 账户数据 (Account Info)

| 字段 | 含义 | 范围 | 重要性 |
|------|------|------|--------|
| TotalEquity | 账户净值 | 浮动 | ⭐⭐⭐ |
| AvailableBalance | 可用余额 | 0-TotalEquity | ⭐⭐⭐ |
| TotalPnLPct | 总收益率 | -100% 至 +∞ | ⭐⭐ |
| MarginUsedPct | 保证金使用率 | 0-100% | ⭐⭐⭐ |
| PositionCount | 持仓个数 | 0-3 | ⭐⭐ |

### 持仓数据 (Position Info)

| 字段 | 含义 | 用途 |
|------|------|------|
| EntryPrice | 入场价 | 计算是否止盈止损 |
| MarkPrice | 当前标记价 | 计算未实现盈亏 |
| UnrealizedPnL% | 未实现盈亏% | 判断是否强平风险 |
| Leverage | 杠杆倍数 | 风险管理 |
| LiquidationPrice | 强平价 | 核心风险指标 |
| MarginUsed | 占用保证金 | 额度管理 |

---

## ⚡ 实际数据示例

### 某时刻的完整 User Prompt

```
时间: 12:00:00 | 周期: #48 | 运行: 240分钟

BTC: 42300.75 (1h: +0.45%, 4h: +1.23%) | MACD: 0.0012 | RSI: 68.50

账户: 净值10000.00 | 余额6000.00 (60.0%) | 盈亏+5.00% | 保证金40.0% | 持仓2个

## 当前持仓

1. BTCUSDT LONG | 入场价42000.00 当前价42300.00 | 盈亏+1.50% | 杠杆5x | 保证金2000 | 强平价35700.00

current_price = 42300.7500, current_ema20 = 42250.300, current_macd = 12.500, current_rsi (7 period) = 68.500

Open Interest: Latest: 1234567 Average: 1000000

Funding Rate: 4.50e-04

Intraday series (15‑minute intervals, oldest → latest):
Mid prices: [42100, 42120, 42140, 42165]
EMA indicators (20‑period): [42200, 42210, 42230, 42250]
MACD indicators: [10.5, 11.2, 12.0, 12.5]
RSI indicators (7‑Period): [65, 66, 67, 68.5]
RSI indicators (14‑Period): [60, 61, 62, 63]

Longer‑term context (1‑hour timeframe):
20‑Period EMA: 42150.300 vs. 50‑Period EMA: 41800.300
3‑Period ATR: 250.300 vs. 14‑Period ATR: 280.300
Current Volume: 5000.300 vs. Average Volume: 4500.300
MACD indicators: [8.0, 9.5, 11.0, 12.5]
RSI indicators (14‑Period): [55, 58, 60, 63]

2. ETHUSDT SHORT | 入场价2500.00 当前价2480.00 | 盈亏+0.80% | 杠杆3x | 保证金1600 | 强平价2820.00

[完整市场数据...]

## 候选币种 (25个)

### 1. BNBUSDT (AI500+OI_Top双重信号)

current_price = 640.2500, ...
[完整市场数据...]

### 2. SOLUSDT (OI_Top持仓增长)

current_price = 185.5000, ...
[完整市场数据...]

... 更多 ...

## 📊 夏普比率: 2.15

---

现在请分析并输出决策（思维链 + JSON）
```

---

## 🎯 AI决策输出格式

```xml
<reasoning>
分析当前市场状况：
- BTC处于上升趋势，RSI即将进入超买区域
- 现有持仓盈利状态，可考虑部分止盈
- BNBUSDT有双重信号，可以追高
</reasoning>

<decision>
```json
[
    {
        "symbol": "BTCUSDT",
        "action": "partial_close",
        "close_percentage": 30,
        "confidence": 80,
        "reasoning": "止盈一部分，锁定收益"
    },
    {
        "symbol": "BNBUSDT",
        "action": "open_long",
        "leverage": 3,
        "position_size_usd": 900,
        "stop_loss": 625.00,
        "take_profit": 670.00,
        "confidence": 75,
        "risk_usd": 200,
        "reasoning": "双重信号，上升趋势确认"
    },
    {
        "symbol": "ETHUSDT",
        "action": "hold",
        "reasoning": "保持空仓持有"
    }
]
```
</decision>
```

---

## 🔧 调试命令

```bash
# 查看最新的User Prompt
tail -100 logs/backend.log | grep -A 50 "=== User Prompt ==="

# 查看AI原始响应
tail -100 logs/backend.log | grep -A 20 "=== AI Response ==="

# 查看解析后的决策
tail -50 logs/backend.log | grep "Decision:"

# 查看数据获取时间
tail -100 logs/backend.log | grep -E "获取市场数据|获取账户|获取持仓"

# 查看完整的一个决策循环
tail -200 logs/backend.log | grep -E "^# Cycle|^# Time|GetFullDecision"
```

---

## ⚠️ 常见问题

**Q: 为什么User Prompt这么大？**
A: 包含了完整的市场数据和所有候选币种的详细信息，AI需要这些数据做出好的决策。

**Q: 可以减少User Prompt的内容吗？**
A: 可以，但会影响AI的决策质量。建议通过减少候选币种数量（calculateMaxCandidates），而不是删除字段。

**Q: System Prompt会变化吗？**
A: 会。硬约束部分（单币仓位、杠杆限制）根据账户净值动态计算。

**Q: 市场数据多久更新一次？**
A: 行情数据实时(秒级)，但User Prompt只在每个3分钟周期重新构建一次。

**Q: 如果数据缺失会怎样？**
A: 系统会使用缺省值或跳过该币种，并记录警告日志。

---

## 📚 相关文档

- `BINANCE_WEBSOCKET_DATA_GUIDE.md` - WebSocket数据结构详解
- `AI_INPUT_DATA_STRUCTURE.md` - 本文档的完整版本
- `BINANCE_QUICK_REFERENCE.md` - Binance数据快速参考
- `OBV_IMPLEMENTATION_GUIDE.md` - OBV指标集成指南


