import { ReactNode } from 'react'
import { Outlet, Link } from 'react-router-dom'
import { Container } from '../components/Container'
import { useLanguage } from '../contexts/LanguageContext'

interface AuthLayoutProps {
  children?: ReactNode
}

export default function AuthLayout({ children }: AuthLayoutProps) {
  const { language, setLanguage } = useLanguage()

  return (
    <div className="min-h-screen" style={{ background: '#0B0E11' }}>
      {/* Simple Header with Logo and Language Selector */}
      <nav
        className="fixed top-0 w-full z-50"
        style={{
          background: 'rgba(11, 14, 17, 0.95)',
          backdropFilter: 'blur(10px)',
        }}
      >
        <Container className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link
            to="/"
            className="flex items-center gap-3 hover:opacity-80 transition-opacity"
          >
            <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-8 h-8" />
            <span className="text-xl font-bold" style={{ color: '#F0B90B' }}>
              NOFX
            </span>
          </Link>

          {/* Language Selector */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setLanguage(language === 'zh' ? 'en' : 'zh')}
              className="px-3 py-1.5 rounded text-sm font-medium transition-colors"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              {language === 'zh' ? 'English' : '中文'}
            </button>
          </div>
        </Container>
      </nav>

      {/* Content with top padding to avoid overlap with fixed header */}
      <div className="pt-16">{children || <Outlet />}</div>
    </div>
  )
}
