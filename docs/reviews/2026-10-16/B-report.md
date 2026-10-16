# Area B：期货决策引擎审计

审计基线：744da725。只读检查所列 kernel、trader 与 store 风控配置代码，并沿真实执行链读取必要的调用方和 Binance 适配器；未修改源码、配置或 git 状态，未调用实盘交易接口。以下共 10 项：1 项 CRITICAL、9 项 HIGH；CONFIRMED 表示代码路径及错误结果已核实，不代表已观察到对应实盘事故。

## 审计发现

### [CRITICAL][CONFIRMED] 趋势 runner 转换按 symbol 撤全部止盈，会撤掉对侧手工仓的 TP
- 位置: trader/auto_trader_vol.go:599、690；trader/binance/futures_orders.go:465、480、507；trader/auto_trader_risk.go:1674；trader/auto_trader_orders.go:756。
- 触发场景: Binance hedge mode 下，同一币有 AI 多仓与手工空仓。trader 在 AI 多仓到达 TP 前重启：内存 tpRunnerDoneMap 被重建为空，watchdog 从现有订单恢复 openTP，却没有恢复 runner 标记。随后 AI 多仓的分批 TP 成交、价格达到 TP 距离，剩余仓进入趋势 runner 分支。程序调用 symbol 级撤单接口，将手工空仓的 TAKE_PROFIT / TAKE_PROFIT_MARKET 一并撤销。手工仓此后即使重新达到原目标也不会按用户原计划止盈。
- 证据:
  ~~~go
  // auto_trader_vol.go
  if tp > 0 && tpRunnerDue(side, posEntryPrice(pos), tp, markPrice) && !at.tpRunnerDone(posKey) {
      _ = at.trader.CancelTakeProfitOrders(symbol)
      at.markTPRunner(posKey)
  }
  // Binance legacy；algo 分支同样没有 positionSide / clientOrderID 过滤
  if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
      ...NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID)...
  }
  ~~~
- 为何现有防护不够: 新开分批 TP 正常成功时 placeProtectiveOrders 会预先 markTPRunner，故不能声称所有新仓都会走到此分支；但该标记仅保存在内存 map（auto_trader.go:480；auto_trader_vol.go:690–702），watchdog 恢复 TP 价格（auto_trader_risk.go:1674–1679）时没有恢复它，上述重启路径可达。isAIManaged(symbol, side) 仅证明当前正在管理的那一侧属于程序，不能授权修改同 symbol 的另一侧。watchdog 对手工仓保持 hands-off，也不会补回其 TP。适配器已经提供 CancelTakeProfitOrdersForSide（futures_orders.go:346），本调用点仍使用全 symbol 接口。
- 建议修复: runner 只撤本次计划登记的、归属于该 trader 与 positionSide 的 TP 订单；至少改用按侧接口，再按程序订单 ID / client order ID 限定所有权。仅在确认撤单成功后标记 runner。
- 历史核对: git log -p -S 'CancelTakeProfitOrders(symbol)' 显示该调用来自 32668b8f；现有 ForSide 接口注释明确记载过“撤掉对侧 hedge TP”的同类事故，当前调用点仍遗漏。

### [HIGH][CONFIRMED] 市价默认模式允许模型交易候选池外甚至已排除的币，并跳过对应方向硬门
- 位置: kernel/engine_analysis.go:474；kernel/engine_position.go:99、260；kernel/engine.go:1031；trader/auto_trader_risk.go:2323、2336、2412、2465；trader/auto_trader_orders.go:136；trader/auto_trader_pending_risk.go:217；trader/auto_trader_marketdata.go:13。
- 触发场景: limit_entry_enabled=false，本轮另有正常候选使 LLM 得到调用机会。模型幻觉输出被 excluded_coins 排除的 SOLUSDT 市价多单；账户 equity=1000、名义价值=100、杠杆=3、实时价=100、SL=95、TP=120，满足通用仓位、保证金、止损带与净 RR。输出可以不填 price（正是市价提示词要求）。该币不存在本轮行情和 GateState，却仍可进入下单路径。
- 证据:
  ~~~go
  // kernel 对单个决策校验，没有候选白名单参数/检查
  validateDecision(&d, ..., hasPos, gateStates[norm])
  // applyHardRiskGates：缺 GateState 只拦限价
  case isLimitOpen && gs == nil:
      blockCode = "NO_GATE_STATE"
  // marketExceptionGate：市价默认模式提前放行
  if !rc.LimitEntryEnabled { return d, nil }
  // entryExecutionBlocked 也仅在存在 gs 时检查禁开码
  if gs := at.cycleGateStates[market.Normalize(symbol)]; gs != nil { ... }
  ~~~
