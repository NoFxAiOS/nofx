import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { formatPrice, formatQuantity } from '../../utils/format'
import type { Language } from '../../i18n/translations'
import type { OpenOrder, Position } from '../../types'
import { formatCompactLevelList, formatTimeframeTrail, formatRiskRewardLinkage } from './reviewContextSummary'

interface PositionProtectionPanelProps {
  traderId?: string
  positions?: Position[]
  language: Language
  exchange?: string
  onSymbolClick?: (symbol: string) => void
}

type ProtectionVisualStatus = 'delegated' | 'pending' | 'missing'
type OrderBucket = 'stop' | 'trailing' | 'takeProfit' | 'other'

type ProtectionRow = {
  orderId: string
  type: string
  triggerPrice: number
  callbackRate: number
  closeRatioPct: number
  valueUsdt: number
  deltaPct: number
  bucket: OrderBucket
  visualStatus: ProtectionVisualStatus
  label: string
}

function normalizeSide(side?: string): string {
  return String(side || '').toUpperCase()
}

function formatSignedPercent(value: number | undefined | null, digits = 2): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(digits)}%`
}

function classifyOrderBucket(order: OpenOrder): OrderBucket {
  const role = String(order.protection_role || '').toLowerCase()
  if (role === 'trailing') return 'trailing'
  if (role === 'take_profit') return 'takeProfit'
  if (role === 'stop_loss') return 'stop'
  const v = String(order.type || '').toUpperCase()
  if (v.includes('TRAILING')) return 'trailing'
  if (v.includes('TAKE_PROFIT') || v.includes('TP')) return 'takeProfit'
  if (v.includes('STOP')) return 'stop'
  return 'other'
}

function getVisualStatus(order: OpenOrder, bucket: OrderBucket, triggerPrice: number, liveTrailingPrice: number): ProtectionVisualStatus {
  const backendStatus = String(order.protection_status || '').toLowerCase()
  if (backendStatus === 'delegated') return 'delegated'
  if (backendStatus === 'pending_activation') return 'pending'
  if (backendStatus === 'missing') return 'missing'
  if (bucket === 'trailing') {
    return liveTrailingPrice > 0 && triggerPrice > 0 ? 'delegated' : 'pending'
  }
  return triggerPrice > 0 ? 'delegated' : 'missing'
}

function getOrderLabel(bucket: OrderBucket, language: Language): string {
  switch (bucket) {
    case 'stop': return language === 'zh' ? '止损委托' : 'Stop Order'
    case 'takeProfit': return language === 'zh' ? '止盈委托' : 'Take-profit Order'
    case 'trailing': return language === 'zh' ? '回撤/Trailing 委托' : 'Trailing Order'
    default: return language === 'zh' ? '保护委托' : 'Protection Order'
  }
}

function buildProtectionRows(position: Position, orders: OpenOrder[], language: Language) {
  const positionQty = position.quantity || 0
  const entryPrice = position.entry_price || 0

  return orders.map((order): ProtectionRow => {
    const triggerPrice = order.stop_price || order.price || 0
    const closeRatioPct = positionQty > 0 && order.quantity > 0 ? (order.quantity / positionQty) * 100 : 0
    const valueUsdt = triggerPrice > 0 && order.quantity > 0 ? triggerPrice * order.quantity : 0
    const bucket = classifyOrderBucket(order)
    const deltaPct = entryPrice > 0 && triggerPrice > 0
      ? ((triggerPrice - entryPrice) / entryPrice) * 100
      : 0
    return {
      orderId: order.order_id,
      type: String(order.type || '').toUpperCase(),
      triggerPrice,
      callbackRate: Number(order.callback_rate || 0),
      closeRatioPct,
      valueUsdt,
      deltaPct,
      bucket,
      visualStatus: getVisualStatus(order, bucket, triggerPrice, 0),
      label: getOrderLabel(bucket, language),
    }
  })
}

function compactProtectionLabel(state: string | undefined, language: Language, exchange?: string): string {
  if (!state) return language === 'zh' ? '未识别' : 'unknown'
  const v = state.toLowerCase()
  const ex = exchange ? exchange.toUpperCase() : 'EX'
  if (v === 'native_trailing_armed') return language === 'zh' ? `${ex} 原生 trailing` : `${ex} native trailing`
  if (v === 'native_partial_trailing_armed') return language === 'zh' ? `${ex} 原生分批 trailing` : `${ex} native partial trailing`
  if (v === 'managed_partial_drawdown_armed') return language === 'zh' ? '托管式分批回撤' : 'managed partial drawdown'
  if (v === 'exchange_protection_verified') return language === 'zh' ? '交易所保护已校验' : 'exchange protection verified'
  if (v === 'drawdown_triggered') return language === 'zh' ? '回撤保护已触发' : 'drawdown triggered'
  return state
}

function compactExecutionMode(mode: string | undefined, language: Language): string {
  if (!mode) return language === 'zh' ? '未确定' : 'undetermined'
  const v = mode.toLowerCase()
  if (v === 'native_trailing_full') return language === 'zh' ? '原生 trailing（整仓）' : 'native trailing (full)'
  if (v === 'native_partial_trailing') return language === 'zh' ? '原生 trailing（分批）' : 'native trailing (partial)'
  if (v === 'managed_partial_drawdown') return language === 'zh' ? '托管式回撤' : 'managed drawdown'
  if (v === 'native_trailing_pending') return language === 'zh' ? '原生 trailing 待激活' : 'native trailing pending'
  if (v === 'disabled') return language === 'zh' ? '未启用' : 'disabled'
  return mode
}

function compactRunnerStateLabel(active: boolean, stage: string | undefined, language: Language): string {
  if (!active && !stage) return language === 'zh' ? '未进入 runner' : 'inactive'
  const stageLabel = stage ? stage.replace(/_/g, ' ') : (language === 'zh' ? '运行中' : 'active')
  return active ? stageLabel : `${stageLabel} (${language === 'zh' ? '待确认' : 'pending'})`
}

function compactSourceLabel(value: string | undefined, language: Language): string {
  if (!value) return '—'
  const v = value.toLowerCase()
  if (v === 'aligned') return language === 'zh' ? '结构对齐' : 'aligned'
  if (v === 'partially_degraded') return language === 'zh' ? '部分降级' : 'partially degraded'
  if (v === 'degraded_to_full_fallback') return language === 'zh' ? '降级到 full/fallback' : 'degraded to full/fallback'
  if (v === 'structure_detached') return language === 'zh' ? '结构脱钩' : 'structure detached'
  if (v === 'unstructured') return language === 'zh' ? '未结构化' : 'unstructured'
  if (v === 'ladder_degraded') return language === 'zh' ? '梯级降级' : 'ladder degraded'
  if (v === 'degraded_to_full_fallback') return language === 'zh' ? '已降级到兜底' : 'degraded to fallback'
  if (v === 'missing_structure_context') return language === 'zh' ? '缺少结构上下文' : 'missing structure context'
  if (v === 'strategy') return language === 'zh' ? '策略' : 'strategy'
  if (v === 'ai_decision') return language === 'zh' ? 'AI 决策' : 'AI'
  if (v === 'primary_resistance') return language === 'zh' ? '主周期阻力' : 'primary resistance'
  if (v === 'primary_support') return language === 'zh' ? '主周期支撑' : 'primary support'
  if (v === 'adjacent_support_flip') return language === 'zh' ? '邻周期支撑翻转' : 'adjacent support flip'
  if (v === 'adjacent_resistance_flip') return language === 'zh' ? '邻周期阻力翻转' : 'adjacent resistance flip'
  if (v === 'support') return language === 'zh' ? '支撑' : 'support'
  if (v === 'resistance') return language === 'zh' ? '阻力' : 'resistance'
  if (v === 'swing') return language === 'zh' ? '摆动结构' : 'swing'
  if (v === 'swing_high') return language === 'zh' ? '摆动高点' : 'swing high'
  if (v === 'swing_low') return language === 'zh' ? '摆动低点' : 'swing low'
  if (v === 'fib' || v === 'fibonacci') return language === 'zh' ? '斐波那契' : 'fibonacci'
  if (v === 'fib_extension') return language === 'zh' ? '斐波延展' : 'fib extension'
  if (v === 'extension_fibonacci') return language === 'zh' ? '斐波延展目标' : 'extension fibonacci'
  if (v === 'extension_swing_trail') return language === 'zh' ? '延展摆动跟踪' : 'extension swing trail'
  if (v === 'primary_target_pullback') return language === 'zh' ? '主目标回撤' : 'primary target pullback'
  if (v === 'trend_continuation_structure') return language === 'zh' ? '趋势延续结构' : 'trend continuation structure'
  if (v === 'first_target') return language === 'zh' ? '第一目标' : 'first target'
  if (v === 'support_stop') return language === 'zh' ? '支撑止损' : 'support stop'
  if (v === 'support_target') return language === 'zh' ? '支撑目标' : 'support target'
  if (v === 'resistance_stop') return language === 'zh' ? '阻力止损' : 'resistance stop'
  if (v === 'resistance_target') return language === 'zh' ? '阻力目标' : 'resistance target'
  if (v === 'break_even') return language === 'zh' ? '保本' : 'break-even'
  if (v === 'structure') return language === 'zh' ? '结构' : 'structure'
  return value.replace(/_/g, ' ')
}

function statusBadge(status: ProtectionVisualStatus, language: Language) {
  if (status === 'delegated') return { label: language === 'zh' ? '已委托' : 'Delegated', cls: 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20' }
  if (status === 'pending') return { label: language === 'zh' ? '已委托，未激活' : 'Placed, pending', cls: 'bg-amber-500/10 text-amber-300 border-amber-500/20' }
  return { label: language === 'zh' ? '未委托' : 'Not placed', cls: 'bg-white/5 text-nofx-text-muted border-white/10' }
}

function summarizeStatus(rows: ProtectionRow[], language: Language): string {
  if (rows.length === 0) return language === 'zh' ? '未委托' : 'Not placed'
  const delegated = rows.filter((r) => r.visualStatus === 'delegated').length
  const pending = rows.filter((r) => r.visualStatus === 'pending').length
  if (delegated > 0 && pending === 0) return language === 'zh' ? `已委托 (${delegated})` : `Delegated (${delegated})`
  if (delegated > 0 || pending > 0) return language === 'zh' ? `已委托 ${delegated} / 待激活 ${pending}` : `Delegated ${delegated} / Pending ${pending}`
  return language === 'zh' ? '未委托' : 'Not placed'
}

function getAuditToggleText(audit: Position['entry_structure_audit'] | undefined, language: Language): string {
  if (!audit) return '—'
  const parts = [
    audit.audit_primary_timeframe ? 'TF' : '',
    audit.audit_adjacent_timeframes ? 'Adj' : '',
    audit.audit_support_resistance ? 'S/R' : '',
    audit.audit_structural_anchors ? 'Anchors' : '',
    audit.audit_fibonacci ? 'Fib' : '',
    audit.require_invalidation_target_linkage ? (language === 'zh' ? '失效/目标联动' : 'Inv/Target linkage') : '',
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' · ') : '—'
}

function nearestLevelMapping(price: number, candidates: Array<{ label: string; value: number }>) {
  if (!price || !Number.isFinite(price) || candidates.length === 0) return null
  let best: { label: string; value: number; diffPct: number } | null = null
  for (const candidate of candidates) {
    if (!candidate.value || !Number.isFinite(candidate.value)) continue
    const diffPct = ((price - candidate.value) / candidate.value) * 100
    if (!best || Math.abs(diffPct) < Math.abs(best.diffPct)) {
      best = { ...candidate, diffPct }
    }
  }
  return best
}

function formatLevelMapping(price: number, candidates: Array<{ label: string; value: number }>, language: Language): string {
  const nearest = nearestLevelMapping(price, candidates)
  if (!nearest) return '—'
  return `${nearest.label} ${formatPrice(nearest.value)} · ${language === 'zh' ? '偏离' : 'drift'} ${formatSignedPercent(nearest.diffPct)}`
}

function ProtectionCard({
  title,
  subtitle,
  rows,
}: {
  title: string
  subtitle?: string
  rows: { label: string; value: string }[]
}) {
  return (
    <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
      <div>
        <div className="text-sm font-medium text-nofx-text-main">{title}</div>
        {subtitle ? <div className="text-[11px] text-nofx-text-muted mt-0.5">{subtitle}</div> : null}
      </div>
      <div className="space-y-1.5 text-xs">
        {rows.map((row, idx) => (
          <div key={idx} className="flex items-start justify-between gap-3">
            <div className="text-nofx-text-muted">{row.label}</div>
            <div className="font-mono text-nofx-text-main text-right">{row.value}</div>
          </div>
        ))}
      </div>
    </div>
  )
}

function OrderGroup({
  title,
  rows,
  language,
}: {
  title: string
  rows: ProtectionRow[]
  language: Language
}) {
  if (rows.length === 0) {
    const badge = statusBadge('missing', language)
    return (
      <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium text-nofx-text-main">{title}</div>
          <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${badge.cls}`}>{badge.label}</span>
        </div>
      </div>
    )
  }

  const visible = rows.slice(0, 3)
  const hiddenCount = Math.max(0, rows.length - 3)

  return (
    <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm font-medium text-nofx-text-main">{title}</div>
        <div className="text-[11px] text-nofx-text-muted">{summarizeStatus(rows, language)}</div>
      </div>
      <div className="space-y-2">
        {visible.map((row) => {
          const badge = statusBadge(row.visualStatus, language)
          return (
            <div key={`${row.orderId}-${row.type}-${row.triggerPrice}`} className="rounded border border-white/10 bg-black/20 px-3 py-2">
              <div className="flex items-center justify-between gap-2">
                <div className="text-xs text-nofx-text-main">{row.label}</div>
                <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${badge.cls}`}>{badge.label}</span>
              </div>
              <div className="mt-1 text-[11px] text-nofx-text-muted space-y-0.5">
                <div>{language === 'zh' ? '价格' : 'Price'}: <span className="font-mono text-nofx-text-main">{formatPrice(row.triggerPrice)}</span> · {language === 'zh' ? '相对开仓' : 'vs entry'}: <span className="font-mono text-nofx-text-main">{formatSignedPercent(row.deltaPct)}</span></div>
                <div>{language === 'zh' ? '持仓比例' : 'Position ratio'}: <span className="font-mono text-nofx-text-main">{row.closeRatioPct > 0 ? `${row.closeRatioPct.toFixed(1)}%` : '—'}</span>{row.callbackRate > 0 ? <> · callback: <span className="font-mono text-nofx-text-main">{row.callbackRate.toFixed(4)}</span></> : null}</div>
              </div>
            </div>
          )
        })}
        {hiddenCount > 0 ? (
          <div className="rounded border border-dashed border-white/10 px-3 py-2 text-[11px] text-nofx-text-muted">
            {language === 'zh' ? `已折叠其余 ${hiddenCount} 组委托` : `${hiddenCount} more groups folded`}
          </div>
        ) : null}
      </div>
    </div>
  )
}

export function PositionProtectionPanel({ traderId, positions, language, exchange, onSymbolClick }: PositionProtectionPanelProps) {
  const [ordersBySymbol, setOrdersBySymbol] = useState<Record<string, OpenOrder[]>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const symbolKeys = useMemo(() => {
    const keys = new Set<string>()
    for (const pos of positions || []) {
      keys.add(String(pos.symbol || '').toUpperCase())
    }
    return [...keys]
  }, [positions])

  useEffect(() => {
    let cancelled = false

    async function load() {
      if (!traderId || !positions || positions.length === 0) {
        setOrdersBySymbol({})
        setError(null)
        return
      }

      setLoading(true)
      setError(null)
      try {
        const entries = await Promise.all(
          symbolKeys.map(async (symbol) => {
            const data = await api.getOpenOrders(traderId, symbol)
            return [symbol, Array.isArray(data) ? data : []] as const
          })
        )
        if (!cancelled) {
          setOrdersBySymbol(Object.fromEntries(entries))
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load protection orders')
          setOrdersBySymbol({})
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    load()
    const timer = window.setInterval(load, 60000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [traderId, positions, symbolKeys])

  if (!positions || positions.length === 0) {
    return (
      <div className="nofx-glass p-5 animate-slide-in relative overflow-hidden group" style={{ animationDelay: '0.18s' }}>
        <div className="relative z-10">
          <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2 mb-3">
            <span className="text-purple-400">🛡</span>
            {language === 'zh' ? '持仓保护执行面板' : 'Position Protection Runtime'}
          </h3>
          <div className="rounded-lg border border-white/10 bg-black/20 px-4 py-5 text-sm text-nofx-text-muted">
            {language === 'zh' ? '当前没有持仓。' : 'No open positions at the moment.'}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="nofx-glass p-5 animate-slide-in relative overflow-hidden group" style={{ animationDelay: '0.18s' }}>
      <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
        <div className="w-24 h-24 rounded-full bg-purple-500 blur-3xl" />
      </div>

      <div className="relative z-10 flex items-center justify-between mb-3">
        <div>
          <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2">
            <span className="text-purple-400">🛡</span>
            {language === 'zh' ? '持仓保护执行面板' : 'Position Protection Runtime'}
          </h3>
          <p className="text-[11px] text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '统一展示：保护是否已委托、是否待激活、每档价格偏移与持仓比例。超过 3 组自动折叠。'
              : 'Unified view of delegated / pending / missing protection, with price delta vs entry and position ratio. More than 3 groups are folded.'}
          </p>
        </div>
      </div>

      <div className="space-y-4 relative z-10">
        {positions.map((position, index) => {
          const symbol = String(position.symbol || '').toUpperCase()
          const side = normalizeSide(position.side)
          const symbolOrders = ordersBySymbol[symbol] || []
          const filteredOrders = symbolOrders.filter((order) => {
            const orderPosSide = normalizeSide(order.position_side)
            return !orderPosSide || orderPosSide === side
          })
          const protectionRows = buildProtectionRows(position, filteredOrders, language)
          const currentPnlPct = Number(position.protection_runtime?.current_pnl_pct ?? position.unrealized_pnl_pct ?? 0)
          const currentDrawdownPct = Number(position.protection_runtime?.current_drawdown_pct ?? 0)
          const peakPnlPct = Number(position.protection_runtime?.drawdown_peak_pnl_pct ?? currentPnlPct)
          const runtimeTiers = position.protection_runtime?.scheduled_tiers || []
          const currentStageMinProfit = Number(position.protection_runtime?.current_drawdown_stage_min_profit_pct ?? 0)
          const currentStageRuleCount = Number(position.protection_runtime?.current_drawdown_stage_rule_count ?? 0)
          const currentDrawdownStage = String(position.protection_runtime?.current_drawdown_stage || runtimeTiers.find((tier) => tier.is_satisfied)?.drawdown_stage || '')
          const structureStage = String(position.protection_runtime?.drawdown_structure_stage || '')
          const structureStopSource = String(position.protection_runtime?.drawdown_structure_stop_source || '')
          const structureTargetSource = String(position.protection_runtime?.drawdown_structure_target_source || '')
          const structureTargetProgress = Number(position.protection_runtime?.drawdown_structure_target_progress ?? 0)
          const structurePrimaryTf = String(position.protection_runtime?.drawdown_structure_primary_timeframe || '')
          const structureEvidence = position.protection_runtime?.drawdown_structure_evidence || []
          const structureTrace = position.protection_runtime?.drawdown_structure_trace || []
          const structureHealth = String(position.protection_runtime?.structure_protection_health || 'unstructured')
          const structureDriftReason = String(position.protection_runtime?.structure_protection_drift_reason || '')
          const structureDetached = Boolean(position.protection_runtime?.structure_protection_detached)
          const drawdownConfigSource = String(position.protection_runtime?.drawdown_config_source || 'strategy')
          const satisfiedTiers = runtimeTiers.filter((tier) => Boolean(tier.is_satisfied))
          const triggeredTiers = runtimeTiers.filter((tier) => Boolean(tier.is_triggered))
          const nextTier = runtimeTiers.find((tier) => !tier.is_satisfied) || runtimeTiers[0] || null
          const breakEvenTriggerPct = Number(position.protection_runtime?.current_break_even_trigger_pct ?? 0)
          const breakEvenGapPct = Number(position.protection_runtime?.next_break_even_gap_pct ?? 0)
          const breakEvenOffsetPct = Number(position.protection_runtime?.break_even_offset_pct ?? 0)
          const breakEvenConfigSource = String(position.protection_runtime?.break_even_config_source || 'strategy')
          const liveBreakEvenStopPrice = Number(position.protection_runtime?.live_break_even_stop_price ?? 0)
          const breakEvenOrderDetected = Boolean(position.protection_runtime?.break_even_order_detected)
          const runnerState = position.protection_runtime?.runner_state
          const runnerActive = Boolean(position.protection_runtime?.runner_mode_active ?? runnerState?.active ?? nextTier?.runner_mode_active)
          const runnerKeepPct = Number(position.protection_runtime?.runner_keep_pct ?? runnerState?.keep_pct ?? nextTier?.runner_keep_pct ?? 0)
          const runnerStopMode = String(position.protection_runtime?.runner_stop_mode || runnerState?.stop_mode || nextTier?.runner_stop_mode || '')
          const runnerStopPrice = Number(position.protection_runtime?.runner_stop_price ?? runnerState?.stop_price ?? nextTier?.runner_stop_price ?? 0)
          const runnerStopSource = String(position.protection_runtime?.runner_stop_source || runnerState?.stop_source || nextTier?.runner_stop_source || '')
          const runnerTargetMode = String(position.protection_runtime?.runner_target_mode || runnerState?.target_mode || nextTier?.runner_target_mode || '')
          const runnerTargetPrice = Number(position.protection_runtime?.runner_target_price ?? runnerState?.target_price ?? nextTier?.runner_target_price ?? 0)
          const runnerTargetSource = String(position.protection_runtime?.runner_target_source || runnerState?.target_source || nextTier?.runner_target_source || '')
          const breakEvenSuppressedByRunner = Boolean(position.protection_runtime?.break_even_suppressed_by_runner ?? runnerState?.break_even_suppressed ?? nextTier?.break_even_suppressed_by_runner)
          const runnerStage = String(runnerState?.stage || nextTier?.drawdown_stage || currentDrawdownStage || '')
          const entryReviewSummary = position.entry_review_summary
          const entryStructureAudit = position.entry_structure_audit
          const entryTf = entryReviewSummary?.timeframe_context as { primary?: string; lower?: string[]; higher?: string[] } | undefined
          const entryRR = entryReviewSummary?.risk_reward as { entry?: number; invalidation?: number; first_target?: number } | undefined
          const entryLevels = entryReviewSummary?.key_levels as { support?: number[]; resistance?: number[]; swing_lows?: number[]; swing_highs?: number[]; fibonacci?: { swing_low?: number; swing_high?: number; levels?: number[] } } | undefined
          const fibSummary = entryLevels?.fibonacci
          const entryReviewContext = entryReviewSummary
            ? {
                timeframe_context: entryTf,
                risk_reward: entryRR,
                key_levels: entryLevels,
              }
            : undefined
          const timeframeTrail = formatTimeframeTrail(entryReviewContext as never)
          const rrLinkage = formatRiskRewardLinkage(entryReviewContext?.risk_reward as never)
          const supportSummary = formatCompactLevelList(entryLevels?.support)
          const resistanceSummary = formatCompactLevelList(entryLevels?.resistance)
          const structureCandidates = [
            ...(entryLevels?.support || []).map((value, idx) => ({ label: `S${idx + 1}`, value })),
            ...(entryLevels?.resistance || []).map((value, idx) => ({ label: `R${idx + 1}`, value })),
            ...((entryLevels?.swing_lows || []).map((value, idx) => ({ label: `swingL${idx + 1}`, value }))),
            ...((entryLevels?.swing_highs || []).map((value, idx) => ({ label: `swingH${idx + 1}`, value }))),
            ...((fibSummary?.levels || []).map((value, idx) => ({ label: `fib${idx + 1}`, value }))),
            ...(entryRR?.invalidation ? [{ label: 'invalid', value: entryRR.invalidation }] : []),
            ...(entryRR?.first_target ? [{ label: 'target', value: entryRR.first_target }] : []),
          ]
          const linkageStatus = (() => {
            if (!entryStructureAudit?.require_invalidation_target_linkage || !entryRR) return null
            const supports = (entryLevels?.support || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const resistances = (entryLevels?.resistance || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const swingLows = ((entryLevels as { swing_lows?: number[] } | undefined)?.swing_lows || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const swingHighs = ((entryLevels as { swing_highs?: number[] } | undefined)?.swing_highs || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const fib = (entryLevels as { fibonacci?: { swing_low?: number; swing_high?: number; levels?: number[] } } | undefined)?.fibonacci
            const fibLevels = (fib?.levels || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            if (fib?.swing_low) fibLevels.push(fib.swing_low)
            if (fib?.swing_high) fibLevels.push(fib.swing_high)
            if (!entryRR.entry || !entryRR.invalidation || !entryRR.first_target) return language === 'zh' ? '缺失' : 'Missing'
            const riskDist = Math.abs(entryRR.entry - entryRR.invalidation)
            const targetDist = Math.abs(entryRR.first_target - entryRR.entry)
            const tol = Math.max(0.0001, Math.min(Math.max(riskDist, targetDist) * 0.35, Math.max(entryRR.entry, entryRR.first_target, entryRR.invalidation) * 0.02))
            const invalidation = entryRR.invalidation
            const firstTarget = entryRR.first_target
            const invalidLinked = [...supports, ...swingLows, ...fibLevels].some((v) => Math.abs(v - invalidation) <= tol)
            const targetLinked = [...resistances, ...swingHighs, ...fibLevels].some((v) => Math.abs(v - firstTarget) <= tol)
            if (invalidLinked && targetLinked) return language === 'zh' ? '已联动' : 'Linked'
            if (invalidLinked || targetLinked) return language === 'zh' ? '部分联动' : 'Partial'
            return language === 'zh' ? '缺失' : 'Missing'
          })()
          const plannedLadderStopCount = Number(position.protection_runtime?.planned_ladder_stop_count ?? 0)
          const plannedLadderTakeProfitCount = Number(position.protection_runtime?.planned_ladder_take_profit_count ?? 0)
          const liveLadderStopCount = Number(position.protection_runtime?.live_ladder_stop_count ?? 0)
          const liveLadderTakeProfitCount = Number(position.protection_runtime?.live_ladder_take_profit_count ?? 0)
          const liveFullStopCount = Number(position.protection_runtime?.live_full_stop_count ?? 0)
          const liveFullTakeProfitCount = Number(position.protection_runtime?.live_full_take_profit_count ?? 0)
          const liveFallbackStopCount = Number(position.protection_runtime?.live_fallback_stop_count ?? 0)
          const fallbackOrderDetected = Boolean(position.protection_runtime?.fallback_order_detected)
          const fullStopPlanned = Boolean(position.protection_runtime?.full_stop_planned)
          const fullTakeProfitPlanned = Boolean(position.protection_runtime?.full_take_profit_planned)
          const fallbackPlanned = Boolean(position.protection_runtime?.fallback_planned)
          const ladderStopDegraded = Boolean(position.protection_runtime?.ladder_stop_degraded)
          const ladderTakeProfitDegraded = Boolean(position.protection_runtime?.ladder_take_profit_degraded)
          const ladderStopDegradedToFull = Boolean(position.protection_runtime?.ladder_stop_degraded_to_full)
          const ladderTakeProfitDegradedToFull = Boolean(position.protection_runtime?.ladder_take_profit_degraded_to_full)

          const stopRows = protectionRows.filter((r) => r.bucket === 'stop')
          const trailingRows = protectionRows.filter((r) => r.bucket === 'trailing')
          const tpRows = protectionRows.filter((r) => r.bucket === 'takeProfit')
          const liveTrailingPrice = trailingRows.length > 0 ? trailingRows[0].triggerPrice : 0

          return (
            <div key={`${symbol}-${side}-${index}`} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div>
                  <div className="font-semibold text-nofx-text-main">
                    <button type="button" onClick={() => onSymbolClick?.(symbol)} className="hover:text-cyan-300 transition-colors">
                      {symbol} / {side}
                    </button>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {language === 'zh' ? '简洁保护视图：状态、来源、价格偏移、持仓比例' : 'Simplified protection view: status, source, price delta, and position ratio'}
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-2 text-xs min-w-[260px]">
                  <div className="rounded border border-white/10 px-3 py-2 bg-black/20">
                    <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '数量' : 'Qty'}</div>
                    <div className="font-mono text-nofx-text-main">{formatQuantity(position.quantity)}</div>
                  </div>
                  <div className="rounded border border-white/10 px-3 py-2 bg-black/20">
                    <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '开仓价' : 'Entry'}</div>
                    <div className="font-mono text-nofx-text-main">{formatPrice(position.entry_price)}</div>
                  </div>
                  <div className="rounded border border-white/10 px-3 py-2 bg-black/20">
                    <div className="text-nofx-text-muted mb-1">PnL %</div>
                    <div className={`font-mono ${position.unrealized_pnl_pct >= 0 ? 'text-nofx-green' : 'text-nofx-red'}`}>
                      {position.unrealized_pnl_pct >= 0 ? '+' : ''}{position.unrealized_pnl_pct.toFixed(2)}%
                    </div>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                <ProtectionCard
                  title={language === 'zh' ? '开仓结构摘要' : 'Entry Structure Summary'}
                  subtitle={language === 'zh' ? '直接看这笔仓位开仓时的主/邻周期、RR 与关键价位' : 'Inspect the primary/adjacent timeframes, RR, and key levels used at entry'}
                  rows={[
                    { label: language === 'zh' ? '决策周期' : 'Decision Cycle', value: position.entry_decision_cycle ? String(position.entry_decision_cycle) : '—' },
                    { label: language === 'zh' ? '周期' : 'Timeframes', value: timeframeTrail.length > 0 ? timeframeTrail.join(' · ') : '—' },
                    { label: language === 'zh' ? 'Entry / 失效 / 目标' : 'Entry / Invalidation / Target', value: rrLinkage.length > 0 ? rrLinkage.join(' · ') : '—' },
                    { label: language === 'zh' ? '主/邻周期' : 'Primary/Adjacent TF', value: timeframeTrail.length > 0 ? timeframeTrail.join(' · ') : '—' },
                    { label: language === 'zh' ? '支撑位' : 'Support', value: entryStructureAudit?.audit_support_resistance ? (supportSummary.length ? supportSummary.join(' / ') : '—') : (language === 'zh' ? '已隐藏' : 'Hidden') },
                    { label: language === 'zh' ? '阻力位' : 'Resistance', value: entryStructureAudit?.audit_support_resistance ? (resistanceSummary.length ? resistanceSummary.join(' / ') : '—') : (language === 'zh' ? '已隐藏' : 'Hidden') },
                    { label: language === 'zh' ? '摆动高/低' : 'Swing High/Low', value: `${(entryLevels?.swing_highs || []).join(', ') || '—'} / ${(entryLevels?.swing_lows || []).join(', ') || '—'}` },
                    { label: language === 'zh' ? '斐波那契' : 'Fibonacci', value: fibSummary ? `${(fibSummary.levels || []).join(', ') || '—'}${fibSummary.swing_low ? ` | low ${fibSummary.swing_low}` : ''}${fibSummary.swing_high ? ` | high ${fibSummary.swing_high}` : ''}` : '—' },
                    { label: language === 'zh' ? '结构联动' : 'Structure Linkage', value: linkageStatus || '—' },
                    { label: language === 'zh' ? '联动来源' : 'Linkage Sources', value: (() => {
                      if (!entryStructureAudit?.require_invalidation_target_linkage || !entryRR) return '—'
                      const supports = (entryLevels?.support || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
                      const resistances = (entryLevels?.resistance || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
                      const swingLows = ((entryLevels as { swing_lows?: number[] } | undefined)?.swing_lows || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
                      const swingHighs = ((entryLevels as { swing_highs?: number[] } | undefined)?.swing_highs || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
                      const fib = (entryLevels as { fibonacci?: { swing_low?: number; swing_high?: number; levels?: number[] } } | undefined)?.fibonacci
                      const fibLevels = (fib?.levels || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
                      if (fib?.swing_low) fibLevels.push(fib.swing_low)
                      if (fib?.swing_high) fibLevels.push(fib.swing_high)
                      if (!entryRR.entry || !entryRR.invalidation || !entryRR.first_target) return '—'
                      const riskDist = Math.abs(entryRR.entry - entryRR.invalidation)
                      const targetDist = Math.abs(entryRR.first_target - entryRR.entry)
                      const tol = Math.max(0.0001, Math.min(Math.max(riskDist, targetDist) * 0.35, Math.max(entryRR.entry, entryRR.first_target, entryRR.invalidation) * 0.02))
                      const findSource = (target: number, groups: Array<[string, number[]]>) => {
                        let best: { name: string; dist: number } | null = null
                        for (const [name, values] of groups) {
                          for (const value of values) {
                            const dist = Math.abs(value - target)
                            if (dist > tol) continue
                            if (!best || dist < best.dist) best = { name, dist }
                          }
                        }
                        return best?.name
                      }
                      const invalidSource = findSource(entryRR.invalidation, [['support', supports], ['swing_low', swingLows], ['fib', fibLevels]])
                      const targetSource = findSource(entryRR.first_target, [['resistance', resistances], ['swing_high', swingHighs], ['fib', fibLevels]])
                      const parts = [invalidSource ? `invalid↔${invalidSource}` : '', targetSource ? `target↔${targetSource}` : ''].filter(Boolean)
                      return parts.length > 0 ? parts.join(' · ') : '—'
                    })() },
                    { label: language === 'zh' ? '审计开关' : 'Audit Toggles', value: getAuditToggleText(entryStructureAudit, language) },
                  ]}
                />

                <ProtectionCard
                  title={language === 'zh' ? '保护总览' : 'Protection Overview'}
                  subtitle={language === 'zh' ? '先看整体，再看具体委托' : 'Read the overall state first, then inspect individual orders'}
                  rows={[
                    { label: language === 'zh' ? '保护状态' : 'Protection State', value: compactProtectionLabel(position.protection_state, language, exchange) },
                    { label: language === 'zh' ? 'Drawdown 模式' : 'Drawdown Mode', value: compactExecutionMode(position.drawdown_execution_mode, language) },
                    { label: language === 'zh' ? '当前利润' : 'Current PnL', value: formatSignedPercent(currentPnlPct) },
                    { label: language === 'zh' ? '峰值 / 回撤' : 'Peak / Drawdown', value: `${formatSignedPercent(peakPnlPct)} / ${currentDrawdownPct.toFixed(2)}%` },
                    { label: language === 'zh' ? '回撤来源' : 'Drawdown Source', value: drawdownConfigSource },
                    { label: language === 'zh' ? '当前档位' : 'Current Stage', value: currentStageMinProfit > 0 ? `${currentStageMinProfit.toFixed(2)}% (${currentStageRuleCount})` : '—' },
                    { label: language === 'zh' ? '阶段标识' : 'Stage Label', value: currentDrawdownStage ? currentDrawdownStage.replace(/_/g, ' ') : '—' },
                    { label: language === 'zh' ? '结构阶段' : 'Structure Stage', value: structureStage ? `${structureStage.replace(/_/g, ' ')} · ${compactSourceLabel(structureHealth, language)}` : compactSourceLabel(structureHealth, language) },
                    { label: language === 'zh' ? '目标进度' : 'Target Progress', value: structureTargetProgress > 0 ? `${(structureTargetProgress * 100).toFixed(1)}%${structurePrimaryTf ? ` · ${structurePrimaryTf}` : ''}` : '—' },
                    { label: language === 'zh' ? '结构来源' : 'Structure Sources', value: structureEvidence.length > 0 ? structureEvidence.map((v) => compactSourceLabel(v.replace(/^anchor:/, ''), language)).join(' · ') : '—' },
                    { label: language === 'zh' ? '偏离原因' : 'Drift Reason', value: structureDriftReason ? compactSourceLabel(structureDriftReason, language) : (structureDetached ? compactSourceLabel('structure_detached', language) : '—') },
                    { label: language === 'zh' ? '结构执行轨迹' : 'Structure Trace', value: structureTrace.length > 0 ? structureTrace.join(' | ') : '—' },
                    { label: language === 'zh' ? '结构止损/目标' : 'Structure Stop/Target', value: structureStopSource || structureTargetSource ? `${compactSourceLabel(structureStopSource, language)} / ${compactSourceLabel(structureTargetSource, language)}` : '—' },
                    { label: language === 'zh' ? 'Runner止损映射' : 'Runner Stop Mapping', value: runnerStopPrice > 0 ? formatLevelMapping(runnerStopPrice, structureCandidates, language) : '—' },
                    { label: language === 'zh' ? 'Runner目标映射' : 'Runner Target Mapping', value: runnerTargetPrice > 0 ? formatLevelMapping(runnerTargetPrice, structureCandidates, language) : '—' },
                    { label: language === 'zh' ? '最近委托结构映射' : 'Nearest Order Structure Map', value: stopRows[0]?.triggerPrice ? formatLevelMapping(stopRows[0].triggerPrice, structureCandidates, language) : (tpRows[0]?.triggerPrice ? formatLevelMapping(tpRows[0].triggerPrice, structureCandidates, language) : '—') },
                    { label: language === 'zh' ? '满足 / 触发' : 'Satisfied / Triggered', value: `${satisfiedTiers.length} / ${triggeredTiers.length}` },
                    { label: language === 'zh' ? '下一档利润门槛' : 'Next Gate', value: nextTier ? `${Number(nextTier.min_profit_pct || 0).toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? 'Runner 状态' : 'Runner State', value: compactRunnerStateLabel(runnerActive, runnerStage, language) },
                    { label: language === 'zh' ? 'Runner 保留' : 'Runner Keep', value: runnerKeepPct > 0 ? `${runnerKeepPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? 'Runner 止损' : 'Runner Stop', value: runnerStopPrice > 0 ? `${formatPrice(runnerStopPrice)} · ${compactSourceLabel(runnerStopSource || runnerStopMode, language)}` : (runnerStopSource || runnerStopMode ? compactSourceLabel(runnerStopSource || runnerStopMode, language) : '—') },
                    { label: language === 'zh' ? 'Runner 目标' : 'Runner Target', value: runnerTargetPrice > 0 ? `${formatPrice(runnerTargetPrice)} · ${compactSourceLabel(runnerTargetSource || runnerTargetMode, language)}` : (runnerTargetSource || runnerTargetMode ? compactSourceLabel(runnerTargetSource || runnerTargetMode, language) : '—') },
                    { label: language === 'zh' ? 'Trailing 实盘委托' : 'Live Trailing Orders', value: `${trailingRows.length}` },
                    { label: language === 'zh' ? 'Ladder 止损' : 'Ladder Stops', value: plannedLadderStopCount > 0 ? `${liveLadderStopCount} / ${plannedLadderStopCount}` : '—' },
                    { label: language === 'zh' ? 'Ladder 止盈' : 'Ladder Take-profits', value: plannedLadderTakeProfitCount > 0 ? `${liveLadderTakeProfitCount} / ${plannedLadderTakeProfitCount}` : '—' },
                    { label: language === 'zh' ? '降级摘要' : 'Degradation Summary', value: [
                      ladderStopDegraded ? (ladderStopDegradedToFull ? (language === 'zh' ? 'SL→Full' : 'SL→Full') : (language === 'zh' ? 'SL部分降级' : 'SL partial')) : '',
                      ladderTakeProfitDegraded ? (ladderTakeProfitDegradedToFull ? (language === 'zh' ? 'TP→Full' : 'TP→Full') : (language === 'zh' ? 'TP部分降级' : 'TP partial')) : '',
                      fallbackOrderDetected ? (language === 'zh' ? 'Fallback已挂' : 'Fallback live') : '',
                      fallbackPlanned && !fallbackOrderDetected ? (language === 'zh' ? 'Fallback待挂' : 'Fallback planned') : '',
                      !ladderStopDegraded && !ladderTakeProfitDegraded && !fallbackOrderDetected ? (language === 'zh' ? '无' : 'None') : ''
                    ].filter(Boolean).join(' · ') },
                  ]}
                />

                <ProtectionCard
                  title={language === 'zh' ? 'Break-even 总览' : 'Break-even Overview'}
                  subtitle={language === 'zh' ? '看是否启用、来自哪里、是否已经挂单' : 'Check whether it is active, where it comes from, and whether it is already placed'}
                  rows={[
                    { label: language === 'zh' ? '当前状态' : 'State', value: position.break_even_state || 'idle' },
                    { label: language === 'zh' ? '配置来源' : 'Config Source', value: breakEvenConfigSource },
                    { label: language === 'zh' ? '触发阈值' : 'Trigger Threshold', value: breakEvenTriggerPct > 0 ? `${breakEvenTriggerPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '距触发还差' : 'Gap to Trigger', value: breakEvenTriggerPct > 0 ? `${breakEvenGapPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '保本偏移' : 'Offset', value: breakEvenTriggerPct > 0 ? `${breakEvenOffsetPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? 'Runner 抑制' : 'Runner Suppression', value: breakEvenSuppressedByRunner ? (language === 'zh' ? '已抑制机械 BE' : 'suppressed by runner') : '—' },
                    { label: language === 'zh' ? '实盘挂单' : 'Live Order', value: breakEvenOrderDetected ? (language === 'zh' ? '已委托' : 'Delegated') : (breakEvenTriggerPct > 0 ? (language === 'zh' ? '未委托' : 'Not placed') : '—') },
                    { label: language === 'zh' ? '保本价' : 'Break-even Price', value: liveBreakEvenStopPrice > 0 ? `${formatPrice(liveBreakEvenStopPrice)} / ${formatSignedPercent(((liveBreakEvenStopPrice - (position.entry_price || 0)) / (position.entry_price || 1)) * 100)}` : '—' },
                    { label: language === 'zh' ? 'Full/Fallback 状态' : 'Full/Fallback State', value: [
                      fullStopPlanned ? `${language === 'zh' ? 'Full SL计划' : 'Full SL planned'}:${liveFullStopCount}` : '',
                      fullTakeProfitPlanned ? `${language === 'zh' ? 'Full TP计划' : 'Full TP planned'}:${liveFullTakeProfitCount}` : '',
                      fallbackPlanned ? `${language === 'zh' ? 'Fallback计划' : 'Fallback planned'}:${liveFallbackStopCount}` : ''
                    ].filter(Boolean).join(' · ') || '—' },
                  ]}
                />

                <OrderGroup title={language === 'zh' ? '止损委托' : 'Stop Orders'} rows={stopRows} language={language} />
                <OrderGroup title={language === 'zh' ? '回撤 / Trailing 委托' : 'Drawdown / Trailing Orders'} rows={trailingRows.map((r) => ({ ...r, visualStatus: r.visualStatus === 'missing' ? getVisualStatus({} as OpenOrder, 'trailing', r.triggerPrice, liveTrailingPrice) : r.visualStatus }))} language={language} />
                <OrderGroup title={language === 'zh' ? '止盈委托' : 'Take-profit Orders'} rows={tpRows} language={language} />
              </div>
            </div>
          )
        })}

        {loading && <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '正在刷新保护状态…' : 'Refreshing protection state…'}</div>}
        {error && <div className="text-xs text-nofx-red">{error}</div>}
      </div>
    </div>
  )
}
