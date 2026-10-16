# Area C 评审报告：美股（币安现货 bStock）策略

范围：`trader/auto_trader_stock*.go`、`trader/binance_bstock/*.go`、`kernel/stockengine/*.go`、`store/stock_*.go`、`market/usstock/*.go`；基准提交 744da725。
方法：只读通读上述文件，并核对 `trader/account_execution.go`、`auto_trader_risk.go`、`auto_trader.go` 中相关调用。未运行交易类调用。下列行号均取自当前工作树。

---

### [CRITICAL][CONFIRMED] 交易所端止损/止盈成交后，若用户在同一标的有手动持仓，程序会保留“幽灵持仓”，之后可能卖出或保护用户自己的股票
- 位置: `trader/auto_trader_stock_state.go:160-169, 207-245`；`trader/auto_trader_stock_protect.go:101-117`；`trader/auto_trader_stock_exec.go:111-154`
- 触发场景:
  1. 程序持有 5 股 AAPLB（trader_positions 里的行），用户手动另持有 M 股 AAPLB，余额 = 5 + M。
  2. 交易所端 STOP_LOSS_LIMIT（或 OCO 的止盈腿）成交，卖出这 5 股。余额变为 M。
  3. 下一次 `stockGatherLive(reconcile=true)` 时：
     - 若 M ≥ 5：`qty-balance > step/2` 不成立，不触发任何收缩，程序仍认为自己持有 5 股。
     - 若 0 < M < 5：进入 `stockShrinkOwned`，但“交易所端成交”的识别只在 `target <= 0` 时才做。程序把记录收缩为 M 股，当作“外部减仓，不计盈亏”，行仍保持 OPEN，实际等于把用户的 M 股认领为程序持仓。
  4. 此时 `GetProtection` 返回 nil，`h.ProtMissing=true`。60 秒保护 tick 取 `persistedStop`。若价格 ≤ 止损价，就走 `stockFallbackSell(h.Owned)`，市价卖出用户的手动股票。若价格仍在止损价上方，则在用户的股票上重新挂止损 OCO。
  5. LLM 下一周期也会看到这个并不存在的持仓，可能 add/close/adjust，同样落在用户的股票上。
- 证据:
  ```go
  if reconcile && qty-balance > maxFloat(info.StepSize, 1e-9)/2 {   // state.go:160
      newQty := at.stockShrinkOwned(...)
  ...
  if target <= 0 { ... lp.Stop ... "stop_loss" ... }                // state.go:214-245，只有余额归零才识别成交
  ...
  case h.ProtMissing: ... if price > 0 && price <= stop { at.stockFallbackSell(ctx, h, now, ...) }  // protect.go:101-109
  ```
  全局搜索可知，没有任何地方查询程序保护单的订单状态（`GetOrderStatus` 只用于限价入场单）。持仓归属只靠“余额 vs 记录”推断。
- 为何现有防护不够: “卖出数量 = min(程序持有, 余额)” 只能防止卖出超过余额，防不了余额里属于用户的部分。设计文档 §11 的“从不整仓卖出”假设了程序记录始终准确，而这里记录在止损成交后失真。仅当用户在该标的零手动持仓时才安全。
- 建议修复: 保护单成交后必须按订单对账，而不是按余额推断。方案：
  - 记录保护单的 orderId / orderListId，每个 tick 先用 `GetOrderStatus` 或带 `nxbs_` 前缀的 myTrades 查这些订单。
  - 若发现 FILLED / PARTIALLY_FILLED，则用实际成交量和成交均价记账，并按成交量扣减程序记录。
  - 不再靠“余额下降”做归属判断，也不要在余额 > 0 时把记录收缩成余额。
  - 兜底：`ProtMissing` 且记录数量与“程序订单成交净额”对不上时，只报警，不自动卖出。

---

### [HIGH][PLAUSIBLE] 止损限价缓冲仅 0.3%，且兜底市价卖出最少延迟 2 个 tick（约 1–3 分钟），跳空/夜盘薄流动性时止损可能长时间不成交
- 位置: `trader/auto_trader_stock.go:72, 75`；`trader/auto_trader_stock_exec.go:40-42`；`trader/auto_trader_stock_protect.go:118-122`
- 触发场景: bStock 7×24 交易，财报后、周末或夜盘价格可以在一两个 tick 内穿过 `stop×(1-0.3%)`。STOP_LOSS_LIMIT 触发后，限价单挂在价格上方不成交。程序要等 `stopTicks>=2`（连续 2 个 60 秒 tick 价格 ≤ 止损）才撤单并市价卖出。夜盘买卖价差通常大于 0.3%。
- 证据:
  ```go
  stockStopLimitBuffer = 0.003
  stockStopFallbackTicks = 2
  return floorStep(stop*(1-stockStopLimitBuffer), info.TickSize)
  ```
  现货不支持市价止损（设计文档记录的 `STOP_LOSS_LIMIT/TAKE_PROFIT_LIMIT`），限价缓冲是唯一的滑点参数。
