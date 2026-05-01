import { useState, useEffect, useCallback } from 'react'
import useSWR from 'swr'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import type { Language } from '../i18n/translations'
import { api } from '../lib/api'
import type { HotCoinResponse, CoinDataResponse, CompositeMarketSnapshot, HotCoinItem } from '../lib/api/market'

type Tab = 'hot' | 'oi-top' | 'oi-low'

export function MarketDataPage() {
  const { language } = useLanguage()
  const [tab, setTab] = useState<Tab>('hot')
  const [exchange, setExchange] = useState('okx')
  const [limit, setLimit] = useState(20)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [selectedCoin, setSelectedCoin] = useState<string | null>(null)

  const fetcher = useCallback(() => {
    if (tab === 'hot') return api.getHotCoins(limit, exchange)
    if (tab === 'oi-top') return api.getOIRanking('top', limit)
    return api.getOIRanking('low', limit)
  }, [tab, exchange, limit])

  const { data, isLoading, mutate } = useSWR<HotCoinResponse>(
    `market-${tab}-${exchange}-${limit}`,
    fetcher,
    { refreshInterval: autoRefresh ? 60000 : 0 }
  )

  const { data: coinData, isLoading: coinLoading } = useSWR<CoinDataResponse>(
    selectedCoin ? `coin-data-${selectedCoin}` : null,
    () => api.getCoinData(selectedCoin!),
    { refreshInterval: autoRefresh ? 180000 : 0, dedupingInterval: 30000 }
  )

  const { data: compositeData } = useSWR<CompositeMarketSnapshot>(
    selectedCoin ? `composite-market-${selectedCoin}-${exchange}` : null,
    () => api.getCompositeMarket(selectedCoin!, exchange, 180, 'chart'),
    { refreshInterval: autoRefresh ? 180000 : 0, dedupingInterval: 30000 }
  )

  // Re-fetch when params change
  useEffect(() => { mutate() }, [tab, exchange, limit, mutate])

  const tabs: { key: Tab; label: string }[] = [
    { key: 'hot', label: t('hotCoins', language) },
    { key: 'oi-top', label: t('oiIncrease', language) },
    { key: 'oi-low', label: t('oiDecrease', language) },
  ]

  const fmt = (n: number, decimals = 2) => {
    if (Math.abs(n) >= 1e9) return (n / 1e9).toFixed(decimals) + 'B'
    if (Math.abs(n) >= 1e6) return (n / 1e6).toFixed(decimals) + 'M'
    if (Math.abs(n) >= 1e3) return (n / 1e3).toFixed(decimals) + 'K'
    return n.toFixed(decimals)
  }

  const pctColor = (v: number) => v >= 0 ? 'text-emerald-400' : 'text-red-400'

  return (
    <div className="max-w-7xl mx-auto px-4 py-6">
      {/* Controls */}
      <div className="flex flex-wrap items-center gap-3 mb-6">
        {/* Tabs */}
        <div className="flex bg-zinc-900 rounded-lg p-1 border border-zinc-800">
          {tabs.map(tb => (
            <button
              key={tb.key}
              onClick={() => setTab(tb.key)}
              className={`px-4 py-1.5 text-sm font-medium rounded-md transition-all ${
                tab === tb.key
                  ? 'bg-nofx-gold/20 text-nofx-gold'
                  : 'text-zinc-500 hover:text-zinc-300'
              }`}
            >
              {tb.label}
            </button>
          ))}
        </div>

        {/* Exchange toggle (only for hot coins) */}
        {tab === 'hot' && (
          <div className="flex bg-zinc-900 rounded-lg p-1 border border-zinc-800">
            {['binance', 'okx'].map(ex => (
              <button
                key={ex}
                onClick={() => setExchange(ex)}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all uppercase ${
                  exchange === ex
                    ? 'bg-zinc-700 text-white'
                    : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {ex}
              </button>
            ))}
          </div>
        )}

        {/* Limit */}
        <div className="flex bg-zinc-900 rounded-lg p-1 border border-zinc-800">
          {[10, 20, 50].map(n => (
            <button
              key={n}
              onClick={() => setLimit(n)}
              className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                limit === n
                  ? 'bg-zinc-700 text-white'
                  : 'text-zinc-500 hover:text-zinc-300'
              }`}
            >
              {n}
            </button>
          ))}
        </div>

        {/* Auto refresh */}
        <button
          onClick={() => setAutoRefresh(!autoRefresh)}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
            autoRefresh
              ? 'border-emerald-600 text-emerald-400 bg-emerald-900/20'
              : 'border-zinc-700 text-zinc-500'
          }`}
        >
          {t('autoRefresh', language)} {autoRefresh ? '●' : '○'}
        </button>

        {data?.updated_at && (
          <span className="text-xs text-zinc-600 ml-auto">
            {new Date(data.updated_at).toLocaleTimeString()}
          </span>
        )}
      </div>

      {/* Table */}
      <div className="bg-zinc-900/50 border border-zinc-800 rounded-xl overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center py-20 text-zinc-500 text-sm">
            {t('loadingText', language)}
          </div>
        ) : !data?.coins?.length ? (
          <div className="flex items-center justify-center py-20 text-zinc-500 text-sm">
            {t('noDataAvailable', language)}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-800 text-zinc-500 text-xs uppercase tracking-wider">
                  <th className="px-4 py-3 text-left w-12">#</th>
                  <th className="px-4 py-3 text-left">{t('symbol', language)}</th>
                  <th className="px-4 py-3 text-right">{t('change24h', language)}</th>
                  <th className="px-4 py-3 text-right">{t('volume24h', language)}</th>
                  <th className="px-4 py-3 text-right">{t('openInterest', language)}</th>
                  <th className="px-4 py-3 text-right">Quality</th>
                  {tab === 'hot' ? (
                    <th className="px-4 py-3 text-right">{t('compositeScore', language)}</th>
                  ) : (
                    <th className="px-4 py-3 text-right">{t('oiChange', language)}</th>
                  )}
                </tr>
              </thead>
              <tbody>
                {data.coins.map((coin, i) => (
                  <tr
                    key={coin.symbol}
                    onClick={() => setSelectedCoin(coin.symbol)}
                    className="border-b border-zinc-800/50 hover:bg-zinc-800/30 cursor-pointer transition-colors"
                  >
                    <td className="px-4 py-3 text-zinc-600 font-mono">{i + 1}</td>
                    <td className="px-4 py-3 font-medium text-white">
                      {coin.symbol.replace('USDT', '')}
                      <span className="text-zinc-600 text-xs ml-1">USDT</span>
                    </td>
                    <td className={`px-4 py-3 text-right font-mono ${pctColor(coin.price_change_24h)}`}>
                      {coin.price_change_24h >= 0 ? '+' : ''}{coin.price_change_24h.toFixed(2)}%
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-zinc-300">
                      ${fmt(coin.volume_24h)}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-zinc-300">
                      ${fmt(coin.oi)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <QualityPills quality={coin.quality} />
                    </td>
                    {tab === 'hot' ? (
                      <td className="px-4 py-3 text-right">
                        <ScoreBar score={coin.score} />
                      </td>
                    ) : (
                      <td className={`px-4 py-3 text-right font-mono ${pctColor(coin.oi_change_pct ?? coin.score)}`}>
                        {formatOIChangeCell(coin)}
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Coin Detail Drawer */}
      {selectedCoin && (
        <CoinDrawer
          symbol={selectedCoin}
          data={coinData}
          composite={compositeData}
          loading={coinLoading}
          language={language}
          onClose={() => setSelectedCoin(null)}
        />
      )}
    </div>
  )
}

function formatOIChangeCell(coin: HotCoinItem): string {
  const v = coin.oi_change_pct ?? coin.score
  const sign = v >= 0 ? '+' : ''
  const window = coin.oi_change_window_seconds ? `/${Math.round(coin.oi_change_window_seconds / 60)}m` : ''
  return `${sign}${v.toFixed(3)}%${window}`
}

function ScoreBar({ score }: { score: number }) {
  const pct = Math.min(score * 100, 100)
  const color = pct > 70 ? 'bg-emerald-500' : pct > 40 ? 'bg-nofx-gold' : 'bg-zinc-600'
  return (
    <div className="flex items-center gap-2 justify-end">
      <span className="text-xs font-mono text-zinc-400">{score.toFixed(2)}</span>
      <div className="w-16 h-1.5 bg-zinc-800 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

function CoinDrawer({
  symbol,
  data,
  composite,
  loading,
  language,
  onClose,
}: {
  symbol: string
  data?: CoinDataResponse
  composite?: CompositeMarketSnapshot
  loading: boolean
  language: Language
  onClose: () => void
}) {
  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/50 z-40" onClick={onClose} />
      {/* Panel */}
      <div className="fixed right-0 top-0 h-full w-full max-w-md bg-zinc-900 border-l border-zinc-800 z-50 overflow-y-auto">
        <div className="p-6">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-lg font-bold text-white">
              {symbol.replace('USDT', '')}
              <span className="text-zinc-500 text-sm ml-2">USDT</span>
            </h2>
            <button
              onClick={onClose}
              className="text-zinc-500 hover:text-white text-xl leading-none"
            >
              ✕
            </button>
          </div>

          {loading ? (
            <div className="text-zinc-500 text-sm py-10 text-center">
              {t('loadingText', language)}
            </div>
          ) : !data ? (
            <div className="text-zinc-500 text-sm py-10 text-center">
              {t('noDataAvailable', language)}
            </div>
          ) : (
            <div className="space-y-6">
              {/* Composite Snapshot */}
              {composite && (
                <Section title="综合市场快照 / AI Context">
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <Metric label="Quality" value={composite.data_quality || '-'} />
                    <Metric label="TTL" value={`${composite.ttl_seconds}s`} />
                    <Metric label="Updated" value={new Date(composite.updated_at).toLocaleTimeString()} />
                    <Metric label="Regime" value={(composite.context as any)?.regime_entry_guidance?.regime || '-'} />
                    <Metric label="Crowding" value={(composite.context as any)?.exchange_flow?.crowding_risk || '-'} />
                  </div>
                  {composite.sources && (
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      {composite.sources.map(src => (
                        <span key={src.name} className={`rounded-full border px-2 py-0.5 text-[10px] ${src.available ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-200' : 'border-zinc-600/30 bg-zinc-800/60 text-zinc-500'}`}>
                          {src.name}{src.available ? '' : ' off'}
                        </span>
                      ))}
                    </div>
                  )}
                </Section>
              )}

              {/* Price */}
              <div className="bg-zinc-800/50 rounded-lg p-4">
                <div className="text-2xl font-bold text-white font-mono">
                  ${data.current_price.toLocaleString(undefined, { maximumFractionDigits: 8 })}
                </div>
                <div className="flex gap-4 mt-2 text-xs">
                  <span className={data.price_change_1h >= 0 ? 'text-emerald-400' : 'text-red-400'}>
                    1h: {data.price_change_1h >= 0 ? '+' : ''}{data.price_change_1h.toFixed(2)}%
                  </span>
                  <span className={data.price_change_4h >= 0 ? 'text-emerald-400' : 'text-red-400'}>
                    4h: {data.price_change_4h >= 0 ? '+' : ''}{data.price_change_4h.toFixed(2)}%
                  </span>
                </div>
              </div>

              {/* Sentiment */}
              <Section title={t('marketSentiment', language)}>
                <Row label={t('fundingRate', language)} value={data.funding_rate != null ? (data.funding_rate * 100).toFixed(4) + '%' : '-'} />
                <Row label={t('longShortRatio', language)} value={data.long_short_ratio?.toFixed(3) ?? '-'} />
                <Row label={t('topTraderRatio', language)} value={data.top_trader_ratio?.toFixed(3) ?? '-'} />
                <Row label={t('takerRatio', language)} value={data.taker_buy_sell_ratio?.toFixed(3) ?? '-'} />
                <Row label={t('depthImbalance', language)} value={data.depth_imbalance != null ? (data.depth_imbalance * 100).toFixed(1) + '%' : '-'} />
              </Section>

              {/* Structural Levels */}
              {data.structural_levels && data.structural_levels.length > 0 && (
                <Section title={t('structuralLevels', language)}>
                  <div className="space-y-1">
                    {data.structural_levels.map((lvl, i) => (
                      <div key={i} className="flex items-center justify-between text-xs">
                        <span className={lvl.type === 'support' ? 'text-emerald-400' : 'text-red-400'}>
                          {lvl.type === 'support' ? t('supportLabel', language) : t('resistanceLabel', language)}
                        </span>
                        <span className="text-zinc-300 font-mono">${lvl.price.toLocaleString(undefined, { maximumFractionDigits: 4 })}</span>
                        <span className="text-zinc-600">{lvl.timeframe}</span>
                        <span className="text-zinc-500">{'★'.repeat(Math.min(lvl.strength, 5))}</span>
                      </div>
                    ))}
                  </div>
                </Section>
              )}

              {/* Fibonacci */}
              {data.fibonacci_levels && (
                <Section title={t('fibonacciLabel', language)}>
                  <div className="text-xs text-zinc-500 mb-2">
                    {data.fibonacci_levels.direction} · {data.fibonacci_levels.timeframe}
                  </div>
                  <div className="space-y-1">
                    {Object.entries(data.fibonacci_levels.levels)
                      .sort(([a], [b]) => parseFloat(a) - parseFloat(b))
                      .map(([level, price]) => (
                        <div key={level} className="flex justify-between text-xs">
                          <span className="text-nofx-gold">{level}</span>
                          <span className="text-zinc-300 font-mono">${price.toLocaleString(undefined, { maximumFractionDigits: 4 })}</span>
                        </div>
                      ))}
                  </div>
                </Section>
              )}

              {/* Chart */}
              {composite ? (
                <MarketMiniChart snapshot={composite} />
              ) : (
                <div className="bg-zinc-800/30 border border-zinc-700/50 rounded-lg p-8 text-center text-zinc-600 text-sm">
                  {t('chartComingSoon', language)}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-xs font-medium text-zinc-500 uppercase tracking-wider mb-3">{title}</h3>
      {children}
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between py-1.5 text-sm">
      <span className="text-zinc-500">{label}</span>
      <span className="text-zinc-200 font-mono">{value}</span>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-white/10 bg-black/20 px-2 py-1.5">
      <div className="text-[10px] uppercase tracking-wide text-zinc-500">{label}</div>
      <div className="truncate font-mono text-xs text-zinc-200">{value}</div>
    </div>
  )
}

function MarketMiniChart({ snapshot }: { snapshot: CompositeMarketSnapshot }) {
  const tf = snapshot.timeframes?.[snapshot.primary_timeframe] || Object.values(snapshot.timeframes || {})[0]
  const bars = (tf?.klines || []).slice(-80)
  const lines = (tf?.lines || snapshot.lines || []).filter(l => Number.isFinite(l.price)).slice(0, 24)
  if (bars.length < 2) {
    return <div className="rounded-lg border border-zinc-700/50 bg-zinc-800/30 p-8 text-center text-sm text-zinc-600">No chart data</div>
  }
  const highs = bars.map(b => b.high)
  const lows = bars.map(b => b.low)
  const linePrices = lines.map(l => l.price)
  const max = Math.max(...highs, ...linePrices)
  const min = Math.min(...lows, ...linePrices)
  const span = Math.max(max - min, 1e-9)
  const w = 360
  const h = 170
  const x = (i: number) => (i / Math.max(bars.length - 1, 1)) * w
  const y = (p: number) => h - ((p - min) / span) * h
  const closePath = bars.map((b, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(2)} ${y(b.close).toFixed(2)}`).join(' ')
  const ema = (tf?.ema20 || []).slice(-bars.length)
  const emaPath = ema.length === bars.length
    ? ema.map((v, i) => `${i === 0 ? 'M' : 'L'} ${x(i).toFixed(2)} ${y(v).toFixed(2)}`).join(' ')
    : ''
  const fmtPrice = (p: number) => p.toLocaleString(undefined, { maximumFractionDigits: p > 10 ? 3 : 6 })

  return (
    <Section title={`Chart · ${tf?.timeframe || snapshot.primary_timeframe}`}>
      <div className="rounded-lg border border-zinc-700/50 bg-zinc-950/60 p-3">
        <svg viewBox={`0 0 ${w} ${h}`} className="h-44 w-full overflow-visible">
          {lines.map((line) => {
            const yy = y(line.price)
            const color = line.kind === 'support' ? '#34d399' : line.kind === 'resistance' ? '#f87171' : '#fbbf24'
            return (
              <g key={line.id}>
                <line x1="0" x2={w} y1={yy} y2={yy} stroke={color} strokeOpacity="0.45" strokeDasharray={line.kind === 'fibonacci' ? '4 4' : '2 3'} />
                <text x="4" y={Math.max(10, yy - 3)} fill={color} fontSize="9">{line.label} {fmtPrice(line.price)}</text>
              </g>
            )
          })}
          <path d={closePath} fill="none" stroke="#38bdf8" strokeWidth="1.6" />
          {emaPath && <path d={emaPath} fill="none" stroke="#a78bfa" strokeWidth="1" strokeOpacity="0.9" />}
          <line x1="0" x2={w} y1={y(snapshot.price)} y2={y(snapshot.price)} stroke="#e5e7eb" strokeOpacity="0.6" />
        </svg>
        <div className="mt-2 flex flex-wrap gap-2 text-[10px] text-zinc-500">
          <span className="text-sky-300">close</span>
          <span className="text-violet-300">EMA20</span>
          <span className="text-emerald-300">support</span>
          <span className="text-red-300">resistance</span>
          <span className="text-amber-300">fib</span>
        </div>
      </div>
    </Section>
  )
}

function QualityPills({ quality }: { quality?: { liquidity_score?: number; open_interest_score?: number; activity_score?: number; reliability_score?: number; tradability_score?: number; risk_penalty?: number } }) {
  if (!quality) return <span className="text-xs text-zinc-600">—</span>
  const items = [
    ['T', quality.tradability_score],
    ['L', quality.liquidity_score],
    ['OI', quality.open_interest_score],
    ['A', quality.activity_score],
    ['R', quality.reliability_score],
  ] as const
  return (
    <div className="flex justify-end gap-1">
      {items.map(([label, value]) => (
        <span key={label} className="rounded border border-zinc-700 bg-zinc-800/70 px-1.5 py-0.5 text-[10px] text-zinc-300">
          {label}:{Math.round((value || 0) * 100)}
        </span>
      ))}
    </div>
  )
}
