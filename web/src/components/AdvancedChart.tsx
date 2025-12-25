import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  IChartApi,
  ISeriesApi,
  Time,
  UTCTimestamp,
  CandlestickSeries,
  LineSeries,
  HistogramSeries,
  createSeriesMarkers,
} from 'lightweight-charts'
import { useLanguage } from '../contexts/LanguageContext'
import { httpClient } from '../lib/httpClient'
import {
  calculateSMA,
  calculateEMA,
  calculateBollingerBands,
  type Kline,
} from '../utils/indicators'
import { Settings, TrendingUp, BarChart2 } from 'lucide-react'

// 订单接口定义
interface OrderMarker {
  time: number
  price: number
  side: 'long' | 'short'
  rawSide: string // 原始 side 字段 (buy/sell from database)
  action: 'open' | 'close'
  pnl?: number
  symbol: string
}

interface AdvancedChartProps {
  symbol: string
  interval?: string
  traderID?: string
  height?: number
  exchange?: string // 交易所类型：binance, bybit, okx, bitget, hyperliquid, aster, lighter
  onSymbolChange?: (symbol: string) => void // 币种切换回调
}

// 指标配置
interface IndicatorConfig {
  id: string
  name: string
  enabled: boolean
  color: string
  params?: any
}

// 热门币种
const POPULAR_SYMBOLS = [
  'BTCUSDT',
  'ETHUSDT',
  'SOLUSDT',
  'BNBUSDT',
  'XRPUSDT',
  'DOGEUSDT',
  'ADAUSDT',
  'AVAXUSDT',
]

