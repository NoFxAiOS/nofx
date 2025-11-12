import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  AIModel,
  Exchange,
} from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t, type Language } from '../i18n/translations'
import { useAuth } from '../contexts/AuthContext'
import { getExchangeIcon } from './ExchangeIcons'
import { getModelIcon } from './ModelIcons'
import { TraderConfigModal } from './TraderConfigModal'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from './TwoStageKeyModal'
import {
  Bot,
  Brain,
  Landmark,
  BarChart3,
  Trash2,
  Plus,
  Users,
  AlertTriangle,
  BookOpen,
  HelpCircle,
  Radio,
  Pencil,
} from 'lucide-react'
import { confirmToast } from '../lib/notify'
import { toast } from 'sonner'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [showSignalSourceModal, setShowSignalSourceModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [supportedExchanges, setSupportedExchanges] = useState<Exchange[]>([])
  const [userSignalSource, setUserSignalSource] = useState<{
    coinPoolUrl: string
    oiTopUrl: string
  }>({
    coinPoolUrl: '',
    oiTopUrl: '',
  })

  const { data: traders, mutate: mutateTraders } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  const { data: allModels, mutate: mutateModels } = useSWR<AIModel[]>(
    user && token ? 'models' : null,
    api.getModelConfigs
  )

  const { data: allExchanges, mutate: mutateExchanges } = useSWR<Exchange[]>(
    user && token ? 'exchanges' : null,
    api.getExchangeConfigs
  )

  // 加载支持的模型和交易所列表（用于选择）
  useEffect(() => {
    const loadSupportedConfigs = async () => {
      try {
        const [supportedModels, supportedExchanges] = await Promise.all([
          api.getSupportedModels(),
          api.getSupportedExchanges(),
        ])
        setSupportedModels(supportedModels)
        setSupportedExchanges(supportedExchanges)
      } catch (err) {
        console.error('Failed to load supported configs:', err)
      }
    }
    loadSupportedConfigs()
  }, [])

  // 加载用户信号源配置
  useEffect(() => {
    const loadSignalSource = async () => {
      if (!user || !token) return

      try {
        const signalSource = await api.getUserSignalSource()
        setUserSignalSource({
          coinPoolUrl: signalSource.coin_pool_url || '',
          oiTopUrl: signalSource.oi_top_url || '',
        })
      } catch (error) {
        console.log('📡 用户信号源配置暂未设置')
      }
    }
    loadSignalSource()
  }, [user, token])

  // 只显示已配置的模型和交易所
  // 注意：后端返回的数据不包含敏感信息（apiKey等），所以通过其他字段判断是否已配置
  const configuredModels =
    allModels?.filter((m) => {
      // 如果模型已启用，说明已配置
      // 或者有自定义API URL，也说明已配置
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    }) || []
  const configuredExchanges =
    allExchanges?.filter((e) => {
      // Aster 交易所检查特殊字段
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      // Hyperliquid 需要检查钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      // 其他交易所：如果已启用，说明已配置（后端返回的已配置交易所会有 enabled: true）
      return e.enabled
    }) || []

  // 只在创建交易员时使用已启用且配置完整的
  // 注意：后端返回的数据不包含敏感信息，所以只检查 enabled 状态和必要的非敏感字段
  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false

      // Aster 交易所需要特殊字段（后端会返回这些非敏感字段）
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== ''
        )
      }

      // Hyperliquid 需要钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }

      // 其他交易所：如果已启用，说明已配置完整（后端只返回已配置的交易所）
      return true
    }) || []

  // 检查模型是否正在被运行中的交易员使用（用于UI禁用）
  const isModelInUse = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId && t.is_running)
  }

  // 检查交易所是否正在被运行中的交易员使用（用于UI禁用）
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId && t.is_running)
  }

  // 检查模型是否被任何交易员使用（包括停止状态的）
  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId) || false
  }

  // 检查交易所是否被任何交易员使用（包括停止状态的）
  const isExchangeUsedByAnyTrader = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId) || false
  }

  // 获取使用特定模型的交易员列表
  const getTradersUsingModel = (modelId: string) => {
    return traders?.filter((t) => t.ai_model === modelId) || []
  }

  // 获取使用特定交易所的交易员列表
  const getTradersUsingExchange = (exchangeId: string) => {
    return traders?.filter((t) => t.exchange_id === exchangeId) || []
  }

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels?.find((m) => m.id === data.ai_model_id)
      const exchange = allExchanges?.find((e) => e.id === data.exchange_id)

      if (!model?.enabled) {
        toast.error(t('modelNotConfigured', language))
        return
      }

      if (!exchange?.enabled) {
        toast.error(t('exchangeNotConfigured', language))
        return
      }

      await toast.promise(api.createTrader(data), {
        loading: '正在创建…',
        success: '创建成功',
        error: '创建失败',
      })
      setShowCreateModal(false)
      mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      toast.error(t('createTraderFailed', language))
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      toast.error(t('getTraderConfigFailed', language))
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    if (!editingTrader) return

    try {
      const model = enabledModels?.find((m) => m.id === data.ai_model_id)
      const exchange = enabledExchanges?.find((e) => e.id === data.exchange_id)

      if (!model) {
        toast.error(t('modelConfigNotExist', language))
        return
      }

      if (!exchange) {
        toast.error(t('exchangeConfigNotExist', language))
        return
      }

      const request = {
        name: data.name,
        ai_model_id: data.ai_model_id,
        exchange_id: data.exchange_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        btc_eth_leverage: data.btc_eth_leverage,
        altcoin_leverage: data.altcoin_leverage,
        trading_symbols: data.trading_symbols,
        custom_prompt: data.custom_prompt,
        override_base_prompt: data.override_base_prompt,
        is_cross_margin: data.is_cross_margin,
        use_coin_pool: data.use_coin_pool,
        use_oi_top: data.use_oi_top,
      }

      await toast.promise(api.updateTrader(editingTrader.trader_id, request), {
        loading: '正在保存…',
        success: '保存成功',
        error: '保存失败',
      })
      setShowEditModal(false)
      setEditingTrader(null)
      mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      toast.error(t('updateTraderFailed', language))
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    {
      const ok = await confirmToast(t('confirmDeleteTrader', language))
      if (!ok) return
    }

    try {
      await toast.promise(api.deleteTrader(traderId), {
        loading: '正在删除…',
        success: '删除成功',
        error: '删除失败',
      })
      mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      toast.error(t('deleteTraderFailed', language))
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await toast.promise(api.stopTrader(traderId), {
          loading: '正在停止…',
          success: '已停止',
          error: '停止失败',
        })
      } else {
        await toast.promise(api.startTrader(traderId), {
          loading: '正在启动…',
          success: '已启动',
          error: '启动失败',
        })
      }
      mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      toast.error(t('operationFailed', language))
    }
  }

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  // 通用删除配置处理函数
  const handleDeleteConfig = async <T extends { id: string }>(config: {
    id: string
    type: 'model' | 'exchange'
    checkInUse: (id: string) => boolean
    getUsingTraders: (id: string) => any[]
    cannotDeleteKey: string
    confirmDeleteKey: string
    allItems: T[] | undefined
    clearFields: (item: T) => T
    buildRequest: (items: T[]) => any
    updateApi: (request: any) => Promise<void>
    setItems: () => void
    closeModal: () => void
    errorKey: string
  }) => {
    // 检查是否有交易员正在使用
    if (config.checkInUse(config.id)) {
      const usingTraders = config.getUsingTraders(config.id)
      const traderNames = usingTraders.map((t) => t.trader_name).join(', ')
      toast.error(
        `${t(config.cannotDeleteKey, language)} · ${t('tradersUsing', language)}: ${traderNames} · ${t('pleaseDeleteTradersFirst', language)}`
      )
      return
    }

    {
      const ok = await confirmToast(t(config.confirmDeleteKey, language))
      if (!ok) return
    }

    try {
      const updatedItems =
        config.allItems?.map((item) =>
          item.id === config.id ? config.clearFields(item) : item
        ) || []

      const request = config.buildRequest(updatedItems)
      await toast.promise(config.updateApi(request), {
        loading: '正在更新配置…',
        success: '配置已更新',
        error: '更新配置失败',
      })

      // 使用 SWR mutate 自动刷新数据
      config.setItems()

      config.closeModal()
    } catch (error) {
      console.error(`Failed to delete ${config.type} config:`, error)
      toast.error(t(config.errorKey, language))
    }
  }

  const handleDeleteModelConfig = async (modelId: string) => {
    await handleDeleteConfig({
      id: modelId,
      type: 'model',
      checkInUse: isModelUsedByAnyTrader,
      getUsingTraders: getTradersUsingModel,
      cannotDeleteKey: 'cannotDeleteModelInUse',
      confirmDeleteKey: 'confirmDeleteModel',
      allItems: allModels,
      clearFields: (m) => ({
        ...m,
        apiKey: '',
        customApiUrl: '',
        customModelName: '',
        enabled: false,
      }),
      buildRequest: (models) => ({
        models: Object.fromEntries(
          models.map((model) => [
            model.provider,
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }),
      updateApi: api.updateModelConfigs,
      setItems: () => {
        // 自动刷新模型列表
        mutateModels()
      },
      closeModal: () => {
        setShowModelModal(false)
        setEditingModel(null)
      },
      errorKey: 'deleteConfigFailed',
    })
  }

  const handleSaveModelConfig = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      // 创建或更新用户的模型配置
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      // 找到要配置的模型（优先从已配置列表，其次从支持列表）
      const modelToUpdate =
        existingModel || supportedModels?.find((m) => m.id === modelId)
      if (!modelToUpdate) {
        toast.error(t('modelNotExist', language))
        return
      }

      if (existingModel) {
        // 更新现有配置
        updatedModels =
          allModels?.map((m) =>
            m.id === modelId
              ? {
                  ...m,
                  apiKey,
                  customApiUrl: customApiUrl || '',
                  customModelName: customModelName || '',
                  enabled: true,
                }
              : m
          ) || []
      } else {
        // 添加新配置
        const newModel = {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }
        updatedModels = [...(allModels || []), newModel]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((model) => [
            model.provider, // 使用 provider 而不是 id
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }

      await toast.promise(api.updateModelConfigs(request), {
        loading: '正在更新模型配置…',
        success: '模型配置已更新',
        error: '更新模型配置失败',
      })

      // 自动刷新模型列表
      mutateModels()

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    await handleDeleteConfig({
      id: exchangeId,
      type: 'exchange',
      checkInUse: isExchangeUsedByAnyTrader,
      getUsingTraders: getTradersUsingExchange,
      cannotDeleteKey: 'cannotDeleteExchangeInUse',
      confirmDeleteKey: 'confirmDeleteExchange',
      allItems: allExchanges,
      clearFields: (e) => ({
        ...e,
        apiKey: '',
        secretKey: '',
        hyperliquidWalletAddr: '',
        asterUser: '',
        asterSigner: '',
        asterPrivateKey: '',
        enabled: false,
      }),
      buildRequest: (exchanges) => ({
        exchanges: Object.fromEntries(
          exchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
            },
          ])
        ),
      }),
      updateApi: api.updateExchangeConfigsEncrypted,
      setItems: () => {
        // 自动刷新交易所列表
        mutateExchanges()
      },
      closeModal: () => {
        setShowExchangeModal(false)
        setEditingExchange(null)
      },
      errorKey: 'deleteExchangeConfigFailed',
    })
  }

  const handleSaveExchangeConfig = async (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string
  ) => {
    try {
      // 找到要配置的交易所（从supportedExchanges中）
      const exchangeToUpdate = supportedExchanges?.find(
        (e) => e.id === exchangeId
      )
      if (!exchangeToUpdate) {
        toast.error(t('exchangeNotExist', language))
        return
      }

      // 创建或更新用户的交易所配置
      const existingExchange = allExchanges?.find((e) => e.id === exchangeId)
      let updatedExchanges

      if (existingExchange) {
        // 更新现有配置
        updatedExchanges =
          allExchanges?.map((e) =>
            e.id === exchangeId
              ? {
                  ...e,
                  apiKey,
                  secretKey,
                  testnet,
                  hyperliquidWalletAddr,
                  asterUser,
                  asterSigner,
                  asterPrivateKey,
                  enabled: true,
                }
              : e
          ) || []
      } else {
        // 添加新配置
        const newExchange = {
          ...exchangeToUpdate,
          apiKey,
          secretKey,
          testnet,
          hyperliquidWalletAddr,
          asterUser,
          asterSigner,
          asterPrivateKey,
          enabled: true,
        }
        updatedExchanges = [...(allExchanges || []), newExchange]
      }

      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
            },
          ])
        ),
      }

      await toast.promise(api.updateExchangeConfigsEncrypted(request), {
        loading: '正在更新交易所配置…',
        success: '交易所配置已更新',
        error: '更新交易所配置失败',
      })

      // 自动刷新交易所列表
      mutateExchanges()

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  const handleSaveSignalSource = async (
    coinPoolUrl: string,
    oiTopUrl: string
  ) => {
    try {
      await toast.promise(api.saveUserSignalSource(coinPoolUrl, oiTopUrl), {
        loading: '正在保存…',
        success: '保存成功',
        error: '保存失败',
      })
      setUserSignalSource({ coinPoolUrl, oiTopUrl })
      setShowSignalSourceModal(false)
    } catch (error) {
      console.error('Failed to save signal source:', error)
      toast.error(t('saveSignalSourceFailed', language))
    }
  }

  return (
    <div className="space-y-4 md:space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 md:gap-0">
        <div className="flex items-center gap-3 md:gap-4">
          <div
            className="w-10 h-10 md:w-12 md:h-12 rounded-xl flex items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              boxShadow: '0 4px 14px rgba(240, 185, 11, 0.4)',
            }}
          >
            <Bot className="w-5 h-5 md:w-6 md:h-6" style={{ color: '#000' }} />
          </div>
          <div>
            <h1
              className="text-xl md:text-2xl font-bold flex items-center gap-2"
              style={{ color: '#EAECEF' }}
            >
              {t('aiTraders', language)}
              <span
                className="text-xs font-normal px-2 py-1 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.15)',
                  color: '#F0B90B',
                }}
              >
                {traders?.length || 0} {t('active', language)}
              </span>
            </h1>
            <p className="text-xs" style={{ color: '#848E9C' }}>
              {t('manageAITraders', language)}
            </p>
          </div>
        </div>

        <div className="flex gap-2 md:gap-3 w-full md:w-auto overflow-hidden flex-wrap md:flex-nowrap">
          <button
            onClick={handleAddModel}
            className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
            style={{
              background: '#2B3139',
              color: '#EAECEF',
              border: '1px solid #474D57',
            }}
          >
            <Plus className="w-3 h-3 md:w-4 md:h-4" />
            {t('aiModels', language)}
          </button>

          <button
            onClick={handleAddExchange}
            className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
            style={{
              background: '#2B3139',
              color: '#EAECEF',
              border: '1px solid #474D57',
            }}
          >
            <Plus className="w-3 h-3 md:w-4 md:h-4" />
            {t('exchanges', language)}
          </button>

          <button
            onClick={() => setShowSignalSourceModal(true)}
            className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
            style={{
              background: '#2B3139',
              color: '#EAECEF',
              border: '1px solid #474D57',
            }}
          >
            <Radio className="w-3 h-3 md:w-4 md:h-4" />
            {t('signalSource', language)}
          </button>

          <button
            onClick={() => setShowCreateModal(true)}
            disabled={
              configuredModels.length === 0 || configuredExchanges.length === 0
            }
            className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1 md:gap-2 whitespace-nowrap"
            style={{
              background:
                configuredModels.length > 0 && configuredExchanges.length > 0
                  ? '#F0B90B'
                  : '#2B3139',
              color:
                configuredModels.length > 0 && configuredExchanges.length > 0
                  ? '#000'
                  : '#848E9C',
            }}
          >
            <Plus className="w-4 h-4" />
            {t('createTrader', language)}
          </button>
        </div>
      </div>

      {/* 信号源配置警告 */}
      {traders &&
        traders.some((t) => t.use_coin_pool || t.use_oi_top) &&
        !userSignalSource.coinPoolUrl &&
        !userSignalSource.oiTopUrl && (
          <div
            className="rounded-lg px-4 py-3 flex items-start gap-3 animate-slide-in"
            style={{
              background: 'rgba(246, 70, 93, 0.1)',
              border: '1px solid rgba(246, 70, 93, 0.3)',
            }}
          >
            <AlertTriangle
              size={20}
              className="flex-shrink-0 mt-0.5"
              style={{ color: '#F6465D' }}
            />
            <div className="flex-1">
              <div className="font-semibold mb-1" style={{ color: '#F6465D' }}>
                ⚠️ {t('signalSourceNotConfigured', language)}
              </div>
              <div className="text-sm" style={{ color: '#848E9C' }}>
                <p className="mb-2">
                  {t('signalSourceWarningMessage', language)}
                </p>
                <p>
                  <strong>{t('solutions', language)}</strong>
                </p>
                <ul className="list-disc list-inside space-y-1 ml-2 mt-1">
                  <li>点击"{t('signalSource', language)}"按钮配置API地址</li>
                  <li>或在交易员配置中禁用"使用币种池"和"使用OI Top"</li>
                  <li>或在交易员配置中设置自定义币种列表</li>
                </ul>
              </div>
              <button
                onClick={() => setShowSignalSourceModal(true)}
                className="mt-3 px-3 py-1.5 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#F0B90B',
                  color: '#000',
                }}
              >
                {t('configureSignalSourceNow', language)}
              </button>
            </div>
          </div>
        )}

      {/* Configuration Status */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
        {/* AI Models */}
        <div className="binance-card p-3 md:p-4">
          <h3
            className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Brain
              className="w-4 h-4 md:w-5 md:h-5"
              style={{ color: '#60a5fa' }}
            />
            {t('aiModels', language)}
          </h3>
          <div className="space-y-2 md:space-y-3">
            {configuredModels.map((model) => {
              const inUse = isModelInUse(model.id)
              return (
                <div
                  key={model.id}
                  className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                    inUse
                      ? 'cursor-not-allowed'
                      : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleModelClick(model.id)}
                >
                  <div className="flex items-center gap-2 md:gap-3">
                    <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                      {getModelIcon(model.provider || model.id, {
                        width: 28,
                        height: 28,
                      }) || (
                        <div
                          className="w-7 h-7 md:w-8 md:h-8 rounded-full flex items-center justify-center text-xs md:text-sm font-bold"
                          style={{
                            background:
                              model.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                            color: '#fff',
                          }}
                        >
                          {getShortName(model.name)[0]}
                        </div>
                      )}
                    </div>
                    <div className="min-w-0">
                      <div
                        className="font-semibold text-sm md:text-base truncate"
                        style={{ color: '#EAECEF' }}
                      >
                        {getShortName(model.name)}
                      </div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {inUse
                          ? t('inUse', language)
                          : model.enabled
                            ? t('enabled', language)
                            : t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div
                    className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${model.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
                  />
                </div>
              )
            })}
            {configuredModels.length === 0 && (
              <div
                className="text-center py-6 md:py-8"
                style={{ color: '#848E9C' }}
              >
                <Brain className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
                <div className="text-xs md:text-sm">
                  {t('noModelsConfigured', language)}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Exchanges */}
        <div className="binance-card p-3 md:p-4">
          <h3
            className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Landmark
              className="w-4 h-4 md:w-5 md:h-5"
              style={{ color: '#F0B90B' }}
            />
            {t('exchanges', language)}
          </h3>
          <div className="space-y-2 md:space-y-3">
            {configuredExchanges.map((exchange) => {
              const inUse = isExchangeInUse(exchange.id)
              return (
                <div
                  key={exchange.id}
                  className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                    inUse
                      ? 'cursor-not-allowed'
                      : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleExchangeClick(exchange.id)}
                >
                  <div className="flex items-center gap-2 md:gap-3">
                    <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                      {getExchangeIcon(exchange.id, { width: 28, height: 28 })}
                    </div>
                    <div className="min-w-0">
                      <div
                        className="font-semibold text-sm md:text-base truncate"
                        style={{ color: '#EAECEF' }}
                      >
                        {getShortName(exchange.name)}
                      </div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {exchange.type.toUpperCase()} •{' '}
                        {inUse
                          ? t('inUse', language)
                          : exchange.enabled
                            ? t('enabled', language)
                            : t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div
                    className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${exchange.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
                  />
                </div>
              )
            })}
            {configuredExchanges.length === 0 && (
              <div
                className="text-center py-6 md:py-8"
                style={{ color: '#848E9C' }}
              >
                <Landmark className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
                <div className="text-xs md:text-sm">
                  {t('noExchangesConfigured', language)}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Traders List */}
      <div className="binance-card p-4 md:p-6">
        <div className="flex items-center justify-between mb-4 md:mb-5">
          <h2
            className="text-lg md:text-xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Users
              className="w-5 h-5 md:w-6 md:h-6"
              style={{ color: '#F0B90B' }}
            />
            {t('currentTraders', language)}
          </h2>
        </div>

        {traders && traders.length > 0 ? (
          <div className="space-y-3 md:space-y-4">
            {traders.map((trader) => (
              <div
                key={trader.trader_id}
                className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 md:gap-4">
                  <div
                    className="w-10 h-10 md:w-12 md:h-12 rounded-full flex items-center justify-center flex-shrink-0"
                    style={{
                      background: trader.ai_model.includes('deepseek')
                        ? '#60a5fa'
                        : '#c084fc',
                      color: '#fff',
                    }}
                  >
                    <Bot className="w-5 h-5 md:w-6 md:h-6" />
                  </div>
                  <div className="min-w-0">
                    <div
                      className="font-bold text-base md:text-lg truncate"
                      style={{ color: '#EAECEF' }}
                    >
                      {trader.trader_name}
                    </div>
                    <div
                      className="text-xs md:text-sm truncate"
                      style={{
                        color: trader.ai_model.includes('deepseek')
                          ? '#60a5fa'
                          : '#c084fc',
                      }}
                    >
                      {getModelDisplayName(
                        trader.ai_model.split('_').pop() || trader.ai_model
                      )}{' '}
                      Model • {trader.exchange_id?.toUpperCase()}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
                  {/* Status */}
                  <div className="text-center">
                    {/* <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                      {t('status', language)}
                    </div> */}
                    <div
                      className={`px-2 md:px-3 py-1 rounded text-xs font-bold ${
                        trader.is_running
                          ? 'bg-green-100 text-green-800'
                          : 'bg-red-100 text-red-800'
                      }`}
                      style={
                        trader.is_running
                          ? {
                              background: 'rgba(14, 203, 129, 0.1)',
                              color: '#0ECB81',
                            }
                          : {
                              background: 'rgba(246, 70, 93, 0.1)',
                              color: '#F6465D',
                            }
                      }
                    >
                      {trader.is_running
                        ? t('running', language)
                        : t('stopped', language)}
                    </div>
                  </div>

                  {/* Actions: 禁止换行，超出横向滚动 */}
                  <div className="flex gap-1.5 md:gap-2 flex-nowrap overflow-x-auto items-center">
                    <button
                      onClick={() => {
                        if (onTraderSelect) {
                          onTraderSelect(trader.trader_id)
                        } else {
                          navigate(`/dashboard?trader=${trader.trader_id}`)
                        }
                      }}
                      className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 whitespace-nowrap"
                      style={{
                        background: 'rgba(99, 102, 241, 0.1)',
                        color: '#6366F1',
                      }}
                    >
                      <BarChart3 className="w-3 h-3 md:w-4 md:h-4" />
                      {t('view', language)}
                    </button>

                    <button
                      onClick={() => handleEditTrader(trader.trader_id)}
                      disabled={trader.is_running}
                      className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap flex items-center gap-1"
                      style={{
                        background: trader.is_running
                          ? 'rgba(132, 142, 156, 0.1)'
                          : 'rgba(255, 193, 7, 0.1)',
                        color: trader.is_running ? '#848E9C' : '#FFC107',
                      }}
                    >
                      <Pencil className="w-3 h-3 md:w-4 md:h-4" />
                      {t('edit', language)}
                    </button>

                    <button
                      onClick={() =>
                        handleToggleTrader(
                          trader.trader_id,
                          trader.is_running || false
                        )
                      }
                      className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap"
                      style={
                        trader.is_running
                          ? {
                              background: 'rgba(246, 70, 93, 0.1)',
                              color: '#F6465D',
                            }
                          : {
                              background: 'rgba(14, 203, 129, 0.1)',
                              color: '#0ECB81',
                            }
                      }
                    >
                      {trader.is_running
                        ? t('stop', language)
                        : t('start', language)}
                    </button>

                    <button
                      onClick={() => handleDeleteTrader(trader.trader_id)}
                      className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
                      style={{
                        background: 'rgba(246, 70, 93, 0.1)',
                        color: '#F6465D',
                      }}
                    >
                      <Trash2 className="w-3 h-3 md:w-4 md:h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div
            className="text-center py-12 md:py-16"
            style={{ color: '#848E9C' }}
          >
            <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
            <div className="text-base md:text-lg font-semibold mb-2">
              {t('noTraders', language)}
            </div>
            <div className="text-xs md:text-sm mb-3 md:mb-4">
              {t('createFirstTrader', language)}
            </div>
            {(configuredModels.length === 0 ||
              configuredExchanges.length === 0) && (
              <div className="text-xs md:text-sm text-yellow-500">
                {configuredModels.length === 0 &&
                configuredExchanges.length === 0
                  ? t('configureModelsAndExchangesFirst', language)
                  : configuredModels.length === 0
                    ? t('configureModelsFirst', language)
                    : t('configureExchangesFirst', language)}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Create Trader Modal */}
      {showCreateModal && (
        <TraderConfigModal
          isOpen={showCreateModal}
          isEditMode={false}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleCreateTrader}
          onClose={() => setShowCreateModal(false)}
        />
      )}

      {/* Edit Trader Modal */}
      {showEditModal && editingTrader && (
        <TraderConfigModal
          isOpen={showEditModal}
          isEditMode={true}
          traderData={editingTrader}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleSaveEditTrader}
          onClose={() => {
            setShowEditModal(false)
            setEditingTrader(null)
          }}
        />
      )}

      {/* Model Configuration Modal */}
      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          configuredModels={allModels || []}
          editingModelId={editingModel}
          onSave={handleSaveModelConfig}
          onDelete={handleDeleteModelConfig}
          onClose={() => {
            setShowModelModal(false)
            setEditingModel(null)
          }}
          language={language}
        />
      )}

      {/* Exchange Configuration Modal */}
      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={supportedExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchangeConfig}
          onDelete={handleDeleteExchangeConfig}
          onClose={() => {
            setShowExchangeModal(false)
            setEditingExchange(null)
          }}
          language={language}
        />
      )}

      {/* Signal Source Configuration Modal */}
      {showSignalSourceModal && (
        <SignalSourceModal
          coinPoolUrl={userSignalSource.coinPoolUrl}
          oiTopUrl={userSignalSource.oiTopUrl}
          onSave={handleSaveSignalSource}
          onClose={() => setShowSignalSourceModal(false)}
          language={language}
        />
      )}
    </div>
  )
}

// Tooltip Helper Component
function Tooltip({
  content,
  children,
}: {
  content: string
  children: React.ReactNode
}) {
  const [show, setShow] = useState(false)

  return (
    <div className="relative inline-block">
      <div
        onMouseEnter={() => setShow(true)}
        onMouseLeave={() => setShow(false)}
        onClick={() => setShow(!show)}
      >
        {children}
      </div>
      {show && (
        <div
          className="absolute z-10 px-3 py-2 text-sm rounded-lg shadow-lg w-64 left-1/2 transform -translate-x-1/2 bottom-full mb-2"
          style={{
            background: '#2B3139',
            color: '#EAECEF',
            border: '1px solid #474D57',
          }}
        >
          {content}
          <div
            className="absolute left-1/2 transform -translate-x-1/2 top-full"
            style={{
              width: 0,
              height: 0,
              borderLeft: '6px solid transparent',
              borderRight: '6px solid transparent',
              borderTop: '6px solid #2B3139',
            }}
          />
        </div>
      )}
    </div>
  )
}

// Signal Source Configuration Modal Component
function SignalSourceModal({
  coinPoolUrl,
  oiTopUrl,
  onSave,
  onClose,
  language,
}: {
  coinPoolUrl: string
  oiTopUrl: string
  onSave: (coinPoolUrl: string, oiTopUrl: string) => void
  onClose: () => void
  language: Language
}) {
  const [coinPool, setCoinPool] = useState(coinPoolUrl || '')
  const [oiTop, setOiTop] = useState(oiTopUrl || '')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(coinPool.trim(), oiTop.trim())
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('signalSourceConfig', language)}
        </h3>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                COIN POOL URL
              </label>
              <input
                type="url"
                value={coinPool}
                onChange={(e) => setCoinPool(e.target.value)}
                placeholder="https://api.example.com/coinpool"
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {t('coinPoolDescription', language)}
              </div>
            </div>

            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                OI TOP URL
              </label>
              <input
                type="url"
                value={oiTop}
                onChange={(e) => setOiTop(e.target.value)}
                placeholder="https://api.example.com/oitop"
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {t('oiTopDescription', language)}
              </div>
            </div>

            <div
              className="p-4 rounded"
              style={{
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
            >
              <div
                className="text-sm font-semibold mb-2"
                style={{ color: '#F0B90B' }}
              >
                ℹ️ {t('information', language)}
              </div>
              <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                <div>{t('signalSourceInfo1', language)}</div>
                <div>{t('signalSourceInfo2', language)}</div>
                <div>{t('signalSourceInfo3', language)}</div>
              </div>
            </div>
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Model Configuration Modal Component
function ModelConfigModal({
  allModels,
  configuredModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allModels: AIModel[]
  configuredModels: AIModel[]
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string
  ) => void
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')

  // 获取当前编辑的模型信息 - 编辑时从已配置的模型中查找，新建时从所有支持的模型中查找
  const selectedModel = editingModelId
    ? configuredModels?.find((m) => m.id === selectedModelId)
    : allModels?.find((m) => m.id === selectedModelId)

  // 如果是编辑现有模型，初始化API Key、Base URL和Model Name
  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '')
      setBaseUrl(selectedModel.customApiUrl || '')
      setModelName(selectedModel.customModelName || '')
    }
  }, [editingModelId, selectedModel])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId || !apiKey.trim()) return

    onSave(
      selectedModelId,
      apiKey.trim(),
      baseUrl.trim() || undefined,
      modelName.trim() || undefined
    )
  }

  // 可选择的模型列表（所有支持的模型）
  const availableModels = allModels || []

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: '#1E2329' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId
              ? t('editAIModel', language)
              : t('addAIModel', language)}
          </h3>
          {editingModelId && (
            <button
              type="button"
              onClick={() => onDelete(editingModelId)}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title={t('delete', language)}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingModelId && (
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('selectModel', language)}
                </label>
                <select
                  value={selectedModelId}
                  onChange={(e) => setSelectedModelId(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                  required
                >
                  <option value="">{t('pleaseSelectModel', language)}</option>
                  {availableModels.map((model) => (
                    <option key={model.id} value={model.id}>
                      {getShortName(model.name)} ({model.provider})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {selectedModel && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getModelIcon(selectedModel.provider || selectedModel.id, {
                      width: 32,
                      height: 32,
                    }) || (
                      <div
                        className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                        style={{
                          background:
                            selectedModel.id === 'deepseek'
                              ? '#60a5fa'
                              : '#c084fc',
                          color: '#fff',
                        }}
                      >
                        {selectedModel.name[0]}
                      </div>
                    )}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedModel.name)}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedModel.provider} • {selectedModel.id}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {selectedModel && (
              <>
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    API Key
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={t('enterAPIKey', language)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {t('customBaseURL', language)}
                  </label>
                  <input
                    type="url"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder={t('customBaseURLPlaceholder', language)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {t('leaveBlankForDefault', language)}
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    Model Name (可选)
                  </label>
                  <input
                    type="text"
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder="例如: deepseek-chat, qwen3-max, gpt-5"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    留空使用默认模型名称
                  </div>
                </div>

                <div
                  className="p-4 rounded"
                  style={{
                    background: 'rgba(240, 185, 11, 0.1)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                >
                  <div
                    className="text-sm font-semibold mb-2"
                    style={{ color: '#F0B90B' }}
                  >
                    ℹ️ {t('information', language)}
                  </div>
                  <div
                    className="text-xs space-y-1"
                    style={{ color: '#848E9C' }}
                  >
                    <div>{t('modelConfigInfo1', language)}</div>
                    <div>{t('modelConfigInfo2', language)}</div>
                    <div>{t('modelConfigInfo3', language)}</div>
                  </div>
                </div>
              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!selectedModel || !apiKey.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Exchange Configuration Modal Component
function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(
    editingExchangeId || ''
  )
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)

  // 币安配置指南展开状态
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster 特定字段
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid 特定字段
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // 安全输入状态
  const [secureInputTarget, setSecureInputTarget] = useState<
    null | 'hyperliquid' | 'aster'
  >(null)

  // 获取当前编辑的交易所信息
  const selectedExchange = allExchanges?.find(
    (e) => e.id === selectedExchangeId
  )

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('') // Don't load existing passphrase for security
      setTestnet(selectedExchange.testnet || false)

      // Aster 字段
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('') // Don't load existing private key for security

      // Hyperliquid 字段
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')
    }
  }, [editingExchangeId, selectedExchange])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (selectedExchangeId === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [selectedExchangeId])

  const handleCopyIP = async (ip: string) => {
    try {
      // 优先使用现代 Clipboard API
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        // 降级方案: 使用传统的 execCommand 方法
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()

        try {
          const successful = document.execCommand('copy')
          if (successful) {
            setCopiedIP(true)
            setTimeout(() => setCopiedIP(false), 2000)
            toast.success(t('ipCopied', language))
          } else {
            throw new Error('复制命令执行失败')
          }
        } finally {
          document.body.removeChild(textArea)
        }
      }
    } catch (err) {
      console.error('复制失败:', err)
      // 显示错误提示
      toast.error(
        t('copyIPFailed', language) || `复制失败: ${ip}\n请手动复制此IP地址`
      )
    }
  }

  // 安全输入处理函数
  const secureInputContextLabel =
    secureInputTarget === 'aster'
      ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid'
        ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputCancel = () => {
    setSecureInputTarget(null)
  }

  const handleSecureInputComplete = ({
    value,
    obfuscationLog,
  }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') {
      setApiKey(trimmed)
    }
    if (secureInputTarget === 'aster') {
      setAsterPrivateKey(trimmed)
    }
    console.log('Secure input obfuscation log:', obfuscationLog)
    setSecureInputTarget(null)
  }

  // 掩盖敏感数据显示
  const maskSecret = (secret: string) => {
    if (!secret || secret.length === 0) return ''
    if (secret.length <= 8) return '*'.repeat(secret.length)
    return (
      secret.slice(0, 4) +
      '*'.repeat(Math.max(secret.length - 8, 4)) +
      secret.slice(-4)
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedExchangeId) return

    // 根据交易所类型验证不同字段
    if (selectedExchange?.id === 'binance') {
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    } else if (selectedExchange?.id === 'hyperliquid') {
      if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return // 验证私钥和钱包地址
      await onSave(
        selectedExchangeId,
        apiKey.trim(),
        '',
        testnet,
        hyperliquidWalletAddr.trim()
      )
    } else if (selectedExchange?.id === 'aster') {
      if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim())
        return
      await onSave(
        selectedExchangeId,
        '',
        '',
        testnet,
        undefined,
        asterUser.trim(),
        asterSigner.trim(),
        asterPrivateKey.trim()
      )
    } else if (selectedExchange?.id === 'okx') {
      if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    } else {
      // 默认情况（其他CEX交易所）
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    }
  }

  // 可选择的交易所列表（所有支持的交易所）
  const availableExchanges = allExchanges || []

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: '#1E2329' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {selectedExchange?.id === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  color: '#F0B90B',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('delete', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingExchangeId && (
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('selectExchange', language)}
                </label>
                <select
                  value={selectedExchangeId}
                  onChange={(e) => setSelectedExchangeId(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                  required
                >
                  <option value="">
                    {t('pleaseSelectExchange', language)}
                  </option>
                  {availableExchanges.map((exchange) => (
                    <option key={exchange.id} value={exchange.id}>
                      {getShortName(exchange.name)} (
                      {exchange.type.toUpperCase()})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {selectedExchange && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getExchangeIcon(selectedExchange.id, {
                      width: 32,
                      height: 32,
                    })}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedExchange.name)}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedExchange.type.toUpperCase()} •{' '}
                      {selectedExchange.id}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {selectedExchange && (
              <>
                {/* Binance 和其他 CEX 交易所的字段 */}
                {(selectedExchange.id === 'binance' ||
                  selectedExchange.type === 'cex') &&
                  selectedExchange.id !== 'hyperliquid' &&
                  selectedExchange.id !== 'aster' && (
                    <>
                      {/* 币安用户配置提示 (D1 方案) */}
                      {selectedExchange.id === 'binance' && (
                        <div
                          className="mb-4 p-3 rounded cursor-pointer transition-colors"
                          style={{
                            background: '#1a3a52',
                            border: '1px solid #2b5278',
                          }}
                          onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span style={{ color: '#58a6ff' }}>ℹ️</span>
                              <span
                                className="text-sm font-medium"
                                style={{ color: '#EAECEF' }}
                              >
                                <strong>币安用户必读：</strong>
                                使用「现货与合约交易」API，不要用「统一账户
                                API」
                              </span>
                            </div>
                            <span style={{ color: '#8b949e' }}>
                              {showBinanceGuide ? '▲' : '▼'}
                            </span>
                          </div>

                          {/* 展开的详细说明 */}
                          {showBinanceGuide && (
                            <div
                              className="mt-3 pt-3"
                              style={{
                                borderTop: '1px solid #2b5278',
                                fontSize: '0.875rem',
                                color: '#c9d1d9',
                              }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <p className="mb-2" style={{ color: '#8b949e' }}>
                                <strong>原因：</strong>统一账户 API
                                权限结构不同，会导致订单提交失败
                              </p>

                              <p
                                className="font-semibold mb-1"
                                style={{ color: '#EAECEF' }}
                              >
                                正确配置步骤：
                              </p>
                              <ol
                                className="list-decimal list-inside space-y-1 mb-3"
                                style={{ paddingLeft: '0.5rem' }}
                              >
                                <li>
                                  登录币安 → 个人中心 →{' '}
                                  <strong>API 管理</strong>
                                </li>
                                <li>
                                  创建 API → 选择「
                                  <strong>系统生成的 API 密钥</strong>」
                                </li>
                                <li>
                                  勾选「<strong>现货与合约交易</strong>」（
                                  <span style={{ color: '#f85149' }}>
                                    不选统一账户
                                  </span>
                                  ）
                                </li>
                                <li>
                                  IP 限制选「<strong>无限制</strong>
                                  」或添加服务器 IP
                                </li>
                              </ol>

                              <p
                                className="mb-2 p-2 rounded"
                                style={{
                                  background: '#3d2a00',
                                  border: '1px solid #9e6a03',
                                }}
                              >
                                💡 <strong>多资产模式用户注意：</strong>
                                如果您开启了多资产模式，将强制使用全仓模式。建议关闭多资产模式以支持逐仓交易。
                              </p>

                              <a
                                href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-block text-sm hover:underline"
                                style={{ color: '#58a6ff' }}
                              >
                                📖 查看币安官方教程 ↗
                              </a>
                            </div>
                          )}
                        </div>
                      )}

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('apiKey', language)}
                        </label>
                        <input
                          type="password"
                          value={apiKey}
                          onChange={(e) => setApiKey(e.target.value)}
                          placeholder={t('enterAPIKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('secretKey', language)}
                        </label>
                        <input
                          type="password"
                          value={secretKey}
                          onChange={(e) => setSecretKey(e.target.value)}
                          placeholder={t('enterSecretKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      {selectedExchange.id === 'okx' && (
                        <div>
                          <label
                            className="block text-sm font-semibold mb-2"
                            style={{ color: '#EAECEF' }}
                          >
                            {t('passphrase', language)}
                          </label>
                          <input
                            type="password"
                            value={passphrase}
                            onChange={(e) => setPassphrase(e.target.value)}
                            placeholder={t('enterPassphrase', language)}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                            required
                          />
                        </div>
                      )}

                      {/* Binance 白名单IP提示 */}
                      {selectedExchange.id === 'binance' && (
                        <div
                          className="p-4 rounded"
                          style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            border: '1px solid rgba(240, 185, 11, 0.2)',
                          }}
                        >
                          <div
                            className="text-sm font-semibold mb-2"
                            style={{ color: '#F0B90B' }}
                          >
                            {t('whitelistIP', language)}
                          </div>
                          <div
                            className="text-xs mb-3"
                            style={{ color: '#848E9C' }}
                          >
                            {t('whitelistIPDesc', language)}
                          </div>

                          {loadingIP ? (
                            <div
                              className="text-xs"
                              style={{ color: '#848E9C' }}
                            >
                              {t('loadingServerIP', language)}
                            </div>
                          ) : serverIP && serverIP.public_ip ? (
                            <div
                              className="flex items-center gap-2 p-2 rounded"
                              style={{ background: '#0B0E11' }}
                            >
                              <code
                                className="flex-1 text-sm font-mono"
                                style={{ color: '#F0B90B' }}
                              >
                                {serverIP.public_ip}
                              </code>
                              <button
                                type="button"
                                onClick={() => handleCopyIP(serverIP.public_ip)}
                                className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                                style={{
                                  background: 'rgba(240, 185, 11, 0.2)',
                                  color: '#F0B90B',
                                }}
                              >
                                {copiedIP
                                  ? t('ipCopied', language)
                                  : t('copyIP', language)}
                              </button>
                            </div>
                          ) : null}
                        </div>
                      )}
                    </>
                  )}

                {/* Hyperliquid 交易所的字段 */}
                {selectedExchange.id === 'hyperliquid' && (
                  <>
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('privateKey', language)}
                      </label>
                      <input
                        type="password"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={t('enterPrivateKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidPrivateKeyDesc', language)}
                      </div>
                    </div>
                  </>
                )}

                {/* Aster 交易所的字段 */}
                {selectedExchange.id === 'aster' && (
                  <>
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('user', language)}
                        <Tooltip content={t('asterUserDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterUser}
                        onChange={(e) => setAsterUser(e.target.value)}
                        placeholder={t('enterUser', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('signer', language)}
                        <Tooltip content={t('asterSignerDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterSigner}
                        onChange={(e) => setAsterSigner(e.target.value)}
                        placeholder={t('enterSigner', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('privateKey', language)}
                        <Tooltip content={t('asterPrivateKeyDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="password"
                        value={asterPrivateKey}
                        onChange={(e) => setAsterPrivateKey(e.target.value)}
                        placeholder={t('enterPrivateKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>
                  </>
                )}

                {/* Hyperliquid 交易所的字段 */}
                {selectedExchange.id === 'hyperliquid' && (
                  <>
                    {/* 安全提示 banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(240, 185, 11, 0.1)',
                        border: '1px solid rgba(240, 185, 11, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: '#F0B90B', fontSize: '16px' }}>
                          🔐
                        </span>
                        <div className="flex-1">
                          <div
                            className="text-sm font-semibold mb-1"
                            style={{ color: '#F0B90B' }}
                          >
                            {t('hyperliquidAgentWalletTitle', language)}
                          </div>
                          <div
                            className="text-xs"
                            style={{ color: '#848E9C', lineHeight: '1.5' }}
                          >
                            {t('hyperliquidAgentWalletDesc', language)}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Agent Private Key 字段 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidAgentPrivateKey', language)}
                      </label>
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={maskSecret(apiKey)}
                            readOnly
                            placeholder={t(
                              'enterHyperliquidAgentPrivateKey',
                              language
                            )}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                          />
                          <button
                            type="button"
                            onClick={() => setSecureInputTarget('hyperliquid')}
                            className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                            style={{
                              background: '#F0B90B',
                              color: '#000',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {apiKey
                              ? t('secureInputReenter', language)
                              : t('secureInputButton', language)}
                          </button>
                          {apiKey && (
                            <button
                              type="button"
                              onClick={() => setApiKey('')}
                              className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: '#1B1F2B',
                                color: '#848E9C',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {t('secureInputClear', language)}
                            </button>
                          )}
                        </div>
                        {apiKey && (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('secureInputHint', language)}
                          </div>
                        )}
                      </div>
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidAgentPrivateKeyDesc', language)}
                      </div>
                    </div>

                    {/* Main Wallet Address 字段 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidMainWalletAddress', language)}
                      </label>
                      <input
                        type="text"
                        value={hyperliquidWalletAddr}
                        onChange={(e) =>
                          setHyperliquidWalletAddr(e.target.value)
                        }
                        placeholder={t(
                          'enterHyperliquidMainWalletAddress',
                          language
                        )}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidMainWalletAddressDesc', language)}
                      </div>
                    </div>
                  </>
                )}

                {/* Aster 交易所的字段 */}
                {selectedExchange.id === 'aster' && (
                  <>
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('user', language)}
                        <Tooltip content={t('asterUserDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterUser}
                        onChange={(e) => setAsterUser(e.target.value)}
                        placeholder={t('enterUser', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('signer', language)}
                        <Tooltip content={t('asterSignerDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterSigner}
                        onChange={(e) => setAsterSigner(e.target.value)}
                        placeholder={t('enterSigner', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('privateKey', language)}
                        <Tooltip content={t('asterPrivateKeyDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={maskSecret(asterPrivateKey)}
                            readOnly
                            placeholder={t('enterPrivateKey', language)}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                          />
                          <button
                            type="button"
                            onClick={() => setSecureInputTarget('aster')}
                            className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                            style={{
                              background: '#F0B90B',
                              color: '#000',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {asterPrivateKey
                              ? t('secureInputReenter', language)
                              : t('secureInputButton', language)}
                          </button>
                          {asterPrivateKey && (
                            <button
                              type="button"
                              onClick={() => setAsterPrivateKey('')}
                              className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: '#1B1F2B',
                                color: '#848E9C',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {t('secureInputClear', language)}
                            </button>
                          )}
                        </div>
                        {asterPrivateKey && (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('secureInputHint', language)}
                          </div>
                        )}
                      </div>
                    </div>

                    <div
                      className="p-4 rounded"
                      style={{
                        background: 'rgba(240, 185, 11, 0.1)',
                        border: '1px solid rgba(240, 185, 11, 0.2)',
                      }}
                    >
                      <div
                        className="text-sm font-semibold mb-2"
                        style={{ color: '#F0B90B' }}
                      >
                        <span className="inline-flex items-center gap-1">
                          <AlertTriangle className="w-4 h-4" />{' '}
                          {t('securityWarning', language)}
                        </span>
                      </div>
                      <div
                        className="text-xs space-y-1"
                        style={{ color: '#848E9C' }}
                      >
                        {selectedExchange.id === 'aster' && (
                          <div>{t('asterUsdtWarning', language)}</div>
                        )}
                        <div>{t('exchangeConfigWarning1', language)}</div>
                        <div>{t('exchangeConfigWarning2', language)}</div>
                        <div>{t('exchangeConfigWarning3', language)}</div>
                      </div>
                    </div>
                  </>
                )}
              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={
                !selectedExchange ||
                (selectedExchange.id === 'binance' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (selectedExchange.id === 'okx' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (selectedExchange.id === 'hyperliquid' &&
                  (!apiKey.trim() || !hyperliquidWalletAddr.trim())) || // 验证私钥和钱包地址
                (selectedExchange.id === 'aster' &&
                  (!asterUser.trim() ||
                    !asterSigner.trim() ||
                    !asterPrivateKey.trim())) ||
                (selectedExchange.type === 'cex' &&
                  selectedExchange.id !== 'hyperliquid' &&
                  selectedExchange.id !== 'aster' &&
                  selectedExchange.id !== 'binance' &&
                  selectedExchange.id !== 'okx' &&
                  (!apiKey.trim() || !secretKey.trim()))
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: '#1E2329' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: '#F0B90B' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#2B3139', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded"
              />
            </div>
          </div>
        </div>
      )}

      {/* Two Stage Key Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={handleSecureInputCancel}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
