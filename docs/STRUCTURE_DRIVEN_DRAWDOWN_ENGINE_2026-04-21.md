# Structure-driven Drawdown Engine Plan（2026-04-21）

## 目标重定义

本方案取代“固定比例 drawdown 表”与单纯 runner 补丁思路。

用户真正要求的是：

> partial close / drawdown 本身必须根据利润上升、主周期与相邻周期结构、支撑阻力、斐波那契、swing 高低点等，动态调整回撤点位和减仓比例。

也就是说，drawdown 不应该是：

```text
profit >= X% -> allow Y% drawdown -> close Z%
```

而应该是：

```text
价格/利润推进 -> 识别当前结构阶段 -> 动态决定回撤容忍、减仓比例、runner 保留、runner stop/target 来源 -> 执行
```

Runner 不是顶层模型；runner 是结构驱动利润管理在特定阶段下的输出结果。

---

## 当前问题

现有模型过于机械：

- drawdown rule 是固定 `min_profit_pct / max_drawdown_pct / close_ratio_pct`；
- native trailing 只是按固定利润门槛 armed；
- 一旦第一档 partial close 触发，剩余仓位仍可能被机械 break-even 快速清掉；
- 系统没有根据结构阶段动态调整回撤容忍和减仓比例；
- 因此 TAO 出现：70% native partial trailing 先打掉大头，剩余又被 break-even stop 清掉，实际接近全退。

这不符合“留一部分去博更大收益”的语义。

---

## 新目标：结构驱动利润管理

新的 drawdown engine 应该做三件事：

1. **识别结构阶段**
2. **动态生成利润保护策略**
3. **把策略交给执行层落地**

---

## 一、结构阶段识别（Structure Stage）

输入：

- 当前利润 `current_pnl_pct`
- 峰值利润 `peak_pnl_pct`
- 当前价 / entry / invalidation / first target
- primary timeframe
- lower / higher timeframe
- support / resistance
- swing highs / lows
- fibonacci levels / extension
- volatility / ATR（可选）

输出结构阶段：

```go
type DrawdownStructureStage string

const (
    StageEarlyProfit          DrawdownStructureStage = "early_profit"
    StageTrendContinuation    DrawdownStructureStage = "trend_continuation"
    StageNearPrimaryTarget    DrawdownStructureStage = "near_primary_target"
    StagePostBreakoutRunner   DrawdownStructureStage = "post_breakout_runner"
    StageExtensionExhaustion  DrawdownStructureStage = "extension_exhaustion"
    StageFailedContinuation   DrawdownStructureStage = "failed_continuation"
)
```

### 阶段含义

#### `early_profit`

刚脱离成本区，利润不厚。  
不应重仓减仓，也不应把 stop 贴太紧。

#### `trend_continuation`

趋势推进中，仍未接近主周期强结构目标。  
可轻度保护利润，但应保留较大 runner。

#### `near_primary_target`

接近主周期 resistance / swing high / fib target。  
应考虑兑现较多利润，并收紧回撤容忍。

#### `post_breakout_runner`

突破关键结构位并完成确认。  
应保留 runner，stop 可上移到结构翻转位。

#### `extension_exhaustion`

接近 fib extension / 多周期衰竭 / 结构阻力密集。  
应大幅降低暴露，runner 也应收紧。

#### `failed_continuation`

延续失败，结构破坏。  
应优先退出，而不是继续等待固定 drawdown。

---

## 二、动态利润保护策略（Dynamic Protection Decision）

结构阶段输出后，engine 生成本轮动态保护决策：

```go
type DrawdownProtectionDecision struct {
    Stage              DrawdownStructureStage `json:"stage"`
    MaxDrawdownPct     float64                `json:"max_drawdown_pct"`
    ReduceRatioPct     float64                `json:"reduce_ratio_pct"`
    RunnerKeepPct      float64                `json:"runner_keep_pct"`
    RunnerStopPrice    float64                `json:"runner_stop_price"`
    RunnerStopSource   string                 `json:"runner_stop_source"`
    RunnerTargetPrice  float64                `json:"runner_target_price"`
    RunnerTargetSource string                 `json:"runner_target_source"`
    Reason             string                 `json:"reason"`
}
```

### 关键语义

- `ReduceRatioPct` 不再是静态配置，而是结构阶段的输出；
- `MaxDrawdownPct` 不再只是 rule 写死，而是可由阶段动态调整；
- `RunnerKeepPct` 是明确策略结果；
- runner stop / target 必须尽量引用结构来源。

---

## 三、建议的动态决策规则（第一版 deterministic）

第一版不做复杂预测，只做 deterministic 结构规则。

### Long 场景

#### early_profit

条件示例：

- `current_pnl_pct < first_gate`
- 或尚未接近 resistance/fib/swing target

