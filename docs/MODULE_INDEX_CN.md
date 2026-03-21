# NOFX 模块索引（中文）

> 状态：首轮扫描版，后续按目录逐步细化

## 后端核心

### `main.go`
系统启动与依赖装配入口。

### `api/`
HTTP API 层，负责：
- 鉴权与会话入口
- 配置管理（模型、交易所、策略、Trader）
- 仪表盘/比赛/行情数据接口
- Telegram 配置入口

关键文件：
- `server.go`：服务启动与路由注册
- `route_registry.go`：路由说明辅助
- `handler_trader*.go`：交易员管理/状态/配置
- `handler_order.go`：订单相关接口
- `handler_exchange.go`：交易所配置接口
- `handler_ai_model.go`：模型配置接口
- `handler_telegram.go`：Telegram 配置接口

### `config/`
全局配置读取，来源于环境变量。

### `crypto/`
敏感数据加密服务，系统启动时优先初始化。

### `manager/`
`TraderManager` 统一管理多个 trader 的加载、运行、停止。

### `kernel/`
策略/Prompt/分析内核。

已观察到文件：
- `engine.go`
- `engine_analysis.go`
- `engine_position.go`
- `engine_prompt.go`
- `grid_engine.go`
- `prompt_builder.go`
- `schema.go`

推测职责：
- 组装策略上下文
- 生成 AI 输入
- 规范 AI 输出结构
- 驱动分析/决策过程

### `market/`
行情数据与指标数据装配。

### `mcp/`
AI 模型客户端抽象层与 provider 注册机制。

### `provider/`
外部数据源接入：
- `coinank`
- `nofxos`
- `hyperliquid`
- `alpaca`
- `twelvedata`

### `trader/`
自动交易与交易所适配核心目录。

结构特征：
- `auto_trader*.go`：通用自动交易逻辑
- `position_*`：持仓重建/快照
- `grid_*`：网格逻辑
- 子目录：各交易所 adapter（binance/bybit/okx/gate/...）

### `store/`
领域模型持久化，包括：
- user
- trader
- strategy
- exchange
- order
- position
- position_history
- equity
- ai_charge
- decision
- telegram_config

## 前端核心

### `web/src/pages/`
页面级入口：
- `LandingPage.tsx`
- `SettingsPage.tsx`
- `TraderDashboardPage.tsx`
- `StrategyStudioPage.tsx`
- `StrategyMarketPage.tsx`
- `DataPage.tsx`
- `FAQPage.tsx`

### `web/src/components/`
按领域分组：
- `auth`
- `charts`
- `common`
- `faq`
- `landing`
- `modals`
- `strategy`
- `trader`
- `ui`

### `web/src/lib/`
前端基础设施：
- HTTP Client
- 配置
- 加密
- 通知
- 文本/剪贴板工具

### `web/src/stores/`
前端状态管理（Zustand）。

## 横切关注点

### 安全
- `auth/`
- `crypto/`
- `security/`
- API 中敏感配置接口

### 可观测性
- `logger/`
- `telemetry/`

### Bot/代理
- `telegram/`
- `telegram/agent`
- `telegram/session`

## 后续索引细化方向

下一轮会继续把以下内容补充进来：
1. 每个核心文件的职责摘要
2. 关键结构体清单
3. 关键接口关系
4. API 与前端页面对照表
5. 决策链/交易链/风控链详细路径
