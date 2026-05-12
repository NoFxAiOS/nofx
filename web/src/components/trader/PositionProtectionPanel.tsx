import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { formatPrice, formatQuantity } from '../../utils/format'
import type { Language } from '../../i18n/translations'
import type { OpenOrder, Position } from '../../types'
import {
  formatTimeframeTrail,
  formatRiskRewardLinkage,
} from './reviewContextSummary'

interface PositionProtectionPanelProps {
  traderId?: string
  positions?: Position[]
  language: Language
  exchange?: string
  onSymbolClick?: (symbol: string) => void
}

type OrderBucket = 'stop' | 'trailing' | 'takeProfit' | 'other'

type UnifiedRow = {
  zone: string
  direction: 'SL' | 'TP' | 'Trail' | 'BE'
  price: number
  deltaPct: number
  ratioPct: number
  status: string
  statusCls: string
  source: string
  orderId?: string
  callbackRate?: number
  tierMinProfit?: number
  tierMaxDD?: number
  tierRunnerKeep?: number
  anchorInfo?: string
  basisType?: string
  atrDistance?: number
}

function normalizeSide(side?: string): string {
  return String(side || '').toUpperCase()
}

function formatSignedPercent(
  value: number | undefined | null,
  digits = 2
): string {
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

function normalizeCallbackRate(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  if (value > 1) return value / 100
  if (value >= 0.1) return value / 100
  return value
}

function getOrderZone(order: OpenOrder): string {
  const id = String(order.client_order_id || '').toLowerCase()
  if (id.includes('ladder_sl') || id.includes('ladder_tp')) return 'Ladder'
  if (id.includes('fallback_maxloss') || id.includes('full_sl')) return 'Ladder'
  if (id.includes('break_even') || id.includes('breakeven')) return 'BE'
  if (id.includes('drawdown') || id.includes('trailing')) return 'DD'
  return 'Ladder'
}

function getOrderDirection(bucket: OrderBucket): 'SL' | 'TP' | 'Trail' | 'BE' {
  switch (bucket) {
    case 'stop':
      return 'SL'
    case 'takeProfit':
      return 'TP'
    case 'trailing':
      return 'Trail'
    default:
      return 'SL'
  }
}

function compactProtectionLabel(
  state: string | undefined,
  language: Language,
  exchange?: string
): string {
  if (!state) return language === 'zh' ? '未识别' : 'unknown'
  const v = state.toLowerCase()
  const ex = exchange ? exchange.toUpperCase() : 'EX'
  if (v === 'native_trailing_armed') return `${ex} trailing`
  if (v === 'native_partial_trailing_armed') return `${ex} partial`
  if (v === 'managed_partial_drawdown_armed') return 'managed'
  if (v === 'exchange_protection_verified')
    return language === 'zh' ? '已校验' : 'verified'
  if (v === 'drawdown_triggered')
    return language === 'zh' ? '已触发' : 'triggered'
  return state.replace(/_/g, ' ')
}

function compactSourceLabel(
  value: string | undefined,
  language: Language
): string {
  if (!value) return '—'
  const v = value.toLowerCase()
  if (v === 'aligned') return language === 'zh' ? '对齐' : 'aligned'
  if (v === 'partially_degraded')
    return language === 'zh' ? '部分降级' : 'partial'
  if (v === 'degraded_to_full_fallback')
    return language === 'zh' ? '兜底' : 'fallback'
  if (v === 'structure_detached') return language === 'zh' ? '脱钩' : 'detached'
  if (v === 'unstructured') return language === 'zh' ? '无结构' : 'unstructured'
  if (v === 'support') return language === 'zh' ? '支撑' : 'support'
  if (v === 'resistance') return language === 'zh' ? '阻力' : 'resistance'
  if (v === 'swing_high') return language === 'zh' ? '摆高' : 'swH'
  if (v === 'swing_low') return language === 'zh' ? '摆低' : 'swL'
  if (v === 'fib' || v === 'fibonacci') return 'fib'
  if (v === 'first_target') return language === 'zh' ? '目标' : 'target'
  if (v === 'break_even') return 'BE'
  return value.replace(/_/g, ' ').slice(0, 12)
}

function buildUnifiedRows(
  position: Position,
  orders: OpenOrder[],
  language: Language
): UnifiedRow[] {
  const entryPrice = position.entry_price || 0
  const positionQty = position.quantity || 0
  const side = normalizeSide(position.side)
  const dirMul = side === 'LONG' ? 1 : -1
  const rows: UnifiedRow[] = []

  // Get planned ladder orders for anchor matching
  const rt = position.protection_runtime
  const plannedLadder = rt?.planned_ladder_orders as
    | {
        stop_loss?: Array<{
          price: number
          basis_type?: string
          anchor_price?: number
          anchor_timeframe?: string
          anchor_source?: string
          atr_distance?: number
        }>
        take_profit?: Array<{
          price: number
          basis_type?: string
          anchor_price?: number
          anchor_timeframe?: string
          anchor_source?: string
          atr_distance?: number
        }>
      }
    | undefined

  function matchPlannedOrder(triggerPrice: number, direction: string) {
    if (!plannedLadder || triggerPrice <= 0) return undefined
    const list =
      direction === 'SL'
        ? plannedLadder.stop_loss
        : plannedLadder.take_profit
    if (!list) return undefined
    const tolerance = triggerPrice * 0.0001
    return list.find((p) => Math.abs(p.price - triggerPrice) < tolerance)
  }

  for (const order of orders) {
    const bucket = classifyOrderBucket(order)
    const triggerPrice = order.stop_price || order.price || 0
    const rawDelta =
      entryPrice > 0 && triggerPrice > 0
        ? ((triggerPrice - entryPrice) / entryPrice) * 100
        : 0
    const deltaPct = rawDelta * dirMul
    const ratioPct =
      positionQty > 0 && order.quantity > 0
        ? (order.quantity / positionQty) * 100
        : 0
    const callbackRate = normalizeCallbackRate(Number(order.callback_rate || 0))
    const zone = getOrderZone(order)
    const direction =
      zone === 'BE' ? ('BE' as const) : getOrderDirection(bucket)

    // Match to planned ladder for anchor info
    const planned = matchPlannedOrder(triggerPrice, direction)

    rows.push({
      zone,
      direction,
      price: triggerPrice,
      deltaPct,
      ratioPct,
      status: language === 'zh' ? '已委托' : 'Live',
      statusCls: 'text-emerald-300',
      source: '',
      orderId: order.order_id,
      callbackRate: callbackRate > 0 ? callbackRate : undefined,
      basisType: planned?.basis_type,
      anchorInfo: planned?.anchor_source
        ? `${planned.anchor_timeframe || ''} ${compactSourceLabel(planned.anchor_source, language)} ${planned.anchor_price ? formatPrice(planned.anchor_price) : ''}`.trim()
        : undefined,
      atrDistance: planned?.atr_distance,
    })
  }

  const runtimeTiers = rt?.scheduled_tiers || []
  for (const tier of runtimeTiers) {
    const activationPrice = Number(
      tier.activation_price || tier.planned_activation_price || 0
    )
    const callbackRate = Number(tier.callback_rate || 0)
    const minProfit = Number(tier.min_profit_pct || 0)
    const maxDD = Number(tier.max_drawdown_pct || 0)
    const closeRatio = Number(tier.close_ratio_pct || 0)
    const isSatisfied = Boolean(tier.is_satisfied)
    const isTriggered = Boolean(tier.is_triggered)
    const isSuperseded =
      String(tier.status || '').toLowerCase() === 'superseded'
    const tierIndex = tier.index || 0
    const anchor = tier.structure_anchor
    const tf = tier.anchor_timeframe || anchor?.timeframe || ''
    const anchorType = String(anchor?.anchor_type || tier.anchor_source || '')
    const anchorPrice = Number(tier.anchor_price ?? anchor?.price ?? 0)

    const rawDelta =
      entryPrice > 0 && activationPrice > 0
        ? ((activationPrice - entryPrice) / entryPrice) * 100
        : minProfit
    const deltaPct = rawDelta * dirMul

    let status: string
    let statusCls: string
    if (isSuperseded) {
      status = language === 'zh' ? '已失效' : 'Superseded'
      statusCls = 'text-nofx-text-muted line-through'
    } else if (isTriggered) {
      status = language === 'zh' ? '已触发' : 'Triggered'
      statusCls = 'text-nofx-red'
    } else if (isSatisfied) {
      status = language === 'zh' ? '跟踪中' : 'Tracking'
      statusCls = 'text-emerald-300'
    } else {
      status = language === 'zh' ? '等待' : 'Pending'
      statusCls = 'text-amber-300'
    }

    rows.push({
      zone: `DD-T${tierIndex}`,
      direction: 'TP',
      price: activationPrice,
      deltaPct,
      ratioPct: closeRatio,
      status,
      statusCls,
      source: '',
      callbackRate: callbackRate > 0 ? callbackRate : undefined,
      tierMinProfit: minProfit,
      tierMaxDD: maxDD,
      tierRunnerKeep: Number(tier.runner_keep_pct || 0),
      anchorInfo:
        anchorPrice > 0
          ? `${tf} ${compactSourceLabel(anchorType, language)} ${formatPrice(anchorPrice)}`
          : undefined,
      basisType:
        tier.basis_type || (anchorPrice > 0 ? 'structural' : undefined),
      atrDistance: Number(tier.atr_distance || 0) || undefined,
    })
  }

  const beState = position.break_even_state
  const liveBePrice = Number(rt?.live_break_even_stop_price ?? 0)
  const beOrderDetected = Boolean(rt?.break_even_order_detected)
  const bePrice = liveBePrice > 0 ? liveBePrice : entryPrice
  const beRawDelta =
    entryPrice > 0 && bePrice > 0
      ? ((bePrice - entryPrice) / entryPrice) * 100
      : 0
  const beDeltaPct = beRawDelta * dirMul

  if (beState === 'armed' || beOrderDetected) {
    rows.push({
      zone: 'BE',
      direction: 'BE',
      price: bePrice,
      deltaPct: beDeltaPct,
      ratioPct: 100,
      status: beOrderDetected
        ? language === 'zh'
          ? '已委托'
          : 'Live'
        : language === 'zh'
          ? '已激活'
          : 'Armed',
      statusCls: beOrderDetected ? 'text-emerald-300' : 'text-amber-300',
      source: '',
    })
  } else if (
    beState === 'pending' ||
    (!beState && rt?.current_break_even_trigger_pct)
  ) {
    rows.push({
      zone: 'BE',
      direction: 'BE',
      price: bePrice,
      deltaPct: beDeltaPct,
      ratioPct: 100,
      status: language === 'zh' ? '待激活' : 'Pending',
      statusCls: 'text-nofx-text-muted',
      source: '',
    })
  }

  return rows
}

function DirectionBadge({ direction }: { direction: UnifiedRow['direction'] }) {
  const cls = {
    SL: 'text-nofx-red border-nofx-red/30 bg-nofx-red/10',
    TP: 'text-nofx-green border-nofx-green/30 bg-nofx-green/10',
    Trail: 'text-cyan-300 border-cyan-500/30 bg-cyan-500/10',
    BE: 'text-amber-300 border-amber-500/30 bg-amber-500/10',
  }[direction]
  return (
    <span
      className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-semibold border ${cls}`}
    >
      {direction}
    </span>
  )
}

function StatusBadge({
  status,
  statusCls,
}: {
  status: string
  statusCls: string
}) {
  const bgMap: Record<string, string> = {
    'text-emerald-300': 'bg-emerald-500/10 border-emerald-500/20',
    'text-amber-300': 'bg-amber-500/10 border-amber-500/20',
    'text-nofx-red': 'bg-red-500/10 border-red-500/20',
    'text-nofx-text-muted line-through':
      'bg-white/5 border-white/10 opacity-60',
  }
  const bg = bgMap[statusCls] || 'bg-white/5 border-white/10'
  return (
    <span
      className={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium border ${statusCls} ${bg}`}
    >
      {status}
    </span>
  )
}

