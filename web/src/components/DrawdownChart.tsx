import { useState } from 'react'
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import { AlertTriangle, TrendingDown, Clock, Target } from 'lucide-react'

interface DrawdownAnalysis {
  max_drawdown: number
  max_drawdown_dollar: number
  current_drawdown: number
  drawdown_periods: Array<{
    start_time: string
    end_time: string
    recovery_time?: string
    peak_equity: number
    trough_equity: number
    drawdown_percent: number
    drawdown_dollar: number
    duration_minutes: number
    recovery_duration_minutes?: number
    is_recovered: boolean
  }>
  recovery_stats: {
    total_drawdowns: number
    recovered_drawdowns: number
    recovery_rate: number
    avg_recovery_time_hours: number
    longest_recovery_time_hours: number
  }
  drawdown_series: Array<{
    timestamp: string
    equity: number
    peak_equity: number
    drawdown_percent: number
    drawdown_dollar: number
    cycle_number: number
  }>
  calculated_at: string
}

interface DrawdownChartProps {
  traderId: string
}

export function DrawdownChart({ traderId }: DrawdownChartProps) {
  const [displayMode, setDisplayMode] = useState<'percent' | 'dollar'>('percent')

  const { data: drawdown, error } = useSWR<DrawdownAnalysis>(
    traderId ? `drawdown-${traderId}` : null,
    () => api.getDrawdownAnalysis(traderId),
    {
      refreshInterval: 30000, // 30秒刷新
      revalidateOnFocus: false,
    }
  )

  if (error) {
    return (
      <div className="binance-card p-6">
        <div
          className="flex items-center gap-3 p-4 rounded"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>
              Failed to load drawdown data
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!drawdown) {
    return (
      <div className="binance-card p-6">
        <div className="animate-pulse space-y-4">
          <div className="skeleton h-6 w-48"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  // 转换数据格式
  const chartData = drawdown.drawdown_series.map((point) => ({
    time: new Date(point.timestamp).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    }),
    equity: point.equity,
    peak: point.peak_equity,
    drawdownPercent: -point.drawdown_percent, // 负数表示回撤
    drawdownDollar: -point.drawdown_dollar,
    cycle: point.cycle_number,
  }))

  // 自定义Tooltip
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="rounded p-3 shadow-xl"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Cycle #{data.cycle}
          </div>
          <div className="font-bold mono" style={{ color: '#EAECEF' }}>
            Equity: ${data.equity.toFixed(2)}
          </div>
          <div className="text-sm" style={{ color: '#848E9C' }}>
            Peak: ${data.peak.toFixed(2)}
          </div>
          <div
            className="text-sm mono font-bold mt-1"
            style={{ color: '#F6465D' }}
          >
            Drawdown: {Math.abs(data.drawdownPercent).toFixed(2)}%
            <span className="text-xs ml-1">
              (${Math.abs(data.drawdownDollar).toFixed(2)})
            </span>
          </div>
        </div>
      )
    }
    return null
  }

  // 获取最严重的5个回撤周期
  const worstDrawdowns = [...drawdown.drawdown_periods]
    .sort((a, b) => b.drawdown_percent - a.drawdown_percent)
    .slice(0, 5)

  return (
    <div className="binance-card p-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 mb-6">
        <div>
          <h3 className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
            📉 Drawdown Analysis
          </h3>
          <div className="flex flex-wrap gap-4 text-sm">
            <div>
              <span style={{ color: '#848E9C' }}>Max Drawdown: </span>
              <span
                className="font-bold"
                style={{
                  color:
                    drawdown.max_drawdown > 20
                      ? '#F6465D'
                      : drawdown.max_drawdown > 10
                      ? '#FF9800'
                      : '#0ECB81',
                }}
              >
                {drawdown.max_drawdown.toFixed(2)}%
              </span>
              <span className="ml-1" style={{ color: '#848E9C' }}>
                (${drawdown.max_drawdown_dollar.toFixed(2)})
              </span>
            </div>
            <div>
              <span style={{ color: '#848E9C' }}>Current: </span>
              <span
                className="font-bold"
                style={{
                  color:
                    drawdown.current_drawdown > 5 ? '#F6465D' : '#0ECB81',
                }}
              >
                {drawdown.current_drawdown.toFixed(2)}%
              </span>
            </div>
          </div>
        </div>

        {/* Display Mode Toggle */}
        <div
          className="flex gap-1 rounded p-1"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          <button
            onClick={() => setDisplayMode('percent')}
            className="px-4 py-2 rounded text-sm font-bold transition-all"
            style={
              displayMode === 'percent'
                ? {
                    background: '#F0B90B',
                    color: '#000',
                    boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            Percent (%)
          </button>
          <button
            onClick={() => setDisplayMode('dollar')}
            className="px-4 py-2 rounded text-sm font-bold transition-all"
            style={
              displayMode === 'dollar'
                ? {
                    background: '#F0B90B',
                    color: '#000',
                    boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            Dollar ($)
          </button>
        </div>
      </div>

      {/* Chart */}
      <div className="my-4" style={{ borderRadius: '8px', overflow: 'hidden' }}>
        <ResponsiveContainer width="100%" height={320}>
          <AreaChart
            data={chartData}
            margin={{ top: 10, right: 20, left: 10, bottom: 30 }}
          >
            <defs>
              <linearGradient id="drawdownGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#F6465D" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#F6465D" stopOpacity={0.1} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
            <XAxis
              dataKey="time"
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 11 }}
              tickLine={{ stroke: '#2B3139' }}
              interval={Math.floor(chartData.length / 10)}
              angle={-15}
              textAnchor="end"
              height={60}
            />
            <YAxis
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 12 }}
              tickLine={{ stroke: '#2B3139' }}
              tickFormatter={(value) =>
                displayMode === 'percent' ? `${value}%` : `$${value}`
              }
            />
            <Tooltip content={<CustomTooltip />} />
            <ReferenceLine
              y={0}
              stroke="#474D57"
              strokeDasharray="3 3"
              label={{ value: 'No Drawdown', fill: '#848E9C', fontSize: 12 }}
            />
            <Area
              type="monotone"
              dataKey={
                displayMode === 'percent' ? 'drawdownPercent' : 'drawdownDollar'
              }
              stroke="#F6465D"
              strokeWidth={2}
              fill="url(#drawdownGradient)"
              connectNulls
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      {/* Recovery Statistics */}
      <div
        className="grid grid-cols-2 md:grid-cols-4 gap-3 p-4 rounded mb-4"
        style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
      >
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            TOTAL DRAWDOWNS
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#EAECEF' }}>
            {drawdown.recovery_stats.total_drawdowns}
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            RECOVERY RATE
          </div>
          <div
            className="text-lg font-bold mono"
            style={{
              color:
                drawdown.recovery_stats.recovery_rate > 80
                  ? '#0ECB81'
                  : drawdown.recovery_stats.recovery_rate > 50
                  ? '#FF9800'
                  : '#F6465D',
            }}
          >
            {drawdown.recovery_stats.recovery_rate.toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            AVG RECOVERY TIME
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#EAECEF' }}>
            {drawdown.recovery_stats.avg_recovery_time_hours.toFixed(1)}h
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            LONGEST RECOVERY
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#F6465D' }}>
            {drawdown.recovery_stats.longest_recovery_time_hours.toFixed(1)}h
          </div>
        </div>
      </div>

      {/* Worst Drawdown Periods */}
      <div>
        <h4
          className="text-sm font-bold mb-3 flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <TrendingDown className="w-4 h-4" style={{ color: '#F6465D' }} />
          Top 5 Worst Drawdown Periods
        </h4>
        <div className="space-y-2">
          {worstDrawdowns.map((period, idx) => (
            <div
              key={idx}
              className="p-3 rounded"
              style={{
                background: 'rgba(246, 70, 93, 0.05)',
                border: '1px solid rgba(246, 70, 93, 0.1)',
              }}
            >
              <div className="flex justify-between items-start mb-2">
                <div>
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    #{idx + 1}
                  </span>
                  <span
                    className="ml-2 text-sm font-bold"
                    style={{ color: '#F6465D' }}
                  >
                    {period.drawdown_percent.toFixed(2)}%
                  </span>
                  <span className="ml-1 text-xs" style={{ color: '#848E9C' }}>
                    (${period.drawdown_dollar.toFixed(2)})
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  {period.is_recovered ? (
                    <span
                      className="text-xs px-2 py-1 rounded"
                      style={{
                        background: 'rgba(14, 203, 129, 0.1)',
                        color: '#0ECB81',
                      }}
                    >
                      ✓ Recovered
                    </span>
                  ) : (
                    <span
                      className="text-xs px-2 py-1 rounded"
                      style={{
                        background: 'rgba(255, 152, 0, 0.1)',
                        color: '#FF9800',
                      }}
                    >
                      ⚠ In Progress
                    </span>
                  )}
                </div>
              </div>
              <div className="flex gap-4 text-xs" style={{ color: '#848E9C' }}>
                <div className="flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  Duration: {(period.duration_minutes / 60).toFixed(1)}h
                </div>
                {period.recovery_duration_minutes && (
                  <div className="flex items-center gap-1">
                    <Target className="w-3 h-3" />
                    Recovery: {(period.recovery_duration_minutes / 60).toFixed(1)}h
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
