# NOFX API 与前端页面映射（主控首轮）

## 1. 前端入口结构

根入口：`web/src/App.tsx`

页面级路由状态包括：
- `competition`
- `traders`
- `trader`
- `strategy`
- `strategy-market`
- `data`
- `faq`
- `login`
- `register`

对应主要页面/组件：
- Competition：`components/trader/CompetitionPage`
- Traders：`components/trader/AITradersPage`
- Trader Dashboard：`pages/TraderDashboardPage`
- Strategy Studio：`pages/StrategyStudioPage`
- Strategy Market：`pages/StrategyMarketPage`
- Data：`pages/DataPage`
- FAQ：`pages/FAQPage`
- Login/Register/Reset：`components/auth/*`
- Settings：`pages/SettingsPage`

## 2. App.tsx 中已观察到的主要 API 调用

### 登录后公共基础数据
- `api.getTraders` ← `GET /api/my-traders` (`web/src/lib/api/traders.ts`)
- `api.getExchangeConfigs` ← `GET /api/exchanges` (`web/src/lib/api/config.ts`)

### Trader Dashboard 相关
在 `web/src/App.tsx` 中直接看到以下 SWR 请求：
- `api.getStatus(selectedTraderId)` ← `GET /api/status?trader_id=...` (`web/src/lib/api/data.ts`)
- `api.getAccount(selectedTraderId)` ← `GET /api/account?trader_id=...`
- `api.getPositions(selectedTraderId)` ← `GET /api/positions?trader_id=...`

附加数据 API（由 dataApi 暴露）：
- `GET /api/decisions`
- `GET /api/decisions/latest`
- `GET /api/statistics`
- `GET /api/equity-history`
- `POST /api/equity-history-batch`
- `GET /api/positions/history`
- `GET /api/competition`
- `GET /api/top-traders`

## 3. 后端 API 结构（按 `api/server.go`）

### 公共接口
- `/api/health`
- `/api/supported-models`
- `/api/supported-exchanges`
- `/api/config`
- `/api/wallet/validate`
- `/api/wallet/generate`
- `/api/crypto/config`
- `/api/crypto/public-key`
- `/api/crypto/decrypt`
- `/api/traders`
- `/api/competition`
- `/api/top-traders`
- `/api/equity-history`
- `/api/equity-history-batch`
- `/api/traders/:id/public-config`
- `/api/klines`
- `/api/symbols`
- `/api/strategies/public`
- `/api/register`
- `/api/login`
- `/api/reset-password`

### 鉴权后接口（部分）
- `/api/logout`
- `/api/user/password`
- `/api/server-ip`
- `/api/my-traders`
- `/api/traders/:id/config`
- `/api/traders` (POST)
- `/api/traders/:id` (PUT/DELETE)
- `/api/traders/:id/start`
- `/api/traders/:id/stop`
- `/api/traders/:id/prompt`
- `/api/traders/:id/sync-balance`
- `/api/traders/:id/close-position`
- `/api/traders/:id/competition`
- `/api/traders/:id/grid-risk`
- `/api/ai-costs`
- `/api/ai-costs/summary`
- `/api/models`
- `/api/exchanges`
- `/api/telegram`
- `/api/strategies*`
- `/api/status`
- `/api/account`
- `/api/positions`
- `/api/positions/history`
- `/api/trades`
- `/api/orders`
- `/api/orders/:id/fills`
- `/api/open-orders`
- `/api/decisions`
- `/api/decisions/latest`
- `/api/statistics`

## 4. 精确映射关系（首轮）

### Competition 页面
前端：`components/trader/CompetitionPage`
后端主要依赖：
- `GET /api/competition`
- `GET /api/top-traders`
- `GET /api/equity-history`
- `POST /api/equity-history-batch`
- `GET /api/traders/:id/public-config`

### Traders 列表 / 管理页
前端：`components/trader/AITradersPage`
后端主要依赖：
- `GET /api/my-traders`
- `POST /api/traders`
- `PUT /api/traders/:id`
- `DELETE /api/traders/:id`
- `POST /api/traders/:id/start`
- `POST /api/traders/:id/stop`
- `GET /api/models`
- `GET /api/exchanges`
- `GET /api/supported-models`
- `GET /api/strategies`
- `GET /api/traders/:id/config`

### Trader Dashboard 页面
前端：`pages/TraderDashboardPage` + charts 相关组件
后端主要依赖：
- `GET /api/status`
- `GET /api/account`
- `GET /api/positions`
- `GET /api/decisions`
- `GET /api/decisions/latest`
- `GET /api/statistics`
- `GET /api/equity-history`
- `GET /api/positions/history`
- `GET /api/orders`
- `GET /api/open-orders`
- `GET /api/trades`
- `GET /api/klines`
- `GET /api/symbols`

### Strategy Studio 页面
前端：`pages/StrategyStudioPage`
后端主要依赖：
- `GET /api/models`
- `GET /api/strategies`
- `GET /api/strategies/default-config`
- `GET /api/strategies/:id`
- `POST /api/strategies`
- `PUT /api/strategies/:id`
- `POST /api/strategies/:id/duplicate`
- `POST /api/strategies/:id/activate`
- `POST /api/strategies/preview-prompt`
- `POST /api/strategies/test-run`

### Settings 页面
前端：`pages/SettingsPage`
后端主要依赖：
- `PUT /api/user/password`
- 以及配置型接口（模型/交易所/Telegram）

### 配置弹窗族
前端：
- `components/trader/ModelConfigModal.tsx`
- `components/trader/ExchangeConfigModal.tsx`
- `components/trader/TelegramConfigModal.tsx`
- `components/trader/TraderConfigModal.tsx`

后端主要依赖：
- `/api/models`
- `/api/exchanges`
- `/api/telegram`
- `/api/wallet/validate`
- `/api/wallet/generate`
- `/api/server-ip`
- `/api/traders/:id/grid-risk`

## 5. 已发现的不一致/疑点

### 5.1 文档/实现路径不一致
- `dataApi.getPublicTraderConfig()` 使用的是 `${API_BASE}/trader/${traderId}/config`
- 但后端公开接口注册的是 `GET /api/traders/:id/public-config` (`api/server.go`)
- 这看起来像真实不一致，值得后续核验是否存在兼容旧路由

### 5.2 前端存在 `/api/admin-login` 调用痕迹
- 文件：`web/src/contexts/AuthContext.tsx`
- 但目前在 `api/server.go` 可见路由中未看到对应注册
- 需要进一步确认：
  - 是否路由已废弃但前端残留
  - 或是否在别处动态注册

### 5.3 路由风格不完全统一
- 一部分统一走 `web/src/lib/api/*`
- 一部分组件直接 `fetch('/api/...')`
- 一部分页面使用 `API_BASE = import.meta.env.VITE_API_BASE || ''`
- 这会增加后续维护成本和接口迁移成本

## 6. 下一步建议

1. 核验 `public-config` 与 `admin-login` 的真实有效性
2. 统一前端 API 访问层，减少散落 `fetch`
3. 为 Dashboard / Strategy / Settings 三大页面分别建立字段依赖矩阵
4. 把 API 路径不一致项整理为首批低风险修复目标
