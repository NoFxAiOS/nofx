import { create } from 'zustand'
import type { TraderConfigData } from '../types'

interface TradersModalState {
  // Modal 显示状态
  showCreateModal: boolean
  showEditModal: boolean
  showModelModal: boolean
  showExchangeModal: boolean

  // 编辑状态
  editingModel: string | null
  editingExchange: string | null
  editingTrader: TraderConfigData | null

  // Actions
  setShowCreateModal: (show: boolean) => void
  setShowEditModal: (show: boolean) => void
  setShowModelModal: (show: boolean) => void
  setShowExchangeModal: (show: boolean) => void

  setEditingModel: (modelId: string | null) => void
  setEditingExchange: (exchangeId: string | null) => void
  setEditingTrader: (trader: TraderConfigData | null) => void

  // 便捷方法
  openModelModal: (modelId?: string) => void
  closeModelModal: () => void
  openExchangeModal: (exchangeId?: string) => void
  closeExchangeModal: () => void

  // 重置
  reset: () => void
}

const initialState = {
  showCreateModal: false,
  showEditModal: false,
  showModelModal: false,
  showExchangeModal: false,
  editingModel: null,
  editingExchange: null,
  editingTrader: null,
}

export const useTradersModalStore = create<TradersModalState>((set) => ({
  ...initialState,

  setShowCreateModal: (show) => set({ showCreateModal: show }),
  setShowEditModal: (show) => set({ showEditModal: show }),
  setShowModelModal: (show) => set({ showModelModal: show }),
  setShowExchangeModal: (show) => set({ showExchangeModal: show }),

  setEditingModel: (modelId) => set({ editingModel: modelId }),
  setEditingExchange: (exchangeId) => set({ editingExchange: exchangeId }),
  setEditingTrader: (trader) => set({ editingTrader: trader }),

  openModelModal: (modelId) => {
    set({ editingModel: modelId || null, showModelModal: true })
  },

  closeModelModal: () => {
    set({ showModelModal: false, editingModel: null })
  },

  openExchangeModal: (exchangeId) => {
    set({ editingExchange: exchangeId || null, showExchangeModal: true })
  },

  closeExchangeModal: () => {
    set({ showExchangeModal: false, editingExchange: null })
  },

  reset: () => set(initialState),
}))
