import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  AIModel,
  Exchange,
  ExchangeAccountState,
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
import { BeginnerGuideCards } from './BeginnerGuideCards'
import { AlertTriangle, Bot, Plus, MessageCircle } from 'lucide-react'
import { confirmToast } from '../../lib/notify'
import { toast } from 'sonner'
import {
  getBeginnerWalletAddress,
  getUserMode,
  setBeginnerWalletAddress as persistBeginnerWalletAddress,
} from '../../lib/onboarding'
import type { Strategy } from '../../types'
import { ApiError } from '../../lib/httpClient'

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
  const [visibleTraderAddresses, setVisibleTraderAddresses] = useState<
    Set<string>
  >(new Set())
  const [visibleExchangeAddresses, setVisibleExchangeAddresses] = useState<
    Set<string>
  >(new Set())
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [quickSetupLoading, setQuickSetupLoading] = useState(false)
  const [beginnerWalletAddress, setBeginnerWalletAddress] = useState<
    string | null
  >(() => getBeginnerWalletAddress())
  const isBeginnerMode = getUserMode() === 'beginner'
  const getErrorMessage = (error: unknown, fallback: string) => {
    if (error instanceof Error && error.message.trim() !== '') {
      return error.message
    }
    return fallback
  }
  const formatActionableDescriptionByKey = (
    errorKey: string,
    params: Record<string, string> = {},
    fallback: string
  ) => {
    const traderName = params.trader_name || params.traderName || 'this trader'
    const modelName = params.model_name || params.modelName || 'selected model'
    const exchangeName =
      params.exchange_name || params.exchangeName || 'selected exchange account'
    const reason = localizeTraderReason(
      params.reason_key,
      params.reason || fallback
    )
    const symbol = params.symbol || ''

    const zh = language === 'zh'

    switch (errorKey) {
      case 'trader.create.invalid_request':
        return zh
          ? '提交的信息不完整，或者格式不正确。请检查后重新提交。'
          : 'The submitted information is incomplete or invalid. Please review it and try again.'
      case 'trader.create.invalid_btc_eth_leverage':
        return zh
          ? 'BTC/ETH 杠杆倍数需要在 1 到 50 倍之间。'
          : 'BTC/ETH leverage must be between 1x and 50x.'
      case 'trader.create.invalid_altcoin_leverage':
        return zh
          ? '山寨币杠杆倍数需要在 1 到 20 倍之间。'
          : 'Altcoin leverage must be between 1x and 20x.'
      case 'trader.create.invalid_symbol':
        return zh
          ? `交易对 ${symbol} 的格式不正确，目前只支持以 USDT 结尾的合约交易对。`
          : `Trading pair ${symbol} is invalid. Only perpetual pairs ending with USDT are supported.`
      case 'trader.create.model_not_found':
        return zh
          ? '还没有找到你选择的 AI 模型。请先到「设置 > 模型配置」添加并启用一个可用模型。'
          : 'The selected AI model was not found. Please add and enable a valid model in Settings > Model Config.'
      case 'trader.create.model_disabled':
        return zh
          ? `AI 模型「${modelName}」目前还没有启用。请先启用它再创建机器人。`
          : `AI model "${modelName}" is currently disabled. Please enable it before creating a trader.`
      case 'trader.create.model_missing_credentials':
        return zh
          ? `AI 模型「${modelName}」缺少 API Key 或支付凭证。请先补全模型配置。`
          : `AI model "${modelName}" is missing API credentials or payment setup. Please complete the model configuration first.`
      case 'trader.create.strategy_required':
        return zh
          ? '你还没有选择交易策略。请先选择一个策略，再继续创建机器人。'
          : 'No trading strategy is selected yet. Please choose a strategy before creating a trader.'
      case 'trader.create.strategy_not_found':
        return zh
          ? '你选择的策略不存在，或者已经被删除了。请重新选择一个可用策略。'
          : 'The selected strategy no longer exists. Please choose another available strategy.'
      case 'trader.create.exchange_not_found':
        return zh
          ? '还没有找到你选择的交易所账户。请先到「设置 > 交易所配置」添加一个可用账户。'
          : 'The selected exchange account was not found. Please add an exchange account in Settings > Exchange Config.'
      case 'trader.create.exchange_disabled':
        return zh
          ? `交易所账户「${exchangeName}」目前处于未启用状态。请先启用它。`
          : `Exchange account "${exchangeName}" is currently disabled. Please enable it first.`
      case 'trader.create.exchange_missing_fields':
        return zh
          ? `交易所账户「${exchangeName}」的配置还不完整。请先补全必填信息。`
          : `Exchange account "${exchangeName}" is incomplete. Please fill in the required fields first.`
      case 'trader.create.exchange_unsupported':
        return zh
          ? `交易所账户「${exchangeName}」当前类型暂不支持机器人创建。`
          : `Exchange account "${exchangeName}" uses a type that is not supported for trader creation.`
      case 'trader.create.exchange_probe_failed':
        return zh
          ? `交易所账户「${exchangeName}」没有通过初始化校验，原因是：${reason}`
          : `Exchange account "${exchangeName}" failed initialization checks: ${reason}`
      case 'trader.start.strategy_missing':
        return zh
          ? `机器人「${traderName}」缺少有效的交易策略配置。`
          : `Trader "${traderName}" does not have a valid strategy configuration.`
      case 'trader.start.model_not_found':
        return zh
          ? `机器人「${traderName}」关联的 AI 模型不存在。请检查模型配置。`
          : `Trader "${traderName}" references an AI model that no longer exists. Please check the model configuration.`
      case 'trader.start.model_disabled':
        return zh
          ? `机器人「${traderName}」关联的 AI 模型「${modelName}」目前还没有启用。`
          : `Trader "${traderName}" uses AI model "${modelName}", which is currently disabled.`
      case 'trader.start.exchange_not_found':
        return zh
          ? `机器人「${traderName}」关联的交易所账户不存在。请检查交易所配置。`
          : `Trader "${traderName}" references an exchange account that no longer exists. Please check the exchange configuration.`
      case 'trader.start.exchange_disabled':
        return zh
          ? `机器人「${traderName}」关联的交易所账户「${exchangeName}」目前还没有启用。`
          : `Trader "${traderName}" uses exchange account "${exchangeName}", which is currently disabled.`
      case 'trader.start.setup_invalid':
      case 'trader.start.load_failed':
        return zh
          ? `机器人「${traderName}」暂时还不能启动，原因是：${reason}`
          : `Trader "${traderName}" cannot be started yet because ${reason}`
      default:
        return fallback
    }
  }
  const localizeTraderReason = (reasonKey?: string, fallback?: string) => {
    const zh = language === 'zh'

    switch (reasonKey) {
      case 'trader.reason.strategy_config_invalid':
        return zh
          ? '当前策略配置内容已损坏，系统暂时无法解析'
          : 'the current strategy configuration is corrupted and cannot be parsed'
      case 'trader.reason.strategy_missing':
        return zh
          ? '当前机器人缺少有效的交易策略配置'
          : 'the trader is missing a valid strategy configuration'
      case 'trader.reason.private_key_invalid':
        return zh
          ? '私钥格式不正确，系统无法识别'
          : 'the private key format is invalid and cannot be recognized'
      case 'trader.reason.hyperliquid_init_failed':
        return zh
          ? 'Hyperliquid 账户初始化失败，请确认私钥、主钱包地址和 Agent Wallet 配置是否正确'
          : 'Hyperliquid account initialization failed. Please verify the private key, main wallet address, and Agent Wallet configuration'
      case 'trader.reason.aster_init_failed':
        return zh
          ? 'Aster 账户初始化失败，请确认 Aster User、Signer 和私钥是否正确'
          : 'Aster account initialization failed. Please verify the Aster User, Signer, and private key'
      case 'trader.reason.exchange_meta_unavailable':
        return zh
          ? '系统暂时无法从交易所读取账户元信息'
          : 'the system could not read account metadata from the exchange'
      case 'trader.reason.hyperliquid_agent_balance_too_high':
        return zh
          ? 'Hyperliquid Agent Wallet 余额过高，不符合当前安全要求'
          : 'the Hyperliquid Agent Wallet balance is too high for the current safety requirements'
      case 'trader.reason.exchange_account_init_failed':
        return zh
          ? '交易所账户初始化失败，请确认钱包地址和 API Key 是否匹配'
          : 'exchange account initialization failed. Please verify that the wallet address and API key match'
      case 'trader.reason.exchange_unsupported':
        return zh
          ? '当前交易所类型暂不支持机器人初始化'
          : 'the selected exchange type is not currently supported for trader initialization'
      case 'trader.reason.exchange_balance_unavailable':
        return zh
          ? '系统暂时无法从交易所读取账户余额'
          : 'the system could not read the account balance from the exchange'
      case 'trader.reason.exchange_service_unreachable':
        return zh
          ? '系统暂时无法连接交易所服务'
          : 'the system could not reach the exchange service right now'
      default:
        return (
          fallback ||
          (zh
            ? '系统返回了一个未知错误'
            : 'an unknown error was returned by the system')
        )
    }
  }
  const normalizeActionableDescription = (
    error: unknown,
    message: string,
    title: string
  ) => {
    if (error instanceof ApiError && error.errorKey) {
      return formatActionableDescriptionByKey(
        error.errorKey,
        error.errorParams,
        message
      )
    }

    const prefixes = [
      '这次未能创建机器人：',
      '机器人创建失败：',
      '这次未能更新机器人：',
      '机器人更新失败：',
      '这次未能启动机器人：',
      'Failed to create trader:',
      'Failed to update trader:',
      'Unable to create trader:',
      'Unable to update trader:',
      'Unable to start trader:',
    ]

    let description = message.trim()
    if (description === title) return ''

    for (const prefix of prefixes) {
      if (description.startsWith(prefix)) {
        description = description.slice(prefix.length).trim()
        break
      }
    }

    return description
  }
  const showActionableError = (title: string, error: unknown) => {
    const message = getErrorMessage(error, title)
    const description = normalizeActionableDescription(error, message, title)

    if (description === '') {
      toast.error(title)
      return
    }

    toast.error(title, {
      description,
    })
  }
  const parseBalanceUsdc = (balance?: string) => {
    if (!balance) return null
    const parsed = Number.parseFloat(balance)
    return Number.isFinite(parsed) ? parsed : null
  }
  const getClaw402BalanceMessage = (balance: number, blocking: boolean) => {
    if (language === 'zh') {
      return blocking
        ? `当前 Claw402 钱包余额为 ${balance.toFixed(6)} USDC，AI 调用无法执行。请先为这个钱包充值，再重新点击启动。`
        : `当前 Claw402 钱包余额仅剩 ${balance.toFixed(6)} USDC，虽然还能尝试启动，但很快可能因为 AI 调用费用不足而停止。建议先补一点 USDC。`
    }

    return blocking
      ? `Your Claw402 wallet balance is ${balance.toFixed(6)} USDC. AI calls cannot run with zero balance. Please top up this wallet before starting again.`
      : `Your Claw402 wallet balance is only ${balance.toFixed(6)} USDC. You can still try to start, but AI calls may stop soon due to insufficient funds.`
  }
  const getClaw402BalanceIssue = (traderId: string) => {
    const trader = traders?.find((item) => item.trader_id === traderId)
    if (!trader) return null

    const model =
      allModels.find((item) => item.id === trader.ai_model) ||
      allModels.find((item) => item.provider === trader.ai_model)

    if (!model || model.provider !== 'claw402') return null

    const balance = parseBalanceUsdc(model.balanceUsdc)
    if (balance === null) return null
    if (balance <= 0) {
      return {
        blocking: true,
        title: language === 'zh' ? '启动失败' : 'Start failed',
        description: getClaw402BalanceMessage(balance, true),
      }
    }
    if (balance < 1) {
      return {
        blocking: false,
        title: language === 'zh' ? 'Claw402 余额偏低' : 'Low Claw402 balance',
        description: getClaw402BalanceMessage(balance, false),
      }
    }

    return null
  }

  const navigateInApp = (path: string) => {
    navigate(path)
  }

  // Toggle wallet address visibility for a trader
  const toggleTraderAddressVisibility = (traderId: string) => {
    setVisibleTraderAddresses((prev) => {
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
    setVisibleExchangeAddresses((prev) => {
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

  const {
    data: traders,
    mutate: mutateTraders,
    isLoading: isTradersLoading,
  } = useSWR<TraderInfo[]>(user && token ? 'traders' : null, api.getTraders, {
    refreshInterval: 5000,
  })
  const {
    data: exchangeAccountStateData,
    mutate: mutateExchangeAccountStates,
    isLoading: isExchangeAccountStatesLoading,
  } = useSWR<{ states: Record<string, ExchangeAccountState> }>(
    user && token ? 'exchange-account-state' : null,
    api.getExchangeAccountState,
    {
      refreshInterval: 30000,
      shouldRetryOnError: false,
    }
  )
  const { data: strategies } = useSWR<Strategy[]>(
    user && token ? 'strategies' : null,
    api.getStrategies,
    { refreshInterval: 30000 }
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
        const [modelConfigs, exchangeConfigs, models] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
        ])
        setAllModels(modelConfigs)
        const clawWalletAddress =
          modelConfigs.find((model) => model.provider === 'claw402')
            ?.walletAddress || null
        if (clawWalletAddress) {
          setBeginnerWalletAddress(clawWalletAddress)
          persistBeginnerWalletAddress(clawWalletAddress)
        }
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
  const enabledClaw402Model =
    enabledModels.find((model) => model.provider === 'claw402') || null
  const enabledClaw402Balance = parseBalanceUsdc(
    enabledClaw402Model?.balanceUsdc
  )
  const claw402BalanceAlert =
    enabledClaw402Model &&
    enabledClaw402Balance !== null &&
    enabledClaw402Balance < 1
      ? {
          blocking: enabledClaw402Balance <= 0,
          title:
            language === 'zh'
              ? enabledClaw402Balance <= 0
                ? 'Claw402 钱包余额为 0'
                : 'Claw402 钱包余额偏低'
              : enabledClaw402Balance <= 0
                ? 'Claw402 wallet balance is zero'
                : 'Claw402 wallet balance is low',
          description: getClaw402BalanceMessage(
            enabledClaw402Balance,
            enabledClaw402Balance <= 0
          ),
        }
      : null
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
    const usingTraders =
      traders?.filter((tr) => tr.exchange_id === exchangeId) || []
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
      const createdTrader = await api.createTrader(data)
      if (createdTrader.startup_warning) {
        toast.success(t('aiTradersToast.created', language), {
          description: createdTrader.startup_warning,
        })
      } else {
        toast.success(t('aiTradersToast.created', language))
      }
      setShowCreateModal(false)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      showActionableError(t('createTraderFailed', language), error)
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
        strategy_id: data.strategy_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        is_cross_margin: data.is_cross_margin,
        show_in_competition: data.show_in_competition,
      }

      await api.updateTrader(editingTrader.trader_id, request)
      toast.success(t('aiTradersToast.saved', language))
      setShowEditModal(false)
      setEditingTrader(null)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      showActionableError(t('updateTraderFailed', language), error)
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    {
      const ok = await confirmToast(t('confirmDeleteTrader', language))
      if (!ok) return
    }

    try {
      await api.deleteTrader(traderId)
      toast.success(t('aiTradersToast.deleted', language))

      await mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      toast.error(t('deleteTraderFailed', language))
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await api.stopTrader(traderId)
        toast.success(t('aiTradersToast.stopped', language))
      } else {
        const claw402Issue = getClaw402BalanceIssue(traderId)
        if (claw402Issue?.blocking) {
          toast.error(claw402Issue.title, {
            description: claw402Issue.description,
          })
          return
        }
        if (claw402Issue && !claw402Issue.blocking) {
          toast.warning(claw402Issue.title, {
            description: claw402Issue.description,
          })
        }
        await api.startTrader(traderId)
        toast.success(t('aiTradersToast.started', language))
      }

      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      showActionableError(
        running
          ? t('aiTradersToast.stopFailed', language)
          : t('aiTradersToast.startFailed', language),
        error
      )
    }
  }

  const handleToggleCompetition = async (
    traderId: string,
    currentShowInCompetition: boolean
  ) => {
    try {
      const newValue = !currentShowInCompetition
      await api.toggleCompetition(traderId, newValue)
      toast.success(
        newValue
          ? t('aiTradersToast.showInCompetition', language)
          : t('aiTradersToast.hideInCompetition', language)
      )

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
      await config.updateApi(request)
      toast.success(t('aiTradersToast.configUpdated', language))

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

      await api.updateModelConfigs(request)
      toast.success(t('aiTradersToast.modelConfigUpdated', language))

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
      await api.deleteExchange(exchangeId)
      toast.success(t('aiTradersToast.exchangeDeleted', language))

      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)
      await mutateExchangeAccountStates()

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

        await api.updateExchangeConfigsEncrypted(request)
        toast.success(t('aiTradersToast.exchangeConfigUpdated', language))
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

        await api.createExchangeEncrypted(createRequest)
        toast.success(t('aiTradersToast.exchangeCreated', language))
      }

      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)
      await mutateExchangeAccountStates()

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

  const handleQuickSetupClaw402 = async () => {
    if (quickSetupLoading) return

    try {
      setQuickSetupLoading(true)
      const result = await api.prepareBeginnerOnboarding()
      setBeginnerWalletAddress(result.address)
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)
      toast.success(
        language === 'zh'
          ? 'Claw402 已默认配置为 DeepSeek'
          : 'Claw402 is configured with DeepSeek by default'
      )
    } catch (error) {
      console.error('Failed to quick setup claw402:', error)
      toast.error(
        language === 'zh'
          ? '一键配置 Claw402 失败'
          : 'Failed to quick setup Claw402'
      )
    } finally {
      setQuickSetupLoading(false)
    }
  }

  const claw402Configured = configuredModels.some(
    (model) => model.provider === 'claw402'
  )
  const hasStrategies = (strategies?.length || 0) > 0
  const hasCreatedTrader = (traders?.length || 0) > 0
  const canCreateTrader =
    configuredModels.length > 0 && configuredExchanges.length > 0

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
              disabled={
                configuredModels.length === 0 ||
                configuredExchanges.length === 0
              }
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

        {isBeginnerMode ? (
          <BeginnerGuideCards
            language={language}
            claw402Ready={claw402Configured}
            exchangeReady={configuredExchanges.length > 0}
            strategyReady={hasStrategies}
            traderReady={hasCreatedTrader}
            canCreateTrader={canCreateTrader}
            walletAddress={beginnerWalletAddress}
            onQuickSetupClaw402={handleQuickSetupClaw402}
            onOpenExchange={handleAddExchange}
            onOpenStrategy={() => navigateInApp('/strategy')}
            onCreateTrader={() => setShowCreateModal(true)}
          />
        ) : null}

        {claw402BalanceAlert ? (
          <div
            className="mb-6 rounded-xl border px-4 py-4 md:px-5 md:py-4 flex flex-col md:flex-row md:items-start md:justify-between gap-3"
            style={{
              borderColor: claw402BalanceAlert.blocking
                ? 'rgba(239, 68, 68, 0.55)'
                : 'rgba(245, 158, 11, 0.45)',
              background: claw402BalanceAlert.blocking
                ? 'rgba(127, 29, 29, 0.22)'
                : 'rgba(120, 53, 15, 0.18)',
            }}
          >
            <div className="flex items-start gap-3">
              <div
                className="mt-0.5 rounded-full p-2"
                style={{
                  background: claw402BalanceAlert.blocking
                    ? 'rgba(239, 68, 68, 0.16)'
                    : 'rgba(245, 158, 11, 0.14)',
                  color: claw402BalanceAlert.blocking ? '#F87171' : '#FBBF24',
                }}
              >
                <AlertTriangle className="w-4 h-4" />
              </div>
              <div>
                <div
                  className="text-sm font-semibold"
                  style={{
                    color: claw402BalanceAlert.blocking ? '#FCA5A5' : '#FDE68A',
                  }}
                >
                  {claw402BalanceAlert.title}
                </div>
                <div
                  className="text-sm mt-1 leading-6"
                  style={{ color: '#D4D4D8' }}
                >
                  {claw402BalanceAlert.description}
                </div>
              </div>
            </div>

            <button
              type="button"
              onClick={() =>
                enabledClaw402Model && handleModelClick(enabledClaw402Model.id)
              }
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider border whitespace-nowrap self-start"
              style={{
                borderColor: claw402BalanceAlert.blocking
                  ? 'rgba(248, 113, 113, 0.45)'
                  : 'rgba(251, 191, 36, 0.35)',
                color: claw402BalanceAlert.blocking ? '#FCA5A5' : '#FDE68A',
                background: 'rgba(0, 0, 0, 0.18)',
              }}
            >
              {language === 'zh' ? '查看 AI 钱包' : 'Open AI wallet'}
            </button>
          </div>
        ) : null}

        {/* Configuration Status Grid */}
        <ConfigStatusGrid
          configuredModels={configuredModels}
          configuredExchanges={configuredExchanges}
          exchangeAccountStates={exchangeAccountStateData?.states}
          isExchangeAccountStatesLoading={isExchangeAccountStatesLoading}
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
          onNavigate={navigateInApp}
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
