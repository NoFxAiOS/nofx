# 交易保护与盈利控制统一方案设计 v1

> 状态：设计方案（批准启动版）
> 时间：2026-03-24
> 适用范围：`MAX-LIUS/nofxmax`
> 目标：把策略配置、AI 决策、交易所执行、保护单闭环校验、异常降级、验证测试打通为一套统一的交易保护与盈利控制体系。

---

## 1. 设计目标

本方案解决的不是“单个止盈止损功能”，而是以下整条链路的统一：

1. 前端/UI 可配置
2. 策略配置可持久化
3. AI 能理解并规范输出（若启用 AI 模式）
4. 执行层能按交易所能力正确下单
5. 开仓后保护单能闭环确认
6. 保护单缺失/失败时有明确补救策略
7. 持仓期内支持动态止盈/止损/保本止损/回撤止盈
8. 全流程具备测试、仿真、验收和灰度验证路径

本方案优先级遵循：

- **P0：先解决真实风控闭环问题**（尤其是“开仓成功但保护单未成功挂上”的裸仓风险）
- **P1：再提升盈利控制能力**（如 Break-even、Regime Filter）
- **P2：最后做 AI 深度联动增强**

---

## 2. 核心原则

### 2.1 逻辑必须打通，不能做孤立功能
任何止盈止损能力都必须同时覆盖：

- UI
- `store` 配置结构
- prompt / AI schema
- 交易所能力映射
- 执行逻辑
- 保护单闭环校验
- 异常降级
- 测试验证

### 2.2 盈亏统一按“币价变化百分比”表达
统一语义，避免混用杠杆倍数导致理解偏差：

- Long：
  - 止盈 = 价格上涨 x%
  - 止损 = 价格下跌 y%
- Short：
  - 止盈 = 价格下跌 x%
  - 止损 = 价格上涨 y%

即：

- 配置层用 **price move percent**
- 执行层将其换算为交易所需要的触发价/委托价
- 杠杆只影响资金收益率，不影响止盈止损触发价格语义

### 2.3 必须先建“交易所能力矩阵”
不同交易所对以下能力支持不一致：

- 原生 stop loss
- 原生 take profit
- reduce-only 条件单
- 分批 TP/SL
- 修改保护单
- OCO / Algo Orders
- 对 stop / tp 的区分能力

因此不能直接写死功能，必须先做能力抽象。

### 2.4 开仓后保护单必须闭环
核心要求：

- 开仓成功后，系统必须确认：
  - stop loss 是否真实存在
  - take profit 是否真实存在
  - 数量 / side / reduce-only / trigger 方向是否正确
- 任一失败：
  - 自动重试
  - 仍失败则平仓

目标是：

**要么保护单成功，要么仓位被撤销，不能长期裸仓。**

### 2.5 先可靠，再盈利增强
优先补：

1. 保护单闭环
2. 回撤止盈可配置化
3. 全仓 / 分批 TP-SL
4. 保本止损

之后再做：

5. Regime Filter
6. AI protection mode
7. 后验评分 / 收益增强

---

## 3. 当前系统现状与缺口

### 3.1 已有能力
当前系统已具备：

- 开仓后调用 `SetStopLoss` / `SetTakeProfit`
- `MaxPositions` / `MinPositionSize` / `PositionValueRatio` 等代码硬约束
- `safeMode`（AI 连续失败后禁止新开仓）
- `drawdown monitor`（盈利大于 5%，峰值回撤超过 40% 则强制平仓）
- 多交易所接口层（Binance / OKX / Gate / KuCoin / Hyperliquid 等）

### 3.2 当前关键缺口

#### 缺口 A：保护单失败后未形成闭环
当前问题：

- 开仓成功后
- 若 `SetStopLoss` / `SetTakeProfit` 失败
- 系统仅记录 warning
- 仓位仍保留

风险：

- 裸仓暴露
- 风险无法接受

#### 缺口 B：回撤止盈是硬编码单规则
当前逻辑（历史基线）：

- `profit > 5%`
- `drawdown >= 40%`
- 触发全平

问题：

- 无法配置
- 不能分批
- 粒度过粗

