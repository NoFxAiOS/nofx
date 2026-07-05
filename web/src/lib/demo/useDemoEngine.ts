import { useEffect, useRef, useState } from 'react'
import type {
  AccountInfo,
  Position,
  DecisionRecord,
  SystemStatus,
  TraderFullStats,
  PositionHistoryResponse,
  HistoricalPosition,
  SymbolStats,
} from '../../types'
import type {
  FlowMarketsResponse,
  FlowMarketItem,
  SignalRankingResponse,
  SignalRankItem,
} from '../api/data'
import { DEMO_UNIVERSE, DEMO_ACTIVE_SYMBOL, demoSeedPrice } from './demoUniverse'

/**
 * useDemoEngine — a client-side showcase data generator. When `active`, it
 * synthesises a fast-evolving, profitable-looking US-equity trading dataset that
 * mirrors the real dashboard data shapes, so every panel animates for a product
 * walkthrough. Returns null when inactive (the dashboard then uses real data).
 *
 * No network, no backend, no real account — pure presentation layer.
 */

const TICK_MS = 200
const INITIAL = 1_000_000

export interface DemoDataset {
  status: SystemStatus
  account: AccountInfo
  positions: Position[]
  decisions: DecisionRecord[]
  fullStats: TraderFullStats
  history: PositionHistoryResponse
  config: {
    scan_interval_minutes: number
    ai_model: string
    strategy_name: string
    btc_eth_leverage: number
    altcoin_leverage: number
  }
  flow: FlowMarketsResponse
  signalRank: SignalRankingResponse
  activeSymbol: string
}

interface PosState {
  symbol: string
  side: 'long' | 'short'
  entry: number
  mark: number
  qty: number
  lev: number
}
interface TradeState {
  id: number
  symbol: string
  side: 'long' | 'short'
  entry: number
  exit: number
  qty: number
  pnl: number
  fee: number
  lev: number
}

interface SimState {
  frame: number
  cycle: number
  realized: number
  fee: number
  wins: number
  losses: number
  grossWin: number
  grossLoss: number
  nextId: number
  decisionTs: number
  positions: PosState[]
  trades: TradeState[]
  // per-symbol flow noise so the bars jiggle independently
  flowNet: Record<string, number>
  signalScore: Record<string, number>
}

const rnd = (a: number, b: number) => a + Math.random() * (b - a)
const pick = <T,>(arr: T[]): T => arr[Math.floor(Math.random() * arr.length)]

// A fixed "short book" — these symbols are short EVERYWHERE (flow outflow,
// bearish signal, short positions, decision candidates) so the topology's
// short row carries connected flow lines through every layer. The rest are long.
const SHORT_SET = new Set([
  'INTC', 'SPCX', 'SKHX', 'SMCI', 'ARM', 'QCOM', 'COIN', 'ORCL', 'DRAM', 'CRM', 'HOOD', 'SNOW',
])
const LONG_POOL = DEMO_UNIVERSE.filter((s) => !SHORT_SET.has(s))

function sideFor(symbol: string): 'long' | 'short' {
  return SHORT_SET.has(symbol.toUpperCase()) ? 'short' : 'long'
}

function newPosition(symbol: string): PosState {
  const entry = demoSeedPrice(symbol) * rnd(0.985, 1.015)
  const side = sideFor(symbol)
  const lev = pick([10, 10, 15, 20, 20])
  const notional = rnd(35_000, 110_000)
  return { symbol, side, entry, mark: entry, qty: notional / entry, lev }
}

