import { useEffect, useMemo, useState } from 'react'
import { BarChart3, Check, Globe2, Search, Star, X } from 'lucide-react'
import type { CoinSourceConfig } from '../../types'

const API_BASE = import.meta.env.VITE_API_BASE || ''

interface CoinSourceEditorProps {
  config: CoinSourceConfig
  onChange: (config: CoinSourceConfig) => void
  disabled?: boolean
  language: string
}

interface MarketSymbol {
  symbol: string
  display?: string
  name?: string
  category?: string
  mark_price?: number
  volume_24h?: number
  change_24h_pct?: number
}

const t = (language: string, zh: string, en: string) => (language === 'zh' ? zh : en)

const categoryLabels: Record<string, string> = {
  stock: 'Stocks',
  commodity: 'Commodities',
  index: 'Indices',
  forex: 'FX',
  pre_ipo: 'Pre-IPO',
  crypto: 'Crypto',
}

const categoryOrder = ['stock', 'commodity', 'index', 'forex', 'pre_ipo', 'crypto']

const rankDirections = [
  { value: 'gainers', labelZh: '涨幅榜', labelEn: 'Gainers' },
  { value: 'losers', labelZh: '跌幅榜', labelEn: 'Losers' },
  { value: 'volume', labelZh: '成交额榜', labelEn: 'Volume' },
] as const

const SELECTED_MARKET_LIMIT = 10
const RANK_LIMIT = 10
const DEFAULT_RANK_LIMIT = 5
const CATALOG_DISPLAY_LIMIT = 120

