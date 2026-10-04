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

// Normalized Vergex direction-change leaderboard item.
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
}

export interface VergexDirectionLeaderboardItem {
  market?: { marketType?: string; marketId?: string; symbol?: string }
  symbol: string
  bias: string
  directionScore: number
  bullishCount: number
  bearishCount: number
  neutralCount: number
  factorDirections?: Record<string, string>
  markPrice: number
  rank: number
  oiRank: number
  factorAsOf?: number
}

export interface VergexDirectionLeaderboardResponse {
  band: number
  universeSize: number
  rankBy: string
  asOf: number
  items: VergexDirectionLeaderboardItem[]
}

export interface VergexDirectionCurrentResponse {
  source?: string
  symbol: string
  direction: string
  new_bias?: string
  stable_since_at?: number
  effective_at?: number
  confirmed_at?: number
  mark_price?: number
  reason?: Record<string, string>
  factor_as_of?: number
  latest_reversal?: VergexDirectionHistoryItem | null
  readiness?: {
    generation?: number
    cutoff_at?: number
    watermark_at?: number
    symbol_count?: number
  }
}

export interface VergexDirectionHistoryItem {
  symbol: string
  prev_bias?: string
  new_bias?: string
  mark_price?: number
  occurred_at?: number
  effective_at?: number
  is_reversal?: boolean
  confirmed_at?: number
  reason?: Record<string, string>
  prev_reason?: Record<string, string> | null
  next_mark_price?: number | null
  next_occurred_at?: number | null
  next_effective_at?: number | null
}

export interface VergexDirectionHistoryResponse {
  items: VergexDirectionHistoryItem[]
  pagination?: {
    current_page: number
    page_size: number
    total_pages: number
    total_items: number
  }
}

