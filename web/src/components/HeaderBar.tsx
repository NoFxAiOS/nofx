import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Menu, X, ChevronDown } from 'lucide-react'
import { t, type Language } from '../i18n/translations'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { OFFICIAL_LINKS } from '../constants/branding'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'backtest'
  | 'strategy'
  | 'debate'
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
      <div className="flex items-center justify-between h-16 px-4 sm:px-6 max-w-[1920px] mx-auto">
        {/* Logo - Always go to home page */}
        <div
          onClick={() => {
            window.location.href = '/'
          }}
          className="flex items-center gap-2 hover:opacity-80 transition-opacity cursor-pointer"
        >
          <img src="/icons/nofx.svg" alt="NOFX Logo" className="w-7 h-7" />
          <span
            className="text-lg font-bold"
            style={{ color: 'var(--brand-yellow)' }}
          >
            NOFX
          </span>
        </div>

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
                      onPageChange('debate')
                    }
                    navigate('/debate')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500"
                  style={{
                    color:
                      currentPage === 'debate'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'debate') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'debate') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {currentPage === 'debate' && (
                    <span
                      className="absolute inset-0 rounded-lg"
                      style={{
                        background: 'rgba(240, 185, 11, 0.15)',
                        zIndex: -1,
                      }}
                    />
                  )}

                  {t('debateNav', language)}
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

          {/* Right Side - Social Links and User Actions */}
          <div className="flex items-center gap-4">
            {/* Social Links - Always visible */}
            <div className="flex items-center gap-1">
              {/* GitHub */}
              <a
                href={OFFICIAL_LINKS.github}
                target="_blank"
                rel="noopener noreferrer"
                className="p-2 rounded-lg transition-all hover:scale-110"
                style={{ color: '#848E9C' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.background = 'rgba(255, 255, 255, 0.05)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.background = 'transparent'
                }}
                title="GitHub"
              >
                <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
              </a>
              {/* Twitter/X */}
              <a
                href={OFFICIAL_LINKS.twitter}
                target="_blank"
                rel="noopener noreferrer"
                className="p-2 rounded-lg transition-all hover:scale-110"
                style={{ color: '#848E9C' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = '#1DA1F2'
                  e.currentTarget.style.background = 'rgba(29, 161, 242, 0.1)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.background = 'transparent'
                }}
                title="Twitter"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
              </a>
              {/* Telegram */}
              <a
                href={OFFICIAL_LINKS.telegram}
                target="_blank"
                rel="noopener noreferrer"
                className="p-2 rounded-lg transition-all hover:scale-110"
                style={{ color: '#848E9C' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = '#0088cc'
                  e.currentTarget.style.background = 'rgba(0, 136, 204, 0.1)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.background = 'transparent'
                }}
                title="Telegram"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
                </svg>
              </a>
            </div>

            {/* Divider */}
            <div className="h-5 w-px" style={{ background: '#2B3139' }} />

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
      </div>

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
                    onPageChange('debate')
                  }
                  navigate('/debate')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 hover:text-yellow-500"
                style={{
                  color:
                    currentPage === 'debate'
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
                {currentPage === 'debate' && (
                  <span
                    className="absolute inset-0 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.15)',
                      zIndex: -1,
                    }}
                  />
                )}

                {t('debateNav', language)}
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
            ].map((item) => (
              <a
                key={item.key}
                href={`#${item.key === 'features' ? 'features' : 'how-it-works'}`}
                className="block text-sm py-2"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {item.label}
              </a>
            ))}

          {/* Social Links - Mobile */}
          <div className="py-3 flex items-center gap-3" style={{ borderTop: '1px solid #2B3139' }}>
            <a
              href={OFFICIAL_LINKS.github}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg"
              style={{ color: '#848E9C', background: 'rgba(255, 255, 255, 0.05)' }}
            >
              <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
              </svg>
            </a>
            <a
              href={OFFICIAL_LINKS.twitter}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg"
              style={{ color: '#848E9C', background: 'rgba(255, 255, 255, 0.05)' }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
              </svg>
            </a>
            <a
              href={OFFICIAL_LINKS.telegram}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg"
              style={{ color: '#848E9C', background: 'rgba(255, 255, 255, 0.05)' }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
              </svg>
            </a>
          </div>

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
