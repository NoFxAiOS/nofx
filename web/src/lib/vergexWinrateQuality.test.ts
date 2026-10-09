import { expect, it } from 'vitest'
import {
  winrateQualityWarnings,
  WINRATE_FRESHNESS_MS,
} from './vergexWinrateQuality'

const now = Date.parse('2026-10-04T10:00:00Z')
const fresh = {
  coverage: 'complete',
  asOf: new Date(now).toISOString(),
  positionsAsOf: new Date(now).toISOString(),
  staleHistoryCount: 0,
}
it('accepts fresh complete data and warns about partial, missing, stale and future data', () => {
  expect(winrateQualityWarnings(fresh, now)).toEqual([])
  expect(
    winrateQualityWarnings({ ...fresh, coverage: 'partial' }, now)
  ).toContain('coverage partial')
  expect(winrateQualityWarnings({}, now)).toEqual([
    'coverage unknown',
    'snapshot time unknown',
    'positions time unknown',
    'history freshness unknown',
  ])
  expect(winrateQualityWarnings({ ...fresh, asOf: 'invalid' }, now)).toContain(
    'snapshot time unknown'
  )
  expect(
    winrateQualityWarnings(fresh, now + WINRATE_FRESHNESS_MS + 1)
  ).toContain('positions older than 15 minutes')
  expect(winrateQualityWarnings(fresh, now - 60001)).toContain(
    'snapshot time is in the future'
  )
  expect(
    winrateQualityWarnings({ ...fresh, staleHistoryCount: 7 }, now)
  ).toContain('7 stale histories')
})
