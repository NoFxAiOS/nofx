# Area D 审查报告：行情数据层（market / bnweight / breakout / provider / kernel Binance 数据 / trader 行情入口）

仓库 commit 744da725，只读审查。路径均相对仓库根 `/Users/zhangyun/workspace/nofx/.claude/worktrees/subagent-models-c250d2`。

总体结论：指标实现（EMA/RSI/ATR/BOLL 的种子与 Wilder 平滑）、已收盘 K 线处理（breakout 的 `fetchKlines` 丢弃形成中 K 线、`calculateTimeframeSeries` 的 ATR 只用已收盘 K 线）整体正确，没有发现未来函数。主要风险集中在「行情失败时的行为」：行情依赖失败会阻断平仓、兜底保护，以及几处持锁做网络 IO 的串行放大。

---

### [HIGH][CONFIRMED] AI 平仓路径依赖完整行情快照：K 线/陈旧检测失败即无法平仓
- 位置: trader/auto_trader_orders.go:957-961（平多）、1033-1037（平空）；trader/auto_trader_marketdata.go:13-26；market/data.go:104-190、1068-1108
- 触发场景: 模型决定 close_long/close_short。执行器先调用 `at.getMarketData(symbol)`，只是为了给 `actionRecord.Price` 和 `recordAndConfirmOrder` 取一个价格。`getMarketData` 实际是 `market.GetWithExchangeAndPrice`，要成功需要同时满足：3m K 线（CoinAnk，失败后 Binance 直连）可得、`isStaleData` 不报陈旧、4h K 线可得且非空。任一失败，平仓直接 `return err`，不会下单。
  - 例 1：CoinAnk 宕机（09-24 事件）且 Binance 直连被 bnweight 冷却拒绝（Normal 优先级，429/418 期间本地直接返回 ErrCooldown）。此时正是行情剧烈、最需要平仓的时刻。
  - 例 2：薄流动性标的（夜间小币、周末 bStock 永续）连续 5 根 3m 收盘价波动 <0.01% 且成交量为 0，`isStaleData` 返回 true，报 "data is stale, possible cache failure"，平仓被拒。
- 证据:
```go
// auto_trader_orders.go:957
marketData, err := at.getMarketData(decision.Symbol)
if err != nil { return err }
actionRecord.Price = marketData.CurrentPrice
...
order, err := at.trader.CloseLong(decision.Symbol, 0)
```
```go
// data.go:132
if isStaleData(klines3m, symbol) { return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol) }
```
- 为何现有防护不够: 交易所侧的止损单仍在，但「AI 主动平仓」「风控触发的平仓」若走同一函数就被行情质量挡住。平仓是降低风险的动作，不应依赖入场分析所需的数据质量。`CloseLong(symbol, 0)` 本身并不需要 K 线。
- 建议修复: 平仓路径只取执行价（`at.trader.GetMarketPrice`），失败时用 0 或最近一次价格继续下单，价格仅用于记录。把完整 `getMarketData` 限定在开仓/改单路径。同时顺带省掉每次平仓的 7+ 次 HTTP 请求和延迟。

---

### [MEDIUM][CONFIRMED] 裸仓兜底保护 `placeComputedProtection` 依赖 Normal 优先级 K 线，限流/供应商故障时放弃
- 位置: trader/auto_trader_risk.go:1741-1749；market/data_klines.go:558-571；market/bnweight/meter.go:315-330、api_client.go:37-38
- 触发场景: 发现无保护持仓后，兜底逻辑要通过 `getMarketTimeframes(symbol, ["1h"], ...)` 取 1h ATR 来算 SL/TP。K 线链路是 CoinAnk，再 Binance 直连（`NewAPIClient` 包装为 Normal 优先级）。若 CoinAnk 异常，同时 Binance 触发 429/418 冷却，Normal 请求被 `admit` 本地直接拒绝；即使没冷却，用量 ≥2000 时 Normal 会被 `admit` 挂起到下一分钟，最长 60 秒。结果：`data == nil` 或 `ATR14 <= 0` 返回 false，持仓继续裸奔，只剩告警。
- 证据:
```go
data, err := at.getMarketTimeframes(symbol, []string{"1h"}, "1h", 99)
if err != nil || data == nil { return false }
tf := data.TimeframeData["1h"]
if tf == nil || tf.ATR14 <= 0 { return false }
```
- 为何现有防护不够: 兜底的存在意义就是行情/限流异常时仍能保护，而它恰恰与这些异常相关（限流多发生在行情剧烈时）。
- 建议修复: ① 兜底路径用 `bnweight.WithPriority(ctx, Critical)` 并让 `getKlinesFromBinanceDirect` 接受 ctx；② ATR 不可得时退化为固定百分比止损（如 min(1/杠杆×0.5, 3%)），保证两腿必挂。

