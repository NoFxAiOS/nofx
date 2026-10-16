# Area A：Binance USDT-M 合约执行与风控审查

基准：`744da725`。仅审查 Binance USDT-M 合约路径；没有修改源码，没有调用交易接口。结论为静态控制流、状态转换和交易所接口语义核验，不表示这些场景已经在该实盘账户发生。

共 11 项：5 项 CRITICAL、5 项 HIGH、1 项 MEDIUM。优先处理未知提交结果、手动仓归属、同步线程撤保护单和首笔止损延迟。

核验说明：数量正常向下量化并拒绝零量；普通开仓会核实 Hedge Mode 和 isolated margin；SL 为 `SELL/LONG`、`BUY/SHORT`，使用 `closePosition=true`；数量型 TP 不带 Hedge Mode 禁止的 `reduceOnly`。SL/TP 均明确使用 `CONTRACT_PRICE`，不是 `MARK_PRICE`，而 watchdog 的越止损判断使用 mark。没有把这些明确的接口选择本身报成缺陷。日基准已有账户级持久化，重启本身不会重置同一天的已存基准；但熔断状态仍有第 10 项的问题。`position_rebuild.go` 的按零 PnL 推断开仓算法目前没有调用点，因此未将其缺陷计入实盘发现。

## 发现

### [CRITICAL][CONFIRMED] 入场提交结果未知时直接遗弃订单，成交仓位可能永久没有程序止损
- 位置：`trader/binance/futures_orders.go:49`、`:58`、`:625`、`:645`；`trader/auto_trader_orders.go:292`、`:307`；`trader/auto_trader_pending.go:382`、`:391`、`:402`；`trader/auto_trader_stream.go:155`、`:265`；`trader/auto_trader_risk.go:1652`。
- 触发场景：Binance 接受并成交一笔市价多单，但响应丢失或返回执行结果未知的 503；`OpenLong` 返回错误，调用方在写 AI 归属和 SL/TP 前退出。该币原来没有 AI mark，且一次性迁移已经完成。后续交易所持仓虽然可见，却被当作手动仓，watchdog 不补止损。限价单同样在收到成功响应后才建立 pending：超时后实际存在的 GTC 单没有内存/数据库计划，下周期还能用新 client ID 再挂一笔；两单日后同时成交，产生额外敞口。
- 证据：
  ```go
  // binance/futures_orders.go:55-59
  NewClientOrderID(getBrOrderID()).Do(context.Background())
  if err != nil { return nil, fmt.Errorf("failed to open long position: %w", ...) }

  // auto_trader_orders.go:292-307
  order, err := at.trader.OpenLong(...)
  if err != nil { return err }
  // ...之后才执行：
  at.markAIManaged(decision.Symbol, "long", entryID)

  // auto_trader_pending.go:391-402
  if err != nil { return fmt.Errorf("failed to place limit entry: %w", err) }
  // ...之后才执行 at.setPendingEntry(...)

  // auto_trader_stream.go:155-156、275-276
  if u.OrderType != "" && !strings.EqualFold(u.OrderType, "LIMIT") { return }
  if pe == nil { return false }
  ```
- 为何现有防护不够：随机 client ID 只保证唯一，调用者无法用它恢复未知提交；没有提交前持久化的订单意图，也没有按 `origClientOrderId` 查单路径。市价 WS 事件直接被过滤；限价 WS 只匹配已知 pending order ID。订单同步写成交账本，不会恢复这里丢失的执行计划或 AI live mark；watchdog 的 `!at.isAIManaged(...)` 跳过恰好阻止兜底。启动 orphan 扫描仅处理仍挂着的 tagged 单，已经成交的单不在 open orders 中。Binance 官方明确说明某类 503 不能视为失败，须先查订单/WS 再重试：[General Info](https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/general-info)。历史检索 `git log -p -S 'NewClientOrderID(getBrOrderID())'` 未发现未知提交状态恢复机制。
- 建议修复：发送前持久化包含策略、方向、数量、保护计划和稳定 client ID 的执行意图；将超时/未知 503 记为 UNKNOWN，使用同一 ID 查单、匹配 WS 并接管实际成交；未查明前保留风险预留并阻止重复入场。重启恢复同时覆盖市价意图和限价意图。