> 2026-03-26 更新：该缺口已完成主线修复并进入当前交付基线。
>
> 当前 Protection Phase 2 已落地能力：
> - Drawdown Take Profit：
>   - 从 `strategy.protection.drawdown_take_profit.rules` 读取规则
>   - 按 `poll_interval_seconds` 调整轮询周期
>   - 多规则匹配
>   - 按 `close_ratio_pct` 执行部分平仓 / 全平
> - Break-even Stop：
>   - 从 `strategy.protection.break_even_stop` 读取运行态配置
>   - 达到利润阈值后撤换旧止损并设置保本止损
> - Ladder TP/SL：
>   - 支持手动多阶 TP / SL protection plan 生成
>   - 支持按 close ratio 拆单执行
>   - 支持开仓后逐阶 open-order 校验
>
> 当前仍未包含部分：
> - AI protection mode
> - Regime Filter
> - 前端更强的多规则编辑体验
> - 更细粒度的执行去重 / 状态持久化 / 联动仲裁

#### 缺口 C：AI 与手动风控配置尚未统一
当前没有统一规则定义：

- 哪些保护参数由 AI 决定
- 哪些由手动配置强制决定
- AI 输出 schema 不足以表达复杂保护计划

#### 缺口 D：交易所能力差异未系统抽象
当前虽然各交易所有实现，但尚未统一建模：

- 能否原生支持分批 TP/SL
- 能否改单
- 能否精确区分 stop / tp
- 哪些场景必须降级为系统轮询平仓

#### 缺口 E：测试与验收路径不足
当前还没有一套专门面向：

- 保护单闭环
- partial close
- break-even
- drawdown partial exit
- exchange capability fallback

的完整测试矩阵。

---

## 4. 目标能力模型

本次方案将统一支持 4 大类保护机制：

### 4.1 全仓止盈止损（Full TP/SL）
用于简单模式：

- 全仓止盈
- 全仓止损
- 支持 `manual` / `ai` 二选一

### 4.2 分批止盈止损（Ladder TP/SL）
用于高级模式：

- 多组价格变化百分比
- 多组平仓比例
- 支持 partial close
- 支持 `manual` / `ai`

### 4.3 回撤止盈（Drawdown Take Profit）
用于利润保护：

- 维护峰值收益
- 当浮盈满足阈值且回撤满足阈值时
- 执行部分平仓/逐步落袋

### 4.4 保本止损（Break-even Stop）
用于盈利后锁定不亏：

- 达到一定利润门槛后
- 自动把止损移动到开仓价附近
- 保护盈利单不回撤成亏损

---

## 5. 配置模型设计

建议在 `store.StrategyConfig` 中新增统一保护配置模块，例如：

```json
{
  "protection": {
    "full_tp_sl": {
      "enabled": true,
      "mode": "manual",
      "take_profit": {
        "enabled": true,
        "price_move_pct": 8
      },
      "stop_loss": {
        "enabled": true,
        "price_move_pct": 3
      }
    },
    "ladder_tp_sl": {
      "enabled": false,
      "mode": "manual",
      "take_profit_enabled": true,
      "stop_loss_enabled": true,
      "rules": [
        {
          "take_profit_pct": 5,
          "take_profit_close_ratio_pct": 30,
          "stop_loss_pct": 2,
          "stop_loss_close_ratio_pct": 50
        }
      ]
    },
    "drawdown_take_profit": {
      "enabled": true,
      "rules": [
        {
          "min_profit_pct": 5,
          "max_drawdown_pct": 40,
          "close_ratio_pct": 100,
          "poll_interval_seconds": 60
        }
      ]
    },
    "break_even_stop": {
      "enabled": false,
      "trigger_mode": "profit_pct",
      "trigger_value": 3,
      "offset_pct": 0.1
    }
  }
}
```

### 5.1 Full TP/SL
字段：

- `enabled`
- `mode`: `manual | ai`
- `take_profit.enabled`
- `take_profit.price_move_pct`
- `stop_loss.enabled`
- `stop_loss.price_move_pct`

### 5.2 Ladder TP/SL
字段：

- `enabled`
- `mode`: `manual | ai`
- `take_profit_enabled`
- `stop_loss_enabled`
- `rules[]`
  - `take_profit_pct`
  - `take_profit_close_ratio_pct`
  - `stop_loss_pct`
  - `stop_loss_close_ratio_pct`

