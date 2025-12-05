import { Bot, Plus } from 'lucide-react'
import { t, type Language } from '../../../i18n/translations'

interface PageHeaderProps {
  language: Language
  tradersCount: number
  configuredModelsCount: number
  configuredExchangesCount: number
  onAddModel: () => void
  onAddExchange: () => void
  onCreateTrader: () => void
}

export function PageHeader({
  language,
  tradersCount,
  configuredModelsCount,
  configuredExchangesCount,
  onAddModel,
  onAddExchange,
  onCreateTrader,
}: PageHeaderProps) {
  const canCreateTrader =
    configuredModelsCount > 0 && configuredExchangesCount > 0

  return (
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
              {tradersCount} {t('active', language)}
            </span>
          </h1>
          <p className="text-xs" style={{ color: '#848E9C' }}>
            {t('manageAITraders', language)}
          </p>
        </div>
      </div>

      <div className="flex gap-2 md:gap-3 w-full md:w-auto overflow-hidden flex-wrap md:flex-nowrap">
        <button
          onClick={onAddModel}
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
          onClick={onAddExchange}
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
          onClick={onCreateTrader}
          disabled={!canCreateTrader}
          className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1 md:gap-2 whitespace-nowrap"
          style={{
            background: canCreateTrader ? '#F0B90B' : '#2B3139',
            color: canCreateTrader ? '#000' : '#848E9C',
          }}
        >
          <Plus className="w-4 h-4" />
          {t('createTrader', language)}
        </button>
      </div>
    </div>
  )
}
