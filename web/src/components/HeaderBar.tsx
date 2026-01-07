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
  | 'strategy-market'
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
  onLoginRequired?: (featureName: string) => void
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
  onLoginRequired,
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
          <span className="text-lg font-bold text-nofx-gold">
            NOFX
          </span>
        </div>

        {/* Desktop Menu */}
        <div className="hidden md:flex items-center justify-between flex-1 ml-8">
          {/* Left Side - Navigation Tabs - Always show all tabs */}
          <div className="flex items-center gap-2">
            {/* Navigation tabs configuration */}
            {(() => {
              // Define all navigation tabs
              const navTabs: { page: Page; path: string; label: string; requiresAuth: boolean }[] = [
                { page: 'strategy-market', path: '/strategy-market', label: language === 'zh' ? '策略市场' : 'Market', requiresAuth: true },
                { page: 'traders', path: '/traders', label: t('configNav', language), requiresAuth: true },
                { page: 'trader', path: '/dashboard', label: t('dashboardNav', language), requiresAuth: true },
                { page: 'strategy', path: '/strategy', label: t('strategyNav', language), requiresAuth: true },
                { page: 'competition', path: '/competition', label: t('realtimeNav', language), requiresAuth: true },
                { page: 'debate', path: '/debate', label: t('debateNav', language), requiresAuth: true },
                { page: 'backtest', path: '/backtest', label: 'Backtest', requiresAuth: true },
                { page: 'faq', path: '/faq', label: t('faqNav', language), requiresAuth: false },
              ]

              const handleNavClick = (tab: typeof navTabs[0]) => {
                // If requires auth and not logged in, show login prompt
                if (tab.requiresAuth && !isLoggedIn) {
                  onLoginRequired?.(tab.label)
                  return
                }
                // Navigate normally
                if (onPageChange) {
                  onPageChange(tab.page)
                }
                navigate(tab.path)
              }

              return navTabs.map((tab) => (
                <button
                  key={tab.page}
                  onClick={() => handleNavClick(tab)}
                  className={`text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 px-3 py-2 rounded-lg
                    ${currentPage === tab.page ? 'text-nofx-gold' : 'text-nofx-text-muted hover:text-nofx-gold'}`}
                >
                  {currentPage === tab.page && (
                    <span
                      className="absolute inset-0 rounded-lg bg-nofx-gold/15 -z-10"
                    />
                  )}
                  {tab.label}
                </button>
              ))
            })()}
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
                className="p-2 rounded-lg transition-all hover:scale-110 text-nofx-text-muted hover:text-white hover:bg-white/5"
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
                className="p-2 rounded-lg transition-all hover:scale-110 text-nofx-text-muted hover:text-[#1DA1F2] hover:bg-[#1DA1F2]/10"
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
                className="p-2 rounded-lg transition-all hover:scale-110 text-nofx-text-muted hover:text-[#0088cc] hover:bg-[#0088cc]/10"
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
                    className="flex items-center gap-2 px-3 py-2 rounded transition-colors bg-nofx-bg-lighter border border-nofx-gold/20 hover:bg-white/5"
                  >
                    <div className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold bg-nofx-gold text-black">
                      {user.email[0].toUpperCase()}
                    </div>
                    <span className="text-sm text-nofx-text-muted">
                      {user.email}
                    </span>
                    <ChevronDown className="w-4 h-4 text-nofx-text-muted" />
                  </button>

                  {userDropdownOpen && (
                    <div className="absolute right-0 top-full mt-2 w-48 rounded-lg shadow-lg overflow-hidden z-50 bg-nofx-bg-lighter border border-nofx-gold/20">
                      <div className="px-3 py-2 border-b border-nofx-gold/20">
                        <div className="text-xs text-nofx-text-muted">
                          {t('loggedInAs', language)}
                        </div>
                        <div className="text-sm font-medium text-nofx-text-muted">
                          {user.email}
                        </div>
                      </div>
                      {onLogout && (
                        <button
                          onClick={() => {
                            onLogout()
                            setUserDropdownOpen(false)
                          }}
                          className="w-full px-3 py-2 text-sm font-semibold transition-colors hover:opacity-80 text-center bg-nofx-danger/20 text-nofx-danger"
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
                    className="px-3 py-2 text-sm font-medium transition-colors rounded text-nofx-text-muted hover:text-white"
                  >
                    {t('signIn', language)}
                  </a>
                  {registrationEnabled && (
                    <a
                      href="/register"
                      className="px-4 py-2 rounded font-semibold text-sm transition-colors hover:opacity-90 bg-nofx-gold text-black"
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
                className="flex items-center gap-2 px-3 py-2 rounded transition-colors text-nofx-text-muted hover:bg-white/5"
              >
                <span className="text-lg">
                  {language === 'zh' ? '🇨🇳' : '🇺🇸'}
                </span>
                <ChevronDown className="w-4 h-4" />
              </button>

              {languageDropdownOpen && (
                <div className="absolute right-0 top-full mt-2 w-32 rounded-lg shadow-lg overflow-hidden z-50 bg-nofx-bg-lighter border border-nofx-gold/20">
                  <button
                    onClick={() => {
                      onLanguageChange?.('zh')
                      setLanguageDropdownOpen(false)
                    }}
                    className={`w-full flex items-center gap-2 px-3 py-2 transition-colors text-nofx-text-muted hover:text-white
                      ${language === 'zh' ? 'bg-nofx-gold/10' : 'hover:bg-white/5'}`}
                  >
                    <span className="text-base">🇨🇳</span>
                    <span className="text-sm">中文</span>
                  </button>
                  <button
                    onClick={() => {
                      onLanguageChange?.('en')
                      setLanguageDropdownOpen(false)
                    }}
                    className={`w-full flex items-center gap-2 px-3 py-2 transition-colors text-nofx-text-muted hover:text-white
                      ${language === 'en' ? 'bg-nofx-gold/10' : 'hover:bg-white/5'}`}
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
          className="md:hidden text-nofx-text-muted hover:text-white"
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
        className="md:hidden overflow-hidden bg-nofx-bg-lighter border-t border-nofx-gold/10"
      >
        <div className="px-4 py-4 space-y-2">
          {/* Mobile Navigation Tabs - Show all tabs */}
          {(() => {
            const navTabs: { page: Page; path: string; label: string; requiresAuth: boolean }[] = [
              { page: 'strategy-market', path: '/strategy-market', label: language === 'zh' ? '策略市场' : 'Market', requiresAuth: true },
              { page: 'traders', path: '/traders', label: t('configNav', language), requiresAuth: true },
              { page: 'trader', path: '/dashboard', label: t('dashboardNav', language), requiresAuth: true },
              { page: 'strategy', path: '/strategy', label: t('strategyNav', language), requiresAuth: true },
              { page: 'competition', path: '/competition', label: t('realtimeNav', language), requiresAuth: true },
              { page: 'debate', path: '/debate', label: t('debateNav', language), requiresAuth: true },
              { page: 'backtest', path: '/backtest', label: 'Backtest', requiresAuth: true },
              { page: 'faq', path: '/faq', label: t('faqNav', language), requiresAuth: false },
            ]

            const handleMobileNavClick = (tab: typeof navTabs[0]) => {
              if (tab.requiresAuth && !isLoggedIn) {
                onLoginRequired?.(tab.label)
                setMobileMenuOpen(false)
                return
              }
              if (onPageChange) {
                onPageChange(tab.page)
              }
              navigate(tab.path)
              setMobileMenuOpen(false)
            }

            return navTabs.map((tab) => (
              <button
                key={tab.page}
                onClick={() => handleMobileNavClick(tab)}
                className={`block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-yellow-500 w-full text-left px-4 py-3 rounded-lg
                  ${currentPage === tab.page ? 'text-nofx-gold' : 'text-nofx-text-muted hover:text-white hover:bg-white/5'}`}
              >
                {currentPage === tab.page && (
                  <span
                    className="absolute inset-0 rounded-lg bg-nofx-gold/15 -z-10"
                  />
                )}
                {tab.label}
                {tab.requiresAuth && !isLoggedIn && (
                  <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-nofx-gold/20 text-nofx-gold">
                    {language === 'zh' ? '需登录' : 'Login'}
                  </span>
                )}
              </button>
            ))
          })()}

          {/* Original Navigation Items - Only on home page */}
          {isHomePage &&
            [
              { key: 'features', label: t('features', language) },
              { key: 'howItWorks', label: t('howItWorks', language) },
            ].map((item) => (
              <a
                key={item.key}
                href={`#${item.key === 'features' ? 'features' : 'how-it-works'}`}
                className="block text-sm py-2 text-nofx-text-muted hover:text-white"
              >
                {item.label}
              </a>
            ))}

          {/* Social Links - Mobile */}
          <div className="py-3 flex items-center gap-3 border-t border-nofx-gold/20">
            <a
              href={OFFICIAL_LINKS.github}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg text-nofx-text-muted bg-white/5 hover:text-white"
            >
              <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
              </svg>
            </a>
            <a
              href={OFFICIAL_LINKS.twitter}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg text-nofx-text-muted bg-white/5 hover:text-[#1DA1F2]"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
              </svg>
            </a>
            <a
              href={OFFICIAL_LINKS.telegram}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 rounded-lg text-nofx-text-muted bg-white/5 hover:text-[#0088cc]"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
              </svg>
            </a>
          </div>

          {/* Language Toggle */}
          <div className="py-2">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xs text-nofx-text-muted">
                {t('language', language)}:
              </span>
            </div>
            <div className="space-y-1">
              <button
                onClick={() => {
                  onLanguageChange?.('zh')
                  setMobileMenuOpen(false)
                }}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded transition-colors ${language === 'zh'
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
                className={`w-full flex items-center gap-3 px-3 py-2 rounded transition-colors ${language === 'en'
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
              className="mt-4 pt-4 border-t border-nofx-gold/20"
            >
              <div className="flex items-center gap-2 px-3 py-2 mb-2 rounded bg-nofx-bg-lighter">
                <div className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold bg-nofx-gold text-black">
                  {user.email[0].toUpperCase()}
                </div>
                <div>
                  <div className="text-xs text-nofx-text-muted">
                    {t('loggedInAs', language)}
                  </div>
                  <div className="text-sm text-nofx-text-muted">
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
                  className="w-full px-4 py-2 rounded text-sm font-semibold transition-colors text-center bg-nofx-danger/20 text-nofx-danger"
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
                  className="block w-full px-4 py-2 rounded text-sm font-medium text-center transition-colors text-nofx-text-muted border border-nofx-text-muted hover:text-white hover:border-white"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  {t('signIn', language)}
                </a>
                {registrationEnabled && (
                  <a
                    href="/register"
                    className="block w-full px-4 py-2 rounded font-semibold text-sm text-center transition-colors bg-nofx-gold text-black hover:opacity-90"
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
