# NOFXi × NOFX 集成方案

> 审视日期: 2026-03-22
> 目标: NOFXi 自然接管 NOFX 的所有能力，成为交易领域的 OpenClaw

## NOFX 现有能力清单

### 核心引擎 (kernel/)
| 模块 | 文件 | 能力 | NOFXi 集成状态 |
|------|------|------|----------------|
| AI 决策引擎 | `engine.go` | 完整的交易上下文构建 → AI 分析 → 结构化决策 | ❌ 未接入 |
| Prompt 构建 | `engine_prompt.go` | 策略引擎+风控约束+多语言 prompt | ❌ 未接入 |
| 持仓分析 | `engine_position.go` | 持仓状态+止损止盈+保证金分析 | ❌ 未接入 |
| 决策 Schema | `schema.go` | 双语数据字典，AI 理解交易数据的基础 | ❌ 未接入 |
| 格式化 | `formatter.go` | 决策结果格式化输出 | ❌ 未接入 |
| 网格引擎 | `grid_engine.go` | 网格策略执行 | ❌ 未接入 |

### 交易执行 (trader/)
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `auto_trader.go` | 自动交易主体：配置、初始化、生命周期 | 🔧 bridge 接口定义了，但未调用 |
| `auto_trader_loop.go` | 交易循环：上下文构建 → AI 决策 → 执行 → 记录 | ❌ 核心！必须接入 |
| `auto_trader_decision.go` | 决策执行：开仓/平仓/调仓 | ❌ 未接入 |
| `auto_trader_risk.go` | 风控：每日亏损限制、最大回撤、安全模式 | ❌ 未接入 |
| `auto_trader_orders.go` | 订单管理：止损止盈、追踪止损 | ❌ 未接入 |
| `auto_trader_grid.go` | 网格交易执行 | ❌ 未接入 |
| 9 交易所 | binance/okx/bybit/bitget/gate/kucoin/hyperliquid/aster/lighter | 🔧 factory.go 接了 6 个 |

### MCP 客户端 (mcp/)
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `client.go` | OpenAI 兼容 + 流式 + 重试 + x402 支付 | ❌ NOFXi 自己写了简单版 |
| `context_guard.go` | 上下文长度预校验，防止超限 | ❌ 未接入 |
| `hooks.go` | AI 调用钩子（日志、计费） | ❌ 未接入 |
| `config.go` | 模型配置管理 | ❌ 未接入 |

### 市场数据 (market/)
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `data.go` | 完整市场数据：价格/EMA/MACD/RSI/布林带/OI/资金费率 | ❌ NOFXi 只用了 ticker |
| `data_indicators.go` | 技术指标计算 | ❌ 未接入 |
| `data_klines.go` | K 线数据获取（CoinAnk 路由） | 🔧 NOFXi 直接调 Binance |
| `historical.go` | 历史数据 | ❌ 未接入 |

### 数据存储 (store/)
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `decision.go` | AI 决策记录（完整链路） | ❌ 未接入 |
| `position.go` | 持仓记录 | ❌ 未接入 |
| `order.go` | 订单记录 | ❌ 未接入 |
| `equity.go` | 权益快照（画收益曲线） | ❌ 未接入 |
| `exchange.go` | 交易所配置（加密存储） | ❌ NOFXi 用 YAML |
| `ai_charge.go` | AI 调用计费 | ❌ 未接入 |
| `grid.go` | 网格策略配置和状态 | ❌ 未接入 |

### Telegram Agent (telegram/)
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `bot.go` | Telegram bot + agent 系统 | 🔧 NOFXi 自己写了简单版 |
| `agent/` | 流式回复、tool calling、记忆 | ❌ 未接入 |

### 其他
| 模块 | 能力 | NOFXi 集成状态 |
|------|------|----------------|
| `wallet/` | USDC 钱包、x402 支付 | ❌ 未接入 |
| `hook/` | HTTP 代理、IP 钩子 | ❌ 未接入 |
| `telemetry/` | 交易经验库 | ❌ 未接入 |
| `config/` | 统一配置管理 | ❌ NOFXi 用独立 YAML |
| `auth/` | 用户认证 | ❌ 未接入 |
| Web 前端 | React 完整 UI（Dashboard/Traders/Strategy/Data/Competition） | ❌ NOFXi 只有简单聊天页 |

---

## 集成优先级

### P0 — 必须接入（NOFXi 才算真正的 NOFX Agent）

1. **kernel/ 决策引擎** → 替换 NOFXi 的简单 LLM 调用
   - `BuildTradingContext()` → 完整的交易上下文
   - `GetFullDecisionWithStrategy()` → 结构化 AI 决策
   - `StrategyEngine` → 策略配置（aggressive/conservative/scalping）
   
2. **trader/auto_trader_loop.go** → NOFXi 的策略运行器直接调用
   - `runCycle()` 就是 NOFX 的核心交易循环
   - 包含：上下文构建 → AI 决策 → 执行 → 风控 → 记录
   
3. **market/data.go** → 替换 NOFXi 简单的 ticker 调用
   - 完整技术指标：EMA20/50、MACD、RSI7/14、布林带、ATR
   - 多时间维度：3min/15min/1h/4h
   - OI（持仓量）数据、资金费率

4. **mcp/client.go** → 替换 NOFXi 的简单 HTTP 调用
   - 流式支持、重试、x402 支付、上下文长度保护

### P1 — 重要增强

5. **store/ 完整数据层** → 替换 NOFXi 的简单 SQLite
   - 决策记录、订单、持仓、权益快照、AI 计费
   
6. **trader/auto_trader_risk.go** → 专业风控
   - 每日亏损限制、最大回撤、安全模式
   
7. **trader/auto_trader_orders.go** → 智能订单
   - 止损止盈、追踪止损、分批建仓

### P2 — 差异化能力

8. **kernel/grid_engine.go** → 网格策略
9. **telegram/agent/** → 升级为流式+tool calling
10. **telemetry/experience.go** → 交易经验库
11. **Web 前端整合** → NOFXi 页面集成到 NOFX 前端

---

## 集成方式

NOFXi 位于 `nofx/nofxi/`，已经是 nofx 主模块的子包。可以直接 import nofx 的所有包。

```go
// NOFXi 可以直接使用 NOFX 的核心引擎
import (
    "nofx/kernel"           // AI 决策引擎
    "nofx/market"           // 市场数据
    "nofx/mcp"              // LLM 客户端
    "nofx/store"            // 数据存储
    "nofx/trader"           // 交易执行
    "nofx/manager"          // Trader 管理器
    "nofx/config"           // 配置管理
    "nofx/wallet"           // 钱包/支付
)
```

### 架构关系

```
NOFX (交易引擎)
├── kernel/    → NOFXi.Thinking 调用
├── market/    → NOFXi.Perception 调用
├── trader/    → NOFXi.Execution 调用
├── store/     → NOFXi.Memory 调用
├── mcp/       → NOFXi.Thinking 调用
└── web/       → NOFXi Web 集成

NOFXi (AI Agent 层)
├── Agent Core     → 编排以上所有
├── Brain          → 自主决策 + 主动通知
├── Sentinel       → 异动感知
├── Learner        → 学习进化
└── Interaction    → Telegram + Web
```

**NOFXi 不是重新造轮子，是给 NOFX 装上大脑。**
