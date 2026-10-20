import { useState, useEffect } from 'react'
import type {
  AIModel,
  Exchange,
  CreateTraderRequest,
  ExchangeAccountStateResponse,
  Strategy,
} from '../../types'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { toast } from 'sonner'
import {
  Pencil,
  Plus,
  X as IconX,
  Sparkles,
  ExternalLink,
  UserPlus,
} from 'lucide-react'
import { httpClient } from '../../lib/httpClient'
import { NofxSelect } from '../ui/select'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

// 交易所注册链接配置
const EXCHANGE_REGISTRATION_LINKS: Record<
  string,
  { url: string; hasReferral?: boolean }
> = {
  binance: {
    url: 'https://www.binance.com/join?ref=NOFXENG',
    hasReferral: true,
  },
  okx: { url: 'https://www.okx.com/join/1865360', hasReferral: true },
  bybit: { url: 'https://partner.bybit.com/b/83856', hasReferral: true },
  hyperliquid: {
    url: 'https://app.hyperliquid.xyz/join/AITRADING',
    hasReferral: true,
  },
  aster: {
    url: 'https://www.asterdex.com/en/referral/fdfc0e',
    hasReferral: true,
  },
  lighter: {
    url: 'https://app.lighter.xyz/?referral=68151432',
    hasReferral: true,
  },
}

import type { TraderConfigData } from '../../types'

// 表单内部状态类型
interface FormState {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id: string
  is_cross_margin: boolean
  show_in_competition: boolean
  scan_interval_minutes: number
  initial_balance?: number
}

interface TraderConfigModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
  isEditMode?: boolean
  availableModels?: AIModel[]
  availableExchanges?: Exchange[]
  onSave?: (data: CreateTraderRequest) => Promise<void>
}

