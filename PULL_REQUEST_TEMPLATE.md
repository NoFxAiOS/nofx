# 功能：自定义量化模型系统 (Issue #1306)

## Summary

- **Problem**: NoFx 平台仅依赖 AI 提示词进行交易决策，存在高 token 成本、非确定性输出、无法回测验证策略等问题。用户无法构建和分享自己的系统化量化交易模型。
- **What changed**: 新增完整的量化模型系统，包括后端 API、数据库存储、执行引擎和前端 UI。支持基于技术指标和交易规则的模型创建、导入导出、回测统计跟踪。
- **What did NOT change (scope boundary)**: 未修改现有 AI 提示词系统；未添加 ML 分类器模型（为未来扩展预留）；未集成实时回测引擎（仅提供统计存储）。

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
- Related #量化模型, #回测, #策略优化

## Testing

What you verified and how:

- [x] `go build ./...` passes
- [ ] `go test ./...` passes (新增 `api/quant_model_test.go` 单元测试)
- [x] Manual testing done (本地验证 API 端点和前端组件渲染)

## Security Impact

- **Secrets/keys handling changed?** (`No`)
- **New/changed API endpoints?** (`Yes` - 新增 11 个 `/api/quant-models/*` 端点)
- **User input validation affected?** (`Yes` - JSON 导入验证和规则表达式解析)

## Compatibility

- **Backward compatible?** (`Yes`)
- **Config/env changes?** (`No`)
- **Migration needed?** (`No` - 新表通过 GORM AutoMigrate 自动创建)
- **If yes, upgrade steps:** N/A

---

## 详细描述

### 新增文件

**后端 (Go):**
| 文件 | 描述 |
|------|------|
| `store/quant_model.go` | QuantModel 数据模型、配置结构体、CRUD 操作 |
| `api/quant_model.go` | 11 个 REST API 端点实现 |
| `api/quant_model_test.go` | API 处理程序单元测试 |
| `kernel/quant_model_engine.go` | 模型执行引擎（指标型/规则型）|
| `market/indicator_calculator.go` | 技术指标计算（RSI, EMA, MACD, ATR, Bollinger）|

**前端 (React/TypeScript):**
| 文件 | 描述 |
|------|------|
| `web/src/types/strategy.ts` | QuantModel TypeScript 类型定义 |
| `web/src/components/strategy/QuantModelEditor.tsx` | 模型编辑器组件 |
| `web/src/pages/QuantModelsPage.tsx` | 量化模型管理页面 |

**修改文件:**
- `api/server.go` - 注册 11 个新端点
- `store/store.go` - 添加 QuantModelStore 到 Store 结构体
- `store/strategy.go` - StrategyConfig 添加 QuantModelIntegration 字段

### API 端点列表

```
GET    /api/quant-models              # 列出用户模型
POST   /api/quant-models              # 创建新模型
GET    /api/quant-models/:id          # 获取模型详情
PUT    /api/quant-models/:id          # 更新模型
DELETE /api/quant-models/:id          # 删除模型
POST   /api/quant-models/:id/export   # 导出为 JSON
POST   /api/quant-models/import       # 从 JSON 导入
POST   /api/quant-models/:id/clone    # 克隆模型
POST   /api/quant-models/:id/backtest-stats  # 更新回测统计
GET    /api/quant-models/templates    # 获取预定义模板
GET    /api/quant-models/public       # 列出公共模型
```

### 数据库 Schema

```sql
CREATE TABLE quant_models (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    description TEXT,
    model_type VARCHAR,  -- indicator_based, rule_based, ml_classifier, ensemble
    version VARCHAR DEFAULT '1.0',
    is_public BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    config TEXT,  -- JSON 配置
    backtest_count INTEGER DEFAULT 0,
    win_rate REAL,
    avg_profit_pct REAL,
    max_drawdown_pct REAL,
    sharpe_ratio REAL,
    usage_count INTEGER DEFAULT 0,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### 支持的模型类型

1. **indicator_based** - 基于技术指标加权评分
   - RSI, EMA, MACD, ATR, Bollinger Bands, SMA
   - 可配置周期、时间框架、权重

2. **rule_based** - 基于逻辑规则条件
   - 支持规则语法: `RSI_14 < 30 AND Close > EMA_20`
   - 支持 AND/OR 组合、优先级排序

3. **ensemble** (预留) - 多模型集成投票

4. **ml_classifier** (预留) - 机器学习分类器

### 导入/导出格式

```json
{
  "version": "1.0",
  "exported_at": "2024-01-15T10:30:00Z",
  "model": {
    "id": "original-id",
    "name": "RSI 超卖策略",
    "description": "基于 RSI 和 EMA 的反弹策略",
    "model_type": "indicator_based",
    "version": "1.0",
    "config": {
      "type": "indicator_based",
      "indicators": [
        { "name": "RSI", "period": 14, "timeframe": "1h", "weight": 0.5 },
        { "name": "EMA", "period": 20, "timeframe": "1h", "weight": 0.5 }
      ],
      "parameters": {
        "lookback_periods": 100,
        "entry_threshold": 70,
        "exit_threshold": 30,
        "max_position_hold_time": 48,
        "min_position_hold_time": 4,
        "max_daily_trades": 3
      },
      "signal_config": {
        "signal_type": "discrete",
        "min_confidence": 65,
        "require_confirmation": true,
        "confirmation_delay": 1
      }
    }
  }
}
```

### 回测统计

模型支持跟踪以下性能指标：
- `win_rate`: 胜率 (0-1)
- `avg_profit_pct`: 平均收益率
- `max_drawdown_pct`: 最大回撤百分比
- `sharpe_ratio`: 夏普比率
- `backtest_count`: 回测次数
- `usage_count`: 实际使用次数

更新端点: `POST /api/quant-models/:id/backtest-stats`

### 前端界面

**量化模型管理页面** (`/quant-models`):
- 模型列表卡片（显示名称、类型、胜率、回测次数）
- 导入/导出按钮
- 详情视图（配置预览、统计信息）

**模型编辑器**:
- 指标配置（添加/删除/调整参数）
- 规则编辑器（条件表达式输入）
- 参数设置（阈值、持仓时间、交易限制）
- 信号生成配置
- 模板选择（预置策略）

### 双语支持

完整支持中英文切换：
- 所有 UI 标签、提示信息、错误消息
- 模型类型名称（"指标型"/"规则型"）
- 操作按钮（"创建"/"导入"/"导出"）

---

## 参考

- Issue #1306: 能不能在策略那里加自己的小型量化模型，可以导入导出，这样的话，算力回测才会有用武之地不用单纯的依赖提示词

## Co-Authored-By

Oz <oz-agent@warp.dev>