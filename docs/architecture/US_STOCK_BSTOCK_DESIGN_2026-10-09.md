# 美股自动交易（币安 bStock 现货）设计方案 · 2026-10-09

状态：**已实现（2026-10-09，分支 feat/us-stock-bstock）**。用户按默认确认 §10 四项；实现与设计的差异见 §11。

## 0. 已确定的决策

| 项 | 决定 | 来源 |
| --- | --- | --- |
| 交易渠道 | 币安**现货** bStock（`AAPLBUSDT`、`SPYBUSDT` 等），普通现货 API | 用户 2026-10-09 |
| 行情数据 | 优先取 bStock 自身 K 线；某个周期根数不够时，从 Yahoo Finance（yfinance 所用的同一接口）取正股数据 | 用户 2026-10-09 |
| 入口 | 策略配置里可选"美股"（新的策略类型） | 用户 2026-10-09 |
| 开仓时段 | **默认值，待确认**：策略里可配，默认仅常规时段（美东 9:30–16:00），可勾选盘前（4:00–9:30）、盘后（16:00–20:00）；周末与美股休市日不开仓。平仓/止损任何时段都执行 | 提议 |
| 持仓周期 | **默认值，待确认**：两套预设，默认"波段"（数天–数周），可选"长线"（数周–数月） | 提议 |

## 1. 实测事实（2026-10-09）

- **bStock 现货交易对**：币安现货 exchangeInfo 中，以 `B` 结尾、USDT 计价、且与合约 `underlyingType=EQUITY` 标的同名的交易对共 **88 个**（AAPLB、NVDAB、TSLAB、SPYB、QQQB、MSTRB、COINB…）。现货 exchangeInfo 本身没有"股票"字段，需交叉匹配识别。
- **交易规则（以 AAPLBUSDT 为例）**：tickSize 0.01、stepSize 0.001、最小名义金额 5 USDT；支持 `LIMIT / MARKET / STOP_LOSS_LIMIT / TAKE_PROFIT_LIMIT`，OCO 可用，`quoteOrderQty` 市价买入可用；同时有 MARGIN 权限（本方案不用杠杆）。
- **交易时间**：bStock 全天 24/7 可交易，成交量集中在美东工作日盘中，周末清淡。
- **价格跟随**：同一时刻 AAPLB 332.29 vs AAPL 正股约 332.0，盘中基本 1:1。
- **K 线深度**：bStock **日线只有 73–121 根**（6–7 月才上线），算不了 200 日均线，周线更少；1h、15m 可取满 1000 根。→ 日线、周线需要 Yahoo 补。
- **Yahoo**：`query1/query2.finance.yahoo.com/v8/finance/chart/{SYMBOL}` 免 key 可用，返回含盘前盘后、纽约时区；并发请求会被限流（实测并发 8 个请求失败），需缓存 + 退避重试。
- **仓库现状**：
  - 现有 `market.IsBStockSymbol` 指的是**合约**里的股票永续（AAPLUSDT 永续），与本方案的现货 bStock 不是一回事，不能混用。
  - `trader/binance_stocks` 是另一个产品（`/sapi/v1/equity/*` 真实美股），未经实盘验证，本方案**不使用**，但它"现金账户、只做多、仓位由成交推导"的写法可作为参考。
  - 网格策略已经走"独立策略类型 + 独立运行周期"的模式（`strategy_type = grid_trading` → `RunGridCycle`），本方案照此新增一种类型。

## 2. 总体架构

新增策略类型 `strategy_type = "us_stock"`，与 `ai_trading`、`grid_trading` 并列：

```
策略配置 (StockConfig)
   │
   ├─ market/usstock      数据层：bStock 标的表、K 线（bStock 优先 → Yahoo 兜底）、美股交易日历与时段
   ├─ kernel/usstock      决策层：美股专用提示词、信号摘要、只做多的决策格式；复用现有 LLM 调用与决策解析
   ├─ trader/binance_bstock  执行层：币安现货下单、余额→持仓、成本价、交易所端止损/止盈
   └─ trader RunStockCycle   运行周期：按交易时段调度、风控、下单、持仓与复盘落库
```