### [CRITICAL][CONFIRMED] 归属仅按币种和方向判断，手动加仓及同向后继仓会被程序整仓平掉
- 位置：`trader/auto_trader.go:968`；`store/ai_managed.go:112`；`trader/auto_trader_orders.go:952`、`:1000`、`:1076`；`trader/auto_trader_risk.go:440`、`:1652`；`trader/binance/futures_orders.go:128`、`:189`、`:885`；`trader/auto_trader_loop.go:763`；`trader/auto_trader_stream.go:298`。
- 触发场景：AI 开 0.01 BTC LONG 后，用户手动同向加 0.02 BTC；live mark 仍只有 `BTCUSDT/long=true`。AI close、风险紧急退出或已有 `closePosition` 止损会平掉交易所的 0.03 BTC，包含用户的 0.02 BTC。另一条路径：旧 AI 仓被交易所止损平掉后，用户在下一次完整周期前手动重开同币同向；或者旧仓在停机期间平掉、用户再开仓后重启。持久化旧 mark 会让新手动仓继承 AI 身份。
- 证据：
  ```go
  // auto_trader.go:972
  return at.store.AIManaged().IsMarked(at.id, symbol, strings.ToLower(side))
  // store/ai_managed.go:115：没有校验当前仓位的入场订单或自有数量
  Where("trader_id = ? AND symbol = ? AND side = ?", traderID, symbol, side)
  // auto_trader_orders.go:1000
  order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
  // binance/futures_orders.go:137-138
  if pos["symbol"] == symbol && pos["side"] == "long" {
      quantity = pos["positionAmt"].(float64)
  }
  // binance/futures_orders.go:885
  ClosePosition(true)
  ```
- 为何现有防护不够：Hedge Mode 只能隔离 LONG 和 SHORT，无法隔离同一方向的 AI 和手动份额。虽已记录 `EntryOrderID`，执行授权只调用 `IsMarked`；平仓甚至明确不使用前面读取的 DB 数量。WS 收到 `Amount==0` 只改 realtime snapshot，不清归属；归属清理位于 `buildTradingContext` 的“上次见过、这次不存在”分支。两次周期之间完成平仓再开仓、或重启导致 first-seen map 为空时，都不会观察到这次身份切换。`896f9300` 的 R6 修正了错误清 mark 的位置，但未解决仓位代际或同侧混仓。
- 建议修复：为每次持仓建立代际和自有数量账本，实时处理成交/归零，执行前核对当前订单来源与代际。发现手动同侧增量时，不得继续使用 `closePosition` 或 `qty=0`；仅允许确定归属数量的减仓和保护。若产品要求绝不接触手动同侧份额，应使用独立子账户隔离，避免交易所聚合仓位造成不可消除的归属歧义。

### [CRITICAL][CONFIRMED] 一次性迁移在每个新 trader 上重新执行，会自动接管当时全部手动仓
- 位置：`trader/auto_trader.go:980`、`:983`、`:990`、`:1001`；`trader/auto_trader_loop.go:343`。
- 触发场景：账户已有用户手开的 ETH LONG；新建一个 trader（新 ID），或者部署时没有保留 `data/ai_marks_seeded_<id>` 文件。其第一次成功决策周期发现自己的 flag 不存在，直接给账户所有现有持仓写 AI mark。从此 watchdog、自动减仓和 AI close 都能修改/关闭这笔手动仓。
- 证据：
  ```go
  flag := fmt.Sprintf("data/ai_marks_seeded_%s", at.id)
  if _, err := os.Stat(flag); err == nil { return }
  positions, err := at.trader.GetPositions()
  // ...不检查入场订单、创建日期或是否真属于旧版本迁移：
  for _, pos := range positions {
      at.markAIManaged(symbol, side)
  }
  os.WriteFile(flag, ...)
  ```
- 为何现有防护不够：这里不是质疑升级时保留旧 AI 仓的既定迁移；问题是没有迁移版本/旧 trader 资格判定，新建 trader 也会跑同样逻辑。`git log -p -S 'seedAIManagedOnce'` 显示 `5e0792e5` 的本意是一次部署迁移；`896f9300` 改为每 trader flag 后，新 ID 天然没有 flag。归属写入之后，所有 hands-off 检查都会放行，不能识别这是错误接管。
- 建议修复：在数据库中记录一次真实的 schema/data 迁移，只允许迁移前已有的、具有旧 AI 订单证据的仓位进入名单。新 trader 默认不接管任何已有仓位；确需接管时由用户明确选择仓位和数量。

