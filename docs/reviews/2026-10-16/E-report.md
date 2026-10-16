# Area E 评审报告：store / API / 安全 / 生命周期（commit 744da725，只读）

总体结论：认证、用户隔离、加密存储、Telegram 绑定与确认机制整体做得扎实（IDOR 均有 `GetForUser` 兜底，已移除解密 oracle，SQL 全部参数化，Telegram 变更类请求必须 /confirm）。没有发现"未授权即可下单/平仓"的路径，也没有发现可直接泄露交易所密钥的路径。下面是在代码里验证到的、仍然有实际影响的问题，按严重度排序。

---

### [MEDIUM][CONFIRMED] Binance `CloseLong/CloseShort(symbol, 0)` 的"全平后撤本侧保护单"是死代码，全平路径永远不撤 SL/TP
- 位置: `trader/binance/futures_orders.go:128-179`（CloseLong）、`:188-232`（CloseShort）；调用方 `trader/auto_trader_orders.go:1000,1076`、`trader/auto_trader_risk.go:443,449`、`api/handler_trader_status.go:248,250`
- 触发场景: 任何传 `quantity==0`（"全平"）的调用。函数开头 `if quantity == 0 { ... quantity = pos["positionAmt"].(float64) }` 已把 quantity 改成持仓量，后面 `if quantity == 0 { CancelProtectiveOrdersForSide(...) }` 永远为假。
- 证据:
```go
if quantity == 0 { positions...; quantity = pos["positionAmt"].(float64) ... }
...
// Full closes cancel the remaining SL/TP orders ...
if quantity == 0 {   // 此时 quantity 必然 != 0
    t.CancelProtectiveOrdersForSide(symbol, ...)
```
- 为何现有防护不够: 主交易循环依赖 `auto_trader_risk.go:1500-1580` 的 orphan sweep（每轮清理没有对应持仓的 STOP/TP 单）和开仓时 `OpenLong/OpenShort` 先撤本侧保护单兜底，所以 AI 平仓路径只是出现"孤儿保护单存活到下一轮 sweep"的窗口。但 `POST /api/traders/:id/close-position`（`handler_trader_status.go:248`）走的是临时 probe 客户端，如果该 trader 没在运行（没有 sweep），孤儿 SL/TP 会一直留在交易所；之后用户在同币种同方向手动开仓，这些旧价位的 reduce-only 条件单会直接作用在用户的手动仓位上（对冲模式下 SELL+LONG 条件单只能减仓，但会按旧价格把用户新仓位平掉）。
- 建议修复: 在重写 quantity 之前记录 `fullClose := quantity == 0`，用它判断是否撤保护单；并加一个单测（全平后 `CancelProtectiveOrdersForSide` 被调用）。

### [MEDIUM][CONFIRMED] Telegram bot 的内部 JWT 永不刷新，到 `JWT_TTL`（默认 7 天）后 Telegram 控制通道静默失效
- 位置: `telegram/bot.go:67-89`（`resolveBotUser`），`auth/auth.go:104-124`（jwtTTL）
- 触发场景: 进程连续运行超过 7 天（或 JWT_TTL_HOURS）且用户没有改密码。
- 证据: `if u.ID == botUserID && u.TokenVersion == botTokenVersion { return true }`——只在用户 ID / TokenVersion 变化时才重新签发 token；`agent.NewManager(..., botToken, ...)` 里的 token 到期后所有 `apiCallTool.executeApproved` 请求都返回 401 "Invalid or expired token"，LLM 只会回复"API error 401"。
- 为何现有防护不够: 没有任何到期检查；`agent.GenerateBotToken` 注释写的是 long-lived 但实际没有被使用。这是用户远程"停机/平仓"的应急通道，失效时间正好不可预测。
- 建议修复: `resolveBotUser` 中解析 `botToken` 的 `exp`，剩余 < 1 天就重签；或给 bot 用单独的长 TTL token（仍受 TokenVersion 吊销）。

