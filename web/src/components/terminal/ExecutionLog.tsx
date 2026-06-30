import { useMemo } from 'react'
import type { DecisionRecord } from '../../types'

/**
 * ExecutionLog renders the AI trading agent's real decisions and order results
 * as a high-density, newest-first terminal stream — Bloomberg-style log on the
 * cream paper theme. Each cycle is a clearly-delimited block: a header bar
 * (cycle no · time · action count), one badge line per AI action, and one
 * indented sub-line per execution-log entry, color-coded by
 * success / throttle / risk.
 *
 * Real data only — renders verbatim what's present on each DecisionRecord;
 * verbose throttle strings are tidied for the gist but never invented.
 */

const C_AMBER = '#c8860b' // throttle / block warnings

// Strip dex prefix + quote suffix to the bare base ticker.
function baseSymbol(raw: string): string {
  return raw
    .toUpperCase()
    .replace(/^XYZ:/, '')
    .replace(/(USDT|USDC|USD)$/, '')
}

// HH:MM:SS from an ISO timestamp; guards against invalid input.
function fmtTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '--:--:--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

type Side = 'long' | 'short' | 'flat'

// Trade side of an action token — drives badge color.
function actionSide(action: string): Side {
  const a = action.toLowerCase()
  if (a === 'open_long' || a === 'close_short') return 'long'
  if (a === 'open_short' || a === 'close_long') return 'short'
  return 'flat'
}

function sideColor(side: Side): string {
  if (side === 'long') return 'var(--tm-up)'
  if (side === 'short') return 'var(--tm-dn)'
  return 'var(--tm-muted)'
}

type LogTone = 'ok' | 'warn' | 'risk' | 'info'

// Classify an execution-log line into a tone for color-coding.
function logTone(line: string): LogTone {
  const s = line.toLowerCase()
  if (s.includes('succeed') || s.includes('success') || line.includes('✓')) return 'ok'
  if (s.includes('throttle') || s.includes('re-entry') || s.includes('cooldown') || s.includes('blocked'))
    return 'warn'
  if (
    s.includes('risk') ||
    s.includes('fail') ||
    s.includes('reject') ||
    s.includes('error') ||
    s.includes('denied') ||
    line.includes('✗') ||
    line.includes('❌')
  )
    return 'risk'
  return 'info'
}

const TONE_COLOR: Record<LogTone, string> = {
  ok: 'var(--tm-up)',
  warn: C_AMBER,
  risk: 'var(--tm-dn)',
  info: 'var(--tm-ink-2)',
}
const TONE_GLYPH: Record<LogTone, string> = { ok: '✓', warn: '⚠', risk: '❌', info: '·' }

/**
 * Tidy a verbose execution-log string to its gist without fabricating data.
 * Strips a leading "└"/glyph the backend may already include (we render our
 * own), the symbol (already shown), and collapses the wordy throttle phrasing
 * to "throttle · closed 28m ago, wait 2m". Falls back to the original line.
 */
function cleanLog(raw: string): string {
  let s = raw.replace(/^[\s└>•·]*[✓✗⚠❌]?\s*/, '').trim()

  const throttle = s.match(/closed\s+([0-9smhd.]+)\s+ago;\s*wait\s+([0-9smhd.]+)/i)
  if (throttle) {
    const ago = throttle[1].replace(/(\d)0s$/, '$1').replace(/0s$/, '')
    const wait = throttle[2].replace(/(\d)0s$/, '$1').replace(/0s$/, '')
    return `throttle · closed ${ago} ago, wait ${wait}`
  }
  // drop a redundant leading "SYMBOL action" prefix when present
  s = s.replace(/^[A-Z0-9:_-]{2,12}\s+(open_long|open_short|close_long|close_short)\s+/i, '')
  return s
}

interface ExecutionLogProps {
  decisions?: DecisionRecord[]
  height?: number
}

export function ExecutionLog({ decisions, height = 440 }: ExecutionLogProps) {
  // Newest cycle first.
  const cycles = useMemo(() => {
    const list = decisions ?? []
    return [...list].sort((a, b) => b.cycle_number - a.cycle_number)
  }, [decisions])

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      {/* header */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 2 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>Execution log</span>
        <span
          className="tm-sc"
          style={{ marginLeft: 'auto', color: cycles.length ? 'var(--tm-up)' : 'var(--tm-muted)' }}
        >
          {cycles.length ? `${cycles.length} cyc` : '—'}
        </span>
      </div>
      <div className="tm-sc" style={{ fontSize: 9, marginBottom: 5 }}>
        Execution log · AI decisions & fills per cycle
      </div>

      {/* legend */}
      <div
        className="tm-sc"
        style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 6, fontSize: 9 }}
      >
        <Legend glyph="✓" c="var(--tm-up)" label="ok" />
        <Legend glyph="⚠" c={C_AMBER} label="throttle" />
        <Legend glyph="❌" c="var(--tm-dn)" label="risk" />
      </div>

      <div className="tm-hair" style={{ marginBottom: 0 }} />

      {!cycles.length ? (
        <div className="tm-sc" style={{ padding: '16px 0' }}>No execution events yet.</div>
      ) : (
        <div
          style={{
            height,
            overflowY: 'auto',
            fontSize: 10,
            lineHeight: 1.5,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {cycles.map((c) => (
            <Cycle key={`${c.cycle_number}-${c.timestamp}`} record={c} />
          ))}
        </div>
      )}
    </div>
  )
}

