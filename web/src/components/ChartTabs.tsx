import { useState, useEffect, useRef } from 'react'
import { EquityChart } from './EquityChart'
import { AdvancedChart } from './AdvancedChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart, ChevronDown, Search } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

interface ChartTabsProps {
  traderId: string
  selectedSymbol?: string // 从外部选择的币种
  updateKey?: number // 强制更新的 key
  exchangeId?: string // 交易所ID
}

type ChartTab = 'equity' | 'kline'
type Interval = '1m' | '5m' | '15m' | '30m' | '1h' | '4h' | '1d'
type MarketType = 'hyperliquid' | 'crypto' | 'stocks' | 'forex' | 'metals'

interface SymbolInfo {
  symbol: string
  name: string
  category: string
}

// 市场类型配置
const MARKET_CONFIG = {
  hyperliquid: { exchange: 'hyperliquid', defaultSymbol: 'BTC', icon: '🔷', label: { zh: 'HL', en: 'HL' }, color: 'cyan', hasDropdown: true },
  crypto: { exchange: 'binance', defaultSymbol: 'BTCUSDT', icon: '₿', label: { zh: '加密', en: 'Crypto' }, color: 'yellow', hasDropdown: false },
  stocks: { exchange: 'alpaca', defaultSymbol: 'AAPL', icon: '📈', label: { zh: '美股', en: 'Stocks' }, color: 'green', hasDropdown: false },
  forex: { exchange: 'forex', defaultSymbol: 'EUR/USD', icon: '💱', label: { zh: '外汇', en: 'Forex' }, color: 'blue', hasDropdown: false },
  metals: { exchange: 'metals', defaultSymbol: 'XAU/USD', icon: '🥇', label: { zh: '金属', en: 'Metals' }, color: 'amber', hasDropdown: false },
}

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
  const [chartSymbol, setChartSymbol] = useState<string>('BTC')
  const [interval, setInterval] = useState<Interval>('5m')
  const [symbolInput, setSymbolInput] = useState('')
  const [marketType, setMarketType] = useState<MarketType>('hyperliquid')
  const [availableSymbols, setAvailableSymbols] = useState<SymbolInfo[]>([])
  const [showDropdown, setShowDropdown] = useState(false)
  const [searchFilter, setSearchFilter] = useState('')
  const dropdownRef = useRef<HTMLDivElement>(null)

  // 根据市场类型确定交易所
  const marketConfig = MARKET_CONFIG[marketType]
  const currentExchange = marketType === 'crypto' ? (exchangeId || marketConfig.exchange) : marketConfig.exchange

  // 获取可用币种列表
  useEffect(() => {
    if (marketConfig.hasDropdown) {
      fetch(`/api/symbols?exchange=${marketConfig.exchange}`)
        .then(res => res.json())
        .then(data => {
          if (data.symbols) {
            // 按类别排序: crypto > stock > forex > commodity > index
            const categoryOrder: Record<string, number> = { crypto: 0, stock: 1, forex: 2, commodity: 3, index: 4 }
            const sorted = [...data.symbols].sort((a: SymbolInfo, b: SymbolInfo) => {
              const orderA = categoryOrder[a.category] ?? 5
              const orderB = categoryOrder[b.category] ?? 5
              if (orderA !== orderB) return orderA - orderB
              return a.symbol.localeCompare(b.symbol)
            })
            setAvailableSymbols(sorted)
          }
        })
        .catch(err => console.error('Failed to fetch symbols:', err))
    }
  }, [marketType, marketConfig.exchange, marketConfig.hasDropdown])

  // 点击外部关闭下拉
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // 切换市场类型时更新默认符号
  const handleMarketTypeChange = (type: MarketType) => {
    setMarketType(type)
    setChartSymbol(MARKET_CONFIG[type].defaultSymbol)
    setShowDropdown(false)
  }

  // 过滤后的币种列表
  const filteredSymbols = availableSymbols.filter(s =>
    s.symbol.toLowerCase().includes(searchFilter.toLowerCase())
  )

  // 当从外部选择币种时，自动切换到K线图
  useEffect(() => {
    if (selectedSymbol) {
      console.log('[ChartTabs] 收到币种选择:', selectedSymbol, 'updateKey:', updateKey)
      setChartSymbol(selectedSymbol)
      setActiveTab('kline')
    }
  }, [selectedSymbol, updateKey])

  // 处理手动输入符号
  const handleSymbolSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (symbolInput.trim()) {
      let symbol = symbolInput.trim().toUpperCase()
      // 加密货币自动加 USDT 后缀
      if (marketType === 'crypto' && !symbol.endsWith('USDT')) {
        symbol = symbol + 'USDT'
      }
      setChartSymbol(symbol)
      setSymbolInput('')
    }
  }

  console.log('[ChartTabs] rendering, activeTab:', activeTab)

  return (
    <div className="binance-card" style={{ background: '#0D1117', borderRadius: '8px', overflow: 'hidden' }}>
      {/* Clean Professional Toolbar */}
      <div
        className="flex items-center justify-between px-3 py-1.5"
        style={{ borderBottom: '1px solid rgba(43, 49, 57, 0.6)', background: '#161B22' }}
      >
        {/* Left: Tab Switcher */}
        <div className="flex items-center gap-1">
          <button
            onClick={() => setActiveTab('equity')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium transition-all ${
              activeTab === 'equity'
                ? 'bg-blue-500/15 text-blue-400'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            <BarChart3 className="w-3 h-3" />
            <span>{t('accountEquityCurve', language)}</span>
          </button>

          <button
            onClick={() => setActiveTab('kline')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium transition-all ${
              activeTab === 'kline'
                ? 'bg-blue-500/15 text-blue-400'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            <CandlestickChart className="w-3 h-3" />
            <span>{t('marketChart', language)}</span>
          </button>

          {/* Market Type Pills - Only when kline active */}
          {activeTab === 'kline' && (
            <>
              <div className="w-px h-3 bg-[#30363D] mx-1" />
              <div className="flex items-center gap-0.5">
                {(Object.keys(MARKET_CONFIG) as MarketType[]).map((type) => {
                  const config = MARKET_CONFIG[type]
                  const isActive = marketType === type
                  return (
                    <button
                      key={type}
                      onClick={() => handleMarketTypeChange(type)}
                      className={`px-2 py-0.5 text-[10px] font-medium rounded transition-all ${
                        isActive
                          ? 'bg-[#21262D] text-white'
                          : 'text-gray-500 hover:text-gray-400'
                      }`}
                    >
                      {config.icon} {language === 'zh' ? config.label.zh : config.label.en}
                    </button>
                  )
                })}
              </div>
            </>
          )}
        </div>

        {/* Right: Symbol + Interval */}
        {activeTab === 'kline' && (
          <div className="flex items-center gap-2">
            {/* Symbol Dropdown */}
            {marketConfig.hasDropdown ? (
              <div className="relative" ref={dropdownRef}>
                <button
                  onClick={() => setShowDropdown(!showDropdown)}
                  className="flex items-center gap-1 px-2 py-1 bg-[#21262D] rounded text-[11px] font-bold text-white hover:bg-[#30363D] transition-all"
                >
                  <span>{chartSymbol}</span>
                  <ChevronDown className={`w-3 h-3 text-gray-400 transition-transform ${showDropdown ? 'rotate-180' : ''}`} />
                </button>
                {showDropdown && (
                  <div className="absolute top-full right-0 mt-1 w-56 bg-[#161B22] border border-[#30363D] rounded-lg shadow-2xl z-50 max-h-72 overflow-hidden">
                    <div className="p-2 border-b border-[#30363D]">
                      <div className="flex items-center gap-2 px-2 py-1 bg-[#0D1117] rounded border border-[#30363D]">
                        <Search className="w-3 h-3 text-gray-500" />
                        <input
                          type="text"
                          value={searchFilter}
                          onChange={(e) => setSearchFilter(e.target.value)}
                          placeholder="Search..."
                          className="flex-1 bg-transparent text-[11px] text-white placeholder-gray-600 focus:outline-none"
                          autoFocus
                        />
                      </div>
                    </div>
                    <div className="overflow-y-auto max-h-52">
                      {['crypto', 'stock', 'forex', 'commodity', 'index'].map(category => {
                        const categorySymbols = filteredSymbols.filter(s => s.category === category)
                        if (categorySymbols.length === 0) return null
                        const labels: Record<string, string> = { crypto: 'Crypto', stock: 'Stocks', forex: 'Forex', commodity: 'Commodities', index: 'Index' }
                        return (
                          <div key={category}>
                            <div className="px-3 py-1 text-[9px] font-medium text-gray-500 bg-[#0D1117] uppercase tracking-wider">{labels[category]}</div>
                            {categorySymbols.map(s => (
                              <button
                                key={s.symbol}
                                onClick={() => { setChartSymbol(s.symbol); setShowDropdown(false); setSearchFilter('') }}
                                className={`w-full px-3 py-1.5 text-left text-[11px] hover:bg-[#21262D] transition-all ${chartSymbol === s.symbol ? 'bg-blue-500/20 text-blue-400' : 'text-gray-300'}`}
                              >
                                {s.symbol}
                              </button>
                            ))}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <span className="px-2 py-1 bg-[#21262D] rounded text-[11px] font-bold text-white">{chartSymbol}</span>
            )}

            {/* Interval Selector */}
            <div className="flex items-center bg-[#21262D] rounded overflow-hidden">
              {INTERVALS.map((int) => (
                <button
                  key={int.value}
                  onClick={() => setInterval(int.value)}
                  className={`px-2 py-1 text-[10px] font-medium transition-all ${
                    interval === int.value
                      ? 'bg-blue-500/30 text-blue-400'
                      : 'text-gray-500 hover:text-gray-300 hover:bg-[#30363D]'
                  }`}
                >
                  {int.label}
                </button>
              ))}
            </div>

            {/* Quick Input */}
            <form onSubmit={handleSymbolSubmit} className="flex items-center">
              <input
                type="text"
                value={symbolInput}
                onChange={(e) => setSymbolInput(e.target.value)}
                placeholder="Symbol..."
                className="w-20 px-2 py-1 bg-[#0D1117] border border-[#30363D] rounded-l text-[10px] text-white placeholder-gray-600 focus:outline-none focus:border-blue-500/50"
              />
              <button type="submit" className="px-2 py-1 bg-[#21262D] border border-[#30363D] border-l-0 rounded-r text-[10px] text-gray-400 hover:text-white hover:bg-[#30363D] transition-all">
                Go
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
              key={`kline-${chartSymbol}-${interval}-${currentExchange}`}
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
                exchange={currentExchange}
                onSymbolChange={setChartSymbol}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}
