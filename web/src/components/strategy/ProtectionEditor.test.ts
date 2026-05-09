import { describe, expect, it } from 'vitest'
import { defaultProtectionConfig, normalizeProtectionConfig } from './ProtectionEditor'

describe('normalizeProtectionConfig', () => {
  it('preserves ladder AI mode and value-source AI modes', () => {
    const config = normalizeProtectionConfig({
      ladder_tp_sl: {
        ...defaultProtectionConfig.ladder_tp_sl,
        enabled: true,
        mode: 'ai',
        take_profit_enabled: true,
        stop_loss_enabled: true,
        take_profit_price: { mode: 'ai', value: 0 },
        take_profit_size: { mode: 'ai', value: 0 },
        stop_loss_price: { mode: 'ai', value: 0 },
        stop_loss_size: { mode: 'ai', value: 0 },
      },
    })

    expect(config.ladder_tp_sl.mode).toBe('ai')
    expect(config.ladder_tp_sl.take_profit_price.mode).toBe('ai')
    expect(config.ladder_tp_sl.take_profit_size.mode).toBe('ai')
    expect(config.ladder_tp_sl.stop_loss_price.mode).toBe('ai')
    expect(config.ladder_tp_sl.stop_loss_size.mode).toBe('ai')
  })
})
