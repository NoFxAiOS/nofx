import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  AIModel,
  Exchange,
} from '../../types'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { useAuth } from '../../contexts/AuthContext'
import { TraderConfigModal } from './TraderConfigModal'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { ExchangeConfigModal } from './ExchangeConfigModal'
import { TelegramConfigModal } from './TelegramConfigModal'
import { ModelConfigModal } from './ModelConfigModal'
import { ConfigStatusGrid } from './ConfigStatusGrid'
import { TradersList } from './TradersList'
import {
  Bot,
  Plus,
  MessageCircle,
} from 'lucide-react'
import { confirmToast } from '../../lib/notify'
import { toast } from 'sonner'

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
  const [showTelegramModal, setShowTelegramModal] = useState(false)
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

  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        try {
          const models = await api.getSupportedModels()
          setSupportedModels(models)
        } catch (err) {
          console.error('Failed to load supported configs:', err)
        }
        return
      }

      try {
        const [
          modelConfigs,
          exchangeConfigs,
          models,
        ] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
        ])
        setAllModels(modelConfigs)
        setAllExchanges(exchangeConfigs)
        setSupportedModels(models)
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
  }, [user, token])

  const configuredModels =
    allModels?.filter((m) => {
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    }) || []

  const configuredExchanges =
    allExchanges?.filter((e) => {
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      return e.enabled
    }) || []

  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== ''
        )
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      return true
    }) || []

  const isModelInUse = (modelId: string) => {
    return traders?.some((tr) => tr.ai_model === modelId && tr.is_running)
  }

  const getModelUsageInfo = (modelId: string) => {
    const usingTraders = traders?.filter((tr) => tr.ai_model === modelId) || []
    const runningCount = usingTraders.filter((tr) => tr.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((tr) => tr.exchange_id === exchangeId && tr.is_running)
  }

  const getExchangeUsageInfo = (exchangeId: string) => {
    const usingTraders = traders?.filter((tr) => tr.exchange_id === exchangeId) || []
    const runningCount = usingTraders.filter((tr) => tr.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((tr) => tr.ai_model === modelId) || false
  }

  const isExchangeUsedByAnyTrader = (exchangeId: string) => {
    return traders?.some((tr) => tr.exchange_id === exchangeId) || false
  }

  const getTradersUsingModel = (modelId: string) => {
    return traders?.filter((tr) => tr.ai_model === modelId) || []
  }

  const getTradersUsingExchange = (exchangeId: string) => {
    return traders?.filter((tr) => tr.exchange_id === exchangeId) || []
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
    if (config.checkInUse(config.id)) {
      const usingTraders = config.getUsingTraders(config.id)
      const traderNames = usingTraders.map((tr) => tr.trader_name).join(', ')
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
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      const modelToUpdate =
        existingModel || supportedModels?.find((m) => m.id === modelId)
      if (!modelToUpdate) {
        toast.error(t('modelNotExist', language))
        return
      }

      if (existingModel) {
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
            model.provider,
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
    if (isExchangeUsedByAnyTrader(exchangeId)) {
      const tradersUsing = getTradersUsingExchange(exchangeId)
      toast.error(
        `${t('cannotDeleteExchangeInUse', language)}: ${tradersUsing.join(', ')}`
      )
      return
    }

    const ok = await confirmToast(t('confirmDeleteExchange', language))
    if (!ok) return

    try {
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: language === 'zh' ? '正在删除交易所账户…' : 'Deleting exchange account...',
        success: language === 'zh' ? '交易所账户已删除' : 'Exchange account deleted',
        error: language === 'zh' ? '删除交易所账户失败' : 'Failed to delete exchange account',
      })

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
    exchangeId: string | null,
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

          <div className="flex gap-2 w-full md:w-auto overflow-x-auto pb-1 md:pb-0 hide-scrollbar">
            <button
              onClick={handleAddModel}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm"
            >
              <div className="flex items-center gap-2">
                <Plus className="w-3 h-3" />
                <span>MODELS_CONFIG</span>
              </div>
            </button>

            <button
              onClick={handleAddExchange}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-zinc-700 bg-black/20 text-zinc-400 hover:text-white hover:border-zinc-500 whitespace-nowrap backdrop-blur-sm"
            >
              <div className="flex items-center gap-2">
                <Plus className="w-3 h-3" />
                <span>EXCHANGE_KEYS</span>
              </div>
            </button>

            <button
              onClick={() => setShowTelegramModal(true)}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all border border-sky-900/50 bg-black/20 text-sky-500 hover:text-sky-300 hover:border-sky-700 whitespace-nowrap backdrop-blur-sm"
            >
              <div className="flex items-center gap-2">
                <MessageCircle className="w-3 h-3" />
                <span>TELEGRAM_BOT</span>
              </div>
            </button>

            <button
              onClick={() => setShowCreateModal(true)}
              disabled={configuredModels.length === 0 || configuredExchanges.length === 0}
              className="group relative px-6 py-2 rounded text-xs font-bold font-mono uppercase tracking-wider transition-all disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap overflow-hidden bg-nofx-gold text-black hover:bg-yellow-400 shadow-[0_0_20px_rgba(240,185,11,0.2)] hover:shadow-[0_0_30px_rgba(240,185,11,0.4)]"
            >
              <span className="relative z-10 flex items-center gap-2">
                <Plus className="w-4 h-4" />
                {t('createTrader', language)}
              </span>
              <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300"></div>
            </button>
          </div>
        </div>

        {/* Configuration Status Grid */}
        <ConfigStatusGrid
          configuredModels={configuredModels}
          configuredExchanges={configuredExchanges}
          visibleExchangeAddresses={visibleExchangeAddresses}
          copiedId={copiedId}
          language={language}
          isModelInUse={isModelInUse}
          getModelUsageInfo={getModelUsageInfo}
          isExchangeInUse={isExchangeInUse}
          getExchangeUsageInfo={getExchangeUsageInfo}
          onModelClick={handleModelClick}
          onExchangeClick={handleExchangeClick}
          onToggleExchangeAddress={toggleExchangeAddressVisibility}
          onCopyAddress={handleCopyAddress}
        />

        {/* Traders List */}
        <TradersList
          traders={traders}
          isLoading={isTradersLoading}
          allExchanges={allExchanges}
          configuredModelsCount={configuredModels.length}
          configuredExchangesCount={configuredExchanges.length}
          visibleTraderAddresses={visibleTraderAddresses}
          copiedId={copiedId}
          language={language}
          onTraderSelect={onTraderSelect}
          onNavigate={(path) => navigate(path)}
          onEditTrader={handleEditTrader}
          onToggleTrader={handleToggleTrader}
          onToggleCompetition={handleToggleCompetition}
          onDeleteTrader={handleDeleteTrader}
          onToggleTraderAddress={toggleTraderAddressVisibility}
          onCopyAddress={handleCopyAddress}
        />

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

        {/* Telegram Bot Modal */}
        {showTelegramModal && (
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        )}
      </div>
    </DeepVoidBackground>
  )
}
