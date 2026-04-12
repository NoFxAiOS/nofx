### 晚间：Position History 主记录 + 子事件流落地
- 历史持仓记录已从“只看最终聚合结果”推进到“主记录 + 子事件流”结构：
  - 主记录：`trader_positions`
  - 子事件流：`position_close_events`
- 新事件流字段覆盖：
  - `position_id / trader_id / exchange_id / symbol / side`
  - `close_reason / execution_source / execution_type`
  - `exchange_order_id`
  - `close_quantity / close_ratio_pct / execution_price / close_value_usdt`
  - `realized_pnl_delta / fee_delta / event_time`
- 当前已落库/可归因的业务原因：
  - `ai_close_long`
  - `ai_close_short`
  - `managed_drawdown`
  - `emergency_protection_close`
  - `native_trailing`
  - `break_even_stop`
  - `full_tp`
  - `full_sl`
  - `ladder_tp`
  - `ladder_sl`
- 当前归因规则：
  - `TRAILING*` → `native_trailing`
  - `TAKE_PROFIT*` + partial/full → `ladder_tp` / `full_tp`
  - `STOP*` + near-entry → `break_even_stop`
  - `STOP*` + partial/full → `ladder_sl` / `full_sl`
- 前端 `PositionHistory` 已同步支持：
  - 展开后显示 `Close Event Flow`
  - 原因标签统一颜色体系（AI / trailing / break-even / TP / SL / unknown）
- 重要边界：
  - 这条链主要影响**这版上线后的新平仓事件**
  - 旧历史不会自动批量重算，只会继续依赖已有字段或 API 侧有限 enrich

## 2026-04-13

### 凌晨：交易复盘与数据积累方案 V1 交付
- 本轮不是继续扩交易逻辑，而是按“少动系统、先盘渠道、先做功课”的原则，正式交付一版《交易复盘与数据积累方案 V1》。
- 已盘清当前核心数据渠道：
  - `decision_records`
  - `trader_orders`
  - `trader_fills`
  - `trader_positions`
  - `position_close_events`
  - `trader_equity_snapshots`
  - strategy / trader runtime config
- 已形成当前建议的真相源分层：
  1. 决策真相源：`decision_records`
  2. 执行真相源：`trader_orders` + `trader_fills`
  3. 仓位主记录真相源：`trader_positions`
  4. 逐段退出真相源：`position_close_events`
  5. 权益时间序列真相源：`trader_equity_snapshots`
- 方案主结论：
  - 当前缺的不是“更多字段”，而是跨层连接能力
  - 下阶段最小增量应优先补：
    - decision ↔ position 连接键
    - close-event ↔ decision 连接键
    - 结构化环境快照（regime / gate / funding / volatility）
    - review 输出模型定义
- 已新增文档：`docs/TRADING_REVIEW_DATA_ACCUMULATION_PLAN_CN.md`
- 已同步更新：
  - `docs/DATA_MODEL_RELATIONS_CN.md`
  - `docs/TODO.md`

## 2026-04-12

### 晚间：Native Trailing 激活价 / 参数来源 / 执行语义再收口
- 前端与运行态已继续补齐 drawdown/native trailing 元信息：
  - `activation_price`
  - `planned_activation_price`
  - `callback_rate`
  - `activation_source`
  - `callback_source`
- 当前展示语义已明确区分：
  - `exchange`：交易所回读到的实际值
  - `request`：本地下单请求值 / 已确认请求值
  - `planned`：按规则推导的理论值
- 三家交易所当前收口状态：
  - **OKX**：可回读 `activePx` + `callbackRatio`
  - **Bitget**：可回读 `triggerPrice` + `rangeRate`
  - **Binance**：当前 SDK / open algo / single algo 查询链路能稳定拿到 activation，但 callback 未在读取模型中暴露，当前只能标记为 `request`，不能伪装成 `exchange`
- 当晚实盘争议点：ADAUSDT 的 trailing `activation=0.2413` 被用户指出与当时行情不符。
- 排查结论：
  1. 旧进程曾未及时切到最新后端二进制，已重新 `go build` 并重启 backend/frontend。
  2. 当前进一步明确执行原则：**native trailing 一旦成功挂上，不允许因为市场继续波动而重写 activePx；只有交易所上掉单 / 查不到 trailing 时才允许 re-arm。**
  3. 因此后续不再采用“activePx 与当前价偏离就刷新”的策略，避免因为重新挂单改变激活价而错过原本应捕获的 trailing 止盈目标。
