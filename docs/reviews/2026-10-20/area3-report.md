**美股区域只读审查报告**

基准提交：`c7045ac4`。对照了 `docs/reviews/2026-10-16/`，重点复核 `a1371d79`、`8c271d34`、`0ddbad23`。确认 **12 项问题：CRITICAL 2 项、HIGH 7 项、MEDIUM 3 项**，以下不重复已修复的旧问题。

未修改源码、未写报告文件、未调用交易接口。`go build ./...` 和相关 stock/regime/protection 离线测试通过，使用 `GOCACHE=$PWD/.gocache`；以下异常分支依据代码和状态转换确认，未做实盘复现。

---

**1. [CRITICAL] 撤销单独止损单会改变客户端 ID，随后对账永久失败，仓位失去止损且无法自动退出**

位置：[binance_bstock/protection.go:94](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance_bstock/protection.go:94)、[auto_trader_stock_persist.go:82](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_persist.go:82)，调用链为 `stockPrepareProtectionChange → stockCancelProtection → stockCheckProtection`。

**触发与影响：** 持仓只有止损单、未设置 TP。调止损、减仓、平仓或止损兜底均会先撤旧保护单。撤单请求主动指定新 `clientOrderId`，但最终状态查询仍要求客户端 ID 等于原值：

```go
// protection.go:94
NewCancelOrderService().Symbol(symbol).OrderID(o.OrderID).
    NewClientOrderID(clientID()).Do(context.Background())

// stock_persist.go:82-83
if ... (r.ClientOrderID != "" && client != r.ClientOrderID) {
    return out, fmt.Errorf("... client ID mismatch")
}
```

