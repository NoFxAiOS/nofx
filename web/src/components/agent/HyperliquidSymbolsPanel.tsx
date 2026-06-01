import { useEffect, useMemo, useState } from 'react'
import { Search, Zap, TrendingDown, TrendingUp } from 'lucide-react'
import { api } from '../../lib/api'
import type { MarketSymbol } from '../../lib/api/data'

interface HyperliquidSymbolsPanelProps {
  language: string
  disabled?: boolean
  onTradeSymbol: (symbol: MarketSymbol) => void
}

function formatVolume(value?: number) {
  const n = Number(value || 0)
  if (n >= 1e9) return `$${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `$${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `$${(n / 1e3).toFixed(0)}K`
  return n > 0 ? `$${n.toFixed(0)}` : '—'
}

function formatPrice(value?: number) {
  const n = Number(value || 0)
  if (!n) return '—'
  if (n >= 1000) return `$${n.toLocaleString('en-US', { maximumFractionDigits: 2 })}`
  if (n >= 1) return `$${n.toFixed(2)}`
  return `$${n.toFixed(5)}`
}

function formatChange(value?: number) {
  const n = Number(value || 0)
  if (!Number.isFinite(n) || n === 0) return '—'
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`
}

const CATEGORY_LABEL: Record<string, { zh: string; en: string }> = {
  stock: { zh: '股票', en: 'Stocks' },
  commodity: { zh: '大宗', en: 'Commodities' },
  index: { zh: '指数', en: 'Indices' },
  forex: { zh: '外汇', en: 'FX' },
  pre_ipo: { zh: 'Pre-IPO', en: 'Pre-IPO' },
  crypto: { zh: '加密', en: 'Crypto' },
}

const CATEGORY_ORDER = ['stock', 'commodity', 'index', 'forex', 'pre_ipo', 'crypto']

export function HyperliquidSymbolsPanel({
  language,
  disabled,
  onTradeSymbol,
}: HyperliquidSymbolsPanelProps) {
  const [symbols, setSymbols] = useState<MarketSymbol[]>([])
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('stock')
  const [ranking, setRanking] = useState<'gainers' | 'losers' | 'volume'>('gainers')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api
      .getSymbols('hyperliquid-xyz')
      .then((res) => {
        if (cancelled) return
        setSymbols(res.symbols || [])
        setError('')
      })
      .catch((err) => {
        if (cancelled) return
        setError(err?.message || 'Failed to load symbols')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const categories = useMemo(() => {
    const unique = new Set(symbols.map((s) => s.category).filter(Boolean))
    return CATEGORY_ORDER.filter((c) => unique.has(c))
  }, [symbols])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return symbols
      .filter((s) => (category === 'all' ? true : s.category === category))
      .filter((s) => {
        if (!q) return true
        return [s.symbol, s.display, s.name, s.category]
          .filter(Boolean)
          .some((v) => String(v).toLowerCase().includes(q))
      })
      .sort((a, b) => {
        if (ranking === 'gainers') return Number(b.change_24h_pct || 0) - Number(a.change_24h_pct || 0)
        if (ranking === 'losers') return Number(a.change_24h_pct || 0) - Number(b.change_24h_pct || 0)
        return Number(b.volume_24h || 0) - Number(a.volume_24h || 0)
      })
      .slice(0, 80)
  }, [category, query, ranking, symbols])

  if (loading) {
    return (
      <div style={{ padding: 12, color: '#848E9C', fontSize: 12 }}>
        {language === 'zh' ? '正在加载 Hyperliquid 全市场标的…' : 'Loading Hyperliquid markets…'}
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 12, color: '#F6465D', fontSize: 12 }}>
        {language === 'zh' ? '标的列表加载失败：' : 'Failed to load symbols: '}{error}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '8px 10px',
          borderRadius: 10,
          border: '1px solid rgba(255,255,255,0.06)',
          background: 'rgba(255,255,255,0.025)',
        }}
      >
        <Search size={13} color="#6c6c82" />
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={language === 'zh' ? '搜索 SAMSUNG / TESLA / GOLD…' : 'Search SAMSUNG / TESLA / GOLD…'}
          style={{
            width: '100%',
            border: 0,
            outline: 'none',
            background: 'transparent',
            color: '#EAECEF',
            fontSize: 12,
          }}
        />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 6 }}>
        {([
          { key: 'gainers', icon: TrendingUp, zh: '涨幅榜', en: 'Gainers' },
          { key: 'losers', icon: TrendingDown, zh: '跌幅榜', en: 'Losers' },
          { key: 'volume', icon: Zap, zh: '成交额', en: 'Volume' },
        ] as const).map((item) => {
          const Icon = item.icon
          const active = ranking === item.key
          return (
            <button
              key={item.key}
              onClick={() => setRanking(item.key)}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 5,
                padding: '7px 6px',
                borderRadius: 10,
                border: active ? '1px solid rgba(240,185,11,0.55)' : '1px solid rgba(255,255,255,0.06)',
                background: active ? 'rgba(240,185,11,0.12)' : 'rgba(255,255,255,0.025)',
                color: active ? '#F0B90B' : '#848E9C',
                fontSize: 11,
                fontWeight: 700,
                cursor: 'pointer',
              }}
            >
              <Icon size={12} />
              {language === 'zh' ? item.zh : item.en}
            </button>
          )
        })}
      </div>

      <div className="hide-scrollbar" style={{ display: 'flex', gap: 6, overflowX: 'auto' }}>
        {categories.map((c) => (
          <button
            key={c}
            onClick={() => setCategory(c)}
            style={{
              padding: '5px 8px',
              borderRadius: 999,
              border: c === category ? '1px solid rgba(240,185,11,0.55)' : '1px solid rgba(255,255,255,0.06)',
              background: c === category ? 'rgba(240,185,11,0.12)' : 'rgba(255,255,255,0.025)',
              color: c === category ? '#F0B90B' : '#848E9C',
              fontSize: 10.5,
              whiteSpace: 'nowrap',
              cursor: 'pointer',
            }}
          >
            {c === 'all'
              ? language === 'zh'
                ? '全部'
                : 'All'
              : CATEGORY_LABEL[c]?.[language === 'zh' ? 'zh' : 'en'] || c}
          </button>
        ))}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 340, overflowY: 'auto', paddingRight: 2 }}>
        {filtered.map((s) => {
          const display = s.display || s.symbol
          return (
            <button
              key={`${s.exchange || 'hyper'}-${s.symbol}`}
              disabled={disabled}
              onClick={() => onTradeSymbol(s)}
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr auto',
                gap: 8,
                textAlign: 'left',
                padding: '10px 11px',
                borderRadius: 10,
                border: '1px solid rgba(255,255,255,0.05)',
                background: 'rgba(255,255,255,0.025)',
                cursor: disabled ? 'not-allowed' : 'pointer',
                opacity: disabled ? 0.6 : 1,
              }}
              title={language === 'zh' ? `点击让 Agent 交易 ${display}` : `Ask agent to trade ${display}`}
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span style={{ color: '#EAECEF', fontWeight: 700, fontSize: 12.5 }}>{display}</span>
                  <span style={{ color: '#4c4c62', fontSize: 10 }}>{s.category}</span>
                  {!!s.maxLeverage && <span style={{ color: '#F0B90B', fontSize: 10 }}>{s.maxLeverage}x</span>}
                </div>
                <div style={{ display: 'flex', gap: 6, alignItems: 'center', color: '#6c6c82', fontSize: 10.5, marginTop: 3 }}>
                  <span style={{ color: '#6c6c82', fontSize: 10.5 }}>
                    Vol {formatVolume(s.volume_24h)} · {formatPrice(s.mark_price)}
                  </span>
                  <span style={{ color: Number(s.change_24h_pct || 0) >= 0 ? '#0ECB81' : '#F6465D', fontSize: 10.5, fontWeight: 700 }}>
                    {formatChange(s.change_24h_pct)}
                  </span>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', color: '#F0B90B', gap: 4, fontSize: 11, fontWeight: 700 }}>
                <Zap size={13} />
                {language === 'zh' ? '交易' : 'Trade'}
              </div>
            </button>
          )
        })}
      </div>

      <div style={{ color: '#4c4c62', fontSize: 10.5, lineHeight: 1.45 }}>
        {language === 'zh'
          ? `已列出 ${symbols.length} 个 Hyperliquid USDC 标的。默认按涨幅榜排序，也可切换跌幅榜/成交额；点击交易会直接创建固定标的 Trader。`
          : `${symbols.length} Hyperliquid USDC markets loaded. Default ranking is 24h gainers; switch to losers/volume. Click Trade to create a fixed-symbol trader directly.`}
      </div>
    </div>
  )
}