function ZoneLabel({ zone }: { zone: string }) {
  let cls = 'text-nofx-text-main'
  if (zone === 'Ladder') cls = 'text-blue-300'
  else if (zone.startsWith('DD')) cls = 'text-purple-300'
  else if (zone === 'BE') cls = 'text-amber-300'
  return <span className={`font-medium ${cls}`}>{zone}</span>
}

function CollapsibleSection({
  title,
  defaultOpen = false,
  children,
}: {
  title: string
  defaultOpen?: boolean
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="rounded border border-white/10 bg-black/20">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-3 py-1.5 text-[11px] font-medium text-nofx-text-muted hover:text-cyan-300 transition-colors"
      >
        <span>{title}</span>
        <span className="text-[9px]">{open ? '▼' : '▶'}</span>
      </button>
      {open && <div className="px-3 pb-2">{children}</div>}
    </div>
  )
}

function KV({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-2">
      <span className="text-nofx-text-muted whitespace-nowrap">{label}</span>
      <span className="font-mono text-nofx-text-main text-right text-[11px]">
        {value}
      </span>
    </div>
  )
}

export function PositionProtectionPanel({
  traderId,
  positions,
  language,
  exchange,
  onSymbolClick,
}: PositionProtectionPanelProps) {
  const [ordersBySymbol, setOrdersBySymbol] = useState<
    Record<string, OpenOrder[]>
  >({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const symbolKeys = useMemo(() => {
    const keys = new Set<string>()
    for (const pos of positions || [])
      keys.add(String(pos.symbol || '').toUpperCase())
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
        if (!cancelled) setOrdersBySymbol(Object.fromEntries(entries))
      } catch (err) {
        if (!cancelled)
          setError(err instanceof Error ? err.message : 'Failed to load orders')
      } finally {
        if (!cancelled) setLoading(false)
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
      <div className="nofx-glass p-6 relative overflow-hidden group">
        <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
          <div className="w-24 h-24 rounded-full bg-purple-500 blur-3xl" />
        </div>
        <h2 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2 mb-4">
          <span className="text-purple-400">◈</span>
          {language === 'zh' ? '持仓保护' : 'Position Protection'}
        </h2>
        <div className="rounded border border-white/10 bg-black/20 px-4 py-3 text-xs text-nofx-text-muted">
          {language === 'zh' ? '当前没有持仓。' : 'No open positions.'}
        </div>
      </div>
    )
  }

  return (
    <div className="nofx-glass p-6 relative overflow-hidden group">
      <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
        <div className="w-24 h-24 rounded-full bg-purple-500 blur-3xl" />
      </div>
      <h2 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2 mb-5 relative z-10">
        <span className="text-purple-400">◈</span>
        {language === 'zh' ? '持仓保护' : 'Position Protection'}
        {positions.length > 0 && (
          <span className="text-xs px-2 py-0.5 rounded bg-purple-500/10 text-purple-300 border border-purple-500/20 font-mono ml-2">
            {positions.length}
          </span>
        )}
      </h2>

      <div className="space-y-4 relative z-10">
        {positions.map((position, index) => {
          const symbol = String(position.symbol || '').toUpperCase()
          const side = normalizeSide(position.side)
          const entryPrice = position.entry_price || 0
          const markPrice = position.mark_price || 0
          const qty = position.quantity || 0
          const entryValue = entryPrice * qty
          const currentValue = markPrice > 0 ? markPrice * qty : entryValue
          const symbolOrders = ordersBySymbol[symbol] || []
          const filteredOrders = symbolOrders.filter((o) => {
            const s = normalizeSide(o.position_side)
            return !s || s === side
          })
          const rt = position.protection_runtime
          const currentPnlPct = Number(
            rt?.current_pnl_pct ?? position.unrealized_pnl_pct ?? 0
          )
          const peakPnlPct = Number(rt?.drawdown_peak_pnl_pct ?? currentPnlPct)
          const currentDrawdownPct = Number(rt?.current_drawdown_pct ?? 0)
          const structureHealth = String(
            rt?.structure_protection_health || 'unstructured'
          )
          const runnerState = rt?.runner_state
          const runnerActive = Boolean(
            rt?.runner_mode_active ?? runnerState?.active
          )
          const runnerKeepPct = Number(
            rt?.runner_keep_pct ?? runnerState?.keep_pct ?? 0
          )
          const runnerStopPrice = Number(
            rt?.runner_stop_price ?? runnerState?.stop_price ?? 0
          )
          const runnerStopSource = String(
            rt?.runner_stop_source || runnerState?.stop_source || ''
          )
          const runnerMigrationNeeded = Boolean(rt?.runner_migration_needed)
          const runnerMigrationReason = String(
            rt?.runner_migration_reason || ''
          )
          const runnerMigrationSafe = Boolean(rt?.runner_migration_safe)
          const runnerMigrationPlan = rt?.runner_migration_plan
          const protectionQuantityDrift = Boolean(rt?.protection_quantity_drift)
          const orphanProtectionCleanupNeeded = Boolean(
            rt?.orphan_protection_cleanup_needed
          )
          const breakEvenSuppressedByRunner = Boolean(
            rt?.break_even_suppressed_by_runner ??
            runnerState?.break_even_suppressed
          )

          const unifiedRows = buildUnifiedRows(
            position,
            filteredOrders,
            language
          )
          const pnlColor =
            currentPnlPct >= 0 ? 'text-nofx-green' : 'text-nofx-red'
          const sideBadgeCls =
            side === 'LONG'
              ? 'bg-nofx-green/15 text-nofx-green border-nofx-green/30'
              : 'bg-nofx-red/15 text-nofx-red border-nofx-red/30'

          const entryReviewSummary = position.entry_review_summary
          const entryStructureAudit = position.entry_structure_audit
          const entryRR = entryReviewSummary?.risk_reward as
            | { entry?: number; invalidation?: number; first_target?: number }
            | undefined

          return (
            <div
              key={`${symbol}-${side}-${index}`}
              className="rounded-lg border border-white/10 bg-black/20 p-4 space-y-3"
            >
              {/* Position Header — Line 1: Symbol + Side + Leverage + PnL */}
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => onSymbolClick?.(symbol)}
                    className="text-base font-bold text-nofx-text-main hover:text-cyan-300 transition-colors"
                  >
                    {symbol}
                  </button>
                  <span
                    className={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-semibold ${sideBadgeCls}`}
                  >
                    {side}
                  </span>
                  {position.leverage && (
                    <span className="text-xs font-mono text-nofx-text-muted">
                      {position.leverage}x
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-3">
                  <span
                    className={`text-base font-bold font-mono ${pnlColor}`}
                  >
                    {formatSignedPercent(currentPnlPct)}
                  </span>
                  <span className="text-[10px] text-nofx-text-muted px-1.5 py-0.5 rounded bg-white/5 border border-white/10">
                    {compactProtectionLabel(
                      position.protection_state,
                      language,
                      exchange
                    )}
                  </span>
                </div>
              </div>

              {/* Position Header — Line 2: Entry / Qty / Value / Peak */}
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-nofx-text-muted">
                <span>
                  Entry{' '}
                  <span className="font-mono text-nofx-text-main">
                    {formatPrice(entryPrice)}
                  </span>
                </span>
                <span>
                  Qty{' '}
                  <span className="font-mono text-nofx-text-main">
                    {formatQuantity(qty)}
                  </span>
                  {' / '}
                  <span className="font-mono text-nofx-text-main">
                    ${entryValue.toFixed(1)}
                  </span>
                </span>
                {markPrice > 0 && (
                  <span>
                    Now{' '}
                    <span className="font-mono text-nofx-text-main">
                      ${currentValue.toFixed(1)}
                    </span>
                  </span>
                )}
                <span>
                  Peak{' '}
                  <span className="font-mono text-nofx-text-main">
                    {formatSignedPercent(peakPnlPct)}
                  </span>
                </span>
              </div>

              {/* Protection Table */}
              {unifiedRows.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="text-nofx-text-muted border-b-2 border-white/10">
                        <th className="text-left py-1.5 pr-2 font-medium w-16">
                          {language === 'zh' ? '区域' : 'Zone'}
                        </th>
                        <th className="text-left py-1.5 px-2 font-medium w-12">
                          {language === 'zh' ? '向' : 'Dir'}
                        </th>
                        <th className="text-right py-1.5 px-2 font-medium">
                          {language === 'zh' ? '价格' : 'Price'}
                        </th>
                        <th className="text-right py-1.5 px-2 font-medium">
                          {language === 'zh' ? '偏移' : 'Δ%'}
                        </th>
                        <th className="text-right py-1.5 px-2 font-medium">
                          {language === 'zh' ? '比例' : '%'}
                        </th>
                        <th className="text-right py-1.5 px-2 font-medium">
                          {language === 'zh' ? '利润阈' : 'Min'}
                        </th>
                        <th className="text-right py-1.5 px-2 font-medium">
                          {language === 'zh' ? '回撤' : 'DD'}
                        </th>
                        <th className="text-center py-1.5 px-2 font-medium">
                          {language === 'zh' ? '状态' : 'St'}
                        </th>
                        <th className="text-left py-1.5 pl-2 font-medium">
                          {language === 'zh' ? '结构位' : 'Anchor'}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {unifiedRows.map((row, ri) => {
                        const deltaColor =
                          row.deltaPct > 0
                            ? 'text-nofx-green'
                            : row.deltaPct < 0
                              ? 'text-nofx-red'
                              : 'text-nofx-text-muted'
                        return (
                          <tr
                            key={`row-${ri}-${row.zone}-${row.price}`}
                            className="border-b border-white/5 hover:bg-white/5 transition-colors"
                          >
                            <td className="py-1.5 pr-2">
                              <ZoneLabel zone={row.zone} />
                            </td>
                            <td className="py-1.5 px-2">
                              <DirectionBadge direction={row.direction} />
                            </td>
                            <td className="py-1.5 px-2 text-right font-mono font-semibold text-nofx-text-main">
                              {row.callbackRate && !row.price
                                ? `cb ${(row.callbackRate * 100).toFixed(2)}%`
                                : formatPrice(row.price)}
                            </td>
                            <td
                              className={`py-1.5 px-2 text-right font-mono ${deltaColor}`}
                            >
                              {row.callbackRate
                                ? `cb ${(row.callbackRate * 100).toFixed(2)}%`
                                : formatSignedPercent(row.deltaPct)}
                            </td>
                            <td className="py-1.5 px-2 text-right font-mono text-nofx-text-main">
                              {row.ratioPct > 0
                                ? `${row.ratioPct.toFixed(0)}%`
                                : '—'}
                            </td>
                            <td className="py-1.5 px-2 text-right font-mono text-nofx-text-muted">
                              {row.tierMinProfit
                                ? `${row.tierMinProfit.toFixed(1)}%`
                                : '—'}
                            </td>
                            <td className="py-1.5 px-2 text-right font-mono text-nofx-text-muted">
                              {row.tierMaxDD
                                ? `${row.tierMaxDD.toFixed(0)}%`
                                : '—'}
                            </td>
                            <td className="py-1.5 px-2 text-center">
                              <StatusBadge
                                status={row.status}
                                statusCls={row.statusCls}
                              />
                            </td>
                            <td
                              className="py-1.5 pl-2 text-nofx-text-muted truncate max-w-[200px]"
                              title={row.anchorInfo || ''}
                            >
                              {row.basisType && (
                                <span
                                  className="inline-block mr-1"
                                  style={{
                                    color:
                                      row.basisType === 'structural'
                                        ? '#60A5FA'
                                        : row.basisType === 'atr_based'
                                          ? '#F59E0B'
                                          : row.basisType === 'fibonacci'
                                            ? '#A78BFA'
                                            : '#9CA3AF',
                                  }}
                                >
                                  {row.basisType === 'structural'
                                    ? '🎯'
                                    : row.basisType === 'atr_based'
                                      ? '📐'
                                      : row.basisType === 'fibonacci'
                                        ? '🌀'
                                        : '📊'}
                                </span>
                              )}
                              <span className="text-nofx-text-main">
                                {row.anchorInfo || '—'}
                              </span>
                              {row.atrDistance ? (
                                <span className="text-nofx-text-muted ml-1">
                                  ({row.atrDistance.toFixed(1)} ATR)
                                </span>
                              ) : null}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="text-xs text-nofx-text-muted border border-white/10 rounded px-3 py-2">
                  {language === 'zh' ? '无保护委托' : 'No protection orders'}
                </div>
              )}

              {/* Compact status bar */}
              <div className="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-nofx-text-muted">
                {currentDrawdownPct > 0 && (
                  <span>
                    DD{' '}
                    <span className="text-nofx-text-main font-mono">
                      {currentDrawdownPct.toFixed(1)}%
                    </span>
                  </span>
                )}
                <span>
                  {language === 'zh' ? '结构' : 'Struct'}:{' '}
                  <span className="text-nofx-text-main">
                    {compactSourceLabel(structureHealth, language)}
                  </span>
                </span>
                {runnerActive && (
                  <span>
                    Runner{' '}
                    <span className="text-nofx-text-main font-mono">
                      {runnerKeepPct > 0
                        ? `keep ${runnerKeepPct.toFixed(0)}%`
                        : 'active'}
                    </span>
                  </span>
                )}
                {runnerStopPrice > 0 && (
                  <span>
                    R-SL{' '}
                    <span className="text-nofx-text-main font-mono">
                      {formatPrice(runnerStopPrice)}
                    </span>
                  </span>
                )}
                {breakEvenSuppressedByRunner && (
                  <span className="text-amber-300">BE suppressed</span>
                )}
                {protectionQuantityDrift && (
                  <span className="text-amber-300">
                    {language === 'zh' ? '覆盖偏移' : 'qty drift'}
                  </span>
                )}
                {orphanProtectionCleanupNeeded && (
                  <span className="text-amber-300">
                    {language === 'zh' ? '需清理' : 'cleanup'}
                  </span>
                )}
                {runnerMigrationNeeded && (
                  <span className="text-cyan-300">
                    {language === 'zh' ? '迁移' : 'migrate'}:{' '}
                    {compactSourceLabel(runnerMigrationReason, language)}
                    {runnerMigrationSafe ? ' ✓' : ''}
                  </span>
                )}
              </div>

              {/* Details (collapsed) */}
              <CollapsibleSection
                title={language === 'zh' ? '详情' : 'Details'}
              >
                <div className="space-y-1 text-[10px]">
                  {runnerMigrationPlan?.cancel_order_id && (
                    <KV
                      label={language === 'zh' ? '迁移计划' : 'Migration'}
                      value={`${runnerMigrationPlan.cancel_order_id} → ${formatPrice(Number(runnerMigrationPlan.new_activation || 0))} cb ${(Number(runnerMigrationPlan.new_callback || 0) * 100).toFixed(2)}%`}
                    />
                  )}
                  {runnerStopSource && (
                    <KV
                      label={language === 'zh' ? 'Runner 来源' : 'Runner src'}
                      value={compactSourceLabel(runnerStopSource, language)}
                    />
                  )}
                  {entryRR && (
                    <KV
                      label={language === 'zh' ? '入场结构' : 'Entry struct'}
                      value={
                        formatRiskRewardLinkage(entryRR as never).join(' · ') ||
                        '—'
                      }
                    />
                  )}
                  {entryReviewSummary?.timeframe_context && (
                    <KV
                      label="TF"
                      value={
                        formatTimeframeTrail({
                          timeframe_context:
                            entryReviewSummary.timeframe_context,
                        } as never).join(' · ') || '—'
                      }
                    />
                  )}
                  {entryStructureAudit && (
                    <KV
                      label={language === 'zh' ? '审计' : 'Audit'}
                      value={
                        [
                          entryStructureAudit.audit_primary_timeframe
                            ? 'TF'
                            : '',
                          entryStructureAudit.audit_support_resistance
                            ? 'S/R'
                            : '',
                          entryStructureAudit.audit_structural_anchors
                            ? 'Anchors'
                            : '',
                          entryStructureAudit.audit_fibonacci ? 'Fib' : '',
                          entryStructureAudit.require_invalidation_target_linkage
                            ? 'Linkage'
                            : '',
                        ]
                          .filter(Boolean)
                          .join(' · ') || '—'
                      }
                    />
                  )}
                </div>
              </CollapsibleSection>
            </div>
          )
        })}

        {loading && (
          <div className="text-[11px] text-nofx-text-muted">
            {language === 'zh' ? '刷新中…' : 'Refreshing…'}
          </div>
        )}
        {error && <div className="text-xs text-nofx-red">{error}</div>}
      </div>
    </div>
  )
}
