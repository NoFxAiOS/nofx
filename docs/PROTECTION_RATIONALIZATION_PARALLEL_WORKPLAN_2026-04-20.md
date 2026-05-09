# Protection Rationalization / Entry-Protection Alignment Parallel Work Plan

_Started: 2026-04-20 21:31 Asia/Shanghai_

## Mission

Advance nofxmax from protection execution hardening to entry/protection rationalization:

- Every `open_long` / `open_short` decision must carry structured key-level and RR rationale.
- Backend must validate direction, RR, executable prices, and protection-plan alignment before execution.
- Exchange/data-source capabilities should be analyzed so the system can use richer source data when available.
- Persistence/API/UI should make the rationale auditable without dumping long JSON.

## Current baseline

Existing data is enough for Phase 1/2:

- raw OHLCV K-lines
- primary timeframe and selected/adjacent timeframes
- EMA / RSI / ATR / BOLL
- OI / funding / quant/ranking data when enabled
- `risk_control.min_risk_reward_ratio`
- protection snapshots and review context persistence already exist
- protection capability matrix exists for native SL/TP/trailing execution

Main gap:

> Key levels, RR, source anchors, executable net-RR, and protection alignment are not yet structured/validated/persisted as first-class objects.

## Target contract fields

### AI output rationale

Candidate struct name:

- `AIRiskRewardRationale`, or
- `AIEntryProtectionRationale`

Suggested JSON shape:

```json
{
  "timeframe_context": {
    "primary": "15m",
    "lower": ["3m", "5m"],
    "higher": ["1h"]
  },
  "key_levels": {
    "support": [123.4, 121.8],
    "resistance": [128.0, 130.5],
    "swing_highs": [129.2],
    "swing_lows": [118.4],
    "fibonacci": {
      "swing_high": 129.2,
      "swing_low": 118.4,
      "levels": [0.382, 0.5, 0.618]
    }
  },
  "volatility_adjustment": {
    "atr14_pct": 1.8,
    "boll_width_pct": 2.4,
    "market_regime": "trend_up",
    "widening_pct": 0.25
  },
  "risk_reward": {
    "entry": 124.0,
    "invalidation": 121.6,
    "first_target": 128.8,
    "gross_estimated_rr": 2.0,
    "net_estimated_rr": 1.85,
    "min_required_rr": 1.5,
    "passed": true
  },
  "execution_constraints": {
    "tick_size": 0.1,
    "price_precision": 1,
    "qty_step_size": 0.001,
    "qty_precision": 3,
    "min_qty": 0.001,
    "min_notional": 5,
    "mark_price": 123.45,
    "last_price": 123.48,
    "index_price": 123.42,
    "best_bid": 123.4,
    "best_ask": 123.5,
    "spread_bps": 8.1,
    "taker_fee_rate": 0.0005,
    "maker_fee_rate": 0.0002,
    "estimated_slippage_bps": 5
  },
  "derivatives_context": {
    "oi_current": 123000000,
    "oi_delta_5m_pct": 0.5,
    "oi_delta_15m_pct": 1.1,
    "oi_delta_1h_pct": 3.2,
    "funding_rate_current": 0.0001,
    "funding_rate_avg_8h": 0.00008,
    "mark_index_basis_bps": 3.5,
    "premium_index": 0.0002,
    "orderbook_imbalance": 0.12,
    "top5_bid_notional": 2200000,
    "top5_ask_notional": 1900000
  },
  "anchors": [
    {
      "type": "support",
      "timeframe": "15m",
      "price": 123.4,
      "reason": "primary breakout retest"
    }
  ],
  "alignment_notes": [
    "full stop is beyond invalidation side",
    "break-even trigger is before first target"
  ]
}
```

## Validation rules

Required for `open_long` / `open_short`:

