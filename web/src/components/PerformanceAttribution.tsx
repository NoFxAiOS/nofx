import { useState } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
  PieChart,
  Pie,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import { AlertTriangle, TrendingUp, TrendingDown, Clock } from 'lucide-react'

interface PerformanceAttribution {
  by_symbol: {
    [symbol: string]: {
      total_trades: number
      winning_trades: number
      losing_trades: number
      win_rate: number
      total_pnl: number
      avg_pnl: number
      contribution_pct: number
    }
  }
  by_side: {
    long: {
      total_trades: number
      total_pnl: number
      win_rate: number
      contribution_pct: number
    }
    short: {
      total_trades: number
      total_pnl: number
      win_rate: number
      contribution_pct: number
    }
  }
  by_timeframe: {
    [timeframe: string]: {
      total_trades: number
      total_pnl: number
      avg_pnl: number
      contribution_pct: number
    }
  }
  summary: {
    total_trades: number
    total_pnl: number
    best_symbol: string
    worst_symbol: string
    best_timeframe: string
    profitable_symbols: number
    unprofitable_symbols: number
  }
  calculated_at: string
}

interface PerformanceAttributionProps {
  traderId: string
  lookbackDays?: number
}

export function PerformanceAttribution({
  traderId,
  lookbackDays = 30,
}: PerformanceAttributionProps) {
  const [view, setView] = useState<'symbol' | 'side' | 'timeframe'>('symbol')

  const { data: attribution, error } = useSWR<PerformanceAttribution>(
    traderId ? `attribution-${traderId}-${lookbackDays}` : null,
    () => api.getPerformanceAttribution(traderId, lookbackDays),
    {
      refreshInterval: 60000, // 60秒刷新
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
              Failed to load performance attribution
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!attribution) {
    return (
      <div className="binance-card p-6">
        <div className="animate-pulse space-y-4">
          <div className="skeleton h-6 w-64"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  // Check if no data available or incomplete data structure
  if (
    !attribution.summary ||
    !attribution.by_symbol ||
    !attribution.by_side ||
    !attribution.by_timeframe ||
    attribution.summary.total_trades === 0 ||
    Object.keys(attribution.by_symbol).length === 0
  ) {
    return (
      <div className="binance-card p-6">
        <h3 className="text-lg font-bold mb-4" style={{ color: '#EAECEF' }}>
          📊 Performance Attribution
        </h3>
        <div className="text-center py-12" style={{ color: '#848E9C' }}>
          <div className="text-6xl mb-4 opacity-30">📈</div>
          <div className="text-lg font-semibold mb-2">
            No Trade Data Available
          </div>
          <div className="text-sm">
            No completed trades found in the last {lookbackDays} days
          </div>
        </div>
      </div>
    )
  }

  // Prepare data for charts
  const symbolData = Object.entries(attribution.by_symbol).map(
    ([symbol, data]) => ({
      symbol,
      pnl: data.total_pnl,
      trades: data.total_trades,
      winRate: data.win_rate,
      contribution: data.contribution_pct,
    })
  )

  const sideData = [
    {
      name: 'Long',
      value: attribution.by_side.long.total_pnl,
      trades: attribution.by_side.long.total_trades,
      winRate: attribution.by_side.long.win_rate,
    },
    {
      name: 'Short',
      value: attribution.by_side.short.total_pnl,
      trades: attribution.by_side.short.total_trades,
      winRate: attribution.by_side.short.win_rate,
    },
  ]

  const timeframeData = Object.entries(attribution.by_timeframe).map(
    ([timeframe, data]) => ({
      timeframe,
      pnl: data.total_pnl,
      trades: data.total_trades,
      avgPnl: data.avg_pnl,
    })
  )

  return (
    <div className="binance-card p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            📊 Performance Attribution
          </h3>
          <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
            Last {lookbackDays} days • {attribution.summary.total_trades} trades
          </p>
        </div>

        {/* View Toggle */}
        <div
          className="flex gap-2 p-1 rounded"
          style={{ background: '#1E2329' }}
        >
          <button
            onClick={() => setView('symbol')}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              view === 'symbol' ? 'active-tab' : ''
            }`}
            style={
              view === 'symbol'
                ? { background: '#F0B90B', color: '#0B0E11' }
                : { color: '#848E9C' }
            }
          >
            By Symbol
          </button>
          <button
            onClick={() => setView('side')}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              view === 'side' ? 'active-tab' : ''
            }`}
            style={
              view === 'side'
                ? { background: '#F0B90B', color: '#0B0E11' }
                : { color: '#848E9C' }
            }
          >
            Long vs Short
          </button>
          <button
            onClick={() => setView('timeframe')}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              view === 'timeframe' ? 'active-tab' : ''
            }`}
            style={
              view === 'timeframe'
                ? { background: '#F0B90B', color: '#0B0E11' }
                : { color: '#848E9C' }
            }
          >
            By Timeframe
          </button>
        </div>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <div
          className="p-4 rounded"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Total P&L
          </div>
          <div
            className="text-2xl font-bold"
            style={{
              color:
                attribution.summary.total_pnl >= 0 ? '#0ECB81' : '#F6465D',
            }}
          >
            {attribution.summary.total_pnl >= 0 ? '+' : ''}
            {attribution.summary.total_pnl.toFixed(2)} USDT
          </div>
        </div>

        <div
          className="p-4 rounded"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Best Symbol
          </div>
          <div className="text-xl font-bold flex items-center gap-2">
            <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
            <span style={{ color: '#EAECEF' }}>
              {attribution.summary.best_symbol || 'N/A'}
            </span>
          </div>
        </div>

        <div
          className="p-4 rounded"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Worst Symbol
          </div>
          <div className="text-xl font-bold flex items-center gap-2">
            <TrendingDown className="w-5 h-5" style={{ color: '#F6465D' }} />
            <span style={{ color: '#EAECEF' }}>
              {attribution.summary.worst_symbol || 'N/A'}
            </span>
          </div>
        </div>

        <div
          className="p-4 rounded"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            Profitable Symbols
          </div>
          <div className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
            {attribution.summary.profitable_symbols} /{' '}
            {attribution.summary.profitable_symbols +
              attribution.summary.unprofitable_symbols}
          </div>
        </div>
      </div>

      {/* Chart Section */}
      <div className="mt-6">
        {view === 'symbol' && (
          <div>
            <h4
              className="text-sm font-semibold mb-4"
              style={{ color: '#EAECEF' }}
            >
              P&L by Symbol
            </h4>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={symbolData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
                <XAxis
                  dataKey="symbol"
                  stroke="#848E9C"
                  style={{ fontSize: '12px' }}
                />
                <YAxis stroke="#848E9C" style={{ fontSize: '12px' }} />
                <Tooltip
                  contentStyle={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    borderRadius: '8px',
                    color: '#EAECEF',
                  }}
                  formatter={(value: number, name: string) => {
                    if (name === 'pnl')
                      return [
                        `${value >= 0 ? '+' : ''}${value.toFixed(2)} USDT`,
                        'P&L',
                      ]
                    if (name === 'winRate')
                      return [`${value.toFixed(1)}%`, 'Win Rate']
                    return [value, name]
                  }}
                />
                <Bar dataKey="pnl" radius={[4, 4, 0, 0]}>
                  {symbolData.map((entry, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={entry.pnl >= 0 ? '#0ECB81' : '#F6465D'}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>

            {/* Symbol Details Table */}
            <div className="mt-6 overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b" style={{ borderColor: '#2B3139' }}>
                    <th
                      className="text-left py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Symbol
                    </th>
                    <th
                      className="text-right py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Trades
                    </th>
                    <th
                      className="text-right py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Win Rate
                    </th>
                    <th
                      className="text-right py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Total P&L
                    </th>
                    <th
                      className="text-right py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Avg P&L
                    </th>
                    <th
                      className="text-right py-3 px-2 font-semibold"
                      style={{ color: '#848E9C' }}
                    >
                      Contribution
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(attribution.by_symbol).map(
                    ([symbol, data]) => (
                      <tr
                        key={symbol}
                        className="border-b"
                        style={{ borderColor: '#2B3139' }}
                      >
                        <td
                          className="py-3 px-2 font-mono font-semibold"
                          style={{ color: '#EAECEF' }}
                        >
                          {symbol}
                        </td>
                        <td
                          className="text-right py-3 px-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {data.total_trades}
                        </td>
                        <td
                          className="text-right py-3 px-2"
                          style={{
                            color: data.win_rate >= 50 ? '#0ECB81' : '#F6465D',
                          }}
                        >
                          {data.win_rate.toFixed(1)}%
                        </td>
                        <td
                          className="text-right py-3 px-2 font-bold"
                          style={{
                            color: data.total_pnl >= 0 ? '#0ECB81' : '#F6465D',
                          }}
                        >
                          {data.total_pnl >= 0 ? '+' : ''}
                          {data.total_pnl.toFixed(2)}
                        </td>
                        <td
                          className="text-right py-3 px-2"
                          style={{
                            color: data.avg_pnl >= 0 ? '#0ECB81' : '#F6465D',
                          }}
                        >
                          {data.avg_pnl >= 0 ? '+' : ''}
                          {data.avg_pnl.toFixed(2)}
                        </td>
                        <td
                          className="text-right py-3 px-2"
                          style={{ color: '#848E9C' }}
                        >
                          {data.contribution_pct.toFixed(1)}%
                        </td>
                      </tr>
                    )
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {view === 'side' && (
          <div>
            <h4
              className="text-sm font-semibold mb-4"
              style={{ color: '#EAECEF' }}
            >
              Long vs Short Performance
            </h4>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Pie Chart */}
              <div>
                <ResponsiveContainer width="100%" height={300}>
                  <PieChart>
                    <Pie
                      data={sideData}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      outerRadius={100}
                      label={(entry) =>
                        `${entry.name}: ${entry.value >= 0 ? '+' : ''}${entry.value.toFixed(2)}`
                      }
                    >
                      <Cell fill="#0ECB81" />
                      <Cell fill="#F6465D" />
                    </Pie>
                    <Tooltip
                      contentStyle={{
                        background: '#1E2329',
                        border: '1px solid #2B3139',
                        borderRadius: '8px',
                        color: '#EAECEF',
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>
              </div>

              {/* Side Stats */}
              <div className="space-y-4">
                {/* Long Stats */}
                <div
                  className="p-4 rounded"
                  style={{
                    background: 'rgba(14, 203, 129, 0.1)',
                    border: '1px solid rgba(14, 203, 129, 0.2)',
                  }}
                >
                  <div
                    className="text-sm font-semibold mb-2"
                    style={{ color: '#0ECB81' }}
                  >
                    LONG Positions
                  </div>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Total Trades:</span>
                      <span style={{ color: '#EAECEF' }}>
                        {attribution.by_side.long.total_trades}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Win Rate:</span>
                      <span style={{ color: '#EAECEF' }}>
                        {attribution.by_side.long.win_rate.toFixed(1)}%
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Total P&L:</span>
                      <span
                        className="font-bold"
                        style={{
                          color:
                            attribution.by_side.long.total_pnl >= 0
                              ? '#0ECB81'
                              : '#F6465D',
                        }}
                      >
                        {attribution.by_side.long.total_pnl >= 0 ? '+' : ''}
                        {attribution.by_side.long.total_pnl.toFixed(2)} USDT
                      </span>
                    </div>
                  </div>
                </div>

                {/* Short Stats */}
                <div
                  className="p-4 rounded"
                  style={{
                    background: 'rgba(246, 70, 93, 0.1)',
                    border: '1px solid rgba(246, 70, 93, 0.2)',
                  }}
                >
                  <div
                    className="text-sm font-semibold mb-2"
                    style={{ color: '#F6465D' }}
                  >
                    SHORT Positions
                  </div>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Total Trades:</span>
                      <span style={{ color: '#EAECEF' }}>
                        {attribution.by_side.short.total_trades}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Win Rate:</span>
                      <span style={{ color: '#EAECEF' }}>
                        {attribution.by_side.short.win_rate.toFixed(1)}%
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: '#848E9C' }}>Total P&L:</span>
                      <span
                        className="font-bold"
                        style={{
                          color:
                            attribution.by_side.short.total_pnl >= 0
                              ? '#0ECB81'
                              : '#F6465D',
                        }}
                      >
                        {attribution.by_side.short.total_pnl >= 0 ? '+' : ''}
                        {attribution.by_side.short.total_pnl.toFixed(2)} USDT
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {view === 'timeframe' && (
          <div>
            <h4
              className="text-sm font-semibold mb-4"
              style={{ color: '#EAECEF' }}
            >
              Performance by Timeframe
            </h4>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={timeframeData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
                <XAxis
                  dataKey="timeframe"
                  stroke="#848E9C"
                  style={{ fontSize: '12px' }}
                />
                <YAxis stroke="#848E9C" style={{ fontSize: '12px' }} />
                <Tooltip
                  contentStyle={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    borderRadius: '8px',
                    color: '#EAECEF',
                  }}
                  formatter={(value: number) => [
                    `${value >= 0 ? '+' : ''}${value.toFixed(2)} USDT`,
                    'P&L',
                  ]}
                />
                <Bar dataKey="pnl" fill="#F0B90B" radius={[4, 4, 0, 0]}>
                  {timeframeData.map((entry, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={entry.pnl >= 0 ? '#0ECB81' : '#F6465D'}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>

      {/* Footer */}
      <div
        className="mt-6 pt-4 border-t text-xs flex items-center gap-2"
        style={{ borderColor: '#2B3139', color: '#848E9C' }}
      >
        <Clock className="w-4 h-4" />
        <span>
          Calculated at: {new Date(attribution.calculated_at).toLocaleString()}
        </span>
      </div>
    </div>
  )
}
