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

// 根据交易所ID推断市场类型
function getMarketTypeFromExchange(exchangeId: string | undefined): MarketType {
  if (!exchangeId) return 'hyperliquid'
  const lower = exchangeId.toLowerCase()
  if (lower.includes('hyperliquid')) return 'hyperliquid'
  // 其他交易所默认使用 crypto 类型
  return 'crypto'
}

export function ChartTabs({ traderId, selectedSymbol, updateKey, exchangeId }: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')
  const [chartSymbol, setChartSymbol] = useState<string>('BTC')
  const [interval, setInterval] = useState<Interval>('5m')
  const [symbolInput, setSymbolInput] = useState('')
  const [marketType, setMarketType] = useState<MarketType>(() => getMarketTypeFromExchange(exchangeId))
  const [availableSymbols, setAvailableSymbols] = useState<SymbolInfo[]>([])
  const [showDropdown, setShowDropdown] = useState(false)
  const [searchFilter, setSearchFilter] = useState('')
  const dropdownRef = useRef<HTMLDivElement>(null)

  // 当交易所ID变化时，自动切换市场类型
  useEffect(() => {
    const newMarketType = getMarketTypeFromExchange(exchangeId)
    setMarketType(newMarketType)
  }, [exchangeId])

  // 根据市场类型确定交易所
  const marketConfig = MARKET_CONFIG[marketType]
  // 优先使用传入的 exchangeId（非 hyperliquid 时）
  const currentExchange = marketType === 'hyperliquid' ? 'hyperliquid' : (exchangeId || marketConfig.exchange)

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
    <div className={`nofx-glass rounded-lg border border-white/5 relative z-10 w-full flex flex-col transition-all duration-300 ${typeof window !== 'undefined' && window.innerWidth < 768 ? 'h-[500px]' : 'h-[600px]'
      }`}>
      {/* 
        Premium Professional Toolbar 
        Mobile: Single row, horizontal scroll with gradient mask
        Desktop: Standard flex-wrap/nowrap
      */}
      <div
        className="relative z-20 flex flex-wrap md:flex-nowrap items-center justify-between gap-y-2 px-3 py-2 shrink-0 backdrop-blur-md bg-[#0B0E11]/80 rounded-t-lg"
        style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.05)' }}
      >
        {/* Left: Tab Switcher */}
        <div className="flex flex-wrap items-center gap-1">
          <button
            onClick={() => setActiveTab('equity')}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[11px] font-medium transition-all ${activeTab === 'equity'
              ? 'bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 shadow-[0_0_10px_rgba(240,185,11,0.1)]'
              : 'text-nofx-text-muted hover:text-nofx-text-main hover:bg-white/5'
              }`}
          >
            <BarChart3 className="w-3.5 h-3.5" />
            <span className="hidden md:inline">{t('accountEquityCurve', language)}</span>
            <span className="md:hidden">Eq</span>
          </button>

          <button
            onClick={() => setActiveTab('kline')}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[11px] font-medium transition-all ${activeTab === 'kline'
              ? 'bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 shadow-[0_0_10px_rgba(240,185,11,0.1)]'
              : 'text-nofx-text-muted hover:text-nofx-text-main hover:bg-white/5'
              }`}
          >
            <CandlestickChart className="w-3.5 h-3.5" />
            <span className="hidden md:inline">{t('marketChart', language)}</span>
            <span className="md:hidden">Kline</span>
          </button>

          {/* Market Type Pills - Only when kline active, HIDDEN on mobile to save space */}
          {activeTab === 'kline' && (
            <div className="hidden md:flex items-center gap-1 ml-2 border-l border-white/10 pl-2">
              {(Object.keys(MARKET_CONFIG) as MarketType[]).map((type) => {
                const config = MARKET_CONFIG[type]
                const isActive = marketType === type
                return (
                  <button
                    key={type}
                    onClick={() => handleMarketTypeChange(type)}
                    className={`px-2.5 py-1 text-[10px] font-medium rounded transition-all border ${isActive
                      ? 'bg-white/10 text-white border-white/20'
                      : 'text-nofx-text-muted border-transparent hover:text-nofx-text-main hover:bg-white/5'
                      }`}
                  >
                    <span className="mr-1 opacity-70">{config.icon}</span>
                    {language === 'zh' ? config.label.zh : config.label.en}
                  </button>
                )
              })}
            </div>
          )}
        </div>

        {/* Right: Symbol + Interval */}
        {activeTab === 'kline' && (
          <div className="flex items-center gap-2 md:gap-3 w-full md:w-auto min-w-0">
            {/* Symbol Dropdown */}
            <div className="shrink-0 relative" ref={dropdownRef}>
              {marketConfig.hasDropdown ? (
                <>
                  <button
                    onClick={() => setShowDropdown(!showDropdown)}
                    className="flex items-center gap-1.5 px-2.5 py-1 bg-black/40 border border-white/10 rounded text-[11px] font-bold text-nofx-text-main hover:border-nofx-gold/30 hover:text-nofx-gold transition-all"
                  >
                    <span>{chartSymbol}</span>
                    <ChevronDown className={`w-3 h-3 text-nofx-text-muted transition-transform ${showDropdown ? 'rotate-180' : ''}`} />
                  </button>
                  {showDropdown && (
                    <div className="absolute top-full right-0 mt-2 w-64 bg-[#0B0E11] border border-white/10 rounded-lg shadow-[0_10px_40px_-10px_rgba(0,0,0,0.5)] z-50 overflow-hidden nofx-glass ring-1 ring-white/5">
                      <div className="p-2 border-b border-white/5">
                        <div className="flex items-center gap-2 px-2 py-1.5 bg-black/40 rounded border border-white/10 focus-within:border-nofx-gold/50 transition-colors">
                          <Search className="w-3.5 h-3.5 text-nofx-text-muted" />
                          <input
                            type="text"
                            value={searchFilter}
                            onChange={(e) => setSearchFilter(e.target.value)}
                            placeholder="Search symbol..."
                            className="flex-1 bg-transparent text-[11px] text-white placeholder-gray-600 focus:outline-none font-mono"
                            autoFocus
                          />
                        </div>
                      </div>
                      <div className="overflow-y-auto max-h-60 custom-scrollbar">
                        {['crypto', 'stock', 'forex', 'commodity', 'index'].map(category => {
                          const categorySymbols = filteredSymbols.filter(s => s.category === category)
                          if (categorySymbols.length === 0) return null
                          const labels: Record<string, string> = { crypto: 'Crypto', stock: 'Stocks', forex: 'Forex', commodity: 'Commodities', index: 'Index' }
                          return (
                            <div key={category}>
                              <div className="px-3 py-1.5 text-[9px] font-bold text-nofx-text-muted/60 bg-white/5 uppercase tracking-wider">{labels[category]}</div>
                              {categorySymbols.map(s => (
                                <button
                                  key={s.symbol}
                                  onClick={() => { setChartSymbol(s.symbol); setShowDropdown(false); setSearchFilter('') }}
                                  className={`w-full px-3 py-2 text-left text-[11px] font-mono hover:bg-white/5 transition-all flex items-center justify-between ${chartSymbol === s.symbol ? 'bg-nofx-gold/10 text-nofx-gold' : 'text-nofx-text-muted'}`}
                                >
                                  <span>{s.symbol}</span>
                                  <span className="text-[9px] opacity-40">{s.name}</span>
                                </button>
                              ))}
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <span className="px-2.5 py-1 bg-black/40 border border-white/10 rounded text-[11px] font-bold text-nofx-text-main font-mono">{chartSymbol}</span>
              )}
            </div>

            {/* Interval Selector - Allow scrolling if needed */}
            <div className="flex items-center bg-black/40 rounded border border-white/10 overflow-x-auto no-scrollbar max-w-[200px] md:max-w-none">
              {INTERVALS.map((int) => (
                <button
                  key={int.value}
                  onClick={() => setInterval(int.value)}
                  className={`px-2 py-1 text-[10px] font-medium transition-all ${interval === int.value
                    ? 'bg-nofx-gold/20 text-nofx-gold'
                    : 'text-nofx-text-muted hover:text-white hover:bg-white/5'
                    }`}
                >
                  {int.label}
                </button>
              ))}
            </div>

            {/* Quick Input - Hidden on mobile, dropdown search is enough */}
            <form onSubmit={handleSymbolSubmit} className="hidden md:flex items-center shrink-0">
              <input
                type="text"
                value={symbolInput}
                onChange={(e) => setSymbolInput(e.target.value)}
                placeholder="Sym"
                className="w-16 px-2 py-1 bg-black/40 border border-white/10 rounded-l text-[10px] text-white placeholder-gray-600 focus:outline-none focus:border-nofx-gold/50 font-mono transition-colors"
              />
              <button type="submit" className="px-2 py-1 bg-white/5 border border-white/10 border-l-0 rounded-r text-[10px] text-nofx-text-muted hover:text-white hover:bg-white/10 transition-all">
                Go
              </button>
            </form>
          </div>
        )}
      </div>

      {/* Tab Content - Chart autosizes to this container */}
      <div className="relative flex-1 bg-[#0B0E11]/50 rounded-b-lg overflow-hidden h-full min-h-0">
        <AnimatePresence mode="wait">
          {activeTab === 'equity' ? (
            <motion.div
              key="equity"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="h-full w-full absolute inset-0"
            >
              <EquityChart traderId={traderId} embedded />
            </motion.div>
          ) : (
            <motion.div
              key={`kline-${chartSymbol}-${interval}-${currentExchange}`}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="h-full w-full absolute inset-0"
            >
              <AdvancedChart
                symbol={chartSymbol}
                interval={interval}
                traderID={traderId}
                // Dynamic auto-sizing via ResizeObserver
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
