# NofxMax 彻底优化执行方案 2026-04-24

## 总览

基于对项目代码的全面审查，以下是四大方向的执行方案。核心原则：**去冗余、通闭环、真AI驱动、数据串联**。

---

## 一、数据源重建：替代 NofxOS，接入交易所原生数据

### 现状

- `provider/nofxos/` 提供 AI500、OI排行、NetFlow、Price排行 → **已不可用**
- `market/api_client.go` 目前仅从 Binance FAPI 拉 K线、当前价、OI、FundingRate
- 前端 CoinSourceEditor 有 5 种模式（static / ai500 / oi_top / oi_low / mixed），全部依赖 NofxOS

### 方案

#### 1.1 交易所原生数据扩展（市场层 `market/`）

从 Binance FAPI 可免费获取且对交易决策有价值的数据：

| 数据 | API | 用途 |
|------|-----|------|
| **多周期 K 线** | `/fapi/v1/klines` | ✅已有 |
| **OI** | `/fapi/v1/openInterest` | ✅已有 |
| **FundingRate** | `/fapi/v1/fundingRate` | ✅已有 |
| **24h Ticker** | `/fapi/v1/ticker/24hr` | 🆕 24h涨跌幅、成交量、成交额，用于热度筛选 |
| **Long/Short Ratio（账户）** | `/futures/data/globalLongShortAccountRatio` | 🆕 多空比 → 情绪指标 |
| **Long/Short Ratio（头部交易者）** | `/futures/data/topLongShortPositionRatio` | 🆕 大户多空比 |
| **Taker Buy/Sell Volume** | `/futures/data/takerlongshortRatio` | 🆕 主动买卖力量 |
| **OI 历史统计** | `/futures/data/openInterestHist` | 🆕 OI变化趋势（替代NofxOS OI排行） |
| **Orderbook Depth** | `/fapi/v1/depth` | 🆕 盘口深度，识别支撑/阻力厚度 |
| **最近成交** | `/fapi/v1/aggTrades` | 🆕 大单检测 |

**实现**：在 `market/api_client.go` 新增 6 个方法，`market/data.go` 扩展 `Data` struct 包含新字段。

#### 1.2 热门币种筛选器（替代 AI500 + OI Top/Low）

**核心逻辑**（新建 `market/hot_coins.go`）：

```
热门币种 = 24hTicker 全量拉取 → 筛选条件：
  1. 24h成交额 > 5000万 USDT（排除山寨低流动性）
  2. OI > 1500万 USDT（有真实资金参与）
  3. 上市时间 > 60天（排除新币收割）
  4. 24h涨跌幅绝对值在合理范围（排除明显被操控的）
  5. 不在黑名单中（用户 excluded_coins 生效）
→ 按综合热度评分排序（成交额权重 + OI权重 + 涨跌幅活跃度）
→ 取 Top N
```

**OI增减排行**也用原生数据重建：
- OI增加排行：`openInterestHist` 计算 4h/24h OI变化率，按变化率降序
- OI减少排行：同上，按变化率升序

#### 1.3 前端 CoinSourceEditor 改造

- 删除所有 `NofxOSBadge` 标识
- `ai500` 模式 → 重命名为 `hot`（热门币种筛选），用交易所原生数据
- `oi_top` / `oi_low` 保持名称，改为交易所原生 OI 数据源
- `mixed` 模式同步更新
- IndicatorConfig 中的 `nofxos_api_key`、`enable_quant_data` 等 NofxOS 专属字段标记废弃

#### 1.4 新增数据融入 AI 分析全流程

在 `kernel/prompt_builder.go` 的 `BuildUserPrompt` 中，将新数据注入提示词：
- Long/Short Ratio → 情绪面数据块
- Taker Buy/Sell → 资金流向数据块
- Top Trader Position → 大户动向数据块
- Orderbook Depth 关键价位 → 盘口支撑/阻力

---

## 二、策略控制模块整合：开仓门禁 + 策略控制 → 合并入 Regime Filter

### 现状

- `RiskControlEditor` 中有"开仓门禁"性质的参数（min_confidence, min_risk_reward_ratio）
- `StrategyControlPolicy`（strict / audit_only / recommend_only）独立存在
- `RegimeFilter` 是开仓前门禁
- `EntryStructureEditor` 也是开仓前约束
- **问题**：分散在 4 个地方，用户不知道哪个管"能不能开"

### 方案

#### 2.1 整合为统一的"开仓门禁"（Pre-Entry Gate）

保留 `RegimeFilter` 作为入口容器，扩展为：

```typescript
interface PreEntryGateConfig {
  enabled: boolean;
  
  // === 市场状态门禁 (原 RegimeFilter) ===
  allowed_regimes: string[];
  block_high_funding: boolean;
  max_funding_rate_abs: number;  // 单位补充：绝对值，0.01 = 1%/8h
  block_high_volatility: boolean;
  max_atr14_pct: number;         // 单位补充：ATR14占价格百分比，3 = 3%
  require_trend_alignment: boolean;
  
  // === 开仓信心门禁 (从 RiskControl 移入) ===
  min_confidence: number;        // 0-100
  min_risk_reward_ratio: number; // 例如 3 表示 1:3
  
  // === 策略控制政策 (从 StrategyControlPolicy 移入) ===
  policy_mode: 'strict' | 'audit_only' | 'recommend_only';
  
  // === 结构化开仓约束 (从 EntryStructure 移入) ===
  entry_structure: EntryStructureConfig;
}
```

