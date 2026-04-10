import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { formatPrice, formatQuantity } from '../../utils/format'
import type { Language } from '../../i18n/translations'
import type { OpenOrder, Position } from '../../types'

interface PositionProtectionPanelProps {
  traderId?: string
  position?: Position
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

export function PositionProtectionPanel({ traderId, position, language }: PositionProtectionPanelProps) {
  const [orders, setOrders] = useState<OpenOrder[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function load() {
      if (!traderId || !position?.symbol) {
        setOrders([])
        setError(null)
        return
      }

      setLoading(true)
      setError(null)
      try {
        const data = await api.getOpenOrders(traderId, position.symbol)
        if (!cancelled) {
          setOrders(Array.isArray(data) ? data : [])
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load protection orders')
          setOrders([])
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
  }, [traderId, position?.symbol])

  const filteredOrders = useMemo(() => {
    if (!position) return []
    const side = normalizeSide(position.side)
    return orders.filter((order) => {
      if (String(order.symbol || '').toUpperCase() !== String(position.symbol || '').toUpperCase()) return false
      const orderPosSide = normalizeSide(order.position_side)
      return !orderPosSide || orderPosSide === side
    })
  }, [orders, position])

  const stopOrders = filteredOrders.filter(isStopLoss)
  const takeProfitOrders = filteredOrders.filter(isTakeProfit)
  const hasRuntimeConfigHint = position != null

  return (
    <div className="nofx-glass p-5 animate-slide-in relative overflow-hidden group" style={{ animationDelay: '0.18s' }}>
      <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
        <div className="w-24 h-24 rounded-full bg-purple-500 blur-3xl" />
      </div>

      <div className="relative z-10 flex items-center justify-between mb-4">
        <div>
          <h3 className="text-lg font-bold text-nofx-text-main uppercase tracking-wide flex items-center gap-2">
            <span className="text-purple-400">🛡</span>
            {language === 'zh' ? '当前仓位保护状态' : 'Position Protection Status'}
          </h3>
          <p className="text-xs text-nofx-text-muted mt-1">
            {position
              ? `${position.symbol} / ${String(position.side).toUpperCase()}`
              : language === 'zh'
                ? '点击一条当前持仓后查看其保护状态'
                : 'Click a current position to inspect its protection status'}
          </p>
        </div>
      </div>

      {!position ? (
        <div className="rounded-lg border border-white/10 bg-black/20 px-4 py-5 text-sm text-nofx-text-muted">
          {language === 'zh'
            ? '尚未选中持仓。请在 Current Positions 里点击任意持仓行。'
            : 'No position selected yet. Click a row in Current Positions.'}
        </div>
      ) : (
        <div className="space-y-4 relative z-10">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '持仓数量' : 'Position Size'}</div>
              <div className="font-mono text-nofx-text-main">{formatQuantity(position.quantity)}</div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '开仓价' : 'Entry Price'}</div>
              <div className="font-mono text-nofx-text-main">{formatPrice(position.entry_price)}</div>
            </div>
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="text-nofx-text-muted mb-1">{language === 'zh' ? '未实现盈亏%' : 'Unrealized PnL %'}</div>
              <div className={`font-mono ${position.unrealized_pnl_pct >= 0 ? 'text-nofx-green' : 'text-nofx-red'}`}>
                {position.unrealized_pnl_pct >= 0 ? '+' : ''}
                {position.unrealized_pnl_pct.toFixed(2)}%
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="font-semibold text-red-300">
                  {language === 'zh' ? '交易所止损保护' : 'Exchange Stop Loss Protection'}
                </div>
                <span className={`text-[10px] px-2 py-1 rounded-full ${stopOrders.length > 0 ? 'bg-red-500/15 text-red-300' : 'bg-white/5 text-nofx-text-muted'}`}>
                  {stopOrders.length > 0
                    ? `${stopOrders.length} ${language === 'zh' ? '条委托' : 'orders'}`
                    : language === 'zh' ? '未检测到' : 'not detected'}
                </span>
              </div>
              {stopOrders.length > 0 ? (
                <div className="space-y-2">
                  {stopOrders.map((order) => {
                    const trigger = order.stop_price || order.price
                    return (
                      <div key={`sl-${order.order_id}`} className="rounded-lg border border-red-500/10 bg-black/20 p-3 text-xs">
                        <div className="flex justify-between gap-3">
                          <span className="text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}</span>
                          <span className="font-mono text-red-200">{formatPrice(trigger)}</span>
                        </div>
                        <div className="flex justify-between gap-3 mt-1">
                          <span className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}</span>
                          <span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span>
                        </div>
                        <div className="flex justify-between gap-3 mt-1">
                          <span className="text-nofx-text-muted">Type</span>
                          <span className="font-mono text-nofx-text-main">{order.type}</span>
                        </div>
                      </div>
                    )
                  })}
                </div>
              ) : (
                <div className="text-xs text-nofx-text-muted">
                  {language === 'zh'
                    ? '当前没有识别到与该仓位匹配的止损委托。'
                    : 'No stop-loss order currently matched for this position.'}
                </div>
              )}
            </div>

            <div className="rounded-xl border border-green-500/20 bg-green-500/5 p-4">
              <div className="flex items-center justify-between mb-3">
                <div className="font-semibold text-green-300">
                  {language === 'zh' ? '交易所止盈保护' : 'Exchange Take Profit Protection'}
                </div>
                <span className={`text-[10px] px-2 py-1 rounded-full ${takeProfitOrders.length > 0 ? 'bg-green-500/15 text-green-300' : 'bg-white/5 text-nofx-text-muted'}`}>
                  {takeProfitOrders.length > 0
                    ? `${takeProfitOrders.length} ${language === 'zh' ? '条委托' : 'orders'}`
                    : language === 'zh' ? '未检测到' : 'not detected'}
                </span>
              </div>
              {takeProfitOrders.length > 0 ? (
                <div className="space-y-2">
                  {takeProfitOrders.map((order) => {
                    const trigger = order.stop_price || order.price
                    return (
                      <div key={`tp-${order.order_id}`} className="rounded-lg border border-green-500/10 bg-black/20 p-3 text-xs">
                        <div className="flex justify-between gap-3">
                          <span className="text-nofx-text-muted">{language === 'zh' ? '触发价' : 'Trigger'}</span>
                          <span className="font-mono text-green-200">{formatPrice(trigger)}</span>
                        </div>
                        <div className="flex justify-between gap-3 mt-1">
                          <span className="text-nofx-text-muted">{language === 'zh' ? '数量' : 'Qty'}</span>
                          <span className="font-mono text-nofx-text-main">{formatQuantity(order.quantity)}</span>
                        </div>
                        <div className="flex justify-between gap-3 mt-1">
                          <span className="text-nofx-text-muted">Type</span>
                          <span className="font-mono text-nofx-text-main">{order.type}</span>
                        </div>
                      </div>
                    )
                  })}
                </div>
              ) : (
                <div className="text-xs text-nofx-text-muted">
                  {language === 'zh'
                    ? '当前没有识别到与该仓位匹配的止盈委托。'
                    : 'No take-profit order currently matched for this position.'}
                </div>
              )}
            </div>
          </div>

          {hasRuntimeConfigHint && (
            <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-nofx-text-muted leading-6">
              <div className="font-semibold text-nofx-text-main mb-1">
                {language === 'zh' ? '运行态保护说明' : 'Runtime Protection Notes'}
              </div>
              <ul className="list-disc pl-5 space-y-1">
                <li>
                  {language === 'zh'
                    ? 'Drawdown Take Profit 与 Break-even Stop 属于本地监控型保护，不一定会在交易所形成独立常驻委托。'
                    : 'Drawdown Take Profit and Break-even Stop are runtime protections and may not always appear as persistent exchange orders.'}
                </li>
                <li>
                  {language === 'zh'
                    ? '本面板优先反映交易所当前可见的止盈/止损委托；运行态保护的配置与执行上下文请结合 Recent Decisions 中的 protection snapshot 一起复核。'
                    : 'This panel reflects currently visible exchange TP/SL orders. Review runtime protection config/execution together with the protection snapshot in Recent Decisions.'}
                </li>
              </ul>
            </div>
          )}

          {loading && <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '正在刷新保护状态…' : 'Refreshing protection status…'}</div>}
          {error && <div className="text-xs text-nofx-red">{error}</div>}
        </div>
      )}
    </div>
  )
}
