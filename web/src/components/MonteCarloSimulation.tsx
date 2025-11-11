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
import { api } from '../lib/api'
import { AlertTriangle, Play, TrendingUp, TrendingDown, Zap } from 'lucide-react'

interface MonteCarloResult {
  simulations: number
  time_horizon_days: number
  initial_balance: number
  percentiles: {
    p5: number
    p25: number
    p50: number
    p75: number
    p95: number
    mean: number
    std_dev: number
  }
  worst_case: {
    final_balance: number
    max_drawdown: number
    return_percent: number
  }
  best_case: {
    final_balance: number
    max_drawdown: number
    return_percent: number
  }
  median_case: {
    final_balance: number
    max_drawdown: number
    return_percent: number
  }
  probability_stats: {
    prob_profit: number
    prob_loss: number
    prob_above_10pct: number
    prob_above_20pct: number
    prob_below_10pct: number
    prob_below_20pct: number
    expected_return: number
    risk_of_ruin: number
  }
  calculated_at: string
}

interface MonteCarloSimulationProps {
  traderId: string
}

export function MonteCarloSimulation({ traderId }: MonteCarloSimulationProps) {
  const [simulations, setSimulations] = useState(1000)
  const [timeHorizon, setTimeHorizon] = useState(30)
  const [result, setResult] = useState<MonteCarloResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const runSimulation = async () => {
    setLoading(true)
    setError(null)

    try {
      const data = await api.runMonteCarloSimulation(traderId, {
        simulations,
        time_horizon_days: timeHorizon,
        include_paths: false,
      })
      setResult(data)
    } catch (err: any) {
      setError(err.message || 'Failed to run simulation')
    } finally {
      setLoading(false)
    }
  }

  // Prepare chart data for percentiles
  const percentileData = result
    ? [
        {
          name: 'P5',
          value: result.percentiles.p5,
          label: '5th %ile',
        },
        {
          name: 'P25',
          value: result.percentiles.p25,
          label: '25th %ile',
        },
        {
          name: 'P50',
          value: result.percentiles.p50,
          label: 'Median',
        },
        {
          name: 'P75',
          value: result.percentiles.p75,
          label: '75th %ile',
        },
        {
          name: 'P95',
          value: result.percentiles.p95,
          label: '95th %ile',
        },
      ]
    : []

  return (
    <div className="binance-card p-6 animate-fade-in">
      {/* Header */}
      <div className="mb-6">
        <h3 className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
          🎲 Monte Carlo Simulation
        </h3>
        <p className="text-sm" style={{ color: '#848E9C' }}>
          Forecast future performance using historical data and statistical modeling
        </p>
      </div>

      {/* Configuration */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div>
          <label className="text-xs font-semibold mb-2 block" style={{ color: '#848E9C' }}>
            NUMBER OF SIMULATIONS
          </label>
          <select
            value={simulations}
            onChange={(e) => setSimulations(Number(e.target.value))}
            className="w-full px-3 py-2 rounded text-sm font-mono"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          >
            <option value={100}>100 (Fast)</option>
            <option value={500}>500</option>
            <option value={1000}>1,000 (Recommended)</option>
            <option value={5000}>5,000 (Slow)</option>
            <option value={10000}>10,000 (Very Slow)</option>
          </select>
        </div>

        <div>
          <label className="text-xs font-semibold mb-2 block" style={{ color: '#848E9C' }}>
            TIME HORIZON (DAYS)
          </label>
          <select
            value={timeHorizon}
            onChange={(e) => setTimeHorizon(Number(e.target.value))}
            className="w-full px-3 py-2 rounded text-sm font-mono"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          >
            <option value={7}>7 days (1 week)</option>
            <option value={14}>14 days (2 weeks)</option>
            <option value={30}>30 days (1 month)</option>
            <option value={60}>60 days (2 months)</option>
            <option value={90}>90 days (3 months)</option>
            <option value={180}>180 days (6 months)</option>
            <option value={365}>365 days (1 year)</option>
          </select>
        </div>

        <div className="flex items-end">
          <button
            onClick={runSimulation}
            disabled={loading}
            className="w-full px-4 py-2 rounded font-bold text-sm flex items-center justify-center gap-2 transition-all"
            style={{
              background: loading ? '#474D57' : '#F0B90B',
              color: '#000',
              cursor: loading ? 'not-allowed' : 'pointer',
            }}
          >
            {loading ? (
              <>
                <div className="spinner"></div>
                Running...
              </>
            ) : (
              <>
                <Play className="w-4 h-4" />
                Run Simulation
              </>
            )}
          </button>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div
          className="flex items-center gap-3 p-4 rounded mb-6"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>
              Simulation Failed
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error}
            </div>
          </div>
        </div>
      )}

      {/* Results */}
      {result && (
        <div className="space-y-6">
          {/* Key Metrics */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div
              className="p-3 rounded"
              style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
            >
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                INITIAL BALANCE
              </div>
              <div className="text-lg font-bold mono" style={{ color: '#EAECEF' }}>
                ${result.initial_balance.toFixed(2)}
              </div>
            </div>

            <div
              className="p-3 rounded"
              style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.1)' }}
            >
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                EXPECTED RETURN
              </div>
              <div
                className="text-lg font-bold mono"
                style={{
                  color:
                    result.probability_stats.expected_return > 0
                      ? '#0ECB81'
                      : '#F6465D',
                }}
              >
                {result.probability_stats.expected_return > 0 ? '+' : ''}
                {result.probability_stats.expected_return.toFixed(2)}%
              </div>
            </div>

            <div
              className="p-3 rounded"
              style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
            >
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                PROFIT PROBABILITY
              </div>
              <div className="text-lg font-bold mono" style={{ color: '#0ECB81' }}>
                {result.probability_stats.prob_profit.toFixed(1)}%
              </div>
            </div>

            <div
              className="p-3 rounded"
              style={{ background: 'rgba(246, 70, 93, 0.05)', border: '1px solid rgba(246, 70, 93, 0.1)' }}
            >
              <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                RISK OF RUIN
              </div>
              <div
                className="text-lg font-bold mono"
                style={{
                  color:
                    result.probability_stats.risk_of_ruin > 20
                      ? '#F6465D'
                      : result.probability_stats.risk_of_ruin > 10
                      ? '#FF9800'
                      : '#0ECB81',
                }}
              >
                {result.probability_stats.risk_of_ruin.toFixed(1)}%
              </div>
            </div>
          </div>

          {/* Percentile Chart */}
          <div>
            <h4 className="text-sm font-bold mb-3" style={{ color: '#EAECEF' }}>
              Distribution of Final Balances (Percentiles)
            </h4>
            <ResponsiveContainer width="100%" height={250}>
              <LineChart
                data={percentileData}
                margin={{ top: 10, right: 20, left: 10, bottom: 20 }}
              >
                <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
                <XAxis
                  dataKey="label"
                  stroke="#5E6673"
                  tick={{ fill: '#848E9C', fontSize: 11 }}
                />
                <YAxis
                  stroke="#5E6673"
                  tick={{ fill: '#848E9C', fontSize: 12 }}
                  tickFormatter={(value) => `$${value.toFixed(0)}`}
                />
                <Tooltip
                  content={({ active, payload }) => {
                    if (active && payload && payload.length) {
                      const data = payload[0].payload
                      return (
                        <div
                          className="rounded p-3 shadow-xl"
                          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
                        >
                          <div className="text-sm font-bold" style={{ color: '#EAECEF' }}>
                            {data.label}
                          </div>
                          <div className="text-lg font-bold mono mt-1" style={{ color: '#F0B90B' }}>
                            ${data.value.toFixed(2)}
                          </div>
                          <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                            {result.initial_balance > 0 ? ((data.value / result.initial_balance - 1) * 100).toFixed(2) : '0.00'}% return
                          </div>
                        </div>
                      )
                    }
                    return null
                  }}
                />
                <ReferenceLine
                  y={result.initial_balance}
                  stroke="#474D57"
                  strokeDasharray="3 3"
                  label={{ value: 'Initial', fill: '#848E9C', fontSize: 11 }}
                />
                <Line
                  type="monotone"
                  dataKey="value"
                  stroke="#F0B90B"
                  strokeWidth={3}
                  dot={{ fill: '#F0B90B', r: 5 }}
                  activeDot={{ r: 7 }}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>

          {/* Scenarios */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Best Case */}
            <div
              className="p-4 rounded"
              style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.1)' }}
            >
              <div className="flex items-center gap-2 mb-3">
                <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
                <h4 className="font-bold" style={{ color: '#0ECB81' }}>
                  Best Case Scenario
                </h4>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Final Balance:</span>
                  <span className="font-bold mono" style={{ color: '#EAECEF' }}>
                    ${result.best_case.final_balance.toFixed(2)}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Return:</span>
                  <span className="font-bold mono" style={{ color: '#0ECB81' }}>
                    +{result.best_case.return_percent.toFixed(2)}%
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Max Drawdown:</span>
                  <span className="font-bold mono" style={{ color: '#F6465D' }}>
                    {result.best_case.max_drawdown.toFixed(2)}%
                  </span>
                </div>
              </div>
            </div>

            {/* Median Case */}
            <div
              className="p-4 rounded"
              style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
            >
              <div className="flex items-center gap-2 mb-3">
                <Zap className="w-5 h-5" style={{ color: '#F0B90B' }} />
                <h4 className="font-bold" style={{ color: '#F0B90B' }}>
                  Median Case Scenario
                </h4>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Final Balance:</span>
                  <span className="font-bold mono" style={{ color: '#EAECEF' }}>
                    ${result.median_case.final_balance.toFixed(2)}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Return:</span>
                  <span
                    className="font-bold mono"
                    style={{
                      color:
                        result.median_case.return_percent > 0 ? '#0ECB81' : '#F6465D',
                    }}
                  >
                    {result.median_case.return_percent > 0 ? '+' : ''}
                    {result.median_case.return_percent.toFixed(2)}%
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Max Drawdown:</span>
                  <span className="font-bold mono" style={{ color: '#F6465D' }}>
                    {result.median_case.max_drawdown.toFixed(2)}%
                  </span>
                </div>
              </div>
            </div>

            {/* Worst Case */}
            <div
              className="p-4 rounded"
              style={{ background: 'rgba(246, 70, 93, 0.05)', border: '1px solid rgba(246, 70, 93, 0.1)' }}
            >
              <div className="flex items-center gap-2 mb-3">
                <TrendingDown className="w-5 h-5" style={{ color: '#F6465D' }} />
                <h4 className="font-bold" style={{ color: '#F6465D' }}>
                  Worst Case Scenario
                </h4>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Final Balance:</span>
                  <span className="font-bold mono" style={{ color: '#EAECEF' }}>
                    ${result.worst_case.final_balance.toFixed(2)}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Return:</span>
                  <span className="font-bold mono" style={{ color: '#F6465D' }}>
                    {result.worst_case.return_percent.toFixed(2)}%
                  </span>
                </div>
                <div className="flex justify-between">
                  <span style={{ color: '#848E9C' }}>Max Drawdown:</span>
                  <span className="font-bold mono" style={{ color: '#F6465D' }}>
                    {result.worst_case.max_drawdown.toFixed(2)}%
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* Probability Details */}
          <div
            className="p-4 rounded"
            style={{ background: 'rgba(240, 185, 11, 0.03)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
          >
            <h4 className="text-sm font-bold mb-3" style={{ color: '#EAECEF' }}>
              Probability Breakdown
            </h4>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Profit:</span>
                <span className="font-bold" style={{ color: '#0ECB81' }}>
                  {result.probability_stats.prob_profit.toFixed(1)}%
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Loss:</span>
                <span className="font-bold" style={{ color: '#F6465D' }}>
                  {result.probability_stats.prob_loss.toFixed(1)}%
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Return &gt; 10%:</span>
                <span className="font-bold" style={{ color: '#0ECB81' }}>
                  {result.probability_stats.prob_above_10pct.toFixed(1)}%
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Return &gt; 20%:</span>
                <span className="font-bold" style={{ color: '#0ECB81' }}>
                  {result.probability_stats.prob_above_20pct.toFixed(1)}%
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Loss &gt; 10%:</span>
                <span className="font-bold" style={{ color: '#FF9800' }}>
                  {result.probability_stats.prob_below_10pct.toFixed(1)}%
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#848E9C' }}>Loss &gt; 20%:</span>
                <span className="font-bold" style={{ color: '#F6465D' }}>
                  {result.probability_stats.prob_below_20pct.toFixed(1)}%
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
