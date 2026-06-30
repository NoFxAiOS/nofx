import { useMemo } from 'react'

/**
 * OrchestrationTopology renders the decision funnel as a chain of tightly-packed
 * dot matrices — one wide grid per layer (flow → signal → decision → execute →
 * hold), columns sitting close together. Every grid is always full: real symbols
 * are SOLID (green long / red short) and SCATTERED across the grid; empty cells
 * are HOLLOW placeholders. Matching symbols connect forward; the engine fans
 * beams to the candidates that progress.
 */

export interface FunnelItem {
  symbol: string
  dir: 'long' | 'short'
}
export interface FunnelLayer {
  key: string
  title: string
  zh: string
  items: FunnelItem[]
}
interface OrchestrationTopologyProps {
  layers: FunnelLayer[]
  className?: string
}

const VB_W = 800
const ENGINE_X = 40
const HEADER_Y = 30
const COLS = 8
const LONG_ROWS = 5
const SHORT_ROWS = 5
const CAP_LONG = COLS * LONG_ROWS // 40
const CAP_SHORT = COLS * SHORT_ROWS // 40
const DX = 11
const DY = 11
const ZONE_GAP = 8
const LAYER_START = 100
const LAYER_STEP = 128

const LONG = 'var(--tm-up)'
const SHORT = 'var(--tm-dn)'

function baseSymbol(raw: string): string {
  return raw.toUpperCase().replace(/^XYZ:/, '').replace(/[-_]/g, '').replace(/(USDT|USDC|USD)$/, '')
}

// evenly spread `count` solid items across `capacity` grid cells
function scatter(count: number, capacity: number): Map<number, number> {
  const m = new Map<number, number>()
  if (count <= 0) return m
  for (let i = 0; i < count; i++) {
    const cell = Math.min(capacity - 1, Math.floor(((i + 0.5) * capacity) / count))
    m.set(cell, i)
  }
  return m
}

interface Cell {
  base?: string
  x: number
  y: number
  dir: 'long' | 'short'
  solid: boolean
}

function splitDedup(items: FunnelItem[]) {
  const seen = new Set<string>()
  const longs: string[] = []
  const shorts: string[] = []
  for (const it of items) {
    const b = baseSymbol(it.symbol)
    if (!b || seen.has(b)) continue
    seen.add(b)
    ;(it.dir === 'short' ? shorts : longs).push(b)
  }
  return { longs: longs.slice(0, CAP_LONG), shorts: shorts.slice(0, CAP_SHORT) }
}

