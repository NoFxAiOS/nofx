import { useEffect, useMemo, useState, useCallback, useRef, type FormEvent } from 'react'
import useSWR from 'swr'
import { motion, AnimatePresence } from 'framer-motion'
import { createChart, ColorType, CrosshairMode, CandlestickSeries, createSeriesMarkers, type IChartApi, type ISeriesApi, type CandlestickData, type UTCTimestamp, type SeriesMarker } from 'lightweight-charts'
import {
  Play,
  Pause,
  Square,
  Download,
  Trash2,
  ChevronRight,
  ChevronLeft,
  Clock,
  TrendingUp,
  TrendingDown,
  Activity,
  BarChart3,
  Brain,
  Zap,
  Target,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Layers,
  Eye,
  ArrowUpRight,
  ArrowDownRight,
  CandlestickChart as CandlestickIcon,
} from 'lucide-react'
import { DeepVoidBackground } from './DeepVoidBackground'
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ReferenceDot,
} from 'recharts'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { confirmToast } from '../lib/notify'
import { DecisionCard } from './DecisionCard'
import { MetricTooltip } from './MetricTooltip'
import type {
  BacktestStatusPayload,
  BacktestPositionStatus,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestKlinesResponse,
  DecisionRecord,
  AIModel,
  Strategy,
} from '../types'

// ============ Types ============
type WizardStep = 1 | 2 | 3
type ViewTab = 'overview' | 'chart' | 'trades' | 'decisions' | 'compare'

const TIMEFRAME_OPTIONS = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d']
const POPULAR_SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT']

// ============ Helper Functions ============
const toLocalInput = (date: Date) => {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}


// ============ Sub Components ============

// Stats Card Component
function StatCard({
  icon: Icon,
  label,
  value,
  suffix,
  trend,
  color = '#EAECEF',
  metricKey,
  language = 'en',
}: {
  icon: typeof TrendingUp
  label: string
  value: string | number
  suffix?: string
  trend?: 'up' | 'down' | 'neutral'
  color?: string
  metricKey?: string
  language?: string
}) {
  const trendColors = {
    up: '#0ECB81',
    down: '#F6465D',
    neutral: '#848E9C',
  }

  return (
    <div
      className="p-4 rounded-xl"
      style={{ background: 'rgba(30, 35, 41, 0.6)', border: '1px solid #2B3139' }}
    >
      <div className="flex items-center gap-2 mb-2">
        <Icon className="w-4 h-4" style={{ color: '#F0B90B' }} />
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {label}
        </span>
        {metricKey && (
          <MetricTooltip metricKey={metricKey} language={language} size={12} />
        )}
      </div>
      <div className="flex items-baseline gap-1">
        <span className="text-xl font-bold" style={{ color }}>
          {value}
        </span>
        {suffix && (
          <span className="text-xs" style={{ color: '#848E9C' }}>
            {suffix}
          </span>
        )}
        {trend && trend !== 'neutral' && (
          <span style={{ color: trendColors[trend] }}>
            {trend === 'up' ? <ArrowUpRight className="w-4 h-4" /> : <ArrowDownRight className="w-4 h-4" />}
          </span>
        )}
      </div>
    </div>
  )
}

// Progress Ring Component
function ProgressRing({ progress, size = 120 }: { progress: number; size?: number }) {
  const strokeWidth = 8
  const radius = (size - strokeWidth) / 2
  const circumference = radius * 2 * Math.PI
  const offset = circumference - (progress / 100) * circumference

  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg className="transform -rotate-90" width={size} height={size}>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="#2B3139"
          strokeWidth={strokeWidth}
          fill="none"
        />
        <motion.circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="#F0B90B"
          strokeWidth={strokeWidth}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={circumference}
          initial={{ strokeDashoffset: circumference }}
          animate={{ strokeDashoffset: offset }}
          transition={{ duration: 0.5 }}
        />
      </svg>
      <div className="absolute inset-0 flex items-center justify-center flex-col">
        <span className="text-2xl font-bold" style={{ color: '#F0B90B' }}>
          {progress.toFixed(0)}%
        </span>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          Complete
        </span>
      </div>
    </div>
  )
}

// Equity Chart Component using Recharts
function BacktestChart({
  equity,
  trades,
}: {
  equity: BacktestEquityPoint[]
  trades: BacktestTradeEvent[]
}) {
  const chartData = useMemo(() => {
    return equity.map((point) => ({
      time: new Date(point.ts).toLocaleString(),
      ts: point.ts,
      equity: point.equity,
      pnl_pct: point.pnl_pct,
    }))
  }, [equity])

  // Find trade points to mark on chart
  const tradeMarkers = useMemo(() => {
    if (!trades.length || !equity.length) return []
    return trades
      .filter((t) => t.action.includes('open') || t.action.includes('close'))
      .map((trade) => {
        // Find closest equity point
        const closest = equity.reduce((prev, curr) =>
          Math.abs(curr.ts - trade.ts) < Math.abs(prev.ts - trade.ts) ? curr : prev
        )
        return {
          ts: closest.ts,
          equity: closest.equity,
          action: trade.action,
          symbol: trade.symbol,
          isOpen: trade.action.includes('open'),
        }
      })
      .slice(-30) // Limit markers
  }, [trades, equity])

  return (
    <div className="w-full h-[300px]">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="equityGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#F0B90B" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#F0B90B" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="rgba(43, 49, 57, 0.5)" strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tick={{ fill: '#848E9C', fontSize: 10 }}
            axisLine={{ stroke: '#2B3139' }}
            tickLine={{ stroke: '#2B3139' }}
            hide
          />
          <YAxis
            tick={{ fill: '#848E9C', fontSize: 10 }}
            axisLine={{ stroke: '#2B3139' }}
            tickLine={{ stroke: '#2B3139' }}
            width={60}
            domain={['auto', 'auto']}
          />
          <Tooltip
            contentStyle={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              borderRadius: 8,
              color: '#EAECEF',
            }}
            labelStyle={{ color: '#848E9C' }}
            formatter={(value: number) => [`$${value.toFixed(2)}`, 'Equity']}
          />
          <Area
            type="monotone"
            dataKey="equity"
            stroke="#F0B90B"
            strokeWidth={2}
            fill="url(#equityGradient)"
            dot={false}
            activeDot={{ r: 4, fill: '#F0B90B' }}
          />
          {/* Trade markers */}
          {tradeMarkers.map((marker, idx) => (
            <ReferenceDot
              key={`${marker.ts}-${idx}`}
              x={chartData.findIndex((d) => d.ts === marker.ts)}
              y={marker.equity}
              r={4}
              fill={marker.isOpen ? '#0ECB81' : '#F6465D'}
              stroke={marker.isOpen ? '#0ECB81' : '#F6465D'}
            />
          ))}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}

