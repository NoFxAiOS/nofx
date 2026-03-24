# NOFX 启动链 / 决策链 / 交易链 / 风控链收口说明（中文）

> 状态：接管收口版 v1  
> 时间：2026-03-24  
> 适用分支：`fox/project-takeover-baseline`

---

## 1. 文档目的

本文件把 NOFX 当前最关键的四条主链路整理成可连续阅读的收口说明，方便：

- 新会话快速接续
- 新开发者定位入口
- 后续专项审计
- 识别“哪里已经形成闭环，哪里仍然只是阶段性打底”

四条链分别是：

1. 启动链
2. 决策链
3. 交易链
4. 风控链

---

## 2. 启动链

## 2.1 主入口

主入口在：`main.go`

## 2.2 当前主流程

1. 加载 `.env`
2. 初始化 logger
3. `config.Init()` 加载配置
4. 初始化加密服务 `crypto.NewCryptoService()`
5. 初始化数据库 `store.NewWithConfig(...)`
6. 初始化 installation id（telemetry）
7. 设置 JWT secret
8. 创建 `TraderManager`
9. 从 store 加载 trader 到内存
10. 创建 API Server
11. 建立 Telegram reload channel
12. 启动 API server goroutine
13. 启动 Telegram bot goroutine
14. 等待退出信号
15. 停止全部 trader 并安全退出

## 2.3 启动链职责边界

启动链负责：

- 系统级依赖准备
- 全局配置注入
- 管理器和服务装配
- 生命周期起停控制

启动链**不负责**：

- AI 决策正确性
- 单次交易执行逻辑
- 页面级前端逻辑

## 2.4 当前收口结论

启动链已经足够集中，可作为后续接管/排障的第一定位入口。

---

## 3. 决策链

## 3.1 核心角色

决策链的核心模块包括：

- `trader/auto_trader_loop.go`
- `kernel/*`
- `market/*`
- `mcp/*`
- `provider/*`

## 3.2 决策链主流程

一次 trading cycle 大致如下：

1. `runCycle()` 进入一个 AI 决策周期
2. 检查 trader 是否仍处于运行状态
3. 检查是否处于 stopUntil 风控暂停期
4. 重置日内状态（若跨日）
5. 构建 trading context
   - 账户状态
   - 当前持仓
   - 候选币
   - 指标/行情/外部数据
6. 保存 equity snapshot
7. 若没有 candidate coins，则记录后跳过
8. 调用 `kernel.GetFullDecisionWithStrategy(...)`
9. 获取：
   - system prompt
   - user prompt
   - raw response
   - CoT trace
   - decisions
10. 记录 AI charge
11. 若 AI 失败：
   - 累加连续失败次数
   - 达到阈值则进入 safe mode
12. 若 AI 成功：
   - 清零连续失败计数
   - 若在 safe mode，则解除
13. 对 decisions 排序：先平仓后开仓
14. 在 safe mode 下过滤开仓动作
15. 逐个执行 decision，并记录 action

## 3.3 决策链的关键约束

- AI 不是最终真理，代码强制规则优先
- close 优先于 open，降低堆仓风险
- safe mode 用来处理 AI 连续失败场景
- decision 相关上下文和输出尽量记录，便于回放与审计

## 3.4 当前收口结论

决策链已经形成“可追踪、可回放、可保护”的基础闭环，但仍不是严格确定性系统。

---

## 4. 交易链

## 4.1 核心角色

交易链的核心模块包括：

- `trader/auto_trader_orders.go`
- `trader/auto_trader_decision.go`
- 各交易所适配器：`trader/binance/*`、`trader/bybit/*`、`trader/okx/*` 等
- `store/order / position / fill` 等持久化逻辑

## 4.2 交易链主流程

以 open decision 为例：

