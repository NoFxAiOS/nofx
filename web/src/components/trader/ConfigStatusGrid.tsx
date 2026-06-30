import {
  Brain,
  Landmark,
  Eye,
  EyeOff,
  Copy,
  Check,
} from 'lucide-react'
import type { AIModel, Exchange, ExchangeAccountState } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { getModelIcon } from '../common/ModelIcons'
import { getExchangeIcon } from '../common/ExchangeIcons'
import {
  getShortName,
  AI_PROVIDER_CONFIG,
  truncateAddress,
} from './model-constants'

interface UsageInfo {
  runningCount: number
  totalCount: number
}

interface ConfigStatusGridProps {
  configuredModels: AIModel[]
  configuredExchanges: Exchange[]
  exchangeAccountStates?: Record<string, ExchangeAccountState>
  isExchangeAccountStatesLoading?: boolean
  visibleExchangeAddresses: Set<string>
  copiedId: string | null
  language: Language
  getModelUsageInfo: (modelId: string) => UsageInfo
  getExchangeUsageInfo: (exchangeId: string) => UsageInfo
  onModelClick: (modelId: string) => void
  onExchangeClick: (exchangeId: string) => void
  onToggleExchangeAddress: (exchangeId: string) => void
  onCopyAddress: (id: string, address: string) => void
}

