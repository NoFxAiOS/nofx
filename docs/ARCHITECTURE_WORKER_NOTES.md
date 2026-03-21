# NOFX 架构工作笔记（主控汇总）

> 目的：快速明确启动链、AI 决策链、交易执行链、风控链，作为后续优化与任务拆分基础。

## 1. 启动链（Startup Chain）

主入口位于 `main.go`。

### 1.1 启动顺序
1. `godotenv.Load()` 读取 `.env` (`main.go`)
2. `logger.Init(nil)` 初始化日志 (`main.go`)
3. `config.Init()` 读取全局配置，随后 `config.Get()` 取配置对象 (`config/config.go`)
4. `crypto.NewCryptoService()` 初始化加密服务，依赖：
   - `DATA_ENCRYPTION_KEY`
   - `RSA_PRIVATE_KEY`
   见 `crypto/crypto.go`
5. `store.NewWithConfig(...)` 初始化数据库 (`main.go` -> `store/*`)
6. `initInstallationID(st)` 初始化匿名 telemetry installation id (`main.go`)
7. `auth.SetJWTSecret(cfg.JWTSecret)` 设置 JWT 密钥 (`main.go`, `auth/auth.go`)
8. `manager.NewTraderManager()` 创建交易员管理器 (`manager/trader_manager.go`)
9. `traderManager.LoadTradersFromStore(st)` 从数据库加载 trader（此函数在 manager 层，运行时会把数据库配置恢复为内存中的 `AutoTrader`）
10. `api.NewServer(...)` 创建 Gin 服务，内部 `setupRoutes()` 注册全部 API (`api/server.go`)
11. `telegram.Start(cfg, st, telegramReloadCh)` 启动 Telegram Bot (`main.go`)
12. 等待系统信号，退出时 `traderManager.StopAll()` 做停机收尾 (`main.go`, `manager/trader_manager.go`)

### 1.2 启动链关键观察
- 加密服务初始化早于数据库，意图是确保数据库中敏感字段读取时可解密（`main.go` 注释明确说明）
- Trader 在 API 启动前已经加载到内存，因此 API 层天然依赖 manager 的内存态
- Telegram bot 热重载通过 `telegramReloadCh` 和 API server 连接

---

## 2. AI 决策链（Decision Chain）

核心执行入口位于 `trader/auto_trader_loop.go` 的 `runCycle()`。

### 2.1 决策链主流程
1. `runCycle()` 开始一个周期 (`trader/auto_trader_loop.go`)
2. 检查：
   - trader 是否仍在运行
   - 是否在 `stopUntil` 风控暂停窗口
   - 是否需要 daily pnl reset
3. `buildTradingContext()` 组装交易上下文（定义在 trader 相关代码，结果写入 `kernel.Context`）
4. `saveEquitySnapshot(ctx)` 独立保存权益快照 (`trader/auto_trader_decision.go`)
5. 若候选币为空，记录 decision log 并跳过 (`trader/auto_trader_loop.go`)
6. `kernel.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, "balanced")`
   - 入口：`kernel/engine_analysis.go`
7. `GetFullDecisionWithStrategy(...)` 内部执行：
   - `fetchMarketDataWithStrategy(ctx, engine)` 拉行情/指标 (`kernel/engine_analysis.go`)
   - `engine.BuildSystemPrompt(...)` 构建系统提示词 (`kernel/engine_prompt.go`)
   - `engine.BuildUserPrompt(ctx)` 构建用户提示词 (`kernel/engine_prompt.go`)
   - `mcpClient.CallWithMessages(systemPrompt, userPrompt)` 调模型 (`kernel/engine_analysis.go`)
   - `parseFullDecisionResponse(...)` 解析 AI 输出 (`kernel/engine_analysis.go`)
   - `validateDecisions(...)` 校验决策（同文件后续逻辑）
8. `runCycle()` 将 prompt / CoT / raw response / decisions 写入 `store.DecisionRecord` (`trader/auto_trader_loop.go`)
9. 若 AI 连续失败 ≥ 3 次，则进入 safe mode，阻止新开仓 (`trader/auto_trader_loop.go`)
10. 对决策排序：先平仓、后开仓（`sortDecisionsByPriority(...)`，调用点在 `runCycle()`）
11. 逐条执行决策 `executeDecisionWithRecord(...)` (`trader/auto_trader_orders.go`)
12. 保存 decision record (`saveDecision()` in `trader/auto_trader_decision.go`)

### 2.2 Prompt 与风控的关系
- `BuildSystemPrompt()` 会把硬性约束和建议性约束一起写入 prompt (`kernel/engine_prompt.go`)
- 其中：
   - `CODE ENFORCED`：后端会再次强制校验
   - `AI GUIDED`：主要是给模型行为约束
- 这说明系统是“双层风控”：
   1. Prompt 约束模型输出
   2. 代码层再做兜底

### 2.3 决策链风险点
- 模型输出解析依赖 XML tag + JSON 提取，容错虽做了，但仍属于脆弱环节（`extractCoTTrace`, `extractDecisions` in `kernel/engine_analysis.go`）
- prompt 很长，且输出含 CoT / raw response，长期运行的存储成本和隐私风险要进一步评估
- safe mode 触发逻辑是“连续 3 次 AI 失败”，但恢复逻辑只要下一次成功就解除，需要后续审视是否过于宽松

---

## 3. 交易执行链（Execution Chain）

核心执行函数位于 `trader/auto_trader_orders.go`。