function initState(): SimState {
  // index 0 = lead (drives the price panels). Then a deep US-equity book — a
  // long bias plus a sizable short book so the topology fills both rows densely.
  const longs = [DEMO_ACTIVE_SYMBOL, 'NVDA', 'GOOGL', 'TSM', 'META', 'AMD', 'MSFT', 'AMZN', 'AVGO']
  const shorts = ['INTC', 'SPCX', 'SKHX', 'SMCI', 'ARM', 'QCOM']
  const positions = [...longs, ...shorts].map((s) => {
    const p = newPosition(s)
    // start each slightly in profit so the board opens green
    const fav = rnd(0.006, 0.03)
    p.mark = p.side === 'long' ? p.entry * (1 + fav) : p.entry * (1 - fav)
    return p
  })
  const flowNet: Record<string, number> = {}
  const signalScore: Record<string, number> = {}
  DEMO_UNIVERSE.forEach((s, i) => {
    flowNet[s] = rnd(120_000, 2_400_000) * (1 - i / (DEMO_UNIVERSE.length + 4))
    // short book scores negative (bearish), everything else positive (bullish)
    signalScore[s] = SHORT_SET.has(s) ? rnd(-1.6, -0.4) : rnd(0.4, 2.0)
  })
  return {
    frame: 0,
    cycle: 1,
    realized: 312_000,
    fee: 2840,
    wins: 412,
    losses: 214,
    grossWin: 690_000,
    grossLoss: 286_000,
    nextId: 5000,
    decisionTs: Date.now(),
    positions,
    trades: [],
    flowNet,
    signalScore,
  }
}

function upnl(p: PosState): number {
  const notional = p.qty * p.entry
  const move = (p.mark - p.entry) / p.entry
  return notional * move * (p.side === 'long' ? 1 : -1)
}

function step(S: SimState) {
  S.frame++

  // drift each position's mark with a favourable bias so the book trends green
  for (const p of S.positions) {
    const bias = p.side === 'long' ? 0.00028 : -0.00028
    p.mark *= 1 + bias + rnd(-0.0011, 0.0011)
    // keep within a believable favourable band of entry
    const move = (p.mark - p.entry) / p.entry
    const dir = p.side === 'long' ? 1 : -1
    const signed = move * dir
    if (signed < -0.014) p.mark = p.entry * (1 - dir * 0.014)
    if (signed > 0.075) p.mark = p.entry * (1 + dir * 0.075)
  }

  // roll a winning trade every ~16 frames. Only rotate a non-lead LONG slot:
  // index 0 (lead, drives price panels) and the short book stay fixed so the
  // order book stays on one symbol and the topology keeps its short flow lines.
  const rotatable = S.positions
    .map((p, i) => ({ p, i }))
    .filter((x) => x.i !== 0 && x.p.side === 'long')
  if (S.frame % 16 === 0 && rotatable.length) {
    let bestIdx = rotatable[0].i
    for (const x of rotatable) {
      if (upnl(x.p) > upnl(S.positions[bestIdx])) bestIdx = x.i
    }
    const isWin = Math.random() < 0.7
    const closed = S.positions[bestIdx]
    const raw = upnl(closed)
    const pnl = isWin ? Math.max(Math.abs(raw), rnd(600, 5200)) : -rnd(250, 2100)
    const fee = rnd(12, 95)
    S.trades.unshift({
      id: S.nextId++,
      symbol: closed.symbol,
      side: closed.side,
      entry: closed.entry,
      exit: closed.mark,
      qty: closed.qty,
      pnl,
      fee,
      lev: closed.lev,
    })
    if (S.trades.length > 40) S.trades.pop()
    S.realized += pnl - fee
    S.fee += fee
    if (pnl >= 0) {
      S.wins++
      S.grossWin += pnl
    } else {
      S.losses++
      S.grossLoss += -pnl
    }
    // reopen a fresh LONG US-equity position (avoid duplicates of current book)
    const held = new Set(S.positions.map((p) => p.symbol))
    const candidates = LONG_POOL.filter((s) => !held.has(s))
    S.positions[bestIdx] = newPosition(candidates.length ? pick(candidates) : closed.symbol)
  }

  // new orchestration cycle every ~24 frames
  if (S.frame % 24 === 0) {
    S.cycle++
    S.decisionTs = Date.now()
  }

  // jiggle flow + signal noise so those panels stay alive (short book stays
  // bearish, longs stay bullish — keeps signal/topology directions consistent)
  for (const s of DEMO_UNIVERSE) {
    S.flowNet[s] = Math.max(20_000, S.flowNet[s] * rnd(0.97, 1.035))
    const next = S.signalScore[s] + rnd(-0.08, 0.09)
    S.signalScore[s] = SHORT_SET.has(s)
      ? Math.max(-2, Math.min(-0.1, next))
      : Math.max(0.1, Math.min(2.4, next))
  }
}