- 为何现有防护不够: excluded_coins 只在候选生成时过滤；LLM 返回值没有再次对照候选白名单。EntryTimingGate 在 ctx.MarketDataMap[d.Symbol] 缺失时返回空 TF 并跳过。市价 Price=0 且无本轮行情/gs 时，accountRiskExposureBlocks 还会因无法定价返回“不阻断、预留风险 0”（auto_trader_risk.go:1959）。执行函数随后自行调用交易适配器获取真实报价，能够真正开仓；通用 RR、止损和 sizing 不会重建 POOR_HISTORY、NEG_EDGE、BTC 等缺失的方向授权。
- 建议修复: 开仓必须属于本轮经过完整信号评估的显式授权集合；管理动作另用持仓所有权集合。该约束应独立于市价/限价模式，执行前再次检查；无法定价的新增风险拒绝，而不是零额预留。
- 历史核对: 9415ea89 的 F03/F04 分支刻意保留了旧市价模式语义，但未给出允许模型越过静态币池或 excluded_coins 的产品授权；上述输入证明保留旧语义会实际越权开仓，而非单纯“未做提示词校验”。

### [HIGH][CONFIRMED] 市价降级限价发生在方向授权之后，能绕过本应禁止的限价入场
- 位置: kernel/signal_layer.go:1882、1933；kernel/engine_prompt.go:1858；trader/auto_trader_loop.go:344、374；trader/auto_trader_risk.go:2218、2338；trader/auto_trader_orders.go:143、154、167；trader/auto_trader_pending.go:195。
- 触发场景: 开启限价模式；某币 LongAllowed=false，唯一多头失败码为 EXTENDED_PUMP_UNCONFIRMED，但锚点仍为 99.5、LongLimitAllowed=true、LongMarketException=false。另有候选可交易，避免整轮 regime skip。模型忽略 wait 指令输出 open_long，SL/TP 与 sizing 本身合规。直接输出 open_long_limit 会被拒绝；输出 open_long 却会在后续变成同一限价单并提交。
- 证据:
  ~~~go
  g.LimitAllowed = anchor > 0
  if isLong && sig.PumpGuard != nil && sig.PumpGuard.Extended && !sig.PumpGuard.Confirmed {
      add("EXTENDED_PUMP_UNCONFIRMED")
  }
  // 市价没有绝对禁开码时先通过 applyHardRiskGates
  case isLimitOpen && !allowed: blockCode = "DIRECTION_DISALLOWED"
  // 后续 marketExceptionGate 只检查锚点可用，没有检查 LongAllowed
  if anchorLive && anchor > 0 {
      d.Action = limitAction
      d.Price = anchor
      d.MarketDegraded = true
      return d, nil
  }
  ~~~
- 为何现有防护不够: LongLimitAllowed 表示“价格锚存在”，不是方向权限。EXTENDED_PUMP_UNCONFIRMED 不属于 absoluteBanCode；executeOpenLimit 的 entryExecutionBlocked 只检查绝对禁开码，不重跑完整限价授权。MarketDegraded 只防止再次转回市价。后续 pending 复检即使撤单，也不能阻止新挂单在复检前成交。
- 建议修复: 先确定最终订单类型再统一授权，或在降级时要求对应 LongAllowed/ShortAllowed=true，并在 executeOpenLimit 入口检查本轮完整方向权限。
- 历史核对: 9415ea89 明确要求“限价无例外”；32668b8f/后续 market gate 的降级逻辑未满足该要求。本项不是否定已文档化的市价例外，而是证明无例外证据的请求可以绕到禁止的限价路径。

