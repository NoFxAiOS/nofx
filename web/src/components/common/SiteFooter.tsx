import { OFFICIAL_LINKS } from '../../constants/branding'
import { t, type Language } from '../../i18n/translations'
import { BrandLogo } from './BrandLogo'

export function SiteFooter({ language }: { language: Language }) {
  return (
    <footer className="nofx-site-footer">
      <div className="nofx-site-footer-inner">
        <BrandLogo />
        <div>
          <p>{t('footerTitle', language)}</p>
          <p>{t('footerWarning', language)}</p>
        </div>
        <div className="nofx-site-footer-links">
          <a
            href={OFFICIAL_LINKS.github}
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub ↗
          </a>
          <a
            href={OFFICIAL_LINKS.twitter}
            target="_blank"
            rel="noopener noreferrer"
          >
            X / Twitter ↗
          </a>
          <a
            href={OFFICIAL_LINKS.telegram}
            target="_blank"
            rel="noopener noreferrer"
          >
            Telegram Community ↗
          </a>
        </div>
      </div>
    </footer>
  )
}
