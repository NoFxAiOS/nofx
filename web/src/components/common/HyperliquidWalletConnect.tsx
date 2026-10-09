import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Check,
  ChevronDown,
  Copy,
  Download,
  ExternalLink,
  Loader2,
  RefreshCw,
  Shield,
  Wallet,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../lib/api'
import type {
  HyperliquidAccountSummary,
  HyperliquidAgentInfo,
} from '../../lib/api/wallet'
import type { Language } from '../../i18n/translations'
import {
  formatUSDC,
  ensureWalletSigningAccount,
  getWalletErrorMessage,
  getWalletProviderName,
  getWalletProviderForAddress,
  normalizeAddress,
  shortAddress,
  signHyperliquidUserAction,
  subscribeWalletProviders,
  type WalletProvider,
} from '../../lib/hyperliquidWallet'

interface HyperliquidWalletConnectProps {
  language: Language
  isLoggedIn: boolean
  variant?: 'dropdown' | 'inline'
  onSaved?: () => void | Promise<void>
}

interface FlowState {
  mainWallet?: string
  agentAddress?: string
  agentPrivateKey?: string
  agentApproved?: boolean
  builderApproved?: boolean
  savedExchangeId?: string
  reusedSavedExchange?: boolean
}

interface ServerReadyProof {
  exchangeId: string
  wallet: string
  agent: string
}

const STORAGE_KEY = 'nofx.hyperliquid.connection.v6'
const AGENT_NAME = 'NOFX Agent'
// Hyperliquid caps agent validity at 180 days and otherwise defaults to ~90 days.
// The validity is encoded in the agent name as a " valid_until <ms>" suffix
// (separator is a single space; timestamp in milliseconds). Hyperliquid strips
// this suffix from the stored/displayed name, so the named slot stays "NOFX Agent".
// A 1-minute buffer keeps clock skew from pushing valid_until past the 180d cap.
const AGENT_VALIDITY_MS = 180 * 24 * 60 * 60 * 1000 - 60 * 1000

function buildAgentName(nowMs: number) {
  return `${AGENT_NAME} valid_until ${nowMs + AGENT_VALIDITY_MS}`
}
const HYPERLIQUID_BUILDER_ADDRESS = '0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d'
// 0.05% (5 bps). Must match the server's defaultHyperliquidBuilderMaxFee and
// the BuilderInfo.Fee=50 (= 5 bps) used at order placement. The user signs
// this exact string when approving the builder during wallet connect.
const HYPERLIQUID_BUILDER_MAX_FEE = '0.05%'

function copy(text: string, label: string) {
  navigator.clipboard?.writeText(text).then(
    () => toast.success(`${label} copied`),
    () => toast.error('Copy failed')
  )
}

function formatAgentExpiry(validUntil: number, language: Language) {
  const dateStr = new Date(validUntil).toLocaleString(
    language === 'zh' ? 'zh-CN' : 'en-US',
    {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }
  )
  const daysLeft = Math.ceil((validUntil - Date.now()) / 86_400_000)
  return { dateStr, daysLeft }
}

function formatSignedUSDC(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '--'
  const sign = value > 0 ? '+' : ''
  return `${sign}${formatUSDC(value)}`
}

