import { lazy, Suspense, useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import useSWR from 'swr'
import { api } from './lib/api'
const TraderDashboardPage = lazy(() =>
  import('./pages/TraderDashboardPage').then((m) => ({
    default: m.TraderDashboardPage,
  }))
)

const AITradersPage = lazy(() =>
  import('./components/trader/AITradersPage').then((m) => ({
    default: m.AITradersPage,
  }))
)
import { LoginPage } from './components/auth/LoginPage'
import { SetupPage } from './components/modals/SetupPage'
const SettingsPage = lazy(() =>
  import('./pages/SettingsPage').then((m) => ({ default: m.SettingsPage }))
)
import { ResetPasswordPage } from './components/auth/ResetPasswordPage'
const CompetitionPage = lazy(() =>
  import('./components/trader/CompetitionPage').then((m) => ({
    default: m.CompetitionPage,
  }))
)
const LandingPage = lazy(() =>
  import('./pages/LandingPage').then((m) => ({ default: m.LandingPage }))
)
const StrategyStudioPage = lazy(() =>
  import('./pages/StrategyStudioPage').then((m) => ({
    default: m.StrategyStudioPage,
  }))
)
const DataPage = lazy(() =>
  import('./pages/DataPage').then((m) => ({ default: m.DataPage }))
)
const ReviewPage = lazy(() =>
  import('./pages/ReviewPage').then((m) => ({ default: m.ReviewPage }))
)
const SystemQualityPage = lazy(() => import('./pages/SystemQualityPage'))
const ResearchLogPage = lazy(() => import('./pages/ResearchLogPage'))
const FactorMiningPage = lazy(() => import('./pages/FactorMiningPage'))
import { LoginRequiredOverlay } from './components/auth/LoginRequiredOverlay'
import HeaderBar from './components/common/HeaderBar'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { ConfirmDialogProvider } from './components/common/ConfirmDialog'
import { t } from './i18n/translations'
import { useSystemConfig } from './hooks/useSystemConfig'

import { OFFICIAL_LINKS } from './constants/branding'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  Exchange,
} from './types'

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

function Redirect({ to }: { to: string }) {
  useEffect(() => {
    window.location.replace(to)
  }, [to])
  return null
}