// Candlestick Chart Component with trade markers
function CandlestickChartComponent({
  runId,
  trades,
  language,
}: {
  runId: string
  trades: BacktestTradeEvent[]
  language: string
}) {
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)

  // Get unique symbols from trades
  const symbols = useMemo(() => {
    const symbolSet = new Set(trades.map((t) => t.symbol))
    return Array.from(symbolSet).sort()
  }, [trades])

  const [selectedSymbol, setSelectedSymbol] = useState<string>(symbols[0] || '')
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('15m')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const CHART_TIMEFRAMES = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d']

  // Update selected symbol when symbols change
  useEffect(() => {
    if (symbols.length > 0 && !symbols.includes(selectedSymbol)) {
      setSelectedSymbol(symbols[0])
    }
  }, [symbols, selectedSymbol])

  // Filter trades for selected symbol
  const symbolTrades = useMemo(() => {
    return trades.filter((t) => t.symbol === selectedSymbol)
  }, [trades, selectedSymbol])

  // Fetch klines and render chart
  useEffect(() => {
    if (!chartContainerRef.current || !selectedSymbol || !runId) return

    const container = chartContainerRef.current

    // Create chart
    const chart = createChart(container, {
      layout: {
        background: { type: ColorType.Solid, color: '#0B0E11' },
        textColor: '#848E9C',
      },
      grid: {
        vertLines: { color: 'rgba(43, 49, 57, 0.5)' },
        horzLines: { color: 'rgba(43, 49, 57, 0.5)' },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
      rightPriceScale: {
        borderColor: '#2B3139',
      },
      timeScale: {
        borderColor: '#2B3139',
        timeVisible: true,
        secondsVisible: false,
      },
      width: container.clientWidth,
      height: 400,
    })

    chartRef.current = chart

    // Add candlestick series
    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    })
    candleSeriesRef.current = candleSeries

    // Fetch klines
    setIsLoading(true)
    setError(null)

    api
      .getBacktestKlines(runId, selectedSymbol, selectedTimeframe)
      .then((data: BacktestKlinesResponse) => {
        const klineData: CandlestickData<UTCTimestamp>[] = data.klines.map((k) => ({
          time: k.time as UTCTimestamp,
          open: k.open,
          high: k.high,
          low: k.low,
          close: k.close,
        }))
        candleSeries.setData(klineData)

        // Add trade markers with improved styling
        const markers: SeriesMarker<UTCTimestamp>[] = symbolTrades
          .map((trade) => {
            const tradeTime = Math.floor(trade.ts / 1000)
            // Find closest kline time
            const closestKline = data.klines.reduce((prev, curr) =>
              Math.abs(curr.time - tradeTime) < Math.abs(prev.time - tradeTime) ? curr : prev
            )
            const isOpen = trade.action.includes('open')
            const isLong = trade.side === 'long' || trade.action.includes('long')
            const pnl = trade.realized_pnl

            // Format display text
            let text = ''
            let color = '#0ECB81' // Default green

            if (isOpen) {
              // Opening position: show direction and price
              if (isLong) {
                text = `▲ Long @${trade.price.toFixed(2)}`
                color = '#0ECB81' // Green for long open
              } else {
                text = `▼ Short @${trade.price.toFixed(2)}`
                color = '#F6465D' // Red for short open
              }
            } else {
              // Closing position: show PnL
              const pnlStr = pnl >= 0 ? `+$${pnl.toFixed(2)}` : `-$${Math.abs(pnl).toFixed(2)}`
              text = `✕ ${pnlStr}`
              color = pnl >= 0 ? '#0ECB81' : '#F6465D' // Green for profit, red for loss
            }

            return {
              time: closestKline.time as UTCTimestamp,
              position: isOpen
                ? (isLong ? 'belowBar' as const : 'aboveBar' as const) // Long below, short above
                : (isLong ? 'aboveBar' as const : 'belowBar' as const), // Close opposite
              color,
              shape: 'circle' as const,
              size: 2,
              text,
            }
          })
          .sort((a, b) => (a.time as number) - (b.time as number))

        createSeriesMarkers(candleSeries, markers)
        chart.timeScale().fitContent()
        setIsLoading(false)
      })
      .catch((err) => {
        setError(err.message || 'Failed to load klines')
        setIsLoading(false)
      })

    // Handle resize
    const handleResize = () => {
      if (chartContainerRef.current) {
        chart.applyOptions({ width: chartContainerRef.current.clientWidth })
      }
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
      chartRef.current = null
      candleSeriesRef.current = null
    }
  }, [runId, selectedSymbol, selectedTimeframe, symbolTrades])

  if (symbols.length === 0) {
    return (
      <div className="py-12 text-center" style={{ color: '#5E6673' }}>
        {language === 'zh' ? '没有交易记录' : 'No trades to display'}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Symbol and Timeframe selectors */}
      <div className="flex items-center gap-4 flex-wrap">
        <div className="flex items-center gap-2">
          <CandlestickIcon size={16} style={{ color: '#F0B90B' }} />
          <span className="text-sm" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '币种' : 'Symbol'}
          </span>
          <select
            value={selectedSymbol}
            onChange={(e) => setSelectedSymbol(e.target.value)}
            className="px-3 py-1.5 rounded text-sm"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            {symbols.map((sym) => (
              <option key={sym} value={sym}>
                {sym}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-2">
          <Clock size={14} style={{ color: '#848E9C' }} />
          <span className="text-sm" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '周期' : 'Interval'}
          </span>
          <div className="flex rounded overflow-hidden" style={{ border: '1px solid #2B3139' }}>
            {CHART_TIMEFRAMES.map((tf) => (
              <button
                key={tf}
                onClick={() => setSelectedTimeframe(tf)}
                className="px-2.5 py-1 text-xs font-medium transition-colors"
                style={{
                  background: selectedTimeframe === tf ? '#F0B90B' : '#1E2329',
                  color: selectedTimeframe === tf ? '#0B0E11' : '#848E9C',
                }}
              >
                {tf}
              </button>
            ))}
          </div>
        </div>

        <span className="text-xs" style={{ color: '#5E6673' }}>
          ({symbolTrades.length} {language === 'zh' ? '笔交易' : 'trades'})
        </span>
      </div>

      {/* Chart container */}
      <div
        ref={chartContainerRef}
        className="w-full rounded-lg overflow-hidden"
        style={{ background: '#0B0E11', minHeight: 400 }}
      >
        {isLoading && (
          <div className="flex items-center justify-center h-[400px]" style={{ color: '#848E9C' }}>
            <RefreshCw className="animate-spin mr-2" size={16} />
            {language === 'zh' ? '加载K线数据...' : 'Loading kline data...'}
          </div>
        )}
        {error && (
          <div className="flex items-center justify-center h-[400px]" style={{ color: '#F6465D' }}>
            <AlertTriangle className="mr-2" size={16} />
            {error}
          </div>
        )}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 text-xs" style={{ color: '#848E9C' }}>
        <div className="flex items-center gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#0ECB81' }} />
          <span>{language === 'zh' ? '开仓/盈利' : 'Open/Profit'}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#F6465D' }} />
          <span>{language === 'zh' ? '亏损平仓' : 'Loss Close'}</span>
        </div>
        <span style={{ color: '#5E6673' }}>|</span>
        <span>▲ Long · ▼ Short · ✕ {language === 'zh' ? '平仓' : 'Close'}</span>
      </div>
    </div>
  )
}

