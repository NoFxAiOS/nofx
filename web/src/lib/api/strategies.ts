import type {
  Strategy,
  StrategyConfig,
} from '../../types'
import { API_BASE, httpClient } from './helpers'

export interface PublicStrategy {
  id: string
  name: string
  description?: string
  author_email?: string
  author_name?: string
  created_at: string
  updated_at?: string
  is_public?: boolean
  usage_count?: number
  is_popular?: boolean
  indicators?: string[]
  risk_level?: string
  config_visible?: boolean
  config?: unknown
  stats?: {
    used_by: number
    rating: number
  }
}

export const strategyApi = {
  async getStrategies(): Promise<Strategy[]> {
    const result = await httpClient.get<{ strategies: Strategy[] }>(`${API_BASE}/strategies`)
    if (!result.success) throw new Error('Failed to fetch strategy list')
    const strategies = result.data?.strategies
    return Array.isArray(strategies) ? strategies : []
  },

  async getStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(`${API_BASE}/strategies/${strategyId}`)
    if (!result.success) throw new Error('Failed to fetch strategy')
    return result.data!
  },

  async getActiveStrategy(): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(`${API_BASE}/strategies/active`)
    if (!result.success) throw new Error('Failed to fetch active strategy')
    return result.data!
  },

  async getDefaultStrategyConfig(): Promise<StrategyConfig> {
    const result = await httpClient.get<StrategyConfig>(`${API_BASE}/strategies/default-config`)
    if (!result.success) throw new Error('Failed to fetch default strategy config')
    return result.data!
  },

  async createStrategy(data: {
    name: string
    description: string
    config: StrategyConfig
  }): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies`, data)
    if (!result.success) throw new Error('Failed to create strategy')
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
    if (!result.success) throw new Error('Failed to update strategy')
    return result.data!
  },

  async deleteStrategy(strategyId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/strategies/${strategyId}`)
    if (!result.success) throw new Error('Failed to delete strategy')
  },

  async activateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies/${strategyId}/activate`)
    if (!result.success) throw new Error('Failed to activate strategy')
    return result.data!
  },

  async duplicateStrategy(strategyId: string): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies/${strategyId}/duplicate`)
    if (!result.success) throw new Error('Failed to duplicate strategy')
    return result.data!
  },

  async getPublicStrategies(): Promise<PublicStrategy[]> {
    const result = await httpClient.get<{ strategies: PublicStrategy[] }>(`${API_BASE}/strategies/public`)
    if (!result.success) throw new Error('Failed to fetch strategies')
    const strategies = result.data?.strategies
    return Array.isArray(strategies) ? strategies : []
  },
}
