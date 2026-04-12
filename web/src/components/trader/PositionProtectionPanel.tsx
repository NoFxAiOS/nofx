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

interface ScheduledAction {
  title: string
  source: string
  trigger: string
  action: string
}

function isStopLoss(order: OpenOrder): boolean {
  const kind = String(order.type || '').toUpperCase()
  return kind.includes('STOP') && !kind.includes('TAKE_PROFIT') && !kind.includes('TP')
}

function isTakeProfit(order: OpenOrder): boolean {
  const kind = String(order.type || '').toUpperCase()
  return kind.includes('TAKE_PROFIT') || kind.includes('TP')
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

function formatBreakEvenState(state: string | undefined, language: Language): string {
  if (!state || state.trim() === '') return language === 'zh' ? '未触发' : 'idle'
  switch (state.trim().toLowerCase()) {
    case 'armed':
      return language === 'zh' ? '已挂单 / 已武装' : 'armed'
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

function buildScheduledActions(position: Position, stopOrders: OpenOrder[], takeProfitOrders: OpenOrder[], language: Language, exchange?: string): ScheduledAction[] {
  const actions: ScheduledAction[] = []

  for (const order of stopOrders) {
    const trigger = order.stop_price || order.price
    const isTrailing = String(order.type || '').toUpperCase().includes('TRAILING')
    actions.push({
      title: isTrailing
        ? (language === 'zh' ? '交易所跟踪保护动作' : 'Exchange trailing protection action')
        : (language === 'zh' ? '交易所止损动作' : 'Exchange stop-loss action'),
      source: exchange ? `${exchange.toUpperCase()} ${language === 'zh' ? '原生委托' : 'native order'}` : (language === 'zh' ? '交易所原生委托' : 'exchange-native order'),
      trigger: isTrailing
        ? `${language === 'zh' ? '激活价' : 'Activation'} ${formatPrice(trigger)}`
        : `${language === 'zh' ? '价格到达' : 'Price reaches'} ${formatPrice(trigger)}`,
      action: isTrailing
        ? `${language === 'zh' ? '按跟踪委托执行保护平仓' : 'Protective close via trailing order'} (${formatQuantity(order.quantity)})`
        : `${language === 'zh' ? '执行止损平仓' : 'Execute stop-loss close'} (${formatQuantity(order.quantity)})`,
    })
  }

  for (const order of takeProfitOrders) {
    const trigger = order.stop_price || order.price
    actions.push({
      title: language === 'zh' ? '交易所止盈动作' : 'Exchange take-profit action',
      source: exchange ? `${exchange.toUpperCase()} ${language === 'zh' ? '原生委托' : 'native order'}` : (language === 'zh' ? '交易所原生委托' : 'exchange-native order'),
      trigger: `${language === 'zh' ? '价格到达' : 'Price reaches'} ${formatPrice(trigger)}`,
      action: `${language === 'zh' ? '执行止盈平仓' : 'Execute take-profit close'} (${formatQuantity(order.quantity)})`,
    })
  }

  if (position.protection_state?.toLowerCase() === 'native_trailing_armed') {
    actions.push({
      title: language === 'zh' ? '原生回撤止盈' : 'Native drawdown trailing',
      source: exchange ? `${exchange.toUpperCase()} ${language === 'zh' ? '原生 trailing' : 'native trailing'}` : (language === 'zh' ? '交易所原生 trailing' : 'exchange-native trailing'),
      trigger: language === 'zh' ? '达到 trailing 激活条件后，随价格动态跟踪' : 'After activation, trailing stop follows price natively',
      action: language === 'zh' ? '发生回撤时整仓退出' : 'Exit full position on qualified drawdown',
    })
  }

  if (position.protection_state?.toLowerCase() === 'native_partial_trailing_armed') {
    actions.push({
      title: language === 'zh' ? '原生分批回撤止盈' : 'Native partial drawdown trailing',
      source: exchange ? `${exchange.toUpperCase()} ${language === 'zh' ? '原生 trailing' : 'native trailing'}` : (language === 'zh' ? '交易所原生 trailing' : 'exchange-native trailing'),
      trigger: language === 'zh' ? '达到 partial trailing 激活条件后，随价格动态跟踪' : 'After partial-trailing activation, exchange tracks price natively',
      action: language === 'zh' ? '发生回撤时执行分批平仓' : 'Execute partial close on qualified drawdown',
    })
  }

  if (position.protection_state?.toLowerCase() === 'managed_partial_drawdown_armed') {
    actions.push({
      title: language === 'zh' ? '托管式分批回撤保护' : 'Managed partial drawdown',
      source: language === 'zh' ? '系统托管保护 / 标准 TP 条件单' : 'system-managed protection / standard TP order',
      trigger: language === 'zh' ? '达到利润门槛并满足回撤比例后，按规则触发' : 'Triggered when profit threshold and drawdown ratio are both satisfied',
      action: language === 'zh' ? '执行分批平仓（非交易所原生 trailing）' : 'Execute partial close (not exchange-native trailing)',
    })
  }

  if ((position.break_even_state || '').toLowerCase() === 'armed') {
    actions.push({
      title: language === 'zh' ? '保本止损动作' : 'Break-even stop action',
      source: language === 'zh' ? '交易所原生 stop / 本地触发后挂单' : 'exchange-native stop / armed after local trigger',
      trigger: language === 'zh' ? '已达到保本触发条件' : 'Break-even trigger already satisfied',
      action: language === 'zh' ? '价格回到保本位附近时执行保护平仓' : 'Protective close around break-even stop level',
    })
  }

  if (actions.length === 0) {
    actions.push({
      title: language === 'zh' ? '暂无已编排动作' : 'No scheduled actions yet',
      source: language === 'zh' ? '当前未检测到交易所委托或已武装运行态保护' : 'No exchange orders or armed runtime protections detected yet',
      trigger: language === 'zh' ? '等待保护委托建立或运行态规则触发' : 'Waiting for protection orders or runtime triggers',
      action: language === 'zh' ? '当前不会给出虚假执行计划' : 'No fabricated execution plan is shown',
    })
  }

  return actions
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
          const stopOrders = filteredOrders.filter(isStopLoss)
          const takeProfitOrders = filteredOrders.filter(isTakeProfit)
          const runtime = position.protection_runtime
          const runtimeTiers = runtime?.scheduled_tiers || []
          const scheduledActions = buildScheduledActions(position, stopOrders, takeProfitOrders, language, exchange)

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
                  <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '保本状态' : 'Break-even State'}</div>
                  <div className="font-mono text-nofx-text-main">{formatBreakEvenState(position.break_even_state, language)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                  <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '回撤止盈执行模式' : 'Drawdown Execution Mode'}</div>
                  <div className="font-mono text-nofx-text-main">{formatExecutionMode(position.drawdown_execution_mode, language)}</div>
                </div>
                <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                  <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '保本止损执行模式' : 'Break-even Execution Mode'}</div>
                  <div className="font-mono text-nofx-text-main">{formatExecutionMode(position.break_even_execution_mode, language)}</div>
                </div>
              </div>

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-4">
                  <div className="font-semibold text-red-300 mb-3">{language === 'zh' ? '当前生效中的保护' : 'Active Protections'}</div>
                  <div className="space-y-2 text-xs">
                    {stopOrders.map((order) => (
                      <div key={`sl-${order.order_id}`} className="rounded-lg border border-red-500/10 bg-black/20 p-3">
                        <div className="font-semibold text-red-200">{language === 'zh' ? '交易所止损' : 'Exchange stop-loss'}</div>
                        <div className="mt-1 text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}: <span className="font-mono text-nofx-text-main">{formatPrice(order.stop_price || order.price)}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}: <span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span></div>
                        <div className="text-nofx-text-muted">Type: <span className="font-mono text-nofx-text-main">{order.type}</span></div>
                      </div>
                    ))}
                    {takeProfitOrders.map((order) => (
                      <div key={`tp-${order.order_id}`} className="rounded-lg border border-green-500/10 bg-black/20 p-3">
                        <div className="font-semibold text-green-200">{language === 'zh' ? '交易所止盈' : 'Exchange take-profit'}</div>
                        <div className="mt-1 text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}: <span className="font-mono text-nofx-text-main">{formatPrice(order.stop_price || order.price)}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}: <span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span></div>
                        <div className="text-nofx-text-muted">Type: <span className="font-mono text-nofx-text-main">{order.type}</span></div>
                      </div>
                    ))}
                    {stopOrders.length === 0 && takeProfitOrders.length === 0 && (
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-nofx-text-muted">
                        {language === 'zh' ? '当前未检测到已生效中的交易所保护委托。' : 'No active exchange protection orders detected yet.'}
                      </div>
                    )}
                  </div>
                </div>

                <div className="rounded-xl border border-indigo-400/20 bg-indigo-500/5 p-4">
                  <div className="font-semibold text-indigo-300 mb-3">{language === 'zh' ? '未来触发动作' : 'Scheduled Protection Actions'}</div>
                  <div className="space-y-2 text-xs">
                    {scheduledActions.map((item, i) => (
                      <div key={`${symbol}-${i}`} className="rounded-lg border border-indigo-400/10 bg-black/20 p-3">
                        <div className="font-semibold text-indigo-200">{item.title}</div>
                        <div className="mt-1 text-nofx-text-muted">{language === 'zh' ? '执行来源' : 'Source'}: <span className="text-nofx-text-main">{item.source}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '触发条件' : 'Trigger'}: <span className="text-nofx-text-main">{item.trigger}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '执行动作' : 'Action'}: <span className="text-nofx-text-main">{item.action}</span></div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {runtimeTiers.length > 0 && (
                <div className="rounded-xl border border-cyan-400/20 bg-cyan-500/5 p-4">
                  <div className="font-semibold text-cyan-300 mb-3">{language === 'zh' ? '多档保护执行计划' : 'Tiered Protection Plan'}</div>
                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 text-xs">
                    {runtimeTiers.map((tier) => (
                      <div key={`${symbol}-tier-${tier.index}`} className="rounded-lg border border-cyan-400/10 bg-black/20 p-3 space-y-1">
                        <div className="font-semibold text-cyan-200">{language === 'zh' ? `第 ${tier.index} 档` : `Tier ${tier.index}`}</div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '来源' : 'Source'}: <span className="text-nofx-text-main">{tier.source}</span></div>
                        <div className="text-nofx-text-muted">Mode: <span className="text-nofx-text-main">{tier.execution_mode}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '最小利润' : 'Min Profit'}: <span className="text-nofx-text-main">{tier.min_profit_pct}%</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '最大回撤' : 'Max Drawdown'}: <span className="text-nofx-text-main">{tier.max_drawdown_pct}%</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '平仓比例' : 'Close Ratio'}: <span className="text-nofx-text-main">{tier.close_ratio_pct}%</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '激活价' : 'Activation'}: <span className="font-mono text-nofx-text-main">{formatPrice(tier.activation_price)}</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '回撤比例' : 'Callback'}: <span className="text-nofx-text-main">{tier.callback_rate}%</span></div>
                        <div className="text-nofx-text-muted">{language === 'zh' ? '计划数量' : 'Planned Qty'}: <span className="font-mono text-nofx-text-main">{formatQuantity(tier.planned_quantity)}</span></div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

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