### 3.1 执行入口
`executeDecisionWithRecord(decision, actionRecord)` 按 action 分发：
- `open_long`
- `open_short`
- `close_long`
- `close_short`
- `hold` / `wait`

### 3.2 开仓链（以 `open_long` 为例）
入口：`executeOpenLongWithRecord()` (`trader/auto_trader_orders.go`)

执行顺序：
1. `at.trader.GetPositions()` 获取当前持仓
2. `enforceMaxPositions(...)` 强制最大持仓数
3. 检查是否已有相同 symbol 同方向仓位
4. `market.GetWithExchange(symbol, at.exchange)` 获取当前价格
5. `at.trader.GetBalance()` 获取余额/权益
6. `enforcePositionValueRatio(...)` 校验仓位价值比例
7. 按 availableBalance 自动缩减仓位，避免保证金不足
8. `enforceMinPositionSize(...)` 校验最小仓位
9. `at.trader.SetMarginMode(...)`
10. `at.trader.OpenLong(...)` 真正下单
11. `recordAndConfirmOrder(...)` 记录并轮询确认订单 (`trader/auto_trader_decision.go`)
12. 记录首次见到仓位时间 `positionFirstSeenTime`
13. `at.trader.SetStopLoss(...)`
14. `at.trader.SetTakeProfit(...)`

`open_short` 基本同构。

### 3.3 平仓链
`close_long` / `close_short` 由对应函数完成，调用交易所适配器的 close 方法，并记录订单及持仓变更。细节在 `trader/auto_trader_orders.go` 同文件后续。

### 3.4 执行链关键观察
- 交易所差异被下沉到 `Trader` 接口及各子目录实现：
  - `trader/binance/*`
  - `trader/bybit/*`
  - `trader/okx/*`
  - `trader/gate/*`
  - `trader/kucoin/*`
  - `trader/hyperliquid/*`
  - 等
- AutoTrader 层承担统一业务规则，交易所 adapter 负责具体 API 细节
- 执行链已经包含“自动缩仓”逻辑，而不只是简单 reject，这对稳定性是优点，但也会影响收益评估和行为可解释性

---

## 4. 风控链（Risk Control Chain）

风控分散在 prompt、执行前校验、运行中监控三层。

### 4.1 Prompt 层风控
`kernel/engine_prompt.go` 中 `BuildSystemPrompt()` 写入：
- 最大持仓数
- BTC/ETH 与 altcoin 的 position value ratio 限制
- 最大保证金使用率
- 最小仓位
- 最大杠杆
- 最低 confidence
- 最低风险收益比

### 4.2 执行前硬校验
位于 `trader/auto_trader_risk.go`：
- `enforcePositionValueRatio()`
- `enforceMinPositionSize()`
- `enforceMaxPositions()`

位于 `trader/auto_trader_orders.go`：
- 同向重复持仓检查
- availableBalance 约束下的自动缩仓

### 4.3 运行时监控
位于 `trader/auto_trader_risk.go`：
- `startDrawdownMonitor()`：每分钟轮询持仓
- `checkPositionDrawdown()`：
  - 维护 `peakPnLCache`
  - 若 `currentPnLPct > 5%` 且从峰值回撤 ≥ 40% 则触发紧急平仓
- `emergencyClosePosition()`：直接平仓

### 4.4 异常风控
位于 `trader/auto_trader_loop.go`：
- AI 连续失败 3 次 → safe mode
- safe mode 下过滤掉所有 `open_long/open_short`
- 保留 close/hold 行为

### 4.5 风控链关键观察
- 风控并非集中在单一模块，而是分布在：
  - `kernel`（提示词约束）
  - `trader`（执行时硬校验）
  - `trader`（运行时 drawdown monitor）
- 目前最需要进一步确认的点：
  1. `MaxMarginUsage` 在 prompt 中存在，但我还未看到执行前硬拒绝逻辑的明确落点，需继续核验
  2. `MinRiskRewardRatio` / `MinConfidence` 更像 AI 约束，是否有后端硬校验需继续确认
  3. 回撤止盈逻辑是固定阈值，不同市场/杠杆/品种可能需要更细粒度参数化

---

## 5. API / 前端主入口观察

### 5.1 API 层
- 路由注册入口：`api/server.go` -> `setupRoutes()`
- 文档注册辅助：`api/route_registry.go`
- API 分层：
  - 公共接口：健康、市场数据、competition、public strategies
  - 鉴权接口：trader / model / exchange / telegram / strategy / user

### 5.2 前端入口
- 根入口：`web/src/App.tsx`
- 主要页面：
  - `TraderDashboardPage.tsx`
  - `SettingsPage.tsx`
  - `StrategyStudioPage.tsx`
  - `StrategyMarketPage.tsx`
  - `DataPage.tsx`
  - `FAQPage.tsx`
- 关键控制页面实际上大量来自 `web/src/components/trader/*` 与 `web/src/components/auth/*`

---

## 6. 当前适合继续拆给工种的任务

### 架构工
- 继续补齐 `buildTradingContext()` 与 `LoadTradersFromStore()` 的精确调用链
- 输出实体关系图（Trader / Strategy / Exchange / Decision / Position / Equity）

### 风控工
- 专查 `MaxMarginUsage` / `MinRiskRewardRatio` / `MinConfidence` 是否真正硬校验
- 专查下单确认、订单同步、幂等与异常补偿逻辑

### 前端工
- 做 API ↔ 页面 ↔ 组件映射
- 查出大 bundle 的主要来源和拆包机会

### 测试工
- 建立“启动 / 创建 trader / 启停 / 决策 / 下单 / 平仓 / 仪表盘”最小回归路径
