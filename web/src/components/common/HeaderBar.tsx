import { useState, useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import {
  ChevronDown,
  Settings,
  LogOut,
  ExternalLink,
  Menu,
  X,
} from 'lucide-react'
import { t, type Language } from '../../i18n/translations'
import { OFFICIAL_LINKS } from '../../constants/branding'
import { getCurrentPageForPath, ROUTES, type Page } from '../../router/paths'
import { HyperliquidWalletConnect } from './HyperliquidWalletConnect'
import { BrandLogo } from './BrandLogo'

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
  onLoginClick,
  isLoggedIn = false,
  currentPage,
  language = 'en',
  user,
  onLogout,
  onPageChange,
  onLoginRequired,
}: HeaderBarProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)
  const [userOpen, setUserOpen] = useState(false)
  const userRef = useRef<HTMLDivElement>(null)
  const resolvedPage = currentPage ?? getCurrentPageForPath(location.pathname)
  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (userRef.current && !userRef.current.contains(event.target as Node))
        setUserOpen(false)
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setUserOpen(false)
        setMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', escape)
    }
  }, [])
  useEffect(() => {
    setMenuOpen(false)
    setUserOpen(false)
  }, [location.pathname])
  const tabs: { page: Page; path: string; label: string; auth: boolean }[] = [
    { page: 'data', path: ROUTES.data, label: 'Data', auth: false },
    {
      page: 'traders',
      path: ROUTES.traders,
      label: t('configNav', language),
      auth: true,
    },
    { page: 'planning', path: ROUTES.planning, label: 'Frontier', auth: true },
    {
      page: 'trader',
      path: ROUTES.dashboard,
      label: t('dashboardNav', language),
      auth: true,
    },
    {
      page: 'strategy',
      path: ROUTES.strategy,
      label: t('strategyNav', language),
      auth: true,
    },
    {
      page: 'competition',
      path: ROUTES.competition,
      label: t('realtimeNav', language),
      auth: true,
    },
    { page: 'faq', path: ROUTES.faq, label: 'Help', auth: false },
  ]
  const open = (tab: (typeof tabs)[number]) => {
    if (tab.auth && !isLoggedIn) {
      onLoginRequired?.(tab.label)
      return
    }
    onPageChange?.(tab.page)
    navigate(tab.path)
  }
  return (
    <nav className="header-bar nofx-navigation" aria-label="Main navigation">
      <div className="nofx-navigation-inner">
        <button
          className="nofx-home"
          onClick={() => navigate(ROUTES.home)}
          aria-label="NOFX home"
        >
          <BrandLogo />
        </button>
        <div className={`nofx-nav-links lg:flex ${menuOpen ? 'is-open' : ''}`}>
          {tabs.map((tab) => (
            <button
              key={tab.page}
              onClick={() => open(tab)}
              aria-current={resolvedPage === tab.page ? 'page' : undefined}
              className="nofx-nav-link whitespace-nowrap"
            >
              {tab.label}
            </button>
          ))}
        </div>
        <div id="dash-header-slot" />
        <div className="nofx-navigation-actions">
          <div className="nofx-nav-wallet">
            <HyperliquidWalletConnect
              language={language}
              isLoggedIn={isLoggedIn}
            />
          </div>
          {isLoggedIn && user ? (
            <div className="nofx-user" ref={userRef}>
              <button
                className="nofx-user-button"
                aria-label={`Account menu for ${user.email}`}
                aria-expanded={userOpen}
                aria-controls="nofx-user-menu"
                onClick={() => setUserOpen(!userOpen)}
              >
                <span className="nofx-user-avatar">
                  {user.email[0].toUpperCase()}
                </span>
                <ChevronDown size={14} />
              </button>
              {userOpen && (
                <div className="nofx-user-menu" id="nofx-user-menu">
                  <div className="nofx-user-identity">
                    <small>Signed in as</small>
                    <strong>{user.email}</strong>
                  </div>
                  <button
                    onClick={() => {
                      navigate(ROUTES.settings)
                      setUserOpen(false)
                    }}
                  >
                    <Settings size={16} /> Account settings
                  </button>
                  {[
                    { name: 'GitHub', href: OFFICIAL_LINKS.github },
                    { name: 'X / Twitter', href: OFFICIAL_LINKS.twitter },
                    {
                      name: 'Telegram Community',
                      href: OFFICIAL_LINKS.telegram,
                    },
                  ].map((link) => (
                    <a
                      key={link.name}
                      href={link.href}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {link.name}
                      <ExternalLink size={14} />
                    </a>
                  ))}
                  {onLogout && (
                    <button
                      className="nofx-sign-out"
                      onClick={() => {
                        onLogout()
                        setUserOpen(false)
                      }}
                    >
                      <LogOut size={16} />
                      {t('exitLogin', language)}
                    </button>
                  )}
                </div>
              )}
            </div>
          ) : (
            resolvedPage !== 'login' &&
            resolvedPage !== 'register' && (
              <button
                className="nofx-sign-in"
                onClick={() =>
                  onLoginClick ? onLoginClick() : navigate(ROUTES.login)
                }
              >
                {t('signIn', language)}
              </button>
            )
          )}
          <button
            className="nofx-nav-toggle lg:hidden"
            aria-label={menuOpen ? 'Close navigation' : 'Open navigation'}
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen(!menuOpen)}
          >
            {menuOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>
      </div>
    </nav>
  )
}
