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

function formatUsdtValue(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 100 ? 2 : 4 })
}

function buildProtectionRows(position: Position, orders: OpenOrder[]) {
  const positionQty = position.quantity || 0

  return orders.map((order) => {
    const triggerPrice = order.stop_price || order.price || 0
    const closeRatioPct = positionQty > 0 && order.quantity > 0 ? (order.quantity / positionQty) * 100 : 0
    const valueUsdt = triggerPrice > 0 && order.quantity > 0 ? triggerPrice * order.quantity : 0
    return {
      orderId: order.order_id,
      type: String(order.type || '').toUpperCase(),
      triggerPrice,
      callbackRate: Number(order.callback_rate || 0),
      closeRatioPct,
      valueUsdt,
    }
  })
}

function normalizeSide(side?: string): string {
  return String(side || '').toUpperCase()
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
    const timer = window.setInterval(load, 30000)
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
              ? '按四类保护展示当前委托、原生 / 本地执行方式与关键参数'
              : 'Grouped by the four protection types with active orders, native/local execution, and key parameters'}
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
          const protectionRows = buildProtectionRows(position, filteredOrders)
          const runtimeTiers = position.protection_runtime?.scheduled_tiers || []
          const currentPnlPct = Number(position.protection_runtime?.current_pnl_pct ?? position.unrealized_pnl_pct ?? 0)
          const currentDrawdownPct = Number(position.protection_runtime?.current_drawdown_pct ?? 0)
          const peakPnlPct = Number(position.protection_runtime?.drawdown_peak_pnl_pct ?? currentPnlPct)
          const currentStageMinProfit = Number(position.protection_runtime?.current_drawdown_stage_min_profit_pct ?? 0)
          const currentStageRuleCount = Number(position.protection_runtime?.current_drawdown_stage_rule_count ?? 0)
          const satisfiedTiers = runtimeTiers.filter((tier) => Boolean(tier.is_satisfied))
          const triggeredTiers = runtimeTiers.filter((tier) => Boolean(tier.is_triggered))
          const nextTier = runtimeTiers.find((tier) => !tier.is_satisfied) || runtimeTiers[0] || null
          const currentStageTier = satisfiedTiers.length > 0 ? satisfiedTiers[satisfiedTiers.length - 1] : null
          const breakEvenTriggerPct = Number(position.protection_runtime?.current_break_even_trigger_pct ?? 0)
          const breakEvenGapPct = Number(position.protection_runtime?.next_break_even_gap_pct ?? 0)
          const breakEvenOffsetPct = Number(position.protection_runtime?.break_even_offset_pct ?? 0)
          const breakEvenConfigSource = String(position.protection_runtime?.break_even_config_source || 'strategy')
          const liveBreakEvenStopPrice = Number(position.protection_runtime?.live_break_even_stop_price ?? 0)
          const breakEvenOrderDetected = Boolean(position.protection_runtime?.break_even_order_detected)
          const trailingOrders = protectionRows.filter((row) => row.type.includes('TRAILING'))
          const liveTrailingPrice = trailingOrders.length > 0 ? trailingOrders[0].triggerPrice : 0

          return (
            <div key={`${symbol}-${side}-${index}`} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div>
                  <div className="font-semibold text-nofx-text-main">
                    <button
                      type="button"
                      onClick={() => onSymbolClick?.(symbol)}
                      className="hover:text-cyan-300 transition-colors"
                    >
                      {symbol} / {side}
                    </button>
                  </div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {language === 'zh' ? '开仓后保护执行视图' : 'Post-open protection execution view'}
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
                  title={language === 'zh' ? '委托型止损（Full / Ladder SL）' : 'Order-based Stop Loss (Full / Ladder SL)'}
                  subtitle={language === 'zh' ? '长期保留，除非仓位扩张时更新' : 'Long-lived protection; update only when position expands'}
                  rows={[
                    { label: language === 'zh' ? '当前状态' : 'State', value: compactProtectionLabel(position.protection_state, language, exchange) },
                    { label: language === 'zh' ? '执行模式' : 'Mode', value: compactExecutionMode(position.drawdown_execution_mode, language) },
                    { label: language === 'zh' ? '委托数量' : 'Orders', value: String(protectionRows.filter((r) => r.type.includes('STOP') && !r.type.includes('TRAILING')).length) },
                  ]}
                />
                <ProtectionCard
                  title={language === 'zh' ? '盈利控制（Drawdown / Native Trailing）' : 'Profit Control (Drawdown / Native Trailing)'}
                  subtitle={language === 'zh' ? '与 generic TP 互斥，接管止盈侧' : 'Owns the TP side and is mutually exclusive with generic TP'}
                  rows={[
                    { label: language === 'zh' ? '执行模式' : 'Mode', value: compactExecutionMode(position.drawdown_execution_mode, language) },
                    { label: language === 'zh' ? '最低利润门槛' : 'Min Profit Gate', value: nextTier ? `${Number(nextTier.min_profit_pct || 0).toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '当前利润' : 'Current PnL', value: `${currentPnlPct.toFixed(2)}%` },
                    { label: language === 'zh' ? '峰值 / 回撤' : 'Peak / Drawdown', value: `${peakPnlPct.toFixed(2)}% / ${currentDrawdownPct.toFixed(2)}%` },
                    { label: language === 'zh' ? '当前档位' : 'Current Stage', value: currentStageMinProfit > 0 ? `${currentStageMinProfit.toFixed(2)}% (${currentStageRuleCount})` : '—' },
                    { label: language === 'zh' ? '满足 / 触发档' : 'Satisfied / Triggered', value: `${satisfiedTiers.length} / ${triggeredTiers.length}` },
                    { label: language === 'zh' ? '激活价（已挂）' : 'Armed Activation', value: liveTrailingPrice > 0 ? `${formatPrice(liveTrailingPrice)}${currentStageTier?.activation_source ? ` / ${currentStageTier.activation_source}` : nextTier?.activation_source ? ` / ${nextTier.activation_source}` : ''}` : '—' },
                    { label: language === 'zh' ? '激活价（理论）' : 'Planned Activation', value: nextTier && Number(nextTier.planned_activation_price || 0) > 0 ? formatPrice(Number(nextTier.planned_activation_price || 0)) : '—' },
                    { label: language === 'zh' ? '回撤 / 回调' : 'Giveback / Callback', value: nextTier ? `${Number(nextTier.max_drawdown_pct || 0).toFixed(2)}% / ${Number(nextTier.callback_rate || 0).toFixed(4)}${nextTier.callback_source ? ` / ${nextTier.callback_source}` : ''}` : '—' },
                    { label: language === 'zh' ? '本地监测' : 'Local Monitor', value: language === 'zh' ? '运行中' : 'active' },
                  ]}
                />
                <ProtectionCard
                  title={language === 'zh' ? '保本止损（Break-even）' : 'Break-even Stop'}
                  subtitle={language === 'zh' ? '独立管理，不覆盖 Drawdown / Ladder / Full' : 'Managed independently; does not replace Drawdown / Ladder / Full'}
                  rows={[
                    { label: language === 'zh' ? '当前状态' : 'State', value: position.break_even_state || 'idle' },
                    { label: language === 'zh' ? '触发阈值' : 'Trigger Threshold', value: breakEvenTriggerPct > 0 ? `${breakEvenTriggerPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '距触发还差' : 'Gap to Trigger', value: breakEvenTriggerPct > 0 ? `${breakEvenGapPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '保本偏移' : 'Offset', value: breakEvenTriggerPct > 0 ? `${breakEvenOffsetPct.toFixed(2)}%` : '—' },
                    { label: language === 'zh' ? '配置来源' : 'Config Source', value: breakEvenConfigSource },
                    { label: language === 'zh' ? '实盘挂单' : 'Live Order', value: breakEvenOrderDetected ? (language === 'zh' ? '已检测' : 'detected') : (language === 'zh' ? '未检测' : 'not detected') },
                    { label: language === 'zh' ? '保本价' : 'Break-even Price', value: liveBreakEvenStopPrice > 0 ? formatPrice(liveBreakEvenStopPrice) : '—' },
                    { label: language === 'zh' ? '本地监测' : 'Local Monitor', value: 'on' },
                  ]}
                />
                <ProtectionCard
                  title={language === 'zh' ? '当前保护委托' : 'Current Protection Orders'}
                  subtitle={language === 'zh' ? '仅展示与交易/保护强相关的信息' : 'Only high-signal trading / protection details'}
                  rows={protectionRows.length > 0 ? protectionRows.map((row) => ({
                    label: `${row.type} @ ${formatPrice(row.triggerPrice)}${row.callbackRate > 0 ? ` / cb ${row.callbackRate.toFixed(4)}` : ''}`,
                    value: `${row.closeRatioPct > 0 ? `${row.closeRatioPct.toFixed(1)}%` : '—'} / ${row.valueUsdt > 0 ? formatUsdtValue(row.valueUsdt) : '—'}U`,
                  })) : [{ label: language === 'zh' ? '委托' : 'Orders', value: language === 'zh' ? '暂无' : 'none' }]}
                />
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
