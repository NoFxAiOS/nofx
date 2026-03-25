# NOFX 开发日志

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

### 主线盘点推进：测试网 / Mock / Replay 支撑现状审计
- 新增 `docs/TESTNET_MOCK_REPLAY_AUDIT_CN.md`
- 对当前仓库的 testnet / mock / replay 支撑进行代码级盘点，结论为：
  - testnet：部分支持（Hyperliquid / Lighter / exchange store 已存在 testnet 开关与路径）
  - mock：已有较多模块级 mock / httptest 基础
  - replay：研究文档层面有概念，但工程交付层面基本未形成
- 结论：当前具备开发级测试支撑基础，但还不具备完整主线交付所需的 replay / paper-trading / 仿真验证体系

