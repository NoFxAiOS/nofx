# 结构驱动 Runner 模型改造方案（仅聚焦 drawdown / partial close）

_时间：2026-04-21_

## 目标

把当前 drawdown 从“固定利润阈值 + 固定 close ratio”模型，升级为**结构驱动的 runner 模型**，只解决这一件事：

1. 第一阶段：兑现一部分利润；
2. 第二阶段：明确保留 runner；
3. runner 的 stop 不直接等同于机械 break-even；
4. runner 的 target 参考主周期 resistance / fib extension / swing high；
5. 避免因为利润轻微回撤把剩余 runner 提前踢出。

不在本轮同时扩散到其他 unrelated 问题（如 attribution、XAU dust residue、通用 UI 美化），只围绕 drawdown / runner 语义。

---

## 一、现状问题

当前实现的核心问题不是“不会 partial close”，而是 partial close 的语义过于机械：

- drawdown rules 用固定 `close_ratio_pct`；
- 一旦满足利润/回撤阈值，就按当前规则直接 close 对应比例；
- 剩余仓位继续受 break-even / stop 约束，容易在轻微回撤中被整体打掉；
- 这导致系统虽然 technically 做了“partial close”，但结果常常接近“分步全退”，没有真正保留 runner 去博更大收益。

TAO 样本就暴露了这个问题：

- 70% native partial trailing 先打掉大头；
- 剩余部分又被 break-even stop 吃掉；
- 最终效果并不是“留 runner”，而是接近整仓退出。

---

## 二、目标语义（新的业务语义）

### 1. 第一阶段：兑现一部分

当利润达到第一阶段，并且主/邻周期结构支持“先锁一部分利润”时：

- 只兑现一部分，例如 30% / 50% / 70%；
- 但这一阶段必须明确声明：**保留 runner 百分比**。

核心语义不再是：

- `close_ratio_pct = 70`

而是更接近：

- `runner_keep_pct = 30`
- 或 `reduce_to_runner_pct = 30`

这样策略表达会更贴近人的交易语言：

> 先锁 70%，但明确保留 30% runner。

---

### 2. 第二阶段：runner 独立管理

Runner 不是“剩余仓位默认继续用 break-even 管着”，而应进入独立语义：

- runner stop 参考结构失效位；
- runner target 参考更高一级结构目标；
- runner 可以继续上移保护，但不能过于机械。

Runner 的 stop 来源优先级建议：

1. 主周期最近支撑/阻力翻转位；
2. 相邻周期回踩确认位；
3. fib 回撤关键位；
4. swing low / swing high；
5. 最后才是机械 break-even。

也就是说：

> break-even 对“第一阶段已锁利润后剩余 runner”的默认优先级应下降。

---

### 3. Runner 的目标不是原 first_target

一旦第一阶段部分止盈后，runner 的目标应允许升级到：

- 主周期 resistance；
- fib extension；
- swing high / swing low；
- 更高时间框架结构目标。

这意味着第一笔单在结构上至少应该具备两个层次的目标：

- first_target：用于第一阶段兑现；
- runner_target：用于保留仓位继续博大级别走势。

---

## 三、配置模型改造建议

当前 drawdown rule:

```json
{
  "min_profit_pct": 0.7,
  "max_drawdown_pct": 55,
  "close_ratio_pct": 70
}
```

建议逐步升级为支持 runner 语义的新结构（可以兼容旧字段）：

```json
{
  "stage_name": "lock_first_profit",
  "min_profit_pct": 0.7,
  "max_drawdown_pct": 55,
  "close_ratio_pct": 70,
  "runner_keep_pct": 30,
  "runner_stop_mode": "structure",
  "runner_target_mode": "structure",
  "runner_target_source": "primary_resistance",
  "runner_stop_source": "adjacent_support_flip"
}
```

或者更清晰地以“reduce-to”表达：

```json
{
  "stage_name": "lock_first_profit",
  "min_profit_pct": 0.7,
  "max_drawdown_pct": 55,
  "reduce_to_position_pct": 30,
  "runner_stop_mode": "structure",
  "runner_target_mode": "structure"
}
```

### 推荐字段

