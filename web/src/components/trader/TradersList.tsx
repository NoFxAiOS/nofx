import { PaperBadge } from './PaperBadge'
import {
  Bot,
  Users,
  BarChart3,
  Trash2,
  Pencil,
  Eye,
  EyeOff,
  Copy,
  Check,
} from 'lucide-react'
import type { TraderInfo, Exchange } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { PunkAvatar, getTraderAvatar } from '../common/PunkAvatar'
import { Badge } from '../ui/badge'
import {
  getModelDisplayName,
  getExchangeDisplayName,
  isPerpDexExchange,
  getWalletAddress,
  truncateAddress,
} from './model-constants'

interface TradersListProps {
  traders: TraderInfo[] | undefined
  isLoading: boolean
  loadError?: unknown
  allExchanges: Exchange[]
  configuredModelsCount: number
  configuredExchangesCount: number
  visibleTraderAddresses: Set<string>
  copiedId: string | null
  language: Language
  onTraderSelect?: (traderId: string) => void
  onNavigate: (path: string) => void
  onEditTrader: (traderId: string) => void
  onToggleTrader: (traderId: string, running: boolean) => void
  onToggleCompetition: (
    traderId: string,
    currentShowInCompetition: boolean
  ) => void
  onDeleteTrader: (traderId: string) => void
  onToggleTraderAddress: (traderId: string) => void
  onCopyAddress: (id: string, address: string) => void
}

export function TradersList({
  traders,
  isLoading,
  loadError,
  allExchanges,
  configuredModelsCount,
  configuredExchangesCount,
  visibleTraderAddresses,
  copiedId,
  language,
  onTraderSelect,
  onNavigate,
  onEditTrader,
  onToggleTrader,
  onToggleCompetition,
  onDeleteTrader,
  onToggleTraderAddress,
  onCopyAddress,
}: TradersListProps) {
  return (
    <div className="binance-card p-4 md:p-6">
      <div className="flex items-center justify-between mb-4 md:mb-5">
        <h2
          className="text-lg md:text-xl font-bold flex items-center gap-2"
          style={{ color: 'var(--fg)' }}
        >
          <Users
            className="w-5 h-5 md:w-6 md:h-6"
            style={{ color: 'var(--brand)' }}
          />
          {t('currentTraders', language)}
        </h2>
      </div>

      {isLoading && !traders ? (
        <TradersLoadingSkeleton />
      ) : loadError && (!traders || traders.length === 0) ? (
        <div
          className="py-8 text-center text-sm"
          style={{ color: 'var(--down)' }}
        >
          ⚠️ {t('traderDashboard.decisionsFetchFailed', language)}
          <div className="text-xs mt-2" style={{ color: 'var(--fg-3)' }}>
            {t('common.retryLater', language) !== 'common.retryLater'
              ? t('common.retryLater', language)
              : '加载失败,稍后自动重试 / Will retry automatically'}
          </div>
        </div>
      ) : traders && traders.length > 0 ? (
        <div className="space-y-3 md:space-y-4">
          {traders.map((trader) => (
            <TraderRow
              key={trader.trader_id}
              trader={trader}
              allExchanges={allExchanges}
              visibleTraderAddresses={visibleTraderAddresses}
              copiedId={copiedId}
              language={language}
              onTraderSelect={onTraderSelect}
              onNavigate={onNavigate}
              onEditTrader={onEditTrader}
              onToggleTrader={onToggleTrader}
              onToggleCompetition={onToggleCompetition}
              onDeleteTrader={onDeleteTrader}
              onToggleTraderAddress={onToggleTraderAddress}
              onCopyAddress={onCopyAddress}
            />
          ))}
        </div>
      ) : (
        <TradersEmptyState
          configuredModelsCount={configuredModelsCount}
          configuredExchangesCount={configuredExchangesCount}
          language={language}
        />
      )}
    </div>
  )
}

function TradersLoadingSkeleton() {
  return (
    <div className="space-y-3 md:space-y-4">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded gap-3 md:gap-4 animate-pulse"
          style={{
            background: 'var(--surface-2)',
            border: '1px solid var(--line)',
          }}
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
  )
}

function TradersEmptyState({
  configuredModelsCount,
  configuredExchangesCount,
  language,
}: {
  configuredModelsCount: number
  configuredExchangesCount: number
  language: Language
}) {
  return (
    <div
      className="text-center py-12 md:py-16"
      style={{ color: 'var(--fg-3)' }}
    >
      <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
      <div className="text-base md:text-lg font-semibold mb-2">
        {t('noTraders', language)}
      </div>
      <div className="text-xs md:text-sm mb-3 md:mb-4">
        {t('createFirstTrader', language)}
      </div>
      {(configuredModelsCount === 0 || configuredExchangesCount === 0) && (
        <div className="text-xs md:text-sm text-warn">
          {configuredModelsCount === 0 && configuredExchangesCount === 0
            ? t('configureModelsAndExchangesFirst', language)
            : configuredModelsCount === 0
              ? t('configureModelsFirst', language)
              : t('configureExchangesFirst', language)}
        </div>
      )}
    </div>
  )
}