Binance 文档明确说明，撤单参数 `newClientOrderId` 会替换被撤订单的 `clientOrderId`。[官方说明](https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-api.md#cancel-order-trade)

因此，**旧止损已撤，但卖出或重挂尚未执行，函数就返回错误**。数据库中的旧单仍未记为终态，下轮继续因同一 ID 不匹配而隔离；保护 tick 和 AI 交易动作均无法恢复。这是新订单对账修复引入的回归。

**建议修复：** 消费撤单响应，用 `origClientOrderId` 验证来源并持久化新 ID／别名；最终成交对账以账户、标的、不可变 `orderId` 为依据，允许经核验的客户端 ID 变更。仅省略该参数不够，服务端也可能自动生成撤单 ID。

---

**2. [CRITICAL] 限价成交记账与成交水位分开提交，重启会重复认领并卖出手动股票**

位置：[auto_trader_stock_protect.go:199](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_protect.go:199)、[auto_trader_stock_persist.go:207](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_persist.go:207)，另见 `savePending:328`、`restoreStockPendings:315`。

**触发与影响：** 原限价单已持久化 `FilledQty=0`，随后成交 2 股。程序成功增加持仓、挂好保护，但在保存新成交水位前崩溃，或者 `savePending` 写库失败：

```go
stockBuyFilled(... inc, avg, ...) // 先提交持仓变化并挂保护
p.FilledQty = executed
at.savePending(p)                // 独立写库，失败只记日志
```

重启恢复的水位仍为 0，同一笔 2 股再次传给 `stockRecordBuy`；已有持仓直接执行：

```go
pos.UpdatePositionQuantityAndPrice(rows[0].ID, qty, price, fee)
```

这里没有订单成交幂等校验。若账户还持有足够手动股票，余额不会暴露重复认领：真实程序仓 2 股会被记录成 4 股，之后保护单或全平会额外卖出 2 股手动股票。

**建议修复：** 将入场累计成交水位和持仓增量放入同一数据库事务；以订单 ID 和累计成交量实现幂等。持久化失败必须保留恢复状态，不能仅记录日志后继续。

---

**3. [HIGH] 首次保护单失败后被永久隔离，承诺的自动补挂实际不会发生**

位置：[auto_trader_stock_exec.go:156](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_exec.go:156)、[auto_trader_stock_state.go:194](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_state.go:194)，另见 `stock_protect.go:96`、`stock_cycle.go:580`。

**触发与影响：** 第一次买入某标的成功、持仓落库成功，但首笔保护单被明确拒绝。例如入场名义金额刚超过 5 USDT，保护单按更低的止损限价计算后不足最低名义金额。

```go
// stockProtectOrEmergency
default:
    logf("PROTECTION FAILED ... the protection tick will retry")
// 函数不返回失败，stockBuyFilled 仍返回成功

// 下一轮 stockGatherLive
if len(owned[sym]) > 0 && !live[sym] && !check.Known {
    quarantine(sym, "missing_records", ...)
}
```

首次保护失败没有产生任何保护单记录，因而每轮都满足 `missing_records`。保护 tick 跳过隔离仓位，AI 的平仓、调止损也被拒绝，裸仓无法自行恢复。

**建议修复：** 持久化“已确认买入、首次保护待建立”状态，区分新仓保护明确失败与升级前归属不明仓位；前者必须重试保护或进入有界紧急退出流程。

---

**4. [HIGH] 同账户多个美股交易员会互相接管保护单，一笔成交可被重复记作两人的退出**

位置：[binance_bstock/protection.go:18](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance_bstock/protection.go:18)、[auto_trader_stock_persist.go:30](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_persist.go:30)，另见 `stock_exec.go:114`、`stock_state.go:165`。创建接口 `api/handler_trader.go:475` 对实盘账户复用仅告警，允许该配置。

**触发与影响：** 同账户 A、B 两个美股交易员各持有同一标的 5 股。执行器只按全局 `nxbs_` 前缀识别程序保护：

```go
strings.HasPrefix(o.ClientOrderID, clientPrefix) // 没有 trader 归属
```

B 建立保护时会发现并撤掉 A 的保护，再仅为 B 的 5 股挂单。A 下一轮又将 B 的订单登记到自己的名下：

```go
StockProtectionOrder{TraderID: at.id, OrderID: leg.id, ...}
```

于是账户总持仓 10 股只有 5 股保护，同一保护订单还会出现在两个 trader 的对账表里。该单成交 5 股后，两人各扣减 5 股并关闭记录，实际剩余 5 股变为无人管理。账户执行锁只能串行化操作，不能解决这种归属错误。

**建议修复：** 至少强制同账户同标的只有一个自动管理者；完整方案需在账户层统一分配库存和保护成交，按明确归属订单 ID 撤单，禁止仅凭公共前缀接管其他 trader 的保护。

---

**5. [HIGH] 切回模拟盘的撤单修复会删除已成交入场单的恢复记录，留下无保护实盘仓位**

位置：[auto_trader_stock_cycle.go:110](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_cycle.go:110)、[binance_bstock/orders.go:232](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance_bstock/orders.go:232)。

**触发与影响：** 重载前保留实盘限价单；停机期间它全部或部分成交；重载后的策略为模拟盘。清理逻辑只看撤单错误：

```go
if err := exec.CancelOrder(r.Symbol, r.OrderID); err != nil {
    continue
}
at.store.StockPaper().DeletePending(at.id, r.Symbol)
```

但 `CancelOrder` 对已经 `FILLED` 的订单直接返回成功：

```go
case OrderStatusTypeFilled, OrderStatusTypeCanceled, ...:
    return nil
```

清理逻辑不消费最终成交量、不记实盘持仓、不挂保护，直接删掉恢复记录。部分成交后成功撤掉余单也有相同问题。随后模拟盘不会再对账这些真实成交。

**建议修复：** 撤单后查询最终累计成交，先补记并保护未处理的实盘成交，再删除 pending；实盘恢复未完成时应保留独立恢复任务，不能因切换模拟模式丢弃。

---

**6. [HIGH] 全平或止损退出不撤限价入场余单，仓位关闭后会重新买入**

位置：[auto_trader_stock_exec.go:177](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_exec.go:177)、[auto_trader_stock_protect.go:187](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_protect.go:187)，另见 `stock_cycle.go:586`。

**触发与影响：** 限价买入 10 股，已经成交并保护 4 股，剩余 6 股仍挂着。AI 全平、人工全平或止损退出只撤保护并卖出当前 4 股：

```go
stockPrepareProtectionChange(sym, now) // 只处理保护单
...
at.stockTrader.SellMarket(sym, qty)
```

退出路径没有取消或标记 `at.stock.pendings[sym]`。该 pending 后续仍会被正常消费：

```go
inc := executed - p.FilledQty
stockBuyFilled(... inc ...)
```

价格再次触及限价时，剩余 6 股重新建立仓位，违背最近的全平／止损意图。`PENDING_ENTRY` 只阻止新入场决策，不会终止已有余单。

**建议修复：** 所有全平入口先持久化退出意图，撤销入场余单并读取最终成交，再平掉最终自有数量；恢复任务持续到“入场单终态且仓位归零”。

---

**7. [HIGH] regime 减仓部分成交后，仍记录目标系数已完全应用，剩余减仓永久丢失**

位置：[auto_trader_stock_regime.go:119](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_regime.go:119)、[auto_trader_stock_exec.go:219](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_exec.go:219)。

**触发与影响：** 原持仓 100 股、已应用系数 1，当前系数下降到 0.5，计划卖出 50 股。市价单仅成交 20 股时，卖出函数仍返回成功并保护剩余 80 股：

```go
remaining := owned - res.ExecutedQty
fullNow := remaining <= 1e-9
...
return out, nil
```

regime 层没有比较计划数量与实际成交量：

```go
case xerr == nil || tooSmall:
    scales.Set(at.id, h.Symbol, regime.Scale)
```

于是保存 `AppliedScale=0.5`。后续当前系数仍为 0.5，减仓条件不再满足；目标应剩 50 股，实际长期剩 80 股。

**建议修复：** 持久化目标剩余数量及已完成成交量；未达到目标时继续恢复任务。不能把“发生了部分成交”当作“目标系数已应用”。

---

**8. [HIGH] regime 卖出与应用系数落库之间没有原子恢复机制，重启会重复按比例减仓**

位置：[auto_trader_stock_regime.go:101](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_regime.go:101)，同文件 `:122`；持仓卖出记账位于 `auto_trader_stock_exec.go:221`。

**触发与影响：** 100 股按 `1 → 0.5` 成功减至 50 股，持仓记录已更新，但在 `scales.Set` 前崩溃，或该写入失败：

```go
out, xerr := at.execStockExitReason(...) // 已成交、已更新持仓
...
if err := scales.Set(... regime.Scale); err != nil {
    logf("regime scale save ... failed") // 仅日志
}
```

重启读到当前持仓 50 股、旧系数仍为 1，再次计算卖出一半，变成 25 股。若系数写入持续失败，每个周期都会再次按比例减仓。

**建议修复：** 下单前保存稳定的减仓计划和目标数量，成交后将持仓变化、成交水位和已应用状态一起提交；重启按原计划剩余量恢复，不能对剩余仓位重新套用原比例。

---

**9. [HIGH] `binance_stocks` 适配器始终成功返回空仓，股票也不计入净值**

位置：[binance_stocks/trader.go:124](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance_stocks/trader.go:124)、同文件 `:72`、`:268`；生产接入点为 `trader/auto_trader.go:391`。

**触发与影响：** 交易员配置 `exchange=binance_stocks`，走通用 AutoTrader 分支。`posCache` 没有任何赋值路径，`GetPositions` 永远返回空数组：

```go
// 注释称 AutoTrader 会逐标的刷新，实际没有这种调用
return []map[string]interface{}{}, nil
```

唯一逐标的持仓查询只被 `CloseLong` 使用。所有真实股票因此从模型持仓、敞口检查和保护监控中消失；而该适配器的 `SetStopLoss` 明确返回不支持，程序止损又依赖这份空持仓快照。

同时 `GetBalance` 只累加现金：

```go
if asset != "USDC" && asset != "USD" && asset != "USDT" { continue }
...
"totalEquity": total
```

例如 10,000 现金买入 2,000 股票，价格未变，净值却变为 8,000，产生虚假 20% 回撤。

**建议修复：** 在允许该适配器实盘自动运行前，实现完整持仓快照、股票估值和程序止损；无法读取账户状态时返回明确失败，不能返回成功空仓。

---

**10. [MEDIUM] 限价单后续成交使用累计均价记增量，持仓成本、手续费和盈亏错误**

位置：[auto_trader_stock_protect.go:196](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_protect.go:196)、[binance_bstock/orders.go:205](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance_bstock/orders.go:205)。

**触发与影响：** 同一限价单第一次成交 1 股、价格 100，第二次再成交 1 股、价格 80。查询接口返回累计均价 90：

```go
avg = number(o.CummulativeQuoteQuantity) / executed
```

但调用者将 90 当成第二股的成交价：

```go
inc := executed - p.FilledQty
incFee := fee * inc / executed
stockBuyFilled(... inc, avg, incFee, ...)
```

最终记账均价为 `(100+90)/2=95`，真实均价应为 90。手续费也错误地按累计手续费比例分摊，而非减去已处理手续费，随后已实现和未实现盈亏都会失真。

**建议修复：** 保存累计成交金额和累计手续费水位；增量价格使用 `ΔQuote/ΔQty`，增量手续费使用 `ΔFee`，与持仓及数量水位原子更新。

---

**11. [MEDIUM] 美股周熔断只在内存锁存，重启或配置重载可提前解除**

位置：[auto_trader_stock_cycle.go:364](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_cycle.go:364)、[auto_trader_stock.go:115](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock.go:115)。

**触发与影响：** 周初净值 100，跌到 85 触发本周禁止开仓；之后回升至 95。此时重启或重载，代码虽然恢复周初净值，却无条件清除熔断状态：

```go
at.stock.weekKey, at.stock.weekStartEquity, at.stock.weekHalted =
    key, base, false

if ddPct > stockWeeklyDrawdownHaltPct && !at.stock.weekHalted {
    at.stock.weekHalted = true
}
```

当前回撤只有 5%，不会再次触发，策略在同一周恢复开仓，与“熔断后持续至下周”的行为不符。`weekHalted` 没有持久化写入或恢复路径。

**建议修复：** 按 trader、运行模式和 ET 周起点持久化熔断触发状态，只在进入新周时解除。

---

**12. [MEDIUM] regime 应用系数由模拟盘与实盘共用，模式切换会漏减仓或重复减仓**

位置：[store/stock_regime_scale.go:14](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/store/stock_regime_scale.go:14)、[auto_trader_stock_regime.go:59](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_stock_regime.go:59)。

**触发与影响：** 同一 trader 切换模拟／实盘。两种模式都读写同一张系数表，主键只有：

```go
TraderID string `gorm:"...;primaryKey"`
Symbol   string `gorm:"...;primaryKey"`
```

而清理逻辑依据当前模式的持仓删除记录：

```go
h := state.Holdings[sym]
if (h == nil || ...) && at.stock.pendings[sym] == nil {
    scales.Delete(at.id, sym)
}
```

因此，模拟盘没有该标的时，会删除仍然存在的实盘应用系数；切回实盘后缺失值按 1 处理，可对已经缩放的仓位再减一次。反过来，模拟盘同标的写入较低系数，会使未执行对应减仓的实盘仓位被认为已经完成缩放。

**建议修复：** 系数及其清理范围必须区分模拟／实盘，并绑定账户和持仓代际；模式切换时分别恢复各自状态。