### [HIGH][CONFIRMED] 决策尾部截断时会回退全文匹配，把 reasoning 中弃用的 JSON 草案当成交易指令
- 位置: kernel/engine_analysis.go:35、545、549、555、569、577；kernel/engine_position.go:100；trader/auto_trader_orders.go:80。
- 触发场景: 已有归程序所有、超过平仓锁定时长的 BTC 多仓。LLM 的摘要包含一个明确弃用的 close 草案，最终输出 hold，但响应在 </decision> 前被截断：
  ~~~text
  <reasoning>弃用草案 [{"symbol":"BTCUSDT","action":"close_long"}]；最终继续持有</reasoning>
  <decision>[{"symbol":"BTCUSDT","action":"hold","reasoning":"趋势完好"}
  ~~~
  结果选中 reasoning 中完整的 close_long 数组，忽略尾部真正的 hold；该 close 能通过管理动作校验和持仓所有权检查。
- 证据:
  ~~~go
  reDecisionTag = regexp.MustCompile("(?s)<decision>(.*?)</decision>")
  if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
      jsonPart = strings.TrimSpace(match[1])
  } else {
      jsonPart = s
  }
  jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
  ~~~
- 为何现有防护不够: 完整闭合标签缺失后，fallback 没有剔除 reasoning，也没有优先从已经出现的 <decision> 开始解析；找到完整数组即返回成功，尚未进入 salvage。validateDecision 只能证明 close 是合法动作，不能辨认它来自弃用草案。使用源码原样正则进行无文件 Python 复核，上述输入确实选择 close_long。
- 建议修复: 只解析唯一的顶层决策载荷；出现 <decision> 后即固定解析边界，即使其结束标签被截断也不得回退 reasoning。纯 JSON 兼容路径应要求整个有效载荷是数组，不能任意全文搜数组。
- 历史核对: 32668b8f 有意恢复“截断前已完整输出的对象”，不应把该设计本身报成漏洞；实际漏洞是载荷边界丢失。另核实：完整对象的部分数组确实会执行，半个 JSON 对象不会被补造成订单；但截断载荷中若出现 blocking_factors:[]，reJSONArray 会先匹配内部 [] 并返回空决策，连前面的完整 close 也会丢失。该空数组路径同样已用源码正则复核。

### [HIGH][CONFIRMED] 连亏禁开期从最早一次亏损计时，第三次亏损后的 24 小时保护会提前解除
- 位置: trader/auto_trader_lossstreak.go:52、64、77、93、134；store/position_query.go:489；trader/auto_trader_lossstreak_test.go:63、155。
- 触发场景: 某币在第 1 天 00:00、12:00、23:00 连续亏损；23:00 第三次亏损才触发熔断，按文件与配置契约应禁到第 2 天 23:00。当前返回的到期时间却是第 2 天 00:00，只禁 1 小时。到 00:01，最早一单还被 24h 查询窗口移除，剩 2 单，prompt 和执行端都解禁。
- 证据:
  ~~~go
  // 数据按 exit_time DESC，即最新在前
  if streak >= maxLosses { break }
  bannedUntil := time.UnixMilli(positions[maxLosses-1].ExitTime).Add(lossStreakBanFor)
  // 两个调用方只读最近 24h
  sinceMs := time.Now().Add(-lossStreakWindow).UnixMilli()
  ~~~
- 为何现有防护不够: 恰好三单时 positions[0] 才是时间顺序上的第三次亏损，positions[2] 是第一次。prompt ban map 与执行 gate 共享同一错误函数；查询窗口过短也使“只改索引”不足以在第一次亏损滚出窗口后维持禁期。现有测试把 6h 前那笔称作第三次亏损并断言剩 18h，固定了倒序理解错误，而不是验证契约。
- 建议修复: 按时间顺序重放连续亏损并在阈值达到时记录触发时刻，持久化 banned_until；如坚持无状态重算，需要足够长的回看窗口、真实触发点和禁期期间后续成交的处理规则。
- 历史核对: 32668b8f 原始实现已存在同样的倒序索引，注释一直要求“第三次亏损触发后 24h”；未发现刻意缩短保护期的设计说明。

