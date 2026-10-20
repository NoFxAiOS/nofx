审查基准：`c7045ac4`。确认 **8 项问题：HIGH 7 项、MEDIUM 1 项**，未确认 CRITICAL。以下标明上次报告中仍未修复的问题；已修复项不重复列入。

全程未编辑源码、未调用交易接口。触发场景说明代码风险，不代表账户已发生相应事故。

---

**1. [HIGH] 未成交的反向开仓参与净风险抵扣，前序订单失败后可突破方向风险上限**

位置：[trader/auto_trader_risk.go:2045](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_risk.go:2045)、[trader/auto_trader_loop.go:343](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_loop.go:343)、同文件 `373–387`。

触发场景：单笔风险预算 2%，净方向风险上限 3%。同批依次提交多 A、空 B、空 C，每笔风险接近 2%。预检查计算的净风险依次为 2%、0%、2%，全部放行。随后 A 因实时 RR、保证金或交易所拒单而失败，B/C 仍继续执行，实际净空风险接近 4%。

证据：

```go
projectedLong := longRisk + at.cycleLongRiskReservedUSD
projectedShort := shortRisk + at.cycleShortRiskReservedUSD
netPct := math.Abs(projectedLong-projectedShort) / equity * 100
```

整个批次先经过 `applyHardRiskGates`，之后才执行。失败分支只释放总风险和单币预留，没有重新验证剩余决策；执行入口及成交后检查也不重新检查净方向上限。反向限价单尚未成交时，同样不能提供实际对冲。

建议：未成交预留分别约束多头、空头的最坏风险，不得作为已成交对冲抵扣；每笔执行前按实际持仓重算，订单失败或撤销后重新验证后续决策。

---

**2. [HIGH] 候选池修复未覆盖行情失败边界：已标记 filtered 的币仍能绕过信号硬门和风险预留**

位置：[kernel/engine_analysis.go:380](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/engine_analysis.go:380)、[trader/auto_trader_risk.go:2293](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_risk.go:2293)、同文件 `2415–2431`、`1959–1960`。

这是对上次候选池问题修复不完整的核验。

触发场景：市价默认模式下，某候选行情获取失败，但其他候选正常，LLM 调用继续。模型输出该失败候选的开仓决策；执行时行情接口恢复，订单即可继续。

证据：

```go
if err != nil {
    markFiltered(ctx, coin.Symbol, "market data unavailable/stale")
    continue // 没有 removeCandidate
}
```

```go
for _, coin := range ctx.CandidateCoins {
    pool[market.Normalize(coin.Symbol)] = true
}
```

该币没有渲染行情，也没有 `GateState`，但仍属于允许开仓的池。市价模式下 `gs == nil` 不拒绝；`marketExceptionGate` 在 `LimitEntryEnabled=false` 时直接放行。

若模型同时省略 `price`，该币又没有 `MarketDataMap`，账户风险检查还会以无法定价为由返回零预留：

```go
if equity <= 0 || entryPx <= 0 || d.StopLoss <= 0 {
    return false, "", 0
}
```

因此后续实时 SL/TP 校验即使通过，也没有补回缺失的信号授权和账户风险预留。

建议：开仓资格必须排除 `FilteredCandidates`，并要求有效行情及本轮方向授权；无法定价的新风险应拒绝。

---

**3. [HIGH] 最终决策截断时，仍可能执行 reasoning 中已经弃用的 JSON 草案**

位置：[kernel/engine_analysis.go:545](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/engine_analysis.go:545)、同文件 `549–569`。

上次 B 报告问题仍未修复。

触发输入示例：

```text
<reasoning>弃用草案 [{"symbol":"BTCUSDT","action":"close_long"}]；
最终选择持有。</reasoning>
<decision>[{"symbol":"BTCUSDT","action":"hold"}
```

证据：完整 `<decision>...</decision>` 匹配失败后，解析器回退全文搜索数组：

```go
jsonPart = s
jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
```

本次使用源码正则在内存中复核，实际选中的是 `close_long`。对于归程序管理、已过平仓锁定期的持仓，该指令能够进入真实平仓路径。

建议：出现 `<decision>` 后固定解析边界；标签或最终载荷不完整时拒绝执行。裸 JSON 兼容路径要求整个载荷完整解析，禁止全文搜寻任意数组。

---

**4. [HIGH] 旧的未收盘快照会随处理时间推进被认定为最终收盘；美股缓存也存在同类问题**

位置：[kernel/signal_layer.go:2407](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/signal_layer.go:2407)、同文件 `3079`；[market/usstock/yahoo.go:59](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/market/usstock/yahoo.go:59)、同文件 `123`、`153–165`。

期货路径为上次 B 报告遗留；本次进一步确认美股 Yahoo 缓存路径受影响。

期货触发场景：14:59:30 获取 14:45–15:00 的形成中 K 线，随后采集其他币或等待 LLM。15:00 后重算趋势时，同一份中间 OHLCV 被当成最终收盘数据。

```go
if time.UnixMilli(last.Time).Add(dur).After(now) {
    klines = klines[:len(klines)-1]
}
```

判定只使用处理时钟，没有采样时刻或最终收盘标记。

美股路径更持久：Yahoo 日线原始响应缓存 6 小时。例如 15:00 ET 缓存了当天未完成日线，16:00 后仍命中该缓存；`yahooBars` 用新的 `nowFunc()` 判断当天已经收盘，便接纳缓存中的盘中值。这会污染选股、ATR 和新增的篮子波动率仓位缩放。

