import { useState, useEffect, useMemo } from 'react'
import { api } from '../../lib/api'
import { useLanguage } from '../../contexts/LanguageContext'
import { t, type Language } from '../../i18n/translations'
import { MetricTooltip } from '../common/MetricTooltip'
import { formatPrice, formatQuantity } from '../../utils/format'
import type {
  HistoricalPosition,
  TraderStats,
  SymbolStats,
  DirectionStats,
  DecisionAction,
  DecisionReviewRef,
  DecisionActionReviewContext,
} from '../../types'

interface PositionHistoryProps {
  traderId: string
  onSymbolClick?: (symbol: string) => void
}

// Format number with proper decimals (for large numbers)
function formatNumber(value: number, decimals: number = 2): string {
  if (Math.abs(value) >= 1000000) {
    return (value / 1000000).toFixed(2) + 'M'
  }
  if (Math.abs(value) >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  }
  return value.toFixed(decimals)
}

// Format duration from minutes
function formatDuration(minutes: number): string {
  if (!minutes || minutes <= 0) return '-'
  if (minutes < 60) return `${minutes.toFixed(0)}m`
  if (minutes < 1440) return `${(minutes / 60).toFixed(1)}h`
  return `${(minutes / 1440).toFixed(1)}d`
}

// Format date
function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return '-'
  return date.toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function findPrimaryOpenDecision(review?: DecisionReviewRef): DecisionAction | undefined {
  return (review?.decisions || []).find((decision) => {
    const action = String(decision.action || '').toLowerCase()
    return action === 'open_long' || action === 'open_short'
  })
}

function formatCompactRr(value?: number | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—'
  return `${value.toFixed(value >= 10 ? 1 : 2)}R`
}

function formatCompactLevelList(levels?: number[]): string[] {
  return (levels || [])
    .filter((value) => typeof value === 'number' && Number.isFinite(value))
    .slice(0, 3)
    .map((value) => formatPrice(value))
}

export function getDecisionAuditSnapshot(review?: DecisionReviewRef) {
  const decision = findPrimaryOpenDecision(review)
  const ctx = decision?.review_context
  const rr = ctx?.risk_reward
  const control = ctx?.control
  const executionConstraintItems = formatExecutionConstraintItems(ctx?.execution_constraints)
  const actionAudit = formatActionAudit(control)
  const normalizedDecision = String(control?.decision || '').toLowerCase()
  const controlStatus = normalizedDecision
    ? {
        label: formatControlDecisionLabel(normalizedDecision),
        tone: normalizedDecision === 'rejected' ? 'danger' : normalizedDecision === 'overridden' || isDowngradedDecision(normalizedDecision) ? 'warn' : 'neutral' as const,
      }
    : null
  const controlBadges = [
    control?.effective_rr && Number.isFinite(control.effective_rr)
      ? { label: `eff ${formatCompactRr(control.effective_rr)} · ${formatControlRrSource(control.effective_rr_source)}` }
      : null,
    control?.constraints_merged ? { label: 'constraints merged', tone: 'warn' as const } : null,
    control?.runtime_rr_recomputed ? { label: 'runtime RR', tone: 'warn' as const } : null,
    control?.no_order_placed ? { label: 'no order placed', tone: 'danger' as const } : null,
  ].filter(Boolean) as { label: string; tone?: 'warn' | 'danger' }[]

  return {
    decision,
    ctx,
    rr,
    control,
    actionAudit,
    controlStatus,
    controlBadges,
    failedChecks: (control?.failed_checks || []).map((check) => formatControlCheck(check)).slice(0, 4),
    support: formatCompactLevelList(ctx?.key_levels?.support),
    resistance: formatCompactLevelList(ctx?.key_levels?.resistance),
    anchors: ctx?.anchors || [],
    executionConstraintItems,
  }
}

function formatOptionalNumber(value?: number | null, maxDecimals = 8): string | undefined {
  if (value === undefined || value === null || !Number.isFinite(value) || value <= 0) return undefined
  return new Intl.NumberFormat('en-US', {
    maximumFractionDigits: maxDecimals,
    minimumFractionDigits: 0,
  }).format(value)
}

function formatFeeRate(value?: number | null): string | undefined {
  if (value === undefined || value === null || !Number.isFinite(value) || value <= 0) return undefined
  return `${(value * 100).toFixed(3)}%`
}

function formatProtectionPolicyReason(reason?: string): string {
  switch (String(reason || '').toLowerCase()) {
    case 'stop_inside_invalidation':
      return 'stop > invalidation'
    case 'target_before_first_target':
      return 'target < 1st target'
    case 'break_even_after_target':
      return 'BE after target'
    case 'fallback_inside_invalidation':
      return 'fallback > invalidation'
    default:
      return reason || 'policy mismatch'
  }
}

function formatProtectionPolicyStatus(protection?: DecisionActionReviewContext['protection']): {
  label: string
  tone: 'neutral' | 'warn' | 'danger'
} | null {
  if (!protection?.policy_status) return null
  switch (protection.policy_status) {
    case 'aligned':
      return { label: 'policy aligned', tone: 'neutral' }
    case 'recomputed':
      return { label: 'policy recomputed', tone: 'warn' }
    case 'rejected':
      return { label: 'policy rejected', tone: 'danger' }
    default:
      return {
        label: `policy ${protection.policy_status}`,
        tone: protection.policy_rejected ? 'danger' : protection.policy_override ? 'warn' : 'neutral',
      }
  }
}

function formatExecutionConstraintItems(constraints?: DecisionActionReviewContext['execution_constraints']): { label: string; tone?: 'cost' }[] {
  if (!constraints) return []
  const items: { label: string; tone?: 'cost' }[] = []
  const pushNumber = (prefix: string, value?: number | null, maxDecimals = 8) => {
    const formatted = formatOptionalNumber(value, maxDecimals)
    if (formatted) items.push({ label: `${prefix} ${formatted}` })
  }

  pushNumber('tick', constraints.tick_size)
  pushNumber('qty', constraints.qty_step_size)
  pushNumber('min', constraints.min_qty)
  pushNumber('ctVal', constraints.contract_value)
  pushNumber('last', constraints.last_price, 4)

  const fee = formatFeeRate(constraints.taker_fee_rate ?? constraints.maker_fee_rate)
  if (fee) items.push({ label: `fee ${fee}`, tone: 'cost' })
  const slippage = formatOptionalNumber(constraints.estimated_slippage_bps, 2)
  if (slippage) items.push({ label: `slip ${slippage}bps`, tone: 'cost' })

  return items
}