export function ConfigStatusGrid({
  configuredModels,
  configuredExchanges,
  exchangeAccountStates,
  isExchangeAccountStatesLoading,
  visibleExchangeAddresses,
  copiedId,
  language,
  getModelUsageInfo,
  getExchangeUsageInfo,
  onModelClick,
  onExchangeClick,
  onToggleExchangeAddress,
  onCopyAddress,
}: ConfigStatusGridProps) {
  const getExchangeStateMeta = (state: ExchangeAccountState | undefined) => {
    if (!state) {
      return {
        label: language === 'zh' ? 'NOT CHECKED' : 'NOT CHECKED',
        className: 'text-nofx-text-muted border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper',
      }
    }

    switch (state.status) {
      case 'ok':
        return {
          label: state.display_balance || '0',
          className: 'text-nofx-success border-nofx-success/20 bg-nofx-success/10',
        }
      case 'disabled':
        return {
          label: language === 'zh' ? 'DISABLED' : 'DISABLED',
          className: 'text-nofx-text-muted border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper',
        }
      case 'missing_credentials':
        return {
          label: language === 'zh' ? 'INCOMPLETE' : 'INCOMPLETE',
          className: 'text-nofx-gold border-nofx-gold/20 bg-nofx-gold/10',
        }
      case 'invalid_credentials':
        return {
          label: language === 'zh' ? 'INVALID KEYS' : 'INVALID KEYS',
          className: 'text-nofx-danger border-nofx-danger/20 bg-nofx-danger/10',
        }
      case 'permission_denied':
        return {
          label: language === 'zh' ? 'NO PERMISSION' : 'NO PERMISSION',
          className: 'text-nofx-gold border-nofx-gold/20 bg-nofx-gold/10',
        }
      default:
        return {
          label: language === 'zh' ? 'UNAVAILABLE' : 'UNAVAILABLE',
          className: 'text-nofx-text-muted border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper',
        }
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* AI Models Card */}
      <div className="bg-nofx-bg-lighter rounded-lg border border-[rgba(26,24,19,0.14)] overflow-hidden">
        <div className="px-4 py-3 border-b border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper flex items-center gap-2">
          <Brain className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-mono tracking-widest text-nofx-text uppercase">
            {t('aiModels', language)}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredModels.map((model) => {
            const usageInfo = getModelUsageInfo(model.id)
            return (
              <div
                key={model.id}
                role="button"
                tabIndex={0}
                className="group relative flex cursor-pointer items-center justify-between rounded-md border border-transparent bg-nofx-bg-deeper p-3 transition-all hover:border-nofx-gold/20 hover:bg-nofx-bg"
                onClick={() => onModelClick(model.id)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    onModelClick(model.id)
                  }
                }}
              >
                <div className="flex items-center gap-4">
                  <div className="relative">
                    <div className="w-10 h-10 rounded-full flex items-center justify-center bg-nofx-bg border border-[rgba(26,24,19,0.14)] relative z-10">
                      {getModelIcon(model.provider || model.id, { width: 20, height: 20 }) || (
                        <span className="text-xs font-bold text-nofx-accent">{getShortName(model.name)[0]}</span>
                      )}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="font-mono text-sm text-nofx-text group-hover:text-nofx-gold transition-colors">
                      {getShortName(model.name)}
                    </div>
                    <div className="text-[10px] text-nofx-text-muted font-mono flex items-center gap-2">
                      {model.customModelName || AI_PROVIDER_CONFIG[model.provider]?.defaultModel || ''}
                    </div>
                    {model.provider === 'claw402' && (model.balanceUsdc || model.walletAddress) ? (
                      <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[10px] font-mono">
                        {model.balanceUsdc ? (
                          <span className="rounded border border-nofx-success/20 bg-nofx-success/10 px-1.5 py-0.5 text-nofx-success">
                            {model.balanceUsdc} USDC
                          </span>
                        ) : null}
                        {model.walletAddress ? (
                          <span className="rounded border border-nofx-gold/20 bg-nofx-gold/10 px-1.5 py-0.5 text-nofx-gold">
                            {truncateAddress(model.walletAddress)}
                          </span>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </div>

                <div className="text-right">
                  {usageInfo.totalCount > 0 ? (
                    <span className={`text-[10px] font-mono px-2 py-1 rounded border ${usageInfo.runningCount > 0
                      ? 'bg-nofx-success/10 border-nofx-success/30 text-nofx-success'
                      : 'bg-nofx-gold/10 border-nofx-gold/30 text-nofx-gold'
                      }`}>
                      {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                    </span>
                  ) : (
                    <span className="text-[10px] font-mono text-nofx-text-muted uppercase tracking-wider">
                      {language === 'zh' ? 'STANDBY' : 'STANDBY'}
                    </span>
                  )}
                </div>
              </div>
            )
          })}

          {configuredModels.length === 0 && (
            <div className="text-center py-10 border border-dashed border-[rgba(26,24,19,0.14)] rounded-lg bg-nofx-bg-deeper">
              <Brain className="w-8 h-8 mx-auto mb-3 text-nofx-text-muted" />
              <div className="text-xs font-mono text-nofx-text-muted uppercase tracking-widest">{t('noModelsConfigured', language)}</div>
            </div>
          )}
        </div>
      </div>

      {/* Exchanges Card */}
      <div className="bg-nofx-bg-lighter rounded-lg border border-[rgba(26,24,19,0.14)] overflow-hidden">
        <div className="px-4 py-3 border-b border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper flex items-center gap-2">
          <Landmark className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-mono tracking-widest text-nofx-text uppercase">
            {t('exchanges', language)}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredExchanges.map((exchange) => {
            const usageInfo = getExchangeUsageInfo(exchange.id)
            const state = exchangeAccountStates?.[exchange.id]
            const stateMeta = getExchangeStateMeta(state)
            return (
              <div
                key={exchange.id}
                role="button"
                tabIndex={0}
                className="group relative flex cursor-pointer items-center justify-between rounded-md border border-transparent bg-nofx-bg-deeper p-3 transition-all hover:border-nofx-gold/20 hover:bg-nofx-bg"
                onClick={() => onExchangeClick(exchange.id)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    onExchangeClick(exchange.id)
                  }
                }}
              >
                <div className="flex items-center gap-4 min-w-0">
                  <div className="relative">
                    <div className="w-10 h-10 rounded-full flex items-center justify-center bg-nofx-bg border border-[rgba(26,24,19,0.14)] relative z-10">
                      {getExchangeIcon(exchange.exchange_type || exchange.id, { width: 20, height: 20 })}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="font-mono text-sm text-nofx-text group-hover:text-nofx-gold transition-colors truncate">
                      {exchange.exchange_type?.toUpperCase() || getShortName(exchange.name)}
                      <span className="text-[10px] text-nofx-text-muted ml-2 border border-[rgba(26,24,19,0.14)] px-1 rounded">
                        {exchange.account_name || 'DEFAULT'}
                      </span>
                    </div>
                    <div className="text-[10px] text-nofx-text-muted font-mono flex items-center gap-2">
                      {exchange.type?.toUpperCase() || 'CEX'}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] font-mono">
                      <span className={`rounded border px-1.5 py-0.5 ${stateMeta.className}`}>
                        {isExchangeAccountStatesLoading && !state
                          ? (language === 'zh' ? 'CHECKING...' : 'CHECKING...')
                          : stateMeta.label}
                      </span>
                      {state?.status !== 'ok' && state?.error_message ? (
                        <span className="text-nofx-text-muted truncate max-w-[220px]">
                          {state.error_message}
                        </span>
                      ) : null}
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
                        <span className="text-[10px] font-mono text-nofx-text-muted bg-nofx-bg-deeper px-1.5 py-0.5 rounded border border-[rgba(26,24,19,0.14)]">
                          {isVisible ? walletAddr : truncateAddress(walletAddr)}
                        </span>
                        <button
                          onClick={(e) => { e.stopPropagation(); onToggleExchangeAddress(exchange.id) }}
                          className="text-nofx-text-muted hover:text-nofx-text"
                        >
                          {isVisible ? <EyeOff size={10} /> : <Eye size={10} />}
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); onCopyAddress(`exchange-${exchange.id}`, walletAddr) }}
                          className="text-nofx-text-muted hover:text-nofx-gold"
                        >
                          {isCopied ? <Check size={10} className="text-nofx-success" /> : <Copy size={10} />}
                        </button>
                      </div>
                    )
                  })()}

                  {usageInfo.totalCount > 0 ? (
                    <span className={`text-[10px] font-mono px-2 py-1 rounded border ${usageInfo.runningCount > 0
                      ? 'bg-nofx-success/10 border-nofx-success/30 text-nofx-success'
                      : 'bg-nofx-gold/10 border-nofx-gold/30 text-nofx-gold'
                      }`}>
                      {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                    </span>
                  ) : (
                    <span className="text-[10px] font-mono text-nofx-text-muted uppercase tracking-wider">
                      {language === 'zh' ? 'STANDBY' : 'STANDBY'}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
          {configuredExchanges.length === 0 && (
            <div className="text-center py-10 border border-dashed border-[rgba(26,24,19,0.14)] rounded-lg bg-nofx-bg-deeper">
              <Landmark className="w-8 h-8 mx-auto mb-3 text-nofx-text-muted" />
              <div className="text-xs font-mono text-nofx-text-muted uppercase tracking-widest">{t('noExchangesConfigured', language)}</div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
