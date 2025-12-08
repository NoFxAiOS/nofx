import { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import HeaderBar from '../components/HeaderBar'
import { Container } from '../components/Container'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { t } from '../i18n/translations'
import { OFFICIAL_LINKS } from '../constants/branding'

interface MainLayoutProps {
  children?: ReactNode
}

export default function MainLayout({ children }: MainLayoutProps) {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const location = useLocation()

  // 根据路径自动判断当前页面
  const getCurrentPage = (): 'competition' | 'traders' | 'trader' | 'faq' => {
    if (location.pathname === '/faq') return 'faq'
    if (location.pathname === '/traders') return 'traders'
    if (location.pathname === '/dashboard') return 'trader'
    if (location.pathname === '/competition') return 'competition'
    return 'competition' // 默认
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={getCurrentPage()}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={() => {
          // React Router handles navigation now
        }}
      />

      {/* Main Content */}
      <Container as="main" className="py-6 pt-24">
        {children || <Outlet />}
      </Container>

      {/* Footer */}
      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <Container
          className="py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
          <div className="mt-4 flex items-center justify-center gap-3 flex-wrap">
            {/* GitHub */}
            <a
              href={OFFICIAL_LINKS.github}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
              style={{
                background: '#1E2329',
                color: '#848E9C',
                border: '1px solid #2B3139',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#2B3139'
                e.currentTarget.style.color = '#EAECEF'
                e.currentTarget.style.borderColor = '#F0B90B'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = '#1E2329'
                e.currentTarget.style.color = '#848E9C'
                e.currentTarget.style.borderColor = '#2B3139'
              }}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 16 16"
                fill="currentColor"
              >
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
              </svg>
              GitHub
            </a>
            {/* Twitter/X */}
            <a
              href={OFFICIAL_LINKS.twitter}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
              style={{
                background: '#1E2329',
                color: '#848E9C',
                border: '1px solid #2B3139',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#2B3139'
                e.currentTarget.style.color = '#EAECEF'
                e.currentTarget.style.borderColor = '#1DA1F2'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = '#1E2329'
                e.currentTarget.style.color = '#848E9C'
                e.currentTarget.style.borderColor = '#2B3139'
              }}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
              </svg>
              Twitter
            </a>
            {/* Telegram */}
            <a
              href={OFFICIAL_LINKS.telegram}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
              style={{
                background: '#1E2329',
                color: '#848E9C',
                border: '1px solid #2B3139',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#2B3139'
                e.currentTarget.style.color = '#EAECEF'
                e.currentTarget.style.borderColor = '#0088cc'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = '#1E2329'
                e.currentTarget.style.color = '#848E9C'
                e.currentTarget.style.borderColor = '#2B3139'
              }}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
              </svg>
              Telegram
            </a>
          </div>
        </Container>
      </footer>
    </div>
  )
}
