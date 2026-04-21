import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { formatPrice, formatQuantity } from '../../utils/format'
import type { Language } from '../../i18n/translations'
import type { OpenOrder, Position } from '../../types'

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
                  title={language === 'zh' ? '保护总览' : 'Protection Overview'}
                  subtitle={language === 'zh' ? '先看整体，再看具体委托' : 'Read the overall state first, then inspect individual orders'}
                  rows={[
                    { label: language === 'zh' ? '保护状态' : 'Protection State', value: compactProtectionLabel(position.protection_state, language, exchange) },
                    { label: language === 'zh' ? 'Drawdown 模式' : 'Drawdown Mode', value: compactExecutionMode(position.drawdown_execution_mode, language) },
                    { label: language === 'zh' ? '当前利润' : 'Current PnL', value: formatSignedPercent(currentPnlPct) },
                    { label: language === 'zh' ? '峰值 / 回撤' : 'Peak / Drawdown', value: `${formatSignedPercent(peakPnlPct)} / ${currentDrawdownPct.toFixed(2)}%` },
                    { label: language === 'zh' ? '回撤来源' : 'Drawdown Source', value: drawdownConfigSource },
                    { label: language === 'zh' ? '当前档位' : 'Current Stage', value: currentStageMinProfit > 0 ? `${currentStageMinProfit.toFixed(2)}% (${currentStageRuleCount})` : '—' },
                    { label: language === 'zh' ? '满足 / 触发' : 'Satisfied / Triggered', value: `${satisfiedTiers.length} / ${triggeredTiers.length}` },
                    { label: language === 'zh' ? '下一档利润门槛' : 'Next Gate', value: nextTier ? `${Number(nextTier.min_profit_pct || 0).toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? 'Trailing 实盘委托' : 'Live Trailing Orders', value: `${trailingRows.length}` },
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
                    { label: language === 'zh' ? '实盘挂单' : 'Live Order', value: breakEvenOrderDetected ? (language === 'zh' ? '已委托' : 'Delegated') : (breakEvenTriggerPct > 0 ? (language === 'zh' ? '未委托' : 'Not placed') : '—') },
                    { label: language === 'zh' ? '保本价' : 'Break-even Price', value: liveBreakEvenStopPrice > 0 ? `${formatPrice(liveBreakEvenStopPrice)} / ${formatSignedPercent(((liveBreakEvenStopPrice - (position.entry_price || 0)) / (position.entry_price || 1)) * 100)}` : '—' },
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
