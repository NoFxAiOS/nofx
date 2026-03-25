# NOFX 待办清单

## P0 - 接管基线
- [x] 克隆仓库并建立接管分支
- [x] 完成首轮结构扫描
- [x] 建立中文接管文档骨架
- [x] 验证后端测试基线
- [x] 安装前端依赖并验证前端测试/构建
- [x] 建立接管结项总文档（阶段性）
- [x] 建立项目记忆归档总表（阶段性）
- [x] 梳理启动链、决策链、交易链、风控链到完整收口版
- [x] 输出交易系统可信性/风险边界收口说明
- [x] 输出交易保护与盈利控制统一方案设计 v1

## P1 - 架构认知
- [x] 输出中文架构骨架
- [x] 输出模块索引
- [x] 输出更完整的中文架构说明（收口版已增强）
- [x] 输出模块级职责说明
- [x] 输出 API ↔ 前端页面映射正式版
- [x] 输出核心数据表/实体关系说明

## P1 - 开发治理
- [x] 建立 `DECISIONS.md`
- [x] 建立 `TEST_PLAN.md`
- [x] 建立 `ACCEPTANCE.md`
- [x] 建立 `RISKS.md`
- [x] 建立 `CHANGE_IMPACT.md`
- [x] 建立伏羲执行工作流文档
- [ ] 更新任务模板与验收模板为长期使用版

## P1 - 中文化收口
- [x] 为关键入口补首轮中文注释：`main.go`
- [x] 为关键入口补首轮中文注释：`api/server.go`
- [x] 为关键入口补首轮中文注释：`manager/trader_manager.go`
- [x] 为关键入口补首轮中文注释：`trader/*` 主入口（已补主循环/风控主链首轮）
- [x] 为关键入口补首轮中文注释：`kernel/*` 主入口（已补 engine / position validate / prompt builder 首轮）

## P2 - 持续优化与二次开发前准备
- [x] 清理首轮外部问题
- [x] 完成首轮低风险性能优化
- [x] 推进首轮前端 API 收束
- [x] 继续统一剩余散落 API 调用（已完成一轮 chart / strategy studio / wallet config 收束）
- [ ] 确认收益相关核心指标与口径
- [ ] 确认稳定性指标（错误率、恢复时间、状态一致性）
- [ ] 确认是否存在测试网 / mock / replay 数据支持（已完成盘点，结论为“部分具备但不完整”）
- [ ] 识别首个低风险高价值二次开发任务
- [x] Phase 1: 交易所能力矩阵抽象（首轮骨架）
- [x] Phase 1: protection 配置结构落库（代码结构与默认值已落地）
- [x] Phase 1: 手动 Full TP/SL UI + 执行闭环（前后端首轮已打通）
- [x] Phase 1: 开仓后保护单闭环确认与失败平仓（首轮最小闭环已接入）
- [ ] Phase 2: Ladder TP/SL + Drawdown Take Profit + Break-even Stop（已完成 Drawdown TP 配置驱动执行首轮）
- [ ] Phase 3: AI protection mode + Regime Filter
