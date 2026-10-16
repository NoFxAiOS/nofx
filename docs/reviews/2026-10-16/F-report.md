# Area F 前端（web/src）审查报告

基线 commit 744da725，只读审查。覆盖：TraderDashboardPage、RiskStatusPanel、PositionHistory、DecisionCard、RiskControlEditor、StockConfigEditor、StrategyStudioPage 保存链路、TraderConfigModal / AITradersPage、httpClient / api 客户端、AuthContext。对照了 Go 侧 `store/strategy_validation.go`、`store/strategy_stock.go`、`store/position_query.go`、`api/strategy.go`（handleUpdateStrategy）、`api/handler_trader_status.go`（handleClosePosition）。

总体结论：平仓按钮（symbol/side 正确、有确认、幂等）、策略保存（15s 超时、乐观锁、服务端 Validate）、陈旧数据标记（STALE）、风险字段默认值提示（与 Go 的 Effective* 一致）都做得比较扎实，没有发现会下错单或静默改 live 模式的路径。发现的问题集中在「展示口径」和「保存时 undefined 被后端合并吞掉」两类。

---

### [HIGH][CONFIRMED] 历史页「净盈亏」卡片把手续费重复扣了一次
- 位置: web/src/components/trader/PositionHistory.tsx:604-616（卡片），:503-510（总盈亏卡片）；store/position_query.go:119-127、:15-23
- 触发场景: 任意有成交历史的交易员，打开「历史仓位」页。后端 `GetFullStats` 自 09-19 审计起已按单笔净额汇总：`net := pos.RealizedPnL - pos.Fee; stats.TotalPnL += net`，所以 `stats.total_pnl` 本身就是扣费后的净值（i18n 也写着「总盈亏(净)」）。前端「净盈亏（扣除手续费后）」卡片又算了一遍 `total_pnl - total_fee`，等于手续费被扣两次。例如净 total_pnl=+30、total_fee=12，页面上「总盈亏(净) +30」旁边的「净盈亏(扣除手续费后)」显示 +18，实际账户只赚了 +30。
- 证据:
```
// PositionHistory.tsx:606-609
((stats.total_pnl || 0) - (stats.total_fee || 0) >= 0 ? '+' : '') +
formatNumber((stats.total_pnl || 0) - (stats.total_fee || 0))
// store/position_query.go:119-123
net := pos.RealizedPnL - pos.Fee
stats.TotalPnL += net
stats.TotalFee += pos.Fee
```
- 为何现有防护不够: 该卡片写于 total_pnl 还是毛额的年代（git log 显示 PositionHistory 里这段随 8e294a5e 重构迁入），09-19 后端改口径时前端没跟。另外 `TotalWin/TotalLoss` 的 Go 注释还写着 "gross, pre-fee"，实际是净额，注释也过期了，容易让后人再次改错。盈利被低估、亏损被放大，用户据此判断策略是否值得继续会被误导。
- 建议修复: 删除「净盈亏」卡片（或直接显示 `stats.total_pnl`）；`total_fee` 只作为副标题。修正 Go 里 TotalWin/TotalLoss 的注释。

### [MEDIUM][CONFIRMED] 历史表底部「总盈亏」、按盈亏排序用的是毛盈亏，与行内净盈亏口径不一致
- 位置: web/src/components/trader/PositionHistory.tsx:455-458、:791、:384-386，对比 :216-219
- 触发场景: 每行显示净盈亏（`realized_pnl - fee`），但分页脚部「总盈亏」`totalFilteredPnl` 对 `realized_pnl` 求和（毛额），按「盈亏」排序也比较毛额。小额仓位上毛盈利/净亏损（手续费大于利润）会被排在盈利一侧，脚部合计比逐行相加大出手续费总额。
- 证据: `(sum, p) => sum + (p.realized_pnl || 0)`；`comparison = (a.realized_pnl || 0) - (b.realized_pnl || 0)`。
- 为何现有防护不够: 行内注释明确声明「Rows show NET PnL … same caliber as the stats cards」，脚部与排序漏改。
- 建议修复: 抽一个 `netPnl(p) = (p.realized_pnl||0) - (p.fee||0)`，行、合计、排序统一使用。