原则：
- **不改动加密货币合约链路**。美股走自己的运行周期，合约策略的硬门、BTC 行情、OI/资金费率等全部不进入美股路径。
- **复用通用设施**：LLM 客户端、决策解析与逐条校验框架、`trader_positions` / `trade_journal`（复盘页、统计、版本效果都能直接看到美股单）、Telegram 通知、执行锁。
- 只支持币安账户。交易所账户沿用用户现有的币安 API key（需开通**现货交易**权限），资金为**现货钱包**里的 USDT，与合约钱包隔离。

## 3. 数据层（market/usstock）

### 3.1 标的表
- 启动时及每 24h：拉现货 exchangeInfo 与合约 exchangeInfo，按"现货 `{X}B/USDT` 且合约存在 `underlyingType ∈ EQUITY 类` 的 `{X}`"生成 bStock 表：`AAPLBUSDT ↔ AAPL`。
- 保留一份静态兜底映射（首次拉取失败时可用），分类失败时**拒绝开仓**而不是放行。
- 提供 API 给前端选股。

### 3.2 K 线获取规则（按周期独立判定，不拼接）
1. 先取 bStock 现货 K 线（`/api/v3/klines`，必要时分页）。
2. 若该周期**收盘完整的根数 < 所需根数**（由指标需求决定，如日线 EMA200 需 ≥ 220 根），整条该周期序列改用 Yahoo 正股数据（`AAPL`）。
3. **不把两段拼成一条**：bStock 是 24/7 的 UTC 日，Yahoo 日线是美股交易日，混在一起会让均线和 ATR 失真。
4. 每个周期在信号里标注数据来源（`bstock` / `yahoo`），提示词里写明。
5. 周线默认直接用 Yahoo（bStock 周线长期不够）。
6. 盘中执行价一律取 bStock 现价；Yahoo 仅用于指标。
7. Yahoo 调用：串行 + 本地缓存（日线 6h、小时线 15min）+ 429/5xx 指数退避；整体失败时该周期标记 `DATA_INSUFFICIENT`，不开仓。

| 周期 | 预计来源（当前上市时长下） |
| --- | --- |
| 1w | Yahoo |
| 1d | Yahoo（bStock 只有 73–121 根） |
| 4h | bStock（1000 根 1h 聚合约 250 根 4h 足够；不足则 Yahoo） |
| 1h | bStock |
| 15m | bStock（仅用于入场时点，可选） |

### 3.3 偏离保护
开仓前比较 bStock 现价与 Yahoo 正股最新价（常规时段用实时价，盘前盘后用 pre/post 价）。偏离超过阈值（默认 1%，可配）→ 不开仓，原因码 `BSTOCK_DIVERGENCE`。休市时段 Yahoo 价格不更新，偏离只作参考，按时段策略决定是否允许开仓。

### 3.4 交易日历与时段
- 时区 `America/New_York`（自动处理夏令时）。
- 时段：盘前 4:00–9:30、常规 9:30–16:00、盘后 16:00–20:00、其余为休市。
- NYSE 休市日与半日市（如感恩节次日 13:00 收盘）内置 2026–2028 年表，到期前日志告警需更新。

## 4. 决策层（kernel/usstock）

- 只做多：动作集 `open_long`（市价或限价）、`add_long`（分批加仓）、`reduce_long`、`close_long`、`adjust_stop`、`hold`、`wait`。不支持做空与杠杆。
- 信号摘要（程序计算，模型只做判断）：
  - 周线/日线趋势（EMA20/50/200、斜率、相对 52 周高低点位置）。
  - 日线 ATR、近 20 日波动率、成交量相对 50 日均量。
  - 关键位：近期摆动高低点、缺口、前高前低。
  - 大盘背景：SPY、QQQ 的日线趋势（同样按 bStock 优先、Yahoo 兜底取数）。
  - 持仓状态：成本价、浮盈亏（R 倍数）、持有天数、距止损距离。
  - 当前时段、距收盘时间、是否财报临近（如可从 Yahoo 获取，先做可选项）。
- 提示词独立成文件，按中文/英文两套，强调中长线：不追日内波动、以日线结构定止损、分批建仓。
- 决策校验：止损必须低于现价且距离在 [1×, 3×] ATR(1d) 之间（可配）；单票仓位、总仓位、最小名义金额由程序强制；不在允许时段的开仓/加仓一律改为 `wait`。

### 两套预设

