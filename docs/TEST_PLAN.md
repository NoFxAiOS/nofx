# NOFX 测试计划（交付版）

## 目标
建立一个可以支撑接管交付与后续深度开发的测试基线，优先保证稳定性、保护链路可靠性，再支持收益相关优化验证。

## 分层

### 1. 后端单元 / 模块测试
当前状态：`go test ./...` 可通过。

已重点覆盖：
- trader 核心逻辑
- protection phase 2 关键执行链
- kernel prompt / schema / analysis
- exchange sync
- telegram agent

本轮新增重点：
- Drawdown Take Profit 规则匹配与轮询间隔
- Break-even Stop 运行态执行与错误路径
- Ladder TP/SL protection plan 生成
- Ladder protection 多阶下单与 open-order 校验

### 2. 前端测试
当前状态：
- `cd web && npm test` 可通过
- `cd web && npm run build` 可通过

本轮重点验证：
- 前端 API 收束后回归未破坏基础页面与认证逻辑
- Strategy Studio / Wallet Config / Grid Risk 相关调用收束后仍可构建

### 3. 集成测试
目标：
- API + store
- TraderManager + store
- 决策链关键路径
- 交易同步关键路径
- protection lifecycle 关键路径

当前状态：
- 已有部分 trader / exchange / sync 集成测试基础
- 尚未形成完整 replay / paper-trading 级联验证闭环

### 4. 回归测试
每次关键改动至少验证：
- Trader 创建/启动/停止
- 模型配置保存/读取
- 交易所配置保存/读取
- 策略保存/读取
- 持仓/订单/权益查询
- 仪表盘关键页面可访问
- protection 配置保存后不破坏后端验证与前端构建

### 5. 高风险专项测试
- 异常恢复
- 重复下单/幂等
- 订单状态同步不一致
- API 密钥敏感信息保护
- 空数据/脏数据输入处理
- 开仓后保护单缺失 / 校验失败
- drawdown partial exit
- break-even stop reset
- ladder partial-close protection verify

## 当前自动化基线（2026-03-26）
- [x] `go test ./...`
- [x] `cd web && npm test`
- [x] `cd web && npm run build`

## 本轮新增覆盖
- [x] AI protection mode 最小执行闭环（策略配置 / AI decision protection plan / post-open apply）
- [x] Regime Filter 开仓前门禁（allowed regimes / funding / volatility / trend alignment）
- [x] 统一 fake trader harness（供 protection / replay / paper-trading 共用）
- [x] protection lifecycle test 骨架（验证闭环已启动）
- [x] paper trader 最小实现与基础单测
- [x] replay fixtures 目录规范与首个 smoke 场景样例
- [x] replay runner / scenario executor 最小实现与 smoke test
- [x] replay scenario 已接入 protection / regime filter 最小验证

## 仍需后续补强的测试项
- [ ] AI protection mode 更完整专项测试
- [ ] Regime Filter 更完整专项测试
- [ ] 更完整的 protection 生命周期集成测试
- [ ] replay runner 与 protection / regime / AI protection 深度集成
- [ ] replay / paper-trading / 仿真验证闭环
- [ ] 收益指标 / 稳定性指标的正式回归口径
