import { formatPrice } from '../../utils/format'
import type { DecisionActionReviewContext } from '../../types'

export function formatCompactLevelList(levels?: number[]): string[] {
  return (levels || [])
    .filter((value) => typeof value === 'number' && Number.isFinite(value))
    .slice(0, 3)
    .map((value) => formatPrice(value))
}

export function formatTimeframeTrail(ctx?: DecisionActionReviewContext): string[] {
  const directPrimary = typeof ctx?.primary_timeframe === 'string' ? ctx.primary_timeframe.trim() : ''
  const primary = typeof ctx?.timeframe_context?.primary === 'string' ? ctx.timeframe_context.primary.trim() : directPrimary
  const lower = (ctx?.timeframe_context?.lower || []).filter((value) => typeof value === 'string' && value.trim()).slice(0, 2)
  const higher = (ctx?.timeframe_context?.higher || []).filter((value) => typeof value === 'string' && value.trim()).slice(0, 2)
  const parts: string[] = []
  if (primary) parts.push(`primary ${primary}`)
  if (lower.length > 0) parts.push(`lower ${lower.join(', ')}`)
  if (higher.length > 0) parts.push(`higher ${higher.join(', ')}`)
  return parts
}

export function formatFibSummary(keyLevels?: DecisionActionReviewContext['key_levels']): string[] {
  const fib = keyLevels?.fibonacci
  if (!fib) return []
  const parts: string[] = []
  if (fib.swing_low) parts.push(`low ${formatPrice(fib.swing_low)}`)
  if (fib.swing_high) parts.push(`high ${formatPrice(fib.swing_high)}`)
  const levels = formatCompactLevelList(fib.levels)
  if (levels.length > 0) parts.push(`levels ${levels.join(' / ')}`)
  return parts
}

export function formatRiskRewardLinkage(rr?: DecisionActionReviewContext['risk_reward'], includeGrossNet = false): string[] {
  if (!rr) return []
  const parts: string[] = []
  if (rr.entry) parts.push(`entry ${formatPrice(rr.entry)}`)
  if (rr.invalidation) parts.push(`invalid ${formatPrice(rr.invalidation)}`)
  if (rr.first_target) parts.push(`target ${formatPrice(rr.first_target)}`)
  if (includeGrossNet && rr.gross_estimated_rr) parts.push(`gross ${rr.gross_estimated_rr.toFixed(2)}R`)
  if (includeGrossNet && rr.net_estimated_rr) parts.push(`net ${rr.net_estimated_rr.toFixed(2)}R`)
  if (!includeGrossNet && parts.length < 2) return []
  return parts
}

export function formatAlignmentNotes(ctx?: DecisionActionReviewContext, limit = 3): string[] {
  return (ctx?.alignment_notes || ctx?.protection?.notes || []).filter(Boolean).slice(0, limit)
}