export function AdvancedChart({
  symbol = 'BTCUSDT',
  interval = '5m',
  traderID,
  height = 550,
  exchange = 'binance', // 默认使用 binance
  onSymbolChange,
}: AdvancedChartProps) {
  const { language } = useLanguage()
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candlestickSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null)
  const indicatorSeriesRef = useRef<Map<string, ISeriesApi<any>>>(new Map())
  const seriesMarkersRef = useRef<any>(null) // Markers primitive for v5

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showIndicatorPanel, setShowIndicatorPanel] = useState(false)

  // 指标配置
  const [indicators, setIndicators] = useState<IndicatorConfig[]>([
    { id: 'volume', name: 'Volume', enabled: true, color: '#3B82F6' },
    { id: 'ma5', name: 'MA5', enabled: false, color: '#FF6B6B', params: { period: 5 } },
    { id: 'ma10', name: 'MA10', enabled: false, color: '#4ECDC4', params: { period: 10 } },
    { id: 'ma20', name: 'MA20', enabled: false, color: '#FFD93D', params: { period: 20 } },
    { id: 'ma60', name: 'MA60', enabled: false, color: '#95E1D3', params: { period: 60 } },
    { id: 'ema12', name: 'EMA12', enabled: false, color: '#A8E6CF', params: { period: 12 } },
    { id: 'ema26', name: 'EMA26', enabled: false, color: '#FFD3B6', params: { period: 26 } },
    { id: 'bb', name: 'Bollinger Bands', enabled: false, color: '#9B59B6' },
  ])

  // 从服务获取K线数据
  const fetchKlineData = async (symbol: string, interval: string) => {
    try {
      const limit = 1500
      const klineUrl = `/api/klines?symbol=${symbol}&interval=${interval}&limit=${limit}&exchange=${exchange}`
      const result = await httpClient.get(klineUrl)

      if (!result.success || !result.data) {
        throw new Error('Failed to fetch kline data')
      }

      return result.data.map((candle: any) => ({
        time: Math.floor(candle.openTime / 1000) as UTCTimestamp,
        open: candle.open,
        high: candle.high,
        low: candle.low,
        close: candle.close,
        volume: candle.volume,
      }))
    } catch (err) {
      console.error('[AdvancedChart] Error fetching kline:', err)
      throw err
    }
  }

  // 解析时间：支持 Unix 时间戳（数字）或字符串格式
  const parseCustomTime = (time: any): number => {
    if (!time) {
      console.warn('[AdvancedChart] Empty time value')
      return 0
    }

    // 如果已经是数字（Unix 时间戳），直接返回
    if (typeof time === 'number') {
      console.log('[AdvancedChart] ✅ Unix timestamp:', time, '(', new Date(time * 1000).toISOString(), ')')
      return time
    }

    const timeStr = String(time)
    console.log('[AdvancedChart] Parsing time string:', timeStr)

    // 尝试标准ISO格式
    const isoTime = new Date(timeStr).getTime()
    if (!isNaN(isoTime) && isoTime > 0) {
      const timestamp = Math.floor(isoTime / 1000)
      console.log('[AdvancedChart] ✅ Parsed as ISO:', timeStr, '→', timestamp, '(', new Date(timestamp * 1000).toISOString(), ')')
      return timestamp
    }

    // 解析自定义格式 "MM-DD HH:mm UTC" (兼容旧数据)
    const match = timeStr.match(/(\d{2})-(\d{2})\s+(\d{2}):(\d{2})\s+UTC/)
    if (match) {
      const currentYear = new Date().getFullYear()
      const [_, month, day, hour, minute] = match
      const date = new Date(Date.UTC(
        currentYear,
        parseInt(month) - 1,
        parseInt(day),
        parseInt(hour),
        parseInt(minute)
      ))
      const timestamp = Math.floor(date.getTime() / 1000)
      console.log('[AdvancedChart] ✅ Parsed as custom format:', timeStr, '→', timestamp, '(', new Date(timestamp * 1000).toISOString(), ')')
      return timestamp
    }

    console.error('[AdvancedChart] ❌ Failed to parse time:', timeStr)
    return 0
  }

  // 获取订单数据
  const fetchOrders = async (traderID: string, symbol: string): Promise<OrderMarker[]> => {
    try {
      console.log('[AdvancedChart] Fetching orders for trader:', traderID, 'symbol:', symbol)
      // 获取已成交的订单，限制50条避免标记太多重叠
      const result = await httpClient.get(`/api/orders?trader_id=${traderID}&symbol=${symbol}&status=FILLED&limit=50`)

      console.log('[AdvancedChart] Orders API response:', result)

      if (!result.success || !result.data) {
        console.warn('[AdvancedChart] No orders found, result:', result)
        return []
      }

      const orders = result.data
      console.log('[AdvancedChart] Raw orders data:', orders)
      const markers: OrderMarker[] = []

      orders.forEach((order: any) => {
        console.log('[AdvancedChart] Processing order:', order)

        // 处理字段名：支持PascalCase和snake_case
        const filledAt = order.filled_at || order.FilledAt || order.created_at || order.CreatedAt
        const avgPrice = order.avg_fill_price || order.AvgFillPrice || order.price || order.Price
        const orderAction = order.order_action || order.OrderAction
        const side = (order.side || order.Side)?.toLowerCase() // BUY/SELL
        const symbol = order.symbol || order.Symbol

        // 跳过没有成交时间或价格的订单
        if (!filledAt || !avgPrice || avgPrice === 0) {
          console.warn('[AdvancedChart] Skipping order - missing data:', { filledAt, avgPrice })
          return
        }

        const timeSeconds = parseCustomTime(filledAt)
        if (timeSeconds === 0) {
          console.warn('[AdvancedChart] Skipping order - invalid time:', filledAt)
          return
        }

        // 根据 order_action 判断是开仓还是平仓
        let action: 'open' | 'close' = 'open'
        let positionSide: 'long' | 'short' = 'long'

        if (orderAction) {
          if (orderAction.includes('OPEN')) {
            action = 'open'
            positionSide = orderAction.includes('LONG') ? 'long' : 'short'
          } else if (orderAction.includes('CLOSE')) {
            action = 'close'
            positionSide = orderAction.includes('LONG') ? 'long' : 'short'
          }
        } else {
          // 如果没有 order_action，根据 side 判断
          positionSide = side === 'buy' ? 'long' : 'short'
        }

        console.log('[AdvancedChart] Order marker:', {
          time: timeSeconds,
          price: avgPrice,
          side: positionSide,
          rawSide: side,
          action,
          orderAction
        })

        markers.push({
          time: timeSeconds,
          price: avgPrice,
          side: positionSide,
          rawSide: side, // 原始 side 字段 (buy/sell)
          action: action,
          symbol,
        })
      })

      console.log('[AdvancedChart] Final markers:', markers)
      return markers
    } catch (err) {
      console.error('[AdvancedChart] Error fetching orders:', err)
      return []
    }
  }

  // 初始化图表
  useEffect(() => {
    if (!chartContainerRef.current) return

    const chart = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth,
      height: height,
      layout: {
        background: { color: '#0B0E11' },
        textColor: '#B7BDC6',
        fontSize: 12,
      },
      grid: {
        vertLines: {
          color: 'rgba(43, 49, 57, 0.2)',
          style: 1,
          visible: true,
        },
        horzLines: {
          color: 'rgba(43, 49, 57, 0.2)',
          style: 1,
          visible: true,
        },
      },
      crosshair: {
        mode: 1,
        vertLine: {
          color: 'rgba(240, 185, 11, 0.5)',
          width: 1,
          style: 2,
          labelBackgroundColor: '#F0B90B',
        },
        horzLine: {
          color: 'rgba(240, 185, 11, 0.5)',
          width: 1,
          style: 2,
          labelBackgroundColor: '#F0B90B',
        },
      },
      rightPriceScale: {
        borderColor: '#2B3139',
        scaleMargins: {
          top: 0.1,
          bottom: 0.25,
        },
        borderVisible: true,
        entireTextOnly: false,
      },
      timeScale: {
        borderColor: '#2B3139',
        timeVisible: true,
        secondsVisible: false,
        borderVisible: true,
        rightOffset: 5,
        barSpacing: 8,
      },
      handleScroll: {
        mouseWheel: true,
        pressedMouseMove: true,
        horzTouchDrag: true,
        vertTouchDrag: true,
      },
      handleScale: {
        axisPressedMouseMove: true,
        mouseWheel: true,
        pinch: true,
      },
    })

    chartRef.current = chart

    // 创建K线系列
    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    })
    candlestickSeriesRef.current = candlestickSeries as any

    // 创建成交量系列
    const volumeSeries = chart.addSeries(HistogramSeries, {
      color: '#26a69a',
      priceFormat: {
        type: 'volume',
      },
      priceScaleId: '',
      lastValueVisible: false,
      priceLineVisible: false,
    })
    volumeSeriesRef.current = volumeSeries as any

    // 响应式调整
    const handleResize = () => {
      if (chartContainerRef.current && chartRef.current) {
        chartRef.current.applyOptions({
          width: chartContainerRef.current.clientWidth,
        })
      }
    }

    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
    }
  }, [height])

  // 加载数据和指标
  useEffect(() => {
    const loadData = async () => {
      if (!candlestickSeriesRef.current) return

      console.log('[AdvancedChart] Loading data for', symbol, interval)
      setLoading(true)
      setError(null)

      try {
        // 1. 获取K线数据
        const klineData = await fetchKlineData(symbol, interval)
        console.log('[AdvancedChart] Loaded', klineData.length, 'klines')
        candlestickSeriesRef.current.setData(klineData)

        // 2. 显示成交量
        if (volumeSeriesRef.current) {
          const volumeEnabled = indicators.find(i => i.id === 'volume')?.enabled
          if (volumeEnabled) {
            const volumeData = klineData.map((k: Kline) => ({
              time: k.time,
              value: k.volume || 0,
              color: k.close >= k.open ? 'rgba(14, 203, 129, 0.5)' : 'rgba(246, 70, 93, 0.5)',
            }))
            volumeSeriesRef.current.setData(volumeData)
          } else {
            // 关闭成交量时清空数据
            volumeSeriesRef.current.setData([])
          }
        }

        // 3. 添加指标
        updateIndicators(klineData)

        // 4. 获取并显示订单标记
        if (traderID && candlestickSeriesRef.current) {
          console.log('[AdvancedChart] Starting to fetch orders...')
          const orders = await fetchOrders(traderID, symbol)
          console.log('[AdvancedChart] Received orders:', orders)

          if (orders.length > 0) {
            console.log('[AdvancedChart] Creating markers from', orders.length, 'orders')

            // 过滤掉无效时间戳的订单（小于2024年的时间戳）
            const minValidTimestamp = new Date('2024-01-01').getTime() / 1000
            const validOrders = orders.filter(order => {
              if (order.time < minValidTimestamp) {
                console.warn('[AdvancedChart] ⚠️ Skipping order with invalid timestamp:', order.time, '(', new Date(order.time * 1000).toISOString(), ')')
                return false
              }
              return true
            })

            console.log('[AdvancedChart] Valid orders:', validOrders.length, 'out of', orders.length)

            const markers = validOrders.map(order => {
              // 直接使用 rawSide 字段判断买卖（更准确）
              // rawSide = 'buy' → 绿色 B
              // rawSide = 'sell' → 红色 S
              const isBuy = order.rawSide === 'buy'

              const marker = {
                time: order.time as Time,
                position: 'belowBar' as const,
                color: isBuy ? '#0ECB81' : '#F6465D', // BUY绿色, SELL红色
                shape: 'circle' as const, // 使用圆形作为背景
                text: isBuy ? 'B' : 'S', // 显示 B 或 S
                size: 1, // 稍微大一点以显示文字
              }

              console.log('[AdvancedChart] ✅ Created marker:', marker.text, 'for', order.rawSide, 'at', new Date(order.time * 1000).toISOString())
              return marker
            })

            console.log('[AdvancedChart] Setting', markers.length, 'markers on candlestick series')
            console.log('[AdvancedChart] Markers data:', JSON.stringify(markers, null, 2))

            try {
              // 使用 v5 API: createSeriesMarkers
              if (seriesMarkersRef.current) {
                // 如果已经存在，更新标记
                seriesMarkersRef.current.setMarkers(markers)
              } else {
                // 首次创建标记
                seriesMarkersRef.current = createSeriesMarkers(candlestickSeriesRef.current, markers)
              }
              console.log('[AdvancedChart] ✅ Markers set successfully!')
            } catch (err) {
              console.error('[AdvancedChart] ❌ Failed to set markers:', err)
            }
          } else {
            console.log('[AdvancedChart] No orders found, clearing markers')
            try {
              if (seriesMarkersRef.current) {
                seriesMarkersRef.current.setMarkers([])
              }
            } catch (err) {
              console.error('[AdvancedChart] Failed to clear markers:', err)
            }
          }
        } else {
          console.log('[AdvancedChart] Skipping markers:', {
            hasTraderID: !!traderID,
            hasSeries: !!candlestickSeriesRef.current
          })
        }

        // 自动适配视图
        chartRef.current?.timeScale().fitContent()
        setLoading(false)
      } catch (err: any) {
        console.error('[AdvancedChart] Error loading data:', err)
        setError(err.message || 'Failed to load chart data')
        setLoading(false)
      }
    }

    loadData()

    // 实时自动刷新 (5秒更新一次)
    const refreshInterval = setInterval(loadData, 5000)
    return () => clearInterval(refreshInterval)
  }, [symbol, interval, traderID, indicators])

  // 更新指标
  const updateIndicators = (klineData: Kline[]) => {
    if (!chartRef.current) return

    // 清除旧指标
    indicatorSeriesRef.current.forEach(series => {
      chartRef.current?.removeSeries(series as any)
    })
    indicatorSeriesRef.current.clear()

    // 添加启用的指标
    indicators.forEach(indicator => {
      if (!indicator.enabled || !chartRef.current) return

      if (indicator.id.startsWith('ma')) {
        const maData = calculateSMA(klineData, indicator.params.period)
        const series = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 2,
          title: indicator.name,
        })
        series.setData(maData as any)
        indicatorSeriesRef.current.set(indicator.id, series)
      } else if (indicator.id.startsWith('ema')) {
        const emaData = calculateEMA(klineData, indicator.params.period)
        const series = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 2,
          title: indicator.name,
          lineStyle: 2, // 虚线
        })
        series.setData(emaData as any)
        indicatorSeriesRef.current.set(indicator.id, series)
      } else if (indicator.id === 'bb') {
        const bbData = calculateBollingerBands(klineData)

        const upperSeries = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 1,
          title: 'BB Upper',
        })
        upperSeries.setData(bbData.map(d => ({ time: d.time as any, value: d.upper })))

        const middleSeries = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 1,
          lineStyle: 2,
          title: 'BB Middle',
        })
        middleSeries.setData(bbData.map(d => ({ time: d.time as any, value: d.middle })))

        const lowerSeries = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 1,
          title: 'BB Lower',
        })
        lowerSeries.setData(bbData.map(d => ({ time: d.time as any, value: d.lower })))

        indicatorSeriesRef.current.set(indicator.id + '_upper', upperSeries)
        indicatorSeriesRef.current.set(indicator.id + '_middle', middleSeries)
        indicatorSeriesRef.current.set(indicator.id + '_lower', lowerSeries)
      }
    })
  }

  // 切换指标
  const toggleIndicator = (id: string) => {
    setIndicators(prev =>
      prev.map(ind => (ind.id === id ? { ...ind, enabled: !ind.enabled } : ind))
    )
  }

  return (
    <div
      className="relative shadow-xl"
      style={{
        background: 'linear-gradient(180deg, #0F1215 0%, #0B0E11 100%)',
        borderRadius: '12px',
        overflow: 'hidden',
        border: '1px solid rgba(43, 49, 57, 0.5)',
      }}
    >
      {/* 标题栏 - 专业化设计 */}
      <div
        className="px-4 py-2.5 space-y-2"
        style={{ borderBottom: '1px solid #2B3139', background: 'linear-gradient(180deg, #1A1E23 0%, #0B0E11 100%)' }}
      >
        {/* 第一行：标题和控制按钮 */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <TrendingUp className="w-5 h-5 text-yellow-400" />
            <h3 className="text-base font-bold" style={{ color: '#F0B90B' }}>
              {symbol}
            </h3>
            <span className="text-xs px-2 py-0.5 rounded" style={{ background: '#2B3139', color: '#848E9C' }}>
              {interval}
            </span>
          </div>

          <div className="flex items-center gap-2">
            {loading && (
              <div className="text-xs px-2 py-1 rounded" style={{ background: '#2B3139', color: '#F0B90B' }}>
                {language === 'zh' ? '更新中...' : 'Updating...'}
              </div>
            )}
            <button
              onClick={() => setShowIndicatorPanel(!showIndicatorPanel)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all"
              style={{
                background: showIndicatorPanel ? 'rgba(240, 185, 11, 0.15)' : 'rgba(255, 255, 255, 0.05)',
                color: showIndicatorPanel ? '#F0B90B' : '#848E9C',
                border: `1px solid ${showIndicatorPanel ? 'rgba(240, 185, 11, 0.3)' : '#2B3139'}`,
              }}
            >
              <Settings className="w-3.5 h-3.5" />
              <span>{language === 'zh' ? '指标' : 'Indicators'}</span>
            </button>
          </div>
        </div>

        {/* 第二行：热门币种快速选择 */}
        {onSymbolChange && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] font-medium mr-1" style={{ color: '#848E9C' }}>
              {language === 'zh' ? '快速选择:' : 'Quick:'}
            </span>
            {POPULAR_SYMBOLS.map((sym) => (
              <button
                key={sym}
                onClick={() => onSymbolChange(sym)}
                className="px-2 py-1 rounded text-[11px] font-medium transition-all"
                style={{
                  background: symbol === sym ? 'rgba(240, 185, 11, 0.2)' : 'rgba(43, 49, 57, 0.5)',
                  color: symbol === sym ? '#F0B90B' : '#848E9C',
                  border: `1px solid ${symbol === sym ? 'rgba(240, 185, 11, 0.4)' : 'transparent'}`,
                }}
              >
                {sym.replace('USDT', '')}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* 指标面板 - 专业化设计 */}
      {showIndicatorPanel && (
        <div
          className="absolute top-16 right-4 z-10 rounded-lg shadow-2xl backdrop-blur-sm"
          style={{
            background: 'linear-gradient(135deg, #1A1E23 0%, #0F1215 100%)',
            border: '1px solid rgba(240, 185, 11, 0.2)',
            maxHeight: '500px',
            minWidth: '280px',
            overflowY: 'auto',
          }}
        >
          {/* 标题栏 */}
          <div
            className="flex items-center justify-between px-4 py-3 border-b"
            style={{ borderColor: 'rgba(43, 49, 57, 0.5)' }}
          >
            <div className="flex items-center gap-2">
              <BarChart2 className="w-4 h-4 text-yellow-400" />
              <h4 className="text-sm font-bold text-white">
                {language === 'zh' ? '技术指标' : 'Technical Indicators'}
              </h4>
            </div>
            <button
              onClick={() => setShowIndicatorPanel(false)}
              className="text-gray-400 hover:text-white transition-colors"
            >
              <span className="text-lg">×</span>
            </button>
          </div>

          {/* 指标列表 */}
          <div className="p-3 space-y-1">
            {indicators.map(indicator => (
              <label
                key={indicator.id}
                className="flex items-center gap-3 p-2.5 rounded-md hover:bg-white/5 cursor-pointer transition-all group"
              >
                <div className="relative">
                  <input
                    type="checkbox"
                    checked={indicator.enabled}
                    onChange={() => toggleIndicator(indicator.id)}
                    className="w-4 h-4 rounded border-gray-600 text-yellow-500 focus:ring-2 focus:ring-yellow-500/50"
                  />
                </div>
                <div
                  className="w-8 h-3 rounded-sm border border-white/10"
                  style={{ backgroundColor: indicator.color }}
                ></div>
                <span className="text-sm text-gray-300 group-hover:text-white transition-colors flex-1">
                  {indicator.name}
                </span>
                {indicator.enabled && (
                  <span className="text-xs text-yellow-400">●</span>
                )}
              </label>
            ))}
          </div>

          {/* 底部提示 */}
          <div
            className="px-4 py-2 text-xs text-gray-500 border-t"
            style={{ borderColor: 'rgba(43, 49, 57, 0.5)' }}
          >
            {language === 'zh' ? '点击选择需要显示的指标' : 'Click to toggle indicators'}
          </div>
        </div>
      )}

      {/* 图表容器 */}
      <div ref={chartContainerRef} style={{ position: 'relative' }} />

      {/* 错误提示 */}
      {error && (
        <div
          className="absolute inset-0 flex items-center justify-center"
          style={{ background: 'rgba(11, 14, 17, 0.9)' }}
        >
          <div className="text-center">
            <div className="text-2xl mb-2">⚠️</div>
            <div style={{ color: '#F6465D' }}>{error}</div>
          </div>
        </div>
      )}

      {/* 图例说明 - 简化版 */}
      <div
        className="flex items-center gap-4 px-4 py-2.5 text-xs"
        style={{ borderTop: '1px solid #2B3139', background: '#0F1215' }}
      >
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold" style={{ background: '#0ECB81', color: '#0B0E11' }}>
            B
          </div>
          <span style={{ color: '#EAECEF' }}>{language === 'zh' ? '买入 (BUY)' : 'BUY'}</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold" style={{ background: '#F6465D', color: '#0B0E11' }}>
            S
          </div>
          <span style={{ color: '#EAECEF' }}>{language === 'zh' ? '卖出 (SELL)' : 'SELL'}</span>
        </div>
      </div>
    </div>
  )
}
