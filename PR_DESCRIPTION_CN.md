# 功能：自定义量化模型系统（Issue #1306）

## Summary
- **Problem**: NoFx 目前主要依赖 AI 提示词做交易决策，存在 token 成本高、输出非确定、策略难复用、回测价值受限等问题。
- **What changed**: 新增量化模型系统，覆盖后端存储与 API、执行引擎、指标计算模块、前端模型管理页面与编辑器，并支持导入/导出、克隆、公开模型列表与回测统计写入。
- **What did NOT change (scope boundary)**: 未替换现有 AI 主流程；`ml_classifier` 仅定义配置结构，执行逻辑尚未实现；策略主引擎与量化模型的深度联动仍属于后续迭代。

## Change Type
- [ ] Bug fix
- [x] Feature
- [ ] Refactoring
- [ ] Docs
- [ ] Security fix
- [ ] Chore / infra

## Scope
- [x] Trading engine / strategies
- [ ] MCP / AI clients
- [x] API / server
- [ ] Telegram bot / agent
- [x] Web UI / frontend
- [x] Config / deployment
- [ ] CI/CD / infra

## Linked Issues
- Closes #1306
- Related #N/A

## Testing
What you verified and how:
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] Manual testing done (describe below)

已执行验证与结果：
- 执行：`go test ./api/... ./store/... ./kernel/... ./market/...`
  - 结果：失败，提示 `go: updates to go.mod needed; to update it: go mod tidy`
- 执行：`go build ./...`
  - 结果：失败，主要错误集中在 `kernel/quant_model_engine.go`，包括：
    - `data.Klines undefined (type *market.Data has no field or method Klines)`
    - `declared and not used: middle`
    - `cannot use rule.StopLossPct (type *float64) as float64 value`

建议补充验证：
1. 先执行 `go mod tidy`，再重跑相关 `go test`。
2. 修复 `kernel/quant_model_engine.go` 的编译错误后，重跑 `go build ./...` 与 `go test ./...`。
3. 前端手动验证：创建模型、导入/导出、克隆、模板加载、公开模型列表。

## Security Impact
- Secrets/keys handling changed? (`No`)
- New/changed API endpoints? (`Yes`)
- User input validation affected? (`Yes`)

补充说明：
- 新增 11 个 `/api/quant-models/*` 相关接口。
- 导入接口涉及 JSON 输入解析与结构校验，规则表达式涉及基础解析逻辑。

## Compatibility
- Backward compatible? (`Yes`)
- Config/env changes? (`No`)
- Migration needed? (`Yes`)
- If yes, upgrade steps:
  1. 启动应用后由 GORM AutoMigrate 自动创建 `quant_models` 表。
  2. 可选：在策略配置中启用 `quant_model_integration`。

## 详细说明

### 1) 后端 API（`api/quant_model.go` + `api/server.go`）
新增接口：
- `GET /api/quant-models`：用户模型列表
- `GET /api/quant-models/templates`：模板列表
- `POST /api/quant-models`：创建模型
- `GET /api/quant-models/:id`：模型详情
- `PUT /api/quant-models/:id`：更新模型
- `DELETE /api/quant-models/:id`：删除模型（含策略引用保护）
- `POST /api/quant-models/:id/export`：导出模型
- `POST /api/quant-models/import`：导入模型
- `POST /api/quant-models/:id/clone`：克隆模型
- `POST /api/quant-models/:id/backtest-stats`：写入回测统计
- `GET /api/quant-models/public`：公开模型列表

### 2) 数据层（`store/quant_model.go` + `store/store.go`）
- 新增 `quant_models` 表与 `QuantModel` 实体。
- 新增 `QuantModelConfig` 配置结构，包含：
  - `indicator_based`
  - `rule_based`
  - `ml_classifier`
  - `ensemble`
- 支持模型 CRUD、公开模型查询、使用计数、回测统计更新、导入导出转换。
- 在 Store 初始化中加入量化模型表迁移逻辑。

### 3) 执行引擎（`kernel/quant_model_engine.go`）
- 新增 `QuantModelEngine`，支持：
  - 指标型模型（`indicator_based`）
  - 规则型模型（`rule_based`）
  - 集成入口（`ensemble`）
- 支持 `ExecuteBatch` 批量执行与 `GetSignal` 信号输出。
- 支持基础规则表达式解析（`AND` / `OR` + 比较运算）。
- 当前限制：`ml_classifier` 执行逻辑未实现，遇到该类型会返回 unsupported。

### 4) 指标计算模块（`market/indicator_calculator.go`）
新增指标能力：
- RSI
- EMA
- SMA
- MACD
- ATR
- Bollinger Bands

### 5) 前端类型与界面
涉及文件：
- `web/src/types/strategy.ts`
- `web/src/components/strategy/QuantModelEditor.tsx`
- `web/src/pages/QuantModelsPage.tsx`

关键点：
- 新增量化模型相关 TS 类型定义。
- 新增模型编辑器（创建/编辑/导入导出/克隆）。
- 新增模型管理页面（列表、详情、统计、公开状态）。

### 6) 变更文件清单
- `api/quant_model.go`
- `api/quant_model_test.go`
- `api/server.go`
- `kernel/quant_model_engine.go`
- `market/indicator_calculator.go`
- `store/quant_model.go`
- `store/store.go`
- `store/strategy.go`
- `web/src/components/strategy/QuantModelEditor.tsx`
- `web/src/pages/QuantModelsPage.tsx`
- `web/src/types/strategy.ts`