### 5.3 Drawdown Take Profit
字段：

- `enabled`
- `rules[]`
  - `min_profit_pct`
  - `max_drawdown_pct`
  - `close_ratio_pct`
  - `poll_interval_seconds`

### 5.4 Break-even Stop
字段：

- `enabled`
- `trigger_mode`: `profit_pct | r_multiple`
- `trigger_value`
- `offset_pct`

### 5.5 配置校验规则
必须加前后端双重校验：

- 百分比必须 > 0
- `close_ratio_pct` 在 `(0, 100]`
- 所有 partial close 的累计比例不能超过 100%
- `poll_interval_seconds` 不能低于安全下限（建议 >= 5s）
- Full TP/SL 与 Ladder TP/SL 若同时启用，需要定义优先级（建议：同类型只能启用一种）

---

## 6. UI 方案

UI 建议落在：

- `Strategy Studio`
- 风险控制区域新增“交易保护”模块

### 6.1 Full TP/SL UI
用户可配置：

- 开关
- 模式：AI / 手动
- 止盈开关 + 百分比
- 止损开关 + 百分比

### 6.2 Ladder TP/SL UI
用户可配置：

- 开关
- 模式：AI / 手动
- 是否启用分批止盈 / 分批止损
- 数组编辑器：
  - 止盈触发涨幅%
  - 止盈平仓比例%
  - 止损触发跌幅%
  - 止损平仓比例%

### 6.3 Drawdown Take Profit UI
用户可配置：

- 开关
- 数组编辑器：
  - 盈利阈值%
  - 峰值回撤%
  - 平仓比例%
  - 轮询周期 s

### 6.4 Break-even UI
用户可配置：

- 开关
- 触发模式（profit_pct / r_multiple）
- 触发值
- 偏移百分比

### 6.5 UI 额外要求
必须显示：

- 当前交易所支持情况
- 当前模式是否会降级为“系统轮询执行”
- 当前策略 protection mode（manual / ai）

---

## 7. AI 联动设计

### 7.1 模式优先级

#### Manual 模式
- AI 不得决定 TP/SL 数值
- AI 只决定：
  - action
  - symbol
  - leverage
  - position_size_usd
  - reasoning

系统根据策略配置自动生成保护计划。

#### AI 模式
- AI 需要输出 protection plan
- 但必须符合 schema 与系统约束

### 7.2 AI 输出 schema 扩展建议
建议扩展决策结构：

```json
{
  "symbol": "XLMUSDT",
  "action": "open_long",
  "leverage": 1,
  "position_size_usd": 50,
  "confidence": 82,
  "reasoning": "...",
  "protection_plan": {
    "mode": "full",
    "take_profit_pct": 8,
    "stop_loss_pct": 3,
    "ladder_rules": []
  }
}
```

### 7.3 Prompt 约束
系统级 prompt 必须明确：

#### 当 protection mode = manual
- AI 不允许擅自生成 TP/SL 数值
- 必须遵循系统预设保护策略

#### 当 protection mode = ai
- AI 必须输出完整且合法的 protection plan
- 若无法给出有效保护计划，优先输出 `wait`

### 7.4 决策校验
AI 输出后需增加 schema validator：

- 字段完整性
- 百分比合法性
- 分批累计比例合法性
- 与当前交易所能力是否兼容
- 与当前策略 protection mode 是否兼容

---

## 8. 交易所能力矩阵设计

建议新增统一 capability 抽象，例如：

```go
type ProtectionCapabilities struct {
    NativeStopLoss bool
    NativeTakeProfit bool
    NativePartialClose bool
    NativeReduceOnly bool
    CanAmendProtectionOrders bool
    CanDistinguishStopAndTP bool
    SupportsAlgoOrders bool
    SupportsOCO bool
}
```

### 8.1 按能力分层

#### A. 原生完整支持
优先使用交易所原生保护单：

- Binance（预期较强）
- OKX（需专项核验细节）
- Gate
- KuCoin

#### B. 部分支持
- 原生支持 full TP/SL
- 但不支持复杂 partial/modify
- 需要部分系统轮询补充