// Trade Timeline Component
function TradeTimeline({ trades }: { trades: BacktestTradeEvent[] }) {
  const recentTrades = useMemo(() => [...trades].slice(-20).reverse(), [trades])

  if (recentTrades.length === 0) {
    return (
      <div className="py-12 text-center" style={{ color: '#5E6673' }}>
        No trades yet
      </div>
    )
  }

  return (
    <div className="space-y-2 max-h-[400px] overflow-y-auto pr-2">
      {recentTrades.map((trade, idx) => {
        const isOpen = trade.action.includes('open')
        const isLong = trade.action.includes('long')
        const bgColor = isOpen ? 'rgba(14, 203, 129, 0.1)' : 'rgba(246, 70, 93, 0.1)'
        const borderColor = isOpen ? 'rgba(14, 203, 129, 0.3)' : 'rgba(246, 70, 93, 0.3)'
        const iconColor = isOpen ? '#0ECB81' : '#F6465D'

        return (
          <motion.div
            key={`${trade.ts}-${trade.symbol}-${idx}`}
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: idx * 0.05 }}
            className="p-3 rounded-lg flex items-center gap-3"
            style={{ background: bgColor, border: `1px solid ${borderColor}` }}
          >
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center"
              style={{ background: `${iconColor}20` }}
            >
              {isLong ? (
                <TrendingUp className="w-4 h-4" style={{ color: iconColor }} />
              ) : (
                <TrendingDown className="w-4 h-4" style={{ color: iconColor }} />
              )}
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-mono font-bold text-sm" style={{ color: '#EAECEF' }}>
                  {trade.symbol.replace('USDT', '')}
                </span>
                <span
                  className="px-2 py-0.5 rounded text-xs font-medium"
                  style={{ background: `${iconColor}20`, color: iconColor }}
                >
                  {trade.action.replace('_', ' ').toUpperCase()}
                </span>
                {trade.leverage && (
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    {trade.leverage}x
                  </span>
                )}
              </div>
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {new Date(trade.ts).toLocaleString()} · Qty: {trade.qty.toFixed(4)} · ${trade.price.toFixed(2)}
              </div>
            </div>
            <div className="text-right">
              <div
                className="font-mono font-bold"
                style={{ color: trade.realized_pnl >= 0 ? '#0ECB81' : '#F6465D' }}
              >
                {trade.realized_pnl >= 0 ? '+' : ''}
                {trade.realized_pnl.toFixed(2)}
              </div>
              <div className="text-xs" style={{ color: '#848E9C' }}>
                USDT
              </div>
            </div>
          </motion.div>
        )
      })}
    </div>
  )
}

// Real-time Positions Display Component
function PositionsDisplay({
  positions,
  language,
}: {
  positions: BacktestPositionStatus[]
  language: string
}) {
  if (!positions || positions.length === 0) {
    return null
  }

  const totalUnrealizedPnL = positions.reduce((sum, p) => sum + p.unrealized_pnl, 0)
  const totalMargin = positions.reduce((sum, p) => sum + p.margin_used, 0)

  return (
    <div
      className="mt-3 p-3 rounded-lg"
      style={{ background: 'rgba(30, 35, 41, 0.8)', border: '1px solid #2B3139' }}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            {language === 'zh' ? '当前持仓' : 'Active Positions'}
          </span>
          <span
            className="px-1.5 py-0.5 rounded text-xs"
            style={{ background: '#F0B90B20', color: '#F0B90B' }}
          >
            {positions.length}
          </span>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <span style={{ color: '#848E9C' }}>
            {language === 'zh' ? '保证金' : 'Margin'}: ${totalMargin.toFixed(2)}
          </span>
          <span
            className="font-medium"
            style={{ color: totalUnrealizedPnL >= 0 ? '#0ECB81' : '#F6465D' }}
          >
            {language === 'zh' ? '浮盈' : 'Unrealized'}: {totalUnrealizedPnL >= 0 ? '+' : ''}
            ${totalUnrealizedPnL.toFixed(2)}
          </span>
        </div>
      </div>

      <div className="space-y-1.5">
        {positions.map((pos) => {
          const isLong = pos.side === 'long'
          const pnlColor = pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D'

          return (
            <motion.div
              key={`${pos.symbol}-${pos.side}`}
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              className="flex items-center justify-between p-2 rounded"
              style={{ background: '#1E2329' }}
            >
              <div className="flex items-center gap-2">
                <div
                  className="w-6 h-6 rounded flex items-center justify-center"
                  style={{ background: isLong ? '#0ECB8120' : '#F6465D20' }}
                >
                  {isLong ? (
                    <TrendingUp className="w-3.5 h-3.5" style={{ color: '#0ECB81' }} />
                  ) : (
                    <TrendingDown className="w-3.5 h-3.5" style={{ color: '#F6465D' }} />
                  )}
                </div>
                <div>
                  <div className="flex items-center gap-1.5">
                    <span className="font-mono font-bold text-sm" style={{ color: '#EAECEF' }}>
                      {pos.symbol.replace('USDT', '')}
                    </span>
                    <span
                      className="px-1 py-0.5 rounded text-[10px] font-medium"
                      style={{
                        background: isLong ? '#0ECB8120' : '#F6465D20',
                        color: isLong ? '#0ECB81' : '#F6465D',
                      }}
                    >
                      {isLong ? 'LONG' : 'SHORT'} {pos.leverage}x
                    </span>
                  </div>
                  <div className="text-[10px]" style={{ color: '#5E6673' }}>
                    {language === 'zh' ? '数量' : 'Qty'}: {pos.quantity.toFixed(4)} ·{' '}
                    {language === 'zh' ? '保证金' : 'Margin'}: ${pos.margin_used.toFixed(2)}
                  </div>
                </div>
              </div>

              <div className="text-right">
                <div className="flex items-center gap-2 text-xs">
                  <span style={{ color: '#848E9C' }}>
                    {language === 'zh' ? '开仓' : 'Entry'}: ${pos.entry_price.toFixed(2)}
                  </span>
                  <span style={{ color: '#EAECEF' }}>
                    {language === 'zh' ? '现价' : 'Mark'}: ${pos.mark_price.toFixed(2)}
                  </span>
                </div>
                <div className="flex items-center justify-end gap-1.5 mt-0.5">
                  <span className="font-mono font-bold" style={{ color: pnlColor }}>
                    {pos.unrealized_pnl >= 0 ? '+' : ''}${pos.unrealized_pnl.toFixed(2)}
                  </span>
                  <span
                    className="px-1 py-0.5 rounded text-[10px] font-medium"
                    style={{ background: `${pnlColor}20`, color: pnlColor }}
                  >
                    {pos.unrealized_pnl_pct >= 0 ? '+' : ''}{pos.unrealized_pnl_pct.toFixed(2)}%
                  </span>
                </div>
              </div>
            </motion.div>
          )
        })}
      </div>
    </div>
  )
}

