import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  TraderConfigData,
  AIModel,
  Exchange,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
  BacktestRunsResponse,
  BacktestStartConfig,
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestRunMetadata,
  Strategy,
  StrategyConfig,
} from '../types'
import { CryptoService } from './crypto'
import { httpClient } from './httpClient'

const API_BASE = '/api'

// Helper function to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  return headers
}

async function handleJSONResponse<T>(res: Response): Promise<T> {
  const text = await res.text()
  if (!res.ok) {
    let message = text || res.statusText
    try {
      const data = text ? JSON.parse(text) : null
      if (data && typeof data === 'object') {
        message = data.error || data.message || message
      }
    } catch {
      /* ignore JSON parse errors */
    }
    throw new Error(message || '请求失败')
  }
  if (!text) {
    return {} as T
  }
  return JSON.parse(text) as T
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    const result = await httpClient.get<TraderInfo[]>(`${API_BASE}/my-traders`)
    if (!result.success) throw new Error('获取trader列表失败')
    return result.data!
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/traders`)
    if (!result.success) throw new Error('获取公开trader列表失败')
    return result.data!
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const result = await httpClient.post<TraderInfo>(
      `${API_BASE}/traders`,
      request
    )
    if (!result.success) throw new Error('创建交易员失败')
    return result.data!
  },

  async deleteTrader(traderId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/traders/${traderId}`)
    if (!result.success) throw new Error('删除交易员失败')
  },

  async startTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(
      `${API_BASE}/traders/${traderId}/start`
    )
    if (!result.success) throw new Error('启动交易员失败')
  },

  async stopTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/traders/${traderId}/stop`)
    if (!result.success) throw new Error('停止交易员失败')
  },

  async closePosition(traderId: string, symbol: string, side: string): Promise<{ message: string }> {
    const result = await httpClient.post<{ message: string }>(
      `${API_BASE}/traders/${traderId}/close-position`,
      { symbol, side }
    )
    if (!result.success) throw new Error('平仓失败')
    return result.data!
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/prompt`,
      { custom_prompt: customPrompt }
    )
    if (!result.success) throw new Error('更新自定义策略失败')
  },

  async getTraderConfig(traderId: string): Promise<TraderConfigData> {
    const result = await httpClient.get<TraderConfigData>(
      `${API_BASE}/traders/${traderId}/config`
    )
    if (!result.success) throw new Error('获取交易员配置失败')
    return result.data!
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const result = await httpClient.put<TraderInfo>(
      `${API_BASE}/traders/${traderId}`,
      request
    )
    if (!result.success) throw new Error('更新交易员失败')
    return result.data!
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(`${API_BASE}/models`)
    if (!result.success) throw new Error('获取模型配置失败')
    return result.data!
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(
      `${API_BASE}/supported-models`
    )
    if (!result.success) throw new Error('获取支持的模型失败')
    return result.data!
  },

  async getPromptTemplates(): Promise<string[]> {
    const res = await fetch(`${API_BASE}/prompt-templates`)
    if (!res.ok) throw new Error('获取提示词模板失败')
    const data = await res.json()
    if (Array.isArray(data.templates)) {
      return data.templates.map((item: { name: string }) => item.name)
    }
    return []
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(`${API_BASE}/models`, encryptedPayload)
    if (!result.success) throw new Error('更新模型配置失败')
  },

  // 交易所配置接口
  async getExchangeConfigs(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(`${API_BASE}/exchanges`)
    if (!result.success) throw new Error('获取交易所配置失败')
    return result.data!
  },

  // 获取系统支持的交易所列表（无需认证）
  async getSupportedExchanges(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(
      `${API_BASE}/supported-exchanges`
    )
    if (!result.success) throw new Error('获取支持的交易所失败')
    return result.data!
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 使用加密传输更新交易所配置
  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const result = await httpClient.put(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('更新交易所配置失败')
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.get<SystemStatus>(url)
    if (!result.success) throw new Error('获取系统状态失败')
    return result.data!
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.get<AccountInfo>(url)
    if (!result.success) throw new Error('获取账户信息失败')
    console.log('Account data fetched:', result.data)
    return result.data!
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.get<Position[]>(url)
    if (!result.success) throw new Error('获取持仓列表失败')
    return result.data!
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('获取决策日志失败')
    return result.data!
  },

  // 获取最新决策（支持trader_id和limit参数）
  async getLatestDecisions(
    traderId?: string,
    limit: number = 5
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.get<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`
    )
    if (!result.success) throw new Error('获取最新决策失败')
    return result.data!
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.get<Statistics>(url)
    if (!result.success) throw new Error('获取统计信息失败')
    return result.data!
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.get<any[]>(url)
    if (!result.success) throw new Error('获取历史数据失败')
    return result.data!
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds }
    )
    if (!result.success) throw new Error('获取批量历史数据失败')
    return result.data!
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('获取前5名交易员失败')
    return result.data!
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/trader/${traderId}/config`
    )
    if (!result.success) throw new Error('获取公开交易员配置失败')
    return result.data!
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('获取竞赛数据失败')
    return result.data!
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.get<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
    if (!result.success) throw new Error('获取服务器IP失败')
    return result.data!
  },

  // Backtest APIs
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

  // Strategy APIs
  async getStrategies(): Promise<Strategy[]> {
    const result = await httpClient.get<Strategy[]>(`${API_BASE}/strategies`)
    if (!result.success) throw new Error('获取策略列表失败')
    return result.data!
  },

  async getStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(`${API_BASE}/strategies/${strategyId}`)
    if (!result.success) throw new Error('获取策略失败')
    return result.data!
  },

  async getActiveStrategy(): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(`${API_BASE}/strategies/active`)
    if (!result.success) throw new Error('获取激活策略失败')
    return result.data!
  },

  async getDefaultStrategyConfig(): Promise<StrategyConfig> {
    const result = await httpClient.get<StrategyConfig>(`${API_BASE}/strategies/default-config`)
    if (!result.success) throw new Error('获取默认策略配置失败')
    return result.data!
  },

  async createStrategy(data: {
    name: string
    description: string
    config: StrategyConfig
  }): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies`, data)
    if (!result.success) throw new Error('创建策略失败')
    return result.data!
  },

  async updateStrategy(
    strategyId: string,
    data: {
      name?: string
      description?: string
      config?: StrategyConfig
    }
  ): Promise<Strategy> {
    const result = await httpClient.put<Strategy>(`${API_BASE}/strategies/${strategyId}`, data)
    if (!result.success) throw new Error('更新策略失败')
    return result.data!
  },

  async deleteStrategy(strategyId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/strategies/${strategyId}`)
    if (!result.success) throw new Error('删除策略失败')
  },

  async activateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies/${strategyId}/activate`)
    if (!result.success) throw new Error('激活策略失败')
    return result.data!
  },

  async duplicateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies/${strategyId}/duplicate`)
    if (!result.success) throw new Error('复制策略失败')
    return result.data!
  },
}
