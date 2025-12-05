import { useState, useEffect } from 'react'
import { EquityChart } from './EquityChart'
import { TradingViewChart } from './TradingViewChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart } from 'lucide-react'

interface ChartTabsProps {
  traderId: string
  selectedSymbol?: string // 从外部选择的币种
  updateKey?: number // 强制更新的 key
}

type ChartTab = 'equity' | 'kline'

export function ChartTabs({ traderId, selectedSymbol, updateKey }: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')
  const [chartSymbol, setChartSymbol] = useState<string>('BTCUSDT')

  // 当从外部选择币种时，自动切换到K线图
  useEffect(() => {
    if (selectedSymbol) {
      console.log('[ChartTabs] 收到币种选择:', selectedSymbol, 'updateKey:', updateKey)
      setChartSymbol(selectedSymbol)
      setActiveTab('kline')
    }
  }, [selectedSymbol, updateKey])

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
          <TradingViewChart
            height={400}
            embedded
            defaultSymbol={chartSymbol}
            key={chartSymbol} // 强制重新渲染当币种变化时
          />
        )}
      </div>
    </div>
  )
}
