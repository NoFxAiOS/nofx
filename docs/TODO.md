

- [x] 已明确“一次交付目标”：后续 coding 任务默认自主推进到可交付再汇报，不再把普通推进步骤、一般阻塞、模型瞬时波动反复抛回给用户。
- [x] 在当前 `nofxmax` 主线上按一次交付模式继续：已完成对 Drawdown / Break-even / protection 真实执行层问题的一轮集中收口，并形成交付总结 `docs/PROTECTION_EXECUTION_DELIVERY_2026-04-20.md`。
- [ ] 基于当前交付结果，进入下一轮新任务：真实持仓验收 / 保护摘要可视化 / fixture 产物取舍。
- protection 配置闭环补充：
  - [x] `PUT /api/strategies/:id` → `GET /api/strategies/:id` API 级回读验证已补齐，确认 Full/Ladder/fallback 深层字段不会因局部更新被冲掉
  - [x] 运行态配置来源已核清：Trader 优先读取自身 `strategy_id` 对应的 strategy；`active strategy` 只在 trader 未绑定 strategy_id 时作为 fallback
  - [x] `GET /api/trader/config/:id` 已补回 `strategy_name`，避免 trader 配置弹窗只拿到 `strategy_id` 导致“看起来没绑对策略”的误判
  - [x] 前端主要 trader 更新路径已核查：编辑 trader 与 dashboard 保存 AI 控制项时，都会显式回传 `strategy_id`
  - [x] 已修复旧版 protection value 结构兼容问题：旧数据中的 `{"enabled":...}` 现在会在反序列化时迁移为新结构 `mode/value`，避免策略页回填时误显示为默认 manual
  - [x] 已补 UI 状态摘要与提示，明确区分“执行开关 enabled”与“整体/子项 mode”，减少“AI 模式已保留但尚未启用执行”被误读成“回到手动”
  - [ ] Protection AI mode 真实环境差异继续深挖：前端保存 payload 与 API legacy upgrade 测试均已锁住，但真实 strategy raw JSON 仍出现旧 shape；需继续抓真实 PUT incoming/merged config 日志，定位未覆盖路径
- 执行层实盘主线（恢复优先级）:
  - [ ] Drawdown 多档委托生命周期：确认是否做到“新单确认成功后再撤旧单”，并验证是否存在保护空窗 / 重复单 / 孤儿单清理不彻底
  - [ ] Break-even 实盘委托可观测性：补强日志/状态面，明确 break-even 何时触发、是否下单、是否被交易所拒绝、是否被其他保护链抑制