- 相关提交：
  - `f94390e4` fix: use live market activation for native drawdown
  - `3b543824` feat: surface actual trailing parameters in runtime
  - `b76bcc28` feat: label trailing runtime parameter sources
  - `856abe26` fix: refresh stale native trailing activation prices （随后被执行语义否决，不再作为最终策略保留）

## 2026-04-11

### 保护单委托系统四大关键修复（交易所原生委托闭环）

#### 交付 1：修复委托单检测导致反复下单 (ee0191a4)
- `placeAndVerifyProtection()` 和 `placeAndVerifyLadderProtection()` 加入 500ms×3 轮延迟重试验证
- 交易所有传播延迟，刚下的委托单还没出现在 GetOpenOrders 返回结果里 → 验证失败 → 重复下单
- 价格容差从 0.2% 放宽到 0.5%（应对交易所精度截断）
- `protection_reconciler` 补单成功后加入 60 秒冷却期，防止和开仓链路叠加导致重复
- protection plan merge 改为 configured + AI 正确合并（additive merge）
- 新增 2 个传播延迟验证测试

#### 交付 2：修复全仓 + 分段 TP/SL 不能同时下单 (604b94fe)
- `BuildConfiguredProtectionPlan()` 改为先构建 ladder，再按覆盖方向抑制 Full
- `placeAndVerifyProtectionPlan()` ladder 分支条件从 `len > 1` 改为 `len > 0`（单步 ladder 不再被跳过）
- Full position SL/TP 只在 ladder 未覆盖的方向上生效，消除两模块互相踩踏
- 新增 ladder wins / Full+Ladder 混合方向共存测试

#### 交付 3：移动止盈止损实战走交易所原生委托 (0e7ffb8f)
- `partial_drawdown_native.go` Mode 从 `"drawdown_partial_candidate"` 改为 `"drawdown_partial_native"`
- `applyNativeTrailingDrawdown()` partial 路径：构建 plan → `placeAndVerifyProtectionPlanWithRetry()` → 标记 `native_partial_trailing_armed`
- 之前 partial drawdown 全走 local fallback（本地轮询平仓），现在走交易所原生委托
- 新增 mode 断言测试

#### 交付 4：利润保护 Break-even 生命周期完善 (b18a8569)
- `refreshBreakEvenFingerprint` 改为返回 `bool`，指示 fingerprint 是否变化
- reconciler 检测到 fingerprint 变化 + 之前已 armed → 主动 re-arm 新数量的 break-even 委托
- `applyBreakEvenStop` 加入 GetOpenOrders 验证循环（与交付 1 同模式），确认委托真正下到交易所
- 新增仓位数量变化触发 re-arm 测试

#### 全量回归
- `go test ./...`：全部通过
- 工作树干净
- 分支：`fox/project-takeover-baseline`

---

## 2026-04-09

### Replay / Paper-Trading 验证闭环深化（多场景 + 错误路径 + protection 集成测试）

#### 新增 replay 场景
- `scenario-eth-short-open-close.json`：ETH 做空开平仓，验证 short 侧 realized PnL 正确性
- `scenario-multi-step-progression.json`：多步价格推进，先做多后做空，验证双向连续交易 PnL 累计
- `scenario-negative-pnl-long.json`：做多亏损场景，验证负收益正确计算
- `scenario-open-with-protection.json`：开仓后持仓不平，验证保护单挂设正确
- `scenario-short-with-protection.json`：做空持仓不平，验证 short 侧保护单挂设
- `scenario-regime-trend-block.json`：趋势不对齐时 regime filter 阻断开仓

#### 新增 replay runner 测试
- 6 个新场景文件驱动测试
- 错误路径覆盖：nil scenario、empty symbol、invalid action、missing price、close without open
- 加载错误覆盖：invalid path、invalid JSON
- 校验错误覆盖：nil result、protection order mismatch、PnL mismatch

#### Protection 生命周期集成测试深化
- regime filter 测试补强：aligned long/short 通过、close 动作不被趋势检查阻断
- protection 执行测试补强：verify 失败、SL 设置失败、TP 设置失败、重试恢复
- ladder protection 测试：多阶 SL/TP 生命周期、partial close 数量校验
- 边界测试：nil plan、无 SL 无 TP plan
- protection plan builder 测试：long/short 方向价格计算、disabled 配置、零入场价

