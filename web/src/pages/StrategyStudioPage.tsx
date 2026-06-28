import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  ArrowDownRight,
  ArrowUpRight,
  Bot,
  Check,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Shield,
  Sparkles,
  Target,
  Trash2,
} from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { api } from '../lib/api'
import { confirmToast, notify } from '../lib/notify'
import type {
  AIStrategyConfig,
  CoinSourceConfig,
  IndicatorConfig,
  RiskControlConfig,
  Strategy,
  StrategyConfig,
} from '../types'
import type { AIModel, Exchange } from '../types'
import type {
  MarketSymbol,
  VergexHeatmapBin,
  VergexHeatmapResponse,
  VergexSignalDimension,
  VergexSignalItem,
  VergexSignalLabResponse,
} from '../lib/api/data'
import { buildDashboardPath } from '../router/paths'

const API_BASE = import.meta.env.VITE_API_BASE || ''

type Scope =
  | 'all'
  | 'crypto'
  | 'stock'
  | 'commodity'
  | 'index'
  | 'forex'
  | 'pre_ipo'
type ListMode = 'claw402' | 'pool'

const scopeOptions: Array<{ value: Scope; zh: string; en: string }> = [
  { value: 'all', zh: '全部 Claw402', en: 'All Claw402' },
  { value: 'stock', zh: '美股', en: 'US Stocks' },
  { value: 'crypto', zh: '加密货币', en: 'Crypto' },
  { value: 'commodity', zh: '大宗商品', en: 'Commodities' },
  { value: 'index', zh: '指数', en: 'Indices' },
  { value: 'forex', zh: '外汇', en: 'FX' },
  { value: 'pre_ipo', zh: 'Pre-IPO', en: 'Pre-IPO' },
]

const categoryPriority: Record<string, number> = {
  stock: 1,
  crypto: 2,
  commodity: 3,
  index: 4,
  forex: 5,
  pre_ipo: 6,
}

const timeframeOptions = ['5m', '15m', '30m', '1h', '4h', '1d']
const barCountOptions = [20, 30, 50]
const topNOptions = [5, 6, 7, 8, 9, 10]
const detailBandOptions = ['5', '10', '15', '20']
const claw402BoardLimit = 30
const confidenceOptions = [65, 75, 82]

const text = (language: string, zh: string, en: string) =>
  language === 'zh' ? zh : en

function modelHasCredential(model: AIModel) {
  return Boolean(model.has_api_key || model.apiKey)
}

function pickTradingModel(models: AIModel[]) {
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

function isHyperliquidExchange(exchange: Exchange) {
  return exchange.exchange_type === 'hyperliquid'
}

function hyperliquidWalletAddress(exchange: Exchange) {
  return exchange.hyperliquidWalletAddr || ''
}

function pickTradingExchange(exchanges: Exchange[]) {
  return (
    exchanges.find(
      (exchange) =>
        isHyperliquidExchange(exchange) &&
        exchange.enabled &&
        modelHasExchangeKey(exchange) &&
        Boolean(exchange.hyperliquidBuilderApproved) &&
        hyperliquidWalletAddress(exchange).trim() !== ''
    ) || null
  )
}

function modelHasExchangeKey(exchange: Exchange) {
  return Boolean(exchange.has_api_key || exchange.apiKey)
}

type Profile = 'careful' | 'balanced' | 'active'

const profileOptions: Array<{
  value: Profile
  zh: string
  en: string
  zhNote: string
  enNote: string
  maxPositions: number
  leverage: number
  confidence: number
  topN: number
  timeframe: string
  bars: number
  margin: number
  promptZh: string
  promptEn: string
}> = [
  {
    value: 'careful',
    zh: '稳健',
    en: 'Careful',
    zhNote: '少交易，只做信号一致的机会',
    enNote: 'Fewer trades, only aligned signals',
    maxPositions: 1,
    leverage: 10,
    confidence: 82,
    topN: 5,
    timeframe: '1h',
    bars: 30,
    margin: 1.0,
    promptZh:
      '稳健模式：只有 Claw402 榜单方向、Signal Lab、成本/清算热力图和原始 K 线同时支持时才开仓；信号冲突时等待。',
    promptEn:
      'Careful mode: open only when the Claw402 board direction, Signal Lab, cost/liquidation heatmap and raw candles agree; wait on conflicts.',
  },
  {
    value: 'balanced',
    zh: '均衡',
    en: 'Balanced',
    zhNote: '默认推荐，机会和风控平衡',
    enNote: 'Recommended balance of opportunity and risk',
    maxPositions: 2,
    leverage: 10,
    confidence: 75,
    topN: 5,
    timeframe: '15m',
    bars: 30,
    margin: 1.0,
    promptZh:
      '均衡模式：优先交易 Claw402 排名靠前且 Signal Lab 与 K 线同向的标的；用清算热力图确定止损和止盈区域。',
    promptEn:
      'Balanced mode: prioritize top Claw402-ranked symbols when Signal Lab agrees with raw candles; use the liquidation heatmap for stop and target zones.',
  },
  {
    value: 'active',
    zh: '进取',
    en: 'Active',
    zhNote: '更快捕捉强趋势，仓位更多',
    enNote: 'Faster trend capture with more positions',
    maxPositions: 3,
    leverage: 10,
    confidence: 68,
    topN: 8,
    timeframe: '5m',
    bars: 50,
    margin: 1.0,
    promptZh:
      '进取模式：可以更快跟随 Claw402 强信号，但必须用 Signal Lab 二次确认，用热力图区避开拥挤清算位，止损必须明确。',
    promptEn:
      'Active mode: follow strong Claw402 signals faster, but require Signal Lab confirmation, avoid crowded liquidation zones, and always set explicit stops.',
  },
]

function getAIConfig(config: StrategyConfig): AIStrategyConfig | null {
  if (config.ai_config) return config.ai_config
  if (config.coin_source && config.indicators && config.risk_control) {
    return {
      coin_source: config.coin_source,
      indicators: config.indicators,
      risk_control: config.risk_control,
      prompt_sections: config.prompt_sections,
      custom_prompt: config.custom_prompt,
    }
  }
  return null
}

function defaultCoinSource(
  source?: Partial<CoinSourceConfig>
): CoinSourceConfig {
  const staticCoins = source?.static_coins || []
  const minVergexLimit =
    staticCoins.length > 0 ? Math.min(staticCoins.length, 10) : 10
  const vergexLimit = Math.min(
    Math.max(source?.vergex_limit || minVergexLimit, minVergexLimit),
    10
  )
  return {
    source_type: 'vergex_signal',
    static_coins: staticCoins,
    excluded_coins: [],
    use_ai500: false,
    ai500_limit: 0,
    use_oi_top: false,
    oi_top_limit: 0,
    use_oi_low: false,
    oi_low_limit: 0,
    use_hyper_all: false,
    use_hyper_main: false,
    hyper_main_limit: 0,
    hyper_rank_category: source?.hyper_rank_category || 'all',
    hyper_rank_direction: 'gainers',
    hyper_rank_limit: 0,
    vergex_limit: vergexLimit,
    vergex_market_type: source?.vergex_market_type || 'all',
    vergex_chain: source?.vergex_chain || 'hyperliquid',
    vergex_liq_band: source?.vergex_liq_band || '',
  }
}

function defaultIndicators(
  indicators?: Partial<IndicatorConfig>
): IndicatorConfig {
  const klines = indicators?.klines || {
    primary_timeframe: '15m',
    primary_count: 30,
    enable_multi_timeframe: false,
  }

  return {
    klines: {
      primary_timeframe: klines.primary_timeframe || '15m',
      primary_count: klines.primary_count || 30,
      longer_timeframe: '',
      longer_count: 0,
      enable_multi_timeframe: false,
      selected_timeframes: [klines.primary_timeframe || '15m'],
    },
    enable_raw_klines: true,
    enable_ema: false,
    enable_macd: false,
    enable_rsi: false,
    enable_atr: false,
    enable_boll: false,
    enable_volume: false,
    enable_oi: false,
    enable_funding_rate: false,
    nofxos_api_key: '',
    enable_quant_data: false,
    enable_quant_oi: false,
    enable_quant_netflow: false,
    enable_oi_ranking: false,
    enable_netflow_ranking: false,
    enable_price_ranking: false,
  }
}

function defaultRisk(risk?: Partial<RiskControlConfig>): RiskControlConfig {
  const leverage =
    risk?.altcoin_max_leverage || risk?.btc_eth_max_leverage || 10
  return {
    max_positions: risk?.max_positions || 2,
    btc_eth_max_leverage: leverage,
    altcoin_max_leverage: leverage,
    btc_eth_max_position_value_ratio:
      risk?.btc_eth_max_position_value_ratio || 10,
    altcoin_max_position_value_ratio:
      risk?.altcoin_max_position_value_ratio || 10,
    max_margin_usage: risk?.max_margin_usage || 1.0,
    min_position_size: risk?.min_position_size || 12,
    min_risk_reward_ratio: risk?.min_risk_reward_ratio || 3,
    min_confidence: risk?.min_confidence || 78,
  }
}

function simplifyConfig(
  config: StrategyConfig | null | undefined
): StrategyConfig {
  const ai = config ? getAIConfig(config) : null
  return {
    strategy_type: 'ai_trading',
    language: config?.language || 'zh',
    ai_config: {
      coin_source: defaultCoinSource(ai?.coin_source),
      indicators: defaultIndicators(ai?.indicators),
      risk_control: defaultRisk(ai?.risk_control),
      custom_prompt: ai?.custom_prompt || '',
      prompt_sections: ai?.prompt_sections,
    },
    grid_config: null,
    publish_config: config?.publish_config,
  }
}

function normalizeSymbol(symbol: string) {
  return symbol
    .trim()
    .toUpperCase()
    .replace(/^XYZ:/, '')
    .replace(/-USDC$/, '')
}

function signalMarketType(item: VergexSignalItem) {
  return (
    item.market_type || (item.category === 'crypto' ? 'core_perp' : 'hip3_perp')
  )
}

function strategySymbolForSignal(item: VergexSignalItem) {
  const symbol = normalizeSymbol(item.symbol)
  return signalMarketType(item) === 'core_perp' ? symbol : `xyz:${symbol}`
}

function categoryLabel(category: string | undefined, language: string) {
  const option = scopeOptions.find((item) => item.value === category)
  if (!option) return category || 'TradeFi'
  return text(language, option.zh, option.en)
}

function profileFromRisk(risk: RiskControlConfig | null | undefined): Profile {
  if (!risk) return 'balanced'
  if (risk.min_confidence >= 80 || risk.max_positions <= 1) return 'careful'
  if (risk.altcoin_max_leverage >= 5 || risk.max_positions >= 3) return 'active'
  return 'balanced'
}

function formatChange(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return ''
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(2)}%`
}

function signalBiasInfo(bias: string | undefined) {
  const normalized = (bias || '').toLowerCase()
  const bullish = ['bullish', 'long', 'buy', 'open_long'].includes(normalized)
  const bearish = ['bearish', 'short', 'sell', 'open_short'].includes(
    normalized
  )
  if (bullish) {
    return {
      label: 'Long Bias',
      hint: 'Long bias',
      classes: 'border-emerald-400/35 bg-emerald-400/10 text-emerald-200',
      icon: ArrowUpRight,
    }
  }
  if (bearish) {
    return {
      label: 'Short Bias',
      hint: 'Short bias',
      classes: 'border-red-400/35 bg-red-400/10 text-red-200',
      icon: ArrowDownRight,
    }
  }
  return {
    label: 'Neutral',
    hint: 'Neutral bias',
    classes: 'border-white/15 bg-white/[0.04] text-nofx-text-muted',
    icon: Target,
  }
}

function formatSignalStrength(item: VergexSignalItem) {
  const parts: string[] = []
  if (typeof item.score === 'number' && Number.isFinite(item.score)) {
    const sign = item.score > 0 ? '+' : ''
    parts.push(`z ${sign}${item.score.toFixed(2)}`)
  }
  if (typeof item.confidence === 'number' && Number.isFinite(item.confidence)) {
    const confidence =
      item.confidence <= 1 ? item.confidence * 100 : item.confidence
    if (confidence > 0) {
      parts.push(`${confidence.toFixed(0)}% conf`)
    }
  }
  return parts.join(' · ') || 'details ready'
}

function signalSortValue(item: VergexSignalItem) {
  return categoryPriority[item.category || ''] || 99
}

function compareSignalItems(a: VergexSignalItem, b: VergexSignalItem) {
  const categoryDelta = signalSortValue(a) - signalSortValue(b)
  if (categoryDelta !== 0) return categoryDelta
  return (
    (a.rank || Number.MAX_SAFE_INTEGER) - (b.rank || Number.MAX_SAFE_INTEGER)
  )
}

function sameSignalItem(a: VergexSignalItem | null, b: VergexSignalItem) {
  if (!a) return false
  return (
    normalizeSymbol(a.symbol) === normalizeSymbol(b.symbol) &&
    signalMarketType(a) === signalMarketType(b)
  )
}

function clampPct(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

function formatMoney(value: number | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  const sign = value < 0 ? '-' : ''
  const abs = Math.abs(value)
  if (abs >= 1_000_000_000)
    return `${sign}$${(abs / 1_000_000_000).toFixed(2)}B`
  if (abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(2)}M`
  if (abs >= 1_000) return `${sign}$${(abs / 1_000).toFixed(2)}K`
  return `${sign}$${abs.toFixed(2)}`
}