### [MEDIUM][CONFIRMED] 历史表「按盈亏%排序」忽略方向，空单排序反了
- 位置: web/src/components/trader/PositionHistory.tsx:387-392，对比行内显示 :237-244
- 触发场景: 选择按 pnl_pct 排序。排序公式固定为 `(exit - entry)/entry`（多头口径），行内显示却按方向取反。盈利的空单在排序里被当成亏损，排序结果与屏幕上显示的百分比不单调。
- 证据: `const aPct = (((a.exit_price || 0) - aPrice) / aPrice) * 100`，无 side 判断；行内 `isLong ? … : (entryPrice - exitPrice)/entryPrice`。
- 建议修复: 排序复用同一个按 side 计算的函数。

### [MEDIUM][CONFIRMED] 决策卡片把限价开仓 / 调止损 / 部分平仓一律标成 WAIT
- 位置: web/src/components/trader/DecisionCard.tsx:30-37、:99-101；Go 合法动作集 kernel/engine_position.go:100-112
- 触发场景: 后端合法动作包含 `open_long_limit`、`open_short_limit`、`adjust_stop_loss`、`partial_close_long`、`partial_close_short`（程序现在大量使用限价挂单入场）。前端 `ACTION_CONFIG` 只有 6 个键，其余走 `|| ACTION_CONFIG.wait`，徽章显示灰色「WAIT」+「OK」。限价开仓还会因 `includes('open')` 展示 entry/SL/TP 网格，于是出现一张标着 WAIT、却带完整入场价与止损止盈的卡片；部分平仓看不到 close_fraction，调止损看不到新止损价。全仓库 grep 这些动作名在 web/src 中零命中（翻译除外）。
- 为何现有防护不够: 没有 default 分支提示未知动作；`success` 徽章为 OK 反而强化「正常等待」的印象。用户靠这个面板判断机器人是否已挂单，可能因此手动重复下单，或没发现已经减仓。
- 建议修复: 补全映射（LIMIT LONG / LIMIT SHORT / ADJUST SL / TRIM LONG / TRIM SHORT）；未知动作直接显示原始动作名而不是 WAIT；partial_close 显示 close_fraction，adjust_stop_loss 显示新 stop_loss。

### [MEDIUM][CONFIRMED] us_stock 配置编辑器把「清空」写成 undefined，后端按「缺省保留旧值」合并，清空永远保存不上
- 位置: web/src/components/strategy/StockConfigEditor.tsx:299（scan_exclude）、:459-462（scan_top_n）、:507-511（scan_exclude 输入）、:550-555（scan_rank 切回 kell）；api/strategy.go:309-322（handleUpdateStrategy 合并）
- 触发场景: 后端 PUT 逻辑是「以现有配置为底，再 `json.Unmarshal(req.Config, &mergedConfig)` 覆盖」，`stock_config` 是 `*StockConfig`（store/strategy.go:145），Unmarshal 会就地合并，缺失的键保留旧值。前端以下操作把字段设为 `undefined`，JSON.stringify 直接丢弃该键：
  1. 删掉最后一个「扫描排除」标的 -> `scan_exclude` 缺省 -> 旧排除列表仍在后端生效；
  2. 把「候选排序」从 momentum 切回 kell -> `scan_rank` 缺省 -> 后端仍是 momentum；
  3. 清空 `scan_top_n` -> 仍保留旧的 TopN。
  这些直接决定喂给 LLM 的候选股票池（us-stock 是 live），而用户以为已改。同文件 :88-91 的注释自己也承认「backend merges PUT bodies… an omitted field would not override」，defaultStockConfig 因此用显式值，但这几处漏了。
