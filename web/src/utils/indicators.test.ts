import { describe, expect, it } from 'vitest'
import { calculateEMA, calculateMACD, type Kline } from './indicators'

function candles(closes: number[]): Kline[] {
  return closes.map((close, index) => ({
    time: index + 1,
    open: close,
    high: close,
    low: close,
    close,
  }))
}

function history(length: number): Kline[] {
  return candles(Array.from({ length }, (_, index) => index + 1))
}

describe('calculateEMA', () => {
  it.each([
    [0, 12],
    [1, 12],
    [11, 12],
    [25, 26],
  ])('returns no values for %i candles with period %i', (length, period) => {
    expect(calculateEMA(history(length), period)).toEqual([])
  })

  it('seeds the first value when exactly one full period is available', () => {
    expect(calculateEMA(candles([10, 20, 30]), 3)).toEqual([
      { time: 3, value: 20 },
    ])
  })

  it('keeps subsequent values aligned without changing the candle input', () => {
    const input = candles([10, 20, 30, 50, 20])
    const original = input.map((candle) => ({ ...candle }))

    expect(calculateEMA(input, 3)).toEqual([
      { time: 3, value: 20 },
      { time: 4, value: 35 },
      { time: 5, value: 27.5 },
    ])
    expect(input).toEqual(original)
  })

  it('returns each close at its original time for period one', () => {
    expect(calculateEMA(candles([3, 5, 2]), 1)).toEqual([
      { time: 1, value: 3 },
      { time: 2, value: 5 },
      { time: 3, value: 2 },
    ])
  })
})

describe('calculateMACD short-history handling', () => {
  it.each([0, 11, 25, 33])(
    'returns no values while %i candles cannot fill the EMA and signal windows',
    (length) => {
      expect(calculateMACD(history(length))).toEqual([])
    }
  )

  it('emits the first default signal after 34 constant-price candles', () => {
    expect(calculateMACD(candles(Array(34).fill(10)))).toEqual([
      { time: 34, macd: 0, signal: 0, histogram: 0 },
    ])
  })

  it('preserves custom-period calculations once enough history is available', () => {
    expect(calculateMACD(candles([10, 20, 30, 40]), 2, 3, 2)).toEqual([
      { time: 4, macd: 5, signal: 5, histogram: 0 },
    ])
  })
})
