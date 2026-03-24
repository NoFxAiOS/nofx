# NOFX API ↔ 前端页面映射说明（中文，正式版）

> 状态：接管收口版 v1  
> 时间：2026-03-24

---

## 1. 文档目的

这份文档用于回答两个问题：

1. 某个页面主要依赖哪些 API
2. 某个 API 大致服务哪个页面 / 场景

这对后续做接口收束、字段调整、页面联调特别重要。

---

## 2. 页面到 API 的主映射

## 2.1 `LandingPage.tsx`

主要用途：
- 登录前落地页 / 官网式入口
- 公开导航与品牌展示

主要依赖：
- 公开配置类接口（如系统配置）
- 不强依赖 trader 私有接口

---

## 2.2 `FAQPage.tsx`

主要用途：
- FAQ / 帮助说明

主要依赖：
- 基本无强业务 API 依赖

---

## 2.3 `StrategyStudioPage.tsx`

主要用途：
- 策略列表、创建、编辑、复制、删除、激活
- 默认策略配置读取
- prompt 预览
- AI test run
- protection 配置编辑

核心 API：
- `GET /api/strategies`
- `GET /api/strategies/:id`
- `GET /api/strategies/active`
- `GET /api/strategies/default-config`
- `POST /api/strategies`
- `PUT /api/strategies/:id`
- `DELETE /api/strategies/:id`
- `POST /api/strategies/:id/activate`
- `POST /api/strategies/:id/duplicate`
- `POST /api/strategies/preview-prompt`
- `POST /api/strategies/test-run`

关联后端文件：
- `api/strategy.go`
- `store/strategy.go`

---

## 2.4 `StrategyMarketPage.tsx`

主要用途：
- 查看公开策略市场
- 展示 public strategy 信息

核心 API：
- `GET /api/strategies/public`

关联后端文件：
- `api/strategy.go`

---

## 2.5 `TraderDashboardPage.tsx`

主要用途：
- 单个 trader 的运行态总览
- 账户状态、持仓、决策、统计、权益曲线、订单历史等

核心 API（按页面常见数据区分）：
- trader 基础状态
  - `GET /api/my-traders`
  - `GET /api/traders/:id/config`
- dashboard 数据
  - `GET /api/status?trader_id=...`
  - `GET /api/account?trader_id=...`
  - `GET /api/positions?trader_id=...`
  - `GET /api/decisions/latest?trader_id=...`
  - `GET /api/statistics?trader_id=...`
  - `GET /api/orders?trader_id=...`
  - `GET /api/equity-history?trader_id=...`
- 控制类操作
  - `POST /api/traders/:id/start`
  - `POST /api/traders/:id/stop`
  - `POST /api/traders/:id/close-position`
  - `POST /api/traders/:id/sync-balance`
  - `PUT /api/traders/:id/prompt`
  - `PUT /api/traders/:id/competition`
- grid 风险扩展
  - `GET /api/traders/:id/grid-risk`

关联后端文件：
- `api/handler_trader.go`
- `api/handler_trader_status.go`
- `api/handler_order.go`
- `api/handler_competition.go`

---

## 2.6 `SettingsPage.tsx`

主要用途：
- AI 模型配置
- 交易所账户配置
- Telegram 配置
- 用户密码 / 系统设置等

核心 API：
- AI 模型
  - `GET /api/models`
  - `PUT /api/models`
  - `GET /api/supported-models`
- 交易所
  - `GET /api/exchanges`
  - `POST /api/exchanges`
  - `PUT /api/exchanges`
  - `DELETE /api/exchanges/:id`
  - `GET /api/supported-exchanges`
- Telegram
  - `GET /api/telegram`
  - `POST /api/telegram`
  - `POST /api/telegram/model`
  - `DELETE /api/telegram/binding`
- 用户/系统
  - `PUT /api/user/password`
  - `GET /api/config`
  - `GET /api/server-ip`
  - `GET /api/crypto/config`
  - `GET /api/crypto/public-key`
  - `POST /api/crypto/decrypt`

关联后端文件：
- `api/handler_ai_model.go`
- `api/handler_exchange.go`
- `api/handler_telegram.go`
- `api/handler_user.go`
- `api/crypto_handler.go`

---

## 2.7 `DataPage.tsx`

主要用途：
- 数据源 / 行情侧页面（当前较轻）

可能依赖：
- `GET /api/klines`
- `GET /api/symbols`

关联后端文件：
- `api/handler_klines.go`

---

## 2.8 `CompetitionPage` / traders 列表相关

虽然不在 `pages/` 根目录内，但它是重要展示面。

主要用途：
- 平台公开比赛页
- trader 排行与收益展示

核心 API：
- `GET /api/traders`
- `GET /api/competition`
- `GET /api/top-traders`
- `GET /api/equity-history`
- `POST /api/equity-history-batch`
- `GET /api/traders/:id/public-config`

关联后端文件：
- `api/handler_competition.go`
- `api/handler_trader_status.go`

---

## 3. 反向看：API 到页面的主要服务对象

## 3.1 策略域 API
主要服务：
- `StrategyStudioPage`
- `StrategyMarketPage`

## 3.2 Trader 运行域 API
主要服务：
- `TraderDashboardPage`
- `AITradersPage`
- 比赛/公开页

## 3.3 配置域 API
主要服务：
- `SettingsPage`
- Setup / 登录前配置流程

## 3.4 公共展示域 API
主要服务：
- `LandingPage`
- `CompetitionPage`
- `StrategyMarketPage`

---

## 4. 当前最需要注意的映射风险

1. 页面可能同时混用：
- `api.ts` 中封装接口
- 页面内直接 `fetch`

2. 同一页面常常不是只打一个 handler 文件，而是跨多个后端 handler

3. `TraderDashboardPage` 是最重的联调页面，改动时最容易影响：
- account
- positions
- decisions
- statistics
- equity history
- grid risk

4. `StrategyStudioPage` 已经变成策略配置的核心枢纽页，后续 Phase 2 protection 也会继续集中在这里

---

## 5. 当前结论

API ↔ 页面主映射已经基本明确：

- 策略编辑 → `strategy.go`
- 设置页 → `handler_ai_model / exchange / telegram / user`
- Trader Dashboard → `handler_trader* / handler_order / handler_competition`
- 比赛与公开展示 → `handler_competition`
- 行情数据 → `handler_klines`

后续若继续做 API 收束，优先级建议是：

1. `StrategyStudioPage`
2. `TraderDashboardPage`
3. `SettingsPage`
4. 公开展示页
