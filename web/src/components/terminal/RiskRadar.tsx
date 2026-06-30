import { useMemo } from 'react'
import type { Position } from '../../types'

/**
 * RiskRadar renders derived risk telemetry for the live trading book — long /
 * short exposure split, leverage usage vs config cap, margin utilization,
 * single-name concentration, max drawdown and position count — as a dense stack
 * of self-explanatory gauge rows. Each row reads as: bilingual label · value +
 * unit · a thin gauge bar · and a one-glance verdict tag. Cream-themed
 * Bloomberg/terminal cockpit.
 *
 * Every value is DERIVED from real props. No synthetic or random data; missing
 * inputs collapse to 0 and divides are guarded.
 */

const C_AMBER = '#c8860b' // escalation tint between green and red

function fmtUsd(n: number): string {
  const a = Math.abs(n)
  const sign = n < 0 ? '-' : ''
  if (a >= 1e6) return `${sign}$${(a / 1e6).toFixed(2)}M`
  if (a >= 1e3) return `${sign}$${(a / 1e3).toFixed(1)}K`
  return `${sign}$${a.toFixed(0)}`
}

function pct(n: number): string {
  return `${n.toFixed(1)}%`
}

function isLong(side: string): boolean {
  return (side || '').toLowerCase() === 'long'
}

// margin utilization escalates green → amber → red as the book fills up
function utilColor(p: number): string {
  if (p > 80) return 'var(--tm-dn)'
  if (p >= 50) return C_AMBER
  return 'var(--tm-up)'
}

interface RiskRadarProps {
  positions?: Position[]
  account?: { total_equity?: number; total_unrealized_profit?: number; margin_used_pct?: number } | null
  config?: { btc_eth_leverage?: number; altcoin_leverage?: number; max_positions?: number } | null
  fullStats?: { max_drawdown_pct?: number; profit_factor?: number; sharpe_ratio?: number; win_rate?: number } | null
}