function formatControlDecisionLabel(decision?: string): string {
  switch (String(decision || '').toLowerCase()) {
    case 'accepted':
      return 'accepted'
    case 'rejected':
      return 'rejected'
    case 'downgraded':
    case 'downgraded_to_wait':
      return 'downgraded to wait'
    case 'overridden':
      return 'overridden'
    default:
      return decision || 'control'
  }
}

function isDowngradedDecision(decision?: string): boolean {
  const normalized = String(decision || '').toLowerCase()
  return normalized === 'downgraded' || normalized === 'downgraded_to_wait'
}

function formatControlRrSource(source?: string): string {
  switch (String(source || '').toLowerCase()) {
    case 'net':
      return 'net'
    case 'gross':
      return 'gross'
    case 'execution_recomputed_net':
      return 'runtime net'
    case 'execution_recomputed_gross':
      return 'runtime gross'
    default:
      return source || 'system'
  }
}

function formatControlCheck(check?: string): string {
  switch (String(check || '').toLowerCase()) {
    case 'runtime_rr_below_min':
      return 'runtime RR below min'
    case 'protection_alignment_mismatch':
      return 'protection alignment mismatch'
    case 'stop_inside_invalidation':
      return 'stop above invalidation'
    case 'target_before_first_target':
      return 'target before first target'
    case 'break_even_after_target':
      return 'break-even after target'
    case 'fallback_inside_invalidation':
      return 'fallback above invalidation'
    default:
      return String(check || 'check_failed').replace(/_/g, ' ')
  }
}

function formatActionLabel(action?: string): string {
  switch (String(action || '').toLowerCase()) {
    case 'wait':
      return 'wait'
    case 'open_long':
      return 'open long'
    case 'open_short':
      return 'open short'
    case 'close_long':
      return 'close long'
    case 'close_short':
      return 'close short'
    default:
      return action || 'action'
  }
}

function formatActionAudit(control?: DecisionActionReviewContext['control']): string | null {
  if (!control) return null
  const original = String(control.original_action || '').trim()
  const final = String(control.final_action || '').trim()
  if (!original && !final) return null
  if (original && final && original !== final) {
    return `${formatActionLabel(original)} → ${formatActionLabel(final)}`
  }
  return formatActionLabel(final || original)
}