---

### [MEDIUM][CONFIRMED] `binanceTopTickers` 持全局锁做网络请求且失败不缓存：超时时每个候选串行等待 15s
- 位置: kernel/engine_data_binance.go:93-118、166-181、747-758；调用方 kernel/signal_layer.go（`btcWeakLongCodes` → `binanceBTC24hLivePct`）、kernel/engine_prompt.go:1723
- 触发场景: `ticker/24hr`（权重 40）请求挂起或超时（本机经本地代理访问 Binance，代理抖动时常见）。`tickersMu` 在整个请求期间被持有；请求失败不写缓存，下一个调用者拿到锁后重新发起，又等 15s（`SafeHTTPClient(15s)`）。每个候选币的 `binanceQuoteVolume24h`、`binanceBTC24hLivePct` 都会走这里，20 个候选 ≈ 20×15s = 5 分钟的决策周期卡死。冷却状态下是本地立即拒绝，不受影响；只有超时/挂起场景触发。
- 证据:
```go
tickersMu.Lock()
defer tickersMu.Unlock()
if tickersCache != nil && time.Since(tickersFetched) < 60*time.Second { return tickersCache, nil }
...
if err := binanceGetFn(ctx, "/fapi/v1/ticker/24hr", &all); err != nil { return nil, ... }  // 无负缓存
```
- 为何现有防护不够: 60s 缓存只对成功结果有效；`binanceBTC24hLivePct` 用 `context.Background()`，没有调用方级别的超时。
- 建议修复: 失败时记录 `lastFailAt`，30s 内直接返回上次缓存（允许陈旧）或快速失败；网络请求放到锁外并用 singleflight（参考同文件 `binanceBTC4hCloses` 的做法）。

---

### [MEDIUM][CONFIRMED] BTC 4h 趋势过滤在拉取失败时静默关闭，且不使用陈旧缓存
- 位置: kernel/engine_data_binance.go:777-841（`binanceBTC4hCloses`）、269-298（`binanceBTC1hCloses`）；kernel/signal_layer.go:1773-1778
- 触发场景: BTC 大跌 → 权重飙升/429 冷却 → `binanceGetFn` 失败 → `binanceBTC4hCloses` 返回 nil → `btc4hShape` 的 `known=false` → `btcWeakLongCodes` 直接 `return codes`（空）。`BTC_4H_DOWNTREND` 与 `BTC_WEAK_LONG` 两道山寨多头闸门在最需要的时候消失。缓存超过 5 分钟就被 `readCache` 丢弃，哪怕最后一次成功数据只有 6 分钟，而 4h 收盘序列几乎不变。
- 证据:
```go
if len(btc4hClosesCache) >= limit && time.Since(btc4hClosesFetched) < 5*time.Minute { ... return out }
...
if err := binanceGetFn(ctx, ...); err != nil { return nil }   // 不回退到旧缓存
```
```go
// signal_layer.go: "Missing data passes — a data gap must not freeze every altcoin long."
if !known { return codes }
```
- 为何现有防护不够: 设计注释已说明「缺失数据放行」是有意的，但这对「失败回退到最近一次成功值」没有任何论证；4h 已收盘序列的陈旧容忍度远大于 5 分钟。
- 建议修复: 抓取失败时回退到最近成功缓存（容忍 ≤4h，等于一根 4h K 线周期），仅在从未成功时才放行；`BTC1h` 同理。

---

### [MEDIUM][CONFIRMED] 陈旧检测只看「收盘价冻结且成交量为 0」，不检查最后一根 K 线的时间
- 位置: market/data.go:1068-1108；market/data.go:132、405；market/data.go:264、1120-1139（`executionTimeframeData`）
- 触发场景: 供应商（CoinAnk）返回滞后序列，例如最后一根 3m K 线停在 30 分钟前，但此前价格有波动。`isStaleData` 只检查最后 5 根是否冻结，通过。`applyLivePrice` 对已收盘 K 线直接 `return false`，不做修补。`currentPrice` 用实盘价，但 ATR/EMA/RSI/1h/4h 序列都是滞后数据，而 `TimeframeData` 正是止损带下限/上限、vol-target 的尺度来源。
  - 信号路径 `getWithTimeframes` 有 `VendorStalenessPct`（实盘价与最后收盘价偏差）兜底，阈值默认 2%，偏差小就放行。执行路径 `GetWithExchangeAndPrice`（`getMarketData`）没有任何这类度量。
