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
        name: language === 'zh' ? '文档' : 'Documentation',
        href: 'https://github.com/NoFxAiOS/nofx/blob/main/README.md',
      },
      { name: 'Issues', href: 'https://github.com/NoFxAiOS/nofx/issues' },
      { name: 'Pull Requests', href: 'https://github.com/NoFxAiOS/nofx/pulls' },
    ],
    supporters: [
      { name: 'Aster DEX', href: 'https://www.asterdex.com/en/referral/fdfc0e' },
      { name: 'Binance', href: 'https://www.maxweb.red/join?ref=NOFXAI' },
      { name: 'Hyperliquid', href: 'https://hyperliquid.xyz/' },
    ],
  }

  return (
    <footer style={{ background: '#0B0E11', borderTop: '1px solid rgba(255, 255, 255, 0.06)' }}>
      <div className="max-w-6xl mx-auto px-4 py-12">
        {/* Top Section */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-10 mb-12">
          {/* Brand */}
          <div className="md:col-span-1">
            <div className="flex items-center gap-3 mb-4">
              <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-8 h-8" />
              <span className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                NOFX
              </span>
            </div>
            <p className="text-sm mb-6" style={{ color: '#5E6673' }}>
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
                    background: 'rgba(255, 255, 255, 0.05)',
                    color: '#848E9C',
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
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#EAECEF' }}>
              {t('links', language)}
            </h4>
            <ul className="space-y-3">
              {links.social.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#F0B90B]"
                    style={{ color: '#5E6673' }}
                  >
                    {link.name}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Resources */}
          <div>
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#EAECEF' }}>
              {t('resources', language)}
            </h4>
            <ul className="space-y-3">
              {links.resources.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#F0B90B] inline-flex items-center gap-1"
                    style={{ color: '#5E6673' }}
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
            <h4 className="text-sm font-semibold mb-4" style={{ color: '#EAECEF' }}>
              {t('supporters', language)}
            </h4>
            <ul className="space-y-3">
              {links.supporters.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm transition-colors hover:text-[#F0B90B] inline-flex items-center gap-2"
                    style={{ color: '#5E6673' }}
                  >
                    {link.name}
                    {link.badge && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded"
                        style={{
                          background: 'rgba(240, 185, 11, 0.1)',
                          color: '#F0B90B',
                        }}
                      >
                        {link.badge}
                      </span>
                    )}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Bottom Section */}
        <div
          className="pt-6 text-center text-xs"
          style={{ color: '#5E6673', borderTop: '1px solid rgba(255, 255, 255, 0.06)' }}
        >
          <p className="mb-2">{t('footerTitle', language)}</p>
          <p style={{ color: '#3C4249' }}>{t('footerWarning', language)}</p>
        </div>
      </div>
    </footer>
  )
}
