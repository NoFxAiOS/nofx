import type {
  DecisionRecord,
  BacktestRunsResponse,
  BacktestStartConfig,
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestRunMetadata,
  BacktestKlinesResponse,
} from '../../types'
import { API_BASE, getAuthHeaders, handleJSONResponse } from './helpers'

export const backtestApi = {
  async getBacktestRuns(params?: {
    state?: string
    search?: string
    limit?: number
    offset?: number
  }): Promise<BacktestRunsResponse> {
    const query = new URLSearchParams()
    if (params?.state) query.set('state', params.state)
    if (params?.search) query.set('search', params.search)
    if (params?.limit) query.set('limit', String(params.limit))
    if (params?.offset) query.set('offset', String(params.offset))
    const res = await fetch(
      `${API_BASE}/backtest/runs${query.toString() ? `?${query}` : ''}`,
      {
        headers: getAuthHeaders(),
      }
    )
    return handleJSONResponse<BacktestRunsResponse>(res)
  },

  async startBacktest(config: BacktestStartConfig): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/start`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ config }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async pauseBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/pause`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async resumeBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/resume`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async stopBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/stop`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async updateBacktestLabel(
    runId: string,
    label: string
  ): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/label`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId, label }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async deleteBacktestRun(runId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/backtest/delete`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    if (!res.ok) {
      throw new Error(await res.text())
    }
  },

  async getBacktestStatus(runId: string): Promise<BacktestStatusPayload> {
    const res = await fetch(`${API_BASE}/backtest/status?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestStatusPayload>(res)
  },

  async getBacktestEquity(
    runId: string,
    timeframe?: string,
    limit?: number
  ): Promise<BacktestEquityPoint[]> {
    const query = new URLSearchParams({ run_id: runId })
    if (timeframe) query.set('tf', timeframe)
    if (limit) query.set('limit', String(limit))
    const res = await fetch(`${API_BASE}/backtest/equity?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestEquityPoint[]>(res)
  },

  async getBacktestTrades(
    runId: string,
    limit = 200
  ): Promise<BacktestTradeEvent[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
    })
    const res = await fetch(`${API_BASE}/backtest/trades?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestTradeEvent[]>(res)
  },

  async getBacktestMetrics(runId: string): Promise<BacktestMetrics> {
    const res = await fetch(`${API_BASE}/backtest/metrics?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestMetrics>(res)
  },

  async getBacktestKlines(
    runId: string,
    symbol: string,
    timeframe?: string
  ): Promise<BacktestKlinesResponse> {
    const query = new URLSearchParams({ run_id: runId, symbol })
    if (timeframe) query.set('timeframe', timeframe)
    const res = await fetch(`${API_BASE}/backtest/klines?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestKlinesResponse>(res)
  },

  async getBacktestTrace(
    runId: string,
    cycle?: number
  ): Promise<DecisionRecord> {
    const query = new URLSearchParams({ run_id: runId })
    if (cycle) query.set('cycle', String(cycle))
    const res = await fetch(`${API_BASE}/backtest/trace?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord>(res)
  },

  async getBacktestDecisions(
    runId: string,
    limit = 20,
    offset = 0
  ): Promise<DecisionRecord[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
      offset: String(offset),
    })
    const res = await fetch(`${API_BASE}/backtest/decisions?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord[]>(res)
  },

  async exportBacktest(runId: string): Promise<Blob> {
    const res = await fetch(`${API_BASE}/backtest/export?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) {
      const text = await res.text()
      try {
        const data = text ? JSON.parse(text) : null
        throw new Error(
          data?.error || data?.message || text || '导出失败，请稍后再试'
        )
      } catch (err) {
        if (err instanceof Error && err.message) {
          throw err
        }
        throw new Error(text || '导出失败，请稍后再试')
      }
    }
    return res.blob()
  },
}
