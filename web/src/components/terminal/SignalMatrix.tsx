import { useMemo } from 'react'
import type { SignalRankItem } from '../../lib/api/data'

/**
 * SignalMatrix renders the vergex (claw402) signal ranking as a high-density
 * heatmap grid — one cell per symbol, colored by directional bias (green =
 * bullish, red = bearish, muted = neutral) with intensity scaled by the signal
 * score. Rank 1 is the strongest signal. Cream-themed Bloomberg/terminal feel.
 *
 * Real ranked data only — no synthetic signals.
 */

function baseSymbol(raw: string): string {
  return raw
    .toUpperCase()
    .replace(/^XYZ:/, '')
    .replace(/(USDT|USDC|USD)$/, '')
}

type Bias = 'bullish' | 'bearish' | 'neutral'

function normBias(raw: string): Bias {
  const b = (raw || '').toLowerCase()
  if (b === 'bullish') return 'bullish'
  if (b === 'bearish') return 'bearish'
  return 'neutral'
}

// solid bias color for the left accent + swatches
const ACCENT: Record<Bias, string> = {
  bullish: 'var(--tm-up)',
  bearish: 'var(--tm-dn)',
  neutral: 'var(--tm-muted)',
}

// background wash, intensity-scaled (alpha 0.10 → 0.42)
function washFor(bias: Bias, intensity: number): string {
  const a = 0.1 + Math.max(0, Math.min(1, intensity)) * 0.32
  if (bias === 'bullish') return `rgba(46,139,87,${a.toFixed(3)})`
  if (bias === 'bearish') return `rgba(214,67,58,${a.toFixed(3)})`
  return 'rgba(138,132,120,0.10)'
}

interface Cell {
  rank: number
  symbol: string
  bias: Bias
  score: number
  intensity: number
}

interface SignalMatrixProps {
  items?: SignalRankItem[]
  max?: number
  /** currently-selected base symbol (drives the liq map + order book) */
  active?: string
  /** click a cell to switch the active instrument */
  onSelect?: (symbol: string) => void
}

export function SignalMatrix({ items, max = 36, active, onSelect }: SignalMatrixProps) {
  const view = useMemo(() => {
    const raw = items ?? []
    const sorted = [...raw].sort((a, b) => a.rank - b.rank).slice(0, max)
    if (!sorted.length) return { cells: [] as Cell[], bull: 0, bear: 0, neut: 0 }

    const scores = sorted.map((s) => s.score)
    const min = Math.min(...scores)
    const span = Math.max(...scores) - min
    const n = sorted.length

    let bull = 0
    let bear = 0
    let neut = 0
    const cells: Cell[] = sorted.map((s, i) => {
      const bias = normBias(s.bias)
      if (bias === 'bullish') bull += 1
      else if (bias === 'bearish') bear += 1
      else neut += 1
      // intensity by normalized score; if scores are uniform, fall back to rank
      // (top ranks brighter, descending across the slice).
      const intensity = span > 0 ? (s.score - min) / span : n > 1 ? 1 - i / (n - 1) : 1
      return { rank: s.rank, symbol: s.symbol, bias, score: s.score, intensity }
    })
    return { cells, bull, bear, neut }
  }, [items, max])

  if (!view.cells.length) {
    return (
      <div style={{ fontFamily: 'var(--tm-mono)' }}>
        <Head />
        <div className="tm-sc">No signal data (claw402).</div>
      </div>
    )
  }

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      <Head />

      {/* legend */}
      <div
        className="tm-sc"
        style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 6, fontSize: 9 }}
      >
        <Swatch c="var(--tm-up)" label="Bullish" />
        <Swatch c="var(--tm-dn)" label="Bearish" />
        <Swatch c="var(--tm-muted)" label="Neutral" />
        {onSelect && <span style={{ color: 'var(--tm-red)' }}>click to switch ▸</span>}
        <span style={{ marginLeft: 'auto' }}>{view.cells.length} signals</span>
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(64px, 1fr))',
          gap: 3,
        }}
      >
        {view.cells.map((c) => {
          const base = baseSymbol(c.symbol)
          const isActive = !!active && base === active.toUpperCase()
          return (
          <div
            key={`${c.rank}-${c.symbol}`}
            title={`${base} · #${c.rank} · ${c.bias} · ${c.score} — click to switch`}
            onClick={onSelect ? () => onSelect(base) : undefined}
            style={{
              padding: '4px 5px',
              background: washFor(c.bias, c.intensity),
              border: isActive ? '1px solid var(--tm-red)' : '1px solid var(--tm-hair)',
              borderLeft: `2px solid ${ACCENT[c.bias]}`,
              outline: isActive ? '1px solid var(--tm-red)' : 'none',
              boxShadow: isActive ? 'inset 0 0 0 1px var(--tm-red)' : 'none',
              lineHeight: 1.15,
              overflow: 'hidden',
              cursor: onSelect ? 'pointer' : 'default',
            }}
          >
            <div
              className="tm-mono"
              style={{
                fontSize: 10,
                fontWeight: 700,
                color: 'var(--tm-ink)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {baseSymbol(c.symbol)}
            </div>
            <div
              className="tm-sc"
              style={{ fontSize: 8, letterSpacing: '0.08em', marginTop: 1 }}
            >
              #{c.rank} · {fmtScore(c.score)}
            </div>
          </div>
          )
        })}
      </div>
    </div>
  )
}

function fmtScore(n: number): string {
  if (!Number.isFinite(n)) return '—'
  if (Math.abs(n) >= 100) return n.toFixed(0)
  if (Math.abs(n) >= 10) return n.toFixed(1)
  return n.toFixed(2)
}

function Head() {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
      <span className="tm-px" style={{ fontSize: 11 }}>Signal matrix</span>
      <span className="tm-sc">Signal matrix · vergex</span>
    </div>
  )
}

function Swatch({ c, label }: { c: string; label: string }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
      <span style={{ width: 8, height: 8, background: c, display: 'inline-block' }} />
      {label}
    </span>
  )
}

export default SignalMatrix
