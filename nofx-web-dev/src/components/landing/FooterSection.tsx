import { t, Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  return (
    <footer
      style={{
        borderTop: '1px solid var(--panel-border)',
        background: 'var(--panel-bg)',
      }}
    >
      <div className="max-w-[1200px] mx-auto px-6 py-10">
        {/* Brand - Hidden */}
        {/* <div className="flex items-center gap-3 mb-8">
          <img src="/images/logo.jpg" alt="Logo" className="w-8 h-8" />
          <div>
            <div
              className="text-lg font-bold"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('appTitle', language)}
            </div>
            <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              {t('futureStandardAI', language)}
            </div>
          </div>
        </div> */}

        {/* Bottom note (kept subtle) */}
        <div
          className="text-center text-xs"
          style={{
            color: 'var(--text-tertiary)',
          }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
