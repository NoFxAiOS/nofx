import { useEffect, useRef, useState, memo } from 'react'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { ChevronDown, TrendingUp, X } from 'lucide-react'

// 支持的交易所列表 (合约格式)
const EXCHANGES = [
  { id: 'BINANCE', name: 'Binance', prefix: 'BINANCE:', suffix: '.P' },
  { id: 'BYBIT', name: 'Bybit', prefix: 'BYBIT:', suffix: '.P' },
  { id: 'OKX', name: 'OKX', prefix: 'OKX:', suffix: '.P' },
  { id: 'BITGET', name: 'Bitget', prefix: 'BITGET:', suffix: '.P' },
  { id: 'MEXC', name: 'MEXC', prefix: 'MEXC:', suffix: '.P' },
  { id: 'GATEIO', name: 'Gate.io', prefix: 'GATEIO:', suffix: '.P' },
] as const

// 热门交易对
const POPULAR_SYMBOLS = [
  'BTCUSDT',
  'ETHUSDT',
  'SOLUSDT',
  'BNBUSDT',
  'XRPUSDT',
  'DOGEUSDT',
  'ADAUSDT',
  'AVAXUSDT',
  'DOTUSDT',
  'LINKUSDT',
  'MATICUSDT',
  'LTCUSDT',
]

// 时间周期选项
const INTERVALS = [
  { id: '1', label: '1m' },
  { id: '5', label: '5m' },
  { id: '15', label: '15m' },
  { id: '30', label: '30m' },
  { id: '60', label: '1H' },
  { id: '240', label: '4H' },
  { id: 'D', label: '1D' },
  { id: 'W', label: '1W' },
]

interface TradingViewChartProps {
  defaultSymbol?: string
  defaultExchange?: string
  height?: number
  showToolbar?: boolean
  embedded?: boolean // 嵌入模式（不显示外层卡片）
}

