# NOFX 架构说明（中文，接管版）

> 状态：初版骨架，后续持续细化

## 1. 顶层架构

NOFX 当前是一个典型的“前端控制台 + 后端服务 + AI 决策层 + 市场数据层 + 交易执行层 + 存储层”的全栈交易系统。

```text
用户/运维
  ↓
React 前端（web） / Telegram Bot
  ↓
Gin API（api）
  ↓
TraderManager / AutoTrader / Kernel Prompt Builder
  ↓
MCP/Provider（AI 模型 + 行情/外部数据）
  ↓
Trader Exchange Adapter（各交易所实现）
  ↓
Store（SQLite/Postgres）
```

## 2. 关键目录职责

- `main.go`：系统启动入口，初始化配置、加密、数据库、TraderManager、API、Telegram
- `api/`：HTTP API 层，对外提供前端/集成接口
- `config/`：环境变量与全局配置
- `crypto/`：敏感信息加密解密
- `manager/`：交易员生命周期管理
- `trader/`：自动交易核心与各交易所适配实现
- `kernel/`：策略配置、prompt 构建、AI 分析相关内核
- `market/`：行情数据与指标数据装配
- `provider/`：第三方市场/数据源提供方
- `mcp/`：模型调用客户端/协议适配层
- `store/`：数据库访问与领域数据持久化
- `telegram/`：Telegram bot 与会话/代理逻辑
- `web/`：前端 React 控制台

## 3. 启动流程（当前已确认）

1. 加载 `.env`
2. 初始化 logger
3. `config.Init()` 加载全局配置
4. 初始化加密服务 `crypto.NewCryptoService()`
5. 初始化数据库 `store.NewWithConfig(...)`
6. 初始化匿名 installation id（telemetry）
7. 设置 JWT secret
8. 创建 `TraderManager`
9. 从数据库加载 trader 到内存，并按配置决定是否自动启动
10. 创建 API server
11. 启动 Telegram bot（若配置）
12. 等待退出信号并安全停机

## 4. 需要重点追踪的三条主链

### 4.1 配置链
用户在前端配置：
- AI 模型
- 交易所账户
- 策略
- Trader

这些配置进入 API 层后，持久化到 `store/`，再在运行时被 `TraderManager` / `AutoTrader` 使用。

### 4.2 决策链
市场数据 + 策略配置 → kernel 生成 prompt → mcp/provider 调用模型 → 解析结果 → 决策/动作落库 → 触发执行

### 4.3 交易链
决策结果 → 交易所适配器下单/平仓/同步 → 持仓、订单、权益、历史写入 store → API 返回给前端展示

## 5. 当前初步判断的重点模块

### 高优先级
- `trader/auto_trader*.go`
- `kernel/*.go`
- `store/*.go`
- `manager/trader_manager.go`
- `api/handler_trader*.go`
- `api/handler_order.go`
- `api/handler_exchange.go`

### 中高优先级
- `provider/coinank/*`
- `mcp/*`
- `telegram/*`
- `web/src/pages/*`
- `web/src/components/trader/*`
- `web/src/components/strategy/*`

## 6. 当前架构疑点/待核验项

1. `.env.example` 中 `NOFX_BACKEND_PORT` 与代码读取 `API_SERVER_PORT` 存在命名差异，需核验是否有兼容层
2. README 中对技术栈/版本的表述与 `go.mod` / 实际依赖可能存在时间差
3. 文档中部分目录描述可能已落后于当前代码结构
4. 不同交易所适配器在下单、同步、异常恢复上的一致性尚未验证
5. 风控逻辑分布在 `kernel/`、`trader/` 还是 `store/` 需要进一步画图确认
