# NOFX 修复候选清单（首批可落地问题）

## P0 - 接口/实现不一致

### 1. 前端存在 `/api/admin-login` 调用，但后端未见对应路由
- 前端：`web/src/contexts/AuthContext.tsx`
- 后端路由扫描：`api/server.go` 未见注册
- 影响：若前端走 admin 登录分支，会直接失败
- 建议动作：
  1. 确认该功能是否已废弃
  2. 若废弃，删除前端残留入口与调用
  3. 若保留，补后端 route + handler + 文档

### 2. 前端存在 `GET /api/prompt-templates` 调用，但后端未见实现
- 前端：`web/src/lib/api/config.ts`
- 后端路由扫描：未见注册
- 影响：调用会 404
- 建议动作：
  1. 若模板机制已废弃，则删除前端 API 封装
  2. 若仍需存在，则补后端只读接口

### 3. public trader config 路径不一致
- 前端：`web/src/lib/api/data.ts` 中 `getPublicTraderConfig()` 使用 `/api/trader/${id}/config`
- 后端：`api/server.go` 注册的是 `/api/traders/:id/public-config`
- 影响：前端公共配置读取可能走错地址
- 建议动作：优先修前端路径，必要时后端加兼容别名

## P1 - 工程一致性问题

### 4. 前端 API 访问方式不统一
- 部分统一走 `web/src/lib/api/*`
- 部分直接 `fetch('/api/...')`
- 部分用 `VITE_API_BASE`
- 影响：接口迁移、鉴权、错误处理、追踪都更难
- 建议动作：逐步收束到统一 API 层

### 5. 历史 admin mode 注释残留
- `api/server.go` 中有“Admin login (used in admin mode, public)”注释，但未见实际路由
- 影响：认知混乱
- 建议动作：补或删，避免误导

## 推荐的首个实际修复顺序
1. 修正 `getPublicTraderConfig()` 路径
2. 标记/清理 `prompt-templates` 残留
3. 标记/清理或恢复 `admin-login`
4. 给这些修复补最小测试/文档
