import { useEffect, useMemo, useState, useCallback, type FormEvent } from 'react'
import useSWR from 'swr'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Play,
  Pause,
  Square,
  Download,
  Trash2,
  ChevronRight,
  ChevronLeft,
  Clock,
  TrendingUp,
  TrendingDown,
  Activity,
  BarChart3,
  Brain,
  Zap,
  Target,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Layers,
  Eye,
  ArrowUpRight,
  ArrowDownRight,
} from 'lucide-react'
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
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { confirmToast } from '../lib/notify'
import { DecisionCard } from './DecisionCard'
import type {
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  DecisionRecord,
  AIModel,
} from '../types'

// ============ Types ============
type WizardStep = 1 | 2 | 3
type ViewTab = 'overview' | 'chart' | 'trades' | 'decisions' | 'compare'

const TIMEFRAME_OPTIONS = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d']
const POPULAR_SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT']

// ============ Helper Functions ============
const toLocalInput = (date: Date) => {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}


// ============ Sub Components ============

// Stats Card Component
function StatCard({
  icon: Icon,
  label,
  value,
  suffix,
  trend,
  color = '#EAECEF',
}: {
  icon: typeof TrendingUp
  label: string
  value: string | number
  suffix?: string
  trend?: 'up' | 'down' | 'neutral'
  color?: string
}) {
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

// Progress Ring Component
function ProgressRing({ progress, size = 120 }: { progress: number; size?: number }) {
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

// Equity Chart Component using Recharts
function BacktestChart({
  equity,
  trades,
}: {
  equity: BacktestEquityPoint[]
  trades: BacktestTradeEvent[]
}) {
  const chartData = useMemo(() => {
    return equity.map((point) => ({
      time: new Date(point.ts).toLocaleString(),
      ts: point.ts,
      equity: point.equity,
      pnl_pct: point.pnl_pct,
    }))
  }, [equity])

  // Find trade points to mark on chart
  const tradeMarkers = useMemo(() => {
    if (!trades.length || !equity.length) return []
    return trades
      .filter((t) => t.action.includes('open') || t.action.includes('close'))
      .map((trade) => {
        // Find closest equity point
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
      .slice(-30) // Limit markers
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
          {/* Trade markers */}
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

// Trade Timeline Component
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

// ============ Main Component ============
export function BacktestPage() {
  const { language } = useLanguage()
  const tr = useCallback(
    (key: string, params?: Record<string, string | number>) => t(`backtestPage.${key}`, language, params),
    [language]
  )

  // State
  const now = new Date()
  const [wizardStep, setWizardStep] = useState<WizardStep>(1)
  const [viewTab, setViewTab] = useState<ViewTab>('overview')
  const [selectedRunId, setSelectedRunId] = useState<string>()
  const [compareRunIds, setCompareRunIds] = useState<string[]>([])
  const [isStarting, setIsStarting] = useState(false)
  const [toast, setToast] = useState<{ text: string; tone: 'info' | 'error' | 'success' } | null>(null)

  // Form state
  const [formState, setFormState] = useState({
    runId: '',
    symbols: 'BTCUSDT,ETHUSDT,SOLUSDT',
    timeframes: ['3m', '15m', '4h'],
    decisionTf: '3m',
    cadence: 20,
    start: toLocalInput(new Date(now.getTime() - 3 * 24 * 3600 * 1000)),
    end: toLocalInput(now),
    balance: 1000,
    fee: 5,
    slippage: 2,
    btcEthLeverage: 5,
    altcoinLeverage: 5,
    fill: 'next_open',
    prompt: 'baseline',
    promptTemplate: 'default',
    customPrompt: '',
    overridePrompt: false,
    cacheAI: true,
    replayOnly: false,
    aiModelId: '',
  })

  // Data fetching
  const { data: runsResp, mutate: refreshRuns } = useSWR(['backtest-runs'], () =>
    api.getBacktestRuns({ limit: 100, offset: 0 })
  , { refreshInterval: 5000 })
  const runs = runsResp?.items ?? []

  const { data: aiModels } = useSWR<AIModel[]>('ai-models', api.getModelConfigs, { refreshInterval: 30000 })

  const { data: status } = useSWR<BacktestStatusPayload>(
    selectedRunId ? ['bt-status', selectedRunId] : null,
    () => api.getBacktestStatus(selectedRunId!),
    { refreshInterval: 2000 }
  )

  const { data: equity } = useSWR<BacktestEquityPoint[]>(
    selectedRunId ? ['bt-equity', selectedRunId] : null,
    () => api.getBacktestEquity(selectedRunId!, '1m', 2000),
    { refreshInterval: 5000 }
  )

  const { data: trades } = useSWR<BacktestTradeEvent[]>(
    selectedRunId ? ['bt-trades', selectedRunId] : null,
    () => api.getBacktestTrades(selectedRunId!, 500),
    { refreshInterval: 5000 }
  )

  const { data: metrics } = useSWR<BacktestMetrics>(
    selectedRunId ? ['bt-metrics', selectedRunId] : null,
    () => api.getBacktestMetrics(selectedRunId!),
    { refreshInterval: 10000 }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    selectedRunId ? ['bt-decisions', selectedRunId] : null,
    () => api.getBacktestDecisions(selectedRunId!, 30),
    { refreshInterval: 5000 }
  )

  const selectedRun = runs.find((r) => r.run_id === selectedRunId)
  const selectedModel = aiModels?.find((m) => m.id === formState.aiModelId)

  // Auto-select first model
  useEffect(() => {
    if (!formState.aiModelId && aiModels?.length) {
      const enabled = aiModels.find((m) => m.enabled)
      if (enabled) setFormState((s) => ({ ...s, aiModelId: enabled.id }))
    }
  }, [aiModels, formState.aiModelId])

  // Auto-select first run
  useEffect(() => {
    if (!selectedRunId && runs.length > 0) {
      setSelectedRunId(runs[0].run_id)
    }
  }, [runs, selectedRunId])

  // Handlers
  const handleFormChange = (key: string, value: string | number | boolean | string[]) => {
    setFormState((prev) => ({ ...prev, [key]: value }))
  }

  const handleStart = async (event: FormEvent) => {
    event.preventDefault()
    if (!selectedModel?.enabled) {
      setToast({ text: tr('toasts.selectModel'), tone: 'error' })
      return
    }

    try {
      setIsStarting(true)
      const start = new Date(formState.start).getTime()
      const end = new Date(formState.end).getTime()
      if (end <= start) throw new Error(tr('toasts.invalidRange'))

      const payload = await api.startBacktest({
        run_id: formState.runId.trim() || undefined,
        symbols: formState.symbols.split(',').map((s) => s.trim()).filter(Boolean),
        timeframes: formState.timeframes,
        decision_timeframe: formState.decisionTf,
        decision_cadence_nbars: formState.cadence,
        start_ts: Math.floor(start / 1000),
        end_ts: Math.floor(end / 1000),
        initial_balance: formState.balance,
        fee_bps: formState.fee,
        slippage_bps: formState.slippage,
        fill_policy: formState.fill,
        prompt_variant: formState.prompt,
        prompt_template: formState.promptTemplate,
        custom_prompt: formState.customPrompt.trim() || undefined,
        override_prompt: formState.overridePrompt,
        cache_ai: formState.cacheAI,
        replay_only: formState.replayOnly,
        ai_model_id: formState.aiModelId,
        leverage: {
          btc_eth_leverage: formState.btcEthLeverage,
          altcoin_leverage: formState.altcoinLeverage,
        },
      })

      setToast({ text: tr('toasts.startSuccess', { id: payload.run_id }), tone: 'success' })
      setSelectedRunId(payload.run_id)
      setWizardStep(1)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.startFailed')
      setToast({ text: errMsg, tone: 'error' })
    } finally {
      setIsStarting(false)
    }
  }

  const handleControl = async (action: 'pause' | 'resume' | 'stop') => {
    if (!selectedRunId) return
    try {
      if (action === 'pause') await api.pauseBacktest(selectedRunId)
      if (action === 'resume') await api.resumeBacktest(selectedRunId)
      if (action === 'stop') await api.stopBacktest(selectedRunId)
      setToast({ text: tr('toasts.actionSuccess', { action, id: selectedRunId }), tone: 'success' })
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.actionFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleDelete = async () => {
    if (!selectedRunId) return
    const confirmed = await confirmToast(tr('toasts.confirmDelete', { id: selectedRunId }), {
      title: language === 'zh' ? '确认删除' : 'Confirm Delete',
      okText: language === 'zh' ? '删除' : 'Delete',
      cancelText: language === 'zh' ? '取消' : 'Cancel',
    })
    if (!confirmed) return
    try {
      await api.deleteBacktestRun(selectedRunId)
      setToast({ text: tr('toasts.deleteSuccess'), tone: 'success' })
      setSelectedRunId(undefined)
      await refreshRuns()
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.deleteFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const handleExport = async () => {
    if (!selectedRunId) return
    try {
      const blob = await api.exportBacktest(selectedRunId)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${selectedRunId}_export.zip`
      link.click()
      URL.revokeObjectURL(url)
      setToast({ text: tr('toasts.exportSuccess', { id: selectedRunId }), tone: 'success' })
    } catch (error: unknown) {
      const errMsg = error instanceof Error ? error.message : tr('toasts.exportFailed')
      setToast({ text: errMsg, tone: 'error' })
    }
  }

  const toggleCompare = (runId: string) => {
    setCompareRunIds((prev) =>
      prev.includes(runId) ? prev.filter((id) => id !== runId) : [...prev, runId].slice(-3)
    )
  }

  const quickRanges = [
    { label: language === 'zh' ? '24小时' : '24h', hours: 24 },
    { label: language === 'zh' ? '3天' : '3d', hours: 72 },
    { label: language === 'zh' ? '7天' : '7d', hours: 168 },
    { label: language === 'zh' ? '30天' : '30d', hours: 720 },
  ]

  const applyQuickRange = (hours: number) => {
    const endDate = new Date()
    const startDate = new Date(endDate.getTime() - hours * 3600 * 1000)
    handleFormChange('start', toLocalInput(startDate))
    handleFormChange('end', toLocalInput(endDate))
  }

  const getStateColor = (state: string) => {
    switch (state) {
      case 'running':
        return '#F0B90B'
      case 'completed':
        return '#0ECB81'
      case 'failed':
      case 'liquidated':
        return '#F6465D'
      case 'paused':
        return '#848E9C'
      default:
        return '#848E9C'
    }
  }

  const getStateIcon = (state: string) => {
    switch (state) {
      case 'running':
        return <Activity className="w-4 h-4" />
      case 'completed':
        return <CheckCircle2 className="w-4 h-4" />
      case 'failed':
      case 'liquidated':
        return <XCircle className="w-4 h-4" />
      case 'paused':
        return <Pause className="w-4 h-4" />
      default:
        return <Clock className="w-4 h-4" />
    }
  }

  // Render
  return (
    <div className="space-y-6">
      {/* Toast */}
      <AnimatePresence>
        {toast && (
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className="p-3 rounded-lg text-sm"
            style={{
              background:
                toast.tone === 'error'
                  ? 'rgba(246,70,93,0.15)'
                  : toast.tone === 'success'
                    ? 'rgba(14,203,129,0.15)'
                    : 'rgba(240,185,11,0.15)',
              color: toast.tone === 'error' ? '#F6465D' : toast.tone === 'success' ? '#0ECB81' : '#F0B90B',
              border: `1px solid ${toast.tone === 'error' ? 'rgba(246,70,93,0.3)' : toast.tone === 'success' ? 'rgba(14,203,129,0.3)' : 'rgba(240,185,11,0.3)'}`,
            }}
          >
            {toast.text}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-3" style={{ color: '#EAECEF' }}>
            <Brain className="w-7 h-7" style={{ color: '#F0B90B' }} />
            {tr('title')}
          </h1>
          <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
            {tr('subtitle')}
          </p>
        </div>
        <button
          onClick={() => setWizardStep(1)}
          className="px-4 py-2 rounded-lg font-medium flex items-center gap-2 transition-all hover:opacity-90"
          style={{ background: '#F0B90B', color: '#0B0E11' }}
        >
          <Play className="w-4 h-4" />
          {language === 'zh' ? '新建回测' : 'New Backtest'}
        </button>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Left Panel - Config / History */}
        <div className="space-y-4">
          {/* Wizard */}
          <div className="binance-card p-5">
            <div className="flex items-center gap-2 mb-4">
              {[1, 2, 3].map((step) => (
                <div key={step} className="flex items-center">
                  <button
                    onClick={() => setWizardStep(step as WizardStep)}
                    className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
                    style={{
                      background: wizardStep >= step ? '#F0B90B' : '#2B3139',
                      color: wizardStep >= step ? '#0B0E11' : '#848E9C',
                    }}
                  >
                    {step}
                  </button>
                  {step < 3 && (
                    <div
                      className="w-8 h-0.5 mx-1"
                      style={{ background: wizardStep > step ? '#F0B90B' : '#2B3139' }}
                    />
                  )}
                </div>
              ))}
              <span className="ml-2 text-xs" style={{ color: '#848E9C' }}>
                {wizardStep === 1
                  ? language === 'zh'
                    ? '选择模型'
                    : 'Select Model'
                  : wizardStep === 2
                    ? language === 'zh'
                      ? '配置参数'
                      : 'Configure'
                    : language === 'zh'
                      ? '确认启动'
                      : 'Confirm'}
              </span>
            </div>

            <form onSubmit={handleStart}>
              <AnimatePresence mode="wait">
                {/* Step 1: Model & Symbols */}
                {wizardStep === 1 && (
                  <motion.div
                    key="step1"
                    initial={{ opacity: 0, x: 20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -20 }}
                    className="space-y-4"
                  >
                    <div>
                      <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                        {tr('form.aiModelLabel')}
                      </label>
                      <select
                        className="w-full p-3 rounded-lg text-sm"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                        value={formState.aiModelId}
                        onChange={(e) => handleFormChange('aiModelId', e.target.value)}
                      >
                        <option value="">{tr('form.selectAiModel')}</option>
                        {aiModels?.map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.name} ({m.provider}) {!m.enabled && '⚠️'}
                          </option>
                        ))}
                      </select>
                      {selectedModel && (
                        <div className="mt-2 flex items-center gap-2 text-xs">
                          <span
                            className="px-2 py-0.5 rounded"
                            style={{
                              background: selectedModel.enabled ? 'rgba(14,203,129,0.1)' : 'rgba(246,70,93,0.1)',
                              color: selectedModel.enabled ? '#0ECB81' : '#F6465D',
                            }}
                          >
                            {selectedModel.enabled ? tr('form.enabled') : tr('form.disabled')}
                          </span>
                        </div>
                      )}
                    </div>

                    <div>
                      <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                        {tr('form.symbolsLabel')}
                      </label>
                      <div className="flex flex-wrap gap-1 mb-2">
                        {POPULAR_SYMBOLS.map((sym) => {
                          const isSelected = formState.symbols.includes(sym)
                          return (
                            <button
                              key={sym}
                              type="button"
                              onClick={() => {
                                const current = formState.symbols.split(',').map((s) => s.trim()).filter(Boolean)
                                const updated = isSelected
                                  ? current.filter((s) => s !== sym)
                                  : [...current, sym]
                                handleFormChange('symbols', updated.join(','))
                              }}
                              className="px-2 py-1 rounded text-xs transition-all"
                              style={{
                                background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                                border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                                color: isSelected ? '#F0B90B' : '#848E9C',
                              }}
                            >
                              {sym.replace('USDT', '')}
                            </button>
                          )
                        })}
                      </div>
                      <textarea
                        className="w-full p-2 rounded-lg text-xs font-mono"
                        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                        value={formState.symbols}
                        onChange={(e) => handleFormChange('symbols', e.target.value)}
                        rows={2}
                      />
                    </div>

                    <button
                      type="button"
                      onClick={() => setWizardStep(2)}
                      disabled={!selectedModel?.enabled}
                      className="w-full py-2.5 rounded-lg font-medium flex items-center justify-center gap-2 transition-all disabled:opacity-50"
                      style={{ background: '#F0B90B', color: '#0B0E11' }}
                    >
                      {language === 'zh' ? '下一步' : 'Next'}
                      <ChevronRight className="w-4 h-4" />
                    </button>
                  </motion.div>
                )}

                {/* Step 2: Parameters */}
                {wizardStep === 2 && (
                  <motion.div
                    key="step2"
                    initial={{ opacity: 0, x: 20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -20 }}
                    className="space-y-4"
                  >
                    <div>
                      <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                        {tr('form.timeRangeLabel')}
                      </label>
                      <div className="flex flex-wrap gap-1 mb-2">
                        {quickRanges.map((r) => (
                          <button
                            key={r.hours}
                            type="button"
                            onClick={() => applyQuickRange(r.hours)}
                            className="px-3 py-1 rounded text-xs"
                            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                          >
                            {r.label}
                          </button>
                        ))}
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <input
                          type="datetime-local"
                          className="p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.start}
                          onChange={(e) => handleFormChange('start', e.target.value)}
                        />
                        <input
                          type="datetime-local"
                          className="p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.end}
                          onChange={(e) => handleFormChange('end', e.target.value)}
                        />
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                        {language === 'zh' ? '时间周期' : 'Timeframes'}
                      </label>
                      <div className="flex flex-wrap gap-1">
                        {TIMEFRAME_OPTIONS.map((tf) => {
                          const isSelected = formState.timeframes.includes(tf)
                          return (
                            <button
                              key={tf}
                              type="button"
                              onClick={() => {
                                const updated = isSelected
                                  ? formState.timeframes.filter((t) => t !== tf)
                                  : [...formState.timeframes, tf]
                                if (updated.length > 0) handleFormChange('timeframes', updated)
                              }}
                              className="px-2 py-1 rounded text-xs transition-all"
                              style={{
                                background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                                border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                                color: isSelected ? '#F0B90B' : '#848E9C',
                              }}
                            >
                              {tf}
                            </button>
                          )
                        })}
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.initialBalanceLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.balance}
                          onChange={(e) => handleFormChange('balance', Number(e.target.value))}
                        />
                      </div>
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.decisionTfLabel')}
                        </label>
                        <select
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.decisionTf}
                          onChange={(e) => handleFormChange('decisionTf', e.target.value)}
                        >
                          {formState.timeframes.map((tf) => (
                            <option key={tf} value={tf}>
                              {tf}
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>

                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => setWizardStep(1)}
                        className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                      >
                        <ChevronLeft className="w-4 h-4" />
                        {language === 'zh' ? '上一步' : 'Back'}
                      </button>
                      <button
                        type="button"
                        onClick={() => setWizardStep(3)}
                        className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                        style={{ background: '#F0B90B', color: '#0B0E11' }}
                      >
                        {language === 'zh' ? '下一步' : 'Next'}
                        <ChevronRight className="w-4 h-4" />
                      </button>
                    </div>
                  </motion.div>
                )}

                {/* Step 3: Advanced & Confirm */}
                {wizardStep === 3 && (
                  <motion.div
                    key="step3"
                    initial={{ opacity: 0, x: 20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: -20 }}
                    className="space-y-4"
                  >
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.btcEthLeverageLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.btcEthLeverage}
                          onChange={(e) => handleFormChange('btcEthLeverage', Number(e.target.value))}
                        />
                      </div>
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.altcoinLeverageLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.altcoinLeverage}
                          onChange={(e) => handleFormChange('altcoinLeverage', Number(e.target.value))}
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-3 gap-2">
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.feeLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.fee}
                          onChange={(e) => handleFormChange('fee', Number(e.target.value))}
                        />
                      </div>
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.slippageLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.slippage}
                          onChange={(e) => handleFormChange('slippage', Number(e.target.value))}
                        />
                      </div>
                      <div>
                        <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                          {tr('form.cadenceLabel')}
                        </label>
                        <input
                          type="number"
                          className="w-full p-2 rounded-lg text-xs"
                          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                          value={formState.cadence}
                          onChange={(e) => handleFormChange('cadence', Number(e.target.value))}
                        />
                      </div>
                    </div>

                    <div>
                      <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                        {language === 'zh' ? '策略风格' : 'Strategy Style'}
                      </label>
                      <div className="flex flex-wrap gap-1">
                        {['baseline', 'aggressive', 'conservative', 'scalping'].map((p) => (
                          <button
                            key={p}
                            type="button"
                            onClick={() => handleFormChange('prompt', p)}
                            className="px-3 py-1.5 rounded text-xs transition-all"
                            style={{
                              background: formState.prompt === p ? 'rgba(240,185,11,0.15)' : '#1E2329',
                              border: `1px solid ${formState.prompt === p ? '#F0B90B' : '#2B3139'}`,
                              color: formState.prompt === p ? '#F0B90B' : '#848E9C',
                            }}
                          >
                            {tr(`form.promptPresets.${p}`)}
                          </button>
                        ))}
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-4 text-xs" style={{ color: '#848E9C' }}>
                      <label className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={formState.cacheAI}
                          onChange={(e) => handleFormChange('cacheAI', e.target.checked)}
                          className="accent-[#F0B90B]"
                        />
                        {tr('form.cacheAiLabel')}
                      </label>
                      <label className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={formState.replayOnly}
                          onChange={(e) => handleFormChange('replayOnly', e.target.checked)}
                          className="accent-[#F0B90B]"
                        />
                        {tr('form.replayOnlyLabel')}
                      </label>
                    </div>

                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => setWizardStep(2)}
                        className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                      >
                        <ChevronLeft className="w-4 h-4" />
                        {language === 'zh' ? '上一步' : 'Back'}
                      </button>
                      <button
                        type="submit"
                        disabled={isStarting}
                        className="flex-1 py-2 rounded-lg font-bold flex items-center justify-center gap-2 disabled:opacity-50"
                        style={{ background: '#F0B90B', color: '#0B0E11' }}
                      >
                        {isStarting ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <Zap className="w-4 h-4" />
                        )}
                        {isStarting ? tr('starting') : tr('start')}
                      </button>
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </form>
          </div>

          {/* Run History */}
          <div className="binance-card p-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
                <Layers className="w-4 h-4" style={{ color: '#F0B90B' }} />
                {tr('runList.title')}
              </h3>
              <span className="text-xs" style={{ color: '#848E9C' }}>
                {runs.length} {language === 'zh' ? '条' : 'runs'}
              </span>
            </div>

            <div className="space-y-2 max-h-[300px] overflow-y-auto">
              {runs.length === 0 ? (
                <div className="py-8 text-center text-sm" style={{ color: '#5E6673' }}>
                  {tr('emptyStates.noRuns')}
                </div>
              ) : (
                runs.map((run) => (
                  <button
                    key={run.run_id}
                    onClick={() => setSelectedRunId(run.run_id)}
                    className="w-full p-3 rounded-lg text-left transition-all"
                    style={{
                      background: run.run_id === selectedRunId ? 'rgba(240,185,11,0.1)' : '#1E2329',
                      border: `1px solid ${run.run_id === selectedRunId ? '#F0B90B' : '#2B3139'}`,
                    }}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-xs" style={{ color: '#EAECEF' }}>
                        {run.run_id.slice(0, 20)}...
                      </span>
                      <span
                        className="flex items-center gap-1 text-xs"
                        style={{ color: getStateColor(run.state) }}
                      >
                        {getStateIcon(run.state)}
                        {tr(`states.${run.state}`)}
                      </span>
                    </div>
                    <div className="flex items-center justify-between mt-1">
                      <span className="text-xs" style={{ color: '#848E9C' }}>
                        {run.summary.progress_pct.toFixed(0)}% · ${run.summary.equity_last.toFixed(0)}
                      </span>
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          toggleCompare(run.run_id)
                        }}
                        className="p-1 rounded"
                        style={{
                          background: compareRunIds.includes(run.run_id)
                            ? 'rgba(240,185,11,0.2)'
                            : 'transparent',
                        }}
                        title={language === 'zh' ? '添加到对比' : 'Add to compare'}
                      >
                        <Eye
                          className="w-3 h-3"
                          style={{
                            color: compareRunIds.includes(run.run_id) ? '#F0B90B' : '#5E6673',
                          }}
                        />
                      </button>
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>
        </div>

        {/* Right Panel - Results */}
        <div className="xl:col-span-2 space-y-4">
          {!selectedRunId ? (
            <div
              className="binance-card p-12 text-center"
              style={{ color: '#5E6673' }}
            >
              <Brain className="w-12 h-12 mx-auto mb-4 opacity-30" />
              <p>{tr('emptyStates.selectRun')}</p>
            </div>
          ) : (
            <>
              {/* Status Bar */}
              <div className="binance-card p-4">
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="flex items-center gap-4">
                    <ProgressRing progress={status?.progress_pct ?? selectedRun?.summary.progress_pct ?? 0} size={80} />
                    <div>
                      <h2 className="font-mono font-bold" style={{ color: '#EAECEF' }}>
                        {selectedRunId}
                      </h2>
                      <div className="flex items-center gap-2 mt-1">
                        <span
                          className="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium"
                          style={{
                            background: `${getStateColor(status?.state ?? selectedRun?.state ?? '')}20`,
                            color: getStateColor(status?.state ?? selectedRun?.state ?? ''),
                          }}
                        >
                          {getStateIcon(status?.state ?? selectedRun?.state ?? '')}
                          {tr(`states.${status?.state ?? selectedRun?.state}`)}
                        </span>
                        {selectedRun?.summary.decision_tf && (
                          <span className="text-xs" style={{ color: '#848E9C' }}>
                            {selectedRun.summary.decision_tf} · {selectedRun.summary.symbol_count} symbols
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {(status?.state === 'running' || selectedRun?.state === 'running') && (
                      <>
                        <button
                          onClick={() => handleControl('pause')}
                          className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                          style={{ border: '1px solid #2B3139' }}
                          title={tr('actions.pause')}
                        >
                          <Pause className="w-4 h-4" style={{ color: '#F0B90B' }} />
                        </button>
                        <button
                          onClick={() => handleControl('stop')}
                          className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                          style={{ border: '1px solid #2B3139' }}
                          title={tr('actions.stop')}
                        >
                          <Square className="w-4 h-4" style={{ color: '#F6465D' }} />
                        </button>
                      </>
                    )}
                    {status?.state === 'paused' && (
                      <button
                        onClick={() => handleControl('resume')}
                        className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                        style={{ border: '1px solid #2B3139' }}
                        title={tr('actions.resume')}
                      >
                        <Play className="w-4 h-4" style={{ color: '#0ECB81' }} />
                      </button>
                    )}
                    <button
                      onClick={handleExport}
                      className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                      style={{ border: '1px solid #2B3139' }}
                      title={tr('detail.exportLabel')}
                    >
                      <Download className="w-4 h-4" style={{ color: '#EAECEF' }} />
                    </button>
                    <button
                      onClick={handleDelete}
                      className="p-2 rounded-lg transition-all hover:bg-[#2B3139]"
                      style={{ border: '1px solid #2B3139' }}
                      title={tr('detail.deleteLabel')}
                    >
                      <Trash2 className="w-4 h-4" style={{ color: '#F6465D' }} />
                    </button>
                  </div>
                </div>

                {(status?.note || status?.last_error) && (
                  <div
                    className="mt-3 p-2 rounded-lg text-xs flex items-center gap-2"
                    style={{
                      background: 'rgba(246,70,93,0.1)',
                      border: '1px solid rgba(246,70,93,0.3)',
                      color: '#F6465D',
                    }}
                  >
                    <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                    {status?.note || status?.last_error}
                  </div>
                )}
              </div>

              {/* Stats Grid */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <StatCard
                  icon={Target}
                  label={language === 'zh' ? '当前净值' : 'Equity'}
                  value={(status?.equity ?? 0).toFixed(2)}
                  suffix="USDT"
                />
                <StatCard
                  icon={TrendingUp}
                  label={language === 'zh' ? '总收益率' : 'Return'}
                  value={`${(metrics?.total_return_pct ?? 0).toFixed(2)}%`}
                  trend={(metrics?.total_return_pct ?? 0) >= 0 ? 'up' : 'down'}
                  color={(metrics?.total_return_pct ?? 0) >= 0 ? '#0ECB81' : '#F6465D'}
                />
                <StatCard
                  icon={AlertTriangle}
                  label={language === 'zh' ? '最大回撤' : 'Max DD'}
                  value={`${(metrics?.max_drawdown_pct ?? 0).toFixed(2)}%`}
                  color="#F6465D"
                />
                <StatCard
                  icon={BarChart3}
                  label={language === 'zh' ? '夏普比率' : 'Sharpe'}
                  value={(metrics?.sharpe_ratio ?? 0).toFixed(2)}
                />
              </div>

              {/* Tabs */}
              <div className="binance-card">
                <div className="flex border-b" style={{ borderColor: '#2B3139' }}>
                  {(['overview', 'chart', 'trades', 'decisions'] as ViewTab[]).map((tab) => (
                    <button
                      key={tab}
                      onClick={() => setViewTab(tab)}
                      className="px-4 py-3 text-sm font-medium transition-all relative"
                      style={{ color: viewTab === tab ? '#F0B90B' : '#848E9C' }}
                    >
                      {tab === 'overview'
                        ? language === 'zh'
                          ? '概览'
                          : 'Overview'
                        : tab === 'chart'
                          ? language === 'zh'
                            ? '图表'
                            : 'Chart'
                          : tab === 'trades'
                            ? language === 'zh'
                              ? '交易'
                              : 'Trades'
                            : language === 'zh'
                              ? 'AI决策'
                              : 'Decisions'}
                      {viewTab === tab && (
                        <motion.div
                          layoutId="tab-indicator"
                          className="absolute bottom-0 left-0 right-0 h-0.5"
                          style={{ background: '#F0B90B' }}
                        />
                      )}
                    </button>
                  ))}
                </div>

                <div className="p-4">
                  <AnimatePresence mode="wait">
                    {viewTab === 'overview' && (
                      <motion.div
                        key="overview"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                      >
                        {equity && equity.length > 0 ? (
                          <BacktestChart equity={equity} trades={trades ?? []} />
                        ) : (
                          <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                            {tr('charts.equityEmpty')}
                          </div>
                        )}

                        {metrics && (
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-4">
                            <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                {language === 'zh' ? '胜率' : 'Win Rate'}
                              </div>
                              <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                {(metrics.win_rate ?? 0).toFixed(1)}%
                              </div>
                            </div>
                            <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                {language === 'zh' ? '盈亏因子' : 'Profit Factor'}
                              </div>
                              <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                {(metrics.profit_factor ?? 0).toFixed(2)}
                              </div>
                            </div>
                            <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                {language === 'zh' ? '总交易数' : 'Total Trades'}
                              </div>
                              <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
                                {metrics.trades ?? 0}
                              </div>
                            </div>
                            <div className="p-3 rounded-lg" style={{ background: '#1E2329' }}>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                {language === 'zh' ? '最佳币种' : 'Best Symbol'}
                              </div>
                              <div className="text-lg font-bold" style={{ color: '#0ECB81' }}>
                                {metrics.best_symbol?.replace('USDT', '') || '-'}
                              </div>
                            </div>
                          </div>
                        )}
                      </motion.div>
                    )}

                    {viewTab === 'chart' && (
                      <motion.div
                        key="chart"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                      >
                        {equity && equity.length > 0 ? (
                          <BacktestChart equity={equity} trades={trades ?? []} />
                        ) : (
                          <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                            {tr('charts.equityEmpty')}
                          </div>
                        )}
                      </motion.div>
                    )}

                    {viewTab === 'trades' && (
                      <motion.div
                        key="trades"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                      >
                        <TradeTimeline trades={trades ?? []} />
                      </motion.div>
                    )}

                    {viewTab === 'decisions' && (
                      <motion.div
                        key="decisions"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="space-y-3 max-h-[500px] overflow-y-auto"
                      >
                        {decisions && decisions.length > 0 ? (
                          decisions.map((d) => (
                            <DecisionCard
                              key={`${d.cycle_number}-${d.timestamp}`}
                              decision={d}
                              language={language}
                            />
                          ))
                        ) : (
                          <div className="py-12 text-center" style={{ color: '#5E6673' }}>
                            {tr('decisionTrail.emptyHint')}
                          </div>
                        )}
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
