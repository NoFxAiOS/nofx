# Drawdown / Break-even / Protection 执行层交付总结（2026-04-20）

## 目标

本轮收尾聚焦 `nofxmax` 真实执行层问题，不再停留在 prompt / config 层讨论，核心目标是：

1. 启动链路稳定，不再因为余额拉取/重复启动导致服务假死。
2. 保护系统职责清晰：
   - 基础保护（ladder / full / fallback）
   - 保本保护（break-even）
   - 利润保护（drawdown / trailing）
3. Reconciler 不再靠粗糙数量判断，而是按角色/来源识别缺失与脏单。
4. 多档 drawdown 只向前升级，不在高利润阶段回头补低档。
5. 小仓位不可执行 ladder 不再无限重试，而是清晰降级为可执行保护。

---

## 本轮主要代码提交

### 启动/稳定性
- `8ec80e0d` fix: stabilize startup and protection validation
- `6362c234` fix: make start script idempotent and port-aware
- `20d876f1` fix(api): avoid reload storms on trader queries

### protection 执行层主线
- `8e734e7e` fix(trader): harden native trailing and fallback execution
- `9e417016` fix(trader): require explicit ladder tiers during reconciliation
- `7218eea6` fix(trader): cleanup unexpected protection orders by role
- `2bb3d138` fix(okx): cleanup protection orders by source tag
- `b4fc18e3` chore(trader): log protection snapshots during reconciliation
- `6fc79818` fix(okx): validate protection quantities before ladder placement
- `f02a3feb` fix: keep drawdown profit stages forward-only
- `d48b4272` Handle non-executable ladder protection degradation
- `83803442` fix(trader): degrade non-executable ladder tiers cleanly

### 交付配套
- `697f778b` feat(web): show decision review context in position history
- `beac93e9` docs: document protection reasoning contract coverage

---

## 现在的保护逻辑（实际执行语义）

### 1. 基础保护层
开仓后先建立基础保护，来源包括：

- ladder stop / ladder TP
- full stop / full TP
- fallback max-loss

特点：
- 这是“开仓即有”的基础防守层。
- 如果 ladder tier 经交易所最小量校验后不可执行，会在执行前被过滤。
- 若一侧 ladder 全部不可执行，则会降级为 full SL / full TP（若有 fallback/full price 可用）。

### 2. break-even 层
盈利达到 break-even trigger 后，独立追加一层 break-even stop：

- 不再取消已有 ladder/full stop-loss。
- 不允许被误判成 ladder tier。
- 同一 position fingerprint 稳定时，不重复 reapply。

### 3. drawdown / trailing 利润保护层
当前 drawdown 采用“利润阶段驱动”的收敛方向：

- 多档规则不再在高利润阶段回头补低利润档。
- 当前只取“已满足的最高 `MinProfitPct` 档位”；同档多个 rule 仍允许一起生效。
- 原生 trailing 仍可作为执行形式，但上层语义已收敛为：
  - 利润进入更高 stage 才升级保护；
  - 利润回落时不回退 stage。

### 4. Reconciler 巡检/修复层
Reconciler 现在负责：

- 检测缺失保护（missing protection）
- 检测多余/脏保护（unexpected protection）
- 清理并重建正确的保护集合
- 在仓位消失时清理 orphan 保护与本地 state/cache

并且已从“粗糙 count 判定”升级为：

- 单 tier ladder 必须按具体价格存在，不能被 break-even / fallback / 旧 stop 冒充
- unexpected stop / TP 按角色识别
- cleanup 对 OKX 优先按 tag 精确删除：
  - `ladder_sl`
  - `full_sl`
  - `fallback_maxloss_sl`
  - `break_even_stop`
  - `ladder_tp`
  - `full_tp`

---

## 关键问题与修复结果

### A. 服务经常“又坏了”
根因：
- backend 启动前强制拉交易所余额会阻塞 readiness
- `start.sh` 每次运行都先杀已有进程
- API query 高频触发 trader reload，扰动运行态

结果：
- backend 启动不再因 `initial_balance` 拉取失败卡住
- `./start.sh` 变为幂等启动；只有 `--restart` 才真正重启
- query 接口不再默认 reload traders，避免 reload storm

### B. ladder 被 break-even/fallback 冒充
根因：
- 旧逻辑只要“有个 stop”就可能认为 protection 满足

结果：
- single ladder SL / TP 现在也必须逐个匹配具体价格
- break-even / fallback / 旧 stop 不能再冒充 ladder

### C. 多余保护单不清 / 清错
根因：
- 旧 cleanup 基本靠数量大概判断
- OKX cleanup 粗暴，容易整锅端

结果：
- unexpected protection 改成按 stop/TP 角色识别
- OKX cleanup 支持按 tag 精确清理
- protection snapshot 日志会打印当前交易所保护快照，便于诊断

### D. ladder 不存在，但系统一直重试
根因：
- 小仓位拆分后单档 quantity 小于交易所最小量
- 旧逻辑会无限尝试一个必然失败的 ladder

