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
- [x] 更新任务模板与验收模板为长期使用版

## P1 - 中文化收口
- [x] 为关键入口补首轮中文注释：`main.go`
- [x] 为关键入口补首轮中文注释：`api/server.go`
- [x] 为关键入口补首轮中文注释：`manager/trader_manager.go`
- [x] 为关键入口补首轮中文注释：`trader/*` 主入口（已补主循环/风控主链首轮）
- [x] 为关键入口补首轮中文注释：`kernel/*` 主入口（已补 engine / position validate / prompt builder 首轮）

- [ ] **P1: 交易复盘与数据积累基础设施（一期）**
  - [x] 盘清当前现有数据渠道（decision / order / fill / position / close-event / equity / config）
  - [x] 输出《交易复盘与数据积累方案 V1》
  - [x] 数据模型文档补充 `position_close_events` 与真相源分层
  - [x] 最小连接键补强（V1）：`entry_decision_cycle` / `exit_decision_cycle` / close-event `decision_cycle`
  - [x] 最小结构化环境快照：第一版 `review_context`（safe mode / ai close gate / ai mode / candidate count / equity / margin）
  - [x] 前端最小产品面：`PositionHistory` 升级为第一版交易复盘面板（展示 position / close-event 决策周期）
  - [ ] 设计 review 输出模型（先定义，不急着全实现）
  - [ ] 下一步连接增强：从 `decision_cycle` 升级到 `decision_record_id` 级强连接

## P2 - 持续优化与二次开发前准备
- [x] 清理首轮外部问题
- [x] 完成首轮低风险性能优化
- [x] 推进首轮前端 API 收束
- [x] 继续统一剩余散落 API 调用（已完成一轮 chart / strategy studio / wallet config 收束）
- [x] 确认收益相关核心指标与口径
- [x] 确认稳定性指标（错误率、恢复时间、状态一致性）
- [x] 确认是否存在测试网 / mock / replay 数据支持（已完成盘点，结论为“部分具备但不完整”）
- [x] 识别首个低风险高价值二次开发任务
- [x] Phase 1: 交易所能力矩阵抽象（首轮骨架）
- [x] Phase 1: protection 配置结构落库（代码结构与默认值已落地）
- [x] Phase 1: 手动 Full TP/SL UI + 执行闭环（前后端首轮已打通）
- [x] Phase 1: 开仓后保护单闭环确认与失败平仓（首轮最小闭环已接入）
- [x] Phase 2: Ladder TP/SL + Drawdown Take Profit + Break-even Stop（手动 ladder 执行链、drawdown 配置驱动执行、break-even 运行态执行均已落地并通过当前自动化基线）
- [x] Phase 3: AI protection mode + Regime Filter
- [x] Phase 3 收口补强：protection setup 重试 + 失败统一平仓保护
- [x] Phase 3 收口补强：Drawdown Take Profit 前端多规则编辑闭环
- [x] Phase 3 收口补强：OKX / NOFXOS 网络健壮性补强
- [x] 验证闭环启动：统一 fake trader harness + protection lifecycle test 骨架 + replay/paper-trading 推进方案
- [x] 下一阶段：继续深化 replay / paper-trading / simulation 验证闭环（已完成多场景覆盖、short 侧、多步价格推进、负收益、regime filter 阻断、错误路径、protection 生命周期集成测试深化）

## P0 - 保护机制实战闭环（2026-04-11~12 推进中）
- [x] 修复委托单检测反复下单（验证延迟重试 + 价格容差 + reconciler 冷却期）
- [x] 修复 Full + Ladder TP/SL 不能共存（Ladder 优先 → Full 只补缺方向）
- [x] 移动止盈止损代码路径从 stub 接入实际执行
- [x] Break-even 生命周期完善（fingerprint re-arm + 委托验证）
- [x] 修复保护单无限累积 bug（reconciler 自动清理重复单 + 孤儿单取消）
- [ ] **P0: Drawdown 原生能力收口**
  - [x] 纠正 partial drawdown 语义：从伪 native 正名为 managed
  - [x] generic cleanup 不误清 native trailing
  - [x] capability 模型拆细：新增 native full / native partial trailing 能力位
  - [x] 三家 trailing 接口签名扩展为支持按 quantity 下单（为 partial native 铺路）
  - [x] 运行态优先尝试 native partial trailing，多档可按 `close_ratio_pct` 计算 quantity 下单
  - [x] 修复多档 drawdown native trailing 只保留一条委托的问题（执行链允许 partial tiers 共存，OKX partial trailing 不再自动清旧）
  - [x] trailing runtime 已补 activation/callback/source 展示链
  - [x] 统一执行原则：native trailing 一旦挂上，不因市价波动改 activePx；只有掉单/查不到时才 re-arm
  - [x] 修复 OKX trailing `activePx` 精度问题：改为按 `tickSz` 对齐后再下单，避免价格被交易所拒绝/截断
  - [ ] 核定 OKX/Binance/Bitget 对 partial trailing close 的真实交易所语义边界（实盘/API 文档验证）
  - [x] native armed 后彻底禁用本地 fallback 接管执行
- [ ] **P1: 持仓保护执行面板前后端交付**
  - [x] 发现并接管现有 `PositionProtectionPanel` 骨架
  - [x] 前端状态语义升级：补齐 `managed_partial_drawdown_armed` / `native_partial_trailing`
  - [x] 执行模式文案升级：明确区分 native full / native partial / managed partial
  - [x] 面板说明文案纠偏：不再把 managed partial 误述为 native trailing
  - [x] 后端补充 trailing 元信息（activation / callback / source）供面板展示
  - [x] 执行面板支持“多档原生跟踪委托”逐档展示（runtime active trailing orders + tier 匹配）
