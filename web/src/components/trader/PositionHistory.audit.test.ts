import { describe, expect, it } from 'vitest'
import { getDecisionAuditSnapshot } from './PositionHistory'
import type { DecisionReviewRef } from '../../types'

function makeReview(policyStatus?: string, policyReasons?: string[]): DecisionReviewRef {
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
          control: {
            decision: 'accepted',
            effective_rr: 1.72,
            effective_rr_source: 'execution_recomputed_net',
            constraints_merged: true,
            runtime_rr_recomputed: true,
          },
          protection: policyStatus
            ? {
                policy_status: policyStatus,
                policy_override: policyStatus !== 'aligned',
                policy_rejected: policyStatus === 'rejected',
                policy_reasons: policyReasons,
              }
            : undefined,
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

  it('extracts compact control outcome fields when present', () => {
    const snap = getDecisionAuditSnapshot(makeReview())

    expect(snap.controlStatus).toEqual({ label: 'accepted', tone: 'neutral' })
    expect(snap.actionAudit).toBeNull()
    expect(snap.controlBadges).toEqual([
      { label: 'eff 1.72R · runtime net' },
      { label: 'constraints merged', tone: 'warn' },
      { label: 'runtime RR', tone: 'warn' },
    ])
    expect(snap.failedChecks).toEqual([])
  })

  it('keeps rejected control outcomes concise and marks no-order placement', () => {
    const review = makeReview()
    if (review.decisions?.[0].review_context) {
      review.decisions[0].review_context.control = {
        decision: 'rejected',
        effective_rr: 0.94,
        effective_rr_source: 'net',
        failed_checks: ['effective_rr_below_min', 'target_before_first_target'],
        no_order_placed: true,
      }
    }

    const snap = getDecisionAuditSnapshot(review)
    expect(snap.controlStatus).toEqual({ label: 'rejected', tone: 'danger' })
    expect(snap.controlBadges).toEqual([
      { label: 'eff 0.94R · net' },
      { label: 'no order placed', tone: 'danger' },
    ])
    expect(snap.failedChecks).toEqual(['effective rr below min', 'target before first target'])
  })

  it('formats action audit when original/final actions are present', () => {
    const review = makeReview()
    if (review.decisions?.[0].review_context) {
      review.decisions[0].review_context.control = {
        ...review.decisions[0].review_context.control,
        original_action: 'open_long',
        final_action: 'wait',
        decision: 'downgraded_to_wait',
        no_order_placed: true,
      }
    }

    const snap = getDecisionAuditSnapshot(review)
    expect(snap.controlStatus).toEqual({ label: 'downgraded to wait', tone: 'warn' })
    expect(snap.actionAudit).toBe('open long → wait')
  })

  it('preserves compact policy transparency fields on protection context', () => {
    const snap = getDecisionAuditSnapshot(makeReview('recomputed', ['stop_inside_invalidation']))

    expect(snap.ctx?.protection?.policy_status).toBe('recomputed')
    expect(snap.ctx?.protection?.policy_override).toBe(true)
    expect(snap.ctx?.protection?.policy_rejected).toBe(false)
    expect(snap.ctx?.protection?.policy_reasons).toEqual(['stop_inside_invalidation'])
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