- 证据: `isStaleData` 全函数不含时间比较；`GetWithExchangeAndPrice` 无 `OpenTime` 与 `time.Now()` 的比较。
- 为何现有防护不够: VENDOR_DIVERGENCE 只覆盖 signal 路径；执行路径（止损带、仓位大小）没有等价检查。
- 建议修复: 在 K 线获取后统一校验 `last.OpenTime + 2*dur >= now`（3m 约 6 分钟，4h 约 8 小时），失败切换到 Binance 直连或报错。同时给 `isStaleData` 加「5 根冻结但成交量为 0」之外的确认，减少对薄流动性标的的误杀（与第 1 条联动）。

---

### [MEDIUM][PLAUSIBLE] `IsXyzDexAsset` 在交易所上市缓存未加载时错误路由，导致数据与订单路径符号不一致
- 位置: market/data.go:950-964、966-999；market/funding_history.go:123-165、207-217
- 触发场景: `binanceListsNative` 只读缓存，不触发加载。进程冷启动后，在第一次 `IsBStockSymbol`/`IsUSEquitySymbol` 之前，`Normalize("PLTRUSDT")` 返回 `xyz:PLTR`，之后缓存加载完成变成 `PLTRUSDT`。若 exchangeInfo 首次加载失败，会被 `bstockRetryEvery` 锁定 1 小时（`lastErr` 分支直接返回 nil 集合），这一小时内所有原生上市的股票永续（PLTR/TSLA/SNDK 等）都被路由到 Hyperliquid 路径，OI 获取失败，被冒号过滤器排除。同一周期内，数据层 key 与订单/持仓匹配的符号还可能不一致。
- 证据:
```go
func binanceListsNative(base string) bool {
	...
	if binanceListed == nil { return false }   // 不触发 loadBStockSymbols
```
- 为何现有防护不够: aefef390 的修复只改了「缓存已加载」的情形，注释明说「nil cache falls back to legacy behavior」。影响偏 fail-closed（候选被排除），所以评为 MEDIUM；若持仓符号 key 因此在周期中途变化，则可能影响持仓匹配，需用日志确认。
- 建议修复: `binanceListsNative` 在缓存为 nil 时先同步调用 `loadBStockSymbols()`（网络失败时在 Normalize 里不要阻塞，可退化为后台加载 + 白名单默认）；失败重试间隔从 1h 缩短到 2-5 分钟。

---

### [LOW][CONFIRMED] `GetSymbolLotFilter` 持锁做网络请求，且每个符号缓存未命中都下载整份 exchangeInfo
- 位置: market/symbol_lot_filter.go:51-91
- 触发场景: `lotFilterMu` 在 `binanceGetJSON("/fapi/v1/exchangeInfo")` 整个期间被持有（HTTP 超时 30s，Normal 在高用量下还可能被 `admit` 挂起至 60s）。冷启动或每 6 小时过期时，N 个候选各自下载约 1-2MB 的完整 exchangeInfo 并串行解析。symbol 不存在时也会缓存默认 `MinNotional=5` 6 小时。
- 证据: `lotFilterMu.Lock(); defer lotFilterMu.Unlock()` 之后才调用 `binanceGetJSON`。
- 建议修复: 一次下载整份 exchangeInfo 填充全部符号（与 `loadBStockSymbols` 共用同一次下载），锁外请求，symbol 缺失时不缓存为权威值。

---

### [LOW][PLAUSIBLE] `kernel.binanceGet` / `sentimentHTTPGet` 每次请求新建 Transport，连接与 goroutine 累积
- 位置: kernel/engine_data_binance.go:75-77；market/sentiment.go:135-138；security/url_validator.go:214-222
- 触发场景: `SafeHTTPClient` 每次创建新的 `http.Transport`（`ForceAttemptHTTP2`，未设 `IdleConnTimeout`）。排行榜构建每 5 分钟约 150 符号×2-3 个请求，每个请求一个新 Transport，空闲连接保持到对端关闭。确认需要在实机观察 fd/goroutine 数。
- 建议修复: 包级复用单个 client（包装 bnweight 后缓存）。

---

### [LOW][PLAUSIBLE] bnweight：418 封禁期间 Critical 请求照发，可能延长封禁
- 位置: market/bnweight/meter.go:315-317、405-407
- 触发场景: 收到 418 后 Normal/Low 被拒，但 Critical（持仓/账户读取、保护单）继续发送。Binance 对封禁期内继续请求的 IP 会延长封禁。Critical 的调用频率若较高（保护看门狗轮询），封禁可能被拉长。这是有意的取舍（保护优先），所以仅提示。
- 建议: 418 期间对 Critical 做最小间隔（如 ≥5s）的节流，只放行下单/撤单类写操作。

---