### [MEDIUM][CONFIRMED] Telegram 的 /confirm 与普通消息共用同一个"车道"，LLM 慢调用会让紧急确认排队最长 60 秒后直接失败
- 位置: `telegram/agent/manager.go:44-55`，`telegram/agent/agent.go:152-154`
- 触发场景: 用户发"停止所有交易"，LLM（GLM 常见数十秒到分钟级）返回 Confirmation required；用户立刻回 `/confirm xxxx`，但如果此前另一条消息的 agent 循环（最多 10 轮 LLM 调用）还在跑，`/confirm` 要等车道，60 秒后返回"上一条消息仍在处理中"，确认码 5 分钟过期。
- 证据: `Manager.Run` 对所有消息（含 `/confirm `）统一 `lane <- struct{}{}`，仅在拿到车道后 `a.Run` 才检查 `/confirm ` 前缀。
- 为何现有防护不够: 紧急停机依赖 LLM 先"理解"再生成请求，没有任何不经 LLM 的确定性命令（如 `/stop`、`/flat`、`/positions`）。
- 建议修复: 在 `Manager.Run` 里先判断 `/confirm`，绕过车道直接调用 `apiTool.confirm`（加互斥保护 pending）；并加几条确定性紧急命令。

### [MEDIUM][CONFIRMED] API 层与 Telegram 的裸 goroutine 没有 recover，任何一次 panic 会带走整个实盘进程
- 位置: `api/handler_trader.go:757,795,923`（update/delete/stop 后台链）、`api/handler_exchange.go`（reload goroutine）、`api/handler_ai_model.go`、`api/strategy.go`（reload goroutine）、`api/handler_trader_status.go:41`（sync-balance reload）、`api/exchange_account_state.go`（每个 exchange 一个 goroutine，`GET /exchanges/account-state` 触发）、`telegram/bot.go` 的 agent goroutine（`agents.Run`）
- 触发场景: 这些 goroutine 调用 `NewAutoTrader`/各交易所构造函数/`Stop()`/`probeExchangeAccountState`/LLM 响应处理。任一处的 nil 解引用或类型断言失败（例如 `futures_orders.go:138` 的 `pos["positionAmt"].(float64)` 无 ok 检查）在非 gin 的 goroutine 里会直接终止进程；gin 的 Recovery 只保护 handler 自身 goroutine。
- 证据: `grep recover()`：trader 包内主循环/监控/流都有 recover（`auto_trader.go:581`、`auto_trader_risk.go:103,148`、`auto_trader_stream.go:223`），但上述 API/telegram goroutine 数量与 recover 数量为 0。
- 为何现有防护不够: 交易所侧止损还在，但进程崩溃到 launchd 拉起之间 watchdog、挂单对账、daily-loss 都停摆；而这几个入口恰恰是用户保存配置/打开设置页就会触发的。
- 建议修复: 加一个统一的 `safeGo(name, fn)`（defer recover + 记录 + 发 ALERT），替换这些 `go func()`。

### [MEDIUM][CONFIRMED] `POST /traders/:id/close-position` 绕过交易器的执行锁与状态记账，直接用新建的 probe 客户端全平该方向全部持仓
- 位置: `api/handler_trader_status.go:165-262`
- 触发场景: 前端"一键平仓"按钮（或 Telegram 经 /confirm 调用）在 trader 运行中点击。
- 证据: 用 `buildExchangeProbeTrader` 新建 `FuturesTrader`，调用 `CloseLong(req.Symbol, 0)`（全平该 symbol 该方向，含用户手动仓位——此账户本来就混合使用）；没有取 `accountExecution.mu`，没有 `markCloseIntent`/`unmarkAIManaged`/清理 `positionStopLoss` 等内存状态（`auto_trader_orders.go:1000-1020` 的常规平仓会做这些），`recordClosePositionOrder` 对 binance 直接跳过。
- 为何现有防护不够: 与正在执行 cycle 的同一币种操作可并发（例如同时在撤换单/补保护），平仓后依赖下一轮 reconcile 修状态；叠加上面第一条的死代码，保护单还可能残留。另外该接口不区分"AI 仓位 / 用户手动仓位"，hands-off 规则在这里不生效（属于用户显式操作，故只评 MEDIUM）。
- 建议修复: 对运行中的 trader 走 `at` 的受锁平仓方法（`withExecutionLock(at.executionMutex(), ...)` + `markCloseIntent`）；对手动仓位至少在响应里标注 `ai_managed=false` 并要求显式 `force=true`。

