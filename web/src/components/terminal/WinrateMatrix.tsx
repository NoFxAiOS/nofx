import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type { VergexWinrateCell, VergexWinrateHolder, VergexWinrateMapResponse } from '../../lib/api/data'
import { demoSeedPrice } from '../../lib/demo/demoUniverse'

/**
 * WinrateMatrix renders the vergex (claw402) holder win-rate matrix — live
 * positions bucketed by entry-cost slot × historical win-rate bin, long and
 * short side by side. Rows are absolute entry-cost price bands (near-price
 * banding: 1% steps around the mark, wider further out); columns are win-rate
 * deciles. A dashed line marks the current price (cost = 100%). Clicking a
 * cell drills into the addresses behind it (paid claw402 call, fetched
 * strictly on demand) with pagination.
 *
 * Real paid data only. Polled at 5 min to spare the claw402 wallet.
 */

// Display grouping — mirrors the upstream panel exactly:
// win bins 20 × 5% merged pairwise into 10 columns of 10%,
// cost slots 0..costBins+1 (0 = below viewport, 17 = above) merged into
// 10 rows with 1,1,2,4 doubling away from the price slot (bin index 9).
const COST_GROUPS: number[][] = [
  [17],
  [13, 14, 15, 16],
  [11, 12],
  [10],
  [9],
  [8],
  [7],
  [5, 6],
  [1, 2, 3, 4],
  [0],
]
const WIN_COLS: [number, number][] = [
  [0, 1], [2, 3], [4, 5], [6, 7], [8, 9],
  [10, 11], [12, 13], [14, 15], [16, 17], [18, 19],
]
const DRILL_LIMIT = 20

function fmtUsd(n: number): string {
  const a = Math.abs(n)
  const f = (v: number) => {
    const s = v.toFixed(1)
    return s.endsWith('.0') ? s.slice(0, -2) : s
  }
  if (a >= 1e9) return `$${f(n / 1e9)}B`
  if (a >= 1e6) return `$${f(n / 1e6)}M`
  if (a >= 1e3) return `$${f(n / 1e3)}K`
  return `$${f(n)}`
}
function fmtPx(n: number): string {
  return n.toFixed(3).replace(/\.?0+$/, '')
}

interface MatrixCell {
  notional: number
  count: number
}

interface SideView {
  rows: { label: string; boundary: boolean; slots: number[]; cells: MatrixCell[] }[]
  totals: MatrixCell[]
  max: number
}

interface DrillSel {
  side: 'long' | 'short'
  winIdx: number
  slots: number[]
  label: string
}

// deterministic synthetic matrix for showcase mode (no paid calls)
function demoResponse(symbol: string): VergexWinrateMapResponse {
  const base = (symbol || 'SP500').toUpperCase().replace(/^XYZ:/, '')
  const mark = demoSeedPrice(base)
  let seed = [...base].reduce((a, c) => a + c.charCodeAt(0), 0)
  const rnd = () => {
    seed = (seed * 9301 + 49297) % 233280
    return seed / 233280
  }
  const cells: VergexWinrateCell[] = []
  for (let r = 0; r < 20; r++) {
    for (let c = 0; c < 18; c++) {
      // mass concentrates near the current price (slot 9/10) and mid-high win rates
      const costNear = Math.exp(-((c - 9.5) ** 2) / 18)
      const winBias = 0.4 + 1.1 * (r / 19) ** 1.6
      const longN = costNear * winBias * 4.2e6 * rnd()
      const shortN = costNear * (1.25 - winBias * 0.5) * 3.6e6 * rnd()
      const mk = (n: number) => ({
        count: n > 1e4 ? 1 + Math.floor(rnd() * 220) : 0,
        notional: n > 1e4 ? n.toFixed(2) : '0',
      })
      cells.push({ row: r, column: c, long: mk(longN), short: mk(shortN) })
    }
  }
  return {
    data: {
      snapshotId: 'DEMOSNAPSHOT0000000000000000A'.slice(0, 26) + '2',
      markPrice: String(mark),
      viewport: { winMin: 0, winMax: 100, costMin: 92, costMax: 108 },
      winBins: 20,
      costBins: 16,
      cells,
      included: { count: 14208, notional: '812345678' },
    },
  }
}

