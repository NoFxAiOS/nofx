审查基于 `c7045ac4`，确认 **5 项问题：2 HIGH、3 MEDIUM**。未发现 paper 执行器误触真实下单或真实账户的路径；以下问题会污染模拟收益、风险表现及晋升依据，未重复上次报告中的已修复项。

**1. [HIGH] 止损市价单始终按触发价成交，跳空时会显著少计亏损**

位置：[trader/paper/paper.go:493](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:493)。

触发场景：多仓止损为 90，下一次报价已经跌到 60。模拟器仍按 `89.973` 成交，而按当前报价及相同滑点应为 `59.982`，每单位少计亏损 `29.991`。历史分钟线整个价格区间都低于止损价时，也会生成区间外的成交价。

证据代码：

```go
hit = o.PositionSide == "LONG" && low <= o.StopPrice || ...
price, maker, reason = p.slipped(o.StopPrice, o.Side), false, "stop_loss"
```

报价撮合虽然传入了当前价格，但成交定价完全忽略它。固定滑点不能覆盖触发价与实际可成交价格之间的跳空。

建议修复：报价触发按当前可执行价格施加不利滑点；历史回放补充开盘价并处理跨越触发价的跳空，避免按已不存在的价格成交。

**2. [HIGH] 历史回放只用分钟收盘价检查强平，穿越强平线后反弹的账户会“复活”**

位置：[trader/paper/paper.go:430](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:430)、[trader/paper/paper.go:514](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:514)。

触发场景：初始资金 100，以默认成本在报价 100 开多 9 单位、10 倍杠杆；尚无有效止损，例如限价离线成交后程序未及时恢复保护。随后一分钟最低 85、收盘及当前报价均回到 100。

按模拟器自己的公式：

- 价格 85：权益约 `-35.720135`，维持保证金为 `3.825`，已满足强平条件。
- 收盘 100：权益约 `99.279865`，维持保证金为 `4.5`，检查通过，仓位继续存活。

证据代码：

```go
p.s.Marks[e.symbol] = e.c.Close
p.match(e.symbol, e.c.Low, e.c.High, ...)
p.liquidate(...)
```

`Low/High` 只参与订单触发，强平始终读取已经设置成收盘价的 `Marks`。这会高估离线或缺少保护期间的账户存活率。

建议修复：回放时加入分钟内强平阈值穿越检查。单持仓场景可以直接判断；多资产、对冲场景需制定一致的保守路径规则，不能仅检查收盘权益。

**3. [MEDIUM] 浮点尾差会把已全平记录重新改成 OPEN，导致晋升统计漏掉整笔交易**

位置：[trader/paper/paper.go:299](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:299)、[store/paper_ledger.go:113](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/store/paper_ledger.go:113)、[scripts/promotion/report.py:312](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/scripts/promotion/report.py:312)。

触发场景：开仓 `0.8`，先平 `0.1`，再平 `0.7`。这些数量均可满足 `0.001` 步长，但 float64 运算结果为：

```text
0.8 - 0.1       = 0.7000000000000001
再减 0.7       = 1.1102230246251565e-16
```

证据代码：

```go
pos.Quantity -= qty
if pos.Quantity <= 0 {
    delete(p.s.Positions, key)
}
```

共享持仓构建器原本会把最后一笔识别为全平，但 paper 的后置修正又执行：

```go
if trade.RemainingQuantity > 0 {
    updates["quantity"], updates["status"] = trade.RemainingQuantity, "OPEN"
    updates["exit_time"], updates["exit_order_id"], updates["close_reason"] = 0, "", ""
}
```

结果是余额已经结算，持仓记录却重新变成 `OPEN`。晋升脚本只统计 `status='CLOSED'`，因此会漏掉该仓累计盈亏、手续费和成交样本。

建议修复：按交易数量步长归一化剩余数量，或使用整数手数记账；账本与持仓表统一使用归一化后的全平判定。不能仅用 `> 0` 区分真实剩余仓与浮点噪声。

**4. [MEDIUM] TAKE_PROFIT_MARKET 被按 Maker 计费，并且完全不应用市价滑点**

位置：[trader/paper/paper.go:483](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:483)、[trader/paper/paper.go:496](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:496)。实盘对应订单类型见 [trader/binance/futures_orders.go:1187](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/binance/futures_orders.go:1187)。

触发场景：任意止盈市价单成交。

证据代码：

```go
price, maker, reason, hit := o.Price, true, "", false
// TAKE_PROFIT_MARKET 分支没有修改 maker
price, reason = o.StopPrice, "take_profit"
```

`execute` 随后根据 `maker` 选择费用。默认费率下，110 USDT 的止盈成交收取 `0.022`，按配置中的 Taker 费率应为 `0.055`，同时还漏计了市价滑点。实盘适配器实际提交的是 `AlgoOrderTypeTakeProfitMarket`，两侧成本模型不一致。

建议修复：止盈市价单使用 Taker 费率，并按方向应用市价成交与滑点规则；Maker 模型仅用于真实限价成交。现有测试也断言了该错误费用，需要同步纠正。

**5. [MEDIUM] 合法的零手续费、零滑点配置被执行器静默替换成默认值**

位置：[store/exchange.go:374](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/store/exchange.go:374)、[trader/paper/paper.go:110](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/trader/paper/paper.go:110)。建对脚本也允许零值：[scripts/promotion/create_pair.py:134](/Users/zhangyun/workspace/nofx/.claude/worktrees/project-review-bug-check-3a1440/scripts/promotion/create_pair.py:134)。

触发场景：用户或脚本明确配置费用、滑点为 0，保存和加载校验均成功。

证据代码：

```go
if cfg.TakerFeeBps == 0 { cfg.TakerFeeBps = 5 }
if cfg.MakerFeeBps == 0 { cfg.MakerFeeBps = 2 }
if cfg.SlippageBps == 0 { cfg.SlippageBps = 3 }
```

数据库保存的实验配置与实际执行成本不一致，会改变成交盈亏和不同换手频率策略的相对排名。

建议修复：区分“未提供”与“明确为零”。默认值在配置解析层补齐，执行器保留已经校验通过的零值。

**验证与边界**

- `go build ./...`、paper 全部测试、store/trader/manager 的 paper 相关测试，以及选定的 paper 并发 race 测试均通过。
- Python 统计计算测试 9 项通过；上述浮点尾差和强平数字另用内存算术复核。未新增复现测试文件，现有测试通过不覆盖上述场景。
- 资金费在实现中明确未模拟，本次未将其作为新增计算 bug。
- 未调用交易接口，未修改源码或配置，测试缓存已清理。审查期间出现的范围外 `web/` 改动未触碰或回退。