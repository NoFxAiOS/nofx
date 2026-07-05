import { useEffect, useMemo, useRef, useState } from 'react'
import { demoSeedPrice, demoTick } from '../../lib/demo/demoUniverse'

const rnd = (a: number, b: number) => a + Math.random() * (b - a)

/**
 * OrderBook renders a live L2 depth ladder for a single instrument, streamed
 * directly from Hyperliquid's public WebSocket (`l2Book`). The app trades a
 * Hyperliquid builder-deployed perp DEX named "xyz" for synthetic / equity
 * markets (xyz:SP500, xyz:SKHX, …) and the main dex for crypto majors
 * (BTC, ETH, …). We resolve which one a symbol belongs to from the xyz dex's
 * `allMids` coin set, then subscribe to the matching `l2Book` feed.
 *
 * Real data only — no synthetic depth.
 */

const HL_INFO = 'https://api.hyperliquid.xyz/info'
const HL_WS = 'wss://api.hyperliquid.xyz/ws'
const DEPTH = 11 // levels per side

interface Level {
  px: number
  sz: number
}
interface BookState {
  coin: string
  bids: Level[]
  asks: Level[]
}

function baseSymbol(raw: string): string {
  return raw
    .toUpperCase()
    .replace(/^XYZ:/, '')
    .replace(/(USDT|USDC|USD)$/, '')
}

// Resolve a base symbol to the Hyperliquid coin id. Members of the xyz dex get
// the "xyz:" prefix; everything else is treated as a main-dex coin.
function resolveCoin(base: string, xyzSet: Set<string>): string {
  if (!base) return ''
  return xyzSet.has(base) ? `xyz:${base}` : base
}

function fmtPx(px: number): string {
  if (px >= 1000) return px.toLocaleString('en-US', { maximumFractionDigits: 1 })
  if (px >= 1) return px.toLocaleString('en-US', { maximumFractionDigits: 3 })
  return px.toLocaleString('en-US', { maximumFractionDigits: 5 })
}
function fmtSz(sz: number): string {
  if (sz >= 1000) return `${(sz / 1000).toFixed(1)}k`
  if (sz >= 1) return sz.toFixed(2)
  return sz.toFixed(3)
}

interface OrderBookProps {
  /** raw business symbol (e.g. position symbol or candidate coin) */
  symbol: string
  /** optional entry price to mark the user's position level on the ladder */
  markPrice?: number
  /** showcase mode — drive a fast synthetic book instead of the live WS feed */
  demo?: boolean
}