- rationale present
- `risk_reward.entry > 0`
- `risk_reward.invalidation > 0`
- `risk_reward.first_target > 0`
- `gross_estimated_rr > 0`
- prefer gate on `net_estimated_rr` when present, else gross fallback

Direction sanity:

- long: `invalidation < entry`, `first_target > entry`
- short: `invalidation > entry`, `first_target < entry`

RR gate:

- use `strategy.risk_control.min_risk_reward_ratio`
- if absent/zero, default proposal: `1.5`
- reject or downgrade open action if RR below min

Execution sanity:

- round entry/invalidation/target by tick size when constraints are present
- recompute or verify net RR after rounding/fee/slippage
- verify stop/tp/protection quantities remain executable under min qty/min notional

Protection alignment sanity:

- Full/Ladder stop should be on or beyond invalidation side
- TP / first ladder target should not contradict first target
- Break-even trigger should not be beyond first target
- Drawdown first profit gate should not be trivial noise relative to ATR/BOLL
- Fallback max-loss should not be looser than an extreme invalidation envelope unless explicitly justified

## Parallel tracks

### Track A — Kernel contract / prompt / parser / validation

Owner: subagent-kernel

Inspect and propose/implement:

- `kernel/engine.go`
- `kernel/engine_analysis.go`
- `kernel/engine_prompt.go`
- `kernel/prompt_builder.go`
- `kernel/protection_reasoning_contract.go`
- parser and negative tests for decisions

Deliverables:

1. exact struct placement for rationale under `Decision`
2. prompt/schema changes requiring rationale on open actions
3. validator function design and tests:
   - missing rationale rejects/downgrades open
   - wrong direction rejects/downgrades
   - RR below min rejects/downgrades
   - empty/wait decisions remain legal
4. notes on backwards compatibility

### Track B — Exchange/data-source capability analysis

Owner: subagent-exchange-data

Inspect and propose/implement:

- current trader interfaces
- exchange adapters for Binance/OKX/Bitget/Bybit/Gate/KuCoin/Hyperliquid/Aster/Lighter
- market data collection pipeline
- existing `ProtectionCapabilities`

Deliverables:

1. `MarketDataCapabilities` design
2. current-source capability matrix:
   - OHLCV / multi-timeframe
   - mark/index/last/bid/ask/spread
   - instrument filters tick/step/minNotional
   - fee schedule or configured fee fallback
   - OI current/history
   - funding/premium/basis
   - order book depth/imbalance
   - liquidation/taker flow if available
3. minimum implementation proposal for execution constraints
4. degraded-mode rules when data is missing

### Track C — Persistence / API / UI audit surface

Owner: subagent-audit-ui

Inspect and propose/implement:

- `store/decision.go`
- decision review context write path
- `ProtectionSnapshot`
- `trader/auto_trader_loop.go`
- `api/handler_order.go`
- web types/components:
  - `web/src/types/trading.ts`
  - `web/src/components/trader/PositionProtectionPanel.tsx`
  - `web/src/components/trader/PositionHistory.tsx`

Deliverables:

1. snapshot/review_context field design
2. API compatibility plan
3. compact UI display plan:
   - RR badge
   - min RR / passed
   - primary timeframe
   - support/resistance chips
   - net RR if available
   - source/anchor expandable details
4. tests/types update list

## Coordination rules

- Prefer incremental implementation. Do not destabilize existing protection execution.
- Keep `[]` no-trade decisions legal.
- Backward compatibility matters: old records and old AI responses must not break history APIs.
- If implementing in code, add tests before broad rewrites.
- If unsure whether to reject or downgrade, default to downgrade-to-wait for AI contract failures unless the existing execution path clearly expects hard errors.

## Final integration plan

After subagent outputs:

1. Merge Track A schema/validator first.
2. Add minimal execution constraints from Track B, with capability-aware optional fields.
3. Persist minimal rationale in Track C.
4. Add tests and run `go test ./...`.
5. Commit docs + code in coherent commits.