function TraderRow({
  trader,
  allExchanges,
  visibleTraderAddresses,
  copiedId,
  language,
  onTraderSelect,
  onNavigate,
  onEditTrader,
  onToggleTrader,
  onToggleCompetition,
  onDeleteTrader,
  onToggleTraderAddress,
  onCopyAddress,
}: {
  trader: TraderInfo
  allExchanges: Exchange[]
  visibleTraderAddresses: Set<string>
  copiedId: string | null
  language: Language
  onTraderSelect?: (traderId: string) => void
  onNavigate: (path: string) => void
  onEditTrader: (traderId: string) => void
  onToggleTrader: (traderId: string, running: boolean) => void
  onToggleCompetition: (
    traderId: string,
    currentShowInCompetition: boolean
  ) => void
  onDeleteTrader: (traderId: string) => void
  onToggleTraderAddress: (traderId: string) => void
  onCopyAddress: (id: string, address: string) => void
}) {
  const exchange = allExchanges.find((e) => e.id === trader.exchange_id)
  const walletAddr = getWalletAddress(exchange)
  const isPerpDex = isPerpDexExchange(exchange?.exchange_type)
  const isVisible = visibleTraderAddresses.has(trader.trader_id)
  const isCopied = copiedId === trader.trader_id

  return (
    <div
      className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
      style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--line)',
      }}
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
            style={{ color: 'var(--fg)' }}
          >
            {trader.trader_name}
          </div>
          <div
            className="text-xs md:text-sm truncate"
            style={{
              color: trader.ai_model.includes('deepseek')
                ? 'var(--info)'
                : 'var(--ai)',
            }}
          >
            {getModelDisplayName(
              trader.ai_model.split('_').pop() || trader.ai_model
            )}{' '}
            Model • {getExchangeDisplayName(trader.exchange_id, allExchanges)}
            <PaperBadge
              exchangeType={exchange?.exchange_type}
              language={language}
            />
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
        {/* Wallet Address for Perp-DEX */}
        {isPerpDex && walletAddr && (
          <div
            className="flex items-center gap-1 px-2 py-1 rounded"
            style={{
              background: 'var(--brand-soft)',
              border:
                '1px solid color-mix(in srgb, var(--brand) 20%, transparent)',
            }}
          >
            <span
              className="text-xs font-mono"
              style={{ color: 'var(--brand)' }}
            >
              {isVisible ? walletAddr : truncateAddress(walletAddr)}
            </span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onToggleTraderAddress(trader.trader_id)
              }}
              className="p-0.5 rounded hover:bg-surface-hover transition-colors"
              title={
                isVisible
                  ? language === 'zh'
                    ? '隐藏'
                    : 'Hide'
                  : language === 'zh'
                    ? '显示'
                    : 'Show'
              }
            >
              {isVisible ? (
                <EyeOff className="w-3 h-3" style={{ color: 'var(--fg-3)' }} />
              ) : (
                <Eye className="w-3 h-3" style={{ color: 'var(--fg-3)' }} />
              )}
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onCopyAddress(trader.trader_id, walletAddr)
              }}
              className="p-0.5 rounded hover:bg-surface-hover transition-colors"
              title={language === 'zh' ? '复制' : 'Copy'}
            >
              {isCopied ? (
                <Check className="w-3 h-3" style={{ color: 'var(--up)' }} />
              ) : (
                <Copy className="w-3 h-3" style={{ color: 'var(--fg-3)' }} />
              )}
            </button>
          </div>
        )}
        {/* Status */}
        <div className="text-center">
          <Badge variant={trader.is_running ? 'up' : 'down'}>
            {trader.is_running
              ? t('running', language)
              : t('stopped', language)}
          </Badge>
        </div>

        {/* Actions */}
        <div className="flex gap-1.5 md:gap-2 flex-nowrap overflow-x-auto items-center">
          <button
            onClick={() => {
              if (onTraderSelect) {
                onTraderSelect(trader.trader_id)
              } else {
                const slug = `${trader.trader_name}-${trader.trader_id.slice(0, 4)}`
                onNavigate(`/dashboard?trader=${encodeURIComponent(slug)}`)
              }
            }}
            className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 whitespace-nowrap"
            style={{
              background: 'var(--ai-soft)',
              color: 'var(--ai)',
            }}
          >
            <BarChart3 className="w-3 h-3 md:w-4 md:h-4" />
            {t('view', language)}
          </button>

          <button
            onClick={() => onEditTrader(trader.trader_id)}
            disabled={trader.is_running}
            className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap flex items-center gap-1"
            style={{
              background: trader.is_running
                ? 'color-mix(in srgb, var(--fg-3) 10%, transparent)'
                : 'var(--warn-soft)',
              color: trader.is_running ? 'var(--fg-3)' : 'var(--warn)',
            }}
          >
            <Pencil className="w-3 h-3 md:w-4 md:h-4" />
            {t('edit', language)}
          </button>

          <button
            onClick={() =>
              onToggleTrader(trader.trader_id, trader.is_running || false)
            }
            className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap"
            style={
              trader.is_running
                ? {
                    background: 'var(--down-soft)',
                    color: 'var(--down)',
                  }
                : {
                    background: 'var(--up-soft)',
                    color: 'var(--up)',
                  }
            }
          >
            {trader.is_running ? t('stop', language) : t('start', language)}
          </button>

          <button
            onClick={() =>
              onToggleCompetition(
                trader.trader_id,
                trader.show_in_competition ?? true
              )
            }
            className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap flex items-center gap-1"
            style={
              trader.show_in_competition !== false
                ? {
                    background: 'var(--up-soft)',
                    color: 'var(--up)',
                  }
                : {
                    background:
                      'color-mix(in srgb, var(--fg-3) 10%, transparent)',
                    color: 'var(--fg-3)',
                  }
            }
            title={
              trader.show_in_competition !== false
                ? '在竞技场显示'
                : '在竞技场隐藏'
            }
          >
            {trader.show_in_competition !== false ? (
              <Eye className="w-3 h-3 md:w-4 md:h-4" />
            ) : (
              <EyeOff className="w-3 h-3 md:w-4 md:h-4" />
            )}
          </button>

          <button
            onClick={() => onDeleteTrader(trader.trader_id)}
            className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
            style={{
              background: 'var(--down-soft)',
              color: 'var(--down)',
            }}
          >
            <Trash2 className="w-3 h-3 md:w-4 md:h-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
