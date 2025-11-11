import { useState } from 'react'
import {
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
import { AlertTriangle, TrendingUp, TrendingDown, Activity } from 'lucide-react'

interface OrderBookData {
  symbol: string
  bids: Array<{ price: number; quantity: number; total: number }>
  asks: Array<{ price: number; quantity: number; total: number }>
  last_update_id: number
  timestamp: string
  stats: {
    best_bid: number
    best_ask: number
    spread: number
    spread_percent: number
    mid_price: number
    bid_depth_10: number
    ask_depth_10: number
    total_bid_volume: number
    total_ask_volume: number
    volume_imbalance: number
    liquidity_score: number
    support_level: number
    resistance_level: number
  }
}

interface DepthChartData {
  bid_levels: Array<{
    price: number
    quantity: number
    cumulative_qty: number
    cumulative_value: number
  }>
  ask_levels: Array<{
    price: number
    quantity: number
    cumulative_qty: number
    cumulative_value: number
  }>
}

interface OrderBookDepthProps {
  symbol: string
  maxLevels?: number
}

export function OrderBookDepth({ symbol, maxLevels = 50 }: OrderBookDepthProps) {
  const [refreshInterval, setRefreshInterval] = useState(5000) // 5秒默认

  const { data: orderBook, error: obError } = useSWR<OrderBookData>(
    symbol ? `orderbook-${symbol}` : null,
    () => api.getOrderBook(symbol, 50),
    {
      refreshInterval,
      revalidateOnFocus: false,
    }
  )

  const { data: depthChart, error: dcError } = useSWR<DepthChartData>(
    symbol ? `orderbook-depth-${symbol}` : null,
    () => api.getOrderBookDepthChart(symbol, maxLevels),
    {
      refreshInterval,
      revalidateOnFocus: false,
    }
  )

  const error = obError || dcError

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
              Failed to load order book data
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!orderBook || !depthChart) {
    return (
      <div className="binance-card p-6">
        <div className="animate-pulse space-y-4">
          <div className="skeleton h-6 w-48"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  // 合并bid和ask数据用于深度图
  const chartData = [
    ...depthChart.bid_levels.reverse().map((level) => ({
      price: level.price,
      bidCumulative: level.cumulative_value,
      askCumulative: 0,
      side: 'bid',
    })),
    ...depthChart.ask_levels.map((level) => ({
      price: level.price,
      bidCumulative: 0,
      askCumulative: level.cumulative_value,
      side: 'ask',
    })),
  ].sort((a, b) => a.price - b.price)

  // 填充累计值（让图表连续）
  let lastBidCum = 0
  let lastAskCum = 0
  chartData.forEach((point) => {
    if (point.bidCumulative > 0) {
      lastBidCum = point.bidCumulative
    } else {
      point.bidCumulative = lastBidCum
    }

    if (point.askCumulative > 0) {
      lastAskCum = point.askCumulative
    } else if (point.side === 'ask') {
      point.askCumulative = lastAskCum
    }
  })

  const getImbalanceSignal = (imbalance: number) => {
    if (imbalance > 0.2) return { text: 'Strong Buy Pressure', color: '#0ECB81' }
    if (imbalance > 0.1) return { text: 'Buy Pressure', color: '#4CAF50' }
    if (imbalance < -0.2) return { text: 'Strong Sell Pressure', color: '#F6465D' }
    if (imbalance < -0.1) return { text: 'Sell Pressure', color: '#FF9800' }
    return { text: 'Balanced', color: '#848E9C' }
  }

  const imbalanceSignal = getImbalanceSignal(orderBook.stats.volume_imbalance)

  return (
    <div className="binance-card p-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 mb-6">
        <div>
          <h3 className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
            📚 Order Book Depth - {symbol}
          </h3>
          <div className="flex flex-wrap gap-4 text-sm">
            <div>
              <span style={{ color: '#848E9C' }}>Mid Price: </span>
              <span className="font-bold mono" style={{ color: '#F0B90B' }}>
                ${orderBook.stats.mid_price.toFixed(2)}
              </span>
            </div>
            <div>
              <span style={{ color: '#848E9C' }}>Spread: </span>
              <span className="font-bold" style={{ color: '#848E9C' }}>
                ${orderBook.stats.spread.toFixed(2)} ({orderBook.stats.spread_percent.toFixed(3)}%)
              </span>
            </div>
          </div>
        </div>

        {/* Refresh Interval Selector */}
        <div>
          <select
            value={refreshInterval}
            onChange={(e) => setRefreshInterval(Number(e.target.value))}
            className="px-3 py-2 rounded text-xs"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          >
            <option value={1000}>1s refresh</option>
            <option value={2000}>2s refresh</option>
            <option value={5000}>5s refresh</option>
            <option value={10000}>10s refresh</option>
          </select>
        </div>
      </div>

      {/* Depth Chart */}
      <div className="mb-6" style={{ borderRadius: '8px', overflow: 'hidden' }}>
        <ResponsiveContainer width="100%" height={350}>
          <AreaChart
            data={chartData}
            margin={{ top: 10, right: 20, left: 10, bottom: 30 }}
          >
            <defs>
              <linearGradient id="bidGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#0ECB81" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#0ECB81" stopOpacity={0.1} />
              </linearGradient>
              <linearGradient id="askGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#F6465D" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#F6465D" stopOpacity={0.1} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#2B3139" />
            <XAxis
              dataKey="price"
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 11 }}
              tickFormatter={(value) => `$${value.toFixed(2)}`}
              domain={['dataMin', 'dataMax']}
            />
            <YAxis
              stroke="#5E6673"
              tick={{ fill: '#848E9C', fontSize: 12 }}
              tickFormatter={(value) => `$${(value / 1000).toFixed(0)}K`}
            />
            <Tooltip
              content={({ active, payload }) => {
                if (active && payload && payload.length) {
                  const data = payload[0].payload
                  const isBid = data.bidCumulative > 0 && data.askCumulative === 0
                  return (
                    <div
                      className="rounded p-3 shadow-xl"
                      style={{ background: '#1E2329', border: '1px solid #2B3139' }}
                    >
                      <div className="text-sm font-bold" style={{ color: '#EAECEF' }}>
                        Price: ${data.price.toFixed(2)}
                      </div>
                      <div
                        className="text-sm font-bold mono mt-1"
                        style={{ color: isBid ? '#0ECB81' : '#F6465D' }}
                      >
                        {isBid ? 'Bid' : 'Ask'} Depth: $
                        {(isBid ? data.bidCumulative : data.askCumulative).toFixed(2)}
                      </div>
                    </div>
                  )
                }
                return null
              }}
            />
            <ReferenceLine
              x={orderBook.stats.mid_price}
              stroke="#F0B90B"
              strokeWidth={2}
              strokeDasharray="5 5"
              label={{
                value: `Mid: $${orderBook.stats.mid_price.toFixed(2)}`,
                fill: '#F0B90B',
                fontSize: 11,
                position: 'top',
              }}
            />
            <Area
              type="stepAfter"
              dataKey="bidCumulative"
              stroke="#0ECB81"
              strokeWidth={2}
              fill="url(#bidGradient)"
            />
            <Area
              type="stepAfter"
              dataKey="askCumulative"
              stroke="#F6465D"
              strokeWidth={2}
              fill="url(#askGradient)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      {/* Statistics Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        <div
          className="p-3 rounded"
          style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.1)' }}
        >
          <div className="flex items-center gap-2 mb-1">
            <TrendingUp className="w-4 h-4" style={{ color: '#0ECB81' }} />
            <div className="text-xs" style={{ color: '#848E9C' }}>
              BID DEPTH (10)
            </div>
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#0ECB81' }}>
            ${(orderBook.stats.bid_depth_10 / 1000).toFixed(1)}K
          </div>
        </div>

        <div
          className="p-3 rounded"
          style={{ background: 'rgba(246, 70, 93, 0.05)', border: '1px solid rgba(246, 70, 93, 0.1)' }}
        >
          <div className="flex items-center gap-2 mb-1">
            <TrendingDown className="w-4 h-4" style={{ color: '#F6465D' }} />
            <div className="text-xs" style={{ color: '#848E9C' }}>
              ASK DEPTH (10)
            </div>
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#F6465D' }}>
            ${(orderBook.stats.ask_depth_10 / 1000).toFixed(1)}K
          </div>
        </div>

        <div
          className="p-3 rounded"
          style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
        >
          <div className="flex items-center gap-2 mb-1">
            <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
            <div className="text-xs" style={{ color: '#848E9C' }}>
              LIQUIDITY SCORE
            </div>
          </div>
          <div className="text-lg font-bold mono" style={{ color: '#F0B90B' }}>
            ${(orderBook.stats.liquidity_score / 1000).toFixed(1)}K
          </div>
        </div>

        <div
          className="p-3 rounded"
          style={{
            background: `rgba(${imbalanceSignal.color === '#0ECB81' ? '14, 203, 129' : imbalanceSignal.color === '#F6465D' ? '246, 70, 93' : '132, 142, 156'}, 0.05)`,
            border: `1px solid rgba(${imbalanceSignal.color === '#0ECB81' ? '14, 203, 129' : imbalanceSignal.color === '#F6465D' ? '246, 70, 93' : '132, 142, 156'}, 0.1)`,
          }}
        >
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            IMBALANCE
          </div>
          <div
            className="text-sm font-bold"
            style={{ color: imbalanceSignal.color }}
          >
            {imbalanceSignal.text}
          </div>
          <div className="text-xs mono" style={{ color: '#848E9C' }}>
            {(orderBook.stats.volume_imbalance * 100).toFixed(1)}%
          </div>
        </div>
      </div>

      {/* Support & Resistance */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          className="p-4 rounded"
          style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.1)' }}
        >
          <div className="flex items-center gap-2 mb-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ background: '#0ECB81' }}
            ></div>
            <h4 className="font-bold" style={{ color: '#0ECB81' }}>
              Support Level
            </h4>
          </div>
          <div className="text-2xl font-bold mono" style={{ color: '#EAECEF' }}>
            ${orderBook.stats.support_level.toFixed(2)}
          </div>
          <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
            Strongest bid wall
          </div>
        </div>

        <div
          className="p-4 rounded"
          style={{ background: 'rgba(246, 70, 93, 0.05)', border: '1px solid rgba(246, 70, 93, 0.1)' }}
        >
          <div className="flex items-center gap-2 mb-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ background: '#F6465D' }}
            ></div>
            <h4 className="font-bold" style={{ color: '#F6465D' }}>
              Resistance Level
            </h4>
          </div>
          <div className="text-2xl font-bold mono" style={{ color: '#EAECEF' }}>
            ${orderBook.stats.resistance_level.toFixed(2)}
          </div>
          <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
            Strongest ask wall
          </div>
        </div>
      </div>
    </div>
  )
}
