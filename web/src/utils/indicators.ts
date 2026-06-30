// Technical indicator calculation utilities

export interface Kline {
  time: number
  open: number
  high: number
  low: number
  close: number
  volume?: number
}

// Simple Moving Average (SMA)
export function calculateSMA(data: Kline[], period: number): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []

  for (let i = period - 1; i < data.length; i++) {
    let sum = 0
    for (let j = 0; j < period; j++) {
      sum += data[i - j].close
    }
    result.push({
      time: data[i].time,
      value: sum / period,
    })
  }

  return result
}

// Exponential Moving Average (EMA)
export function calculateEMA(data: Kline[], period: number): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []
  const multiplier = 2 / (period + 1)

  // Use SMA for the first EMA value
  let ema = 0
  for (let i = 0; i < period; i++) {
    ema += data[i].close
  }
  ema = ema / period
  result.push({ time: data[period - 1].time, value: ema })

  // Subsequent EMA values
  for (let i = period; i < data.length; i++) {
    ema = (data[i].close - ema) * multiplier + ema
    result.push({ time: data[i].time, value: ema })
  }

  return result
}

// MACD indicator
export interface MACDData {
  time: number
  macd: number
  signal: number
  histogram: number
}

export function calculateMACD(
  data: Kline[],
  fastPeriod = 12,
  slowPeriod = 26,
  signalPeriod = 9
): MACDData[] {
  const fastEMA = calculateEMA(data, fastPeriod)
  const slowEMA = calculateEMA(data, slowPeriod)

  // Compute the MACD line
  const macdLine: Array<{ time: number; value: number }> = []
  for (let i = 0; i < slowEMA.length; i++) {
    const fastValue = fastEMA.find(e => e.time === slowEMA[i].time)
    if (fastValue) {
      macdLine.push({
        time: slowEMA[i].time,
        value: fastValue.value - slowEMA[i].value,
      })
    }
  }

  // Compute the signal line (EMA of MACD)
  const signalLine = calculateEMAFromValues(macdLine, signalPeriod)

  // Build the MACD data
  const result: MACDData[] = []
  for (let i = 0; i < signalLine.length; i++) {
    const macdValue = macdLine.find(m => m.time === signalLine[i].time)
    if (macdValue) {
      result.push({
        time: signalLine[i].time,
        macd: macdValue.value,
        signal: signalLine[i].value,
        histogram: macdValue.value - signalLine[i].value,
      })
    }
  }

  return result
}

// Compute EMA from an array of values (helper function)
function calculateEMAFromValues(
  data: Array<{ time: number; value: number }>,
  period: number
): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []
  const multiplier = 2 / (period + 1)

  if (data.length < period) return []

  // Use SMA for the first EMA value
  let ema = 0
  for (let i = 0; i < period; i++) {
    ema += data[i].value
  }
  ema = ema / period
  result.push({ time: data[period - 1].time, value: ema })

  // Subsequent EMA values
  for (let i = period; i < data.length; i++) {
    ema = (data[i].value - ema) * multiplier + ema
    result.push({ time: data[i].time, value: ema })
  }

  return result
}

// RSI indicator
export function calculateRSI(data: Kline[], period = 14): Array<{ time: number; value: number }> {
  const result: Array<{ time: number; value: number }> = []

  if (data.length < period + 1) return []

  // Compute price changes
  const changes: number[] = []
  for (let i = 1; i < data.length; i++) {
    changes.push(data[i].close - data[i - 1].close)
  }

  // Compute initial average gain/loss
  let avgGain = 0
  let avgLoss = 0
  for (let i = 0; i < period; i++) {
    if (changes[i] > 0) {
      avgGain += changes[i]
    } else {
      avgLoss += Math.abs(changes[i])
    }
  }
  avgGain = avgGain / period
  avgLoss = avgLoss / period

  // Compute RSI
  for (let i = period; i < changes.length; i++) {
    const currentChange = changes[i]

    if (currentChange > 0) {
      avgGain = (avgGain * (period - 1) + currentChange) / period
      avgLoss = (avgLoss * (period - 1)) / period
    } else {
      avgGain = (avgGain * (period - 1)) / period
      avgLoss = (avgLoss * (period - 1) + Math.abs(currentChange)) / period
    }

    const rs = avgGain / avgLoss
    const rsi = 100 - 100 / (1 + rs)

    result.push({
      time: data[i + 1].time,
      value: rsi,
    })
  }

  return result
}

// Bollinger Bands
export interface BollingerBands {
  time: number
  upper: number
  middle: number
  lower: number
}

export function calculateBollingerBands(
  data: Kline[],
  period = 20,
  stdDev = 2
): BollingerBands[] {
  const result: BollingerBands[] = []

  for (let i = period - 1; i < data.length; i++) {
    // Compute SMA
    let sum = 0
    for (let j = 0; j < period; j++) {
      sum += data[i - j].close
    }
    const sma = sum / period

    // Compute standard deviation
    let variance = 0
    for (let j = 0; j < period; j++) {
      variance += Math.pow(data[i - j].close - sma, 2)
    }
    const std = Math.sqrt(variance / period)

    result.push({
      time: data[i].time,
      upper: sma + stdDev * std,
      middle: sma,
      lower: sma - stdDev * std,
    })
  }

  return result
}
