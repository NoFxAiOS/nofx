export const L = (zh: string, en: string, language: string) =>
  language === 'zh' ? zh : en

export function isNum(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v)
}

/** 固定小数位；null/非数字显示 —。 */
export function fmtNum(v: unknown, digits: number): string {
  return isNum(v) ? v.toFixed(digits) : '—'
}

export const fmtIC = (v: unknown) => fmtNum(v, 3)
export const fmtT = (v: unknown) => fmtNum(v, 2)

/** 小数收益 → 百分比，保留 1 位。 */
export function fmtPct(v: unknown): string {
  return isNum(v) ? `${(v * 100).toFixed(1)}%` : '—'
}

export function fmtInt(v: unknown): string {
  return isNum(v) ? Math.round(v).toLocaleString('en-US') : '—'
}

export function fmtText(v: unknown): string {
  return typeof v === 'string' && v !== '' ? v : '—'
}
