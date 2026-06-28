import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  AlertCircle,
  ArrowRight,
  CircleDollarSign,
  Download,
  CheckCircle2,
  Copy,
  ExternalLink,
  KeyRound,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Wallet,
  Zap,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../lib/api'
import { buildDashboardPath, ROUTES } from '../../router/paths'
import type {
  AIModel,
  CurrentBeginnerWalletResponse,
  Exchange,
  ExchangeAccountState,
  TraderInfo,
} from '../../types'
import { HyperliquidWalletConnect } from '../common/HyperliquidWalletConnect'

type LaunchStepStatus = 'ready' | 'action' | 'blocked'

interface AutopilotLaunchPanelProps {
  models: AIModel[]
  exchanges: Exchange[]
  exchangeAccountStates: Record<string, ExchangeAccountState>
  traders?: TraderInfo[]
  isLoggedIn: boolean
  language: string
  onRefresh: () => Promise<void>
  onOpenClaw402Config?: () => void
  onOpenHyperliquidConfig?: () => void
}

const MIN_AI_FEE_USDC = 1
const MIN_TRADING_USDC = 12

function parseNumber(value?: string | number) {
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0
  if (!value) return 0
  const parsed = Number(value.replace(/[,$\s]/g, ''))
  return Number.isFinite(parsed) ? parsed : 0
}

function shortAddress(address?: string) {
  if (!address) return '--'
  return `${address.slice(0, 6)}…${address.slice(-4)}`
}

function formatUSDC(value: number) {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

async function copyText(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value)
    toast.success(`${label} copied`)
  } catch {
    toast.error('Copy failed')
  }
}

function BeginnerHyperliquidGuide({
  hasInjectedWallet,
}: {
  hasInjectedWallet: boolean
}) {
  const steps = [
    {
      title: 'Prepare an EVM wallet',
      detail: hasInjectedWallet
        ? 'Wallet extension detected. Unlock it, then connect below.'
        : 'Install Rabby or MetaMask, create or import a wallet, then return here.',
      icon: Wallet,
    },
    {
      title: 'Open Hyperliquid',
      detail:
        'Use the same wallet on Hyperliquid. Deposit USDC there as trading collateral.',
      icon: CircleDollarSign,
    },
    {
      title: 'Authorize NOFX',
      detail:
        'Back in NOFX, approve the Agent and builder fee. NOFX stores the Agent key, not your main wallet key.',
      icon: KeyRound,
    },
  ]

  return (
    <div className="rounded-xl border border-nofx-gold/20 bg-[linear-gradient(180deg,rgba(240,185,11,0.1),rgba(0,0,0,0.16))] p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="text-sm font-semibold text-white">
            New to Hyperliquid?
          </div>
          <p className="mt-1 text-xs leading-5 text-nofx-text-muted">
            Start here if you do not have a trading wallet or have never used
            Hyperliquid before.
          </p>
        </div>
        <div
          className={`w-fit rounded-full px-2.5 py-1 text-[11px] font-semibold ${
            hasInjectedWallet
              ? 'bg-emerald-500/10 text-emerald-300'
              : 'bg-nofx-gold/10 text-nofx-gold'
          }`}
        >
          {hasInjectedWallet ? 'Wallet detected' : 'Wallet needed'}
        </div>
      </div>

      <div className="mt-4 grid gap-3">
        {steps.map((step, index) => {
          const Icon = step.icon
          return (
            <div key={step.title} className="flex gap-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-black/25 text-nofx-gold">
                <Icon className="h-3.5 w-3.5" />
              </div>
              <div>
                <div className="text-sm font-semibold text-zinc-100">
                  {index + 1}. {step.title}
                </div>
                <p className="mt-0.5 text-xs leading-5 text-nofx-text-muted">
                  {step.detail}
                </p>
              </div>
            </div>
          )
        })}
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {!hasInjectedWallet ? (
          <>
            <a
              href="https://rabby.io/"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-2 text-xs font-semibold text-zinc-200 hover:border-white/20 hover:bg-white/[0.07]"
            >
              <Download className="h-3.5 w-3.5" />
              Install Rabby
            </a>
            <a
              href="https://metamask.io/download/"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-2 text-xs font-semibold text-zinc-200 hover:border-white/20 hover:bg-white/[0.07]"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              MetaMask
            </a>
          </>
        ) : null}
        <a
          href="https://app.hyperliquid.xyz/"
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-2 rounded-lg bg-nofx-gold px-3 py-2 text-xs font-bold text-black hover:bg-yellow-400"
        >
          Open Hyperliquid
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      </div>
    </div>
  )
}

