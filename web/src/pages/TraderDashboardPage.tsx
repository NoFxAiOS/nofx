import { PaperBadge } from '../components/trader/PaperBadge'
import { useEffect, useState, useRef } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { ChartTabs } from '../components/charts/ChartTabs'
import { DecisionCard } from '../components/trader/DecisionCard'
import { PositionHistory } from '../components/trader/PositionHistory'
import { PunkAvatar, getTraderAvatar } from '../components/common/PunkAvatar'
import { confirmToast, notify } from '../lib/notify'
import { formatPrice, formatQuantity } from '../utils/format'
import { t, type Language } from '../i18n/translations'
import {
  LogOut,
  Loader2,
  Eye,
  EyeOff,
  Copy,
  Check,
  AlertTriangle,
  Brain,
  Monitor,
} from 'lucide-react'
import { NofxSelect } from '../components/ui/select'
import {
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Change,
  DataTable,
  EmptyState,
  PnL,
  Skeleton,
  Stat,
  Tabs,
  type DataTableColumn,
  type TabItem,
} from '../components/ui'
import { GridRiskPanel } from '../components/strategy/GridRiskPanel'
import { RiskStatusPanel } from '../components/trader/RiskStatusPanel'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  Exchange,
} from '../types'

// --- Helper Functions ---

// Get friendly AI model display name
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name
    ? `${typeName} - ${exchange.account_name}`
    : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for kline charts
function getExchangeTypeFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'binance'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return 'binance' // Default to binance for charts
  return exchange.exchange_type === 'paper'
    ? 'binance'
    : exchange.exchange_type?.toLowerCase() || 'binance'
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

// --- Components ---

interface TraderDashboardPageProps {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  accountFailed?: boolean
  positions?: Position[]
  positionsFailed?: boolean
  decisions?: DecisionRecord[]
  decisionsFailed?: boolean
  decisionsLimit: number
  onDecisionsLimitChange: (limit: number) => void
  stats?: Statistics
  lastUpdate: string
  language: Language
  exchanges?: Exchange[]
}

