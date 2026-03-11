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
import { DeepVoidBackground } from './DeepVoidBackground'
import { ExchangeConfigModal } from './traders/ExchangeConfigModal'
import { PunkAvatar, getTraderAvatar } from './PunkAvatar'
import {
  Bot,
  Brain,
  Landmark,
  BarChart3,
  Trash2,
  Plus,
  Users,
  Pencil,
  Eye,
  EyeOff,
  ExternalLink,
  Copy,
  Check,
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

// Top models available through BlockRun wallet providers
const BLOCKRUN_MODELS = [
  { id: 'gpt-5.4', name: 'GPT-5.4', desc: 'OpenAI · Flagship' },
  { id: 'claude-opus-4.6', name: 'Claude Opus 4.6', desc: 'Anthropic · Flagship' },
  { id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro', desc: 'Google · Flagship' },
  { id: 'grok-3', name: 'Grok 3', desc: 'xAI · Flagship' },
  { id: 'deepseek-chat', name: 'DeepSeek Chat', desc: 'DeepSeek · Flagship' },
  { id: 'minimax-m2.5', name: 'MiniMax M2.5', desc: 'MiniMax · Flagship' },
]

// AI Provider configuration - default models and API links
const AI_PROVIDER_CONFIG: Record<string, {
  defaultModel: string
  apiUrl: string
  apiName: string
}> = {
  deepseek: {
    defaultModel: 'deepseek-chat',
    apiUrl: 'https://platform.deepseek.com/api_keys',
    apiName: 'DeepSeek',
  },
  qwen: {
    defaultModel: 'qwen3-max',
    apiUrl: 'https://dashscope.console.aliyun.com/apiKey',
    apiName: 'Alibaba Cloud',
  },
  openai: {
    defaultModel: 'gpt-5.2',
    apiUrl: 'https://platform.openai.com/api-keys',
    apiName: 'OpenAI',
  },
  claude: {
    defaultModel: 'claude-opus-4-6',
    apiUrl: 'https://console.anthropic.com/settings/keys',
    apiName: 'Anthropic',
  },
  gemini: {
    defaultModel: 'gemini-3-pro-preview',
    apiUrl: 'https://aistudio.google.com/app/apikey',
    apiName: 'Google AI Studio',
  },
  grok: {
    defaultModel: 'grok-3-latest',
    apiUrl: 'https://console.x.ai/',
    apiName: 'xAI',
  },
  kimi: {
    defaultModel: 'moonshot-v1-auto',
    apiUrl: 'https://platform.moonshot.ai/console/api-keys',
    apiName: 'Moonshot',
  },
  minimax: {
    defaultModel: 'MiniMax-M2.5',
    apiUrl: 'https://platform.minimax.io',
    apiName: 'MiniMax',
  },
  'blockrun-base': {
    defaultModel: 'gpt-5.4',
    apiUrl: 'https://blockrun.ai',
    apiName: 'BlockRun',
  },
  'blockrun-sol': {
    defaultModel: 'gpt-5.4',
    apiUrl: 'https://sol.blockrun.ai',
    apiName: 'BlockRun',
  },
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayName(exchangeId: string | undefined, exchanges: Exchange[]): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges.find(e => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...' // Show truncated UUID if not found
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name ? `${typeName} - ${exchange.account_name}` : typeName
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
  const [allModels, setAllModels] = useState<AIModel[]>([])
  const [allExchanges, setAllExchanges] = useState<Exchange[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [visibleTraderAddresses, setVisibleTraderAddresses] = useState<Set<string>>(new Set())
  const [visibleExchangeAddresses, setVisibleExchangeAddresses] = useState<Set<string>>(new Set())
  const [copiedId, setCopiedId] = useState<string | null>(null)

  // Toggle wallet address visibility for a trader
  const toggleTraderAddressVisibility = (traderId: string) => {
    setVisibleTraderAddresses(prev => {
      const next = new Set(prev)
      if (next.has(traderId)) {
        next.delete(traderId)
      } else {
        next.add(traderId)
      }
      return next
    })
  }

  // Toggle wallet address visibility for an exchange
  const toggleExchangeAddressVisibility = (exchangeId: string) => {
    setVisibleExchangeAddresses(prev => {
      const next = new Set(prev)
      if (next.has(exchangeId)) {
        next.delete(exchangeId)
      } else {
        next.add(exchangeId)
      }
      return next
    })
  }

  // Copy wallet address to clipboard
  const handleCopyAddress = async (id: string, address: string) => {
    try {
      await navigator.clipboard.writeText(address)
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }

  const { data: traders, mutate: mutateTraders, isLoading: isTradersLoading } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        // 未登录时只加载公开的支持模型
        try {
          const supportedModels = await api.getSupportedModels()
          setSupportedModels(supportedModels)
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
        ] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
        ])
        setAllModels(modelConfigs)
        setAllExchanges(exchangeConfigs)
        setSupportedModels(supportedModels)
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
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

  // 检查模型被哪些交易员使用
  const getModelUsageInfo = (modelId: string) => {
    const usingTraders = traders?.filter((t) => t.ai_model === modelId) || []
    const runningCount = usingTraders.filter((t) => t.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  // 检查交易所是否正在被运行中的交易员使用（用于UI禁用）
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId && t.is_running)
  }

  // 检查交易所被哪些交易员使用
  const getExchangeUsageInfo = (exchangeId: string) => {
    const usingTraders = traders?.filter((t) => t.exchange_id === exchangeId) || []
    const runningCount = usingTraders.filter((t) => t.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
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
      // Immediately refresh traders list for better UX
      await mutateTraders()
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
    console.log('🔥🔥🔥 handleSaveEditTrader CALLED with data:', data)
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
        strategy_id: data.strategy_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        is_cross_margin: data.is_cross_margin,
        show_in_competition: data.show_in_competition,
      }

      console.log('🔥 handleSaveEditTrader - data:', data)
      console.log('🔥 handleSaveEditTrader - data.strategy_id:', data.strategy_id)
      console.log('🔥 handleSaveEditTrader - request:', request)

      await toast.promise(api.updateTrader(editingTrader.trader_id, request), {
        loading: '正在保存…',
        success: '保存成功',
        error: '保存失败',
      })
      setShowEditModal(false)
      setEditingTrader(null)
      // Immediately refresh traders list for better UX
      await mutateTraders()
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

      // Immediately refresh traders list for better UX
      await mutateTraders()
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

      // Immediately refresh traders list to update running status
      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      toast.error(t('operationFailed', language))
    }
  }

  const handleToggleCompetition = async (traderId: string, currentShowInCompetition: boolean) => {
    try {
      const newValue = !currentShowInCompetition
      await toast.promise(api.toggleCompetition(traderId, newValue), {
        loading: '正在更新…',
        success: newValue ? '已在竞技场显示' : '已在竞技场隐藏',
        error: '更新失败',
      })

      // Immediately refresh traders list to update status
      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle competition visibility:', error)
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
    refreshApi: () => Promise<T[]>
    setItems: (items: T[]) => void
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

      // 重新获取用户配置以确保数据同步
      const refreshedItems = await config.refreshApi()
      config.setItems(refreshedItems)

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
      refreshApi: api.getModelConfigs,
      setItems: (items) => {
        // 使用函数式更新确保状态正确更新
        setAllModels([...items])
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

      // 重新获取用户配置以确保数据同步
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    // 检查是否有trader在使用此交易所账户
    if (isExchangeUsedByAnyTrader(exchangeId)) {
      const tradersUsing = getTradersUsingExchange(exchangeId)
      toast.error(
        `${t('cannotDeleteExchangeInUse', language)}: ${tradersUsing.join(', ')}`
      )
      return
    }

    // 确认删除
    const ok = await confirmToast(t('confirmDeleteExchange', language))
    if (!ok) return

    try {
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: language === 'zh' ? '正在删除交易所账户…' : 'Deleting exchange account...',
        success: language === 'zh' ? '交易所账户已删除' : 'Exchange account deleted',
        error: language === 'zh' ? '删除交易所账户失败' : 'Failed to delete exchange account',
      })

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to delete exchange config:', error)
      toast.error(t('deleteExchangeConfigFailed', language))
    }
  }

  const handleSaveExchangeConfig = async (
    exchangeId: string | null, // null for creating new account
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number
  ) => {
    try {
      if (exchangeId) {
        // 更新现有账户配置
        const existingExchange = allExchanges?.find((e) => e.id === exchangeId)
        if (!existingExchange) {
          toast.error(t('exchangeNotExist', language))
          return
        }

        const request = {
          exchanges: {
            [exchangeId]: {
              enabled: true,
              api_key: apiKey || '',
              secret_key: secretKey || '',
              passphrase: passphrase || '',
              testnet: testnet || false,
              hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
              aster_user: asterUser || '',
              aster_signer: asterSigner || '',
              aster_private_key: asterPrivateKey || '',
              lighter_wallet_addr: lighterWalletAddr || '',
              lighter_private_key: lighterPrivateKey || '',
              lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
              lighter_api_key_index: lighterApiKeyIndex || 0,
            },
          },
        }

        await toast.promise(api.updateExchangeConfigsEncrypted(request), {
          loading: language === 'zh' ? '正在更新交易所配置…' : 'Updating exchange config...',
          success: language === 'zh' ? '交易所配置已更新' : 'Exchange config updated',
          error: language === 'zh' ? '更新交易所配置失败' : 'Failed to update exchange config',
        })
      } else {
        // 创建新账户
        const createRequest = {
          exchange_type: exchangeType,
          account_name: accountName,
          enabled: true,
          api_key: apiKey || '',
          secret_key: secretKey || '',
          passphrase: passphrase || '',
          testnet: testnet || false,
          hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
          aster_user: asterUser || '',
          aster_signer: asterSigner || '',
          aster_private_key: asterPrivateKey || '',
          lighter_wallet_addr: lighterWalletAddr || '',
          lighter_private_key: lighterPrivateKey || '',
          lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
          lighter_api_key_index: lighterApiKeyIndex || 0,
        }

        await toast.promise(api.createExchangeEncrypted(createRequest), {
          loading: language === 'zh' ? '正在创建交易所账户…' : 'Creating exchange account...',
          success: language === 'zh' ? '交易所账户已创建' : 'Exchange account created',
          error: language === 'zh' ? '创建交易所账户失败' : 'Failed to create exchange account',
        })
      }

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

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

  return (
    <DeepVoidBackground className="py-8" disableAnimation>
      <div className="w-full px-4 md:px-8 space-y-8 animate-fade-in">
        {/* Header - Terminal Style */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 border-b border-white/10 pb-6">
          <div className="flex items-center gap-4">
            <div className="relative group">
              <div className="absolute -inset-1 bg-nofx-gold/20 rounded-xl blur opacity-0 group-hover:opacity-100 transition duration-500"></div>
              <div className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center bg-black border border-nofx-gold/30 text-nofx-gold relative z-10 shadow-[0_0_15px_rgba(240,185,11,0.1)]">
                <Bot className="w-6 h-6 md:w-7 md:h-7" />
              </div>
            </div>
            <div>
              <h1 className="text-2xl md:text-3xl font-bold font-mono tracking-tight text-white flex items-center gap-3 uppercase">
                {t('aiTraders', language)}
                <span className="text-xs font-mono font-normal px-2 py-0.5 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 tracking-wider">
                  {traders?.length || 0} ACTIVE_NODES
                </span>
              </h1>
              <p className="text-xs font-mono text-zinc-500 uppercase tracking-widest mt-1 ml-1 flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
                SYSTEM_READY
              </p>
            </div>
          </div>

          <div className="flex flex-wrap gap-2 w-full md:w-auto justify-center md:justify-end pt-2 md:pt-0">
            <button
              onClick={handleAddModel}
              className="px-3 md:px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm flex-shrink-0"
            >
              <div className="flex items-center gap-1.5 md:gap-2">
                <Plus className="w-3 h-3" />
                <span className="hidden sm:inline">MODELS_CONFIG</span>
                <span className="sm:hidden">MODELS</span>
              </div>
            </button>

            <button
              onClick={handleAddExchange}
              className="px-3 md:px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm flex-shrink-0"
            >
              <div className="flex items-center gap-1.5 md:gap-2">
                <Plus className="w-3 h-3" />
                <span className="hidden sm:inline">EXCHANGE_KEYS</span>
                <span className="sm:hidden">KEYS</span>
              </div>
            </button>

            <button
              onClick={() => setShowCreateModal(true)}
              disabled={
                configuredModels.length === 0 ||
                configuredExchanges.length === 0
              }
              className="group relative px-4 md:px-6 py-2 rounded text-xs font-bold font-mono uppercase tracking-wider transition-all disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap overflow-hidden bg-nofx-gold text-black hover:bg-yellow-400 shadow-[0_0_20px_rgba(240,185,11,0.2)] hover:shadow-[0_0_30px_rgba(240,185,11,0.4)] flex-shrink-0"
            >
              <span className="relative z-10 flex items-center gap-1.5 md:gap-2">
                <Plus className="w-4 h-4" />
                <span className="hidden sm:inline">{t('createTrader', language)}</span>
                <span className="sm:hidden">{t('create', language)}</span>
              </span>
              <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300"></div>
            </button>
          </div>
        </div>

        {/* Configuration Status Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* AI Models Card */}
          <div className="nofx-glass rounded-lg border border-white/5 overflow-hidden">
            <div className="px-4 py-3 border-b border-white/5 bg-black/20 flex items-center gap-2 backdrop-blur-sm">
              <Brain className="w-4 h-4 text-nofx-gold" />
              <h3 className="text-sm font-mono tracking-widest text-zinc-300 uppercase">
                {t('aiModels', language)}
              </h3>
            </div>

            <div className="p-4 space-y-3">
              {configuredModels.map((model) => {
                const inUse = isModelInUse(model.id)
                const usageInfo = getModelUsageInfo(model.id)
                return (
                  <div
                    key={model.id}
                    className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${inUse ? 'opacity-80' : 'hover:bg-white/5 hover:border-white/10 cursor-pointer'
                      } bg-black/20`}
                    onClick={() => handleModelClick(model.id)}
                  >
                    <div className="flex items-center gap-4">
                      <div className="relative">
                        <div className="absolute inset-0 bg-indigo-500/20 rounded-full blur-sm group-hover:bg-indigo-500/30 transition-all"></div>
                        <div className="w-10 h-10 rounded-full flex items-center justify-center bg-black border border-white/10 relative z-10">
                          {getModelIcon(model.provider || model.id, { width: 20, height: 20 }) || (
                            <span className="text-xs font-bold text-indigo-400">{getShortName(model.name)[0]}</span>
                          )}
                        </div>
                      </div>

                      <div className="min-w-0">
                        <div className="font-mono text-sm text-zinc-200 group-hover:text-nofx-gold transition-colors">
                          {getShortName(model.name)}
                        </div>
                        <div className="text-[10px] text-zinc-500 font-mono flex items-center gap-2">
                          {model.customModelName || AI_PROVIDER_CONFIG[model.provider]?.defaultModel || ''}
                        </div>
                      </div>
                    </div>

                    <div className="text-right">
                      {usageInfo.totalCount > 0 ? (
                        <span className={`text-[10px] font-mono px-2 py-1 rounded border ${usageInfo.runningCount > 0
                          ? 'bg-green-500/10 border-green-500/30 text-green-400'
                          : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                          }`}>
                          {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                        </span>
                      ) : (
                        <span className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">
                          {language === 'zh' ? '就绪' : 'STANDBY'}
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}

              {configuredModels.length === 0 && (
                <div className="text-center py-10 border border-dashed border-zinc-800 rounded-lg bg-black/20">
                  <Brain className="w-8 h-8 mx-auto mb-3 text-zinc-700" />
                  <div className="text-xs font-mono text-zinc-500 uppercase tracking-widest">{t('noModelsConfigured', language)}</div>
                </div>
              )}
            </div>
          </div>

          {/* Exchanges Card */}
          <div className="nofx-glass rounded-lg border border-white/5 overflow-hidden">
            <div className="px-4 py-3 border-b border-white/5 bg-black/20 flex items-center gap-2 backdrop-blur-sm">
              <Landmark className="w-4 h-4 text-nofx-gold" />
              <h3 className="text-sm font-mono tracking-widest text-zinc-300 uppercase">
                {t('exchanges', language)}
              </h3>
            </div>

            <div className="p-4 space-y-3">
              {configuredExchanges.map((exchange) => {
                const inUse = isExchangeInUse(exchange.id)
                const usageInfo = getExchangeUsageInfo(exchange.id)
                return (
                  <div
                    key={exchange.id}
                    className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${inUse ? 'opacity-80' : 'hover:bg-white/5 hover:border-white/10 cursor-pointer'
                      } bg-black/20`}
                    onClick={() => handleExchangeClick(exchange.id)}
                  >
                    <div className="flex items-center gap-4 min-w-0">
                      <div className="relative">
                        <div className="absolute inset-0 bg-yellow-500/20 rounded-full blur-sm group-hover:bg-yellow-500/30 transition-all"></div>
                        <div className="w-10 h-10 rounded-full flex items-center justify-center bg-black border border-white/10 relative z-10">
                          {getExchangeIcon(exchange.exchange_type || exchange.id, { width: 20, height: 20 })}
                        </div>
                      </div>

                      <div className="min-w-0">
                        <div className="font-mono text-sm text-zinc-200 group-hover:text-nofx-gold transition-colors truncate">
                          {exchange.exchange_type?.toUpperCase() || getShortName(exchange.name)}
                          <span className="text-[10px] text-zinc-500 ml-2 border border-zinc-800 px-1 rounded">
                            {exchange.account_name || 'DEFAULT'}
                          </span>
                        </div>
                        <div className="text-[10px] text-zinc-500 font-mono flex items-center gap-2">
                          {exchange.type?.toUpperCase() || 'CEX'}
                        </div>
                      </div>
                    </div>

                    <div className="flex flex-col items-end gap-1">
                      {/* Wallet Address Display Logic */}
                      {(() => {
                        const walletAddr = exchange.hyperliquidWalletAddr || exchange.asterUser || exchange.lighterWalletAddr
                        if (exchange.type !== 'dex' || !walletAddr) return null
                        const isVisible = visibleExchangeAddresses.has(exchange.id)
                        const isCopied = copiedId === `exchange-${exchange.id}`

                        return (
                          <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                            <span className="text-[10px] font-mono text-zinc-400 bg-black/40 px-1.5 py-0.5 rounded border border-zinc-800">
                              {isVisible ? walletAddr : truncateAddress(walletAddr)}
                            </span>
                            <button
                              onClick={(e) => { e.stopPropagation(); toggleExchangeAddressVisibility(exchange.id) }}
                              className="text-zinc-600 hover:text-zinc-300"
                            >
                              {isVisible ? <EyeOff size={10} /> : <Eye size={10} />}
                            </button>
                            <button
                              onClick={(e) => { e.stopPropagation(); handleCopyAddress(`exchange-${exchange.id}`, walletAddr) }}
                              className="text-zinc-600 hover:text-nofx-gold"
                            >
                              {isCopied ? <Check size={10} className="text-green-500" /> : <Copy size={10} />}
                            </button>
                          </div>
                        )
                      })()}

                      {usageInfo.totalCount > 0 ? (
                        <span className={`text-[10px] font-mono px-2 py-1 rounded border ${usageInfo.runningCount > 0
                          ? 'bg-green-500/10 border-green-500/30 text-green-400'
                          : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                          }`}>
                          {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                        </span>
                      ) : (
                        <span className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">
                          {language === 'zh' ? '就绪' : 'STANDBY'}
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
              {configuredExchanges.length === 0 && (
                <div className="text-center py-10 border border-dashed border-zinc-800 rounded-lg bg-black/20">
                  <Landmark className="w-8 h-8 mx-auto mb-3 text-zinc-700" />
                  <div className="text-xs font-mono text-zinc-500 uppercase tracking-widest">{t('noExchangesConfigured', language)}</div>
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

          {isTradersLoading ? (
            /* Loading Skeleton */
            <div className="space-y-3 md:space-y-4">
              {[1, 2, 3].map((i) => (
                <div
                  key={i}
                  className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded gap-3 md:gap-4 animate-pulse"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                >
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="w-10 h-10 md:w-12 md:h-12 rounded-full skeleton"></div>
                    <div className="min-w-0 space-y-2">
                      <div className="skeleton h-5 w-32"></div>
                      <div className="skeleton h-3 w-24"></div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="skeleton h-6 w-16"></div>
                    <div className="skeleton h-6 w-16"></div>
                    <div className="skeleton h-8 w-20"></div>
                  </div>
                </div>
              ))}
            </div>
          ) : traders && traders.length > 0 ? (
            <div className="space-y-3 md:space-y-4">
              {traders.map((trader) => (
                <div
                  key={trader.trader_id}
                  className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                >
                  <div className="flex items-center gap-3 md:gap-4">
                    <div className="flex-shrink-0">
                      <PunkAvatar
                        seed={getTraderAvatar(trader.trader_id, trader.trader_name)}
                        size={48}
                        className="rounded-lg hidden md:block"
                      />
                      <PunkAvatar
                        seed={getTraderAvatar(trader.trader_id, trader.trader_name)}
                        size={40}
                        className="rounded-lg md:hidden"
                      />
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
                        Model • {getExchangeDisplayName(trader.exchange_id, allExchanges)}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
                    {/* Wallet Address for Perp-DEX - placed before status for alignment */}
                    {(() => {
                      const exchange = allExchanges.find(e => e.id === trader.exchange_id)
                      const walletAddr = getWalletAddress(exchange)
                      const isPerpDex = isPerpDexExchange(exchange?.exchange_type)
                      if (!isPerpDex || !walletAddr) return null

                      const isVisible = visibleTraderAddresses.has(trader.trader_id)
                      const isCopied = copiedId === trader.trader_id

                      return (
                        <div
                          className="flex items-center gap-1 px-2 py-1 rounded"
                          style={{
                            background: 'rgba(240, 185, 11, 0.08)',
                            border: '1px solid rgba(240, 185, 11, 0.2)',
                          }}
                        >
                          <span className="text-xs font-mono" style={{ color: '#F0B90B' }}>
                            {isVisible ? walletAddr : truncateAddress(walletAddr)}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              toggleTraderAddressVisibility(trader.trader_id)
                            }}
                            className="p-0.5 rounded hover:bg-gray-700 transition-colors"
                            title={isVisible ? (language === 'zh' ? '隐藏' : 'Hide') : (language === 'zh' ? '显示' : 'Show')}
                          >
                            {isVisible ? (
                              <EyeOff className="w-3 h-3" style={{ color: '#848E9C' }} />
                            ) : (
                              <Eye className="w-3 h-3" style={{ color: '#848E9C' }} />
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleCopyAddress(trader.trader_id, walletAddr)
                            }}
                            className="p-0.5 rounded hover:bg-gray-700 transition-colors"
                            title={language === 'zh' ? '复制' : 'Copy'}
                          >
                            {isCopied ? (
                              <Check className="w-3 h-3" style={{ color: '#0ECB81' }} />
                            ) : (
                              <Copy className="w-3 h-3" style={{ color: '#848E9C' }} />
                            )}
                          </button>
                        </div>
                      )
                    })()}
                    {/* Status */}
                    <div className="text-center">
                      {/* <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                      {t('status', language)}
                    </div> */}
                      <div
                        className={`px-2 md:px-3 py-1 rounded text-xs font-bold ${trader.is_running
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
                            // 使用 slug 格式: name-id前4位
                            const slug = `${trader.trader_name}-${trader.trader_id.slice(0, 4)}`
                            navigate(`/dashboard?trader=${encodeURIComponent(slug)}`)
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
                        onClick={() => handleToggleCompetition(trader.trader_id, trader.show_in_competition ?? true)}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap flex items-center gap-1"
                        style={
                          trader.show_in_competition !== false
                            ? {
                              background: 'rgba(14, 203, 129, 0.1)',
                              color: '#0ECB81',
                            }
                            : {
                              background: 'rgba(132, 142, 156, 0.1)',
                              color: '#848E9C',
                            }
                        }
                        title={trader.show_in_competition !== false ? '在竞技场显示' : '在竞技场隐藏'}
                      >
                        {trader.show_in_competition !== false ? (
                          <Eye className="w-3 h-3 md:w-4 md:h-4" />
                        ) : (
                          <EyeOff className="w-3 h-3 md:w-4 md:h-4" />
                        )}
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
            configuredModels={allModels}
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
            allExchanges={allExchanges}
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
      </div>
    </DeepVoidBackground>
  )
}

// Step indicator component for Model Config
function ModelStepIndicator({ currentStep, labels }: { currentStep: number; labels: string[] }) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: index < currentStep ? '#0ECB81' : index === currentStep ? '#8B5CF6' : '#2B3139',
                color: index <= currentStep ? '#000' : '#848E9C',
              }}
            >
              {index < currentStep ? <Check className="w-4 h-4" /> : index + 1}
            </div>
            <span
              className="text-xs font-medium hidden sm:block"
              style={{ color: index === currentStep ? '#EAECEF' : '#848E9C' }}
            >
              {label}
            </span>
          </div>
          {index < labels.length - 1 && (
            <div
              className="w-8 h-0.5 mx-1"
              style={{ background: index < currentStep ? '#0ECB81' : '#2B3139' }}
            />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

// Model card component
function ModelCard({
  model,
  selected,
  onClick,
  configured,
}: {
  model: AIModel
  selected: boolean
  onClick: () => void
  configured?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col items-center gap-2 p-4 rounded-xl transition-all hover:scale-105"
      style={{
        background: selected ? 'rgba(139, 92, 246, 0.15)' : '#0B0E11',
        border: selected ? '2px solid #8B5CF6' : '2px solid #2B3139',
      }}
    >
      <div className="relative">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black border border-white/10">
          {getModelIcon(model.provider || model.id, { width: 32, height: 32 }) || (
            <span className="text-lg font-bold" style={{ color: '#A78BFA' }}>{model.name[0]}</span>
          )}
        </div>
        {selected && (
          <div
            className="absolute -top-1 -right-1 w-5 h-5 rounded-full flex items-center justify-center"
            style={{ background: '#0ECB81' }}
          >
            <Check className="w-3 h-3 text-black" />
          </div>
        )}
        {configured && !selected && (
          <div
            className="absolute -top-1 -right-1 w-4 h-4 rounded-full flex items-center justify-center"
            style={{ background: '#F0B90B' }}
          >
            <Check className="w-2.5 h-2.5 text-black" />
          </div>
        )}
      </div>
      <span className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {getShortName(model.name)}
      </span>
      <span
        className="text-[10px] px-2 py-0.5 rounded-full uppercase tracking-wide"
        style={{ background: 'rgba(139, 92, 246, 0.2)', color: '#A78BFA' }}
      >
        {model.provider}
      </span>
    </button>
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
  const [currentStep, setCurrentStep] = useState(editingModelId ? 1 : 0)
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')

  const selectedModel = editingModelId
    ? configuredModels?.find((m) => m.id === selectedModelId)
    : allModels?.find((m) => m.id === selectedModelId)

  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '')
      setBaseUrl(selectedModel.customApiUrl || '')
      setModelName(selectedModel.customModelName || '')
    }
  }, [editingModelId, selectedModel])

  const handleSelectModel = (modelId: string) => {
    setSelectedModelId(modelId)
    setCurrentStep(1)
  }

  const handleBack = () => {
    if (editingModelId) {
      onClose()
    } else {
      setCurrentStep(0)
      setSelectedModelId('')
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId || !apiKey.trim()) return
    onSave(selectedModelId, apiKey.trim(), baseUrl.trim() || undefined, modelName.trim() || undefined)
  }

  const availableModels = allModels || []
  const configuredIds = new Set(configuredModels?.map(m => m.id) || [])
  const stepLabels = language === 'zh' ? ['选择模型', '配置 API'] : ['Select Model', 'Configure API']

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4 overflow-y-auto backdrop-blur-sm">
      <div
        className="rounded-2xl w-full max-w-2xl relative my-8 shadow-2xl"
        style={{ background: 'linear-gradient(180deg, #1E2329 0%, #181A20 100%)', maxHeight: 'calc(100vh - 4rem)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {currentStep > 0 && !editingModelId && (
              <button type="button" onClick={handleBack} className="p-2 rounded-lg hover:bg-white/10 transition-colors">
                <svg className="w-5 h-5" style={{ color: '#848E9C' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
            )}
            <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
              {editingModelId ? t('editAIModel', language) : t('addAIModel', language)}
            </h3>
          </div>
          <div className="flex items-center gap-2">
            {editingModelId && (
              <button
                type="button"
                onClick={() => onDelete(editingModelId)}
                className="p-2 rounded-lg hover:bg-red-500/20 transition-colors"
                style={{ color: '#F6465D' }}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
            <button type="button" onClick={onClose} className="p-2 rounded-lg hover:bg-white/10 transition-colors" style={{ color: '#848E9C' }}>
              ✕
            </button>
          </div>
        </div>

        {/* Step Indicator */}
        {!editingModelId && (
          <div className="px-6">
            <ModelStepIndicator currentStep={currentStep} labels={stepLabels} />
          </div>
        )}

        {/* Content */}
        <div className="px-6 pb-6 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 16rem)' }}>
          {/* Step 0: Select Model */}
          {currentStep === 0 && !editingModelId && (
            <div className="space-y-4">
              <div className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? '选择 AI 模型提供商' : 'Choose Your AI Provider'}
              </div>
              <div className="grid grid-cols-3 sm:grid-cols-4 gap-3">
                {availableModels.filter(m => !m.provider?.startsWith('blockrun')).map((model) => (
                  <ModelCard
                    key={model.id}
                    model={model}
                    selected={selectedModelId === model.id}
                    onClick={() => handleSelectModel(model.id)}
                    configured={configuredIds.has(model.id)}
                  />
                ))}
              </div>
              {availableModels.some(m => m.provider?.startsWith('blockrun')) && (
                <>
                  <div className="flex items-center gap-3 pt-2">
                    <div className="flex-1 h-px" style={{ background: '#2B3139' }} />
                    <span className="text-xs font-medium px-2" style={{ color: '#848E9C' }}>
                      {language === 'zh' ? '通过钱包支付' : 'Via BlockRun Wallet'}
                    </span>
                    <div className="flex-1 h-px" style={{ background: '#2B3139' }} />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    {availableModels.filter(m => m.provider?.startsWith('blockrun')).map((model) => (
                      <ModelCard
                        key={model.id}
                        model={model}
                        selected={selectedModelId === model.id}
                        onClick={() => handleSelectModel(model.id)}
                        configured={configuredIds.has(model.id)}
                      />
                    ))}
                  </div>
                </>
              )}
              <div className="text-xs text-center pt-2" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '带金色标记的模型已配置' : 'Models with gold badge are already configured'}
              </div>
            </div>
          )}

          {/* Step 1: Configure */}
          {(currentStep === 1 || editingModelId) && selectedModel && (
            <form onSubmit={handleSubmit} className="space-y-5">
              {/* Selected Model Header */}
              <div className="p-4 rounded-xl flex items-center gap-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
                <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black border border-white/10">
                  {getModelIcon(selectedModel.provider || selectedModel.id, { width: 32, height: 32 }) || (
                    <span className="text-lg font-bold" style={{ color: '#A78BFA' }}>{selectedModel.name[0]}</span>
                  )}
                </div>
                <div className="flex-1">
                  <div className="font-semibold text-lg" style={{ color: '#EAECEF' }}>
                    {getShortName(selectedModel.name)}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {selectedModel.provider} • {AI_PROVIDER_CONFIG[selectedModel.provider]?.defaultModel || selectedModel.id}
                  </div>
                </div>
                {AI_PROVIDER_CONFIG[selectedModel.provider] && (
                  <a
                    href={AI_PROVIDER_CONFIG[selectedModel.provider].apiUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-2 rounded-lg transition-all hover:scale-105"
                    style={{ background: 'rgba(139, 92, 246, 0.1)', border: '1px solid rgba(139, 92, 246, 0.3)' }}
                  >
                    <ExternalLink className="w-4 h-4" style={{ color: '#A78BFA' }} />
                    <span className="text-sm font-medium" style={{ color: '#A78BFA' }}>
                      {selectedModel.provider?.startsWith('blockrun')
                        ? (language === 'zh' ? '开始使用' : 'Get Started')
                        : (language === 'zh' ? '获取 API Key' : 'Get API Key')}
                    </span>
                  </a>
                )}
              </div>

              {/* Kimi Warning */}
              {selectedModel.provider === 'kimi' && (
                <div className="p-4 rounded-xl" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.3)' }}>
                  <div className="flex items-start gap-2">
                    <span style={{ fontSize: '16px' }}>⚠️</span>
                    <div className="text-sm" style={{ color: '#F6465D' }}>
                      {t('kimiApiNote', language)}
                    </div>
                  </div>
                </div>
              )}

              {/* API Key / Wallet Private Key */}
              <div className="space-y-2">
                <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
                  <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                  </svg>
                  {selectedModel.provider?.startsWith('blockrun')
                    ? (language === 'zh' ? '钱包私钥 *' : 'Wallet Private Key *')
                    : 'API Key *'}
                </label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={
                    selectedModel.provider === 'blockrun-base'
                      ? '0x... (EVM private key)'
                      : selectedModel.provider === 'blockrun-sol'
                      ? 'bs58 encoded key (Solana)'
                      : t('enterAPIKey', language)
                  }
                  className="w-full px-4 py-3 rounded-xl"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  required
                />
              </div>

              {/* Custom Base URL (hidden for BlockRun) */}
              {!selectedModel.provider?.startsWith('blockrun') && (
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
                    <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                    </svg>
                    {t('customBaseURL', language)}
                  </label>
                  <input
                    type="url"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder={t('customBaseURLPlaceholder', language)}
                    className="w-full px-4 py-3 rounded-xl"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {t('leaveBlankForDefault', language)}
                  </div>
                </div>
              )}

              {/* Custom Model Name (hidden for BlockRun) */}
              {!selectedModel.provider?.startsWith('blockrun') && (
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
                    <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
                    </svg>
                    {t('customModelName', language)}
                  </label>
                  <input
                    type="text"
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder={t('customModelNamePlaceholder', language)}
                    className="w-full px-4 py-3 rounded-xl"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {t('leaveBlankForDefaultModel', language)}
                  </div>
                </div>
              )}

              {/* BlockRun Model Selector */}
              {selectedModel.provider?.startsWith('blockrun') && (
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
                    <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                    </svg>
                    {language === 'zh' ? '选择模型' : 'Select Model'}
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    {BLOCKRUN_MODELS.map((m) => {
                      const isSelected = (modelName || BLOCKRUN_MODELS[0].id) === m.id
                      return (
                        <button
                          key={m.id}
                          type="button"
                          onClick={() => setModelName(m.id)}
                          className="flex flex-col items-start px-3 py-2 rounded-xl text-left transition-all"
                          style={{
                            background: isSelected ? 'rgba(37, 99, 235, 0.2)' : '#0B0E11',
                            border: isSelected ? '1px solid #2563EB' : '1px solid #2B3139',
                          }}
                        >
                          <span className="text-xs font-semibold" style={{ color: isSelected ? '#60A5FA' : '#EAECEF' }}>
                            {m.name}
                          </span>
                          <span className="text-[10px]" style={{ color: '#848E9C' }}>{m.desc}</span>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}

              {/* Info Box */}
              <div className="p-4 rounded-xl" style={{ background: 'rgba(139, 92, 246, 0.1)', border: '1px solid rgba(139, 92, 246, 0.2)' }}>
                <div className="text-sm font-semibold mb-2 flex items-center gap-2" style={{ color: '#A78BFA' }}>
                  <Brain className="w-4 h-4" />
                  {t('information', language)}
                </div>
                <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                  <div>• {t('modelConfigInfo1', language)}</div>
                  <div>• {t('modelConfigInfo2', language)}</div>
                  <div>• {t('modelConfigInfo3', language)}</div>
                </div>
              </div>

              {/* Buttons */}
              <div className="flex gap-3 pt-4">
                <button type="button" onClick={handleBack} className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5" style={{ background: '#2B3139', color: '#848E9C' }}>
                  {editingModelId ? t('cancel', language) : (language === 'zh' ? '返回' : 'Back')}
                </button>
                <button
                  type="submit"
                  disabled={!selectedModel || !apiKey.trim()}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{ background: '#8B5CF6', color: '#fff' }}
                >
                  {t('saveConfig', language)}
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
                  </svg>
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