export function AutopilotLaunchPanel({
  models,
  exchanges,
  exchangeAccountStates,
  traders = [],
  isLoggedIn,
  language,
  onRefresh,
  onOpenClaw402Config,
  onOpenHyperliquidConfig,
}: AutopilotLaunchPanelProps) {
  const navigate = useNavigate()
  const [wallet, setWallet] = useState<CurrentBeginnerWalletResponse | null>(
    null
  )
  const [walletLoading, setWalletLoading] = useState(false)
  const [launching, setLaunching] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [hasInjectedWallet, setHasInjectedWallet] = useState(false)
  const isZh = language === 'zh'

  useEffect(() => {
    setHasInjectedWallet(
      typeof window !== 'undefined' &&
        Boolean((window as Window & { ethereum?: unknown }).ethereum)
    )
  }, [])

  const claw402Model = useMemo(
    () =>
      models.find(
        (model) =>
          model.provider === 'claw402' &&
          model.enabled &&
          (model.has_api_key || model.apiKey || model.walletAddress)
      ) || null,
    [models]
  )

  const feeWalletAddress = claw402Model?.walletAddress || wallet?.address || ''
  const feeWalletBalance = parseNumber(
    claw402Model?.balanceUsdc || wallet?.balance_usdc
  )
  const feeReady =
    Boolean(feeWalletAddress) && feeWalletBalance >= MIN_AI_FEE_USDC

  const hyperliquidExchange = useMemo(
    () =>
      exchanges.find(
        (exchange) =>
          exchange.exchange_type === 'hyperliquid' &&
          exchange.enabled &&
          Boolean(exchange.hyperliquidWalletAddr) &&
          Boolean(exchange.hyperliquidBuilderApproved)
      ) || null,
    [exchanges]
  )

  const hyperliquidConnected = Boolean(hyperliquidExchange)
  const exchangeState = hyperliquidExchange
    ? exchangeAccountStates[hyperliquidExchange.id]
    : undefined
  const tradingBalance = parseNumber(
    exchangeState?.available_balance ?? exchangeState?.total_equity
  )
  const tradingBalanceReady =
    hyperliquidConnected &&
    exchangeState?.status === 'ok' &&
    tradingBalance >= MIN_TRADING_USDC

  const autopilotTrader = useMemo(
    () =>
      traders.find((trader) => trader.trader_name === 'NOFX Autopilot') ||
      traders.find((trader) =>
        (trader.strategy_name || '').toLowerCase().includes('claw402')
      ) ||
      null,
    [traders]
  )

  const allReady = feeReady && hyperliquidConnected && tradingBalanceReady

  const loadWallet = async () => {
    setWalletLoading(true)
    try {
      setWallet(await api.getCurrentBeginnerWallet())
    } catch {
      setWallet(null)
    } finally {
      setWalletLoading(false)
    }
  }

  useEffect(() => {
    void loadWallet()
  }, [])

  const refreshEverything = async () => {
    setRefreshing(true)
    try {
      await Promise.all([onRefresh(), loadWallet()])
    } finally {
      setRefreshing(false)
    }
  }

  const ensureClaw402Strategy = async () => {
    const strategies = await api.getStrategies()
    const existing =
      strategies.find(
        (strategy) =>
          strategy.is_active &&
          strategy.config?.ai_config?.coin_source?.source_type ===
            'vergex_signal'
      ) ||
      strategies.find((strategy) =>
        strategy.name.toLowerCase().includes('claw402')
      )

    if (existing) {
      if (!existing.is_active) {
        await api.activateStrategy(existing.id)
      }
      return existing.id
    }

    const config = await api.getDefaultStrategyConfig()
    const created = await api.createStrategy({
      name: 'NOFX Claw402 Auto Strategy',
      description:
        'Single built-in strategy: Claw402 board, per-symbol details, raw candles, then execution.',
      config,
    })
    if (created?.id) {
      await api.activateStrategy(created.id)
      return created.id
    }

    const refreshed = await api.getStrategies()
    const fallback = refreshed.find((strategy) =>
      strategy.name.toLowerCase().includes('claw402')
    )
    if (!fallback) throw new Error('Failed to create Claw402 strategy')
    await api.activateStrategy(fallback.id)
    return fallback.id
  }

  const launchAutopilot = async () => {
    if (!claw402Model || !hyperliquidExchange) return
    setLaunching(true)
    try {
      let trader = autopilotTrader
      if (!trader) {
        const strategyId = await ensureClaw402Strategy()
        trader = await api.createTrader({
          name: 'NOFX Autopilot',
          ai_model_id: claw402Model.id,
          exchange_id: hyperliquidExchange.id,
          strategy_id: strategyId,
          scan_interval_minutes: 15,
          is_cross_margin: true,
          show_in_competition: true,
          btc_eth_leverage: 10,
          altcoin_leverage: 10,
        })
      }
      if (!trader.is_running) {
        await api.startTrader(trader.trader_id)
      }
      await onRefresh()
      toast.success('NOFX Autopilot is running')
      navigate(buildDashboardPath(trader.trader_id))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Launch failed')
    } finally {
      setLaunching(false)
    }
  }

  const steps: Array<{
    title: string
    detail: string
    status: LaunchStepStatus
    meta?: string
    action?: JSX.Element
  }> = [
    {
      title: 'AI fee wallet',
      detail:
        'Pays Claw402.ai data and model calls with Base USDC. This is separate from trading collateral.',
      status: feeReady ? 'ready' : 'action',
      meta: feeWalletAddress
        ? `${shortAddress(feeWalletAddress)} · ${formatUSDC(feeWalletBalance)} USDC`
        : 'Base USDC wallet required',
      action: feeWalletAddress ? (
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => onOpenClaw402Config?.()}
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-nofx-gold hover:text-yellow-300"
          >
            <CircleDollarSign className="h-3.5 w-3.5" />
            Deposit
          </button>
          <button
            type="button"
            onClick={() => void copyText(feeWalletAddress, 'AI fee wallet')}
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-nofx-gold hover:text-yellow-300"
          >
            <Copy className="h-3.5 w-3.5" />
            Copy
          </button>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => onOpenClaw402Config?.()}
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-nofx-gold hover:text-yellow-300"
        >
          <ArrowRight className="h-3.5 w-3.5" />
          Open
        </button>
      ),
    },
    {
      title: 'Hyperliquid trading wallet',
      detail:
        'Connect an EVM wallet, approve a NOFX Agent, approve the builder fee, then save it to NOFX.',
      status: hyperliquidConnected ? 'ready' : 'action',
      meta: hyperliquidExchange?.hyperliquidWalletAddr
        ? `${shortAddress(hyperliquidExchange.hyperliquidWalletAddr)} · authorized`
        : 'Agent and trading authorization required',
      action: (
        <button
          type="button"
          onClick={() => onOpenHyperliquidConfig?.()}
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-nofx-gold hover:text-yellow-300"
        >
          <Wallet className="h-3.5 w-3.5" />
          Open
        </button>
      ),
    },
    {
      title: 'Trading balance',
      detail:
        'Deposit USDC to Hyperliquid. NOFX uses it as margin for the Claw402 Autopilot strategy.',
      status: tradingBalanceReady
        ? 'ready'
        : hyperliquidConnected
          ? 'action'
          : 'blocked',
      meta: hyperliquidConnected
        ? `${formatUSDC(tradingBalance)} USDC available`
        : 'Connect Hyperliquid first',
    },
    {
      title: 'NOFX Autopilot',
      detail:
        'Reads the Claw402 board, fetches Signal Lab and liquidation structure, confirms with candles, then trades full-size 10x only when the setup is strong enough.',
      status: allReady ? 'ready' : 'blocked',
      meta: autopilotTrader?.is_running
        ? 'Running'
        : autopilotTrader
          ? 'Ready to start'
          : 'Ready to create when setup is complete',
    },
  ]

  const renderPrimaryAction = () => {
    if (!feeReady) {
      return (
        <button
          type="button"
          onClick={() => {
            if (onOpenClaw402Config) {
              onOpenClaw402Config()
            } else {
              navigate(ROUTES.welcome)
            }
          }}
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-nofx-gold px-4 py-3 text-sm font-bold text-black hover:bg-yellow-400"
        >
          Open Claw402 wallet
          <ArrowRight className="h-4 w-4" />
        </button>
      )
    }

    if (!hyperliquidConnected) {
      return (
        <button
          type="button"
          onClick={() => {
            if (onOpenHyperliquidConfig) {
              onOpenHyperliquidConfig()
            } else {
              document
                .getElementById('hyperliquid-quick-connect')
                ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
            }
          }}
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-nofx-gold px-4 py-3 text-sm font-bold text-black hover:bg-yellow-400"
        >
          Connect Hyperliquid
          <ArrowRight className="h-4 w-4" />
        </button>
      )
    }

    if (!tradingBalanceReady) {
      return (
        <a
          href="https://app.hyperliquid.xyz/"
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-nofx-gold px-4 py-3 text-sm font-bold text-black hover:bg-yellow-400"
        >
          Deposit USDC on Hyperliquid
          <ExternalLink className="h-4 w-4" />
        </a>
      )
    }

    if (autopilotTrader?.is_running) {
      return (
        <button
          type="button"
          onClick={() =>
            navigate(buildDashboardPath(autopilotTrader.trader_id))
          }
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-emerald-400 px-4 py-3 text-sm font-bold text-black hover:bg-emerald-300"
        >
          Open dashboard
          <ArrowRight className="h-4 w-4" />
        </button>
      )
    }

    return (
      <button
        type="button"
        onClick={launchAutopilot}
        disabled={launching || !allReady}
        className="inline-flex items-center justify-center gap-2 rounded-lg bg-nofx-gold px-4 py-3 text-sm font-bold text-black hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {launching ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          <Zap className="h-4 w-4" />
        )}
        Start NOFX Autopilot
      </button>
    )
  }

  return (
    <section className="overflow-hidden rounded-xl border border-nofx-gold/20 bg-[linear-gradient(135deg,rgba(20,17,7,0.92),rgba(8,11,16,0.9)_42%,rgba(7,14,18,0.88))] shadow-[0_20px_80px_rgba(0,0,0,0.28)]">
      <div className="grid gap-0 xl:grid-cols-[1.05fr_0.95fr]">
        <div className="p-5 md:p-6">
          <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-nofx-gold/25 bg-nofx-gold/10 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-nofx-gold">
                <ShieldCheck className="h-3.5 w-3.5" />
                Guided Launch
              </div>
              <h2 className="text-2xl font-bold tracking-tight text-white md:text-3xl">
                Start NOFX Autopilot in minutes
              </h2>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-nofx-text-muted">
                One strategy, one launch path. Fund the AI fee wallet, authorize
                Hyperliquid, deposit USDC, then run the Claw402 Autopilot.
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => void refreshEverything()}
                disabled={refreshing || walletLoading}
                className="inline-flex items-center justify-center gap-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-2 text-xs font-semibold text-nofx-text-muted hover:text-white disabled:opacity-60"
              >
                <RefreshCw
                  className={`h-3.5 w-3.5 ${refreshing || walletLoading ? 'animate-spin' : ''}`}
                />
                Refresh
              </button>
              {renderPrimaryAction()}
            </div>
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            {steps.map((step, index) => (
              <div
                key={step.title}
                className="rounded-lg border border-white/10 bg-black/20 p-4"
              >
                <div className="flex items-start gap-3">
                  <div
                    className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-sm font-bold ${
                      step.status === 'ready'
                        ? 'border-emerald-400/30 bg-emerald-500/15 text-emerald-300'
                        : step.status === 'action'
                          ? 'border-nofx-gold/30 bg-nofx-gold/15 text-nofx-gold'
                          : 'border-white/10 bg-white/[0.04] text-nofx-text-muted'
                    }`}
                  >
                    {step.status === 'ready' ? (
                      <CheckCircle2 className="h-4 w-4" />
                    ) : step.status === 'action' ? (
                      index + 1
                    ) : (
                      <AlertCircle className="h-4 w-4" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <h3 className="font-semibold text-white">{step.title}</h3>
                      {step.action}
                    </div>
                    <p className="mt-1 text-xs leading-5 text-nofx-text-muted">
                      {step.detail}
                    </p>
                    <div className="mt-3 font-mono text-xs text-nofx-gold/90">
                      {step.meta}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        <aside className="border-t border-white/10 bg-black/20 p-5 md:p-6 xl:border-l xl:border-t-0">
          <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-white">
            <Wallet className="h-4 w-4 text-nofx-gold" />
            Hyperliquid setup
          </div>
          {hyperliquidConnected ? (
            <div className="rounded-lg border border-emerald-400/25 bg-emerald-500/10 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-emerald-200">
                <CheckCircle2 className="h-4 w-4" />
                Trading authorization is ready
              </div>
              <div className="mt-2 font-mono text-xs text-emerald-100/80">
                {shortAddress(hyperliquidExchange?.hyperliquidWalletAddr)}
              </div>
              <p className="mt-3 text-xs leading-5 text-emerald-100/70">
                Funds stay in your Hyperliquid account. NOFX only stores the
                authorized Agent key required for automated execution.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              <BeginnerHyperliquidGuide hasInjectedWallet={hasInjectedWallet} />
              <div id="hyperliquid-quick-connect">
                <HyperliquidWalletConnect
                  language={isZh ? 'zh' : 'en'}
                  isLoggedIn={isLoggedIn}
                  variant="inline"
                  onSaved={refreshEverything}
                />
              </div>
            </div>
          )}
        </aside>
      </div>
    </section>
  )
}
