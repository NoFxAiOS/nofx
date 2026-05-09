# NOFX 修复候选清单（首批可落地问题）

## P0 - 接口/实现不一致

### 1. 前端存在 `/api/admin-login` 调用代码，但后端未见对应路由
- 前端：`web/src/contexts/AuthContext.tsx`
- 后端路由扫描：`api/server.go` 未见注册
- 进一步核验：`loginAdmin` 当前未被任何前端组件/页面调用
- 判断：历史残留死代码，而非当前活跃功能
- 状态：**已修复**
- 修复内容：
  1. 清理 `loginAdmin` 相关死代码与类型定义
  2. 移除 `AuthContext` 中误导性的 admin mode 注释
  3. 移除后端 `api/server.go` 中误导性 admin login 注释

### 2. 前端存在 `GET /api/prompt-templates` 调用封装，但后端未见实现
- 前端：`web/src/lib/api/config.ts`
- 后端路由扫描：未见注册
- 进一步核验：`getPromptTemplates()` 当前未被任何前端代码调用
- 判断：历史残留死代码
- 状态：**已修复**
- 修复内容：
  1. 清理未使用 API 封装
  2. 保持前后端接口集合一致

### 3. public trader config 路径不一致
- 前端：`web/src/lib/api/data.ts` 中 `getPublicTraderConfig()` 原先使用 `/api/trader/${id}/config`
- 后端：`api/server.go` 注册的是 `/api/traders/:id/public-config`
- 状态：**已修复**
- 修复提交：`a0c40676` `fix: align public trader config route`

## P1 - 工程一致性问题

### 4. 前端 API 访问方式不统一
- 部分统一走 `web/src/lib/api/*`
- 部分直接 `fetch('/api/...')`
- 部分用 `VITE_API_BASE`
- 影响：接口迁移、鉴权、错误处理、追踪都更难
- 建议动作：逐步收束到统一 API 层
- 当前状态：**基本完成**
- 2026-04-09 更新：
  - `AuthContext.tsx` login/logout 已迁移到 `httpClient`
  - 仅 `TerminalHero.tsx` klines 保留 raw `fetch`（设计决策：背景轮询不应触发全局 toast）

### 5. 历史 admin mode 注释残留
- `api/server.go` 中有“Admin login (used in admin mode, public)”注释，但未见实际路由
- 影响：认知混乱
- 状态：**已修复**

## 当前推荐的下一步
1. 优先继续核定 OKX/Binance/Bitget 对 partial trailing close 的真实交易所语义边界（实盘/API 文档验证）
2. 继续收束前端 API 调用到统一 API 层
3. 扫描并清理其它未使用/失配的前后端接口封装
4. 补一个轻量检查，防止前端再引入不存在的 `/api/*` 路径


## 已完成的推进（性能与交付）
- 顶层页面已改为懒加载，减少首屏主入口负担
- Trader Dashboard 已拆分为更细粒度 chunk（ChartTabs / PositionHistory / GridRiskPanel）
- Vite 已配置 manualChunks，主入口共享包已从约 640k 降到约 203k
- MetricTooltip 已改为按需动态加载 KaTeX，减少非必要静态依赖
- Recharts 入口组件已改为懒加载（EquityChart / ComparisonChart）

## 当前状态判断
- 前后端接口失配：当前未发现
- 前端测试：通过
- 后端测试：通过
- 前端构建：通过
- 剩余工作重心：继续做低风险性能优化与交付收尾，而不是高风险重构
