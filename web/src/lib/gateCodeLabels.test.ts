import { describe, expect, it } from 'vitest'
import { gateCodeLabel } from './gateCodeLabels'

// 2026-10-10 per-candidate block reasons
describe('gateCodeLabel', () => {
  it('keeps numeric suffixes visible (zh)', () => {
    expect(gateCodeLabel('RR_MAX_0.24', 'zh').label).toBe('盈亏比 0.24')
    expect(gateCodeLabel('VENDOR_DIVERGENCE_-1.22', 'zh').label).toBe(
      '行情源偏离 -1.22%'
    )
    expect(gateCodeLabel('EMA20_STRETCH_58.4_GT_10', 'zh').label).toBe(
      '追高 58.4%'
    )
    expect(gateCodeLabel('RR_MAX_0.24', 'zh').raw).toBe('RR_MAX_0.24')
  })
  it('translates plain and en codes', () => {
    expect(gateCodeLabel('BTC_4H_DOWNTREND', 'zh').label).toBe('BTC 4H 下跌')
    expect(gateCodeLabel('MICRO_TREND_NOT_SHORT', 'en').label).toBe(
      'micro-trend not short'
    )
  })
  it('labels the 2026-10 anti-chase entry gates', () => {
    expect(gateCodeLabel('CHASE_EMA20_1H_7.4_GT_6', 'zh').label).toBe(
      '追高(1h EMA20 偏离 7.4% > 6%)'
    )
    expect(gateCodeLabel('COUNTER_EMA20_1H_3.8_GT_3', 'en').label).toBe(
      'counter-trend vs 1h EMA20 3.8% > 3%'
    )
    expect(gateCodeLabel('RSI1H_EXHAUSTED_81', 'zh').label).toBe(
      '1h RSI 极端 81'
    )
    expect(gateCodeLabel('CHASE_MOVE4H_9.2_GT_6', 'zh').label).toBe(
      '4h 已涨跌过大 9.2% > 6%'
    )
    expect(gateCodeLabel('CHASE_MOVE4H_9.2_GT_6', 'id').label).toBe(
      'pergerakan 4h terlalu besar 9.2% > 6%'
    )
    expect(gateCodeLabel('CHASE_EMA20_1H_7.4_GT_6', 'zh').raw).toBe(
      'CHASE_EMA20_1H_7.4_GT_6'
    )
  })
  it('renders unknown codes raw', () => {
    expect(gateCodeLabel('SOMETHING_NEW_3', 'zh').label).toBe('SOMETHING_NEW_3')
  })
})
