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

