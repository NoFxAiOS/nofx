import { useState } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../../lib/api'
import { useLanguage } from '../../contexts/LanguageContext'
import { useAuth } from '../../contexts/AuthContext'
import { t } from '../../i18n/translations'
import {
  AlertTriangle,
  BarChart3,
  DollarSign,
  Percent,
  TrendingUp as ArrowUp,
  TrendingDown as ArrowDown,
} from 'lucide-react'

interface EquityPoint {
  timestamp: string
  total_equity: number
  pnl: number
  pnl_pct: number
  cycle_number: number
}

interface EquityChartProps {
  traderId?: string
  embedded?: boolean // Embedded mode (does not show the outer card)
}

export function EquityChart({ traderId, embedded = false }: EquityChartProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const [displayMode, setDisplayMode] = useState<'dollar' | 'percent'>('dollar')

  const { data: history, error, isLoading } = useSWR<EquityPoint[]>(
    user && token && traderId ? `equity-history-${traderId}` : null,
    () => api.getEquityHistory(traderId, true),
    {
      refreshInterval: 30000, // Refresh every 30s (historical data updates less frequently)
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: account } = useSWR(
    user && token && traderId ? `account-${traderId}` : null,
    () => api.getAccount(traderId, true),
    {
      refreshInterval: 15000, // Refresh every 15s (matches backend cache)
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  // Loading state - show skeleton
  if (isLoading) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3 className="text-lg font-semibold mb-6" style={{ color: '#1A1813' }}>
            {t('accountEquityCurve', language)}
          </h3>
        )}
        <div className="animate-pulse">
          <div className="skeleton h-64 w-full rounded"></div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        <div
          className="flex items-center gap-3 p-4 rounded"
          style={{
            background: 'rgba(214, 67, 58, 0.1)',
            border: '1px solid rgba(214, 67, 58, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#D6433A' }} />
          <div>
            <div className="font-semibold" style={{ color: '#D6433A' }}>
              {t('loadingError', language)}
            </div>
            <div className="text-sm" style={{ color: '#8A8478' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Filter out invalid data: points where total_equity is 0 or less than 1 (caused by API failures)
  const validHistory = history?.filter((point) => point.total_equity > 1) || []

  if (!validHistory || validHistory.length === 0) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3 className="text-lg font-semibold mb-6" style={{ color: '#1A1813' }}>
            {t('accountEquityCurve', language)}
          </h3>
        )}
        <div className="text-center py-16" style={{ color: '#8A8478' }}>
          <div className="mb-4 flex justify-center opacity-50">
            <BarChart3 className="w-16 h-16" />
          </div>
          <div className="text-lg font-semibold mb-2">
            {t('noHistoricalData', language)}
          </div>
          <div className="text-sm">{t('dataWillAppear', language)}</div>
        </div>
      </div>
    )
  }

  // Limit to the most recent data points (performance optimization)
  // If there are more than 2000 points, only show the most recent 2000
  const MAX_DISPLAY_POINTS = 2000
  const displayHistory =
    validHistory.length > MAX_DISPLAY_POINTS
      ? validHistory.slice(-MAX_DISPLAY_POINTS)
      : validHistory

  // Compute the initial balance (prefer the configured value from account, fall back to deriving from history)
  const initialBalance =
    account?.initial_balance || // Read the real initial balance from the trader config
    (validHistory[0]
      ? validHistory[0].total_equity - validHistory[0].pnl
      : undefined) || // Fallback: equity - pnl
    1000 // Default value (matches the default config used when creating a trader)

  // Transform the data format
  const chartData = displayHistory.map((point, index) => {
    const pnl = point.total_equity - initialBalance
    const pnlPct = ((pnl / initialBalance) * 100).toFixed(2)
    return {
      time: new Date(point.timestamp).toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
      }),
      value: displayMode === 'dollar' ? point.total_equity : parseFloat(pnlPct),
      cycle: point.cycle_number ?? index + 1,
      raw_equity: point.total_equity,
      raw_pnl: pnl,
      raw_pnl_pct: parseFloat(pnlPct),
    }
  })

  const currentValue = chartData[chartData.length - 1]
  const isProfit = currentValue.raw_pnl >= 0

  // Compute the Y-axis range
  const calculateYDomain = () => {
    if (displayMode === 'percent') {
      // Percent mode: find the min/max values, leave a 20% margin
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values)
      const maxVal = Math.max(...values)
      const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
      const padding = Math.max(range * 0.2, 1) // Leave at least a 1% margin
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    } else {
      // Dollar mode: anchor on the initial balance, leave a 10% margin above and below
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values, initialBalance)
      const maxVal = Math.max(...values, initialBalance)
      const range = maxVal - minVal
      const padding = Math.max(range * 0.15, initialBalance * 0.01) // Leave at least a 1% margin
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    }
  }

  // Custom Tooltip - Binance Style
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="rounded p-3 shadow-xl"
          style={{ background: '#F7F4EC', border: '1px solid rgba(26, 24, 19, 0.14)' }}
        >
          <div className="text-xs mb-1" style={{ color: '#8A8478' }}>
            Cycle #{data.cycle != null ? data.cycle : '—'}
          </div>
          <div className="font-bold mono" style={{ color: '#1A1813' }}>
            {data.raw_equity.toFixed(2)} USDT
          </div>
          <div
            className="text-sm mono font-bold"
            style={{ color: data.raw_pnl >= 0 ? '#2E8B57' : '#D6433A' }}
          >
            {data.raw_pnl >= 0 ? '+' : ''}
            {data.raw_pnl.toFixed(2)} USDT ({data.raw_pnl_pct >= 0 ? '+' : ''}
            {data.raw_pnl_pct}%)
          </div>
        </div>
      )
    }
    return null
  }

  return (
    <div className={embedded ? 'p-3 sm:p-5' : 'binance-card p-3 sm:p-5 animate-fade-in'}>
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
        <div className="flex-1">
          {!embedded && (
            <h3
              className="text-base sm:text-lg font-bold mb-2"
              style={{ color: '#1A1813' }}
            >
              {t('accountEquityCurve', language)}
            </h3>
          )}
          <div className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4">
            <span
              className="text-2xl sm:text-3xl font-bold mono"
              style={{ color: '#1A1813' }}
            >
              {account?.total_equity.toFixed(2) || '0.00'}
              <span
                className="text-base sm:text-lg ml-1"
                style={{ color: '#8A8478' }}
              >
                USDT
              </span>
            </span>
            <div className="flex items-center gap-2 flex-wrap">
              <span
                className="text-sm sm:text-lg font-bold mono px-2 sm:px-3 py-1 rounded flex items-center gap-1"
                style={{
                  color: isProfit ? '#2E8B57' : '#D6433A',
                  background: isProfit
                    ? 'rgba(46, 139, 87, 0.1)'
                    : 'rgba(214, 67, 58, 0.1)',
                  border: `1px solid ${
                    isProfit
                      ? 'rgba(46, 139, 87, 0.2)'
                      : 'rgba(214, 67, 58, 0.2)'
                  }`,
                }}
              >
                {isProfit ? (
                  <ArrowUp className="w-4 h-4" />
                ) : (
                  <ArrowDown className="w-4 h-4" />
                )}
                {isProfit ? '+' : ''}
                {currentValue.raw_pnl_pct}%
              </span>
              <span
                className="text-xs sm:text-sm mono"
                style={{ color: '#8A8478' }}
              >
                ({isProfit ? '+' : ''}
                {currentValue.raw_pnl.toFixed(2)} USDT)
              </span>
            </div>
          </div>
        </div>

        {/* Display Mode Toggle */}
        <div
          className="flex gap-0.5 sm:gap-1 rounded p-0.5 sm:p-1 self-start sm:self-auto"
          style={{ background: '#E8E2D5', border: '1px solid rgba(26, 24, 19, 0.14)' }}
        >
          <button
            onClick={() => setDisplayMode('dollar')}
            className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
            style={
              displayMode === 'dollar'
                ? {
                    background: '#E0483B',
                    color: '#F1ECE2',
                  }
                : { background: 'transparent', color: '#8A8478' }
            }
          >
            <DollarSign className="w-4 h-4" /> USDT
          </button>
          <button
            onClick={() => setDisplayMode('percent')}
            className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
            style={
              displayMode === 'percent'
                ? {
                    background: '#E0483B',
                    color: '#F1ECE2',
                  }
                : { background: 'transparent', color: '#8A8478' }
            }
          >
            <Percent className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Chart */}
      <div
        className="my-2"
        style={{
          borderRadius: '8px',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        {/* NOFX Watermark */}
        <div
          style={{
            position: 'absolute',
            top: '15px',
            right: '15px',
            fontSize: '20px',
            fontWeight: 'bold',
            color: 'rgba(224, 72, 59, 0.15)',
            zIndex: 10,
            pointerEvents: 'none',
            fontFamily: 'monospace',
          }}
        >
          NOFX
        </div>
        <ResponsiveContainer width="100%" height={280}>
          <LineChart
            data={chartData}
            margin={{ top: 10, right: 20, left: 5, bottom: 30 }}
          >
            <defs>
              <linearGradient id="colorGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#E0483B" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#E0483B" stopOpacity={0.2} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(26, 24, 19, 0.10)" />
            <XAxis
              dataKey="time"
              stroke="#6B6557"
              tick={{ fill: '#6B6557', fontSize: 11 }}
              tickLine={{ stroke: 'rgba(26, 24, 19, 0.14)' }}
              interval={Math.floor(chartData.length / 10)}
              angle={-15}
              textAnchor="end"
              height={60}
            />
            <YAxis
              stroke="#6B6557"
              tick={{ fill: '#6B6557', fontSize: 12 }}
              tickLine={{ stroke: 'rgba(26, 24, 19, 0.14)' }}
              domain={calculateYDomain()}
              tickFormatter={(value) =>
                displayMode === 'dollar' ? `$${value.toFixed(0)}` : `${value}%`
              }
            />
            <Tooltip content={<CustomTooltip />} />
            <ReferenceLine
              y={displayMode === 'dollar' ? initialBalance : 0}
              stroke="rgba(26, 24, 19, 0.2)"
              strokeDasharray="3 3"
              label={{
                value:
                  displayMode === 'dollar'
                    ? t('initialBalance', language).split(' ')[0]
                    : '0%',
                fill: '#8A8478',
                fontSize: 12,
              }}
            />
            <Line
              type="natural"
              dataKey="value"
              stroke="url(#colorGradient)"
              strokeWidth={3}
              dot={chartData.length > 50 ? false : { fill: '#E0483B', r: 3 }}
              activeDot={{
                r: 6,
                fill: '#E0483B',
                stroke: '#F1ECE2',
                strokeWidth: 2,
              }}
              connectNulls={true}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Footer Stats */}
      <div
        className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3 pt-3"
        style={{ borderTop: '1px solid rgba(26, 24, 19, 0.14)' }}
      >
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(224, 72, 59, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#8A8478' }}
          >
            {t('initialBalance', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#1A1813' }}
          >
            {initialBalance.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(224, 72, 59, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#8A8478' }}
          >
            {t('currentEquity', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#1A1813' }}
          >
            {currentValue.raw_equity.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(224, 72, 59, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#8A8478' }}
          >
            {t('historicalCycles', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#1A1813' }}
          >
            {validHistory.length} {t('cycles', language)}
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(224, 72, 59, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: '#8A8478' }}
          >
            {t('displayRange', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: '#1A1813' }}
          >
            {validHistory.length > MAX_DISPLAY_POINTS
              ? `${t('recent', language)} ${MAX_DISPLAY_POINTS}`
              : t('allData', language)}
          </div>
        </div>
      </div>
    </div>
  )
}
