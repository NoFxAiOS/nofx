import { api } from './api'
import type { MarketSymbol } from './api/data'
import type { AIModel, Exchange, StrategyConfig } from '../types'

export interface QuickTradeResult {
  traderId: string
  traderName: string
  strategyId: string
  strategyName: string
  symbol: string
  display: string
  reusedTrader: boolean
  /** Whether the trader was successfully started after creation. */
  started: boolean
  /** Set when start failed — the trader exists but is NOT running. */
  startError?: string
}

function compactSymbolName(symbol: string) {
  return symbol.replace(/^xyz:/i, '').replace(/[^A-Za-z0-9_-]+/g, '').slice(0, 16) || 'SYMBOL'
}

function pickEnabledModel(models: AIModel[]) {
  return models.find((m) => m.enabled)
}

function pickHyperliquidExchange(exchanges: Exchange[]) {
  return exchanges.find((e) => {
    const type = (e.exchange_type || e.id || '').toLowerCase()
    return type === 'hyperliquid' && e.enabled && !!e.hyperliquidWalletAddr?.trim()
  })
}

// Returns the custom prompt for the quick-create flow. Stocks default to a
// long-biased, momentum-seeking trader because the Hyperliquid US-stock
// products are designed for one-directional exposure and shorting individual
// equities through the agent has rarely been what the user actually wanted.
// Crypto stays bidirectional but conservative.
function buildQuickCreatePrompt(symbol: string, category: string | undefined, language: 'zh' | 'en'): string {
  const isStock = (category || '').toLowerCase() === 'stock'
  if (isStock) {
    return language === 'zh'
      ? `只交易 Hyperliquid USDC 永续合约 ${symbol} (美股)。\n\n核心策略: 直接做多, 不做空。\n- 出现以下任一情形主动开多: 突破前期高点、放量上涨、关键支撑位回踩反弹、强势板块同步走强、宏观/财报/事件利好。\n- 不要在没有任何看多依据时硬等"完美进场点"。轻仓试错优于错过趋势。\n- 单标的策略, 不切换到其他股票。\n- 风控: 严格止损 (1-3% 单笔), 利润目标至少 2:1, 单笔不超过权益的 25%, 默认 2-3 倍杠杆, 切勿满仓。\n- 每次决策前必须先调 get_balance, get_positions, get_market_snapshot 拿到实时数据。`
      : `Trade only the Hyperliquid USDC perpetual market ${symbol} (US equity).\n\nCore strategy: long-only — do not short.\n- Open longs proactively on any of these: breakout above prior high, volume expansion on up bars, pullback to a key support that holds, strong sector tape, macro/earnings/news catalysts.\n- Don't wait endlessly for the "perfect entry" — a small probing long beats missing a trend.\n- Single-symbol; do not rotate into other stocks.\n- Risk: stop-loss 1-3% per trade, profit target ≥ 2:1, max 25% of equity per trade, default 2-3x leverage, never go all-in.\n- Always call get_balance, get_positions, get_market_snapshot before deciding.`
  }
  // Crypto / other → keep bidirectional but cautious.
  return language === 'zh'
    ? `只交易 Hyperliquid USDC 永续合约 ${symbol}。\n\n核心策略: 多空双向, 谨慎进场。\n- 每次决策前必须调 get_balance, get_positions, get_market_snapshot, 拿到实时价格、成交量、资金费率、持仓量。\n- 趋势明确 + 成交量配合 + 风险回报比 ≥ 2:1 才开仓; 模糊就空仓等。\n- 风控: 严格止损 (1-3% 单笔), 单笔不超过权益的 25%, 默认 3 倍杠杆, 切勿满仓。\n- 单标的策略, 不切换到其他币种。`
    : `Trade only the Hyperliquid USDC perpetual market ${symbol}.\n\nCore strategy: bidirectional but disciplined.\n- Always call get_balance, get_positions, get_market_snapshot first for live price, volume, funding, and OI.\n- Open positions only when trend, volume, and a ≥ 2:1 risk/reward all line up; otherwise stay flat.\n- Risk: stop-loss 1-3% per trade, max 25% of equity per trade, default 3x leverage, never go all-in.\n- Single-symbol; do not rotate.`
}

function buildSingleSymbolConfig(
  base: StrategyConfig,
  symbol: string,
  category: string | undefined,
  language: 'zh' | 'en'
): StrategyConfig {
  const staticCoinSource = {
    source_type: 'static' as const,
    static_coins: [symbol],
    excluded_coins: [],
    use_ai500: false,
    use_oi_top: false,
    use_oi_low: false,
    use_hyper_all: false,
    use_hyper_main: false,
  }
  const customPrompt = buildQuickCreatePrompt(symbol, category, language)

  return {
    ...base,
    strategy_type: 'ai_trading',
    language,
    coin_source: staticCoinSource,
    custom_prompt: customPrompt,
    ai_config: {
      ...(base.ai_config || {}),
      coin_source: staticCoinSource,
      indicators: base.ai_config?.indicators || base.indicators!,
      risk_control: base.ai_config?.risk_control || base.risk_control!,
      prompt_sections: base.ai_config?.prompt_sections || base.prompt_sections,
      custom_prompt: customPrompt,
    },
  }
}