export function OrderBook({ symbol, markPrice, demo = false }: OrderBookProps) {
  const base = useMemo(() => baseSymbol(symbol || ''), [symbol])
  const [xyzSet, setXyzSet] = useState<Set<string>>(new Set())
  const [book, setBook] = useState<BookState | null>(null)
  const [status, setStatus] = useState<'connecting' | 'live' | 'down'>('connecting')

  // one-time: fetch the xyz dex coin universe so we can resolve symbols
  useEffect(() => {
    let alive = true
    fetch(HL_INFO, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'allMids', dex: 'xyz' }),
    })
      .then((r) => r.json())
      .then((mids: Record<string, string>) => {
        if (!alive) return
        const set = new Set<string>()
        for (const k of Object.keys(mids || {})) set.add(k.replace(/^xyz:/, '').toUpperCase())
        setXyzSet(set)
      })
      .catch(() => {
        /* CORS / offline — fall back to main-dex resolution (empty set) */
      })
    return () => {
      alive = false
    }
  }, [])

  const coin = useMemo(() => resolveCoin(base, xyzSet), [base, xyzSet])

  // synthetic showcase feed — keeps price levels stable for stretches (so each
  // row flashes independently as its size changes) with a gentle upward drift.
  useEffect(() => {
    if (!demo || !base) return
    const seed = demoSeedPrice(base)
    const tickSz = demoTick(seed)
    const dp = tickSz < 1 ? 3 : 1
    let mid = seed
    let frame = 0
    const mkSizes = () =>
      Array.from({ length: DEPTH }, (_, i) => +(rnd(0.4, 6) * (1 + i * 0.12)).toFixed(3))
    const askSz = mkSizes()
    const bidSz = mkSizes()
    const emit = () => {
      const asks: Level[] = askSz.map((sz, i) => ({ px: +(mid + tickSz * (i + 1)).toFixed(dp), sz }))
      const bids: Level[] = bidSz.map((sz, i) => ({ px: +(mid - tickSz * (i + 1)).toFixed(dp), sz }))
      setBook({ coin: `xyz:${base}`, bids, asks })
    }
    setStatus('live')
    emit()
    const id = setInterval(() => {
      frame++
      const n = 2 + Math.floor(Math.random() * 3)
      for (let k = 0; k < n; k++) {
        const arr = Math.random() < 0.5 ? askSz : bidSz
        const i = Math.floor(Math.random() * DEPTH)
        arr[i] = +Math.max(0.05, arr[i] * rnd(0.6, 1.5)).toFixed(3)
      }
      // gentle mean-reverting wiggle around the seed (NO unbounded drift, so the
      // order book stays aligned with the cost/liq map + candle over a long run)
      if (frame % 5 === 0) {
        mid = +(mid + (seed - mid) * 0.3 + (Math.random() - 0.5) * tickSz * 2).toFixed(dp)
      }
      emit()
    }, 130)
    return () => clearInterval(id)
  }, [demo, base])

  // live L2 stream
  const pending = useRef<BookState | null>(null)
  useEffect(() => {
    if (!coin || demo) return
    let ws: WebSocket | null = null
    let raf: number | null = null
    let retry: ReturnType<typeof setTimeout> | null = null
    let closed = false

    const connect = () => {
      setStatus('connecting')
      ws = new WebSocket(HL_WS)
      ws.onopen = () => {
        ws?.send(JSON.stringify({ method: 'subscribe', subscription: { type: 'l2Book', coin } }))
      }
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.channel !== 'l2Book' || !msg.data) return
          const lv = msg.data.levels
          if (!Array.isArray(lv) || lv.length < 2) return
          const toLevels = (arr: { px: string; sz: string }[]): Level[] =>
            arr.slice(0, DEPTH).map((l) => ({ px: parseFloat(l.px), sz: parseFloat(l.sz) }))
          pending.current = { coin: msg.data.coin, bids: toLevels(lv[0]), asks: toLevels(lv[1]) }
          setStatus('live')
        } catch {
          /* ignore malformed frame */
        }
      }
      ws.onclose = () => {
        if (closed) return
        setStatus('down')
        retry = setTimeout(connect, 1500)
      }
      ws.onerror = () => ws?.close()
    }

    connect()
    // flush on every animation frame (~16ms) so individual rows tick the instant
    // the venue pushes a change — millisecond-level, not a batched interval
    const loop = () => {
      if (pending.current) {
        setBook(pending.current)
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
        ws?.send(JSON.stringify({ method: 'unsubscribe', subscription: { type: 'l2Book', coin } }))
      } catch {
        /* socket already gone */
      }
      ws?.close()
    }
  }, [coin])

  const view = useMemo(() => {
    if (!book) return null
    const asks = book.asks.slice(0, DEPTH)
    const bids = book.bids.slice(0, DEPTH)
    // cumulative depth for background bars
    let ca = 0
    const askRows = asks.map((l) => ({ ...l, cum: (ca += l.sz) }))
    let cb = 0
    const bidRows = bids.map((l) => ({ ...l, cum: (cb += l.sz) }))
    const maxCum = Math.max(ca, cb, 1)
    const bestAsk = asks[0]?.px ?? 0
    const bestBid = bids[0]?.px ?? 0
    const mid = bestAsk && bestBid ? (bestAsk + bestBid) / 2 : 0
    const spread = bestAsk && bestBid ? bestAsk - bestBid : 0
    const spreadBps = mid ? (spread / mid) * 10000 : 0
    // buy/sell pressure across the visible book (by notional)
    const bidVol = bidRows.reduce((s, l) => s + l.sz * l.px, 0)
    const askVol = askRows.reduce((s, l) => s + l.sz * l.px, 0)
    const bidPct = bidVol + askVol > 0 ? (bidVol / (bidVol + askVol)) * 100 : 50
    // the single visible level nearest the user's entry — marked with ▸
    let markLevel: number | undefined
    if (markPrice) {
      let bd = Infinity
      for (const l of [...asks, ...bids]) {
        const d = Math.abs(l.px - markPrice)
        if (d < bd) {
          bd = d
          markLevel = l.px
        }
      }
    }
    return { askRows: askRows.reverse(), bidRows, maxCum, mid, spread, spreadBps, bidPct, markLevel }
  }, [book, markPrice])

  const rowH = 16

  return (
    <div style={{ fontFamily: 'var(--tm-mono)' }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
        <span className="tm-px" style={{ fontSize: 11 }}>Order book</span>
        <span className="tm-sc">L2 · {coin || base || '—'}</span>
        <span
          className="tm-sc"
          style={{ marginLeft: 'auto', color: status === 'live' ? 'var(--tm-up)' : 'var(--tm-muted)' }}
        >
          {status === 'live' ? '● live' : status === 'connecting' ? '○ sync' : '○ down'}
        </span>
      </div>

      {!view ? (
        <div className="tm-sc" style={{ padding: '16px 0' }}>Connecting to Hyperliquid…</div>
      ) : (
        <div style={{ fontSize: 11 }}>
          {/* column header */}
          <div className="tm-sc" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 4, marginBottom: 2 }}>
            <span>price</span>
            <span style={{ textAlign: 'right' }}>size</span>
            <span style={{ textAlign: 'right' }}>cum $</span>
          </div>

          {/* asks (red), best ask nearest the mid — keyed by PRICE so each level
              keeps its identity and flashes independently when its size changes */}
          {view.askRows.map((l) => (
            <Row key={`a-${l.px}`} px={l.px} sz={l.sz} cum={l.cum} maxCum={view.maxCum} side="ask" h={rowH} mark={view.markLevel} />
          ))}

          {/* mid / spread */}
          <div
            className="tm-mono"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '3px 0',
              margin: '2px 0',
              borderTop: '1px solid var(--tm-hair)',
              borderBottom: '1px solid var(--tm-hair)',
            }}
          >
            <span className="tm-px" style={{ fontSize: 12, color: 'var(--tm-red)' }}>{fmtPx(view.mid)}</span>
            <span className="tm-sc" style={{ marginLeft: 'auto' }}>spread {fmtPx(view.spread)} · {view.spreadBps.toFixed(1)}bps</span>
          </div>

          {/* bids (green) — keyed by price, same independent-flash behavior */}
          {view.bidRows.map((l) => (
            <Row key={`b-${l.px}`} px={l.px} sz={l.sz} cum={l.cum} maxCum={view.maxCum} side="bid" h={rowH} mark={view.markLevel} />
          ))}

          {/* buy/sell pressure across the visible book */}
          <div style={{ marginTop: 7 }}>
            <div style={{ display: 'flex', height: 6 }}>
              <div style={{ width: `${view.bidPct}%`, background: 'var(--tm-up)', transition: 'width 0.2s ease-out' }} />
              <div style={{ flex: 1, background: 'var(--tm-dn)' }} />
            </div>
            <div className="tm-sc" style={{ display: 'flex', fontSize: 9, marginTop: 2 }}>
              <span className="tm-up">B {view.bidPct.toFixed(1)}%</span>
              <span style={{ marginLeft: 'auto' }} className="tm-dn">{(100 - view.bidPct).toFixed(1)}% S</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