- 为何现有防护不够:
  - 单笔 1% 风险的仓位计算（`sizing.go`）按“止损价成交”算，没有计入跳空。
  - 兜底卖出依赖 tick，而 tick 在 `TryLock` 拿不到锁时直接 return（见下一条），决策周期或期货周期进行时兜底会被整段跳过。
- 建议修复:
  - 缓冲改为按标的 ATR%/价差动态取值（例如 max(0.5%, 0.25×ATR1d%)）。
  - 兜底触发改为“价格已穿过止损且限价单挂着未成交”时 1 个 tick 内处理。
  - 在 tick 里直接比较 `price <= stopLimit` 判断“限价已被穿越”。

---

### [MEDIUM][CONFIRMED] 设计文档承诺的“财报前后不开新仓”没有程序硬门，只有一句提示词
- 位置: `kernel/stockengine/validate.go:78-179`（无财报检查）；`kernel/stockengine/prompts.go:65`；设计文档 §6
- 触发场景: LLM 在财报前一天（`earn_days=0/1`）开仓，止损放在 1.5–3×ATR。财报盘后跳空 8–15% 直接穿过止损和 0.3% 缓冲，实际亏损远超“单笔 1%”的预算。手动模式下提示词甚至可能没有 universe 表（`freshScanResults` 只在有新鲜扫描报告时提供），模型根本看不到财报日期。
- 证据: `DaysToEarnings` 只在 `universe.go` / `prompts.go` 里用于展示；`validateOne` 里没有任何引用。
- 建议修复: 在 `validateOne` 增加硬门：若 `DaysToEarnings` 已知且 ≤ N（默认 2），拒绝 open_long / add_long，原因码 `EARNINGS_NEAR`。财报日期未知时，对非 ETF 按配置决定放行或拒绝。

---

### [MEDIUM][CONFIRMED] 美股周期整段持有账户级执行锁（含 Yahoo 抓取、扫描和最多 2 次 LLM 调用）；保护 tick 用 TryLock，拿不到就整轮跳过
- 位置: `trader/auto_trader_stock_cycle.go:392-399`；`trader/auto_trader_stock_protect.go:23-27`；`trader/account_execution.go:55-60`；`trader/auto_trader_risk.go:99-102`
- 触发场景:
  - 锁键是 `userID:exchangeID`。美股交易员与期货交易员共用同一币安账户时，共用同一把锁。美股决策周期（串行 Yahoo 抓取 + LLM + repair，可能数分钟）期间，期货的 30 秒保护看门狗 `safeProtectionPass` 和回撤监控会被阻塞。
  - 反过来，期货 AI 周期（注释写明可持锁数分钟）期间，美股 `stockProtectionTick` 的 `TryLock` 失败，直接 return，没有日志，也没有计数。此时美股的补挂保护、兜底卖出全部停摆。
- 证据:
  ```go
  mu := at.stockMutex(); mu.Lock(); defer mu.Unlock()   // cycle.go:395-397，整个 runStockCycle
  if !mu.TryLock() { return }                           // protect.go:24
  ```
- 建议修复: 数据抓取与 LLM 调用放在锁外，只在下单阶段加锁。保护 tick 的补挂/兜底路径应当用阻塞锁加短超时，并在多次跳过时告警。

---

### [MEDIUM][PLAUSIBLE] 市价买单遇到超时/5xx（成交状态未知）时不做对账，可能留下无记录、无止损的持仓
- 位置: `trader/binance_bstock/orders.go:13-17, 50-64`；`trader/binance_bstock/trader.go:54`；`trader/auto_trader_stock_exec.go:189-196`
- 触发场景: `BuyMarketNotional` 使用 15 秒 HTTP 超时和随机的 clientOrderId。请求在交易所已成交后超时，`place` 返回错误，`stockOpenLive` 直接 `return nil, err`，没有写 trader_positions，也没有挂保护。下一周期这部分股票只会出现在 `manualNotes` 里，被当成用户的手动持仓。
- 证据: `place()` 注释自己写着 “Even a timeout may have executed at the exchange”，但调用方没有任何按 clientOrderId 或 `nxbs_` 前缀的事后查询。
- 建议修复: 下单前生成并保存 clientOrderId；遇到非明确拒绝的错误时，用 `GET /api/v3/order?origClientOrderId=` 或查最近的 `nxbs_` 订单确认状态，成交了就按正常买入流程记账并挂保护。

---

