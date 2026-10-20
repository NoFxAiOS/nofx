import { PaperBadge } from './PaperBadge'
import {
  Brain,
  Landmark,
  Eye,
  EyeOff,
  Copy,
  Check,
  Pencil,
  Trash2,
} from 'lucide-react'
import type { AIModel, Exchange, ExchangeAccountState } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { getModelIcon } from '../common/ModelIcons'
import { getExchangeIcon } from '../common/ExchangeIcons'
import { Badge } from '../ui/badge'
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
  isModelInUse: (modelId: string) => boolean | undefined
  getModelUsageInfo: (modelId: string) => UsageInfo
  isExchangeInUse: (exchangeId: string) => boolean | undefined
  getExchangeUsageInfo: (exchangeId: string) => UsageInfo
  onModelClick: (modelId: string) => void
  onModelDelete: (modelId: string) => void
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
  isModelInUse,
  getModelUsageInfo,
  isExchangeInUse,
  getExchangeUsageInfo,
  onModelClick,
  onModelDelete,
  onExchangeClick,
  onToggleExchangeAddress,
  onCopyAddress,
}: ConfigStatusGridProps) {
  const getExchangeStateMeta = (state: ExchangeAccountState | undefined) => {
    if (!state) {
      return {
        label: language === 'zh' ? '未检查' : 'Not checked',
        className: 'text-fg-3 border-line-strong/80 bg-surface/40',
      }
    }

    switch (state.status) {
      case 'ok':
        return {
          label: state.display_balance || '0',
          className: 'text-up border-up/30 bg-up-soft',
        }
      case 'disabled':
        return {
          label: language === 'zh' ? '已禁用' : 'Disabled',
          className: 'text-fg-3 border-line-strong/80 bg-surface/40',
        }
      case 'missing_credentials':
        return {
          label: language === 'zh' ? '配置不完整' : 'Incomplete',
          className: 'text-warn border-warn/30 bg-warn-soft',
        }
      case 'invalid_credentials':
        return {
          label: language === 'zh' ? '密钥无效' : 'Invalid keys',
          className: 'text-down border-down/30 bg-down-soft',
        }
      case 'permission_denied':
        return {
          label: language === 'zh' ? '无余额权限' : 'No permission',
          className: 'text-warn border-warn/30 bg-warn-soft',
        }
      default:
        return {
          label: language === 'zh' ? '暂时无法获取' : 'Unavailable',
          className: 'text-fg-2 border-line-strong/60 bg-surface-hover/50',
        }
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* AI Models Card */}
      <div className="nofx-glass rounded-lg border border-nofx-line/50 overflow-hidden">
        <div className="px-4 py-3 border-b border-nofx-line/50 bg-surface-2 flex items-center gap-2 ">
          <Brain className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-semibold text-fg">
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
                className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${
                  inUse ? 'opacity-80' : 'hover:bg-fg/5 hover:border-nofx-line'
                } bg-surface-2`}
              >
                <div className="flex items-center gap-4">
                  <div className="relative">
                    <div className="w-10 h-10 rounded-full flex items-center justify-center bg-surface-2 border border-nofx-line relative z-10">
                      {getModelIcon(model.provider || model.id, {
                        width: 20,
                        height: 20,
                      }) || (
                        <span className="text-xs font-bold text-ai">
                          {getShortName(model.name)[0]}
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="text-sm font-medium text-fg group-hover:text-brand transition-colors">
                      {getShortName(model.name)}
                    </div>
                    <div className="text-[11px] text-fg-3 flex items-center gap-2">
                      {model.customModelName ||
                        AI_PROVIDER_CONFIG[model.provider]?.defaultModel ||
                        ''}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-1.5">
                  {/* Edit / delete controls */}
                  <button
                    type="button"
                    title={language === 'zh' ? '编辑' : 'Edit'}
                    onClick={(e) => {
                      e.stopPropagation()
                      onModelClick(model.id)
                    }}
                    className="p-1.5 rounded-md text-fg-3 hover:text-nofx-gold hover:bg-fg/5 transition-all"
                  >
                    <Pencil className="w-3.5 h-3.5" />
                  </button>
                  <button
                    type="button"
                    title={language === 'zh' ? '删除' : 'Delete'}
                    disabled={inUse}
                    onClick={(e) => {
                      e.stopPropagation()
                      if (!inUse) onModelDelete(model.id)
                    }}
                    className={`p-1.5 rounded-md transition-all ${
                      inUse
                        ? 'text-fg-3 cursor-not-allowed'
                        : 'text-fg-3 hover:text-down hover:bg-down-soft'
                    }`}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                  <div className="text-right">
                    {usageInfo.totalCount > 0 ? (
                      <Badge
                        size="xs"
                        variant={usageInfo.runningCount > 0 ? 'up' : 'warn'}
                      >
                        {t('tradersRunningOfTotal', language, {
                          running: usageInfo.runningCount,
                          total: usageInfo.totalCount,
                        })}
                      </Badge>
                    ) : (
                      <span className="text-[11px] text-fg-3">
                        {t('configStandby', language)}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            )
          })}

          {configuredModels.length === 0 && (
            <div className="text-center py-10 border border-dashed border-line rounded-lg bg-surface-2">
              <Brain className="w-8 h-8 mx-auto mb-3 text-fg-3" />
              <div className="text-xs text-fg-3">
                {t('noModelsConfigured', language)}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Exchanges Card */}
      <div className="nofx-glass rounded-lg border border-nofx-line/50 overflow-hidden">
        <div className="px-4 py-3 border-b border-nofx-line/50 bg-surface-2 flex items-center gap-2 ">
          <Landmark className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-semibold text-fg">
            {t('exchanges', language)}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredExchanges.map((exchange) => {
            const inUse = isExchangeInUse(exchange.id)
            const usageInfo = getExchangeUsageInfo(exchange.id)
            const state = exchangeAccountStates?.[exchange.id]
            const stateMeta = getExchangeStateMeta(state)
            return (
              <div
                key={exchange.id}
                className={`group relative flex items-center justify-between p-3 rounded-md transition-all border border-transparent ${
                  inUse
                    ? 'opacity-80'
                    : 'hover:bg-fg/5 hover:border-nofx-line cursor-pointer'
                } bg-surface-2`}
                onClick={() => onExchangeClick(exchange.id)}
              >
                <div className="flex items-center gap-4 min-w-0">
                  <div className="relative">
                    <div className="w-10 h-10 rounded-full flex items-center justify-center bg-surface-2 border border-nofx-line relative z-10">
                      {getExchangeIcon(exchange.exchange_type || exchange.id, {
                        width: 20,
                        height: 20,
                      })}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="text-sm font-medium text-fg group-hover:text-brand transition-colors truncate">
                      {exchange.exchange_type?.toUpperCase() ||
                        getShortName(exchange.name)}
                      <PaperBadge
                        exchangeType={exchange.exchange_type}
                        language={language}
                      />
                      <span className="text-[11px] font-normal text-fg-3 ml-2 border border-line px-1 rounded">
                        {exchange.account_name || t('defaultAccount', language)}
                      </span>
                    </div>
                    <div className="text-[11px] text-fg-3 flex items-center gap-2">
                      {exchange.type?.toUpperCase() || 'CEX'}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px]">
                      <span
                        className={`rounded border px-1.5 py-0.5 ${state?.status === 'ok' ? 'num' : ''} ${stateMeta.className}`}
                      >
                        {isExchangeAccountStatesLoading && !state
                          ? language === 'zh'
                            ? '检查中...'
                            : 'Checking...'
                          : stateMeta.label}
                      </span>
                      {state?.status !== 'ok' && state?.error_message ? (
                        <span className="text-fg-3 truncate max-w-[220px]">
                          {state.error_message}
                        </span>
                      ) : null}
                    </div>
                  </div>
                </div>

                <div className="flex flex-col items-end gap-1">
                  {/* Wallet Address Display Logic */}
                  {(() => {
                    const walletAddr =
                      exchange.hyperliquidWalletAddr ||
                      exchange.asterUser ||
                      exchange.lighterWalletAddr
                    if (exchange.type !== 'dex' || !walletAddr) return null
                    const isVisible = visibleExchangeAddresses.has(exchange.id)
                    const isCopied = copiedId === `exchange-${exchange.id}`

                    return (
                      <div
                        className="flex items-center gap-1"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <span className="text-[10px] font-mono text-fg-3 bg-surface-hover px-1.5 py-0.5 rounded border border-line">
                          {isVisible ? walletAddr : truncateAddress(walletAddr)}
                        </span>
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            onToggleExchangeAddress(exchange.id)
                          }}
                          className="text-fg-3 hover:text-fg-2"
                        >
                          {isVisible ? <EyeOff size={10} /> : <Eye size={10} />}
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            onCopyAddress(`exchange-${exchange.id}`, walletAddr)
                          }}
                          className="text-fg-3 hover:text-nofx-gold"
                        >
                          {isCopied ? (
                            <Check size={10} className="text-up" />
                          ) : (
                            <Copy size={10} />
                          )}
                        </button>
                      </div>
                    )
                  })()}

                  {usageInfo.totalCount > 0 ? (
                    <Badge
                      size="xs"
                      variant={usageInfo.runningCount > 0 ? 'up' : 'warn'}
                    >
                      {t('tradersRunningOfTotal', language, {
                        running: usageInfo.runningCount,
                        total: usageInfo.totalCount,
                      })}
                    </Badge>
                  ) : (
                    <span className="text-[11px] text-fg-3">
                      {t('configStandby', language)}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
          {configuredExchanges.length === 0 && (
            <div className="text-center py-10 border border-dashed border-line rounded-lg bg-surface-2">
              <Landmark className="w-8 h-8 mx-auto mb-3 text-fg-3" />
              <div className="text-xs text-fg-3">
                {t('noExchangesConfigured', language)}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
