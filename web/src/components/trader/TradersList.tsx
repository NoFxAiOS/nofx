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
  onToggleCompetition: (traderId: string, currentShowInCompetition: boolean) => void
  onDeleteTrader: (traderId: string) => void
  onToggleTraderAddress: (traderId: string) => void
  onCopyAddress: (id: string, address: string) => void
}

export function TradersList({
  traders,
  isLoading,
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
          style={{ color: '#EAECEF' }}
        >
          <Users
            className="w-5 h-5 md:w-6 md:h-6"
            style={{ color: '#F0B90B' }}
          />
          {t('currentTraders', language)}
        </h2>
      </div>

      {isLoading ? (
        <TradersLoadingSkeleton />
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
      style={{ color: '#848E9C' }}
    >
      <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
      <div className="text-base md:text-lg font-semibold mb-2">
        {t('noTraders', language)}
      </div>
      <div className="text-xs md:text-sm mb-3 md:mb-4">
        {t('createFirstTrader', language)}
      </div>
      {(configuredModelsCount === 0 ||
        configuredExchangesCount === 0) && (
          <div className="text-xs md:text-sm text-yellow-500">
            {configuredModelsCount === 0 &&
              configuredExchangesCount === 0
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
  onToggleCompetition: (traderId: string, currentShowInCompetition: boolean) => void
  onDeleteTrader: (traderId: string) => void
  onToggleTraderAddress: (traderId: string) => void
  onCopyAddress: (id: string, address: string) => void
}) {
  const exchange = allExchanges.find(e => e.id === trader.exchange_id)
  const walletAddr = getWalletAddress(exchange)
  const isPerpDex = isPerpDexExchange(exchange?.exchange_type)
  const isVisible = visibleTraderAddresses.has(trader.trader_id)
  const isCopied = copiedId === trader.trader_id

  return (
    <div
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
        {/* Wallet Address for Perp-DEX */}
        {isPerpDex && walletAddr && (
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
                onToggleTraderAddress(trader.trader_id)
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
                onCopyAddress(trader.trader_id, walletAddr)
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
        )}
        {/* Status */}
        <div className="text-center">
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
              background: 'rgba(99, 102, 241, 0.1)',
              color: '#6366F1',
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
              onToggleTrader(
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
            onClick={() => onToggleCompetition(trader.trader_id, trader.show_in_competition ?? true)}
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
            onClick={() => onDeleteTrader(trader.trader_id)}
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
  )
}
