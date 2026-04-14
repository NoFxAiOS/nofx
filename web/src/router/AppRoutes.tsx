import { type ReactNode, useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import useSWR from 'swr'
import {
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useSearchParams,
} from 'react-router-dom'
import HeaderBar from '../components/common/HeaderBar'
import { SiteFooter } from '../components/common/SiteFooter'
import { LoginRequiredOverlay } from '../components/auth/LoginRequiredOverlay'
import { LoginPage } from '../components/auth/LoginPage'
import { RegisterPage } from '../components/auth/RegisterPage'
import { ResetPasswordPage } from '../components/auth/ResetPasswordPage'
import { SetupPage } from '../components/modals/SetupPage'
import { CompetitionPage } from '../components/trader/CompetitionPage'
import { AITradersPage } from '../components/trader/AITradersPage'
import { FAQPage } from '../pages/FAQPage'
import { LandingPage } from '../pages/LandingPage'
import { BeginnerOnboardingPage } from '../pages/BeginnerOnboardingPage'
import { DataPage } from '../pages/DataPage'
import { SettingsPage } from '../pages/SettingsPage'
import { StrategyMarketPage } from '../pages/StrategyMarketPage'
import { StrategyStudioPage } from '../pages/StrategyStudioPage'
import { TraderDashboardPage } from '../pages/TraderDashboardPage'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { t } from '../i18n/translations'
import { api } from '../lib/api'
import { getUserMode } from '../lib/onboarding'
import type {
  AccountInfo,
  DecisionRecord,
  Exchange,
  Position,
  Statistics,
  SystemStatus,
  TraderInfo,
} from '../types'
import {
  buildDashboardPath,
  LEGACY_HASH_ROUTES,
  ROUTES,
  type Page,
} from './paths'

function getTraderSlug(trader: TraderInfo) {
  const idPrefix = trader.trader_id.slice(0, 4)
  return `${trader.trader_name}-${idPrefix}`
}

function findTraderBySlug(slug: string, traderList: TraderInfo[]) {
  const lastDashIndex = slug.lastIndexOf('-')
  if (lastDashIndex === -1) {
    return traderList.find((trader) => trader.trader_name === slug)
  }

  const name = slug.slice(0, lastDashIndex)
  const idPrefix = slug.slice(lastDashIndex + 1)
  return traderList.find(
    (trader) =>
      trader.trader_name === name && trader.trader_id.startsWith(idPrefix)
  )
}

function LoadingScreen() {
  const { language } = useLanguage()

  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ background: '#0B0E11' }}
    >
      <div className="text-center">
        <img
          src="/icons/nofx.svg"
          alt="NoFx Logo"
          className="w-16 h-16 mx-auto mb-4 animate-pulse"
        />
        <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
      </div>
    </div>
  )
}

function LegacyHashRedirect() {
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    const hashRoute = LEGACY_HASH_ROUTES[location.hash.slice(1)]
    if (!hashRoute) {
      return
    }

    if (hashRoute === location.pathname && location.hash === '') {
      return
    }

    navigate(
      {
        pathname: hashRoute,
        search: location.search,
      },
      { replace: true }
    )
  }, [location.hash, location.pathname, location.search, navigate])

  return null
}

interface AppChromeProps {
  children: ReactNode
  currentPage?: Page
  showFooter?: boolean
  wrapInMain?: boolean
  animateContent?: boolean
  extraContent?: ReactNode
}

function AppChrome({
  children,
  currentPage,
  showFooter = true,
  wrapInMain = true,
  animateContent = false,
  extraContent,
}: AppChromeProps) {
  const location = useLocation()
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')

  const handleLoginRequired = (featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }

  const content = animateContent ? (
    <AnimatePresence mode="wait">
      <motion.div
        key={`${location.pathname}${location.search}`}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -8 }}
        transition={{ duration: 0.15, ease: 'easeOut' }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  ) : (
    children
  )

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
      />

      {wrapInMain ? (
        <main className="min-h-screen pt-16">{content}</main>
      ) : (
        content
      )}

      {showFooter ? <SiteFooter language={language} /> : null}

      <LoginRequiredOverlay
        isOpen={loginOverlayOpen}
        onClose={() => setLoginOverlayOpen(false)}
        featureName={loginOverlayFeature}
      />

      {extraContent}
    </div>
  )
}

