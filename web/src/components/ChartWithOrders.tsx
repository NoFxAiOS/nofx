import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  IChartApi,
  ISeriesApi,
  Time,
  UTCTimestamp,
  CandlestickSeries,
  createSeriesMarkers,
} from 'lightweight-charts'
import { useLanguage } from '../contexts/LanguageContext'
import { httpClient } from '../lib/httpClient'

// 订单接口定义
interface OrderMarker {
  time: number // Unix timestamp (seconds)
  price: number
  side: string // BUY, SELL
  orderAction: string // OPEN_LONG, CLOSE_LONG, STOP_LOSS, TAKE_PROFIT, etc.
  status: string // NEW, FILLED, CANCELED, etc.
  symbol: string
}

// K线数据接口
interface KlineData {
  time: UTCTimestamp
  open: number
  high: number
  low: number
  close: number
  volume?: number
}

interface ChartWithOrdersProps {
  symbol: string
  interval?: string // 1m, 5m, 15m, 1h, 4h, 1d
  traderID?: string // 用于获取该trader的订单
  height?: number
  exchange?: string // 交易所类型：binance, bybit, okx, bitget, hyperliquid, aster, lighter
}

export function ChartWithOrders({
  symbol = 'BTCUSDT',
  interval = '5m',
  traderID,
  height = 500,
  exchange = 'binance', // 默认使用 binance
}: ChartWithOrdersProps) {
  const { language } = useLanguage()
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candlestickSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const seriesMarkersRef = useRef<any>(null) // Markers primitive for v5
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tooltipData, setTooltipData] = useState<any>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)

  // 解析时间：支持 Unix 时间戳（数字）或字符串格式
  const parseCustomTime = (time: any): number => {
    if (!time) {
      console.warn('[ChartWithOrders] Empty time value')
      return 0
    }

    // 如果已经是数字（Unix 时间戳）
    if (typeof time === 'number') {
      // 判断是毫秒还是秒：如果大于 10^12 则认为是毫秒（2001年之后的毫秒时间戳）
      if (time > 1000000000000) {
        const seconds = Math.floor(time / 1000)
        console.log('[ChartWithOrders] ✅ Unix timestamp (ms→s):', time, '→', seconds, '(', new Date(time).toISOString(), ')')
        return seconds
      }
      console.log('[ChartWithOrders] ✅ Unix timestamp (s):', time, '(', new Date(time * 1000).toISOString(), ')')
      return time
    }

    const timeStr = String(time)
    console.log('[ChartWithOrders] Parsing time string:', timeStr)

    // 尝试标准ISO格式
    const isoTime = new Date(timeStr).getTime()
    if (!isNaN(isoTime) && isoTime > 0) {
      const timestamp = Math.floor(isoTime / 1000)
      console.log('[ChartWithOrders] ✅ Parsed as ISO:', timeStr, '→', timestamp, '(', new Date(timestamp * 1000).toISOString(), ')')
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
      console.log('[ChartWithOrders] ✅ Parsed as custom format:', timeStr, '→', timestamp, '(', new Date(timestamp * 1000).toISOString(), ')')
      return timestamp
    }

    console.error('[ChartWithOrders] ❌ Failed to parse time:', timeStr)
    return 0
  }

  // 从我们的服务获取K线数据
  const fetchKlineData = async (symbol: string, interval: string): Promise<KlineData[]> => {
    try {
      const limit = 2000 // 获取最近2000根K线 (更多历史数据)
      const klineUrl = `/api/klines?symbol=${symbol}&interval=${interval}&limit=${limit}&exchange=${exchange}`

      const result = await httpClient.get(klineUrl)

      if (!result.success || !result.data) {
        throw new Error('Failed to fetch kline data from our service')
      }

      const data = result.data

      // 转换后端数据格式到 lightweight-charts 格式
      // 后端返回的是 market.Kline 格式: {OpenTime, Open, High, Low, Close, Volume, ...}
      return data.map((candle: any) => ({
        time: Math.floor(candle.openTime / 1000) as UTCTimestamp, // 毫秒转秒
        open: candle.open,
        high: candle.high,
        low: candle.low,
        close: candle.close,
        volume: candle.volume,
      }))
    } catch (err) {
      console.error('Error fetching kline data:', err)
      throw err
    }
  }

  // 获取订单数据
  const fetchOrders = async (traderID: string, symbol: string): Promise<OrderMarker[]> => {
    try {
      // 从后端 API 获取该 trader 的订单记录（只获取已成交的订单）
      const result = await httpClient.get(`/api/orders?trader_id=${traderID}&symbol=${symbol}&status=FILLED&limit=50`)

      if (!result.success || !result.data) {
        console.warn('Failed to fetch orders:', result.message)
        return []
      }

      const orders = result.data
      const markers: OrderMarker[] = []

      // 转换订单数据为标记格式
      orders.forEach((order: any) => {
        const createdAt = order.created_at || order.CreatedAt
        const filledAt = order.filled_at || order.FilledAt
        const avgPrice = order.avg_fill_price || order.AvgFillPrice
        const price = order.price || order.Price
        const orderAction = order.order_action || order.OrderAction
        const side = order.side || order.Side
        const status = order.status || order.Status
        const symbol = order.symbol || order.Symbol

        // 使用成交时间（如果有）或创建时间
        const orderTime = filledAt || createdAt
        if (!orderTime) return

        const timeSeconds = parseCustomTime(orderTime)
        if (timeSeconds === 0) return

        // 使用平均成交价（如果有）或订单价格
        const orderPrice = avgPrice || price
        if (!orderPrice || orderPrice === 0) return

        markers.push({
          time: timeSeconds,
          price: orderPrice,
          side: side || 'BUY',
          orderAction: orderAction || 'UNKNOWN',
          status: status || 'FILLED',
          symbol: symbol || '',
        })
      })

      console.log(`[ChartWithOrders] Loaded ${markers.length} order markers for ${symbol}`)
      return markers
    } catch (err) {
      console.error('Error fetching orders:', err)
      return []
    }
  }

  // 初始化图表
  useEffect(() => {
    if (!chartContainerRef.current) {
      console.error('[ChartWithOrders] Container ref is null')
      return
    }

    console.log('[ChartWithOrders] Initializing chart for', symbol, interval)

    try {
      // 创建图表
      const chart = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth,
      height: height,
      layout: {
        background: { color: '#0B0E11' },
        textColor: '#EAECEF',
      },
      grid: {
        vertLines: { color: 'rgba(43, 49, 57, 0.5)' },
        horzLines: { color: 'rgba(43, 49, 57, 0.5)' },
      },
      crosshair: {
        mode: 1, // Normal crosshair
      },
      rightPriceScale: {
        borderColor: '#2B3139',
      },
      timeScale: {
        borderColor: '#2B3139',
        timeVisible: true,
        secondsVisible: false,
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

    // 创建K线系列 (使用 v5 API)
    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    })

    candlestickSeriesRef.current = candlestickSeries as any

    // 响应式调整
    const handleResize = () => {
      if (chartContainerRef.current && chartRef.current) {
        chartRef.current.applyOptions({
          width: chartContainerRef.current.clientWidth,
        })
      }
    }

      window.addEventListener('resize', handleResize)

      // 监听鼠标移动，显示 OHLC 信息
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
        setTooltipData({
          time: param.time,
          open: candleData.open,
          high: candleData.high,
          low: candleData.low,
          close: candleData.close,
          x: param.point.x,
          y: param.point.y,
        })
      })

      return () => {
        window.removeEventListener('resize', handleResize)
        chart.remove()
      }
    } catch (err) {
      console.error('[ChartWithOrders] Failed to initialize chart:', err)
      setError('Failed to initialize chart')
    }
  }, [height])

  // 加载数据
  useEffect(() => {
    const loadData = async () => {
      if (!candlestickSeriesRef.current) {
        console.log('[ChartWithOrders] Candlestick series not ready yet')
        return
      }

      console.log('[ChartWithOrders] Loading data for', symbol, interval, 'trader:', traderID)
      setLoading(true)
      setError(null)

      try {
        // 1. 获取K线数据
        console.log('[ChartWithOrders] Fetching kline data...')
        const klineData = await fetchKlineData(symbol, interval)
        console.log('[ChartWithOrders] Kline data received:', klineData.length, 'candles')
        candlestickSeriesRef.current.setData(klineData)

        // 构建 K 线时间集合，用于快速查找
        const klineTimeSet = new Set(klineData.map(k => k.time as number))
        const klineMinTime = klineData.length > 0 ? klineData[0].time : 0
        const klineMaxTime = klineData.length > 0 ? klineData[klineData.length - 1].time : 0
        console.log('[ChartWithOrders] Kline time range:', klineMinTime, '-', klineMaxTime, 'candles:', klineData.length)

        // 计算时间周期的秒数
        const getIntervalSeconds = (interval: string): number => {
          const match = interval.match(/(\d+)([smhd])/)
          if (!match) return 60 // 默认1分钟
          const [, num, unit] = match
          const n = parseInt(num)
          switch (unit) {
            case 's': return n
            case 'm': return n * 60
            case 'h': return n * 3600
            case 'd': return n * 86400
            default: return 60
          }
        }
        const intervalSeconds = getIntervalSeconds(interval)
        console.log('[ChartWithOrders] Interval:', interval, '=', intervalSeconds, 'seconds')

        // 2. 获取订单数据并添加标记
        if (traderID) {
          console.log('[ChartWithOrders] Fetching orders for trader:', traderID, 'symbol:', symbol)
          const orders = await fetchOrders(traderID, symbol)
          console.log('[ChartWithOrders] Received orders:', orders.length, 'orders')

          if (orders.length === 0) {
            console.log('[ChartWithOrders] No orders to display')
          }

          // 转换订单为图表标记，并对齐到 K 线时间
          const markers: Array<{
            time: Time
            position: 'belowBar'
            color: string
            shape: 'circle'
            text: string
            price: number
            size: number
          }> = []

          orders.forEach((order) => {
            // 将订单时间对齐到 K 线周期（向下取整）
            const alignedTime = Math.floor(order.time / intervalSeconds) * intervalSeconds

            // 检查对齐后的时间是否在 K 线数据中存在
            if (!klineTimeSet.has(alignedTime)) {
              console.warn('[ChartWithOrders] ⚠️ Skipping order - no matching kline:',
                order.time, '→', alignedTime, '(', new Date(order.time * 1000).toISOString(), ')')
              return
            }

            const isBuy = order.side === 'BUY'
            markers.push({
              time: alignedTime as Time,
              position: 'belowBar' as const,
              color: isBuy ? '#0ECB81' : '#F6465D',
              shape: 'circle' as const,
              text: isBuy ? 'B' : 'S',
              price: order.price,
              size: 1,
            })
          })

          console.log('[ChartWithOrders] Valid markers (with matching klines):', markers.length, 'out of', orders.length)

          console.log('[ChartWithOrders] Setting', markers.length, 'markers on chart')

          try {
            // 使用 v5 API: createSeriesMarkers
            if (seriesMarkersRef.current) {
              // 如果已经存在，更新标记
              seriesMarkersRef.current.setMarkers(markers)
            } else {
              // 首次创建标记
              seriesMarkersRef.current = createSeriesMarkers(candlestickSeriesRef.current, markers)
            }
            console.log('[ChartWithOrders] ✅ Markers set successfully!')
          } catch (err) {
            console.error('[ChartWithOrders] ❌ Failed to set markers:', err)
          }
        }

        // 自动适配视图
        chartRef.current?.timeScale().fitContent()

        setLoading(false)
      } catch (err) {
        console.error('Error loading chart data:', err)
        setError(language === 'zh' ? '加载图表数据失败' : 'Failed to load chart data')
        setLoading(false)
      }
    }

    loadData()

    // 自动刷新 - 每30秒更新一次K线数据
    const refreshInterval = setInterval(() => {
      loadData()
    }, 30000) // 30秒

    return () => {
      clearInterval(refreshInterval)
    }
  }, [symbol, interval, traderID, language])

  return (
    <div className="relative" style={{ background: '#0B0E11', borderRadius: '8px', overflow: 'hidden' }}>
      {/* 标题栏 */}
      <div className="flex items-center justify-between p-4" style={{ borderBottom: '1px solid #2B3139' }}>
        <div className="flex items-center gap-3">
          <span className="text-xl">📈</span>
          <h3 className="text-lg font-bold" style={{ color: '#EAECEF' }}>
            {symbol} {interval}
          </h3>
        </div>
        {loading && (
          <div className="text-sm" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '加载中...' : 'Loading...'}
          </div>
        )}
      </div>

      {/* 图表容器 */}
      <div style={{ position: 'relative' }}>
        <div ref={chartContainerRef} />

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
            </div>
          </div>
        )}
      </div>

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

      {/* 图例说明 */}
      <div className="flex items-center gap-4 p-4 text-xs" style={{ borderTop: '1px solid #2B3139', color: '#848E9C' }}>
        <div className="flex items-center gap-2">
          <span className="font-bold" style={{ color: '#0ECB81' }}>B</span>
          <span>{language === 'zh' ? 'BUY (买入)' : 'BUY'}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="font-bold" style={{ color: '#F6465D' }}>S</span>
          <span>{language === 'zh' ? 'SELL (卖出)' : 'SELL'}</span>
        </div>
      </div>
    </div>
  )
}