function build(S: SimState): DemoDataset {
  const liveUpnl = S.positions.reduce((s, p) => s + upnl(p), 0)
  const equity = INITIAL + S.realized + liveUpnl
  const pnl = equity - INITIAL
  const marginUsed = S.positions.reduce((s, p) => s + (p.qty * p.entry) / p.lev, 0)
  const total = S.wins + S.losses

  const account = {
    total_equity: equity,
    wallet_balance: equity - liveUpnl,
    unrealized_profit: liveUpnl,
    total_unrealized_profit: liveUpnl, // RiskRadar reads this extra field
    available_balance: Math.max(0, equity - marginUsed),
    total_pnl: pnl,
    total_pnl_pct: (pnl / INITIAL) * 100,
    initial_balance: INITIAL,
    daily_pnl: S.realized + liveUpnl,
    position_count: S.positions.length,
    margin_used: marginUsed,
    margin_used_pct: Math.min(82, (marginUsed / equity) * 100 * 1.6),
  } as unknown as AccountInfo

  const positions: Position[] = S.positions.map((p) => {
    const u = upnl(p)
    const notional = p.qty * p.entry
    const margin = notional / p.lev
    const liq = p.side === 'long' ? p.entry * (1 - 0.9 / p.lev) : p.entry * (1 + 0.9 / p.lev)
    return {
      symbol: p.symbol,
      side: p.side,
      entry_price: p.entry,
      mark_price: p.mark,
      quantity: p.qty,
      leverage: p.lev,
      unrealized_pnl: u,
      unrealized_pnl_pct: (u / margin) * 100,
      liquidation_price: liq,
      margin_used: margin,
    }
  })

  const winRate = total > 0 ? (S.wins / total) * 100 : 0
  const avgWin = S.wins > 0 ? S.grossWin / S.wins : 0
  const avgLoss = S.losses > 0 ? S.grossLoss / S.losses : 0
  const fullStats: TraderFullStats = {
    total_trades: total,
    win_trades: S.wins,
    loss_trades: S.losses,
    win_rate: winRate,
    profit_factor: S.grossLoss > 0 ? S.grossWin / S.grossLoss : S.grossWin,
    sharpe_ratio: 2.05 + Math.sin(S.frame / 90) * 0.12,
    total_pnl: pnl,
    total_fee: S.fee,
    avg_win: avgWin,
    avg_loss: avgLoss,
    // percent semantics (5.2 = -5.2%), matching the real stats endpoint
    max_drawdown_pct: 5.2,
  }

  // recent closed trades
  const histPositions = S.trades.map(
    (t) =>
      ({
        id: t.id,
        symbol: t.symbol,
        side: t.side,
        quantity: t.qty,
        entry_price: t.entry,
        exit_price: t.exit,
        exit_time: new Date(S.decisionTs - (S.nextId - t.id) * 47_000).toISOString(),
        realized_pnl: t.pnl,
        fee: t.fee,
        leverage: t.lev,
        status: 'closed',
        close_reason: t.pnl >= 0 ? 'take_profit' : 'stop_loss',
      }) as unknown as HistoricalPosition,
  )

  // per-symbol aggregates
  const bySym = new Map<string, { n: number; w: number; pnl: number }>()
  for (const t of S.trades) {
    const e = bySym.get(t.symbol) || { n: 0, w: 0, pnl: 0 }
    e.n++
    if (t.pnl >= 0) e.w++
    e.pnl += t.pnl
    bySym.set(t.symbol, e)
  }
  const symbolStats: SymbolStats[] = [...bySym.entries()]
    .map(([symbol, e]) => ({
      symbol,
      total_trades: e.n,
      win_trades: e.w,
      win_rate: e.n > 0 ? (e.w / e.n) * 100 : 0,
      total_pnl: e.pnl,
      avg_pnl: e.n > 0 ? e.pnl / e.n : 0,
      avg_hold_mins: Math.round(rnd(6, 38)),
    }))
    .sort((a, b) => b.total_trades - a.total_trades)

  const history = {
    positions: histPositions,
    stats: null,
    symbol_stats: symbolStats,
    direction_stats: [],
  } as unknown as PositionHistoryResponse

  // flow markets — long names show net BUYING (inflow), the short book shows net
  // SELLING (outflow) so the topology's FLOW layer feeds the short row too.
  const mkItem = (s: string, net: number): FlowMarketItem => {
    const buyShare = net >= 0 ? 0.56 + Math.min(0.3, net / 6_000_000) : 0.4
    const gross = Math.abs(net)
    return {
      key: `xyz:${s}`,
      marketType: 'hip3_perp',
      symbol: s,
      netFlow: String(Math.round(net)),
      buyNotional: String(Math.round(gross * buyShare)),
      sellNotional: String(Math.round(gross * (1 - buyShare))),
      trades: Math.round(rnd(150, 9000)),
      latestPrice: String(demoSeedPrice(s)),
    }
  }
  const inflow: FlowMarketItem[] = LONG_POOL.slice()
    .sort((a, b) => S.flowNet[b] - S.flowNet[a])
    .map((s) => mkItem(s, S.flowNet[s]))
  const outflow: FlowMarketItem[] = [...SHORT_SET].map((s) => mkItem(s, -Math.abs(S.flowNet[s]) * 0.6))
  const flow: FlowMarketsResponse = { data: { by: 'netFlow', window: '1h', inflow, outflow } }

  // signal ranking — mostly bullish US equities
  const ranked = [...DEMO_UNIVERSE].sort((a, b) => S.signalScore[b] - S.signalScore[a])
  const items: SignalRankItem[] = ranked.map((s, i) => {
    const score = S.signalScore[s]
    return {
      rank: i + 1,
      symbol: s,
      market_type: 'hip3_perp',
      bias: SHORT_SET.has(s) ? 'bearish' : 'bullish',
      score: Math.round(score * 100) / 100,
      category: 'us_equity',
    }
  })
  const signalRank: SignalRankingResponse = { items }

  // decision candidates — top longs plus the short book, so the DECISION layer
  // (and execution log) carries shorts through to EXECUTE/HOLD.
  const candidates = [...new Set([...ranked.slice(0, 8), ...SHORT_SET])]
  const decisions: DecisionRecord[] = Array.from({ length: 4 }).map((_, k) => {
    const cyc = S.cycle - k
    const acts = S.positions.slice(0, 6).map((p) => ({
      action: 'hold',
      symbol: p.symbol,
      quantity: p.qty,
      leverage: p.lev,
      price: p.mark,
      confidence: Math.round(rnd(62, 88)),
      reasoning: 'Signal Lab confirms trend; cost/liq structure supports the level.',
      timestamp: new Date(S.decisionTs - k * 300_000).toISOString(),
    }))
    return {
      timestamp: new Date(S.decisionTs - k * 300_000).toISOString(),
      cycle_number: cyc,
      system_prompt: '',
      input_prompt: '',
      cot_trace:
        'US-equity tape is broadly bid: SP500 and semis (NVDA, MU, TSM) lead net inflow with bullish Signal Lab bias. Holding winners ≥ entry, trimming only on structure breaks.',
      decision_json: '',
      account_state: {} as never,
      positions: [],
      candidate_coins: candidates,
      decisions: acts as never,
      execution_log: S.positions
        .slice(0, 6)
        .map((p) => `${p.symbol} hold succeeded`),
      success: true,
    } as unknown as DecisionRecord
  })

  const status = {
    is_running: true,
    call_count: S.cycle,
    scan_interval: '5m',
    ai_model: 'claw402',
    strategy_type: 'ai_trading',
  } as unknown as SystemStatus

  return {
    status,
    account,
    positions,
    decisions,
    fullStats,
    history,
    config: {
      scan_interval_minutes: 5,
      ai_model: 'claw402',
      strategy_name: 'NOFX Claw402 Auto Strategy',
      btc_eth_leverage: 10,
      altcoin_leverage: 10,
    },
    flow,
    signalRank,
    activeSymbol: DEMO_ACTIVE_SYMBOL,
  }
}

export function useDemoEngine(active: boolean): DemoDataset | null {
  const ref = useRef<SimState | null>(null)
  const [, tick] = useState(0)

  useEffect(() => {
    if (!active) {
      ref.current = null
      return
    }
    ref.current = initState()
    tick((n) => n + 1)
    const id = setInterval(() => {
      if (ref.current) {
        step(ref.current)
        tick((n) => n + 1)
      }
    }, TICK_MS)
    return () => clearInterval(id)
  }, [active])

  if (!active || !ref.current) return null
  return build(ref.current)
}
