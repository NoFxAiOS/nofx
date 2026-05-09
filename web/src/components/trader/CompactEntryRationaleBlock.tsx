import { formatPrice } from '../../utils/format'
import type { Language, } from '../../i18n/translations'
import type { DecisionActionReasonAnchor } from '../../types'

type CompactEntryRationaleBlockProps = {
  language: Language
  timeframeTrail: string[]
  rrSummary: string[]
  supportSummary: string[]
  resistanceSummary: string[]
  fibLevels: number[]
  anchors: DecisionActionReasonAnchor[]
  alignmentNotes: string[]
  toneColors: (tone: 'danger' | 'warn' | 'neutral') => { border: string; bg: string; color: string }
}

export function CompactEntryRationaleBlock({
  language,
  timeframeTrail,
  rrSummary,
  supportSummary,
  resistanceSummary,
  fibLevels,
  anchors,
  alignmentNotes,
  toneColors,
}: CompactEntryRationaleBlockProps) {
  if (timeframeTrail.length === 0 && rrSummary.length === 0 && supportSummary.length === 0 && resistanceSummary.length === 0 && fibLevels.length === 0 && anchors.length === 0 && alignmentNotes.length === 0) {
    return null
  }

  return (
    <div className="mt-3 pt-3 space-y-2" style={{ borderTop: '1px solid #2B3139' }}>
      <div className="text-[11px]" style={{ color: '#848E9C' }}>
        {language === 'zh' ? '开仓结构依据 / rationale' : 'entry rationale'}
      </div>
      {timeframeTrail.length > 0 && (
        <div className="text-[11px]" style={{ color: '#AAB2BD' }}>
          tf · {timeframeTrail.join(' · ')}
        </div>
      )}
      {rrSummary.length > 0 && (
        <div className="text-[11px] font-mono" style={{ color: '#C9D1D9' }}>
          {rrSummary.join(' · ')}
        </div>
      )}
      {(supportSummary.length > 0 || resistanceSummary.length > 0) && (
        <div className="text-[11px]" style={{ color: '#AAB2BD' }}>
          {supportSummary.length > 0 ? `S ${supportSummary.join(' / ')}` : ''}
          {supportSummary.length > 0 && resistanceSummary.length > 0 ? ' · ' : ''}
          {resistanceSummary.length > 0 ? `R ${resistanceSummary.join(' / ')}` : ''}
        </div>
      )}
      <div className="flex flex-wrap gap-1.5 text-[10px]">
        {fibLevels.length > 0 && (
          <span className="inline-flex items-center rounded-full px-2 py-0.5" style={{ border: '1px solid rgba(168, 85, 247, 0.25)', background: 'rgba(168, 85, 247, 0.12)', color: '#D8B4FE' }}>
            fib {fibLevels.length} levels
          </span>
        )}
        {anchors.slice(0, 3).map((anchor, idx) => (
          <span key={`${anchor.type}-${anchor.timeframe}-${anchor.price}-${idx}`} className="inline-flex items-center rounded-full px-2 py-0.5" style={toneColors('neutral')}>
            {anchor.type || 'anchor'}{anchor.timeframe ? `@${anchor.timeframe}` : ''}{anchor.price ? ` ${formatPrice(anchor.price)}` : ''}
          </span>
        ))}
      </div>
      {alignmentNotes.length > 0 && (
        <div className="text-[11px]" style={{ color: '#848E9C' }}>
          {alignmentNotes.join(' · ')}
        </div>
      )}
    </div>
  )
}
