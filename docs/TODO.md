
- 主线已切换：从 `Protection AI Workflow` 收口转向执行层问题排查：
  - [ ] Drawdown 多档委托生命周期：确认是否做到“新单成功确认后再撤旧单”，并验证是否存在保护空窗
  - [ ] Break-even 实盘委托可观测性与成功率：定位为何真实环境中几乎未观察到成功 break-even 委托
- protection 配置闭环补充：
  - [x] `PUT /api/strategies/:id` → `GET /api/strategies/:id` API 级回读验证已补齐，确认 Full/Ladder/fallback 深层字段不会因局部更新被冲掉
  - [x] 运行态配置来源已核清：Trader 优先读取自身 `strategy_id` 对应的 strategy；`active strategy` 只在 trader 未绑定 strategy_id 时作为 fallback
  - [x] `GET /api/trader/config/:id` 已补回 `strategy_name`，避免 trader 配置弹窗只拿到 `strategy_id` 导致“看起来没绑对策略”的误判
  - [x] 前端主要 trader 更新路径已核查：编辑 trader 与 dashboard 保存 AI 控制项时，都会显式回传 `strategy_id`
  - [ ] 下一步若 UI/实盘仍异常，优先核查具体 trader 当前绑定的 `strategy_id` 是否就是策略页正在编辑的那份；再看是否存在页面展示与 trader 绑定对象不一致
