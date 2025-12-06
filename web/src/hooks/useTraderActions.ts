import { api } from '../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  TraderConfigData,
  AIModel,
  Exchange,
} from '../types'
import { t } from '../i18n/translations'
import { confirmToast } from '../lib/notify'
import { toast } from 'sonner'
import type { Language } from '../i18n/translations'

interface UseTraderActionsParams {
  traders: TraderInfo[] | undefined
  allModels: AIModel[]
  allExchanges: Exchange[]
  supportedModels: AIModel[]
  supportedExchanges: Exchange[]
  language: Language
  mutateTraders: () => Promise<any>
  setAllModels: (models: AIModel[]) => void
  setAllExchanges: (exchanges: Exchange[]) => void
  setShowCreateModal: (show: boolean) => void
  setShowEditModal: (show: boolean) => void
  setShowModelModal: (show: boolean) => void
  setShowExchangeModal: (show: boolean) => void
  setEditingModel: (modelId: string | null) => void
  setEditingExchange: (exchangeId: string | null) => void
  editingTrader: TraderConfigData | null
  setEditingTrader: (trader: TraderConfigData | null) => void
}

export function useTraderActions({
  traders,
  allModels,
  allExchanges,
  supportedModels,
  supportedExchanges,
  language,
  mutateTraders,
  setAllModels,
  setAllExchanges,
  setShowCreateModal,
  setShowEditModal,
  setShowModelModal,
  setShowExchangeModal,
  setEditingModel,
  setEditingExchange,
  editingTrader,
  setEditingTrader,
}: UseTraderActionsParams) {
  // 检查模型是否正在被运行中的交易员使用(用于UI禁用)
  const isModelInUse = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId && t.is_running) || false
  }

  // 检查交易所是否正在被运行中的交易员使用(用于UI禁用)
  const isExchangeInUse = (exchangeId: string) => {
    return (
      traders?.some((t) => t.exchange_id === exchangeId && t.is_running) ||
      false
    )
  }

  // 检查模型是否被任何交易员使用(包括停止状态的)
  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId) || false
  }

  // 检查交易所是否被任何交易员使用(包括停止状态的)
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
    if (!editingTrader || !editingTrader.trader_id) return

    try {
      const enabledModels = allModels?.filter((m) => m.enabled) || []
      const enabledExchanges =
        allExchanges?.filter((e) => {
          if (!e.enabled) return false

          // Aster 交易所需要特殊字段
          if (e.id === 'aster') {
            return (
              e.asterUser &&
              e.asterUser.trim() !== '' &&
              e.asterSigner &&
              e.asterSigner.trim() !== ''
            )
          }

          // Hyperliquid 需要钱包地址
          if (e.id === 'hyperliquid') {
            return (
              e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
            )
          }

          return true
        }) || []

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
        system_prompt_template: data.system_prompt_template,
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

  const handleDeleteModel = async (modelId: string) => {
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

  const handleSaveModel = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      // 创建或更新用户的模型配置
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      // 找到要配置的模型(优先从已配置列表,其次从支持列表)
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

  const handleDeleteExchange = async (exchangeId: string) => {
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
        passphrase: '', // OKX专用
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
              passphrase: exchange.passphrase || '', // OKX专用
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
      refreshApi: api.getExchangeConfigs,
      setItems: (items) => {
        // 使用函数式更新确保状态正确更新
        setAllExchanges([...items])
      },
      closeModal: () => {
        setShowExchangeModal(false)
        setEditingExchange(null)
      },
      errorKey: 'deleteExchangeConfigFailed',
    })
  }

  const handleSaveExchange = async (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string, // OKX专用
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string
  ) => {
    try {
      // 找到要配置的交易所(从supportedExchanges中)
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
                  passphrase, // OKX专用
                  testnet,
                  hyperliquidWalletAddr,
                  asterUser,
                  asterSigner,
                  asterPrivateKey,
                  lighterWalletAddr,
                  lighterPrivateKey,
                  lighterApiKeyPrivateKey,
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
          passphrase, // OKX专用
          testnet,
          hyperliquidWalletAddr,
          asterUser,
          asterSigner,
          asterPrivateKey,
          lighterWalletAddr,
          lighterPrivateKey,
          lighterApiKeyPrivateKey,
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
              passphrase: exchange.passphrase || '', // OKX专用
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
              lighter_wallet_addr: exchange.lighterWalletAddr || '',
              lighter_private_key: exchange.lighterPrivateKey || '',
              lighter_api_key_private_key: exchange.lighterApiKeyPrivateKey || '',
            },
          ])
        ),
      }

      await toast.promise(api.updateExchangeConfigsEncrypted(request), {
        loading: '正在更新交易所配置…',
        success: '交易所配置已更新',
        error: '更新交易所配置失败',
      })

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

  return {
    // 辅助函数
    isModelInUse,
    isExchangeInUse,
    isModelUsedByAnyTrader,
    isExchangeUsedByAnyTrader,
    getTradersUsingModel,
    getTradersUsingExchange,

    // 事件处理函数
    handleCreateTrader,
    handleEditTrader,
    handleSaveEditTrader,
    handleDeleteTrader,
    handleToggleTrader,
    handleAddModel,
    handleAddExchange,
    handleModelClick,
    handleExchangeClick,
    handleSaveModel,
    handleDeleteModel,
    handleSaveExchange,
    handleDeleteExchange,
  }
}
