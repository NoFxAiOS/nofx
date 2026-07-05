import { api } from '../api'
import type { AIModel, Exchange } from '../../types'

export function modelHasCredential(model: AIModel) {
  return Boolean(
    model.has_api_key ||
    model.apiKey ||
    (model.provider === 'claw402' && model.walletAddress)
  )
}

export function exchangeHasKey(exchange: Exchange) {
  return Boolean(exchange.has_api_key || exchange.apiKey)
}

export function isHyperliquidExchange(exchange: Exchange) {
  return exchange.exchange_type === 'hyperliquid'
}

/** Prefers the claw402 model, falls back to any enabled model with a credential. */
export function pickTradingModel(models: AIModel[]) {
  return (
    models.find(
      (model) =>
        model.provider === 'claw402' &&
        model.enabled &&
        modelHasCredential(model)
    ) ||
    models.find((model) => model.enabled && modelHasCredential(model)) ||
    null
  )
}

export function pickTradingExchange(exchanges: Exchange[]) {
  return (
    exchanges.find(
      (exchange) =>
        isHyperliquidExchange(exchange) &&
        exchange.enabled &&
        exchangeHasKey(exchange) &&
        Boolean(exchange.hyperliquidBuilderApproved) &&
        (exchange.hyperliquidWalletAddr || '').trim() !== ''
    ) || null
  )
}

/**
 * Resolves a launch-capable AI model, auto-provisioning the beginner claw402
 * wallet when none is configured yet. Returns null when nothing could be
 * resolved — the caller routes the user into claw402 setup.
 */
export async function resolveLaunchModel(): Promise<AIModel | null> {
  let models = await api.getModelConfigs()
  let model = pickTradingModel(models)
  if (model) return model

  const onboarding = await api.prepareBeginnerOnboarding()
  models = await api.getModelConfigs()
  model =
    models.find(
      (item) =>
        item.id === onboarding.configured_model_id &&
        item.enabled &&
        modelHasCredential(item)
    ) || pickTradingModel(models)
  if (model) return model

  if (onboarding.configured_model_id && onboarding.private_key) {
    await api.updateModelConfigs({
      models: {
        [onboarding.configured_model_id]: {
          enabled: true,
          api_key: onboarding.private_key,
          custom_api_url: '',
          custom_model_name: onboarding.default_model,
        },
      },
    })
    models = await api.getModelConfigs()
    model =
      models.find(
        (item) =>
          item.id === onboarding.configured_model_id &&
          item.enabled &&
          modelHasCredential(item)
      ) || pickTradingModel(models)
  }

  return model
}

/**
 * Resolves a launch-capable exchange. Returns the exchange or a message
 * explaining the most specific missing prerequisite.
 */
export async function resolveLaunchExchange(): Promise<
  { exchange: Exchange } | { exchange: null; reason: string }
> {
  const exchanges = await api.getExchangeConfigs()
  const ready = pickTradingExchange(exchanges)
  if (ready) return { exchange: ready }

  const hyperliquid = exchanges.find(isHyperliquidExchange)
  if (!hyperliquid) {
    return {
      exchange: null,
      reason:
        'No Hyperliquid account is connected. Connect Hyperliquid and authorize the NOFX agent first.',
    }
  }
  if (!hyperliquid.enabled) {
    return {
      exchange: null,
      reason: 'The Hyperliquid account is disabled. Enable it first.',
    }
  }
  if (!exchangeHasKey(hyperliquid)) {
    return {
      exchange: null,
      reason:
        'The Hyperliquid agent key is missing. Reconnect Hyperliquid and save the agent wallet.',
    }
  }
  if (!hyperliquid.hyperliquidBuilderApproved) {
    return {
      exchange: null,
      reason:
        'Hyperliquid builder authorization is not complete. Finish wallet authorization first.',
    }
  }
  return {
    exchange: null,
    reason:
      'The Hyperliquid wallet address is missing. Reconnect Hyperliquid first.',
  }
}