#### 2.2 单位明确化（全面审查）

| 字段 | 当前问题 | 修正 |
|------|----------|------|
| `max_funding_rate_abs` | 看不出 0.01 是 1% 还是 0.01% | 标注：绝对值，0.01 = 每8h费率1%（对应年化 ~1095%） |
| `max_atr14_pct` | 不明确 | 标注：ATR14 / 当前价 × 100，3 = 波动率3% |
| `trigger_value` (break_even) | profit_pct 模式下是百分比，r_multiple 模式下是倍数 | 根据 trigger_mode 动态显示单位 |
| `offset_pct` (break_even) | 不明确方向 | 补充：正值表示在开仓价上方（long）或下方（short）偏移 |
| `min_profit_pct` (drawdown) | 是否含杠杆 | 补充：PnL%含杠杆效果 |
| `max_drawdown_pct` (drawdown) | 从峰值回撤的百分比 | 补充：从峰值PnL%的回落幅度 |

#### 2.3 前端改造

- `RiskControlEditor` 只保留真正的风控参数（max_positions、leverage、position_value_ratio、max_margin_usage、min_position_size）
- 把 min_confidence、min_risk_reward_ratio 移到新的 PreEntryGate 编辑器
- 删除独立的 `StrategyControlPolicy` 编辑面板
- `EntryStructureEditor` 内嵌到 PreEntryGate 内部
- 每个参数旁边显示单位和示例值提示

#### 2.4 互斥/层级关系示例提示

在 UI 中增加 InfoBlock 说明：
- **互斥**：Regime Filter 阻止开仓时，Entry Structure 不会被评估
- **层级**：Regime Filter（第一道）→ Entry Structure（第二道）→ AI Confidence（第三道）
- **包含**：所有"能不能开"的判断统一在这一个面板里

---

## 三、开仓结构判断细化 + 保护系统 AI 真正驱动

### 3.1 开仓/清仓位置对齐支撑阻力/Fibonacci

**现状**：EntryStructure 的 require_fibonacci / require_support_resistance 是 bool 开关，但：
- AI prompt 没有要求输出具体的支撑/阻力价格列表
- 保护系统的 TP/SL 价格没有参考这些结构位
- 开仓价和清仓价与结构位的契合度没有闭环验证

**方案**：

1. **扩展 AI 输出 schema**（`kernel/engine_prompt.go`）：
   - `entry_protection_rationale` 增加 `key_levels` 数组：
     ```json
     "key_levels": [
       {"price": 95200, "type": "resistance", "timeframe": "1h", "source": "previous_swing_high"},
       {"price": 93800, "type": "support", "timeframe": "15m", "source": "fibonacci_0.618"},
       {"price": 92500, "type": "support", "timeframe": "4h", "source": "volume_profile_poc"}
     ]
     ```
   - 这些 key_levels 直接驱动 protection_plan 的 TP/SL 定价

2. **Fibonacci 计算模块**（新建 `market/fibonacci.go`）：
   - 给定一段 K 线的 swing high/low，自动计算 0.236 / 0.382 / 0.5 / 0.618 / 0.786 回撤位
   - 注入到 AI 的市场数据中，减轻 AI 自己计算的不确定性

3. **支撑阻力自动识别**（新建 `market/structure.go`）：
   - 用多周期 K 线的 swing point 检测算法，识别关键支撑/阻力
   - 按主周期、上下一级周期分组
   - 注入到 prompt 数据中

4. **闭环验证**（`trader/protection_plan.go` + `trader/auto_trader_decision.go`）：
   - AI 输出的 stop_loss / take_profit 必须与 key_levels 中最近的结构位对齐（容差范围内）
   - 偏离结构位超过阈值时，记录 warning 但不阻止（audit 模式）或降级（strict 模式）

### 3.2 Ladder AI 模式真正驱动

**Bug**：Ladder 在 AI 模式下，价格几乎每次都是开仓价 ±0.9% / ±1.5%，仓位 50% → 看起来是机械化的硬编码回退。

**诊断路径**：

1. 查看 `trader/protection_plan.go` 中 ladder AI 模式的 resolve 逻辑 → 如果 AI 没输出 ladder_rules，回退到什么默认值
2. 查看 `kernel/engine_prompt.go` 中 ladder 相关 prompt → AI 是否被要求必须输出具体的 ladder 规则
3. 查看 AI 实际输出的 protection_plan → 是否真的包含有意义的 ladder_rules

**修复**：

1. **Prompt 强化**：当 ladder mode=ai 时，prompt 必须明确要求：
   - 每一档 TP 价格必须对应一个结构位（支撑/阻力/Fibonacci 回撤）
   - 仓位分配必须基于风险评估而非固定比例
   - 必须输出 `ladder_rules` 数组，否则判定为格式不合规

