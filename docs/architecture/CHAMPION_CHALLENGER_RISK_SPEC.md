# Champion-Challenger A/B 测试框架 - 生产级风控规范

> **版本**: v1.1.1
> **最后更新**: 2026-01-22
> **状态**: 生产就绪

本文档定义了 Champion-Challenger 策略 A/B 测试框架的风控规范，确保跨实现一致性和生产环境稳定性。

---

## 目录

- [1. 组合风险计算 - 双口径并行](#1-组合风险计算---双口径并行)
- [2. Risk Parity Gate - P95 口径](#2-risk-parity-gate---p95-口径)
- [3. Dominance Gate - 评分项/约束项分离](#3-dominance-gate---评分项约束项分离)
- [4. Evidence Gate - Bootstrap 日 PnL 对齐](#4-evidence-gate---bootstrap-日-pnl-对齐)
- [5. Regime 硬定义](#5-regime-硬定义)
- [6. UCB 预算分配 - 虚拟先验](#6-ucb-预算分配---虚拟先验)
- [7. v1.1.1 实现一致性补丁](#7-v111-实现一致性补丁)
- [变更历史](#变更历史)

---

## 1. 组合风险计算 - 双口径并行

### 1.1 设计原理

组合风险需要同时满足两个需求：
- **实时性**: 每笔交易前快速检查
- **准确性**: 考虑品种间相关性的真实组合风险

因此采用双口径并行：
- **快口径**: 保守估计，实时计算
- **准口径**: 精确计算，含相关性矩阵，每小时更新

### 1.2 数据结构

```python
@dataclass
class PortfolioRiskMetrics:
    """组合风险度量"""
    fast_risk: float          # 快口径 (实时)
    accurate_risk: float      # 准口径 (含相关性)
    last_accurate_update: datetime
```

### 1.3 快口径计算 (实时)

```python
def calc_fast_risk(self, positions: List[Position], equity: float) -> float:
    """
    快口径: Σ (position_notional × volatility × leverage) / equity
    用于实时风控，保守但快
    """
    total_risk = 0.0
    for pos in positions:
        position_notional = abs(pos.size * pos.mark_price)
        volatility = self._get_symbol_volatility(pos.symbol)
        risk_contribution = (position_notional * volatility * pos.leverage) / equity
        total_risk += risk_contribution

    return total_risk / self.target_portfolio_vol
```

### 1.4 准口径计算 (含相关性)

```python
def calc_accurate_risk(self, positions: List[Position], equity: float) -> float:
    """
    准口径: 相关性合成的组合波动
    σ_p = sqrt(r^T × Σ × r)
    其中 Σ = diag(σ) × C × diag(σ)
    """
    if not positions:
        return 0.0

    self._maybe_update_correlation_matrix([p.symbol for p in positions])

    symbols = [p.symbol for p in positions]

    # 构建风险暴露向量 r
    r = np.array([
        (abs(p.size * p.mark_price) / equity) * p.leverage * np.sign(p.size)
        for p in positions
    ])

    # 构建协方差矩阵 Σ = diag(σ) × C × diag(σ)
    sigma_vec = np.array([self._volatilities.get(s, 0.02) for s in symbols])
    C = self._get_correlation_submatrix(symbols)
    cov_matrix = np.diag(sigma_vec) @ C @ np.diag(sigma_vec)

    # 组合波动: σ_p = sqrt(r^T × Σ × r)
    portfolio_var = r.T @ cov_matrix @ r
    portfolio_vol = np.sqrt(max(portfolio_var, 0))

    return portfolio_vol / self.target_portfolio_vol
```

### 1.5 执行规则

```yaml
portfolio_risk_enforcement:
  # 实时检查 (每笔交易前)
  realtime_check:
    metric: "fast_risk"
    threshold: 0.95
    action_when_exceeded: "block_new_position"

  # 定期复核 (每小时)
  periodic_check:
    metric: "accurate_risk"
    frequency: "1h"
    thresholds:
      - level: 1.0
        action: "reduce_challengers_first"
        reduce_until: 0.90
      - level: 1.1
        action: "reduce_all_proportionally"
        reduce_until: 0.85
      - level: 1.2
        action: "emergency_flatten_challengers"
        alert: "CRITICAL"
```

---

## 2. Risk Parity Gate - P95 口径

### 2.1 设计原理

使用 P95 分位而非均值比较风险用量，防止峰值风险被均值掩盖。

### 2.2 实现

```python
def risk_parity_gate(input: GateInput) -> Tuple[bool, str]:
    """
    用 P95 而非均值比较风险用量
    """
    champion = input.champion_results

    for cid, challenger in input.challenger_results.items():

        # RP-1: 风险预算占用偏差 (P95 口径)
        champion_risk_p95 = np.percentile(champion.risk_used_timeseries, 95)
        challenger_risk_p95 = np.percentile(challenger.risk_used_timeseries, 95)

        champion_ratio_p95 = champion_risk_p95 / champion.allocated_risk_budget
        challenger_ratio_p95 = challenger_risk_p95 / challenger.allocated_risk_budget

        if champion_ratio_p95 > 0:
            deviation = abs(challenger_ratio_p95 - champion_ratio_p95) / champion_ratio_p95
            if deviation > 0.20:
                return False, (
                    f"RP-1 失败: Challenger {cid} P95风险占用比偏差 "
                    f"{deviation:.1%} > 20%"
                )

        # RP-2: 杠杆偏差 (P95 口径)
        champion_leverage_p95 = np.percentile(champion.leverage_timeseries, 95)
        challenger_leverage_p95 = np.percentile(challenger.leverage_timeseries, 95)

        leverage_diff = abs(challenger_leverage_p95 - champion_leverage_p95)
        if leverage_diff > 1.0:
            return False, f"RP-2 失败: Challenger {cid} P95杠杆偏差 {leverage_diff:.1f} > 1.0"

    # RP-3: 组合层准口径检查
    accurate_risk_p95 = np.percentile(input.portfolio_state.accurate_risk_timeseries, 95)
    if accurate_risk_p95 > 1.1:
        return False, f"RP-3 失败: 组合准口径P95风险 {accurate_risk_p95:.2f} > 1.1"

    return True, "Risk Parity Gate 通过"
```

---

## 3. Dominance Gate - 评分项/约束项分离

### 3.1 设计原理

明确区分：
- **评分项** (5项，计入 wins): 净收益、Sharpe、ProfitFactor、Calmar、胜率
- **约束项** (2项，硬门槛): ES95、MaxDD

**通过条件**: `wins >= 3 AND ES95_ok AND MaxDD_ok`

### 3.2 实现

```python
def dominance_gate(champion: StrategyResults, challenger: StrategyResults) -> Tuple[bool, Dict]:
    """
    评分项 vs 约束项 明确分离
    """
    results = {'scoring_metrics': {}, 'constraint_metrics': {}, 'summary': {}}
    wins = 0

    # ═══════════════════════════════════════════════════════════════════════
    # 评分项 (计入 wins)
    # ═══════════════════════════════════════════════════════════════════════

    # S-1: 净收益 (B > A * 1.05)
    if challenger.net_pnl > champion.net_pnl * 1.05:
        wins += 1
        results['scoring_metrics']['net_pnl'] = {'passed': True}

    # S-2: Sharpe Ratio (B > A + 0.1)
    if challenger.sharpe > champion.sharpe + 0.1:
        wins += 1
        results['scoring_metrics']['sharpe'] = {'passed': True}

    # S-3: Profit Factor (B > A)
    if challenger.profit_factor > champion.profit_factor:
        wins += 1
        results['scoring_metrics']['profit_factor'] = {'passed': True}

    # S-4: Calmar Ratio (B > A)
    if challenger.calmar > champion.calmar:
        wins += 1
        results['scoring_metrics']['calmar'] = {'passed': True}

    # S-5: 胜率 (B > A，当交易次数足够时)
    if challenger.trades_count >= 20 and champion.trades_count >= 20:
        if challenger.win_rate > champion.win_rate:
            wins += 1
            results['scoring_metrics']['win_rate'] = {'passed': True}

    # ═══════════════════════════════════════════════════════════════════════
    # 约束项 (硬门槛，不计入 wins)
    # ═══════════════════════════════════════════════════════════════════════

    # C-1: ES95 (Expected Shortfall) - B <= A * 1.1
    es95_ok = challenger.es95 <= champion.es95 * 1.1

    # C-2: MaxDD - 允许轻微更差(1.05)，但必须 Calmar 补偿
    if challenger.max_drawdown <= champion.max_drawdown * 1.05:
        maxdd_ok = True
    elif challenger.max_drawdown <= champion.max_drawdown * 1.10:
        maxdd_ok = challenger.calmar >= champion.calmar
    else:
        maxdd_ok = False

    # ═══════════════════════════════════════════════════════════════════════
    # 总判定
    # ═══════════════════════════════════════════════════════════════════════

    wins_required = 3
    passed = (wins >= wins_required) and es95_ok and maxdd_ok

    results['summary'] = {
        'wins': wins,
        'wins_required': wins_required,
        'es95_ok': es95_ok,
        'maxdd_ok': maxdd_ok,
        'passed': passed
    }

    return passed, results
```

---

## 4. Evidence Gate - Bootstrap 日 PnL 对齐

### 4.1 设计原理

Bootstrap 使用日 PnL 差值序列（自然对齐时间），而非逐笔对齐（不可执行）。

### 4.2 实现

```python
def evidence_gate(
    champion: StrategyResults,
    challenger: StrategyResults,
    historical_cycles: List[ABTestCycle],
    regime_summary: RegimeSummary
) -> Tuple[bool, Dict]:
    """
    使用日 PnL 差值进行 Bootstrap
    """
    results = {}

    # E-1: 最小样本量门槛
    MIN_TRADES = 30
    MIN_ACTIVE_DAYS = 15

    sample_ok = (challenger.trades_count >= MIN_TRADES or
                 challenger.active_days >= MIN_ACTIVE_DAYS)

    if not sample_ok:
        return False, {'verdict': 'INSUFFICIENT_SAMPLE'}

    # E-2: 分段稳健性 (用日 PnL)
    aligned_daily = align_daily_pnl(champion.daily_pnl, challenger.daily_pnl)
    n_days = len(aligned_daily)
    segment_size = n_days // 4

    positive_segments = 0
    for i in range(4):
        start_idx = i * segment_size
        end_idx = (i + 1) * segment_size if i < 3 else n_days
        segment_data = aligned_daily[start_idx:end_idx]
        champ_pnl = sum(d[1] for d in segment_data)
        chall_pnl = sum(d[2] for d in segment_data)
        if chall_pnl > champ_pnl:
            positive_segments += 1

    segment_ok = positive_segments >= 3
    if not segment_ok:
        return False, {'verdict': 'SEGMENT_ROBUSTNESS_FAILED'}

    # E-3: Bootstrap (用日 PnL 差值)
    daily_pnl_diff = np.array([d[2] - d[1] for d in aligned_daily])

    n_bootstrap = 1000
    bootstrap_means = []
    for _ in range(n_bootstrap):
        sample_indices = np.random.choice(n_days, size=n_days, replace=True)
        sample = daily_pnl_diff[sample_indices]
        bootstrap_means.append(np.mean(sample))

    ci_lower = np.percentile(bootstrap_means, 2.5)
    bootstrap_ok = ci_lower > 0

    if not bootstrap_ok:
        return False, {'verdict': 'BOOTSTRAP_FAILED', 'ci_lower': ci_lower}

    # E-4: 跨 Regime 重复性
    recent_wins = get_recent_consecutive_wins(challenger.id, historical_cycles)
    if len(recent_wins) >= 3:
        regimes_in_wins = [cycle.regime_summary.primary_regime for cycle in recent_wins[-3:]]
        unique_regimes = len(set(regimes_in_wins))
        regime_diversity_ok = unique_regimes >= 2
    else:
        regime_diversity_ok = False

    # 总判定
    promote_ready = all([sample_ok, segment_ok, bootstrap_ok, regime_diversity_ok])

    if promote_ready:
        return True, {'verdict': 'PROMOTE_READY'}
    else:
        return False, {'verdict': 'KEEP_OBSERVING'}


def align_daily_pnl(
    champion_daily: Dict[date, float],
    challenger_daily: Dict[date, float]
) -> List[Tuple[date, float, float]]:
    """对齐两个策略的日 PnL，只保留两者都有数据的日期"""
    common_dates = sorted(set(champion_daily.keys()) & set(challenger_daily.keys()))
    return [(d, champion_daily[d], challenger_daily[d]) for d in common_dates]
```

---

## 5. Regime 硬定义

### 5.1 设计原理

Regime 定义必须跨实现一致，使用硬编码阈值：
- **波动率**: ATR 相对历史的分位数
- **趋势**: ADX 绝对阈值

### 5.2 定义

```python
@dataclass
class RegimeDefinition:
    """Regime 定义规范 - 跨实现必须一致"""

    # 波动率 Regime 阈值
    VOL_HIGH_PERCENTILE = 0.70   # ATR >= 70%分位 → high
    VOL_LOW_PERCENTILE = 0.30    # ATR <= 30%分位 → low

    # 趋势 Regime 阈值
    TREND_ADX_THRESHOLD = 25     # ADX >= 25 → trending


# 所有可能的 Primary Regime 枚举
VALID_PRIMARY_REGIMES = [
    "high_trending",
    "high_ranging",
    "mid_trending",
    "mid_ranging",
    "low_trending",
    "low_ranging"
]
```

### 5.3 计算

```python
def calculate_regime(market_data: MarketData, lookback_days: int = 90) -> RegimeSummary:
    """
    计算当前 Regime

    输出:
    - vol_regime: "high" | "mid" | "low"
    - trend_regime: "trending" | "ranging"
    - primary_regime: "{vol_regime}_{trend_regime}"
    """
    # 波动率 Regime
    atr_series = market_data.get_atr("BTCUSDT", period=14, lookback=lookback_days)
    current_atr = atr_series[-1]
    atr_percentile = percentileofscore(atr_series, current_atr) / 100.0

    if atr_percentile >= RegimeDefinition.VOL_HIGH_PERCENTILE:
        vol_regime = "high"
    elif atr_percentile <= RegimeDefinition.VOL_LOW_PERCENTILE:
        vol_regime = "low"
    else:
        vol_regime = "mid"

    # 趋势 Regime
    adx = market_data.get_adx("BTCUSDT", period=14)
    trend_regime = "trending" if adx >= RegimeDefinition.TREND_ADX_THRESHOLD else "ranging"

    # 组合
    primary_regime = f"{vol_regime}_{trend_regime}"

    return RegimeSummary(
        vol_regime=vol_regime,
        trend_regime=trend_regime,
        primary_regime=primary_regime,
        atr_percentile=atr_percentile,
        adx=adx,
        calculated_at=datetime.utcnow()
    )
```

---

## 6. UCB 预算分配 - 虚拟先验

### 6.1 设计原理

新策略使用虚拟先验 `(n=1, mean=0)` 替代 `inf`，避免不稳定的探索行为。

### 6.2 实现

```python
class BudgetAllocator:
    """UCB 预算分配器"""

    def __init__(self, config: BudgetConfig):
        self.champion_min_budget = 0.50
        self.challenger_max_budget = 0.25
        self.challenger_min_budget = 0.05
        self.exploration_factor = 1.0

        # 虚拟先验参数
        self.prior_n_cycles = 1          # 新策略视为有 1 轮数据
        self.prior_mean_return = 0.0     # 新策略先验收益为 0

    def allocate(
        self,
        champion: Strategy,
        challengers: List[Strategy],
        historical_performance: Dict[str, List[float]]
    ) -> Dict[str, float]:
        """返回: {strategy_id: risk_budget}"""

        if not challengers:
            return {champion.id: 1.0}

        allocations = {champion.id: self.champion_min_budget}
        remaining_budget = 1.0 - self.champion_min_budget

        total_cycles = max(sum(len(v) for v in historical_performance.values()), 1)

        # 计算 UCB 分数 (用虚拟先验)
        ucb_scores = {}
        for c in challengers:
            perf = historical_performance.get(c.id, [])

            if len(perf) == 0:
                n_cycles = self.prior_n_cycles
                mean_return = self.prior_mean_return
            else:
                n_cycles = len(perf)
                mean_return = np.mean(perf)

            exploration_bonus = self.exploration_factor * np.sqrt(
                np.log(total_cycles + 1) / n_cycles
            )
            ucb_scores[c.id] = mean_return + exploration_bonus

        # Softmax 归一化
        scores = np.array([ucb_scores[c.id] for c in challengers])
        scores = np.clip(scores, -5, 5)
        exp_scores = np.exp(scores - np.max(scores))
        weights = exp_scores / np.sum(exp_scores)

        # 分配并 clamp
        for i, c in enumerate(challengers):
            raw_budget = remaining_budget * weights[i]
            clamped = max(self.challenger_min_budget,
                         min(raw_budget, self.challenger_max_budget))
            allocations[c.id] = clamped

        # 归一化
        total = sum(allocations.values())
        return {k: v / total for k, v in allocations.items()}
```

---

## 7. v1.1.1 实现一致性补丁

> 本节修复 v1.1 中 6 个可能导致生产环境风控误判的边界问题。
> 所有修改向后兼容，无需改动 v1.1 主体逻辑。

### 7.A 相关性矩阵输入形状

**问题**: `np.corrcoef(returns)` 把每一行当变量，如果 returns 形状错误会得到错误的相关矩阵。

**硬规范**:
- `returns.shape` 必须是 `(n_symbols, window)`
- `returns` 必须是 symbol log returns（标的对数收益率），不是策略 PnL
- 若数据源返回 `(window, n_symbols)`，必须转置后再使用

```python
def _fetch_hourly_returns(self, symbols: List[str], window: int) -> np.ndarray:
    """
    获取小时级收益率矩阵

    Returns:
        np.ndarray: shape = (n_symbols, window)
                    returns[i] 是第 i 个 symbol 的收益率序列
    """
    raw_returns = self._data_source.get_hourly_returns(symbols, window)

    # v1.1.1: 强制形状检查
    if raw_returns.shape[0] == window and raw_returns.shape[1] == len(symbols):
        raw_returns = raw_returns.T

    assert raw_returns.shape == (len(symbols), window), (
        f"returns shape 必须是 (n_symbols, window), "
        f"got {raw_returns.shape}, expected ({len(symbols)}, {window})"
    )

    return raw_returns
```

### 7.B 波动率口径统一

**问题**: 波动率换算公式注释误导，`ddof` 不一致会导致跨实现偏差。

**硬规范**:
- `sigma_vec` 必须是日波动率
- 若输入是 1h returns: `sigma_daily = std(returns, ddof=1) * sqrt(24)`
- `ddof` 固定为 1（样本标准差）

```python
def _calculate_daily_volatility(self, hourly_returns: np.ndarray) -> np.ndarray:
    """
    从小时收益率计算日波动率

    Args:
        hourly_returns: shape = (n_symbols, window)，1h 对数收益率

    Returns:
        np.ndarray: shape = (n_symbols,)，日波动率
    """
    # v1.1.1: 固定 ddof=1，换算到日波动（不是年化）
    hourly_std = np.std(hourly_returns, axis=1, ddof=1)
    daily_vol = hourly_std * np.sqrt(24)  # 1h → 日波动
    return daily_vol
```

### 7.C fast/accurate volatility 同源

**问题**: 快口径和准口径使用不同的波动率来源，导致阈值语义不一致。

**硬规范**:
- `fast_risk` 的 volatility 默认取与 accurate 同源的 `sigma_daily(symbol)`
- 仅当 `sigma_daily` 不可用时，才 fallback 到 ATR%

```python
def _get_symbol_volatility(self, symbol: str) -> float:
    """
    获取单品种日波动率 - v1.1.1 同源规范

    优先级:
    1. 缓存的历史波动率 (与 accurate risk 同源)
    2. ATR% fallback (当历史数据不足时)
    """
    # v1.1.1: 优先使用与 accurate risk 同源的波动率
    if self._volatilities and symbol in self._volatilities:
        return self._volatilities[symbol]

    # Fallback: 使用 ATR%
    atr_percent = self._data_source.get_atr_percent(symbol, period=14)
    if atr_percent is not None:
        return atr_percent

    # 最终 fallback: 保守默认值
    return 0.03  # 3% 日波动作为保守估计
```

### 7.D Evidence 分段下限

**问题**: `segment_size = n_days // 4` 在 `n_days < 8` 时会出现空段。

**硬规范**:
- `aligned_days < 20`: 返回 `INSUFFICIENT_SAMPLE`
- `aligned_days >= 20 且 < 40`: 使用 2 段规则 (正段数 >= 2)
- `aligned_days >= 40`: 使用 4 段规则 (正段数 >= 3)

```python
MIN_DAYS_FOR_SEGMENT = 20
MIN_DAYS_FOR_4_SEGMENTS = 40

def _check_segment_robustness(
    aligned_daily: List[Tuple[date, float, float]]
) -> Tuple[bool, Dict]:
    """分段稳健性检查 - v1.1.1 修订版"""
    n_days = len(aligned_daily)

    # v1.1.1: 硬门槛检查
    if n_days < MIN_DAYS_FOR_SEGMENT:
        return False, {
            'error': 'INSUFFICIENT_DAYS_FOR_SEGMENT',
            'n_days': n_days,
            'min_required': MIN_DAYS_FOR_SEGMENT
        }

    # v1.1.1: 根据天数选择分段数
    if n_days >= MIN_DAYS_FOR_4_SEGMENTS:
        n_segments = 4
        required_positive = 3
    else:
        n_segments = 2
        required_positive = 2

    segment_size = n_days // n_segments
    positive_segments = 0

    for i in range(n_segments):
        start_idx = i * segment_size
        end_idx = (i + 1) * segment_size if i < n_segments - 1 else n_days
        segment_data = aligned_daily[start_idx:end_idx]

        if len(segment_data) == 0:
            continue

        champ_pnl = sum(d[1] for d in segment_data)
        chall_pnl = sum(d[2] for d in segment_data)

        if chall_pnl > champ_pnl:
            positive_segments += 1

    return positive_segments >= required_positive, {
        'n_segments': n_segments,
        'positive_count': positive_segments,
        'required': required_positive
    }
```

### 7.E Budget 分配改水位法

**问题**: clamp 回填逻辑会导致总预算不等于 `remaining_budget`，归一化后会意外稀释 Champion 占比。

**硬规范**:
- Champion 不低于 `CHAMPION_ABSOLUTE_FLOOR` (0.40)
- 分配顺序: min → 按权重分配 → max 封顶 → 继续分配剩余
- 最终归一化不得稀释 Champion 到 floor 以下

```python
CHAMPION_ABSOLUTE_FLOOR = 0.40

class BudgetAllocatorV1_1_1:
    """v1.1.1 水位法预算分配"""

    def __init__(self, config: BudgetConfig):
        self.champion_min_budget = 0.50
        self.champion_absolute_floor = CHAMPION_ABSOLUTE_FLOOR
        self.challenger_max_budget = 0.25
        self.challenger_min_budget = 0.05
        self.exploration_factor = 1.0
        self.prior_n_cycles = 1
        self.prior_mean_return = 0.0

    def allocate(
        self,
        champion: Strategy,
        challengers: List[Strategy],
        historical_performance: Dict[str, List[float]]
    ) -> Dict[str, float]:
        """
        水位法预算分配

        流程:
        1. Champion 固定 champion_min_budget
        2. 每个 Challenger 先分配 min_budget
        3. 剩余预算按 UCB softmax 权重分配
        4. 遇到 max 封顶，剩余继续分配给未封顶者
        5. 若总预算超支，从 Champion 扣除（但不低于 floor）
        """
        if not challengers:
            return {champion.id: 1.0}

        # Step 1: Champion 固定预算
        allocations = {champion.id: self.champion_min_budget}

        # Step 2: 每个 Challenger 先分配 min
        for c in challengers:
            allocations[c.id] = self.challenger_min_budget

        used_budget = sum(allocations.values())
        remaining_budget = 1.0 - used_budget

        if remaining_budget < 0:
            shortfall = -remaining_budget
            new_champion_budget = max(
                self.champion_min_budget - shortfall,
                self.champion_absolute_floor
            )
            allocations[champion.id] = new_champion_budget
            remaining_budget = 0

        if remaining_budget <= 0:
            total = sum(allocations.values())
            return {k: v / total for k, v in allocations.items()}

        # Step 3: 计算 UCB 权重
        total_cycles = max(sum(len(v) for v in historical_performance.values()), 1)

        ucb_scores = {}
        for c in challengers:
            perf = historical_performance.get(c.id, [])
            if len(perf) == 0:
                n_cycles = self.prior_n_cycles
                mean_return = self.prior_mean_return
            else:
                n_cycles = len(perf)
                mean_return = np.mean(perf)

            exploration_bonus = self.exploration_factor * np.sqrt(
                np.log(total_cycles + 1) / n_cycles
            )
            ucb_scores[c.id] = mean_return + exploration_bonus

        scores = np.array([ucb_scores[c.id] for c in challengers])
        scores = np.clip(scores, -5, 5)
        exp_scores = np.exp(scores - np.max(scores))
        weights = exp_scores / np.sum(exp_scores)
        weight_map = {c.id: weights[i] for i, c in enumerate(challengers)}

        # Step 4: 水位法分配
        headroom = {c.id: self.challenger_max_budget - allocations[c.id] for c in challengers}

        for _ in range(len(challengers) + 1):
            if remaining_budget <= 1e-9:
                break

            eligible = [cid for cid, h in headroom.items() if h > 1e-9]
            if not eligible:
                break

            eligible_weight_sum = sum(weight_map[cid] for cid in eligible)
            if eligible_weight_sum <= 1e-9:
                share = remaining_budget / len(eligible)
                for cid in eligible:
                    add = min(share, headroom[cid])
                    allocations[cid] += add
                    headroom[cid] -= add
                    remaining_budget -= add
            else:
                for cid in eligible:
                    normalized_weight = weight_map[cid] / eligible_weight_sum
                    desired_add = remaining_budget * normalized_weight
                    actual_add = min(desired_add, headroom[cid])
                    allocations[cid] += actual_add
                    headroom[cid] -= actual_add
                    remaining_budget -= actual_add

        # Step 5: 确保 Champion 不被稀释
        total = sum(allocations.values())
        allocations = {k: v / total for k, v in allocations.items()}

        if allocations[champion.id] < self.champion_absolute_floor:
            allocations[champion.id] = self.champion_absolute_floor
            challenger_total = 1.0 - self.champion_absolute_floor
            challenger_sum = sum(allocations[c.id] for c in challengers)
            if challenger_sum > 0:
                scale = challenger_total / challenger_sum
                for c in challengers:
                    allocations[c.id] *= scale

        return allocations
```

### 7.F Accurate risk 的方向定义

**问题**: `r` 向量的方向和 `C` 矩阵的来源如果不一致，组合方差中的对冲效应会计算错误。

**硬规范**:
- `r_i` 的符号代表仓位方向: long = +, short = -
- `C` (相关性矩阵) 必须来自标的收益率 (symbol log returns)
- 不得使用策略 PnL 收益率计算 `C`

```python
def calc_accurate_risk(self, positions: List[Position], equity: float) -> float:
    """
    准口径组合风险 - v1.1.1 修订

    数学定义:
    - r_i = (|size × price| / equity) × leverage × sign(size)
    - Σ = diag(σ_daily) × C × diag(σ_daily)
    - C 来自 symbol log returns (不是策略 PnL)
    - σ_p = sqrt(r^T × Σ × r)
    - risk_ratio = σ_p / target_vol
    """
    if not positions:
        return 0.0

    symbols = [p.symbol for p in positions]
    self._maybe_update_correlation_matrix(symbols)

    # v1.1.1: 风险暴露向量 (方向定义明确)
    r = np.array([
        (abs(p.size * p.mark_price) / equity) * p.leverage * np.sign(p.size)
        for p in positions
    ])

    # v1.1.1: sigma_vec 必须是日波动率
    sigma_vec = np.array([self._volatilities.get(s, 0.03) for s in symbols])

    # C 来自 symbol log returns
    C = self._get_correlation_submatrix(symbols)

    # Σ = diag(σ) @ C @ diag(σ)
    cov_matrix = np.diag(sigma_vec) @ C @ np.diag(sigma_vec)

    # 组合波动
    portfolio_var = r.T @ cov_matrix @ r
    portfolio_vol = np.sqrt(max(portfolio_var, 0))

    return portfolio_vol / self.target_portfolio_vol
```

---

## 变更历史

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| v1.1.1 | 2026-01-22 | 实现一致性补丁 (A-F): 形状检查、ddof 固定、vol 同源、分段下限、水位法、方向定义 |
| v1.1 | 2026-01-22 | 初版生产规范: 双口径风险、P95 Risk Parity、评分/约束分离、日 PnL Bootstrap、Regime 硬定义、UCB 虚拟先验 |

---

## v1.1.1 变更汇总表

| 补丁 | 原问题 | v1.1.1 修订 |
|------|--------|-------------|
| A | `np.corrcoef` 维度可能错 | 强制 `returns.shape = (n_symbols, window)` |
| B | 波动率口径注释误导 | 固定 `ddof=1`, 明确"换算到日波动" |
| C | fast/accurate vol 不同源 | fast 优先用 accurate 同源 `sigma_daily` |
| D | `segment_size=0` 导致空段 | `n_days<20` 拒绝; `<40` 用2段; `>=40` 用4段 |
| E | clamp 回填导致比例跳变 | 水位法 + champion 绝对下限 0.40 |
| F | r/C 方向定义可能混淆 | 硬定义: r 用仓位方向, C 用标的收益率 |
