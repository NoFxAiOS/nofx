// 2026-10-10 per-candidate block reasons: human labels for hard_entry_gate
// failure codes. Families are matched by regex; numeric suffixes stay visible.
// Unknown codes fall back to the raw code.
import { t, type Language } from '../i18n/translations'

type Rule = {
  re: RegExp
  key: string // translation key (gateCode.*), "{v}" is replaced by the suffix
  v?: (m: RegExpMatchArray) => string
}

const pair = (m: RegExpMatchArray) => `${m[1]} / ${m[2]}`

// Order matters: first match wins.
const RULES: Rule[] = [
  { re: /^BTC_4H_DOWNTREND$/, key: 'gateCode.btc4hDowntrend' },
  { re: /^BTC_4H_STRONGBULL$/, key: 'gateCode.btc4hStrongBull' },
  {
    re: /^BTC_WEAK_LONG_(-?[\d.]+)_VS_(-?[\d.]+)$/,
    key: 'gateCode.btcWeakLong',
    v: pair,
  },
  { re: /^BTC_WEAK_LONG/, key: 'gateCode.btcWeakLong', v: () => '' },
  { re: /^MICRO_TREND_NOT_LONG$/, key: 'gateCode.microNotLong' },
  { re: /^MICRO_TREND_NOT_SHORT$/, key: 'gateCode.microNotShort' },
  { re: /^RR_MAX_(-?[\d.]+)$/, key: 'gateCode.rrMax', v: (m) => m[1] },
  { re: /^RR_MAX/, key: 'gateCode.rrMax', v: () => '' },
  { re: /^STOP_PLAN_OUT_OF_BAND$/, key: 'gateCode.stopOutOfBand' },
  { re: /^STOP_PLAN_NO_STRUCTURE$/, key: 'gateCode.stopNoStructure' },
  {
    re: /^WIDE_STOP_(-?[\d.]+)_GT_(-?[\d.]+)$/,
    key: 'gateCode.wideStop',
    v: (m) => `${m[1]}% > ${m[2]}%`,
  },
  {
    re: /^EMA20_STRETCH_(-?[\d.]+)_GT_(-?[\d.]+)$/,
    key: 'gateCode.ema20Stretch',
    v: (m) => `${m[1]}%`,
  },
  {
    re: /^CHASE_EMA20_1H_(-?[\d.]+)_GT_(-?[\d.]+)$/,
    key: 'gateCode.chaseEma20_1h',
    v: (m) => `${m[1]}% > ${m[2]}%`,
  },
  {
    re: /^COUNTER_EMA20_1H_(-?[\d.]+)_GT_(-?[\d.]+)$/,
    key: 'gateCode.counterEma20_1h',
    v: (m) => `${m[1]}% > ${m[2]}%`,
  },
  {
    re: /^RSI1H_EXHAUSTED_(-?[\d.]+)$/,
    key: 'gateCode.rsi1hExhausted',
    v: (m) => m[1],
  },
  {
    re: /^CHASE_MOVE4H_(-?[\d.]+)_GT_(-?[\d.]+)$/,
    key: 'gateCode.chaseMove4h',
    v: (m) => `${m[1]}% > ${m[2]}%`,
  },
  { re: /^EXTENDED_PUMP_UNCONFIRMED$/, key: 'gateCode.extendedPump' },
  {
    re: /^CONSENSUS_OPPOSED_(\d+)$/,
    key: 'gateCode.consensusOpposed',
    v: (m) => m[1],
  },
  {
    re: /^VENDOR_DIVERGENCE_(-?[\d.]+)$/,
    key: 'gateCode.vendorDivergence',
    v: (m) => `${m[1]}%`,
  },
  { re: /^LIMIT_ANCHOR_SUPPRESSED$/, key: 'gateCode.limitAnchorSuppressed' },
  { re: /^REGIME_LINE_BROKEN$/, key: 'gateCode.regimeLineBroken' },
  { re: /^DATA_INSUFFICIENT$/, key: 'gateCode.dataInsufficient' },
  { re: /^POOR_HISTORY$/, key: 'gateCode.poorHistory' },
  { re: /^LOSS_STREAK_BANNED$/, key: 'gateCode.lossStreakBanned' },
  {
    re: /^NEG_EDGE_(.+)$/,
    key: 'gateCode.negEdge',
    v: (m) => m[1].replace(/_LT_/g, ' < ').replace(/_/g, ' ').toLowerCase(),
  },
  { re: /^STOCK_WEEKEND$/, key: 'gateCode.stockWeekend' },
  { re: /^MIN_SIZE_DEAD_ZONE$/, key: 'gateCode.minSizeDeadZone' },
  { re: /^SHORT_TOP_CONFIRM_MISSING$/, key: 'gateCode.shortTopConfirmMissing' },
]

/** Human label for a gate code; `raw` is always the original code (tooltip). */
export function gateCodeLabel(
  code: string,
  language: Language
): { label: string; raw: string } {
  for (const rule of RULES) {
    const m = code.match(rule.re)
    if (!m) continue
    const v = rule.v ? rule.v(m) : ''
    const text = t(rule.key, language).replace('{v}', v).trim()
    if (text === rule.key) break // missing translation → raw
    return { label: text, raw: code }
  }
  return { label: code, raw: code }
}