### [LOW][CONFIRMED] CoinAnk K 线解析无长度检查、无顺序校验
- 位置: provider/coinank/coinank_api/kline.go:36-46；market/data_klines.go:82-101
- 触发场景: 供应商返回行长度 <9 时 `k[7]`/`k[8]` 下标越界 panic（取决于上层是否 recover）；返回降序或重复时间戳时 EMA/ATR 静默算错，且没有任何校验。
- 建议修复: 解析时检查 `len(k) >= 9`，并在 `getKlinesFromCoinAnk` 里校验升序且 `OpenTime` 严格递增，不满足则走 Binance 直连。

---

### [LOW][CONFIRMED] 其他数据保真度问题
- 强平流 `!forceOrder@arr` 每个符号每 1000ms 只推送最新一条，级联时系统性低估（market/liquidation_feed.go:48），而 `Long24hUSD` 在提示词里当作完整数据呈现；`liqFeedLoop` 的退避时间从不重置（:94-107）；`recordLiquidation` 每条事件全量扫描 `liqEvents`（:240-263），而该切片其实没有读者，纯属内存/CPU 浪费。
- `GetWithExchangeAndPrice` 的 `priceChange4h = klines4h[len-2].Close`（data.go:234-239）是「当前 4h K 线开盘价」，窗口为 0-4h，随相位变化；`calculatePriceChangeByBars` 在主周期为 4h 时同理（data_klines.go:261-290）。`CHASE_MOVE4H` 闸门读取该值（kernel/signal_layer.go:1711），临近 4h 边界会漏判跨边界的大波动。
- `calculateLongerTermData`/`calculateIntradaySeries` 的 ATR 与成交量对比包含形成中 K 线（data_klines.go:374、392-402），ATR 偏低、Current Volume 相对 Average 系统性偏低；09-28 对 `calculateTimeframeSeries` 的修复没有覆盖这两处。
- nofxos 默认 API key 硬编码并以 `auth=` 放在 URL 查询串里（provider/nofxos/client.go:19、99-101）；HTTP 错误的 `url.Error` 含完整 URL，会被日志带出。vergex relay 的 `relayTTL` 为 30 分钟（provider/vergex/relay.go:21），候选列表最多陈旧 30 分钟；只要是合法 JSON 对象（含错误体）就覆盖旧缓存（relay.go:53-60）。
- `getFundingRate` 的注释仍写「1-hour cache」，实际 TTL 5 分钟（data.go:687），仅注释不一致。
- `binanceGetJSON`（market/data.go:673-681）与 `kernel.binanceGet` 不检查 HTTP 状态码，靠 JSON 解码失败间接报错；`GetSymbolLotFilter`、`loadBStockSymbols` 在错误体恰好是合法 JSON 时会把「空结果」当成功处理。

---

## 优化建议

1. **平仓走轻量价格路径**（对应第 1 条）：只取一次 `GetMarketPrice`，省掉每次平仓约 7 个请求（3m/4h/1h/1d K 线、OI、funding×2），平仓延迟从数秒降到数百毫秒，同时去除行情依赖。预期：杜绝因数据问题无法平仓的情形，降低限流压力。
2. **CoinAnk 熔断器**：连续失败 N 次后直接走 Binance 直连 5-10 分钟。当前每个周期、每个 TF 都会先等 CoinAnk（最坏 30s 超时），最坏情况 5 TF×60s。预期：供应商故障期间单币数据获取时间从分钟级降到秒级。
3. **exchangeInfo 单次下载共享**：`GetSymbolLotFilter`、`loadBStockSymbols` 合并为一次下载、一份缓存、singleflight。预期：冷启动少 100+ 次 1-2MB 下载，消除持锁串行等待。
4. **指标增量计算**：`calculateTimeframeSeries` 对最近 `count` 个点都从头重算 EMA/MACD/RSI/BOLL（O(count×n)，n 最高 1500）。改为一次遍历产生序列，可降低每次数据构建的 CPU，并避免候选多时拉长周期。
5. **复用 HTTP client**：`kernel.binanceGet`、`sentimentHTTPGet` 复用带 bnweight 包装的单例 client，启用连接复用，降低排行榜构建时的 TLS 握手与 fd 增长。

---
总结（3 行）：
1. 最严重：平多/平空路径硬依赖完整行情快照（含陈旧检测与 4h K 线），行情或限流故障时 AI 平仓会被拒，建议只取执行价。
2. 其次：裸仓兜底保护、BTC 趋势过滤、`binanceTopTickers` 持锁串行，都在「限流/供应商故障」这类相关场景下失效或放大；陈旧检测缺少 K 线时间校验。
3. 指标实现与已收盘 K 线处理本身正确，未发现未来函数或毫秒/秒单位错误；其余为 LOW 级数据保真度与注释问题。
