import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  IChartApi,
  IPriceLine,
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
import { AlertTriangle, BarChart2, Settings, X } from 'lucide-react'
import { useThemeVersion, type ChartTheme } from '../../lib/chartTheme'
import { getCandleChartTheme } from './candleTheme'
import { Badge, Button, formatSigned } from '../ui'
import { cn } from '../../lib/cn'

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
  side: string // BUY/SELL
  position_side: string // LONG/SHORT
  type: string // LIMIT/STOP_MARKET/TAKE_PROFIT_MARKET
  price: number // Limit order price
  stop_price: number // Trigger price (SL/TP)
  quantity: number
  status: string
}

// Colour role of an open-order price line: stop loss = down, take profit = up,
// anything else (limit) = brand
type PriceLineTone = 'up' | 'down' | 'brand'

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
const getBaseUnit = (
  exchange: string,
  symbol: string,
  language: string
): string => {
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

// Adaptive price precision — TradingView style (fewer decimals for large prices)
const formatLegendPrice = (value: number | undefined): string => {
  if (value === undefined || value === null || !Number.isFinite(value))
    return '-'
  const abs = Math.abs(value)
  if (abs >= 1000) return value.toFixed(2)
  if (abs >= 1) return value.toFixed(4)
  if (abs >= 0.01) return value.toFixed(5)
  return value.toFixed(6)
}

// Axis precision mirrors the legend: small-price symbols (0.0737) show
// meaningful decimals on the price scale instead of everything collapsing
// to "0.07" (2026-09-27 user report).
const priceAxisPrecision = (value: number | undefined): number => {
  const abs = Math.abs(value ?? 0)
  if (abs >= 1000) return 2
  if (abs >= 1) return 4
  if (abs >= 0.01) return 5
  return 6
}

// Volume histogram: one bar per candle, coloured by candle direction. Colours
// are baked into each bar, so the histogram is rebuilt when the theme changes.
const buildVolumeData = (klines: Kline[], th: ChartTheme) =>
  klines.map((k: Kline) => ({
    time: k.time as Time,
    value: k.volume || 0,
    color: k.close >= k.open ? th.volumeUp : th.volumeDown,
  }))

// mergeKlines folds a freshly fetched tail into the loaded history: bars with
// a known time replace the stored bar (the forming candle updates), newer bars
// append. Both inputs are ascending by time.
export const mergeKlines = <T extends { time: number }>(
  history: T[],
  tail: T[]
): T[] => {
  if (tail.length === 0) return history
  const firstTail = tail[0].time
  let cut = history.length
  while (cut > 0 && history[cut - 1].time >= firstTail) cut--
  return history.slice(0, cut).concat(tail)
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
  const themeVersion = useThemeVersion()
  const quoteUnit = getQuoteUnit(exchange)
  const baseUnit = getBaseUnit(exchange, symbol, language)
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candlestickSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null)
  const indicatorSeriesRef = useRef<Map<string, ISeriesApi<any>>>(new Map())
  const seriesMarkersRef = useRef<any>(null) // Markers primitive for v5
  const currentMarkersDataRef = useRef<any[]>([]) // Store current marker data
  const klineDataRef = useRef<
    Map<number, { volume: number; quoteVolume: number }>
  >(new Map()) // Store kline extra data
  const priceLinesRef = useRef<
    Array<{ line: IPriceLine; tone: PriceLineTone }>
  >([]) // Store open order price lines

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showIndicatorPanel, setShowIndicatorPanel] = useState(false)
  const [showOrderMarkers, setShowOrderMarkers] = useState(true) // Order marker toggle, default on
  const isInitialLoadRef = useRef(true) // Track if this is initial load
  const [legendData, setLegendData] = useState<{
    open: number
    high: number
    low: number
    close: number
    volume: number
    quoteVolume: number
    change: number
    changePercent: number
    amplitude: number
  } | null>(null) // TradingView-style legend (hovered candle or latest)
  const klineListRef = useRef<any[]>([]) // Sorted kline array for legend lookups
  const lastOrdersFetchRef = useRef(0) // order markers refresh at most every 60s

  // Market stats (current candle)
  const [marketStats, setMarketStats] = useState<{
    open: number
    price: number
    priceChange: number
    priceChangePercent: number
    high: number
    low: number
    volume: number // Quantity (BTC/shares)
    quoteVolume: number // Turnover (USDT/USD)
  } | null>(null)

  // Indicator configuration
  const [indicators, setIndicators] = useState<IndicatorConfig[]>([
    { id: 'volume', name: 'Volume', enabled: true, color: '#3B82F6' },
    {
      id: 'ma5',
      name: 'MA5',
      enabled: false,
      color: '#FF6B6B',
      params: { period: 5 },
    },
    {
      id: 'ma10',
      name: 'MA10',
      enabled: false,
      color: '#4ECDC4',
      params: { period: 10 },
    },
    {
      id: 'ma20',
      name: 'MA20',
      enabled: false,
      color: '#FFD93D',
      params: { period: 20 },
    },
    {
      id: 'ma60',
      name: 'MA60',
      enabled: false,
      color: '#95E1D3',
      params: { period: 60 },
    },
    {
      id: 'ema12',
      name: 'EMA12',
      enabled: false,
      color: '#A8E6CF',
      params: { period: 12 },
    },
    {
      id: 'ema26',
      name: 'EMA26',
      enabled: false,
      color: '#FFD3B6',
      params: { period: 26 },
    },
    { id: 'bb', name: 'Bollinger Bands', enabled: false, color: '#9B59B6' },
  ])

  // Fetch kline data from service
  const fetchKlineData = async (
    symbol: string,
    interval: string,
    limit = 1500
  ) => {
    try {
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
        volume: candle.volume, // Quantity (BTC/shares)
        quoteVolume: candle.quoteVolume, // Turnover (USDT/USD)
      }))

      // Sort by time and deduplicate (lightweight-charts requires ascending, unique times)
      const sortedData = rawData.sort((a: any, b: any) => a.time - b.time)
      const dedupedData = sortedData.filter(
        (item: any, index: number, arr: any[]) =>
          index === 0 || item.time !== arr[index - 1].time
      )

      if (rawData.length !== dedupedData.length) {
        console.warn(
          '[AdvancedChart] Removed',
          rawData.length - dedupedData.length,
          'duplicate klines'
        )
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
        return seconds
      }
      return time
    }

    const timeStr = String(time)

    // Try standard ISO format
    const isoTime = new Date(timeStr).getTime()
    if (!isNaN(isoTime) && isoTime > 0) {
      const timestamp = Math.floor(isoTime / 1000)
      return timestamp
    }

    // Parse custom format "MM-DD HH:mm UTC" (for legacy data)
    const match = timeStr.match(/(\d{2})-(\d{2})\s+(\d{2}):(\d{2})\s+UTC/)
    if (match) {
      const currentYear = new Date().getFullYear()
      const [_, month, day, hour, minute] = match
      const date = new Date(
        Date.UTC(
          currentYear,
          parseInt(month) - 1,
          parseInt(day),
          parseInt(hour),
          parseInt(minute)
        )
      )
      const timestamp = Math.floor(date.getTime() / 1000)
      return timestamp
    }

    console.error('[AdvancedChart] ❌ Failed to parse time:', timeStr)
    return 0
  }

  // Fetch order data
  const fetchOrders = async (
    traderID: string,
    symbol: string
  ): Promise<OrderMarker[]> => {
    try {
      // Fetch filled orders, up to 200 for more history
      const result = await httpClient.request(
        `/api/orders?trader_id=${traderID}&symbol=${symbol}&status=FILLED&limit=200`,
        { silent: true }
      )

      if (!result.success || !result.data) {
        console.warn('[AdvancedChart] No orders found, result:', result)
        return []
      }

      const orders = result.data
      const markers: OrderMarker[] = []

      orders.forEach((order: any) => {
        // Handle field names: support PascalCase and snake_case
        const filledAt =
          order.filled_at ||
          order.FilledAt ||
          order.created_at ||
          order.CreatedAt
        const avgPrice =
          order.avg_fill_price ||
          order.AvgFillPrice ||
          order.price ||
          order.Price
        const orderAction = order.order_action || order.OrderAction
        const side = (order.side || order.Side)?.toLowerCase() // BUY/SELL
        const symbol = order.symbol || order.Symbol

        // Skip orders without fill time or price
        if (!filledAt || !avgPrice || avgPrice === 0) {
          console.warn('[AdvancedChart] Skipping order - missing data:', {
            filledAt,
            avgPrice,
          })
          return
        }

        const timeSeconds = parseCustomTime(filledAt)
        if (timeSeconds === 0) {
          console.warn(
            '[AdvancedChart] Skipping order - invalid time:',
            filledAt
          )
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

        markers.push({
          time: timeSeconds,
          price: avgPrice,
          side: positionSide,
          rawSide: side, // Original side field (buy/sell)
          action: action,
          symbol,
        })
      })

      return markers
    } catch (err) {
      console.error('[AdvancedChart] Error fetching orders:', err)
      return []
    }
  }

  // Fetch exchange open orders (TP/SL)
  const fetchOpenOrders = async (
    traderID: string,
    symbol: string
  ): Promise<OpenOrder[]> => {
    try {
      const result = await httpClient.request(
        `/api/open-orders?trader_id=${traderID}&symbol=${symbol}`,
        { silent: true }
      )

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

    const th = getCandleChartTheme()
    const chart = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth || 800,
      height: chartContainerRef.current.clientHeight || height,
      layout: {
        background: { color: th.background },
        textColor: th.text,
        fontSize: 12,
      },
      grid: {
        vertLines: {
          color: th.grid,
          style: 1,
          visible: true,
        },
        horzLines: {
          color: th.grid,
          style: 1,
          visible: true,
        },
      },
      crosshair: {
        mode: 1,
        vertLine: {
          color: th.crosshair,
          width: 1,
          style: 2,
          labelBackgroundColor: th.crosshairLabel,
        },
        horzLine: {
          color: th.crosshair,
          width: 1,
          style: 2,
          labelBackgroundColor: th.crosshairLabel,
        },
      },
      rightPriceScale: {
        borderColor: th.border,
        scaleMargins: {
          top: 0.1,
          bottom: 0.08,
        },
        borderVisible: true,
        entireTextOnly: false,
      },
      timeScale: {
        borderColor: th.border,
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

    // Create candlestick series — solid up / down bodies and wicks
    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: th.up,
      downColor: th.down,
      borderUpColor: th.up,
      borderDownColor: th.down,
      wickUpColor: th.up,
      wickDownColor: th.down,
    })
    candlestickSeriesRef.current = candlestickSeries as any

    // Create volume series — overlay pane with scaleMargins top:0.8 keeps the
    // histogram in the BOTTOM 20% of the chart (TradingView standard). The old
    // config let volume occupy the full height and overlap the candles
    // (2026-09-27 user report).
    const volumeSeries = chart.addSeries(HistogramSeries, {
      color: th.up,
      priceFormat: {
        type: 'volume',
      },
      priceScaleId: 'vol-overlay',
      lastValueVisible: false,
      priceLineVisible: false,
    })
    volumeSeriesRef.current = volumeSeries as any
    volumeSeries.priceScale().applyOptions({
      scaleMargins: { top: 0.8, bottom: 0 },
    })

    // Responsive resize (ResizeObserver)
    const resizeObserver = new ResizeObserver((entries) => {
      if (entries.length === 0 || !entries[0].contentRect) return
      const { width, height } = entries[0].contentRect
      chart.applyOptions({ width, height })
    })

    if (chartContainerRef.current) {
      resizeObserver.observe(chartContainerRef.current)
    }

    // Listen for crosshair movement to update the TradingView-style legend
    chart.subscribeCrosshairMove((param) => {
      if (!param.time || !param.point || !candlestickSeriesRef.current) {
        setLegendData(null) // Fall back to the latest candle
        return
      }

      const data = param.seriesData.get(candlestickSeriesRef.current as any)
      if (!data) {
        setLegendData(null)
        return
      }

      const candleData = data as any
      const klineExtra = klineDataRef.current.get(param.time as number) || {
        volume: 0,
        quoteVolume: 0,
      }

      // Change vs previous candle close (TradingView legend behaviour)
      const list = klineListRef.current
      const idx = list.findIndex((k) => k.time === (param.time as number))
      const prevClose = idx > 0 ? list[idx - 1].close : candleData.open
      const change = candleData.close - prevClose
      const changePercent = prevClose > 0 ? (change / prevClose) * 100 : 0

      // Amplitude: (high-low) vs previous close, exchange-app convention
      const ampBase = prevClose > 0 ? prevClose : candleData.open
      const amplitude =
        ampBase > 0 ? ((candleData.high - candleData.low) / ampBase) * 100 : 0

      setLegendData({
        open: candleData.open,
        high: candleData.high,
        low: candleData.low,
        close: candleData.close,
        volume: klineExtra.volume,
        quoteVolume: klineExtra.quoteVolume,
        change,
        changePercent,
        amplitude,
      })
    })

    return () => {
      resizeObserver.disconnect()
      chart.remove()
    }
  }, []) // Chart is created once, ResizeObserver handles dimension changes

  // Re-apply colours when the app theme (dark/light) changes. Only options and
  // colours are touched here: klines, indicators and order markers are
  // recoloured from memory, so a theme toggle does not refetch anything.
  useEffect(() => {
    const chart = chartRef.current
    if (!chart) return
    const th = getCandleChartTheme()
    chart.applyOptions({
      layout: { background: { color: th.background }, textColor: th.text },
      grid: { vertLines: { color: th.grid }, horzLines: { color: th.grid } },
      crosshair: {
        vertLine: {
          color: th.crosshair,
          labelBackgroundColor: th.crosshairLabel,
        },
        horzLine: {
          color: th.crosshair,
          labelBackgroundColor: th.crosshairLabel,
        },
      },
      rightPriceScale: { borderColor: th.border },
      timeScale: { borderColor: th.border },
    })
    candlestickSeriesRef.current?.applyOptions({
      upColor: th.up,
      downColor: th.down,
      borderUpColor: th.up,
      borderDownColor: th.down,
      wickUpColor: th.up,
      wickDownColor: th.down,
    })

    // Volume bars carry per-bar colours: rebuild them from the loaded klines
    const volumeEnabled = indicators.find((i) => i.id === 'volume')?.enabled
    if (
      volumeSeriesRef.current &&
      volumeEnabled &&
      klineListRef.current.length > 0
    ) {
      volumeSeriesRef.current.setData(buildVolumeData(klineListRef.current, th))
    }

    // Open-order price lines keep their role (SL / TP / limit)
    priceLinesRef.current.forEach(({ line, tone }) => {
      line.applyOptions({ color: th[tone] })
    })

    // Order markers: buy sits below its bar (up), sell above its bar (down)
    if (currentMarkersDataRef.current.length > 0) {
      currentMarkersDataRef.current = currentMarkersDataRef.current.map(
        (m) => ({
          ...m,
          color: m.position === 'belowBar' ? th.up : th.down,
        })
      )
      seriesMarkersRef.current?.setMarkers(
        showOrderMarkers ? currentMarkersDataRef.current : []
      )
    }
  }, [themeVersion])

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

      // Only show loading on first load, avoid flicker on refresh
      if (!isRefresh) {
        setLoading(true)
      }
      setError(null)

      try {
        // 1. Fetch kline data. Refreshes pull only the newest bars and merge
        // them into the loaded history (2026-10-07 review: re-fetching 1500
        // bars every 5s burned exchange weight for 1-2 changed candles).
        const prev = klineListRef.current
        const klineData =
          isRefresh && prev && prev.length > 0
            ? mergeKlines(prev, await fetchKlineData(symbol, interval, 3))
            : await fetchKlineData(symbol, interval)
        if (klineData.length > 0) {
          const lastClose = klineData[klineData.length - 1].close
          const prec = priceAxisPrecision(lastClose)
          candlestickSeriesRef.current.applyOptions({
            priceFormat: {
              type: 'price',
              precision: prec,
              minMove: Number('0.' + '0'.repeat(Math.max(0, prec - 1)) + '1'),
            },
          })
        }
        candlestickSeriesRef.current.setData(klineData)

        // Store volume/quoteVolume data for tooltip
        klineDataRef.current.clear()
        klineData.forEach((k: any) => {
          klineDataRef.current.set(k.time, {
            volume: k.volume || 0,
            quoteVolume: k.quoteVolume || 0,
          })
        })
        klineListRef.current = klineData

        // 1.5 Calculate market stats
        if (klineData.length > 1) {
          const latestKline = klineData[klineData.length - 1]
          const prevKline = klineData[klineData.length - 2]

          // Price change: current candle close vs previous candle close
          const priceChange = latestKline.close - prevKline.close
          const priceChangePercent = (priceChange / prevKline.close) * 100

          setMarketStats({
            open: latestKline.open,
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
            open: latestKline.open,
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
          const volumeEnabled = indicators.find(
            (i) => i.id === 'volume'
          )?.enabled
          if (volumeEnabled) {
            volumeSeriesRef.current.setData(
              buildVolumeData(klineData, getCandleChartTheme())
            )
          } else {
            // Clear data when volume is disabled
            volumeSeriesRef.current.setData([])
          }
        }

        // 3. Add indicators
        updateIndicators(klineData)

        // 4. Fetch and display order markers
        const ordersDue =
          !isRefresh || Date.now() - lastOrdersFetchRef.current >= 60_000
        if (traderID && candlestickSeriesRef.current && ordersDue) {
          lastOrdersFetchRef.current = Date.now()
          const orders = await fetchOrders(traderID, symbol)

          if (orders.length > 0) {
            // Extract sorted kline time array
            const klineTimes = klineData.map((k: any) => k.time as number)
            const klineMinTime = klineTimes[0] || 0
            const klineMaxTime = klineTimes[klineTimes.length - 1] || 0

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
            const ordersByCandle = new Map<
              number,
              { buys: number; sells: number }
            >()

            orders.forEach((order) => {
              // Use binary search to find matching kline candle time
              const candleTime = findCandleTime(order.time)

              if (candleTime === null) {
                console.warn(
                  '[AdvancedChart] ⚠️ Skipping order outside kline range:',
                  order.time,
                  '(',
                  new Date(order.time * 1000).toISOString(),
                  ')'
                )
                return
              }

              const existing = ordersByCandle.get(candleTime) || {
                buys: 0,
                sells: 0,
              }
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
                  color: getCandleChartTheme().up,
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
                  color: getCandleChartTheme().down,
                  shape: 'circle' as const,
                  text: counts.sells > 1 ? `S${counts.sells}` : 'S',
                  size: 1,
                })
              }
            })

            // Sort by time (lightweight-charts requires chronological order)
            markers.sort((a, b) => (a.time as number) - (b.time as number))

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
                seriesMarkersRef.current = createSeriesMarkers(
                  candlestickSeriesRef.current,
                  markersToShow
                )
              }
            } catch (err) {
              console.error('[AdvancedChart] ❌ Failed to set markers:', err)
            }
          } else {
            try {
              if (seriesMarkersRef.current) {
                seriesMarkersRef.current.setMarkers([])
              }
            } catch (err) {
              console.error('[AdvancedChart] Failed to clear markers:', err)
            }
          }
        }

        // Initial load: show only the most recent ~VISIBLE_BARS candles instead of
        // fitContent — 1500 bars squeezed into one screen made candles ~2px wide
        // and unreadable (user report 2026-09-27). Scroll/zoom still reaches the
        // full fetched history natively.
        if (isInitialLoadRef.current) {
          const VISIBLE_BARS = 120
          const bars = klineListRef.current
          const ts = chartRef.current?.timeScale()
          if (ts && bars && bars.length > 2) {
            const from = bars[Math.max(0, bars.length - VISIBLE_BARS)]
            const to = bars[bars.length - 1]
            ts.setVisibleRange({ from: from.time as any, to: to.time as any })
          } else if (ts) {
            ts.applyOptions({ barSpacing: 10 })
          }
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
    const refreshInterval = setInterval(() => {
      if (typeof document !== 'undefined' && document.hidden) return
      loadData(true)
    }, 5000)
    return () => clearInterval(refreshInterval)
  }, [symbol, interval, traderID, exchange])

  // Refresh open order price lines separately (every 60s, avoid frequent exchange API calls)
  useEffect(() => {
    if (!traderID || !candlestickSeriesRef.current) return

    // Load open orders and display price lines
    const loadOpenOrders = async () => {
      try {
        // Clear old price lines first
        priceLinesRef.current.forEach(({ line }) => {
          try {
            candlestickSeriesRef.current?.removePriceLine(line)
          } catch (e) {
            // Ignore clear error
          }
        })
        priceLinesRef.current = []

        const openOrders = await fetchOpenOrders(traderID, symbol)

        if (openOrders.length > 0 && candlestickSeriesRef.current) {
          openOrders.forEach((order) => {
            // Get trigger price (SL/TP use stop_price, limit orders use price)
            const linePrice =
              order.stop_price > 0 ? order.stop_price : order.price
            if (linePrice <= 0) return

            // Determine order type
            const isStopLoss =
              order.type.includes('STOP') || order.type.includes('SL')
            const isTakeProfit =
              order.type.includes('TAKE_PROFIT') || order.type.includes('TP')
            const isLimit = order.type === 'LIMIT'

            // Set price line style
            let tone: PriceLineTone = 'brand' // Default yellow
            const lineStyle = 2 // dashed
            let title = ''

            if (isStopLoss) {
              tone = 'down' // red - stop loss
              title = `SL ${order.quantity}`
            } else if (isTakeProfit) {
              tone = 'up' // green - take profit
              title = `TP ${order.quantity}`
            } else if (isLimit) {
              tone = 'brand' // yellow - limit order
              title = `Limit ${order.side} ${order.quantity}`
            } else {
              title = `${order.type} ${order.quantity}`
            }

            const priceLine = candlestickSeriesRef.current?.createPriceLine({
              price: linePrice,
              color: getCandleChartTheme()[tone],
              lineWidth: 1,
              lineStyle: lineStyle,
              axisLabelVisible: true,
              title: title,
            })

            if (priceLine) {
              priceLinesRef.current.push({ line: priceLine, tone })
            }
          })
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
      const markersToShow = showOrderMarkers
        ? currentMarkersDataRef.current
        : []
      seriesMarkersRef.current.setMarkers(markersToShow)
    } catch (err) {
      console.error('[AdvancedChart] ❌ Failed to toggle markers:', err)
    }
  }, [showOrderMarkers])

  // Update indicators
  const updateIndicators = (klineData: Kline[]) => {
    if (!chartRef.current) return

    // Clear old indicators
    indicatorSeriesRef.current.forEach((series) => {
      chartRef.current?.removeSeries(series as any)
    })
    indicatorSeriesRef.current.clear()

    // Add enabled indicators
    indicators.forEach((indicator) => {
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
        upperSeries.setData(
          bbData.map((d) => ({ time: d.time as any, value: d.upper }))
        )

        const middleSeries = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 1,
          lineStyle: 2,
          title: 'BB Middle',
        })
        middleSeries.setData(
          bbData.map((d) => ({ time: d.time as any, value: d.middle }))
        )

        const lowerSeries = chartRef.current.addSeries(LineSeries, {
          color: indicator.color,
          lineWidth: 1,
          title: 'BB Lower',
        })
        lowerSeries.setData(
          bbData.map((d) => ({ time: d.time as any, value: d.lower }))
        )

        indicatorSeriesRef.current.set(indicator.id + '_upper', upperSeries)
        indicatorSeriesRef.current.set(indicator.id + '_middle', middleSeries)
        indicatorSeriesRef.current.set(indicator.id + '_lower', lowerSeries)
      }
    })
  }

  // Toggle indicator
  const toggleIndicator = (id: string) => {
    setIndicators((prev) =>
      prev.map((ind) =>
        ind.id === id ? { ...ind, enabled: !ind.enabled } : ind
      )
    )
  }

  // Legend data: hovered candle while crosshair is active, else the latest one
  const legend =
    legendData ??
    (marketStats
      ? (() => {
          // prevClose derived the same way priceChange was computed
          const prevClose = marketStats.price - marketStats.priceChange
          const ampBase = prevClose > 0 ? prevClose : marketStats.open
          const amplitude =
            ampBase > 0
              ? ((marketStats.high - marketStats.low) / ampBase) * 100
              : 0
          return {
            open: marketStats.open,
            high: marketStats.high,
            low: marketStats.low,
            close: marketStats.price,
            volume: marketStats.volume,
            quoteVolume: marketStats.quoteVolume,
            change: marketStats.priceChange,
            changePercent: marketStats.priceChangePercent,
            amplitude,
          }
        })()
      : null)

  return (
    <div className="relative flex h-full w-full flex-col overflow-hidden bg-surface">
      {/* Compact header: symbol + price on the left, chart toggles on the right */}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-x-2 gap-y-1 border-b border-line bg-surface-2 px-3 py-1.5">
        {/* Left: Symbol Info + Price */}
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
          {/* Symbol & Interval */}
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-sm font-semibold text-fg">{symbol}</span>
            <Badge variant="neutral" size="xs">
              {interval}
            </Badge>
            <Badge variant="neutral" size="xs" className="uppercase">
              {exchange?.toUpperCase()}
            </Badge>
          </div>

          {/* Price Display */}
          {marketStats && (
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 sm:border-l sm:border-line sm:pl-3">
              <span
                className={cn(
                  'num text-base font-semibold',
                  marketStats.priceChange >= 0 ? 'text-up' : 'text-down'
                )}
              >
                {marketStats.price.toLocaleString(undefined, {
                  minimumFractionDigits: 2,
                  maximumFractionDigits:
                    exchange === 'forex' || exchange === 'metals' ? 4 : 2,
                })}
              </span>
              <Badge
                variant={marketStats.priceChange >= 0 ? 'up' : 'down'}
                className="num"
              >
                {formatSigned(marketStats.priceChangePercent, { suffix: '%' })}
              </Badge>

              {/* Compact H/L */}
              <div className="flex items-center gap-2 text-[11px] text-fg-3">
                <span>
                  H{' '}
                  <span className="num text-fg-2">
                    {marketStats.high.toFixed(2)}
                  </span>
                </span>
                <span>
                  L{' '}
                  <span className="num text-fg-2">
                    {marketStats.low.toFixed(2)}
                  </span>
                </span>
                {marketStats.volume > 0 && baseUnit && (
                  <span>
                    Vol{' '}
                    <span className="num text-fg-2">
                      {formatVolume(marketStats.volume)}
                    </span>
                  </span>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Right: Controls */}
        <div className="flex shrink-0 items-center gap-1">
          {loading && (
            <span className="mr-1 animate-pulse text-[11px] text-fg-3">
              {t('advancedChart.updating', language)}
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowIndicatorPanel(!showIndicatorPanel)}
            className={cn(
              'h-6 gap-1 px-2 text-[11px]',
              showIndicatorPanel && 'bg-info-soft text-info'
            )}
          >
            <Settings className="h-3 w-3" />
            <span>{t('advancedChart.indicators', language)}</span>
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowOrderMarkers(!showOrderMarkers)}
            className={cn(
              'h-6 px-2 text-[11px]',
              showOrderMarkers && 'bg-up-soft text-up'
            )}
            title={t('advancedChart.orderMarkers', language)}
          >
            <span>B/S</span>
          </Button>
        </div>
      </div>

      {/* Indicator panel */}
      {showIndicatorPanel && (
        <div className="absolute right-3 top-12 z-20 max-h-[500px] min-w-[260px] overflow-y-auto rounded-lg border border-line bg-surface shadow-pop">
          {/* Title bar */}
          <div className="flex items-center justify-between border-b border-line px-3 py-2.5">
            <div className="flex items-center gap-2">
              <BarChart2 className="h-4 w-4 text-brand" />
              <h4 className="text-sm font-semibold text-fg">
                {t('advancedChart.technicalIndicators', language)}
              </h4>
            </div>
            <button
              onClick={() => setShowIndicatorPanel(false)}
              className="text-fg-3 transition-colors hover:text-fg"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          {/* Indicator list */}
          <div className="space-y-0.5 p-2">
            {indicators.map((indicator) => (
              <label
                key={indicator.id}
                className="group flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 transition-colors hover:bg-surface-hover"
              >
                <input
                  type="checkbox"
                  checked={indicator.enabled}
                  onChange={() => toggleIndicator(indicator.id)}
                  className="h-4 w-4 cursor-pointer rounded border-line-strong accent-brand"
                />
                <span
                  className="h-3 w-8 rounded-sm border border-line"
                  style={{ backgroundColor: indicator.color }}
                />
                <span className="flex-1 text-sm text-fg-2 transition-colors group-hover:text-fg">
                  {indicator.name}
                </span>
                {indicator.enabled && (
                  <span className="text-xs text-brand">●</span>
                )}
              </label>
            ))}
          </div>

          {/* Bottom hint */}
          <div className="border-t border-line px-3 py-2 text-xs text-fg-3">
            {t('advancedChart.clickToToggle', language)}
          </div>
        </div>
      )}

      {/* Chart container */}
      <div className="relative min-h-0 flex-1">
        <div ref={chartContainerRef} className="h-full w-full" />

        {/* TradingView-style OHLC legend (always visible, follows crosshair) */}
        {legend && (
          <div className="pointer-events-none absolute left-3 top-2 z-10 select-none text-xs leading-[18px]">
            <div className="font-medium text-fg">
              {symbol.replace(/USDT$/, '')} · {interval} ·{' '}
              {exchange?.toUpperCase()}
            </div>
            <div className="flex flex-wrap gap-x-2">
              {(() => {
                const bull = legend.close >= legend.open
                const c = bull ? 'text-up' : 'text-down'
                const oLabel = language === 'zh' ? '开' : 'O'
                const hLabel = language === 'zh' ? '高' : 'H'
                const lLabel = language === 'zh' ? '低' : 'L'
                const cLabel = language === 'zh' ? '收' : 'C'
                const ampLabel = language === 'zh' ? '振幅' : 'AMP'
                return (
                  <>
                    <span className={cn('num', c)}>
                      {oLabel}={formatLegendPrice(legend.open)}
                    </span>
                    <span className={cn('num', c)}>
                      {hLabel}={formatLegendPrice(legend.high)}
                    </span>
                    <span className={cn('num', c)}>
                      {lLabel}={formatLegendPrice(legend.low)}
                    </span>
                    <span className={cn('num', c)}>
                      {cLabel}={formatLegendPrice(legend.close)}
                    </span>
                    <span className={cn('num', c)}>
                      {legend.change >= 0 ? '+' : ''}
                      {formatLegendPrice(legend.change)} (
                      {legend.changePercent >= 0 ? '+' : ''}
                      {legend.changePercent.toFixed(2)}%)
                    </span>
                    <span className="num text-fg-3">
                      {ampLabel} {legend.amplitude.toFixed(2)}%
                    </span>
                    {legend.volume > 0 && baseUnit && (
                      <span className="num text-fg-3">
                        V({baseUnit}) {formatVolume(legend.volume)}
                      </span>
                    )}
                    {legend.quoteVolume > 0 && quoteUnit && (
                      <span className="num text-fg-3">
                        V({quoteUnit}) {formatVolume(legend.quoteVolume)}
                      </span>
                    )}
                  </>
                )
              })()}
            </div>
          </div>
        )}

        {/* NOFX watermark */}
        <div className="pointer-events-none absolute bottom-[20%] right-[5%] z-[1] select-none">
          <div className="text-[56px] font-bold tracking-[4px] text-brand/[0.12]">
            NOFX
          </div>
        </div>
      </div>

      {/* Error message */}
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-surface/90">
          <div className="text-center">
            <AlertTriangle className="mx-auto mb-2 h-6 w-6 text-down" />
            <div className="text-sm text-down">{error}</div>
          </div>
        </div>
      )}
    </div>
  )
}
