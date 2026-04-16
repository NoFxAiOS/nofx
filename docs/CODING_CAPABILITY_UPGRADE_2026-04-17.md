# 编程能力提升清单（2026-04-17）

> 目标：减少“拿到任务后反复回来问”的低价值打断，把当前 OpenClaw 编程协作能力升级到更自主、更连续、更可交付的状态。

## 已完成的能力补强

### 1. 持续推进工作流已固化
- 已有 `docs/DURABLE_EXECUTION_WORKFLOW_CN.md`
- 当前新增“编程任务自治升级”规则：
  - 先做再问
  - 只有在高风险/不可逆/外部动作/方向分叉时再问
  - 默认按 Contract → 最小改动 → 验证 → 文档记忆 → 提交

### 2. 原生多 agent / subagent 基础能力已打通
已实测通过：
- 原生多 agent 骨架：`work / qa / docs`
- subagent: spawn
- subagent: completion + announce
- subagent: timeout
- subagent: kill
- subagent: steer
- subagent: cleanup: delete
- subagents list / kill / steer

这意味着后续 coding 任务可以默认拆成：
- 主会话：目标、优先级、收口
- subagent：只读调查、长耗时分析、测试与验证

### 3. 关键阻塞已定位并跨过
- ACP 不是当前首选：未完整配置，不适合先依赖
- 原生 subagent 一度卡在 `streamTo` / allowlist，现已打通主链

## 现在就要执行的新默认策略

### A. 少问用户，多交付阶段结果
默认行为改为：
1. 自己先读代码 / 跑测试 / 起 subagent / 查文档
2. 给阶段性结果，而不是每步都回来问
3. 确实需要用户决策时，提供明确的两三个选项和利弊

### B. 把子任务外包给 subagent
适合默认拆给 subagent 的工作：
- 仓库结构普查
- 指定目录/模块代码阅读
- 测试用例与 fixture 证据采集
- 文档对齐与差异梳理
- 定向验证与回归检查

### C. 编程任务固定采用 contract-first
每个任务开始前，先内部锁定：
- Objective
- Acceptance
- Non-goals
- Constraints

没有这四个，不直接大改。

## 仍建议后续补强的点

### 1. 补本地 agentic-coding 持久记忆
当前 `~/agentic-coding/` 不存在。
建议后续建立本地持久目录，用来保存：
- coding 偏好
- contract 模板
- 证据模板
- handoff 模板

这样下次不会每次重新组织执行纪律。

### 2. 继续补高阶 subagent 回归
尚未逐项验证：
- auto-archive
- info/log
- thread-bound session
- nested/orchestrator
- fanout 并行编排

### 3. 未来再补 ACP harness
ACP（codex/claude/gemini）这条链还没配完，当前不作为默认依赖。
等原生 subagent 稳定投入使用后，再考虑补 ACP agent runtime。

## 结论

当前最有效的“编程能力提升”不是换个空泛 skill 名字，而是三件已经落地的事：
1. durable workflow
2. contract-first 执行纪律
3. 原生 subagent 协作链路

后续真正的提升方向是：
- 少打断
- 多交付
- 用 subagent 分担长耗时工作
- 把阶段性结果和证据带回来
