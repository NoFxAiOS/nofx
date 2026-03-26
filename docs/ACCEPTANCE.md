# NOFX 验收标准（交付版）

## 接管阶段验收

### 文档
- [x] 有中文项目总览
- [x] 有中文架构说明
- [x] 有模块索引
- [x] 有开发日志
- [x] 有待办与决策记录
- [x] 有接管执行工作流文档
- [x] 有阶段性接管结项总文档
- [x] 有项目记忆归档总表（阶段性版本）

### 基线
- [x] 后端测试可执行并通过
- [x] 前端测试可执行并通过
- [x] 前端构建可执行并通过
- [x] 关键运行方式被记录

### 认知
- [x] 已明确启动链（基础版）
- [x] 已明确决策链（完整收口版）
- [x] 已明确交易链（完整收口版）
- [x] 已明确风控链（完整收口版）
- [x] 已输出首轮风险清单

### 外部问题与交付
- [x] 首轮前后端接口失配已清理
- [x] 首轮明显死代码/误导注释已清理
- [x] 当前版本达到阶段性可交付状态

## 本轮主线功能交付验收（Protection Phase 2 + API 收束）

### A. 前端 API 收束
- [x] `ChartTabs` 已接入统一 HTTP client
- [x] `GridRiskPanel` 已接入统一 HTTP client
- [x] `StrategyStudioPage` 残留直接 API 调用已收束
- [x] `ModelConfigModal` 钱包相关 API 已收束
- [x] 收束后前端测试通过
- [x] 收束后前端构建通过

### B. Drawdown Take Profit
- [x] 不再依赖硬编码单规则
- [x] 支持从 `strategy.protection.drawdown_take_profit.rules` 读取规则
- [x] 支持 `poll_interval_seconds` 调整监控频率
- [x] 支持多规则匹配
- [x] 支持按 `close_ratio_pct` 部分平仓 / 全平
- [x] 后端测试通过

### C. Break-even Stop
- [x] 支持从 `strategy.protection.break_even_stop` 读取运行态配置
- [x] 达到利润阈值后可执行保本止损移动
- [x] 支持在可改单交易所上先取消旧止损再重挂
- [x] 已覆盖低于阈值 / 取消失败 / 正常设置路径测试
- [x] 后端测试通过

### D. Ladder TP/SL
- [x] 支持手动 ladder protection plan 生成
- [x] 支持多阶 TP / SL 价格换算
- [x] 支持 close ratio 累计裁剪到 100%
- [x] 支持按阶梯比例拆单执行
- [x] 支持开仓后逐阶 open-order 校验
- [x] 对不支持 partial close 的交易所会安全阻断
- [x] 后端测试通过

### E. 全量基线
- [x] `go test ./...` 通过
- [x] `cd web && npm test` 通过
- [x] `cd web && npm run build` 通过

## 仍不在本轮验收闭包内的项
- [ ] 更强的前端多规则编辑体验
- [ ] replay / paper-trading / 仿真执行闭环

## 后续功能开发验收模板

每个功能至少回答：
1. 目标是什么
2. 改了哪些模块
3. 对哪些链路有影响
4. 如何测试
5. 是否更新文档/决策
6. 是否有回归风险