function DecisionAuditPanel({ review }: { review?: DecisionReviewRef }) {
  const audit = getDecisionAuditSnapshot(review)
  const protection = audit.ctx?.protection
  const policyStatus = formatProtectionPolicyStatus(protection)
  const policyReasons = protection?.policy_reasons || []
  if (
    !audit.ctx &&
    !audit.rr &&
    audit.support.length === 0 &&
    audit.resistance.length === 0 &&
    audit.executionConstraintItems.length === 0 &&
    !policyStatus &&
    !audit.controlStatus &&
    !audit.actionAudit &&
    audit.controlBadges.length === 0 &&
    audit.failedChecks.length === 0
  ) {
    return null
  }

  const pass = audit.rr?.passed
  const passCls = pass
    ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20'
    : 'bg-rose-500/10 text-rose-300 border-rose-500/20'

  return (
    <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[11px] text-nofx-text-muted">Entry audit</span>
        <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-medium ${passCls}`}>
          RR {formatCompactRr(audit.rr?.net_estimated_rr ?? audit.rr?.gross_estimated_rr)}
        </span>
        {audit.controlStatus ? (
          <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${
            audit.controlStatus.tone === 'danger'
              ? 'border-rose-500/20 bg-rose-500/10 text-rose-200'
              : audit.controlStatus.tone === 'warn'
                ? 'border-amber-500/20 bg-amber-500/10 text-amber-200'
                : 'border-cyan-500/20 bg-cyan-500/10 text-cyan-200'
          }`}>
            {audit.controlStatus.label}
          </span>
        ) : null}
        {policyStatus ? (
          <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] ${
            policyStatus.tone === 'danger'
              ? 'border-rose-500/20 bg-rose-500/10 text-rose-200'
              : policyStatus.tone === 'warn'
                ? 'border-amber-500/20 bg-amber-500/10 text-amber-200'
                : 'border-emerald-500/20 bg-emerald-500/10 text-emerald-200'
          }`}>
            {policyStatus.label}
          </span>
        ) : null}
        {audit.ctx?.min_risk_reward ? (
          <span className="inline-flex items-center rounded-full border border-white/10 px-2 py-0.5 text-[10px] text-nofx-text-muted">
            min {formatCompactRr(audit.ctx.min_risk_reward)}
          </span>
        ) : null}
        {typeof pass === 'boolean' ? (
          <span className="inline-flex items-center rounded-full border border-white/10 px-2 py-0.5 text-[10px] text-nofx-text-main">
            {pass ? 'pass' : 'fail'}
          </span>
        ) : null}
        {audit.ctx?.primary_timeframe ? (
          <span className="inline-flex items-center rounded-full border border-cyan-500/20 bg-cyan-500/10 px-2 py-0.5 text-[10px] text-cyan-200">
            {audit.ctx.primary_timeframe}
          </span>
        ) : null}
      </div>

      {audit.controlBadges.length > 0 && (
        <div className="flex flex-wrap gap-1.5 text-[10px]">
          {audit.controlBadges.map((item, idx) => (
            <span
              key={`control-badge-${idx}`}
              className={`inline-flex items-center rounded-full border px-2 py-0.5 ${
                item.tone === 'danger'
                  ? 'border-rose-500/20 bg-rose-500/10 text-rose-200'
                  : item.tone === 'warn'
                    ? 'border-amber-500/20 bg-amber-500/10 text-amber-200'
                    : 'border-sky-500/20 bg-sky-500/10 text-sky-200'
              }`}
            >
              {item.label}
            </span>
          ))}
        </div>
      )}

      {audit.actionAudit ? (
        <div className="text-[10px] text-nofx-text-muted">
          action {audit.actionAudit}
        </div>
      ) : null}

      {audit.failedChecks.length > 0 && (
        <div className="flex flex-wrap gap-1.5 text-[10px]">
          {audit.failedChecks.map((check, idx) => (
            <span key={`failed-${idx}`} className="inline-flex items-center rounded-full border border-rose-500/20 bg-rose-500/10 px-2 py-0.5 text-rose-200">
              failed · {check}
            </span>
          ))}
        </div>
      )}

      {(audit.support.length > 0 || audit.resistance.length > 0) && (
        <div className="flex flex-wrap gap-1.5 text-[10px]">
          {audit.support.map((value, idx) => (
            <span key={`s-${idx}`} className="inline-flex items-center rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-0.5 text-emerald-200">
              S {value}
            </span>
          ))}
          {audit.resistance.map((value, idx) => (
            <span key={`r-${idx}`} className="inline-flex items-center rounded-full border border-rose-500/20 bg-rose-500/10 px-2 py-0.5 text-rose-200">
              R {value}
            </span>
          ))}
        </div>
      )}

      {policyReasons.length > 0 && (
        <div className="flex flex-wrap gap-1.5 text-[10px]">
          {policyReasons.map((reason, idx) => (
            <span key={`policy-${idx}`} className="inline-flex items-center rounded-full border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-amber-200">
              {formatProtectionPolicyReason(reason)}
            </span>
          ))}
        </div>
      )}

      {audit.executionConstraintItems.length > 0 && (
        <div className="flex flex-wrap gap-1.5 text-[10px]">
          {audit.executionConstraintItems.map((item, idx) => (
            <span
              key={`exec-${idx}`}
              className={`inline-flex items-center rounded-full border px-2 py-0.5 ${
                item.tone === 'cost'
                  ? 'border-amber-500/20 bg-amber-500/10 text-amber-200'
                  : 'border-slate-500/20 bg-slate-500/10 text-slate-200'
              }`}
            >
              {item.label}
            </span>
          ))}
        </div>
      )}

      {audit.anchors.length > 0 && (
        <details className="text-[11px] text-nofx-text-muted">
          <summary className="cursor-pointer select-none">anchors ({audit.anchors.length})</summary>
          <div className="mt-2 space-y-1">
            {audit.anchors.slice(0, 5).map((anchor, idx) => (
              <div key={idx} className="rounded border border-white/10 bg-white/5 px-2 py-1.5">
                <span className="text-nofx-text-main">{anchor.type || 'anchor'}</span>
                {anchor.timeframe ? <span>{` · ${anchor.timeframe}`}</span> : null}
                {anchor.price ? <span>{` · ${formatPrice(anchor.price)}`}</span> : null}
                {anchor.reason ? <span>{` · ${anchor.reason}`}</span> : null}
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  )
}

function formatReviewContextSummary(reviewContext?: Record<string, unknown>): string {
  if (!reviewContext) return '—'

  const safeMode = reviewContext.safe_mode
  const safeModeReason = reviewContext.safe_mode_reason
  const aiDecisionMode = reviewContext.ai_decision_mode
  const candidateCount = reviewContext.candidate_count
  const positionCount = reviewContext.position_count
  const marginUsedPct = reviewContext.margin_used_pct

  const parts: string[] = []
  if (typeof aiDecisionMode === 'string' && aiDecisionMode) parts.push(`mode=${aiDecisionMode}`)
  if (typeof candidateCount === 'number') parts.push(`candidates=${candidateCount}`)
  if (typeof positionCount === 'number') parts.push(`positions=${positionCount}`)
  if (typeof marginUsedPct === 'number') parts.push(`margin=${marginUsedPct.toFixed(1)}%`)
  if (typeof safeMode === 'boolean') parts.push(`safe=${safeMode ? 'on' : 'off'}`)
  if (typeof safeModeReason === 'string' && safeModeReason) parts.push(`reason=${safeModeReason}`)

  return parts.length > 0 ? parts.join(' | ') : '—'
}

function formatProtectionSourceLabel(source?: string): string {
  switch (String(source || '').toLowerCase()) {
    case 'ai_decision':
      return 'AI'
    case 'strategy':
      return 'Strategy'
    case 'none':
      return 'None'
    default:
      return source || '—'
  }
}

function formatProtectionBadge(sourceLabel: string, modeLabel?: string, kind?: 'full' | 'ladder' | 'drawdown' | 'break_even') {
  const colorMap = {
    full: { bg: 'rgba(14, 203, 129, 0.10)', border: '1px solid rgba(14, 203, 129, 0.22)', color: '#8CF4C4' },
    ladder: { bg: 'rgba(59, 130, 246, 0.10)', border: '1px solid rgba(59, 130, 246, 0.22)', color: '#93C5FD' },
    drawdown: { bg: 'rgba(168, 85, 247, 0.10)', border: '1px solid rgba(168, 85, 247, 0.22)', color: '#D8B4FE' },
    break_even: { bg: 'rgba(249, 115, 22, 0.10)', border: '1px solid rgba(249, 115, 22, 0.22)', color: '#FDBA74' },
  } as const
  return {
    label: modeLabel ? `${sourceLabel} · ${modeLabel}` : sourceLabel,
    style: colorMap[kind || 'full'],
  }
}

function formatProtectionSummary(snapshot?: HistoricalPosition['protection_snapshot']): { label: string; style: React.CSSProperties }[] {
  if (!snapshot) return []
  const parts: { label: string; style: React.CSSProperties }[] = []
  if (snapshot.full_tp_sl?.enabled) {
    parts.push(formatProtectionBadge('Full', snapshot.full_tp_sl.mode || 'manual', 'full'))
  }
  if (snapshot.ladder_tp_sl?.enabled) {
    parts.push(formatProtectionBadge('Ladder', snapshot.ladder_tp_sl.mode || 'manual', 'ladder'))
  }
  if (snapshot.drawdown && snapshot.drawdown.length > 0) {
    const first = snapshot.drawdown[0]
    parts.push(formatProtectionBadge('Drawdown', `${first.mode || 'manual'} · ${formatProtectionSourceLabel(first.source)}`, 'drawdown'))
  }
  if (snapshot.break_even?.enabled) {
    parts.push(formatProtectionBadge('Break-even', formatProtectionSourceLabel(snapshot.break_even.source), 'break_even'))
  }
  return parts
}

// Stats Card Component with formula tooltip
function StatCard({
  title,
  value,
  suffix,
  color,
  icon,
  subtitle,
  metricKey,
  language = 'en',
}: {
  title: string
  value: string | number
  suffix?: string
  color?: string
  icon: string
  subtitle?: string
  metricKey?: string
  language?: string
}) {
  return (
    <div
      className="rounded-lg p-4 transition-all duration-200 hover:scale-[1.02]"
      style={{
        background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        border: '1px solid #2B3139',
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
      }}
    >
      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">{icon}</span>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {title}
        </span>
        {metricKey && (
          <MetricTooltip metricKey={metricKey} language={language} size={12} />
        )}
      </div>
      <div className="flex items-baseline gap-1">
        <span
          className="text-xl font-bold font-mono"
          style={{ color: color || '#EAECEF' }}
        >
          {value}
        </span>
        {suffix && (
          <span className="text-sm" style={{ color: '#848E9C' }}>
            {suffix}
          </span>
        )}
      </div>
      {subtitle && (
        <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
          {subtitle}
        </div>
      )}
    </div>
  )
}

// Symbol Stats Row
function SymbolStatsRow({ stat, onSymbolClick }: { stat: SymbolStats; onSymbolClick?: (symbol: string) => void }) {
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'
  const winRateColor =
    winRate >= 60 ? '#0ECB81' : winRate >= 40 ? '#F0B90B' : '#F6465D'

  return (
    <div
      className="flex items-center justify-between p-3 rounded-lg transition-all duration-200 hover:bg-white/5"
      style={{ borderBottom: '1px solid #2B3139' }}
    >
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => onSymbolClick?.(stat.symbol)}
          className="font-mono font-semibold hover:text-cyan-300 transition-colors"
          style={{ color: '#EAECEF' }}
        >
          {(stat.symbol || '').replace('USDT', '')}
        </button>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {stat.total_trades || 0} trades
        </span>
      </div>
      <div className="flex items-center gap-6">
        <div className="text-right">
          <div className="text-xs" style={{ color: '#848E9C' }}>
            Win Rate
          </div>
          <div className="font-mono font-semibold" style={{ color: winRateColor }}>
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div className="text-right min-w-[80px]">
          <div className="text-xs" style={{ color: '#848E9C' }}>
            P&L
          </div>
          <div className="font-mono font-semibold" style={{ color: pnlColor }}>
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
      </div>
    </div>
  )
}

// Direction Stats Card
function DirectionStatsCard({ stat, language }: { stat: DirectionStats; language: Language }) {
  const isLong = (stat.side || '').toLowerCase() === 'long'
  const iconColor = isLong ? '#0ECB81' : '#F6465D'
  const totalPnl = stat.total_pnl || 0
  const winRate = stat.win_rate || 0
  const tradeCount = stat.trade_count || 0
  const avgPnl = stat.avg_pnl || 0
  const pnlColor = totalPnl >= 0 ? '#0ECB81' : '#F6465D'

  return (
    <div
      className="rounded-lg p-4"
      style={{
        background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        border: `1px solid ${iconColor}33`,
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <span className="text-xl">{isLong ? '📈' : '📉'}</span>
        <span
          className="font-bold uppercase"
          style={{ color: iconColor }}
        >
          {stat.side || 'Unknown'}
        </span>
      </div>
      <div className="grid grid-cols-4 gap-4">
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.trades', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
            {tradeCount}
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.winRate', language)}
          </div>
          <div
            className="font-mono font-semibold"
            style={{
              color:
                winRate >= 60
                  ? '#0ECB81'
                  : winRate >= 40
                    ? '#F0B90B'
                    : '#F6465D',
            }}
          >
            {winRate.toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.totalPnL', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: pnlColor }}>
            {totalPnl >= 0 ? '+' : ''}
            {formatNumber(totalPnl)}
          </div>
        </div>
        <div>
          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
            {t('positionHistory.avgPnL', language)}
          </div>
          <div className="font-mono font-semibold" style={{ color: avgPnl >= 0 ? '#0ECB81' : '#F6465D' }}>
            {avgPnl >= 0 ? '+' : ''}
            {formatNumber(avgPnl)}
          </div>
        </div>
      </div>
    </div>
  )
}

// Position Row Component
function PositionRow({ position, onSymbolClick }: { position: HistoricalPosition; onSymbolClick?: (symbol: string) => void }) {
  const [expanded, setExpanded] = useState(false)
  const side = position.side || ''
  const isLong = side.toUpperCase() === 'LONG'
  const realizedPnl = position.realized_pnl || 0
  const isProfitable = realizedPnl >= 0
  const sideColor = isLong ? '#0ECB81' : '#F6465D'
  const pnlColor = isProfitable ? '#0ECB81' : '#F6465D'

  const formatExecutionSourceLabel = (value: string): string => {
    const v = String(value || '').toLowerCase()
    if (v === 'ai_close_long' || v === 'ai_close_short') return v
    if (v === 'managed_drawdown') return 'Managed Drawdown'
    if (v === 'emergency_protection_close') return 'Emergency Protection Close'
    if (v === 'ladder_tp') return 'Ladder TP'
    if (v === 'ladder_sl') return 'Ladder SL'
    if (v === 'full_tp') return 'Full TP'
    if (v === 'full_sl') return 'Full SL'
    if (v === 'close_long' || v === 'close_short') return `AI ${v}`
    if (v.includes('native_trailing') || v.includes('trailing')) return 'Native Trailing'
    if (v.includes('break_even')) return 'Break-even Stop'
    if (v.includes('take_profit') || v === 'tp') return 'Take Profit'
    if (v.includes('stop_loss') || v === 'sl') return 'Stop Loss'
    if (v.includes('manual')) return 'Manual'
    if (v === 'sync') return 'Exchange Sync'
    if (v === 'unknown' || v === '') return 'Unknown'
    return value
  }

  const getExecutionSourceBadgeStyle = (value: string) => {
    const v = String(value || '').toLowerCase()
    if (v.includes('ai_close')) return { background: 'rgba(96,165,250,0.14)', color: '#60A5FA', border: '1px solid rgba(96,165,250,0.3)' }
    if (v.includes('native_trailing')) return { background: 'rgba(168,85,247,0.14)', color: '#C084FC', border: '1px solid rgba(168,85,247,0.3)' }
    if (v.includes('break_even')) return { background: 'rgba(251,191,36,0.14)', color: '#F0B90B', border: '1px solid rgba(251,191,36,0.3)' }
    if (v === 'ladder_tp' || v === 'full_tp' || v.includes('take_profit')) return { background: 'rgba(14,203,129,0.14)', color: '#0ECB81', border: '1px solid rgba(14,203,129,0.3)' }
    if (v === 'ladder_sl' || v === 'full_sl' || v.includes('stop_loss') || v === 'managed_drawdown' || v === 'emergency_protection_close') return { background: 'rgba(246,70,93,0.14)', color: '#F6465D', border: '1px solid rgba(246,70,93,0.3)' }
    return { background: 'rgba(132,142,156,0.14)', color: '#AAB2BD', border: '1px solid rgba(132,142,156,0.25)' }
  }


  // Calculate holding time
  const entryTime = position.entry_time ? new Date(position.entry_time).getTime() : 0
  const exitTime = position.exit_time ? new Date(position.exit_time).getTime() : 0
  const holdingMinutes = entryTime && exitTime && exitTime > entryTime ? (exitTime - entryTime) / 60000 : 0

  // Calculate PnL percentage based on entry price
  const entryPrice = position.entry_price || 0
  const exitPrice = position.exit_price || 0
  let pnlPct = 0
  if (entryPrice > 0) {
    if (isLong) {
      pnlPct = ((exitPrice - entryPrice) / entryPrice) * 100
    } else {
      pnlPct = ((entryPrice - exitPrice) / entryPrice) * 100
    }
  }

  // Use entry_quantity for display (original position size)
  const displayQty = position.entry_quantity || position.quantity || 0

  const closeRatioPct = position.close_ratio_pct || 0
  const closeValueUsdt = position.close_value_usdt || (exitPrice * displayQty)
  const executionSource = formatExecutionSourceLabel(position.execution_source || position.close_reason || 'unknown')
  const executionOrderType = position.execution_order_type || 'unknown'

  return (
    <>
    <tr
      className="transition-all duration-200 hover:bg-white/5 cursor-pointer"
      style={{ borderBottom: expanded ? 'none' : '1px solid #2B3139' }}
      onClick={() => setExpanded((v) => !v)}
    >
      {/* Symbol */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onSymbolClick?.(position.symbol) }}
            className="font-mono font-semibold hover:text-cyan-300 transition-colors"
            style={{ color: '#EAECEF' }}
          >
            {(position.symbol || '').replace('USDT', '')}
          </button>
          <span
            className="px-2 py-0.5 rounded text-xs font-semibold uppercase"
            style={{
              background: `${sideColor}22`,
              color: sideColor,
              border: `1px solid ${sideColor}44`,
            }}
          >
            {side}
          </span>
        </div>
      </td>

      {/* Entry Price */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatPrice(entryPrice)}
      </td>

      {/* Exit Price */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatPrice(exitPrice)}
      </td>

      {/* Quantity */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#848E9C' }}>
        {formatQuantity(displayQty)}
      </td>

      {/* Position Value (Entry Price * Quantity) */}
      <td className="py-3 px-4 text-right font-mono" style={{ color: '#EAECEF' }}>
        {formatNumber(entryPrice * displayQty)}
      </td>

      {/* P&L */}
      <td className="py-3 px-4 text-right">
        <div className="font-mono font-semibold" style={{ color: pnlColor }}>
          {isProfitable ? '+' : ''}
          {formatNumber(realizedPnl)}
        </div>
        <div className="text-xs" style={{ color: pnlColor }}>
          {pnlPct >= 0 ? '+' : ''}
          {pnlPct.toFixed(2)}%
        </div>
      </td>

      {/* Fee - show more precision for small fees */}
      <td className="py-3 px-4 text-right font-mono text-xs" style={{ color: '#848E9C' }}>
        -{((position.fee || 0) < 0.01 && (position.fee || 0) > 0)
          ? (position.fee || 0).toFixed(4)
          : (position.fee || 0).toFixed(2)}
      </td>

      {/* Duration */}
      <td className="py-3 px-4 text-center text-sm" style={{ color: '#848E9C' }}>
        {formatDuration(holdingMinutes)}
      </td>

      {/* Exit Time */}
      <td className="py-3 px-4 text-right text-xs" style={{ color: '#848E9C' }}>
        {formatDate(position.exit_time)}
      </td>
    </tr>
    {expanded && (
      <tr style={{ borderBottom: '1px solid #2B3139', background: 'rgba(255,255,255,0.02)' }}>
        <td colSpan={9} className="px-4 pb-4 pt-0">
          <div className="rounded-lg border border-white/10 bg-black/20 p-4 mt-2 space-y-3">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3 text-xs">
              <div>
                <div style={{ color: '#848E9C' }}>{'委托来源 / Source'}</div>
                <div className="px-2 py-1 rounded text-[11px] font-semibold inline-flex" style={getExecutionSourceBadgeStyle(position.execution_source || position.close_reason || 'unknown')}>
                  {executionSource}
                </div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {formatProtectionSummary(position.protection_snapshot).map((item, idx) => (
                    <span key={idx} className="px-2 py-1 rounded text-[11px] font-medium" style={item.style}>
                      {item.label}
                    </span>
                  ))}
                </div>
              </div>
              <div>
                <div style={{ color: '#848E9C' }}>{'委托类型 / Order Type'}</div>
                <div className="font-mono" style={{ color: '#EAECEF' }}>{executionOrderType}</div>
              </div>
              <div>
                <div style={{ color: '#848E9C' }}>{'入场 / 出场决策周期'}</div>
                <div className="font-mono" style={{ color: '#EAECEF' }}>
                  {`${position.entry_decision_cycle || '—'} / ${position.exit_decision_cycle || '—'}`}
                </div>
              </div>
              <div>
                <div style={{ color: '#848E9C' }}>{'复盘上下文 / Review Context'}</div>
                <div className="text-[11px] leading-5" style={{ color: '#EAECEF' }}>
                  {formatReviewContextSummary(position.exit_decision_review?.review_context || position.entry_decision_review?.review_context)}
                </div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {formatProtectionSummary(position.exit_decision_review?.protection_snapshot || position.entry_decision_review?.protection_snapshot).map((item, idx) => (
                    <span key={idx} className="px-2 py-1 rounded text-[11px] font-medium" style={item.style}>
                      {item.label}
                    </span>
                  ))}
                </div>
                <div className="mt-2">
                  <DecisionAuditPanel review={position.entry_decision_review || position.exit_decision_review} />
                </div>
              </div>
              <div>
                <div style={{ color: '#848E9C' }}>{'成交比例 / Close Ratio'}</div>
                <div className="font-mono" style={{ color: '#EAECEF' }}>{closeRatioPct > 0 ? `${closeRatioPct.toFixed(2)}%` : '—'}</div>
              </div>
              <div>
                <div style={{ color: '#848E9C' }}>{'成交价值 / Value USDT'}</div>
                <div className="font-mono" style={{ color: '#EAECEF' }}>{formatNumber(closeValueUsdt)}</div>
              </div>
            </div>

            {position.close_events && position.close_events.length > 0 && (
              <div>
                <div className="text-xs mb-2" style={{ color: '#848E9C' }}>{'分段平仓事件 / Close Event Flow'}</div>
                <div className="space-y-2">
                  {position.close_events.map((event) => (
                    <div key={event.id} className="rounded-lg border border-white/10 bg-white/5 p-3 grid grid-cols-1 md:grid-cols-6 gap-3 text-xs">
                      <div>
                        <div style={{ color: '#848E9C' }}>{'原因 / Reason'}</div>
                        <div className="px-2 py-1 rounded text-[11px] font-semibold inline-flex" style={getExecutionSourceBadgeStyle(event.execution_source || event.close_reason)}>{formatExecutionSourceLabel(event.execution_source || event.close_reason)}</div>
                        {event.protection_status ? (
                          <div className="mt-2">
                            <span className="px-2 py-1 rounded text-[11px] font-medium" style={{ background: 'rgba(255,255,255,0.06)', color: '#C9D1D9', border: '1px solid rgba(255,255,255,0.08)' }}>
                              {`Protection: ${event.protection_status}`}
                            </span>
                          </div>
                        ) : null}
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'类型 / Type'}</div>
                        <div className="font-mono" style={{ color: '#EAECEF' }}>{event.execution_type || 'unknown'}</div>
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'数量 / Ratio'}</div>
                        <div className="font-mono" style={{ color: '#EAECEF' }}>{`${formatQuantity(event.close_quantity)} / ${event.close_ratio_pct.toFixed(2)}%`}</div>
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'价格 / Value'}</div>
                        <div className="font-mono" style={{ color: '#EAECEF' }}>{`${formatPrice(event.execution_price)} / ${formatNumber(event.close_value_usdt)}`}</div>
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'决策周期 / Cycle'}</div>
                        <div className="font-mono" style={{ color: '#EAECEF' }}>{event.decision_cycle || '—'}</div>
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'复盘上下文 / Review'}</div>
                        <div className="text-[11px] leading-5" style={{ color: '#EAECEF' }}>
                          {formatReviewContextSummary(event.decision_review?.review_context)}
                        </div>
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          {formatProtectionSummary(event.decision_review?.protection_snapshot).map((item, idx) => (
                            <span key={idx} className="px-2 py-1 rounded text-[11px] font-medium" style={item.style}>
                              {item.label}
                            </span>
                          ))}
                        </div>
                        <div className="mt-2">
                          <DecisionAuditPanel review={event.decision_review} />
                        </div>
                      </div>
                      <div>
                        <div style={{ color: '#848E9C' }}>{'PnL / Time'}</div>
                        <div className="font-mono" style={{ color: '#EAECEF' }}>{`${event.realized_pnl_delta >= 0 ? '+' : ''}${formatNumber(event.realized_pnl_delta)} / ${formatDate(event.event_time)}`}</div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </td>
      </tr>
    )}
    </>
  )
}

export function PositionHistory({ traderId, onSymbolClick }: PositionHistoryProps) {
  const { language } = useLanguage()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [positions, setPositions] = useState<HistoricalPosition[]>([])
  const [stats, setStats] = useState<TraderStats | null>(null)
  const [symbolStats, setSymbolStats] = useState<SymbolStats[]>([])
  const [directionStats, setDirectionStats] = useState<DirectionStats[]>([])

  // Pagination state
  const [pageSize, setPageSize] = useState<number>(20)
  const [currentPage, setCurrentPage] = useState<number>(1)

  // Filter state
  const [filterSymbol, setFilterSymbol] = useState<string>('all')
  const [filterSide, setFilterSide] = useState<string>('all')
  const [sortBy, setSortBy] = useState<'time' | 'pnl' | 'pnl_pct'>('time')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true)
        setError(null)
        // Fetch more data than needed to support filtering, but respect pageSize for initial load
        const data = await api.getPositionHistory(traderId, Math.max(200, pageSize * 5))
        setPositions(data.positions || [])
        setStats(data.stats)
        setSymbolStats(data.symbol_stats || [])
        setDirectionStats(data.direction_stats || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load history')
      } finally {
        setLoading(false)
      }
    }

    if (traderId) {
      fetchData()
    }
  }, [traderId, pageSize])

  // Get unique symbols for filter
  const uniqueSymbols = useMemo(() => {
    const symbols = new Set(positions.map((p) => p.symbol))
    return Array.from(symbols).sort()
  }, [positions])

  // Filtered and sorted positions (before pagination)
  const filteredAndSortedPositions = useMemo(() => {
    let result = [...positions]

    // Apply filters
    if (filterSymbol !== 'all') {
      result = result.filter((p) => p.symbol === filterSymbol)
    }
    if (filterSide !== 'all') {
      result = result.filter(
        (p) => (p.side || '').toUpperCase() === filterSide.toUpperCase()
      )
    }

    // Apply sorting
    result.sort((a, b) => {
      let comparison = 0
      switch (sortBy) {
        case 'time':
          comparison =
            new Date(a.exit_time || 0).getTime() - new Date(b.exit_time || 0).getTime()
          break
        case 'pnl':
          comparison = (a.realized_pnl || 0) - (b.realized_pnl || 0)
          break
        case 'pnl_pct': {
          const aPrice = a.entry_price || 1
          const bPrice = b.entry_price || 1
          const aPct = ((a.exit_price || 0) - aPrice) / aPrice * 100
          const bPct = ((b.exit_price || 0) - bPrice) / bPrice * 100
          comparison = aPct - bPct
          break
        }
      }
      return sortOrder === 'desc' ? -comparison : comparison
    })

    return result
  }, [positions, filterSymbol, filterSide, sortBy, sortOrder])

  // Pagination calculations
  const totalFilteredCount = filteredAndSortedPositions.length
  const totalPages = Math.ceil(totalFilteredCount / pageSize)

  // Reset to page 1 when filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [filterSymbol, filterSide, sortBy, sortOrder, pageSize])

  // Paginated positions (for display)
  const paginatedPositions = useMemo(() => {
    const startIndex = (currentPage - 1) * pageSize
    return filteredAndSortedPositions.slice(startIndex, startIndex + pageSize)
  }, [filteredAndSortedPositions, currentPage, pageSize])

  // For backwards compatibility, keep filteredPositions as the paginated result
  const filteredPositions = paginatedPositions

  // Calculate profit/loss ratio (avg win / avg loss)
  const profitLossRatio = useMemo(() => {
    if (!stats) return 0
    const avgWin = stats.avg_win || 0
    const avgLoss = stats.avg_loss || 0
    if (avgLoss === 0) return avgWin > 0 ? Infinity : 0
    return avgWin / avgLoss
  }, [stats])

  if (loading) {
    return (
      <div
        className="flex items-center justify-center p-12"
        style={{ color: '#848E9C' }}
      >
        <div className="animate-spin mr-3">
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24">
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        </div>
        {t('positionHistory.loading', language)}
      </div>
    )
  }

  if (error) {
    return (
      <div
        className="rounded-lg p-6 text-center"
        style={{
          background: 'rgba(246, 70, 93, 0.1)',
          border: '1px solid rgba(246, 70, 93, 0.3)',
          color: '#F6465D',
        }}
      >
        {error}
      </div>
    )
  }

  if (positions.length === 0) {
    return (
      <div
        className="rounded-lg p-12 text-center"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
        }}
      >
        <div className="text-4xl mb-4">📊</div>
        <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
          {t('positionHistory.noHistory', language)}
        </div>
        <div style={{ color: '#848E9C' }}>
          {t('positionHistory.noHistoryDesc', language)}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Overall Stats - Row 1: Core Metrics */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
          <StatCard
            icon="📊"
            title={t('positionHistory.totalTrades', language)}
            value={stats.total_trades || 0}
            subtitle={t('positionHistory.winLoss', language, { win: stats.win_trades || 0, loss: stats.loss_trades || 0 })}
            language={language}
          />
          <StatCard
            icon="🎯"
            title={t('positionHistory.winRate', language)}
            value={(stats.win_rate || 0).toFixed(1)}
            suffix="%"
            color={
              (stats.win_rate || 0) >= 60
                ? '#0ECB81'
                : (stats.win_rate || 0) >= 40
                  ? '#F0B90B'
                  : '#F6465D'
            }
            metricKey="win_rate"
            language={language}
          />
          <StatCard
            icon="💰"
            title={t('positionHistory.totalPnL', language)}
            value={((stats.total_pnl || 0) >= 0 ? '+' : '') + formatNumber(stats.total_pnl || 0)}
            color={(stats.total_pnl || 0) >= 0 ? '#0ECB81' : '#F6465D'}
            subtitle={`${t('positionHistory.fee', language)}: -${formatNumber(stats.total_fee || 0)}`}
            metricKey="total_return"
            language={language}
          />
          <StatCard
            icon="📈"
            title={t('positionHistory.profitFactor', language)}
            value={(stats.profit_factor || 0).toFixed(2)}
            color={(stats.profit_factor || 0) >= 1.5 ? '#0ECB81' : (stats.profit_factor || 0) >= 1 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.profitFactorDesc', language)}
            metricKey="profit_factor"
            language={language}
          />
          <StatCard
            icon="⚖️"
            title={t('positionHistory.plRatio', language)}
            value={profitLossRatio === Infinity ? '∞' : profitLossRatio.toFixed(2)}
            color={profitLossRatio >= 1.5 ? '#0ECB81' : profitLossRatio >= 1 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.plRatioDesc', language)}
            metricKey="expectancy"
            language={language}
          />
        </div>
      )}

      {/* Overall Stats - Row 2: Advanced Metrics */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
          <StatCard
            icon="📉"
            title={t('positionHistory.sharpeRatio', language)}
            value={(stats.sharpe_ratio || 0).toFixed(2)}
            color={(stats.sharpe_ratio || 0) >= 1 ? '#0ECB81' : (stats.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F6465D'}
            subtitle={t('positionHistory.sharpeRatioDesc', language)}
            metricKey="sharpe_ratio"
            language={language}
          />
          <StatCard
            icon="🔻"
            title={t('positionHistory.maxDrawdown', language)}
            value={(stats.max_drawdown_pct || 0).toFixed(1)}
            suffix="%"
            color={(stats.max_drawdown_pct || 0) <= 10 ? '#0ECB81' : (stats.max_drawdown_pct || 0) <= 20 ? '#F0B90B' : '#F6465D'}
            metricKey="max_drawdown"
            language={language}
          />
          <StatCard
            icon="🏆"
            title={t('positionHistory.avgWin', language)}
            value={'+' + formatNumber(stats.avg_win || 0)}
            color="#0ECB81"
            metricKey="avg_trade_pnl"
            language={language}
          />
          <StatCard
            icon="💸"
            title={t('positionHistory.avgLoss', language)}
            value={'-' + formatNumber(stats.avg_loss || 0)}
            color="#F6465D"
            language={language}
          />
          <StatCard
            icon="💵"
            title={t('positionHistory.netPnL', language)}
            value={((stats.total_pnl || 0) - (stats.total_fee || 0) >= 0 ? '+' : '') + formatNumber((stats.total_pnl || 0) - (stats.total_fee || 0))}
            color={(stats.total_pnl || 0) - (stats.total_fee || 0) >= 0 ? '#0ECB81' : '#F6465D'}
            subtitle={t('positionHistory.netPnLDesc', language)}
            language={language}
          />
        </div>
      )}

      {/* Direction Stats */}
      {directionStats.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {directionStats.map((stat) => (
            <DirectionStatsCard key={stat.side} stat={stat} language={language} />
          ))}
        </div>
      )}

      {/* Symbol Performance */}
      {symbolStats.length > 0 && (
        <div
          className="rounded-lg p-4"
          style={{
            background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
            border: '1px solid #2B3139',
          }}
        >
          <div className="flex items-center gap-2 mb-4">
            <span className="text-lg">🏅</span>
            <span className="font-semibold" style={{ color: '#EAECEF' }}>
              {t('positionHistory.symbolPerformance', language)}
            </span>
          </div>
          <div className="space-y-1">
            {symbolStats.slice(0, 10).map((stat) => (
              <SymbolStatsRow key={stat.symbol} stat={stat} onSymbolClick={onSymbolClick} />
            ))}
          </div>
        </div>
      )}

      {/* Position List */}
      <div
        className="rounded-lg overflow-hidden"
        style={{
          background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
          border: '1px solid #2B3139',
        }}
      >
        {/* Filters */}
        <div
          className="flex flex-wrap items-center gap-4 p-4"
          style={{ borderBottom: '1px solid #2B3139' }}
        >
          <div className="flex items-center gap-2">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.symbol', language)}:
            </span>
            <select
              value={filterSymbol}
              onChange={(e) => setFilterSymbol(e.target.value)}
              className="rounded px-3 py-1.5 text-sm"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              <option value="all">{t('positionHistory.allSymbols', language)}</option>
              {uniqueSymbols.map((symbol) => (
                <option key={symbol} value={symbol}>
                  {(symbol || '').replace('USDT', '')}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.side', language)}:
            </span>
            <div className="flex rounded overflow-hidden" style={{ border: '1px solid #2B3139' }}>
              {['all', 'LONG', 'SHORT'].map((side) => (
                <button
                  key={side}
                  onClick={() => setFilterSide(side)}
                  className="px-3 py-1.5 text-sm capitalize transition-colors"
                  style={{
                    background: filterSide === side ? '#2B3139' : 'transparent',
                    color: filterSide === side ? '#EAECEF' : '#848E9C',
                  }}
                >
                  {side === 'all' ? t('positionHistory.all', language) : side}
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-2 ml-auto">
            <span className="text-sm" style={{ color: '#848E9C' }}>
              {t('positionHistory.sort', language)}:
            </span>
            <select
              value={`${sortBy}-${sortOrder}`}
              onChange={(e) => {
                const [by, order] = e.target.value.split('-') as [
                  'time' | 'pnl' | 'pnl_pct',
                  'asc' | 'desc',
                ]
                setSortBy(by)
                setSortOrder(order)
              }}
              className="rounded px-3 py-1.5 text-sm"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              <option value="time-desc">{t('positionHistory.latestFirst', language)}</option>
              <option value="time-asc">{t('positionHistory.oldestFirst', language)}</option>
              <option value="pnl-desc">{t('positionHistory.highestPnL', language)}</option>
              <option value="pnl-asc">{t('positionHistory.lowestPnL', language)}</option>
            </select>
          </div>
        </div>

        {/* Table */}
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr style={{ background: '#0B0E11' }}>
                <th
                  className="py-3 px-4 text-left text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.symbol', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.entry', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.exit', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.qty', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.value', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.pnl', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.fee', language)}
                </th>
                <th
                  className="py-3 px-4 text-center text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.duration', language)}
                </th>
                <th
                  className="py-3 px-4 text-right text-xs font-semibold uppercase tracking-wider"
                  style={{ color: '#848E9C' }}
                >
                  {t('positionHistory.closedAt', language)}
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredPositions.map((position) => (
                <PositionRow key={position.id} position={position} onSymbolClick={onSymbolClick} />
              ))}
            </tbody>
          </table>
        </div>

        {/* Footer with Pagination */}
        <div
          className="flex flex-wrap items-center justify-between gap-4 p-4 text-sm"
          style={{ borderTop: '1px solid #2B3139', color: '#848E9C' }}
        >
          {/* Left: Count info */}
          <div className="flex items-center gap-4">
            <span>
              {t('positionHistory.showingPositions', language, { count: totalFilteredCount, total: positions.length })}
            </span>
            {totalFilteredCount > 0 && (
              <span>
                {t('positionHistory.totalPnL', language)}:{' '}
                <span
                  style={{
                    color:
                      filteredAndSortedPositions.reduce((sum, p) => sum + (p.realized_pnl || 0), 0) >= 0
                        ? '#0ECB81'
                        : '#F6465D',
                  }}
                >
                  {filteredAndSortedPositions.reduce((sum, p) => sum + (p.realized_pnl || 0), 0) >= 0
                    ? '+'
                    : ''}
                  {formatNumber(
                    filteredAndSortedPositions.reduce((sum, p) => sum + (p.realized_pnl || 0), 0)
                  )}
                </span>
              </span>
            )}
          </div>

          {/* Right: Pagination controls */}
          <div className="flex items-center gap-3">
            {/* Page size selector */}
            <div className="flex items-center gap-2">
              <span className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '每页' : 'Per page'}:
              </span>
              <select
                value={pageSize}
                onChange={(e) => setPageSize(Number(e.target.value))}
                className="rounded px-2 py-1 text-sm"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                <option value={20}>20</option>
                <option value={50}>50</option>
                <option value={100}>100</option>
              </select>
            </div>

            {/* Page navigation */}
            {totalPages > 1 && (
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setCurrentPage(1)}
                  disabled={currentPage === 1}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  «
                </button>
                <button
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === 1 ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  ‹
                </button>
                <span className="px-3 text-xs" style={{ color: '#EAECEF' }}>
                  {currentPage} / {totalPages}
                </span>
                <button
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  ›
                </button>
                <button
                  onClick={() => setCurrentPage(totalPages)}
                  disabled={currentPage === totalPages}
                  className="px-2 py-1 rounded text-xs transition-colors disabled:opacity-30"
                  style={{
                    background: currentPage === totalPages ? 'transparent' : '#2B3139',
                    color: '#EAECEF',
                  }}
                >
                  »
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