- 为何现有防护不够: 保存成功后编辑器会用服务端返回值覆盖（fetchStrategies({strategyId}) 且无未保存改动时），字段会「弹回」旧值，用户有可能察觉，但没有任何提示，也不报错；若用户不盯着看就以为已生效。
- 建议修复: 清空时发送显式零值：`scan_exclude: []`、`scan_top_n: 0`、`scan_rank: ''`（Go 接受空串，见 strategy_stock.go Validate 的 `case ""`）。同样检查 CoinSourceEditor.tsx:76 的 `use_static: undefined`（该处对应字段是否有 omitempty 需要确认，PLAUSIBLE）。

### [MEDIUM][CONFIRMED] 风险状态面板在请求失败后无限期展示旧数据且没有任何陈旧标记
- 位置: web/src/components/trader/RiskStatusPanel.tsx:92-107
- 触发场景: 面板用 SWR 30s 轮询。`if (error && !data)` 才显示「不可用」；一旦拿到过一次数据，后续全部请求失败（后端重启、token 过期被静默、网络断）时 SWR 保留旧 `data`，面板继续渲染旧的「日亏熔断：OK / 回撤熔断：OK / 无亏损冷却」。而主面板的账户/持仓已经有 STALE 徽章（F17），只有风险面板没有。若此时恰好已触发 `daily_halted`，用户仍看到 OK，可能手动开仓以为系统没停。
- 证据: `const { data, error } = useSWR(...)`；仅 `error && !data` 分支处理失败；`data` 存在时 `error` 被忽略。
- 建议修复: `error && data` 时在标题行显示「数据已过期（上次成功 hh:mm）」并把整体 tone 置为 warn；可用 SWR 的 `isValidating`/记录最近成功时间。

### [MEDIUM][PLAUSIBLE] 平仓失败的真实原因被吞掉，只显示「Failed to close position」
- 位置: web/src/lib/api/traders.ts:86-96，web/src/pages/TraderDashboardPage.tsx:262-270；api/handler_trader_status.go（SafeInternalError）
- 触发场景: 用户点平仓，交易所拒单（保证金模式、reduce-only 冲突、最小名义价值等）或请求 30s 超时。`closePosition` 对 `!result.success` 一律 `throw new Error('Failed to close position')`，丢弃 `result.message`。超时场景下订单可能已成交，但界面只提示失败且不会 `mutate` 刷新持仓（刷新只在成功分支），用户看到旧持仓还在，会再点一次或去交易所手工处理。后端平仓是幂等的（无仓位返回 NO_POSITION），所以不会双平，但用户信息不足。
- 为何现有防护不够: 只有 success 分支刷新数据。
- 建议修复: 抛出 `ApiError(result.message, …)`；`finally` 里总是 `mutate(positions-…)`，让失败后也能看到真实仓位。需确认 `SafeInternalError` 是否会把原因透传（决定能否显示具体原因）。

### [LOW][CONFIRMED] 「停止交易员」无二次确认
- 位置: web/src/components/trader/AITradersPage.tsx:520-540；对比同文件 handleDeleteTrader 有 confirmToast
- 触发场景: 实盘有持仓时误点停止按钮，立刻 `SignalStop()` + 后台 Stop()，保护看门狗/对账随之停止；界面不提示是否有持仓/挂单。
- 建议修复: running 时先用 confirmToast，且在有持仓时在文案里列出持仓数量。

### [LOW][CONFIRMED] 策略保存失败时错误信息被截断到 120 字符
- 位置: web/src/pages/StrategyStudioPage.tsx:526-541
- 触发场景: 服务端 `validateBoundsAndLadder` 的错误文本较长（如 tp_full_at_r 与 tp_trim_at_r 关系、breakeven_arm_r 与 profit_lock_at_r 关系），`body.slice(0, 120)` 直接切掉原因，且显示的是原始 JSON 字符串 `{"error":"…`。服务端校验本身是有效的，不会存下非法配置，只是用户难以得知改哪里。
- 建议修复: 解析 JSON 取 `error` 字段完整显示。

