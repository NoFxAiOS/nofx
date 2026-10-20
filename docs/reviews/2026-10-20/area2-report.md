基于 `HEAD c7045ac4`，对照 `docs/reviews/2026-10-16/` 及后续修复，确认 **7 项问题：3 项 CRITICAL、4 项 HIGH**。其中包含修复残留和本轮新确认的路径；已修复问题不复报。

全程未修改源码或报告文件，未调用交易接口。构建、测试使用了指定的 `.gocache`。

---

**1. [CRITICAL] `ErrSubmitUnknown` 未被执行层处理，查回失败后仍会遗弃真实订单**

位置：`trader/binance/futures_orders.go:147`；`trader/auto_trader_orders.go:297`；`trader/auto_trader_pending.go:391`；`trader/auto_trader_loop.go:373`；`trader/auto_trader_stream.go:275`。

触发场景：Binance 已接受市价或 GTC 入场单，但提交响应丢失，随后三秒内查单也因网络故障失败。

证据：

```go
// futures_orders.go:147：明确返回 UNKNOWN
return nil, false, fmt.Errorf("%w (%s clientOrderId=%s): submit: %w; lookup: %v",
    ErrSubmitUnknown, symbol, clientID, submitErr, lookupErr)

// auto_trader_orders.go:297-300
order, err := at.trader.OpenLong(...)
if err != nil {
    return err
}
```

执行层没有消费 `ErrSubmitUnknown` 的分支：市价路径在写 AI 归属和保护计划前返回；限价路径在建立 pending 前返回。主循环将其作为普通失败处理并释放风险预留。WS 成交恢复又要求已有 pending，无法接管这笔订单。

因此，市价成交仓位可能一直被视为手动仓而没有止损；未成交限价单则可能与下一周期的新订单同时成交。**客户端订单号查回修复只覆盖了短时间内查回成功的情况。**

建议：提交前持久化订单意图、稳定 client ID 和保护计划；UNKNOWN 必须保留账户风险预留、阻止同侧重复入场，并持续通过 REST/WS 恢复，直到确认终态。

---

**2. [CRITICAL] 全平修复在确认仓位归零前撤保护、清归属，部分成交余仓会永久脱管**

位置：`trader/binance/futures_orders.go:280`、`:307`、`:325`、`:390`；`trader/auto_trader_orders.go:1015`、`:1021`；辅助证据：`trader/auto_trader_decision.go:299`。

触发场景：AI 仓位为 1，程序提交全平市价单；盘口流动性或价格限制使其最终只成交 0.4，余量过期，仍剩仓位 0.6。

证据：

```go
// futures_orders.go:325-328：提交成功即撤保护，没有检查成交终态或剩余仓位
if fullClose {
    if err := t.CancelProtectiveOrdersForSide(...); err != nil {
        ...
    }
}

// auto_trader_orders.go:1017-1021
at.ClearRecordedStopLoss(decision.Symbol, "long")
at.ClearInitialStopLoss(decision.Symbol, "long")
at.ClearExitMode(decision.Symbol, "long")
at.ClearPeakPnLCache(decision.Symbol, "long")
at.unmarkAIManaged(decision.Symbol, "long")
```

这里调用的 `recordAndConfirmOrder` 对 Binance 直接返回，交给 OrderSync 记账，**并不等待平仓确认**。后续 watchdog 又会跳过已经被清除 AI 标记的余仓。