// ============ Main Component ============
export function BacktestPage() {
  const { language } = useLanguage()
  const tr = useCallback(
    (key: string, params?: Record<string, string | number>) => t(`backtestPage.${key}`, language, params),
    [language]
  )

  // State
  const now = new Date()
  const [wizardStep, setWizardStep] = useState<WizardStep>(1)
  const [viewTab, setViewTab] = useState<ViewTab>('overview')
  const [selectedRunId, setSelectedRunId] = useState<string>()
  const [compareRunIds, setCompareRunIds] = useState<string[]>([])
  const [isStarting, setIsStarting] = useState(false)
  const [toast, setToast] = useState<{ text: string; tone: 'info' | 'error' | 'success' } | null>(null)

  // Form state
  const [formState, setFormState] = useState({
    runId: '',
    symbols: 'BTCUSDT,ETHUSDT,SOLUSDT',
    timeframes: ['3m', '15m', '4h'],
    decisionTf: '3m',
    cadence: 20,
    start: toLocalInput(new Date(now.getTime() - 3 * 24 * 3600 * 1000)),
    end: toLocalInput(now),
    balance: 1000,
    fee: 5,
    slippage: 2,
    btcEthLeverage: 5,
    altcoinLeverage: 5,
    fill: 'next_open',
    prompt: 'baseline',
    promptTemplate: 'default',
    customPrompt: '',
    overridePrompt: false,
    cacheAI: true,
    replayOnly: false,
    aiModelId: '',
    strategyId: '', // Optional: use saved strategy from Strategy Studio
  })

  // Data fetching
  const { data: runsResp, mutate: refreshRuns } = useSWR(['backtest-runs'], () =>
    api.getBacktestRuns({ limit: 100, offset: 0 })
    , { refreshInterval: 5000 })
  const runs = runsResp?.items ?? []

  const { data: aiModels } = useSWR<AIModel[]>('ai-models', api.getModelConfigs, { refreshInterval: 30000 })
  const { data: strategies } = useSWR<Strategy[]>('strategies', api.getStrategies, { refreshInterval: 30000 })

  const { data: status } = useSWR<BacktestStatusPayload>(
    selectedRunId ? ['bt-status', selectedRunId] : null,
    () => api.getBacktestStatus(selectedRunId!),
    { refreshInterval: 2000 }
  )

  const { data: equity } = useSWR<BacktestEquityPoint[]>(
    selectedRunId ? ['bt-equity', selectedRunId] : null,
    () => api.getBacktestEquity(selectedRunId!, '1m', 2000),
    { refreshInterval: 5000 }
  )

  const { data: trades } = useSWR<BacktestTradeEvent[]>(
    selectedRunId ? ['bt-trades', selectedRunId] : null,
    () => api.getBacktestTrades(selectedRunId!, 500),
    { refreshInterval: 5000 }
  )

  const { data: metrics } = useSWR<BacktestMetrics>(
    selectedRunId ? ['bt-metrics', selectedRunId] : null,
    () => api.getBacktestMetrics(selectedRunId!),
    { refreshInterval: 10000 }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    selectedRunId ? ['bt-decisions', selectedRunId] : null,
    () => api.getBacktestDecisions(selectedRunId!, 30),
    { refreshInterval: 5000 }
  )

  const selectedRun = runs.find((r) => r.run_id === selectedRunId)
  const selectedModel = aiModels?.find((m) => m.id === formState.aiModelId)
  const selectedStrategy = strategies?.find((s) => s.id === formState.strategyId)

  // Check if selected strategy has dynamic coin source
  const strategyHasDynamicCoins = useMemo(() => {
    if (!selectedStrategy) return false
    const coinSource = selectedStrategy.config?.coin_source
    if (!coinSource) return false

    // Check explicit source_type
    if (coinSource.source_type === 'ai500' || coinSource.source_type === 'oi_top') {
      return true
    }
    if (coinSource.source_type === 'mixed' && (coinSource.use_ai500 || coinSource.use_oi_top)) {
      return true
    }

    // Also check flags for backward compatibility (when source_type is empty or not set)
    const srcType = coinSource.source_type as string
    if (!srcType) {
      if (coinSource.use_ai500 || coinSource.use_oi_top) {
        return true
      }
    }

    return false
  }, [selectedStrategy])

  // Get coin source description
  const coinSourceDescription = useMemo(() => {
    if (!selectedStrategy?.config?.coin_source) return null
    const cs = selectedStrategy.config.coin_source

    // Infer source_type from flags if empty (backward compatibility)
    let sourceType = cs.source_type as string
    if (!sourceType) {
      if (cs.use_ai500 && cs.use_oi_top) {
        sourceType = 'mixed'
      } else if (cs.use_ai500) {
        sourceType = 'ai500'
      } else if (cs.use_oi_top) {
        sourceType = 'oi_top'
      } else if (cs.static_coins?.length) {
        sourceType = 'static'
      }
    }

    switch (sourceType) {
      case 'ai500':
        return { type: 'AI500', limit: cs.ai500_limit || 30 }
      case 'oi_top':
        return { type: 'OI Top', limit: cs.oi_top_limit || 30 }
      case 'mixed':
        const sources = []
        if (cs.use_ai500) sources.push(`AI500(${cs.ai500_limit || 30})`)
        if (cs.use_oi_top) sources.push(`OI Top(${cs.oi_top_limit || 30})`)
        if (cs.static_coins?.length) sources.push(`Static(${cs.static_coins.length})`)
        return { type: 'Mixed', desc: sources.join(' + ') }
      case 'static':
        return { type: 'Static', coins: cs.static_coins || [] }
      default:
        return null
    }
  }, [selectedStrategy])

  // Auto-select first model
  useEffect(() => {
    if (!formState.aiModelId && aiModels?.length) {
      const enabled = aiModels.find((m) => m.enabled)
      if (enabled) setFormState((s) => ({ ...s, aiModelId: enabled.id }))
    }
  }, [aiModels, formState.aiModelId])

  // Auto-select first run
  useEffect(() => {
    if (!selectedRunId && runs.length > 0) {
      setSelectedRunId(runs[0].run_id)
    }
  }, [runs, selectedRunId])

  // Handlers
  const handleFormChange = (key: string, value: string | number | boolean | string[]) => {
    setFormState((prev) => ({ ...prev, [key]: value }))
  }

  const handleStart = async (event: FormEvent) => {
    event.preventDefault()
    if (!selectedModel?.enabled) {
      setToast({ text: tr('toasts.selectModel'), tone: 'error' })
      return
    }

    try {
      setIsStarting(true)
      const start = new Date(formState.start).getTime()
      const end = new Date(formState.end).getTime()
      if (end <= start) throw new Error(tr('toasts.invalidRange'))

      // Parse user symbols - if using dynamic coin strategy, allow empty
      const userSymbols = formState.symbols.split(',').map((s) => s.trim()).filter(Boolean)

      // Only send empty symbols if user deliberately cleared them and strategy has dynamic coin source
      const symbolsToSend = (userSymbols.length === 0 && strategyHasDynamicCoins) ? [] : userSymbols

      const payload = await api.startBacktest({
        run_id: formState.runId.trim() || undefined,
        strategy_id: formState.strategyId || undefined, // Use saved strategy from Strategy Studio
        symbols: symbolsToSend,
        timeframes: formState.timeframes,
        decision_timeframe: formState.decisionTf,
        decision_cadence_nbars: formState.cadence,
        start_ts: Math.floor(start / 1000),
        end_ts: Math.floor(end / 1000),
        initial_balance: formState.balance,
        fee_bps: formState.fee,
        slippage_bps: formState.slippage,
        fill_policy: formState.fill,
        prompt_variant: formState.prompt,
        prompt_template: formState.promptTemplate,
        custom_prompt: formState.customPrompt.trim() || undefined,
        override_prompt: formState.overridePrompt,
        cache_ai: formState.cacheAI,
        replay_only: formState.replayOnly,
        ai_model_id: formState.aiModelId,
        leverage: {
          btc_eth_leverage: formState.btcEthLeverage,
          altcoin_leverage: formState.altcoinLeverage,
        },
      })

      setToast({ text: tr('toasts.startSuccess', { id: payload.run_id }), tone: 'success' })
      setSelectedRunId(payload.run_id)
      setWizardStep(1)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.startFailed')
      setToast({ text: errMsg, tone: 'error' })
    } finally {
      setIsStarting(false)
    }
  }

  const handleControl = async (action: 'pause' | 'resume' | 'stop') => {
    if (!selectedRunId) return
    try {
      if (action === 'pause') await api.pauseBacktest(selectedRunId)
      if (action === 'resume') await api.resumeBacktest(selectedRunId)
      if (action === 'stop') await api.stopBacktest(selectedRunId)
      setToast({ text: tr('toasts.actionSuccess', { action, id: selectedRunId }), tone: 'success' })
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.actionFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleDelete = async () => {
    if (!selectedRunId) return
    const confirmed = await confirmToast(tr('toasts.confirmDelete', { id: selectedRunId }), {
      title: language === 'zh' ? '确认删除' : 'Confirm Delete',
      okText: language === 'zh' ? '删除' : 'Delete',
      cancelText: language === 'zh' ? '取消' : 'Cancel',
    })
    if (!confirmed) return
    try {
      await api.deleteBacktestRun(selectedRunId)
      setToast({ text: tr('toasts.deleteSuccess'), tone: 'success' })
      setSelectedRunId(undefined)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.deleteFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleExport = async () => {
    if (!selectedRunId) return
    try {
      const blob = await api.exportBacktest(selectedRunId)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${selectedRunId}_export.zip`
      link.click()
      URL.revokeObjectURL(url)
      setToast({ text: tr('toasts.exportSuccess', { id: selectedRunId }), tone: 'success' })
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.exportFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const toggleCompare = (runId: string) => {
    setCompareRunIds((prev) =>
      prev.includes(runId) ? prev.filter((id) => id !== runId) : [...prev, runId].slice(-3)
    )
  }

  const quickRanges = [
    { label: language === 'zh' ? '24小时' : '24h', hours: 24 },
    { label: language === 'zh' ? '3天' : '3d', hours: 72 },
    { label: language === 'zh' ? '7天' : '7d', hours: 168 },
    { label: language === 'zh' ? '30天' : '30d', hours: 720 },
  ]

  const applyQuickRange = (hours: number) => {
    const endDate = new Date()
    const startDate = new Date(endDate.getTime() - hours * 3600 * 1000)
    handleFormChange('start', toLocalInput(startDate))
    handleFormChange('end', toLocalInput(endDate))
  }

  const getStateColor = (state: string) => {
    switch (state) {
      case 'running':
        return '#F0B90B'
      case 'completed':
        return '#0ECB81'
      case 'failed':
      case 'liquidated':
        return '#F6465D'
      case 'paused':
        return '#848E9C'
      default:
        return '#848E9C'
    }
  }

  const getStateIcon = (state: string) => {
    switch (state) {
      case 'running':
        return <Activity className="w-4 h-4" />
      case 'completed':
        return <CheckCircle2 className="w-4 h-4" />
      case 'failed':
      case 'liquidated':
        return <XCircle className="w-4 h-4" />
      case 'paused':
        return <Pause className="w-4 h-4" />
      default:
        return <Clock className="w-4 h-4" />
    }
  }

  // Render
  return (
    <DeepVoidBackground className="py-8" disableAnimation>
      <div className="w-full px-4 md:px-8 space-y-6">
        {/* Toast */}
        <AnimatePresence>
          {toast && (
            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              className="p-3 rounded-lg text-sm"
              style={{
                background:
                  toast.tone === 'error'
                    ? 'rgba(246,70,93,0.15)'
                    : toast.tone === 'success'
                      ? 'rgba(14,203,129,0.15)'
                      : 'rgba(240,185,11,0.15)',
                color: toast.tone === 'error' ? '#F6465D' : toast.tone === 'success' ? '#0ECB81' : '#F0B90B',
                border: `1px solid ${toast.tone === 'error' ? 'rgba(246,70,93,0.3)' : toast.tone === 'success' ? 'rgba(14,203,129,0.3)' : 'rgba(240,185,11,0.3)'}`,
              }}
            >
              {toast.text}
            </motion.div>
          )}
        </AnimatePresence>

        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold flex items-center gap-3" style={{ color: '#EAECEF' }}>
              <Brain className="w-7 h-7" style={{ color: '#F0B90B' }} />
              {tr('title')}
            </h1>
            <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
              {tr('subtitle')}
            </p>
          </div>
          <button
            onClick={() => setWizardStep(1)}
            className="px-4 py-2 rounded-lg font-medium flex items-center gap-2 transition-all hover:opacity-90"
            style={{ background: '#F0B90B', color: '#0B0E11' }}
          >
            <Play className="w-4 h-4" />
            {language === 'zh' ? '新建回测' : 'New Backtest'}
          </button>
        </div>

        <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
          {/* Left Panel - Config / History */}
          <div className="space-y-4">
            {/* Wizard */}
            <div className="binance-card p-5">
              <div className="flex items-center gap-2 mb-4">
                {[1, 2, 3].map((step) => (
                  <div key={step} className="flex items-center">
                    <button
                      onClick={() => setWizardStep(step as WizardStep)}
                      className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
                      style={{
                        background: wizardStep >= step ? '#F0B90B' : '#2B3139',
                        color: wizardStep >= step ? '#0B0E11' : '#848E9C',
                      }}
                    >
                      {step}
                    </button>
                    {step < 3 && (
                      <div
                        className="w-8 h-0.5 mx-1"
                        style={{ background: wizardStep > step ? '#F0B90B' : '#2B3139' }}
                      />
                    )}
                  </div>
                ))}
                <span className="ml-2 text-xs" style={{ color: '#848E9C' }}>
                  {wizardStep === 1
                    ? language === 'zh'
                      ? '选择模型'
                      : 'Select Model'
                    : wizardStep === 2
                      ? language === 'zh'
                        ? '配置参数'
                        : 'Configure'
                      : language === 'zh'
                        ? '确认启动'
                        : 'Confirm'}
                </span>
              </div>

              <form onSubmit={handleStart}>
                <AnimatePresence mode="wait">
                  {/* Step 1: Model & Symbols */}
                  {wizardStep === 1 && (
                    <motion.div
                      key="step1"
                      initial={{ opacity: 0, x: 20 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0, x: -20 }}
                      className="space-y-4"
                    >
                      <div>
                        <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                          {tr('form.aiModelLabel')}
                        </label>
                        <select
                          className="w-full p-3 rounded-lg text-sm"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.aiModelId}
                          onChange={(e) => handleFormChange('aiModelId', e.target.value)}
                        >
                          <option value="">{tr('form.selectAiModel')}</option>
                          {aiModels?.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.name} ({m.provider}) {!m.enabled && '⚠️'}
                            </option>
                          ))}
                        </select>
                        {selectedModel && (
                          <div className="mt-2 flex items-center gap-2 text-xs">
                            <span
                              className="px-2 py-0.5 rounded"
                              style={{
                                background: selectedModel.enabled ? 'rgba(14,203,129,0.1)' : 'rgba(246,70,93,0.1)',
                                color: selectedModel.enabled ? '#0ECB81' : '#F6465D',
                              }}
                            >
                              {selectedModel.enabled ? tr('form.enabled') : tr('form.disabled')}
                            </span>
                          </div>
                        )}
                      </div>

                      {/* Strategy Selection (Optional) */}
                      <div>
                        <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                          {language === 'zh' ? '策略配置（可选）' : 'Strategy (Optional)'}
                        </label>
                        <select
                          className="w-full p-3 rounded-lg text-sm"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.strategyId}
                          onChange={(e) => handleFormChange('strategyId', e.target.value)}
                        >
                          <option value="">{language === 'zh' ? '不使用保存的策略' : 'No saved strategy'}</option>
                          {strategies?.map((s) => (
                            <option key={s.id} value={s.id}>
                              {s.name} {s.is_active && '✓'} {s.is_default && '⭐'}
                            </option>
                          ))}
                        </select>
                        {formState.strategyId && coinSourceDescription && (
                          <div className="mt-2 p-2 rounded" style={{ background: 'rgba(240,185,11,0.1)', border: '1px solid rgba(240,185,11,0.2)' }}>
                            <div className="flex items-center gap-2 text-xs">
                              <span style={{ color: '#F0B90B' }}>
                                {language === 'zh' ? '币种来源:' : 'Coin Source:'}
                              </span>
                              <span className="font-medium" style={{ color: '#EAECEF' }}>
                                {coinSourceDescription.type}
                                {coinSourceDescription.limit && ` (${coinSourceDescription.limit})`}
                                {coinSourceDescription.desc && ` - ${coinSourceDescription.desc}`}
                              </span>
                            </div>
                            {strategyHasDynamicCoins && (
                              <div className="text-xs mt-1" style={{ color: '#F0B90B' }}>
                                {language === 'zh'
                                  ? '⚡ 清空下方币种输入框即可使用策略的动态币种'
                                  : '⚡ Clear the symbols field below to use strategy\'s dynamic coins'}
                              </div>
                            )}
                          </div>
                        )}
                      </div>

                      <div>
                        <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                          {tr('form.symbolsLabel')}
                          {strategyHasDynamicCoins && (
                            <span className="ml-2" style={{ color: '#5E6673' }}>
                              ({language === 'zh' ? '可选 - 策略已配置币种来源' : 'Optional - strategy has coin source'})
                            </span>
                          )}
                        </label>
                        {!strategyHasDynamicCoins && (
                          <div className="flex flex-wrap gap-1 mb-2">
                            {POPULAR_SYMBOLS.map((sym) => {
                              const isSelected = formState.symbols.includes(sym)
                              return (
                                <button
                                  key={sym}
                                  type="button"
                                  onClick={() => {
                                    const current = formState.symbols.split(',').map((s) => s.trim()).filter(Boolean)
                                    const updated = isSelected
                                      ? current.filter((s) => s !== sym)
                                      : [...current, sym]
                                    handleFormChange('symbols', updated.join(','))
                                  }}
                                  className="px-2 py-1 rounded text-xs transition-all"
                                  style={{
                                    background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                                    border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                                    color: isSelected ? '#F0B90B' : '#848E9C',
                                  }}
                                >
                                  {sym.replace('USDT', '')}
                                </button>
                              )
                            })}
                          </div>
                        )}
                        <div className="relative">
                          <textarea
                            className="w-full p-2 rounded-lg text-xs font-mono"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                            value={formState.symbols}
                            onChange={(e) => handleFormChange('symbols', e.target.value)}
                            rows={2}
                            placeholder={strategyHasDynamicCoins
                              ? (language === 'zh' ? '留空将使用策略配置的币种来源' : 'Leave empty to use strategy coin source')
                              : ''
                            }
                          />
                          {strategyHasDynamicCoins && formState.symbols && (
                            <button
                              type="button"
                              onClick={() => handleFormChange('symbols', '')}
                              className="absolute top-2 right-2 px-2 py-1 rounded text-xs"
                              style={{ background: '#F0B90B', color: '#0B0E11' }}
                            >
                              {language === 'zh' ? '清空使用策略币种' : 'Clear to use strategy'}
                            </button>
                          )}
                        </div>
                      </div>

                      <button
                        type="button"
                        onClick={() => setWizardStep(2)}
                        disabled={!selectedModel?.enabled}
                        className="w-full py-2.5 rounded-lg font-medium flex items-center justify-center gap-2 transition-all disabled:opacity-50"
                        style={{ background: '#F0B90B', color: '#0B0E11' }}
                      >
                        {language === 'zh' ? '下一步' : 'Next'}
                        <ChevronRight className="w-4 h-4" />
                      </button>
                    </motion.div>
                  )}

                  {/* Step 2: Parameters */}
                  {wizardStep === 2 && (
                    <motion.div
                      key="step2"
                      initial={{ opacity: 0, x: 20 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0, x: -20 }}
                      className="space-y-4"
                    >
                      <div>
                        <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                          {tr('form.timeRangeLabel')}
                        </label>
                        <div className="flex flex-wrap gap-1 mb-2">
                          {quickRanges.map((r) => (
                            <button
                              key={r.hours}
                              type="button"
                              onClick={() => applyQuickRange(r.hours)}
                              className="px-3 py-1 rounded text-xs"
                              style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                            >
                              {r.label}
                            </button>
                          ))}
                        </div>
                        <div className="grid grid-cols-2 gap-2">
                          <input
                            type="datetime-local"
                            className="p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.start}
                            onChange={(e) => handleFormChange('start', e.target.value)}
                          />
                          <input
                            type="datetime-local"
                            className="p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.end}
                            onChange={(e) => handleFormChange('end', e.target.value)}
                          />
                        </div>
                      </div>

                      <div>
                        <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                          {language === 'zh' ? '时间周期' : 'Timeframes'}
                        </label>
                        <div className="flex flex-wrap gap-1">
                          {TIMEFRAME_OPTIONS.map((tf) => {
                            const isSelected = formState.timeframes.includes(tf)
                            return (
                              <button
                                key={tf}
                                type="button"
                                onClick={() => {
                                  const updated = isSelected
                                    ? formState.timeframes.filter((t) => t !== tf)
                                    : [...formState.timeframes, tf]
                                  if (updated.length > 0) handleFormChange('timeframes', updated)
                                }}
                                className="px-2 py-1 rounded text-xs transition-all"
                                style={{
                                  background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                                  border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                                  color: isSelected ? '#F0B90B' : '#848E9C',
                                }}
                              >
                                {tf}
                              </button>
                            )
                          })}
                        </div>
                      </div>

                      <div className="grid grid-cols-2 gap-3">
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.initialBalanceLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.balance}
                            onChange={(e) => handleFormChange('balance', Number(e.target.value))}
                          />
                        </div>
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.decisionTfLabel')}
                          </label>
                          <select
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.decisionTf}
                            onChange={(e) => handleFormChange('decisionTf', e.target.value)}
                          >
                            {formState.timeframes.map((tf) => (
                              <option key={tf} value={tf}>
                                {tf}
                              </option>
                            ))}
                          </select>
                        </div>
                      </div>

                      <div className="flex gap-2">
                        <button
                          type="button"
                          onClick={() => setWizardStep(1)}
                          className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                          style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        >
                          <ChevronLeft className="w-4 h-4" />
                          {language === 'zh' ? '上一步' : 'Back'}
                        </button>
                        <button
                          type="button"
                          onClick={() => setWizardStep(3)}
                          className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                          style={{ background: '#F0B90B', color: '#0B0E11' }}
                        >
                          {language === 'zh' ? '下一步' : 'Next'}
                          <ChevronRight className="w-4 h-4" />
                        </button>
                      </div>
                    </motion.div>
                  )}

                  {/* Step 3: Advanced & Confirm */}
                  {wizardStep === 3 && (
                    <motion.div
                      key="step3"
                      initial={{ opacity: 0, x: 20 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0, x: -20 }}
                      className="space-y-4"
                    >
                      <div className="grid grid-cols-2 gap-3">
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.btcEthLeverageLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.btcEthLeverage}
                            onChange={(e) => handleFormChange('btcEthLeverage', Number(e.target.value))}
                          />
                        </div>
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.altcoinLeverageLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.altcoinLeverage}
                            onChange={(e) => handleFormChange('altcoinLeverage', Number(e.target.value))}
                          />
                        </div>
                      </div>

                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.feeLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.fee}
                            onChange={(e) => handleFormChange('fee', Number(e.target.value))}
                          />
                        </div>
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.slippageLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.slippage}
                            onChange={(e) => handleFormChange('slippage', Number(e.target.value))}
                          />
                        </div>
                        <div>
                          <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                            {tr('form.cadenceLabel')}
                          </label>
                          <input
                            type="number"
                            className="w-full p-2 rounded-lg text-xs"
                            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                            value={formState.cadence}
                            onChange={(e) => handleFormChange('cadence', Number(e.target.value))}
                          />
                        </div>
                      </div>

                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {language === 'zh' ? '策略风格' : 'Strategy Style'}
                        </label>
                        <div className="flex flex-wrap gap-1">
                          {['baseline', 'aggressive', 'conservative', 'scalping'].map((p) => (
                            <button
                              key={p}
                              type="button"
                              onClick={() => handleFormChange('prompt', p)}
                              className="px-3 py-1.5 rounded text-xs transition-all"
                              style={{
                                background: formState.prompt === p ? 'rgba(240,185,11,0.15)' : '#1E2329',
                                border: `1px solid ${formState.prompt === p ? '#F0B90B' : '#2B3139'}`,
                                color: formState.prompt === p ? '#F0B90B' : '#848E9C',
                              }}
                            >
                              {tr(`form.promptPresets.${p}`)}
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-4 text-xs" style={{ color: '#848E9C' }}>
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formState.cacheAI}
                            onChange={(e) => handleFormChange('cacheAI', e.target.checked)}
                            className="accent-[#F0B90B]"
                          />
                          {tr('form.cacheAiLabel')}
                        </label>
                        <label className="flex items-center gap-2 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={formState.replayOnly}
                            onChange={(e) => handleFormChange('replayOnly', e.target.checked)}
                            className="accent-[#F0B90B]"
                          />
                          {tr('form.replayOnlyLabel')}
                        </label>
                      </div>

                      <div className="flex gap-2">
                        <button
                          type="button"
                          onClick={() => setWizardStep(2)}
                          className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                          style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                        >
                          <ChevronLeft className="w-4 h-4" />
                          {language === 'zh' ? '上一步' : 'Back'}
                        </button>
                        <button
                          type="submit"
                          disabled={isStarting}
                          className="flex-1 py-2 rounded-lg font-bold flex items-center justify-center gap-2 disabled:opacity-50"
                          style={{ background: '#F0B90B', color: '#0B0E11' }}
                        >
                          {isStarting ? (
                            <RefreshCw className="w-4 h-4 animate-spin" />
                          ) : (
                            <Zap className="w-4 h-4" />
                          )}
                          {isStarting ? tr('starting') : tr('start')}
                        </button>
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </form>
            </div>

            {/* Run History */}
            <div className="binance-card p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
                  <Layers className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  {tr('runList.title')}
                </h3>
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {runs.length} {language === 'zh' ? '条' : 'runs'}
                </span>
              </div>

              <div className="space-y-2 max-h-[300px] overflow-y-auto">
                {runs.length === 0 ? (
                  <div className="py-8 text-center text-sm" style={{ color: '#5E6673' }}>
                    {tr('emptyStates.noRuns')}
                  </div>
                ) : (
                  runs.map((run) => (
                    <button
                      key={run.run_id}
                      onClick={() => setSelectedRunId(run.run_id)}
                      className="w-full p-3 rounded-lg text-left transition-all"
                      style={{
                        background: run.run_id === selectedRunId ? 'rgba(240,185,11,0.1)' : '#1E2329',
                        border: `1px solid ${run.run_id === selectedRunId ? '#F0B90B' : '#2B3139'}`,
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono text-xs" style={{ color: '#EAECEF' }}>
                          {run.run_id.slice(0, 20)}...
                        </span>
                        <span
                          className="flex items-center gap-1 text-xs"
                          style={{ color: getStateColor(run.state) }}
                        >
                          {getStateIcon(run.state)}
                          {tr(`states.${run.state}`)}
                        </span>
                      </div>
                      <div className="flex items-center justify-between mt-1">
                        <span className="text-xs" style={{ color: '#848E9C' }}>
                          {run.summary.progress_pct.toFixed(0)}% · ${run.summary.equity_last.toFixed(0)}
                        </span>
                        <button
                          onClick={(e) => {
                            e.stopPropagation()
                            toggleCompare(run.run_id)
                          }}
                          className="p-1 rounded"
                          style={{
                            background: compareRunIds.includes(run.run_id)
                              ? 'rgba(240,185,11,0.2)'
                              : 'transparent',
                          }}
                          title={language === 'zh' ? '添加到对比' : 'Add to compare'}
                        >
                          <Eye
                            className="w-3 h-3"
                            style={{
                              color: compareRunIds.includes(run.run_id) ? '#F0B90B' : '#5E6673',
                            }}
                          />
                        </button>
                      </div>
                    </button>
                  ))
                )}
              </div>
            </div>
          </div>

          {/* Right Panel - Results */}
          <div className="xl:col-span-2 space-y-4">
            {!selectedRunId ? (
              <div
                className="binance-card p-12 text-center"
                style={{ color: '#5E6673' }}
              >
                <Brain className="w-12 h-12 mx-auto mb-4 opacity-30" />
                <p>{tr('emptyStates.selectRun')}</p>
              </div>
            ) : (
              <>
                {/* Status Bar */}
                <div className="binance-card p-4">
                  <div className="flex flex-wrap items-center justify-between gap-4">
                    <div className="flex items-center gap-4">
                      <ProgressRing progress={status?.progress_pct ?? selectedRun?.summary.progress_pct ?? 0} size={80} />
                      <div>
                        <h2 className="font-mono font-bold" style={{ color: '#EAECEF' }}>
                          {selectedRunId}
                        </h2>
                        <div className="flex items-center gap-2 mt-1">
                          <span
                            className="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium"
                            style={{
                              background: `${getStateColor(status?.state ?? selectedRun?.state ?? '')}20`,
                              color: getStateColor(status?.state ?? selectedRun?.state ?? ''),
                            }}
                          >
                            {getStateIcon(status?.state ?? selectedRun?.state ?? '')}
                            {tr(`states.${status?.state ?? selectedRun?.state}`)}
                          </span>
                          {selectedRun?.summary.decision_tf && (
                            <span className="text-xs" style={{ color: '#848E9C' }}>
                              {selectedRun.summary.decision_tf} · {selectedRun.summary.symbol_count} symbols
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      {(status?.state === 'running' || selectedRun?.state === 'running') && (
                        <>
                          <button
                            onClick={() => handleControl('pause')}
                            className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                            style={{ border: '1px solid #2B3139' }}
                            title={tr('actions.pause')}
                          >
                            <Pause className="w-4 h-4" style={{ color: '#F0B90B' }} />
                          </button>
                          <button
                            onClick={() => handleControl('stop')}
                            className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                            style={{ border: '1px solid #2B3139' }}
                            title={tr('actions.stop')}
                          >
                            <Square className="w-4 h-4" style={{ color: '#F6465D' }} />
                          </button>
                        </>
                      )}
                      {status?.state === 'paused' && (
                        <button
                          onClick={() => handleControl('resume')}
                          className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                          style={{ border: '1px solid #2B3139' }}
                          title={tr('actions.resume')}
                        >
                          <Play className="w-4 h-4" style={{ color: '#0ECB81' }} />
                        </button>
                      )}
                      <button
                        onClick={handleExport}
                        className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                        style={{ border: '1px solid #2B3139' }}
                        title={tr('detail.exportLabel')}
                      >
                        <Download className="w-4 h-4" style={{ color: '#EAECEF' }} />
                      </button>
                      <button
                        onClick={handleDelete}
                        className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                        style={{ border: '1px solid #2B3139' }}
                        title={tr('detail.deleteLabel')}
                      >
                        <Trash2 className="w-4 h-4" style={{ color: '#F6465D' }} />
                      </button>
                    </div>
                  </div>

                  {(status?.note || status?.last_error) && (
                    <div
                      className="mt-3 p-2 rounded-lg text-xs flex items-center gap-2"
                      style={{
                        background: 'rgba(246,70,93,0.1)',
                        border: '1px solid rgba(246,70,93,0.3)',
                        color: '#F6465D',
                      }}
                    >
                      <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                      {status?.note || status?.last_error}
                    </div>
                  )}

                  {/* Real-time Positions Display */}
                  {status?.positions && status.positions.length > 0 && (
                    <PositionsDisplay positions={status.positions} language={language} />
                  )}
                </div>

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  <StatCard
                    icon={Target}
                    label={language === 'zh' ? '当前净值' : 'Equity'}
                    value={(status?.equity ?? 0).toFixed(2)}
                    suffix="USDT"
                    language={language}
                  />
                  <StatCard
                    icon={TrendingUp}
                    label={language === 'zh' ? '总收益率' : 'Return'}
                    value={`${(metrics?.total_return_pct ?? 0).toFixed(2)}%`}
                    trend={(metrics?.total_return_pct ?? 0) >= 0 ? 'up' : 'down'}
                    color={(metrics?.total_return_pct ?? 0) >= 0 ? '#0ECB81' : '#F6465D'}
                    metricKey="total_return"
                    language={language}
                  />
                  <StatCard
                    icon={AlertTriangle}
                    label={language === 'zh' ? '最大回撤' : 'Max DD'}
                    value={`${(metrics?.max_drawdown_pct ?? 0).toFixed(2)}%`}
                    color="#F6465D"
                    metricKey="max_drawdown"
                    language={language}
                  />
                  <StatCard
                    icon={BarChart3}
                    label={language === 'zh' ? '夏普比率' : 'Sharpe'}
                    value={(metrics?.sharpe_ratio ?? 0).toFixed(2)}
                    metricKey="sharpe_ratio"
                    language={language}
                  />
                </div>

                {/* Tabs */}
                <div className="binance-card">
                  <div className="flex border-b" style={{ borderColor: '#2B3139' }}>
                    {(['overview', 'chart', 'trades', 'decisions'] as ViewTab[]).map((tab) => (
                      <button
                        key={tab}
                        onClick={() => setViewTab(tab)}
                        className="px-4 py-3 text-sm font-medium transition-all relative"
                        style={{ color: viewTab === tab ? '#F0B90B' : '#848E9C' }}
                      >
                        {tab === 'overview'
                          ? language === 'zh'
                            ? '概览'
                            : 'Overview'
                          : tab === 'chart'
                            ? language === 'zh'
                              ? '图表'
                              : 'Chart'
                            : tab === 'trades'
                              ? language === 'zh'
                                ? '交易'
                                : 'Trades'
                              : language === 'zh'
                                ? 'AI决策'
                                : 'Decisions'}
                        {viewTab === tab && (
                          <motion.div
                            layoutId="tab-indicator"
                            className="absolute bottom-0 left-0 right-0 h-0.5"
                            style={{ background: '#F0B90B' }}
                          />
                        )}
                      </button>
                    ))}
                  </div>

                  <div className="p-4">
                    <AnimatePresence mode="wait">
                      {viewTab === 'overview' && (
                        <motion.div
                          key="overview"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          exit={{ opacity: 0 }}
                        >
                          {equity && equity.length > 0 ? (
                            <BacktestChart equity={equity} trades={trades ?? []} />
                          ) : (
                            <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                              {tr('charts.equityEmpty')}
                            </div>
                          )}

                          {metrics && (
                            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-4">
                              <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                                <div className="flex items-center gap-1 text-xs" style={{ color: '#848E9C' }}>
                                  {language === 'zh' ? '胜率' : 'Win Rate'}
                                  <MetricTooltip metricKey="win_rate" language={language} size={11} />
                                </div>
                                <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                  {(metrics.win_rate ?? 0).toFixed(1)}%
                                </div>
                              </div>
                              <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                                <div className="flex items-center gap-1 text-xs" style={{ color: '#848E9C' }}>
                                  {language === 'zh' ? '盈亏因子' : 'Profit Factor'}
                                  <MetricTooltip metricKey="profit_factor" language={language} size={11} />
                                </div>
                                <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                  {(metrics.profit_factor ?? 0).toFixed(2)}
                                </div>
                              </div>
                              <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                                <div className="text-xs" style={{ color: '#848E9C' }}>
                                  {language === 'zh' ? '总交易数' : 'Total Trades'}
                                </div>
                                <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                  {metrics.trades ?? 0}
                                </div>
                              </div>
                              <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                                <div className="text-xs" style={{ color: '#848E9C' }}>
                                  {language === 'zh' ? '最佳币种' : 'Best Symbol'}
                                </div>
                                <div className="text-lg font-bold" style={{ color: '#0ECB81' }}>
                                  {metrics.best_symbol?.replace('USDT', '') || '-'}
                                </div>
                              </div>
                            </div>
                          )}
                        </motion.div>
                      )}

                      {viewTab === 'chart' && (
                        <motion.div
                          key="chart"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          exit={{ opacity: 0 }}
                          className="space-y-6"
                        >
                          {/* Equity Chart */}
                          <div>
                            <h4 className="text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
                              {language === 'zh' ? '资金曲线' : 'Equity Curve'}
                            </h4>
                            {equity && equity.length > 0 ? (
                              <BacktestChart equity={equity} trades={trades ?? []} />
                            ) : (
                              <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                                {tr('charts.equityEmpty')}
                              </div>
                            )}
                          </div>

                          {/* Candlestick Chart with Trade Markers */}
                          {selectedRunId && trades && trades.length > 0 && (
                            <div>
                              <h4 className="text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
                                {language === 'zh' ? 'K线图 & 交易标记' : 'Candlestick & Trade Markers'}
                              </h4>
                              <CandlestickChartComponent
                                runId={selectedRunId}
                                trades={trades}
                                language={language}
                              />
                            </div>
                          )}
                        </motion.div>
                      )}

                      {viewTab === 'trades' && (
                        <motion.div
                          key="trades"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          exit={{ opacity: 0 }}
                        >
                          <TradeTimeline trades={trades ?? []} />
                        </motion.div>
                      )}

                      {viewTab === 'decisions' && (
                        <motion.div
                          key="decisions"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          exit={{ opacity: 0 }}
                          className="space-y-3 max-h-[500px] overflow-y-auto"
                        >
                          {decisions && decisions.length > 0 ? (
                            decisions.map((d) => (
                              <DecisionCard
                                key={`${d.cycle_number}-${d.timestamp}`}
                                decision={d}
                                language={language}
                              />
                            ))
                          ) : (
                            <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                              {tr('decisionTrail.emptyHint')}
                            </div>
                          )}
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </div>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