### [CRITICAL][CONFIRMED] 订单同步用另一把锁及旧持仓快照整币撤单，能撤掉刚建立的实盘保护
- 位置：`trader/binance/order_sync.go:109`、`:210`、`:465`、`:506`；`trader/binance/futures_orders.go:730`、`:766`；`trader/binance/futures_positions.go:18`；`trader/account_execution.go:55`。
- 触发场景：同步线程正在补记 X 的旧平仓成交，持仓快照显示 X 已空（账户另有 Y，因此可形成非 nil 的缓存）。用户这时重新手动开 X 并挂好 SL/TP；同步尾部复用最长 15 秒的快照，判定 X 空仓，执行整币条件单撤销。即使没有命中缓存，也存在“读到空仓 → 新入场及保护完成 → 撤单”的合法并发顺序。AI 的新保护也可能被同样撤除；手动仓的保护被撤后，watchdog 按 hands-off 永不恢复。
- 证据：
  ```go
  // order_sync.go:109-112：仅保护同步，不是 AutoTrader 的 executionMutex
  accountLock, _ := binanceSyncAccounts.LoadOrStore(exchangeID, &sync.Mutex{})
  mu := accountLock.(*sync.Mutex)
  mu.Lock()
  // order_sync.go:507-516
  if positions, err := t.GetPositions(); err == nil {
      // ...构造 openSymbols
      if !openSymbols[symbol] { t.CancelStopOrders(symbol) }
  }
  // futures_orders.go:767-769：不按旧订单 ID、方向、client ID 或所有者过滤
  t.client.NewCancelAllAlgoOpenOrdersService().Symbol(symbol).Do(context.Background())
  ```
- 为何现有防护不够：`GetPositions` 是缓存接口，cleanup 前没有 invalidate；同步锁与交易执行锁完全不同。批量撤销操作到真正执行时会包含刚创建的新单，前面读到的 flat 不能证明这些订单是旧仓残单。历史 `32668b8f` 引入 sibling 清理；后续 side-scoped 修复并没有替换这一条 symbol-wide 清理路径。此处也不检查被撤订单是不是用户手动单。
- 建议修复：同步仅记账，清理由同一账户执行协调器处理；先固定待删除的旧仓保护订单 ID/代际，只撤已证实属于该旧仓的订单，禁止 `CancelAllAlgoOpenOrders(symbol)`。人工交易不受程序 mutex 约束，仍需订单来源和代际校验，不能仅靠加锁或刷新快照。

### [CRITICAL][CONFIRMED] 首笔止损前有串行 REST 轮询，所谓 1/2/4 秒失败预算不能约束裸仓时长
- 位置：`trader/auto_trader_orders.go:310`、`:321`、`:328`、`:359`、`:646`、`:720`；`trader/binance/futures.go:103`；`trader/auto_trader_protection.go:137`；`trader/auto_trader_loop.go:20`；`trader/auto_trader_risk.go:99`；`trader/auto_trader_stream.go:155`、`:215`。
- 触发场景：市价单已经成交且 POST 返回成功，但随后 5 次 GetOrderStatus 都读超时，每次耗满 15 秒。程序至少经过约 76.6 秒才结束填价轮询；接着 fresh position 查询还可再耗 15 秒，之后才开始首笔止损逻辑或拒单退出。即使保护下单后来能成功，这段时间真实仓位已经裸露。多 pending/多决策的串行远程调用也会把限价 fill worker 排在账户锁后。
- 证据：
  ```go
  // auto_trader_orders.go:650-655
  for attempt := 0; attempt < 5; attempt++ {
      if attempt > 0 { time.Sleep(400 * time.Millisecond) }
      status, err := at.trader.GetOrderStatus(symbol, orderID)
      if err != nil { continue }
  }
  // futures.go:103
  const binanceHTTPTimeout = 15 * time.Second
  // orders.go:328-359：这些步骤都在初始保护之前
  at.invalidateExecutionCache()
  if filledQty, ok := at.positionQty(...); ok && filledQty > 0 { ... }
  // ...actualFillRisk 等
  slErr, tpErr := at.placeProtectiveOrders(...)
  // protection.go:141：只有进入保护失败路径后才开始三次等待
  for _, delay := range []time.Duration{time.Second, 2*time.Second, 4*time.Second} { ... }
  ```
