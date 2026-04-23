import { API_BASE, httpClient } from './helpers'

export interface HotCoinItem {
  symbol: string
  score: number
  volume_24h: number
  oi: number
  price_change_24h: number
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
}
