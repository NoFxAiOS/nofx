import { api } from '../api'
import { ApiError } from '../httpClient'
import {
  describeLaunchFailures,
  primarySetupTarget,
  runLaunchPreflight,
} from './preflight'
import { resolveLaunchExchange, resolveLaunchModel } from './resolve'
import type { LaunchOutcome } from './types'

export const AUTOPILOT_TRADER_NAME = 'NOFX Autopilot'

export interface LaunchAutopilotOptions {
  /**
   * Provides the strategy id to trade. Called only AFTER preflight passes so
   * a failed launch never mutates or activates a strategy as a side effect.
   */
  ensureStrategy: () => Promise<string>
  scanIntervalMinutes?: number
}

/**
 * The single Autopilot launch path shared by Strategy Studio and the guided
 * launch panel. Order matters: resolve → preflight (server-side, fresh
 * balances) → strategy → create/update trader → start. No side effects happen
 * before preflight passes.
 */
export async function launchAutopilot(
  options: LaunchAutopilotOptions
): Promise<LaunchOutcome> {
  const { ensureStrategy, scanIntervalMinutes = 5 } = options

  try {
    const model = await resolveLaunchModel()
    if (!model) {
      return {
        ok: false,
        kind: 'setup',
        message:
          'No enabled AI model is ready. Create or fund the Claw402 wallet first.',
        setupTarget: 'claw402',
      }
    }

    const exchangeResult = await resolveLaunchExchange()
    if (!exchangeResult.exchange) {
      return {
        ok: false,
        kind: 'setup',
        message: exchangeResult.reason,
        setupTarget: 'hyperliquid',
      }
    }
    const exchange = exchangeResult.exchange

    const preflight = await runLaunchPreflight({
      ai_model_id: model.id,
      exchange_id: exchange.id,
    })
    if (!preflight.ready) {
      return {
        ok: false,
        kind: 'preflight',
        message:
          describeLaunchFailures(preflight) ||
          'Launch prerequisites are not ready yet.',
        preflight,
        setupTarget: primarySetupTarget(preflight),
      }
    }

    const strategyId = await ensureStrategy()

    const traderRequest = {
      name: AUTOPILOT_TRADER_NAME,
      ai_model_id: model.id,
      exchange_id: exchange.id,
      strategy_id: strategyId,
      scan_interval_minutes: scanIntervalMinutes,
      is_cross_margin: true,
      show_in_competition: true,
      btc_eth_leverage: 10,
      altcoin_leverage: 10,
    }

    // Re-fetch the live trader list before deciding create vs update. Stale
    // props/snapshots would create a duplicate "NOFX Autopilot" — paying the
    // slow first-create cost again and orphaning dashboards onto a deleted id.
    const existingTraders = await api.getTraders(true)
    const existing =
      existingTraders.find(
        (trader) => trader.trader_name === AUTOPILOT_TRADER_NAME
      ) ||
      existingTraders.find((trader) =>
        (trader.strategy_name || '').toLowerCase().includes('claw402')
      ) ||
      null

    const autopilot = existing
      ? await api.updateTrader(existing.trader_id, traderRequest)
      : await api.createTrader(traderRequest)

    if (!autopilot.is_running) {
      try {
        await api.startTrader(autopilot.trader_id)
      } catch (err) {
        // Launch is idempotent: the update path restarts a running trader
        // asynchronously, so a racing "already running" rejection is success.
        const alreadyRunning =
          err instanceof ApiError &&
          err.errorKey === 'trader.start.already_running'
        if (!alreadyRunning) throw err
      }
    }

    return {
      ok: true,
      traderId: autopilot.trader_id,
      warning: autopilot.startup_warning,
    }
  } catch (err) {
    // The server re-runs preflight on start; surface its structured result if
    // readiness changed between our check and the start call.
    if (err instanceof ApiError && err.errorKey === 'trader.start.preflight_failed') {
      const preflight = err.errorData?.preflight
      if (preflight) {
        return {
          ok: false,
          kind: 'preflight',
          message: describeLaunchFailures(preflight) || err.message,
          preflight,
          setupTarget: primarySetupTarget(preflight),
        }
      }
    }
    return {
      ok: false,
      kind: 'error',
      message:
        err instanceof Error ? err.message : 'Failed to launch NOFX Autopilot',
    }
  }
}

/**
 * Default strategy provisioning for the guided panel: reuse the active
 * Claw402 strategy, otherwise create and activate it.
 */
export async function ensureClaw402Strategy(): Promise<string> {
  const strategies = await api.getStrategies()
  const existing =
    strategies.find(
      (strategy) =>
        strategy.is_active &&
        strategy.config?.ai_config?.coin_source?.source_type === 'vergex_signal'
    ) ||
    strategies.find((strategy) =>
      strategy.name.toLowerCase().includes('claw402')
    )

  if (existing) {
    if (!existing.is_active) {
      await api.activateStrategy(existing.id)
    }
    return existing.id
  }

  const config = await api.getDefaultStrategyConfig()
  const created = await api.createStrategy({
    name: 'NOFX Claw402 Auto Strategy',
    description:
      'Single built-in strategy: Claw402 board, per-symbol details, raw candles, then execution.',
    config,
  })
  if (created?.id) {
    await api.activateStrategy(created.id)
    return created.id
  }

  const refreshed = await api.getStrategies()
  const fallback = refreshed.find((strategy) =>
    strategy.name.toLowerCase().includes('claw402')
  )
  if (!fallback) throw new Error('Failed to create Claw402 strategy')
  await api.activateStrategy(fallback.id)
  return fallback.id
}