export async function createHyperliquidQuickTrader(
  symbolInput: MarketSymbol | { symbol: string; display?: string; category?: string },
  language: 'zh' | 'en'
): Promise<QuickTradeResult> {
  const symbol = symbolInput.symbol
  const display = symbolInput.display || symbol
  // Category drives the strategy bias: stocks default to long-only, crypto
  // stays bidirectional. Passed through from the panel button which already
  // knows whether each row is a stock or a crypto perpetual.
  const category = (symbolInput as { category?: string }).category
  const compact = compactSymbolName(display)
  const traderName = `HL ${compact} Quick`.slice(0, 50)
  const strategyName = `HL ${compact} Strategy`.slice(0, 50)

  const [models, exchanges, traders, strategies] = await Promise.all([
    api.getModelConfigs(),
    api.getExchangeConfigs(),
    api.getTraders(true),
    api.getStrategies().catch(() => []),
  ])

  const model = pickEnabledModel(models)
  if (!model) {
    throw new Error(language === 'zh' ? '没有可用 AI 模型，请先在 Config 里启用模型。' : 'No enabled AI model. Enable a model in Config first.')
  }

  const exchange = pickHyperliquidExchange(exchanges)
  if (!exchange) {
    throw new Error(language === 'zh' ? '没有可用 Hyperliquid 钱包，请先连接并保存 Hyperliquid。' : 'No usable Hyperliquid wallet. Connect and save Hyperliquid first.')
  }

  const existingTrader = traders.find((tr: any) =>
    String(tr.name || '').toLowerCase() === traderName.toLowerCase() ||
    (String(tr.exchange_id || '') === exchange.id && String(tr.trading_symbols || '').split(',').map((s) => s.trim()).includes(symbol))
  )
  if (existingTrader) {
    const existing = existingTrader as any
    const existingId = existing.trader_id || existing.id
    const wasRunning = Boolean(existing.is_running)
    let started = wasRunning
    let startError: string | undefined
    if (!wasRunning) {
      try {
        await api.startTrader(existingId)
        started = true
      } catch (err: any) {
        startError = err?.message || String(err)
      }
    }
    return {
      traderId: existingId,
      traderName: existing.trader_name || existing.name || traderName,
      strategyId: existing.strategy_id || '',
      strategyName,
      symbol,
      display,
      reusedTrader: true,
      started,
      startError,
    }
  }

  let strategy = strategies.find((s: any) => String(s.name || '').toLowerCase() === strategyName.toLowerCase()) as any
  if (!strategy?.id) {
    const defaultConfig = await api.getDefaultStrategyConfig()
    const config = buildSingleSymbolConfig(defaultConfig, symbol, category, language)
    const isStock = (category || '').toLowerCase() === 'stock'
    const description = language === 'zh'
      ? isStock
        ? `Hyperliquid ${display} (美股) 单标的快速做多策略 — 主动捕捉突破、放量、回踩反弹。`
        : `Hyperliquid ${display} 单标的快速交易策略 — 多空双向, 等趋势 + 量能确认。`
      : isStock
        ? `Hyperliquid ${display} single-symbol long-only momentum strategy.`
        : `Hyperliquid ${display} single-symbol bidirectional trading strategy.`
    strategy = await api.createStrategy({
      name: strategyName,
      description,
      config,
    } as any)
  }

  const isStock = (category || '').toLowerCase() === 'stock'
  const traderPrompt = language === 'zh'
    ? isStock
      ? `固定只交易 Hyperliquid ${symbol} (美股), 单向做多, 不做空。\n关注: 突破、放量、回踩反弹、宏观/财报利好。\n禁: 切换标的、做空、满仓、无止损。`
      : `固定只交易 Hyperliquid ${symbol}, 多空双向, 不要扩展到其他标的。每次决策前先看余额、仓位、最新价格。`
    : isStock
      ? `Only trade Hyperliquid ${symbol} (US stock), long-only, no shorting.\nWatch for: breakouts, volume spikes, support reclaims, macro/earnings catalysts.\nForbidden: rotating to other symbols, shorting, going all-in, trading without a stop.`
      : `Only trade Hyperliquid ${symbol}; bidirectional, do not expand to other symbols. Re-check balance, positions, and live price before every decision.`

  const trader = await api.createTrader({
    name: traderName,
    ai_model_id: model.id,
    exchange_id: exchange.id,
    strategy_id: strategy.id,
    scan_interval_minutes: 5,
    trading_symbols: symbol,
    show_in_competition: false,
    custom_prompt: traderPrompt,
  })

  const traderId = trader.trader_id || (trader as any).id

  // The whole point of the ⚡ button is "do it now". After creating the trader
  // we immediately start it; the previous behavior of stopping at "created,
  // please start manually" was the single biggest source of the "agent does
  // nothing" complaint. If start fails (e.g. wallet not funded yet), we
  // report it so the chat reply can be honest about the state.
  let started = false
  let startError: string | undefined
  try {
    await api.startTrader(traderId)
    started = true
  } catch (err: any) {
    startError = err?.message || String(err)
  }

  return {
    traderId,
    traderName: trader.trader_name || (trader as any).name || traderName,
    strategyId: strategy.id,
    strategyName,
    symbol,
    display,
    reusedTrader: false,
    started,
    startError,
  }
}
