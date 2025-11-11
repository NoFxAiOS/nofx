import { useState } from 'react'
import { CorrelationHeatmap } from '../components/CorrelationHeatmap'
import { DrawdownChart } from '../components/DrawdownChart'
import { MonteCarloSimulation } from '../components/MonteCarloSimulation'
import { OrderBookDepth } from '../components/OrderBookDepth'
import { BarChart3, TrendingDown, Dices, BookOpen, ArrowLeft } from 'lucide-react'
import useSWR from 'swr'
import { api } from '../lib/api'

type AnalyticsTab = 'correlation' | 'drawdown' | 'montecarlo' | 'orderbook'

export function AnalyticsPage() {
  const [activeTab, setActiveTab] = useState<AnalyticsTab>('drawdown')
  const [selectedTrader, setSelectedTrader] = useState<string>('')
  const [orderBookSymbol, setOrderBookSymbol] = useState<string>('BTCUSDT')

  // Fetch traders list
  const { data: traders } = useSWR('my-traders', api.getTraders)

  // Common symbols for correlation analysis
  const correlationSymbols = ['BTC', 'ETH', 'SOL', 'BNB', 'XRP']

  const tabs = [
    {
      id: 'drawdown' as AnalyticsTab,
      label: 'Drawdown Analysis',
      icon: TrendingDown,
      color: '#F6465D',
    },
    {
      id: 'correlation' as AnalyticsTab,
      label: 'Correlation Matrix',
      icon: BarChart3,
      color: '#F0B90B',
    },
    {
      id: 'montecarlo' as AnalyticsTab,
      label: 'Monte Carlo Simulation',
      icon: Dices,
      color: '#0ECB81',
    },
    {
      id: 'orderbook' as AnalyticsTab,
      label: 'Order Book Depth',
      icon: BookOpen,
      color: '#4CAF50',
    },
  ]

  return (
    <div className="min-h-screen p-4 md:p-6 lg:p-8" style={{ background: '#0B0E11' }}>
      {/* Header */}
      <div className="max-w-7xl mx-auto mb-6">
        <button
          onClick={() => window.history.back()}
          className="flex items-center gap-2 mb-4 px-3 py-2 rounded transition-all hover:bg-opacity-80"
          style={{ background: '#1E2329', color: '#848E9C' }}
        >
          <ArrowLeft className="w-4 h-4" />
          Back
        </button>

        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
          <div>
            <h1
              className="text-2xl md:text-3xl font-bold mb-2"
              style={{ color: '#EAECEF' }}
            >
              📊 Advanced Analytics Dashboard
            </h1>
            <p className="text-sm" style={{ color: '#848E9C' }}>
              Comprehensive analysis tools for trading performance and market insights
            </p>
          </div>

          {/* Trader Selector */}
          {activeTab !== 'orderbook' && (
            <div>
              <label
                className="text-xs font-semibold mb-2 block"
                style={{ color: '#848E9C' }}
              >
                SELECT TRADER
              </label>
              <select
                value={selectedTrader}
                onChange={(e) => setSelectedTrader(e.target.value)}
                className="px-4 py-2 rounded text-sm font-mono min-w-[200px]"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                <option value="">Select a trader...</option>
                {traders?.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Symbol Selector for Order Book */}
          {activeTab === 'orderbook' && (
            <div>
              <label
                className="text-xs font-semibold mb-2 block"
                style={{ color: '#848E9C' }}
              >
                SELECT SYMBOL
              </label>
              <select
                value={orderBookSymbol}
                onChange={(e) => setOrderBookSymbol(e.target.value)}
                className="px-4 py-2 rounded text-sm font-mono min-w-[200px]"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                <option value="BTCUSDT">BTC/USDT</option>
                <option value="ETHUSDT">ETH/USDT</option>
                <option value="SOLUSDT">SOL/USDT</option>
                <option value="BNBUSDT">BNB/USDT</option>
                <option value="XRPUSDT">XRP/USDT</option>
                <option value="ADAUSDT">ADA/USDT</option>
                <option value="DOGEUSDT">DOGE/USDT</option>
              </select>
            </div>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="max-w-7xl mx-auto mb-6">
        <div
          className="flex gap-2 p-1 rounded overflow-x-auto"
          style={{ background: '#1E2329' }}
        >
          {tabs.map((tab) => {
            const Icon = tab.icon
            const isActive = activeTab === tab.id

            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className="flex items-center gap-2 px-4 py-3 rounded text-sm font-bold transition-all whitespace-nowrap"
                style={
                  isActive
                    ? {
                        background: tab.color,
                        color: '#000',
                        boxShadow: `0 2px 12px ${tab.color}40`,
                      }
                    : {
                        background: 'transparent',
                        color: '#848E9C',
                      }
                }
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </button>
            )
          })}
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto">
        {/* No Trader Selected Warning */}
        {activeTab !== 'orderbook' && !selectedTrader && (
          <div
            className="binance-card p-8 text-center"
            style={{ background: 'rgba(255, 152, 0, 0.05)', border: '1px solid rgba(255, 152, 0, 0.1)' }}
          >
            <div className="text-4xl mb-4">⚠️</div>
            <h3 className="text-lg font-bold mb-2" style={{ color: '#FF9800' }}>
              No Trader Selected
            </h3>
            <p className="text-sm" style={{ color: '#848E9C' }}>
              Please select a trader from the dropdown above to view analytics
            </p>
          </div>
        )}

        {/* Drawdown Analysis */}
        {activeTab === 'drawdown' && selectedTrader && (
          <DrawdownChart traderId={selectedTrader} />
        )}

        {/* Correlation Matrix */}
        {activeTab === 'correlation' && selectedTrader && (
          <div className="space-y-6">
            <CorrelationHeatmap
              traderId={selectedTrader}
              symbols={correlationSymbols}
              timeframe="1h"
            />

            {/* Info Card */}
            <div
              className="binance-card p-4"
              style={{ background: 'rgba(240, 185, 11, 0.05)', border: '1px solid rgba(240, 185, 11, 0.1)' }}
            >
              <h4 className="text-sm font-bold mb-2" style={{ color: '#F0B90B' }}>
                💡 How to Read Correlation Matrix
              </h4>
              <ul className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                <li>
                  • <strong>+1.0</strong>: Perfect positive correlation (move together)
                </li>
                <li>
                  • <strong>0.0</strong>: No correlation (independent)
                </li>
                <li>
                  • <strong>-1.0</strong>: Perfect negative correlation (move opposite)
                </li>
                <li>
                  • <strong>High correlation (|r| &gt; 0.7)</strong>: Assets move similarly - risk concentration
                </li>
                <li>
                  • <strong>Low correlation (|r| &lt; 0.3)</strong>: Good for diversification
                </li>
              </ul>
            </div>
          </div>
        )}

        {/* Monte Carlo Simulation */}
        {activeTab === 'montecarlo' && selectedTrader && (
          <div className="space-y-6">
            <MonteCarloSimulation traderId={selectedTrader} />

            {/* Info Card */}
            <div
              className="binance-card p-4"
              style={{ background: 'rgba(14, 203, 129, 0.05)', border: '1px solid rgba(14, 203, 129, 0.1)' }}
            >
              <h4 className="text-sm font-bold mb-2" style={{ color: '#0ECB81' }}>
                💡 Understanding Monte Carlo Simulation
              </h4>
              <ul className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                <li>
                  • Simulates thousands of possible future outcomes based on historical performance
                </li>
                <li>
                  • <strong>Expected Return</strong>: Average outcome across all simulations
                </li>
                <li>
                  • <strong>Percentiles</strong>: P5 = 5% worst case, P95 = 95% best case
                </li>
                <li>
                  • <strong>Risk of Ruin</strong>: Probability of losing 50%+ of capital
                </li>
                <li>
                  • Uses Geometric Brownian Motion (GBM) for realistic price modeling
                </li>
              </ul>
            </div>
          </div>
        )}

        {/* Order Book Depth */}
        {activeTab === 'orderbook' && (
          <div className="space-y-6">
            <OrderBookDepth symbol={orderBookSymbol} maxLevels={50} />

            {/* Info Card */}
            <div
              className="binance-card p-4"
              style={{ background: 'rgba(76, 175, 80, 0.05)', border: '1px solid rgba(76, 175, 80, 0.1)' }}
            >
              <h4 className="text-sm font-bold mb-2" style={{ color: '#4CAF50' }}>
                💡 Order Book Analysis Guide
              </h4>
              <ul className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                <li>
                  • <strong>Bid Depth</strong>: Total value of buy orders (green area)
                </li>
                <li>
                  • <strong>Ask Depth</strong>: Total value of sell orders (red area)
                </li>
                <li>
                  • <strong>Support Level</strong>: Price with strongest buy wall
                </li>
                <li>
                  • <strong>Resistance Level</strong>: Price with strongest sell wall
                </li>
                <li>
                  • <strong>Imbalance &gt; 20%</strong>: Strong directional pressure
                </li>
                <li>
                  • Large orders (3σ above average) indicate whale activity
                </li>
              </ul>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