#### 前端 API 收束
- `AuthContext.tsx` login 从 raw `fetch` 迁移到 `httpClient.post`
- `AuthContext.tsx` logout 从 raw `fetch` 迁移到 `httpClient.post`
- `TerminalHero.tsx` klines 保留 raw `fetch`（设计决策：背景轮询不应触发全局 toast）

#### 基线验证
- `go test ./...`：通过
- `cd web && npm test`：通过（108 tests）
- `cd web && npm run build`：通过

#### 当前结论
- replay / paper-trading 验证闭环已从最小闭环深化到多场景、多侧、多步骤、错误路径全覆盖
- protection 生命周期集成测试已覆盖 full/ladder/regime/retry/failure 主要路径
- 前端 API 散落调用已基本收束完毕
- TODO 和 ACCEPTANCE 中的最后一个未完成项已标记完成

## 2026-04-07

### Replay / Paper-Trading 验证继续向“试运行语义”推进
- `trader/paper/trader.go` 已补最小 realized PnL 计算：
  - long: `(exit - entry) * qty`
  - short: `(entry - exit) * qty`
- `trader/replay/runner.go` 已增强：
  - `ScenarioExpected` / `Result` 新增 `realized_pnl`
  - replay 结果开始校验 closed pnl 汇总值
  - regime filter 只在 `open_long` / `open_short` 上生效，不再误拦 `close_long` / `close_short`
  - action 级价格覆盖会同步刷新 marketData，避免场景价格推进与过滤逻辑脱节
- `trader/replay/runner_test.go` 新增场景验证：
  - `TestRunScenarioCloseNotBlockedByRegimeFilter`
  - 确认趋势失配场景下平仓动作仍可执行，并正确落负收益
- `fixtures/replay/scenario-btc-long-open-close-smoke.json` 已补显式开仓价格与 `realized_pnl = 4` 期望。
- `trader/paper/trader_test.go` 已补 realized PnL 断言，short 平仓样例当前校验为 `20`。

### 本轮验证
- `go test ./trader/replay ./trader/paper`：通过
- `go test ./...`：通过
- 当前 replay / paper-trading 验证闭环已从“状态存在”继续推进到“收益结果可校验 + 平仓不被风控门禁误伤”。

## 2026-04-07

### Replay / Paper-Trading 验证闭环继续推进（open-close 收口）
- `trader/replay/runner.go` 已扩展：
  - `ScenarioAction` 新增 `price` 字段（支持按动作覆盖当前价格）
  - 支持 `close_long` / `close_short` 动作
  - `ScenarioExpected` 新增 `closed_pnl_count`
  - `Result` 新增 `ClosedPnLCount`
  - 运行结果新增 `GetClosedPnL` 校验
- `trader/paper/trader.go` 已增强：
  - 平仓后自动清理同 symbol 遗留保护单，避免出现“已平仓但保护单仍挂单”的不一致状态
- 新增场景：
  - `fixtures/replay/scenario-btc-long-open-close-smoke.json`
  - 覆盖 `open_long -> close_long` 最小闭环
  - 当前期望已更新为：平仓后 `protection_orders = 0`
- `trader/replay/runner_test.go` 新增：
  - `TestRunScenarioOpenCloseLifecycle`
  - 校验 replay 场景从开仓到平仓后的结果一致性
- `fixtures/replay/README.md` 已同步更新最小字段示例，纳入 close 动作与 `closed_pnl_count`。

### 基线复验
- `go test ./trader/replay ./trader/paper`：通过
- `go test ./...`：通过
- `cd web && npm test`：通过（108 tests）
- `cd web && npm run build`：通过

### 当前结论
- 验证闭环已从“开仓+挂保护”继续推进到“开仓→平仓→closed pnl 校验”最小闭环。
- 下一步应继续补多场景（多动作/多阶段价格推进/异常路径）并推进 simulation 维度验证。

## 2026-03-29

### Protection / 网络可靠性收口
- `trader/protection_execution.go` 为 protection setup 新增统一重试封装：
  - 手动 protection plan 路径接入重试
  - AI fallback protection 路径接入重试
