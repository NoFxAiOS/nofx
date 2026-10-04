import type { VergexWinrateMapData } from './api/data'

// A conservative display warning, not a claim about the upstream SLA. The
// matrix polls every 5 minutes; positions/snapshots older than 15m are not live.
export const WINRATE_FRESHNESS_MS = 15 * 60 * 1000

export function winrateQualityWarnings(
  data?: VergexWinrateMapData,
  now = Date.now()
): string[] {
  if (!data) return []
  const warnings: string[] = []
  if (data.coverage !== 'complete')
    warnings.push(`coverage ${data.coverage ?? 'unknown'}`)
  for (const [label, value] of [
    ['snapshot', data.asOf],
    ['positions', data.positionsAsOf],
  ] as const) {
    const time = value ? Date.parse(value) : NaN
    if (!Number.isFinite(time)) warnings.push(`${label} time unknown`)
    else if (now - time > WINRATE_FRESHNESS_MS)
      warnings.push(`${label} older than 15 minutes`)
    else if (time - now > 60000) warnings.push(`${label} time is in the future`)
  }
  if (data.staleHistoryCount == null) warnings.push('history freshness unknown')
  else if (data.staleHistoryCount > 0)
    warnings.push(`${data.staleHistoryCount} stale histories`)
  return warnings
}