### [MEDIUM][PLAUSIBLE] 日损失熔断 / 回撤峰值基线不感知资金划转与手动仓位盈亏
- 位置: `trader/auto_trader_risk.go:2136-2200`（`anchorDailyBaseline`/`dailyLossHaltBlocks`）、`store/risk_state.go:74-91`（`__peak__` 只增不减）
- 触发场景: 用户把 USDT 在合约钱包与现货之间划转（给 us-stock 现货账户补资金很常见），或在同一账户手动交易。基线是"当日 UTC 首次看到的 total equity"，peak 永不衰减。
- 证据: 全仓库 `grep -i transfer|deposit|withdraw` 在 trader 风控代码里无命中；`UpdatePeak` 只做 `day_start_equity < equity` 更新。划出资金 ≥ `daily_max_loss_pct` 会误触发"开仓暂停"（保守方向）；划入资金则抬高等效容忍亏损额（激进方向，真实亏损要更多才触发）；回撤熔断（grid 的 `checkMaxDrawdown`）在提现后相对旧 peak 永远显示回撤。
- 为何现有防护不够: 没有 equity 里扣除净划转的逻辑，也没有 peak 重置入口。需要确认的是主 trader 的 `account_max_drawdown_pct` 用的是 initial_balance 还是 peak（我只看到 grid 用 peak）。
- 建议修复: 拉取交易所 income/transfer 流水（Binance `/fapi/v1/income` 的 TRANSFER 类型）调整基线；或提供"重置基线"的受控 API，并在划转检测到时自动重锚并告警。

### [MEDIUM][PLAUSIBLE] 删除 trader 没有"仍有持仓/挂单"检查，且会清空 AI 归属表，持仓变成"手动仓位"被永久放生
- 位置: `api/handler_trader.go:777-806`，`store/trader.go:159-190`（`Delete` 清 `TraderPosition/TraderOrder/AIManagedPosition/PendingEntryDB/...`）
- 触发场景: 在 mac-nofx 有 AI 开的仓位和交易所侧 SL/TP 时误点删除（或经 Telegram /confirm 删除），随后重建 trader。
- 证据: handler 只做 `GetForUser` 校验后就后台 `RemoveTraderAndThen(..., store.Trader().Delete)`；Stop 不平仓；`AIManagedPosition` 被删后新 trader 的 `isAIManaged()` 为假，hands-off 规则使程序不再管理/平这些仓位，also 保护 watchdog 不会补它们。
- 为何现有防护不够: 没有任何 pre-delete 守卫；这是用户显式操作，但后果是实盘仓位脱离程序管理。
- 建议修复: 删除前检查 `GetPositions()`/`GetOpenOrders` 非空则拒绝（或要求 `force=true` 并列出受影响仓位）；归属表改为软删除。

### [LOW][CONFIRMED] Telegram bot token 会随网络错误写入日志（并明文存库）
- 位置: `telegram/bot.go:93-101,129-141`（`logger.Errorf("Telegram bot failed to start: %v")`、`"getUpdates failed: %v"`），`telegram/notify/notify.go:123,127,203`；token 存储 `store/telegram_config.go:25`（`BotToken string` 非 `EncryptedString`）
- 触发场景: 网络抖动/超时。tgbotapi v5.5.1 的 `bot.Client.Do(req)` 直接 `return nil, err`（`bot.go:110-112,234-236`），`*url.Error` 的字符串包含完整 URL `https://api.telegram.org/bot<TOKEN>/getUpdates`。
- 为何现有防护不够: 日志里的 token 允许他人冒充 bot 发消息/抢占 getUpdates（无法伪造用户消息进入 agent，因为 chat 绑定在 Telegram 服务端认证）。交易所密钥没有发现同类泄露（Binance 的 url.Error 只含 timestamp/signature，API key 在 header）。
- 建议修复: 日志前对 err 做 `strings.ReplaceAll(err.Error(), token, "***")`；`BotToken` 改用 `crypto.EncryptedString`。

### [LOW][CONFIRMED] `show_in_competition` 默认 true，未认证接口公开真实资金曲线/持仓数/保证金占用
- 位置: `api/handler_trader.go:399-402`（默认 true），`api/server.go`（`/api/traders`、`/api/competition`、`/api/top-traders`、`/api/equity-history*` 均无需认证），`docker-compose.yml` 中 `API_SERVER_HOST=0.0.0.0`
- 触发场景: 实盘 trader 创建时没关掉竞赛可见，并且 API 对外可达（docker/nginx 部署或绑定 0.0.0.0）。返回 total_equity、total_pnl、position_count、margin_used_pct。
- 建议修复: 默认 false；公共接口对金额做归一化（只给收益率）。默认原生运行绑定 127.0.0.1 时无影响。

