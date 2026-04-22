import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DecisionCard } from './DecisionCard'
import type { DecisionRecord } from '../../types'

const baseDecision: DecisionRecord = {
  timestamp: '2026-04-22T12:14:19Z',
  cycle_number: 4796,
  system_prompt: '',
  input_prompt: '',
  cot_trace: '',
  decision_json: '',
  raw_response: '',
  candidate_coins: [],
  account_snapshot: {
    total_balance: 68.44,
    available_balance: 28.32,
    total_unrealized_profit: 0,
    position_count: 1,
    margin_used_pct: 58.6,
  },
  decisions: [
    {
      action: 'open_long',
      symbol: 'TRUMPUSDT',
      quantity: 0,
      leverage: 1,
      price: 0,
      stop_loss: 2.984,
      take_profit: 3.061,
      confidence: 78,
      order_id: 0,
      timestamp: '2026-04-22T12:14:16Z',
      success: false,
      error: 'regime filter blocked open_long for TRUMPUSDT',
      review_context: {
        timeframe_context: { primary: '15m', lower: ['5m'], higher: ['1h'] },
        key_levels: {
          support: [2.91],
          resistance: [3.06],
          fibonacci: { swing_high: 3.12, swing_low: 2.78, levels: [2.91, 2.99, 3.06] },
        },
        anchors: [{ type: 'resistance', timeframe: '15m', price: 3.06, reason: 'local rejection' }],
        control: {
          decision: 'rejected',
          original_action: 'open_long',
          final_action: 'open_long',
          reasons: ['trend alignment failed for open_long under regime gate'],
          failed_checks: ['trend_misaligned'],
          regime_current: 'choppy',
          regime_allowed: ['trend', 'breakout'],
          regime_primary_timeframe: '15m',
          regime_atr14_pct: 2.31,
          regime_trend_aligned: false,
          no_order_placed: true,
        },
      },
    },
  ],
  success: true,
}

describe('DecisionCard', () => {
  it('renders compact audit and regime context badges for rejected actions', () => {
    render(<DecisionCard decision={baseDecision} language="en" />)

    expect(screen.getByText('rejected')).toBeInTheDocument()
    expect(screen.getAllByText('no order placed').length).toBeGreaterThan(0)
    expect(screen.getByText(/failed · trend misaligned/i)).toBeInTheDocument()
    expect(screen.getByText(/regime choppy/i)).toBeInTheDocument()
    expect(screen.getByText(/allowed trend/i)).toBeInTheDocument()
    expect(screen.getByText(/allowed breakout/i)).toBeInTheDocument()
    expect(screen.getAllByText(/trend misaligned/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/ATR 2.31%/i)).toBeInTheDocument()
    expect(screen.getAllByText(/tf 15m/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/fib 3 levels/i)).toBeInTheDocument()
    expect(screen.getByText(/anchors 1/i)).toBeInTheDocument()
  })
})
