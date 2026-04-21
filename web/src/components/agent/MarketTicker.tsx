import { useState, useEffect } from 'react'
// icons reserved for future use

interface TickerData {
  symbol: string
  lastPrice: string
  priceChangePercent: string
  highPrice: string
  lowPrice: string
  volume: string
}

const SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT']

const SYMBOL_ICONS: Record<string, string> = {
  BTC: '₿',
  ETH: 'Ξ',
  SOL: '◎',
}

export function MarketTicker() {
  const [tickers, setTickers] = useState<Record<string, TickerData>>({})
  const [loading, setLoading] = useState(true)

  const fetchTickers = async () => {
    try {
      // Batch fetch: single API call for all symbols
      const res = await fetch(`/api/agent/tickers?symbols=${SYMBOLS.join(',')}`)
      const data = await res.json()
      const map: Record<string, TickerData> = {}
      if (Array.isArray(data)) {
        data.forEach((r: TickerData) => {
          if (r.lastPrice && r.symbol) map[r.symbol] = r
        })
      }
      setTickers(map)
    } catch {
      // ignore — will retry on next interval
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTickers()
    const interval = setInterval(fetchTickers, 15000)
    return () => clearInterval(interval)
  }, [])

  const formatPrice = (price: string) => {
    const n = parseFloat(price)
    if (n >= 1000) return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    if (n >= 1) return n.toFixed(2)
    return n.toFixed(4)
  }

  const formatVolume = (vol: string) => {
    const n = parseFloat(vol)
    if (n >= 1e9) return (n / 1e9).toFixed(1) + 'B'
    if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
    if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K'
    return n.toFixed(0)
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {SYMBOLS.map((sym) => (
          <div
            key={sym}
            style={{
              padding: '12px',
              background: 'rgba(255,255,255,0.02)',
              borderRadius: 10,
              border: '1px solid rgba(255,255,255,0.04)',
              height: 56,
            }}
          >
            <div style={{
              width: '60%',
              height: 10,
              background: 'rgba(255,255,255,0.04)',
              borderRadius: 4,
              animation: 'pulse 1.5s infinite',
            }} />
          </div>
        ))}
        <style>{`
          @keyframes pulse {
            0%, 100% { opacity: 0.4; }
            50% { opacity: 0.8; }
          }
        `}</style>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {SYMBOLS.map((sym) => {
        const t = tickers[sym]
        if (!t) return null
        const pct = parseFloat(t.priceChangePercent)
        const isUp = pct > 0
        const isDown = pct < 0
        const color = isUp ? '#00e5a0' : isDown ? '#F6465D' : '#6c6c82'
        const bgColor = isUp ? 'rgba(0,229,160,0.06)' : isDown ? 'rgba(246,70,93,0.06)' : 'rgba(108,108,130,0.06)'
        const label = sym.replace('USDT', '')
        const icon = SYMBOL_ICONS[label] || label[0]

        return (
          <div
            key={sym}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '10px 11px',
              background: 'rgba(255,255,255,0.02)',
              borderRadius: 10,
              border: '1px solid rgba(255,255,255,0.04)',
              transition: 'all 0.15s ease',
              cursor: 'default',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = 'rgba(255,255,255,0.04)'
              e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = 'rgba(255,255,255,0.02)'
              e.currentTarget.style.borderColor = 'rgba(255,255,255,0.04)'
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: 8,
                  background: bgColor,
                  display: 'grid',
                  placeItems: 'center',
                  fontSize: 13,
                  fontWeight: 700,
                  color: color,
                  fontFamily: 'system-ui',
                }}
              >
                {icon}
              </div>
              <div>
                <div style={{ fontSize: 12.5, fontWeight: 600, color: '#e0e0ec', letterSpacing: '-0.01em' }}>
                  {label}
                </div>
                <div style={{ fontSize: 10, color: '#4c4c62' }}>
                  Vol {formatVolume(t.volume)}
                </div>
              </div>
            </div>
            <div style={{ textAlign: 'right' }}>
              <div style={{ fontSize: 12.5, fontWeight: 600, color: '#e0e0ec', fontFamily: '"IBM Plex Mono", monospace', letterSpacing: '-0.02em' }}>
                ${formatPrice(t.lastPrice)}
              </div>
              <div style={{
                fontSize: 10.5,
                fontWeight: 600,
                color,
                fontFamily: '"IBM Plex Mono", monospace',
              }}>
                {isUp ? '+' : ''}{pct.toFixed(2)}%
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
