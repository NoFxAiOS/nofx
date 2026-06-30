import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderFullStats,
  CompetitionData,
  PositionHistoryResponse,
} from '../../types'
import { API_BASE, httpClient } from './helpers'

export interface Kline {
  openTime: number
  open: number
  high: number
  low: number
  close: number
  volume: number
  closeTime: number
}

// Vergex net-flow market ranking (GET /api/vergex/flow-markets). Numeric fields
// arrive as strings from the upstream API.
export interface FlowMarketItem {
  key: string
  marketType: string
  symbol: string
  netFlow: string
  buyNotional: string
  sellNotional: string
  trades: number
  latestPrice: string
}
export interface FlowMarketsResponse {
  data?: {
    by?: string
    window?: string
    inflow?: FlowMarketItem[]
    outflow?: FlowMarketItem[]
  }
}

// Vergex signal ranking item (GET /api/vergex/signal-ranking).
export interface SignalRankItem {
  rank: number
  symbol: string
  market_type: string
  bias: string // "bullish" | "bearish" | "neutral"
  score: number
  category?: string
}
export interface SignalRankingResponse {
  items?: SignalRankItem[]
}

export interface MarketSymbol {
  symbol: string
  display?: string
  name: string
  category: 'crypto' | 'stock' | 'forex' | 'commodity' | 'index' | string
  exchange?: string
  volume_24h?: number
  mark_price?: number
  change_24h_pct?: number
  prev_day_price?: number
  maxLeverage?: number
  sz_decimals?: number
}

export interface SymbolListResponse {
  exchange: string
  symbols: MarketSymbol[]
  count: number
}

export interface VergexSignalItem {
  rank?: number
  symbol: string
  market_type?: string
  bias?: string
  confidence?: number
  score?: number
  category?: string
}

export interface VergexSignalRankingResponse {
  items: VergexSignalItem[]
  raw?: unknown
}

export interface VergexDetailRequest {
  marketType: string
  symbol: string
  chain?: string
  liqBand?: string
}

export interface VergexSignalDimension {
  key?: string
  family?: string
  label?: string
  what?: string
  kind?: string
  direction?: string
  strength?: string
  percentile?: number
  detail?: string
}

export interface VergexSignalLevels {
  markPrice?: number
  poc?: number
  pocDistPct?: number
  magnet?: number
  magnetDistPct?: number
  resistance?: number
  resistanceDistPct?: number
  support?: number
  supportDistPct?: number
  valueAreaHigh?: number
  valueAreaLow?: number
}

export interface VergexSignalMetrics {
  shortLiqAbove?: number
  longLiqBelow?: number
  longOverhangPnl?: number
  shortOverhangPnl?: number
  gLong?: number
  gShort?: number
  cascadeVulnPct?: number
  top10Pct?: number
  convexity?: number
  includedPositions?: number
  state?: string
}

export interface VergexSignalLabData {
  market?: {
    chain?: string
    marketType?: string
    marketId?: string
    symbol?: string
    displayName?: string
    isActive?: boolean
  }
  band?: string
  bias?: string
  structureRead?: string
  confidence?: string
  dimensions?: VergexSignalDimension[]
  levels?: VergexSignalLevels
  metrics?: VergexSignalMetrics
  compositeZ?: number
  rank?: number
  universeSize?: number
}

export interface VergexSignalLabResponse {
  data?: VergexSignalLabData
  meta?: unknown
}

export interface VergexHeatmapBin {
  px?: number
  bucketStartPrice?: number
  bucketEndPrice?: number
  longCost?: number
  shortCost?: number
  longLiq?: number
  shortLiq?: number
}

export interface VergexHeatmapData {
  market?: {
    chain?: string
    marketType?: string
    marketId?: string
    symbol?: string
    displayName?: string
    isActive?: boolean
  }
  markPrice?: number
  binStep?: number
  costAddrs?: number
  liqAddrs?: number
  bins?: VergexHeatmapBin[]
  cost?: {
    state?: string
    reason?: string
    totalPositions?: number
    includedPositions?: number
    excludedPositions?: number
    weightSource?: string
  }
  liquidation?: {
    state?: string
    reason?: string
  }
}

export interface VergexHeatmapResponse {
  data?: VergexHeatmapData
  meta?: unknown
}

function vergexDetailQuery(params: VergexDetailRequest) {
  const query = new URLSearchParams()
  query.set('marketType', params.marketType)
  query.set('symbol', params.symbol)
  query.set('chain', params.chain || 'mainnet')
  query.set('liqBand', params.liqBand || '15')
  return query.toString()
}

