import { describe, expect, it } from 'vitest'
import { formatPercent, formatPrice, formatQuantity } from './format'

const invalidValues = [
  { label: 'null', value: null },
  { label: 'undefined', value: undefined },
  { label: 'NaN', value: Number.NaN },
]

describe.each([
  { name: 'formatPrice', format: formatPrice },
  { name: 'formatQuantity', format: formatQuantity },
])('$name empty and zero values', ({ format }) => {
  it.each([
    ...invalidValues,
    { label: 'zero', value: 0 },
    { label: 'negative zero', value: -0 },
  ])('returns "0" for $label', ({ value }) => {
    expect(format(value)).toBe('0')
  })
})

describe.each([
  { name: 'formatPrice', format: formatPrice },
  { name: 'formatQuantity', format: formatQuantity },
])('$name with zero minimum decimals', ({ format }) => {
  it.each<[number, string]>([
    [12, '12'],
    [-12, '-12'],
    [1200, '1200'],
    [-1200, '-1200'],
    [12.00001, '12'],
    [999.99999, '1000'],
    [-999.99999, '-1000'],
  ])(
    'formats integral or rounded value %s without a trailing dot',
    (value, expected) => {
      expect(format(value, 0)).toBe(expected)
    }
  )
})

describe('formatPrice', () => {
  it.each<[number, string]>([
    [0.000000999999999, '0.000000999999999'],
    [0.000001, '0.000001'],
    [0.0000010000009, '0.000001000001'],
    [0.000099999999, '0.000099999999'],
    [0.0001, '0.0001'],
    [0.00010000009, '0.0001000001'],
    [0.0099999999, '0.0099999999'],
    [0.01, '0.01'],
    [0.010000009, '0.01000001'],
    [0.99999999, '0.99999999'],
    [1, '1.00'],
    [1.00009, '1.0001'],
    [999.9999, '999.9999'],
    [1000, '1000.00'],
    [1000.009, '1000.01'],
  ])('formats precision-boundary price %s as %s', (value, expected) => {
    expect(formatPrice(value)).toBe(expected)
  })

  it.each<[number, string]>([
    [0.000000166, '0.000000166'],
    [0.000000000000001, '0.000000000000001'],
    [-0.000000166, '-0.000000166'],
    [-0.000001123456789, '-0.000001123457'],
    [-0.123456789, '-0.12345679'],
    [-12.34567, '-12.3457'],
    [-1000.009, '-1000.01'],
    [12345.678, '12345.68'],
    [999.99999, '1000.00'],
  ])('preserves sign and useful precision for %s', (value, expected) => {
    expect(formatPrice(value)).toBe(expected)
  })

  it.each<[number, string]>([
    [12.34, '12.34'],
    [12.3, '12.30'],
    [12, '12.00'],
    [0.5, '0.50'],
    [1000.1, '1000.10'],
  ])(
    'trims extra zeros while keeping two decimals for %s',
    (value, expected) => {
      expect(formatPrice(value)).toBe(expected)
    }
  )

  it.each<[number, number, string]>([
    [1.25, 0, '1.25'],
    [12, 1, '12.0'],
    [12.3, 3, '12.300'],
    [1.23456789, 6, '1.234568'],
    [1234.56789, 6, '1234.567890'],
    [0.000000166, 4, '0.000000166'],
  ])(
    'formats %s with minimum %s decimals as %s',
    (value, decimals, expected) => {
      expect(formatPrice(value, decimals)).toBe(expected)
    }
  )
})

describe('formatQuantity', () => {
  it.each<[number, string]>([
    [0.99999999, '0.99999999'],
    [1, '1.00'],
    [1.00009, '1.0001'],
    [999.9999, '999.9999'],
    [1000, '1000.00'],
    [1000.009, '1000.01'],
    [999999.99, '999999.99'],
    [1000000, '1000000.00'],
    [1000000.009, '1000000.01'],
  ])('formats precision-boundary quantity %s as %s', (value, expected) => {
    expect(formatQuantity(value)).toBe(expected)
  })

  it.each<[number, string]>([
    [0.00000001, '0.00000001'],
    [-0.123456789, '-0.12345679'],
    [-12.34567, '-12.3457'],
    [-1234.567, '-1234.57'],
    [-1000000.009, '-1000000.01'],
    [1234567.896, '1234567.90'],
  ])(
    'preserves sign and magnitude-based precision for %s',
    (value, expected) => {
      expect(formatQuantity(value)).toBe(expected)
    }
  )

  it.each<[number, string]>([
    [12.34, '12.34'],
    [12.3, '12.30'],
    [12, '12.00'],
    [0.5, '0.50'],
    [1000.1, '1000.10'],
  ])(
    'trims extra zeros while keeping two decimals for %s',
    (value, expected) => {
      expect(formatQuantity(value)).toBe(expected)
    }
  )

  it.each<[number, number, string]>([
    [12.34, 0, '12.34'],
    [12, 1, '12.0'],
    [12.3, 3, '12.300'],
    [1.23456789, 6, '1.234568'],
    [0.123456789, 4, '0.12345679'],
    [1234.56789, 4, '1234.5679'],
    [999999.9, 0, '999999.9'],
    [1000000.4, 0, '1000000'],
    [1000000.6, 0, '1000001'],
  ])(
    'formats %s with minimum %s decimals as %s',
    (value, decimals, expected) => {
      expect(formatQuantity(value, decimals)).toBe(expected)
    }
  )
})

describe('formatPercent', () => {
  it.each(invalidValues)(
    'returns "0.00" for $label by default',
    ({ value }) => {
      expect(formatPercent(value)).toBe('0.00')
    }
  )

  describe.each(invalidValues)('$label fallback', ({ value }) => {
    it.each<[number, string]>([
      [0, '0'],
      [1, '0.0'],
      [3, '0.000'],
      [4, '0.0000'],
    ])('respects %s decimal places', (decimals, expected) => {
      expect(formatPercent(value, decimals)).toBe(expected)
    })
  })

  it.each<[number, string]>([
    [0, '0.00'],
    [-0, '0.00'],
    [12.3456, '12.35'],
    [-12.3456, '-12.35'],
    [1.2, '1.20'],
    [12, '12.00'],
    [0.125, '0.13'],
    [0.0049, '0.00'],
    [0.0051, '0.01'],
    [-0.0051, '-0.01'],
  ])('formats percentage %s without scaling or a suffix', (value, expected) => {
    expect(formatPercent(value)).toBe(expected)
  })

  it.each<[number, number, string]>([
    [12.3456, 0, '12'],
    [12.6, 0, '13'],
    [12.3456, 1, '12.3'],
    [12.3456, 3, '12.346'],
    [-12.3456, 3, '-12.346'],
    [0, 4, '0.0000'],
    [1.2, 4, '1.2000'],
  ])(
    'formats %s with exactly %s decimals as %s',
    (value, decimals, expected) => {
      expect(formatPercent(value, decimals)).toBe(expected)
    }
  )
})
