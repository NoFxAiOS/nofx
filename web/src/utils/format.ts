/**
 * 数字格式化工具
 *
 * formatPrice: 根据数值大小自适应显示精度，避免极小数显示为 0.0000
 */

/**
 * 格式化价格，根据数值大小自适应精度
 * 对于极小的数字（如 meme 币价格 0.000000166），会保留足够的有效数字
 *
 * @param price 价格数值
 * @param minDecimals 最少小数位数（默认 2）
 * @returns 格式化后的字符串
 */
export function formatPrice(price: number | undefined | null, minDecimals = 2): string {
  if (price === undefined || price === null || isNaN(price)) {
    return '0'
  }

  if (price === 0) {
    return '0'
  }

  const absPrice = Math.abs(price)

  // 根据价格大小决定显示精度
  let decimals: number
  if (absPrice < 0.000001) {
    // 极小价格 (如 CHEEMS, SHIB 等 meme 币)
    decimals = 15
  } else if (absPrice < 0.0001) {
    // 很小价格 (如 PEPE, FLOKI, BONK)
    decimals = 12
  } else if (absPrice < 0.01) {
    // 小价格
    decimals = 10
  } else if (absPrice < 1) {
    // 中等价格
    decimals = 8
  } else if (absPrice < 1000) {
    // 正常价格
    decimals = 4
  } else {
    // 大价格 (如 BTC)
    decimals = 2
  }

  // 确保至少有 minDecimals 位小数
  decimals = Math.max(decimals, minDecimals)

  // 格式化并去除尾部多余的零
  let formatted = price.toFixed(decimals)

  // 去除尾部零（保留小数点后至少 minDecimals 位）
  if (formatted.includes('.')) {
    // 先去掉所有尾部零
    formatted = formatted.replace(/\.?0+$/, '')
    // 如果小数位不足 minDecimals，补零
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
 * 格式化数量，根据数值大小自适应精度
 *
 * @param quantity 数量
 * @param minDecimals 最少小数位数（默认 2）
 * @returns 格式化后的字符串
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
 * 格式化百分比
 *
 * @param value 百分比值
 * @param decimals 小数位数（默认 2）
 * @returns 格式化后的字符串
 */
export function formatPercent(value: number | undefined | null, decimals = 2): string {
  if (value === undefined || value === null || isNaN(value)) {
    return '0.00'
  }
  return value.toFixed(decimals)
}

export default { formatPrice, formatQuantity, formatPercent }