function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { config: systemConfig, loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/welcome') return 'traders'
    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/strategy' || hash === 'strategy') return 'strategy'
    if (path === '/data' || hash === 'data') return 'data'
    if (path === '/review' || hash === 'review') return 'review'
    if (path === '/system-quality' || hash === 'system-quality')
      return 'system-quality'
    if (path === '/research-log' || hash === 'research-log')
      return 'research-log'
    if (path === '/factor-mining' || hash === 'factor-mining')
      return 'factor-mining'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  // Login required overlay state
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')

  const handleLoginRequired = (featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }

  // Unified page navigation handler
  const navigateToPage = (page: Page) => {
    const pathMap: Record<Page, string> = {
      competition: '/competition',
      data: '/data',
      review: '/review',
      'system-quality': '/system-quality',
      'research-log': '/research-log',
      'factor-mining': '/factor-mining',
      traders: '/traders',
      trader: '/dashboard',
      strategy: '/strategy',
      login: '/login',
      register: '/register',
    }
    const path = pathMap[page]
    if (path) {
      window.history.pushState({}, '', path)
      setRoute(path)
      setCurrentPage(page)
    }
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
  // 从 URL 参数读取初始 trader 标识（格式: name-id前4位）
  const [selectedTraderSlug, setSelectedTraderSlug] = useState<
    string | undefined
  >(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('trader') || undefined
  })
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()

  // 生成 trader URL slug（name + ID 前 4 位）
  const getTraderSlug = (trader: TraderInfo) => {
    const idPrefix = trader.trader_id.slice(0, 4)
    return `${trader.trader_name}-${idPrefix}`
  }

  // 从 slug 解析并匹配 trader
  const findTraderBySlug = (slug: string, traderList: TraderInfo[]) => {
    // slug 格式: name-xxxx (xxxx 是 ID 前 4 位)
    const lastDashIndex = slug.lastIndexOf('-')
    if (lastDashIndex === -1) {
      // 没有 dash，直接按 name 匹配
      return traderList.find((t) => t.trader_name === slug)
    }
    const name = slug.slice(0, lastDashIndex)
    const idPrefix = slug.slice(lastDashIndex + 1)
    return traderList.find(
      (t) => t.trader_name === name && t.trader_id.startsWith(idPrefix)
    )
  }
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [decisionsLimit, setDecisionsLimit] = useState<number>(5)
  const hasPersistedAuth =
    !!localStorage.getItem('auth_token') && !!localStorage.getItem('auth_user')

  // Poll-off states: stop polling after 3 consecutive failures
  const [accountPollOff, setAccountPollOff] = useState(false)
  const [positionsPollOff, setPositionsPollOff] = useState(false)

  // Reset poll-off states when trader changes
  useEffect(() => {
    setAccountPollOff(false)
    setPositionsPollOff(false)
  }, [selectedTraderId])

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)
      const params = new URLSearchParams(window.location.search)
      const traderParam = params.get('trader')

      if (path === '/welcome') {
        setCurrentPage('traders')
      } else if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/strategy' || hash === 'strategy') {
        setCurrentPage('strategy')
      } else if (path === '/data' || hash === 'data') {
        setCurrentPage('data')
      } else if (path === '/review' || hash === 'review') {
        setCurrentPage('review')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
        // 如果 URL 中有 trader 参数（slug 格式），更新选中的 trader
        setSelectedTraderSlug(traderParam || undefined)
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  // setCurrentPage(page);
  // window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取trader列表（仅在用户登录时）
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    () => api.getTraders(currentPage === 'trader'),
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

  // 获取exchanges列表（用于显示交易所名称）
  const { data: exchanges } = useSWR<Exchange[]>(
    user && token ? 'exchanges' : null,
    api.getExchangeConfigs,
    {
      refreshInterval: 60000, // 1分钟刷新一次
      shouldRetryOnError: false,
    }
  )

  // 当获取到traders后，根据 URL 中的 trader slug 设置选中的 trader，或默认选中第一个
  useEffect(() => {
    if (!traders || traders.length === 0) {
      return
    }

    if (selectedTraderSlug) {
      // 通过 slug 找到对应的 trader
      const trader = findTraderBySlug(selectedTraderSlug, traders)
      const nextTraderId = trader?.trader_id || traders[0].trader_id
      if (nextTraderId !== selectedTraderId) {
        setSelectedTraderId(nextTraderId)
      }
      return
    }

    if (!selectedTraderId) {
      // Default to a RUNNING trader with the freshest cycle count — a stopped
      // trader at the top of the list (stale Cycles/Decision panels) is never
      // the interesting default. Falls back to the plain first entry.
      const running = traders.filter((t) => t.is_running)
      const pool = running.length > 0 ? running : traders
      const pick = pool.reduce(
        (best, t) => ((t.cycles ?? 0) > (best.cycles ?? 0) ? t : best),
        pool[0]
      )
      setSelectedTraderId(pick.trader_id)
    }
  }, [traders, selectedTraderId, selectedTraderSlug])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId, true),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId, true),
    {
      // F17 (2026-10-01 review): after the 2-retry backoff the poll keeps a
      // SLOW heartbeat (60s) instead of latching OFF with refreshInterval 0 —
      // the old latch froze the panel on stale numbers until a manual reload
      // or another panel's mutate happened to fire a request. onSuccess
      // clears the latch as soon as any heartbeat succeeds.
      refreshInterval: accountPollOff ? 60000 : 15000,
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
        if (accountPollOff) setAccountPollOff(false)
      },
    }
  )

  const { data: positions } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId, true),
    {
      refreshInterval: positionsPollOff ? 60000 : 15000, // F17: slow heartbeat, never a permanent latch
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
        if (positionsPollOff) setPositionsPollOff(false)
      },
    }
  )

  const { data: decisions, error: decisionsError } = useSWR<DecisionRecord[]>(
    currentPage === 'trader' && selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionsLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId, decisionsLimit, true),
    {
      // 15s poll, matching positions/account. A transient failure backs off
      // and retries — it must never silently kill the feed for the session
      // (that used to freeze the decisions panel until a manual reload).
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
      onErrorRetry: (_err, _key, _config, revalidate, { retryCount }) => {
        if (retryCount >= 5) return // next refreshInterval tick retries anyway
        setTimeout(() => revalidate({ retryCount }), 1000 * retryCount)
      },
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId, true),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  const effectiveAccount = account
  const effectivePositions = positions
  const effectiveDecisions = decisions

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/welcome') {
      setCurrentPage('traders')
    } else if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: 'var(--bg)' }}
      >
        <div className="text-center">
          <img
            src="/icons/nofx.svg"
            alt="NoFx Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: 'var(--fg)' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // First-time setup: redirect to /setup if system not initialized
  if (systemConfig && !systemConfig.initialized && !user) {
    return <SetupPage />
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/setup') {
    // If already initialized, redirect to login
    if (systemConfig?.initialized) {
      return <Redirect to="/login" />
    }
    return <SetupPage />
  }
  if (route === '/welcome') {
    // Beginner claw402 onboarding was removed; land on the traders page.
    return <Redirect to="/traders" />
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  if (route === '/settings') {
    if ((!user || !token) && !hasPersistedAuth) {
      return <Redirect to="/login" />
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={navigateToPage}
        />
        <SettingsPage />
      </div>
    )
  }
  // Data page - publicly accessible with embedded dashboard
  if (route === '/data') {
    const dataPageNavigate = (page: Page) => {
      navigateToPage(page)
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="data"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={dataPageNavigate}
        />
        <main className="pt-[52px]">
          <DataPage />
        </main>
        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    )
  }
  // Review page (复盘) - requires authentication
  if (route === '/review') {
    if ((!user || !token) && !hasPersistedAuth) {
      return <Redirect to="/login" />
    }
    const reviewPageNavigate = (page: Page) => {
      navigateToPage(page)
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="review"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={reviewPageNavigate}
        />
        <main className="pt-[52px]">
          <ReviewPage language={language} />
        </main>
        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    )
  }
  // System quality page (系统质量) - requires authentication
  if (route === '/system-quality') {
    if ((!user || !token) && !hasPersistedAuth) {
      return <Redirect to="/login" />
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="system-quality"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={navigateToPage}
        />
        <main className="pt-[52px]">
          <SystemQualityPage language={language} />
        </main>
        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    )
  }
  // Factor mining page (因子挖掘) - requires authentication
  if (route === '/factor-mining') {
    if ((!user || !token) && !hasPersistedAuth) {
      return <Redirect to="/login" />
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="factor-mining"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={navigateToPage}
        />
        <main className="pt-[52px]">
          <FactorMiningPage language={language} />
        </main>
        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    )
  }

  // Research log page (量化研究记录) - requires authentication
  if (route === '/research-log') {
    if ((!user || !token) && !hasPersistedAuth) {
      return <Redirect to="/login" />
    }
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--bg)', color: 'var(--fg)' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="research-log"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onLoginRequired={handleLoginRequired}
          onPageChange={navigateToPage}
        />
        <main className="pt-[52px]">
          <ResearchLogPage language={language} />
        </main>
        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    )
  }
  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Redirect unauthenticated users to landing page
  if (!user || !token) {
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: 'var(--bg)', color: 'var(--fg)' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
        onPageChange={navigateToPage}
      />

      {/* Main Content with Page Transitions */}
      <main className="min-h-screen pt-[52px]">
        <AnimatePresence mode="wait">
          <motion.div
            key={currentPage}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
          >
            {currentPage === 'competition' ? (
              <CompetitionPage />
            ) : currentPage === 'data' ? (
              <DataPage />
            ) : currentPage === 'traders' ? (
              <AITradersPage
                onTraderSelect={(traderId) => {
                  setSelectedTraderId(traderId)
                  const trader = traders?.find(
                    (item) => item.trader_id === traderId
                  )
                  const url = new URL(window.location.href)
                  url.pathname = '/dashboard'
                  if (trader) {
                    const slug = getTraderSlug(trader)
                    url.searchParams.set('trader', slug)
                    setSelectedTraderSlug(slug)
                  } else {
                    url.searchParams.delete('trader')
                    setSelectedTraderSlug(undefined)
                  }
                  window.history.pushState({}, '', url.toString())
                  setRoute('/dashboard')
                  setCurrentPage('trader')
                }}
              />
            ) : currentPage === 'strategy' ? (
              <StrategyStudioPage />
            ) : (
              <TraderDashboardPage
                selectedTrader={selectedTrader}
                status={status}
                account={effectiveAccount}
                accountFailed={accountPollOff}
                positions={effectivePositions}
                positionsFailed={positionsPollOff}
                decisions={effectiveDecisions}
                decisionsFailed={!!decisionsError}
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
                  // 更新 URL 参数（使用 slug: name-id前4位）
                  const trader = traders?.find((t) => t.trader_id === traderId)
                  if (trader) {
                    const slug = getTraderSlug(trader)
                    setSelectedTraderSlug(slug)
                    const url = new URL(window.location.href)
                    url.searchParams.set('trader', slug)
                    window.history.replaceState({}, '', url.toString())
                  }
                }}
                onNavigateToTraders={() => {
                  window.history.pushState({}, '', '/traders')
                  setRoute('/traders')
                  setCurrentPage('traders')
                }}
                exchanges={exchanges}
              />
            )}
          </motion.div>
        </AnimatePresence>
      </main>

      {/* Footer */}
      <footer
        className="mt-16"
        style={{
          borderTop: '1px solid var(--line)',
          background: 'var(--surface-hover)',
        }}
      >
        <div
          className="max-w-[1920px] mx-auto px-6 py-6 text-center text-sm"
          style={{ color: 'var(--fg-3)' }}
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
                background: 'var(--surface-hover)',
                color: 'var(--fg-3)',
                border: '1px solid var(--line)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--line)'
                e.currentTarget.style.color = 'var(--fg)'
                e.currentTarget.style.borderColor = 'var(--brand)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'var(--surface-hover)'
                e.currentTarget.style.color = 'var(--fg-3)'
                e.currentTarget.style.borderColor = 'var(--line)'
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
                background: 'var(--surface-hover)',
                color: 'var(--fg-3)',
                border: '1px solid var(--line)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--line)'
                e.currentTarget.style.color = 'var(--fg)'
                e.currentTarget.style.borderColor = '#1DA1F2'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'var(--surface-hover)'
                e.currentTarget.style.color = 'var(--fg-3)'
                e.currentTarget.style.borderColor = 'var(--line)'
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
                background: 'var(--surface-hover)',
                color: 'var(--fg-3)',
                border: '1px solid var(--line)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--line)'
                e.currentTarget.style.color = 'var(--fg)'
                e.currentTarget.style.borderColor = '#0088cc'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'var(--surface-hover)'
                e.currentTarget.style.color = 'var(--fg-3)'
                e.currentTarget.style.borderColor = 'var(--line)'
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
        </div>
      </footer>

      {/* Login Required Overlay */}
      <LoginRequiredOverlay
        isOpen={loginOverlayOpen}
        onClose={() => setLoginOverlayOpen(false)}
        featureName={loginOverlayFeature}
      />
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <Suspense
            fallback={
              <div role="status" className="p-8 text-center">
                Loading…
              </div>
            }
          >
            <App />
          </Suspense>
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
