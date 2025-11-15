import { Landmark } from 'lucide-react'
import { t, type Language } from '../../../i18n/translations'
import { getExchangeIcon } from '../../ExchangeIcons'
import { getShortName } from '../index'
import type { Exchange } from '../../../types'

interface ExchangesSectionProps {
  language: Language
  configuredExchanges: Exchange[]
  isExchangeInUse: (exchangeId: string) => boolean
  onExchangeClick: (exchangeId: string) => void
}

export function ExchangesSection({
  language,
  configuredExchanges,
  isExchangeInUse,
  onExchangeClick,
}: ExchangesSectionProps) {
  return (
    <div className="binance-card p-3 md:p-4">
      <h3
        className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
        style={{ color: '#EAECEF' }}
      >
        <Landmark
          className="w-4 h-4 md:w-5 md:h-5"
          style={{ color: '#F0B90B' }}
        />
        {t('exchanges', language)}
      </h3>
      <div className="space-y-2 md:space-y-3">
        {configuredExchanges.map((exchange) => {
          const inUse = isExchangeInUse(exchange.id)
          return (
            <div
              key={exchange.id}
              className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                inUse
                  ? 'cursor-not-allowed'
                  : 'cursor-pointer hover:bg-gray-700'
              }`}
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              onClick={() => onExchangeClick(exchange.id)}
            >
              <div className="flex items-center gap-2 md:gap-3">
                <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                  {getExchangeIcon(exchange.id, { width: 28, height: 28 })}
                </div>
                <div className="min-w-0">
                  <div
                    className="font-semibold text-sm md:text-base truncate"
                    style={{ color: '#EAECEF' }}
                  >
                    {getShortName(exchange.name)}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {exchange.type.toUpperCase()} •{' '}
                    {inUse
                      ? t('inUse', language)
                      : exchange.enabled
                        ? t('enabled', language)
                        : t('configured', language)}
                  </div>
                </div>
              </div>
              <div
                className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${exchange.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
              />
            </div>
          )
        })}
        {configuredExchanges.length === 0 && (
          <div
            className="text-center py-6 md:py-8"
            style={{ color: '#848E9C' }}
          >
            <Landmark className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
            <div className="text-xs md:text-sm">
              {t('noExchangesConfigured', language)}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
