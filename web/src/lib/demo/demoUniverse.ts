/**
 * Shared constants for the dashboard demo/showcase mode. US-equity-led synthetic
 * universe (these are real xyz-dex markets so the cost/liq heatmap still resolves)
 * plus plausible seed prices for the synthetic order book / candle feeds.
 *
 * This drives the "Demo" presentation mode only. It never touches the backend,
 * the live trader, or any real account — it is purely a client-side animation
 * layer for product walkthroughs.
 */

export const DEMO_UNIVERSE = [
  // longs (bullish book)
  'SP500', 'NVDA', 'MU', 'GOOGL', 'TSM', 'META', 'AMD', 'AAPL', 'MSFT', 'AMZN',
  'NFLX', 'TSLA', 'AVGO', 'NBIS', 'XYZ100', 'PLTR', 'TXN', 'LRCX', 'AMAT', 'MRVL',
  // short book (bearish names — see SHORT_SET in the engine)
  'INTC', 'SPCX', 'SKHX', 'SMCI', 'ARM', 'QCOM', 'COIN', 'ORCL', 'DRAM', 'CRM',
  'HOOD', 'SNOW',
]

// Lead instrument for the price panels (real heatmap + resolvable book/candles).
export const DEMO_ACTIVE_SYMBOL = 'SP500'

const DEMO_SEED_PX: Record<string, number> = {
  SP500: 6485, NVDA: 184.2, MU: 142.6, GOOGL: 351.8, TSM: 451.3, META: 723.5,
  AMD: 168.4, AAPL: 245.9, MSFT: 498.2, AMZN: 228.4, NFLX: 921.5, TSLA: 412.8,
  AVGO: 358.1, NBIS: 266.1, XYZ100: 1182, PLTR: 78.4, TXN: 205.3, LRCX: 102.6,
  AMAT: 218.7, MRVL: 118.2, INTC: 41.2, SPCX: 64.3, SKHX: 88.7, SMCI: 44.1,
  ARM: 162.4, QCOM: 178.9, COIN: 312.5, ORCL: 192.3, DRAM: 72.4, CRM: 342.1,
  HOOD: 58.6, SNOW: 198.4,
}

/** Stable plausible seed price for a base symbol (deterministic fallback). */
export function demoSeedPrice(base: string): number {
  const b = base.toUpperCase().replace(/^XYZ:/, '')
  if (DEMO_SEED_PX[b]) return DEMO_SEED_PX[b]
  let h = 0
  for (let i = 0; i < b.length; i++) h = (h * 31 + b.charCodeAt(i)) % 100000
  return 40 + (h % 900)
}

/** Reasonable tick size for a price level given its magnitude. */
export function demoTick(px: number): number {
  if (px >= 1000) return 1
  if (px >= 100) return 0.1
  if (px >= 10) return 0.05
  return 0.01
}
