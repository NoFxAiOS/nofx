# NOFX 指标口径基线（收益 / 稳定性）

> 目的：把后续开发、验收、复盘要用的核心指标口径统一下来，避免“能看懂数据但口径不一致”。

## 1. 收益指标口径

### 1.1 账户级
- **Total Equity**：账户总权益 = 可用余额 + 未实现盈亏
- **Total PnL %**：`(当前总权益 - 初始资金) / 初始资金 × 100`
- **Daily PnL**：自然日维度已实现 + 未实现变动，用于日内风控观察

### 1.2 交易级
- **Realized PnL**：平仓后实际盈亏，包含手续费
- **Trade PnL %**：`(出场价 - 进场价) / 进场价 × 杠杆 × 100`（空头反向）
- **Risk/Reward Ratio**：基于 stop_loss / take_profit 的目标盈亏比

### 1.3 保护链相关
- **Peak Unrealized PnL %**：持仓生命周期内达到过的最高浮盈比例
- **Drawdown from Peak %**：从峰值浮盈回撤的比例
- **Break-even Trigger Hit**：达到保本移动阈值的次数/命中率
- **Protection Coverage Rate**：开仓后成功完成保护单挂设并校验通过的比例

## 2. 稳定性指标口径

### 2.1 执行可靠性
- **Order Success Rate**：下单成功次数 / 下单尝试次数
- **Protection Verification Success Rate**：保护单校验成功次数 / 保护单尝试次数
- **Emergency Close on Protection Failure**：保护失败后紧急平仓成功率

### 2.2 状态一致性
- **Position/Order Consistency**：持仓状态、开放订单、数据库记录三者是否一致
- **Decision/Execution Consistency**：AI 决策动作与最终执行动作是否一致
- **PnL Consistency**：前端展示、数据库记录、交易所返回是否在可接受误差内一致

### 2.3 恢复能力
- **AI Failure Recovery**：AI 连续失败后 safe mode 触发与恢复情况
- **State Recovery Time**：异常后恢复到可继续决策 / 可继续同步的时间
- **Data Freshness Guard Hit**：行情陈旧保护命中次数

## 3. 当前建议门禁
- 基线门禁：`go test ./...`、`cd web && npm test`、`cd web && npm run build`
- 新增功能验收至少补：
  - 影响链路说明
  - 指标口径是否受影响
  - 是否需要新增专项测试

## 4. 当前结论
- 收益口径：已形成账户级 / 交易级 / 保护链三级口径
- 稳定性口径：已形成执行可靠性 / 状态一致性 / 恢复能力三级口径
- 后续如进入 replay / paper trading 阶段，应直接复用本文件作为统一统计基线