function TradersRoute({
  showBeginnerOnboarding = false,
}: {
  showBeginnerOnboarding?: boolean
}) {
  const navigate = useNavigate()
  const { user, token } = useAuth()
  const { data: traders } = useSWR<TraderInfo[]>(
    user && token ? 'traders-route' : null,
    api.getTraders,
    {
      refreshInterval: 5000,
      shouldRetryOnError: false,
    }
  )

  return (
    <AppChrome
      currentPage="traders"
      animateContent
      extraContent={showBeginnerOnboarding ? <BeginnerOnboardingPage /> : null}
    >
      <AITradersPage
        onTraderSelect={(traderId) => {
          const trader = traders?.find((item) => item.trader_id === traderId)
          navigate(
            buildDashboardPath(trader ? getTraderSlug(trader) : undefined)
          )
        }}
      />
    </AppChrome>
  )
}

function DashboardRoute() {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const selectedTraderSlug = searchParams.get('trader') || undefined
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [decisionsLimit, setDecisionsLimit] = useState(5)
  const [accountPollOff, setAccountPollOff] = useState(false)
  const [positionsPollOff, setPositionsPollOff] = useState(false)
  const [decisionsPollOff, setDecisionsPollOff] = useState(false)

  useEffect(() => {
    setAccountPollOff(false)
    setPositionsPollOff(false)
    setDecisionsPollOff(false)
  }, [selectedTraderId])

  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders-dashboard' : null,
    () => api.getTraders(true),
    {
      refreshInterval: 10000,
      shouldRetryOnError: false,
    }
  )

  const { data: exchanges } = useSWR<Exchange[]>(
    user && token ? 'exchanges-dashboard' : null,
    api.getExchangeConfigs,
    {
      refreshInterval: 60000,
      shouldRetryOnError: false,
    }
  )

  useEffect(() => {
    if (!traders || traders.length === 0) {
      return
    }

    if (selectedTraderSlug) {
      const trader = findTraderBySlug(selectedTraderSlug, traders)
      const nextTraderId = trader?.trader_id || traders[0].trader_id
      if (nextTraderId !== selectedTraderId) {
        setSelectedTraderId(nextTraderId)
      }
      return
    }

    if (!selectedTraderId) {
      setSelectedTraderId(traders[0].trader_id)
    }
  }, [selectedTraderId, selectedTraderSlug, traders])

  const { data: status } = useSWR<SystemStatus>(
    selectedTraderId ? `status-${selectedTraderId}` : null,
    () => api.getStatus(selectedTraderId, true),
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    selectedTraderId ? `account-${selectedTraderId}` : null,
    () => api.getAccount(selectedTraderId, true),
    {
      refreshInterval: accountPollOff ? 0 : 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
      onErrorRetry: (_err, _key, _config, revalidate, { retryCount }) => {
        if (retryCount >= 2) {
          setAccountPollOff(true)
          return
        }
        setTimeout(() => revalidate({ retryCount }), 500)
      },
      onSuccess: () => {
        if (accountPollOff) {
          setAccountPollOff(false)
        }
      },
    }
  )

  const { data: positions } = useSWR<Position[]>(
    selectedTraderId ? `positions-${selectedTraderId}` : null,
    () => api.getPositions(selectedTraderId, true),
    {
      refreshInterval: positionsPollOff ? 0 : 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
      onErrorRetry: (_err, _key, _config, revalidate, { retryCount }) => {
        if (retryCount >= 2) {
          setPositionsPollOff(true)
          return
        }
        setTimeout(() => revalidate({ retryCount }), 500)
      },
      onSuccess: () => {
        if (positionsPollOff) {
          setPositionsPollOff(false)
        }
      },
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionsLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId, decisionsLimit, true),
    {
      refreshInterval: decisionsPollOff ? 0 : 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
      onErrorRetry: (_err, _key, _config, revalidate, { retryCount }) => {
        if (retryCount >= 2) {
          setDecisionsPollOff(true)
          return
        }
        setTimeout(() => revalidate({ retryCount }), 500)
      },
      onSuccess: () => {
        if (decisionsPollOff) {
          setDecisionsPollOff(false)
        }
      },
    }
  )

  const { data: stats } = useSWR<Statistics>(
    selectedTraderId ? `statistics-${selectedTraderId}` : null,
    () => api.getStatistics(selectedTraderId, true),
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      setLastUpdate(new Date().toLocaleTimeString())
    }
  }, [account])

  const selectedTrader = traders?.find(
    (trader) => trader.trader_id === selectedTraderId
  )

  return (
    <AppChrome currentPage="trader" animateContent>
      <TraderDashboardPage
        selectedTrader={selectedTrader}
        status={status}
        account={account}
        accountFailed={accountPollOff}
        positions={positions}
        positionsFailed={positionsPollOff}
        decisions={decisions}
        decisionsFailed={decisionsPollOff}
        decisionsLimit={decisionsLimit}
        onDecisionsLimitChange={setDecisionsLimit}
        stats={stats}
        lastUpdate={lastUpdate}
        language={language}
        traders={traders}
        tradersError={tradersError}
        selectedTraderId={selectedTraderId}
        onTraderSelect={(traderId) => {
          setSelectedTraderId(traderId)
          const trader = traders?.find((item) => item.trader_id === traderId)
          navigate(
            buildDashboardPath(trader ? getTraderSlug(trader) : undefined),
            {
              replace: true,
            }
          )
        }}
        onNavigateToTraders={() => navigate(ROUTES.traders)}
        exchanges={exchanges}
      />
    </AppChrome>
  )
}

