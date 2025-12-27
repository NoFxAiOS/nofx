import { useState, useEffect } from 'react'
import { EquityChart } from './EquityChart'
import { AdvancedChart } from './AdvancedChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

interface ChartTabsProps {
  traderId: string
  selectedSymbol?: string // 从外部选择的币种
  updateKey?: number // 强制更新的 key
  exchangeId?: string // 交易所ID
}

type ChartTab = 'equity' | 'kline'
type Interval = '1m' | '5m' | '15m' | '30m' | '1h' | '4h' | '1d'

const INTERVALS: { value: Interval; label: string }[] = [
  { value: '1m', label: '1m' },
  { value: '5m', label: '5m' },
  { value: '15m', label: '15m' },
  { value: '30m', label: '30m' },
  { value: '1h', label: '1h' },
  { value: '4h', label: '4h' },
  { value: '1d', label: '1d' },
]

export function ChartTabs({ traderId, selectedSymbol, updateKey, exchangeId }: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')
  const [chartSymbol, setChartSymbol] = useState<string>('BTCUSDT')
  const [interval, setInterval] = useState<Interval>('5m')
  const [symbolInput, setSymbolInput] = useState('')

  // 当从外部选择币种时，自动切换到K线图
  useEffect(() => {
    if (selectedSymbol) {
      console.log('[ChartTabs] 收到币种选择:', selectedSymbol, 'updateKey:', updateKey)
      setChartSymbol(selectedSymbol)
      setActiveTab('kline')
    }
  }, [selectedSymbol, updateKey])

  // 处理手动输入币种
  const handleSymbolSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (symbolInput.trim()) {
      const symbol = symbolInput.trim().toUpperCase()
      setChartSymbol(symbol)
      setSymbolInput('')
    }
  }

  console.log('[ChartTabs] rendering, activeTab:', activeTab)

  return (
    <div className="binance-card">
      {/* Tab Headers - 专业化工具栏 */}
      <div
        className="flex items-center justify-between px-4 py-2"
        style={{
          borderBottom: '1px solid #2B3139',
          background: 'linear-gradient(180deg, #1A1E23 0%, #0B0E11 100%)',
        }}
      >
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => {
              console.log('[ChartTabs] switching to equity')
              setActiveTab('equity')
            }}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${activeTab === 'equity'
              ? 'bg-yellow-500/15 text-yellow-400 border border-yellow-500/40'
              : 'text-gray-400 hover:text-gray-200 hover:bg-white/5'
              }`}
          >
            <BarChart3 className="w-3.5 h-3.5" />
            <span>{t('accountEquityCurve', language)}</span>
          </button>

          <button
            onClick={() => {
              console.log('[ChartTabs] switching to kline')
              setActiveTab('kline')
            }}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${activeTab === 'kline'
              ? 'bg-yellow-500/15 text-yellow-400 border border-yellow-500/40'
              : 'text-gray-400 hover:text-gray-200 hover:bg-white/5'
              }`}
          >
            <CandlestickChart className="w-3.5 h-3.5" />
            <span>{t('marketChart', language)}</span>
          </button>
        </div>

        {/* 币种选择器和时间周期选择器 - 仅在K线图模式下显示 */}
        {activeTab === 'kline' && (
          <div className="flex items-center gap-2">
            {/* 当前币种显示 */}
            <div className="px-2.5 py-1 bg-[#1A1E23] border border-[#2B3139] rounded text-xs font-bold text-yellow-400">
              {chartSymbol}
            </div>

            <div className="w-px h-4 bg-[#2B3139]"></div>

            {/* 时间周期选择器 - 更紧凑专业 */}
            <div className="flex items-center gap-0.5">
              {INTERVALS.map((int) => (
                <button
                  key={int.value}
                  onClick={() => setInterval(int.value)}
                  className={`px-2 py-1 text-[10px] font-medium transition-all ${
                    interval === int.value
                      ? 'bg-yellow-500/20 text-yellow-400 rounded'
                      : 'text-gray-500 hover:text-gray-300'
                  }`}
                >
                  {int.label}
                </button>
              ))}
            </div>

            <div className="w-px h-4 bg-[#2B3139]"></div>

            {/* 币种输入框 - 更紧凑 */}
            <form onSubmit={handleSymbolSubmit} className="flex items-center gap-1.5">
              <input
                type="text"
                value={symbolInput}
                onChange={(e) => setSymbolInput(e.target.value)}
                placeholder="输入币种..."
                className="px-2 py-1 bg-[#1A1E23] border border-[#2B3139] rounded text-[11px] text-white placeholder-gray-600 focus:outline-none focus:border-yellow-500/50 w-24"
              />
              <button
                type="submit"
                className="px-2 py-1 bg-yellow-500/15 text-yellow-400 border border-yellow-500/30 rounded text-[10px] font-medium hover:bg-yellow-500/25 transition-all"
              >
                GO
              </button>
            </form>
          </div>
        )}
      </div>

      {/* Tab Content */}
      <div className="relative overflow-hidden min-h-[400px]">
        <AnimatePresence mode="wait">
          {activeTab === 'equity' ? (
            <motion.div
              key="equity"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <EquityChart traderId={traderId} embedded />
            </motion.div>
          ) : (
            <motion.div
              key={`kline-${chartSymbol}-${interval}-${exchangeId}`}
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <AdvancedChart
                symbol={chartSymbol}
                interval={interval}
                traderID={traderId}
                height={550}
                exchange={exchangeId || 'binance'}
                onSymbolChange={setChartSymbol}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}
