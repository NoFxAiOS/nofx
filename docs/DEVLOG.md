
- 阶段切换：`Protection AI Workflow` 主线阶段性收口，后续主线转向执行层真实问题：Drawdown 多档委托生命周期 + Break-even 实盘委托可观测性。
- 2026-04-16：继续完成 protection 配置闭环核查，已确认当前链路分层如下：
  - Strategy Studio 保存使用 `PUT /api/strategies/:id`，保存后重新 `GET /api/strategies` 刷新编辑态；前端 `ie()` 仅做展示层默认值补齐。
  - Trader 运行时**不会读取 active strategy 作为实时配置源**；真正使用的是 trader 记录里的 `strategy_id`，由 `store.Trader().GetFullConfig()` 优先按该 ID 载入 strategy；只有 `strategy_id` 为空时才 fallback 到 active/default strategy。
  - 启动 trader 时会 `RemoveTrader` 后重新 `LoadUserTradersFromStore`，因此**重启后会拿到数据库里 strategy_id 对应的最新配置**。
  - 由此判断：若 UI 中“策略页保存后刷新正常”，但真实运行/某 trader 页面仍表现异常，下一层应优先排查 **trader 绑定的 strategy_id 不是当前编辑那份**，而不是继续怀疑 protection merge。
- 2026-04-16：补了一个 API 可观测性缺口修复：`handleGetTraderConfig` 现在会返回 `strategy_name`（此前只回 `strategy_id`，但前端查看 trader 配置弹窗会尝试展示 `strategy_name`）。这个缺口会放大“看起来没绑对策略/没保存成功”的错觉，现已补齐并通过 `go test ./api/...`。