| | 波段（默认） | 长线 |
| --- | --- | --- |
| 趋势周期 / 入场周期 | 日线 / 1h | 周线 / 日线 |
| 决策时点（美东，交易日） | 9:45、12:30、15:30 | 15:30 |
| 止损 | 日线结构位，1.5–3×ATR(1d) | 周线/日线结构位，2–4×ATR(1d) |
| 止盈 | 1R 后移保本 + 移动止损（日线 EMA20 或 2×ATR 追踪） | 仅移动止损（日线 EMA50 或 3×ATR） |
| 最长持有 | 30 个交易日后复核 | 不设上限，每周复核 |

预设只是初始值，策略里各参数可单独改。

## 5. 执行层（trader/binance_bstock）

实现现有 `types.Trader` 接口（与网格/合约同一接口，便于复用执行设施）：

- **开仓**：市价买入用 `quoteOrderQty`（按 USDT 金额）或数量；限价单用 `LIMIT GTC`。
- **持仓**：由现货余额中的 `{X}B` 资产（free + locked）得到持仓数量；成本价由 `/api/v3/myTrades` 按 FIFO 计算并持久化，避免每次全量回溯。
- **止损/止盈**：成交后挂交易所端 OCO（止盈限价 + 止损限价单 `STOP_LOSS_LIMIT`，止损限价比触发价低一定滑点缓冲）；只设止损时用单独 `STOP_LOSS_LIMIT`。调整止损 = 撤单重挂。程序侧再加一道保护：价格跌破止损而交易所单未成交时，市价卖出。
- **数量与价格**：按 `LOT_SIZE / PRICE_FILTER / NOTIONAL / PERCENT_PRICE_BY_SIDE` 过滤器取整与校验。
- **不支持的操作**：做空、杠杆、保证金模式 → 明确返回错误。
- **首次启动预检**：用 `/api/v3/order/test`（只校验、不成交）验证 API key 有现货交易权限且能交易 bStock；失败则策略不启动并提示原因。
- 订单统一带 `newClientOrderId` 前缀，便于区分程序单和你的手动单（与合约侧做法一致）。

## 6. 运行周期（RunStockCycle）

- 调度：按预设的美东决策时点触发（不是固定每 N 分钟）；另有每分钟一次的轻量保护检查（止损兜底、挂单状态）。
- 非允许时段：不开仓、不加仓；已有持仓照常管理，交易所端止损单 24/7 有效。
- 风控（程序强制，可配）：
  - 单票最大仓位（默认 20% 权益）、总持仓上限（默认 80% 权益）、最多持仓数（默认 5）。
  - 单笔风险：按止损距离计算仓位，单笔亏损 ≤ 权益的 1%（默认）。
  - 日内/周内最大回撤熔断：触发后当周不再开仓。
  - 财报日前后（若有数据）默认不开新仓。
- 落库：持仓写 `trader_positions`、成交写复盘日志 `trade_journal`（`ai_managed=true`），决策写 `decision_records`，与现有统计、复盘、版本效果页面打通。
- **模拟模式**：新增"仅模拟"开关，**首次上线默认开启**——完整跑数据、决策与风控，但不下单，只记录本应下的单。观察几天再切实盘。

## 7. 策略配置与界面

`StrategyConfig` 新增：

```jsonc
"strategy_type": "us_stock",
"stock_config": {
  "symbols": ["AAPLBUSDT", "NVDABUSDT", "SPYBUSDT"],   // 选股（从 88 个 bStock 中选）
  "preset": "swing",                                   // swing | position
  "sessions": { "regular": true, "pre_market": false, "after_hours": false },
  "data_fallback_yahoo": true,                         // K 线不足时用 Yahoo
  "max_divergence_pct": 1.0,
  "max_position_pct": 20, "max_total_exposure_pct": 80, "max_positions": 5,
  "risk_per_trade_pct": 1.0,
  "stop_atr_min": 1.5, "stop_atr_max": 3.0,
  "paper_trading": true
}
```

- 策略工作室的"策略类型"新增"美股（币安 bStock）"；选中后显示美股配置面板：选股（带搜索的多选，数据来自新接口 `GET /api/usstock/symbols`）、预设、时段勾选、数据源开关、风控参数、模拟模式开关。
- 中、英、印尼文文案。
- 现有加密货币策略的配置界面不受影响。

## 8. 分期与分工