### [HIGH][CONFIRMED] 复盘硬规则用模型的可选 price 计算，市价单漏填或伪填即可绕过止损距离规则
- 位置: trader/auto_trader_rules.go:48；kernel/rule_engine.go:103、116、188、264；kernel/engine_prompt.go:201、235；trader/auto_trader_loop.go:335。
- 触发场景: 用户启用硬规则 {"field":"stop_loss_pct","op":">","value":4}，用于禁止止损超过 4% 的开仓。市价模式按提示词省略 price；真实入场 100、SL=95、TP=120，5% 止损本应被该规则阻止，其他通用门均可通过。此时规则读取到 stop_loss_pct=0，不触发。相同几何的限价单有 price，规则正常阻断。
- 证据:
  ~~~go
  violations, warnings := kernel.CheckDecisionAgainstRules(rules, d, equity, d.Price)
  // 默认数值为 0，仅 price>0 才计算
  if price > 0 {
      if d.StopLoss > 0 { c.stopLossPc = absPct(price, d.StopLoss) }
      ...
  }
  case "stop_loss_pct":
      return evalNumeric(cond.Op, c.stopLossPc, toFloat(cond.Value)), ...
  ~~~
- 为何现有防护不够: 注释声称未知 price 会跳过百分比条件，实际上没有 available/unknown 标志，所有条件直接对零求值；因而 > 阈值静默绕过，< 阈值（如 risk_reward<2）则可能无差别误拦市价单。规则系统 fail-closed 只覆盖加载/格式错误，不覆盖输入事实缺失。模型即使主动填 price，也未保证其等于真实入场价。执行端只重跑固定风控，不重跑复盘规则。
- 建议修复: 用程序定价的最终订单计划构造规则事实，市价使用真实执行报价，限价使用最终锚点；事实缺失必须显式处理。订单类型或价格被后续改写时重新检查依赖这些事实的规则。
- 历史核对: git log -p -S 'equity, d.Price' 显示 32668b8f 起一直如此；9415ea89 修复限价覆盖与规则库失败处理，但没有修复市价参考价来源。

### [HIGH][CONFIRMED] 收盘判定使用处理时钟，旧快照中的 forming candle 会在等待后被冒充为最终收盘
- 位置: kernel/signal_layer.go:2393、2397、3067；kernel/market_regime.go:217；kernel/engine_prompt.go:1627；kernel/engine_analysis.go:347、380、235；trader/auto_trader_risk.go:2465。
- 触发场景: 14:59:30 拉取 14:45–15:00 的 15m forming bar，保留当时的 OHLCV。多币串行采集、提示词构建或 LLM 等待跨过 15:00 后，程序用 15:01 的 time.Now() 重算该旧快照，认为 14:45 的 bar 已闭合，并将 14:59:30 的中间 close/volume 当成最终值。最后 30 秒真正发生的反转不在快照中，却可以影响 MICRO_TREND、结构位与方向许可。
- 证据:
  ~~~go
  // 唯一闭合条件是 openTime + duration 相对“现在”
  if time.UnixMilli(last.Time).Add(dur).After(now) {
      klines = klines[:len(klines)-1]
  }
  // LLM 返回后，用原 ctx 快照及新的时钟做时点门
  kernel.ExecutionMicroTrend(ctx.MarketDataMap[d.Symbol], time.Now())
  ~~~
- 为何现有防护不够: market.KlineBar 与 TimeframeSeriesData 不保留 final 标志/采样时刻（market/types.go:47、57）；“现在已过收盘时刻”不能证明旧数据当时已经最终收盘。DataQuality 检查 bar 数，VendorDivergence 检查价格差，都不核验 bar 是否已完成采样。市价执行虽然重新获取报价和 ATR，却不以新完整数据重新计算整个 timing 授权；错开的时点门已经放行。closedKlinesAt 的历史 cutoff 避免了用尚未到时的 4h bar 做前一小时 regime 确认，但仍无法识别这种旧的未完成快照。
- 建议修复: 拉取时按每个 timeframe 的采样时刻固定 closed 数据集合，或传递交易所 final/closeTime 及观测时刻；后续重算不能令快照中的 forming bar 自动转正。跨收盘边界执行前获取已完成的新 bar 再重评。
- 历史核对: git log -p -S 'time.UnixMilli(last.Time).Add(dur).After(now)' 显示 32668b8f 的初衷就是仅用闭合 K 线；后续去掉“无条件丢尾 bar”的修复保护了滞后行情，但没有处理采样时刻与处理时刻的区别。本项是已完成状态错误，不是声称代码读取了未来真实行情。

