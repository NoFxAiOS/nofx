import { useMemo } from 'react'
import { motion } from 'framer-motion'
import { TrendingUp, TrendingDown } from 'lucide-react'
import type { BacktestTradeEvent } from '../../types'

// ============ Trade Timeline ============

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

// ============ Trades Tab Content ============

interface BacktestTradesTabProps {
  trades: BacktestTradeEvent[] | undefined
}

export function BacktestTradesTab({ trades }: BacktestTradesTabProps) {
  return (
    <motion.div
      key="trades"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
    >
      <TradeTimeline trades={trades ?? []} />
    </motion.div>
  )
}
