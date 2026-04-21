import { describe, expect, it } from 'vitest'
import { buildStrategySavePayload } from './StrategyStudioPage'
import type { StrategyConfig } from '../types'

describe('buildStrategySavePayload', () => {
  it('preserves protection mode/value shape for AI configs', () => {
    const editingConfig: StrategyConfig = {
      language: 'zh',
      strategy_type: 'ai_trading',
      coin_source: { source_type: 'static', static_coins: ['BTCUSDT'], use_ai500: false, use_oi_top: false, use_oi_low: false },
      indicators: { klines: { primary_timeframe: '15m', primary_count: 25, enable_multi_timeframe: false }, enable_raw_klines: true },
      risk_control: { max_positions: 1, btc_eth_max_leverage: 1, altcoin_max_leverage: 1, max_margin_usage: 0.9, min_position_size: 10, min_risk_reward_ratio: 2, min_confidence: 70 },
      protection: {
        full_tp_sl: {
          enabled: true,
          mode: 'ai',
          take_profit: { mode: 'ai', value: 0 },
          stop_loss: { mode: 'ai', value: 0 },
          fallback_max_loss: { mode: 'disabled', value: 0 },
        },
        ladder_tp_sl: {
          enabled: true,
          mode: 'ai',
          take_profit_enabled: true,
          stop_loss_enabled: true,
          take_profit_price: { mode: 'ai', value: 0 },
          take_profit_size: { mode: 'ai', value: 0 },
          stop_loss_price: { mode: 'ai', value: 0 },
          stop_loss_size: { mode: 'ai', value: 0 },
          fallback_max_loss: { mode: 'disabled', value: 0 },
          rules: [],
        },
        drawdown_take_profit: { enabled: false, rules: [] },
        break_even_stop: { enabled: false, trigger_mode: 'profit_pct', trigger_value: 1, offset_pct: 0.1 },
        regime_filter: { enabled: false, allowed_regimes: ['narrow', 'standard', 'wide'], block_high_funding: false, max_funding_rate_abs: 0.01, block_high_volatility: false, max_atr14_pct: 3, require_trend_alignment: false },
      },
    }

    const payload = buildStrategySavePayload({
      name: '策略1',
      description: '',
      is_public: false,
      config_visible: true,
    }, editingConfig, 'zh')

    expect(payload.config.protection.full_tp_sl.mode).toBe('ai')
    expect(payload.config.protection.full_tp_sl.take_profit).toEqual({ mode: 'ai', value: 0 })
    expect(payload.config.protection.full_tp_sl.stop_loss).toEqual({ mode: 'ai', value: 0 })
    expect(payload.config.protection.ladder_tp_sl.mode).toBe('ai')
    expect(payload.config.protection.ladder_tp_sl.take_profit_price).toEqual({ mode: 'ai', value: 0 })
    expect(payload.config.protection.ladder_tp_sl.take_profit_size).toEqual({ mode: 'ai', value: 0 })
    expect(payload.config.protection.ladder_tp_sl.stop_loss_price).toEqual({ mode: 'ai', value: 0 })
    expect(payload.config.protection.ladder_tp_sl.stop_loss_size).toEqual({ mode: 'ai', value: 0 })
  })

  it('defaults strategy control policy mode to strict when omitted', () => {
    const editingConfig: StrategyConfig = {
      language: 'en',
      strategy_type: 'ai_trading',
      coin_source: { source_type: 'static', static_coins: ['BTCUSDT'], use_ai500: false, use_oi_top: false, use_oi_low: false },
      indicators: { klines: { primary_timeframe: '15m', primary_count: 25, enable_multi_timeframe: false }, enable_raw_klines: true },
      risk_control: { max_positions: 1, btc_eth_max_leverage: 1, altcoin_max_leverage: 1, max_margin_usage: 0.9, min_position_size: 10, min_risk_reward_ratio: 2, min_confidence: 70 },
      protection: {
        full_tp_sl: { enabled: false, mode: 'disabled', take_profit: { mode: 'disabled', value: 0 }, stop_loss: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 } },
        ladder_tp_sl: { enabled: false, mode: 'disabled', take_profit_enabled: false, stop_loss_enabled: false, take_profit_price: { mode: 'disabled', value: 0 }, take_profit_size: { mode: 'disabled', value: 0 }, stop_loss_price: { mode: 'disabled', value: 0 }, stop_loss_size: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 }, rules: [] },
        drawdown_take_profit: { enabled: false, rules: [] },
        break_even_stop: { enabled: false, trigger_mode: 'profit_pct', trigger_value: 1, offset_pct: 0.1 },
        regime_filter: { enabled: false, allowed_regimes: ['narrow', 'standard', 'wide'], block_high_funding: false, max_funding_rate_abs: 0.01, block_high_volatility: false, max_atr14_pct: 3, require_trend_alignment: false },
      },
    }

    const payload = buildStrategySavePayload({
      name: 'Strategy 1',
      description: '',
      is_public: false,
      config_visible: true,
    }, editingConfig, 'en')

    expect(payload.config.strategy_control_policy).toEqual({ mode: 'strict' })
  })

  it('preserves explicit strategy control policy mode in payload', () => {
    const editingConfig: StrategyConfig = {
      language: 'en',
      strategy_type: 'ai_trading',
      coin_source: { source_type: 'static', static_coins: ['BTCUSDT'], use_ai500: false, use_oi_top: false, use_oi_low: false },
      indicators: { klines: { primary_timeframe: '15m', primary_count: 25, enable_multi_timeframe: false }, enable_raw_klines: true },
      risk_control: { max_positions: 1, btc_eth_max_leverage: 1, altcoin_max_leverage: 1, max_margin_usage: 0.9, min_position_size: 10, min_risk_reward_ratio: 2, min_confidence: 70 },
      protection: {
        full_tp_sl: { enabled: false, mode: 'disabled', take_profit: { mode: 'disabled', value: 0 }, stop_loss: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 } },
        ladder_tp_sl: { enabled: false, mode: 'disabled', take_profit_enabled: false, stop_loss_enabled: false, take_profit_price: { mode: 'disabled', value: 0 }, take_profit_size: { mode: 'disabled', value: 0 }, stop_loss_price: { mode: 'disabled', value: 0 }, stop_loss_size: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 }, rules: [] },
        drawdown_take_profit: { enabled: false, rules: [] },
        break_even_stop: { enabled: false, trigger_mode: 'profit_pct', trigger_value: 1, offset_pct: 0.1 },
        regime_filter: { enabled: false, allowed_regimes: ['narrow', 'standard', 'wide'], block_high_funding: false, max_funding_rate_abs: 0.01, block_high_volatility: false, max_atr14_pct: 3, require_trend_alignment: false },
      },
      strategy_control_policy: { mode: 'recommend_only' },
    }

    const payload = buildStrategySavePayload({
      name: 'Strategy 2',
      description: '',
      is_public: false,
      config_visible: true,
    }, editingConfig, 'en')

    expect(payload.config.strategy_control_policy).toEqual({ mode: 'recommend_only' })
  })

  it('preserves drawdown dual-mode semantics and normalizes disabled mode in payload', () => {
    const editingConfig: StrategyConfig = {
      language: 'en',
      strategy_type: 'ai_trading',
      coin_source: { source_type: 'static', static_coins: ['BTCUSDT'], use_ai500: false, use_oi_top: false, use_oi_low: false },
      indicators: { klines: { primary_timeframe: '15m', primary_count: 25, enable_multi_timeframe: false }, enable_raw_klines: true },
      risk_control: { max_positions: 1, btc_eth_max_leverage: 1, altcoin_max_leverage: 1, max_margin_usage: 0.9, min_position_size: 10, min_risk_reward_ratio: 2, min_confidence: 70 },
      protection: {
        full_tp_sl: { enabled: false, mode: 'disabled', take_profit: { mode: 'disabled', value: 0 }, stop_loss: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 } },
        ladder_tp_sl: { enabled: false, mode: 'disabled', take_profit_enabled: false, stop_loss_enabled: false, take_profit_price: { mode: 'disabled', value: 0 }, take_profit_size: { mode: 'disabled', value: 0 }, stop_loss_price: { mode: 'disabled', value: 0 }, stop_loss_size: { mode: 'disabled', value: 0 }, fallback_max_loss: { mode: 'disabled', value: 0 }, rules: [] },
        drawdown_take_profit: {
          enabled: true,
          mode: 'ai',
          rules: [{ min_profit_pct: 7, max_drawdown_pct: 35, close_ratio_pct: 80, poll_interval_seconds: 30 }],
        },
        break_even_stop: { enabled: false, trigger_mode: 'profit_pct', trigger_value: 1, offset_pct: 0.1 },
        regime_filter: { enabled: false, allowed_regimes: ['narrow', 'standard', 'wide'], block_high_funding: false, max_funding_rate_abs: 0.01, block_high_volatility: false, max_atr14_pct: 3, require_trend_alignment: false },
      },
    }

    const payload = buildStrategySavePayload({
      name: 'Strategy DD',
      description: '',
      is_public: false,
      config_visible: true,
    }, editingConfig, 'en')

    expect(payload.config.protection.drawdown_take_profit.mode).toBe('ai')
    expect(payload.config.protection.drawdown_take_profit.rules).toEqual([
      { min_profit_pct: 7, max_drawdown_pct: 35, close_ratio_pct: 80, poll_interval_seconds: 30 },
    ])

    const disabledPayload = buildStrategySavePayload({
      name: 'Strategy DD 2',
      description: '',
      is_public: false,
      config_visible: true,
    }, {
      ...editingConfig,
      protection: {
        ...editingConfig.protection,
        drawdown_take_profit: { enabled: false, mode: 'disabled', rules: [] },
      },
    }, 'en')

    expect(disabledPayload.config.protection.drawdown_take_profit.mode).toBe('disabled')
  })
})