### [HIGH][CONFIRMED] 波动率减仓后仍用旧数量执行 1R 减半，可把本应保留的 runner 全部平掉
- 位置: trader/auto_trader_vol.go:249、338、350、376、492、645；trader/binance/futures_orders.go:148、160。
- 触发场景: AI 仓位快照数量 100，当前波动率目标为 50（风险配置降低或波动率上升），同时首次到达 profit lock。当前 SL 已由 early breakeven arm 移到保本，r1TrimDone=false。vol-target 先卖 50；紧接着 1R 分支仍从原 pos 读取 qty=100，再卖 50，实际全平。正确的“当前剩余仓减半”应第二次只卖 25，留下 25。
- 证据:
  ~~~go
  reduceQty := qty - target/markPrice
  if _, err := at.reducePosition(symbol, side, reduceQty); err == nil {
      at.markVolResize(posKey)
  }
  // pos 未刷新、未改写数量，继续使用
  if at.processRProfitRules(pos, markPrice) { continue }
  ...
  qty, _ := pos["positionAmt"].(float64)
  ...
  at.reducePosition(symbol, side, qty*0.5)
  ~~~
- 为何现有防护不够: volResizeAllowed 只限制波动率调仓频率，r1TrimDone 只防止重复执行 1R，两者不能协调同轮的两个合法减仓。reducePosition 直接将显式 qty 交给适配器，不重读或缩放；Binance CloseLong 对显式非零 quantity 原样下单。两次各 50 都不超过各自执行时的剩余数量，因此不会被交易所拒绝。此场景无需假设止损挂单验证异常：SL 已保本时 1R 直接允许 trim。
- 建议修复: 每次成交后更新该周期和 realtime 缓存中的剩余数量，以实际成交量为准；或成功 vol resize 后结束该仓本轮其余数量动作，下一次用新快照执行。统一生成每仓单次减仓计划可避免规则叠加。
- 历史核对: 7c11a0d9 将利润规则抽为 processRProfitRules，但保留了先 vol resize 再传旧 pos 的顺序；未发现这是刻意清仓的策略要求。

### [HIGH][CONFIRMED] 对冲持仓的退出信号按同币第一条持仓定方向，会给另一侧提供相反的平仓证据
- 位置: kernel/engine_prompt.go:749、1036、1795；kernel/signal_layer.go:1253、1259；trader/auto_trader_orders.go:1023。
- 触发场景: Context.Positions 先列 BTC 手工多仓，再列同币 AI 空仓（两边均为 AI 也一样）。AI 空仓已持有超过 4h，主 TF 下跌且收阴，RSI=40，空头结构未反转。渲染 AI 空仓时，computeCoinSignal 仍选中了第一条多仓方向，使 exit_rule_triggered=true；按空头语义应为 false。模型据此提前平掉正确运行的空头。
- 证据:
  ~~~go
  // formatPositionInfo 已知当前 pos，却没有把 pos.Side 传给信号函数
  e.computeCoinSignal(marketData, quantData, ctx, nil)
  // 函数仅按 symbol 找第一条，并立即 break
  for _, p := range ctx.Positions {
      if market.Normalize(p.Symbol) == market.Normalize(data.Symbol) {
          ... opt.PositionSide = "long" / "short"
          break
      }
  }
  // longExit 的 down+bearish 条件对正在盈利的 short 恰好是正向证据
  (primary.Trend == "down" && primary.LastClosedCandle == "bearish")
  ~~~
- 为何现有防护不够: 渲染手工仓时的 hold-only 分支不改变 ctx，后面 AI 仓仍会误选它的 side。close_short 只检查是否为程序所有、平仓时长等，不会确认模型引用的 ExitTriggered 是否属于该侧。10-09 的方向感知修复只覆盖同币单侧持仓；hedge mode 的两个仓位不是同一分析对象。
- 建议修复: formatPositionInfo 显式传入当前 positionSide；候选的 side 留空。持仓方向相关状态以 (symbol, side) 为键，避免从 symbol 猜方向。
- 历史核对: 1764e2bb 明确记录 ONUSDT 因“空头收到多头退出语义”而误平的实盘事故，并引入 PositionSide；当前 first-match 接线在 hedge 场景仍复现同一语义错误，不能视为有意 trade-off。

