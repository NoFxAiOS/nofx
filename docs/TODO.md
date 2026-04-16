

- [x] 已落地长期任务“不断线”执行方案：新增 `docs/DURABLE_EXECUTION_WORKFLOW_CN.md`，把 TaskFlow × Agentic Coding 组合固化为 `Flow 卡片 → Contract → 最小改动 → 证据验证 → 文档记忆 → 提交` 的标准流程，用于解决模型波动、上下文漂移、会话中断后任务半途断开的老问题。
- [ ] 后续对 Drawdown / Break-even / protection 实盘问题，统一按 durable workflow 执行并在阶段结束时更新 flow 状态与证据链。
- protection 配置闭环补充：
  - [x] `PUT /api/strategies/:id` → `GET /api/strategies/:id` API 级回读验证已补齐，确认 Full/Ladder/fallback 深层字段不会因局部更新被冲掉
  - [x] 运行态配置来源已核清：Trader 优先读取自身 `strategy_id` 对应的 strategy；`active strategy` 只在 trader 未绑定 strategy_id 时作为 fallback
  - [x] `GET /api/trader/config/:id` 已补回 `strategy_name`，避免 trader 配置弹窗只拿到 `strategy_id` 导致“看起来没绑对策略”的误判
  - [x] 前端主要 trader 更新路径已核查：编辑 trader 与 dashboard 保存 AI 控制项时，都会显式回传 `strategy_id`
  - [x] 已修复旧版 protection value 结构兼容问题：旧数据中的 `{"enabled":...}` 现在会在反序列化时迁移为新结构 `mode/value`，避免策略页回填时误显示为默认 manual
  - [x] 已补 UI 状态摘要与提示，明确区分“执行开关 enabled”与“整体/子项 mode”，减少“AI 模式已保留但尚未启用执行”被误读成“回到手动”
  - [ ] 请 UI 复测：Full / Ladder / fallback 保存后刷新是否符合预期；重点观察 `enabled=false + mode=ai` 是否仍会被误解为 manual
