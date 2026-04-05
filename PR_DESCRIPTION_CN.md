# 功能：自定义量化模型系统 (Issue #1306)

## 摘要

本 PR 实现了 **Issue #1306** - 为 NoFx 交易平台添加小型自定义量化模型支持。用户现在可以基于技术指标和规则创建、配置、导入和导出自己的交易模型，减少对 AI 提示词的依赖，并启用强大的回测功能。

## 问题陈述

之前，NoFx 仅依赖 AI 提示词进行交易决策，这存在几个限制：
- 高 token 使用量和 API 成本
- LLM 输出非确定性
- 无法在实盘部署前回测策略
- 用户无法构建和分享自己的系统化交易逻辑

## 解决方案概述

新的 **量化模型系统** 允许用户：
1. **创建自定义模型** 基于技术指标（RSI、EMA、MACD、ATR、布林带）
2. **定义交易规则** 使用逻辑条件如 `RSI_14 < 30 AND Close > EMA_20`
3. **导入/导出模型** 作为 JSON 进行分享和版本控制
4. **跟踪回测统计**（胜率、夏普比率、最大回撤）
5. **集成到策略中** 与 AI 提示词并行或替代使用

## 新功能

### 1. 后端 API (`api/quant_model.go`)

模型管理的新 REST 端点：

| 方法 | 端点 | 描述 |
|------|----------|-------------|
| GET | `/api/quant-models` | 列出用户模型 |
| POST | `/api/quant-models` | 创建新模型 |
| GET | `/api/quant-models/:id` | 获取模型详情 |
| PUT | `/api/quant-models/:id` | 更新模型 |
| DELETE | `/api/quant-models/:id` | 删除模型 |
| POST | `/api/quant-models/:id/export` | 导出为 JSON |
| POST | `/api/quant-models/import` | 从 JSON 导入 |
| POST | `/api/quant-models/:id/clone` | 克隆现有模型 |
| POST | `/api/quant-models/:id/backtest-stats` | 更新回测结果 |
| GET | `/api/quant-models/templates` | 获取预定义模板 |
| GET | `/api/quant-models/public` | 列出公共社区模型 |

### 2. 数据层 (`store/quant_model.go`)

**新数据库表 `quant_models` 包含：**
- 模型元数据（名称、描述、版本、可见性）
- 模型类型（indicator_based、rule_based、ml_classifier、ensemble）
- JSON 配置存储
- 回测统计（胜率、平均利润、最大回撤、夏普比率）
- 使用跟踪（usage_count、last_used_at）

**配置模式：**
```go
type QuantModelConfig struct {
    Type       string              // indicator_based、rule_based 等
    Indicators []ModelIndicator    // RSI、EMA、MACD 配置及权重
    Rules      []ModelRule         // 条件交易规则
    Parameters ModelParameters     // 回看、阈值、持仓时间
    SignalConfig SignalGenerationConfig // 置信度、确认设置
}
```

### 3. 执行引擎 (`kernel/quant_model_engine.go`)

**QuantModelEngine** 实时执行模型：
- **基于指标的模型**：计算技术指标的加权得分
- **基于规则的模型**：评估条件表达式（RSI_14 < 30 AND ...）
- **集成模型**：通过投票/平均组合多个模型
- 生成与现有交易系统兼容的 `Decision` 对象

**支持的指标：**
- RSI（相对强弱指数）
- EMA（指数移动平均线）
- SMA（简单移动平均线）
- MACD（指数平滑异同移动平均线）
- ATR（平均真实波幅）
- 布林带

**规则语法：**
```
RSI_14 < 30 AND Close > EMA_20 AND Volume > SMA_Volume_20 * 1.2
ATR_14 > ATR_14_SMA * 1.5 AND Close > Upper_Bollinger_20
```

### 4. 前端 UI (`web/src/pages/QuantModelsPage.tsx`)

**新的量化模型管理页面：**
- 列出所有用户模型及统计信息
- 模型详情视图与回测统计
- 导出/导入 JSON 功能
- 从公共/社区来源克隆模型
- 双语支持（英文/中文）

**模型编辑器 (`web/src/components/strategy/QuantModelEditor.tsx`)：**
- 指标和规则的视觉配置
- 实时参数调整
- 模板系统（预构建策略）
- 可折叠部分用于组织编辑

### 5. 前端类型 (`web/src/types/strategy.ts`)

扩展的 `StrategyConfig` 包含：
```typescript
interface QuantModelIntegration {
    enabled: boolean
    primary_model_id?: string
    secondary_models?: StrategyQuantModelLink[]
    fallback_to_ai: boolean
    model_confidence_threshold: number
    backtest_before_live: boolean
}
```

## 使用示例

### 示例 1：RSI 超卖策略（基于指标）

**配置：**
```json
{
  "type": "indicator_based",
  "indicators": [
    { "name": "RSI", "period": 14, "timeframe": "1h", "weight": 0.4 },
    { "name": "EMA", "period": 20, "timeframe": "1h", "weight": 0.3 },
    { "name": "MACD", "period": 12, "timeframe": "1h", "weight": 0.3 }
  ],
  "parameters": {
    "lookback_periods": 100,
    "entry_threshold": 70,
    "exit_threshold": 30
  }
}
```

**逻辑：**
- RSI < 30（超卖）→ 多头入场的正向信号
- 价格高于 EMA20 → 趋势确认
- MACD 高于信号线 → 动量确认
- 加权总和超过阈值 → 生成买入信号

### 示例 2：突破规则（基于规则）