export const dataApi = {
  async getSymbols(exchange = 'hyperliquid-xyz'): Promise<SymbolListResponse> {
    const result = await httpClient.get<SymbolListResponse>(
      `${API_BASE}/symbols?exchange=${encodeURIComponent(exchange)}`
    )
    if (!result.success) throw new Error('Failed to fetch symbol list')
    return result.data || { exchange, symbols: [], count: 0 }
  },

  async getVergexSignalRanking(
    limit = 30
  ): Promise<VergexSignalRankingResponse> {
    const result = await httpClient.get<VergexSignalRankingResponse>(
      `${API_BASE}/vergex/signal-ranking?marketType=all&limit=${limit}`
    )
    if (!result.success)
      throw new Error('Failed to fetch Claw402/Vergex signal ranking')
    return result.data || { items: [] }
  },

  async getVergexSignalLab(
    params: VergexDetailRequest
  ): Promise<VergexSignalLabResponse> {
    const result = await httpClient.request<VergexSignalLabResponse>(
      `${API_BASE}/vergex/signal-lab?${vergexDetailQuery(params)}`,
      { timeout: 90000 }
    )
    if (!result.success)
      throw new Error(result.message || 'Failed to fetch Signal Lab')
    return result.data || {}
  },

  async getVergexCostLiquidationHeatmap(
    params: VergexDetailRequest
  ): Promise<VergexHeatmapResponse> {
    const result = await httpClient.request<VergexHeatmapResponse>(
      `${API_BASE}/vergex/cost-liquidation-heatmap?${vergexDetailQuery(params)}`,
      { timeout: 90000 }
    )
    if (!result.success)
      throw new Error(
        result.message || 'Failed to fetch cost/liquidation heatmap'
      )
    return result.data || {}
  },

  async getStatus(traderId?: string, silent?: boolean): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.request<SystemStatus>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch system status')
    return result.data!
  },

  async getAccount(traderId?: string, silent?: boolean): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.request<AccountInfo>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch account info')
    return result.data!
  },

  async getPositions(traderId?: string, silent?: boolean): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.request<Position[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch positions')
    return result.data!
  },

  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('Failed to fetch decision logs')
    return result.data!
  },

  async getLatestDecisions(
    traderId?: string,
    limit: number = 5,
    silent?: boolean
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.request<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch latest decisions')
    return result.data!
  },

  async getStatistics(
    traderId?: string,
    silent?: boolean
  ): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.request<Statistics>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch statistics')
    return result.data!
  },

  async getFullStats(
    traderId?: string,
    silent?: boolean
  ): Promise<TraderFullStats> {
    const url = traderId
      ? `${API_BASE}/statistics/full?trader_id=${traderId}`
      : `${API_BASE}/statistics/full`
    const result = await httpClient.request<TraderFullStats>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch full statistics')
    return result.data!
  },

  async getKlines(
    symbol: string,
    interval = '5m',
    exchange = 'hyperliquid',
    limit = 60,
    silent?: boolean
  ): Promise<Kline[]> {
    const params = new URLSearchParams({
      symbol,
      interval,
      exchange,
      limit: String(limit),
    })
    const result = await httpClient.request<Kline[]>(
      `${API_BASE}/klines?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch klines')
    return result.data!
  },

  async getFlowMarkets(
    aiModelId?: string,
    chain = 'mainnet',
    window = '1h',
    limit = 25,
    silent?: boolean
  ): Promise<FlowMarketsResponse> {
    const params = new URLSearchParams({ chain, window, limit: String(limit) })
    if (aiModelId) params.set('ai_model_id', aiModelId)
    const result = await httpClient.request<FlowMarketsResponse>(
      `${API_BASE}/vergex/flow-markets?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch flow markets')
    return result.data!
  },

  async getSignalRanking(
    aiModelId?: string,
    chain = 'mainnet',
    marketType = 'all',
    limit = 25,
    silent?: boolean
  ): Promise<SignalRankingResponse> {
    const params = new URLSearchParams({ chain, marketType, limit: String(limit) })
    if (aiModelId) params.set('ai_model_id', aiModelId)
    const result = await httpClient.request<SignalRankingResponse>(
      `${API_BASE}/vergex/signal-ranking?${params}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch signal ranking')
    return result.data!
  },

  async getEquityHistory(traderId?: string, silent?: boolean): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.request<any[]>(url, { silent })
    if (!result.success) throw new Error('Failed to fetch equity history')
    return result.data!
  },

  async getEquityHistoryBatch(
    traderIds: string[],
    hours?: number
  ): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds, hours: hours || 0 }
    )
    if (!result.success) throw new Error('Failed to fetch batch equity history')
    return result.data!
  },

  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('Failed to fetch top traders')
    return result.data!
  },

  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/traders/${traderId}/public-config`
    )
    if (!result.success) throw new Error('Failed to fetch public trader config')
    return result.data!
  },

  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('Failed to fetch competition data')
    return result.data!
  },

  async getPositionHistory(
    traderId: string,
    limit: number = 100,
    silent?: boolean
  ): Promise<PositionHistoryResponse> {
    const result = await httpClient.request<PositionHistoryResponse>(
      `${API_BASE}/positions/history?trader_id=${traderId}&limit=${limit}`,
      { silent }
    )
    if (!result.success) throw new Error('Failed to fetch position history')
    return result.data!
  },
}
