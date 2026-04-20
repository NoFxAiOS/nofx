import { describe, expect, it } from 'vitest'
import { getDecisionAuditSnapshot } from './PositionHistory'
import type { DecisionReviewRef } from '../../types'

function makeReview(): DecisionReviewRef {
  return {
    decision_record_id: 1,
    cycle_number: 12,
    timestamp: '2026-04-20T12:00:00Z',
    decisions: [
      {
        action: 'open_long',
        symbol: 'BTCUSDT',
        quantity: 1,
        leverage: 5,
        price: 84000,
        order_id: 1,
        timestamp: '2026-04-20T12:00:00Z',
        success: true,
        review_context: {
          primary_timeframe: '1h',
          min_risk_reward: 1.5,
          risk_reward: {
            gross_estimated_rr: 2.1,
            net_estimated_rr: 1.8,
            passed: true,
          },
          execution_constraints: {
            tick_size: 0.1,
            qty_step_size: 0.001,
            min_qty: 0.01,
            contract_value: 0.01,
            last_price: 84123.4,
            taker_fee_rate: 0.0005,
            estimated_slippage_bps: 1.2,
          },
        },
      },
    ],
  }
}

describe('getDecisionAuditSnapshot', () => {
  it('extracts compact execution constraint fields when present', () => {
    const snap = getDecisionAuditSnapshot(makeReview())

    expect(snap.executionConstraintItems.map((item) => item.label)).toEqual([
      'tick 0.1',
      'qty 0.001',
      'min 0.01',
      'ctVal 0.01',
      'last 84,123.4',
      'fee 0.050%',
      'slip 1.2bps',
    ])
  })

  it('omits execution constraint items when values are absent', () => {
    const review = makeReview()
    if (review.decisions?.[0].review_context?.execution_constraints) {
      review.decisions[0].review_context.execution_constraints = {}
    }

    const snap = getDecisionAuditSnapshot(review)
    expect(snap.executionConstraintItems).toEqual([])
  })
})