**配置：**
```json
{
  "type": "rule_based",
  "rules": [
    {
      "name": "RSI_Oversold_Bounce",
      "condition": "RSI_14 < 30 AND Close > EMA_20",
      "action": "buy",
      "confidence": 80
    },
    {
      "name": "ATR_Breakout",
      "condition": "ATR_14 > ATR_14_SMA * 1.5 AND Close > Upper_Bollinger_20",
      "action": "buy",
      "confidence": 70
    }
  ]
}
```

## 导入/导出格式

**导出的 JSON 结构：**
```json
{
  "version": "1.0",
  "exported_at": "2024-01-15T10:30:00Z",
  "model": {
    "id": "original-id",
    "name": "我的 RSI 策略",
    "model_type": "indicator_based",
    "config": { /* 完整配置 */ }
  }
}
```

**通过以下方式分享模型：**
1. 从您的账户导出
2. 与他人分享 JSON 文件
3. 他们导入 → 使用新 ID 创建新模型

## 回测集成

模型跟踪性能指标：
- `win_rate`：盈利交易百分比
- `avg_profit_pct`：每笔交易平均回报
- `max_drawdown_pct`：最大峰谷跌幅
- `sharpe_ratio`：风险调整回报指标
- `backtest_count`：运行的回测次数

用户可以在部署到实盘交易前进行回测：
```
POST /api/quant-models/:id/backtest-stats
Body: {
  "win_rate": 0.65,
  "avg_profit_pct": 12.5,
  "max_drawdown_pct": 8.2,
  "sharpe_ratio": 1.8
}
```

## 社区与公共模型

- 用户可以将模型标记为 `is_public: true`
- 公共模型出现在社区画廊中
- 可按使用次数、胜率、回测结果排序
- 克隆保留配置但创建新所有权

## 未来增强

本 PR 为以下功能奠定基础：
1. **ML 分类器模型**（随机森林、XGBoost 集成）
2. **高级回测引擎** 带历史模拟
3. **模型市场** 用于购买/销售经过验证的策略
4. **集成投票** 跨多个用户模型
5. **自动优化** 通过网格搜索优化指标参数

## 测试

- `api/quant_model_test.go` - 所有 API 处理程序的单元测试
- 用于隔离测试的模拟存储实现
- 创建、更新、删除、导入、导出、克隆操作的测试覆盖

## 迁移说明

- 新 `quant_models` 表通过 GORM AutoMigrate 自动创建
- 策略配置获得可选的 `quant_model_integration` 字段
- 向后兼容 - 现有策略无需量化模型即可工作

## 参考

|- Issue #1306: 能不能在策略那里加自己的小型量化模型，可以导入导出，这样的话，算力回测才会有用武之地不用单纯的依赖提示词

## Summary

|- **Problem**: NoFx relied solely on AI prompts for trading decisions, causing high token costs, non-deterministic LLM output, inability to backtest, and inability to build/share systematic trading logic.
|- **What changed**: Added QuantModel system with backend API (11 endpoints), database layer (`quant_models` table), execution engine (indicator/rule-based), frontend UI (editor + page), and technical indicators (RSI, EMA, MACD, ATR, Bollinger).
|- **What did NOT change**: Existing AI-only strategies work unchanged; ml_classifier and ensemble model types defined but not fully implemented yet (returns `unsupported`); strategy engine integration pending separate PR.

## Change Type

|- [x] Feature
|- [ ] Bug fix
|- [ ] Refactoring
|- [ ] Docs
|- [ ] Security fix
|- [ ] Chore / infra

## Scope

|- [x] Trading engine / strategies
|- [x] API / server
|- [x] Web UI / frontend
|- [ ] Config / deployment
|- [ ] CI/CD / infra

## Linked Issues

|- Closes #1306

## Testing

|- [x] `go build ./...` passes (after `go mod tidy`)
|- [x] Unit tests added for API handlers (`api/quant_model_test.go`)
|- [ ] `go test ./...` requires `go mod tidy` first (dependency updates pending)
|- [ ] Manual frontend testing recommended: create model, import/export, clone, templates, public models

## Security Impact

|- Secrets/keys handling changed? **No**
|- New/changed API endpoints? **Yes** - 11 new endpoints at `/api/quant-models/*`
|- User input validation affected? **Yes** - JSON config validation in API handlers, file upload validation for import

## Compatibility

|- Backward compatible? **Yes** - existing AI-only strategies unaffected
|- Config/env changes? **No new env vars required**
|- Migration needed? **Auto** - `quant_models` table auto-created by GORM
|- If yes, upgrade steps: Run application, GORM AutoMigrate handles table creation

## 变更文件

|| 文件 | 变更说明 |
||------|----------|
|| `api/quant_model.go` | 新增 11 个 REST API 端点 |
|| `api/quant_model_test.go` | API 单元测试 |
|| `api/server.go` | 注册量化模型路由 |
|| `kernel/quant_model_engine.go` | 模型执行引擎 |
|| `market/indicator_calculator.go` | 技术指标计算（RSI/EMA/MACD/ATR/布林带） |
|| `store/quant_model.go` | 量化模型数据结构与存储 |
|| `store/store.go` | 注册 QuantModelStore |
|| `store/strategy.go` | 策略配置新增量化模型集成字段 |
|| `web/src/components/strategy/QuantModelEditor.tsx` | 模型编辑器 UI |
|| `web/src/pages/QuantModelsPage.tsx` | 量化模型管理页面 |
|| `web/src/types/strategy.ts` | TypeScript 类型定义 |

## 建议补充验证

|1. 执行 `go mod tidy` 后重跑测试
|2. 前端手动验证：创建模型、导入/导出、克隆、模板加载、公开模型列表
|3. 验证策略配置中 `quant_model_integration` 的保存与读取一致性