这是新增 `fullClose` 判断生效后放大的风险：原先未撤掉的 SL，现在会随未完成的平仓一起被删除。Binance 官方确认市价单可能部分成交后过期：[Futures 官方说明](https://www.binance.com/en-NG/blog/futures/421499824684903916)。

建议：建立持久化的平仓恢复状态；保留原保护和 AI 归属，直到订单终态且新鲜仓位快照确认归零。存在余仓时继续平仓或保留有效止损，不能以 POST 成功作为生命周期完成。

---

**3. [CRITICAL] 离线成交恢复会给已不存在的仓位创建 AI 标记，随后接管手动后继仓**

位置：`trader/auto_trader_reconcile.go:234`、`:250`、`:269`；`trader/auto_trader.go:1018`；`trader/auto_trader_risk.go:1652`。

触发场景：

1. AI 限价入场单在程序离线期间成交，pending 行仍在数据库。
2. 用户在重启前已经手动平掉该仓。
3. 重启恢复查到订单 `FILLED`，但当前没有仓位。
4. 用户随后手动开同币同方向仓位。

证据：

```go
// auto_trader_reconcile.go:234：查询现仓之前就创建归属
at.markAIManaged(row.Symbol, row.Side, row.OrderID)

// :269-275：确认无现仓后，仅删除 pending 行，没有清除此标记
if !found {
    if err := at.store.PendingEntry().Delete(...); err != nil {
        ...
    }
    return
}
```

运行时授权仍只读取 `(trader, symbol, side)` 标记：

```go
return at.store.AIManaged().IsMarked(at.id, symbol, strings.ToLower(side))
```

无仓恢复分支也没有建立 `positionFirstSeenTime`，正常周期的“仓位消失”清理无法移除这个标记。新手动仓因此会被 watchdog 补保护、调整或整仓关闭。删除一次性迁移并未堵住这条自动接管路径。

建议：确认当前确有本次成交对应的自有仓位后再建立 live mark；确认空仓时按 entry order ID 条件清理旧标记，避免误删新一代仓位归属。运行时授权需要持仓代际及自有数量。

---

**4. [HIGH] 同步清理仍存在“读到空仓后撤掉新仓保护”的竞态**

位置：`trader/binance/order_sync.go:109`、`:507`、`:526`；`trader/binance/futures_orders.go:434`、`:457`、`:473`；辅助证据：`trader/account_execution.go:55`。

触发顺序：

1. OrderSync 用新鲜快照确认 X/LONG 空仓。
2. AutoTrader 开出新的 X/LONG，并成功挂 SL/TP。
3. OrderSync 随后列出条件单，将新 SL/TP 当作孤儿撤销。

证据：

```go
// order_sync.go:109：同步锁与 AutoTrader 的账户执行锁不同
accountLock, _ := binanceSyncAccounts.LoadOrStore(exchangeID, &sync.Mutex{})

// :525-526
if !openSides[symbol][side] {
    if err := t.CancelProgramProtectiveOrdersForSide(symbol, side); err != nil {
        ...
    }
}
```

新取消方法在空仓判断**之后**重新查询订单，只按方向和 broker 前缀过滤，没有核对旧仓代际或旧保护单 ID。刷新缓存及过滤手动单修复有效，但不能消除上述并发顺序。

结果是已核验成功的新保护被异步删除，仓位至少裸露到下一次修复；若撞上初始保护核验，还可能触发无必要的紧急平仓。

建议：清理由账户执行协调器串行处理，并只撤销事先确认属于旧仓的保护单 ID；不要根据空仓快照，随后重新枚举并撤销该侧所有程序保护。

---

**5. [HIGH] 市价订单按限价锚点计算风险预留，可绕过单币及账户总风险上限**

位置：`trader/auto_trader_risk.go:2513`、`:2019`、`:2059`；`trader/auto_trader_orders.go:563`；`trader/auto_trader_fill_risk.go:51`。

触发场景：开启限价模式，同时允许市价例外。GateState 保存的是限价锚点，但实际执行为市价。

证据：

```go
// auto_trader_risk.go:2513-2519：不区分市价/限价，覆盖风险计算价格
basis := gs.LongEntryPrice
if strings.Contains(d.Action, "short") {
    basis = gs.ShortEntryPrice
}
if basis > 0 {
    entryPx = basis
}

// :2019
candidateRisk := d.PositionSizeUSD * math.Abs(entryPx-d.StopLoss) / entryPx
```

具体可通过的数值：

- 净值 10,000，单币风险上限 1.5%，即 150。
- 同币已有 LONG 止损风险 130。
- 新 SHORT 名义金额 1,000，市价 100，限价锚点 101，SL 103，TP 94。
- 门禁预留风险：`1000 × 2 / 101 ≈ 19.80`，合计 149.80，放行。
- 市价实际数量为 10，新增止损风险为 30，合计 **160，超过单币上限**。

成交后检查只验证该笔是否超过单笔预算；30 加交易成本远低于 150，仍会通过。5% 容差无法弥补总量门使用错误价格的问题。

建议：市价使用实际执行参考价，限价使用最终规范化限价；动作转换后重新计算风险。最终 sizing 及成交恢复还应复核账户、方向和单币总风险。

---

**6. [HIGH] 日亏锁存仍依赖开仓意图，纯持仓周期触线不会被锁存**

位置：`trader/auto_trader_loop.go:115`；`trader/auto_trader_risk.go:2485`；`trader/auto_trader_pending.go:450`。

触发场景：日基准 1,000，上限 10%。没有 pending 入场单时，本周期净值已降至 890，但模型只返回 `hold/wait/close`；下一周期净值恢复至 950，模型提出新开仓。

证据：

```go
// auto_trader_loop.go:115：这里只锚定基准
at.anchorDailyBaseline(ctx.Account.TotalEquity)

// auto_trader_risk.go:2485-2486：只有开仓决策才检查并写锁存
if strings.HasPrefix(d.Action, "open_") {
    if halt := at.dailyLossHaltBlocks(rc, ctx.Account.TotalEquity); halt != "" {
        ...
    }
}
```

另一个调用点 `pendingAccountHaltReason` 只有存在账户 pending 时才运行。因此，890 这个已被周期读取的触线净值不会写入锁存；恢复至 950 后，新开仓仍被允许。

建议：每次获得有效账户净值快照后立即评估并锁存，放在候选池判断和模型调用之前；保护监控也应在没有 pending 时评估日亏状态。入场门只消费熔断状态。

---

**7. [HIGH] pending 保护修复会重新补挂已成交的分批 TP，重复卖掉 runner**

位置：`trader/auto_trader_pending.go:536`、`:742`、`:785`；`trader/auto_trader_orders.go:730`、`:747`、`:817`。

触发场景：trend 模式，TP 比例 50%，其他自动减仓关闭。入场 GTC 总量 20，已成交 10，仍有 10 挂单；已挂的 TP 5 成交后，剩余 runner 5。价格回落到 TP 下方，但未触及移动止损，下一次 pending 检查运行。

证据：

```go
// auto_trader_pending.go：每次部分成交轮询都重做累计量保护
at.protectExecutedSlice(pe, status)

// :742、:785
pe.ProtectedQty = 0
slice := executed

// auto_trader_orders.go:730、:817
tpQty := quantity * tpFractionForMode(mode, at.effectiveTPCloseFraction())
missing := quantity - protectiveCoverage(orders, side, kind, price)
```

原入场订单的 `executedQty` 仍为 10，已成交的 TP 已不在 open orders 中，于是程序再次补挂 TP 5。下一次触及目标就会卖掉全部 runner。

pending 路径没有记录已完成的 TP 数量，也不读取 `tpRunnerDone`。此问题不需要发生数量舍入，与上次报告的 TP dust 问题不同。

建议：持久化 TP 目标量和已成交量，按“目标减已成交减仍挂数量”补单；累计入场量不能等同于当前剩余仓位。SL 覆盖修复和已完成的 TP 阶段需要分别处理。

---

验证结果：

- `GOCACHE="$PWD/.gocache" go build ./...`：通过。
- Binance `TestReviewK*`：通过，使用内存 HTTP transport。
- 成交风险、5% 容差、风险敞口、日亏锁存、部分成交及实时保护相关定向测试：通过。
- `TestBinanceFullCloseCancelsOwnSideProtection`：因沙箱禁止本地端口监听而无法运行。
- **5% 单笔风险容差比较式本身未发现错误**；它仍保留结构 SL/TP、净 RR 和强平距离检查。上述发现来自调用链及状态转换核验，现有通过测试未覆盖这些边界。