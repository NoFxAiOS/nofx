import { useMemo } from 'react'
import {
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
  Legend,
  Area,
  ComposedChart,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionTraderData } from '../types'
import { getTraderColor } from '../utils/traderColors'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, TrendingUp, TrendingDown, Zap } from 'lucide-react'

interface ComparisonChartProps {
  traders: CompetitionTraderData[]
}

export function ComparisonChart({ traders }: ComparisonChartProps) {
  const { language } = useLanguage()

  // Generate unique key for SWR
  const tradersKey = traders
    .map((t) => t.trader_id)
    .sort()
    .join(',')

  const { data: allTraderHistories, isLoading } = useSWR(
    traders.length > 0 ? `all-equity-histories-${tradersKey}` : null,
    async () => {
      const traderIds = traders.map((trader) => trader.trader_id)
      const batchData = await api.getEquityHistoryBatch(traderIds)
      return traders.map((trader) => {
        const history = batchData.histories?.[trader.trader_id] || []

        // If backend doesn't return total_pnl_pct, calculate it from equity
        if (history.length > 0 && history[0].total_pnl_pct === undefined) {
          const initialEquity = history[0].total_equity
          history.forEach((point: any) => {
            point.total_pnl_pct = initialEquity > 0
              ? ((point.total_equity - initialEquity) / initialEquity) * 100
              : 0
          })
        }

        return history
      })
    },
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const traderHistories = useMemo(() => {
    if (!allTraderHistories) {
      return traders.map(() => ({ data: undefined }))
    }
    return allTraderHistories.map((data) => ({ data }))
  }, [allTraderHistories, traders.length])

  const combinedData = useMemo(() => {
    const allLoaded = traderHistories.every((h) => h.data)
    if (!allLoaded) return []

    const timestampMap = new Map<
      string,
      {
        timestamp: string
        time: string
        traders: Map<string, { pnl_pct: number; equity: number }>
      }
    >()

    traderHistories.forEach((history, index) => {
      const trader = traders[index]
      if (!history.data) return

      history.data.forEach((point: any) => {
        const ts = point.timestamp

        if (!timestampMap.has(ts)) {
          const time = new Date(ts).toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit',
          })
          timestampMap.set(ts, {
            timestamp: ts,
            time,
            traders: new Map(),
          })
        }

        timestampMap.get(ts)!.traders.set(trader.trader_id, {
          pnl_pct: point.total_pnl_pct || 0,
          equity: point.total_equity,
        })
      })
    })

    const combined = Array.from(timestampMap.entries())
      .sort(([tsA], [tsB]) => new Date(tsA).getTime() - new Date(tsB).getTime())
      .map(([ts, data], index) => {
        const entry: any = {
          index: index + 1,
          time: data.time,
          timestamp: ts,
        }

        traders.forEach((trader) => {
          const traderData = data.traders.get(trader.trader_id)
          if (traderData) {
            entry[`${trader.trader_id}_pnl_pct`] = traderData.pnl_pct
            entry[`${trader.trader_id}_equity`] = traderData.equity
          }
        })

        return entry
      })

    return combined
  }, [allTraderHistories, traders])

  // Get trader color
  const traderColor = (traderId: string) => getTraderColor(traders, traderId)

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <div className="relative">
          <div className="w-16 h-16 border-4 border-t-transparent rounded-full animate-spin"
               style={{ borderColor: '#F0B90B', borderTopColor: 'transparent' }} />
          <TrendingUp className="w-6 h-6 absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2"
                      style={{ color: '#F0B90B' }} />
        </div>
        <div className="text-sm mt-4 font-medium" style={{ color: '#848E9C' }}>
          {t('loadingChartData', language) || 'Loading chart data...'}
        </div>
      </div>
    )
  }

  if (combinedData.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <div className="w-20 h-20 rounded-2xl flex items-center justify-center mb-4"
             style={{ background: 'rgba(240, 185, 11, 0.1)' }}>
          <BarChart3 className="w-10 h-10" style={{ color: '#F0B90B', opacity: 0.6 }} />
        </div>
        <div className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
          {t('noHistoricalData', language)}
        </div>
        <div className="text-sm text-center max-w-xs" style={{ color: '#848E9C' }}>
          {t('dataWillAppear', language)}
        </div>
      </div>
    )
  }

  const MAX_DISPLAY_POINTS = 500
  const displayData =
    combinedData.length > MAX_DISPLAY_POINTS
      ? combinedData.slice(-MAX_DISPLAY_POINTS)
      : combinedData

  // Calculate Y axis domain with better padding
  const calculateYDomain = () => {
    const allValues: number[] = []
    displayData.forEach((point) => {
      traders.forEach((trader) => {
        const value = point[`${trader.trader_id}_pnl_pct`]
        if (value !== undefined && !isNaN(value)) {
          allValues.push(value)
        }
      })
    })

    if (allValues.length === 0) return [-2, 2]

    const minVal = Math.min(...allValues)
    const maxVal = Math.max(...allValues)

    // Ensure zero is visible and add symmetric padding
    const absMax = Math.max(Math.abs(maxVal), Math.abs(minVal), 0.5)
    const padding = absMax * 0.3

    return [
      Math.floor((Math.min(minVal, 0) - padding) * 10) / 10,
      Math.ceil((Math.max(maxVal, 0) + padding) * 10) / 10
    ]
  }

  // Custom Tooltip
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      const date = new Date(data.timestamp)
      const dateStr = date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })

      return (
        <div
          className="rounded-xl p-4 shadow-2xl backdrop-blur-sm"
          style={{
            background: 'rgba(30, 35, 41, 0.95)',
            border: '1px solid rgba(240, 185, 11, 0.2)',
            minWidth: '200px'
          }}
        >
          <div className="flex items-center gap-2 mb-3 pb-2" style={{ borderBottom: '1px solid #2B3139' }}>
            <Zap className="w-3.5 h-3.5" style={{ color: '#F0B90B' }} />
            <span className="text-xs font-medium" style={{ color: '#F0B90B' }}>
              {dateStr} {data.time}
            </span>
          </div>
          <div className="space-y-2.5">
            {traders.map((trader) => {
              const pnlPct = data[`${trader.trader_id}_pnl_pct`]
              const equity = data[`${trader.trader_id}_equity`]
              if (pnlPct === undefined) return null
              const isPositive = pnlPct >= 0

              return (
                <div key={trader.trader_id} className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-2">
                    <div className="w-2.5 h-2.5 rounded-full"
                         style={{ background: traderColor(trader.trader_id) }} />
                    <span className="text-xs font-medium truncate max-w-[100px]"
                          style={{ color: '#EAECEF' }}>
                      {trader.trader_name}
                    </span>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-bold mono flex items-center gap-1"
                         style={{ color: isPositive ? '#0ECB81' : '#F6465D' }}>
                      {isPositive ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
                      {isPositive ? '+' : ''}{pnlPct.toFixed(2)}%
                    </div>
                    <div className="text-[10px] mono" style={{ color: '#5E6673' }}>
                      ${equity?.toFixed(2)}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )
    }
    return null
  }

  // Calculate stats
  const lastPoint = displayData[displayData.length - 1]
  const traderStats = traders.map(trader => {
    const currentPnl = lastPoint?.[`${trader.trader_id}_pnl_pct`] || 0
    const currentEquity = lastPoint?.[`${trader.trader_id}_equity`] || 0
    return { ...trader, currentPnl, currentEquity }
  }).sort((a, b) => b.currentPnl - a.currentPnl)

  const leader = traderStats[0]
  const gap = traderStats.length > 1
    ? Math.abs(traderStats[0].currentPnl - traderStats[1].currentPnl).toFixed(2)
    : '0.00'

  return (
    <div className="space-y-4">
      {/* Mini Stats Bar */}
      <div className="flex items-center gap-3 flex-wrap">
        {traderStats.slice(0, 5).map((trader, idx) => (
          <div key={trader.trader_id}
               className="flex items-center gap-2 px-3 py-1.5 rounded-full transition-all hover:scale-105"
               style={{
                 background: idx === 0 ? 'rgba(240, 185, 11, 0.15)' : 'rgba(43, 49, 57, 0.5)',
                 border: `1px solid ${idx === 0 ? 'rgba(240, 185, 11, 0.3)' : '#2B3139'}`
               }}>
            <div className="w-2 h-2 rounded-full"
                 style={{ background: traderColor(trader.trader_id) }} />
            <span className="text-xs font-medium truncate max-w-[80px]"
                  style={{ color: '#EAECEF' }}>
              {trader.trader_name}
            </span>
            <span className="text-xs font-bold mono"
                  style={{ color: trader.currentPnl >= 0 ? '#0ECB81' : '#F6465D' }}>
              {trader.currentPnl >= 0 ? '+' : ''}{trader.currentPnl.toFixed(2)}%
            </span>
          </div>
        ))}
      </div>

      {/* Chart */}
      <div className="relative rounded-xl overflow-hidden"
           style={{ background: 'linear-gradient(180deg, rgba(11, 14, 17, 0.8) 0%, rgba(11, 14, 17, 1) 100%)' }}>
        {/* Watermark */}
        <div style={{
          position: 'absolute',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          fontSize: '80px',
          fontWeight: 'bold',
          color: 'rgba(240, 185, 11, 0.03)',
          zIndex: 1,
          pointerEvents: 'none',
          fontFamily: 'monospace',
          letterSpacing: '0.1em',
        }}>
          NOFX
        </div>

        <ResponsiveContainer width="100%" height={420}>
          <ComposedChart
            data={displayData}
            margin={{ top: 20, right: 20, left: 10, bottom: 20 }}
          >
            <defs>
              {traders.map((trader) => (
                <linearGradient
                  key={`area-gradient-${trader.trader_id}`}
                  id={`area-gradient-${trader.trader_id}`}
                  x1="0" y1="0" x2="0" y2="1"
                >
                  <stop offset="0%" stopColor={traderColor(trader.trader_id)} stopOpacity={0.3} />
                  <stop offset="100%" stopColor={traderColor(trader.trader_id)} stopOpacity={0} />
                </linearGradient>
              ))}
              {/* Glow filter */}
              <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
                <feGaussianBlur stdDeviation="2" result="coloredBlur"/>
                <feMerge>
                  <feMergeNode in="coloredBlur"/>
                  <feMergeNode in="SourceGraphic"/>
                </feMerge>
              </filter>
            </defs>

            <CartesianGrid strokeDasharray="3 3" stroke="#1E2329" vertical={false} />

            <XAxis
              dataKey="time"
              stroke="#2B3139"
              tick={{ fill: '#5E6673', fontSize: 10 }}
              tickLine={false}
              axisLine={{ stroke: '#2B3139' }}
              interval={Math.max(Math.floor(displayData.length / 8), 1)}
            />

            <YAxis
              stroke="#2B3139"
              tick={{ fill: '#5E6673', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              domain={calculateYDomain()}
              tickFormatter={(value) => `${value.toFixed(1)}%`}
              width={50}
            />

            <Tooltip content={<CustomTooltip />} />

            {/* Zero reference line */}
            <ReferenceLine
              y={0}
              stroke="#474D57"
              strokeDasharray="8 4"
              strokeWidth={1}
            />

            {/* Area fills for top 2 traders */}
            {traders.slice(0, 2).map((trader) => (
              <Area
                key={`area-${trader.trader_id}`}
                type="monotone"
                dataKey={`${trader.trader_id}_pnl_pct`}
                fill={`url(#area-gradient-${trader.trader_id})`}
                stroke="none"
                connectNulls
              />
            ))}

            {/* Lines for all traders */}
            {traders.map((trader, idx) => (
              <Line
                key={trader.trader_id}
                type="monotone"
                dataKey={`${trader.trader_id}_pnl_pct`}
                stroke={traderColor(trader.trader_id)}
                strokeWidth={idx === 0 ? 3 : 2}
                dot={false}
                activeDot={{
                  r: 6,
                  fill: traderColor(trader.trader_id),
                  stroke: '#0B0E11',
                  strokeWidth: 2,
                  filter: 'url(#glow)',
                }}
                name={trader.trader_name}
                connectNulls
                style={{ filter: idx === 0 ? 'url(#glow)' : undefined }}
              />
            ))}

            <Legend
              wrapperStyle={{ paddingTop: '16px' }}
              iconType="circle"
              iconSize={8}
              formatter={(value) => {
                const trader = traders.find((t) => t.trader_name === value)
                const pnl = trader ? lastPoint?.[`${trader.trader_id}_pnl_pct`] || 0 : 0
                return (
                  <span style={{ color: '#EAECEF', fontSize: '12px', fontWeight: 500 }}>
                    {value}
                    <span style={{
                      color: pnl >= 0 ? '#0ECB81' : '#F6465D',
                      marginLeft: '6px',
                      fontFamily: 'monospace'
                    }}>
                      ({pnl >= 0 ? '+' : ''}{pnl.toFixed(2)}%)
                    </span>
                  </span>
                )
              }}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {/* Bottom Stats */}
      <div className="grid grid-cols-4 gap-2">
        <div className="p-3 rounded-lg text-center"
             style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}>
          <div className="text-[10px] uppercase tracking-wider mb-1" style={{ color: '#848E9C' }}>
            {t('leader', language)}
          </div>
          <div className="text-sm font-bold truncate" style={{ color: '#F0B90B' }}>
            {leader?.trader_name || '-'}
          </div>
        </div>
        <div className="p-3 rounded-lg text-center" style={{ background: 'rgba(14, 203, 129, 0.05)' }}>
          <div className="text-[10px] uppercase tracking-wider mb-1" style={{ color: '#848E9C' }}>
            {t('leadPnL', language) || 'Lead PnL'}
          </div>
          <div className="text-sm font-bold mono"
               style={{ color: (leader?.currentPnl || 0) >= 0 ? '#0ECB81' : '#F6465D' }}>
            {(leader?.currentPnl || 0) >= 0 ? '+' : ''}{(leader?.currentPnl || 0).toFixed(2)}%
          </div>
        </div>
        <div className="p-3 rounded-lg text-center" style={{ background: 'rgba(96, 165, 250, 0.05)' }}>
          <div className="text-[10px] uppercase tracking-wider mb-1" style={{ color: '#848E9C' }}>
            {t('currentGap', language)}
          </div>
          <div className="text-sm font-bold mono" style={{ color: '#60a5fa' }}>
            {gap}%
          </div>
        </div>
        <div className="p-3 rounded-lg text-center" style={{ background: 'rgba(139, 92, 246, 0.05)' }}>
          <div className="text-[10px] uppercase tracking-wider mb-1" style={{ color: '#848E9C' }}>
            {t('dataPoints', language)}
          </div>
          <div className="text-sm font-bold mono" style={{ color: '#8b5cf6' }}>
            {displayData.length}
          </div>
        </div>
      </div>
    </div>
  )
}
