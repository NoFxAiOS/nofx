import { create } from 'zustand'
import type { AIModel, Exchange } from '../types'
import { api } from '../lib/api'

interface TradersConfigState {
  // 数据
  allModels: AIModel[]
  allExchanges: Exchange[]
  supportedModels: AIModel[]
  supportedExchanges: Exchange[]

  // 计算属性
  configuredModels: AIModel[]
  configuredExchanges: Exchange[]

  // Actions
  setAllModels: (models: AIModel[]) => void
  setAllExchanges: (exchanges: Exchange[]) => void
  setSupportedModels: (models: AIModel[]) => void
  setSupportedExchanges: (exchanges: Exchange[]) => void

  // 异步加载
  loadConfigs: (user: any, token: string | null) => Promise<void>

  // 重置
  reset: () => void
}

const initialState = {
  allModels: [],
  allExchanges: [],
  supportedModels: [],
  supportedExchanges: [],
  configuredModels: [],
  configuredExchanges: [],
}

export const useTradersConfigStore = create<TradersConfigState>((set, get) => ({
  ...initialState,

  setAllModels: (models) => {
    set({ allModels: models })
    // 更新 configuredModels
    const configuredModels = models.filter((m) => {
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    })
    set({ configuredModels })
  },

  setAllExchanges: (exchanges) => {
    set({ allExchanges: exchanges })
    // 更新 configuredExchanges
    const configuredExchanges = exchanges.filter((e) => {
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      // 修复: 添加 enabled 判断,与原始逻辑保持一致
      return e.enabled || (e.apiKey && e.apiKey.trim() !== '')
    })
    set({ configuredExchanges })
  },

  setSupportedModels: (models) => set({ supportedModels: models }),
  setSupportedExchanges: (exchanges) => set({ supportedExchanges: exchanges }),

  loadConfigs: async (user, token) => {
    if (!user || !token) {
      // 未登录时只加载公开的支持模型和交易所
      try {
        const [supportedModels, supportedExchanges] = await Promise.all([
          api.getSupportedModels(),
          api.getSupportedExchanges(),
        ])
        get().setSupportedModels(supportedModels)
        get().setSupportedExchanges(supportedExchanges)
      } catch (err) {
        console.error('Failed to load supported configs:', err)
      }
      return
    }

    try {
      const [
        modelConfigs,
        exchangeConfigs,
        supportedModels,
        supportedExchanges,
      ] = await Promise.all([
        api.getModelConfigs(),
        api.getExchangeConfigs(),
        api.getSupportedModels(),
        api.getSupportedExchanges(),
      ])

      get().setAllModels(modelConfigs)
      get().setAllExchanges(exchangeConfigs)
      get().setSupportedModels(supportedModels)
      get().setSupportedExchanges(supportedExchanges)
    } catch (error) {
      console.error('Failed to load configs:', error)
    }
  },

  reset: () => set(initialState),
}))
