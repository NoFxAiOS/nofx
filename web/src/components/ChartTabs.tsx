import { useState } from 'react'
import { EquityChart } from './EquityChart'
import { TradingViewChart } from './TradingViewChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart } from 'lucide-react'

interface ChartTabsProps {
  traderId: string
}

type ChartTab = 'equity' | 'kline'

export function ChartTabs({ traderId }: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')

  console.log('[ChartTabs] rendering, activeTab:', activeTab)

  return (
    <div className="binance-card">
      {/* Tab Headers - 这是Tab切换按钮区域 */}
      <div
        className="flex items-center gap-2 p-3"
        style={{
          borderBottom: '1px solid #2B3139',
          background: '#0B0E11',
        }}
      >
        <button
          onClick={() => {
            console.log('[ChartTabs] switching to equity')
            setActiveTab('equity')
          }}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold"
          style={
            activeTab === 'equity'
              ? {
                  background: 'rgba(240, 185, 11, 0.15)',
                  color: '#F0B90B',
                  border: '1px solid rgba(240, 185, 11, 0.3)',
                }
              : {
                  background: 'transparent',
                  color: '#848E9C',
                  border: '1px solid transparent',
                }
          }
        >
          <BarChart3 className="w-4 h-4" />
          {t('accountEquityCurve', language)}
        </button>

        <button
          onClick={() => {
            console.log('[ChartTabs] switching to kline')
            setActiveTab('kline')
          }}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold"
          style={
            activeTab === 'kline'
              ? {
                  background: 'rgba(240, 185, 11, 0.15)',
                  color: '#F0B90B',
                  border: '1px solid rgba(240, 185, 11, 0.3)',
                }
              : {
                  background: 'transparent',
                  color: '#848E9C',
                  border: '1px solid transparent',
                }
          }
        >
          <CandlestickChart className="w-4 h-4" />
          {t('marketChart', language)}
        </button>
      </div>

      {/* Tab Content */}
      <div>
        {activeTab === 'equity' ? (
          <EquityChart traderId={traderId} embedded />
        ) : (
          <TradingViewChart height={400} embedded />
        )}
      </div>
    </div>
  )
}