function formatCompactNumber(value?: number) {
  if (!value || Number.isNaN(value)) return '—'
  if (value >= 1_000_000_000) return `$${(value / 1_000_000_000).toFixed(1)}B`
  if (value >= 1_000_000) return `$${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1_000) return `$${(value / 1_000).toFixed(1)}K`
  return `$${value.toFixed(0)}`
}

function displaySymbol(symbol?: MarketSymbol) {
  return symbol?.display || symbol?.symbol || ''
}

export function CoinSourceEditor({ config, onChange, disabled, language }: CoinSourceEditorProps) {
  const [symbols, setSymbols] = useState<MarketSymbol[]>([])
  const [loadingSymbols, setLoadingSymbols] = useState(false)
  const [symbolError, setSymbolError] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState<string>('all')

  useEffect(() => {
    let cancelled = false
    const loadSymbols = async () => {
      setLoadingSymbols(true)
      setSymbolError(null)
      try {
        const res = await fetch(`${API_BASE}/api/symbols?exchange=hyperliquid-xyz`)
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const data = await res.json()
        const rows: MarketSymbol[] = data.symbols || []
        if (!cancelled) setSymbols(rows)
      } catch (err) {
        if (!cancelled) setSymbolError(err instanceof Error ? err.message : 'Failed to load symbols')
      } finally {
        if (!cancelled) setLoadingSymbols(false)
      }
    }
    loadSymbols()
    return () => {
      cancelled = true
    }
  }, [])

  const selectedCoins = config.static_coins || []
  const selectedSet = useMemo(() => new Set(selectedCoins), [selectedCoins])
  const selectedMarketSymbols = useMemo(
    () => selectedCoins.map((coin) => symbols.find((s) => s.symbol === coin) || { symbol: coin, display: coin }),
    [selectedCoins, symbols]
  )

  const filteredSymbols = useMemo(() => {
    const q = query.trim().toLowerCase()
    return symbols
      .filter((symbol) => category === 'all' || (symbol.category || 'crypto') === category)
      .filter((symbol) => {
        if (!q) return true
        return [symbol.symbol, symbol.display, symbol.name, symbol.category]
          .filter(Boolean)
          .some((v) => String(v).toLowerCase().includes(q))
      })
      .slice(0, CATALOG_DISPLAY_LIMIT)
  }, [symbols, query, category])

  const rankedPreview = useMemo(() => {
    const rankCategory = config.hyper_rank_category || 'stock'
    const rankDirection = config.hyper_rank_direction || 'gainers'
    const rankLimit = Math.min(Math.max(config.hyper_rank_limit || DEFAULT_RANK_LIMIT, 1), RANK_LIMIT)
    const filtered = symbols.filter((symbol) => rankCategory === 'all' || (symbol.category || 'crypto') === rankCategory)
    const sorted = [...filtered].sort((a, b) => {
      if (rankDirection === 'losers') return (a.change_24h_pct || 0) - (b.change_24h_pct || 0)
      if (rankDirection === 'volume') return (b.volume_24h || 0) - (a.volume_24h || 0)
      return (b.change_24h_pct || 0) - (a.change_24h_pct || 0)
    })
    return sorted.slice(0, rankLimit)
  }, [symbols, config.hyper_rank_category, config.hyper_rank_direction, config.hyper_rank_limit])

  const chooseSource = (sourceType: CoinSourceConfig['source_type']) => {
    if (disabled) return
    onChange({
      ...config,
      source_type: sourceType,
      use_ai500: false,
      use_oi_top: false,
      use_oi_low: false,
      use_hyper_all: false,
      use_hyper_main: false,
      hyper_rank_category: config.hyper_rank_category || 'stock',
      hyper_rank_direction: config.hyper_rank_direction || 'gainers',
      hyper_rank_limit: Math.min(Math.max(config.hyper_rank_limit || DEFAULT_RANK_LIMIT, 1), RANK_LIMIT),
    })
  }

  const updateRank = (patch: Partial<CoinSourceConfig>) => {
    if (disabled) return
    onChange({
      ...config,
      source_type: 'hyper_rank',
      use_ai500: false,
      use_oi_top: false,
      use_oi_low: false,
      use_hyper_all: false,
      use_hyper_main: false,
      hyper_rank_category: config.hyper_rank_category || 'stock',
      hyper_rank_direction: config.hyper_rank_direction || 'gainers',
      hyper_rank_limit: Math.min(Math.max(config.hyper_rank_limit || DEFAULT_RANK_LIMIT, 1), RANK_LIMIT),
      ...patch,
    })
  }

  const addSymbol = (symbol: MarketSymbol) => {
    if (disabled || selectedSet.has(symbol.symbol) || selectedCoins.length >= SELECTED_MARKET_LIMIT) return
    onChange({
      ...config,
      source_type: 'static',
      use_ai500: false,
      use_oi_top: false,
      use_oi_low: false,
      use_hyper_all: false,
      use_hyper_main: false,
      static_coins: [...selectedCoins, symbol.symbol],
    })
  }

  const removeSymbol = (symbol: string) => {
    if (disabled) return
    onChange({
      ...config,
      static_coins: selectedCoins.filter((coin) => coin !== symbol),
    })
  }

  return (
    <div className="space-y-5">
      <div className="rounded-2xl border border-sky-400/20 bg-gradient-to-br from-sky-500/10 via-nofx-bg-lighter to-nofx-bg p-4">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2 text-nofx-text">
              <Globe2 className="w-5 h-5 text-sky-300" />
              <h3 className="font-semibold">
                {t(language, 'Hyperliquid 原生标的', 'Native Hyperliquid universe')}
              </h3>
            </div>
            <p className="mt-1 text-xs text-nofx-text-muted">
              {t(
                language,
                '只使用 Hyperliquid 实时 Universe / K 线 / 标记价格；不混入外部聚合数据。',
                'Uses Hyperliquid live universe, candles and mark prices only; no external aggregate datasets are mixed in.'
              )}
            </p>
          </div>
          <span className="rounded-full border border-sky-400/30 bg-sky-400/10 px-3 py-1 text-[11px] text-sky-200">
            {symbols.length || '—'} {t(language, '个可视化标的', 'visual markets')}
          </span>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        {[
          {
            value: 'hyper_rank' as const,
            icon: BarChart3,
            title: t(language, '动态榜单', 'Dynamic ranking'),
            desc: t(language, '美股/大宗/指数/FX/Crypto 的涨幅榜、跌幅榜、成交额榜；默认 Top 5，最多 Top 10', 'Gainers, losers and volume rankings by asset class; default Top 5, max Top 10'),
          },
          {
            value: 'static' as const,
            icon: Star,
            title: t(language, '自选单标的/组合', 'Selected market(s)'),
            desc: t(language, '从下方卡片点选 1-10 个固定标的', 'Pick 1-10 fixed markets from visual cards below'),
          },
        ].map(({ value, icon: Icon, title, desc }) => {
          const active = config.source_type === value
          return (
            <button
              key={value}
              type="button"
              disabled={disabled}
              onClick={() => chooseSource(value)}
              className={`rounded-xl border p-4 text-left transition-all ${
                active
                  ? 'border-sky-300/70 bg-sky-400/10 shadow-[0_0_24px_rgba(56,189,248,0.12)]'
                  : 'border-white/10 bg-nofx-bg hover:border-sky-400/40 hover:bg-white/[0.03]'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <Icon className="w-5 h-5 text-sky-300" />
                {active && <Check className="w-4 h-4 text-sky-300" />}
              </div>
              <div className="mt-3 text-sm font-semibold text-nofx-text">{title}</div>
              <div className="mt-1 text-xs leading-5 text-nofx-text-muted">{desc}</div>
            </button>
          )
        })}
      </div>

      {config.source_type === 'hyper_rank' && (
        <div className="space-y-3 rounded-2xl border border-violet-400/20 bg-violet-500/5 p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-nofx-text">{t(language, '榜单规则', 'Ranking rule')}</div>
              <div className="text-xs text-nofx-text-muted">{t(language, '动态选出当前榜单前 N 个；默认 Top 5，最多 Top 10。下方仍显示全量可见标的，可手动改成自选。', 'Select current top N dynamically; default Top 5, max Top 10. The full visible market catalog remains below for manual selection.')}</div>
            </div>
            <select
              value={Math.min(Math.max(config.hyper_rank_limit || DEFAULT_RANK_LIMIT, 1), RANK_LIMIT)}
              disabled={disabled}
              onChange={(e) => updateRank({ hyper_rank_limit: Math.min(Number(e.target.value) || DEFAULT_RANK_LIMIT, RANK_LIMIT) })}
              className="rounded-lg border border-violet-300/20 bg-nofx-bg px-3 py-1.5 text-sm text-nofx-text"
            >
              {Array.from({ length: RANK_LIMIT }, (_, i) => i + 1).map((n) => (
                <option key={n} value={n}>Top {n}</option>
              ))}
            </select>
          </div>

          <div className="grid gap-2 sm:grid-cols-3 xl:grid-cols-6">
            {[...categoryOrder, 'all'].map((cat) => (
              <button
                key={cat}
                type="button"
                disabled={disabled}
                onClick={() => updateRank({ hyper_rank_category: cat as CoinSourceConfig['hyper_rank_category'] })}
                className={`rounded-xl border px-3 py-2 text-xs transition-all ${
                  (config.hyper_rank_category || 'stock') === cat
                    ? 'border-sky-300/70 bg-sky-400/10 text-sky-100'
                    : 'border-white/10 bg-white/[0.02] text-nofx-text-muted hover:text-white'
                }`}
              >
                {cat === 'all' ? t(language, '全部', 'All') : categoryLabels[cat] || cat}
              </button>
            ))}
          </div>

          <div className="grid gap-2 sm:grid-cols-3">
            {rankDirections.map((item) => (
              <button
                key={item.value}
                type="button"
                disabled={disabled}
                onClick={() => updateRank({ hyper_rank_direction: item.value })}
                className={`rounded-xl border px-3 py-2 text-sm transition-all ${
                  (config.hyper_rank_direction || 'gainers') === item.value
                    ? 'border-violet-300/70 bg-violet-400/10 text-violet-100'
                    : 'border-white/10 bg-white/[0.02] text-nofx-text-muted hover:text-white'
                }`}
              >
                {t(language, item.labelZh, item.labelEn)}
              </button>
            ))}
          </div>

          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
            {rankedPreview.map((symbol, index) => (
              <div key={symbol.symbol} className="rounded-xl border border-white/10 bg-black/20 p-3">
                <div className="text-[11px] text-nofx-text-muted">#{index + 1}</div>
                <div className="mt-1 text-sm font-semibold text-nofx-text">{displaySymbol(symbol)}</div>
                <div className="mt-2 flex items-center justify-between text-[11px]">
                  <span className="text-nofx-text-muted">Vol {formatCompactNumber(symbol.volume_24h)}</span>
                  {typeof symbol.change_24h_pct === 'number' && (
                    <span className={symbol.change_24h_pct >= 0 ? 'text-nofx-success' : 'text-nofx-danger'}>
                      {symbol.change_24h_pct >= 0 ? '+' : ''}{symbol.change_24h_pct.toFixed(2)}%
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-4">
          <div>
            <div className="mb-2 flex items-center justify-between gap-3 text-sm font-medium text-nofx-text">
              <span>{t(language, '自选标的', 'Selected markets')}</span>
              <span className="text-xs font-normal text-nofx-text-muted">{selectedCoins.length}/{SELECTED_MARKET_LIMIT}</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {selectedMarketSymbols.length > 0 ? selectedMarketSymbols.map((symbol) => (
                <span key={symbol.symbol} className="inline-flex items-center gap-2 rounded-full border border-sky-300/25 bg-sky-400/10 px-3 py-1.5 text-sm text-sky-100">
                  {displaySymbol(symbol)}
                  {!disabled && (
                    <button type="button" onClick={() => removeSymbol(symbol.symbol)} className="text-sky-200 hover:text-white">
                      <X className="w-3.5 h-3.5" />
                    </button>
                  )}
                </span>
              )) : (
                <span className="text-xs text-nofx-text-muted">
                  {t(language, '点击下方标的卡片添加。', 'Click market cards below to add.')}
                </span>
              )}
            </div>
          </div>

          <div className="rounded-2xl border border-white/10 bg-nofx-bg/80 p-3">
            <div className="mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-nofx-text-muted" />
                <input
                  value={query}
                  disabled={disabled}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t(language, '搜索 SAMSUNG / TESLA / GOLD…', 'Search SAMSUNG / TESLA / GOLD…')}
                  className="w-full rounded-xl border border-white/10 bg-nofx-bg py-2 pl-9 pr-3 text-sm text-nofx-text outline-none focus:border-sky-400/50"
                />
              </div>
              <div className="flex flex-wrap gap-1.5">
                {['all', ...categoryOrder].map((cat) => (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => setCategory(cat)}
                    className={`rounded-full px-2.5 py-1 text-[11px] transition-colors ${
                      category === cat ? 'bg-sky-400 text-black' : 'bg-white/5 text-nofx-text-muted hover:text-white'
                    }`}
                  >
                    {cat === 'all' ? t(language, '全部', 'All') : categoryLabels[cat] || cat}
                  </button>
                ))}
              </div>
            </div>

            {loadingSymbols && <div className="py-8 text-center text-sm text-nofx-text-muted">{t(language, '加载 Hyperliquid 标的中…', 'Loading Hyperliquid markets…')}</div>}
            {symbolError && <div className="py-6 text-center text-sm text-nofx-danger">{symbolError}</div>}

            {!loadingSymbols && !symbolError && (
              <div className="grid max-h-[420px] gap-2 overflow-y-auto pr-1 sm:grid-cols-2 xl:grid-cols-3">
                {filteredSymbols.map((symbol) => {
                  const selected = selectedSet.has(symbol.symbol)
                  const change = symbol.change_24h_pct
                  return (
                    <button
                      key={symbol.symbol}
                      type="button"
                      disabled={disabled || selected || selectedCoins.length >= SELECTED_MARKET_LIMIT}
                      onClick={() => addSymbol(symbol)}
                      className={`rounded-xl border p-3 text-left transition-all ${
                        selected
                          ? 'border-sky-300/50 bg-sky-400/10'
                          : 'border-white/10 bg-white/[0.02] hover:border-sky-400/40 hover:bg-sky-400/[0.06]'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="text-sm font-semibold text-nofx-text">{displaySymbol(symbol)}</div>
                          <div className="mt-0.5 text-[11px] text-nofx-text-muted">{categoryLabels[symbol.category || 'crypto'] || symbol.category || 'Crypto'}</div>
                        </div>
                        {selected && <Check className="w-4 h-4 text-sky-300" />}
                      </div>
                      <div className="mt-3 flex items-center justify-between text-[11px]">
                        <span className="text-nofx-text-muted">Vol {formatCompactNumber(symbol.volume_24h)}</span>
                        {typeof change === 'number' && (
                          <span className={change >= 0 ? 'text-nofx-success' : 'text-nofx-danger'}>
                            {change >= 0 ? '+' : ''}{change.toFixed(2)}%
                          </span>
                        )}
                      </div>
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        </div>
    </div>
  )
}