function getSavedState(): FlowState {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

function saveState(state: FlowState) {
  const safeState = { ...state }
  if (safeState.savedExchangeId) {
    delete safeState.agentPrivateKey
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(safeState))
}

export function HyperliquidWalletConnect({
  language,
  isLoggedIn,
  variant = 'dropdown',
  onSaved,
}: HyperliquidWalletConnectProps) {
  const inline = variant === 'inline'
  const [open, setOpen] = useState(inline)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [state, setState] = useState<FlowState>(() => getSavedState())
  const currentMainWalletRef = useRef(state.mainWallet)
  const reconciliationGenerationRef = useRef(0)
  const agentRefreshGenerationRef = useRef(0)
  const serverProofGenerationRef = useRef(0)
  const walletProviderRef = useRef<WalletProvider>()
  currentMainWalletRef.current = state.mainWallet
  const [account, setAccount] = useState<HyperliquidAccountSummary | null>(null)
  const [balanceLoading, setBalanceLoading] = useState(false)
  const [balanceError, setBalanceError] = useState('')
  const [agentInfo, setAgentInfo] = useState<HyperliquidAgentInfo | null>(null)
  const [agentInfoLoading, setAgentInfoLoading] = useState(false)
  const [hasWalletProvider, setHasWalletProvider] = useState(false)
  const [walletProviders, setWalletProviders] = useState<WalletProvider[]>([])
  const [selectedWalletProvider, setSelectedWalletProvider] =
    useState<WalletProvider>()
  // Exact server config whose saved agent and builder authorization were
  // verified live. Readiness must bind all three identities, not just wallet.
  const [serverReadyProof, setServerReadyProof] =
    useState<ServerReadyProof | null>(null)
  const text = useMemo(
    () => ({
      title: language === 'zh' ? 'Hyperliquid Wallet' : 'Hyperliquid Wallet',
      connect:
        language === 'zh' ? 'Connect Hyperliquid' : 'Connect Hyperliquid',
      connected: language === 'zh' ? 'Connected' : 'Connected',
      mainWallet:
        language === 'zh' ? 'Connect your wallet' : 'Connect your wallet',
      generateAgent:
        language === 'zh'
          ? 'Create a trading key for NOFX'
          : 'Create a trading key for NOFX',
      approveAgent:
        language === 'zh'
          ? 'Approve it in your wallet (trade-only, cannot withdraw)'
          : 'Approve it in your wallet (trade-only, cannot withdraw)',
      approveBuilder:
        language === 'zh'
          ? 'Approve the small per-trade builder fee'
          : 'Approve the small per-trade builder fee',
      save: language === 'zh' ? 'Save to NOFX — done' : 'Save to NOFX — done',
      done:
        language === 'zh'
          ? 'All set — trading authorized'
          : 'All set — trading authorized',
      balance:
        language === 'zh' ? 'Hyperliquid balance' : 'Hyperliquid balance',
      withdrawable: language === 'zh' ? 'Withdrawable' : 'Withdrawable',
      equity: language === 'zh' ? 'Equity' : 'Equity',
      marginUsed: language === 'zh' ? 'Margin used' : 'Margin used',
      unrealizedPnl: language === 'zh' ? 'Unrealized PnL' : 'Unrealized PnL',
      refresh: language === 'zh' ? 'Refresh' : 'Refresh',
      noCustody:
        language === 'zh'
          ? 'Funds stay in your Hyperliquid account; NOFX only stores the authorized agent wallet.'
          : 'Funds stay in your Hyperliquid account; NOFX only stores the authorized agent wallet.',
      agentExpiry:
        language === 'zh'
          ? 'Agent authorization expires'
          : 'Agent authorization expires',
      agentExpired: language === 'zh' ? 'Expired' : 'Expired',
      agentNoAuth:
        language === 'zh'
          ? 'No NOFX agent authorization found'
          : 'No NOFX agent authorization found',
      renewAgent:
        language === 'zh'
          ? 'Renew agent authorization (+180d)'
          : 'Renew agent authorization (+180d)',
      renewHint:
        language === 'zh'
          ? 'Hyperliquid forbids reusing an agent, so renewal creates a new agent approved for 180 days, then updates the stored key in NOFX (sign-in required).'
          : 'Hyperliquid forbids reusing an agent, so renewal creates a new agent approved for 180 days, then updates the stored key in NOFX (sign-in required).',
      noWalletTitle:
        language === 'zh' ? 'No EVM wallet detected' : 'No EVM wallet detected',
      noWalletDetail:
        language === 'zh'
          ? 'Install Rabby or MetaMask, create or import a wallet, then return here to connect Hyperliquid.'
          : 'Install Rabby or MetaMask, create or import a wallet, then return here to connect Hyperliquid.',
      installRabby: language === 'zh' ? 'Install Rabby' : 'Install Rabby',
      installMetaMask:
        language === 'zh' ? 'Install MetaMask' : 'Install MetaMask',
    }),
    [language]
  )

  useEffect(() => {
    return subscribeWalletProviders((providers) => {
      setWalletProviders(providers)
      setHasWalletProvider(providers.length > 0)
      setSelectedWalletProvider((selected) => {
        if (selected && providers.includes(selected)) return selected
        if (currentMainWalletRef.current && providers.length === 1) {
          walletProviderRef.current = providers[0]
          return providers[0]
        }
        walletProviderRef.current = undefined
        return undefined
      })
    })
  }, [])

  useEffect(() => {
    saveState(state)
  }, [state])

  useEffect(() => {
    if (!isLoggedIn) {
      serverProofGenerationRef.current++
      setServerReadyProof(null)
      return
    }
    const generation = ++serverProofGenerationRef.current
    let cancelled = false
    void (async () => {
      try {
        const configs = await api.getExchangeConfigs()
        const currentWallet = normalizeAddress(state.mainWallet || '')
        let candidates = configs.filter(
          (exchange) =>
            exchange.exchange_type === 'hyperliquid' &&
            exchange.enabled &&
            Boolean(exchange.hyperliquidBuilderApproved) &&
            Boolean(exchange.hyperliquidAgentAddress) &&
            (exchange.hyperliquidWalletAddr || '').trim() !== '' &&
            (!currentWallet ||
              normalizeAddress(exchange.hyperliquidWalletAddr || '') ===
                currentWallet)
        )
        candidates = state.savedExchangeId
          ? candidates.filter(
              (exchange) => exchange.id === state.savedExchangeId
            )
          : candidates.length === 1
            ? candidates
            : []
        for (const exchange of candidates) {
          try {
            const response = await api.getHyperliquidAgent(
              exchange.hyperliquidWalletAddr!
            )
            const savedAgentAddress = normalizeAddress(
              exchange.hyperliquidAgentAddress || ''
            )
            const approved = response.agents.some(
              (agent) =>
                normalizeAddress(agent.address) === savedAgentAddress &&
                agent.validUntil > Date.now()
            )
            if (approved && response.builderApproved) {
              if (
                !cancelled &&
                serverProofGenerationRef.current === generation
              ) {
                setServerReadyProof({
                  exchangeId: exchange.id,
                  wallet: exchange.hyperliquidWalletAddr || '',
                  agent: exchange.hyperliquidAgentAddress || '',
                })
              }
              return
            }
          } catch {
            continue
          }
        }
        if (!cancelled && serverProofGenerationRef.current === generation)
          setServerReadyProof(null)
      } catch {
        if (!cancelled && serverProofGenerationRef.current === generation)
          setServerReadyProof(null)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [isLoggedIn, state.mainWallet, state.savedExchangeId])

  useEffect(() => {
    if (!isLoggedIn || !state.mainWallet) return
    const mainWallet = state.mainWallet
    const generation = ++reconciliationGenerationRef.current
    let cancelled = false
    void (async () => {
      try {
        const [configs, agentResponse] = await Promise.all([
          api.getExchangeConfigs(),
          api.getHyperliquidAgent(mainWallet),
        ])
        if (cancelled || reconciliationGenerationRef.current !== generation)
          return
        const eligible = configs.filter(
          (exchange) =>
            exchange.exchange_type === 'hyperliquid' &&
            exchange.enabled &&
            normalizeAddress(exchange.hyperliquidWalletAddr || '') ===
              normalizeAddress(mainWallet)
        )
        const existing = state.savedExchangeId
          ? eligible.find((exchange) => exchange.id === state.savedExchangeId)
          : eligible.length === 1
            ? eligible[0]
            : undefined
        if (!existing) {
          setAgentInfo(null)
          setState((prev) =>
            normalizeAddress(prev.mainWallet || '') ===
            normalizeAddress(mainWallet)
              ? {
                  ...prev,
                  agentAddress: undefined,
                  agentPrivateKey: undefined,
                  agentApproved: false,
                  builderApproved: false,
                  savedExchangeId: undefined,
                  reusedSavedExchange: false,
                }
              : prev
          )
          return
        }
        const savedAgentAddress = normalizeAddress(
          existing.hyperliquidAgentAddress || ''
        )
        const approvedAgent = savedAgentAddress
          ? agentResponse.agents.find(
              (agent) =>
                normalizeAddress(agent.address) === savedAgentAddress &&
                agent.validUntil > Date.now()
            ) || null
          : null
        setAgentInfo(approvedAgent)
        setState((prev) => {
          if (
            normalizeAddress(prev.mainWallet || '') !==
            normalizeAddress(mainWallet)
          ) {
            return prev
          }
          return {
            ...prev,
            agentAddress: savedAgentAddress || undefined,
            agentPrivateKey: undefined,
            agentApproved: Boolean(approvedAgent),
            builderApproved: Boolean(
              existing.hyperliquidBuilderApproved &&
              agentResponse.builderApproved
            ),
            savedExchangeId: existing.id,
            reusedSavedExchange: true,
          }
        })
      } catch {
        if (!cancelled) {
          setState((prev) =>
            normalizeAddress(prev.mainWallet || '') ===
            normalizeAddress(mainWallet)
              ? { ...prev, agentApproved: false }
              : prev
          )
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [isLoggedIn, state.mainWallet, state.savedExchangeId])

  useEffect(() => {
    const subscriptions = walletProviders.map((provider) => {
      const handler = (accounts: unknown) => {
        if (walletProviderRef.current !== provider) return
        const next =
          Array.isArray(accounts) && typeof accounts[0] === 'string'
            ? normalizeAddress(accounts[0])
            : undefined
        if (
          normalizeAddress(currentMainWalletRef.current || '') === (next || '')
        ) {
          return
        }
        currentMainWalletRef.current = next
        setState(next ? { mainWallet: next } : {})
        setAccount(null)
        setAgentInfo(null)
        setBalanceError('')
        setError(
          next
            ? 'Wallet account changed. Review and restart authorization.'
            : 'Wallet disconnected. Connect a wallet to continue.'
        )
      }
      provider.on?.('accountsChanged', handler)
      return { provider, handler }
    })
    return () => {
      subscriptions.forEach(({ provider, handler }) =>
        provider.removeListener?.('accountsChanged', handler)
      )
    }
  }, [walletProviders])

  useEffect(() => {
    if (open && state.mainWallet) {
      void refreshBalance(state.mainWallet)
      void refreshAgentInfo(state.mainWallet)
    }
  }, [open, state.mainWallet])

  async function refreshAgentInfo(
    address = state.mainWallet,
    expectedAgentAddress = state.agentAddress,
    expectedBuilderApproved = state.builderApproved,
    expectedExchangeId = state.savedExchangeId
  ): Promise<boolean> {
    if (!address) return false
    // A direct live refresh supersedes every older readiness lookup, not just
    // older calls to this function.
    reconciliationGenerationRef.current++
    serverProofGenerationRef.current++
    const generation = ++agentRefreshGenerationRef.current
    const requestedAddress = normalizeAddress(address)
    setAgentInfoLoading(true)
    setServerReadyProof(null)
    if (
      normalizeAddress(currentMainWalletRef.current || '') === requestedAddress
    ) {
      setAgentInfo(null)
    }
    try {
      const res = await api.getHyperliquidAgent(address)
      if (agentRefreshGenerationRef.current !== generation) return false
      const savedAgentAddress = normalizeAddress(expectedAgentAddress || '')
      const approvedAgent = savedAgentAddress
        ? res.agents.find(
            (agent) =>
              normalizeAddress(agent.address) === savedAgentAddress &&
              agent.validUntil > Date.now()
          ) || null
        : null
      const agentApproved = Boolean(approvedAgent)
      const builderApproved = Boolean(
        expectedBuilderApproved && res.builderApproved
      )
      const liveReady = agentApproved && builderApproved
      if (
        normalizeAddress(currentMainWalletRef.current || '') ===
        requestedAddress
      ) {
        setAgentInfo(approvedAgent)
        setState((prev) => {
          if (
            normalizeAddress(prev.mainWallet || '') !== requestedAddress ||
            normalizeAddress(prev.agentAddress || '') !== savedAgentAddress
          ) {
            return prev
          }
          return {
            ...prev,
            agentApproved,
            builderApproved,
          }
        })
        setServerReadyProof(
          liveReady && expectedExchangeId && expectedAgentAddress
            ? {
                exchangeId: expectedExchangeId,
                wallet: address,
                agent: expectedAgentAddress,
              }
            : null
        )
      }
      return liveReady
    } catch {
      if (
        normalizeAddress(currentMainWalletRef.current || '') ===
        requestedAddress
      ) {
        setAgentInfo(null)
        setServerReadyProof(null)
        setState((prev) =>
          normalizeAddress(prev.mainWallet || '') === requestedAddress
            ? { ...prev, agentApproved: false, builderApproved: false }
            : prev
        )
      }
      return false
    } finally {
      if (
        normalizeAddress(currentMainWalletRef.current || '') ===
        requestedAddress
      ) {
        setAgentInfoLoading(false)
      }
    }
  }

  async function refreshBalance(address = state.mainWallet) {
    if (!address) return
    setBalanceLoading(true)
    setBalanceError('')
    try {
      const summary = await api.getHyperliquidAccount(address)
      setAccount(summary)
    } catch (err) {
      setAccount(null)
      setBalanceError(
        err instanceof Error
          ? err.message
          : 'Failed to load Hyperliquid balance'
      )
    } finally {
      setBalanceLoading(false)
    }
  }

  async function reuseSavedExchangeIfPresent(address: string) {
    if (!isLoggedIn) return false
    try {
      const [configs, agentResponse] = await Promise.all([
        api.getExchangeConfigs(),
        api.getHyperliquidAgent(address),
      ])
      const eligible = configs.filter(
        (exchange) =>
          exchange.exchange_type === 'hyperliquid' &&
          exchange.enabled &&
          normalizeAddress(exchange.hyperliquidWalletAddr || '') ===
            normalizeAddress(address)
      )
      const existing = serverReadyProof?.exchangeId
        ? eligible.find(
            (exchange) => exchange.id === serverReadyProof.exchangeId
          )
        : eligible.length === 1
          ? eligible[0]
          : undefined
      if (!existing) return false
      const savedAgentAddress = normalizeAddress(
        existing.hyperliquidAgentAddress || ''
      )
      const approvedAgent = savedAgentAddress
        ? agentResponse.agents.find(
            (agent) =>
              normalizeAddress(agent.address) === savedAgentAddress &&
              agent.validUntil > Date.now()
          ) || null
        : null
      setAgentInfo(approvedAgent)
      setState((prev) => {
        if (
          normalizeAddress(prev.mainWallet || '') !== normalizeAddress(address)
        ) {
          return prev
        }
        return {
          ...prev,
          agentAddress: savedAgentAddress || undefined,
          agentPrivateKey: undefined,
          agentApproved: Boolean(approvedAgent),
          // Existing configs default to false in the backend unless the exact
          // approveBuilderFee flow has already persisted a successful approval.
          builderApproved: Boolean(
            existing.hyperliquidBuilderApproved && agentResponse.builderApproved
          ),
          savedExchangeId: existing.id,
          reusedSavedExchange: true,
        }
      })
      return true
    } catch {
      return false
    }
  }

  const agentReady = Boolean(state.agentAddress)
  const agentApprovedReady = Boolean(state.agentApproved)
  const builderReady = Boolean(state.builderApproved)

  const complete = Boolean(
    state.mainWallet &&
    state.savedExchangeId &&
    state.agentApproved &&
    state.builderApproved &&
    serverReadyProof?.exchangeId === state.savedExchangeId &&
    normalizeAddress(serverReadyProof?.wallet || '') ===
      normalizeAddress(state.mainWallet) &&
    normalizeAddress(serverReadyProof?.agent || '') ===
      normalizeAddress(state.agentAddress || '')
  )
  // Trigger shows "connected" when either this browser finished the flow or
  // the server already holds a fully-authorized exchange.
  const connectedAddr = complete
    ? state.mainWallet
    : serverReadyProof?.wallet || undefined
  const connectionProgress = [
    { label: 'Wallet', done: Boolean(state.mainWallet) },
    {
      label: 'Authorize',
      done: Boolean(agentApprovedReady && builderReady),
    },
    { label: 'Ready', done: complete },
  ]
  const currentPrompt = !state.mainWallet
    ? 'Connect your wallet to begin'
    : !agentReady
      ? 'Prepare secure trading access'
      : !agentApprovedReady
        ? 'Approve trade-only access · 1 of 2'
        : !builderReady
          ? 'Finish authorization · 2 of 2'
          : !complete
            ? 'Verify the saved connection'
            : 'Hyperliquid is ready'

  async function connectWallet() {
    setError('')
    const expectedWallet = serverReadyProof?.wallet || state.mainWallet
    if (walletProviders.length > 0 && !selectedWalletProvider) {
      setError('Choose the wallet extension you want to connect.')
      return
    }
    const provider =
      selectedWalletProvider ||
      (await getWalletProviderForAddress(expectedWallet))
    if (!provider) {
      setError(
        language === 'zh'
          ? 'No EVM wallet detected. Install MetaMask, Rabby, OKX or Coinbase Wallet.'
          : 'No EVM wallet detected. Install MetaMask, Rabby, OKX or Coinbase Wallet.'
      )
      return
    }
    setBusy(true)
    try {
      const accounts = await provider.request({ method: 'eth_requestAccounts' })
      const first =
        Array.isArray(accounts) && typeof accounts[0] === 'string'
          ? accounts[0]
          : ''
      if (!first) throw new Error('Wallet returned no account')
      const normalized = normalizeAddress(first)
      if (
        serverReadyProof?.wallet &&
        normalized !== normalizeAddress(serverReadyProof.wallet)
      ) {
        currentMainWalletRef.current = undefined
        setState({})
        throw new Error(
          `Connected wallet ${shortAddress(normalized)} does not match the configured Hyperliquid account ${shortAddress(serverReadyProof.wallet)}. Switch the active account in your wallet extension and retry.`
        )
      }
      walletProviderRef.current = provider
      currentMainWalletRef.current = normalized
      setState((prev) => {
        const sameWallet = prev.mainWallet === normalized
        return {
          ...prev,
          mainWallet: normalized,
          agentAddress: sameWallet ? prev.agentAddress : undefined,
          agentPrivateKey: sameWallet ? prev.agentPrivateKey : undefined,
          agentApproved: sameWallet ? prev.agentApproved : false,
          builderApproved: sameWallet ? prev.builderApproved : false,
          savedExchangeId: sameWallet ? prev.savedExchangeId : undefined,
          reusedSavedExchange: sameWallet ? prev.reusedSavedExchange : false,
        }
      })
      const [, reusedSavedExchange] = await Promise.all([
        refreshBalance(normalized),
        reuseSavedExchangeIfPresent(normalized),
      ])
      // Key generation is a server-side preparation step, not a wallet
      // approval. Do it automatically so the next visible action is the first
      // signature the user actually needs to review.
      if (isLoggedIn && !reusedSavedExchange) {
        const wallet = await api.generateWallet()
        setState((prev) => {
          if (
            normalizeAddress(prev.mainWallet || '') !== normalized ||
            prev.savedExchangeId
          ) {
            return prev
          }
          return {
            ...prev,
            agentAddress: normalizeAddress(wallet.address),
            agentPrivateKey: wallet.private_key,
            agentApproved: false,
            builderApproved: false,
          }
        })
      }
    } catch (err) {
      const message = getWalletErrorMessage(err, 'Wallet connection failed')
      setError(
        /at least one account|no accounts?|account is required/i.test(message)
          ? 'No account is available in this wallet. Create or import an account in the selected wallet, then try again.'
          : message
      )
    } finally {
      setBusy(false)
    }
  }

  async function generateAgentWallet() {
    setError('')
    if (!state.mainWallet) return
    setBusy(true)
    try {
      const wallet = await api.generateWallet()
      setState((prev) => ({
        ...prev,
        agentAddress: normalizeAddress(wallet.address),
        agentPrivateKey: wallet.private_key,
        agentApproved: false,
        builderApproved: false,
        savedExchangeId: undefined,
      }))
      toast.success('NOFX agent wallet generated')
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to generate agent wallet'
      )
    } finally {
      setBusy(false)
    }
  }

  async function signAndSubmit(
    action: Record<string, unknown>,
    primaryType: string,
    fields: { name: string; type: string }[],
    expectedWallet: string
  ) {
    const provider =
      walletProviderRef.current ||
      (await getWalletProviderForAddress(expectedWallet))
    if (!provider || !expectedWallet) throw new Error('Wallet is not connected')
    walletProviderRef.current = provider
    assertCurrentWallet(expectedWallet)
    await ensureWalletSigningAccount(provider, expectedWallet)
    assertCurrentWallet(expectedWallet)
    const { action: signedAction, signature } = await signHyperliquidUserAction(
      provider,
      expectedWallet,
      action,
      primaryType,
      fields
    )
    assertCurrentWallet(expectedWallet)
    await api.submitHyperliquidApproval(
      signedAction,
      Number(signedAction.nonce),
      signature
    )
    assertCurrentWallet(expectedWallet)
  }

  function assertCurrentWallet(expectedWallet: string) {
    if (
      normalizeAddress(currentMainWalletRef.current || '') !==
      normalizeAddress(expectedWallet)
    ) {
      throw new Error(
        'Wallet account changed. Review and restart authorization.'
      )
    }
  }

  async function refreshAfterSave() {
    try {
      await onSaved?.()
    } catch {
      toast.error(
        'Connection saved. Refresh the page to update dashboard data.'
      )
    }
  }

  async function approveAgent() {
    setError('')
    const walletSnapshot = state.mainWallet
    if (!state.agentAddress || !walletSnapshot) return
    setBusy(true)
    try {
      const nonce = Date.now()
      const action = {
        type: 'approveAgent',
        hyperliquidChain: 'Mainnet',
        agentAddress: state.agentAddress,
        agentName: buildAgentName(nonce),
        nonce,
      }
      await signAndSubmit(
        action,
        'HyperliquidTransaction:ApproveAgent',
        [
          { name: 'hyperliquidChain', type: 'string' },
          { name: 'agentAddress', type: 'address' },
          { name: 'agentName', type: 'string' },
          { name: 'nonce', type: 'uint64' },
        ],
        walletSnapshot
      )
      setState((prev) =>
        normalizeAddress(prev.mainWallet || '') ===
        normalizeAddress(walletSnapshot)
          ? { ...prev, agentApproved: true, savedExchangeId: undefined }
          : prev
      )
      toast.success('Hyperliquid agent approved')
      void refreshAgentInfo()
    } catch (err) {
      setError(getWalletErrorMessage(err, 'Agent approval failed'))
    } finally {
      setBusy(false)
    }
  }

  async function renewAgentAuthorization() {
    setError('')
    if (!isLoggedIn) {
      setError(
        'Renewal requires signing in: Hyperliquid forbids reusing the same agent, so renewal creates a new agent and updates the stored key.'
      )
      return
    }
    const walletSnapshot = state.mainWallet
    if (!walletSnapshot) return
    reconciliationGenerationRef.current++
    agentRefreshGenerationRef.current++
    serverProofGenerationRef.current++
    setServerReadyProof(null)
    setAgentInfo(null)
    setState((prev) => ({
      ...prev,
      agentApproved: false,
      builderApproved: false,
    }))
    setBusy(true)
    try {
      const configs = await api.getExchangeConfigs()
      const eligible = configs.filter(
        (exchange) =>
          exchange.exchange_type === 'hyperliquid' &&
          exchange.enabled &&
          normalizeAddress(exchange.hyperliquidWalletAddr || '') ===
            normalizeAddress(walletSnapshot)
      )
      const existing = state.savedExchangeId
        ? eligible.find((exchange) => exchange.id === state.savedExchangeId)
        : eligible.length === 1
          ? eligible[0]
          : undefined
      if (!existing) {
        throw new Error(
          eligible.length > 1
            ? 'Multiple enabled Hyperliquid configs match this wallet. Select the exact saved connection before renewing.'
            : 'No matching enabled NOFX config was found. Save the connection before renewing its agent.'
        )
      }
      const wallet = await api.generateWallet()
      const newAgentAddress = normalizeAddress(wallet.address)
      const nonce = Date.now()
      const action = {
        type: 'approveAgent',
        hyperliquidChain: 'Mainnet',
        agentAddress: newAgentAddress,
        agentName: buildAgentName(nonce),
        nonce,
      }
      await signAndSubmit(
        action,
        'HyperliquidTransaction:ApproveAgent',
        [
          { name: 'hyperliquidChain', type: 'string' },
          { name: 'agentAddress', type: 'address' },
          { name: 'agentName', type: 'string' },
          { name: 'nonce', type: 'uint64' },
        ],
        walletSnapshot
      )
      const recoveryState = {
        agentAddress: newAgentAddress,
        agentPrivateKey: wallet.private_key,
        agentApproved: true,
        builderApproved: false,
        savedExchangeId: existing.id,
        reusedSavedExchange: true,
      }
      setState((prev) =>
        normalizeAddress(prev.mainWallet || '') ===
        normalizeAddress(walletSnapshot)
          ? { ...prev, ...recoveryState }
          : prev
      )
      assertCurrentWallet(walletSnapshot)
      const existingBuilderApproved = Boolean(
        existing.hyperliquidBuilderApproved
      )
      await api.updateExchangeConfigsEncrypted({
        exchanges: {
          [existing.id]: {
            enabled: true,
            api_key: wallet.private_key,
            secret_key: '',
            passphrase: '',
            hyperliquid_wallet_addr: walletSnapshot,
            hyperliquid_unified_account: true,
            hyperliquid_builder_approved: existingBuilderApproved,
            testnet: false,
          },
        },
      })
      setState((prev) =>
        normalizeAddress(prev.mainWallet || '') ===
        normalizeAddress(walletSnapshot)
          ? {
              ...prev,
              agentAddress: newAgentAddress,
              agentPrivateKey: undefined,
              agentApproved: true,
              builderApproved: existingBuilderApproved,
              savedExchangeId: existing.id,
              reusedSavedExchange: true,
            }
          : prev
      )
      toast.success('Agent renewed (new agent, valid 180 days)')
      await refreshAgentInfo(
        walletSnapshot,
        newAgentAddress,
        existingBuilderApproved,
        existing.id
      )
    } catch (err) {
      setError(getWalletErrorMessage(err, 'Agent renewal failed'))
    } finally {
      setBusy(false)
    }
  }

  async function approveBuilderFee() {
    setError('')
    const walletSnapshot = state.mainWallet
    if (!walletSnapshot) return
    setBusy(true)
    let approvalComplete = false
    try {
      const nonce = Date.now()
      const action = {
        type: 'approveBuilderFee',
        hyperliquidChain: 'Mainnet',
        maxFeeRate: HYPERLIQUID_BUILDER_MAX_FEE,
        builder: normalizeAddress(HYPERLIQUID_BUILDER_ADDRESS),
        nonce,
      }
      await signAndSubmit(
        action,
        'HyperliquidTransaction:ApproveBuilderFee',
        [
          { name: 'hyperliquidChain', type: 'string' },
          { name: 'maxFeeRate', type: 'string' },
          { name: 'builder', type: 'address' },
          { name: 'nonce', type: 'uint64' },
        ],
        walletSnapshot
      )
      approvalComplete = true
      assertCurrentWallet(walletSnapshot)
      let createdExchangeId: string | undefined
      if (isLoggedIn && state.savedExchangeId) {
        await api.updateExchangeConfigsEncrypted({
          exchanges: {
            [state.savedExchangeId]: {
              enabled: true,
              api_key: '',
              secret_key: '',
              passphrase: '',
              hyperliquid_wallet_addr: walletSnapshot,
              hyperliquid_unified_account: true,
              hyperliquid_builder_approved: true,
              testnet: false,
            },
          },
        })
      } else if (isLoggedIn && state.agentPrivateKey) {
        const result = await api.createExchangeEncrypted({
          exchange_type: 'hyperliquid',
          account_name: `Hyperliquid ${shortAddress(walletSnapshot)}`,
          enabled: true,
          api_key: state.agentPrivateKey,
          hyperliquid_wallet_addr: walletSnapshot,
          hyperliquid_unified_account: true,
          hyperliquid_builder_approved: true,
          testnet: false,
        })
        createdExchangeId = result.id
      }
      setState((prev) =>
        normalizeAddress(prev.mainWallet || '') ===
        normalizeAddress(walletSnapshot)
          ? {
              ...prev,
              agentPrivateKey: createdExchangeId
                ? undefined
                : prev.agentPrivateKey,
              builderApproved: true,
              savedExchangeId: createdExchangeId || prev.savedExchangeId,
            }
          : prev
      )
      await refreshAgentInfo(
        walletSnapshot,
        state.agentAddress,
        true,
        createdExchangeId || state.savedExchangeId
      )
      await refreshAfterSave()
      toast.success('Trading authorization finalized')
    } catch (err) {
      if (approvalComplete) {
        setState((prev) =>
          normalizeAddress(prev.mainWallet || '') ===
          normalizeAddress(walletSnapshot)
            ? { ...prev, builderApproved: true }
            : prev
        )
      }
      setError(
        approvalComplete
          ? `Wallet authorization succeeded, but NOFX could not save the connection. Use “Save connection” to retry. ${err instanceof Error ? err.message : ''}`
          : getWalletErrorMessage(err, 'Trading authorization failed')
      )
    } finally {
      setBusy(false)
    }
  }

  async function saveExchange() {
    setError('')
    if (!isLoggedIn) {
      setError('Please sign in before saving the agent wallet for trading.')
      return
    }
    const walletSnapshot = state.mainWallet
    if (!walletSnapshot || !state.builderApproved) return
    setBusy(true)
    try {
      const configs = await api.getExchangeConfigs()
      const eligible = configs.filter(
        (exchange) =>
          exchange.exchange_type === 'hyperliquid' &&
          exchange.enabled &&
          normalizeAddress(exchange.hyperliquidWalletAddr || '') ===
            normalizeAddress(walletSnapshot)
      )
      const existing = state.savedExchangeId
        ? eligible.find((exchange) => exchange.id === state.savedExchangeId)
        : eligible.length === 1
          ? eligible[0]
          : undefined
      if (!state.savedExchangeId && eligible.length > 1) {
        throw new Error(
          'Multiple enabled Hyperliquid configs match this wallet. Select the exact connection before saving.'
        )
      }
      assertCurrentWallet(walletSnapshot)
      if (existing) {
        await api.updateExchangeConfigsEncrypted({
          exchanges: {
            [existing.id]: {
              enabled: true,
              api_key: state.agentPrivateKey || '',
              secret_key: '',
              passphrase: '',
              hyperliquid_wallet_addr: walletSnapshot,
              hyperliquid_unified_account: true,
              hyperliquid_builder_approved: true,
              testnet: false,
            },
          },
        })
        setState((prev) =>
          normalizeAddress(prev.mainWallet || '') ===
          normalizeAddress(walletSnapshot)
            ? {
                ...prev,
                agentPrivateKey: undefined,
                savedExchangeId: existing.id,
                reusedSavedExchange: !state.agentPrivateKey,
                builderApproved: true,
              }
            : prev
        )
        toast.success(
          state.agentPrivateKey
            ? 'Hyperliquid account updated in NOFX'
            : 'Existing Hyperliquid account authorization updated'
        )
        await refreshAgentInfo(
          walletSnapshot,
          state.agentAddress,
          state.builderApproved,
          existing.id
        )
        await refreshAfterSave()
        return
      }
      if (!state.agentPrivateKey) {
        throw new Error(
          'Generate and authorize a new agent wallet before saving'
        )
      }
      const result = await api.createExchangeEncrypted({
        exchange_type: 'hyperliquid',
        account_name: `Hyperliquid ${shortAddress(walletSnapshot)}`,
        enabled: true,
        api_key: state.agentPrivateKey,
        hyperliquid_wallet_addr: walletSnapshot,
        hyperliquid_unified_account: true,
        hyperliquid_builder_approved: true,
        testnet: false,
      })
      setState((prev) =>
        normalizeAddress(prev.mainWallet || '') ===
        normalizeAddress(walletSnapshot)
          ? {
              ...prev,
              agentPrivateKey: undefined,
              savedExchangeId: result.id,
              reusedSavedExchange: false,
            }
          : prev
      )
      toast.success('Hyperliquid account saved to NOFX')
      await refreshAgentInfo(
        walletSnapshot,
        state.agentAddress,
        state.builderApproved,
        result.id
      )
      await refreshAfterSave()
    } catch (err) {
      setError(getWalletErrorMessage(err, 'Failed to save Hyperliquid account'))
    } finally {
      setBusy(false)
    }
  }

  function resetTradingAuthorization() {
    setOpen(true)
    setError('')
    setState((prev) => ({
      ...prev,
      agentApproved: prev.agentApproved || Boolean(prev.savedExchangeId),
      builderApproved: false,
      reusedSavedExchange:
        Boolean(prev.savedExchangeId) || prev.reusedSavedExchange,
    }))
  }

  function resetFlow() {
    window.localStorage.removeItem(STORAGE_KEY)
    setState({})
    setAccount(null)
    setBalanceError('')
    setError('')
  }

  return (
    <div className={inline ? 'relative w-full' : 'relative'}>
      {!inline && (
        <button
          type="button"
          onClick={() => setOpen((value) => !value)}
          className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-bold transition-all border ${
            connectedAddr
              ? 'bg-nofx-success/10 border-nofx-success/30 text-nofx-success'
              : 'bg-nofx-gold/10 border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/20'
          }`}
        >
          <Wallet className="w-4 h-4" />
          <span>
            {connectedAddr ? shortAddress(connectedAddr) : text.connect}
          </span>
          <ChevronDown className="w-4 h-4" />
        </button>
      )}

      {(open || inline) && (
        <div
          className={`${inline ? 'relative w-full' : 'absolute right-0 top-full mt-2 w-[min(420px,calc(100vw-2rem))] shadow-2xl shadow-black/10'} max-h-[calc(100vh-5.5rem)] overflow-y-auto rounded-2xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg-lighter z-[80]`}
        >
          <div className="flex items-start justify-between gap-4 border-b border-[rgba(26,24,19,0.14)] p-4 sm:p-5">
            <div className="min-w-0">
              <h2 className="font-bold text-nofx-text">Connect Hyperliquid</h2>
              <p className="mt-1 text-xs leading-5 text-nofx-text-muted">
                {currentPrompt}
              </p>
            </div>
            {!inline && (
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="p-1 rounded hover:bg-[rgba(26,24,19,0.06)] text-nofx-text-muted"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>

          <div className="space-y-4 p-4 sm:p-5">
            <div
              className="grid grid-cols-3 gap-2"
              aria-label="Connection progress"
            >
              {connectionProgress.map((step, index) => (
                <div key={step.label} className="min-w-0 text-center">
                  <div
                    className={`mx-auto flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold ${
                      step.done
                        ? 'bg-nofx-success text-white'
                        : index ===
                            connectionProgress.findIndex((item) => !item.done)
                          ? 'bg-nofx-gold text-white'
                          : 'bg-nofx-bg-deeper text-nofx-text-muted'
                    }`}
                  >
                    {step.done ? <Check className="w-3.5 h-3.5" /> : index + 1}
                  </div>
                  <span className="mt-1 block truncate text-[11px] font-medium text-nofx-text-muted">
                    {step.label}
                  </span>
                </div>
              ))}
            </div>

            <div className="rounded-xl border border-nofx-success/20 bg-nofx-success/5 p-3">
              <div className="flex items-start gap-2">
                <Shield className="mt-0.5 h-4 w-4 shrink-0 text-nofx-success" />
                <div>
                  <p className="text-xs font-semibold text-nofx-text">
                    NOFX receives trade-only access; it can never withdraw your
                    funds.
                  </p>
                  <p className="mt-1 text-[11px] leading-5 text-nofx-text-muted">
                    Two wallet approvals: authorize the NOFX Agent, then approve
                    a maximum 0.05% builder fee. Your main wallet key never
                    leaves your wallet.
                  </p>
                </div>
              </div>
            </div>

            {!state.mainWallet && walletProviders.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3 text-xs">
                  <span className="font-semibold text-nofx-text">
                    Choose wallet
                  </span>
                  {serverReadyProof?.wallet && (
                    <span className="text-nofx-text-muted">
                      Expected {shortAddress(serverReadyProof.wallet)}
                    </span>
                  )}
                </div>
                <div
                  className="grid grid-cols-2 gap-2"
                  role="group"
                  aria-label="Wallet extension"
                >
                  {walletProviders.map((provider, index) => {
                    const selected = selectedWalletProvider === provider
                    return (
                      <button
                        key={`${getWalletProviderName(provider)}-${index}`}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => {
                          walletProviderRef.current = provider
                          setSelectedWalletProvider(provider)
                          setError('')
                        }}
                        className={`flex min-h-11 items-center justify-between gap-2 rounded-lg border px-3 py-2 text-left text-xs font-semibold transition-colors ${
                          selected
                            ? 'border-nofx-gold bg-nofx-gold/10 text-nofx-text'
                            : 'border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper text-nofx-text hover:border-[rgba(26,24,19,0.28)]'
                        }`}
                      >
                        <span className="flex min-w-0 items-center gap-2">
                          <Wallet className="h-4 w-4 shrink-0" />
                          <span className="truncate">
                            {getWalletProviderName(provider)}
                          </span>
                        </span>
                        {selected && <Check className="h-4 w-4 shrink-0" />}
                      </button>
                    )
                  })}
                </div>
              </div>
            )}

            {error && (
              <div className="rounded-lg border border-nofx-danger/30 bg-nofx-danger/10 p-3 text-xs text-nofx-danger">
                {error}
              </div>
            )}

            {!state.mainWallet && !hasWalletProvider && (
              <div className="rounded-xl border border-nofx-gold/20 bg-nofx-gold/5 p-3">
                <div className="text-sm font-semibold text-nofx-text">
                  {text.noWalletTitle}
                </div>
                <p className="mt-1 text-xs leading-5 text-nofx-text-muted">
                  {text.noWalletDetail}
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  <a
                    href="https://rabby.io/"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-2 rounded-lg border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper px-3 py-2 text-xs font-semibold text-nofx-text hover:border-[rgba(26,24,19,0.24)] hover:bg-nofx-bg"
                  >
                    <Download className="h-3.5 w-3.5" />
                    {text.installRabby}
                  </a>
                  <a
                    href="https://metamask.io/download/"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-2 rounded-lg border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper px-3 py-2 text-xs font-semibold text-nofx-text hover:border-[rgba(26,24,19,0.24)] hover:bg-nofx-bg"
                  >
                    <ExternalLink className="h-3.5 w-3.5" />
                    {text.installMetaMask}
                  </a>
                </div>
              </div>
            )}

            {!complete && (
              <div className="grid grid-cols-1 gap-2">
                {!state.mainWallet && (
                  <ActionButton
                    busy={busy}
                    onClick={connectWallet}
                    label="Connect wallet"
                  />
                )}
                {state.mainWallet && !agentReady && (
                  <ActionButton
                    busy={busy}
                    onClick={generateAgentWallet}
                    label="Prepare secure access"
                  />
                )}
                {agentReady && !agentApprovedReady && (
                  <ActionButton
                    busy={busy}
                    onClick={
                      state.reusedSavedExchange || state.savedExchangeId
                        ? renewAgentAuthorization
                        : approveAgent
                    }
                    label={
                      state.reusedSavedExchange || state.savedExchangeId
                        ? 'Re-authorize trade-only access'
                        : 'Approve trade-only access'
                    }
                  />
                )}
                {agentApprovedReady && !builderReady && (
                  <ActionButton
                    busy={busy}
                    onClick={approveBuilderFee}
                    label="Approve fee & finish"
                  />
                )}
                {builderReady && !state.savedExchangeId && (
                  <ActionButton
                    busy={busy}
                    onClick={saveExchange}
                    label="Save connection"
                  />
                )}
              </div>
            )}

            <div
              className={`rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper p-3 space-y-2 text-xs ${state.mainWallet ? '' : 'hidden'}`}
            >
              {state.mainWallet && (
                <div className="flex items-center justify-between gap-3">
                  <span className="text-nofx-text-muted">Main</span>
                  <button
                    type="button"
                    onClick={() => copy(state.mainWallet!, 'Main wallet')}
                    className="font-mono text-nofx-text hover:text-nofx-gold flex items-center gap-1"
                  >
                    {shortAddress(state.mainWallet)}{' '}
                    <Copy className="w-3 h-3" />
                  </button>
                </div>
              )}
              {complete && state.agentAddress && (
                <div className="flex items-center justify-between gap-3">
                  <span className="text-nofx-text-muted">Agent</span>
                  <button
                    type="button"
                    onClick={() => copy(state.agentAddress!, 'Agent wallet')}
                    className="font-mono text-nofx-text hover:text-nofx-gold flex items-center gap-1"
                  >
                    {shortAddress(state.agentAddress)}{' '}
                    <Copy className="w-3 h-3" />
                  </button>
                </div>
              )}
              <div className="flex items-center justify-between gap-3">
                <span className="text-nofx-text-muted">Network</span>
                <span className="font-mono text-nofx-text">
                  Hyperliquid Mainnet
                </span>
              </div>
              {complete && state.mainWallet && (
                <div className="flex items-center justify-between gap-3 border-t border-[rgba(26,24,19,0.14)] pt-2">
                  <span className="text-nofx-text-muted">
                    {text.agentExpiry}
                  </span>
                  {agentInfoLoading && !agentInfo ? (
                    <span className="font-mono text-nofx-text-muted">
                      Loading…
                    </span>
                  ) : agentInfo ? (
                    (() => {
                      const { dateStr, daysLeft } = formatAgentExpiry(
                        agentInfo.validUntil,
                        language
                      )
                      const expired = daysLeft < 0
                      const soon = daysLeft >= 0 && daysLeft <= 14
                      const tone = expired
                        ? 'text-nofx-danger'
                        : soon
                          ? 'text-nofx-gold'
                          : 'text-nofx-text'
                      return (
                        <span className={`font-mono text-right ${tone}`}>
                          {dateStr}
                          <span className="ml-1 opacity-80">
                            ({expired ? text.agentExpired : `${daysLeft}d`})
                          </span>
                        </span>
                      )
                    })()
                  ) : (
                    <span className="font-mono text-nofx-text-muted">
                      {text.agentNoAuth}
                    </span>
                  )}
                </div>
              )}
            </div>

            {complete && agentInfo && (
              <div className="rounded-xl border border-nofx-gold/20 bg-nofx-gold/5 p-3 space-y-2 text-xs">
                <button
                  type="button"
                  disabled={busy}
                  onClick={renewAgentAuthorization}
                  className="w-full flex items-center justify-center gap-2 rounded-xl border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-2.5 text-sm font-bold text-nofx-gold transition hover:bg-nofx-gold/20 disabled:opacity-60 disabled:cursor-not-allowed"
                >
                  {busy ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <RefreshCw className="w-4 h-4" />
                  )}
                  {text.renewAgent}
                </button>
                <p className="text-[11px] leading-relaxed text-nofx-text-muted">
                  {text.renewHint}
                </p>
              </div>
            )}

            {complete && state.mainWallet && (
              <div className="rounded-xl border border-nofx-gold/20 bg-nofx-gold/5 p-3 space-y-3 text-xs">
                <div className="flex items-center justify-between gap-3">
                  <span className="font-bold text-nofx-text">
                    {text.balance}
                  </span>
                  <button
                    type="button"
                    onClick={() => void refreshBalance()}
                    disabled={balanceLoading}
                    className="flex items-center gap-1 text-nofx-text-muted hover:text-nofx-gold disabled:opacity-60"
                  >
                    <RefreshCw
                      className={`w-3 h-3 ${balanceLoading ? 'animate-spin' : ''}`}
                    />
                    {text.refresh}
                  </button>
                </div>
                {balanceError ? (
                  <div className="rounded-lg border border-nofx-danger/30 bg-nofx-danger/10 p-2 text-nofx-danger">
                    {balanceError}
                  </div>
                ) : (
                  <div className="grid grid-cols-2 gap-2">
                    <div className="rounded-lg bg-nofx-bg-deeper p-2">
                      <div className="text-nofx-text-muted">
                        {text.withdrawable}
                      </div>
                      <div className="mt-1 font-mono text-sm font-bold text-nofx-success">
                        {balanceLoading && !account
                          ? 'Loading…'
                          : `${formatUSDC(account?.withdrawable)} USDC`}
                      </div>
                    </div>
                    <div className="rounded-lg bg-nofx-bg-deeper p-2">
                      <div className="text-nofx-text-muted">{text.equity}</div>
                      <div className="mt-1 font-mono text-sm font-bold text-nofx-text">
                        {balanceLoading && !account
                          ? 'Loading…'
                          : `${formatUSDC(account?.accountValue)} USDC`}
                      </div>
                    </div>
                    <div className="rounded-lg bg-nofx-bg-deeper p-2">
                      <div className="text-nofx-text-muted">
                        {text.marginUsed}
                      </div>
                      <div className="mt-1 font-mono text-sm font-bold text-nofx-text">
                        {formatUSDC(account?.totalMarginUsed)} USDC
                      </div>
                    </div>
                    <div className="rounded-lg bg-nofx-bg-deeper p-2">
                      <div className="text-nofx-text-muted">
                        {text.unrealizedPnl}
                      </div>
                      <div
                        className={`mt-1 font-mono text-sm font-bold ${(account?.unrealizedPnl ?? 0) >= 0 ? 'text-nofx-success' : 'text-nofx-danger'}`}
                      >
                        {formatSignedUSDC(account?.unrealizedPnl)} USDC
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            <div className="grid grid-cols-1 gap-2">
              {complete && (
                <>
                  <div className="rounded-lg border border-nofx-success/30 bg-nofx-success/10 p-3 text-sm text-nofx-success flex items-center gap-2">
                    <Shield className="w-4 h-4" /> {text.done}
                  </div>
                  <button
                    type="button"
                    onClick={resetTradingAuthorization}
                    className="w-full flex items-center justify-center gap-2 rounded-xl border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-3 text-sm font-bold text-nofx-gold transition hover:bg-nofx-gold/20"
                  >
                    {language === 'zh'
                      ? 'Re-authorize trading'
                      : 'Re-authorize trading'}
                  </button>
                </>
              )}
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-[rgba(26,24,19,0.14)]">
              <a
                href="https://app.hyperliquid.xyz/"
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-nofx-text-muted hover:text-nofx-gold flex items-center gap-1"
              >
                Open Hyperliquid <ExternalLink className="w-3 h-3" />
              </a>
              <button
                type="button"
                onClick={resetFlow}
                className="text-xs text-nofx-text-muted hover:text-nofx-danger"
              >
                Reset
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function ActionButton({
  busy,
  onClick,
  label,
}: {
  busy: boolean
  onClick: () => void
  label: string
}) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      className="w-full flex items-center justify-center gap-2 rounded-xl bg-nofx-gold px-4 py-3 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-60 disabled:cursor-not-allowed"
    >
      {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
      {label}
    </button>
  )
}