function Legend({ glyph, c, label }: { glyph: string; c: string; label: string }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
      <span style={{ color: c, fontSize: 10 }}>{glyph}</span>
      <span>{label}</span>
    </span>
  )
}

interface CycleProps {
  record: DecisionRecord
}

function Cycle({ record }: CycleProps) {
  const time = fmtTime(record.timestamp)
  const actions = record.decisions ?? []
  const logs = record.execution_log ?? []
  const count = actions.length

  return (
    <div style={{ borderBottom: '1px solid var(--tm-hair)', padding: '5px 0' }}>
      {/* cycle header bar */}
      <div
        className="tm-sc"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 7,
          fontSize: 9,
          padding: '2px 6px',
          marginBottom: actions.length || logs.length || record.error_message ? 4 : 0,
          background: 'rgba(26,24,19,0.045)',
          borderLeft: `2px solid ${record.success ? 'var(--tm-hair)' : 'var(--tm-dn)'}`,
          color: 'var(--tm-ink-2)',
        }}
      >
        <span style={{ color: 'var(--tm-ink)', fontWeight: 700 }}>CYCLE {record.cycle_number}</span>
        <span style={{ color: 'var(--tm-muted)' }}>·</span>
        <span style={{ color: 'var(--tm-muted)' }}>{time}</span>
        <span style={{ marginLeft: 'auto', color: 'var(--tm-muted)' }}>
          {count === 0 ? 'no action' : `${count} action${count > 1 ? 's' : ''}`}
        </span>
        {!record.success ? (
          <span style={{ color: 'var(--tm-dn)', fontWeight: 700 }}>FAULT</span>
        ) : null}
      </div>

      {/* AI action lines */}
      {actions.map((a, i) => {
        const side = actionSide(a.action)
        const aTime = fmtTime(a.timestamp)
        return (
          <div
            key={`act-${i}`}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '1px 6px',
              color: 'var(--tm-ink-2)',
            }}
          >
            <span style={{ color: 'var(--tm-muted)', flex: '0 0 auto', minWidth: 52 }}>
              {aTime !== '--:--:--' ? aTime : time}
            </span>
            <ActionBadge action={a.action} side={side} />
            <span style={{ color: 'var(--tm-ink)', fontWeight: 600, flex: '0 0 auto', minWidth: 48 }}>
              {baseSymbol(a.symbol)}
            </span>
            {a.confidence != null ? (
              <span style={{ color: 'var(--tm-muted)', flex: '0 0 auto' }}>
                conf<span style={{ color: 'var(--tm-ink-2)' }}>{Math.round(a.confidence)}</span>
              </span>
            ) : null}
          </div>
        )
      })}

      {/* execution-log result sub-lines */}
      {logs.map((line, i) => (
        <SubLine key={`log-${i}`} tone={logTone(line)} text={cleanLog(line)} />
      ))}

      {/* fault message */}
      {record.error_message ? (
        <SubLine tone="risk" text={cleanLog(record.error_message)} />
      ) : null}
    </div>
  )
}

function ActionBadge({ action, side }: { action: string; side: Side }) {
  const c = sideColor(side)
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        flex: '0 0 auto',
        padding: '0 5px',
        height: 14,
        fontSize: 9,
        letterSpacing: '0.04em',
        color: c,
        border: `1px solid ${c}`,
        background:
          side === 'long'
            ? 'rgba(46,139,87,0.08)'
            : side === 'short'
              ? 'rgba(214,67,58,0.08)'
              : 'transparent',
        borderRadius: 2,
        whiteSpace: 'nowrap',
      }}
    >
      {action.toLowerCase()}
    </span>
  )
}

function SubLine({ tone, text }: { tone: LogTone; text: string }) {
  const color = TONE_COLOR[tone]
  const glyph = TONE_GLYPH[tone]
  return (
    <div
      style={{
        display: 'flex',
        gap: 6,
        padding: '0 6px 0 10px',
        marginLeft: 6,
        borderLeft: '1px solid var(--tm-hair)',
        color,
      }}
    >
      <span style={{ flex: '0 0 auto', width: 10, textAlign: 'center' }}>{glyph}</span>
      <span style={{ wordBreak: 'break-word' }}>{text}</span>
    </div>
  )
}

export default ExecutionLog
