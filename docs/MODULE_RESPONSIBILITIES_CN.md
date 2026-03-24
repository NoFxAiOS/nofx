# NOFX 模块级职责说明（中文，正式版）

> 状态：接管收口版 v1  
> 时间：2026-03-24  
> 分支：`fox/project-takeover-baseline`

---

## 1. 文档目的

这份文档不是简单重复目录树，而是回答：

- 每个模块到底负责什么
- 它和上下游模块的边界是什么
- 后续开发时，问题应该优先在哪一层解决

---

## 2. 顶层模块职责

## 2.1 `main.go`

职责：
- 系统启动入口
- 装配配置、加密、数据库、TraderManager、API、Telegram
- 控制系统级生命周期与安全停机

不负责：
- 具体交易逻辑
- AI prompt 生成
- 前端页面逻辑

---

## 2.2 `config/`

职责：
- 读取环境变量与全局运行配置
- 提供统一配置访问入口

适合放这里的内容：
- 端口
- DB 连接
- JWT secret
- 第三方服务基础配置

不适合放这里的内容：
- 单个 trader 的运行策略
- 用户级业务配置

---

## 2.3 `crypto/`

职责：
- 敏感字段加解密
- 为账户密钥等敏感信息提供统一保护层

它属于系统基础设施层，不应掺杂业务决策。

---

## 2.4 `api/`

职责：
- HTTP API 入口
- 路由装配
- 鉴权边界
- 请求参数校验、响应整形
- 把前端/外部请求分发到 store / manager / trader 等下层

关键子类：
- `handler_user.go`：用户与认证相关接口
- `handler_ai_model.go`：AI 模型配置
- `handler_exchange.go`：交易所账户配置
- `handler_trader.go` / `handler_trader_config.go` / `handler_trader_status.go`：Trader 生命周期、状态、配置
- `strategy.go`：策略配置、预览、测试运行、默认配置
- `handler_order.go`：订单/成交历史侧接口
- `handler_competition.go`：比赛与排行榜展示
- `handler_ai_cost.go`：AI 调用成本统计
- `handler_telegram.go`：Telegram 配置与绑定
- `handler_wallet.go`：钱包相关接口
- `handler_klines.go`：行情数据查询

边界：
- API 层应该做“入口治理”，不应承载复杂交易执行算法

---

## 2.5 `manager/`

职责：
- 持有运行态 trader 实例
- 负责 trader 的加载、启动、停止、查询
- 负责 competition 数据缓存等运行时管理逻辑

核心文件：
- `trader_manager.go`

边界：
- manager 管生命周期
- trader 管实际交易逻辑

---

## 2.6 `trader/`

职责：
- 自动交易主逻辑
- 各交易所适配器
- 下单、平仓、同步、保护、风控运行态逻辑

建议按三层理解：

### A. AutoTrader 主链
- `auto_trader.go`
- `auto_trader_loop.go`
- `auto_trader_orders.go`
- `auto_trader_decision.go`
- `auto_trader_risk.go`

这层负责：
- trading cycle
- decision 执行
- 运行态风控
- 订单/持仓本地记录

### B. Grid 专项
- `auto_trader_grid.go`
- `auto_trader_grid_levels.go`
- `auto_trader_grid_orders.go`
- `auto_trader_grid_regime.go`
- `grid_regime.go`

这层负责：
- grid strategy 的箱体、档位、订单、方向调节

### C. Protection 新链
- `protection_capabilities.go`
- `protection_plan.go`
- `protection_execution.go`

这层负责：
- 交易保护能力矩阵
- manual protection plan
- 开仓后 protection 挂单、最小校验、失败平仓

### D. Exchange Adapter
- `binance/` `bybit/` `okx/` `bitget/` `gate/` `kucoin/` `hyperliquid/` 等

这层负责：
- 把统一 Trader 接口落到各交易所实际 API/SDK
- 是系统最容易出现“跨平台行为差异”的一层

---

## 2.7 `kernel/`

职责：
- 策略引擎
- prompt 构建
- AI 输入上下文组织
- 决策前内核校验

关键文件：
- `engine.go`：策略引擎主入口
- `engine_prompt.go` / `prompt_builder.go`：prompt 生成
- `engine_position.go`：AI 决策前的约束校验
- `engine_analysis.go`：分析辅助
- `grid_engine.go`：网格策略内核
- `formatter.go`：上下文格式化
- `schema.go`：结构定义

边界：
- kernel 决定“如何让 AI 看世界”
- trader 决定“如何真正执行世界”

---

## 2.8 `market/`

职责：
- 行情数据拉取与聚合
- 为决策与展示提供价格/时间周期数据

它更偏数据装配层，不应该承担业务决策。

---

## 2.9 `provider/`

职责：
- 第三方数据源接入
- 市场/排行/外部量化数据提供

如：
- `coinank`
- `nofxos`
- `hyperliquid`
- `twelvedata`

---

## 2.10 `mcp/`

职责：
- AI 模型客户端与 provider 适配
- 屏蔽不同模型供应商的调用差异

边界：
- 它负责“怎么调模型”
- 不负责“该做什么决策”

---

## 2.11 `store/`

职责：
- 统一持久化层
- 管理用户、模型、交易所、策略、trader、订单、持仓、决策、权益、AI 成本、Telegram 配置等数据

关键子模块：
- `user.go`
- `ai_model.go`
- `exchange.go`
- `trader.go`
- `strategy.go`
- `decision.go`
- `order.go`
- `position.go`
- `equity.go`
- `ai_charge.go`
- `telegram_config.go`
- `grid.go`

边界：
- store 负责数据真相的持久化
- 不负责复杂业务编排

---

## 2.12 `telegram/`

职责：
- Telegram bot 生命周期
- 用户绑定与交互代理

---

## 2.13 `web/`

职责：
- React 控制台
- Trader 运行状态、策略配置、页面导航、排行榜、设置页、策略市场等展示与操作入口

关键页面：
- `StrategyStudioPage.tsx`
- `StrategyMarketPage.tsx`
- `TraderDashboardPage.tsx`
- `SettingsPage.tsx`
- `LandingPage.tsx`
- `FAQPage.tsx`
- `DataPage.tsx`

---

## 3. 开发时的落点建议

### 如果问题是“字段不对 / 页面调用错了”
优先看：
- `web/`
- `api/`
- `store/strategy.go` / DTO 结构

### 如果问题是“AI 决策质量 / prompt 不合理”
优先看：
- `kernel/`
- `provider/`
- `mcp/`

### 如果问题是“能决策但不能稳定执行”
优先看：
- `trader/`
- 各交易所 adapter
- protection 相关文件

### 如果问题是“数据展示与真实收益不一致”
优先看：
- `store/order.go`
- `store/position.go`
- `store/equity.go`
- `store/decision.go`
- `api/handler_order.go`
- competition / dashboard 展示层

---

## 4. 当前结论

NOFX 的模块边界已经足够形成“可持续接管”的工程结构：

- `api` 管入口
- `manager` 管生命周期
- `kernel` 管 AI 输入与约束
- `trader` 管执行与运行态风控
- `store` 管持久化真相
- `web` 管控制台与操作面

下一阶段的重点不再是“搞清目录”，而是围绕这些边界继续做正确性、口径一致性与第二阶段功能演化。