- 为何现有防护不够：市价 WS 成交被过滤；30 秒 watchdog 和 WS worker 都要同一个 execution mutex，当前开仓调用尚未返回时无法介入。7 秒只是 retry sleep 合计，不包含先前查单、查仓、每次查保护/下保护的 HTTP 耗时，更不是最大裸仓时间。`896f9300` 加成交价轮询、`10-07 B1-4` 加 15 秒 HTTP timeout、`10-11` 加快速 SL 重试后，这个组合路径仍存在；已运行的重试单测注入的是即时 mock 和虚拟等待，没有覆盖慢 REST。
- 建议修复：从首次成交/确认提交起建立单调时钟总截止时间；先建立可验证的原计划紧急止损，再做非必要填价/账本工作；查单与失败恢复共用剩余时间预算。允许可靠 WS 市价回执直接触发保护，并缩短执行锁内的远程串行工作；截止时间到达必须进入持久恢复状态并执行有界退出。

### [HIGH][CONFIRMED] 全平已有部分成交仓时不撤入场余单，关闭后的仓位会自动重新打开
- 位置：`trader/auto_trader_orders.go:947`、`:999`、`:1013`、`:1074`；`trader/auto_trader_risk.go:440`；`trader/auto_trader_pending.go:531`、`:563`；`trader/binance/futures_orders.go:256`。
- 触发场景：GTC 买单数量 1，已成交 0.4、尚有 0.6 挂单；0.4 已有保护。AI 明确 `close_long`，或风险监控全平 0.4 后，0.6 入场单没有被取消。价格再回到限价，剩余 0.6 成交，账户重新出现 LONG，尽管最近的执行意图是全平。直到原单到期或方向门另行失效前，这个风险持续存在。
- 证据：
  ```go
  // auto_trader_orders.go:999-1017
  at.markCloseIntent(decision.Symbol, "long", "ai_close")
  order, err := at.trader.CloseLong(decision.Symbol, 0)
  // 成功后直接 ClearRecordedStopLoss / ClearInitialStopLoss / unmarkAIManaged
  // 没有 cancelPendingSide 或将 pending 置为 abort/recovery

  // auto_trader_pending.go:531-536
  case "PARTIALLY_FILLED":
      pe.Cycles++
      at.protectExecutedSlice(pe, status)
  ```
- 为何现有防护不够：pending 生命周期只处理方向失效、越 SL、期限及已有 RecoveryReason，不消费“本方向已主动全平”的意图；关闭与入场余单不是一个事务。`CancelProtectiveOrdersForSide` 即使被调用也只撤 STOP/TP，不撤 LIMIT。现有 `cancelPendingWithExecuted` 已能处理撤单与成交竞态，但全平路径未使用它。10-11 的换单修复只覆盖替换入场，不覆盖退出。
- 建议修复：所有全平入口先把该方向 pending 持久标记为退出中，撤销并确认余单终态，再依据最终成交累计量与当前自有净仓平仓；保留恢复任务直到“入场订单终态且自有仓位为零”。交易所 SL 全平后的余单也应走相同处理。

