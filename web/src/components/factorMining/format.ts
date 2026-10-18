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

export type ForwardBadgeVariant = 'neutral' | 'up' | 'warn' | 'down'

/** 前向校验 verdict → 徽标文案与配色；未知/缺失返回 null。 */
export function forwardBadge(
  fw:
    | {
        verdict?: string | null
        n_days?: number | null
        min_days?: number | null
      }
    | null
    | undefined,
  language: string
): { text: string; variant: ForwardBadgeVariant } | null {
  switch (fw?.verdict) {
    case 'pending': {
      const n = isNum(fw.n_days) ? Math.round(fw.n_days) : 0
      const min = isNum(fw.min_days) ? Math.round(fw.min_days) : '?'
      return {
        text: L(`观察中 ${n}/${min} 天`, `Watching ${n}/${min} d`, language),
        variant: 'neutral',
      }
    }
    case 'confirmed':
      return { text: L('已确认', 'Confirmed', language), variant: 'up' }
    case 'inconclusive':
      return { text: L('不确定', 'Inconclusive', language), variant: 'warn' }
    case 'failed':
      return { text: L('已失效', 'Failed', language), variant: 'down' }
    default:
      return null
  }
}