策略：

- `reduce_ratio_pct = 0`
- `runner_keep_pct = 100`
- `max_drawdown_pct` 较宽
- 不机械 BE 清 runner

#### trend_continuation

条件示例：

- 当前利润上升
- 未接近主周期 resistance / fib extension
- lower timeframe 未破结构

策略：

- 轻度减仓或不减仓：`0-30%`
- 保留大部分 runner：`70-100%`
- runner stop 放在 adjacent support / swing low / fib retracement 附近

#### near_primary_target

条件示例：

- 当前价接近主周期 resistance / swing high / fib level

策略：

- 减仓增加：`40-70%`
- runner 保留：`30-60%`
- max drawdown 收紧

#### post_breakout_runner

条件示例：

- price 突破 resistance
- 相邻周期确认 retest

策略：

- 不急于继续减仓
- 保留 runner
- stop 上移到 breakout/retest support
- target 看更高 fib extension / swing high

#### extension_exhaustion

条件示例：

- 到达 fib extension
- higher timeframe resistance 密集
- lower timeframe momentum 衰竭

策略：

- 大幅减仓：`70-90%`
- runner 很小：`10-30%`
- stop 显著收紧

---

## 四、Break-even 与 Runner 的关系

这是 TAO 暴露出的关键问题。

旧语义：

```text
profit reaches BE trigger -> set BE stop for remaining position
```

新语义：

```text
如果仓位已进入 runner mode，break-even 不应默认成为 runner 的 primary stop。
```

### 建议规则

- 若未进入 runner mode：BE 可正常保护；
- 若已进入 runner mode：
  - runner stop 优先来自结构；
  - BE 只能作为 fallback；
  - 若结构 stop 低于 BE 但结构仍有效，可以允许 runner 承受更大回撤；
  - 不应因为轻微回撤到 BE 直接清掉全部 runner。

新增状态：

```go
BreakEvenSuppressedByRunner bool
```

---

## 五、执行层职责

执行层不定义业务语义，只执行上层决策。

可用执行形式：

- native trailing
- managed partial close
- full stop
- structure stop
- fallback stop

但不允许交易所 native trailing 反向决定业务含义。

---

## 六、配置模型建议

保留旧字段兼容：

```json
{
  "min_profit_pct": 0.7,
  "max_drawdown_pct": 55,
  "close_ratio_pct": 70
}
```

新增结构驱动字段：

```json
{
  "mode": "structure_dynamic",
  "runner_enabled": true,
  "min_runner_keep_pct": 20,
  "max_first_reduce_pct": 60,
  "runner_stop_mode": "structure",
  "runner_target_mode": "structure",
  "break_even_runner_policy": "fallback_only"
}
```

### 字段说明

- `mode`: `fixed_rules | structure_dynamic`
- `runner_enabled`: 是否启用 runner 语义
- `min_runner_keep_pct`: 最少保留 runner 比例
- `max_first_reduce_pct`: 第一阶段最大减仓比例
- `runner_stop_mode`: `break_even | structure | trailing_structure`
- `runner_target_mode`: `fixed | structure`
- `break_even_runner_policy`: `primary | fallback_only | disabled_for_runner`

---

## 七、运行态状态

需要记录：

```go
type DrawdownRunnerState struct {
    Active                       bool
    Symbol                       string
    Side                         string
    PositionFingerprint           string
    Stage                        DrawdownStructureStage
    RunnerKeepPct                float64
    LastReduceRatioPct           float64
    RunnerStopPrice              float64
    RunnerStopSource             string
    RunnerTargetPrice            float64
    RunnerTargetSource           string
    BreakEvenSuppressedByRunner  bool
}
```

---

## 八、UI / 审计字段

实时持仓和历史审计应显示：

- structure stage
- dynamic max drawdown pct
- reduce ratio pct
- runner keep pct
- runner stop price/source
- runner target price/source
- whether BE is suppressed by runner
- why the stage was chosen

---

## 九、第一阶段落地范围

只做最小闭环：

1. 新增结构动态模式配置字段；
2. 新增 deterministic stage evaluator；
3. 在 drawdown monitor 中生成 dynamic decision；
4. partial close 后进入 runner state；
5. runner active 时 BE 不再默认清 runner；
6. runtime/UI 显示 runner state。

暂不做复杂预测或外部模型。

---

## 十、验收标准

完成后，TAO 类场景应满足：

- 第一段利润兑现后，剩余仓位进入 runner state；
- 如果价格轻微回撤到机械 BE，但主/邻周期结构仍有效，runner 不应被直接全清；
- runner stop/target 必须能解释：来自 support / resistance / swing / fib；
- UI 能明确显示当前 drawdown stage 与 runner 语义；
- 若结构失效，则 runner 可以退出，但必须显示结构失效原因。