#### C. 不完全支持 / 行为有歧义
如：
- Hyperliquid 对 stop / tp 区分与取消逻辑存在特殊性

#### D. 基本不支持
- 只能系统轮询执行
- 或直接禁用某些 protection 选项

### 8.2 能力驱动的执行策略
系统必须自动根据 capability 选择：

- 原生保护单执行
- 混合模式
- 系统轮询平仓降级模式

---

## 9. Position Protection Planner（统一保护计划生成器）

建议新增一个统一规划层，而不是把逻辑散在 trader 和 AI 里。

### 9.1 输入

- 策略 protection config
- AI protection plan（若启用 AI 模式）
- entry price
- position side
- exchange capability
- current position quantity

### 9.2 输出

统一生成 `ProtectionPlan`：

- full take profit order(s)
- full stop loss order(s)
- ladder orders
- drawdown rules
- break-even trigger rules
- execution mode（native / fallback / hybrid）

### 9.3 作用
统一解决：

- full 与 ladder 冲突
- partial close 总量超限
- 交易所不支持能力时自动降级
- long / short 百分比到价格转换
- reduce-only / positionSide 方向统一

---

## 10. 开仓后保护单闭环确认流程

建议实现严格的 P0 流程：

### 10.1 开仓后流程
1. 主单成交
2. 读取实际持仓数量 / entry price
3. 通过 Planner 生成目标保护计划
4. 挂 stop / tp / ladder orders
5. 查询 open orders / algo orders
6. 校验订单存在性与正确性

### 10.2 校验内容
必须确认：

- stop loss 存在
- take profit 存在
- 数量正确
- side / positionSide 正确
- reduce-only 正确
- trigger 价格正确

### 10.3 失败处理
若校验失败：

1. 重试挂单（有限次）
2. 再次校验
3. 仍失败：
   - 记录风险事件
   - 立即平仓
   - 写 decision log / alert

### 10.4 核心目标
实现：

**要么保护单成功，要么仓位不保留。**

---

## 11. 回撤止盈（Drawdown Take Profit）设计

这是对当前硬编码逻辑的升级版。

### 11.1 当前已有逻辑
现有代码在：

- `trader/auto_trader_risk.go`

当前规则：

- 当前利润 > 5%
- 峰值回撤 >= 40%
- 触发全平

### 11.2 升级目标
改为可配置多组规则，支持 partial close。

### 11.3 执行逻辑
对每个持仓维护：

- 当前浮盈 %
- 历史峰值浮盈 %
- 当前回撤 %
- 已执行过的 drawdown rule 状态

当某条规则命中时：

- 平掉指定比例
- 记录 rule 已执行
- 避免重复执行同一档位

### 11.4 轮询周期
建议：

- 默认 30s / 60s
- 不建议无限缩短
- 最终按配置值与系统最小下限共同约束

---

## 12. 全仓 / 分批 TP-SL 设计

### 12.1 Full TP/SL

#### Manual
系统按 entry price 计算：

- Long TP = `entry * (1 + tp_pct)`
- Long SL = `entry * (1 - sl_pct)`
- Short 反向

#### AI
AI 输出百分比，系统校验后换算价格并执行。

### 12.2 Ladder TP/SL
每一组 rule 定义：

- 止盈触发价涨跌幅
- 止盈平仓比例
- 止损触发价涨跌幅
- 止损平仓比例

系统生成：

- 多个 reduce-only 条件单
- 若交易所不支持，则系统进入轮询执行模式

### 12.3 重要约束
- 分批总平仓比例不能 > 100%
- 同一 position 的 TP/SL 计划不能互相冲突
- 必须通过 Planner 合并和去重

---

## 13. Break-even Stop 设计

### 13.1 触发模式
支持：

- `profit_pct`
- `r_multiple`

### 13.2 行为
一旦触发：

- 计算新的 stop loss
- 调整到开仓价附近（可带微小 offset）

### 13.3 执行细节
- 若交易所支持改单：直接 amend
- 否则：
  - cancel old stop
  - create new stop
  - 再做闭环确认

---

## 14. Regime Filter 设计（P1）

### 14.1 目的
提升盈利能力和稳定性，不是生存底线功能。