建议：保存快照采样时间及收盘状态，不能随调用时间推进改变同一快照的收盘属性；缓存跨 K 线或交易日收盘边界时强制刷新最终数据。

---

**5. [HIGH] 复盘硬规则仍使用模型可省略的 price，止损距离规则可以静默失效**

位置：[trader/auto_trader_rules.go:48](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_rules.go:48)、[kernel/rule_engine.go:116](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/rule_engine.go:116)、同文件 `188`。

上次 B 报告问题仍未修复。

触发场景：启用硬规则 `stop_loss_pct > 4 → block`。市价决策省略 `price`，真实入场 100、SL 95、TP 120。真实止损距离为 5%，但规则看到的是零，开仓继续通过其他通用风控。

```go
kernel.CheckDecisionAgainstRules(rules, d, equity, d.Price)

if price > 0 {
    c.stopLossPc = absPct(price, d.StopLoss)
}
```

执行端不重新运行复盘规则。`forceMarketEntries` 还会将模型输出的限价决策改成市价并清零 `Price`，同样进入这个缺口。反向条件如 `risk_reward < 2` 则可能错误拦截正常市价单。

建议：使用程序确定的最终入场报价或限价锚点构造规则事实；无法取得必要事实时明确拒绝新增风险。订单类型或价格改变后重新检查规则。

---

**6. [HIGH] 同币双向持仓的退出信号仍采用第一条持仓方向**

位置：[kernel/engine_prompt.go:1042](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/engine_prompt.go:1042)、同文件 `1808–1817`；[kernel/signal_layer.go:1265](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/signal_layer.go:1265)。

上次 B 报告问题仍未修复。

触发场景：同币同时持有多仓和空仓，列表第一条为多仓。分别渲染两侧持仓时，`computeCoinSignal` 都重新从列表头查找同币第一条：

```go
if market.Normalize(p.Symbol) == market.Normalize(data.Symbol) {
    opt.PositionSide = ...
    break
}
```

因此空仓也采用多头退出条件。下跌趋势继续、收出阴线时，程序可能向空仓输出 `exit_rule_triggered=true`，尽管对应空头退出条件为 false。超过持仓锁定期后，模型据此提出的错误平空能够执行。

建议：由当前被渲染的 `PositionInfo` 显式传入方向；退出信号按 `symbol+side` 生成，不能从同币第一条持仓推断。

---

**7. [HIGH] “日线上涨禁空”仍未保证获取日线，缺失数据会使该风控失效**

位置：[kernel/market_regime.go:51](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/market_regime.go:51)、[kernel/signal_layer.go:3004](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/signal_layer.go:3004)、[trader/auto_trader_risk.go:2611](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/auto_trader_risk.go:2611)。

上次 B 报告问题仍未修复。

触发场景：普通加密币配置 `BlockShort1dUptrend=true`，时间周期选择 `15m/1h/4h`。运行时只补充 1h/4h；额外补充日线仅适用于 bStock。

```go
if !market.IsBStockSymbol(symbol) {
    return out
}
```

没有日线时，`TimeframeTrend(..., "1d")` 返回空字符串，而执行门只拒绝 `"up"`：

```go
if rc.BlockShort1dUptrend {
    trend := kernel.TimeframeTrend(ctx.MarketDataMap[d.Symbol], "1d")
    if trend == "up" {
        continue
    }
}
```

实际日线处于上涨趋势也能开空；挂单重检查存在相同缺口。

建议：启用此风控时强制将 `1d` 加入获取和重检查列表；日线缺失或历史不足时拒绝新开空仓。

---

**8. [MEDIUM] 行情陈旧检测仍不检查 K 线时间，过期序列可以继续驱动交易**

位置：[market/data.go:1068](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/market/data.go:1068)、同文件 `405`、`445–449`；[kernel/signal_layer.go:2097](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/kernel/signal_layer.go:2097)。

上次 D 报告问题仍未修复；与第 4 项不同，此处是已经过期的历史序列。

触发场景：供应商返回停在 30 分钟前的 3m 序列，其最后几根有正常波动和成交量；实时价格又恰好接近旧收盘价。

`isStaleData` 仅检查最后五根是否价格冻结且成交量为零，不比较 `OpenTime/CloseTime`。价格有波动即返回正常；供应商偏差门只检查价格差：

```go
math.Abs(*sig.DataFreshness.VendorDivergencePct) > opt.MaxVendorDivergencePct
```

旧价格与现价接近即可通过。信号虽计算了结算年龄，却没有将超龄作为硬门，旧结构和 ATR 仍可用于开仓计划。

建议：按各周期核验最新应有的已收盘时间，并检查时间连续性；超龄后切换来源，仍无法取得新数据则拒绝新增风险。

---

验证结果：

- `GOCACHE="$PWD/.gocache" GOPROXY=off go build ./...` 通过。
- 决策解析、`factor_ensemble`、`liquidity_sweep` 定向测试通过。
- `market/usstock` 测试及 `trader` 的 `TestBatch2Gates*` 测试通过。
- `factor_ensemble live_scores` 导出/读取和 `liquidity_sweep` 本轮未发现可确认的新资金风险。
- 审查期间出现的 `web/` 并发改动未触碰；本报告直接返回，未写入仓库。