### [LOW][CONFIRMED] 交易员编辑弹窗的「全仓/逐仓」开关对 Binance 无效，但界面默认且可选「全仓」
- 位置: web/src/components/trader/TraderConfigModal.tsx:94,150,502-517；trader/auto_trader.go:238-241
- 触发场景: 后端 `entryUsesCrossMargin()` 对 binance 恒为 false（强制逐仓），弹窗默认 `is_cross_margin: true` 并允许切换，界面上看到「全仓」会误以为账户在全仓运行。无资金风险（后端强制逐仓），但会误导风险判断。
- 建议修复: 选中 Binance 交易所时禁用该开关并提示「Binance 强制逐仓」。

---

## 已核对且无问题的点（供参考）
- 平仓按钮: 发送 `pos.symbol` + `pos.side.toUpperCase()`，后端用 `strings.ToLower(req.Side)` 匹配，symbol/side 正确；有 confirmToast，含 symbol 与方向；对冲模式下同 symbol 多空按钮会同时置灰，属可接受行为；后端全平会撤该方向的保护单（futures_orders.go:173-178）。列表包含用户手动仓位，点击平仓是用户主动行为，不算越界。
- 持仓盈亏: 后端 `unrealized_pnl_pct` 为保证金口径 ROI（含杠杆），前端直接显示，未二次换算；总权益/总盈亏/保证金占用口径与 Go account 返回一致。
- Go 校验与前端范围: RiskControlEditor 的默认值提示（early_close 4h、pump 20、chase 6/3/78/6、max_account_risk 10、net_directional 6、vendor_divergence 1、risk_per_trade 1.5、tp_max_r<1 视为无上限）与 `Effective*` 一致；max_margin_usage 的 0-1 换算正确；us_stock 默认值（divergence 1、position 20、exposure 80、positions 5、risk 1.0、ATR 带）与 Go 常量一致。实盘开关 `paper_trading` 取消勾选时有红色警告，且必须点保存才生效，不是静默开启。
- 保存链路: 乐观锁（base_updated_at/409）、15s 超时、dirty 草稿保护均正常；服务端 `Validate()` 在持久化前运行。
- 认证: token 存 localStorage（常见做法，XSS 面取决于 CSP，未发现 innerHTML 注入点，仅 TradingViewChart 写入固定脚本）；401 统一清理并跳转；未发现 token/API key 被 console.log；交易所/模型接口返回的是 Safe* 结构，不回传密钥；更新交易所时空密钥字段后端按「空则不改」处理（store/exchange.go:254-272），不会误清空。
- 陈旧数据: 账户/持仓轮询失败 3 次后标 STALE 并保留 60s 心跳，onSuccess 清除，逻辑无死锁。

---

## 优化建议
1. 在决策卡片和历史页引入统一的 `actionMeta` / `netPnl` 工具函数并加 vitest 用例（对照 Go 的合法动作集生成映射表）。预期: 杜绝前后端口径漂移（本次 3 个 MEDIUM/HIGH 都属此类）。
2. 仪表盘 KPI 区增加「今日已实现盈亏（净）」和「当前总 open risk（R）」两个数，数据后端已有（daily_pnl、risk-status.positions[].current_r）。预期: 手动决策时一眼看到真实风险敞口，而不是只看未实现盈亏。
3. 持仓表补一列「到止损距离（R / %）」: 已有 `protection.sl_price` 和 `mark_price`，前端即可算；无止损（`sl==null` 且无 protection_error）的行整行高亮红色。预期: 更早发现裸仓。
4. 平仓前确认弹窗显示预计名义价值与当前 uPnL（数据已在行内）。预期: 降低手滑平错仓位的概率。
5. 轮询失败时把「上次成功时间」写进 STALE 徽章（现在只写 STALE），并对 RiskStatusPanel 复用同一机制。预期: 缩短发现后端异常的时间。