function formatNumber(value: number | undefined, digits = 2) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return value.toFixed(digits)
}

function formatPrice(value: number | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `$${value.toFixed(value >= 100 ? 2 : 4)}`
}

function formatSignedPct(value: number | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(1)}%`
}

function metricPct(value: number | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  const pct = Math.abs(value) <= 1 ? value * 100 : value
  return `${pct.toFixed(1)}%`
}

function directionStyle(direction?: string) {
  const normalized = (direction || '').toLowerCase()
  if (normalized === 'bullish') {
    return {
      dot: 'bg-emerald-400',
      text: 'text-emerald-300',
      chip: 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300',
      bar: 'bg-emerald-400/70',
    }
  }
  if (normalized === 'bearish') {
    return {
      dot: 'bg-rose-400',
      text: 'text-rose-300',
      chip: 'border-rose-400/25 bg-rose-400/10 text-rose-300',
      bar: 'bg-rose-400/70',
    }
  }
  return {
    dot: 'bg-slate-400',
    text: 'text-nofx-text-muted',
    chip: 'border-white/10 bg-white/[0.05] text-nofx-text-muted',
    bar: 'bg-slate-500/70',
  }
}

function SignalDimensionRow({ item }: { item: VergexSignalDimension }) {
  const style = directionStyle(item.direction)
  const percentile =
    typeof item.percentile === 'number' && Number.isFinite(item.percentile)
      ? clampPct(item.percentile)
      : null
  const chipLabel = [item.direction, item.strength].filter(Boolean).join(' · ')

  return (
    <div className="border-t border-white/10 px-3 py-3">
      <div className="grid gap-3 md:grid-cols-[1fr_150px]">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`h-2 w-2 rounded-full ${style.dot}`} />
            <span className="text-sm font-semibold text-nofx-text">
              {item.label || item.key || 'Signal factor'}
            </span>
            {chipLabel ? (
              <span
                className={`rounded-md border px-2 py-0.5 text-[11px] ${style.chip}`}
              >
                {chipLabel}
              </span>
            ) : null}
          </div>
          <div className="mt-1 text-xs leading-5 text-nofx-text-muted">
            {item.detail || item.what || 'No detail returned.'}
          </div>
        </div>
        {percentile !== null ? (
          <div className="flex items-center gap-2">
            <div className="h-2 flex-1 overflow-hidden rounded-full bg-white/10">
              <div
                className={`h-full rounded-full ${style.bar}`}
                style={{ width: `${percentile}%` }}
              />
            </div>
            <span className="w-10 text-right font-mono text-xs text-nofx-text-muted">
              {percentile.toFixed(0)}
            </span>
          </div>
        ) : null}
      </div>
    </div>
  )
}

function DetailMetricCard({
  label,
  value,
  note,
  tone = 'neutral',
}: {
  label: string
  value: string
  note?: string
  tone?: 'neutral' | 'green' | 'red' | 'cyan' | 'gold'
}) {
  const toneClass =
    tone === 'green'
      ? 'text-emerald-300'
      : tone === 'red'
        ? 'text-rose-300'
        : tone === 'cyan'
          ? 'text-cyan-300'
          : tone === 'gold'
            ? 'text-nofx-gold'
            : 'text-nofx-text'

  return (
    <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
      <div className="text-xs text-nofx-text-muted">{label}</div>
      <div className={`mt-2 font-mono text-lg ${toneClass}`}>{value}</div>
      {note ? (
        <div className="mt-2 text-xs leading-5 text-nofx-text-muted">
          {note}
        </div>
      ) : null}
    </div>
  )
}

function BandSelector({
  activeBand,
  loading,
  onBandChange,
}: {
  activeBand: string
  loading?: boolean
  onBandChange?: (band: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {detailBandOptions.map((band) => (
        <button
          key={band}
          type="button"
          onClick={() => onBandChange?.(band)}
          disabled={loading}
          className={`rounded-lg border px-4 py-2 font-mono text-sm transition disabled:cursor-not-allowed disabled:opacity-50 ${
            activeBand === band
              ? 'border-cyan-300/60 bg-cyan-300/12 text-cyan-200 shadow-[0_0_24px_rgba(103,232,249,0.12)]'
              : 'border-white/10 bg-white/[0.03] text-nofx-text-muted hover:border-white/25 hover:text-white'
          }`}
        >
          ±{band}%
        </button>
      ))}
    </div>
  )
}

function SignalLabPanel({
  lab,
  activeBand,
  loading,
  onBandChange,
}: {
  lab: VergexSignalLabResponse | null
  activeBand: string
  loading?: boolean
  onBandChange?: (band: string) => void
}) {
  const data = lab?.data
  if (!data) {
    return (
      <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-nofx-text-muted">
        Signal Lab has not loaded yet.
      </div>
    )
  }

  const bias = signalBiasInfo(data.bias)
  const BiasIcon = bias.icon
  const levels = data.levels || {}
  const metrics = data.metrics || {}
  const dimensions = data.dimensions || []

  return (
    <section className="overflow-hidden rounded-lg border border-white/10 bg-[#0d121a] shadow-[0_18px_80px_rgba(0,0,0,0.28)]">
      <div className="border-b border-white/10 bg-white/[0.015] px-5 py-4">
        <div className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-start">
          <div>
            <div className="text-base font-semibold text-nofx-text">
              Signal Lab
              <span className="ml-3 text-sm font-normal text-nofx-text-muted">
                full-book cost basis + node liquidation map · facts only
              </span>
            </div>
            <div className="mt-4">
              <BandSelector
                activeBand={activeBand}
                loading={loading}
                onBandChange={onBandChange}
              />
            </div>
          </div>
          <div className="flex flex-col items-start gap-2 lg:items-end">
            <div className="rounded-full bg-emerald-400/10 px-3 py-1 text-xs font-semibold text-emerald-300">
              live
            </div>
            {data.confidence ? (
              <div className="font-mono text-sm text-nofx-text-muted">
                confidence {data.confidence}
              </div>
            ) : null}
          </div>
        </div>
      </div>

      <div className="px-5 py-5">
        <div className="flex flex-wrap items-end gap-4">
          <div
            className={`text-4xl font-bold ${directionStyle(data.bias).text}`}
          >
            {bias.label}
          </div>
          {data.rank ? (
            <div className="pb-1 font-mono text-base text-emerald-300">
              market #{data.rank}/{data.universeSize || 30}
            </div>
          ) : null}
          {typeof data.compositeZ === 'number' ? (
            <div className="pb-1 font-mono text-base text-emerald-300">
              z {data.compositeZ >= 0 ? '+' : ''}
              {data.compositeZ.toFixed(2)}
            </div>
          ) : null}
          <BiasIcon
            className={`mb-1 h-6 w-6 ${directionStyle(data.bias).text}`}
          />
        </div>
      </div>

      {data.structureRead ? (
        <div className="mx-5 rounded-md border-l-4 border-cyan-300/35 bg-white/[0.04] px-4 py-4 text-base leading-8 text-nofx-text">
          {data.structureRead}
        </div>
      ) : null}

      {dimensions.length > 0 ? (
        <div className="pt-5">
          <div className="px-5 pb-2 text-sm font-semibold text-nofx-text">
            Factors
            <span className="ml-2 text-sm font-normal text-nofx-text-muted">
              bar = cross-market percentile
            </span>
          </div>
          {dimensions.map((dimension, index) => (
            <SignalDimensionRow
              key={`${dimension.key || dimension.label || 'factor'}-${index}`}
              item={dimension}
            />
          ))}
        </div>
      ) : null}

      <div className="border-t border-white/10 p-5">
        <div className="text-base font-semibold text-nofx-text">
          Key levels price {formatPrice(levels.markPrice)}
        </div>
        <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <DetailMetricCard
            label="Fair-value magnet (POC)"
            value={`${formatPrice(levels.poc)} (${formatSignedPct(levels.pocDistPct)})`}
            note="Where the most cost is concentrated."
            tone="neutral"
          />
          <DetailMetricCard
            label="Strongest liq cluster"
            value={`${formatPrice(levels.magnet)} (${formatSignedPct(levels.magnetDistPct)})`}
            note="Largest forced-close cluster in the selected band."
            tone="cyan"
          />
          <DetailMetricCard
            label="Resistance above"
            value={`${formatPrice(levels.resistance)} (${formatSignedPct(levels.resistanceDistPct)})`}
            note="Trapped longs may sell to break even as price returns."
            tone="red"
          />
          <DetailMetricCard
            label="Support below"
            value={`${formatPrice(levels.support)} (${formatSignedPct(levels.supportDistPct)})`}
            note="Trapped shorts may cover as price falls back."
            tone="green"
          />
        </div>

        <div className="mt-6 text-base font-semibold text-nofx-text">
          Structure metrics
        </div>
        <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <DetailMetricCard
            label="Squeeze fuel above"
            value={formatMoney(metrics.shortLiqAbove)}
            note="Short liquidation fuel above current price."
            tone="green"
          />
          <DetailMetricCard
            label="Flush fuel below"
            value={formatMoney(metrics.longLiqBelow)}
            note="Long liquidation fuel below current price."
            tone="red"
          />
          <DetailMetricCard
            label="Cascade vulnerability"
            value={metricPct(metrics.cascadeVulnPct)}
            note="Share of OI close to force-close."
            tone="neutral"
          />
          <DetailMetricCard
            label="Long book PnL"
            value={formatMoney(metrics.longOverhangPnl)}
            note={`avg ${metricPct(metrics.gLong)}`}
            tone="green"
          />
          <DetailMetricCard
            label="Short book PnL"
            value={formatMoney(metrics.shortOverhangPnl)}
            note={`avg ${metricPct(metrics.gShort)}`}
            tone="red"
          />
          <DetailMetricCard
            label="Top-10 concentration"
            value={metricPct(metrics.top10Pct)}
            note="Share held by the top 10 addresses."
            tone="neutral"
          />
        </div>
      </div>
    </section>
  )
}

function binPrice(bin: VergexHeatmapBin) {
  if (typeof bin.px === 'number') return bin.px
  if (
    typeof bin.bucketStartPrice === 'number' &&
    typeof bin.bucketEndPrice === 'number'
  ) {
    return (bin.bucketStartPrice + bin.bucketEndPrice) / 2
  }
  return 0
}

function binValue(bin: VergexHeatmapBin) {
  return (
    Math.abs(bin.longCost || 0) +
    Math.abs(bin.shortCost || 0) +
    Math.abs(bin.longLiq || 0) +
    Math.abs(bin.shortLiq || 0)
  )
}

function sideBarWidth(value: number | undefined, maxValue: number) {
  if (!value || !Number.isFinite(value) || maxValue <= 0) return '0%'
  return `${Math.max(1.5, Math.min(46, (Math.abs(value) / maxValue) * 46))}%`
}

function ChartGridLines() {
  return (
    <div className="pointer-events-none absolute inset-y-0 left-[92px] right-0">
      {[12.5, 25, 37.5, 50, 62.5, 75, 87.5].map((left) => (
        <div
          key={left}
          className={`absolute top-0 h-full w-px ${
            left === 50 ? 'bg-white/25' : 'bg-white/[0.07]'
          }`}
          style={{ left: `${left}%` }}
        />
      ))}
    </div>
  )
}

function HeatmapChartRow({
  bin,
  maxLeft,
  maxRight,
  markPrice,
}: {
  bin: VergexHeatmapBin
  maxLeft: number
  maxRight: number
  markPrice?: number
}) {
  const price = binPrice(bin)
  const isCurrent =
    typeof markPrice === 'number' &&
    typeof bin.bucketStartPrice === 'number' &&
    typeof bin.bucketEndPrice === 'number' &&
    markPrice >= bin.bucketStartPrice &&
    markPrice <= bin.bucketEndPrice

  return (
    <div
      className="relative z-10 grid grid-cols-[78px_minmax(0,1fr)] items-center gap-3"
      title={[
        `Price ${formatPrice(price)}`,
        `Long cost ${formatMoney(bin.longCost)}`,
        `Short cost ${formatMoney(bin.shortCost)}`,
        `Long liquidation ${formatMoney(bin.longLiq)}`,
        `Short liquidation ${formatMoney(bin.shortLiq)}`,
      ].join(' · ')}
    >
      <div
        className={`text-right font-mono text-xs ${
          isCurrent ? 'text-nofx-gold' : 'text-nofx-text-muted'
        }`}
      >
        {formatPrice(price)}
      </div>
      <div
        className={`relative h-6 overflow-visible rounded-sm ${
          isCurrent ? 'bg-nofx-gold/10' : 'bg-white/[0.015]'
        }`}
      >
        {isCurrent ? (
          <>
            <div className="absolute inset-x-0 top-1/2 h-px bg-nofx-gold" />
            <div className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md bg-nofx-gold px-2 py-1 font-mono text-xs font-bold text-black">
              Mark {formatPrice(markPrice)}
            </div>
          </>
        ) : null}
        <div
          className="absolute right-1/2 top-[5px] h-2 rounded-l bg-rose-500/80"
          style={{ width: sideBarWidth(bin.shortCost, maxLeft) }}
        />
        <div
          className="absolute left-1/2 top-[5px] h-2 rounded-r bg-emerald-500/80"
          style={{ width: sideBarWidth(bin.longCost, maxRight) }}
        />
        <div
          className="absolute right-1/2 bottom-[5px] h-2 rounded-l bg-orange-400"
          style={{ width: sideBarWidth(bin.longLiq, maxLeft) }}
        />
        <div
          className="absolute left-1/2 bottom-[5px] h-2 rounded-r bg-cyan-300"
          style={{ width: sideBarWidth(bin.shortLiq, maxRight) }}
        />
      </div>
    </div>
  )
}

function CostLiquidationHeatmap({
  heatmap,
}: {
  heatmap: VergexHeatmapResponse | null
}) {
  const data = heatmap?.data
  const bins = (data?.bins || [])
    .filter((bin) => binValue(bin) > 0)
    .slice()
    .sort((a, b) => binPrice(b) - binPrice(a))

  if (!data || bins.length === 0) {
    return (
      <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-nofx-text-muted">
        Cost/liquidation heatmap has not loaded yet.
      </div>
    )
  }

  const maxLeft = Math.max(
    ...bins.map((bin) =>
      Math.max(Math.abs(bin.shortCost || 0), Math.abs(bin.longLiq || 0))
    ),
    1
  )
  const maxRight = Math.max(
    ...bins.map((bin) =>
      Math.max(Math.abs(bin.longCost || 0), Math.abs(bin.shortLiq || 0))
    ),
    1
  )
  const longLiqTotal = bins.reduce((sum, bin) => sum + (bin.longLiq || 0), 0)
  const shortLiqTotal = bins.reduce((sum, bin) => sum + (bin.shortLiq || 0), 0)
  const includedCost = data.cost?.includedPositions || data.costAddrs || 0
  const includedLiq = data.liqAddrs || 0

  return (
    <section className="overflow-hidden rounded-lg border border-white/10 bg-[#0d121a] shadow-[0_18px_80px_rgba(0,0,0,0.28)]">
      <div className="border-b border-white/10 bg-white/[0.015] px-5 py-4">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="text-base font-semibold text-nofx-text">
              Cost / Liquidation Heatmap
              <span className="ml-3 text-sm font-normal text-nofx-text-muted">
                position cost distribution · liquidation clusters
              </span>
            </div>
            <div className="mt-3 flex flex-wrap gap-3 text-sm text-nofx-text-muted">
              <span>{includedCost.toLocaleString()} cost positions</span>
              <span>{includedLiq.toLocaleString()} liquidation prices</span>
              <span>
                mark{' '}
                <span className="font-semibold text-nofx-text">
                  {formatPrice(data.markPrice)}
                </span>
              </span>
            </div>
            {data.liquidation?.reason ? (
              <div className="mt-2 text-sm text-nofx-gold">
                Liquidation prices use latest snapshot; incremental trades can
                lag.
              </div>
            ) : null}
          </div>
          <div className="rounded-full bg-emerald-400/10 px-3 py-1 text-xs font-semibold text-emerald-300">
            live
          </div>
        </div>
      </div>

      <div className="px-5 py-5">
        <div className="mb-4 flex flex-wrap justify-center gap-4 text-sm text-nofx-text-muted">
          <span className="inline-flex items-center gap-1">
            <span className="h-3 w-3 rounded bg-emerald-500/70" />
            Long cost
          </span>
          <span className="inline-flex items-center gap-1">
            <span className="h-3 w-3 rounded bg-rose-500/70" />
            Short cost
          </span>
          <span className="inline-flex items-center gap-1 text-orange-300">
            <span className="h-3 w-3 rounded bg-orange-400" />
            Long liquidation
          </span>
          <span className="inline-flex items-center gap-1 text-cyan-300">
            <span className="h-3 w-3 rounded bg-cyan-300" />
            Short liquidation
          </span>
        </div>

        <div className="relative rounded-lg border border-white/10 bg-black/20 px-4 py-5">
          <ChartGridLines />
          <div className="relative z-10 max-h-[720px] space-y-1 overflow-y-auto pr-2">
            {bins.map((bin, index) => (
              <HeatmapChartRow
                key={`${binPrice(bin)}-${index}`}
                bin={bin}
                maxLeft={maxLeft}
                maxRight={maxRight}
                markPrice={data.markPrice}
              />
            ))}
          </div>
          <div className="relative z-10 mt-4 grid grid-cols-[78px_minmax(0,1fr)] items-center gap-3 text-xs text-nofx-text-muted">
            <div />
            <div className="grid grid-cols-5 font-mono">
              <span>{formatMoney(maxLeft)}</span>
              <span className="text-center">{formatMoney(maxLeft / 2)}</span>
              <span className="text-center">$0</span>
              <span className="text-center">{formatMoney(maxRight / 2)}</span>
              <span className="text-right">{formatMoney(maxRight)}</span>
            </div>
          </div>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <DetailMetricCard
            label="Flush fuel below"
            value={formatMoney(longLiqTotal)}
            note="Long liquidations can force sell into downside breaks."
            tone="red"
          />
          <DetailMetricCard
            label="Squeeze fuel above"
            value={formatMoney(shortLiqTotal)}
            note="Short liquidations can force buy into upside breaks."
            tone="cyan"
          />
          <DetailMetricCard
            label="Bin step"
            value={formatNumber(data.binStep, 4)}
            note={`${bins.length} active price bins returned.`}
            tone="neutral"
          />
        </div>
      </div>
    </section>
  )
}

export function StrategyStudioPage() {
  const { token } = useAuth()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const [strategies, setStrategies] = useState<Strategy[]>([])
  const [selectedStrategy, setSelectedStrategy] = useState<Strategy | null>(
    null
  )
  const [editingConfig, setEditingConfig] = useState<StrategyConfig | null>(
    null
  )
  const [symbols, setSymbols] = useState<MarketSymbol[]>([])
  const [signals, setSignals] = useState<VergexSignalItem[]>([])
  const [detailSignal, setDetailSignal] = useState<VergexSignalItem | null>(
    null
  )
  const [signalLab, setSignalLab] = useState<VergexSignalLabResponse | null>(
    null
  )
  const [heatmap, setHeatmap] = useState<VergexHeatmapResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [symbolsLoading, setSymbolsLoading] = useState(false)
  const [symbolsError, setSymbolsError] = useState('')
  const [signalsLoading, setSignalsLoading] = useState(false)
  const [signalsError, setSignalsError] = useState('')
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [detailLiqBand, setDetailLiqBand] = useState('15')
  const [listMode, setListMode] = useState<ListMode>('claw402')
  const [hasChanges, setHasChanges] = useState(false)

  const aiConfig = editingConfig?.ai_config || null
  const coinSource = aiConfig?.coin_source
  const indicators = aiConfig?.indicators
  const risk = aiConfig?.risk_control
  const selectedSymbols = coinSource?.static_coins || []
  const scope = 'all' as Scope
  const activeProfile = profileFromRisk(risk)

  const signalMap = useMemo(() => {
    const map = new Map<string, VergexSignalItem>()
    for (const item of signals) {
      map.set(normalizeSymbol(item.symbol), item)
    }
    return map
  }, [signals])

  const visibleSymbols = useMemo(() => {
    const tradefi = symbols.filter((item) => item.category !== 'crypto')
    const scoped =
      scope === 'all'
        ? tradefi
        : tradefi.filter((item) => item.category === scope)
    return [...scoped].sort((a, b) => {
      const aSignal = signalMap.get(normalizeSymbol(a.symbol))
      const bSignal = signalMap.get(normalizeSymbol(b.symbol))
      const aRank = aSignal?.rank || Number.MAX_SAFE_INTEGER
      const bRank = bSignal?.rank || Number.MAX_SAFE_INTEGER
      if (aRank !== bRank) return aRank - bRank
      return (b.volume_24h || 0) - (a.volume_24h || 0)
    })
  }, [scope, signalMap, symbols])

  const visibleSignalItems = useMemo(() => {
    const scoped =
      scope === 'all'
        ? signals
        : signals.filter((item) => item.category === scope)
    return scoped.slice().sort(compareSignalItems)
  }, [scope, signals])

  const selectedSet = useMemo(
    () => new Set(selectedSymbols.map(normalizeSymbol)),
    [selectedSymbols]
  )

  const loadStrategies = useCallback(
    async (preferredStrategyId?: string) => {
      if (!token) return
      setLoading(true)
      try {
        const result = await api.getStrategies()
        setStrategies(result)
        const next =
          (preferredStrategyId
            ? result.find((item) => item.id === preferredStrategyId)
            : null) ||
          result.find((item) => item.is_active) ||
          result[0] ||
          null
        setSelectedStrategy(next)
        setEditingConfig(next ? simplifyConfig(next.config) : null)
        setHasChanges(false)
      } catch (err) {
        notify.error(
          err instanceof Error ? err.message : 'Failed to load strategies'
        )
      } finally {
        setLoading(false)
      }
    },
    [token]
  )

  const loadSymbols = useCallback(async () => {
    setSymbolsLoading(true)
    setSymbolsError('')
    try {
      const result = await api.getSymbols('hyperliquid-xyz')
      setSymbols(result.symbols || [])
    } catch (err) {
      setSymbolsError(
        err instanceof Error ? err.message : 'Symbol list unavailable'
      )
    } finally {
      setSymbolsLoading(false)
    }
  }, [])

  const loadSignals = useCallback(async () => {
    if (!token) return
    setSignalsLoading(true)
    setSignalsError('')
    try {
      const result = await api.getVergexSignalRanking(claw402BoardLimit)
      setSignals(result.items || [])
      setListMode('claw402')
    } catch (err) {
      setSignalsError(
        err instanceof Error ? err.message : 'Claw402.ai board unavailable'
      )
    } finally {
      setSignalsLoading(false)
    }
  }, [token])

  const loadSignalDetail = useCallback(
    async (item: VergexSignalItem, bandOverride?: string) => {
      if (!token) return
      const nextBand =
        bandOverride || detailLiqBand || coinSource?.vergex_liq_band || '15'
      const params = {
        marketType: signalMarketType(item),
        symbol: strategySymbolForSignal(item),
        chain: 'mainnet',
        liqBand: nextBand,
      }

      setDetailLiqBand(nextBand)
      setDetailSignal(item)
      setSignalLab(null)
      setHeatmap(null)
      setDetailError('')
      setDetailLoading(true)

      window.requestAnimationFrame(() => {
        document
          .getElementById('claw402-detail-panel')
          ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
      })

      const [labResult, heatmapResult] = await Promise.allSettled([
        api.getVergexSignalLab(params),
        api.getVergexCostLiquidationHeatmap(params),
      ])

      const errors: string[] = []
      if (labResult.status === 'fulfilled') {
        setSignalLab(labResult.value)
      } else {
        errors.push(
          `Signal Lab: ${
            labResult.reason instanceof Error
              ? labResult.reason.message
              : 'unavailable'
          }`
        )
      }

      if (heatmapResult.status === 'fulfilled') {
        setHeatmap(heatmapResult.value)
      } else {
        errors.push(
          `Heatmap: ${
            heatmapResult.reason instanceof Error
              ? heatmapResult.reason.message
              : 'unavailable'
          }`
        )
      }

      setDetailError(errors.join(' · '))
      setDetailLoading(false)
    },
    [coinSource?.vergex_liq_band, detailLiqBand, token]
  )

  const selectDetailBand = useCallback(
    (band: string) => {
      setDetailLiqBand(band)
      if (detailSignal) {
        void loadSignalDetail(detailSignal, band)
      }
    },
    [detailSignal, loadSignalDetail]
  )

  useEffect(() => {
    void loadStrategies()
    void loadSymbols()
    void loadSignals()
  }, [loadStrategies, loadSymbols, loadSignals])

  const patchAI = (patch: Partial<AIStrategyConfig>) => {
    setEditingConfig((prev) => {
      const base = simplifyConfig(prev)
      return {
        ...base,
        language: language as 'zh' | 'en',
        ai_config: {
          ...base.ai_config!,
          ...patch,
        },
      }
    })
    setHasChanges(true)
  }

  const patchCoinSource = (patch: Partial<CoinSourceConfig>) => {
    patchAI({
      coin_source: defaultCoinSource({
        ...coinSource,
        ...patch,
      }),
    })
  }

  const patchIndicators = (patch: Partial<IndicatorConfig>) => {
    patchAI({
      indicators: defaultIndicators({
        ...indicators,
        ...patch,
      }),
    })
  }

  const patchRisk = (patch: Partial<RiskControlConfig>) => {
    patchAI({
      risk_control: defaultRisk({
        ...risk,
        ...patch,
      }),
    })
  }

  const createStrategy = async () => {
    if (!token) return
    try {
      const response = await fetch(
        `${API_BASE}/api/strategies/default-config?lang=${language}`,
        { headers: { Authorization: `Bearer ${token}` } }
      )
      const defaultConfig = response.ok
        ? simplifyConfig(await response.json())
        : simplifyConfig(null)
      defaultConfig.language = language as 'zh' | 'en'
      defaultConfig.ai_config = {
        ...defaultConfig.ai_config!,
        coin_source: defaultCoinSource({
          ...defaultConfig.ai_config?.coin_source,
          static_coins: [],
          hyper_rank_category: 'all',
          vergex_limit: 10,
          vergex_market_type: 'all',
        }),
        indicators: defaultIndicators({
          ...defaultConfig.ai_config?.indicators,
          klines: {
            primary_timeframe: '15m',
            primary_count: 30,
            enable_multi_timeframe: false,
            selected_timeframes: ['15m'],
          },
        }),
        risk_control: defaultRisk({
          ...defaultConfig.ai_config?.risk_control,
          max_positions: 2,
          btc_eth_max_leverage: 10,
          altcoin_max_leverage: 10,
          btc_eth_max_position_value_ratio: 10,
          altcoin_max_position_value_ratio: 10,
          max_margin_usage: 1.0,
          min_confidence: 78,
          min_risk_reward_ratio: 3,
        }),
        custom_prompt:
          'NOFX Autopilot reads the Claw402.ai board each cycle, fetches Signal Lab and cost/liquidation structure for every candidate, confirms with raw OHLCV candles, then trades only when the full-size setup is justified.',
        prompt_sections: undefined,
      }
      const created = await api.createStrategy({
        name: text(
          language,
          'NOFX Claw402 自动策略',
          'NOFX Claw402 Auto Strategy'
        ),
        description: text(
          language,
          '唯一内置策略：读取 Claw402.ai 榜单，逐个拉取详情，再结合原始 K 线自动交易。',
          'The single built-in strategy: read the Claw402.ai board, fetch per-symbol details, then trade with raw candles.'
        ),
        config: defaultConfig,
      })
      await loadStrategies(created.id)
      setHasChanges(false)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to create strategy'
      )
    }
  }

  const saveStrategy = async (
    activateAfter = false,
    overrideConfig?: StrategyConfig,
    successMessage?: string
  ) => {
    if (!token || !selectedStrategy || (!editingConfig && !overrideConfig))
      return
    setSaving(true)
    try {
      const config = simplifyConfig(overrideConfig || editingConfig)
      config.language = language as 'zh' | 'en'
      const response = await fetch(
        `${API_BASE}/api/strategies/${selectedStrategy.id}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            name: selectedStrategy.name,
            description: selectedStrategy.description,
            config,
            is_public: selectedStrategy.is_public,
            config_visible: selectedStrategy.config_visible,
          }),
        }
      )
      if (!response.ok) throw new Error('Failed to save strategy')
      if (activateAfter) {
        await api.activateStrategy(selectedStrategy.id)
      }
      setHasChanges(false)
      notify.success(
        successMessage ||
          (activateAfter
            ? text(language, '策略已保存并启用', 'Strategy saved and activated')
            : text(language, '策略已保存', 'Strategy saved'))
      )
      await loadStrategies(selectedStrategy.id)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to save strategy'
      )
    } finally {
      setSaving(false)
    }
  }

  const buildUnifiedClaw402Config = (): StrategyConfig => {
    const base = simplifyConfig(editingConfig)
    base.language = language as 'zh' | 'en'
    base.ai_config = {
      ...base.ai_config!,
      coin_source: defaultCoinSource({
        ...base.ai_config?.coin_source,
        static_coins: [],
        hyper_rank_category: 'all',
        vergex_limit: 10,
        vergex_market_type: 'all',
        vergex_chain: 'hyperliquid',
      }),
      indicators: defaultIndicators({
        ...base.ai_config?.indicators,
        klines: {
          primary_timeframe: '15m',
          primary_count: 30,
          enable_multi_timeframe: false,
          selected_timeframes: ['15m'],
        },
      }),
      risk_control: defaultRisk({
        ...base.ai_config?.risk_control,
        max_positions: 2,
        btc_eth_max_leverage: 10,
        altcoin_max_leverage: 10,
        btc_eth_max_position_value_ratio: 10,
        altcoin_max_position_value_ratio: 10,
        max_margin_usage: 1.0,
        min_confidence: 78,
        min_risk_reward_ratio: 3,
      }),
      custom_prompt:
        'Run NOFX Autopilot: use the Claw402.ai ranking as the candidate universe, verify each candidate with Signal Lab and cost/liquidation structure, confirm timing with raw OHLCV candles, and only open full-size 10x positions when the setup is strong enough.',
      prompt_sections: undefined,
    }
    return base
  }

  const resolveOneClickModel = async () => {
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

    if (!model && onboarding.configured_model_id && onboarding.private_key) {
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

    if (!model) {
      throw new Error(
        'No enabled AI model is ready. Create or fund the Claw402 wallet first.'
      )
    }

    return model
  }

  const resolveOneClickExchange = async () => {
    const exchanges = await api.getExchangeConfigs()
    const exchange = pickTradingExchange(exchanges)
    if (exchange) return exchange

    const hyperliquid = exchanges.find(isHyperliquidExchange)
    if (!hyperliquid) {
      throw new Error(
        'No Hyperliquid account is ready. Connect Hyperliquid and authorize the NOFX agent first.'
      )
    }
    if (!hyperliquid.enabled) {
      throw new Error('The Hyperliquid account is disabled. Enable it first.')
    }
    if (!modelHasExchangeKey(hyperliquid)) {
      throw new Error(
        'The Hyperliquid agent key is missing. Reconnect Hyperliquid and save the agent wallet.'
      )
    }
    if (!hyperliquid.hyperliquidBuilderApproved) {
      throw new Error(
        'Hyperliquid builder authorization is not complete. Finish wallet authorization first.'
      )
    }
    if (!hyperliquidWalletAddress(hyperliquid).trim()) {
      throw new Error(
        'The Hyperliquid wallet address is missing. Reconnect Hyperliquid first.'
      )
    }

    throw new Error(
      'No ready Hyperliquid account found. Check the wallet authorization.'
    )
  }

  const startUnifiedClaw402Agent = async () => {
    if (!selectedStrategy) return
    const config = buildUnifiedClaw402Config()
    const traderName = 'NOFX Autopilot'

    setSaving(true)
    try {
      setEditingConfig(config)
      setHasChanges(true)

      await api.updateStrategy(selectedStrategy.id, {
        name: selectedStrategy.name,
        description:
          selectedStrategy.description ||
          'Autonomous market selection powered by Claw402.ai Signal Lab, liquidation structure, and raw candles.',
        config,
      })
      await api.activateStrategy(selectedStrategy.id)

      const [model, exchange] = await Promise.all([
        resolveOneClickModel(),
        resolveOneClickExchange(),
      ])

      const traderRequest = {
        name: traderName,
        ai_model_id: model.id,
        exchange_id: exchange.id,
        strategy_id: selectedStrategy.id,
        scan_interval_minutes: 15,
        is_cross_margin: true,
        show_in_competition: true,
      }

      const existingTraders = await api.getTraders(true)
      const existingAutopilot = existingTraders.find(
        (trader) => trader.trader_name === traderName
      )
      const autopilot = existingAutopilot
        ? await api.updateTrader(existingAutopilot.trader_id, traderRequest)
        : await api.createTrader(traderRequest)

      if (autopilot.startup_warning) {
        notify.warning(autopilot.startup_warning)
      }

      if (!autopilot.is_running) {
        await api.startTrader(autopilot.trader_id)
      }
      notify.success(`${traderName} started`)
      setHasChanges(false)
      await loadStrategies(selectedStrategy.id)
      navigate(buildDashboardPath(autopilot.trader_id))
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to launch NOFX Autopilot'
      )
    } finally {
      setSaving(false)
    }
  }

  const activateStrategy = async () => {
    if (!selectedStrategy) return
    try {
      await api.activateStrategy(selectedStrategy.id)
      notify.success(text(language, '已设为当前策略', 'Strategy activated'))
      await loadStrategies(selectedStrategy.id)
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to activate strategy'
      )
    }
  }

  const deleteStrategy = async () => {
    if (!selectedStrategy || selectedStrategy.is_active) return
    const ok = await confirmToast(
      text(language, '删除这个策略？', 'Delete this strategy?'),
      {
        title: text(language, '确认删除', 'Confirm delete'),
        okText: text(language, '删除', 'Delete'),
        cancelText: text(language, '取消', 'Cancel'),
      }
    )
    if (!ok) return
    try {
      await api.deleteStrategy(selectedStrategy.id)
      notify.success(text(language, '策略已删除', 'Strategy deleted'))
      await loadStrategies()
    } catch (err) {
      notify.error(
        err instanceof Error ? err.message : 'Failed to delete strategy'
      )
    }
  }

  const toggleSymbol = (symbol: string) => {
    const normalized = normalizeSymbol(symbol)
    const next = selectedSet.has(normalized)
      ? selectedSymbols.filter((item) => normalizeSymbol(item) !== normalized)
      : [...selectedSymbols, symbol].slice(0, 10)
    const nextLimit =
      next.length > 0
        ? Math.min(next.length, 10)
        : Math.min(Math.max(coinSource?.vergex_limit || 5, 5), 10)
    patchCoinSource({
      static_coins: next,
      vergex_limit: nextLimit,
      vergex_market_type: 'all',
    })
  }

  const setScope = (nextScope: Scope) => {
    patchCoinSource({
      hyper_rank_category: nextScope,
      static_coins: [],
      vergex_market_type: 'all',
    })
  }

  const setTimeframe = (timeframe: string) => {
    patchIndicators({
      klines: {
        primary_timeframe: timeframe,
        primary_count: indicators?.klines.primary_count || 30,
        enable_multi_timeframe: false,
        selected_timeframes: [timeframe],
      },
    })
  }

  const setBarCount = (count: number) => {
    patchIndicators({
      klines: {
        primary_timeframe: indicators?.klines.primary_timeframe || '15m',
        primary_count: count,
        enable_multi_timeframe: false,
        selected_timeframes: [indicators?.klines.primary_timeframe || '15m'],
      },
    })
  }

  const setLeverage = (leverage: number) => {
    patchRisk({
      btc_eth_max_leverage: leverage,
      altcoin_max_leverage: leverage,
    })
  }

  const applyProfile = (profile: (typeof profileOptions)[number]) => {
    setEditingConfig((prev) => {
      const base = simplifyConfig(prev)
      const currentAI = base.ai_config!
      return {
        ...base,
        language: language as 'zh' | 'en',
        ai_config: {
          ...currentAI,
          coin_source: defaultCoinSource({
            ...currentAI.coin_source,
            vergex_limit: profile.topN,
          }),
          indicators: defaultIndicators({
            ...currentAI.indicators,
            klines: {
              primary_timeframe: profile.timeframe,
              primary_count: profile.bars,
              enable_multi_timeframe: false,
              selected_timeframes: [profile.timeframe],
            },
          }),
          risk_control: defaultRisk({
            ...currentAI.risk_control,
            max_positions: profile.maxPositions,
            btc_eth_max_leverage: profile.leverage,
            altcoin_max_leverage: profile.leverage,
            max_margin_usage: profile.margin,
            min_confidence: profile.confidence,
          }),
          custom_prompt: text(language, profile.promptZh, profile.promptEn),
          prompt_sections: undefined,
        },
      }
    })
    setHasChanges(true)
  }

  if (loading) {
    return (
      <div className="flex min-h-[70vh] items-center justify-center">
        <Loader2 className="h-7 w-7 animate-spin text-nofx-gold" />
      </div>
    )
  }

  return (
    <DeepVoidBackground className="min-h-[calc(100vh-64px)] bg-nofx-bg">
      <div className="border-b border-white/10 bg-nofx-bg/75 px-5 py-4 backdrop-blur">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold text-nofx-text">
              {text(language, 'NOFX Autopilot', 'NOFX Autopilot')}
            </h1>
            <p className="mt-1 text-sm text-nofx-text-muted">
              {text(
                language,
                'Autonomous market selection powered by Claw402.ai Signal Lab, liquidation structure, and raw candles.',
                'Autonomous market selection powered by Claw402.ai Signal Lab, liquidation structure, and raw candles.'
              )}
            </p>
          </div>
          <button
            type="button"
            onClick={startUnifiedClaw402Agent}
            disabled={saving || !selectedStrategy}
            className="inline-flex items-center gap-2 rounded-lg bg-nofx-gold px-4 py-2 text-sm font-semibold text-black hover:bg-yellow-400"
          >
            {saving ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Bot className="h-4 w-4" />
            )}
            {text(language, 'Launch Autopilot', 'Launch Autopilot')}
          </button>
        </div>
      </div>

      <div className="grid min-h-[calc(100vh-137px)] grid-cols-1">
        <aside className="hidden border-r border-white/10 bg-black/15 p-3">
          <div className="mb-2 px-2 text-xs font-medium uppercase tracking-wide text-nofx-text-muted">
            {text(language, '我的策略', 'My strategies')}
          </div>
          <div className="space-y-2">
            {strategies.map((strategy) => (
              <button
                key={strategy.id}
                type="button"
                onClick={() => {
                  setSelectedStrategy(strategy)
                  setEditingConfig(simplifyConfig(strategy.config))
                  setHasChanges(false)
                }}
                className={`w-full rounded-lg border px-3 py-3 text-left transition ${
                  selectedStrategy?.id === strategy.id
                    ? 'border-nofx-gold bg-nofx-gold/10'
                    : 'border-white/10 bg-white/[0.02] hover:border-white/20'
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="line-clamp-2 text-sm font-medium text-nofx-text">
                    {strategy.name}
                  </span>
                  {strategy.is_active ? (
                    <span className="rounded bg-emerald-500/15 px-1.5 py-0.5 text-[10px] text-emerald-300">
                      {text(language, '使用中', 'Active')}
                    </span>
                  ) : null}
                </div>
                {strategy.description ? (
                  <div className="mt-1 line-clamp-2 text-xs text-nofx-text-muted">
                    {strategy.description}
                  </div>
                ) : null}
              </button>
            ))}
          </div>
        </aside>

        <main className="overflow-y-auto p-5">
          {selectedStrategy && aiConfig && coinSource && indicators && risk ? (
            <div className="mx-auto max-w-7xl space-y-4">
              <section className="hidden rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <input
                      value={selectedStrategy.name}
                      onChange={(event) => {
                        setSelectedStrategy({
                          ...selectedStrategy,
                          name: event.target.value,
                        })
                        setHasChanges(true)
                      }}
                      className="w-full bg-transparent text-lg font-semibold text-nofx-text outline-none"
                    />
                    <input
                      value={selectedStrategy.description || ''}
                      onChange={(event) => {
                        setSelectedStrategy({
                          ...selectedStrategy,
                          description: event.target.value,
                        })
                        setHasChanges(true)
                      }}
                      placeholder={text(
                        language,
                        '一句话说明这个策略',
                        'One-line strategy note'
                      )}
                      className="mt-1 w-full bg-transparent text-sm text-nofx-text-muted outline-none placeholder:text-nofx-text-muted/50"
                    />
                    {hasChanges ? (
                      <div className="mt-2 text-xs text-nofx-gold">
                        {text(language, '有未保存修改', 'Unsaved changes')}
                      </div>
                    ) : null}
                  </div>
                  <div className="hidden flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => saveStrategy(true)}
                      disabled={saving}
                      className="inline-flex items-center gap-2 rounded-lg bg-emerald-400 px-3 py-2 text-sm font-semibold text-black disabled:cursor-not-allowed disabled:opacity-45"
                    >
                      {saving ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Check className="h-4 w-4" />
                      )}
                      {text(language, '保存并使用', 'Save and use')}
                    </button>
                    <button
                      type="button"
                      onClick={() => saveStrategy()}
                      disabled={saving || !hasChanges}
                      className="inline-flex items-center gap-2 rounded-lg bg-nofx-gold px-3 py-2 text-sm font-semibold text-black disabled:cursor-not-allowed disabled:opacity-45"
                    >
                      {saving ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Save className="h-4 w-4" />
                      )}
                      {text(language, '保存', 'Save')}
                    </button>
                    {!selectedStrategy.is_active ? (
                      <button
                        type="button"
                        onClick={activateStrategy}
                        className="inline-flex items-center gap-2 rounded-lg border border-emerald-400/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-300 hover:bg-emerald-500/15"
                      >
                        <Check className="h-4 w-4" />
                        {text(language, '仅设为当前', 'Activate only')}
                      </button>
                    ) : null}
                    {!selectedStrategy.is_active ? (
                      <button
                        type="button"
                        onClick={deleteStrategy}
                        className="inline-flex items-center gap-2 rounded-lg border border-red-400/25 bg-red-500/10 px-3 py-2 text-sm text-red-300 hover:bg-red-500/15"
                      >
                        <Trash2 className="h-4 w-4" />
                        {text(language, '删除', 'Delete')}
                      </button>
                    ) : null}
                  </div>
                </div>
              </section>

              <section className="rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2 text-sm font-semibold text-nofx-text">
                      <Sparkles className="h-4 w-4 text-nofx-gold" />
                      Signal Board
                    </div>
                    <div className="mt-1 text-xs text-nofx-text-muted">
                      Live Claw402.ai ranking · Signal Lab · liquidation map
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => {
                        if (signals.length === 0) {
                          void loadSignals()
                        } else {
                          setListMode('claw402')
                        }
                      }}
                      disabled={signalsLoading}
                      className={`hidden items-center gap-2 rounded-lg border px-3 py-2 text-xs disabled:opacity-50 ${
                        listMode === 'claw402'
                          ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-gold'
                          : 'border-white/10 text-nofx-text-muted hover:text-white'
                      }`}
                    >
                      <Sparkles className="h-3.5 w-3.5" />
                      {signals.length === 0
                        ? 'Load Claw402 board'
                        : 'Claw402 board'}
                    </button>
                    <button
                      type="button"
                      onClick={() => setListMode('pool')}
                      disabled={symbolsLoading}
                      className={`hidden items-center gap-2 rounded-lg border px-3 py-2 text-xs disabled:opacity-50 ${
                        listMode === 'pool'
                          ? 'border-white/30 bg-white/10 text-white'
                          : 'border-white/10 text-nofx-text-muted hover:text-white'
                      }`}
                    >
                      <RefreshCw
                        className={`h-3.5 w-3.5 ${symbolsLoading ? 'animate-spin' : ''}`}
                      />
                      Symbol pool
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        void loadSignals()
                      }}
                      disabled={signalsLoading}
                      className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-2 text-xs text-nofx-text-muted hover:text-white disabled:opacity-50"
                    >
                      <RefreshCw
                        className={`h-3.5 w-3.5 ${signalsLoading ? 'animate-spin' : ''}`}
                      />
                      Refresh
                    </button>
                    {selectedSymbols.length > 0 ? (
                      <button
                        type="button"
                        onClick={() =>
                          patchCoinSource({
                            static_coins: [],
                            vergex_market_type: 'all',
                          })
                        }
                        className="hidden rounded-lg border border-white/10 px-3 py-2 text-xs text-nofx-text-muted hover:text-white"
                      >
                        Clear selected
                      </button>
                    ) : null}
                  </div>
                </div>

                <div className="hidden mb-4 flex-wrap gap-2">
                  {scopeOptions.map((option) => {
                    const signalCount =
                      option.value === 'all'
                        ? signals.length
                        : signals.filter(
                            (item) => item.category === option.value
                          ).length
                    const poolCount =
                      option.value === 'all'
                        ? symbols.filter((item) => item.category !== 'crypto')
                            .length
                        : symbols.filter(
                            (item) => item.category === option.value
                          ).length
                    const count =
                      listMode === 'claw402' ? signalCount : poolCount
                    return (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => setScope(option.value)}
                        className={`rounded-lg border px-3 py-2 text-xs transition ${
                          scope === option.value
                            ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-gold'
                            : 'border-white/10 bg-black/15 text-nofx-text-muted hover:text-white'
                        }`}
                      >
                        {option.en}
                        {count > 0 ? (
                          <span className="ml-2 opacity-70">{count}</span>
                        ) : null}
                      </button>
                    )
                  })}
                </div>

                <div className="hidden mb-4 gap-3 md:grid-cols-2">
                  <button
                    type="button"
                    onClick={() => {
                      patchCoinSource({
                        static_coins: [],
                        vergex_market_type: 'all',
                      })
                      setListMode('claw402')
                      if (signals.length === 0) {
                        void loadSignals()
                      }
                    }}
                    className={`rounded-lg border p-4 text-left transition ${
                      selectedSymbols.length === 0
                        ? 'border-emerald-300 bg-emerald-400/10'
                        : 'border-white/10 bg-black/15 hover:border-white/25'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold text-nofx-text">
                        Follow Claw402.ai board dynamically
                      </div>
                      {selectedSymbols.length === 0 ? (
                        <Check className="h-4 w-4 text-emerald-300" />
                      ) : null}
                    </div>
                    <div className="mt-2 text-xs text-nofx-text-muted">
                      At runtime, trade the current range Top{' '}
                      {coinSource.vergex_limit || 5}; the board refreshes each
                      cycle.
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => {
                      setListMode('claw402')
                      if (signals.length === 0) {
                        void loadSignals()
                      }
                    }}
                    className={`rounded-lg border p-4 text-left transition ${
                      selectedSymbols.length > 0
                        ? 'border-nofx-gold bg-nofx-gold/10'
                        : 'border-white/10 bg-black/15 hover:border-white/25'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold text-nofx-text">
                        Pinned universe
                      </div>
                      {selectedSymbols.length > 0 ? (
                        <Check className="h-4 w-4 text-nofx-gold" />
                      ) : null}
                    </div>
                    <div className="mt-2 text-xs text-nofx-text-muted">
                      {selectedSymbols.length > 0
                        ? `${selectedSymbols.length} symbols fixed; trade only these.`
                        : 'Autopilot uses the live Claw402 board by default.'}
                    </div>
                  </button>
                </div>

                <div className="hidden mb-4 flex-wrap items-center gap-3">
                  <span className="text-sm text-nofx-text-muted">
                    {selectedSymbols.length > 0
                      ? `${selectedSymbols.length} selected`
                      : `Without manual picks, runtime uses Claw402.ai Top ${coinSource.vergex_limit || 5} in this range`}
                  </span>
                  {selectedSymbols.length === 0 ? (
                    <select
                      value={coinSource.vergex_limit || 5}
                      onChange={(event) =>
                        patchCoinSource({
                          vergex_limit: Math.max(Number(event.target.value), 5),
                        })
                      }
                      className="rounded-lg border border-white/10 bg-nofx-bg px-3 py-2 text-sm text-nofx-text"
                    >
                      {topNOptions.map((value) => (
                        <option key={value} value={value}>
                          Top {value}
                        </option>
                      ))}
                    </select>
                  ) : null}
                </div>

                {symbolsError || signalsError ? (
                  <div className="mb-4 rounded-lg border border-amber-400/20 bg-amber-500/10 px-3 py-2 text-xs text-amber-200">
                    {symbolsError || signalsError}
                  </div>
                ) : null}

                {listMode === 'claw402' &&
                signals.length === 0 &&
                !signalsLoading ? (
                  <button
                    type="button"
                    onClick={() => {
                      void loadSignals()
                    }}
                    className="mb-4 inline-flex items-center gap-2 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-3 text-sm font-semibold text-nofx-gold hover:bg-nofx-gold/15"
                  >
                    <Sparkles className="h-4 w-4" />
                    Load Signal Board
                  </button>
                ) : null}

                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  {listMode === 'claw402' && signals.length > 0
                    ? visibleSignalItems.map((item) => {
                        const symbol = normalizeSymbol(item.symbol)
                        const selected = selectedSet.has(symbol)
                        const detailSelected = sameSignalItem(
                          detailSignal,
                          item
                        )
                        const bias = signalBiasInfo(item.bias)
                        const BiasIcon = bias.icon
                        return (
                          <div
                            key={`claw402-${item.rank || 0}-${symbol}`}
                            role="button"
                            tabIndex={0}
                            onClick={() => void loadSignalDetail(item)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' || event.key === ' ') {
                                event.preventDefault()
                                void loadSignalDetail(item)
                              }
                            }}
                            className={`cursor-pointer rounded-lg border p-3 text-left transition ${
                              detailSelected || selected
                                ? 'border-nofx-gold bg-nofx-gold/10'
                                : 'border-white/10 bg-black/20 hover:border-white/25'
                            }`}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <span className="font-mono text-base font-semibold text-nofx-text">
                                {symbol}
                              </span>
                              <span className="font-mono text-xs text-nofx-gold">
                                #{item.rank || '-'}
                              </span>
                            </div>
                            <div className="mt-4 flex items-center justify-between gap-3">
                              <div
                                className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-xs font-semibold ${bias.classes}`}
                              >
                                <BiasIcon className="h-3.5 w-3.5" />
                                {bias.label}
                              </div>
                              <span className="font-mono text-xs text-nofx-text-muted">
                                {formatSignalStrength(item)}
                              </span>
                            </div>
                            <div className="mt-4 flex items-center justify-between gap-3 border-t border-white/10 pt-3 text-[11px] uppercase tracking-wide text-nofx-text-muted">
                              <span>{categoryLabel(item.category, 'en')}</span>
                              <span>{signalMarketType(item)}</span>
                            </div>
                          </div>
                        )
                      })
                    : visibleSymbols.map((item) => {
                        const symbol = normalizeSymbol(item.symbol)
                        const signal = signalMap.get(symbol)
                        const selected = selectedSet.has(symbol)
                        return (
                          <button
                            key={`${item.exchange}-${symbol}`}
                            type="button"
                            onClick={() => toggleSymbol(symbol)}
                            className={`rounded-lg border p-3 text-left transition ${
                              selected
                                ? 'border-nofx-gold bg-nofx-gold/10'
                                : 'border-white/10 bg-black/20 hover:border-white/25'
                            }`}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <span className="font-mono text-sm font-semibold text-nofx-text">
                                {symbol}
                              </span>
                              <span className="text-[10px] text-nofx-text-muted">
                                {signal?.rank
                                  ? `#${signal.rank}`
                                  : formatChange(item.change_24h_pct)}
                              </span>
                            </div>
                            <div className="mt-2 flex items-center justify-between gap-2 text-[11px] text-nofx-text-muted">
                              <span>{categoryLabel(item.category, 'en')}</span>
                              <span>
                                {signal?.bias ||
                                  (item.mark_price
                                    ? `$${item.mark_price.toFixed(2)}`
                                    : 'ready')}
                              </span>
                            </div>
                          </button>
                        )
                      })}
                </div>

                {listMode === 'claw402' ? (
                  <div
                    id="claw402-detail-panel"
                    className="mt-4 scroll-mt-28 space-y-4"
                  >
                    {detailSignal ? (
                      <>
                        <section className="rounded-lg border border-white/10 bg-[#0d121a] px-4 py-3">
                          <div className="grid gap-3 lg:grid-cols-[1fr_auto] lg:items-center">
                            <div className="min-w-0">
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="font-mono text-xl font-semibold text-nofx-text">
                                  {normalizeSymbol(detailSignal.symbol)}
                                </span>
                                <span className="rounded-md bg-nofx-gold/10 px-2 py-1 text-xs font-semibold text-nofx-gold">
                                  #{detailSignal.rank || '-'}
                                </span>
                                <span className="rounded-md bg-white/[0.05] px-2 py-1 text-xs text-nofx-text-muted">
                                  {categoryLabel(detailSignal.category, 'en')}
                                </span>
                              </div>
                              <div className="mt-2 flex flex-wrap gap-2 font-mono text-xs text-nofx-text-muted">
                                <span>{signalMarketType(detailSignal)}</span>
                                <span>·</span>
                                <span>
                                  {strategySymbolForSignal(detailSignal)}
                                </span>
                                <span>·</span>
                                <span>mainnet</span>
                                <span>·</span>
                                <span>±{detailLiqBand}% band</span>
                              </div>
                            </div>
                            <div className="flex flex-wrap items-center gap-2 lg:justify-end">
                              <button
                                type="button"
                                onClick={() =>
                                  void loadSignalDetail(
                                    detailSignal,
                                    detailLiqBand
                                  )
                                }
                                disabled={detailLoading}
                                className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-2 text-xs text-nofx-text-muted hover:text-white disabled:opacity-50"
                              >
                                <RefreshCw
                                  className={`h-3.5 w-3.5 ${
                                    detailLoading ? 'animate-spin' : ''
                                  }`}
                                />
                                Refresh
                              </button>
                            </div>
                          </div>
                          {detailLoading ? (
                            <div className="mt-3 inline-flex items-center gap-2 text-xs text-nofx-text-muted">
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              Loading Signal Lab and heatmap...
                            </div>
                          ) : null}
                          {detailError ? (
                            <div className="mt-3 rounded-md border border-amber-400/20 bg-amber-500/10 px-3 py-2 text-xs text-amber-200">
                              {detailError}
                            </div>
                          ) : null}
                        </section>

                        <CostLiquidationHeatmap heatmap={heatmap} />
                        <SignalLabPanel
                          lab={signalLab}
                          activeBand={detailLiqBand}
                          loading={detailLoading}
                          onBandChange={selectDetailBand}
                        />
                      </>
                    ) : (
                      <div className="rounded-lg border border-white/10 bg-black/20 px-4 py-4 text-sm text-nofx-text-muted">
                        NOFX Autopilot reviews the Claw402 board, Signal Lab,
                        liquidation structure, and raw candles automatically.
                      </div>
                    )}
                  </div>
                ) : null}

                {listMode === 'claw402' &&
                signals.length > 0 &&
                visibleSignalItems.length === 0 ? (
                  <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 text-sm text-nofx-text-muted">
                    No Claw402 markets available.
                  </div>
                ) : null}

                {listMode === 'pool' &&
                visibleSymbols.length === 0 &&
                !symbolsLoading ? (
                  <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-3 text-sm text-nofx-text-muted">
                    No markets available.
                  </div>
                ) : null}
              </section>

              <details className="hidden rounded-lg border border-white/10 bg-black/15 p-4">
                <summary className="cursor-pointer text-sm font-semibold text-nofx-text">
                  {text(language, '高级设置', 'Advanced settings')}
                </summary>
                <div className="mt-4 rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                  <div className="mb-3 text-sm font-semibold text-nofx-text">
                    {text(language, '交易风格', 'Trading style')}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {profileOptions.map((profile) => (
                      <button
                        key={profile.value}
                        type="button"
                        onClick={() => applyProfile(profile)}
                        className={`rounded-lg border px-3 py-2 text-sm transition ${
                          activeProfile === profile.value
                            ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-gold'
                            : 'border-white/10 text-nofx-text-muted hover:text-white'
                        }`}
                      >
                        {text(language, profile.zh, profile.en)}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="mt-4 grid gap-4 lg:grid-cols-2">
                  <div className="rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                    <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-nofx-text">
                      <Sparkles className="h-4 w-4 text-nofx-gold" />
                      {text(language, '原始 K 线', 'Raw candles')}
                    </div>
                    <div className="space-y-4">
                      <div>
                        <div className="mb-2 text-xs text-nofx-text-muted">
                          {text(language, '时间周期', 'Timeframe')}
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {timeframeOptions.map((timeframe) => (
                            <button
                              key={timeframe}
                              type="button"
                              onClick={() => setTimeframe(timeframe)}
                              className={`rounded-lg border px-3 py-2 text-sm ${
                                indicators.klines.primary_timeframe ===
                                timeframe
                                  ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-gold'
                                  : 'border-white/10 text-nofx-text-muted hover:text-white'
                              }`}
                            >
                              {timeframe}
                            </button>
                          ))}
                        </div>
                      </div>
                      <div>
                        <div className="mb-2 text-xs text-nofx-text-muted">
                          {text(language, 'K 线根数', 'Bars')}
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {barCountOptions.map((count) => (
                            <button
                              key={count}
                              type="button"
                              onClick={() => setBarCount(count)}
                              className={`rounded-lg border px-3 py-2 text-sm ${
                                indicators.klines.primary_count === count
                                  ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-gold'
                                  : 'border-white/10 text-nofx-text-muted hover:text-white'
                              }`}
                            >
                              {count}
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                    <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-nofx-text">
                      <Shield className="h-4 w-4 text-emerald-300" />
                      {text(language, '交易参数', 'Trading parameters')}
                    </div>
                    <div className="grid gap-4 sm:grid-cols-3">
                      <label className="space-y-2">
                        <span className="text-xs text-nofx-text-muted">
                          {text(language, '最多持仓', 'Max positions')}
                        </span>
                        <select
                          value={risk.max_positions}
                          onChange={(event) =>
                            patchRisk({
                              max_positions: Number(event.target.value),
                            })
                          }
                          className="w-full rounded-lg border border-white/10 bg-nofx-bg px-3 py-2 text-sm text-nofx-text"
                        >
                          {[1, 2, 3, 4, 5].map((value) => (
                            <option key={value} value={value}>
                              {value}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label className="space-y-2">
                        <span className="text-xs text-nofx-text-muted">
                          {text(language, '杠杆', 'Leverage')}
                        </span>
                        <select
                          value={risk.altcoin_max_leverage}
                          onChange={(event) =>
                            setLeverage(Number(event.target.value))
                          }
                          className="w-full rounded-lg border border-white/10 bg-nofx-bg px-3 py-2 text-sm text-nofx-text"
                        >
                          {[1, 2, 3, 5, 8, 10].map((value) => (
                            <option key={value} value={value}>
                              {value}x
                            </option>
                          ))}
                        </select>
                      </label>
                      <label className="space-y-2">
                        <span className="text-xs text-nofx-text-muted">
                          {text(language, '开仓置信度', 'Entry confidence')}
                        </span>
                        <select
                          value={risk.min_confidence}
                          onChange={(event) =>
                            patchRisk({
                              min_confidence: Number(event.target.value),
                            })
                          }
                          className="w-full rounded-lg border border-white/10 bg-nofx-bg px-3 py-2 text-sm text-nofx-text"
                        >
                          {confidenceOptions.map((value) => (
                            <option key={value} value={value}>
                              {value}%
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>
                  </div>
                </div>

                <div className="mt-4 rounded-lg border border-white/10 bg-nofx-bg-lighter p-4">
                  <div className="mb-2 text-sm font-semibold text-nofx-text">
                    {text(language, '策略备注', 'Strategy note')}
                  </div>
                  <textarea
                    value={aiConfig.custom_prompt || ''}
                    onChange={(event) =>
                      patchAI({ custom_prompt: event.target.value })
                    }
                    placeholder={text(
                      language,
                      '例如：只做趋势清晰的标的；如果榜单信号和 K 线冲突就不开仓。',
                      'Example: only trade clean trends; skip entries when board signals conflict with candles.'
                    )}
                    className="h-28 w-full resize-none rounded-lg border border-white/10 bg-nofx-bg px-3 py-2 text-sm text-nofx-text outline-none placeholder:text-nofx-text-muted/50"
                  />
                </div>
              </details>
            </div>
          ) : (
            <div className="flex h-full items-center justify-center">
              <button
                type="button"
                onClick={createStrategy}
                className="inline-flex items-center gap-2 rounded-lg bg-nofx-gold px-4 py-2 text-sm font-semibold text-black hover:bg-yellow-400"
              >
                <Plus className="h-4 w-4" />
                {text(language, 'Initialize Autopilot', 'Initialize Autopilot')}
              </button>
            </div>
          )}
        </main>
      </div>
    </DeepVoidBackground>
  )
}

export default StrategyStudioPage
