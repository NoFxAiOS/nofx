import { useEffect, useMemo, useRef, useState } from 'react'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type { VergexHeatmapBin } from '../../lib/api/data'
import { demoSeedPrice, demoTick } from '../../lib/demo/demoUniverse'

const lmRnd = (a: number, b: number) => a + Math.random() * (b - a)

/**
 * LiquidationMap renders the vergex (claw402) cost / liquidation heatmap as a
 * vertical price ladder — position-cost concentration plus liquidation fuel by
 * price level. Long metrics diverge right, short metrics diverge left from the
 * mark price. Cream-themed adaptation of a Bloomberg-style liquidation map.
 *
 * Real paid data only (hip3_perp synthetic markets). Polled at 5 min to spare
 * the claw402 wallet.
 */

const C_LONG_COST = 'var(--tm-up)' // forest green
const C_SHORT_COST = 'var(--tm-dn)' // crimson
const C_LONG_LIQ = '#c8860b' // amber — long-liquidation fuel (price falls)
const C_SHORT_LIQ = '#2c7a9e' // teal  — short-liquidation fuel (price rises)

function fmtUsd(n: number): string {
  const a = Math.abs(n)
  if (a >= 1e9) return `$${(n / 1e9).toFixed(2)}B`
  if (a >= 1e6) return `$${(n / 1e6).toFixed(2)}M`
  if (a >= 1e3) return `$${(n / 1e3).toFixed(1)}K`
  return `$${n.toFixed(0)}`
}
function fmtPx(n: number): string {
  if (n >= 1000) return n.toLocaleString('en-US', { maximumFractionDigits: 0 })
  if (n >= 1) return n.toLocaleString('en-US', { maximumFractionDigits: 2 })
  return n.toLocaleString('en-US', { maximumFractionDigits: 4 })
}

interface Row extends VergexHeatmapBin {
  px: number
  longCost: number
  shortCost: number
  longLiq: number
  shortLiq: number
}

interface LiquidationMapProps {
  symbol: string
  marketType?: string
  /** fixed height of the scrollable ladder (px); auto-centres on the mark */
  height?: number
  /** showcase mode — render a synthetic ladder centred on the demo seed price */
  demo?: boolean
}