1. 获取当前持仓
2. 检查最大持仓等代码强制规则
3. 拉取执行行情数据
4. 获取账户余额与净值
5. 执行仓位价值比例检查
6. 执行最小开仓金额检查
7. 计算 quantity
8. 设置 margin mode
9. 调用交易所适配器 `OpenLong/OpenShort`
10. 记录 order id
11. 本地记录与确认订单
12. 记录 position first seen time
13. 进入 protection 执行路径

## 4.3 当前 protection Phase 1 已进入交易链

当前已经接入：

- `ProtectionConfig`
- `ProtectionCapabilities`
- `BuildManualProtectionPlan`
- `applyPostOpenProtection`
- 开仓后最小保护单校验
- 失败立即平仓逻辑

这意味着交易链不再只是“开仓成功就结束”，而是开始要求：

**开仓后必须尝试保护，保护失败不能裸仓保留。**

## 4.4 平仓链

close decision 当前主流程：

1. 获取当前执行价格
2. 优先从本地 store 读取 open position
3. 不存在则回退到交易所仓位查询
4. 调用 `CloseLong/CloseShort`
5. 记录 close order
6. 同步后续统计数据

## 4.5 当前收口结论

交易链已经从“基础下单链”升级到“带保护意识的执行链”，但跨交易所一致性仍待专项核验。

---

## 5. 风控链

## 5.1 风控链的组成

当前风控并不是单个模块，而是多层叠加：

1. **配置层风险参数**
2. **代码强制规则**
3. **AI 提示层约束**
4. **运行态保护机制**
5. **开仓后 protection**

## 5.2 代码强制规则（已确认）

至少包括：

- 最大持仓数
- 最小仓位金额
- 单仓名义价值比例
- 交易顺序优化（先平后开）
- AI 连续失败触发 safe mode

## 5.3 运行态保护

### Safe mode
当 AI 连续失败达到阈值时：

- 阻止新开仓
- 保留已有仓位保护
- 后续周期继续尝试恢复

### Drawdown monitor
当前已有：

- 峰值盈利缓存
- 当前盈利/回撤计算
- 命中阈值后紧急平仓

但它仍是旧逻辑风格，尚未完全并入统一 protection planner。

## 5.4 Protection Phase 1

当前已收口到统一 protection 方向的部分：

- Full TP/SL 配置结构
- 后端执行路径
- 前端配置入口
- 开仓后最小校验
- 校验失败立即平仓

## 5.5 尚未完成的风控统一项

- Ladder TP/SL 执行化
- Drawdown Take Profit 统一化
- Break-even Stop 执行化
- 交易所能力矩阵细化
- 更强的 protection verify

## 5.6 当前收口结论

风控链已具备底线保护，但还没有达到“统一、强一致、可形式化审计”的程度。

---

## 6. 四条链之间的关系

```text
启动链
  ↓
TraderManager / AutoTrader 生命周期
  ↓
决策链（context → AI → decisions）
  ↓
交易链（open/close/sync/protection）
  ↓
风控链（代码强制 + safe mode + protection）
  ↺ 反向影响后续决策与执行
```

关键理解：

- 启动链是装配与托底
- 决策链产出动作意图
- 交易链把动作意图落到交易所
- 风控链不是独立末端，而是贯穿决策与执行全过程

---

## 7. 当前最值得继续推进的后续项

1. `SYSTEM_TRUST_BOUNDARY_CN.md` 持续细化
2. protection Phase 2 执行化
3. 交易所适配器行为矩阵专项
4. 统计/PnL 口径专项
5. 关键入口中文注释继续补齐

---

## 8. 当前结论

NOFX 的四条核心链现在已经足够清晰，能够支持：

- 新会话从本地文档继续接管
- 二次开发不再从零摸索
- 后续围绕 execution / risk / pnl 做更专业的专项收口

但这份收口并不等于所有链路已经完全验收。

更准确地说：

**核心链路已经完成“结构性接管”，下一阶段要做的是“正确性与一致性强化”。**
