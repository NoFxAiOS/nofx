import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('../../lib/api', () => ({
  api: {
    getOpenOrders: vi.fn(async () => []),
  },
}))

import { PositionProtectionPanel } from './PositionProtectionPanel'
import type { Position } from '../../types'

describe('PositionProtectionPanel degradation summary', () => {
  it('renders ladder/full/fallback degradation summary concisely', async () => {
    const positions: Position[] = [{
      symbol: 'BTCUSDT',
      side: 'long',
      entry_price: 100,
      mark_price: 104,
      quantity: 1,
      leverage: 5,
      unrealized_pnl: 4,
      unrealized_pnl_pct: 4,
      liquidation_price: 70,
      margin_used: 20,
      protection_state: 'exchange_protection_verified',
      break_even_state: 'idle',
      drawdown_execution_mode: 'native_partial_trailing',
      protection_runtime: {
        current_pnl_pct: 4,
        drawdown_peak_pnl_pct: 6,
        current_drawdown_pct: 1.2,
        drawdown_config_source: 'strategy',
        current_drawdown_stage_min_profit_pct: 3,
        current_drawdown_stage_rule_count: 1,
        planned_ladder_stop_count: 2,
        planned_ladder_take_profit_count: 2,
        live_ladder_stop_count: 0,
        live_ladder_take_profit_count: 1,
        live_full_stop_count: 1,
        live_full_take_profit_count: 0,
        fallback_order_detected: true,
        live_fallback_stop_count: 1,
        full_stop_planned: false,
        full_take_profit_planned: false,
        fallback_planned: true,
        ladder_stop_degraded: true,
        ladder_take_profit_degraded: true,
        ladder_stop_degraded_to_full: true,
        ladder_take_profit_degraded_to_full: false,
        scheduled_tiers: [
          {
            index: 1,
            min_profit_pct: 3,
            max_drawdown_pct: 1,
            close_ratio_pct: 50,
            activation_price: 103,
            callback_rate: 0.4,
            planned_quantity: 0.5,
            source: 'native',
            execution_mode: 'native_partial_trailing',
            is_satisfied: true,
            is_triggered: true,
          },
        ],
      },
    }]

    render(
      <PositionProtectionPanel
        traderId="t-1"
        positions={positions}
        language="en"
        exchange="okx"
      />
    )

    expect(await screen.findByText('Ladder Stops')).toBeInTheDocument()
    expect(screen.getByText('0 / 2')).toBeInTheDocument()
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByText('SL→Full · TP partial · Fallback live')).toBeInTheDocument()
    expect(screen.getByText('Full/Fallback State')).toBeInTheDocument()
    expect(screen.getByText('Fallback planned:1')).toBeInTheDocument()
  })
})