export function LiquidationMap({ symbol, marketType = 'hip3_perp', height = 460, demo = false }: LiquidationMapProps) {
  // Synthetic markets live under marketType "hip3_perp"; crypto majors under
  // "perp". We try the caller's guess first and fall back to the other so the
  // heatmap resolves for ANY symbol that has one.
  const fetcher = (mt: string) =>
    api.getVergexCostLiquidationHeatmap({ marketType: mt, symbol, chain: 'mainnet', liqBand: '15' })
  const opts = { refreshInterval: 300000, revalidateOnFocus: false, keepPreviousData: true }

  const primary = useSWR(symbol && !demo ? ['heatmap', marketType, symbol] : null, () => fetcher(marketType), opts)
  const primaryHasBins = !!primary.data?.data?.bins?.length
  const altMt = marketType === 'perp' ? 'hip3_perp' : 'perp'
  const needAlt = !demo && !primaryHasBins && !primary.isLoading && primary.data !== undefined
  const alt = useSWR(needAlt && symbol ? ['heatmap', altMt, symbol] : null, () => fetcher(altMt), opts)

  // showcase mode: drive a slow ticker so the synthetic ladder gently breathes
  const [demoFrame, setDemoFrame] = useState(0)
  useEffect(() => {
    if (!demo) return
    const id = setInterval(() => setDemoFrame((f) => f + 1), 420)
    return () => clearInterval(id)
  }, [demo])

  // stable base ladder for the active symbol (regenerated only on symbol change),
  // centred on the same seed price the order book / candles use so all three
  // price panels stay consistent.
  const demoBase = useMemo(() => {
    if (!demo) return null
    const base = (symbol || 'SP500').toUpperCase().replace(/^XYZ:/, '')
    const mark = demoSeedPrice(base)
    const tick = demoTick(mark)
    const N = 44
    const scale = mark * 1.4e4
    const bins = [] as { px: number; lc: number; sc: number; ll: number; sl: number }[]
    for (let i = -N / 2; i <= N / 2; i++) {
      const px = +(mark + i * tick * 2).toFixed(tick < 1 ? 3 : 1)
      const dist = Math.abs(i) / (N / 2)
      const near = Math.max(0, 1 - dist) ** 1.4
      const far = dist ** 1.3
      const below = i < 0
      bins.push({
        px,
        lc: (below ? near : near * 0.18) * scale * lmRnd(0.5, 1),
        sc: (!below ? near : near * 0.18) * scale * lmRnd(0.5, 1),
        ll: (below ? far : 0) * scale * lmRnd(0.4, 0.9),
        sl: (!below ? far : 0) * scale * lmRnd(0.4, 0.9),
      })
    }
    return { base, mark, bins, costAddrs: Math.round(lmRnd(22000, 31000)), liqAddrs: Math.round(lmRnd(16000, 23000)) }
  }, [demo, symbol])

  // per-frame view: each bin breathes on its own phase (gentle ±, not a refresh)
  const demoData = useMemo(() => {
    if (!demoBase) return undefined
    const f = demoFrame
    const w = (v: number, amp: number, ph: number) => v * (1 + amp * Math.sin(f * 0.5 + ph))
    const bins: VergexHeatmapBin[] = demoBase.bins.map((b, i) => ({
      px: b.px,
      longCost: w(b.lc, 0.12, i * 0.6),
      shortCost: w(b.sc, 0.12, i * 0.9 + 1.7),
      longLiq: w(b.ll, 0.16, i * 0.5 + 3.1),
      shortLiq: w(b.sl, 0.16, i * 0.8 + 4.6),
    }) as unknown as VergexHeatmapBin)
    return {
      data: { bins, markPrice: demoBase.mark, costAddrs: demoBase.costAddrs, liqAddrs: demoBase.liqAddrs, market: { symbol: demoBase.base } },
    }
  }, [demoBase, demoFrame])

  const data = demo ? demoData : primaryHasBins ? primary.data : alt.data
  const isLoading = demo ? false : primary.isLoading || (needAlt && alt.isLoading)
  const error = demo ? undefined : primaryHasBins ? undefined : alt.error || primary.error

  const [hover, setHover] = useState<number | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const markRef = useRef<HTMLDivElement>(null)

  const view = useMemo(() => {
    const d = data?.data
    // keepPreviousData can leave a stale heatmap on screen when the selected
    // symbol has no hip3 market (e.g. a crypto major) — detect the mismatch and
    // treat it as no-data so the panel honestly reflects the requested symbol.
    const requested = (symbol || '').toUpperCase().replace(/^XYZ:/, '')
    const loaded = (d?.market?.symbol || '').toUpperCase().replace(/^XYZ:/, '')
    const stale = !!loaded && loaded !== requested
    const raw = stale ? [] : d?.bins ?? []
    const rows: Row[] = raw
      .map((b) => ({
        px: b.px ?? ((b.bucketStartPrice ?? 0) + (b.bucketEndPrice ?? 0)) / 2,
        longCost: b.longCost ?? 0,
        shortCost: b.shortCost ?? 0,
        longLiq: b.longLiq ?? 0,
        shortLiq: b.shortLiq ?? 0,
        ...b,
      }))
      .filter((r) => r.px > 0 && (r.longCost || r.shortCost || r.longLiq || r.shortLiq))
      .sort((a, b) => b.px - a.px)
    const maxSide = rows.reduce(
      (m, r) => Math.max(m, r.longCost + r.longLiq, r.shortCost + r.shortLiq),
      1,
    )
    const totals = rows.reduce(
      (t, r) => ({
        lc: t.lc + r.longCost,
        sc: t.sc + r.shortCost,
        ll: t.ll + r.longLiq,
        sl: t.sl + r.shortLiq,
      }),
      { lc: 0, sc: 0, ll: 0, sl: 0 },
    )
    return { rows, maxSide, mark: stale ? 0 : d?.markPrice ?? 0, costAddrs: stale ? 0 : d?.costAddrs ?? 0, liqAddrs: stale ? 0 : d?.liqAddrs ?? 0, totals, dispSymbol: stale ? symbol : d?.market?.symbol || symbol }
  }, [data, symbol])

  const markRowIdx = useMemo(() => {
    if (!view.mark || !view.rows.length) return -1
    let best = 0
    let bd = Infinity
    view.rows.forEach((r, i) => {
      const dd = Math.abs(r.px - view.mark)
      if (dd < bd) {
        bd = dd
        best = i
      }
    })
    return best
  }, [view])

  // centre the scroll ladder on the mark (current) price once data arrives.
  // Uses bounding-rect math (not offsetTop, which depends on the offsetParent)
  // and re-applies on the next frame so it lands after layout settles.
  useEffect(() => {
    const sc = scrollRef.current
    const mk = markRef.current
    if (!sc || !mk) return
    const apply = () => {
      const rel = mk.getBoundingClientRect().top - sc.getBoundingClientRect().top + sc.scrollTop
      sc.scrollTop = rel - sc.clientHeight / 2 + mk.offsetHeight / 2
    }
    apply()
    const id = requestAnimationFrame(apply)
    return () => cancelAnimationFrame(id)
  }, [markRowIdx, view.rows.length, height, view.dispSymbol])

  const rowH = view.rows.length > 44 ? 8 : view.rows.length > 28 ? 11 : 15
  const hv = hover != null ? view.rows[hover] : null

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 3 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>Cost / Liq map</span>
        <span className="tm-sc">{view.dispSymbol}</span>
        <span className="tm-sc" style={{ marginLeft: 'auto', color: view.rows.length ? 'var(--tm-up)' : 'var(--tm-muted)' }}>
          {view.rows.length ? '● live' : isLoading ? '○ sync' : '○ —'}
        </span>
      </div>

      {/* legend */}
      <div className="tm-sc" style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 4, fontSize: 9 }}>
        <Swatch c={C_LONG_COST} label="Long cost" />
        <Swatch c={C_SHORT_COST} label="Short cost" />
        <Swatch c={C_LONG_LIQ} label="Long liq" />
        <Swatch c={C_SHORT_LIQ} label="Short liq" />
      </div>

      {/* hover readout / mark line */}
      <div className="tm-mono" style={{ fontSize: 10, color: 'var(--tm-ink-2)', minHeight: 14, marginBottom: 2 }}>
        {hv ? (
          <span>
            <b>{fmtPx(hv.px)}</b> · Cost line <span style={{ color: C_LONG_COST }}>{fmtUsd(hv.longCost)}</span>/<span style={{ color: C_SHORT_COST }}>{fmtUsd(hv.shortCost)}</span>
            {' · '}liq <span style={{ color: C_LONG_LIQ }}>{fmtUsd(hv.longLiq)}</span>/<span style={{ color: C_SHORT_LIQ }}>{fmtUsd(hv.shortLiq)}</span>
          </span>
        ) : (
          <span className="tm-sc">mark <b style={{ color: 'var(--tm-red)' }}>{view.mark ? fmtPx(view.mark) : '—'}</b> · {view.costAddrs.toLocaleString()} positions / {view.liqAddrs.toLocaleString()} liq levels</span>
        )}
      </div>

      {error && !view.rows.length ? (
        <div className="tm-sc" style={{ padding: '16px 0' }}>No cost/liq heatmap for {view.dispSymbol} (crypto / main-dex markets have none).</div>
      ) : !view.rows.length ? (
        <div className="tm-sc" style={{ padding: '16px 0' }}>Loading cost/liquidation map…</div>
      ) : (
        <div>
          <div ref={scrollRef} style={{ maxHeight: height, overflowY: 'auto' }}>
          {view.rows.map((r, i) => {
            const isMark = i === markRowIdx
            const lcW = (r.longCost / view.maxSide) * 100
            const llW = (r.longLiq / view.maxSide) * 100
            const scW = (r.shortCost / view.maxSide) * 100
            const slW = (r.shortLiq / view.maxSide) * 100
            const showLabel = i % 4 === 0 || isMark
            return (
              <div
                key={i}
                ref={isMark ? markRef : undefined}
                onMouseEnter={() => setHover(i)}
                onMouseLeave={() => setHover(null)}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '52px 1fr 1fr',
                  alignItems: 'center',
                  height: rowH,
                  background: hover === i ? 'rgba(26,24,19,0.05)' : 'transparent',
                  borderTop: isMark ? '1px solid var(--tm-red)' : 'none',
                }}
              >
                <span style={{ fontSize: 9, textAlign: 'right', paddingRight: 6, color: isMark ? 'var(--tm-red)' : 'var(--tm-muted)', fontWeight: isMark ? 700 : 400 }}>
                  {showLabel ? fmtPx(r.px) : ''}
                </span>
                {/* short side — bars anchored at center, extend left (cost nearest center) */}
                <div style={{ position: 'relative', height: '100%', display: 'flex', justifyContent: 'flex-end' }}>
                  <div style={{ width: `${slW}%`, background: C_SHORT_LIQ, opacity: 0.85 }} />
                  <div style={{ width: `${scW}%`, background: C_SHORT_COST }} />
                </div>
                {/* long side — bars from center, extend right (cost nearest center) */}
                <div style={{ position: 'relative', height: '100%', display: 'flex', justifyContent: 'flex-start' }}>
                  <div style={{ width: `${lcW}%`, background: C_LONG_COST }} />
                  <div style={{ width: `${llW}%`, background: C_LONG_LIQ, opacity: 0.85 }} />
                </div>
              </div>
            )
          })}
          </div>
          {/* totals footer */}
          <div className="tm-sc" style={{ display: 'flex', gap: 10, marginTop: 4, fontSize: 9, flexWrap: 'wrap' }}>
            <span>Cost line <span style={{ color: C_LONG_COST }}>{fmtUsd(view.totals.lc)}</span>/<span style={{ color: C_SHORT_COST }}>{fmtUsd(view.totals.sc)}</span></span>
            <span>liq <span style={{ color: C_LONG_LIQ }}>{fmtUsd(view.totals.ll)}</span>/<span style={{ color: C_SHORT_LIQ }}>{fmtUsd(view.totals.sl)}</span></span>
          </div>
        </div>
      )}
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

export default LiquidationMap