- `stage_name`
- `min_profit_pct`
- `max_drawdown_pct`
- `close_ratio_pct`（兼容旧版）
- `reduce_to_position_pct`（新版优先）
- `runner_keep_pct`
- `runner_stop_mode`: `break_even | structure | trailing_structure`
- `runner_stop_source`: `support | resistance | swing | fib | mixed`
- `runner_target_mode`: `fixed | structure`
- `runner_target_source`: `primary_resistance | primary_support | fib_extension | swing_high | swing_low`

---

## 四、执行语义改造建议

### Phase A：最小可落地版本（优先）

不直接做完整“动态结构追踪引擎”，先做最小有用版本：

#### A1. 部分止盈后记录 runner 状态

新增持仓级 runtime state：

- 是否已进入 runner 模式
- 已执行到哪个 profit stage
- runner 剩余比例
- runner stop source
- runner target source

#### A2. 第一阶段 partial close 后，禁止 break-even 立即把 runner 当成整仓清掉

规则：

- 如果 position 已进入 runner mode，且 runner_stop_mode = structure，
  则 break-even 不应直接覆盖 runner 的全部剩余仓位；
- break-even 只能：
  - 作用于未进入 runner 语义前的仓位；
  - 或作为 fallback，而不是 primary stop。

#### A3. Runner stop 先用“最近结构位”近似

第一版不必做复杂结构引擎，先直接使用开仓审计里已经有的：

- support / resistance
- swing highs / lows
- fibonacci
- invalidation / first_target linkage

例如 long runner：

- 若有比 break-even 更合理的 support / fib / swing low，则优先用它；
- 若没有，再回退到 break-even。

#### A4. Runner target 先从 entry review summary 提取

第一版可优先读取：

- resistance
- swing highs
- fibonacci extension-like levels

在 UI 上显示：

- `first_target`
- `runner_target`
- `runner_stop_source`

---

### Phase B：结构驱动增强版

在第一版稳定后，再升级：

#### B1. 运行中重新评估结构

不是只用开仓时结构，而是：

- 根据主周期 / 相邻周期的最新市场结构重新调整 runner stop；
- 但要有 hysteresis / cooldown，避免频繁抖动。

#### B2. Drawdown stage 和 runner stage 解耦

当前 drawdown 更像“触发即 close”。
后续应改成：

- `profit stage`：盈利到达哪个层级；
- `runner policy`：这一层级 runner 怎么管；
- `execution form`：native trailing / managed stop / structure stop。

#### B3. 原生 trailing 只作为执行形式，不是业务语义本身

即：

- 上层定义“保留 runner + 结构 stop + 结构 target”；
- 下层可选择用 native trailing 执行其中一部分；
- 但不应该由交易所 native trailing 反过来定义业务语义。

---

## 五、UI/审计需要同步补的字段

为了让后续每一笔都能验收，至少需要在 runtime/history 里增加：

- `profit_stage_name`
- `runner_mode_active`
- `runner_keep_pct`
- `runner_stop_price`
- `runner_stop_source`
- `runner_target_price`
- `runner_target_source`
- `break_even_suppressed_by_runner`（是否因 runner 语义而抑制机械 BE）

这样你才能一眼看到：

> 这笔单现在是不是已经从“普通仓位”转成“runner 仓位”。

---

## 六、实现优先级（只做这一件事）

### P1：先改语义，不急着追求复杂

优先做：

1. drawdown rule 支持 runner 字段；
2. 部分止盈后记录 runner state；
3. runner mode 下，BE 不直接清空 runner；
4. runner stop / target 先从已有结构审计字段选取；
5. UI/runtime 显示 runner state。

### P2：再做更动态的结构更新

等 P1 稳定后，再做：

- 根据最新主/邻周期结构动态调整 runner stop/target。

---

## 七、验收标准

这次改完后，至少要满足：

1. 第一阶段 partial close 后，剩余仓位进入明确 runner 模式；
2. 剩余 runner 不会因为机械 BE 在轻微回撤中立即全清；
3. runner stop / target 能在 UI/审计里明确显示来源（support / resistance / swing / fib）；
4. TAO 类样本不再出现“先 70% partial，再剩余立即被 BE 全吃掉”的近似整仓退出效果；
5. 仍然保持 RR 和保护一致性，不引入无保护空窗。