- `trader/auto_trader_orders.go` 将 protection setup 失败后的处理统一为立即平仓保护，避免任何非 verify 类错误留下裸仓
- `trader/protection_execution_test.go` 新增“首轮失败、次轮恢复”的重试测试，覆盖最小恢复闭环

### 前端 Drawdown 多规则编辑闭环
- `web/src/components/strategy/ProtectionEditor.tsx` 已将 drawdown take profit 从单规则编辑升级为多规则编辑：
  - 支持新增规则
  - 支持逐条编辑
  - 支持删除规则
- 这使前端配置能力与后端 `strategy.protection.drawdown_take_profit.rules` 的多规则执行模型保持一致

### OKX / NOFXOS 网络层健壮性补强
- `trader/okx/trader.go`：
  - 引入独立 transport，避免依赖全局默认 transport
  - 为 EOF / timeout / reset 等瞬时网络错误补重试
- `provider/nofxos/client.go`：
  - 改为 trusted request 路径
  - 补 `ValidateURL` 校验
  - 限制 host 为 `nofxos.ai`

### 本轮验证
- `go test ./...`：通过
- `cd web && npm test`：通过（108 tests）
- `cd web && npm run build`：通过

### 当前结论
- 本轮不是新增大功能，而是把上一阶段 protection 主线继续做“可靠性收口”
- 当前工作树这批改动已具备提交条件
- 下一阶段应继续推进 replay / paper-trading / simulation 验证闭环，而不是重新扩散到新的高风险改造

## 2026-03-26

### Replay runner 深化到 protection / regime filter
- `trader/replay/runner.go` 已扩展支持：
  - `scenario.protection`
  - `scenario.regime_filter`
  - `funding_rates`
  - `blocked` 结果校验
- `trader/replay/runner_test.go` 已新增 blocked-by-funding 场景测试
- `fixtures/replay/scenario-btc-long-protection-smoke.json` 已升级为带 protection / regime filter 的 smoke 场景

- 新增 `trader/replay/runner.go`
- 新增 `trader/replay/runner_test.go`
- 当前已支持：
  - 读取 replay scenario
  - 驱动 paper trader 执行 open_long / open_short smoke 场景
  - 自动生成最小 protection orders
  - 对 expected 结果做校验
- 这意味着验证闭环已经从“有 paper trader + fixtures”继续推进到“可执行 replay smoke runner”

- 新增最小 simulated trader：`trader/paper/trader.go`
- 新增 paper trader 单测：`trader/paper/trader_test.go`
- 新增 replay fixtures 规范：`fixtures/replay/README.md`
- 新增首个 smoke 场景样例：`fixtures/replay/scenario-btc-long-protection-smoke.json`
- 当前验证闭环已从“测试骨架”推进到“可执行 paper-trading 最小实现 + fixtures 入口”

- 新增统一 fake trader harness：`trader/testutil/fake_trader.go`
- 新增 protection lifecycle 测试骨架：`trader/protection_lifecycle_test.go`
- 新增推进方案文档：`docs/REPLAY_PAPER_TRADING_PLAN_CN.md`
- 当前目标不再停留在“盘点缺失”，而是开始把 replay / paper-trading / simulation 的测试底座落地

### 当前状态
- protection / regime / AI protection 现在已不再是孤立功能点
- 已开始形成可复用的验证底座，便于后续继续补：
  - protection 生命周期集成测试
  - paper trader 最小实现
  - replay fixtures 规范

- 新增长期模板文档：
  - `docs/TASK_TEMPLATE.md`
  - `docs/ACCEPTANCE_TEMPLATE.md`
- 新增指标口径基线文档：
  - `docs/METRICS_BASELINE_CN.md`
- 将 `docs/TODO.md` 中“长期模板 / 收益指标 / 稳定性指标 / Phase 3”四项全部收口到完成态

### Protection Phase 3 最小闭环落地
- `store/strategy.go` 新增 `RegimeFilterConfig`
- `kernel/engine.go` 为 AI 决策结构补入 `protection_plan`
- 新增 `trader/protection_phase3.go`：
  - 实现 AI protection plan → ProtectionPlan 的最小转换
  - 实现 Regime Filter 的开仓前门禁
  - 基于 funding / ATR14 / 趋势同向 / regime level 做最小过滤