### 14.2 初版建议
先做轻量 regime：

- 趋势
- 震荡
- 高波动
- 破坏期/异常期

### 14.3 输入来源
优先走系统量化指标，不依赖 AI：

- ATR / 波动率
- 趋势斜率
- 价格结构
- OI / NetFlow 辅助

### 14.4 输出作用
- 是否允许开仓
- 是否降低仓位
- 用 full TP 还是 ladder TP
- 调整 stop 宽度

---

## 15. 异常与降级策略

### 15.1 交易所不支持 protection 功能
处理：

- UI 提示为“系统执行模式”
- 下单后由系统轮询价格，命中阈值后下 reduce/close 单

### 15.2 保护单部分失败
处理：

- 重试
- 重试失败则平仓
- 记录风险事件

### 15.3 AI 超时/网络错误
处理：

- 只允许 close / hold
- 禁止 new open
- 必要时进入 safe mode

### 15.4 数据源缺失
处理：

- 优先换参考数据源
- 若执行必需数据缺失，则拒绝开仓
- 不允许在 protection 关键路径上“静默跳过”

---

## 16. 测试与验证方案

### 16.1 单元测试
覆盖：

- 配置解析与校验
- 百分比 → 价格转换（long / short）
- Planner 合并规则
- capability 驱动降级
- break-even 触发逻辑
- drawdown partial close 逻辑

### 16.2 集成测试
覆盖场景：

1. 开仓成功 + 保护单成功
2. 开仓成功 + 挂单失败 + 重试成功
3. 开仓成功 + 挂单失败 + 最终平仓
4. Full TP 命中
5. Full SL 命中
6. Ladder TP 命中
7. Ladder SL 命中
8. Drawdown rule 命中
9. Break-even 生效
10. 交易所 capability 不足时走 fallback

### 16.3 仿真/回放测试
建议加入：

- 历史 K 线回放
- 保护单触发回放
- partial close 资金曲线对比
- drawdown / Sharpe / 胜率 / PF 对比

### 16.4 实盘灰度顺序
建议：

1. 仅启用保护单闭环确认
2. 手动 full TP/SL
3. 手动 ladder TP/SL
4. drawdown take profit
5. break-even stop
6. AI mode protection
7. regime filter

---

## 17. 分阶段实施路线

### Phase 1（P0 最小闭环）
1. 交易所能力矩阵抽象
2. StrategyConfig protection 模块
3. UI 初版：手动 Full TP/SL
4. Planner 初版
5. 开仓后保护单闭环确认
6. 失败即平仓
7. 单元测试 + 集成测试

### Phase 2（P0 扩展）
8. 手动 Ladder TP/SL
9. 可配置 Drawdown Take Profit
10. Break-even Stop
11. 测试与灰度

### Phase 3（P1 增强）
12. Regime Filter
13. AI Protection Mode
14. AI Schema Validator
15. 回放验证

---

## 18. 当前最值得优先处理的事项

### P0-1：保护单闭环确认
原因：

- 当前真实裸仓风险最高
- 比盈利优化更优先

### P0-2：手动 Full TP/SL
原因：

- 先做最小可控版本
- 可验证性最好

### P0-3：Drawdown Take Profit 配置化
原因：

- 当前已有雏形
- 升级成本相对较低
- 对利润保护价值高

---

## 19. 当前暂不建议立刻做的事

### 19.1 先跳过 AI 模式复杂 protection 输出
原因：

- 如果 capability / planner / validation 没建好
- AI mode 只会放大复杂度和 bug 面

### 19.2 先不要把所有交易所一次性做满
建议顺序：

- 先选能力较完整的交易所打样
- 再推广到其它交易所

推荐首批重点：

- Binance
- OKX
- Gate
- KuCoin

---

## 20. 最终建议

建议按以下顺序执行：

1. 先把 **保护单闭环确认 + 手动 Full TP/SL** 打通
2. 再做 **手动 Ladder TP/SL + Drawdown Take Profit + Break-even**
3. 最后再做 **AI protection mode + Regime Filter**

本方案的关键不是“功能多”，而是：

**配置 → AI/手动模式 → Planner → 交易所执行 → 保护单校验 → 失败补救 → 测试验证**

这条链必须全通。