| 期 | 内容 | 主要文件 | 执行者 |
| --- | --- | --- | --- |
| P1 | 数据层：bStock 标的表、K 线 + Yahoo 兜底、日历与时段、偏离保护 | `market/usstock/*` | Codex |
| P2 | 执行层：现货适配器、成本价、OCO/止损、过滤器、预检 | `trader/binance_bstock/*` | Codex |
| P3 | 配置与接口：`StockConfig`、校验、版本记录、`/api/usstock/symbols` | `store/strategy.go`、`api/*` | Sonnet |
| P4 | 决策层与运行周期：提示词、信号摘要、`RunStockCycle`、风控、模拟模式、落库 | `kernel/usstock/*`、`trader/auto_trader_stock*.go` | Opus 统筹 + Sonnet |
| P5 | 前端：策略类型、美股配置面板、i18n | `web/src/...` | Sonnet |
| P6 | 联调：模拟模式实跑数日、复核后切实盘 | — | 用户 + Opus |

P1、P2、P3 可并行；P4 依赖 P1–P3；P5 依赖 P3。每期带单元测试（HTTP 用 httptest 模拟，不触真实账户）。

## 9. 风险与未决事项

1. **API key 权限**：需确认现有币安 API key 已开现货交易，且你的账户地区可交易 bStock。P2 的预检会给出明确结论。
2. **Yahoo 非官方接口**：可能限流或改版。已设计缓存与降级（数据不足则不开仓）；长期可考虑加付费源作为第二兜底。
3. **bStock 与正股的偏离**：休市时段 bStock 仍在交易，价格可能偏离正股；默认只在常规时段开仓、并有偏离保护。
4. **交易所端止损在流动性差的时段**：`STOP_LOSS_LIMIT` 可能挂单不成交，程序兜底会市价卖出，但休市时段的滑点可能较大。
5. **分红、拆股等公司行为**：bStock 如何处理需按币安规则确认；Yahoo 日线默认使用未复权价，拆股时指标会断层——实现时使用复权收盘价计算指标。
6. **与手动交易共用账户**：你的手动单不带程序前缀，程序不会接管；但现货 USDT 余额共用，会影响可用资金。

## 10. 需要你确认

1. 开仓时段：按"策略里可配，默认仅常规时段"？
2. 持仓周期：按"波段为默认、长线为可选预设"？
3. 首次上线默认开启"仅模拟"模式，观察几天后再切实盘？
4. 风控默认值（单票 20%、总仓 80%、最多 5 只、单笔风险 1%）是否合适？

## 11. 实现说明（2026-10-09）

- 分支 `feat/us-stock-bstock`：P1 数据层 `market/usstock`、P2 执行层 `trader/binance_bstock`、P3/P5 配置与前端、P4 决策层 `kernel/stockengine` 与运行周期 `trader/auto_trader_stock*.go`。
- **持仓归属**（用户也在同一账户手动交易）：程序只管理自己 `trader_positions` 记录的数量；卖出数量 = min(程序持有, 交易所余额)，从不整仓卖出；撤单只撤 `nxbs_` 前缀的程序单；余额少于程序持有时缩减记录并告警。
- **限价入场单**：下单当日常规时段收盘未成交即撤销；停止策略时撤销程序挂单。
- **模拟模式**：独立的 `stock_paper_accounts / stock_paper_positions` 表，初始 10000 USDT，不进入实盘统计与复盘日志。切换到实盘后需把交易员的初始资金改为实际现货权益。
- 实盘数据端到端验证（模拟、不下单、AI 用固定决策）：88 个交易对；日线/周线来自 Yahoo、1h 来自 bStock；AAPL/NVDA/SPY/QQQ 价差 0.07–0.12%；开仓校验通过，按单票 20% 上限计算仓位。
- 尚未验证：真实 API key 的现货/bStock 交易权限（由启动预检给出结论）、真实 AI 模型的决策质量（需在模拟模式下观察）。

## § 价格周期扫描模式（2026-10-14）

在手动选标的之外，新增按 Oliver Kell 价格周期自动选标的的模式。