export function TraderConfigModal({
  isOpen,
  onClose,
  traderData,
  isEditMode = false,
  availableModels = [],
  availableExchanges = [],
  onSave,
}: TraderConfigModalProps) {
  const { language } = useLanguage()
  const [formData, setFormData] = useState<FormState>({
    trader_name: '',
    ai_model: '',
    exchange_id: '',
    strategy_id: '',
    is_cross_margin: true,
    show_in_competition: true,
    scan_interval_minutes: 3,
  })
  const [isSaving, setIsSaving] = useState(false)
  const [strategies, setStrategies] = useState<Strategy[]>([])
  const [isFetchingBalance, setIsFetchingBalance] = useState(false)
  const [balanceFetchError, setBalanceFetchError] = useState<string>('')

  // 获取用户的策略列表
  useEffect(() => {
    const fetchStrategies = async () => {
      try {
        const result = await httpClient.get<{ strategies: Strategy[] }>(
          '/api/strategies'
        )
        if (result.success && result.data?.strategies) {
          const strategyList = result.data.strategies
          setStrategies(strategyList)
          // 如果没有选择策略，默认选中激活的策略
          if (!formData.strategy_id && !isEditMode) {
            const activeStrategy = strategyList.find((s) => s.is_active)
            if (activeStrategy) {
              setFormData((prev) => ({
                ...prev,
                strategy_id: activeStrategy.id,
              }))
            } else if (strategyList.length > 0) {
              setFormData((prev) => ({
                ...prev,
                strategy_id: strategyList[0].id,
              }))
            }
          }
        }
      } catch (error) {
        console.error('Failed to fetch strategies:', error)
      }
    }
    if (isOpen) {
      fetchStrategies()
    }
  }, [isOpen])

  useEffect(() => {
    if (traderData) {
      setFormData({
        ...traderData,
        strategy_id: traderData.strategy_id || '',
      })
    } else if (!isEditMode) {
      setFormData({
        trader_name: '',
        ai_model: availableModels[0]?.id || '',
        exchange_id: availableExchanges[0]?.id || '',
        strategy_id: '',
        is_cross_margin: true,
        show_in_competition: true,
        scan_interval_minutes: 3,
      })
    }
  }, [traderData, isEditMode, availableModels, availableExchanges])

  if (!isOpen) return null

  const handleInputChange = (field: keyof FormState, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleExchangeChange = (exchangeId: string) => {
    setBalanceFetchError('')
    setFormData((prev) => {
      if (prev.exchange_id === exchangeId) {
        return prev
      }

      const next: FormState = { ...prev, exchange_id: exchangeId }

      // Exchange balance belongs to the selected exchange, not the trader record.
      // Clear the old baseline so we don't carry Exchange B's balance into Exchange A.
      if (isEditMode) {
        next.initial_balance = undefined
      }

      return next
    })
  }

  const handleFetchCurrentBalance = async () => {
    if (!isEditMode) {
      setBalanceFetchError(t('fetchBalanceEditModeOnly', language))
      return
    }

    if (!formData.exchange_id) {
      setBalanceFetchError(t('balanceFetchFailed', language))
      return
    }

    setIsFetchingBalance(true)
    setBalanceFetchError('')

    try {
      const result = await httpClient.get<ExchangeAccountStateResponse>(
        '/api/exchanges/account-state'
      )

      const selectedState = result.data?.states?.[formData.exchange_id]
      if (result.success && selectedState?.status === 'ok') {
        const currentBalance =
          selectedState.total_equity ?? selectedState.available_balance ?? 0
        setFormData((prev) => ({ ...prev, initial_balance: currentBalance }))
        toast.success(t('balanceFetched', language))
      } else {
        setBalanceFetchError(
          selectedState?.error_message ||
            result.message ||
            t('balanceFetchFailed', language)
        )
      }
    } catch (error) {
      console.error(t('balanceFetchFailed', language) + ':', error)
      setBalanceFetchError(
        error instanceof Error && error.message
          ? error.message
          : t('balanceFetchNetworkError', language)
      )
    } finally {
      setIsFetchingBalance(false)
    }
  }

  const handleSave = async () => {
    if (!onSave) return

    setIsSaving(true)
    try {
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        strategy_id: formData.strategy_id,
        is_cross_margin: formData.is_cross_margin,
        show_in_competition: formData.show_in_competition,
        scan_interval_minutes: formData.scan_interval_minutes,
      }

      // 只在编辑模式时包含initial_balance
      if (isEditMode && formData.initial_balance !== undefined) {
        saveData.initial_balance = formData.initial_balance
      }

      await onSave(saveData)
    } catch (error) {
      console.error(t('saveFailed', language) + ':', error)
    } finally {
      setIsSaving(false)
    }
  }

  const selectedStrategy = strategies.find((s) => s.id === formData.strategy_id)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-surface-2 bg-opacity-50 p-4 overflow-y-auto">
      <div
        className="bg-surface-hover border border-line rounded-xl shadow-2xl max-w-2xl w-full my-8"
        style={{ maxHeight: 'calc(100vh - 4rem)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-line bg-gradient-to-r from-surface-hover to-line sticky top-0 z-10 rounded-t-xl">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-brand to-brand flex items-center justify-center text-brand-fg">
              {isEditMode ? (
                <Pencil className="w-5 h-5" />
              ) : (
                <Plus className="w-5 h-5" />
              )}
            </div>
            <div>
              <h2 className="text-xl font-bold text-fg">
                {isEditMode
                  ? t('editTrader', language)
                  : t('createTrader', language)}
              </h2>
              <p className="text-sm text-fg-3 mt-1">
                {isEditMode
                  ? t('editTraderConfig', language)
                  : t('selectStrategyAndConfigParams', language)}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-fg-3 hover:text-fg hover:bg-line transition-colors flex items-center justify-center"
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div
          className="p-6 space-y-6 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Basic Info */}
          <div className="bg-surface-2 border border-line rounded-lg p-5">
            <h3 className="text-lg font-semibold text-fg mb-5 flex items-center gap-2">
              <span className="text-brand">1</span> {t('basicConfig', language)}
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-fg block mb-2">
                  {t('traderNameRequired', language)}
                </label>
                <input
                  type="text"
                  value={formData.trader_name}
                  onChange={(e) =>
                    handleInputChange('trader_name', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg focus:border-brand focus:outline-none"
                  placeholder={t('enterTraderNamePlaceholder', language)}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-fg block mb-2">
                    {t('aiModelRequired', language)}
                  </label>
                  <NofxSelect
                    value={formData.ai_model}
                    onChange={(val) => handleInputChange('ai_model', val)}
                    className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg"
                    options={availableModels.map((model) => ({
                      value: model.id,
                      label: getShortName(model.name || model.id).toUpperCase(),
                    }))}
                  />
                </div>
                <div>
                  <label className="text-sm text-fg block mb-2">
                    {t('exchangeRequired', language)}
                  </label>
                  <NofxSelect
                    value={formData.exchange_id}
                    onChange={handleExchangeChange}
                    className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg"
                    options={availableExchanges.map((exchange) => ({
                      value: exchange.id,
                      label:
                        getShortName(
                          exchange.name || exchange.exchange_type || exchange.id
                        ).toUpperCase() +
                        (exchange.exchange_type === 'paper'
                          ? ` [${t('paper.badge', language)}]`
                          : '') +
                        (exchange.account_name
                          ? ` - ${exchange.account_name}`
                          : ''),
                    }))}
                  />
                  {/* Exchange Registration Link */}
                  {formData.exchange_id &&
                    (() => {
                      // Find the selected exchange to get its type
                      const selectedExchange = availableExchanges.find(
                        (e) => e.id === formData.exchange_id
                      )
                      const exchangeType =
                        selectedExchange?.exchange_type?.toLowerCase() || ''
                      const regLink = EXCHANGE_REGISTRATION_LINKS[exchangeType]
                      if (!regLink) return null
                      return (
                        <a
                          href={regLink.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="mt-2 inline-flex items-center gap-1.5 text-xs text-fg-3 hover:text-brand transition-colors"
                        >
                          <UserPlus className="w-3.5 h-3.5" />
                          <span>{t('noExchangeAccount', language)}</span>
                          {regLink.hasReferral && (
                            <span className="px-1.5 py-0.5 bg-brand/10 text-brand rounded text-[10px]">
                              {t('discount', language)}
                            </span>
                          )}
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      )
                    })()}
                </div>
              </div>
            </div>
          </div>

          {/* Strategy Selection */}
          <div className="bg-surface-2 border border-line rounded-lg p-5">
            <h3 className="text-lg font-semibold text-fg mb-5 flex items-center gap-2">
              <span className="text-brand">2</span>{' '}
              {t('selectTradingStrategy', language)}
              <Sparkles className="w-4 h-4 text-brand" />
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-fg block mb-2">
                  {t('useStrategy', language)}
                </label>
                <NofxSelect
                  value={formData.strategy_id}
                  onChange={(val) => handleInputChange('strategy_id', val)}
                  className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg"
                  options={[
                    { value: '', label: t('noStrategyManual', language) },
                    ...strategies.map((strategy) => ({
                      value: strategy.id,
                      label:
                        strategy.name +
                        (strategy.is_active
                          ? t('strategyActive', language)
                          : '') +
                        (strategy.is_default
                          ? t('strategyDefault', language)
                          : ''),
                    })),
                  ]}
                />
                {strategies.length === 0 && (
                  <p className="text-xs text-fg-3 mt-2">
                    {t('noStrategyHint', language)}
                  </p>
                )}
              </div>

              {/* Strategy Preview */}
              {selectedStrategy && (
                <div className="mt-3 p-4 bg-surface-hover border border-line rounded-lg">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-brand text-sm font-medium">
                      {t('strategyDetails', language)}
                    </span>
                    {selectedStrategy.is_active && (
                      <span className="px-2 py-0.5 bg-up-soft text-up text-xs rounded">
                        {t('activating', language)}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-fg-3 mb-2">
                    {selectedStrategy.description ||
                      (language === 'zh' ? '无描述' : 'No description')}
                  </p>
                  <div className="grid grid-cols-2 gap-2 text-xs text-fg-3">
                    <div>
                      {t('coinSource', language)}:{' '}
                      {(() => {
                        const cs = selectedStrategy.config.coin_source
                        const labels: Record<string, string> = {
                          static: '固定币种',
                          ai500: 'AI500',
                          oi_top: 'OI Top',
                          oi_low: 'OI Low',
                          piggy_dash: '猪猪冲刺',
                          short_scan: '做空扫描',
                          mixed: '混合',
                        }
                        if (cs.source_type === 'mixed') {
                          let n = 0
                          if (cs.use_ai500) n++
                          if (cs.use_oi_top) n++
                          if (cs.use_oi_low) n++
                          if (cs.use_piggy_dash) n++
                          if (cs.use_short_scan) n++
                          if (
                            cs.use_static !== false &&
                            (cs.static_coins || []).length > 0
                          )
                            n++
                          return `混合(${n}源)`
                        }
                        return labels[cs.source_type] || cs.source_type
                      })()}
                    </div>
                    <div>
                      {t('marginLimit', language)}:{' '}
                      {(
                        (selectedStrategy.config.risk_control
                          ?.max_margin_usage || 0.9) * 100
                      ).toFixed(0)}
                      %
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Trading Parameters */}
          <div className="bg-surface-2 border border-line rounded-lg p-5">
            <h3 className="text-lg font-semibold text-fg mb-5 flex items-center gap-2">
              <span className="text-brand">3</span>{' '}
              {t('tradingParams', language)}
            </h3>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-fg block mb-2">
                    {t('marginMode', language)}
                  </label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', true)}
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        formData.is_cross_margin
                          ? 'bg-brand text-brand-fg'
                          : 'bg-surface-2 text-fg-3 border border-line'
                      }`}
                    >
                      {t('crossMargin', language)}
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        handleInputChange('is_cross_margin', false)
                      }
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        !formData.is_cross_margin
                          ? 'bg-brand text-brand-fg'
                          : 'bg-surface-2 text-fg-3 border border-line'
                      }`}
                    >
                      {t('isolatedMargin', language)}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="text-sm text-fg block mb-2">
                    {t('aiScanInterval', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.scan_interval_minutes}
                    onChange={(e) => {
                      const parsedValue = Number(e.target.value)
                      const safeValue = Number.isFinite(parsedValue)
                        ? Math.max(3, parsedValue)
                        : 3
                      handleInputChange('scan_interval_minutes', safeValue)
                    }}
                    className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg focus:border-brand focus:outline-none"
                    min="3"
                    max="60"
                    step="1"
                  />
                  <p className="text-xs text-fg-3 mt-1">
                    {t('scanIntervalRecommend', language)}
                  </p>
                </div>
              </div>

              {/* Competition visibility */}
              <div>
                <label className="text-sm text-fg block mb-2">
                  {t('competitionDisplay', language)}
                </label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() =>
                      handleInputChange('show_in_competition', true)
                    }
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      formData.show_in_competition
                        ? 'bg-brand text-brand-fg'
                        : 'bg-surface-2 text-fg-3 border border-line'
                    }`}
                  >
                    {t('show', language)}
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      handleInputChange('show_in_competition', false)
                    }
                    className={`flex-1 px-3 py-2 rounded text-sm ${
                      !formData.show_in_competition
                        ? 'bg-brand text-brand-fg'
                        : 'bg-surface-2 text-fg-3 border border-line'
                    }`}
                  >
                    {t('hide', language)}
                  </button>
                </div>
                <p className="text-xs text-fg-3 mt-1">
                  {t('hiddenInCompetition', language)}
                </p>
              </div>

              {/* Initial Balance (Edit mode only) */}
              {isEditMode && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="text-sm text-fg">
                      {t('initialBalanceLabel', language)}
                    </label>
                    <button
                      type="button"
                      onClick={handleFetchCurrentBalance}
                      disabled={isFetchingBalance}
                      className="px-3 py-1 text-xs bg-brand text-brand-fg rounded hover:bg-brand transition-colors disabled:bg-fg-3 disabled:cursor-not-allowed"
                    >
                      {isFetchingBalance
                        ? t('fetching', language)
                        : t('fetchCurrentBalance', language)}
                    </button>
                  </div>
                  <input
                    type="number"
                    value={formData.initial_balance || 0}
                    onChange={(e) =>
                      handleInputChange(
                        'initial_balance',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 bg-surface-2 border border-line rounded text-fg focus:border-brand focus:outline-none"
                    min="100"
                    step="0.01"
                  />
                  <p className="text-xs text-fg-3 mt-1">
                    {t('balanceUpdateHint', language)}
                  </p>
                  {balanceFetchError && (
                    <p className="text-xs text-down mt-1">
                      {balanceFetchError}
                    </p>
                  )}
                </div>
              )}

              {/* Create mode info */}
              {!isEditMode && (
                <div className="p-3 bg-surface-hover border border-line rounded flex items-center gap-2">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="w-4 h-4 text-brand"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <circle cx="12" cy="12" r="10" />
                    <line x1="12" x2="12" y1="8" y2="12" />
                    <line x1="12" x2="12.01" y1="16" y2="16" />
                  </svg>
                  <span className="text-sm text-fg-3">
                    {t('autoFetchBalanceInfo', language)}
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-line bg-gradient-to-r from-surface-hover to-line sticky bottom-0 z-10 rounded-b-xl">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-line text-fg rounded-lg hover:bg-line transition-all duration-200 border border-line-strong"
          >
            {t('cancel', language)}
          </button>
          {onSave && (
            <button
              onClick={handleSave}
              disabled={
                isSaving ||
                !formData.trader_name ||
                !formData.ai_model ||
                !formData.exchange_id
              }
              className="px-8 py-3 bg-gradient-to-r from-brand to-brand text-brand-fg rounded-lg hover:from-brand hover:to-brand transition-all duration-200 disabled:bg-fg-3 disabled:cursor-not-allowed font-medium shadow-lg"
            >
              {isSaving
                ? t('saving', language)
                : isEditMode
                  ? t('editTrader', language)
                  : t('createTraderButton', language)}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
