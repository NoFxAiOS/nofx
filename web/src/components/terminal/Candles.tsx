import { useMemo } from 'react'
import type { Kline } from '../../lib/api/data'

interface CandlesProps {
  data: Kline[]
  width?: number
  height?: number
  /** stretch to fill the parent's height (parent must have a definite height) */
  fill?: boolean
}

/**
 * Candles renders a compact OHLC candlestick chart from real kline data
 * (GET /api/klines). Up candles use the terminal's profit green, down candles
 * the loss red. Purely presentational — the parent fetches the real series.
 */
export function Candles({ data, width = 640, height = 150, fill = false }: CandlesProps) {
  const candles = useMemo(() => {
    if (!data || data.length === 0) return []
    const slice = data.slice(-40)
    const highs = slice.map((k) => k.high)
    const lows = slice.map((k) => k.low)
    const max = Math.max(...highs)
    const min = Math.min(...lows)
    const span = max - min || 1
    const pad = 6
    const gap = (width - pad * 2) / slice.length
    const bodyW = Math.max(2, gap * 0.6)
    const y = (v: number) => pad + (1 - (v - min) / span) * (height - pad * 2)
    return slice.map((k, i) => {
      const cx = pad + gap * i + gap / 2
      const up = k.close >= k.open
      return {
        cx,
        up,
        wickTop: y(k.high),
        wickBot: y(k.low),
        bodyTop: y(Math.max(k.open, k.close)),
        bodyBot: y(Math.min(k.open, k.close)),
        bodyW,
      }
    })
  }, [data, width, height])

  if (candles.length === 0) return null

  return (
    <svg
      width="100%"
      height={fill ? '100%' : undefined}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio={fill ? 'none' : 'xMidYMid meet'}
      role="img"
      aria-label="Candlestick chart"
      style={{ display: 'block', ...(fill ? { height: '100%' } : {}) }}
    >
      {candles.map((c, i) => {
        const color = c.up ? 'var(--tm-up)' : 'var(--tm-dn)'
        return (
          <g key={i} stroke={color} fill={color}>
            <line x1={c.cx} y1={c.wickTop} x2={c.cx} y2={c.wickBot} strokeWidth={1} />
            <rect
              x={c.cx - c.bodyW / 2}
              y={c.bodyTop}
              width={c.bodyW}
              height={Math.max(1, c.bodyBot - c.bodyTop)}
            />
          </g>
        )
      })}
    </svg>
  )
}

export default Candles