export function AppRoutes() {
  const { user, token, isLoading } = useAuth()
  const { config: systemConfig, loading: configLoading } = useSystemConfig()
  const isAuthenticated = !!user && !!token

  if (isLoading || configLoading) {
    return <LoadingScreen />
  }

  if (systemConfig && !systemConfig.initialized && !user) {
    return <SetupPage />
  }

  return (
    <>
      <LegacyHashRedirect />
      <Routes>
        <Route path={ROUTES.home} element={<LandingPage />} />
        <Route path={ROUTES.login} element={<LoginPage />} />
        <Route path={ROUTES.register} element={<RegisterPage />} />
        <Route path={ROUTES.resetPassword} element={<ResetPasswordPage />} />
        <Route
          path={ROUTES.setup}
          element={
            systemConfig?.initialized ? (
              <Navigate to={ROUTES.login} replace />
            ) : (
              <SetupPage />
            )
          }
        />
        <Route
          path={ROUTES.faq}
          element={
            <AppChrome currentPage="faq" showFooter={false} wrapInMain={false}>
              <FAQPage />
            </AppChrome>
          }
        />
        <Route
          path={ROUTES.data}
          element={
            <AppChrome currentPage="data" showFooter={false}>
              <DataPage />
            </AppChrome>
          }
        />
        <Route
          path={ROUTES.settings}
          element={
            isAuthenticated ? (
              <AppChrome showFooter={false}>
                <SettingsPage />
              </AppChrome>
            ) : (
              <Navigate to={ROUTES.login} replace />
            )
          }
        />
        <Route
          path={ROUTES.welcome}
          element={
            isAuthenticated ? (
              getUserMode() === 'beginner' ? (
                <TradersRoute showBeginnerOnboarding />
              ) : (
                <Navigate to={ROUTES.traders} replace />
              )
            ) : (
              <Navigate to={ROUTES.login} replace />
            )
          }
        />
        <Route
          path={ROUTES.competition}
          element={
            isAuthenticated ? (
              <AppChrome currentPage="competition" animateContent>
                <CompetitionPage />
              </AppChrome>
            ) : (
              <LandingPage />
            )
          }
        />
        <Route
          path={ROUTES.strategyMarket}
          element={
            isAuthenticated ? (
              <AppChrome currentPage="strategy-market" animateContent>
                <StrategyMarketPage />
              </AppChrome>
            ) : (
              <LandingPage />
            )
          }
        />
        <Route
          path={ROUTES.traders}
          element={isAuthenticated ? <TradersRoute /> : <LandingPage />}
        />
        <Route
          path={ROUTES.dashboard}
          element={isAuthenticated ? <DashboardRoute /> : <LandingPage />}
        />
        <Route
          path={ROUTES.strategy}
          element={
            isAuthenticated ? (
              <AppChrome currentPage="strategy" animateContent>
                <StrategyStudioPage />
              </AppChrome>
            ) : (
              <LandingPage />
            )
          }
        />
        <Route path="*" element={<Navigate to={ROUTES.home} replace />} />
      </Routes>
    </>
  )
}
