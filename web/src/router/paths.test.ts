import { describe, expect, it } from 'vitest'
import {
  buildDashboardPath,
  getCurrentPageForPath,
  LEGACY_HASH_ROUTES,
  ROUTES,
} from './paths'

describe('router paths helpers', () => {
  it('maps pathname to current navigation page', () => {
    expect(getCurrentPageForPath(ROUTES.home)).toBeUndefined()
    expect(getCurrentPageForPath(ROUTES.welcome)).toBe('traders')
    expect(getCurrentPageForPath(ROUTES.dashboard)).toBe('trader')
    expect(getCurrentPageForPath(ROUTES.strategyMarket)).toBe('strategy-market')
  })

  it('builds dashboard path with optional trader query', () => {
    expect(buildDashboardPath()).toBe(ROUTES.dashboard)
    expect(buildDashboardPath('alpha-1234')).toBe(
      '/dashboard?trader=alpha-1234'
    )
    expect(buildDashboardPath('alpha beta')).toBe(
      '/dashboard?trader=alpha%20beta'
    )
  })

  it('keeps legacy hash redirects aligned with current routes', () => {
    expect(LEGACY_HASH_ROUTES.trader).toBe(ROUTES.dashboard)
    expect(LEGACY_HASH_ROUTES.details).toBe(ROUTES.dashboard)
    expect(LEGACY_HASH_ROUTES.strategy).toBe(ROUTES.strategy)
  })
})
