import { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import HeaderBar from '../components/HeaderBar'
import { Container } from '../components/Container'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { t } from '../i18n/translations'

interface MainLayoutProps {
  children?: ReactNode
}

export default function MainLayout({ children }: MainLayoutProps) {
  const { language } = useLanguage()
  const { user, logout } = useAuth()
  const location = useLocation()

  // 根据路径自动判断当前页面
  const getCurrentPage = (): 'competition' | 'traders' | 'trader' => {
    if (location.pathname === '/traders') return 'traders'
    if (location.pathname === '/dashboard') return 'trader'
    if (location.pathname === '/competition') return 'competition'
    return 'competition' // 默认
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: 'var(--background)', color: 'var(--text-primary)' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={getCurrentPage()}
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
        style={{ borderTop: '1px solid var(--panel-border)', background: 'var(--panel-bg)' }}
      >
        <Container
          className="py-6 text-center text-sm"
          style={{ color: 'var(--text-secondary)' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </Container>
      </footer>
    </div>
  )
}
