import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Menu,
  X,
  ChevronDown,
  ChevronRight,
  Check,
  Lock,
  LogOut,
  Repeat,
  Settings,
} from 'lucide-react'
import { ThemeToggle } from './ThemeToggle'
import { buttonVariants } from '../ui'
import { t, type Language } from '../../i18n/translations'
import { OFFICIAL_LINKS } from '../../constants/branding'
import {
  getPostAuthPath,
  getUserMode,
  setUserMode,
  type UserMode,
} from '../../lib/onboarding'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'strategy'
  | 'data'
  | 'review'
  | 'system-quality'
  | 'research-log'
  | 'factor-mining'
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

interface NavTab {
  page: Page
  path: string
  label: string
  requiresAuth: boolean
}

const LANGUAGES: { code: Language; label: string }[] = [
  { code: 'zh', label: '中文' },
  { code: 'en', label: 'English' },
  { code: 'id', label: 'Bahasa' },
]

function buildNavTabs(language: Language): NavTab[] {
  return [
    {
      page: 'data',
      path: '/data',
      label: language === 'zh' ? '数据' : language === 'id' ? 'Data' : 'Data',
      requiresAuth: false,
    },
    {
      page: 'traders',
      path: '/traders',
      label: t('configNav', language),
      requiresAuth: true,
    },
    {
      page: 'trader',
      path: '/dashboard',
      label: t('dashboardNav', language),
      requiresAuth: true,
    },
    {
      page: 'strategy',
      path: '/strategy',
      label: t('strategyNav', language),
      requiresAuth: true,
    },
    {
      page: 'review',
      path: '/review',
      label: t('reviewNav', language),
      requiresAuth: true,
    },
    {
      page: 'system-quality',
      path: '/system-quality',
      label: language === 'zh' ? '系统质量' : 'System Quality',
      requiresAuth: true,
    },
    {
      page: 'factor-mining',
      path: '/factor-mining',
      label: language === 'zh' ? '因子挖掘' : 'Factor Mining',
      requiresAuth: true,
    },
    {
      page: 'research-log',
      path: '/research-log',
      label: language === 'zh' ? '研究记录' : 'Research Log',
      requiresAuth: true,
    },
    {
      page: 'competition',
      path: '/competition',
      label: t('realtimeNav', language),
      requiresAuth: true,
    },
  ]
}

const socialIconClass =
  'inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-3 transition-colors hover:bg-surface-hover hover:text-fg'

const SOCIAL_LINKS = [
  {
    name: 'GitHub',
    href: OFFICIAL_LINKS.github,
    iconSize: 18,
    viewBox: '0 0 16 16',
    path: 'M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z',
  },
  {
    name: 'Twitter',
    href: OFFICIAL_LINKS.twitter,
    iconSize: 16,
    viewBox: '0 0 24 24',
    path: 'M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z',
  },
  {
    name: 'Telegram',
    href: OFFICIAL_LINKS.telegram,
    iconSize: 16,
    viewBox: '0 0 24 24',
    path: 'M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z',
  },
]

const dropdownPanelClass =
  'absolute right-0 top-full mt-1 z-50 rounded-md border border-line bg-surface p-1 shadow-pop'