export function WinrateMatrix({
  symbol,
  marketType = 'hip3_perp',
  demo = false,
}: {
  symbol: string
  marketType?: string
  demo?: boolean
}) {
  const fetcher = () =>
    api.getVergexHolderWinrateMap({
      marketType,
      symbol,
      chain: 'mainnet',
      winMin: 0,
      winMax: 100,
      costMin: 92,
      costMax: 108,
    })
  const live = useSWR(symbol && !demo ? ['winrate-map', marketType, symbol] : null, fetcher, {
    refreshInterval: 300000,
    revalidateOnFocus: false,
    keepPreviousData: true,
  })

  const data = demo ? demoResponse(symbol) : live.data
  const isLoading = demo ? false : live.isLoading
  const error = demo ? undefined : live.error

  const [sel, setSel] = useState<DrillSel | null>(null)
  const [drillOffset, setDrillOffset] = useState(0)
  const [drill, setDrill] = useState<{
    items: VergexWinrateHolder[]
    total: number
    nextOffset: number | null
    loading: boolean
    error?: string
  }>({ items: [], total: 0, nextOffset: null, loading: false })

  const d = data?.data
  const mark = d?.markPrice ? +d.markPrice : 0

  const view = useMemo(() => {
    if (!d || !d.cells?.length || !d.viewport) return null
    const cellOf = new Map<string, VergexWinrateCell>()
    for (const c of d.cells!) cellOf.set(`${c.row}:${c.column}`, c)
    const costBins = d.costBins ?? 16
    const lo = d.viewport.costMin
    const hi = d.viewport.costMax
    const w = (hi - lo) / costBins
    const slotLower = (t: number) =>
      t === 0 ? 0 : t === costBins + 1 ? hi : lo + (t - 1) * w

    const buildSide = (side: 'long' | 'short'): SideView => {
      const rows: SideView['rows'] = []
      let max = 0
      const totals: MatrixCell[] = WIN_COLS.map(() => ({ notional: 0, count: 0 }))
      for (const slots of COST_GROUPS) {
        const first = slots[0]
        const last = slots[slots.length - 1]
        const lower = slotLower(first)
        const upper = first === 0 ? lo : last >= costBins ? hi : slotLower(last + 1)
        let label: string
        if (first === 0) label = `< ${fmtPx((lo * mark) / 100)}`
        else if (last === costBins + 1) label = `≥ ${fmtPx((lower * mark) / 100)}`
        else label = `${fmtPx((lower * mark) / 100)} – ${fmtPx((upper * mark) / 100)}`
        const boundary = Math.abs(lower - 100) < 1e-8
        const cells: MatrixCell[] = WIN_COLS.map(([r0, r1], i) => {
          let notional = 0
          let count = 0
          for (let r = r0; r <= r1; r++) {
            for (const s of slots) {
              const c = cellOf.get(`${r}:${s}`)
              if (!c) continue
              notional += +(c[side]?.notional ?? 0)
              count += c[side]?.count ?? 0
            }
          }
          if (notional > max) max = notional
          totals[i].notional += notional
          totals[i].count += count
          return { notional, count }
        })
        rows.push({ label, boundary, slots, cells })
      }
      return { rows, totals, max }
    }
    return {
      long: buildSide('long'),
      short: buildSide('short'),
      snapshotId: d.snapshotId ?? '',
    }
  }, [d, mark])

  // drilldown fetch — strictly on demand (paid per call); showcase mode gets
  // a deterministic synthetic page so the interaction is demonstrable.
  useEffect(() => {
    if (!sel || !view?.snapshotId) return
    if (demo) {
      const n = 14
      const seedBase = Math.abs(Math.floor(demoSeedPrice(symbol) * 1e6)) || 12345
      setDrill({
        items: Array.from({ length: n }, (_, i) => ({
          address: `0x${(seedBase + i * 7919).toString(16).padStart(8, '0')}${'a1b2c3d4e5f6789012345678abcdef01'.slice(0, 32)}`,
          side: sel.side,
          size: String(1200 / (i + 1)),
          entryPrice: String(mark * (0.93 + 0.005 * i)),
          notional: String(5.2e6 / (i + 1)),
          costRatio: String(93 + i * 0.5),
          winRate: String(10 * sel.winIdx + (i % 10)),
          roundTrips: 1 + (i % 7),
        })),
        total: 137,
        nextOffset: DRILL_LIMIT,
        loading: false,
      })
      return
    }
    let alive = true
    setDrill((s) => ({ ...s, loading: true, error: undefined }))
    api
      .getVergexHolderWinrateHolders(
        {
          marketType,
          symbol,
          chain: 'mainnet',
          winMin: 0,
          winMax: 100,
          costMin: 92,
          costMax: 108,
          snapshotId: view.snapshotId,
          row: WIN_COLS[sel.winIdx][0],
          rowEnd: WIN_COLS[sel.winIdx][1],
          column: sel.slots[0],
          columnEnd: sel.slots[sel.slots.length - 1],
          side: sel.side,
          offset: drillOffset,
          limit: DRILL_LIMIT,
        },
        true
      )
      .then((res) => {
        if (!alive) return
        setDrill({
          items: res.data?.items ?? [],
          total: res.data?.total ?? 0,
          nextOffset: res.data?.nextOffset ?? null,
          loading: false,
        })
      })
      .catch((err) => {
        if (!alive) return
        setDrill((s) => ({ ...s, loading: false, error: String(err?.message || err) }))
      })
    return () => {
      alive = false
    }
  }, [sel, drillOffset, view?.snapshotId, marketType, symbol, demo, mark])

  const dispSymbol = (symbol || '').toUpperCase().replace(/^XYZ:/, '')
  const hasView = !!view
  const included = d?.included

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 3 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>Win-rate matrix</span>
        <span className="tm-sc">{dispSymbol}</span>
        {mark > 0 && (
          <span className="tm-sc" style={{ color: 'var(--tm-red)' }}>
            mark <b>{fmtPx(mark)}</b>
          </span>
        )}
        {included && (
          <span className="tm-sc">
            {included.count?.toLocaleString()} addrs / {fmtUsd(+included.notional)}
          </span>
        )}
        <span
          className="tm-sc"
          style={{ marginLeft: 'auto', color: hasView ? 'var(--tm-up)' : 'var(--tm-muted)' }}
        >
          {demo && hasView ? '● demo' : hasView ? '● live' : isLoading ? '○ sync' : '○ —'}
        </span>
      </div>
      <div className="tm-sc" style={{ fontSize: 9, marginBottom: 4 }}>
        Entry-cost rows × win-rate columns · dashed line = current price (cost 100%) ·
        click a cell for the addresses behind it
      </div>

      {error && !hasView ? (
        <div className="tm-sc" style={{ padding: '14px 0' }}>
          No win-rate matrix for {dispSymbol} ({String((error as Error)?.message || error)}).
        </div>
      ) : !hasView ? (
        <div className="tm-sc" style={{ padding: '14px 0' }}>Loading win-rate matrix…</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: 18 }}>
          {(['long', 'short'] as const).map((side) => {
            const sv = view![side]
            return (
              <div key={side} style={{ minWidth: 0 }}>
                <div
                  className="tm-px"
                  style={{
                    fontSize: 10,
                    marginBottom: 3,
                    color: side === 'long' ? 'var(--tm-up)' : 'var(--tm-dn)',
                  }}
                >
                  {side === 'long' ? 'Long' : 'Short'}
                </div>
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '78px repeat(10, minmax(0,1fr))',
                    gap: 1,
                    fontSize: 9,
                  }}
                >
                  <div />
                  {WIN_COLS.map((_, i) => (
                    <div key={i} className="tm-sc" style={{ textAlign: 'center', paddingBottom: 2 }}>
                      {i * 10}-{(i + 1) * 10}%
                    </div>
                  ))}
                  {sv.rows.map((row) => (
                    <MatrixRow
                      key={row.label}
                      row={row}
                      side={side}
                      max={sv.max}
                      sel={sel}
                      onPick={(winIdx, cell) => {
                        if (cell.notional <= 0) return
                        setDrillOffset(0)
                        setSel({
                          side,
                          winIdx,
                          slots: row.slots,
                          label: `${side === 'long' ? 'Long' : 'Short'} · win ${winIdx * 10}-${(winIdx + 1) * 10}% × cost ${row.label}`,
                        })
                      }}
                    />
                  ))}
                  <div className="tm-sc" style={{ textAlign: 'right', paddingRight: 6, alignSelf: 'center' }}>
                    Σ
                  </div>
                  {sv.totals.map((t, i) => (
                    <div key={i} className="tm-sc" style={{ textAlign: 'center', lineHeight: 1.3 }}>
                      <div>{fmtUsd(t.notional)}</div>
                      <div style={{ fontSize: 8 }}>{t.count ? t.count.toLocaleString() : ''}</div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {sel && (
        <div style={{ marginTop: 12 }}>
          <div className="tm-rule" style={{ margin: '8px 0 6px' }} />
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
            <span className="tm-px" style={{ fontSize: 10 }}>{sel.label}</span>
            {!demo && <span className="tm-sc">snapshot {view?.snapshotId}</span>}
            <span className="tm-sc" style={{ marginLeft: 'auto', fontSize: 9 }}>
              {drill.loading ? 'loading…' : `${drill.total.toLocaleString()} addresses`}
            </span>
            <button
              className="tm-sc"
              style={{ font: 'inherit', cursor: 'pointer', border: 'none', background: 'none', textDecoration: 'underline' }}
              onClick={() => setSel(null)}
            >
              close
            </button>
          </div>
          {drill.error ? (
            <div className="tm-sc" style={{ padding: '8px 0' }}>Drilldown failed: {drill.error}</div>
          ) : drill.items.length === 0 && !drill.loading ? (
            <div className="tm-sc" style={{ padding: '8px 0' }}>No addresses in this cell.</div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 9.5 }}>
                <thead>
                  <tr>
                    {['address', 'side', 'size', 'entry', 'notional', 'cost/mark', 'win rate', 'rounds'].map((h) => (
                      <th
                        key={h}
                        className="tm-sc"
                        style={{ textAlign: 'right', padding: '3px 6px', borderBottom: '1px solid var(--tm-rule)' }}
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {drill.items.map((it) => (
                    <tr key={it.address}>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{it.address}</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px', color: it.side === 'long' ? 'var(--tm-up)' : 'var(--tm-dn)' }}>
                        {it.side === 'long' ? 'long' : 'short'}
                      </td>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{(+it.size).toLocaleString()}</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{fmtPx(+it.entryPrice)}</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px', fontWeight: 700 }}>{fmtUsd(+it.notional)}</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{(+it.costRatio).toFixed(1)}%</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{(+it.winRate).toFixed(1)}%</td>
                      <td style={{ textAlign: 'right', padding: '2px 6px' }}>{it.roundTrips}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginTop: 4 }} className="tm-sc">
                <button
                  style={{ font: 'inherit', cursor: 'pointer', border: 'none', background: 'none', textDecoration: 'underline' }}
                  disabled={drillOffset === 0 || drill.loading}
                  onClick={() => setDrillOffset(Math.max(0, drillOffset - DRILL_LIMIT))}
                >
                  ← prev
                </button>
                <span>
                  {drillOffset + 1}–{drillOffset + drill.items.length} / {drill.total.toLocaleString()}
                </span>
                <button
                  style={{ font: 'inherit', cursor: drill.nextOffset == null ? 'default' : 'pointer', border: 'none', background: 'none', textDecoration: 'underline' }}
                  disabled={drill.nextOffset == null || drill.loading}
                  onClick={() => drill.nextOffset != null && setDrillOffset(drill.nextOffset)}
                >
                  next →
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function MatrixRow({
  row, side, max, sel, onPick,
}: {
  row: SideView['rows'][number]
  side: 'long' | 'short'
  max: number
  sel: DrillSel | null
  onPick: (winIdx: number, cell: MatrixCell) => void
}) {
  const active = (winIdx: number) =>
    !!sel &&
    sel.side === side &&
    sel.winIdx === winIdx &&
    sel.slots[0] === row.slots[0] &&
    sel.slots[sel.slots.length - 1] === row.slots[row.slots.length - 1]
  return (
    <>
      <div
        className="tm-sc"
        style={{
          textAlign: 'right',
          paddingRight: 6,
          alignSelf: 'center',
          fontSize: 8.5,
          whiteSpace: 'nowrap',
          borderTop: row.boundary ? '1.5px dashed var(--tm-red)' : undefined,
          color: row.boundary ? 'var(--tm-red)' : undefined,
        }}
        title={row.boundary ? 'current price (cost = 100%)' : undefined}
      >
        {row.label}
      </div>
      {row.cells.map((cell, i) => {
        const alpha =
          cell.notional > 0 && max > 0
            ? Math.min(0.92, 0.1 + (0.82 * Math.log10(1 + cell.notional)) / Math.log10(1 + max))
            : 0
        return (
          <button
            key={i}
            onClick={() => onPick(i, cell)}
            disabled={cell.notional <= 0}
            style={{
              border: active(i) ? '1.5px solid var(--tm-ink)' : '1px solid rgba(26,24,19,0.08)',
              borderTop: row.boundary ? '1.5px dashed var(--tm-red)' : '1px solid rgba(26,24,19,0.08)',
              background:
                cell.notional > 0
                  ? side === 'long'
                    ? `rgba(46,139,87,${alpha.toFixed(2)})`
                    : `rgba(214,67,58,${alpha.toFixed(2)})`
                  : 'transparent',
              color: alpha > 0.45 ? '#f7f4ec' : 'var(--tm-ink-2)',
              cursor: cell.notional > 0 ? 'pointer' : 'default',
              minHeight: 30,
              padding: '2px 1px',
              font: 'inherit',
              lineHeight: 1.25,
            }}
          >
            <div style={{ fontWeight: 700 }}>{cell.notional > 0 ? fmtUsd(cell.notional) : '0'}</div>
            <div style={{ fontSize: 7.5, opacity: 0.85 }}>
              {cell.count > 0 ? `${cell.count.toLocaleString()}a` : ''}
            </div>
          </button>
        )
      })}
    </>
  )
}

export default WinrateMatrix
