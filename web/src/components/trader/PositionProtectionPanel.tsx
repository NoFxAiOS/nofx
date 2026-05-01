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
  source: string
}

// ── Helper functions (unchanged) ──────────────────────────────────────

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
    case 'stop': return language === 'zh' ? '止损' : 'Stop Loss'
    case 'takeProfit': return language === 'zh' ? '止盈' : 'Take Profit'
    case 'trailing': return language === 'zh' ? 'Trailing' : 'Trailing'
    default: return language === 'zh' ? '保护' : 'Protection'
  }
}

function getOrderSourceLabel(order: OpenOrder, language: Language): string {
  const id = String(order.client_order_id || '').toLowerCase()
  if (id.includes('fallback_maxloss')) return language === 'zh' ? '兜底止损' : 'Fallback SL'
  if (id.includes('full_sl')) return language === 'zh' ? '全仓止损' : 'Full SL'
  if (id.includes('full_tp')) return language === 'zh' ? '全仓止盈' : 'Full TP'
  if (id.includes('ladder_sl')) return language === 'zh' ? 'Ladder 止损' : 'Ladder SL'
  if (id.includes('ladder_tp')) return language === 'zh' ? 'Ladder 止盈' : 'Ladder TP'
  if (id.includes('break_even') || id.includes('breakeven')) return language === 'zh' ? '保本止损' : 'Break-even SL'
  if (id.includes('drawdown') || id.includes('trailing')) return language === 'zh' ? '回撤跟踪' : 'Drawdown trailing'
  return ''
}

function buildProtectionRows(position: Position, orders: OpenOrder[], language: Language) {
  const positionQty = position.quantity || 0
  const entryPrice = position.entry_price || 0
  return orders.map((order): ProtectionRow => {
    const triggerPrice = order.stop_price || order.price || 0
    const closeRatioPct = positionQty > 0 && order.quantity > 0 ? (order.quantity / positionQty) * 100 : 0
    const valueUsdt = triggerPrice > 0 && order.quantity > 0 ? triggerPrice * order.quantity : 0
    const bucket = classifyOrderBucket(order)
    const deltaPct = entryPrice > 0 && triggerPrice > 0 ? ((triggerPrice - entryPrice) / entryPrice) * 100 : 0
    return {
      orderId: order.order_id,
      type: String(order.type || '').toUpperCase(),
      triggerPrice, callbackRate: Number(order.callback_rate || 0),
      closeRatioPct, valueUsdt, deltaPct, bucket,
      visualStatus: getVisualStatus(order, bucket, triggerPrice, 0),
      label: getOrderLabel(bucket, language),
      source: getOrderSourceLabel(order, language),
    }
  })
}