export function RiskRadar({ positions, account, config, fullStats }: RiskRadarProps) {
  const pos = positions ?? []

  const m = useMemo(() => {
    const equity = account?.total_equity ?? 0

    let longNotional = 0
    let shortNotional = 0
    let levSum = 0
    let levCount = 0
    let maxLev = 0
    let marginSum = 0
    let topNotional = 0

    for (const p of pos) {
      const px = p.mark_price || p.entry_price || 0
      const notional = Math.abs(p.quantity || 0) * px
      if (isLong(p.side)) longNotional += notional
      else shortNotional += notional

      const lev = p.leverage || 0
      if (lev > 0) {
        levSum += lev
        levCount += 1
        if (lev > maxLev) maxLev = lev
      }
      marginSum += p.margin_used || 0
      if (notional > topNotional) topNotional = notional
    }

    const totalNotional = longNotional + shortNotional
    const netNotional = longNotional - shortNotional
    const longShare = totalNotional > 0 ? (longNotional / totalNotional) * 100 : 0
    const shortShare = totalNotional > 0 ? (shortNotional / totalNotional) * 100 : 0

    const avgLev = levCount > 0 ? levSum / levCount : 0
    const configMax = Math.max(config?.btc_eth_leverage ?? 0, config?.altcoin_leverage ?? 0)
    const levUse = configMax > 0 ? Math.min(100, (avgLev / configMax) * 100) : 0

    const marginPct =
      account?.margin_used_pct != null
        ? account.margin_used_pct
        : equity > 0
          ? (marginSum / equity) * 100
          : 0

    const concentration = totalNotional > 0 ? (topNotional / totalNotional) * 100 : 0

    const ddFrac = fullStats?.max_drawdown_pct ?? 0
    const drawdown = ddFrac * 100

    const count = pos.length
    const maxPositions = config?.max_positions ?? 0
    const countUse = maxPositions > 0 ? Math.min(100, (count / maxPositions) * 100) : 0

    const upnl = account?.total_unrealized_profit ?? 0

    return {
      longNotional,
      shortNotional,
      netNotional,
      longShare,
      shortShare,
      totalNotional,
      avgLev,
      maxLev,
      configMax,
      levUse,
      marginPct,
      concentration,
      drawdown,
      count,
      maxPositions,
      countUse,
      upnl,
    }
  }, [pos, account, config, fullStats])

  const hasData = pos.length > 0 || account != null
  if (!hasData) {
    return <div className="tm-sc" style={{ padding: '16px 0' }}>No live risk data.</div>
  }

  // ── one-glance verdicts ──────────────────────────────────────────────
  // Net exposure bias: Long-lean / Short-lean / Balanced by the long-share spread around 50%.
  const biasSkew = m.longShare - m.shortShare
  const exposureTag: Verdict =
    m.totalNotional === 0
      ? { text: 'Flat', tone: 'muted' }
      : biasSkew > 15
        ? { text: 'Long-lean', tone: 'up' }
        : biasSkew < -15
          ? { text: 'Short-lean', tone: 'dn' }
          : { text: 'Balanced', tone: 'ink' }

  // Leverage: Safe / High / Risky by avg vs cap.
  const levTag: Verdict =
    m.configMax === 0 || m.avgLev === 0
      ? { text: '—', tone: 'muted' }
      : m.levUse > 80
        ? { text: 'Risky', tone: 'dn' }
        : m.levUse >= 50
          ? { text: 'High', tone: 'amber' }
          : { text: 'Safe', tone: 'up' }

  // Margin used: Ample / Tight / Risky.
  const marginTag: Verdict =
    m.marginPct > 80
      ? { text: 'Risky', tone: 'dn' }
      : m.marginPct >= 50
        ? { text: 'Tight', tone: 'amber' }
        : { text: 'Ample', tone: 'up' }

  // Concentration: Spread / Concentrated.
  const concTag: Verdict =
    m.totalNotional === 0
      ? { text: '—', tone: 'muted' }
      : m.concentration >= 35
        ? { text: 'Concentrated', tone: 'amber' }
        : { text: 'Spread', tone: 'up' }

  // Drawdown: Calm / Caution / Deep by depth.
  const ddTag: Verdict =
    m.drawdown <= 0
      ? { text: 'Calm', tone: 'up' }
      : m.drawdown >= 20
        ? { text: 'Deep', tone: 'dn' }
        : { text: 'Caution', tone: 'amber' }

  // Positions: Room / Full.
  const countTag: Verdict =
    m.maxPositions === 0
      ? { text: `${m.count}`, tone: 'muted' }
      : m.count >= m.maxPositions
        ? { text: 'Full', tone: 'amber' }
        : { text: 'Room', tone: 'up' }

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      {/* header */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 1 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>Risk radar</span>
        <span
          className="tm-sc"
          style={{ marginLeft: 'auto', color: m.totalNotional > 0 ? 'var(--tm-up)' : 'var(--tm-muted)' }}
        >
          {m.totalNotional > 0 ? '● live' : '○ flat'}
        </span>
      </div>
      <div className="tm-sc" style={{ fontSize: 9, marginBottom: 8 }}>
        Risk radar · live position-risk check
      </div>

      {/* Net exposure — diverging long/short split, the visual centerpiece */}
      <div style={{ marginBottom: 9, paddingBottom: 9, borderBottom: '1px solid var(--tm-hair)' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', marginBottom: 4 }}>
          <Label zh="Net exposure" en="NET EXPOSURE" />
          <Tag verdict={exposureTag} />
          <span className="tm-mono" style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--tm-ink)' }}>
            long {pct(m.longShare)}
            <span style={{ color: 'var(--tm-muted)' }}> / </span>
            short {pct(m.shortShare)}
          </span>
        </div>
        <div style={{ display: 'flex', height: 7, background: 'var(--tm-hair)', overflow: 'hidden' }}>
          <div style={{ width: `${m.longShare}%`, background: 'var(--tm-up)' }} />
          <div style={{ width: `${m.shortShare}%`, background: 'var(--tm-dn)' }} />
        </div>
        <div className="tm-mono" style={{ display: 'flex', justifyContent: 'space-between', fontSize: 9, marginTop: 3 }}>
          <span style={{ color: 'var(--tm-up)' }}>long {fmtUsd(m.longNotional)}</span>
          <span style={{ color: 'var(--tm-ink-2)' }}>
            net <b style={{ color: m.netNotional >= 0 ? 'var(--tm-up)' : 'var(--tm-dn)' }}>{fmtUsd(m.netNotional)}</b>
          </span>
          <span style={{ color: 'var(--tm-dn)' }}>short {fmtUsd(m.shortNotional)}</span>
        </div>
      </div>

      {/* gauge rows */}
      <GaugeRow
        zh="Leverage"
        en="LEVERAGE"
        value={`${m.avgLev.toFixed(1)}× avg`}
        sub={`/ ${m.maxLev > 0 ? `${m.maxLev.toFixed(0)}×` : '—'} peak · ${m.configMax > 0 ? `${m.configMax}×` : '—'} cap`}
        fill={m.levUse}
        color={levTag.tone === 'dn' ? 'var(--tm-dn)' : levTag.tone === 'amber' ? C_AMBER : 'var(--tm-up)'}
        verdict={levTag}
      />
      <GaugeRow
        zh="Margin used"
        en="MARGIN USED"
        value={pct(m.marginPct)}
        sub="of equity"
        fill={Math.min(100, Math.max(0, m.marginPct))}
        color={utilColor(m.marginPct)}
        verdict={marginTag}
      />
      <GaugeRow
        zh="Concentration"
        en="CONCENTRATION"
        value={pct(m.concentration)}
        sub="top-position share"
        fill={m.concentration}
        color={concTag.tone === 'amber' ? C_AMBER : 'var(--tm-up)'}
        verdict={concTag}
      />
      <GaugeRow
        zh="Drawdown"
        en="MAX DRAWDOWN"
        value={`-${pct(m.drawdown)}`}
        sub="peak drawdown"
        fill={Math.min(100, m.drawdown)}
        color="var(--tm-red)"
        verdict={ddTag}
        valueColor="var(--tm-dn)"
      />
      <GaugeRow
        zh="Positions"
        en="POSITIONS"
        value={m.maxPositions > 0 ? `${m.count} / ${m.maxPositions}` : `${m.count}`}
        sub="held / cap"
        fill={m.maxPositions > 0 ? m.countUse : 0}
        color={countTag.tone === 'amber' ? C_AMBER : 'var(--tm-up)'}
        verdict={countTag}
      />

      {/* unrealized PnL footer */}
      <div
        style={{
          display: 'flex',
          alignItems: 'baseline',
          marginTop: 8,
          paddingTop: 7,
          borderTop: '1px solid var(--tm-hair)',
        }}
      >
        <Label zh="Unrealized PnL" en="UNREALIZED PNL" />
        <span
          className="tm-mono"
          style={{ marginLeft: 'auto', fontSize: 13, fontWeight: 700, color: m.upnl >= 0 ? 'var(--tm-up)' : 'var(--tm-dn)' }}
        >
          {m.upnl >= 0 ? '+' : ''}{fmtUsd(m.upnl)}
        </span>
      </div>
    </div>
  )
}

// ── verdict tag ────────────────────────────────────────────────────────
type Tone = 'up' | 'dn' | 'amber' | 'ink' | 'muted'

interface Verdict {
  text: string
  tone: Tone
}

function toneColor(tone: Tone): string {
  switch (tone) {
    case 'up':
      return 'var(--tm-up)'
    case 'dn':
      return 'var(--tm-dn)'
    case 'amber':
      return C_AMBER
    case 'ink':
      return 'var(--tm-ink)'
    default:
      return 'var(--tm-muted)'
  }
}

function Tag({ verdict }: { verdict: Verdict }) {
  const c = toneColor(verdict.tone)
  return (
    <span
      style={{
        marginLeft: 6,
        padding: '0 4px',
        fontSize: 9,
        lineHeight: '13px',
        letterSpacing: '0.08em',
        color: c,
        border: `1px solid ${c}`,
        borderRadius: 2,
      }}
    >
      {verdict.text}
    </span>
  )
}

// ── bilingual label block ──────────────────────────────────────────────
function Label({ zh, en }: { zh: string; en: string }) {
  return (
    <span style={{ display: 'inline-flex', flexDirection: 'column', lineHeight: 1.2 }}>
      <span style={{ fontSize: 11, color: 'var(--tm-ink)', fontWeight: 600 }}>{zh}</span>
      <span className="tm-sc" style={{ fontSize: 8, letterSpacing: '0.12em' }}>{en}</span>
    </span>
  )
}

interface GaugeRowProps {
  zh: string
  en: string
  value: string
  sub?: string
  fill: number
  color: string
  verdict: Verdict
  valueColor?: string
}

function GaugeRow({ zh, en, value, sub, fill, color, verdict, valueColor }: GaugeRowProps) {
  const w = Math.min(100, Math.max(0, fill))
  return (
    <div style={{ marginBottom: 9 }}>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 4 }}>
        <Label zh={zh} en={en} />
        <Tag verdict={verdict} />
        <span
          className="tm-mono"
          style={{ marginLeft: 'auto', fontSize: 12, fontWeight: 600, color: valueColor ?? 'var(--tm-ink)' }}
        >
          {value}
        </span>
      </div>
      <div style={{ height: 5, background: 'var(--tm-hair)', overflow: 'hidden' }}>
        <div style={{ width: `${w}%`, height: '100%', background: color, transition: 'width 0.2s ease-out' }} />
      </div>
      {sub && (
        <div className="tm-sc" style={{ fontSize: 8, marginTop: 2 }}>{sub}</div>
      )}
    </div>
  )
}

export default RiskRadar
