# 模型波动与任务中断容灾方案（2026-04-17）

> 目标：减少因模型稳定性波动、超时、空响应、会话中断导致的任务半途断开，提升一次性交付体验。

## 1. 问题定义

当前影响用户体验的不是单一 bug，而是下面几类执行层失稳：

1. 模型无输出 / 输出卡死
2. 单次长会话因上下文或响应波动中断
3. 子任务未拆分，导致主会话一旦受阻就整体停摆
4. 阶段成果只存在聊天上下文，没有落盘
5. 失败后立即回来问用户，而不是先走内部 fallback

## 2. 新默认策略

后续 coding / 复杂任务默认采用以下容灾顺序：

### Level 1：流程级容灾（默认）
优先靠流程，而不是先赌模型：

- 任务先拆成 Flow 卡片
- 先锁 Contract
- 最小改动
- 每个阶段写回 docs / memory
- 主会话只做 orchestration，长耗时/只读调查交给 subagent

作用：
即使一次模型响应断掉，也不会丢整个任务状态。

### Level 2：子任务级容灾（默认）
默认把以下工作交给 subagent：

- 仓库普查
- 指定模块阅读
- 测试验证
- 文档差异梳理
- 长耗时分析

作用：
主会话负责收口，子任务失败只影响局部，不影响总交付。

### Level 3：模型级容灾（应启用）
当当前模型出现明显波动时，不先回来打断用户，而是优先采用：

1. 同任务重试一次（仅当失败信号像瞬时抖动）
2. 切到已配置 fallback 模型继续
3. 把大任务拆成更小的 subagent 任务，降低单次响应压力

当前主模型配置已具备 fallback：
- primary: `lt-openai-0.2/gpt-5.4`
- fallbacks:
  - `lt/claude-opus-4-6`
  - `lt-haiku/claude-haiku-4-5`
  - `google/gemini-2.5-pro`

执行原则：
- 不因一次 provider 波动就立刻把任务还给用户
- 优先内部 failover，再给最终整合结果

### Level 4：证据级容灾（默认）
每个关键阶段都至少保留一种落盘证据：

- docs/DEVLOG.md
- docs/TODO.md
- repo workflow docs
- workspace memory/YYYY-MM-DD.md
- `~/agentic-coding/*`

作用：
即使会话中断，后续也能从文件恢复，而不是重新摸索。

## 3. 用户体验策略

### 3.1 少打断
默认不因为以下原因来回问用户：
- 需要继续读代码
- 需要再跑测试
- 需要起一个只读 subagent
- 需要查本地文档
- 需要在 fallback 模型间切换

### 3.2 真正该问时才问
只在这些情况中断用户：
- 高风险外部动作
- 不可逆操作
- 影响运行中服务/agent 的重启或 reload
- 明确产品方向分叉
- 缺少用户掌握的机器外信息/凭据

### 3.3 以阶段结果代替过程确认
默认汇报：
- 已完成什么
- 已验证什么
- 剩余风险是什么
- 下一步是什么

而不是每一步都先问可不可以继续。

## 4. 已落地能力

### 4.1 本地 agentic-coding 持久体系
已创建：
- `~/agentic-coding/memory.md`
- `~/agentic-coding/contracts.md`
- `~/agentic-coding/evidence.md`
- `~/agentic-coding/handoffs.md`

用途：
- 持久保存 coding 执行纪律
- 保存 contract / evidence / handoff 模板
- 让后续会话更少退化回“反复问用户”模式

### 4.2 原生多 agent / subagent 已可用
已实测通过：
- spawn
- completion + announce
- timeout
- kill
- steer
- cleanup: delete

这让“主会话编排 + 子会话干活 + 结果回传”的容灾链路成立。

## 5. 后续默认执行方式

后续复杂任务默认按以下顺序执行：

1. 主会话建立 Contract
2. 能拆出去的调查/验证工作交给 subagent
3. 当前模型异常时优先 fallback，不立刻中断用户
4. 阶段成果持续写入 docs / memory / agentic-coding
5. 最终一次性交付结果包

## 6. 结论

提升用户体验的关键，不是单纯换一个更强模型，而是：

1. 可恢复工作流
2. 子任务隔离
3. 模型 fallback
4. 文件化证据与记忆
5. 少打断、多收口

后续默认以这套方案执行。
