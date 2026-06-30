/**
 * Number formatting utilities
 *
 * formatPrice: adapts display precision to the magnitude of the value,
 * avoiding very small numbers being shown as 0.0000
 */

/**
 * Format a price, adapting precision to the magnitude of the value.
 * For very small numbers (e.g. meme coin price 0.000000166), it keeps
 * enough significant digits.
 *
 * @param price the price value
 * @param minDecimals minimum number of decimal places (default 2)
 * @returns the formatted string
 */
export function formatPrice(price: number | undefined | null, minDecimals = 2): string {
  if (price === undefined || price === null || isNaN(price)) {
    return '0'
  }

  if (price === 0) {
    return '0'
  }

  const absPrice = Math.abs(price)

  // Determine display precision based on price magnitude
  let decimals: number
  if (absPrice < 0.000001) {
    // Extremely small price (e.g. meme coins like CHEEMS, SHIB)
    decimals = 15
  } else if (absPrice < 0.0001) {
    // Very small price (e.g. PEPE, FLOKI, BONK)
    decimals = 12
  } else if (absPrice < 0.01) {
    // Small price
    decimals = 10
  } else if (absPrice < 1) {
    // Medium price
    decimals = 8
  } else if (absPrice < 1000) {
    // Normal price
    decimals = 4
  } else {
    // Large price (e.g. BTC)
    decimals = 2
  }

  // Ensure at least minDecimals decimal places
  decimals = Math.max(decimals, minDecimals)

  // Format and strip extra trailing zeros
  let formatted = price.toFixed(decimals)

  // Strip trailing zeros (keep at least minDecimals decimal places)
  if (formatted.includes('.')) {
    // First remove all trailing zeros
    formatted = formatted.replace(/\.?0+$/, '')
    // Pad with zeros if there are fewer than minDecimals decimals
    const dotIndex = formatted.indexOf('.')
    if (dotIndex === -1) {
      formatted += '.' + '0'.repeat(minDecimals)
    } else {
      const currentDecimals = formatted.length - dotIndex - 1
      if (currentDecimals < minDecimals) {
        formatted += '0'.repeat(minDecimals - currentDecimals)
      }
    }
  }

  return formatted
}

/**
 * Format a quantity, adapting precision to the magnitude of the value.
 *
 * @param quantity the quantity
 * @param minDecimals minimum number of decimal places (default 2)
 * @returns the formatted string
 */
export function formatQuantity(quantity: number | undefined | null, minDecimals = 2): string {
  if (quantity === undefined || quantity === null || isNaN(quantity)) {
    return '0'
  }

  if (quantity === 0) {
    return '0'
  }

  const absQty = Math.abs(quantity)

  let decimals: number
  if (absQty >= 1000000) {
    decimals = 0
  } else if (absQty >= 1000) {
    decimals = 2
  } else if (absQty >= 1) {
    decimals = 4
  } else {
    decimals = 8
  }

  decimals = Math.max(decimals, minDecimals)

  let formatted = quantity.toFixed(decimals)
  if (formatted.includes('.')) {
    formatted = formatted.replace(/\.?0+$/, '')
    const dotIndex = formatted.indexOf('.')
    if (dotIndex === -1) {
      formatted += '.' + '0'.repeat(minDecimals)
    } else {
      const currentDecimals = formatted.length - dotIndex - 1
      if (currentDecimals < minDecimals) {
        formatted += '0'.repeat(minDecimals - currentDecimals)
      }
    }
  }

  return formatted
}

/**
 * Format a percentage
 *
 * @param value the percentage value
 * @param decimals number of decimal places (default 2)
 * @returns the formatted string
 */
export function formatPercent(value: number | undefined | null, decimals = 2): string {
  if (value === undefined || value === null || isNaN(value)) {
    return '0.00'
  }
  return value.toFixed(decimals)
}

export default { formatPrice, formatQuantity, formatPercent }
