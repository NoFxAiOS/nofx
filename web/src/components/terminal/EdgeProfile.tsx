import { useMemo } from 'react'
import type { HistoricalPosition } from '../../types'

/**
 * EdgeProfile — where the money actually comes from. Aggregates recent closed
 * trades into hold-duration buckets plus a long/short split, each with net
 * PnL, fee load and win rate. This is the panel that made the fee-drag and
 * churn problems visible, kept on the dashboard so regressions show up
 * immediately.
 */

interface EdgeProfileProps {
  positions?: HistoricalPosition[]
}

interface BucketAgg {
  label: string
  n: number
  net: number
  fees: number
  wins: number
}

function newBucket(label: string): BucketAgg {
  return { label, n: 0, net: 0, fees: 0, wins: 0 }
}

function add(bucket: BucketAgg, pos: HistoricalPosition) {
  bucket.n += 1
  bucket.net += pos.realized_pnl || 0
  bucket.fees += pos.fee || 0
  if ((pos.realized_pnl || 0) > 0) bucket.wins += 1
}

function fmtUsd(n: number): string {
  const sign = n < 0 ? '-' : '+'
  return `${sign}$${Math.abs(n).toFixed(2)}`
}

/** Accepts epoch ms (the API's format) or a date string, returns epoch ms. */
function toEpochMs(value: number | string): number {
  if (typeof value === 'number') return value
  const numeric = Number(value)
  if (Number.isFinite(numeric) && numeric > 0) return numeric
  return Date.parse(value)
}

export function EdgeProfile({ positions }: EdgeProfileProps) {
  const { holdBuckets, sideBuckets, sample } = useMemo(() => {
    const holds = [
      newBucket('<15m'),
      newBucket('15-60m'),
      newBucket('1-3h'),
      newBucket('>3h'),
    ]
    const sides = [newBucket('long'), newBucket('short')]
    let counted = 0

    for (const pos of positions ?? []) {
      if ((pos.status || '').toUpperCase() !== 'CLOSED') continue
      const entry = toEpochMs(pos.entry_time)
      const exit = toEpochMs(pos.exit_time)
      if (!Number.isFinite(entry) || !Number.isFinite(exit) || exit <= entry) {
        continue
      }
      counted += 1

      const holdMin = (exit - entry) / 60000
      const holdBucket =
        holdMin < 15 ? holds[0] : holdMin < 60 ? holds[1] : holdMin < 180 ? holds[2] : holds[3]
      add(holdBucket, pos)

      const sideBucket =
        (pos.side || '').toLowerCase() === 'short' ? sides[1] : sides[0]
      add(sideBucket, pos)
    }

    return { holdBuckets: holds, sideBuckets: sides, sample: counted }
  }, [positions])

  if (sample === 0) {
    return <div className="tm-sc">No closed trades yet.</div>
  }

  const maxAbsNet = Math.max(0.01, ...holdBuckets.map((b) => Math.abs(b.net)))

  const row = (bucket: BucketAgg) => {
    const winPct = bucket.n > 0 ? (100 * bucket.wins) / bucket.n : 0
    const up = bucket.net >= 0
    return (
      <div key={bucket.label} style={{ marginBottom: 7 }}>
        <div className="tm-mono" style={{ display: 'flex', alignItems: 'baseline', fontSize: 11, marginBottom: 2 }}>
          <span style={{ fontWeight: 500, minWidth: 52 }}>{bucket.label}</span>
          <span className="tm-sc">
            {bucket.n} trades · {bucket.n > 0 ? `${winPct.toFixed(0)}% win` : '—'} · fees ${bucket.fees.toFixed(2)}
          </span>
          <span className={up ? 'tm-up' : 'tm-dn'} style={{ marginLeft: 'auto', fontWeight: 600 }}>
            {bucket.n > 0 ? fmtUsd(bucket.net) : '—'}
          </span>
        </div>
        {/* diverging net bar around a center axis */}
        <div style={{ display: 'flex', height: 4, background: 'var(--tm-hair)' }}>
          <div style={{ width: '50%', display: 'flex', justifyContent: 'flex-end' }}>
            {!up && (
              <div style={{ height: 4, width: `${(Math.abs(bucket.net) / maxAbsNet) * 100}%`, background: 'var(--tm-dn)' }} />
            )}
          </div>
          <div style={{ width: '50%' }}>
            {up && bucket.net > 0 && (
              <div style={{ height: 4, width: `${(bucket.net / maxAbsNet) * 100}%`, background: 'var(--tm-up)' }} />
            )}
          </div>
        </div>
      </div>
    )
  }

  // one-line takeaway: does patience pay on this book?
  const shortHolds = holdBuckets[0].net + holdBuckets[1].net
  const longHolds = holdBuckets[2].net + holdBuckets[3].net
  const takeaway =
    longHolds > shortHolds
      ? `edge concentrates in holds ≥ 1h (${fmtUsd(longHolds)} vs ${fmtUsd(shortHolds)} under 1h)`
      : `short holds outperform on this sample (${fmtUsd(shortHolds)} vs ${fmtUsd(longHolds)} ≥ 1h)`

  return (
    <div>
      {holdBuckets.map(row)}
      <div style={{ borderTop: '1px solid var(--tm-hair)', margin: '8px 0 7px' }} />
      {sideBuckets.map(row)}
      <div className="tm-sc" style={{ marginTop: 6, fontSize: 9 }}>
        last {sample} closed · {takeaway}
      </div>
    </div>
  )
}

export default EdgeProfile
