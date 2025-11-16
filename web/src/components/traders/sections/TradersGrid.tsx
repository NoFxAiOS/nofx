import { Bot, BarChart3, Trash2, Pencil } from 'lucide-react'
import { t, type Language } from '../../../i18n/translations'
import { getModelDisplayName } from '../index'
import type { TraderInfo } from '../../../types'

interface TradersGridProps {
  language: Language
  traders: TraderInfo[] | undefined
  onTraderSelect: (traderId: string) => void
  onEditTrader: (traderId: string) => void
  onDeleteTrader: (traderId: string) => void
  onToggleTrader: (traderId: string, running: boolean) => void
}

export function TradersGrid({
  language,
  traders,
  onTraderSelect,
  onEditTrader,
  onDeleteTrader,
  onToggleTrader,
}: TradersGridProps) {
  if (!traders || traders.length === 0) {
    return (
      <div className="text-center py-12 md:py-16" style={{ color: '#848E9C' }}>
        <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
        <div className="text-base md:text-lg font-semibold mb-2">
          {t('noTraders', language)}
        </div>
        <div className="text-xs md:text-sm mb-3 md:mb-4">
          {t('createFirstTrader', language)}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-3 md:space-y-4">
      {traders.map((trader) => (
        <div
          key={trader.trader_id}
          className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <div className="flex items-center gap-3 md:gap-4">
            <div
              className="w-10 h-10 md:w-12 md:h-12 rounded-full flex items-center justify-center flex-shrink-0"
              style={{
                background: trader.ai_model.includes('deepseek')
                  ? '#60a5fa'
                  : '#c084fc',
                color: '#fff',
              }}
            >
              <Bot className="w-5 h-5 md:w-6 md:h-6" />
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
                Model • {trader.exchange_id?.toUpperCase()}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
            {/* Status */}
            <div className="text-center">
              <div
                className={`px-2 md:px-3 py-1 rounded text-xs font-bold ${
                  trader.is_running
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

            {/* Actions: 禁止换行,超出横向滚动 */}
            <div className="flex gap-1.5 md:gap-2 flex-nowrap overflow-x-auto items-center">
              <button
                onClick={() => onTraderSelect(trader.trader_id)}
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
                  onToggleTrader(trader.trader_id, trader.is_running || false)
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
                {trader.is_running ? t('stop', language) : t('start', language)}
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
      ))}
    </div>
  )
}
