import { useState, useEffect, useRef } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Menu, X, ChevronDown } from 'lucide-react'
import { t, type Language } from '../i18n/translations'
import { Container } from './Container'
import { useSystemConfig } from '../hooks/useSystemConfig'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'backtest'
  | 'strategy'
  | 'faq'
  | 'login'
  | 'register'

interface HeaderBarProps {
  onLoginClick?: () => void
  isLoggedIn?: boolean
  isHomePage?: boolean
  currentPage?: Page
  language?: Language
  onLanguageChange?: (lang: Language) => void
  user?: { email: string } | null
  onLogout?: () => void
  onPageChange?: (page: Page) => void
}

export default function HeaderBar({
  isLoggedIn = false,
  isHomePage = false,
  currentPage,
  language = 'zh' as Language,
  onLanguageChange,
  user,
  onLogout,
  onPageChange,
}: HeaderBarProps) {
  const navigate = useNavigate()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [languageDropdownOpen, setLanguageDropdownOpen] = useState(false)
  const [userDropdownOpen, setUserDropdownOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const userDropdownRef = useRef<HTMLDivElement>(null)
  const { config: systemConfig } = useSystemConfig()
  const registrationEnabled = systemConfig?.registration_enabled !== false

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setLanguageDropdownOpen(false)
      }
      if (
        userDropdownRef.current &&
        !userDropdownRef.current.contains(event.target as Node)
      ) {
        setUserDropdownOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [])

  return (
    <nav className="fixed top-0 w-full z-50 header-bar">
      <Container className="flex items-center justify-between h-16">
        {/* Logo */}
        <Link
          to="/"
          className="flex items-center gap-3 hover:opacity-80 transition-opacity cursor-pointer"
        >
          <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-8 h-8" />
          <span
            className="text-xl font-bold"
            style={{ color: 'var(--brand-yellow)' }}
          >
            NOFX
          </span>
          <span
            className="text-sm hidden sm:block"
            style={{ color: 'var(--text-secondary)' }}
          >
            Agentic Trading OS
          </span>
        </Link>

        {/* Desktop Menu */}
        <div className="hidden md:flex items-center justify-between flex-1 ml-8">
          {/* Left Side - Navigation Tabs */}
          <div className="flex items-center gap-4">
            {isLoggedIn ? (
              // Main app navigation when logged in
              <>
                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('competition')
                    }
                    navigate('/competition')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'competition'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'competition' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('realtimeNav', language)}
                </button>

                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('traders')
                    }
                    navigate('/traders')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'traders'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'traders') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'traders') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'traders' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('configNav', language)}
                </button>

                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('trader')
                    }
                    navigate('/dashboard')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'trader'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'trader') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'trader') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'trader' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('dashboardNav', language)}
                </button>

                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('strategy')
                    }
                    navigate('/strategy')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'strategy'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'strategy') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'strategy') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {currentPage === 'strategy' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('strategyNav', language)}
                </button>

                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('backtest')
                    }
                    navigate('/backtest')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'backtest'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'backtest') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'backtest') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {currentPage === 'backtest' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  Backtest
                </button>

                <button
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('faq')
                    }
                    navigate('/faq')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'faq'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'faq') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'faq') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'faq' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('faqNav', language)}
                </button>
              </>
            ) : (
              // Landing page navigation when not logged in
              <>
                <a
                  href="/competition"
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'competition'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'competition' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('realtimeNav', language)}
                </a>

                <a
                  href="/faq"
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'faq'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'faq') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'faq') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  {currentPage === 'faq' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('faqNav', language)}
                </a>
              </>
            )}
          </div>

          {/* Right Side - Original Navigation Items and Login */}
          <div className="flex items-center gap-6">
            {/* Only show original navigation items on home page */}
            {isHomePage &&
              [
                { key: 'features', label: t('features', language) },
                { key: 'howItWorks', label: t('howItWorks', language) },
                { key: 'GitHub', label: 'GitHub' },
                { key: 'community', label: t('community', language) },
              ].map((item) => (
                <a
                  key={item.key}
                  href={
                    item.key === 'GitHub'
                      ? 'https://github.com/tinkle-community/nofx'
                      : item.key === 'community'
                        ? 'https://t.me/nofx_dev_community'
                        : `#${item.key === 'features' ? 'features' : 'how-it-works'}`
                  }
                  target={
                    item.key === 'GitHub' || item.key === 'community'
                      ? '_blank'
                      : undefined
                  }
                  rel={
                    item.key === 'GitHub' || item.key === 'community'
                      ? 'noopener noreferrer'
                      : undefined
                  }
                  className="text-sm transition-colors relative group"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {item.label}
                  <span
                    className="absolute -bottom-1 left-0 w-0 h-0.5 group-hover:w-full transition-all duration-300"
                    style={{ background: 'var(--brand-yellow)' }}
                  />
                </a>
              ))}

            {/* User Info and Actions */}
            {isLoggedIn && user ? (
              <div className="flex items-center gap-3">
                {/* User Info with Dropdown */}
                <div className="relative" ref={userDropdownRef}>
                  <button
                    onClick={() => setUserDropdownOpen(!userDropdownOpen)}
                    className="flex items-center gap-2 px-3 py-2 rounded transition-colors"
                    style={{
                      background: 'var(--panel-bg)',
                      border: '1px solid var(--panel-border)',
                    }}
                    onMouseEnter={(e) =>
                      (e.currentTarget.style.background =
                        'rgba(255, 255, 255, 0.05)')
                    }
                    onMouseLeave={(e) =>
                      (e.currentTarget.style.background = 'var(--panel-bg)')
                    }
                  >
                    <div
                      className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold"
                      style={{
                        background: 'var(--brand-yellow)',
                        color: 'var(--brand-black)',
                      }}
                    >
                      {user.email[0].toUpperCase()}
                    </div>
                    <span
                      className="text-sm"
                      style={{ color: 'var(--brand-light-gray)' }}
                    >
                      {user.email}
                    </span>
                    <ChevronDown
                      className="w-4 h-4"
                      style={{ color: 'var(--brand-light-gray)' }}
                    />
                  </button>

                  {userDropdownOpen && (
                    <div
                      className="absolute right-0 top-full mt-2 w-48 rounded-lg shadow-lg overflow-hidden z-50"
                      style={{
                        background: 'var(--brand-dark-gray)',
                        border: '1px solid var(--panel-border)',
                      }}
                    >
                      <div
                        className="px-3 py-2 border-b"
                        style={{ borderColor: 'var(--panel-border)' }}
                      >
                        <div
                          className="text-xs"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {t('loggedInAs', language)}
                        </div>
                        <div
                          className="text-sm font-medium"
                          style={{ color: 'var(--brand-light-gray)' }}
                        >
                          {user.email}
                        </div>
                      </div>
                      {onLogout && (
                        <button
                          onClick={() => {
                            onLogout()
                            setUserDropdownOpen(false)
                          }}
                          className="w-full px-3 py-2 text-sm font-semibold transition-colors hover:opacity-80 text-center"
                          style={{
                            background: 'var(--binance-red-bg)',
                            color: 'var(--binance-red)',
                          }}
                        >
                          {t('exitLogin', language)}
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ) : (
              /* Show login/register buttons when not logged in and not on login/register pages */
              currentPage !== 'login' &&
              currentPage !== 'register' && (
                <div className="flex items-center gap-3">
                  <a
                    href="/login"
                    className="px-3 py-2 text-sm font-medium transition-colors rounded"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('signIn', language)}
                  </a>
                  {registrationEnabled && (
                    <a
                      href="/register"
                      className="px-4 py-2 rounded font-semibold text-sm transition-colors hover:opacity-90"
                      style={{
                        background: 'var(--brand-yellow)',
                        color: 'var(--brand-black)',
                      }}
                    >
                      {t('signUp', language)}
                    </a>
                  )}
                </div>
              )
            )}

            {/* Language Toggle - Always at the rightmost */}
            <div className="relative" ref={dropdownRef}>
              <button
                onClick={() => setLanguageDropdownOpen(!languageDropdownOpen)}
                className="flex items-center gap-2 px-3 py-2 rounded transition-colors"
                style={{ color: 'var(--brand-light-gray)' }}
                onMouseEnter={(e) =>
                  (e.currentTarget.style.background =
                    'rgba(255, 255, 255, 0.05)')
                }
                onMouseLeave={(e) =>
                  (e.currentTarget.style.background = 'transparent')
                }
              >
                <span className="text-lg">
                  {language === 'zh' ? '🇨🇳' : '🇺🇸'}
                </span>
                <ChevronDown className="w-4 h-4" />
              </button>

              {languageDropdownOpen && (
                <div
                  className="absolute right-0 top-full mt-2 w-32 rounded-lg shadow-lg overflow-hidden z-50"
                  style={{
                    background: 'var(--brand-dark-gray)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <button
                    onClick={() => {
                      onLanguageChange?.('zh')
                      setLanguageDropdownOpen(false)
                    }}
                    className={`w-full flex items-center gap-2 px-3 py-2 transition-colors ${
                      language === 'zh' ? '' : 'hover:opacity-80'
                    }`}
                    style={{
                      color: 'var(--brand-light-gray)',
                      background:
                        language === 'zh'
                          ? 'rgba(240, 185, 11, 0.1)'
                          : 'transparent',
                    }}
                  >
                    <span className="text-base">🇨🇳</span>
                    <span className="text-sm">中文</span>
                  </button>
                  <button
                    onClick={() => {
                      onLanguageChange?.('en')
                      setLanguageDropdownOpen(false)
                    }}
                    className={`w-full flex items-center gap-2 px-3 py-2 transition-colors ${
                      language === 'en' ? '' : 'hover:opacity-80'
                    }`}
                    style={{
                      color: 'var(--brand-light-gray)',
                      background:
                        language === 'en'
                          ? 'rgba(240, 185, 11, 0.1)'
                          : 'transparent',
                    }}
                  >
                    <span className="text-base">🇺🇸</span>
                    <span className="text-sm">English</span>
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Mobile Menu Button */}
        <motion.button
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          className="md:hidden"
          style={{ color: 'var(--brand-light-gray)' }}
          whileTap={{ scale: 0.9 }}
        >
          {mobileMenuOpen ? (
            <X className="w-6 h-6" />
          ) : (
            <Menu className="w-6 h-6" />
          )}
        </motion.button>
      </Container>

      {/* Mobile Menu */}
      <motion.div
        initial={false}
        animate={
          mobileMenuOpen
            ? { height: 'auto', opacity: 1 }
            : { height: 0, opacity: 0 }
        }
        transition={{ duration: 0.3 }}
        className="md:hidden overflow-hidden"
        style={{
          background: 'var(--brand-dark-gray)',
          borderTop: '1px solid rgba(240, 185, 11, 0.1)',
        }}
      >
        <div className="px-4 py-4 space-y-3">
          {/* New Navigation Tabs */}
          {isLoggedIn ? (
            <button
              onClick={() => {
                console.log(
                  '移动端 实时 button clicked, onPageChange:',
                  onPageChange
                )
                onPageChange?.('competition')
                setMobileMenuOpen(false)
              }}
              className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
              style={{
                color:
                  currentPage === 'competition'
                    ? 'var(--brand-yellow)'
                    : 'var(--brand-light-gray)',
                padding: '12px 16px',
                borderRadius: '8px',
                position: 'relative',
                width: '100%',
                textAlign: 'left',
              }}
            >
              {/* Background for selected state */}
              {currentPage === 'competition' && (
                <span
                  className="absolute inset-0 rounded-lg"
                  style={{
                    background: 'rgba(240, 185, 11, 0.15)',
                    zIndex: -1,
                  }}
                />
              )}

              {t('realtimeNav', language)}
            </button>
          ) : (
            <a
              href="/competition"
              className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
              style={{
                color:
                  currentPage === 'competition'
                    ? 'var(--brand-yellow)'
                    : 'var(--brand-light-gray)',
                padding: '12px 16px',
                borderRadius: '8px',
                position: 'relative',
              }}
            >
              {/* Background for selected state */}
              {currentPage === 'competition' && (
                <span
                  className="absolute inset-0 rounded-lg"
                  style={{
                    background: 'rgba(240, 185, 11, 0.15)',
                    zIndex: -1,
                  }}
                />
              )}

              {t('realtimeNav', language)}
            </a>
          )}
          {/* Only show 配置 and 看板 when logged in */}
          {isLoggedIn && (
            <>
              <button
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('traders')
                  }
                  navigate('/traders')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'traders'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                {currentPage === 'traders' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                {t('configNav', language)}
              </button>
              <button
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('trader')
                  }
                  navigate('/dashboard')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'trader'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                {currentPage === 'trader' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                {t('dashboardNav', language)}
              </button>
              <button
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('strategy')
                  }
                  navigate('/strategy')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'strategy'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                {currentPage === 'strategy' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                {t('strategyNav', language)}
              </button>
              <button
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('backtest')
                  }
                  navigate('/backtest')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'backtest'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                {currentPage === 'backtest' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                Backtest
              </button>
              <button
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('faq')
                  }
                  navigate('/faq')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'faq'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                {currentPage === 'faq' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                {t('faqNav', language)}
              </button>
            </>
          )}

          {/* Original Navigation Items - Only on home page */}
          {isHomePage &&
            [
              { key: 'features', label: t('features', language) },
              { key: 'howItWorks', label: t('howItWorks', language) },
              { key: 'GitHub', label: 'GitHub' },
              { key: 'community', label: t('community', language) },
            ].map((item) => (
              <a
                key={item.key}
                href={
                  item.key === 'GitHub'
                    ? 'https://github.com/tinkle-community/nofx'
                    : item.key === 'community'
                      ? 'https://t.me/nofx_dev_community'
                      : `#${item.key === 'features' ? 'features' : 'how-it-works'}`
                }
                target={
                  item.key === 'GitHub' || item.key === 'community'
                    ? '_blank'
                    : undefined
                }
                rel={
                  item.key === 'GitHub' || item.key === 'community'
                    ? 'noopener noreferrer'
                    : undefined
                }
                className="block text-sm py-2"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {item.label}
              </a>
            ))}

          {/* Language Toggle */}
          <div className="py-2">
            <div className="flex items-center gap-2 mb-2">
              <span
                className="text-xs"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {t('language', language)}:
              </span>
            </div>
            <div className="space-y-1">
              <button
                onClick={() => {
                  onLanguageChange?.('zh')
                  setMobileMenuOpen(false)
                }}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded transition-colors ${
                  language === 'zh'
                    ? 'bg-yellow-500 text-black'
                    : 'text-gray-400 hover:text-white'
                }`}
              >
                <span className="text-lg">🇨🇳</span>
                <span className="text-sm">中文</span>
              </button>
              <button
                onClick={() => {
                  onLanguageChange?.('en')
                  setMobileMenuOpen(false)
                }}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded transition-colors ${
                  language === 'en'
                    ? 'bg-yellow-500 text-black'
                    : 'text-gray-400 hover:text-white'
                }`}
              >
                <span className="text-lg">🇺🇸</span>
                <span className="text-sm">English</span>
              </button>
            </div>
          </div>

          {/* User info and logout for mobile when logged in */}
          {isLoggedIn && user && (
            <div
              className="mt-4 pt-4"
              style={{ borderTop: '1px solid var(--panel-border)' }}
            >
              <div
                className="flex items-center gap-2 px-3 py-2 mb-2 rounded"
                style={{ background: 'var(--panel-bg)' }}
              >
                <div
                  className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold"
                  style={{
                    background: 'var(--brand-yellow)',
                    color: 'var(--brand-black)',
                  }}
                >
                  {user.email[0].toUpperCase()}
                </div>
                <div>
                  <div
                    className="text-xs"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('loggedInAs', language)}
                  </div>
                  <div
                    className="text-sm"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {user.email}
                  </div>
                </div>
              </div>
              {onLogout && (
                <button
                  onClick={() => {
                    onLogout()
                    setMobileMenuOpen(false)
                  }}
                  className="w-full px-4 py-2 rounded text-sm font-semibold transition-colors text-center"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {t('exitLogin', language)}
                </button>
              )}
            </div>
          )}

          {/* Show login/register buttons when not logged in and not on login/register pages */}
          {!isLoggedIn &&
            currentPage !== 'login' &&
            currentPage !== 'register' && (
              <div className="space-y-2 mt-2">
                <a
                  href="/login"
                  className="block w-full px-4 py-2 rounded text-sm font-medium text-center transition-colors"
                  style={{
                    color: 'var(--brand-light-gray)',
                    border: '1px solid var(--brand-light-gray)',
                  }}
                  onClick={() => setMobileMenuOpen(false)}
                >
                  {t('signIn', language)}
                </a>
                {registrationEnabled && (
                  <a
                    href="/register"
                    className="block w-full px-4 py-2 rounded font-semibold text-sm text-center transition-colors"
                    style={{
                      background: 'var(--brand-yellow)',
                      color: 'var(--brand-black)',
                    }}
                    onClick={() => setMobileMenuOpen(false)}
                  >
                    {t('signUp', language)}
                  </a>
                )}
              </div>
            )}
        </div>
      </motion.div>
    </nav>
  )
}
