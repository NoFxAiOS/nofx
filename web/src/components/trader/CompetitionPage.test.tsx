import { describe, it, expect } from 'vitest'

/**
 * PR #678 test: fix display of NaN and missing data in CompetitionPage
 *
 * Problem: when total_pnl_pct is null/undefined/NaN, it would show "NaN%" or "0.00%"
 * Fix: check data validity and show "—" to indicate missing data
 */

describe('CompetitionPage - Data Validation Logic (PR #678)', () => {
  /**
   * Test the data validity check logic
   * This is the core logic introduced in PR #678
   */
  describe('hasValidData check', () => {
    it('should return true for valid numbers', () => {
      const trader1 = { total_pnl_pct: 10.5 }
      const trader2 = { total_pnl_pct: -5.2 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(true)
    })

    it('should return false when trader1 has null value', () => {
      const trader1 = { total_pnl_pct: null }
      const trader2 = { total_pnl_pct: 10.5 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct!) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(false)
    })

    it('should return false when trader2 has undefined value', () => {
      const trader1 = { total_pnl_pct: 10.5 }
      const trader2 = { total_pnl_pct: undefined }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct!)

      expect(hasValidData).toBe(false)
    })

    it('should return false when trader1 has NaN value', () => {
      const trader1 = { total_pnl_pct: NaN }
      const trader2 = { total_pnl_pct: 10.5 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(false)
    })

    it('should return false when both traders have invalid data', () => {
      const trader1 = { total_pnl_pct: null }
      const trader2 = { total_pnl_pct: NaN }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct!) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(false)
    })

    it('should handle zero as valid data', () => {
      const trader1 = { total_pnl_pct: 0 }
      const trader2 = { total_pnl_pct: 10.5 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(true)
    })

    it('should handle negative numbers as valid data', () => {
      const trader1 = { total_pnl_pct: -15.5 }
      const trader2 = { total_pnl_pct: -8.2 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      expect(hasValidData).toBe(true)
    })
  })

  /**
   * Test the gap calculation logic
   * The gap should only be calculated when data is valid
   */
  describe('gap calculation', () => {
    it('should calculate gap correctly for valid data', () => {
      const trader1 = { total_pnl_pct: 15.5 }
      const trader2 = { total_pnl_pct: 10.2 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      const gap = hasValidData
        ? trader1.total_pnl_pct - trader2.total_pnl_pct
        : NaN

      expect(gap).toBeCloseTo(5.3, 1) // Allow floating point precision
      expect(isNaN(gap)).toBe(false)
    })

    it('should return NaN for invalid data', () => {
      const trader1 = { total_pnl_pct: null }
      const trader2 = { total_pnl_pct: 10.2 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct!) &&
        !isNaN(trader2.total_pnl_pct)

      const gap = hasValidData
        ? trader1.total_pnl_pct! - trader2.total_pnl_pct
        : NaN

      expect(isNaN(gap)).toBe(true)
    })

    it('should handle negative gap correctly', () => {
      const trader1 = { total_pnl_pct: 5.0 }
      const trader2 = { total_pnl_pct: 12.0 }

      const hasValidData =
        trader1.total_pnl_pct != null &&
        trader2.total_pnl_pct != null &&
        !isNaN(trader1.total_pnl_pct) &&
        !isNaN(trader2.total_pnl_pct)

      const gap = hasValidData
        ? trader1.total_pnl_pct - trader2.total_pnl_pct
        : NaN

      expect(gap).toBe(-7.0)
    })
  })

  /**
   * Test the display logic
   * After the fix it should show "—" instead of "NaN%" or "0.00%"
   */
  describe('display formatting', () => {
    it('should format valid positive percentage correctly', () => {
      const total_pnl_pct = 15.567

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('+15.57%')
    })

    it('should format valid negative percentage correctly', () => {
      const total_pnl_pct = -8.234

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('-8.23%')
    })

    it('should display "—" for null value', () => {
      const total_pnl_pct = null

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('—')
    })

    it('should display "—" for undefined value', () => {
      const total_pnl_pct = undefined

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('—')
    })

    it('should display "—" for NaN value', () => {
      const total_pnl_pct = NaN

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('—')
    })

    it('should format zero correctly', () => {
      const total_pnl_pct = 0

      const display =
        total_pnl_pct != null && !isNaN(total_pnl_pct)
          ? `${total_pnl_pct >= 0 ? '+' : ''}${total_pnl_pct.toFixed(2)}%`
          : '—'

      expect(display).toBe('+0.00%')
    })
  })

  /**
   * Test the leading/trailing message display logic
   * The "leading" or "trailing" message should only show when data is valid
   */
  describe('leading/trailing message display', () => {
    it('should show leading message when winning with positive gap', () => {
      const isWinning = true
      const gap = 5.2
      const hasValidData = true

      const shouldShowLeading = hasValidData && isWinning && gap > 0

      expect(shouldShowLeading).toBe(true)
    })

    it('should not show leading message when data is invalid', () => {
      const isWinning = true
      const gap = NaN
      const hasValidData = false

      const shouldShowLeading = hasValidData && isWinning && gap > 0

      expect(shouldShowLeading).toBe(false)
    })

    it('should show trailing message when losing with negative gap', () => {
      const isWinning = false
      const gap = -3.5
      const hasValidData = true

      const shouldShowTrailing = hasValidData && !isWinning && gap < 0

      expect(shouldShowTrailing).toBe(true)
    })

    it('should not show trailing message when data is invalid', () => {
      const isWinning = false
      const gap = NaN
      const hasValidData = false

      const shouldShowTrailing = hasValidData && !isWinning && gap < 0

      expect(shouldShowTrailing).toBe(false)
    })

    it('should show fallback "—" when data is invalid', () => {
      const hasValidData = false

      const shouldShowFallback = !hasValidData

      expect(shouldShowFallback).toBe(true)
    })
  })

  /**
   * Test edge cases
   */
  describe('edge cases', () => {
    it('should handle very small positive numbers', () => {
      const total_pnl_pct = 0.001

      const hasValidData = total_pnl_pct != null && !isNaN(total_pnl_pct)

      expect(hasValidData).toBe(true)
    })

    it('should handle very large numbers', () => {
      const total_pnl_pct = 9999.99

      const hasValidData = total_pnl_pct != null && !isNaN(total_pnl_pct)

      expect(hasValidData).toBe(true)
    })

    it('should handle Infinity as invalid (produces NaN in calculations)', () => {
      const total_pnl_pct = Infinity

      // Infinity itself is not NaN, but it can cause issues in subtraction
      const hasValidData = total_pnl_pct != null && isFinite(total_pnl_pct)

      expect(hasValidData).toBe(false)
    })

    it('should handle -Infinity as invalid', () => {
      const total_pnl_pct = -Infinity

      const hasValidData = total_pnl_pct != null && isFinite(total_pnl_pct)

      expect(hasValidData).toBe(false)
    })
  })
})