### [MEDIUM][CONFIRMED] 周回撤熔断与风险敞口使用整个现货钱包的权益（含用户手动仓位、USDT 划转、以 B 结尾的非股票币）
- 位置: `trader/auto_trader_stock_cycle.go:335-358`；`trader/binance_bstock/account.go:63-100`；`trader/binance_bstock/rules.go:28`；`trader/auto_trader_stock_state.go:128-150`
- 触发场景:
  1. 用户把 USDT 在现货与合约钱包之间划转，或充提：一次提走超过 10% 即触发“周回撤熔断”，本周不再开仓。反过来，充入资金会掩盖真实回撤。
  2. `BStockTrader.symbols()` 只按 “BaseAsset 以 B 结尾且 symbol == base+USDT” 筛选，不做股票分类，所以 BNBUSDT、ARBUSDT、TRBUSDT 等也算“bStock”。这些币的余额按市值计入 `totalEquity` 与 `Exposure`，币价波动直接影响回撤熔断、仓位上限和风险额。
  3. 用户手动股票的盈亏同样影响程序的熔断。
- 证据: `GetBalance` 遍历 `rules`（全部以 B 结尾的 USDT 交易对）累加 `qty*mark`；`stockWeekRisk` 用该权益对比周初第一份快照。
- 为何现有防护不够: 设计文档 §9.6 只说明“USDT 共用”，没有处理划转和非股票币。
- 建议修复: 熔断与 R 仓位只基于程序自有仓位（trader_positions）的已实现与未实现盈亏，加上分配给本策略的资金上限。余额类持仓限定为 `usstock.LookupSymbol` 命中的标的。

---

### [MEDIUM][CONFIRMED] 重启后切成模拟盘时，程序无法撤销上次保留的实盘限价入场单，只会每次启动报警
- 位置: `trader/auto_trader_stock_cycle.go:58-62, 69-101`；`trader/auto_trader.go:322-327`；`trader/auto_trader_stock.go:190-192`
- 触发场景: 用户保存配置把策略由实盘改为模拟（`RemoveTraderForReload` → `StopForRestart` 保留挂单）。新实例 `newStockExecutor` 在模拟模式返回 nil，`at.stockTrader == nil`。`cancelOrphanStockPendings` 对每一行都走 “no executor” 分支，不撤单，也不删库里的 pending 行。交易所上的限价买单继续存在，成交后无人保护；每次启动还会重复报警。
- 证据:
  ```go
  if at.stockTrader == nil { failed = append(failed, label+": no executor"); continue }
  ```
  744da725 的测试里 `fx.spot` 是注入的假执行器，生产环境的模拟模式并没有这个执行器，所以测试没有覆盖这个路径。
- 建议修复: 切换到模拟模式前（`RemoveTraderForReload` 时判断新配置是否为模拟），不要保留挂单，直接撤销；或者在 `cancelOrphanStockPendings` 里按需用凭证临时构造执行器完成撤单。

---

### [MEDIUM][CONFIRMED] 买入已成交但 trader_positions 写库失败时，直接返回错误，不挂保护单
- 位置: `trader/auto_trader_stock_exec.go:158-164`
- 触发场景: 成交已发生，`stockRecordBuy` 因 SQLite 锁/磁盘问题失败，函数返回错误而没有进入 `stockProtectOrEmergency`。这部分股票既无记录又无止损，之后被当成用户手动持仓。
- 证据:
  ```go
  owned, err := at.stockRecordBuy(...)
  if err != nil { return nil, fmt.Errorf("fill recorded on the exchange but the position row failed: %w", err) }
  ```
- 建议修复: 写库失败时仍按成交数量立即挂保护单（或紧急卖出），并强告警；同时把成交写入本地重试队列。

---

### [MEDIUM][CONFIRMED] 市价卖出部分成交时，`full=true` 仍把整条记录关闭，剩余股票既无记录也无保护
- 位置: `trader/auto_trader_stock_exec.go:143-152`
- 触发场景: 薄流动性时段（夜盘、周末）的市价卖单只成交一部分（现货市价单吃完深度后剩余部分过期）。`fullNow := full || remaining < step`，而 `full` 在平仓和兜底卖出时恒为 true。记录被整体 `ClosePositionFully`，`lastProtect` 被清除，剩余股票没有保护，也不再属于程序。
- 证据:
  ```go
  remaining := h.Owned - res.ExecutedQty
  fullNow := full || remaining < math.Max(info.StepSize, 1e-9)
  ```
- 建议修复: 以 `remaining` 为准：`fullNow := remaining < step`。仍有剩余时，保留记录并重新挂保护；兜底卖出场景要重试卖出。

---

