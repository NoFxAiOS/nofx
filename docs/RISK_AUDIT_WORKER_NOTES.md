# NOFX 风险审计工作笔记（主控首轮）

> 聚焦：稳定性、安全性、配置一致性、交易系统特有风险。

## P0 / 高风险

### 1. JWT 默认回退值存在生产风险
- 文件：`config/config.go`
- 现象：若未配置 `JWT_SECRET`，系统会回退到 `default-jwt-secret-change-in-production`
- 风险：部署失误时可能出现弱默认密钥，导致认证可被伪造
- 建议：生产模式下强制要求显式配置，不允许使用默认值启动

### 2. CORS 全开放
- 文件：`api/server.go`
- 现象：`Access-Control-Allow-Origin` 设为 `*`
- 风险：对带鉴权接口的浏览器场景不够稳妥，可能扩大攻击面
- 建议：改为可配置白名单，区分开发/生产

### 3. 环境变量模板与代码读取项疑似不一致
- 文件：`.env.example`, `config/config.go`
- 现象：模板中出现 `NOFX_BACKEND_PORT`，代码读取 `API_SERVER_PORT`
- 风险：部署者按模板配置后，服务端口可能未按预期生效
- 建议：统一命名或做兼容读取，并补测试/文档

### 4. AI 输出解析链天然脆弱
- 文件：`kernel/engine_analysis.go`
- 现象：依赖 `<reasoning>` / `<decision>` 标记和 JSON 提取
- 风险：模型输出轻微漂移就可能导致解析失败、safe mode、或者误跳过交易
- 建议：
  - 强化 schema validation
  - 增加更严格的 fallback 和错误分类
  - 为关键模型/provider 做解析一致性测试样本

## P1 / 中高风险

### 5. Safe mode 恢复条件可能过于宽松
- 文件：`trader/auto_trader_loop.go`
- 现象：连续 3 次 AI 失败触发 safe mode；下次 AI 成功即解除
- 风险：在外部模型服务波动时，系统可能在“失败-恢复-失败”间抖动
- 建议：增加冷却窗口、连续成功阈值、或区分错误类型

### 6. 风控规则分散，多处实现，后续容易漂移
- 文件：`kernel/engine_prompt.go`, `trader/auto_trader_orders.go`, `trader/auto_trader_risk.go`
- 现象：一部分风控在 prompt，一部分在执行代码，一部分在监控器
- 风险：规则变更时容易出现“prompt 已更新但硬校验没改”或反过来
- 建议：把关键风控参数和校验逻辑收束到单独风控层/策略验证层

### 7. 自动缩仓逻辑会影响策略可解释性
- 文件：`trader/auto_trader_orders.go`
- 现象：仓位过大时系统自动调小，而不是硬拒绝
- 风险：
  - 实际执行与 AI 决策不一致
  - 收益分析可能失真
  - 用户难以理解“为什么不是按决策仓位执行”
- 建议：记录 `requested_size` 与 `executed_size`，并在前端/日志中明确展示

### 8. 前端构建主包过大
- 文件：前端构建输出（`npm run build`）
- 现象：主 JS 包约 2MB，构建器已告警
- 风险：首次加载慢，控制台体验和部署性能受影响
- 建议：按页面与图表/富文本/数学渲染做代码分割

## P2 / 中风险

### 9. Token blacklist 为内存实现
- 文件：`auth/auth.go`
- 现象：logout blacklist 仅保存在进程内存中
- 风险：多实例部署或重启后，登出 token 状态不一致
- 建议：后续如进入多实例/云部署，迁移到持久化共享存储

### 10. 前端 `npm install` 阶段出现 8 个漏洞提示
- 场景：`web/npm install` 输出
- 风险：未确认是否可利用，但应纳入依赖审计
- 建议：做一次 `npm audit` 分级查看，不盲目全量 fix

### 11. Husky prepare 在子目录提示 `.git can't be found`
- 场景：`cd web && npm install`
- 风险：不是功能阻塞，但说明前端子目录脚本对 mono-repo / 子目录安装兼容性一般
- 建议：后续清理前端工程脚本时一并处理

## 需要继续深挖的点

1. `MaxMarginUsage` 是否有后端硬限制
2. `MinRiskRewardRatio` / `MinConfidence` 是否仅停留在 prompt 层
3. 订单确认 `recordAndConfirmOrder()` 的轮询/异常处理边界
4. 各交易所 `order_sync.go` 的一致性与幂等策略
5. 敏感配置接口返回值是否在所有 handler 中都做了安全裁剪