function TradingViewChartComponent({
  defaultSymbol = 'BTCUSDT',
  defaultExchange = 'BINANCE',
  height = 400,
  showToolbar = true,
  embedded = false,
}: TradingViewChartProps) {
  const { language } = useLanguage()
  const containerRef = useRef<HTMLDivElement>(null)
  const [exchange, setExchange] = useState(defaultExchange)
  const [symbol, setSymbol] = useState(defaultSymbol)
  const [timeInterval, setTimeInterval] = useState('60')
  const [customSymbol, setCustomSymbol] = useState('')
  const [showExchangeDropdown, setShowExchangeDropdown] = useState(false)
  const [showSymbolDropdown, setShowSymbolDropdown] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)

  // 当外部传入的 defaultSymbol 变化时，更新内部 symbol
  useEffect(() => {
    if (defaultSymbol && defaultSymbol !== symbol) {
      // console.log('[TradingViewChart] 更新币种:', defaultSymbol)
      setSymbol(defaultSymbol)
    }
  }, [defaultSymbol])

  // 当外部传入的 defaultExchange 变化时，更新内部 exchange
  useEffect(() => {
    if (defaultExchange && defaultExchange !== exchange) {
      const normalizedExchange = defaultExchange.toUpperCase()
      // console.log('[TradingViewChart] 更新交易所:', normalizedExchange)
      if (EXCHANGES.some(e => e.id === normalizedExchange)) {
        setExchange(normalizedExchange)
      }
    }
  }, [defaultExchange])

  // 获取完整的交易对符号 (合约格式: BINANCE:BTCUSDT.P)
  const getFullSymbol = () => {
    const exchangeInfo = EXCHANGES.find((e) => e.id === exchange)
    const prefix = exchangeInfo?.prefix || 'BINANCE:'
    const suffix = exchangeInfo?.suffix || '.P'
    return `${prefix}${symbol}${suffix}`
  }

  // 加载 TradingView Widget
  useEffect(() => {
    if (!containerRef.current) return

    // 清空容器
    containerRef.current.innerHTML = ''

    // 创建 widget 容器
    const widgetContainer = document.createElement('div')
    widgetContainer.className = 'tradingview-widget-container'
    widgetContainer.style.height = '100%'
    widgetContainer.style.width = '100%'

    const widgetDiv = document.createElement('div')
    widgetDiv.className = 'tradingview-widget-container__widget'
    widgetDiv.style.height = '100%'
    widgetDiv.style.width = '100%'

    widgetContainer.appendChild(widgetDiv)
    containerRef.current.appendChild(widgetContainer)

    // 加载 TradingView 脚本
    const script = document.createElement('script')
    script.src =
      'https://s3.tradingview.com/external-embedding/embed-widget-advanced-chart.js'
    script.type = 'text/javascript'
    script.async = true
    script.innerHTML = JSON.stringify({
      width: '100%',
      height: '100%',
      symbol: getFullSymbol(),
      interval: timeInterval,
      timezone: 'Etc/UTC',
      theme: 'dark',
      style: '1',
      locale: language === 'zh' ? 'zh_CN' : 'en',
      enable_publishing: false,
      backgroundColor: 'rgba(11, 14, 17, 1)',
      gridColor: 'rgba(43, 49, 57, 0.5)',
      hide_top_toolbar: !showToolbar,
      hide_legend: false,
      save_image: false,
      calendar: false,
      hide_volume: false,
      support_host: 'https://www.tradingview.com',
    })

    widgetContainer.appendChild(script)

    return () => {
      if (containerRef.current) {
        containerRef.current.innerHTML = ''
      }
    }
  }, [exchange, symbol, timeInterval, language, showToolbar])

  // 处理自定义交易对输入
  const handleCustomSymbolSubmit = () => {
    if (customSymbol.trim()) {
      let sym = customSymbol.trim().toUpperCase()
      // 如果没有 USDT 后缀，自动加上
      if (!sym.endsWith('USDT')) {
        sym = sym + 'USDT'
      }
      setSymbol(sym)
      setCustomSymbol('')
      setShowSymbolDropdown(false)
    }
  }

  return (
    <div
      className={`${embedded ? '' : 'binance-card'} overflow-hidden ${embedded ? '' : 'animate-fade-in'} ${isFullscreen
          ? 'fixed inset-0 z-50 rounded-none flex flex-col'
          : ''
        }`}
      style={isFullscreen ? { background: '#0B0E11' } : undefined}
    >
      {/* Header */}
      <div
        className="flex flex-wrap items-center gap-2 p-3 sm:p-4"
        style={{ borderBottom: embedded ? 'none' : '1px solid #2B3139' }}
      >
        {!embedded && (
          <div className="flex items-center gap-2">
            <TrendingUp className="w-5 h-5" style={{ color: '#F0B90B' }} />
            <h3
              className="text-base sm:text-lg font-bold"
              style={{ color: '#EAECEF' }}
            >
              {t('marketChart', language)}
            </h3>
          </div>
        )}

        {/* Controls */}
        <div className={`flex flex-wrap items-center gap-2 ${embedded ? '' : 'ml-auto'}`}>
          {/* Exchange Selector */}
          <div className="relative">
            <button
              onClick={() => {
                setShowExchangeDropdown(!showExchangeDropdown)
                setShowSymbolDropdown(false)
              }}
              className="flex items-center gap-1 px-3 py-1.5 rounded text-sm font-medium transition-all"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              {EXCHANGES.find((e) => e.id === exchange)?.name || exchange}
              <ChevronDown className="w-4 h-4" style={{ color: '#848E9C' }} />
            </button>

            {showExchangeDropdown && (
              <div
                className="absolute top-full left-0 mt-1 py-1 rounded-lg shadow-xl z-20 min-w-[120px]"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                }}
              >
                {EXCHANGES.map((ex) => (
                  <button
                    key={ex.id}
                    onClick={() => {
                      setExchange(ex.id)
                      setShowExchangeDropdown(false)
                    }}
                    className="w-full px-4 py-2 text-left text-sm transition-all hover:bg-opacity-50"
                    style={{
                      color: exchange === ex.id ? '#F0B90B' : '#EAECEF',
                      background:
                        exchange === ex.id
                          ? 'rgba(240, 185, 11, 0.1)'
                          : 'transparent',
                    }}
                  >
                    {ex.name}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Symbol Selector */}
          <div className="relative">
            <button
              onClick={() => {
                setShowSymbolDropdown(!showSymbolDropdown)
                setShowExchangeDropdown(false)
              }}
              className="flex items-center gap-1 px-3 py-1.5 rounded text-sm font-bold transition-all"
              style={{
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.3)',
                color: '#F0B90B',
              }}
            >
              {symbol}
              <ChevronDown className="w-4 h-4" />
            </button>

            {showSymbolDropdown && (
              <div
                className="absolute top-full left-0 mt-1 py-2 rounded-lg shadow-xl z-20 w-[280px]"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                }}
              >
                {/* Custom Input */}
                <div className="px-3 pb-2" style={{ borderBottom: '1px solid #2B3139' }}>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={customSymbol}
                      onChange={(e) => setCustomSymbol(e.target.value.toUpperCase())}
                      onKeyDown={(e) => e.key === 'Enter' && handleCustomSymbolSubmit()}
                      placeholder={t('enterSymbol', language)}
                      className="flex-1 px-3 py-1.5 rounded text-sm"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                    />
                    <button
                      onClick={handleCustomSymbolSubmit}
                      className="px-3 py-1.5 rounded text-sm font-medium"
                      style={{
                        background: '#F0B90B',
                        color: '#0B0E11',
                      }}
                    >
                      OK
                    </button>
                  </div>
                </div>

                {/* Popular Symbols */}
                <div className="px-2 pt-2">
                  <div
                    className="text-xs px-2 py-1 mb-1"
                    style={{ color: '#848E9C' }}
                  >
                    {t('popularSymbols', language)}
                  </div>
                  <div className="grid grid-cols-3 gap-1">
                    {POPULAR_SYMBOLS.map((sym) => (
                      <button
                        key={sym}
                        onClick={() => {
                          setSymbol(sym)
                          setShowSymbolDropdown(false)
                        }}
                        className="px-2 py-1.5 rounded text-xs font-medium transition-all"
                        style={{
                          color: symbol === sym ? '#F0B90B' : '#EAECEF',
                          background:
                            symbol === sym
                              ? 'rgba(240, 185, 11, 0.1)'
                              : 'rgba(43, 49, 57, 0.3)',
                        }}
                      >
                        {sym.replace('USDT', '')}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Interval Selector */}
          <div
            className="flex gap-0.5 p-0.5 rounded"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            {INTERVALS.map((int) => (
              <button
                key={int.id}
                onClick={() => setTimeInterval(int.id)}
                className="px-2 py-1 rounded text-xs font-medium transition-all"
                style={{
                  background: timeInterval === int.id ? '#F0B90B' : 'transparent',
                  color: timeInterval === int.id ? '#0B0E11' : '#848E9C',
                }}
              >
                {int.label}
              </button>
            ))}
          </div>

          {/* Fullscreen Toggle */}
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="p-1.5 rounded transition-all"
            style={{
              background: isFullscreen ? '#F0B90B' : 'transparent',
              color: isFullscreen ? '#0B0E11' : '#848E9C',
              border: '1px solid #2B3139',
            }}
            title={isFullscreen ? t('exitFullscreen', language) : t('fullscreen', language)}
          >
            {isFullscreen ? (
              <X className="w-4 h-4" />
            ) : (
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M8 3H5a2 2 0 00-2 2v3m18 0V5a2 2 0 00-2-2h-3m0 18h3a2 2 0 002-2v-3M3 16v3a2 2 0 002 2h3" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Chart Container */}
      <div
        ref={containerRef}
        style={{
          height: isFullscreen ? 'calc(100vh - 65px)' : height,
          background: '#0B0E11',
          overflow: 'hidden',
        }}
      />

      {/* Click outside to close dropdowns */}
      {(showExchangeDropdown || showSymbolDropdown) && (
        <div
          className="fixed inset-0 z-10"
          onClick={() => {
            setShowExchangeDropdown(false)
            setShowSymbolDropdown(false)
          }}
        />
      )}
    </div>
  )
}

// 使用 memo 避免不必要的重渲染
export const TradingViewChart = memo(TradingViewChartComponent)
