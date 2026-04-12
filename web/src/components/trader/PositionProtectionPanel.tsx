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
}

function formatUsdtValue(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 100 ? 2 : 4 })
}

function buildProtectionRows(position: Position, orders: OpenOrder[]) {
  const entryPrice = position.entry_price || 0
  const positionQty = position.quantity || 0

  return orders.map((order) => {
    const triggerPrice = order.stop_price || order.price || 0
    const closeRatioPct = positionQty > 0 && order.quantity > 0 ? (order.quantity / positionQty) * 100 : 0
    const valueUsdt = triggerPrice > 0 && order.quantity > 0 ? triggerPrice * order.quantity : 0
    return {
      orderId: order.order_id,
      type: String(order.type || '').toUpperCase(),
      triggerPrice,
      closeRatioPct,
      valueUsdt,
      entryPrice,
    }
  })
}

function normalizeSide(side?: string): string {
  return String(side || '').toUpperCase()
}

function formatProtectionState(state: string | undefined, language: Language, exchange?: string): string {
  if (!state) return language === 'zh' ? '未知' : 'unknown'
  const value = state.trim().toLowerCase()
  const exchangeLabel = exchange ? exchange.toUpperCase() : (language === 'zh' ? '交易所' : 'exchange')
  switch (value) {
    case 'exchange_protection_verified':
      return language === 'zh' ? '交易所保护已校验' : 'exchange protection verified'
    case 'break_even_armed':
      return language === 'zh' ? '保本保护已挂单' : 'break-even armed'
    case 'native_trailing_armed':
      return language === 'zh' ? `${exchangeLabel} 原生移动保护已激活（整仓）` : `${exchangeLabel} native trailing armed (full)`
    case 'native_partial_trailing_armed':
      return language === 'zh' ? `${exchangeLabel} 原生分批移动保护已激活` : `${exchangeLabel} native partial trailing armed`
    case 'managed_partial_drawdown_armed':
      return language === 'zh' ? '托管式分批回撤保护已激活' : 'managed partial drawdown armed'
    case 'drawdown_triggered':
      return language === 'zh' ? '回撤保护已触发' : 'drawdown triggered'
    default:
      return state
  }
}

function formatExecutionMode(mode: string | undefined, language: Language): string {
  if (!mode) return language === 'zh' ? '未确定' : 'undetermined'
  switch (mode.trim().toLowerCase()) {
    case 'native_trailing_full':
      return language === 'zh' ? '交易所原生 trailing（整仓）' : 'exchange-native trailing (full close)'
    case 'native_partial_trailing':
      return language === 'zh' ? '交易所原生 trailing（分批）' : 'exchange-native trailing (partial close)'
    case 'managed_partial_drawdown':
      return language === 'zh' ? '托管式分批回撤保护' : 'managed partial drawdown'
    case 'native_trailing_pending':
      return language === 'zh' ? '支持原生 trailing（待满足激活条件）' : 'native trailing supported (awaiting activation)'
    case 'disabled':
      return language === 'zh' ? '未启用回撤保护' : 'drawdown disabled'
    case 'native_full_local_partial':
      return language === 'zh' ? '整仓原生 / 分批本地' : 'native for full close / local for partial'
    case 'native_stop':
      return language === 'zh' ? '交易所原生 stop' : 'exchange-native stop'
    case 'local_only':
      return language === 'zh' ? '仅本地执行' : 'local only'
    case 'local_fallback':
      return language === 'zh' ? '本地 fallback（含分批回撤）' : 'local fallback (incl. partial drawdown)'
    default:
      return mode
  }
}

