import { t, type Language } from '../../i18n/translations'

export function PaperBadge({
  exchangeType,
  language,
}: {
  exchangeType?: string
  language: Language
}) {
  if (exchangeType !== 'paper') return null
  return (
    <span className="ml-2 inline-flex rounded border border-ai/40 bg-ai-soft px-2 py-0.5 text-xs font-semibold text-ai">
      {t('paper.badge', language)}
    </span>
  )
}
