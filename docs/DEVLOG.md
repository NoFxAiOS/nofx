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
