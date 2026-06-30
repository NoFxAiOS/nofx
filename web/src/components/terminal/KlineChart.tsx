import { useEffect, useMemo, useRef, useState } from 'react'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type { Kline } from '../../lib/api/data'
import { Candles } from './Candles'

/**
 * KlineChart shows a live candlestick chart. History is seeded from the backend
 * kline endpoint, then the latest bar streams in real time from Hyperliquid's
 * public `candle` WebSocket (the forming candle ticks live and rolls over each
 * interval). Resolves crypto majors to the main dex and synthetic markets to
 * the `xyz:` builder dex, matching the order book.
 *
 * Real OHLC only — no synthetic data.
 */

const HL_INFO = 'https://api.hyperliquid.xyz/info'
const HL_WS = 'wss://api.hyperliquid.xyz/ws'
const INTERVAL = '1m'
const MAX_BARS = 90

function baseSymbol(raw: string): string {
  return raw.toUpperCase().replace(/^XYZ:/, '').replace(/(USDT|USDC|USD)$/, '')
}

interface KlineChartProps {
  symbol: string
  /** target chart height in px (ignored when fill) */
  height?: number
  /** stretch the chart to fill the parent's remaining height */
  fill?: boolean
}

export function KlineChart({ symbol, height = 360, fill = false }: KlineChartProps) {
  const base = baseSymbol(symbol || '')

  // history seed (resynced occasionally; the WS carries the live bar)
  const { data: seed, isLoading } = useSWR(
    base ? ['kline', base, INTERVAL] : null,
    () => api.getKlines(base, INTERVAL, 'hyperliquid', MAX_BARS, true),
    { refreshInterval: 60000, revalidateOnFocus: false, shouldRetryOnError: false, keepPreviousData: true },
  )

  // resolve the Hyperliquid coin id (xyz: dex membership)
  const [xyzSet, setXyzSet] = useState<Set<string>>(new Set())
  useEffect(() => {
    let alive = true
    fetch(HL_INFO, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ type: 'allMids', dex: 'xyz' }) })
      .then((r) => r.json())
      .then((mids: Record<string, string>) => {
        if (!alive) return
        const set = new Set<string>()
        for (const k of Object.keys(mids || {})) set.add(k.replace(/^xyz:/, '').toUpperCase())
        setXyzSet(set)
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])
  const coin = useMemo(() => (base ? (xyzSet.has(base) ? `xyz:${base}` : base) : ''), [base, xyzSet])

  // live bar from the candle WS
  const [liveBar, setLiveBar] = useState<Kline | null>(null)
  const [wsLive, setWsLive] = useState(false)
  const pending = useRef<Kline | null>(null)
  useEffect(() => {
    if (!coin) return
    setLiveBar(null)
    let ws: WebSocket | null = null
    let raf: number | null = null
    let retry: ReturnType<typeof setTimeout> | null = null
    let closed = false

    const connect = () => {
      ws = new WebSocket(HL_WS)
      ws.onopen = () => ws?.send(JSON.stringify({ method: 'subscribe', subscription: { type: 'candle', coin, interval: INTERVAL } }))
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.channel !== 'candle' || !msg.data) return
          const d = msg.data
          pending.current = { openTime: d.t, closeTime: d.T, open: +d.o, high: +d.h, low: +d.l, close: +d.c, volume: +d.v }
          setWsLive(true)
        } catch {
          /* ignore */
        }
      }
      ws.onclose = () => {
        if (closed) return
        setWsLive(false)
        retry = setTimeout(connect, 1500)
      }
      ws.onerror = () => ws?.close()
    }
    connect()
    const loop = () => {
      if (pending.current) {
        setLiveBar(pending.current)
        pending.current = null
      }
      raf = requestAnimationFrame(loop)
    }
    raf = requestAnimationFrame(loop)

    return () => {
      closed = true
      if (raf) cancelAnimationFrame(raf)
      if (retry) clearTimeout(retry)
      try {
        ws?.send(JSON.stringify({ method: 'unsubscribe', subscription: { type: 'candle', coin, interval: INTERVAL } }))
      } catch {
        /* socket gone */
      }
      ws?.close()
    }
  }, [coin])

  // merge the live bar into the seeded history
  const candles = useMemo(() => {
    const hist = seed ?? []
    if (!liveBar) return hist
    const arr = [...hist]
    const last = arr[arr.length - 1]
    if (last && liveBar.openTime === last.openTime) arr[arr.length - 1] = liveBar
    else if (!last || liveBar.openTime > last.openTime) arr.push(liveBar)
    return arr.slice(-MAX_BARS)
  }, [seed, liveBar])

  const last = candles.length ? candles[candles.length - 1].close : 0
  const first = candles.length ? candles[0].open : 0
  const chg = first ? ((last - first) / first) * 100 : 0
  const live = wsLive && candles.length > 0

  return (
    <div style={{ fontFamily: 'var(--tm-mono)', ...(fill ? { display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 } : {}) }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>{base || 'MARKET'}</span>
        <span className="tm-sc">{INTERVAL} · Live candles</span>
        <span className="tm-sc" style={{ marginLeft: 'auto', color: live ? 'var(--tm-up)' : 'var(--tm-muted)' }}>
          {live ? '● live' : isLoading || candles.length ? '○ sync' : '○ —'}
        </span>
      </div>
      {last > 0 && (
        <div className="tm-mono" style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 4, fontSize: 12 }}>
          <span style={{ fontWeight: 600 }}>${last.toLocaleString('en-US', { maximumFractionDigits: 4 })}</span>
          <span className={chg >= 0 ? 'tm-up' : 'tm-dn'} style={{ fontSize: 11 }}>{chg >= 0 ? '+' : ''}{chg.toFixed(2)}%</span>
          <span className="tm-sc" style={{ marginLeft: 'auto' }}>{candles.length} bars · {INTERVAL}</span>
        </div>
      )}
      {candles.length > 0 ? (
        fill ? (
          <div style={{ flex: 1, minHeight: 0 }}>
            <Candles data={candles} width={380} height={height} fill />
          </div>
        ) : (
          <Candles data={candles} width={380} height={height} />
        )
      ) : (
        <div className="tm-sc" style={{ padding: '20px 0' }}>Loading market…</div>
      )}
    </div>
  )
}

export default KlineChart