### [MEDIUM][CONFIRMED] LLM 决策（最多两次调用）之后，按旧快照价格直接下单，没有再次校验价格、偏离和止损
- 位置: `trader/auto_trader_stock_cycle.go:41-73, 145`；`trader/auto_trader_stock_cycle.go:302-333`；`kernel/stockengine/decisions.go:211-233`
- 触发场景: 快照（价格、Yahoo 偏离、`RefFresh`）在 LLM 调用前取得，调用加 repair 可能耗时 1–3 分钟。`execStockEntry` 用 `snap.Price` 定仓位，并以该价格下 `BuyMarketNotional`。期间价格若大幅变动，成交价可能低于止损或已超出 1% 偏离阈值。止损风险按旧价计算，实际可能被立即触发而紧急卖出。
- 证据: `execStockEntry` 里 `entry := snap.Price` 与 `size, err := stockengine.SizeOrder(sizing, d)` 只用快照；下单前没有 `GetMarketPrice` 对比。
- 建议修复: 下单前拉取最新 bStock 价格（以及最新 `GetQuote`），超过阈值（例如 0.5% 或偏离超限）就放弃或重新定量；同时检查 `stop < price`。

---

### [LOW][PLAUSIBLE] 市价卖出使用决策周期开头读取的 `Balance`，止损单在窗口内成交时可能卖出用户的手动股票
- 位置: `trader/auto_trader_stock_exec.go:111-126`
- 触发场景: 周期开头读取余额；LLM 调用期间（锁被周期持有，但交易所端止损单仍可成交），止损单成交，程序后续对同一标的发出 close/reduce。`qty = min(qty, min(h.Owned, h.Balance))` 使用的是旧余额，因此仍会卖出 Owned 数量，吃掉用户的手动股票。
- 建议修复: `CancelProtection` 之后重新读取余额与保护单状态，再计算卖出量。

---

### [LOW][PLAUSIBLE] 单个标的的成交历史出错会让 `GetBalance/GetPositions` 整体失败，进而让保护 tick 整轮停摆
- 位置: `trader/binance_bstock/account.go:79-96, 123-126`；`trader/binance_bstock/history.go:67-90`；`trader/auto_trader_stock_protect.go:85-89`
- 触发场景: 账户里任一以 B 结尾的标的（含 BNB、ARB 等）有无法解析的成交（例如手续费资产无法换算，`applyFill` 返回错误），`CostBasis` 报错，`GetBalance` 失败。`stockTickLive` 在 `stockGatherLive` 出错时只写日志就 return，所有标的的补挂保护和兜底卖出都不再执行。
- 建议修复: 持仓估值对单个标的的成本错误降级处理（按成本未知继续），不能让它阻断保护流程。

---

### [LOW][PLAUSIBLE] 挂着的限价入场单不计入 MAX_POSITIONS 与总敞口上限，成交后可能超限
- 位置: `kernel/stockengine/validate.go:34-45, 157-159`；`trader/auto_trader_stock_exec.go:204-208`
- 触发场景: 限价单只占用 USDT（计入 `Available` 的减少），不计入 `Exposure` 与 `held`。多个限价单在同一天陆续成交，持仓数和敞口可能超过配置的上限；`PENDING_ENTRY` 只防止同一标的重复。
- 建议修复: 校验时把挂单数量和名义金额按成交处理，计入 `held`、`cycleNotional`。

---

## 优化建议

1. 模拟盘贴近实盘：`stockTickPaper` 止损按止损价成交（`protect.go:59-63`），没有跳空滑点和手续费（`store/stock_paper.go` 的 `OpenOrAdd`/`Reduce` 无费率）。建议按触发后的实际价格或 `stop×(1-缓冲)` 成交并计入手续费，这样模拟结果才能用来判断是否切实盘。预期影响：避免因模拟收益偏高而过早上线。
2. 价位口径统一：Yahoo 日线、周线经 adjclose 复权（`yahoo.go:209-216`），小时线、4 小时线未复权，而下单是 bStock 的未复权价。摆动高低点、EMA 来自复权价，给分红股票的历史价位略偏低（1–2%）。建议用未复权 OHLC 给价位，复权只用于动量与收益计算。预期影响：止损位更贴近真实价格。
3. 保护单状态对账：除第一条致命问题的修复外，把保护单的 orderId 写入库，重启后直接按订单状态恢复，不依赖 `lastProtect`（重启后丢失移动止损，会按 `InitialStop` 重挂，见 `persistedStop`）。
4. 跳空敞口预算：在仓位计算中加入“跳空缓冲”（例如风险额 × 0.7，或财报窗口内风险额减半），预期可显著降低尾部亏损。
5. 兜底卖出提速：把 tick 周期缩短到 15–20 秒，或订阅用户数据流，对止损触发后的限价未成交做秒级响应。