export interface VergexDetailRequest {
  marketType: string
  symbol: string
  chain?: string
  liqBand?: string
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

// ── Vergex holder win-rate matrix (GET /api/vergex/holder-winrate-map) ──
// Numeric notional fields arrive as strings from the upstream API.
export interface VergexWinrateSide {
  count: number
  notional: string
}
export interface VergexWinrateCell {
  row: number
  column: number
  long: VergexWinrateSide
  short: VergexWinrateSide
}
export interface VergexWinrateBucket {
  count: number
  notional: string
}
export interface VergexWinrateMapData {
  snapshotId?: string
  asOf?: string
  positionsAsOf?: string
  priceAsOf?: string
  historyMode?: string
  staleHistoryCount?: number
  coverage?: string
  markPrice?: string
  priceSource?: string
  metricVersion?: string
  minRoundTrips?: number
  total?: VergexWinrateBucket
  included?: VergexWinrateBucket
  excluded?: Record<string, VergexWinrateBucket>
  viewport?: {
    winMin: number
    winMax: number
    costMin: number
    costMax: number
  }
  winBins?: number
  costBins?: number
  costRange?: [string, string]
  cells?: VergexWinrateCell[]
  water?: {
    long?: VergexWinrateWaterSide
    short?: VergexWinrateWaterSide
  }
}
export interface VergexWinrateWaterSide {
  aboveWater?: VergexWinrateBucket & { largestNotional?: string }
  belowWater?: VergexWinrateBucket
  atCost?: VergexWinrateBucket
}
export interface VergexWinrateMapResponse {
  data?: VergexWinrateMapData
  meta?: unknown
}

// ── Address drilldown (GET /api/v1/vergex/holder-winrate-map/holders) ──
export interface VergexWinrateHolder {
  address: string
  side: string
  size: string
  entryPrice: string
  notional: string
  costRatio: string
  winRate: string
  roundTrips: number
  historyAsOf?: string
  historyStale?: boolean
}
export interface VergexWinrateHoldersResponse {
  data?: {
    snapshotId?: string
    minRoundTrips?: number
    total?: number
    nextOffset?: number | null
    items?: VergexWinrateHolder[]
  }
  meta?: unknown
}

export interface VergexWinrateRequest {
  marketType: string
  symbol: string
  chain?: string
  winMin?: number
  winMax?: number
  costMin?: number
  costMax?: number
  minRoundTrips?: number
}

export interface VergexWinrateHoldersRequest extends VergexWinrateRequest {
  snapshotId: string
  row: number
  rowEnd: number
  column: number
  columnEnd: number
  side?: 'long' | 'short'
  offset?: number
  limit?: number
}

function vergexWinrateQuery(params: VergexWinrateRequest) {
  const query = new URLSearchParams()
  query.set('marketType', params.marketType)
  query.set('symbol', params.symbol)
  query.set('chain', params.chain || 'mainnet')
  if (params.winMin != null) query.set('winMin', String(params.winMin))
  if (params.winMax != null) query.set('winMax', String(params.winMax))
  if (params.costMin != null) query.set('costMin', String(params.costMin))
  if (params.costMax != null) query.set('costMax', String(params.costMax))
  if (params.minRoundTrips && params.minRoundTrips > 1)
    query.set('minRoundTrips', String(params.minRoundTrips))
  return query
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

  async getVergexDirectionChangeLeaderboard(
    limit = 30
  ): Promise<VergexSignalRankingResponse> {
    const result = await httpClient.get<VergexDirectionLeaderboardResponse>(
      `${API_BASE}/vergex/direction-change/leaderboard`
    )
    if (!result.success)
      throw new Error('Failed to fetch Claw402/Vergex direction leaderboard')
    const items = (result.data?.items || []).slice(0, limit).map((item) => ({
      rank: item.rank,
      symbol: item.symbol,
      market_type: item.market?.marketType,
      bias: item.bias,
      score: item.directionScore,
      category: item.market?.marketType === 'core_perp' ? 'crypto' : undefined,
    }))
    return { items }
  },

  async getVergexDirectionChangeCurrent(
    symbol: string
  ): Promise<VergexDirectionCurrentResponse> {
    const query = new URLSearchParams({ symbol })
    const result = await httpClient.request<VergexDirectionCurrentResponse>(
      `${API_BASE}/vergex/direction-change/current?${query}`,
      { timeout: 90000 }
    )
    if (!result.success)
      throw new Error(result.message || 'Failed to fetch current direction')
    return result.data!
  },

  async getVergexDirectionChangeHistory(
    symbol: string,
    type: 'all' | 'reversal' | 'non_reversal' = 'all',
    page = 1,
    pageSize = 20
  ): Promise<VergexDirectionHistoryResponse> {
    const query = new URLSearchParams({
      symbol,
      type,
      page: String(page),
      page_size: String(pageSize),
    })
    const result = await httpClient.request<VergexDirectionHistoryResponse>(
      `${API_BASE}/vergex/direction-change/history?${query}`,
      { timeout: 90000 }
    )
    if (!result.success)
      throw new Error(result.message || 'Failed to fetch direction history')
    return result.data || { items: [] }
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

  async getVergexHolderWinrateMap(
    params: VergexWinrateRequest
  ): Promise<VergexWinrateMapResponse> {
    const result = await httpClient.request<VergexWinrateMapResponse>(
      `${API_BASE}/vergex/holder-winrate-map?${vergexWinrateQuery(params)}`,
      { timeout: 90000 }
    )
    if (!result.success)
      throw new Error(
        result.message || 'Failed to fetch holder win-rate matrix'
      )
    return result.data || {}
  },

  async getVergexHolderWinrateHolders(
    params: VergexWinrateHoldersRequest,
    silent?: boolean
  ): Promise<VergexWinrateHoldersResponse> {
    const query = vergexWinrateQuery(params)
    query.set('snapshotId', params.snapshotId)
    query.set('row', String(params.row))
    query.set('rowEnd', String(params.rowEnd))
    query.set('column', String(params.column))
    query.set('columnEnd', String(params.columnEnd))
    query.set('side', params.side || 'long')
    query.set('offset', String(params.offset ?? 0))
    query.set('limit', String(params.limit ?? 50))
    const result = await httpClient.request<VergexWinrateHoldersResponse>(
      `${API_BASE}/vergex/holder-winrate-map/holders?${query}`,
      { silent, timeout: 90000 }
    )
    if (!result.success)
      throw new Error(result.message || 'Failed to fetch win-rate holders')
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

  async getDirectionChangeLeaderboard(
    limit = 25,
    silent?: boolean
  ): Promise<SignalRankingResponse> {
    const result = await httpClient.request<VergexDirectionLeaderboardResponse>(
      `${API_BASE}/vergex/direction-change/leaderboard`,
      { silent }
    )
    if (!result.success)
      throw new Error('Failed to fetch direction leaderboard')
    return {
      items: (result.data?.items || []).slice(0, limit).map((item) => ({
        rank: item.rank,
        symbol: item.symbol,
        market_type: item.market?.marketType || '',
        bias: item.bias,
        score: item.directionScore,
        category:
          item.market?.marketType === 'core_perp' ? 'crypto' : undefined,
      })),
    }
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
