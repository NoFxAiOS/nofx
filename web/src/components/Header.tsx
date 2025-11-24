import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { Container } from './Container'

interface HeaderProps {
  simple?: boolean // For login/register pages
}

export function Header({ simple = false }: HeaderProps) {
  const { language } = useLanguage()

  return (
    <header className="glass sticky top-0 z-50 backdrop-blur-xl">
      <Container className="py-4">
        <div className="flex items-center justify-between">
          {/* Left - Logo and Title */}
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center">
              <img src="/icons/ares.svg" alt="Ares Logo" className="w-8 h-8" />
            </div>
            <div>
              <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>
                {t('appTitle', language)}
              </h1>
              {!simple && (
                <p className="text-xs mono" style={{ color: 'var(--text-secondary)' }}>
                  {t('subtitle', language)}
                </p>
              )}
            </div>
          </div>
        </div>
      </Container>
    </header>
  )
}
