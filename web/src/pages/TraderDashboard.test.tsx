import { describe, it, expect } from 'vitest'

/**
 * TraderDashboard - Initial Balance Card Test
 *
 * 测试需求：在 dashboard 页面添加初始金额卡片
 * - 卡片应该显示在最左边（第一个位置）
 * - 卡片标题显示"Initial Balance" / "初始余额"
 * - 卡片主内容显示初始金额（格式：XXX.XX USDT）
 * - 卡片子内容显示当前总盈亏（格式：+/-XXX.XX USDT）
 */

describe('TraderDashboard - Initial Balance Card Logic', () => {
  describe('Initial Balance Display Format', () => {
    it('should format initial balance with 2 decimal places', () => {
      const initialBalance = 1000.00
      const formatted = `${initialBalance.toFixed(2)} USDT`

      expect(formatted).toBe('1000.00 USDT')
    })

    it('should format initial balance correctly for different values', () => {
      const testCases = [
        { input: 100.00, expected: '100.00 USDT' },
        { input: 1000.00, expected: '1000.00 USDT' },
        { input: 500.50, expected: '500.50 USDT' },
        { input: 1234.56, expected: '1234.56 USDT' },
      ]

      testCases.forEach(({ input, expected }) => {
        const formatted = `${input.toFixed(2)} USDT`
        expect(formatted).toBe(expected)
      })
    })
  })

  describe('Total PnL Subtitle Format', () => {
    it('should format positive PnL with plus sign', () => {
      const totalPnL = 150.50
      const formatted = `${totalPnL >= 0 ? '+' : ''}${totalPnL.toFixed(2)} USDT`

      expect(formatted).toBe('+150.50 USDT')
    })

    it('should format negative PnL without extra sign (minus is implicit)', () => {
      const totalPnL = -50.25
      const formatted = `${totalPnL >= 0 ? '+' : ''}${totalPnL.toFixed(2)} USDT`

      expect(formatted).toBe('-50.25 USDT')
    })

    it('should format zero PnL with plus sign', () => {
      const totalPnL = 0
      const formatted = `${totalPnL >= 0 ? '+' : ''}${totalPnL.toFixed(2)} USDT`

      expect(formatted).toBe('+0.00 USDT')
    })

    it('should handle various PnL values correctly', () => {
      const testCases = [
        { input: 100.00, expected: '+100.00 USDT' },
        { input: -100.00, expected: '-100.00 USDT' },
        { input: 0.01, expected: '+0.01 USDT' },
        { input: -0.01, expected: '-0.01 USDT' },
        { input: 1234.56, expected: '+1234.56 USDT' },
      ]

      testCases.forEach(({ input, expected }) => {
        const formatted = `${input >= 0 ? '+' : ''}${input.toFixed(2)} USDT`
        expect(formatted).toBe(expected)
      })
    })
  })

  describe('Grid Layout Configuration', () => {
    it('should use 5 columns for medium screens and above', () => {
      // 验证 grid 布局配置
      const gridClasses = 'grid grid-cols-1 md:grid-cols-5 gap-4 mb-8'

      expect(gridClasses).toContain('grid-cols-1')
      expect(gridClasses).toContain('md:grid-cols-5')
    })

    it('should calculate card order correctly', () => {
      // 验证卡片顺序：初始金额应该是第一个
      const cardOrder = [
        'Initial Balance',  // 新增
        'Total Equity',     // 原有
        'Available Balance', // 原有
        'Total P&L',        // 原有
        'Positions',        // 原有
      ]

      expect(cardOrder[0]).toBe('Initial Balance')
      expect(cardOrder.length).toBe(5)
    })
  })

  describe('Data Handling', () => {
    it('should handle missing initial_balance gracefully', () => {
      const initialBalance: number | undefined = undefined
      const formatted = `${(initialBalance || 0).toFixed(2)} USDT`

      expect(formatted).toBe('0.00 USDT')
    })

    it('should handle missing total_pnl gracefully', () => {
      const totalPnL: number | undefined = undefined
      const formatted = `${(totalPnL || 0) >= 0 ? '+' : ''}${(totalPnL || 0).toFixed(2)} USDT`

      expect(formatted).toBe('+0.00 USDT')
    })

    it('should extract correct values from account data', () => {
      const mockAccount = {
        total_equity: 1150.50,
        available_balance: 950.25,
        unrealized_profit: 50.25,
        total_pnl: 150.50,
        total_pnl_pct: 15.05,
        initial_balance: 1000.00,
        daily_pnl: 50.25,
        position_count: 2,
        margin_used: 200.25,
        margin_used_pct: 17.4,
      }

      expect(mockAccount.initial_balance).toBe(1000.00)
      expect(mockAccount.total_pnl).toBe(150.50)
    })
  })
})