function formatLadderPlanLine(rule: any, idx: number, language: Language): string {
  const tpPrice = Number(rule.take_profit_price ?? rule.Price ?? rule.price ?? 0)
  const tpPct = Number(rule.take_profit_pct ?? 0)
  const tpRatio = Number(rule.take_profit_close_ratio_pct ?? rule.CloseRatioPct ?? rule.close_ratio_pct ?? 0)
  const slPrice = Number(rule.stop_loss_price ?? rule.Price ?? rule.price ?? 0)
  const slPct = Number(rule.stop_loss_pct ?? 0)
  const slRatio = Number(rule.stop_loss_close_ratio_pct ?? rule.CloseRatioPct ?? rule.close_ratio_pct ?? 0)
  const buf = Number(rule.volatility_buffer_pct ?? 0)
  const left = language === 'zh' ? `#${idx + 1}` : `#${idx + 1}`
  const parts: string[] = []
  if (tpPrice > 0 || tpPct > 0 || tpRatio > 0) {
    parts.push(`TP ${tpPrice > 0 ? formatPrice(tpPrice) : '—'}${tpPct > 0 ? ` (${tpPct.toFixed(2)}%)` : ''}${tpRatio > 0 ? ` · ${tpRatio.toFixed(0)}%` : ''}`)
  }
  if (slPrice > 0 || slPct > 0 || slRatio > 0) {
    parts.push(`SL ${slPrice > 0 ? formatPrice(slPrice) : '—'}${slPct > 0 ? ` (${slPct.toFixed(2)}%)` : ''}${slRatio > 0 ? ` · ${slRatio.toFixed(0)}%` : ''}`)
  }
  if (buf > 0) parts.push(`${language === 'zh' ? '缓冲' : 'buffer'} ${buf.toFixed(2)}%`)
  return `${left} ${parts.join(' / ') || '—'}`
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
  if (v === 'degraded_to_full_fallback') return language === 'zh' ? '已降级到兜底' : 'degraded to fallback'
  if (v === 'structure_detached') return language === 'zh' ? '结构脱钩' : 'structure detached'
  if (v === 'unstructured') return language === 'zh' ? '未结构化' : 'unstructured'
  if (v === 'ladder_degraded') return language === 'zh' ? '梯级降级' : 'ladder degraded'
  if (v === 'runner_migration_needed') return language === 'zh' ? 'Runner 需迁移' : 'runner migration needed'
  if (v === 'higher_timeframe_runner') return language === 'zh' ? '高周期 Runner' : 'higher timeframe runner'
  if (v === 'higher_timeframe_structure_trail') return language === 'zh' ? '高周期结构跟踪' : 'higher timeframe trail'
  if (v === 'higher_timeframe_runner_target') return language === 'zh' ? '高周期 Runner 目标' : 'higher timeframe runner target'
  if (v === 'missing_higher_runner_anchor') return language === 'zh' ? '缺少高周期 Runner 锚点' : 'missing higher runner anchor'
  if (v === 'missing_live_trailing') return language === 'zh' ? '缺少实盘 trailing' : 'missing live trailing'
  if (v === 'live_trailing_differs_from_higher_runner_plan') return language === 'zh' ? '实盘 trailing 偏离高周期计划' : 'live trailing differs from higher runner plan'
  if (v === 'tightens_or_preserves_live_trailing') return language === 'zh' ? '收紧或保持当前保护' : 'tightens/preserves live trailing'
  if (v === 'would_loosen_live_trailing') return language === 'zh' ? '会放宽当前保护' : 'would loosen live trailing'
  if (v === 'migration_not_safe') return language === 'zh' ? '迁移不安全' : 'migration not safe'
  if (v === 'invalid_desired_trailing_plan') return language === 'zh' ? '目标 trailing 计划无效' : 'invalid desired trailing plan'
  if (v === 'manual_replace_ready') return language === 'zh' ? '可人工替换' : 'manual replace ready'
  if (v === 'replace_native_trailing') return language === 'zh' ? '替换原生 trailing' : 'replace native trailing'
  if (v === 'protection_order_quantity_exceeds_position') return language === 'zh' ? '保护单覆盖剩余仓位' : 'protection order covers remaining position'
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
  if (status === 'pending') return { label: language === 'zh' ? '待激活' : 'Pending', cls: 'bg-amber-500/10 text-amber-300 border-amber-500/20' }
  return { label: language === 'zh' ? '未委托' : 'Missing', cls: 'bg-white/5 text-nofx-text-muted border-white/10' }
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



function getAuditToggleText(audit: Position['entry_structure_audit'] | undefined, language: Language): string {
  if (!audit) return '—'
  const parts = [
    audit.audit_primary_timeframe ? 'TF' : '',
    audit.audit_adjacent_timeframes ? 'Adj' : '',
    audit.audit_support_resistance ? 'S/R' : '',
    audit.audit_structural_anchors ? 'Anchors' : '',
    audit.audit_fibonacci ? 'Fib' : '',
    audit.require_invalidation_target_linkage ? (language === 'zh' ? '联动' : 'Linkage') : '',
  ].filter(Boolean)
  return parts.length > 0 ? parts.join(' · ') : '—'
}

// ── Structural alignment badge ────────────────────────────────────────

function compactStructureLabel(label: string, language: Language): string {
  if (label === 'inv') return language === 'zh' ? '失效位' : 'invalidation'
  if (label === 'tgt') return language === 'zh' ? '目标位' : 'target'
  if (label.startsWith('S')) return language === 'zh' ? `支撑${label.slice(1)}` : label
  if (label.startsWith('R')) return language === 'zh' ? `阻力${label.slice(1)}` : label
  if (label.startsWith('swL')) return language === 'zh' ? `摆动低点${label.slice(3)}` : label
  if (label.startsWith('swH')) return language === 'zh' ? `摆动高点${label.slice(3)}` : label
  return label
}

function structuralBufferBadge(price: number, candidates: Array<{ label: string; value: number }>, language: Language): { text: string; cls: string; title: string } {
  const nearest = nearestLevelMapping(price, candidates)
  if (!nearest) return { text: '—', cls: 'text-nofx-text-muted', title: language === 'zh' ? '没有可比较的结构位' : 'No structure level available' }
  const absDiff = Math.abs(nearest.diffPct)
  const label = compactStructureLabel(nearest.label, language)
  const suffix = language === 'zh' ? `${label} · 缓冲 ${absDiff.toFixed(1)}%` : `${label} · ${absDiff.toFixed(1)}% buffer`
  const title = language === 'zh'
    ? `离最近结构位 ${label} 约 ${absDiff.toFixed(2)}%。这里衡量的是保护价相对结构位预留的有效突破/跌破缓冲，不是“越贴越好”。⚪ <0.05% 基本贴线，容易被针刺/噪声触发；✅ 0.05–1.0% 通常属于可接受结构缓冲；⚠️ 1.0–2.0% 缓冲偏宽，需要 ATR/波动解释；❌ >2.0% 可能脱离原结构锚点。`
    : `About ${absDiff.toFixed(2)}% away from nearest structure level ${label}. This measures effective-break buffer around the structure, not “closer is always better”. ⚪ <0.05% nearly naked on the level; ✅ 0.05–1.0% generally acceptable buffer; ⚠️ 1.0–2.0% wide buffer, needs ATR/volatility explanation; ❌ >2.0% may be detached from the structure anchor.`
  if (absDiff < 0.05) return { text: `⚪ ${suffix}`, cls: 'text-nofx-text-muted', title }
  if (absDiff <= 1.0) return { text: `✅ ${suffix}`, cls: 'text-emerald-300', title }
  if (absDiff <= 2.0) return { text: `⚠️ ${suffix}`, cls: 'text-amber-300', title }
  return { text: `❌ ${suffix}`, cls: 'text-red-400', title }
}

// ── Collapsible section ───────────────────────────────────────────────

function CollapsibleSection({ title, defaultOpen = false, children }: { title: string; defaultOpen?: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="rounded-lg border border-white/10 bg-black/20">
      <button type="button" onClick={() => setOpen(!open)} className="w-full flex items-center justify-between px-3 py-2 text-xs font-medium text-nofx-text-main hover:text-cyan-300 transition-colors">
        <span>{title}</span>
        <span className="text-nofx-text-muted">{open ? '▼' : '▶'}</span>
      </button>
      {open && <div className="px-3 pb-3">{children}</div>}
    </div>
  )
}

// ── KV row helper ─────────────────────────────────────────────────────

function KV({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="text-nofx-text-muted whitespace-nowrap">{label}</span>
      <span className="font-mono text-nofx-text-main text-right">{value}</span>
    </div>
  )
}

// ── Main component ────────────────────────────────────────────────────

export function PositionProtectionPanel({ traderId, positions, language, exchange, onSymbolClick }: PositionProtectionPanelProps) {
  const [ordersBySymbol, setOrdersBySymbol] = useState<Record<string, OpenOrder[]>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const symbolKeys = useMemo(() => {
    const keys = new Set<string>()
    for (const pos of positions || []) keys.add(String(pos.symbol || '').toUpperCase())
    return [...keys]
  }, [positions])

  useEffect(() => {
    let cancelled = false
    async function load() {
      if (!traderId || !positions || positions.length === 0) { setOrdersBySymbol({}); setError(null); return }
      setLoading(true); setError(null)
      try {
        const entries = await Promise.all(symbolKeys.map(async (symbol) => {
          const data = await api.getOpenOrders(traderId, symbol)
          return [symbol, Array.isArray(data) ? data : []] as const
        }))
        if (!cancelled) setOrdersBySymbol(Object.fromEntries(entries))
      } catch (err) {
        if (!cancelled) { setError(err instanceof Error ? err.message : 'Failed to load protection orders'); setOrdersBySymbol({}) }
      } finally { if (!cancelled) setLoading(false) }
    }
    load()
    const timer = window.setInterval(load, 60000)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [traderId, positions, symbolKeys])

  if (!positions || positions.length === 0) {
    return (
      <div className="nofx-glass p-5 relative overflow-hidden">
        <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2 mb-3">
          <span className="text-purple-400">🛡</span>
          {language === 'zh' ? '持仓保护执行面板' : 'Position Protection Runtime'}
        </h3>
        <div className="rounded-lg border border-white/10 bg-black/20 px-4 py-5 text-sm text-nofx-text-muted">
          {language === 'zh' ? '当前没有持仓。' : 'No open positions.'}
        </div>
      </div>
    )
  }

  return (
    <div className="nofx-glass p-5 relative overflow-hidden">
      <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2 mb-4">
        <span className="text-purple-400">🛡</span>
        {language === 'zh' ? '持仓保护执行面板' : 'Position Protection Runtime'}
      </h3>

      <div className="space-y-4">
        {positions.map((position, index) => {
          const symbol = String(position.symbol || '').toUpperCase()
          const side = normalizeSide(position.side)
          const entryPrice = position.entry_price || 0
          const symbolOrders = ordersBySymbol[symbol] || []
          const filteredOrders = symbolOrders.filter((o) => { const s = normalizeSide(o.position_side); return !s || s === side })
          const protectionRows = buildProtectionRows(position, filteredOrders, language)
          const rt = position.protection_runtime
          const currentPnlPct = Number(rt?.current_pnl_pct ?? position.unrealized_pnl_pct ?? 0)
          const peakPnlPct = Number(rt?.drawdown_peak_pnl_pct ?? currentPnlPct)
          const currentDrawdownPct = Number(rt?.current_drawdown_pct ?? 0)
          const currentDrawdownStage = String(rt?.current_drawdown_stage || '')
          const runtimeTiers = rt?.scheduled_tiers || []
          const satisfiedTiers = runtimeTiers.filter((t) => Boolean(t.is_satisfied))
          const triggeredTiers = runtimeTiers.filter((t) => Boolean(t.is_triggered))
          const nextTier = runtimeTiers.find((t) => !t.is_satisfied) || runtimeTiers[0] || null
          const runnerState = rt?.runner_state
          const runnerActive = Boolean(rt?.runner_mode_active ?? runnerState?.active ?? nextTier?.runner_mode_active)
          const runnerKeepPct = Number(rt?.runner_keep_pct ?? runnerState?.keep_pct ?? nextTier?.runner_keep_pct ?? 0)
          const runnerStopPrice = Number(rt?.runner_stop_price ?? runnerState?.stop_price ?? nextTier?.runner_stop_price ?? 0)
          const runnerStopSource = String(rt?.runner_stop_source || runnerState?.stop_source || nextTier?.runner_stop_source || '')
          const runnerStage = String(runnerState?.stage || nextTier?.drawdown_stage || currentDrawdownStage || '')
          const structureHealth = String(rt?.structure_protection_health || 'unstructured')
          const structureDriftReason = String(rt?.structure_protection_drift_reason || '')
          const structureDetached = Boolean(rt?.structure_protection_detached)
          const structurePrimaryTf = String(rt?.drawdown_structure_primary_timeframe || '')
          const structureTrace = rt?.drawdown_structure_trace || []
          const runnerMigrationNeeded = Boolean(rt?.runner_migration_needed)
          const runnerMigrationReason = String(rt?.runner_migration_reason || '')
          const runnerMigrationAnchor = rt?.runner_migration_anchor as { timeframe?: string; price?: number; anchor_type?: string; reason?: string; distance_pct?: number } | undefined
          const runnerDesiredActivation = Number(rt?.runner_migration_desired_activation ?? 0)
          const runnerDesiredCallback = Number(rt?.runner_migration_desired_callback ?? 0)
          const runnerLiveActivation = Number(rt?.runner_migration_live_activation ?? 0)
          const runnerLiveCallback = Number(rt?.runner_migration_live_callback ?? 0)
          const runnerMigrationSafe = Boolean(rt?.runner_migration_safe)
          const runnerMigrationSafetyReason = String(rt?.runner_migration_safety_reason || '')
          const runnerMigrationWouldLoosen = Boolean(rt?.runner_migration_would_loosen)
          const runnerMigrationWouldTighten = Boolean(rt?.runner_migration_would_tighten)
          const runnerMigrationActionable = Boolean(rt?.runner_migration_actionable)
          const runnerMigrationActionableReason = String(rt?.runner_migration_actionable_reason || '')
          const runnerMigrationPlan = rt?.runner_migration_plan
          const protectionQuantityDrift = Boolean(rt?.protection_quantity_drift)
          const protectionQuantityDriftReason = String(rt?.protection_quantity_drift_reason || '')
          const protectionPositionQuantity = Number(rt?.protection_position_quantity ?? position.quantity ?? 0)
          const protectionMaxOrderQuantity = Number(rt?.protection_max_order_quantity ?? 0)
          const protectionMaxOrderID = String(rt?.protection_max_order_id || '')
          const protectionQuantityDriftOrders = rt?.protection_quantity_drift_orders || []
          const orphanProtectionCleanupNeeded = Boolean(rt?.orphan_protection_cleanup_needed)
          const orphanProtectionOrderCount = Number(rt?.orphan_protection_order_count ?? 0)
          const breakEvenTriggerPct = Number(rt?.current_break_even_trigger_pct ?? 0)
          const breakEvenGapPct = Number(rt?.next_break_even_gap_pct ?? 0)
          const breakEvenSuppressedByRunner = Boolean(rt?.break_even_suppressed_by_runner ?? runnerState?.break_even_suppressed ?? nextTier?.break_even_suppressed_by_runner)
          const plannedLadderStopCount = Number(rt?.planned_ladder_stop_count ?? 0)
          const plannedLadderTakeProfitCount = Number(rt?.planned_ladder_take_profit_count ?? 0)
          const liveLadderStopCount = Number(rt?.live_ladder_stop_count ?? 0)
          const liveLadderTakeProfitCount = Number(rt?.live_ladder_take_profit_count ?? 0)
          const plannedLadderOrders = rt?.planned_ladder_orders || {}
          const plannedLadderTPOrders = Array.isArray(plannedLadderOrders.take_profit) ? plannedLadderOrders.take_profit : []
          const plannedLadderSLOrders = Array.isArray(plannedLadderOrders.stop_loss) ? plannedLadderOrders.stop_loss : []

          // Entry structure data
          const entryReviewSummary = position.entry_review_summary
          const entryStructureAudit = position.entry_structure_audit
          const entryLevels = entryReviewSummary?.key_levels as { support?: number[]; resistance?: number[]; swing_lows?: number[]; swing_highs?: number[]; fibonacci?: { swing_low?: number; swing_high?: number; levels?: number[] } } | undefined
          const entryRR = entryReviewSummary?.risk_reward as { entry?: number; invalidation?: number; first_target?: number } | undefined
          const fibSummary = entryLevels?.fibonacci
          const structureCandidates = [
            ...(entryLevels?.support || []).map((v, i) => ({ label: `S${i + 1}`, value: v })),
            ...(entryLevels?.resistance || []).map((v, i) => ({ label: `R${i + 1}`, value: v })),
            ...(entryLevels?.swing_lows || []).map((v, i) => ({ label: `swL${i + 1}`, value: v })),
            ...(entryLevels?.swing_highs || []).map((v, i) => ({ label: `swH${i + 1}`, value: v })),
            ...((fibSummary?.levels || []).map((v, i) => ({ label: `fib${i + 1}`, value: v }))),
            ...(entryRR?.invalidation ? [{ label: 'inv', value: entryRR.invalidation }] : []),
            ...(entryRR?.first_target ? [{ label: 'tgt', value: entryRR.first_target }] : []),
          ]

          // Linkage status
          const linkageStatus = (() => {
            if (!entryStructureAudit?.require_invalidation_target_linkage || !entryRR) return null
            const supports = (entryLevels?.support || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const resistances = (entryLevels?.resistance || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const swingLows = (entryLevels?.swing_lows || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const swingHighs = (entryLevels?.swing_highs || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
            const fibLevels = [...(fibSummary?.levels || []).filter((v): v is number => typeof v === 'number' && Number.isFinite(v))]
            if (fibSummary?.swing_low) fibLevels.push(fibSummary.swing_low)
            if (fibSummary?.swing_high) fibLevels.push(fibSummary.swing_high)
            if (!entryRR.entry || !entryRR.invalidation || !entryRR.first_target) return language === 'zh' ? '缺失' : 'Missing'
            const riskDist = Math.abs(entryRR.entry - entryRR.invalidation)
            const targetDist = Math.abs(entryRR.first_target - entryRR.entry)
            const tol = Math.max(0.0001, Math.min(Math.max(riskDist, targetDist) * 0.35, Math.max(entryRR.entry, entryRR.first_target, entryRR.invalidation) * 0.02))
            const invalidLinked = [...supports, ...swingLows, ...fibLevels].some((v) => Math.abs(v - entryRR.invalidation!) <= tol)
            const targetLinked = [...resistances, ...swingHighs, ...fibLevels].some((v) => Math.abs(v - entryRR.first_target!) <= tol)
            if (invalidLinked && targetLinked) return language === 'zh' ? '已联动' : 'Linked'
            if (invalidLinked || targetLinked) return language === 'zh' ? '部分联动' : 'Partial'
            return language === 'zh' ? '缺失' : 'Missing'
          })()

          const ladderSummary = (() => {
            const parts: string[] = []
            if (plannedLadderStopCount > 0) parts.push(`${liveLadderStopCount}/${plannedLadderStopCount} SL`)
            if (plannedLadderTakeProfitCount > 0) parts.push(`${liveLadderTakeProfitCount}/${plannedLadderTakeProfitCount} TP`)
            return parts.length > 0 ? `Ladder: ${parts.join(', ')}` : null
          })()

          const pnlColor = currentPnlPct >= 0 ? 'text-nofx-green' : 'text-nofx-red'

          return (
            <div key={`${symbol}-${side}-${index}`} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
              {/* ── Layer 1: Position Header ── */}
              <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                <button type="button" onClick={() => onSymbolClick?.(symbol)} className="font-semibold text-nofx-text-main hover:text-cyan-300 transition-colors">
                  {symbol} / {side}
                </button>
                <span className="text-nofx-text-muted">Entry: <span className="font-mono text-nofx-text-main">{formatPrice(entryPrice)}</span></span>
                <span className="text-nofx-text-muted">Qty: <span className="font-mono text-nofx-text-main">{formatQuantity(position.quantity)}</span></span>
                <span className={`font-mono font-semibold ${pnlColor}`}>{formatSignedPercent(currentPnlPct)}</span>
                <span className="text-nofx-text-muted">Peak: <span className="font-mono text-nofx-text-main">{formatSignedPercent(peakPnlPct)}</span></span>
                {position.leverage && <span className="text-nofx-text-muted font-mono">{position.leverage}x</span>}
              </div>

              {/* ── Layer 2: Protection Orders Table ── */}
              {protectionRows.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="text-nofx-text-muted border-b border-white/10">
                        <th className="text-left py-1.5 pr-3 font-medium">{language === 'zh' ? '类型' : 'Type'}</th>
                        <th className="text-right py-1.5 px-3 font-medium">{language === 'zh' ? '触发价' : 'Trigger'}</th>
                        <th className="text-right py-1.5 px-3 font-medium">{language === 'zh' ? '偏移' : 'vs Entry'}</th>
                        <th className="text-right py-1.5 px-3 font-medium">{language === 'zh' ? '比例' : 'Ratio'}</th>
                        <th className="text-left py-1.5 px-3 font-medium">{language === 'zh' ? '结构缓冲' : 'Structure Buffer'}</th>
                        <th className="text-right py-1.5 pl-3 font-medium">{language === 'zh' ? '状态' : 'Status'}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {protectionRows.map((row) => {
                        const badge = statusBadge(row.visualStatus, language)
                        const align = row.bucket === 'trailing' && row.callbackRate > 0
                          ? { text: language === 'zh' ? '按回调比例执行' : 'callback-based', cls: 'text-nofx-text-muted', title: language === 'zh' ? 'Trailing 单没有固定止损/止盈价；触发后按回调比例移动，所以不做静态结构缓冲距离评估。' : 'Trailing order has no fixed stop/target price; it moves by callback after activation, so no static structure-buffer distance is evaluated.' }
                          : structuralBufferBadge(row.triggerPrice, structureCandidates, language)
                        const deltaColor = row.deltaPct > 0 ? 'text-nofx-green' : row.deltaPct < 0 ? 'text-nofx-red' : 'text-nofx-text-muted'
                        return (
                          <tr key={`${row.orderId}-${row.type}-${row.triggerPrice}`} className="border-b border-white/5">
                            <td className="py-1.5 pr-3 text-nofx-text-main">
                              {row.label}
                              {row.source ? <span className="text-cyan-300 ml-1">· {row.source}</span> : null}
                              {row.closeRatioPct > 0 && row.closeRatioPct < 100 ? <span className="text-nofx-text-muted ml-1">({row.closeRatioPct.toFixed(0)}%)</span> : null}
                            </td>
                            <td className="py-1.5 px-3 text-right font-mono text-nofx-text-main">
                              {row.bucket === 'trailing' && row.callbackRate > 0 ? `cb ${row.callbackRate.toFixed(3)}%` : formatPrice(row.triggerPrice)}
                            </td>
                            <td className={`py-1.5 px-3 text-right font-mono ${deltaColor}`}>
                              {row.bucket === 'trailing' && row.callbackRate > 0 ? '—' : formatSignedPercent(row.deltaPct)}
                            </td>
                            <td className="py-1.5 px-3 text-right font-mono text-nofx-text-main">
                              {row.closeRatioPct > 0 ? `${row.closeRatioPct.toFixed(0)}%` : '—'}
                            </td>
                            <td className={`py-1.5 px-3 text-left text-[11px] ${align.cls}`} title={align.title}>{align.text}</td>
                            <td className="py-1.5 pl-3 text-right">
                              <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${badge.cls}`}>{badge.label}</span>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="text-xs text-nofx-text-muted border border-white/10 rounded-lg px-3 py-2">
                  {language === 'zh' ? '无保护委托' : 'No protection orders'}
                </div>
              )}

              {/* Summary line: ladder + protection state */}
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-nofx-text-muted">
                <span>{language === 'zh' ? '保护' : 'Protection'}: <span className="text-nofx-text-main">{compactProtectionLabel(position.protection_state, language, exchange)}</span></span>
                <span>{language === 'zh' ? '模式' : 'Mode'}: <span className="text-nofx-text-main">{compactExecutionMode(position.drawdown_execution_mode, language)}</span></span>
                {ladderSummary && <span className="text-nofx-text-main">{ladderSummary}</span>}
                {linkageStatus && <span>{language === 'zh' ? '联动' : 'Linkage'}: <span className="text-nofx-text-main">{linkageStatus}</span></span>}
              </div>

              {ladderSummary && (
                <div className="rounded-lg border border-yellow-500/15 bg-yellow-500/[0.04] p-3 text-xs">
                  <div className="flex items-center justify-between gap-2 mb-2"><div className="text-[11px] font-medium text-nofx-text-muted uppercase tracking-wide">{language === 'zh' ? 'Ladder 计划价位' : 'Ladder planned levels'}</div><div className="text-[10px] text-nofx-text-muted">{language === 'zh' ? '计划 vs 实盘委托' : 'plan vs live orders'}</div></div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                    {plannedLadderTPOrders.length > 0 && (
                      <div className="space-y-1">
                        <div className="text-[10px] text-nofx-text-muted">{language === 'zh' ? '止盈' : 'Take profit'}</div>
                        {plannedLadderTPOrders.map((rule: any, idx: number) => <div key={`tp-${idx}`} className="font-mono text-[11px] text-nofx-text-main">{formatLadderPlanLine(rule, idx, language)}</div>)}
                      </div>
                    )}
                    {plannedLadderSLOrders.length > 0 && (
                      <div className="space-y-1">
                        <div className="text-[10px] text-nofx-text-muted">{language === 'zh' ? '止损' : 'Stop loss'}</div>
                        {plannedLadderSLOrders.map((rule: any, idx: number) => <div key={`sl-${idx}`} className="font-mono text-[11px] text-nofx-text-main">{formatLadderPlanLine(rule, idx, language)}</div>)}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* ── Layer 3: Drawdown Tier Structure Map ── */}
              {runtimeTiers.length > 0 && (
                <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
                  <div className="text-[11px] font-medium text-nofx-text-muted uppercase tracking-wide mb-2">{language === 'zh' ? 'Drawdown 分层结构位' : 'Drawdown Tier Structure Map'}</div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-2 text-xs">
                    {runtimeTiers.map((tier) => {
                      const anchor = tier.structure_anchor
                      const tf = tier.anchor_timeframe || anchor?.timeframe || '—'
                      const anchorPrice = Number(tier.anchor_price ?? anchor?.price ?? 0)
                      const anchorType = String(anchor?.anchor_type || tier.anchor_source || '')
                      return (
                        <div key={`${symbol}-${side}-tier-${tier.index}`} className="rounded-md border border-white/10 bg-black/20 p-2 space-y-1">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-nofx-text-main font-semibold">T{tier.index} {tier.stage_name || ''}</span>
                            <span className="text-nofx-text-muted">{tf}</span>
                          </div>
                          <KV label={language === 'zh' ? '利润 / 回撤' : 'Profit / DD'} value={`${Number(tier.min_profit_pct || 0).toFixed(2)}% / ${Number(tier.max_drawdown_pct || 0).toFixed(0)}%`} />
                          <KV label={language === 'zh' ? '仓位' : 'Close'} value={`${Number(tier.close_ratio_pct || 0).toFixed(0)}%${tier.runner_keep_pct ? ` · runner keep ${Number(tier.runner_keep_pct).toFixed(0)}%` : ''}`} />
                          <KV label={language === 'zh' ? '触发 / 回调' : 'Activation / CB'} value={`${formatPrice(Number(tier.activation_price || tier.planned_activation_price || 0))} · ${(Number(tier.callback_rate || 0) * 100).toFixed(2)}%`} />
                          <KV label={language === 'zh' ? '结构位' : 'Anchor'} value={anchorPrice > 0 ? `${tf} · ${compactSourceLabel(anchorType, language)} · ${formatPrice(anchorPrice)}` : '—'} />
                          {anchor?.used_for && <KV label={language === 'zh' ? '用途' : 'Use'} value={compactSourceLabel(anchor.used_for, language)} />}
                          {(tier.reason_anchor || anchor?.reason) && <div className="text-[10px] text-nofx-text-muted leading-snug line-clamp-2">{tier.reason_anchor || anchor?.reason}</div>}
                        </div>
                      )
                    })}
                  </div>
                </div>
              )}

              {/* ── Layer 4: Runtime Status (collapsed) ── */}
              <CollapsibleSection title={language === 'zh' ? '📊 运行时状态' : '📊 Runtime Status'}>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
                  {/* Left: Drawdown Runtime */}
                  <div className="space-y-1.5">
                    <div className="text-[11px] font-medium text-nofx-text-muted uppercase tracking-wide mb-1">{language === 'zh' ? '回撤运行时' : 'Drawdown Runtime'}</div>
                    <KV label={language === 'zh' ? '阶段' : 'Stage'} value={currentDrawdownStage ? currentDrawdownStage.replace(/_/g, ' ') : '—'} />
                    <KV label={language === 'zh' ? '峰值 / 回撤' : 'Peak / DD'} value={`${formatSignedPercent(peakPnlPct)} / ${currentDrawdownPct.toFixed(2)}%`} />
                    <KV label={language === 'zh' ? '下一门槛' : 'Next gate'} value={nextTier ? `${Number(nextTier.min_profit_pct || 0).toFixed(2)}%` : '—'} />
                    <KV label={language === 'zh' ? '满足 / 触发' : 'Satisfied / Triggered'} value={`${satisfiedTiers.length} / ${triggeredTiers.length}`} />
                    <KV label="Runner" value={`${compactRunnerStateLabel(runnerActive, runnerStage, language)}${runnerKeepPct > 0 ? ` (keep ${runnerKeepPct.toFixed(0)}%)` : ''}`} />
                    {runnerStopPrice > 0 && <KV label={language === 'zh' ? 'Runner 止损' : 'Runner stop'} value={`${formatPrice(runnerStopPrice)} · ${compactSourceLabel(runnerStopSource, language)}`} />}
                  </div>
                  {/* Right: Break-even + Structure */}
                  <div className="space-y-1.5">
                    <div className="text-[11px] font-medium text-nofx-text-muted uppercase tracking-wide mb-1">{language === 'zh' ? '保本 & 结构' : 'Break-even & Structure'}</div>
                    <KV label={language === 'zh' ? '保本' : 'Break-even'} value={`${position.break_even_state || 'idle'}${breakEvenTriggerPct > 0 ? ` | ${language === 'zh' ? '触发' : 'trig'}: ${breakEvenTriggerPct.toFixed(2)}% | ${language === 'zh' ? '差' : 'gap'}: ${breakEvenGapPct.toFixed(2)}%` : ''}`} />
                    {breakEvenSuppressedByRunner && <KV label={language === 'zh' ? 'Runner 抑制' : 'Runner suppressed'} value={language === 'zh' ? '是' : 'yes'} />}
                    <KV label={language === 'zh' ? '结构' : 'Structure'} value={`${compactSourceLabel(structureHealth, language)}${structurePrimaryTf ? ` · ${structurePrimaryTf}` : ''}`} />
                    {structureDriftReason && <KV label={language === 'zh' ? '偏离' : 'Drift'} value={compactSourceLabel(structureDriftReason, language)} />}
                    {runnerMigrationNeeded && <KV label={language === 'zh' ? 'Runner 迁移' : 'Runner migration'} value={`${runnerMigrationReason ? compactSourceLabel(runnerMigrationReason, language) : (language === 'zh' ? '需要' : 'needed')}${runnerMigrationSafe ? ` · ${language === 'zh' ? '安全' : 'safe'}` : ''}`} />}
                    {runnerMigrationSafetyReason && <KV label={language === 'zh' ? '迁移安全' : 'Migration safety'} value={`${compactSourceLabel(runnerMigrationSafetyReason, language)}${runnerMigrationWouldLoosen ? ` · ${language === 'zh' ? '会放宽' : 'would loosen'}` : ''}${runnerMigrationWouldTighten ? ` · ${language === 'zh' ? '会收紧' : 'would tighten'}` : ''}`} />}
                    {(runnerMigrationActionable || runnerMigrationActionableReason) && <KV label={language === 'zh' ? '可执行' : 'Actionable'} value={`${runnerMigrationActionable ? (language === 'zh' ? '是' : 'yes') : (language === 'zh' ? '否' : 'no')}${runnerMigrationActionableReason ? ` · ${compactSourceLabel(runnerMigrationActionableReason, language)}` : ''}`} />}
                    {runnerMigrationPlan?.cancel_order_id && <KV label={language === 'zh' ? '迁移计划' : 'Migration plan'} value={`${compactSourceLabel(runnerMigrationPlan.action, language)} · ${language === 'zh' ? '需确认' : 'confirm'} · ${runnerMigrationPlan.cancel_order_id}`} />}
                    {runnerMigrationPlan?.new_activation && <KV label={language === 'zh' ? '计划新单' : 'Planned new order'} value={`${formatQuantity(runnerMigrationPlan.quantity || 0)} @ ${formatPrice(runnerMigrationPlan.new_activation)} · ${(Number(runnerMigrationPlan.new_callback || 0) * 100).toFixed(2)}%`} />}
                    {runnerMigrationAnchor?.price && <KV label={language === 'zh' ? 'Runner 锚点' : 'Runner anchor'} value={`${runnerMigrationAnchor.timeframe || '—'} · ${compactSourceLabel(runnerMigrationAnchor.anchor_type, language)} · ${formatPrice(runnerMigrationAnchor.price)}`} />}
                    {(runnerDesiredActivation > 0 || runnerLiveActivation > 0) && <KV label={language === 'zh' ? '迁移触发价' : 'Migration activation'} value={`${runnerLiveActivation > 0 ? formatPrice(runnerLiveActivation) : '—'} → ${runnerDesiredActivation > 0 ? formatPrice(runnerDesiredActivation) : '—'}`} />}
                    {(runnerDesiredCallback > 0 || runnerLiveCallback > 0) && <KV label={language === 'zh' ? '迁移回调' : 'Migration callback'} value={`${runnerLiveCallback > 0 ? `${(runnerLiveCallback * 100).toFixed(2)}%` : '—'} → ${runnerDesiredCallback > 0 ? `${(runnerDesiredCallback * 100).toFixed(2)}%` : '—'}`} />}
                    {protectionQuantityDrift && <KV label={language === 'zh' ? '保护覆盖' : 'Protection coverage'} value={`${compactSourceLabel(protectionQuantityDriftReason, language)} · pos ${formatQuantity(protectionPositionQuantity)} / max order ${formatQuantity(protectionMaxOrderQuantity)}${protectionMaxOrderID ? ` · ${protectionMaxOrderID}` : ''}`} />}
                    {protectionQuantityDriftOrders.length > 0 && <KV label={language === 'zh' ? '覆盖订单' : 'Coverage orders'} value={protectionQuantityDriftOrders.slice(0, 2).map((o) => `${o.order_id || '—'} ${formatQuantity(Number(o.quantity || 0))}`).join(' | ')} />}
                    {orphanProtectionCleanupNeeded && <KV label={language === 'zh' ? '清仓清理' : 'Orphan cleanup'} value={`${language === 'zh' ? '需要清理保护单' : 'protection cleanup needed'} · ${orphanProtectionOrderCount}`} />}
                    {structureDetached && !structureDriftReason && <KV label={language === 'zh' ? '偏离' : 'Drift'} value={compactSourceLabel('structure_detached', language)} />}
                    {structureTrace.length > 0 && <KV label={language === 'zh' ? '轨迹' : 'Trace'} value={structureTrace.slice(-3).join(' → ')} />}
                  </div>
                </div>
              </CollapsibleSection>

              {/* ── Entry Structure Summary (collapsed) ── */}
              {entryReviewSummary && (
                <CollapsibleSection title={language === 'zh' ? '📐 开仓结构摘要' : '📐 Entry Structure Summary'}>
                  <div className="space-y-1.5 text-xs">
                    <KV label={language === 'zh' ? '周期' : 'Timeframes'} value={formatTimeframeTrail({ timeframe_context: entryReviewSummary.timeframe_context } as never).join(' · ') || '—'} />
                    <KV label={language === 'zh' ? 'Entry / 失效 / 目标' : 'Entry / Inv / Target'} value={formatRiskRewardLinkage(entryRR as never).join(' · ') || '—'} />
                    {entryStructureAudit?.audit_support_resistance && (
                      <>
                        <KV label={language === 'zh' ? '支撑' : 'Support'} value={formatCompactLevelList(entryLevels?.support).join(' / ') || '—'} />
                        <KV label={language === 'zh' ? '阻力' : 'Resistance'} value={formatCompactLevelList(entryLevels?.resistance).join(' / ') || '—'} />
                      </>
                    )}
                    <KV label={language === 'zh' ? '斐波' : 'Fib'} value={fibSummary ? (fibSummary.levels || []).map(formatPrice).join(', ') || '—' : '—'} />
                    <KV label={language === 'zh' ? '审计' : 'Audit'} value={getAuditToggleText(entryStructureAudit, language)} />
                  </div>
                </CollapsibleSection>
              )}
            </div>
          )
        })}

        {loading && <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '刷新中…' : 'Refreshing…'}</div>}
        {error && <div className="text-xs text-nofx-red">{error}</div>}
      </div>
    </div>
  )
}