结果：
- 在 `placeAndVerifyProtectionPlan` 前引入 `validateProtectionPlanExecution`
- OKX 使用 `ValidateProtectionQuantity` 做真实可执行性校验
- 不可执行的 ladder tier 先过滤，再决定是否降级
- 不再把一个必然被交易所拒绝的 tier 无限重试

### E. 多档 drawdown 升级容易乱
根因：
- 原来所有满足 `MinProfitPct` 的规则都可能 arm，导致高利润阶段还回头补低档

结果：
- 现在 drawdown 只取当前满足的最高利润阶段
- 同档多规则允许共存，不同利润阶段不再回头补挂

---

## 当前已验证

已运行并通过：

```bash
go test ./...
```

以及多轮定向测试：

- trader protection execution / reconciliation
- drawdown native trailing
- break-even coexistence
- ladder degradation
- api reload behavior
- kernel reasoning contract path

---

## 还保留的现实边界

### 1. 原生 trailing 并不等于最终业务逻辑
交易所原生 trailing 主要基于价格回撤与 callback ratio，而不是用户想要的“利润阶段升级语义”。

当前方向是：
- 上层按利润 stage 决定是否升级保护；
- 底层可选择 native trailing 作为执行形式之一。

### 2. 小仓位场景下，ladder 不是总能原样落地
这是交易所约束，不是代码 bug。

当前策略：
- 先校验可执行性；
- 再执行；
- 不可执行就降级，而不是装作能下。

### 3. 剩余 fixture 仍需人工决策是否保留
当前工作树残留测试产物：

- `docs/fixtures/protection-test-run-last-result.json`
- `docs/fixtures/protection-test-run-open-bias-fixture.json`

它们更像运行产物/fixture 草案，不属于本轮核心保护逻辑修复提交。

---

## 建议后续新任务方向

如果后续开新任务，建议优先按以下顺序推进：

1. **真实持仓实盘验收**
   - 对当前活跃 symbol 逐个核对：
     - 基础保护是否存在
     - break-even 是否按 trigger arm
     - drawdown stage 是否只向前升级
     - cleanup 是否精确、不误伤

2. **drawdown stage 可视化摘要**
   - 在日志/UI 中明确展示：
     - 当前 profit stage
     - 当前生效的保护 profile
     - ladder 是否降级
     - break-even / trailing 是否 armed
   - 2026-04-21 更新：`PositionProtectionPanel` 已补运行态摘要字段与展示，现可直接看到：
     - drawdown 当前档位 / satisfied / triggered / next gate
     - break-even live order / break-even price
     - trailing live order count
     - ladder planned vs live count
     - degradation summary（如 `SL→Full` / `TP partial` / `Fallback live`）
     - full/fallback state
   - 对应验收样例已补：
     - `trader/position_protection_runtime_test.go`
     - `web/src/components/trader/PositionProtectionPanel.test.tsx`

3. **决定 fixture 产物是否入库**
   - 若保留，则作为正式 fixture；
   - 若不保留，则忽略/删除，保持工作树干净。

---

## 交付结论

本轮 `nofxmax` 主线已从“保护系统职责混乱、重复单/缺单/误判并存”的状态，收敛到：

- 启动稳定
- query 不再扰动运行态
- ladder / full / fallback / break-even / drawdown 分层更清晰
- reconciliation 按角色与来源处理
- 多档 drawdown 只向前升级
- 不可执行 ladder 有明确降级链路
- 全仓测试通过

这已经达到“可交付成品”的阶段，后续新任务可转入：
- 真实持仓验收
- 保护摘要可视化
- 运营/文档/fixture 最后一轮整理

---

## 2026-04-21 ADA drawdown partial-close follow-up

真实持仓验收中发现 ADAUSDT LONG 曾出现“原始保护语义为部分清仓，但实盘多笔 close_long 累计整仓清掉”的异常样本。

### 复盘结论

- 当时 AI decision records 对 ADA 输出为 `hold`，不是 AI 主动 `close_long`。
- 当时 protection snapshot 显示：
  - `ladder_tp_sl.take_profit_enabled=false`
  - `ladder_tp_sl.stop_loss_enabled=true`
  - drawdown rules 为 70% / 85% partial close
  - break-even enabled
- 因此该异常不是 TP ladder 自然全清，也不是 AI close，而是 drawdown partial-close 执行链与 OKX fill sync 共同暴露的问题。

### 已修复

- `705037bc fix: guard drawdown partial closes against duplicate re-fire`
  - 为 drawdown 增加 position entry/quantity + rule fingerprint guard。
  - 同一持仓、同一剩余数量、同一 rule 已执行后，后续轮询不再重复 partial close。
  - 仓位 fingerprint 变化后允许继续下一次合法保护评估。
- `52219aa1 fix: preserve drawdown close source in okx sync`
  - OKX fills-history sync 从 tag 还原 close source。
  - 后续 close events 可保留 `managed_drawdown` / `native_trailing` / `break_even_stop` / ladder/full 等来源，不再全部扁平化为 `close_long` / `close_short`。

### 验证

已通过：

```bash
go test ./api ./kernel ./trader
cd web && npm run build
```

