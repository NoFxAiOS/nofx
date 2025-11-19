import { t, Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  return (
    <footer
      style={{
        borderTop: '1px solid var(--panel-border)',
        background: 'var(--brand-dark-gray)',
      }}
    >
      <div className="max-w-[1200px] mx-auto px-6 py-10">
        {/* Brand */}
        <div className="flex items-center gap-3 mb-8">
          <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-8 h-8" />
          <div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              NOFX
            </div>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {t('futureStandardAI', language)}
            </div>
          </div>
        </div>

        {/* Multi-link columns */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-3 gap-8">
          <div>
            <h3
              className="text-sm font-semibold mb-3"
              style={{ color: '#EAECEF' }}
            >
              {t('links', language)}
            </h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://github.com/tinkle-community/nofx"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  GitHub
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://t.me/nofx_dev_community"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Telegram
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://x.com/nofx_official"
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
              style={{ color: '#EAECEF' }}
            >
              {t('resources', language)}
            </h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://github.com/tinkle-community/nofx/blob/main/README.md"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t('documentation', language)}
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://github.com/tinkle-community/nofx/issues"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Issues
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://github.com/tinkle-community/nofx/pulls"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Pull Requests
                </a>
              </li>
            </ul>
          </div>

          <div>
            <h3
              className="text-sm font-semibold mb-3"
              style={{ color: '#EAECEF' }}
            >
              {t('supporters', language)}
            </h3>
            <ul className="space-y-2 text-sm" style={{ color: '#848E9C' }}>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://www.asterdex.com/en/referral/fdfc0e"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Aster DEX
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://www.maxweb.red/join?ref=NOFXAI"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Binance
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
                  href="https://hyperliquid.xyz/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Hyperliquid
                </a>
              </li>
              <li>
                <a
                  className="hover:text-[#F0B90B]"
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