### [HIGH][CONFIRMED] 开启“日线上涨禁空”不会自动拉取日线，常见 timeframe 配置让硬门静默失效
- 位置: store/strategy.go:744；kernel/engine_analysis.go:307、326、379；kernel/market_regime.go:28、51；kernel/signal_layer.go:2992；trader/auto_trader_risk.go:2520；trader/auto_trader_pending_risk.go:166、267。
- 触发场景: 普通加密币策略选择 15m/1h/4h，block_short_1d_uptrend=true；日线实际处于上涨趋势，15m 有回调，其他短单条件合规。行情采集不会因该开关补 1d，TimeframeTrend(ctxData,"1d") 返回空字符串，市价和限价空单都不会被日线禁空门阻断。
- 证据:
  ~~~go
  // 通用必需周期只追加 1h 与 4h
  for _, tf := range append(append([]string{}, timeframes...), "1h", "4h") { ... }
  // 仅 bstock 额外追加 1d，普通币直接返回
  if !market.IsBStockSymbol(symbol) { return out }
  // 执行端只匹配确知的 up，缺失时放过
  if rc.BlockShort1dUptrend {
      trend := kernel.TimeframeTrend(ctx.MarketDataMap[d.Symbol], "1d")
      if trend == "up" { ... continue }
  }
  ~~~
- 为何现有防护不够: ComputeSymbolSignals 只计算已抓取的 timeframe；没有日线则 TimeframeTrend 返回 ""。DataQuality 也只要求已配置及通用必需周期，不知道该风控开关。pending 复检虽有同样的 1d 条件，但 recheckConfiguredTimeframes 同样没有按该开关补日线，因此不会补救。
- 建议修复: 在统一的“策略实际必需周期”解析中加入风险开关依赖：开启 block_short_1d_uptrend 就抓取 1d，并把足够闭合日线列为此门的必要输入；不可获取时明确拒绝新增空头风险或按显式配置处理。
- 历史核对: 32668b8f 将该开关声明为 CODE ENFORCED；e1a28e80 的必需周期扩展只服务 regime，未覆盖这项已有风控依赖。

## 指定四项失败测试的结论

本次按用户指定的 GOCACHE 运行四项测试，全部复现失败：

~~~text
GOCACHE=$PWD/.gocache go test ./kernel -run 'Test(AllowedCandidateKeepsFullJSON|PromptBudgetFallbackRemovesRawKlines|RegimeSkipKeepsCallForAllowedDirection|DirectionHintRendering)$' -count=1
~~~

**这四个失败当前不证明“合法候选丢 JSON / OHLCV stripping 损坏 / regime skip 错跳过合法方向 / scanner hint 渲染消失”的生产回归。当前直接原因是 fixture 不再满足硬门。**

- 公共 buildTF（kernel/signal_layer_test.go:25–46）生成 80 根单调递增的 close：1.000→1.079，却把 CurrentPrice 固定为 1；RSI 从真实 close 重算为 100，手填 RSI14Values=55 并不控制信号层。
- 本次实际输出的多头阻断为 COUNTER_EMA20_1H_6.4_GT_3 + RSI1H_EXHAUSTED_100；空头为 CHASE_EMA20_1H_6.4_GT_6 + CONSENSUS_OPPOSED_50。长短双向都被禁止后，kernel/engine_prompt.go:817 合法压缩成表；scanner_hint 在 :825 才渲染，故一并省略；engine_analysis.go:176 的 skip 也按 HardBlocked 合法执行。
- TestPromptBudgetFallbackRemovesRawKlines 在检查“原始 full prompt 是否含 ohlcv”时就失败（prompt_review_fixes_test.go:43），根本未执行 stripping；独立的 TestStripRawKlinesPreservesSurroundingJSON 本次通过。
- 594c1616（10-11）明确新增默认 6%/3% EMA 距离、RSI 78、4h 6% 门。1764e2bb（10-09）历史记录还指出其中三项曾受实时 BTC 取数影响而 flaky；当前 fixture 又确定性触发新的 anti-chase 门。应固定市场依赖与适用阈值，使用真实能通过方向门的 fixture 后再验证原契约，不能通过移除生产硬门来“修绿测试”。
- 另运行现有 kernel 的解析、校验、校价、anti-chase、market regime、regime line、入场模式、规则条件及 OHLCV stripping 相关测试通过；trader 的 market-exception、vendor/anti-chase gate、stop tighten、同周期 breakeven/trailing、实时 hands-off、止损确认后 trim 测试通过；连亏、vol target 与 trailing 的现有算术测试也通过。测试通过不覆盖上述跨层输入组合；尤其连亏测试当前断言了错误到期点。未新增测试或源文件。