interface RowProps {
  px: number
  sz: number
  cum: number
  maxCum: number
  side: 'ask' | 'bid'
  h: number
  mark?: number
}
function fmtNotional(n: number): string {
  if (n >= 1e9) return `$${(n / 1e9).toFixed(2)}B`
  if (n >= 1e6) return `$${(n / 1e6).toFixed(2)}M`
  if (n >= 1e3) return `$${(n / 1e3).toFixed(1)}K`
  return `$${n.toFixed(0)}`
}
function Row({ px, sz, cum, maxCum, side, h, mark }: RowProps) {
  const pct = Math.min(100, (cum / maxCum) * 100)
  const color = side === 'ask' ? 'var(--tm-dn)' : 'var(--tm-up)'
  // bold cumulative-depth bar, saturated toward the edge, that animates its
  // width as the book updates (the live "growing ladder" effect)
  const bar = side === 'ask'
    ? 'linear-gradient(to left, rgba(214,67,58,0.36), rgba(214,67,58,0.05))'
    : 'linear-gradient(to left, rgba(46,139,87,0.36), rgba(46,139,87,0.05))'
  const isMark = mark != null && px === mark

  // this Row instance is keyed by price, so these refs persist across updates —
  // we flash green/red only when THIS level's size actually changes, and keep
  // the direction class fixed until the next change so the animation isn't cut
  // short by the 60fps re-renders.
  const prevSz = useRef(sz)
  const dirRef = useRef('')
  if (sz !== prevSz.current) {
    dirRef.current = sz > prevSz.current ? 'ob-up' : 'ob-dn'
    prevSz.current = sz
  }

  return (
    <div style={{ position: 'relative', height: h, display: 'flex', alignItems: 'center' }}>
      {/* per-row flash overlay — keyed by size so it remounts (replays the
          animation) exactly when this level's size changes */}
      <div key={sz} className={dirRef.current} style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }} />
      <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: `${pct}%`, background: bar, transition: 'width 0.16s ease-out' }} />
      <div style={{ position: 'relative', display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 4, width: '100%', alignItems: 'center' }}>
        <span style={{ color, fontWeight: isMark ? 700 : 500 }}>
          {isMark ? '▸ ' : ''}{fmtPx(px)}
        </span>
        <span style={{ textAlign: 'right', color: 'var(--tm-ink)' }}>{fmtSz(sz)}</span>
        <span style={{ textAlign: 'right', color: 'var(--tm-muted)' }}>{fmtNotional(cum * px)}</span>
      </div>
    </div>
  )
}

export default OrderBook
