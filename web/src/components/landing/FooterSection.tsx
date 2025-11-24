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
        {/* Brand */}
        <div className="flex items-center gap-3 mb-8">
          <img src="/icons/ares.svg" alt="Ares Logo" className="w-8 h-8" />
          <div>
            <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
              Ares
            </div>
            <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              {t('futureStandardAI', language)}
            </div>
          </div>
        </div>

        {/* Multi-link columns */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-3 gap-8">
          <div>
            <h3
              className="text-sm font-semibold mb-3"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('links', language)}
            </h3>
            <ul className="space-y-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <li>
                <a
                  className="hover:text-[var(--accent-red)]"
                  href="https://x.com/ares_ai"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  X (Twitter)
                </a>
              </li>
            </ul>
          </div>

          <div>
            <h3
              className="text-sm font-semibold mb-3"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('supporters', language)}
            </h3>
            <ul className="space-y-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <li>
                <a
                  className="hover:text-[var(--accent-red)]"
                  href="https://www.asterdex.com/en/referral/fdfc0e"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Aster DEX
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[var(--accent-red)]"
                  href="https://www.maxweb.red/join?ref=AresAI"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Binance
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[var(--accent-red)]"
                  href="https://hyperliquid.xyz/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Hyperliquid
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[var(--accent-red)]"
                  href="https://amber.ac/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Amber.ac{' '}
                  <span className="opacity-70">
                    {t('strategicInvestment', language)}
                  </span>
                </a>
              </li>
            </ul>
          </div>
        </div>

        {/* Bottom note (kept subtle) */}
        <div
          className="pt-6 mt-8 text-center text-xs"
          style={{
            color: 'var(--text-tertiary)',
            borderTop: '1px solid var(--panel-border)',
          }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
