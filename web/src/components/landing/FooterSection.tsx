import { Github, Send } from 'lucide-react'
import { t, Language } from '../../i18n/translations'
import { OFFICIAL_LINKS } from '../../constants/branding'

interface FooterSectionProps {
  language: Language
}

const XIcon = () => (
  <svg viewBox="0 0 24 24" className="w-4 h-4" fill="currentColor">
    <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
  </svg>
)

export default function FooterSection({ language }: FooterSectionProps) {
  const navLinks = [
    { name: 'Partners', href: '#' },
    { name: 'Docs', href: 'https://github.com/NoFxAiOS/nofx/blob/main/README.md' },
    { name: 'GitHub', href: OFFICIAL_LINKS.github },
    { name: 'Community', href: OFFICIAL_LINKS.telegram },
  ]

  const socialLinks = [
    { name: 'GitHub', href: OFFICIAL_LINKS.github, icon: Github },
    { name: 'X', href: OFFICIAL_LINKS.twitter, icon: XIcon },
    { name: 'Telegram', href: OFFICIAL_LINKS.telegram, icon: Send },
  ]

  return (
    <footer className="w-full bg-black border-t border-zinc-800">
      <div className="max-w-7xl mx-auto px-6 py-12">

        {/* Main Footer Content */}
        <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-start">

          {/* Left - Brand & CTA */}
          <div className="md:col-span-3">
            <p className="text-nofx-gold text-xs font-bold uppercase tracking-wider mb-2">
              Start Building
            </p>
            <h3 className="text-4xl md:text-5xl font-black text-nofx-gold tracking-tight">
              NOFX
            </h3>
          </div>

          {/* Center - Navigation Links */}
          <div className="md:col-span-5">
            <div className="grid grid-cols-2 gap-4">
              {navLinks.map((link) => (
                <a
                  key={link.name}
                  href={link.href}
                  target={link.href.startsWith('http') ? '_blank' : undefined}
                  rel={link.href.startsWith('http') ? 'noopener noreferrer' : undefined}
                  className="text-zinc-500 hover:text-nofx-gold transition-colors text-sm"
                >
                  {link.name}
                </a>
              ))}
            </div>
          </div>

          {/* Right - Social Links */}
          <div className="md:col-span-4 flex md:justify-end">
            <div className="flex items-center gap-4">
              {socialLinks.map((link) => {
                const Icon = link.icon
                return (
                  <a
                    key={link.name}
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-zinc-500 hover:text-nofx-gold transition-colors"
                    title={link.name}
                  >
                    <Icon />
                  </a>
                )
              })}
            </div>
          </div>

        </div>

        {/* Bottom Section */}
        <div className="mt-12 pt-6 border-t border-zinc-800/50 text-center">
          <p className="text-zinc-600 text-xs">
            {t('footerTitle', language)}
          </p>
          <p className="text-zinc-700 text-xs mt-1">
            {t('footerWarning', language)}
          </p>
        </div>

      </div>
    </footer>
  )
}