- `trader/protection_execution.go` 已接入 AI protection plan 优先落地路径
- `trader/auto_trader_orders.go` 已在 open_long / open_short 前接入 regime gate
- `api/strategy.go` 已补 `regime_filter` 配置校验
- `web/src/components/strategy/ProtectionEditor.tsx` 已补 Phase 3 前端配置入口并修正旧阶段文案
- `web/src/types/strategy.ts` 已同步新增 protection 类型

### 当前结论
- 原先剩余的三类事项（收口、定标、二阶段启动）已完成本轮收口
- Protection Phase 3 已达到“最小可交付闭环”状态
- 后续再做时，重点应转向：
  - 更强的多规则编辑体验
  - 更完整的专项测试
  - replay / paper trading / 仿真验证闭环

## 2026-03-21

### 项目接管启动
- 克隆仓库：`https://github.com/MAX-LIUS/nofxmax.git`
- 确认当前基线分支：`dev`
- 新建接管工作分支：`fox/project-takeover-baseline`
- 验证 GitHub CLI 登录可用
- 完成仓库目录首轮扫描
- 阅读关键入口文件：`README.md`、`main.go`、`config/config.go`、`api/server.go`
- 运行后端测试：`go test ./...`，通过
- 安装前端依赖：`cd web && npm install`
- 运行前端测试：`npm test`，通过（108 tests）
- 运行前端构建：`npm run build`，通过
- 建立中文接管文档骨架：
  - `docs/PROJECT_OVERVIEW_CN.md`
  - `docs/ARCHITECTURE_CN.md`
  - `docs/MODULE_INDEX_CN.md`

### 初步观察
- 系统以 Go 后端为主，React 前端为控制台
- `main.go` 启动链清晰，包含 config / crypto / store / manager / api / telegram
- 交易系统适配器较多，后续需要重点审计一致性与异常恢复机制
- 文档存在一定基础，但不够支撑系统化接管
- 用户优先级为：收益、稳定性
- 前端生产包较大（主 bundle 超 2MB），后续需要评估代码分割与性能优化

## 2026-03-23

### 接管阶段代码与文档推进
- 清理前端残留 `/api/admin-login` 死代码
- 清理前端残留 `/api/prompt-templates` 死代码
- 修复 public trader config 路径不一致
- 清理 admin mode / admin login 误导注释
- 完成首轮前后端接口对账，当前未发现新增明显失配

### 前端性能与交付优化
- 顶层页面改为懒加载
- 拆分 Trader Dashboard 重模块：
  - `ChartTabs`
  - `PositionHistory`
  - `GridRiskPanel`
- Vite 配置 `manualChunks`
- KaTeX 改为按需加载
- Recharts 入口组件改为按需加载
- 主入口共享包已从早期超大体积下降到约 `203k` 级别

### API 层收束推进
- 收束 `web/src/lib/config.ts`
- 收束 `web/src/lib/api/strategies.ts`
- 收束 `web/src/pages/StrategyMarketPage.tsx`
- 收束 `web/src/pages/SettingsPage.tsx`
- 收束 `web/src/lib/crypto.ts`
- 收束 `web/src/contexts/AuthContext.tsx` 中的 `resetPassword`

### 接管结项资产落仓
- 新增：`docs/FUXI_WORKFLOW_CN.md`
- 新增：`docs/PROJECT_HANDOVER_CLOSURE_CN.md`
- 新增：`docs/PROJECT_MEMORY_ARCHIVE_CN.md`
- 更新：`docs/ACCEPTANCE.md`
- 持续维护：`docs/FIX_CANDIDATES.md`

### 当前基线确认
- `go test ./...`：通过
- `cd web && npm test`：通过（108 tests）
- `cd web && npm run build`：通过
- 当前分支处于阶段性可交付状态，但接管工程整体仍未完全结项

## 2026-03-24

### 交易保护与盈利控制方案设计启动
- 基于对 `trader/auto_trader*`、`kernel/*`、交易所适配器、`store/strategy.go` 的风控规则审计
- 输出统一设计文档：`docs/TRADING_PROTECTION_UNIFIED_PLAN_CN.md`
- 明确后续实施遵循“配置 → AI/手动模式 → Planner → 交易所执行 → 保护单校验 → 失败补救 → 测试验证”的全链路思路
- 方案内已确定分阶段实施路线：
  - Phase 1：能力矩阵 + protection 配置结构 + 手动 Full TP/SL + 开仓后保护单闭环
  - Phase 2：Ladder TP/SL + Drawdown Take Profit + Break-even Stop
  - Phase 3：AI protection mode + Regime Filter

