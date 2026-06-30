import { Github, Send, ExternalLink } from 'lucide-react'
import { t, Language } from '../../i18n/translations'
import { OFFICIAL_LINKS } from '../../constants/branding'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({ language }: FooterSectionProps) {
  const links = {
    social: [
      { name: 'GitHub', href: OFFICIAL_LINKS.github, icon: Github },
      {
        name: 'X (Twitter)',
        href: OFFICIAL_LINKS.twitter,
        icon: () => (
          <svg viewBox="0 0 24 24" className="w-4 h-4" fill="currentColor">
            <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
          </svg>
        ),
      },
      { name: 'Telegram', href: OFFICIAL_LINKS.telegram, icon: Send },
    ],
    resources: [
      {
        name: language === 'zh' ? 'Documentation' : 'Documentation',
        href: 'https://github.com/NoFxAiOS/nofx/blob/main/README.md',
      },
      { name: 'Issues', href: 'https://github.com/NoFxAiOS/nofx/issues' },
      { name: 'Pull Requests', href: 'https://github.com/NoFxAiOS/nofx/pulls' },
    ],
    supporters: [
      { name: 'Binance', href: 'https://www.binance.com/join?ref=NOFXENG' },
      { name: 'Bybit', href: 'https://partner.bybit.com/b/83856' },
      { name: 'OKX', href: 'https://www.okx.com/join/1865360' },
      { name: 'Bitget', href: 'https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172' },
      { name: 'Gate.io', href: 'https://www.gatenode.xyz/share/VQBGUAxY' },
      { name: 'KuCoin', href: 'https://www.kucoin.com/r/broker/CXEV7XKK' },
      { name: 'Hyperliquid', href: 'https://app.hyperliquid.xyz/join/AITRADING' },
      { name: 'Aster DEX', href: 'https://www.asterdex.com/en/referral/fdfc0e' },
      { name: 'Lighter', href: 'https://app.lighter.xyz/?referral=68151432' },
    ],
  }

  return (
    <footer style={{ background: '#F1ECE2', borderTop: '1px solid rgba(26, 24, 19, 0.14)' }}>
      <div className="max-w-6xl mx-auto px-4 py-8 md:py-12">
        {/* Top Section */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8 md:gap-10 mb-8 md:mb-12">
          {/* Brand */}
          <div className="md:col-span-1">
            <div className="flex items-center gap-3 mb-4">
              <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-8 h-8" />
              <span className="text-xl font-bold" style={{ color: '#1A1813' }}>
                NOFX
              </span>
            </div>
            <p className="text-sm mb-6" style={{ color: '#8A8478' }}>
              {t('futureStandardAI', language)}
            </p>
            {/* Social Icons */}
            <div className="flex items-center gap-3">
              {links.social.map((link) => (
                <a
                  key={link.name}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="w-9 h-9 rounded-lg flex items-center justify-center transition-all hover:scale-110"
                  style={{
                    background: '#E8E2D5',
                    color: '#8A8478',
                  }}
                  title={link.name}
                >
                  <link.icon className="w-4 h-4" />
                </a>
              ))}
            </div>
          </div>

          {/* Links */}
          <div>
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#1A1813' }}>
              {t('links', language)}
            </h4>
            <ul className="space-y-3">
              {links.social.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#E0483B]"
                    style={{ color: '#8A8478' }}
                  >
                    {link.name}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Resources */}
          <div>
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#1A1813' }}>
              {t('resources', language)}
            </h4>
            <ul className="space-y-3">
              {links.resources.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#E0483B] inline-flex items-center gap-1"
                    style={{ color: '#8A8478' }}
                  >
                    {link.name}
                    <ExternalLink className="w-3 h-3 opacity-50" />
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Supporters */}
          <div>
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#1A1813' }}>
              {t('supporters', language)}
            </h4>
            <div className="flex flex-wrap gap-2">
              {links.supporters.map((link) => (
                <a
                  key={link.name}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs border border-[rgba(26,24,19,0.14)] bg-nofx-bg-lighter rounded px-3 py-1.5 transition-all hover:border-[#E0483B] hover:text-[#E0483B] hover:bg-[#E0483B]/10"
                  style={{ color: '#8A8478' }}
                >
                  {link.name}
                </a>
              ))}
            </div>
          </div>
        </div>

        {/* Bottom Section */}
        <div
          className="pt-6 text-center text-xs"
          style={{ color: '#8A8478', borderTop: '1px solid rgba(26, 24, 19, 0.14)' }}
        >
          <p className="mb-2">{t('footerTitle', language)}</p>
          <p style={{ color: '#B4B2A9' }}>{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