- **配置**（`StockConfig`）：`symbol_source` 三种模式——`manual`（默认）只交易 `symbols`，至少 1 只；`scan` 只交易扫描器选出的 `scan_top_n` 名加持仓，`symbols` 被忽略、可以为空；`hybrid`（混合）每周期必含 `symbols`，再加扫描器选出的 `scan_top_n` 名与持仓，`symbols` 至少 1 只。`scan_top_n`（0 = 默认 10，最大 50）；`scan_exclude`（扫描器不会选中的交易对）。`symbols` 不再有 20 只上限（格式与重复校验保留）。
- **扫描**（`kernel/stockengine/cycle.go`、`scan.go`）：用日线（260 根）对所有已上市 bStock 交易对做 `ClassifyCycle`（阶段、`LongSetup`、0–100 分、距 EMA10/20 的 ATR 倍数、相对 SPY 63 日强弱、量比），结果进程内缓存、单飞刷新。
- **刷新节奏**：启动时（扫描模式）与每 60 秒保护 tick 都调用 `WarmScanReport(20min)`——报告超过 20 分钟才在后台重扫，不阻塞；决策周期调用 `GetScanReport(ctx, 45min)`，报告不超过 45 分钟直接用，否则等一次实时扫描。扫描失败时沿用上一份报告；完全没有报告则只用持仓（混合模式另加固定标的）。
- **每周期标的列表**（扫描模式）：程序持有的标的（排序）→ 混合模式下的 `symbols` 固定标的 → 扫描器 `TopLong(scan_top_n)`（排除 `scan_exclude` 与已纳入者），去重，仅保留 `LookupSymbol` 能找到的。持仓必须始终在列表里，否则校验器会把对它的平仓/调整拒为 `UNKNOWN_SYMBOL`。列表为空（扫描不可用或没有做多形态，且无持仓、无固定标的）时本周期记为成功的空操作，不调用 AI。每周期使用配置副本（`Symbols` = 本周期列表）生成快照、提示词和校验上下文，不改动运行时配置。
- **快照**（两种模式）：`SymbolSnapshot.Cycle`——扫描报告里有该标的则用其结果，否则用本周期日线与 SPY 日线现场计算。
- **提示词**：用户提示在每个标的快照后多一行 `cycle(Kell, daily): phase=… long_setup=… score=… ext10=…ATR dist20=…ATR rs63_vs_SPY=…% vol=…x <notes>`（中文为"价格周期"）；系统提示（中英、两种模式）加入"价格周期法"一段：优先 wedge_pop / ema_crossback / base_n_break，reversal_extension 仅为早期信号，base_forming 观察，exhaustion_extension / wedge_drop / downtrend 不开仓，持仓遇 exhaustion_extension 减仓或收紧、遇 wedge_drop/downtrend 平仓或收紧；扫描分数只是候选过滤，所有程序规则不变，`wait` 随时可选。扫描模式再加一句说明候选由扫描器选出（混合模式另含手动指定的标的）且逐周期变化。输出契约不变。
- **排序方式**（`scan_rank`，仅扫描/混合模式使用）：`kell`（默认）= 现有 Kell 周期做多形态按分数排序；`momentum` = 趋势过滤动量排序（`kernel/stockengine/momentum.go`，`ClassifyMomentum`）。已有策略不配置则保持 `kell`。
- **动量规则**：只考虑日线收盘价站上 200 日均线（`close[t] > mean(close[t−199..t])`）的标的，按 12-1 动量 `close[t−21]/close[t−252]−1` 从高到低取 `scan_top_n`（同值按 RS63 降序、代码升序）；低于 SMA200 的标的永不入选（只做多）；日线不足 253 根视为无效。扫描报告每个结果带 `momentum`（`valid`/`mom12_1_pct`/`above_sma200`），`ScanReport.TopMomentum` 负责排序，不改变 `Results` 的 Kell 顺序。
- **IC 证据**：[`docs/research/2026-10-15-trend-momentum-ic.md`](../research/2026-10-15-trend-momentum-ic.md)——Kell 分数 IC ≈ −0.014（无优势），`sma200_mom` IC ≈ +0.049（t 2.6，逐年为正）；因此作为可选排序而非替换默认值。
- **提示词**（`momentum` 排序）：系统提示增加一行说明候选按动量排序、Kell 阶段只是入场时机参考（回踩 EMA10/20 优先、exhaustion_extension 不追高、wedge_drop/downtrend 等待），Kell 段落中的禁入条款相应改为「wedge_drop/downtrend 绝不开仓，exhaustion_extension 不追高」；快照多输出 `mom12_1=… above_sma200=…`（仅在有效时）。决策周期日志为 `scan top N rank=kell|momentum: …`。
- **接口**：`GET /api/usstock/scan` 返回 `{"scanning":bool,"generated_at":RFC3339|null,"results":[…],"skipped":{…}}`；无缓存报告时后台启动扫描并返回 `scanning:true`。
