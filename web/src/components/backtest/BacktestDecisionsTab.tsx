import { motion } from 'framer-motion'
import { DecisionCard } from '../trader/DecisionCard'
import type { Language } from '../../i18n/translations'
import type { DecisionRecord } from '../../types'

interface BacktestDecisionsTabProps {
  decisions: DecisionRecord[] | undefined
  language: Language
  tr: (key: string) => string
}

export function BacktestDecisionsTab({ decisions, language, tr }: BacktestDecisionsTabProps) {
  return (
    <motion.div
      key="decisions"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="space-y-3 max-h-[500px] overflow-y-auto"
    >
      {decisions && decisions.length > 0 ? (
        decisions.map((d) => (
          <DecisionCard
            key={`${d.cycle_number}-${d.timestamp}`}
            decision={d}
            language={language}
          />
        ))
      ) : (
        <div className="py-12 text-center" style={{ color: '#5E6673' }}>
          {tr('decisionTrail.emptyHint')}
        </div>
      )}
    </motion.div>
  )
}