const dropdownItemClass =
  'flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-[13px] text-fg-2 transition-colors hover:bg-surface-hover hover:text-fg'

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
  const [userMode, setUserModeState] = useState<UserMode>(
    () => getUserMode() ?? 'advanced'
  )
  const dropdownRef = useRef<HTMLDivElement>(null)
  const userDropdownRef = useRef<HTMLDivElement>(null)

  const navigateInApp = (path: string) => {
    navigate(path)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  const handleSwitchMode = (nextMode: UserMode) => {
    setUserMode(nextMode)
    setUserModeState(nextMode)
    setUserDropdownOpen(false)
    navigateInApp(getPostAuthPath(nextMode))
  }

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

  const navTabs = buildNavTabs(language)

  const handleNavClick = (tab: NavTab, closeMobile = false) => {
    // If requires auth and not logged in, show login prompt
    if (tab.requiresAuth && !isLoggedIn) {
      onLoginRequired?.(tab.label)
      if (closeMobile) setMobileMenuOpen(false)
      return
    }
    if (onPageChange) {
      onPageChange(tab.page)
    }
    navigate(tab.path)
    if (closeMobile) setMobileMenuOpen(false)
  }

  const currentLanguageLabel =
    LANGUAGES.find((l) => l.code === language)?.label ?? '中文'

  return (
    <nav className="header-bar fixed top-0 z-50 w-full">
      <div className="mx-auto flex h-[52px] max-w-[1920px] items-center justify-between gap-4 px-4 sm:px-6">
        {/* Logo - Always go to home page */}
        <button
          type="button"
          onClick={() => {
            window.location.href = '/'
          }}
          className="flex shrink-0 items-center gap-2 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50"
        >
          <img src="/icons/nofx.svg" alt="NOFX Logo" className="h-6 w-6" />
          <span className="text-base font-semibold text-fg">NOFX</span>
        </button>

        {/* Desktop Main Nav */}
        <div className="hidden min-w-0 flex-1 items-stretch gap-0.5 self-stretch overflow-x-auto pl-4 [scrollbar-width:none] xl:flex 2xl:gap-1 2xl:pl-6 [&::-webkit-scrollbar]:hidden">
          {navTabs.map((tab) => {
            const active = currentPage === tab.page
            return (
              <button
                key={tab.page}
                type="button"
                onClick={() => handleNavClick(tab)}
                className={`relative flex shrink-0 items-center whitespace-nowrap border-b-2 px-2.5 text-[13px] 2xl:px-3 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/50 ${
                  active
                    ? 'border-brand text-fg'
                    : 'border-transparent text-fg-3 hover:text-fg'
                }`}
              >
                {tab.label}
                {tab.requiresAuth && !isLoggedIn && (
                  <Lock className="ml-1 h-3 w-3 text-fg-disabled" />
                )}
              </button>
            )
          })}
        </div>

        {/* Desktop Right Side */}
        <div className="hidden shrink-0 items-center gap-1 xl:flex">
          <div className="hidden items-center gap-0.5 2xl:flex">
            {SOCIAL_LINKS.map((link) => (
              <a
                key={link.name}
                href={link.href}
                target="_blank"
                rel="noopener noreferrer"
                className={socialIconClass}
                title={link.name}
              >
                <svg
                  width={link.iconSize}
                  height={link.iconSize}
                  viewBox={link.viewBox}
                  fill="currentColor"
                >
                  <path d={link.path} />
                </svg>
              </a>
            ))}
          </div>

          <div className="mx-1.5 hidden h-5 w-px bg-line 2xl:block" />

          {isLoggedIn && user ? (
            <div className="relative" ref={userDropdownRef}>
              <button
                type="button"
                onClick={() => setUserDropdownOpen(!userDropdownOpen)}
                className="flex h-8 items-center gap-2 rounded-md border border-line bg-surface-2 py-0 pl-1 pr-2 transition-colors hover:border-line-strong"
              >
                <span className="flex h-6 w-6 items-center justify-center rounded-full bg-brand text-[11px] font-bold text-brand-fg">
                  {user.email[0].toUpperCase()}
                </span>
                <span className="hidden max-w-[140px] truncate text-[13px] text-fg-2 2xl:inline">
                  {user.email}
                </span>
                <ChevronDown className="h-3.5 w-3.5 text-fg-3" />
              </button>

              <AnimatePresence>
                {userDropdownOpen && (
                  <motion.div
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -4 }}
                    transition={{ duration: 0.12 }}
                    className={`${dropdownPanelClass} w-52`}
                  >
                    <div className="border-b border-line px-2.5 py-2">
                      <div className="text-xs text-fg-3">
                        {t('loggedInAs', language)}
                      </div>
                      <div className="truncate text-[13px] font-medium text-fg">
                        {user.email}
                      </div>
                    </div>
                    <div className="p-1">
                      <button
                        type="button"
                        onClick={() => {
                          window.location.href = '/settings'
                          setUserDropdownOpen(false)
                        }}
                        className={dropdownItemClass}
                      >
                        <Settings className="h-3.5 w-3.5 text-fg-3" />
                        Settings
                      </button>
                      <button
                        type="button"
                        onClick={() =>
                          handleSwitchMode(
                            userMode === 'beginner' ? 'advanced' : 'beginner'
                          )
                        }
                        className={dropdownItemClass}
                      >
                        <Repeat className="h-3.5 w-3.5 text-fg-3" />
                        {userMode === 'beginner'
                          ? language === 'zh'
                            ? '切到老手模式'
                            : 'Switch to Advanced'
                          : language === 'zh'
                            ? '切到新手模式'
                            : 'Switch to Beginner'}
                      </button>
                      {onLogout && (
                        <button
                          type="button"
                          onClick={() => {
                            onLogout()
                            setUserDropdownOpen(false)
                          }}
                          className="flex w-full items-center gap-2 rounded px-2.5 py-1.5 text-left text-[13px] font-medium text-down transition-colors hover:bg-down-soft"
                        >
                          <LogOut className="h-3.5 w-3.5" />
                          {t('exitLogin', language)}
                        </button>
                      )}
                    </div>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          ) : (
            currentPage !== 'login' &&
            currentPage !== 'register' && (
              <a
                href="/login"
                className={buttonVariants({ variant: 'secondary', size: 'md' })}
              >
                {t('signIn', language)}
              </a>
            )
          )}

          <ThemeToggle />

          {/* Language Switcher */}
          <div className="relative" ref={dropdownRef}>
            <button
              type="button"
              onClick={() => setLanguageDropdownOpen(!languageDropdownOpen)}
              className="flex h-8 items-center gap-1.5 rounded-md px-2 text-[13px] text-fg-2 transition-colors hover:bg-surface-hover hover:text-fg"
            >
              {currentLanguageLabel}
              <ChevronDown className="h-3.5 w-3.5 text-fg-3" />
            </button>

            <AnimatePresence>
              {languageDropdownOpen && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -4 }}
                  transition={{ duration: 0.12 }}
                  className={`${dropdownPanelClass} w-36`}
                >
                  {LANGUAGES.map((l) => (
                    <button
                      key={l.code}
                      type="button"
                      onClick={() => {
                        onLanguageChange?.(l.code)
                        setLanguageDropdownOpen(false)
                      }}
                      className={dropdownItemClass}
                    >
                      <span className="w-3.5">
                        {language === l.code && (
                          <Check className="h-3.5 w-3.5 text-brand" />
                        )}
                      </span>
                      {l.label}
                    </button>
                  ))}
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>

        {/* Mobile Menu Button */}
        <button
          type="button"
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          aria-label="Toggle menu"
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-2 transition-colors hover:bg-surface-hover hover:text-fg xl:hidden"
        >
          {mobileMenuOpen ? (
            <X className="h-5 w-5" />
          ) : (
            <Menu className="h-5 w-5" />
          )}
        </button>
      </div>

      {/* Mobile Menu Overlay */}
      <AnimatePresence>
        {mobileMenuOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="fixed inset-x-0 bottom-0 top-[52px] z-40 overflow-y-auto bg-surface xl:hidden"
          >
            <div className="flex min-h-full flex-col px-4 py-4">
              {/* Navigation Links */}
              <nav className="flex flex-col gap-0.5">
                {navTabs.map((tab) => {
                  const active = currentPage === tab.page
                  return (
                    <button
                      key={tab.page}
                      type="button"
                      onClick={() => handleNavClick(tab, true)}
                      className={`flex items-center gap-2.5 rounded-md px-3 py-2.5 text-left text-[15px] font-medium transition-colors ${
                        active
                          ? 'bg-surface-2 text-fg'
                          : 'text-fg-2 hover:bg-surface-hover hover:text-fg'
                      }`}
                    >
                      <span
                        className={`h-4 w-0.5 rounded-full ${
                          active ? 'bg-brand' : 'bg-transparent'
                        }`}
                      />
                      {tab.label}
                      {tab.requiresAuth && !isLoggedIn && (
                        <Lock className="ml-auto h-3.5 w-3.5 text-fg-disabled" />
                      )}
                    </button>
                  )
                })}
              </nav>

              {/* On-page anchors (home only) */}
              {isHomePage && (
                <div className="mt-4 border-t border-line pt-4">
                  {[
                    { key: 'features', label: t('features', language) },
                    { key: 'install', label: t('howItWorks', language) },
                  ].map((item) => (
                    <a
                      key={item.key}
                      href={`#${item.key}`}
                      className="flex items-center gap-1.5 rounded-md px-3 py-2.5 text-[15px] text-fg-2 transition-colors hover:bg-surface-hover hover:text-fg"
                      onClick={() => setMobileMenuOpen(false)}
                    >
                      <ChevronRight className="h-4 w-4 text-fg-3" />
                      {item.label}
                    </a>
                  ))}
                </div>
              )}

              {/* Bottom Actions */}
              <div className="mt-auto space-y-4 pt-8">
                <div className="flex items-center gap-1">
                  {SOCIAL_LINKS.map((link) => (
                    <a
                      key={link.name}
                      href={link.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={socialIconClass}
                      title={link.name}
                    >
                      <svg
                        width={link.iconSize}
                        height={link.iconSize}
                        viewBox={link.viewBox}
                        fill="currentColor"
                      >
                        <path d={link.path} />
                      </svg>
                    </a>
                  ))}
                </div>

                <div className="flex items-center justify-between">
                  <ThemeToggle />
                </div>

                <div className="grid grid-cols-2 items-stretch gap-2">
                  {/* Language Switcher */}
                  <div className="flex rounded-md border border-line bg-surface-2 p-0.5">
                    {LANGUAGES.map((l) => (
                      <button
                        key={l.code}
                        type="button"
                        onClick={() => {
                          onLanguageChange?.(l.code)
                        }}
                        className={`flex-1 rounded px-1 py-1.5 text-xs font-semibold transition-colors ${
                          language === l.code
                            ? 'bg-brand-soft text-brand'
                            : 'text-fg-3 hover:text-fg'
                        }`}
                      >
                        {l.code.toUpperCase()}
                      </button>
                    ))}
                  </div>

                  {/* Auth Action */}
                  {isLoggedIn && user ? (
                    <button
                      type="button"
                      onClick={() => {
                        onLogout?.()
                        setMobileMenuOpen(false)
                      }}
                      className="rounded-md border border-down/30 bg-down-soft text-[13px] font-semibold text-down transition-colors hover:bg-down/20"
                    >
                      {t('exitLogin', language)}
                    </button>
                  ) : (
                    currentPage !== 'login' &&
                    currentPage !== 'register' && (
                      <a
                        href="/login"
                        className={buttonVariants({
                          variant: 'primary',
                          size: 'md',
                        })}
                      >
                        {t('signIn', language)}
                      </a>
                    )
                  )}
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </nav>
  )
}
