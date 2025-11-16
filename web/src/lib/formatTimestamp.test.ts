import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { formatTimestamp } from './formatTimestamp'

describe('formatTimestamp', () => {
  let originalDate: typeof Date

  beforeEach(() => {
    originalDate = global.Date
  })

  afterEach(() => {
    global.Date = originalDate
  })

  describe('when timestamp is today', () => {
    it('should display only time in zh-CN format', () => {
      // Mock current date: 2025-11-16 14:30:00
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Test timestamp: same day, different time
      const timestamp = '2025-11-16T10:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should only show time, not date
      expect(result).toMatch(/^\d{2}:\d{2}$/)
      expect(result).not.toContain('11')
      expect(result).not.toContain('16')
    })

    it('should display only time in en-US format', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      const timestamp = '2025-11-16T10:45:00Z'
      const result = formatTimestamp(timestamp, 'en')

      // Should only show time
      expect(result).toMatch(/^\d{2}:\d{2}$/)
      expect(result).not.toContain('11')
      expect(result).not.toContain('16')
    })

    it('should handle edge case: exact same time', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      const timestamp = '2025-11-16T14:30:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      expect(result).toMatch(/^\d{2}:\d{2}$/)
    })

    it('should handle edge case: just before midnight', () => {
      const mockNow = new Date('2025-11-16T23:59:59Z')
      vi.setSystemTime(mockNow)

      const timestamp = '2025-11-16T23:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      expect(result).toMatch(/^\d{2}:\d{2}$/)
    })
  })

  describe('when timestamp is not today', () => {
    it('should display date and time in zh-CN format', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Yesterday
      const timestamp = '2025-11-15T10:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should contain both date and time
      expect(result).toContain('11')
      expect(result).toContain('15')
      expect(result).toMatch(/\d{2}:\d{2}/)
    })

    it('should display date and time in en-US format', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Yesterday
      const timestamp = '2025-11-15T10:45:00Z'
      const result = formatTimestamp(timestamp, 'en')

      // Should contain both date and time
      expect(result).toContain('11')
      expect(result).toContain('15')
      expect(result).toMatch(/\d{2}:\d{2}/)
    })

    it('should handle timestamps from previous month', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Previous month
      const timestamp = '2025-10-25T10:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should contain month and day
      expect(result).toContain('10')
      expect(result).toContain('25')
    })

    it('should handle timestamps from previous year', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Previous year
      const timestamp = '2024-12-25T10:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should contain date information
      expect(result).toContain('12')
      expect(result).toContain('25')
    })
  })

  describe('edge cases', () => {
    it('should handle timezone differences correctly', () => {
      // Current time in UTC
      const mockNow = new Date('2025-11-16T02:00:00Z')
      vi.setSystemTime(mockNow)

      // Same calendar day in UTC, but might be different in local timezone
      const timestamp = '2025-11-16T01:00:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should recognize as same day based on Date.toDateString() comparison
      expect(result).toMatch(/^\d{2}:\d{2}$/)
    })

    it('should handle invalid timestamp gracefully', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      const invalidTimestamp = 'invalid-date'

      // Should not throw error
      expect(() => formatTimestamp(invalidTimestamp, 'zh')).not.toThrow()
    })

    it('should default to zh-CN when language is unknown', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      const timestamp = '2025-11-16T10:45:00Z'
      const result = formatTimestamp(timestamp, 'fr') // French not supported

      // Should still return a valid time string
      expect(result).toMatch(/\d{2}:\d{2}/)
    })
  })

  describe('format consistency', () => {
    it('should always use 2-digit format for hours and minutes', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Early morning time
      const timestamp = '2025-11-16T03:05:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should be "03:05" not "3:5"
      expect(result).toMatch(/^\d{2}:\d{2}$/)
    })

    it('should use 2-digit format for month and day when showing date', () => {
      const mockNow = new Date('2025-11-16T14:30:00Z')
      vi.setSystemTime(mockNow)

      // Single digit month and day
      const timestamp = '2025-03-05T10:45:00Z'
      const result = formatTimestamp(timestamp, 'zh')

      // Should contain "03" and "05"
      expect(result).toContain('03')
      expect(result).toContain('05')
    })
  })
})