export function OrchestrationTopology({ layers, className }: OrchestrationTopologyProps) {
  const { prepared, cellsByLayer, realByLayer, height, cy, colX } = useMemo(() => {
    const prep = layers.map((l) => ({ ...l, ...splitDedup(l.items) }))
    const xs = prep.map((_, i) => LAYER_START + i * LAYER_STEP)

    const gridH = (LONG_ROWS + SHORT_ROWS) * DY + ZONE_GAP
    const h = HEADER_Y + gridH + 14
    const centerY = HEADER_Y + gridH / 2
    const longTop = centerY - gridH / 2
    const shortTop = longTop + LONG_ROWS * DY + ZONE_GAP

    const cells: Cell[][] = []
    const realMaps: Map<string, { x: number; y: number; dir: 'long' | 'short' }>[] = []

    prep.forEach((d, li) => {
      const layerCells: Cell[] = []
      const real = new Map<string, { x: number; y: number; dir: 'long' | 'short' }>()
      const longScatter = scatter(d.longs.length, CAP_LONG)
      const shortScatter = scatter(d.shorts.length, CAP_SHORT)

      for (let idx = 0; idx < CAP_LONG; idx++) {
        const c = idx % COLS
        const r = Math.floor(idx / COLS)
        const x = xs[li] + c * DX
        const y = longTop + r * DY
        const itemIdx = longScatter.get(idx)
        const base = itemIdx !== undefined ? d.longs[itemIdx] : undefined
        layerCells.push({ base, x, y, dir: 'long', solid: !!base })
        if (base) real.set(base, { x, y, dir: 'long' })
      }
      for (let idx = 0; idx < CAP_SHORT; idx++) {
        const c = idx % COLS
        const r = Math.floor(idx / COLS)
        const x = xs[li] + c * DX
        const y = shortTop + r * DY
        const itemIdx = shortScatter.get(idx)
        const base = itemIdx !== undefined ? d.shorts[itemIdx] : undefined
        layerCells.push({ base, x, y, dir: 'short', solid: !!base })
        if (base) real.set(base, { x, y, dir: 'short' })
      }
      cells.push(layerCells)
      realMaps.push(real)
    })

    return { prepared: prep, cellsByLayer: cells, realByLayer: realMaps, height: h, cy: centerY, colX: xs }
  }, [layers])

  const edges = useMemo(() => {
    const out: { x1: number; y1: number; x2: number; y2: number; dir: string; key: string }[] = []
    for (let l = 0; l < realByLayer.length - 1; l++) {
      const right = realByLayer[l + 1]
      if (right.size === 0) continue
      realByLayer[l].forEach((a, base) => {
        const b = right.get(base)
        if (b) out.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, dir: a.dir, key: `${l}-${base}` })
      })
    }
    return out
  }, [realByLayer])

  // engine fans to real nodes in the first non-empty layer — balanced across
  // long (top) and short (bottom) so both halves get dispatch lines/beams
  const engineTargets = useMemo(() => {
    const idx = realByLayer.findIndex((m) => m.size > 0)
    if (idx < 0) return [] as { x: number; y: number; dir: 'long' | 'short' }[]
    const all = [...realByLayer[idx].values()]
    const longs = all.filter((n) => n.dir === 'long').slice(0, 24)
    const shorts = all.filter((n) => n.dir === 'short').slice(0, 24)
    return [...longs, ...shorts]
  }, [realByLayer])

  return (
    <svg width="100%" viewBox={`0 0 ${VB_W} ${height}`} role="img"
      aria-label="Decision funnel matrix: flow, signal, decision, execute, hold"
      className={className} style={{ display: 'block' }}>

      {prepared.map((d, li) => (
        <g key={d.key} fontFamily="var(--tm-mono)">
          <text x={colX[li] - 2} y={12} fontSize={8} fill="var(--tm-muted)" style={{ letterSpacing: '0.08em' }}>{d.title}</text>
          <text x={colX[li] - 2} y={23} fontSize={8}>
            <tspan fill="var(--tm-up)">L{d.longs.length}</tspan>
            <tspan fill="var(--tm-muted)"> </tspan>
            <tspan fill="var(--tm-dn)">S{d.shorts.length}</tspan>
          </text>
        </g>
      ))}

      <g strokeWidth={0.6} strokeDasharray="2 3" opacity={0.4}>
        {engineTargets.map((n, i) => (
          <line key={i} x1={ENGINE_X} y1={cy} x2={n.x} y2={n.y} stroke={n.dir === 'short' ? SHORT : LONG} />
        ))}
      </g>
      <g strokeWidth={0.8} strokeDasharray="2 3" opacity={0.5}>
        {edges.map((e) => (
          <line key={e.key} x1={e.x1} y1={e.y1} x2={e.x2} y2={e.y2} stroke={e.dir === 'short' ? SHORT : LONG} />
        ))}
      </g>

      {engineTargets.map((n, i) => (
        <circle key={`b0-${i}`} r={1.8} fill={n.dir === 'short' ? SHORT : LONG}>
          <animate attributeName="cx" values={`${ENGINE_X};${n.x}`} dur={`${0.5 + (i % 5) * 0.08}s`} begin={`${(i % 8) * 0.07}s`} repeatCount="indefinite" />
          <animate attributeName="cy" values={`${cy};${n.y}`} dur={`${0.5 + (i % 5) * 0.08}s`} begin={`${(i % 8) * 0.07}s`} repeatCount="indefinite" />
          <animate attributeName="opacity" values="0.9;0.9;0" dur={`${0.5 + (i % 5) * 0.08}s`} begin={`${(i % 8) * 0.07}s`} repeatCount="indefinite" />
        </circle>
      ))}
      {edges.map((e, i) => (
        <circle key={`be-${e.key}`} r={2.1} fill={e.dir === 'short' ? SHORT : LONG}>
          <animate attributeName="cx" values={`${e.x1};${e.x2}`} dur={`${0.45 + (i % 4) * 0.08}s`} begin={`${(i % 6) * 0.06}s`} repeatCount="indefinite" />
          <animate attributeName="cy" values={`${e.y1};${e.y2}`} dur={`${0.45 + (i % 4) * 0.08}s`} begin={`${(i % 6) * 0.06}s`} repeatCount="indefinite" />
          <animate attributeName="opacity" values="1;1;0" dur={`${0.45 + (i % 4) * 0.08}s`} begin={`${(i % 6) * 0.06}s`} repeatCount="indefinite" />
        </circle>
      ))}

      {cellsByLayer.map((layer, li) =>
        layer.map((cell, ci) => {
          const color = cell.dir === 'short' ? SHORT : LONG
          if (cell.solid) {
            return <circle key={`${li}-${ci}`} cx={cell.x} cy={cell.y} r={2.7} fill={color} stroke={color} strokeWidth={0.5} />
          }
          return <circle key={`${li}-${ci}`} cx={cell.x} cy={cell.y} r={1.8} fill="none" stroke={color} strokeWidth={0.6} opacity={0.2} />
        })
      )}

      <circle cx={ENGINE_X} cy={cy} r={18} fill="none" stroke="var(--tm-red)" strokeWidth={1.5}>
        <animate attributeName="r" values="18;34" dur="2.2s" repeatCount="indefinite" />
        <animate attributeName="opacity" values="0.6;0" dur="2.2s" repeatCount="indefinite" />
      </circle>
      <circle cx={ENGINE_X} cy={cy} r={18} fill="var(--tm-red)" />
      <text x={ENGINE_X} y={cy + 3} textAnchor="middle" fontFamily="var(--tm-px)" fontSize={7} fill="var(--tm-paper)">NOFX</text>
    </svg>
  )
}

export default OrchestrationTopology
