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
import { useLanguage } from '../../contexts/LanguageContext'
import { httpClient } from '../../lib/httpClient'
import { t } from '../../i18n/translations'
import {
  calculateSMA,
  calculateEMA,
  calculateBollingerBands,
  type Kline,
} from '../../utils/indicators'
import { Settings, BarChart2 } from 'lucide-react'

// Order marker interface
interface OrderMarker {
  time: number
  price: number
  side: 'long' | 'short'
  rawSide: string // Original side field (buy/sell from database)
  action: 'open' | 'close'
  pnl?: number
  symbol: string
}

// Open orders interface (exchange TP/SL orders)
interface OpenOrder {
  order_id: string
  symbol: string
  side: string          // BUY/SELL
  position_side: string // LONG/SHORT
  type: string          // LIMIT/STOP_MARKET/TAKE_PROFIT_MARKET
  price: number         // Limit order price
  stop_price: number    // Trigger price (SL/TP)
  quantity: number
  status: string
}

interface AdvancedChartProps {
  symbol: string
  interval?: string
  traderID?: string
  height?: number
  exchange?: string // Exchange type: binance, bybit, okx, bitget, hyperliquid, aster, lighter
  onSymbolChange?: (symbol: string) => void // Symbol change callback
}

// Indicator configuration
interface IndicatorConfig {
  id: string
  name: string
  enabled: boolean
  color: string
  params?: any
}

// Get quote currency unit
const getQuoteUnit = (exchange: string): string => {
  if (['alpaca'].includes(exchange)) {
    return 'USD'
  }
  if (['forex', 'metals'].includes(exchange)) {
    return '' // Forex/metals have no real volume
  }
  return 'USDT' // Crypto defaults to USDT
}

// Get base volume unit
const getBaseUnit = (exchange: string, symbol: string, language: string): string => {
  if (['alpaca'].includes(exchange)) {
    return t('advancedChart.shares', language as 'en' | 'zh' | 'id')
  }
  if (['forex', 'metals'].includes(exchange)) {
    return ''
  }
  // Crypto: extract base asset from symbol
  const base = symbol.replace(/USDT$|USD$|BUSD$/, '')
  return base || t('advancedChart.units', language as 'en' | 'zh' | 'id')
}

// Format large numbers
const formatVolume = (value: number): string => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toFixed(2)
}