### Phase 1 首轮落地：protection 配置与开仓后闭环骨架
- `store/strategy.go` 新增统一 `ProtectionConfig` 及 Full TP/SL、Ladder TP/SL、Drawdown Take Profit、Break-even Stop 配置结构
- `api/strategy.go` 新增 protection 配置校验，避免非法阈值直接进入策略执行
- 新增 `trader/protection_capabilities.go`，建立交易所保护能力矩阵骨架
- 新增 `trader/protection_plan.go`，实现手动 Full TP/SL 保护计划生成
- 新增 `trader/protection_execution.go`，实现开仓后保护单挂单与最小校验逻辑
- 将 `trader/auto_trader_orders.go` 接入统一 protection 执行路径：
  - 开多 / 开空后优先按手动 protection plan 执行
  - 若未启用手动 protection，则回退使用 AI decision 自带的 SL/TP
  - 若保护单校验失败或交易所能力不满足要求，则触发立即平仓，避免裸仓保留
- 基线验证：`go test ./...` 通过

### Phase 1 前端接续：strategy protection 配置入口
- `web/src/types/strategy.ts` 补齐 `ProtectionConfig` 相关前端类型
- 新增 `web/src/components/strategy/ProtectionEditor.tsx`
- `web/src/pages/StrategyStudioPage.tsx` 已接入 protection 配置分区：
  - 在策略加载、创建、语言切换时补齐默认 protection 配置
  - 在 Strategy Studio 中新增 Protection / Profit Control 折叠区
  - 开放 Full TP/SL 手动配置入口
  - Ladder / Drawdown / Break-even 先以前端配置骨架形式暴露，为后续执行链预留
- 前端验证：`cd web && npm test`、`cd web && npm run build` 通过

### 接管收口推进：可信边界与四条核心链路
- 新增 `docs/SYSTEM_TRUST_BOUNDARY_CN.md`，明确当前系统的可信边界、风险边界与不可误判为已完成的事项
- 新增 `docs/SYSTEM_CHAINS_CLOSURE_CN.md`，收口启动链 / 决策链 / 交易链 / 风控链的当前结构与边界
- 补充首轮中文注释到关键入口与核心链路：
  - `main.go`
  - `api/server.go`
  - `manager/trader_manager.go`
  - `trader/auto_trader_loop.go`
  - `trader/auto_trader_risk.go`
- 基线验证：`go test ./...` 通过

### 接管收口推进：kernel 主入口注释与架构增强
- 补充 `kernel/engine.go`、`kernel/engine_position.go`、`kernel/prompt_builder.go` 首轮中文注释
- `docs/ARCHITECTURE_CN.md` 增补风控链视角，明确 risk_control / protection / kernel 校验 / trader 运行态保护之间的关系
- 基线验证：`go test ./...` 通过

### Protection Phase 2 收口：Break-even Stop / Ladder TP-SL 执行链完成
- `trader/auto_trader_risk.go` 已接入 Break-even Stop 运行态执行链：
  - 从 `strategy.protection.break_even_stop` 读取配置
  - 在达到浮盈阈值后自动撤换旧止损并设置保本止损
  - 已补专项测试覆盖取消旧止损 / 触发阈值 / 错误路径
- `trader/protection_plan.go` 已扩展支持手动 Ladder TP/SL protection plan：
  - 支持多阶 take-profit / stop-loss 价格换算
  - 支持分批 close ratio 累计裁剪到 100%
  - ladder manual 模式优先于 full TP/SL 回退路径
- `trader/protection_execution.go` 已接入 ladder protection 执行与校验：
  - 根据阶梯 close ratio 拆分保护单数量
  - 下发多阶 `SetStopLoss` / `SetTakeProfit`
  - 拉取 open orders 做逐阶验证
  - 若交易所能力矩阵不支持 partial close，则直接 fail-safe 阻断
- 新增测试：
  - `trader/protection_plan_test.go`
  - `trader/protection_execution_test.go`
  - `trader/auto_trader_risk_test.go`（break-even 补强）
- 当前结论：Protection Phase 2 的手动 ladder / drawdown / break-even 三条主线执行链均已落地
- 验证：
  - `go test ./...` 通过
  - `cd web && npm test` 通过（108 tests）
  - `cd web && npm run build` 通过