2. **后端验证**：`protection_plan.go` 中，AI 输出的 ladder_rules 需经过结构位对齐检查

3. **Fallback 改进**：如果 AI 真的没输出合理的 ladder，不用机械化默认值，而是用市场数据自动生成的支撑/阻力作为 fallback ladder

### 3.3 Drawdown AI 模式真正驱动

**同样的问题**：Drawdown 在 AI 模式下可能也是用了固定的 min_profit_pct / max_drawdown_pct，没有真正由 AI 根据市场结构动态决定。

**修复**：

1. Prompt 要求 drawdown_rules 的 min_profit_pct 必须参考当前时间周期的 ATR 和结构位
2. max_drawdown_pct 需要考虑波动率（高波动市场允许更宽松的回撤容忍）
3. 后端验证 drawdown_rules 的合理性（min_profit_pct 不能低于当前 ATR 的某个倍数）

### 3.4 Protection Editor checkbox bug 修复

**Bug**：Full/Ladder 的执行选框取消后不能再选中。

**分析**：`ProtectionEditor.tsx` 中 `enabled` checkbox 的 onChange handler 可能有逻辑问题，当 enabled=false 时某些 UI 状态导致 checkbox 变 disabled。

**修复**：检查 `cardStyle(fullEnabled)` 中 `controlMutedStyle` → `pointerEvents: 'none'` 在 enabled=false 时阻止了所有交互，包括重新勾选。需要让 enabled checkbox 本身始终可交互。

---

## 四、持仓保护执行面板优化

### 现状

`PositionProtectionPanel.tsx`（657行）：
- 显示每个持仓的保护委托（stop/trailing/takeProfit/other）
- 没有展示与结构位的契合度
- 没有展示 drawdown 运行态状态
- 缺少 break_even 的触发状态
- 数据展示混乱，不适合人类快速判断

### 方案

#### 4.1 面板结构重建

每个持仓展示 3 层信息：

**第一层：持仓基础 + 当前盈亏**
- Symbol、方向、杠杆、开仓价、当前价、未实现PnL%、峰值PnL%

**第二层：保护委托状态**（精简表格）
| 保护类型 | 触发价 | 距离% | 结构位契合 | 状态 |
|----------|--------|-------|-----------|------|
| 止损 | 93,200 | -1.8% | ✅ 15m EMA支撑 | 已委托 |
| TP1 (30%) | 96,500 | +1.7% | ✅ 1h 阻力位 | 已委托 |
| TP2 (50%) | 98,000 | +3.3% | ✅ Fib 0.618 | 已委托 |
| Trailing | 回调2% | — | — | 待激活 |

**第三层：运行态保护状态**
- Drawdown：当前阶段（哪个规则激活）、距离触发的百分比
- Break-even：是否已触发、当前止损是否已抬升
- AI 最新分析摘要（1-2 行）

#### 4.2 结构位契合度指标

- 每个保护委托的触发价旁边，显示最近的结构位及距离
- 用颜色编码：绿色=对齐良好（<0.3%偏离）、黄色=大致对齐（0.3-1%）、红色=偏离结构位

#### 4.3 去冗余

- 删掉没有实际作用的状态芯片/标签
- 数据用表格而不是嵌套卡片
- 统一数值格式：价格用逗号分隔，百分比统一带+/-符号

---

## 五、执行优先级

| 阶段 | 任务 | 风险 | 耗时 |
|------|------|------|------|
| **P0** | Protection checkbox bug 修复 | 低 | 30min |
| **P1** | 数据源重建：交易所原生API + 热门币种筛选 | 中 | 1-2天 |
| **P2** | Fibonacci/支撑阻力自动计算模块 | 中 | 1天 |
| **P3** | Ladder/Drawdown AI prompt 强化 + 后端验证 | 中 | 1天 |
| **P4** | 策略控制模块合并（RegimeFilter 扩展） | 中 | 1天 |
| **P5** | 单位明确化 + 示例提示全面补充 | 低 | 半天 |
| **P6** | PositionProtectionPanel 重构 | 中 | 1天 |
| **P7** | 新数据融入 AI 全流程 prompt | 中 | 1天 |

建议先修 P0 bug，然后 P1→P2→P3 串联完成"数据→结构→AI保护"的完整闭环，再做 P4-P7 的优化整理。

---

## 六、需要讨论确认的点

1. **NofxOS 完全废弃** vs 保留为可选？如果后续恢复了是否还要用？
2. **交易所优先级**：现在 market/ 全部用 Binance FAPI，是否需要支持多交易所数据源？
3. **结构位计算精度**：自动 Fibonacci + swing detection 可能会有误差，是否接受 AI 做最终判断而自动计算只做辅助？
4. **Regime Filter 扩展后的字段名**：是继续叫 `regime_filter` 还是改名 `pre_entry_gate`？改名涉及数据库 schema 迁移。
5. **PositionProtectionPanel 数据来源**：结构位数据是存在 position 记录里（开仓时记录），还是实时重新计算？