export function AdvancedChart({
  symbol = 'BTCUSDT',
  interval = '5m',
  traderID,
  height = 550,
  exchange = 'binance', // Default to binance
  onSymbolChange: _onSymbolChange, // Available for future use
}: AdvancedChartProps) {
  void _onSymbolChange // Prevent unused warning
  const { language } = useLanguage()
  const quoteUnit = getQuoteUnit(exchange)
  const baseUnit = getBaseUnit(exchange, symbol, language)
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candlestickSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null)
  const indicatorSeriesRef = useRef<Map<string, ISeriesApi<any>>>(new Map())
  const seriesMarkersRef = useRef<any>(null) // Markers primitive for v5
  const currentMarkersDataRef = useRef<any[]>([]) // Store current marker data
  const klineDataRef = useRef<Map<number, { volume: number; quoteVolume: number }>>(new Map()) // Store kline extra data
  const priceLinesRef = useRef<any[]>([]) // Store open order price lines

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showIndicatorPanel, setShowIndicatorPanel] = useState(false)
  const [showOrderMarkers, setShowOrderMarkers] = useState(true) // Order marker toggle, default on
  const isInitialLoadRef = useRef(true) // Track if this is initial load
  const [tooltipData, setTooltipData] = useState<any>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)

  // Market stats (current candle)
  const [marketStats, setMarketStats] = useState<{
    price: number
    priceChange: number
    priceChangePercent: number
    high: number
    low: number
    volume: number      // Quantity (BTC/shares)
    quoteVolume: number // Turnover (USDT/USD)
  } | null>(null)

  // Indicator configuration
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

  // Fetch kline data from service
  const fetchKlineData = async (symbol: string, interval: string) => {
    try {
      const limit = 1500
      const klineUrl = `/api/klines?symbol=${symbol}&interval=${interval}&limit=${limit}&exchange=${exchange}`
      const result = await httpClient.request(klineUrl, { silent: true })

      if (!result.success || !result.data) {
        throw new Error('Failed to fetch kline data')
      }

      // Convert data format
      const rawData = result.data.map((candle: any) => ({
        time: Math.floor(candle.openTime / 1000) as UTCTimestamp,
        open: candle.open,
        high: candle.high,
        low: candle.low,
        close: candle.close,
        volume: candle.volume,           // Quantity (BTC/shares)
        quoteVolume: candle.quoteVolume, // Turnover (USDT/USD)
      }))

      // Sort by time and deduplicate (lightweight-charts requires ascending, unique times)
      const sortedData = rawData.sort((a: any, b: any) => a.time - b.time)
      const dedupedData = sortedData.filter((item: any, index: number, arr: any[]) =>
        index === 0 || item.time !== arr[index - 1].time
      )

      if (rawData.length !== dedupedData.length) {
        console.warn('[AdvancedChart] Removed', rawData.length - dedupedData.length, 'duplicate klines')
      }

      return dedupedData
    } catch (err) {
      console.error('[AdvancedChart] Error fetching kline:', err)
      throw err
    }
  }

  // Parse time: supports Unix timestamp (number) or string format
  const parseCustomTime = (time: any): number => {
    if (!time) {
      console.warn('[AdvancedChart] Empty time value')
      return 0
    }

    // If already a number (Unix timestamp)
    if (typeof time === 'number') {
      // Determine ms vs seconds: if > 10^12, treat as milliseconds
      if (time > 1000000000000) {
        const seconds = Math.floor(time / 1000)
        console.log('[AdvancedChart] ✅ Unix timestamp (ms→s):', time, '→', seconds, '(', new Date(time).toISOString(), ')')
        return seconds
      }
      console.log('[AdvancedChart] ✅ Unix timestamp (s):', time, '(', new Date(time * 1000).toISOString(), ')')
      return time
    }

    const timeStr = String(time)
    console.log('[AdvancedChart] Parsing time string:', timeStr)

    // Try standard ISO format
    const isoTime = new Date(timeStr).getTime()
    if (!isNaN(isoTime) && isoTime > 0) {
      const timestamp = Math.floor(isoTime / 1000)
      console.log('[AdvancedChart] ✅ Parsed as ISO:', timeStr, '→', timestamp, '(', new Date(timestamp * 1000).toISOString(), ')')
      return timestamp
    }

    // Parse custom format "MM-DD HH:mm UTC" (for legacy data)
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

  // Fetch order data
  const fetchOrders = async (traderID: string, symbol: string): Promise<OrderMarker[]> => {
    try {
      console.log('[AdvancedChart] Fetching orders for trader:', traderID, 'symbol:', symbol)
      // Fetch filled orders, up to 200 for more history
      const result = await httpClient.request(
        `/api/orders?trader_id=${traderID}&symbol=${symbol}&status=FILLED&limit=200`,
        { silent: true }
      )

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

        // Handle field names: support PascalCase and snake_case
        const filledAt = order.filled_at || order.FilledAt || order.created_at || order.CreatedAt
        const avgPrice = order.avg_fill_price || order.AvgFillPrice || order.price || order.Price
        const orderAction = order.order_action || order.OrderAction
        const side = (order.side || order.Side)?.toLowerCase() // BUY/SELL
        const symbol = order.symbol || order.Symbol

        // Skip orders without fill time or price
        if (!filledAt || !avgPrice || avgPrice === 0) {
          console.warn('[AdvancedChart] Skipping order - missing data:', { filledAt, avgPrice })
          return
        }

        const timeSeconds = parseCustomTime(filledAt)
        if (timeSeconds === 0) {
          console.warn('[AdvancedChart] Skipping order - invalid time:', filledAt)
          return
        }

        // Determine open/close from order_action
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
          // If no order_action, infer from side
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
          rawSide: side, // Original side field (buy/sell)
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

  // Fetch exchange open orders (TP/SL)
  const fetchOpenOrders = async (traderID: string, symbol: string): Promise<OpenOrder[]> => {
    try {
      console.log('[AdvancedChart] Fetching open orders for trader:', traderID, 'symbol:', symbol)
      const result = await httpClient.request(
        `/api/open-orders?trader_id=${traderID}&symbol=${symbol}`,
        { silent: true }
      )

      console.log('[AdvancedChart] Open orders API response:', result)

      if (!result.success || !result.data) {
        console.warn('[AdvancedChart] No open orders found')
        return []
      }

      return result.data as OpenOrder[]
    } catch (err) {
      console.error('[AdvancedChart] Error fetching open orders:', err)
      return []
    }
  }

  // Initialize chart
  useEffect(() => {
    if (!chartContainerRef.current) return

    const chart = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth || 800,
      height: chartContainerRef.current.clientHeight || height,
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
      localization: {
        timeFormatter: (time: number) => {
          const date = new Date(time * 1000)
          return date.toLocaleString('zh-CN', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            hour12: false,
          })
        },
      },
    })

    chartRef.current = chart

    // Create candlestick series
    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    })
    candlestickSeriesRef.current = candlestickSeries as any

    // Create volume series
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

    // Responsive resize (ResizeObserver)
    const resizeObserver = new ResizeObserver((entries) => {
      if (entries.length === 0 || !entries[0].contentRect) return
      const { width, height } = entries[0].contentRect
      chart.applyOptions({ width, height })
    })

    if (chartContainerRef.current) {
      resizeObserver.observe(chartContainerRef.current)
    }

    // Listen for crosshair movement to show OHLC info
    chart.subscribeCrosshairMove((param) => {
      if (!param.time || !param.point || !candlestickSeriesRef.current) {
        setTooltipData(null)
        return
      }

      const data = param.seriesData.get(candlestickSeriesRef.current as any)
      if (!data) {
        setTooltipData(null)
        return
      }

      const candleData = data as any

      // Get volume and quoteVolume from stored data
      const klineExtra = klineDataRef.current.get(param.time as number) || { volume: 0, quoteVolume: 0 }

      setTooltipData({
        time: param.time,
        open: candleData.open,
        high: candleData.high,
        low: candleData.low,
        close: candleData.close,
        volume: klineExtra.volume,
        quoteVolume: klineExtra.quoteVolume,
        x: param.point.x,
        y: param.point.y,
      })
    })

    return () => {
      resizeObserver.disconnect()
      chart.remove()
    }
  }, []) // Chart is created once, ResizeObserver handles dimension changes


  // Load data and indicators
  useEffect(() => {
    // Reset initial load flag when symbol/interval changes (for auto-fit)
    isInitialLoadRef.current = true

    // Clear old marker data to prevent stale data in new chart
    currentMarkersDataRef.current = []
    if (seriesMarkersRef.current) {
      try {
        seriesMarkersRef.current.setMarkers([])
      } catch (e) {
        // Ignore errors, will be recreated later
      }
      seriesMarkersRef.current = null
    }

    const loadData = async (isRefresh = false) => {
      if (!candlestickSeriesRef.current) return

      console.log('[AdvancedChart] Loading data for', symbol, interval, isRefresh ? '(refresh)' : '')
      // Only show loading on first load, avoid flicker on refresh
      if (!isRefresh) {
        setLoading(true)
      }
      setError(null)

      try {
        // 1. Fetch kline data
        const klineData = await fetchKlineData(symbol, interval)
        console.log('[AdvancedChart] Loaded', klineData.length, 'klines')
        candlestickSeriesRef.current.setData(klineData)

        // Store volume/quoteVolume data for tooltip
        klineDataRef.current.clear()
        klineData.forEach((k: any) => {
          klineDataRef.current.set(k.time, { volume: k.volume || 0, quoteVolume: k.quoteVolume || 0 })
        })

        // 1.5 Calculate market stats
        if (klineData.length > 1) {
          const latestKline = klineData[klineData.length - 1]
          const prevKline = klineData[klineData.length - 2]

          // Price change: current candle close vs previous candle close
          const priceChange = latestKline.close - prevKline.close
          const priceChangePercent = (priceChange / prevKline.close) * 100

          setMarketStats({
            price: latestKline.close,
            priceChange,
            priceChangePercent,
            high: latestKline.high,
            low: latestKline.low,
            volume: latestKline.volume || 0,
            quoteVolume: latestKline.quoteVolume || 0,
          })
        } else if (klineData.length === 1) {
          const latestKline = klineData[0]
          setMarketStats({
            price: latestKline.close,
            priceChange: 0,
            priceChangePercent: 0,
            high: latestKline.high,
            low: latestKline.low,
            volume: latestKline.volume || 0,
            quoteVolume: latestKline.quoteVolume || 0,
          })
        }

        // 2. Display volume
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
            // Clear data when volume is disabled
            volumeSeriesRef.current.setData([])
          }
        }

        // 3. Add indicators
        updateIndicators(klineData)

        // 4. Fetch and display order markers
        if (traderID && candlestickSeriesRef.current) {
          console.log('[AdvancedChart] Starting to fetch orders...')
          const orders = await fetchOrders(traderID, symbol)
          console.log('[AdvancedChart] Received orders:', orders)

          if (orders.length > 0) {
            console.log('[AdvancedChart] Creating markers from', orders.length, 'orders')

            // Extract sorted kline time array
            const klineTimes = klineData.map((k: any) => k.time as number)
            const klineMinTime = klineTimes[0] || 0
            const klineMaxTime = klineTimes[klineTimes.length - 1] || 0
            console.log('[AdvancedChart] Kline time range:', klineMinTime, '-', klineMaxTime, '(', klineTimes.length, 'candles)')

            // Binary search: find the kline candle for the order time
            // Return the largest kline time <= orderTime
            const findCandleTime = (orderTime: number): number | null => {
              if (orderTime < klineMinTime || orderTime > klineMaxTime) {
                return null // Out of range
              }

              let left = 0
              let right = klineTimes.length - 1

              while (left < right) {
                const mid = Math.ceil((left + right + 1) / 2)
                if (klineTimes[mid] <= orderTime) {
                  left = mid
                } else {
                  right = mid - 1
                }
              }

              return klineTimes[left]
            }

            // Group orders by kline time
            const ordersByCandle = new Map<number, { buys: number; sells: number }>()

            orders.forEach(order => {
              // Use binary search to find matching kline candle time
              const candleTime = findCandleTime(order.time)

              if (candleTime === null) {
                console.warn('[AdvancedChart] ⚠️ Skipping order outside kline range:',
                  order.time, '(', new Date(order.time * 1000).toISOString(), ')')
                return
              }

              const existing = ordersByCandle.get(candleTime) || { buys: 0, sells: 0 }
              if (order.rawSide === 'buy') {
                existing.buys++
              } else {
                existing.sells++
              }
              ordersByCandle.set(candleTime, existing)
            })

            // Create markers for each kline with orders
            const markers: Array<{
              time: Time
              position: 'belowBar' | 'aboveBar'
              color: string
              shape: 'circle'
              text: string
              size: number
            }> = []

            ordersByCandle.forEach((counts, candleTime) => {
              // Show buy markers (green, below bar)
              if (counts.buys > 0) {
                markers.push({
                  time: candleTime as Time,
                  position: 'belowBar' as const,
                  color: '#0ECB81',
                  shape: 'circle' as const,
                  text: counts.buys > 1 ? `B${counts.buys}` : 'B',
                  size: 1,
                })
              }
              // Show sell markers (red, above bar)
              if (counts.sells > 0) {
                markers.push({
                  time: candleTime as Time,
                  position: 'aboveBar' as const,
                  color: '#F6465D',
                  shape: 'circle' as const,
                  text: counts.sells > 1 ? `S${counts.sells}` : 'S',
                  size: 1,
                })
              }
            })

            // Sort by time (lightweight-charts requires chronological order)
            markers.sort((a, b) => (a.time as number) - (b.time as number))

            console.log('[AdvancedChart] Valid markers:', markers.length, 'out of', orders.length)

            console.log('[AdvancedChart] Setting', markers.length, 'markers on candlestick series')
            console.log('[AdvancedChart] Markers data:', JSON.stringify(markers, null, 2))

            try {
              // Store marker data for later toggle use
              currentMarkersDataRef.current = markers

              // Using v5 API: createSeriesMarkers
              const markersToShow = showOrderMarkers ? markers : []

              if (seriesMarkersRef.current) {
                // If already exists, update markers
                seriesMarkersRef.current.setMarkers(markersToShow)
              } else {
                // First time creating markers
                seriesMarkersRef.current = createSeriesMarkers(candlestickSeriesRef.current, markersToShow)
              }
              console.log('[AdvancedChart] ✅ Markers updated! Count:', markersToShow.length, 'Visible:', showOrderMarkers)
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

        // Auto-fit view only on initial load, avoid jitter on refresh
        if (isInitialLoadRef.current) {
          chartRef.current?.timeScale().fitContent()
          isInitialLoadRef.current = false
        }
        setLoading(false)
      } catch (err: any) {
        console.error('[AdvancedChart] Error loading data:', err)
        setError(err.message || 'Failed to load chart data')
        setLoading(false)
      }
    }

    loadData(false) // Initial load

    // Real-time auto-refresh (every 5 seconds)
    const refreshInterval = setInterval(() => loadData(true), 5000)
    return () => clearInterval(refreshInterval)
  }, [symbol, interval, traderID, exchange])

  // Refresh open order price lines separately (every 60s, avoid frequent exchange API calls)
  useEffect(() => {
    if (!traderID || !candlestickSeriesRef.current) return

    // Load open orders and display price lines
    const loadOpenOrders = async () => {
      try {
        // Clear old price lines first
        priceLinesRef.current.forEach(line => {
          try {
            candlestickSeriesRef.current?.removePriceLine(line)
          } catch (e) {
            // Ignore clear error
          }
        })
        priceLinesRef.current = []

        const openOrders = await fetchOpenOrders(traderID, symbol)
        console.log('[AdvancedChart] Open orders for price lines:', openOrders)

        if (openOrders.length > 0 && candlestickSeriesRef.current) {
          openOrders.forEach(order => {
            // Get trigger price (SL/TP use stop_price, limit orders use price)
            const linePrice = order.stop_price > 0 ? order.stop_price : order.price
            if (linePrice <= 0) return

            // Determine order type
            const isStopLoss = order.type.includes('STOP') || order.type.includes('SL')
            const isTakeProfit = order.type.includes('TAKE_PROFIT') || order.type.includes('TP')
            const isLimit = order.type === 'LIMIT'

            // Set price line style
            let lineColor = '#F0B90B' // Default yellow
            const lineStyle = 2 // dashed
            let title = ''

            if (isStopLoss) {
              lineColor = '#F6465D' // red - stop loss
              title = `SL ${order.quantity}`
            } else if (isTakeProfit) {
              lineColor = '#0ECB81' // green - take profit
              title = `TP ${order.quantity}`
            } else if (isLimit) {
              lineColor = '#F0B90B' // yellow - limit order
              title = `Limit ${order.side} ${order.quantity}`
            } else {
              title = `${order.type} ${order.quantity}`
            }

            const priceLine = candlestickSeriesRef.current?.createPriceLine({
              price: linePrice,
              color: lineColor,
              lineWidth: 1,
              lineStyle: lineStyle,
              axisLabelVisible: true,
              title: title,
            })

            if (priceLine) {
              priceLinesRef.current.push(priceLine)
            }
          })
          console.log('[AdvancedChart] ✅ Created', priceLinesRef.current.length, 'price lines for pending orders')
        }
      } catch (err) {
        console.error('[AdvancedChart] Error loading open orders:', err)
      }
    }

    // Initial load (delay 1s to wait for chart initialization)
    const initialTimeout = setTimeout(loadOpenOrders, 1000)

    // Refresh open orders every 60 seconds
    const openOrdersInterval = setInterval(loadOpenOrders, 60000)

    return () => {
      clearTimeout(initialTimeout)
      clearInterval(openOrdersInterval)
    }
  }, [symbol, traderID])

  // Handle order marker show/hide separately to avoid reloading data
  useEffect(() => {
    if (!seriesMarkersRef.current) return

    try {
      const markersToShow = showOrderMarkers ? currentMarkersDataRef.current : []
      seriesMarkersRef.current.setMarkers(markersToShow)
      console.log('[AdvancedChart] 🔄 Toggled markers visibility:', showOrderMarkers, 'Count:', markersToShow.length)
    } catch (err) {
      console.error('[AdvancedChart] ❌ Failed to toggle markers:', err)
    }
  }, [showOrderMarkers])

  // Update indicators
  const updateIndicators = (klineData: Kline[]) => {
    if (!chartRef.current) return

    // Clear old indicators
    indicatorSeriesRef.current.forEach(series => {
      chartRef.current?.removeSeries(series as any)
    })
    indicatorSeriesRef.current.clear()

    // Add enabled indicators
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
          lineStyle: 2, // dashed
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

  // Toggle indicator
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
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Compact Professional Header */}
      <div
        className="flex items-center justify-between px-4 py-2"
        style={{ borderBottom: '1px solid rgba(43, 49, 57, 0.6)', background: '#0D1117', flexShrink: 0 }}
      >
        {/* Left: Symbol Info + Price */}
        <div className="flex items-center gap-4">
          {/* Symbol & Interval */}
          <div className="flex items-center gap-2">
            <span className="text-sm font-bold text-white">{symbol}</span>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#1F2937] text-gray-400">{interval}</span>
            <span
              className="text-[10px] px-1.5 py-0.5 rounded font-medium uppercase"
              style={{
                background: exchange === 'hyperliquid' ? 'rgba(80, 227, 194, 0.1)' : 'rgba(243, 186, 47, 0.1)',
                color: exchange === 'hyperliquid' ? '#50E3C2' : '#F3BA2F',
              }}
            >
              {exchange?.toUpperCase()}
            </span>
          </div>

          {/* Price Display */}
          {marketStats && (
            <div className="flex items-center gap-3 pl-3 border-l border-[#2B3139]">
              <span
                className="text-base font-bold tabular-nums"
                style={{ color: marketStats.priceChange >= 0 ? '#10B981' : '#EF4444' }}
              >
                {marketStats.price.toLocaleString(undefined, {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: exchange === 'forex' || exchange === 'metals' ? 4 : 2
                })}
              </span>
              <span
                className="text-xs font-medium px-1.5 py-0.5 rounded tabular-nums"
                style={{
                  background: marketStats.priceChange >= 0 ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                  color: marketStats.priceChange >= 0 ? '#10B981' : '#EF4444',
                }}
              >
                {marketStats.priceChange >= 0 ? '+' : ''}{marketStats.priceChangePercent.toFixed(2)}%
              </span>

              {/* Compact H/L */}
              <div className="flex items-center gap-2 text-[11px] text-gray-500">
                <span>H <span className="text-gray-300">{marketStats.high.toFixed(2)}</span></span>
                <span>L <span className="text-gray-300">{marketStats.low.toFixed(2)}</span></span>
                {marketStats.volume > 0 && baseUnit && (
                  <span>Vol <span className="text-gray-300">{formatVolume(marketStats.volume)}</span></span>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Right: Controls */}
        <div className="flex items-center gap-1.5">
          {loading && (
            <span className="text-[10px] text-yellow-400 animate-pulse mr-2">
              {t('advancedChart.updating', language)}
            </span>
          )}
          <button
            onClick={() => setShowIndicatorPanel(!showIndicatorPanel)}
            className="flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium transition-all"
            style={{
              background: showIndicatorPanel ? 'rgba(96, 165, 250, 0.15)' : 'transparent',
              color: showIndicatorPanel ? '#60A5FA' : '#6B7280',
            }}
          >
            <Settings className="w-3 h-3" />
            <span>{t('advancedChart.indicators', language)}</span>
          </button>

          <button
            onClick={() => setShowOrderMarkers(!showOrderMarkers)}
            className="flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium transition-all"
            style={{
              background: showOrderMarkers ? 'rgba(16, 185, 129, 0.15)' : 'transparent',
              color: showOrderMarkers ? '#10B981' : '#6B7280',
            }}
            title={t('advancedChart.orderMarkers', language)}
          >
            <span>B/S</span>
          </button>
        </div>
      </div>

      {/* Indicator panel - professional design */}
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
          {/* Title bar */}
          <div
            className="flex items-center justify-between px-4 py-3 border-b"
            style={{ borderColor: 'rgba(43, 49, 57, 0.5)' }}
          >
            <div className="flex items-center gap-2">
              <BarChart2 className="w-4 h-4 text-yellow-400" />
              <h4 className="text-sm font-bold text-white">
                {t('advancedChart.technicalIndicators', language)}
              </h4>
            </div>
            <button
              onClick={() => setShowIndicatorPanel(false)}
              className="text-gray-400 hover:text-white transition-colors"
            >
              <span className="text-lg">×</span>
            </button>
          </div>

          {/* Indicator list */}
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

          {/* Bottom hint */}
          <div
            className="px-4 py-2 text-xs text-gray-500 border-t"
            style={{ borderColor: 'rgba(43, 49, 57, 0.5)' }}
          >
            {t('advancedChart.clickToToggle', language)}
          </div>
        </div>
      )}

      {/* Chart container */}
      <div style={{ position: 'relative', flex: 1, minHeight: 0 }}>
        <div ref={chartContainerRef} style={{ height: '100%', width: '100%' }} />

        {/* OHLC Tooltip */}
        {tooltipData && (
          <div
            ref={tooltipRef}
            style={{
              position: 'absolute',
              left: '10px',
              top: '10px',
              padding: '8px 12px',
              background: 'rgba(15, 18, 21, 0.95)',
              border: '1px solid rgba(240, 185, 11, 0.3)',
              borderRadius: '6px',
              color: '#EAECEF',
              fontSize: '12px',
              fontFamily: 'monospace',
              pointerEvents: 'none',
              zIndex: 10,
              backdropFilter: 'blur(10px)',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.5)',
            }}
          >
            <div style={{ marginBottom: '6px', color: '#F0B90B', fontWeight: 'bold', fontSize: '11px' }}>
              {new Date((tooltipData.time as number) * 1000).toLocaleString(language === 'zh' ? 'zh-CN' : 'en-US', {
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
              })}
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '4px 12px', fontSize: '11px' }}>
              <span style={{ color: '#848E9C' }}>O:</span>
              <span style={{ color: '#EAECEF', fontWeight: '500' }}>{tooltipData.open?.toFixed(2)}</span>

              <span style={{ color: '#848E9C' }}>H:</span>
              <span style={{ color: '#0ECB81', fontWeight: '500' }}>{tooltipData.high?.toFixed(2)}</span>

              <span style={{ color: '#848E9C' }}>L:</span>
              <span style={{ color: '#F6465D', fontWeight: '500' }}>{tooltipData.low?.toFixed(2)}</span>

              <span style={{ color: '#848E9C' }}>C:</span>
              <span style={{
                color: tooltipData.close >= tooltipData.open ? '#0ECB81' : '#F6465D',
                fontWeight: 'bold'
              }}>
                {tooltipData.close?.toFixed(2)}
              </span>

              {tooltipData.volume > 0 && baseUnit && (
                <>
                  <span style={{ color: '#848E9C' }}>V({baseUnit}):</span>
                  <span style={{ color: '#3B82F6', fontWeight: '500' }}>
                    {formatVolume(tooltipData.volume)}
                  </span>
                </>
              )}

              {tooltipData.quoteVolume > 0 && quoteUnit && (
                <>
                  <span style={{ color: '#848E9C' }}>V({quoteUnit}):</span>
                  <span style={{ color: '#3B82F6', fontWeight: '500' }}>
                    {formatVolume(tooltipData.quoteVolume)}
                  </span>
                </>
              )}
            </div>
          </div>
        )}

        {/* NOFX watermark */}
        <div
          style={{
            position: 'absolute',
            bottom: '20%',
            right: '5%',
            pointerEvents: 'none',
            userSelect: 'none',
            zIndex: 1,
          }}
        >
          <div
            style={{
              fontSize: '56px',
              fontWeight: '700',
              color: 'rgba(240, 185, 11, 0.12)',
              letterSpacing: '4px',
              fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
              textShadow: '0 2px 30px rgba(240, 185, 11, 0.2)',
            }}
          >
            NOFX
          </div>
        </div>
      </div>

      {/* Error message */}
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

    </div>
  )
}