### [HIGH][CONFIRMED] 分批 TP 按未量化数量核验，修补不足一手的差额会升级成整仓止盈
- 位置：`trader/auto_trader_orders.go:725`、`:804`、`:897`、`:756`；`trader/binance/futures_orders.go:992`；`trader/binance/futures_positions.go:304`；`trader/auto_trader_risk.go:1700`。
- 触发场景：仓位 0.3，step/minQty 均 0.1，trend TP fraction=0.5。上层要求 TP 0.15，Binance 适配器合法向下量化成 0.1；核验仍要求 0.15，于是报 TP 不完整且不设置 runner mark。下次 watchdog/pending 修复计算缺口 0.05；这个缺口格式化失败后 `SetTakeProfit` 自动使用 `closePosition=true`，形成整仓 TP。原本可成交的分批 TP 因一次正常舍入而被修成全平。
- 证据：
  ```go
  // auto_trader_orders.go:725、812、819
  tpQty := quantity * tpFractionForMode(mode, at.effectiveTPCloseFraction())
  missing := quantity - protectiveCoverage(orders, side, kind, price)
  return t.SetTakeProfit(symbol, side, missing, price)
  // orders.go:897：比较的是未量化 tpQty，容差只有约 1e-6
  if ... protectiveCoverage(...)+math.Max(1e-9, quantities[1]*1e-6) < quantities[1] { ... }
  // futures_orders.go:993-1006
  qtyStr, qerr := t.FormatQuantity(symbol, quantity)
  if qerr == nil && qty > 0 { algo = algo.Quantity(qtyStr) } else {
      algo = algo.ClosePosition(true)
  }
  ```
