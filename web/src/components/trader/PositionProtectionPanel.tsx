import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { formatPrice, formatQuantity } from '../../utils/format'
import type { Language } from '../../i18n/translations'
import type { OpenOrder, Position } from '../../types'

interface PositionProtectionPanelProps {
  traderId?: string
  positions?: Position[]
  language: Language
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

function formatProtectionState(state: string | undefined, language: Language): string {
  if (!state) return language === 'zh' ? '未知' : 'unknown'
  const value = state.trim().toLowerCase()
  switch (value) {
    case 'exchange_protection_verified':
      return language === 'zh' ? '交易所保护已校验' : 'exchange protection verified'
    case 'break_even_armed':
      return language === 'zh' ? '保本保护已挂单' : 'break-even armed'
    case 'native_trailing_armed':
      return language === 'zh' ? '交易所原生移动保护已激活' : 'native trailing armed'
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

export function PositionProtectionPanel({ traderId, positions, language }: PositionProtectionPanelProps) {
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
            {language === 'zh' ? '多持仓保护状态' : 'Multi-position Protection Status'}
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
            {language === 'zh' ? '多持仓保护状态' : 'Multi-position Protection Status'}
          </h3>
          <p className="text-xs text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '每个持仓独立展示其交易所委托保护与本地运行态保护说明'
              : 'Each position shows its own exchange-native protection and local runtime notes'}
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

          return (
            <div key={`${symbol}-${side}-${index}`} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div>
                  <div className="font-semibold text-nofx-text-main">{symbol} / {side}</div>
                  <div className="text-xs text-nofx-text-muted mt-1">
                    {language === 'zh' ? '逐仓保护状态' : 'Per-position protection status'}
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

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-4">
                  <div className="flex items-center justify-between mb-3">
                    <div className="font-semibold text-red-300">{language === 'zh' ? '交易所委托保护：止损' : 'Exchange-native Protection: Stop Loss'}</div>
                    <span className={`text-[10px] px-2 py-1 rounded-full ${stopOrders.length > 0 ? 'bg-red-500/15 text-red-300' : 'bg-white/5 text-nofx-text-muted'}`}>
                      {stopOrders.length > 0 ? `${stopOrders.length} ${language === 'zh' ? '条委托' : 'orders'}` : language === 'zh' ? '未检测到' : 'not detected'}
                    </span>
                  </div>
                  {stopOrders.length > 0 ? (
                    <div className="space-y-2">
                      {stopOrders.map((order) => {
                        const trigger = order.stop_price || order.price
                        return (
                          <div key={`sl-${order.order_id}`} className="rounded-lg border border-red-500/10 bg-black/20 p-3 text-xs">
                            <div className="flex justify-between gap-3"><span className="text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}</span><span className="font-mono text-red-200">{formatPrice(trigger)}</span></div>
                            <div className="flex justify-between gap-3 mt-1"><span className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}</span><span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span></div>
                            <div className="flex justify-between gap-3 mt-1"><span className="text-nofx-text-muted">Type</span><span className="font-mono text-nofx-text-main">{order.type}</span></div>
                          </div>
                        )
                      })}
                    </div>
                  ) : (
                    <div className="text-xs text-nofx-text-muted space-y-1">
                      <div>{language === 'zh' ? '当前没有识别到与该仓位匹配的止损委托。' : 'No stop-loss order currently matched for this position.'}</div>
                      <div>{language === 'zh' ? '原则：只要交易所支持，止损应优先下到交易所。' : 'Policy: if supported, stop loss should be maintained on-exchange.'}</div>
                    </div>
                  )}
                </div>

                <div className="rounded-xl border border-green-500/20 bg-green-500/5 p-4">
                  <div className="flex items-center justify-between mb-3">
                    <div className="font-semibold text-green-300">{language === 'zh' ? '交易所委托保护：止盈' : 'Exchange-native Protection: Take Profit'}</div>
                    <span className={`text-[10px] px-2 py-1 rounded-full ${takeProfitOrders.length > 0 ? 'bg-green-500/15 text-green-300' : 'bg-white/5 text-nofx-text-muted'}`}>
                      {takeProfitOrders.length > 0 ? `${takeProfitOrders.length} ${language === 'zh' ? '条委托' : 'orders'}` : language === 'zh' ? '未检测到' : 'not detected'}
                    </span>
                  </div>
                  {takeProfitOrders.length > 0 ? (
                    <div className="space-y-2">
                      {takeProfitOrders.map((order) => {
                        const trigger = order.stop_price || order.price
                        return (
                          <div key={`tp-${order.order_id}`} className="rounded-lg border border-green-500/10 bg-black/20 p-3 text-xs">
                            <div className="flex justify-between gap-3"><span className="text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}</span><span className="font-mono text-green-200">{formatPrice(trigger)}</span></div>
                            <div className="flex justify-between gap-3 mt-1"><span className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}</span><span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span></div>
                            <div className="flex justify-between gap-3 mt-1"><span className="text-nofx-text-muted">Type</span><span className="font-mono text-nofx-text-main">{order.type}</span></div>
                          </div>
                        )
                      })}
                    </div>
                  ) : (
                    <div className="text-xs text-nofx-text-muted space-y-1">
                      <div>{language === 'zh' ? '当前没有识别到与该仓位匹配的止盈委托。' : 'No take-profit order currently matched for this position.'}</div>
                      <div>{language === 'zh' ? '原则：只要交易所支持，止盈应优先下到交易所。' : 'Policy: if supported, take profit should be maintained on-exchange.'}</div>
                    </div>
                  )}
                </div>
              </div>

              <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-nofx-text-muted leading-6 space-y-2">
                <div className="font-semibold text-nofx-text-main mb-1">{language === 'zh' ? '本地运行态保护' : 'Local Runtime Protection'}</div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                  <div className="rounded border border-white/10 px-3 py-2 bg-black/20">
                    <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '保护巡检状态' : 'Protection Reconcile State'}</div>
                    <div className="font-mono text-nofx-text-main">{formatProtectionState(position.protection_state, language)}</div>
                  </div>
                  <div className="rounded border border-white/10 px-3 py-2 bg-black/20">
                    <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '保本止损状态' : 'Break-even State'}</div>
                    <div className="font-mono text-nofx-text-main">{formatBreakEvenState(position.break_even_state, language)}</div>
                  </div>
                </div>
                <ul className="list-disc pl-5 space-y-1">
                  <li>{language === 'zh' ? 'Drawdown Take Profit 与 Break-even Stop 仅在交易所无法直接表达规则时，才应由本地持续监控执行。' : 'Drawdown TP and Break-even Stop should only remain local when the exchange cannot express the rule natively.'}</li>
                  <li>{language === 'zh' ? '每个持仓都应独立维护保护，不应因别的持仓已完成挂单而停止监控。' : 'Each position should keep its own protection state; one protected position must not stop monitoring another.'}</li>
                </ul>
              </div>
            </div>
          )
        })}

        <div className="rounded-lg border border-indigo-400/20 bg-indigo-500/5 p-4 text-xs text-nofx-text-muted leading-6">
          <div className="font-semibold text-indigo-300 mb-2">{language === 'zh' ? '当前交易所原生保护能力摘要（基于系统能力矩阵）' : 'Current Exchange-native Protection Capability Summary'}</div>
          <ul className="list-disc pl-5 space-y-1">
            <li>{language === 'zh' ? 'Binance / OKX：原生 stop / tp / partial close 能力较完整。' : 'Binance / OKX currently expose the strongest native stop / tp / partial-close support in the system.'}</li>
            <li>{language === 'zh' ? 'Gate / KuCoin / Bybit / Bitget / Aster：支持原生 stop / tp / partial close，但改单能力相对弱。' : 'Gate / KuCoin / Bybit / Bitget / Aster support native stop / tp / partial close, but amend flows are weaker.'}</li>
            <li>{language === 'zh' ? 'Hyperliquid：支持原生 stop / tp，但 stop/tp 区分与取消存在特殊性。' : 'Hyperliquid supports native stop / tp, but stop/tp distinction and cancellation semantics are special.'}</li>
            <li>{language === 'zh' ? 'Lighter：支持 stop / tp，但 partial close 能力矩阵当前偏保守。' : 'Lighter supports stop / tp, but the current safety matrix is conservative about partial close.'}</li>
          </ul>
        </div>

        {loading && <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '正在刷新保护状态…' : 'Refreshing protection status…'}</div>}
        {error && <div className="text-xs text-nofx-red">{error}</div>}
      </div>
    </div>
  )
}
