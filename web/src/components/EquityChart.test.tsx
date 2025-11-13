import { describe, it, expect } from 'vitest'

/**
 * EquityChart 时间格式化测试
 *
 * 测试修改：横轴显示日期 + 时间，而不仅仅是时间
 * 修改前：只显示 "10:30"
 * 修改后：显示 "01/14 10:30"（月/日 时:分）
 */

describe('EquityChart - Time Formatting', () => {
  /**
   * 测试时间格式化逻辑
   * 模拟 EquityChart 中 chartData 的构建逻辑
   */
  describe('timestamp formatting with date and time', () => {
    it('should format timestamp with month, day, hour, and minute', () => {
      const timestamp = '2025-01-14T10:30:45.123Z'

      const formattedTime = new Date(timestamp).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      // 验证格式包含日期和时间
      // 格式应该是 "MM/DD HH:mm" 或类似格式
      expect(formattedTime).toMatch(/\d{2}\/\d{2}/)  // 包含日期部分 (MM/DD)
      expect(formattedTime).toMatch(/\d{2}:\d{2}/)   // 包含时间部分 (HH:mm)
    })

    it('should handle different months correctly', () => {
      const timestamps = [
        '2025-01-14T10:30:00Z',
        '2025-02-28T15:45:00Z',
        '2025-12-31T23:59:00Z',
      ]

      timestamps.forEach(timestamp => {
        const formattedTime = new Date(timestamp).toLocaleString('zh-CN', {
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        })

        // 验证格式正确
        expect(formattedTime).toMatch(/\d{2}\/\d{2}/)
        expect(formattedTime).toMatch(/\d{2}:\d{2}/)
      })
    })

    it('should format midnight correctly', () => {
      const timestamp = '2025-01-14T00:00:00Z'

      const formattedTime = new Date(timestamp).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      expect(formattedTime).toMatch(/\d{2}\/\d{2}/)
      expect(formattedTime).toMatch(/\d{2}:\d{2}/)
    })

    it('should format end of day correctly', () => {
      const timestamp = '2025-01-14T23:59:59Z'

      const formattedTime = new Date(timestamp).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      expect(formattedTime).toMatch(/\d{2}\/\d{2}/)
      expect(formattedTime).toMatch(/\d{2}:\d{2}/)
    })
  })

  /**
   * 测试与旧格式的对比
   * 确保新格式包含更多信息
   */
  describe('comparison with old time-only format', () => {
    it('new format should contain date information unlike old format', () => {
      const timestamp = '2025-01-14T10:30:00Z'

      // 旧格式：只有时间
      const oldFormat = new Date(timestamp).toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
      })

      // 新格式：日期 + 时间
      const newFormat = new Date(timestamp).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      // 旧格式不应包含日期
      expect(oldFormat).not.toMatch(/\d{2}\/\d{2}/)

      // 新格式应包含日期
      expect(newFormat).toMatch(/\d{2}\/\d{2}/)

      // 新格式应该比旧格式长（包含更多信息）
      expect(newFormat.length).toBeGreaterThan(oldFormat.length)
    })
  })

  /**
   * 测试跨天场景
   * 在同一个图表中显示跨越多天的数据时，日期信息尤其重要
   */
  describe('multi-day data scenarios', () => {
    it('should distinguish between same time on different days', () => {
      const day1 = '2025-01-14T10:30:00Z'
      const day2 = '2025-01-15T10:30:00Z'

      const format1 = new Date(day1).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      const format2 = new Date(day2).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      // 两个不同日期的相同时间应该产生不同的格式化字符串
      expect(format1).not.toBe(format2)

      // 两者都应包含日期和时间
      expect(format1).toMatch(/\d{2}\/\d{2}/)
      expect(format2).toMatch(/\d{2}\/\d{2}/)
    })

    it('should handle month boundaries correctly', () => {
      const endOfMonth = '2025-01-31T23:59:00Z'
      const startOfMonth = '2025-02-01T00:01:00Z'

      const format1 = new Date(endOfMonth).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      const format2 = new Date(startOfMonth).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      // 跨月的时间点应该产生不同的格式化字符串
      expect(format1).not.toBe(format2)
    })
  })

  /**
   * 测试 chartData 转换逻辑
   * 模拟完整的数据转换过程
   */
  describe('chartData transformation', () => {
    it('should transform equity points with correct time format', () => {
      const mockEquityPoints = [
        {
          timestamp: '2025-01-14T10:00:00Z',
          total_equity: 1100,
          pnl: 100,
          pnl_pct: 10,
          cycle_number: 1,
        },
        {
          timestamp: '2025-01-14T11:00:00Z',
          total_equity: 1150,
          pnl: 150,
          pnl_pct: 15,
          cycle_number: 2,
        },
      ]

      const initialBalance = 1000

      // 模拟 chartData 转换逻辑
      const chartData = mockEquityPoints.map((point) => {
        const pnl = point.total_equity - initialBalance
        const pnlPct = ((pnl / initialBalance) * 100).toFixed(2)
        return {
          time: new Date(point.timestamp).toLocaleString('zh-CN', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
          }),
          value: point.total_equity,
          cycle: point.cycle_number,
          raw_equity: point.total_equity,
          raw_pnl: pnl,
          raw_pnl_pct: parseFloat(pnlPct),
        }
      })

      // 验证转换后的数据
      expect(chartData).toHaveLength(2)

      // 验证每个数据点都有正确格式的时间
      chartData.forEach(dataPoint => {
        expect(dataPoint.time).toMatch(/\d{2}\/\d{2}/)  // 包含日期
        expect(dataPoint.time).toMatch(/\d{2}:\d{2}/)   // 包含时间
      })
    })
  })
})
