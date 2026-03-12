import { motion } from 'framer-motion'
import {
  TrendingUp,
  TrendingDown,
  Activity,
  ArrowUpRight,
  ArrowDownRight,
} from 'lucide-react'
import { MetricTooltip } from '../common/MetricTooltip'
import { t, type Language } from '../../i18n/translations'
import { EquityChart } from './BacktestChartTab'
import type {
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestPositionStatus,
} from '../../types'

// ============ Stat Card ============

interface StatCardProps {
  icon: typeof TrendingUp
  label: string
  value: string | number
  suffix?: string
  trend?: 'up' | 'down' | 'neutral'
  color?: string
  metricKey?: string
  language?: string
}

export function StatCard({
  icon: Icon,
  label,
  value,
  suffix,
  trend,
  color = '#EAECEF',
  metricKey,
  language = 'en',
}: StatCardProps) {
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

// ============ Progress Ring ============

interface ProgressRingProps {
  progress: number
  size?: number
}

export function ProgressRing({ progress, size = 120 }: ProgressRingProps) {
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

// ============ Positions Display ============

interface PositionsDisplayProps {
  positions: BacktestPositionStatus[]
  language: Language
}

export function PositionsDisplay({ positions, language }: PositionsDisplayProps) {
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
            {t('backtestOverview.activePositions', language)}
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
            {t('backtestOverview.margin', language)}: ${totalMargin.toFixed(2)}
          </span>
          <span
            className="font-medium"
            style={{ color: totalUnrealizedPnL >= 0 ? '#0ECB81' : '#F6465D' }}
          >
            {t('backtestOverview.unrealized', language)}: {totalUnrealizedPnL >= 0 ? '+' : ''}
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
                    {t('backtestOverview.qty', language)}: {pos.quantity.toFixed(4)} ·{' '}
                    {t('backtestOverview.margin', language)}: ${pos.margin_used.toFixed(2)}
                  </div>
                </div>
              </div>

              <div className="text-right">
                <div className="flex items-center gap-2 text-xs">
                  <span style={{ color: '#848E9C' }}>
                    {t('backtestOverview.entry', language)}: ${pos.entry_price.toFixed(2)}
                  </span>
                  <span style={{ color: '#EAECEF' }}>
                    {t('backtestOverview.mark', language)}: ${pos.mark_price.toFixed(2)}
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

// ============ Overview Tab Content ============

interface BacktestOverviewTabProps {
  equity: BacktestEquityPoint[] | undefined
  trades: BacktestTradeEvent[] | undefined
  metrics: BacktestMetrics | undefined
  language: Language
  tr: (key: string) => string
}

export function BacktestOverviewTab({
  equity,
  trades,
  metrics,
  language,
  tr,
}: BacktestOverviewTabProps) {
  return (
    <motion.div
      key="overview"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
    >
      {equity && equity.length > 0 ? (
        <EquityChart equity={equity} trades={trades ?? []} />
      ) : (
        <div className="py-12 text-center" style={{ color: '#5E6673' }}>
          {tr('charts.equityEmpty')}
        </div>
      )}

      {metrics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-4">
          <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
            <div className="flex items-center gap-1 text-xs" style={{ color: '#848E9C' }}>
              {t('backtestOverview.winRate', language)}
              <MetricTooltip metricKey="win_rate" language={language} size={11} />
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {(metrics.win_rate ?? 0).toFixed(1)}%
            </div>
          </div>
          <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
            <div className="flex items-center gap-1 text-xs" style={{ color: '#848E9C' }}>
              {t('backtestOverview.profitFactor', language)}
              <MetricTooltip metricKey="profit_factor" language={language} size={11} />
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {(metrics.profit_factor ?? 0).toFixed(2)}
            </div>
          </div>
          <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {t('backtestOverview.totalTrades', language)}
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {metrics.trades ?? 0}
            </div>
          </div>
          <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {t('backtestOverview.bestSymbol', language)}
            </div>
            <div className="text-lg font-bold" style={{ color: '#0ECB81' }}>
              {metrics.best_symbol?.replace('USDT', '') || '-'}
            </div>
          </div>
        </div>
      )}
    </motion.div>
  )
}
