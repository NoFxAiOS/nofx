import { API_BASE, httpClient } from './helpers'

export interface CandidateQuality {
  passed?: boolean
  reasons?: string[]
  liquidity_score?: number
  open_interest_score?: number
  activity_score?: number
  momentum_score?: number
  reliability_score?: number
  tradability_score?: number
  risk_penalty?: number
}

export interface HotCoinItem {
  symbol: string
  score: number
  tradability_score?: number
  volume_24h: number
  oi: number
  oi_change_pct?: number
  oi_change_window_seconds?: number
  oi_source?: string
  price_change_24h: number
  source?: string
  quality?: CandidateQuality
}

export interface HotCoinResponse {
  coins: HotCoinItem[]
  updated_at: string
  exchange: string
}

export interface CoinDataResponse {
  symbol: string
  current_price: number
  price_change_1h: number
  price_change_4h: number
  funding_rate: number
  long_short_ratio?: number
  top_trader_ratio?: number
  taker_buy_sell_ratio?: number
  depth_bid_total?: number
  depth_ask_total?: number
  depth_imbalance?: number
  fibonacci_levels?: {
    swing_high: number
    swing_low: number
    timeframe: string
    levels: Record<string, number>
    direction: string
  }
  structural_levels?: {
    price: number
    type: string
    timeframe: string
    strength: number
    source: string
  }[]
  open_interest?: {
    Latest: number
    Average: number
  }
}

export interface CompositeMarketLine {
  id: string
  price: number
  kind: string
  label: string
  timeframe?: string
  strength?: number
  source?: string
  distance_pct?: number
}

export interface CompositeMarketTimeframe {
  timeframe: string
  klines?: Array<{ time: number; open: number; high: number; low: number; close: number; volume: number }>
  ema20?: number[]
  ema50?: number[]
  rsi14?: number[]
  atr14?: number
  lines?: CompositeMarketLine[]
}

export interface CompositeMarketSnapshot {
  symbol: string
  exchange: string
  primary_timeframe: string
  updated_at: string
  expires_at: string
  ttl_seconds: number
  stale?: boolean
  price: number
  price_change_1h: number
  price_change_4h: number
  data_quality?: string
  sources?: Array<{ name: string; available: boolean; reason?: string; updated_at?: string }>
  context?: unknown
  timeframes?: Record<string, CompositeMarketTimeframe>
  lines?: CompositeMarketLine[]
  ai_compact?: string
}

export const marketApi = {
  async getHotCoins(limit = 20, exchange = 'binance', excluded?: string[]): Promise<HotCoinResponse> {
    const params = new URLSearchParams({ limit: String(limit), exchange })
    if (excluded?.length) params.set('excluded', excluded.join(','))
    const result = await httpClient.get<HotCoinResponse>(`${API_BASE}/market/hot-coins?${params}`)
    if (!result.success) throw new Error('Failed to fetch hot coins')
    return result.data!
  },

  async getOIRanking(direction: 'top' | 'low' = 'top', limit = 20, excluded?: string[]): Promise<HotCoinResponse> {
    const params = new URLSearchParams({ direction, limit: String(limit) })
    if (excluded?.length) params.set('excluded', excluded.join(','))
    const result = await httpClient.get<HotCoinResponse>(`${API_BASE}/market/oi-ranking?${params}`)
    if (!result.success) throw new Error('Failed to fetch OI ranking')
    return result.data!
  },

  async getCoinData(symbol: string): Promise<CoinDataResponse> {
    const result = await httpClient.get<CoinDataResponse>(`${API_BASE}/market/coin-data?symbol=${encodeURIComponent(symbol)}`)
    if (!result.success) throw new Error('Failed to fetch coin data')
    return result.data!
  },

  async getCompositeMarket(symbol: string, exchange = 'okx', ttl = 180, view: 'summary' | 'chart' | 'ai' | 'full' = 'chart'): Promise<CompositeMarketSnapshot> {
    const params = new URLSearchParams({ symbol, exchange, ttl: String(ttl), view })
    const result = await httpClient.get<CompositeMarketSnapshot>(`${API_BASE}/market/composite?${params}`)
    if (!result.success) throw new Error('Failed to fetch composite market snapshot')
    return result.data!
  },
}
