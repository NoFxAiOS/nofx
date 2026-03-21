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
- `api.getTraders` ← 对应后端 `GET /api/my-traders`
- `api.getExchangeConfigs` ← 对应后端 `GET /api/exchanges`

### Trader Dashboard 相关
在 `web/src/App.tsx` 中直接看到以下 SWR 请求：
- `api.getStatus(selectedTraderId)` ← 预期对应 trader 状态接口
- `api.getAccount(selectedTraderId)` ← 预期对应账户信息接口
- `api.getPositions(selectedTraderId)` ← 预期对应持仓接口

> 说明：这些具体 endpoint 名称还需要继续顺着 `web/src/lib/*` 与 handler 文件精确核对。

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

## 4. 初步映射关系

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
- `GET /api/strategies`

### Trader Dashboard 页面
前端：`pages/TraderDashboardPage`
后端预期依赖：
- trader status/account/positions/decisions/statistics/equity 相关接口
- 这部分还需顺着 `web/src/lib/api` 精确补全

### Strategy Studio 页面
前端：`pages/StrategyStudioPage`
后端主要依赖：
- `GET /api/strategies`
- `GET /api/strategies/active`
- `GET /api/strategies/default-config`
- `GET /api/strategies/:id`
- `POST /api/strategies`
- `POST /api/strategies/preview-prompt`
- `POST /api/strategies/test-run`

### Settings 页面
前端：`pages/SettingsPage`
后端主要依赖：
- `GET /api/models`
- `PUT /api/models`
- `GET /api/exchanges`
- `POST /api/exchanges`
- `PUT /api/exchanges`
- `DELETE /api/exchanges/:id`
- `GET /api/telegram`
- `POST /api/telegram`
- `POST /api/telegram/model`
- `DELETE /api/telegram/binding`
- `PUT /api/user/password`

## 5. 当前边界不清晰点

1. `App.tsx` 承担了较多页面状态与数据请求职责，后续要评估是否过重
2. Trader Dashboard 真实使用的 endpoint 还需从 `web/src/lib/api*` 精确反查
3. 页面路由既看 pathname 也看 hash，存在历史兼容痕迹，后续要评估是否统一
4. 认证前后页面混用同一个 App 状态机，后续可能影响可维护性

## 6. 下一步建议

1. 精确读取 `web/src/lib/api*`，补全 API 函数 → endpoint 对照表
2. 为 Dashboard / Strategy / Settings 三大页分别画请求矩阵
3. 标出“页面依赖哪些字段”，便于后端改接口时做影响分析