export function TraderDashboardPage({
  selectedTrader,
  status,
  account,
  accountFailed,
  positions,
  positionsFailed,
  decisions,
  decisionsFailed,
  decisionsLimit,
  onDecisionsLimitChange,
  lastUpdate,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
  exchanges,
}: TraderDashboardPageProps) {
  const [closingPosition, setClosingPosition] = useState<string | null>(null)
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<
    string | undefined
  >(undefined)
  const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
  const chartSectionRef = useRef<HTMLDivElement>(null)
  const [showWalletAddress, setShowWalletAddress] = useState<boolean>(false)
  const [copiedAddress, setCopiedAddress] = useState<boolean>(false)
  const [bottomTab, setBottomTab] = useState<'positions' | 'history'>(
    'positions'
  )

  // Current positions pagination
  const [positionsPageSize, setPositionsPageSize] = useState<number>(20)
  const [positionsCurrentPage, setPositionsCurrentPage] = useState<number>(1)

  // Calculate paginated positions
  const totalPositions = positions?.length || 0
  const totalPositionPages = Math.ceil(totalPositions / positionsPageSize)
  const paginatedPositions =
    positions?.slice(
      (positionsCurrentPage - 1) * positionsPageSize,
      positionsCurrentPage * positionsPageSize
    ) || []

  // Reset page when positions change
  useEffect(() => {
    setPositionsCurrentPage(1)
  }, [selectedTraderId, positionsPageSize])

  // Auto-set chart symbol for grid trading
  useEffect(() => {
    if (status?.strategy_type === 'grid_trading' && status?.grid_symbol) {
      setSelectedChartSymbol(status.grid_symbol)
    }
  }, [status?.strategy_type, status?.grid_symbol])

  // Get current exchange info for perp-dex wallet display
  const currentExchange = exchanges?.find(
    (e) => e.id === selectedTrader?.exchange_id
  )
  const walletAddress = getWalletAddress(currentExchange)
  const isPerpDex = isPerpDexExchange(currentExchange?.exchange_type)

  // Copy wallet address to clipboard
  const handleCopyAddress = async () => {
    if (!walletAddress) return
    try {
      await navigator.clipboard.writeText(walletAddress)
      setCopiedAddress(true)
      setTimeout(() => setCopiedAddress(false), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }

  // Handle symbol click from Decision Card
  const handleSymbolClick = (symbol: string) => {
    // Set the selected symbol
    setSelectedChartSymbol(symbol)
    // Scroll to chart section
    setTimeout(() => {
      chartSectionRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    }, 100)
  }

  // Close position handler
  const handleClosePosition = async (symbol: string, side: string) => {
    if (!selectedTraderId) return

    const sideLabel = side === 'LONG' ? 'LONG' : 'SHORT'
    const confirmMsg = t('traderDashboard.confirmClosePosition', language, {
      symbol,
      side: sideLabel,
    })

    const confirmed = await confirmToast(confirmMsg, {
      title: t('traderDashboard.confirmClose', language),
      okText: t('traderDashboard.confirm', language),
      cancelText: t('traderDashboard.cancel', language),
    })

    if (!confirmed) return

    setClosingPosition(symbol)
    try {
      await api.closePosition(selectedTraderId, symbol, side)
      notify.success(t('traderDashboard.positionClosed', language))
      // Use SWR mutate to refresh data instead of reloading page
      await Promise.all([
        mutate(`positions-${selectedTraderId}`),
        mutate(`account-${selectedTraderId}`),
      ])
    } catch (err: unknown) {
      const errorMsg =
        err instanceof Error
          ? err.message
          : t('traderDashboard.closeFailed', language)
      notify.error(errorMsg)
    } finally {
      setClosingPosition(null)
    }
  }

  const accountLoading = !account && !accountFailed
  const accountMissing = accountFailed && !account
  const pnlTone: 'up' | 'down' | 'default' =
    account?.total_pnl === undefined
      ? 'default'
      : account.total_pnl > 0
        ? 'up'
        : account.total_pnl < 0
          ? 'down'
          : 'default'
  const isStale = !!(accountFailed || positionsFailed)

  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <EmptyState
          icon={<AlertTriangle className="h-10 w-10 text-brand" />}
          title={t('traderDashboard.connectionFailed', language)}
          description={t('traderDashboard.connectionFailedDesc', language)}
          action={
            <Button variant="primary" onClick={() => window.location.reload()}>
              {t('traderDashboard.retry', language)}
            </Button>
          }
        />
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <EmptyState
          icon={<Monitor className="h-10 w-10 text-brand" />}
          title={t('dashboardEmptyTitle', language)}
          description={t('dashboardEmptyDescription', language)}
          action={
            <Button variant="primary" onClick={onNavigateToTraders}>
              {t('goToTradersPage', language)}
            </Button>
          }
        />
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="w-full space-y-3 px-4 pt-4 md:px-6">
        <Card className="p-4">
          <Skeleton className="mb-3 h-6 w-48" />
          <div className="flex gap-3">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-28" />
          </div>
        </Card>
        <Card className="grid grid-cols-2 gap-4 p-4 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i}>
              <Skeleton className="mb-2 h-3 w-20" />
              <Skeleton className="h-6 w-28" />
            </div>
          ))}
        </Card>
        <Card className="p-4">
          <Skeleton className="mb-3 h-5 w-40" />
          <Skeleton className="h-64 w-full" />
        </Card>
      </div>
    )
  }

  const goToChart = (symbol: string) => {
    setSelectedChartSymbol(symbol)
    setChartUpdateKey(Date.now())
    chartSectionRef.current?.scrollIntoView({
      behavior: 'smooth',
      block: 'start',
    })
  }

  const renderOrders = (pos: Position) => {
    const sl = pos.protection?.sl_price
    const tp = pos.protection?.tp_price
    const lim = pos.protection?.limit_price
    // F17 (2026-10-01 review): "query failed" (unknown) must not render
    // like "no orders" — a naked-looking position may actually be fine,
    // and a "protected-looking" silence may be a blind read.
    if (pos.protection?.protection_error) {
      return (
        <span
          className="text-warn"
          title={t('traderDashboard.ordersUnknown', language)}
        >
          ?
        </span>
      )
    }
    if (sl == null && tp == null && lim == null) {
      return (
        <span
          className="text-fg-3"
          title={t('traderDashboard.ordersMissing', language)}
        >
          —
        </span>
      )
    }
    return (
      <span className="inline-flex flex-col items-end gap-0.5 text-xs leading-tight">
        {sl != null && (
          <span className="text-down" title={`SL ${formatPrice(sl)}`}>
            SL {formatPrice(sl)}
          </span>
        )}
        {tp != null && (
          <span className="text-up" title={`TP ${formatPrice(tp)}`}>
            TP {formatPrice(tp)}
          </span>
        )}
        {lim != null && (
          <span className="text-brand" title={`LIMIT ${formatPrice(lim)}`}>
            LMT {formatPrice(lim)}
          </span>
        )}
      </span>
    )
  }

  const hdr = (label: string, hint?: string) => (
    <span title={hint}>{label}</span>
  )

  // F18 (2026-10-01 review): the risk columns (entry/mark/leverage/SL-TP/liq)
  // must stay visible on mobile because the close button does; the table
  // scrolls inside its card. Only the low-stakes `value` column is md+.
  const positionColumns: DataTableColumn<Position>[] = [
    {
      key: 'symbol',
      header: t('symbol', language),
      render: (pos) => <span className="num font-semibold">{pos.symbol}</span>,
    },
    {
      key: 'side',
      header: t('side', language),
      align: 'center',
      render: (pos) => (
        <Badge variant={pos.side === 'long' ? 'up' : 'down'} size="xs">
          {t(pos.side === 'long' ? 'long' : 'short', language).toUpperCase()}
        </Badge>
      ),
    },
    {
      key: 'action',
      header: t('traderDashboard.action', language),
      align: 'center',
      render: (pos) => (
        <Button
          variant="danger"
          size="sm"
          className="h-6 px-2 text-[11px]"
          onClick={(e) => {
            e.stopPropagation()
            handleClosePosition(pos.symbol, pos.side.toUpperCase())
          }}
          disabled={closingPosition === pos.symbol}
          title={t('traderDashboard.closePosition', language)}
        >
          {closingPosition === pos.symbol ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <LogOut className="h-3 w-3" />
          )}
          {t('traderDashboard.close', language)}
        </Button>
      ),
    },
    {
      key: 'entry',
      header: hdr(
        t('traderDashboard.entry', language),
        t('entryPrice', language)
      ),
      numeric: true,
      render: (pos) => formatPrice(pos.entry_price),
    },
    {
      key: 'mark',
      header: hdr(
        t('traderDashboard.mark', language),
        t('markPrice', language)
      ),
      numeric: true,
      render: (pos) => formatPrice(pos.mark_price),
    },
    {
      key: 'qty',
      header: hdr(t('traderDashboard.qty', language), t('quantity', language)),
      numeric: true,
      render: (pos) => formatQuantity(pos.quantity),
    },
    {
      key: 'value',
      header: hdr(
        t('traderDashboard.value', language),
        t('positionValue', language)
      ),
      numeric: true,
      className: 'hidden md:table-cell',
      headerClassName: 'hidden md:table-cell',
      render: (pos) => (pos.quantity * pos.mark_price).toFixed(2),
    },
    {
      key: 'lev',
      header: hdr(t('traderDashboard.lev', language), t('leverage', language)),
      numeric: true,
      render: (pos) => `${pos.leverage}x`,
    },
    {
      key: 'upnl',
      header: hdr(
        t('traderDashboard.uPnL', language),
        t('unrealizedPnL', language)
      ),
      numeric: true,
      render: (pos) => (
        <PnL value={pos.unrealized_pnl} currency="" className="font-semibold" />
      ),
    },
    {
      key: 'pnlpct',
      header: hdr(
        t('traderDashboard.pnlPct', language),
        t('unrealizedPnL', language)
      ),
      numeric: true,
      render: (pos) => (
        <Change value={pos.unrealized_pnl_pct ?? 0} className="font-semibold" />
      ),
    },
    {
      key: 'orders',
      header: hdr(
        t('traderDashboard.orders', language),
        t('traderDashboard.ordersHint', language)
      ),
      numeric: true,
      className: 'py-1',
      render: renderOrders,
    },
    {
      key: 'liq',
      header: hdr(t('traderDashboard.liq', language), t('liqPrice', language)),
      numeric: true,
      className: 'text-fg-3',
      render: (pos) => formatPrice(pos.liquidation_price),
    },
  ]

  const pageBtn = (label: string, onClick: () => void, disabled: boolean) => (
    <Button
      variant="ghost"
      size="sm"
      className="h-6 px-2"
      onClick={onClick}
      disabled={disabled}
    >
      {label}
    </Button>
  )

  const tabItems: TabItem<'positions' | 'history'>[] = [
    {
      key: 'positions',
      label: t('currentPositions', language),
      count: positions?.length ?? 0,
    },
    { key: 'history', label: t('positionHistory.title', language) },
  ]

  return (
    <div className="w-full space-y-3 px-4 pb-10 pt-4 md:px-6">
      {/* Trader header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2">
          <div className="flex items-center gap-3">
            <div className="relative">
              <PunkAvatar
                seed={getTraderAvatar(
                  selectedTrader.trader_id,
                  selectedTrader.trader_name
                )}
                size={40}
                className="rounded-lg border border-line"
              />
              <span
                className={`absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-bg ${isStale ? 'bg-down' : 'bg-up'}`}
                title={isStale ? 'STALE' : 'ONLINE'}
              />
            </div>
            <div className="min-w-0">
              <h1 className="truncate text-xl font-semibold leading-tight text-fg">
                {selectedTrader.trader_name}
              </h1>
              <div className="num text-[11px] text-fg-3">
                ID: {selectedTrader.trader_id.slice(0, 8)}...
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            <Badge
              variant={selectedTrader.ai_model.includes('qwen') ? 'ai' : 'info'}
              title="AI Model"
            >
              {getModelDisplayName(
                selectedTrader.ai_model.split('_').pop() ||
                  selectedTrader.ai_model
              )}
            </Badge>
            <Badge title="Exchange">
              {getExchangeDisplayNameFromList(
                selectedTrader.exchange_id,
                exchanges
              )}
            </Badge>
            <PaperBadge
              exchangeType={currentExchange?.exchange_type}
              language={language}
            />
            <Badge variant="brand" title="Strategy">
              {selectedTrader.strategy_name || 'No Strategy'}
            </Badge>
            {status && (
              <>
                <Badge className="hidden md:inline-flex">
                  Cycles: <span className="num">{status.call_count}</span>
                </Badge>
                <Badge className="hidden md:inline-flex">
                  Runtime:{' '}
                  <span className="num">{status.runtime_minutes} min</span>
                </Badge>
              </>
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* F17 (2026-10-01 review): the status used to read ONLINE
              unconditionally — stale account/position data looked live.
              Fetch failure => STALE (red). */}
          <Badge variant={isStale ? 'down' : 'up'} title="System status">
            {isStale ? 'STALE' : 'ONLINE'}
          </Badge>
          {account ? (
            <span className="num hidden text-[11px] text-fg-3 sm:inline">
              {lastUpdate}
            </span>
          ) : accountFailed ? (
            <span className="text-xs text-down">
              {t('traderDashboard.accountFetchFailed', language)}
            </span>
          ) : null}

          {/* Trader Selector */}
          {traders && traders.length > 0 && (
            <NofxSelect
              value={selectedTraderId || ''}
              onChange={(val) => onTraderSelect(val)}
              options={traders.map((t) => ({
                value: t.trader_id,
                label: t.trader_name,
              }))}
              className="h-8 rounded-md border border-line bg-surface-2 px-2 text-[13px] font-medium text-fg"
            />
          )}

          {/* Wallet Address Display for Perp-DEX */}
          {exchanges && isPerpDex && (
            <div className="flex h-8 items-center gap-1 rounded-md border border-line bg-surface-2 px-2">
              {walletAddress ? (
                <>
                  <span className="num text-xs text-brand">
                    {showWalletAddress
                      ? walletAddress
                      : truncateAddress(walletAddress)}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-1"
                    onClick={() => setShowWalletAddress(!showWalletAddress)}
                    title={
                      showWalletAddress
                        ? t('traderDashboard.hideAddress', language)
                        : t('traderDashboard.showFullAddress', language)
                    }
                  >
                    {showWalletAddress ? (
                      <EyeOff className="h-3.5 w-3.5" />
                    ) : (
                      <Eye className="h-3.5 w-3.5" />
                    )}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-1"
                    onClick={handleCopyAddress}
                    title={t('traderDashboard.copyAddress', language)}
                  >
                    {copiedAddress ? (
                      <Check className="h-3.5 w-3.5 text-up" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </>
              ) : (
                <span className="text-xs text-fg-3">
                  {t('traderDashboard.noAddressConfigured', language)}
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* KPI strip */}
      <Card dense>
        <CardBody className="grid grid-cols-2 gap-x-4 gap-y-3 md:grid-cols-4 md:divide-x md:divide-line [&>*]:md:pl-4 [&>*:first-child]:md:pl-0">
          {accountLoading ? (
            [1, 2, 3, 4].map((i) => (
              <div key={i}>
                <Skeleton className="mb-2 h-3 w-20" />
                <Skeleton className="h-6 w-28" />
              </div>
            ))
          ) : (
            <>
              <Stat
                label={`${t('totalEquity', language)} (USDT)`}
                value={
                  accountMissing
                    ? '--'
                    : (account?.total_equity?.toFixed(2) ?? '--')
                }
                delta={account ? account.total_pnl_pct || 0 : undefined}
              />
              <Stat
                label={`${t('availableBalance', language)} (USDT)`}
                value={
                  accountMissing
                    ? '--'
                    : (account?.available_balance?.toFixed(2) ?? '--')
                }
                hint={
                  accountMissing
                    ? '--'
                    : `${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '--'}% ${t('free', language)}`
                }
              />
              <Stat
                label={`${t('totalPnL', language)} (USDT)`}
                tone={pnlTone}
                value={
                  accountMissing ? (
                    '--'
                  ) : account?.total_pnl !== undefined ? (
                    <PnL value={account.total_pnl} currency="" />
                  ) : (
                    '--'
                  )
                }
                delta={account ? account.total_pnl_pct || 0 : undefined}
              />
              <Stat
                label={t('positions', language)}
                value={
                  accountMissing ? '--' : `${account?.position_count ?? '--'}`
                }
                hint={
                  accountMissing
                    ? `${t('margin', language)}: --`
                    : `${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) ?? '--'}%`
                }
              />
            </>
          )}
        </CardBody>
      </Card>

      {/* Program-enforced risk state (AI strategies) */}
      {status?.strategy_type !== 'grid_trading' && selectedTraderId && (
        <RiskStatusPanel traderId={selectedTraderId} language={language} />
      )}

      {/* Grid Risk Panel - Only show for grid trading strategy */}
      {status?.strategy_type === 'grid_trading' && selectedTraderId && (
        <GridRiskPanel
          traderId={selectedTraderId}
          language={language}
          refreshInterval={5000}
        />
      )}

      {/* Main area: chart (left) + recent decisions (right) */}
      <div className="grid grid-cols-1 items-start gap-3 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
        {/* Chart Tabs (Equity / K-line) */}
        <Card ref={chartSectionRef} className="scroll-mt-20 overflow-hidden">
          <ChartTabs
            traderId={selectedTrader.trader_id}
            selectedSymbol={selectedChartSymbol}
            updateKey={chartUpdateKey}
            exchangeId={getExchangeTypeFromList(
              selectedTrader.exchange_id,
              exchanges
            )}
          />
        </Card>

        {/* Recent Decisions */}
        <Card className="flex flex-col xl:sticky xl:top-[60px] xl:max-h-[calc(100vh-72px)]">
          <CardHeader
            title={t('recentDecisions', language)}
            subtitle={
              decisions && decisions.length > 0
                ? t('lastCycles', language, { count: decisions.length })
                : undefined
            }
            actions={
              <NofxSelect
                value={decisionsLimit}
                onChange={(val) => onDecisionsLimitChange(Number(val))}
                options={[
                  { value: 5, label: '5' },
                  { value: 10, label: '10' },
                  { value: 20, label: '20' },
                  { value: 50, label: '50' },
                  { value: 100, label: '100' },
                ]}
                className="h-7 rounded-md border border-line bg-surface-2 px-2 text-xs font-medium text-fg"
              />
            }
          />
          <div className="custom-scrollbar max-h-[70vh] min-h-0 flex-1 space-y-2 overflow-y-auto p-3 xl:max-h-none">
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard
                  key={i}
                  decision={decision}
                  language={language}
                  onSymbolClick={handleSymbolClick}
                />
              ))
            ) : decisionsFailed ? (
              <EmptyState
                icon={<AlertTriangle className="h-8 w-8 text-warn" />}
                title={t('traderDashboard.decisionsFetchFailed', language)}
              />
            ) : (
              <EmptyState
                icon={<Brain className="h-8 w-8" />}
                title={t('noDecisionsYet', language)}
                description={t('aiDecisionsWillAppear', language)}
              />
            )}
          </div>
        </Card>
      </div>

      {/* Positions / history */}
      <Card>
        <div className="px-4">
          <Tabs
            items={tabItems}
            value={bottomTab}
            onChange={setBottomTab}
            className="border-b-0"
          />
        </div>
        <div className="border-t border-line">
          {bottomTab === 'positions' ? (
            positions && positions.length > 0 ? (
              <div>
                <DataTable
                  columns={positionColumns}
                  data={paginatedPositions}
                  rowKey={(pos, i) => `${pos.symbol}-${pos.side}-${i}`}
                  onRowClick={(pos) => goToChart(pos.symbol)}
                  maxHeight={520}
                />
                {/* Pagination footer */}
                {totalPositions > 10 && (
                  <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-3 py-2 text-xs text-fg-3">
                    <span>
                      {t('traderDashboard.showingPositions', language, {
                        shown: paginatedPositions.length,
                        total: totalPositions,
                      })}
                    </span>
                    <div className="flex items-center gap-3">
                      <div className="flex items-center gap-2">
                        <span>{t('traderDashboard.perPage', language)}:</span>
                        <NofxSelect
                          value={positionsPageSize}
                          onChange={(val) => setPositionsPageSize(Number(val))}
                          options={[
                            { value: 20, label: '20' },
                            { value: 50, label: '50' },
                            { value: 100, label: '100' },
                          ]}
                          className="h-7 rounded-md border border-line bg-surface-2 px-2 text-xs text-fg"
                        />
                      </div>
                      {totalPositionPages > 1 && (
                        <div className="flex items-center gap-1">
                          {pageBtn(
                            '«',
                            () => setPositionsCurrentPage(1),
                            positionsCurrentPage === 1
                          )}
                          {pageBtn(
                            '‹',
                            () =>
                              setPositionsCurrentPage((p) =>
                                Math.max(1, p - 1)
                              ),
                            positionsCurrentPage === 1
                          )}
                          <span className="num px-2 text-fg">
                            {positionsCurrentPage} / {totalPositionPages}
                          </span>
                          {pageBtn(
                            '›',
                            () =>
                              setPositionsCurrentPage((p) =>
                                Math.min(totalPositionPages, p + 1)
                              ),
                            positionsCurrentPage === totalPositionPages
                          )}
                          {pageBtn(
                            '»',
                            () => setPositionsCurrentPage(totalPositionPages),
                            positionsCurrentPage === totalPositionPages
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            ) : positionsFailed ? (
              <EmptyState
                icon={<AlertTriangle className="h-8 w-8 text-warn" />}
                title={t('traderDashboard.positionsFetchFailed', language)}
              />
            ) : (
              <EmptyState
                title={t('noPositions', language)}
                description={t('noActivePositions', language)}
              />
            )
          ) : (
            selectedTraderId && <PositionHistory traderId={selectedTraderId} />
          )}
        </div>
      </Card>
    </div>
  )
}