export function PositionProtectionPanel({ traderId, positions, language, exchange }: PositionProtectionPanelProps) {
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

      <div className="relative z-10 flex items-center justify-between mb-4">
        <div>
          <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2">
            <span className="text-purple-400">🛡</span>
            {language === 'zh' ? '持仓保护执行面板' : 'Position Protection Runtime'}
          </h3>
          <p className="text-xs text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '按持仓展示当前生效保护、未来触发动作，以及执行来源'
              : 'Per-position view of active protections, upcoming triggers, and execution source'}
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
          const nextTier = runtimeTiers.length > 0 ? runtimeTiers[0] : null
          const currentPnlPct = position.unrealized_pnl_pct || 0

          return (
            <div key={`${symbol}-${side}-${index}`} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div>
                  <div className="font-semibold text-nofx-text-main">{symbol} / {side}</div>
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
                <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                  <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '当前保护状态' : 'Protection State'}</div>
                  <div className="font-mono text-nofx-text-main">{formatProtectionState(position.protection_state, language, exchange)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                  <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '回撤止盈执行模式' : 'Drawdown Execution Mode'}</div>
                  <div className="font-mono text-nofx-text-main">{formatExecutionMode(position.drawdown_execution_mode, language)}</div>
                </div>
                {nextTier && (
                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 md:col-span-2">
                    <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '原生 trailing 激活条件' : 'Native trailing activation gate'}</div>
                    <div className="font-mono text-nofx-text-main">
                      {language === 'zh'
                        ? `当前利润 ${currentPnlPct.toFixed(2)}% / 最低要求 ${Number(nextTier.min_profit_pct || 0).toFixed(2)}%`
                        : `Current PnL ${currentPnlPct.toFixed(2)}% / Required ${Number(nextTier.min_profit_pct || 0).toFixed(2)}%`}
                    </div>
                    {currentPnlPct < Number(nextTier.min_profit_pct || 0) && (
                      <div className="text-nofx-text-muted mt-1">
                        {language === 'zh'
                          ? '尚未达到最小利润门槛，所以不会挂出 drawdown trailing 委托。'
                          : 'Drawdown trailing will not be armed until min-profit threshold is reached.'}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="rounded-xl border border-cyan-400/20 bg-cyan-500/5 p-4">
                <div className="font-semibold text-cyan-300 mb-3">{language === 'zh' ? '当前保护委托' : 'Current Protection Orders'}</div>
                <div className="space-y-2 text-xs">
                  {protectionRows.map((row) => (
                    <div key={row.orderId} className="rounded-lg border border-cyan-400/10 bg-black/20 p-3 grid grid-cols-1 md:grid-cols-4 gap-2">
                      <div>
                        <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '类型' : 'Type'}</div>
                        <div className="font-mono text-nofx-text-main">{row.type}</div>
                      </div>
                      <div>
                        <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '触发价格' : 'Trigger Price'}</div>
                        <div className="font-mono text-nofx-text-main">{formatPrice(row.triggerPrice)}</div>
                      </div>
                      <div>
                        <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '成交比例' : 'Close Ratio'}</div>
                        <div className="font-mono text-nofx-text-main">{row.closeRatioPct > 0 ? `${row.closeRatioPct.toFixed(1)}%` : '—'}</div>
                      </div>
                      <div>
                        <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '仓位价值 USDT' : 'Value USDT'}</div>
                        <div className="font-mono text-nofx-text-main">{row.valueUsdt > 0 ? formatUsdtValue(row.valueUsdt) : '—'}</div>
                      </div>
                    </div>
                  ))}
                  {protectionRows.length === 0 && (
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-nofx-text-muted">
                      {language === 'zh' ? '当前未检测到已生效中的保护委托。' : 'No active protection orders detected.'}
                    </div>
                  )}
                </div>
              </div>

              <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-nofx-text-muted leading-6 space-y-2">
                <div className="font-semibold text-nofx-text-main mb-1">{language === 'zh' ? '执行边界说明' : 'Execution Boundary Notes'}</div>
                <ul className="list-disc pl-5 space-y-1">
                  <li>{language === 'zh' ? '当前面板已区分：交易所原生 trailing（整仓 / 分批）与托管式分批回撤保护。' : 'This panel now distinguishes exchange-native trailing (full / partial) from managed partial drawdown.'}</li>
                  <li>{language === 'zh' ? '原生 trailing 一旦 armed，本地不应再伪造第二套执行计划；若看到 managed，则说明当前仍未走上原生 partial。' : 'Once native trailing is armed, local fallback should not present a second execution path; managed indicates partial native is not in effect.'}</li>
                  <li>{language === 'zh' ? '保本止损 armed 状态会随仓位数量/开仓价变化自动重置。' : 'Break-even armed state auto-resets when position size or entry price changes.'}</li>
                </ul>
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
