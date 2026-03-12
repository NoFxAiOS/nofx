import { useEffect, useMemo, useState, useRef } from 'react'
import { motion } from 'framer-motion'
import {
  createChart,
  ColorType,
  CrosshairMode,
  CandlestickSeries,
  createSeriesMarkers,
  type IChartApi,
  type ISeriesApi,
  type CandlestickData,
  type UTCTimestamp,
  type SeriesMarker,
} from 'lightweight-charts'
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
import {
  Clock,
  AlertTriangle,
  RefreshCw,
  CandlestickChart as CandlestickIcon,
} from 'lucide-react'
import { api } from '../../lib/api'
import { t, type Language } from '../../i18n/translations'
import type {
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestKlinesResponse,
} from '../../types'

// ============ Equity Chart (Recharts) ============

interface EquityChartProps {
  equity: BacktestEquityPoint[]
  trades: BacktestTradeEvent[]
}

export function EquityChart({ equity, trades }: EquityChartProps) {
  const chartData = useMemo(() => {
    return equity.map((point) => ({
      time: new Date(point.ts).toLocaleString(),
      ts: point.ts,
      equity: point.equity,
      pnl_pct: point.pnl_pct,
    }))
  }, [equity])

  const tradeMarkers = useMemo(() => {
    if (!trades.length || !equity.length) return []
    return trades
      .filter((t) => t.action.includes('open') || t.action.includes('close'))
      .map((trade) => {
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
      .slice(-30)
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

// ============ Candlestick Chart with Trade Markers ============

interface CandlestickChartProps {
  runId: string
  trades: BacktestTradeEvent[]
  language: Language
}

export function CandlestickChartComponent({ runId, trades, language }: CandlestickChartProps) {
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)

  const symbols = useMemo(() => {
    const symbolSet = new Set(trades.map((t) => t.symbol))
    return Array.from(symbolSet).sort()
  }, [trades])

  const [selectedSymbol, setSelectedSymbol] = useState<string>(symbols[0] || '')
  const [selectedTimeframe, setSelectedTimeframe] = useState<string>('15m')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const CHART_TIMEFRAMES = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d']

  useEffect(() => {
    if (symbols.length > 0 && !symbols.includes(selectedSymbol)) {
      setSelectedSymbol(symbols[0])
    }
  }, [symbols, selectedSymbol])

  const symbolTrades = useMemo(() => {
    return trades.filter((t) => t.symbol === selectedSymbol)
  }, [trades, selectedSymbol])

  useEffect(() => {
    if (!chartContainerRef.current || !selectedSymbol || !runId) return

    const container = chartContainerRef.current

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

    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#0ECB81',
      downColor: '#F6465D',
      borderUpColor: '#0ECB81',
      borderDownColor: '#F6465D',
      wickUpColor: '#0ECB81',
      wickDownColor: '#F6465D',
    })
    candleSeriesRef.current = candleSeries

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

        const markers: SeriesMarker<UTCTimestamp>[] = symbolTrades
          .map((trade) => {
            const tradeTime = Math.floor(trade.ts / 1000)
            const closestKline = data.klines.reduce((prev, curr) =>
              Math.abs(curr.time - tradeTime) < Math.abs(prev.time - tradeTime) ? curr : prev
            )
            const isOpen = trade.action.includes('open')
            const isLong = trade.side === 'long' || trade.action.includes('long')
            const pnl = trade.realized_pnl

            let text = ''
            let color = '#0ECB81'

            if (isOpen) {
              if (isLong) {
                text = `▲ Long @${trade.price.toFixed(2)}`
                color = '#0ECB81'
              } else {
                text = `▼ Short @${trade.price.toFixed(2)}`
                color = '#F6465D'
              }
            } else {
              const pnlStr = pnl >= 0 ? `+$${pnl.toFixed(2)}` : `-$${Math.abs(pnl).toFixed(2)}`
              text = `✕ ${pnlStr}`
              color = pnl >= 0 ? '#0ECB81' : '#F6465D'
            }

            return {
              time: closestKline.time as UTCTimestamp,
              position: isOpen
                ? (isLong ? 'belowBar' as const : 'aboveBar' as const)
                : (isLong ? 'aboveBar' as const : 'belowBar' as const),
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
        {t('backtestChart.noTrades', language)}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4 flex-wrap">
        <div className="flex items-center gap-2">
          <CandlestickIcon size={16} style={{ color: '#F0B90B' }} />
          <span className="text-sm" style={{ color: '#848E9C' }}>
            {t('backtestChart.symbol', language)}
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
            {t('backtestChart.interval', language)}
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
          ({symbolTrades.length} {t('backtestChart.trades', language)})
        </span>
      </div>

      <div
        ref={chartContainerRef}
        className="w-full rounded-lg overflow-hidden"
        style={{ background: '#0B0E11', minHeight: 400 }}
      >
        {isLoading && (
          <div className="flex items-center justify-center h-[400px]" style={{ color: '#848E9C' }}>
            <RefreshCw className="animate-spin mr-2" size={16} />
            {t('backtestChart.loadingKline', language)}
          </div>
        )}
        {error && (
          <div className="flex items-center justify-center h-[400px]" style={{ color: '#F6465D' }}>
            <AlertTriangle className="mr-2" size={16} />
            {error}
          </div>
        )}
      </div>

      <div className="flex items-center gap-4 text-xs" style={{ color: '#848E9C' }}>
        <div className="flex items-center gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#0ECB81' }} />
          <span>{t('backtestChart.openProfit', language)}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full" style={{ background: '#F6465D' }} />
          <span>{t('backtestChart.lossClose', language)}</span>
        </div>
        <span style={{ color: '#5E6673' }}>|</span>
        <span>▲ Long · ▼ Short · ✕ {t('backtestChart.close', language)}</span>
      </div>
    </div>
  )
}

// ============ Chart Tab Content ============

interface BacktestChartTabProps {
  equity: BacktestEquityPoint[] | undefined
  trades: BacktestTradeEvent[] | undefined
  selectedRunId: string
  language: Language
  tr: (key: string) => string
}

export function BacktestChartTab({
  equity,
  trades,
  selectedRunId,
  language,
  tr,
}: BacktestChartTabProps) {
  return (
    <motion.div
      key="chart"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="space-y-6"
    >
      <div>
        <h4 className="text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
          {t('backtestChart.equityCurve', language)}
        </h4>
        {equity && equity.length > 0 ? (
          <EquityChart equity={equity} trades={trades ?? []} />
        ) : (
          <div className="py-12 text-center" style={{ color: '#5E6673' }}>
            {tr('charts.equityEmpty')}
          </div>
        )}
      </div>

      {selectedRunId && trades && trades.length > 0 && (
        <div>
          <h4 className="text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
            {t('backtestChart.candlestickTradeMarkers', language)}
          </h4>
          <CandlestickChartComponent
            runId={selectedRunId}
            trades={trades}
            language={language}
          />
        </div>
      )}
    </motion.div>
  )
}