### [LOW][CONFIRMED] 每次受保护 GET 都 `LoadUserTradersFromStore`，与 Stop/reload 共用 `loadMu`，配置保存期间整个看板被卡住
- 位置: `api/server.go:641-669`（`getTraderFromQuery`），`manager/trader_manager.go:405-415,466-486`
- 证据: `loadMu` 在 `removeTrader` 里被持有直到 `t.Stop()`（join 进行中的 AI cycle，注释自述 1-10 分钟）返回；任何看板轮询（account/positions/status）先拿 `loadMu`。此外对"之前加载失败"的 trader，每次轮询都会重试 `NewAutoTrader`（含交易所网络调用，`NewFuturesTrader` 里 `syncBinanceServerTime`/`setDualSidePosition`）。
- 影响: 非资金风险，但保存策略/切模型期间前端一片超时，用户容易误以为失败而重复保存（又触发新一轮 Stop）。
- 建议修复: `getTraderFromQuery` 只在 `GetTrader` 找不到且 DB 里该 trader 应加载时才尝试加载，并对失败加退避。

### [LOW][CONFIRMED] 反向代理部署下登录限流桶被所有客户端共享，可被匿名用户锁死登录
- 位置: `api/server.go:44`（`SetTrustedProxies(nil)`），`api/request_limits.go:35-41`
- 场景: nginx→nofx 时 `c.ClientIP()` 永远是代理地址，`/api/login` 的 10 次/分钟桶全局共享；任何人每分钟打 10 次就能让真实用户无法登录（也无法经 UI 紧急停机；Telegram 通道不受影响）。原生 127.0.0.1 部署不受影响。
- 建议修复: docker 部署时配置 trusted proxies 为 nginx 网段，使用 `X-Forwarded-For`。

---

## 已核对、未发现问题的点
- 用户隔离: `getTraderFromQuery`/`GetForUser` 覆盖所有 `trader_id` 入口；`system-quality`、`entry-quality-stats`、`orders/:id/fills` 均有归属校验；公共接口要求 `show_in_competition`。
- 注册仅限首个用户（`CreateFirst`），`reset-password` 已关闭，JWT 校验 HS256 + issuer + exp + 黑名单 + `TokenVersion`；`JWT_SECRET` 未配置时随机生成（重启后会话失效，安全方向）。
- 密钥存储: `EncryptedString`（AES-GCM）覆盖 API key/secret/passphrase/私钥，`GET /exchanges` 只返回 Safe 结构；日志里的 AI key 仅显示前后 4 位且 `len>8` 才打印。
- SQL: 所有 `Raw/Exec` 参数化或使用常量，`VACUUM INTO` 路径拒绝单引号。
- 生命周期: `StartTrader` 在 `loadMu` 内解析并启动（已修孤儿实例）；`RunVersion` 意图版本避免 reload 覆盖新的 stop；`StopAll` 先 `SignalStop` 再并发 `StopForRestart`，保留 `is_running` 以便重启自动恢复；数据库 `synchronous=FULL`。
- Telegram: 绑定码 128 位、5 分钟、一次性且存哈希；未绑定 chat 一律拒绝；变更类请求只允许 `traders|strategies` 前缀且需 nonce 确认。

## 优化建议
1. **确定性紧急命令**（`/positions`、`/stop <trader>`、`/flat`）绕过 LLM 与车道，只需 chat 绑定 + 二次确认；预期：把"远程停机"从几十秒~分钟（LLM 链）降到秒级，并避免上面 JWT 过期/车道排队问题。
2. **统一 `safeGo` + 进程级 panic 告警**：把所有后台 goroutine 收口，预期消除"配置保存触发 panic → 实盘进程重启"的盲区窗口。
3. **转账感知的风险基线**：从交易所 income 流水自动扣除净划转，并提供受控的"重锚"操作；预期：避免在给 us-stock 现货账户补资金当天误触发日损熔断，或在入金后放宽熔断。
4. **删除/停止 trader 的持仓守卫与"停机时保护确认"**：Stop 前后检查是否有裸仓位，并通知；预期：避免误删造成的仓位脱管。
5. **看板读路径与 reload 解耦**（去掉每次 GET 的 `LoadUserTradersFromStore`）：预期：保存配置期间 UI 不再卡死，减少用户重复点击保存造成的多余 Stop/Start 周期。
