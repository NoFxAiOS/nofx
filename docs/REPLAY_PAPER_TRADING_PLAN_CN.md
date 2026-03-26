# NOFX Replay / Paper-Trading 推进方案（第一版）

> 目标：把当前“部分具备但不完整”的 testnet / mock / replay 状态，推进成可持续演进的验证闭环。

## 1. 当前推进目标
本轮先不做大而全 replay 引擎，而是先落三层基础：

1. **统一 fake trader harness**
2. **protection 生命周期集成测试骨架**
3. **paper-trading / replay 的目录与执行模型约束**

## 2. 本轮已落地基础
- 新增：`trader/testutil/fake_trader.go`
- 新增：`trader/paper/trader.go`
- 新增：`fixtures/replay/README.md`
- 新增：`fixtures/replay/scenario-btc-long-protection-smoke.json`
- 作用：
  - 统一复用假交易执行器
  - 提供最小 paper trader 实现
  - 捕获 open orders / protection apply / close 行为
  - 为 drawdown / break-even / ladder / regime filter / AI protection 提供共享测试底座
  - 为后续 replay fixture 和 runner 约定统一入口

## 3. 后续执行块

### Block A：Protection 生命周期集成测试
目标：验证开仓后保护单挂设、校验、失败兜底、持仓期保护触发。

优先场景：
- manual full TP/SL 成功
- AI protection full 成功
- ladder partial-close 成功
- protection verify 失败 → emergency close
- drawdown trigger → partial / full close
- break-even trigger → cancel old SL + set new SL
- regime filter block open

建议文件：
- `trader/protection_lifecycle_test.go`

### Block B：Paper-Trading 模式骨架
目标：提供不触达真实交易所的主线执行模式。

最小设计：
- 复用 Trader interface
- 用 fake trader / simulated trader 替代真实交易所 trader
- 允许：
  - 开平仓
  - stop / tp / ladder 记录
  - 持仓与订单状态推进
- 暂不追求撮合精度，只先保证主链路验证可重复

建议目录：
- `trader/paper/`
- `fixtures/replay/`

### Block C：Replay 数据与驱动模型
目标：建立后续回放验证的最小规范。

当前已落地：
- `trader/replay/runner.go`
- `trader/replay/runner_test.go`
- 能力：读取 scenario → 驱动 paper trader → 输出结果 → 做 expected 校验

建议数据结构：
- market candles
- funding snapshots
- oi snapshots
- expected decisions (optional)
- expected protection events

建议流程：
1. 输入历史切片
2. 推进 market state
3. 调用 strategy / decision / execution
4. 记录 protection / close / pnl / consistency 事件
5. 输出验收报告

## 4. 当前建议顺序
1. 先补 `protection_lifecycle_test.go`
2. 再补 `paper trader` 最小实现
3. 最后补 `fixtures/replay/README.md` + 样例数据格式

## 5. 完成定义
进入“验证闭环已启动”状态，至少满足：
- 有统一 fake trader harness
- 有 protection 生命周期集成测试
- 有 replay / paper-trading 目录规范与首轮方案文档

当前状态：
- [x] 统一 fake trader harness
- [x] protection lifecycle test 骨架
- [x] paper trader 最小实现
- [x] replay fixtures 规范
- [x] replay runner / scenario executor 最小实现
