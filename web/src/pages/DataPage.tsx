import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

export function DataPage() {
  const { language } = useLanguage()

  return (
    <div className="w-full h-[calc(100vh-64px)]">
      <iframe
        src="https://nofxos.ai/dashboard"
        title={t('dataCenter', language)}
        className="w-full h-full border-0"
        allow="fullscreen"
      />
    </div>
  )
}
