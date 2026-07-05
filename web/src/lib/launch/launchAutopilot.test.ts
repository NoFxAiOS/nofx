import { beforeEach, describe, expect, it, vi } from 'vitest'
import { launchAutopilot } from './launchAutopilot'
import { ApiError } from '../httpClient'
import type { LaunchPreflightResult } from './types'

const mocks = vi.hoisted(() => ({
  api: {
    getTraders: vi.fn(),
    createTrader: vi.fn(),
    updateTrader: vi.fn(),
    startTrader: vi.fn(),
  },
  runLaunchPreflight: vi.fn(),
  resolveLaunchModel: vi.fn(),
  resolveLaunchExchange: vi.fn(),
}))

vi.mock('../api', () => ({ api: mocks.api }))
vi.mock('./preflight', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./preflight')>()),
  runLaunchPreflight: mocks.runLaunchPreflight,
}))
vi.mock('./resolve', () => ({
  resolveLaunchModel: mocks.resolveLaunchModel,
  resolveLaunchExchange: mocks.resolveLaunchExchange,
}))

function readyPreflight(): LaunchPreflightResult {
  return {
    ready: true,
    checks: [],
    min_ai_fee_usdc: 1,
    min_trading_usdc: 12,
    checked_at: new Date().toISOString(),
  }
}

function failedPreflight(): LaunchPreflightResult {
  return {
    ready: false,
    checks: [
      {
        id: 'ai_wallet_funds',
        status: 'failed',
        code: 'AI_WALLET_INSUFFICIENT_FUNDS',
        message: 'AI wallet needs 1 USDC.',
      },
    ],
    min_ai_fee_usdc: 1,
    min_trading_usdc: 12,
    checked_at: new Date().toISOString(),
  }
}

describe('launchAutopilot', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.resolveLaunchModel.mockResolvedValue({ id: 'model-1' })
    mocks.resolveLaunchExchange.mockResolvedValue({
      exchange: { id: 'ex-1' },
    })
    mocks.api.getTraders.mockResolvedValue([])
    mocks.api.createTrader.mockResolvedValue({
      trader_id: 't-1',
      is_running: false,
    })
    mocks.api.startTrader.mockResolvedValue(undefined)
  })

  it('never touches the strategy when preflight fails', async () => {
    mocks.runLaunchPreflight.mockResolvedValue(failedPreflight())
    const ensureStrategy = vi.fn()

    const outcome = await launchAutopilot({ ensureStrategy })

    expect(ensureStrategy).not.toHaveBeenCalled()
    expect(mocks.api.createTrader).not.toHaveBeenCalled()
    expect(outcome.ok).toBe(false)
    if (outcome.ok || outcome.kind !== 'preflight') {
      throw new Error('expected a preflight failure outcome')
    }
    expect(outcome.setupTarget).toBe('claw402')
    expect(outcome.message).toContain('AI wallet needs 1 USDC.')
  })

  it('creates and starts the trader after preflight passes', async () => {
    mocks.runLaunchPreflight.mockResolvedValue(readyPreflight())
    const ensureStrategy = vi.fn().mockResolvedValue('strat-1')

    const outcome = await launchAutopilot({
      ensureStrategy,
      scanIntervalMinutes: 15,
    })

    expect(ensureStrategy).toHaveBeenCalledTimes(1)
    expect(mocks.api.createTrader).toHaveBeenCalledWith(
      expect.objectContaining({
        strategy_id: 'strat-1',
        scan_interval_minutes: 15,
        ai_model_id: 'model-1',
        exchange_id: 'ex-1',
      })
    )
    expect(mocks.api.startTrader).toHaveBeenCalledWith('t-1')
    expect(outcome).toEqual(
      expect.objectContaining({ ok: true, traderId: 't-1' })
    )
  })

  it('updates the existing autopilot instead of creating a duplicate', async () => {
    mocks.runLaunchPreflight.mockResolvedValue(readyPreflight())
    mocks.api.getTraders.mockResolvedValue([
      { trader_id: 't-old', trader_name: 'NOFX Autopilot', is_running: false },
    ])
    mocks.api.updateTrader.mockResolvedValue({
      trader_id: 't-old',
      is_running: true,
    })

    const outcome = await launchAutopilot({
      ensureStrategy: vi.fn().mockResolvedValue('strat-1'),
    })

    expect(mocks.api.updateTrader).toHaveBeenCalled()
    expect(mocks.api.createTrader).not.toHaveBeenCalled()
    expect(mocks.api.startTrader).not.toHaveBeenCalled()
    expect(outcome).toEqual(
      expect.objectContaining({ ok: true, traderId: 't-old' })
    )
  })

  it('surfaces the server-side preflight result when start is rejected', async () => {
    mocks.runLaunchPreflight.mockResolvedValue(readyPreflight())
    mocks.api.startTrader.mockRejectedValue(
      new ApiError(
        'preflight failed',
        'trader.start.preflight_failed',
        undefined,
        400,
        { preflight: failedPreflight() }
      )
    )

    const outcome = await launchAutopilot({
      ensureStrategy: vi.fn().mockResolvedValue('strat-1'),
    })

    expect(outcome.ok).toBe(false)
    if (outcome.ok || outcome.kind !== 'preflight') {
      throw new Error('expected a preflight failure outcome')
    }
    expect(outcome.setupTarget).toBe('claw402')
  })

  it('routes missing exchange setup to the hyperliquid anchor', async () => {
    mocks.resolveLaunchExchange.mockResolvedValue({
      exchange: null,
      reason: 'No Hyperliquid account is connected.',
    })

    const outcome = await launchAutopilot({ ensureStrategy: vi.fn() })

    expect(outcome).toEqual(
      expect.objectContaining({
        ok: false,
        kind: 'setup',
        setupTarget: 'hyperliquid',
      })
    )
    expect(mocks.runLaunchPreflight).not.toHaveBeenCalled()
  })
})