- 为何现有防护不够：向下量化本身正确，错误是计划/核验/补差没有使用同一已量化目标。`21cfccfa` 保住了 -4130 重试的请求形态，但本场景在进入 -4130 重试前，第一次修补请求就已经变成 close-all。`da5ef63a` 记录了“整笔 TP 小于一手可全平”的实验选择；这里最初的 0.1 TP 完全有效，是修补差额触发了该分支，不能用该选择解释。官方将 `closePosition` 明确定义为关闭触发时该方向全部当前仓位：[New Algo Order](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade#new-algo-order-trade)。
- 建议修复：在上层按交易所步长一次性计算并保存实际 TP 目标数量，核验和补差均使用该值。差额低于一手视为无需补单或明确的 dust 状态；修补路径绝不能将有界数量请求转换为 close-all。

### [HIGH][CONFIRMED] 合法 tick 舍入超过 0.1% 时，已成功下单的止损被判缺失并触发紧急平仓
- 位置：`trader/binance/futures.go:295`；`trader/binance/futures_positions.go:368`；`trader/auto_trader_orders.go:765`；`trader/auto_trader_protection.go:137`、`:189`。
- 触发场景：交易所 tick=0.000001，计划 SL=0.00011549。适配器舍入为 0.000115，偏差约 0.424%，小于它允许的 1%，因此该 STOP 可以成功挂出；上层固定 0.1% 匹配却永远不承认它。三次快速重试仍产生同一价格，最终提交紧急退出；后续 watchdog 还可能因同一不匹配反复撤换止损。任何有效计划落在这种 tick 区间都会触发，和 API 是否正常无关。
- 证据：
  ```go
  // futures.go:299-303
  q := math.Round(price/tickSize) * tickSize
  if ... math.Abs(rp-price)/price > 0.01 { return "", fmt.Errorf(...) }
  // auto_trader_orders.go:777
  return match && orderClosesSide(o, side) && o.StopPrice > 0 &&
      math.Abs(o.StopPrice-price)/price < 0.001
  // auto_trader_protection.go:194
  if !enoughProtection(..., price, qty) { return fmt.Errorf("SL coverage unconfirmed") }
  ```
- 为何现有防护不够：量化后的实际价格没有反馈到计划、风险预算和核验；两层容差合同矛盾。`2c78e277` 已修复“必须是 tick 倍数”，但未把归一化后的值用于上层比较。现有 `TestQuantizeToTick` 只检查适配器输出，不检查它能否被 `verifyInitialStopCoverage` 承认。nearest-tick 还可把 LONG SL 向下舍入而增加真实止损距离，填后风险检查使用的仍是原计划价。
- 建议修复：入场前统一归一化 SL/TP 并基于实际价格重新算风险、RR 和数量；方向性舍入需明确，避免扩大损失预算。保护核验比较同一个规范化价格/整数 tick，取消互相矛盾的百分比容差。

### [HIGH][CONFIRMED] 普通市价替换分支漏用撤单成交保护，缓存可让旧限价成交后再开一笔市价仓
- 位置：`trader/auto_trader_orders.go:60`、`:72`、`:192`；`trader/auto_trader_pending.go:104`、`:628`、`:636`、`:649`；`trader/binance/futures_positions.go:18`。
- 触发场景：本周期已缓存持仓（另有 Y，没有 X），X 有待成交限价单；模型决定用市价开 X。撤旧限价单时与成交竞态，最终回执是 FILLED、执行数量 1，保护成功。`cancelPending` 返回 nil 并移除 pending，但普通 dispatch 丢掉 executed 数量；随后 `executeOpenLongWithRecord` 复用小于 15 秒的旧持仓缓存，看不见刚成交的 X，再提交市价单 1。真实仓位短时变为 2，甚至随后被填后风险门整仓退出。
- 证据：
  ```go
  // auto_trader_orders.go:60-63
  if err := at.cancelPendingSide(decision.Symbol, "long"); err != nil { return ... }
  return at.executeOpenLongWithRecord(decision, actionRecord)
  // auto_trader_pending.go:104-106、628-630
  return at.cancelPending(pe)
  _, err := at.cancelPendingWithExecuted(pe) // executed 被丢弃
  return err
  // 同文件 :636-644 已有正确的替换专用保护，但上述路径没有调用
  executed, err := at.cancelPendingWithExecuted(pe)
  if executed > 0 { return fmt.Errorf("old limit entry filled during replacement ...") }
  ```
- 为何现有防护不够：成功的 `protectExecutedSlice` 不刷新持仓缓存；锁只能防程序线程同时执行，不能阻止交易所撮合。`21cfccfa` 明确承诺换单若已成交应终止新入场，并确实在限价替换及 crossed-anchor 分支使用了新 helper；普通 `open_long/open_short` dispatch 仍走旧 helper。事后的 actualFillRisk 可以减轻长期超额敞口，但不能撤销已经发生的重复成交、手续费、滑点和强制退出。
- 建议修复：所有“取消旧入场后再入场”的 dispatch 统一调用 `cancelPendingForReplacement`；只要旧单曾有成交就停止新入场。撤单终态后 invalidate 并重新读取仓位，作为第二层验证。

### [HIGH][CONFIRMED] 日亏损熔断没有锁存，触线后净值稍恢复就会在同一天重新开仓
- 位置：`trader/auto_trader_risk.go:2177`、`:2194`、`:2394`；`trader/auto_trader_pending_risk.go:35`；`store/risk_state.go:20`。
- 触发场景：当日基准 1000、日亏损上限 10%；净值 895 时确认触发“停开至次 UTC 日”。浮动盈亏随后回升到 905（或账户补入资金），同日下一次检查计算亏损 9.5%，返回空字符串，允许模型重新开仓。这样可在阈值附近反复恢复交易，违反日内熔断合同。
- 证据：
  ```go
  // auto_trader_risk.go:2194-2199
  lossPct := (at.dayStartEquity - equity) / at.dayStartEquity * 100
  if lossPct >= capPct {
      return fmt.Sprintf("... opens halted until next UTC day", ...)
  }
  return ""
  ```
- 为何现有防护不够：持久化的是基准，不是熔断触发状态；没有账户/UTC 日的 `halted=true`。pending 的账户检查只是重算同一个函数，也不会保留停开状态。`git log -p -S 'lossPct := (at.dayStartEquity - equity)'` 对应 `84279f0d`，提交说明明确为触发后当日停开；这与账户回撤允许恢复后解除的另一条规则不同。已运行的日亏损测试只覆盖触线及跨日，没有覆盖同日回升。
- 建议修复：首次触线即原子持久化账户级当天熔断状态，所有 trader/new-entry/pending 检查共享；仅 UTC 日切换或用户明确重置时解除。保留原始基准和触发净值以便审计。

### [MEDIUM][CONFIRMED] BNB 手续费数值被当作 USDT，净利润和连续亏损熔断可判反
- 位置：`trader/binance/futures_account.go:202`、`:215`、`:247`、`:260`；`trader/binance/order_sync.go:444`；`store/order.go:257`；`trader/auto_trader_lossstreak.go:62`。
- 触发场景：USD-M 账户启用 BNB 抵手续费，两笔成交合计手续费 0.002 BNB；假设成交时 BNB/USDT=600，成本为 1.2 USDT，交易毛利 0.5 USDT，实际净亏 0.7 USDT。适配器把 0.002 当作 USDT，账本得到净利 0.498 USDT；这笔净亏会被视作盈利，中断本应形成的连续亏损禁开。
- 证据：
  ```go
  // futures_account.go:202、215；FromID 路径也相同
  fee, _ := strconv.ParseFloat(at.Commission, 64)
  trade := types.TradeRecord{ /* ... */ Fee: fee }
  // order_sync.go:444-445
  Commission: trade.Fee,
  CommissionAsset: "USDT",
  // auto_trader_lossstreak.go:62
  if p.RealizedPnL-p.Fee < 0 { streak++ } else { break }
  ```
- 为何现有防护不够：SDK 的 AccountTrade 提供 `CommissionAsset`，这里未读取，并在落库时覆盖为 USDT；账本和 loss-streak 直接使用未转换费用。正常用 USDT 支付手续费时不会触发。日内净值门读真实余额，不能修正逐笔净利润和连续亏损判定。官方确认 USD-M 可用 BNB 支付佣金：[Binance Futures 费用说明](https://www.binance.com/en/support/faq/detail/98488a516eb84e3eb34605683dffd554)。相关 `git log -p -S` 检索未发现币种换算；当前缺陷延续自拆分前实现。资金费也未进入逐笔净利润，但当前通知明确标注“未含资金费”，未把这一已声明口径单独作为缺陷。
- 建议修复：保留原手续费币种、数量及成交时折算汇率，风险/统计统一使用 USDT 等值费用；汇率缺失时标记净值结果未知，不能默认为极小 USDT 成本而打断亏损序列。补录资金费时按持仓区间归属，避免与直接读取权益的账户门重复扣减。

## 优化建议

1. 缓存 `exchangeInfo` 的 PRICE_FILTER、LOT_SIZE、MARKET_LOT_SIZE、MIN_NOTIONAL，按版本/TTL 更新，并让量化后的执行计划贯通 sizing、提交、核验。预期减少每次下保护前重复拉取全市场规则的延迟和请求权重，降低保护窗口及舍入导致的无效交易。当前市价 min-notional 固定 10 USDT，也应改用真实规则；超出 MARKET_LOT_SIZE 的退出应分片。
2. 记录从首次成交到 SL 已核验的单调时钟延迟，以及账户执行锁等待、REST 时长和 UNKNOWN 订单数量。预期使保护 SLA 变成可观测指标，及时识别慢 REST/长周期工作造成的裸仓，而不是仅根据“每 30 秒运行”判断安全。
3. 限价入场及部分成交采用“执行意图 → 已提交/未知 → 已成交净自有量 → 已保护/退出中”的持久状态机，并保存保护订单 ID、实际 tick 价和实际 TP 数量。预期降低重启、撤单竞态、部分减仓后的重复保护及意外再入场。
4. 增加按持仓区间的费用/资金费归属，并将划入划出现金流从用于评价交易损失的权益变化中区分出来。预期提高净 RR、连续亏损门及策略优劣统计的准确度；账户日基准仍按既定合同保存，不用当前净值覆写。
5. 把长时间市场数据抓取、历史成交同步、通知与报表工作移出账户执行临界区；执行协调器只处理新鲜快照验证和状态提交，保护任务优先调度。预期缩短限价成交后的 stop 延迟，也减少多策略共账户时的相互阻塞。

## 已执行验证与边界

- 先读取并遵守 `review-common.md`；查阅相关函数及调用者、已有测试，以及上述涉及的 `git log -p -S` 历史。只访问公开 Binance 文档，没有访问账户或调用交易 API。
- 使用本机已缓存 Go 1.25.3、`GOCACHE=$PWD/.gocache`、`GOPROXY=off`、`-mod=readonly` 运行选定现有测试：Binance 数量向下舍入/最小名义金额/trigger tick/client ID，以及 trader 日亏损基准、限价价格、entry ownership tag、初始止损重试和 WS reader 不等待执行锁；全部通过。系统默认 Go 1.24.3 不满足 go.mod，首次尝试仅报版本错误；没有下载工具链或依赖。
- 未新增测试文件、未运行全量测试、未以实盘故障注入验证。现有测试通过只证明已有用例，不覆盖本报告给出的跨模块、超时和手动混仓场景。