## 已核实的边界

- 错侧 SL/TP、SL 在 entry 错侧、净 RR、ATR 止损带、强平距离及最终最小名义价值，在市价与限价执行函数中有实际二次校验，不能仅因 kernel 的检查较弱就断言可绕过。主检查见 trader/auto_trader_risk.go:632–759，两个路径都调用它。
- 同侧“加仓”当前会被市价/限价入口的“已有同方向持仓”检查拒绝（auto_trader_orders.go:205；auto_trader_pending.go:223）；vol target 只减仓，不会自动补仓。提示词/日志所说“AI 可 add-back”与当前执行能力不同，本次没有据此臆报可绕过 sizing 的加仓路径。
- 内核未做统一决策去重；规范 symbol 的同侧重复开仓在成交后有持仓检查，限价重复项走替换逻辑，因此没有仅凭重复数组就认定必然双倍开仓。另有一个可验证的批次耦合点：resolveTakeProfitOptions 会先修改共享 GateState 再验证各决策（engine_position.go:557；engine_analysis.go:462），重复/被拒条目可影响同币另一条的 TP；建议修复计划对象时一并消除共享变更。
- 独立手工仓的 close/partial/adjust 执行入口有所有权检查；不能把 kernel 未核所有权直接报为“可以平手工仓”。第 1 项是已穿透这些检查的具体撤单路径。
- auto_trader_shadow.go 只记账和评估 counterfactual，没有发现其结果反向改变当前实盘决策；本次未把纯 shadow 统计偏差作为 live finding。
- 已检查 % / fraction / bps / USDT notional / coin quantity / R 的主要转换；未发现足以独立确认的新 100 倍单位错误。第 8 项是数量快照过期，不是 USDT 与 coin 混用。

## 优化建议

1. **预先生成不可变的最终可执行计划，再交给模型选编号。** 目前 kernel 按 stop/RR 生成菜单（signal_layer.go:1894–1900），但 TPMaxR 到执行校验才截断（auto_trader_risk.go:698–716）。把成本、TP cap、lot filter 和真实最小可交易量纳入同一个计划，并在 kernel 与执行端共享，可减少“LLM 已选、最后又被机械拒绝”的轮次，缩短有效机会从识别到提交的时间；同时避免批次内共享 GateState 被后一个选择改写。
2. **连亏查询按周期批量读取，成交事件更新。** 当前 lossStreakBlocks 每个开仓重复查询该 trader 全部 24h closed rows（auto_trader_lossstreak.go:134–145），prompt 又单独读一次。修正第 5 项的时序后，以同一批数据构造每币禁期，提交前仅检查新增成交变化，可降低 DB 开销和 LLM 返回后的执行延迟，并使 prompt 与实际 gate 的截止时刻一致。
3. **把行情采集与外部信号富化移出逐币渲染，固定本轮共同观测版本。** fetchMarketDataWithStrategy 串行逐仓、逐候选采集（engine_analysis.go:347、374）；computeCoinSignal 又在渲染时读取 BTC、盘口与多空指标（engine_prompt.go:1672、1723、1739）。采用有并发上限的采集阶段及统一的 per-cycle 快照，配合第 7 项的收盘状态修复，可降低多币时间偏移、减少信号失效后才提交的订单。收益应以快照年龄分布和 signal→fill 的 RR 衰减量化，不能只以 token 数评估。

## 验证与限制

代码与历史核对为只读；未新建复现测试文件，动态补充检查仅在内存中使用源码正则确认两种截断输入的匹配结果。没有访问实盘数据库确认当前 trader 的每个开关或是否已发生事故；配置相关项已明确触发前提。Go 命令均使用 GOCACHE=$PWD/.gocache，审计结束删除该缓存目录，除此之外仅创建本报告。

