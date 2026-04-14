# Protection Test-Run Fixture

文件：`docs/fixtures/protection-test-run-fixture.json`

## 目的
用于在 StrategyStudio 的 **AI Test Run** 面板里验证当前 protection 改造是否真正影响 AI 输出质量，重点观察：

1. `open_long/open_short` 是否开始输出 `protection_plan`
2. `protection_plan.mode` 是否更稳定地在 `full` / `ladder` 之间选择
3. `close_long/close_short/hold/wait` 是否不再乱带 `protection_plan`
4. `parsed_decisions` 与 `parse_error` 是否健康

## 推荐验证步骤

1. 打开 StrategyStudio
2. 复制该 fixture 中的 `config` 到当前策略配置（或人工对齐关键 protection 字段）
3. 选择一个可用 AI 模型
4. 使用 `balanced` variant 先跑一轮
5. 重点查看右侧 **Parsed Decisions** 卡片

## 重点观察项

### 场景 A：close / hold / wait
预期：
- 不要出现 `protection_plan`
- reasoning 可以解释为何不输出 protection

### 场景 B：open + full
预期：
- `protection_plan.mode = full`
- 有 `take_profit_pct / stop_loss_pct`
- 不应再附带 `ladder_rules`

### 场景 C：open + ladder
预期：
- `protection_plan.mode = ladder`
- 有 `ladder_rules`
- 不应再混入多余的 flat full TP/SL 语义

## 判定标准

### 好结果
- `parsed_decisions` 非空
- `parse_error` 为空
- open/close/wait 的 protection 语义明显更干净
- full / ladder 选择有明显理由，不是随机

### 差结果
- `parse_error` 非空
- close/hold/wait 仍带 protection_plan
- open 动作同时输出 full + ladder 混乱语义
- protection_plan 经常缺关键字段

## 说明
- fallback max loss 仍是 **manual config guardrail**，不期待 AI 在 `protection_plan` 中发明这条兜底保护。
- 该 fixture 主要用于验证 **AI 输出质量**，不是执行层测试。
