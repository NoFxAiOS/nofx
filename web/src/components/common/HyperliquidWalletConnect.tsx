import { useEffect, useMemo, useState } from 'react'
import { Check, ChevronDown, Copy, ExternalLink, Loader2, RefreshCw, Shield, Wallet, X } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../lib/api'
import type { HyperliquidAccountSummary } from '../../lib/api/wallet'
import type { Language } from '../../i18n/translations'

declare global {
  interface Window {
    ethereum?: WalletProvider & { providers?: WalletProvider[] }
  }
}

type WalletProvider = {
  request: (args: { method: string; params?: unknown[] }) => Promise<unknown>
  on?: (event: string, handler: (...args: unknown[]) => void) => void
  removeListener?: (event: string, handler: (...args: unknown[]) => void) => void
  isMetaMask?: boolean
  isRabby?: boolean
  isOkxWallet?: boolean
  isCoinbaseWallet?: boolean
  isTrust?: boolean
  isPhantom?: boolean
  isBackpack?: boolean
  isBraveWallet?: boolean
  isExodus?: boolean
  isFrame?: boolean
}

type StepStatus = 'pending' | 'active' | 'done' | 'error'

interface HyperliquidWalletConnectProps {
  language: Language
  isLoggedIn: boolean
  variant?: 'dropdown' | 'inline'
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

const STORAGE_KEY = 'nofx.hyperliquid.connection.v6'
const AGENT_NAME = 'NOFX Agent'
const HYPERLIQUID_BUILDER_ADDRESS = '0x891dc6f05ad47a3c1a05da55e7a7517971faaf0d'
const HYPERLIQUID_BUILDER_MAX_FEE = '0.1%'

function shortAddress(address?: string) {
  if (!address) return ''
  return `${address.slice(0, 6)}…${address.slice(-4)}`
}

function copy(text: string, label: string) {
  navigator.clipboard?.writeText(text).then(
    () => toast.success(`${label} copied`),
    () => toast.error('Copy failed')
  )
}

function normalizeAddress(address: string) {
  return address.trim().toLowerCase()
}


function getWalletProviders(): WalletProvider[] {
  const injected = window.ethereum
  if (!injected) return []
  const providers = Array.isArray(injected.providers) && injected.providers.length > 0
    ? injected.providers
    : [injected]
  const seen = new Set<WalletProvider>()
  return providers.filter((provider) => {
    if (!provider || seen.has(provider)) return false
    seen.add(provider)
    return true
  })
}

function getPreferredWalletProvider(): WalletProvider | undefined {
  const providers = getWalletProviders()
  return providers.find((provider) => provider.isRabby)
    || providers.find((provider) => provider.isMetaMask)
    || providers.find((provider) => provider.isCoinbaseWallet)
    || providers.find((provider) => provider.isPhantom)
    || providers.find((provider) => provider.isBraveWallet)
    || providers.find((provider) => provider.isBackpack)
    || providers.find((provider) => provider.isOkxWallet)
    || providers.find((provider) => provider.isTrust)
    || providers.find((provider) => provider.isExodus)
    || providers.find((provider) => provider.isFrame)
    || providers[0]
}

function walletSupportLabel(language: Language) {
  return language === 'zh'
    ? '支持 MetaMask、Rabby、Coinbase、Phantom、Brave、Backpack、OKX、Trust 等 EVM 钱包。'
    : 'Supports MetaMask, Rabby, Coinbase Wallet, Phantom, Brave, Backpack, OKX, Trust and other EVM wallets.'
}

function formatUSDC(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '--'
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

function formatSignedUSDC(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '--'
  const sign = value > 0 ? '+' : ''
  return `${sign}${formatUSDC(value)}`
}

function splitSignature(signature: string) {
  const hex = signature.startsWith('0x') ? signature.slice(2) : signature
  if (hex.length !== 130) {
    throw new Error('Invalid wallet signature length')
  }
  const v = parseInt(hex.slice(128, 130), 16)
  return {
    r: `0x${hex.slice(0, 64)}`,
    s: `0x${hex.slice(64, 128)}`,
    v: v < 27 ? v + 27 : v,
  }
}

function buildTypedData(primaryType: string, fields: { name: string; type: string }[], message: Record<string, unknown>) {
  return {
    domain: {
      name: 'HyperliquidSignTransaction',
      version: '1',
      chainId: 421614,
      verifyingContract: '0x0000000000000000000000000000000000000000',
    },
    types: {
      EIP712Domain: [
        { name: 'name', type: 'string' },
        { name: 'version', type: 'string' },
        { name: 'chainId', type: 'uint256' },
        { name: 'verifyingContract', type: 'address' },
      ],
      [primaryType]: fields,
    },
    primaryType,
    message,
  }
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

export function HyperliquidWalletConnect({ language, isLoggedIn, variant = 'dropdown' }: HyperliquidWalletConnectProps) {
  const inline = variant === 'inline'
  const [open, setOpen] = useState(inline)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [state, setState] = useState<FlowState>(() => getSavedState())
  const [account, setAccount] = useState<HyperliquidAccountSummary | null>(null)
  const [balanceLoading, setBalanceLoading] = useState(false)
  const [balanceError, setBalanceError] = useState('')
  const text = useMemo(
    () => ({
      title: language === 'zh' ? 'Hyperliquid 钱包' : 'Hyperliquid Wallet',
      connect: language === 'zh' ? '连接 Hyperliquid' : 'Connect Hyperliquid',
      connected: language === 'zh' ? '已连接' : 'Connected',
      mainWallet: language === 'zh' ? 'EVM 主钱包' : 'EVM main wallet',
      generateAgent: language === 'zh' ? '生成 NOFX Agent 钱包' : 'Generate NOFX agent wallet',
      approveAgent: language === 'zh' ? '授权 Agent 交易' : 'Authorize agent trading',
      approveBuilder: language === 'zh' ? '完成交易授权' : 'Finalize trading authorization',
      save: language === 'zh' ? '保存到 NOFX' : 'Save to NOFX',
      done: language === 'zh' ? '流程已完成' : 'Flow complete',
      balance: language === 'zh' ? 'Hyperliquid 余额' : 'Hyperliquid balance',
      withdrawable: language === 'zh' ? '可用' : 'Withdrawable',
      equity: language === 'zh' ? '权益' : 'Equity',
      marginUsed: language === 'zh' ? '已用保证金' : 'Margin used',
      unrealizedPnl: language === 'zh' ? '未实现盈亏' : 'Unrealized PnL',
      refresh: language === 'zh' ? '刷新' : 'Refresh',
      noCustody: language === 'zh' ? '资金保留在你的 Hyperliquid 账户；NOFX 只保存已授权 Agent 钱包。' : 'Funds stay in your Hyperliquid account; NOFX only stores the authorized agent wallet.',
    }),
    [language]
  )

  useEffect(() => {
    saveState(state)
  }, [state])


  useEffect(() => {
    if (!isLoggedIn || !state.mainWallet) return
    let cancelled = false
    api.getExchangeConfigs()
      .then((configs) => {
        if (cancelled) return
        const existing = configs.find((exchange) =>
          exchange.exchange_type === 'hyperliquid' &&
          normalizeAddress(exchange.hyperliquidWalletAddr || '') === normalizeAddress(state.mainWallet!)
        )
        if (!existing) return
        setState((prev) => {
          if (normalizeAddress(prev.mainWallet || '') !== normalizeAddress(state.mainWallet!)) return prev
          const serverBuilderApproved = Boolean(existing.hyperliquidBuilderApproved)
          if (
            prev.savedExchangeId === existing.id &&
            prev.agentApproved === true &&
            prev.builderApproved === serverBuilderApproved &&
            prev.reusedSavedExchange === true
          ) {
            return prev
          }
          return {
            ...prev,
            agentPrivateKey: undefined,
            agentApproved: true,
            builderApproved: serverBuilderApproved,
            savedExchangeId: existing.id,
            reusedSavedExchange: true,
          }
        })
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [isLoggedIn, state.mainWallet])

  useEffect(() => {
    const handler = (accounts: unknown) => {
      const next = Array.isArray(accounts) && typeof accounts[0] === 'string' ? normalizeAddress(accounts[0]) : undefined
      if (next) {
        setState((prev) => ({ ...prev, mainWallet: next }))
      }
    }
    const provider = getPreferredWalletProvider()
    provider?.on?.('accountsChanged', handler)
    return () => provider?.removeListener?.('accountsChanged', handler)
  }, [])

  useEffect(() => {
    if (open && state.mainWallet) {
      void refreshBalance(state.mainWallet)
    }
  }, [open, state.mainWallet])

  async function refreshBalance(address = state.mainWallet) {
    if (!address) return
    setBalanceLoading(true)
    setBalanceError('')
    try {
      const summary = await api.getHyperliquidAccount(address)
      setAccount(summary)
    } catch (err) {
      setAccount(null)
      setBalanceError(err instanceof Error ? err.message : 'Failed to load Hyperliquid balance')
    } finally {
      setBalanceLoading(false)
    }
  }

  async function reuseSavedExchangeIfPresent(address: string) {
    if (!isLoggedIn) return false
    try {
      const configs = await api.getExchangeConfigs()
      const existing = configs.find((exchange) =>
        exchange.exchange_type === 'hyperliquid' &&
        normalizeAddress(exchange.hyperliquidWalletAddr || '') === normalizeAddress(address)
      )
      if (!existing) return false
      setState((prev) => ({
        ...prev,
        mainWallet: normalizeAddress(address),
        agentAddress: prev.mainWallet === normalizeAddress(address) ? prev.agentAddress : undefined,
        agentPrivateKey: undefined,
        agentApproved: true,
        // Existing configs default to false in the backend unless the exact
        // approveBuilderFee flow has already persisted a successful approval.
        builderApproved: Boolean(existing.hyperliquidBuilderApproved),
        savedExchangeId: existing.id,
        reusedSavedExchange: true,
      }))
      return true
    } catch {
      return false
    }
  }

  const savedReady = Boolean(state.savedExchangeId)
  const agentReady = Boolean(state.agentAddress || savedReady)
  const agentApprovedReady = Boolean(state.agentApproved || savedReady)
  const builderReady = Boolean(state.builderApproved)
  const steps: { key: keyof FlowState; label: string; status: StepStatus }[] = [
    { key: 'mainWallet', label: text.mainWallet, status: state.mainWallet ? 'done' : 'active' },
    { key: 'agentAddress', label: text.generateAgent, status: agentReady ? 'done' : state.mainWallet ? 'active' : 'pending' },
    { key: 'agentApproved', label: text.approveAgent, status: agentApprovedReady ? 'done' : agentReady ? 'active' : 'pending' },
    { key: 'builderApproved', label: text.approveBuilder, status: builderReady ? 'done' : agentApprovedReady ? 'active' : 'pending' },
    { key: 'savedExchangeId', label: text.save, status: state.savedExchangeId ? 'done' : builderReady ? 'active' : 'pending' },
  ]

  const complete = Boolean(state.mainWallet && state.savedExchangeId && state.builderApproved)

  async function connectWallet() {
    setError('')
    const provider = getPreferredWalletProvider()
    if (!provider) {
      setError(language === 'zh' ? '未检测到 EVM 钱包，请安装 MetaMask / Rabby / OKX / Coinbase Wallet。' : 'No EVM wallet detected. Install MetaMask, Rabby, OKX or Coinbase Wallet.')
      return
    }
    setBusy(true)
    try {
      const accounts = await provider.request({ method: 'eth_requestAccounts' })
      const first = Array.isArray(accounts) && typeof accounts[0] === 'string' ? accounts[0] : ''
      if (!first) throw new Error('Wallet returned no account')
      const normalized = normalizeAddress(first)
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
      await Promise.all([
        refreshBalance(normalized),
        reuseSavedExchangeIfPresent(normalized),
      ])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Wallet connection failed')
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
      setError(err instanceof Error ? err.message : 'Failed to generate agent wallet')
    } finally {
      setBusy(false)
    }
  }

  async function signAndSubmit(action: Record<string, unknown>, primaryType: string, fields: { name: string; type: string }[]) {
    const provider = getPreferredWalletProvider()
    if (!provider || !state.mainWallet) throw new Error('Wallet is not connected')
    const typedData = buildTypedData(primaryType, fields, action)
    const raw = await provider.request({
      method: 'eth_signTypedData_v4',
      params: [state.mainWallet, JSON.stringify(typedData)],
    })
    if (typeof raw !== 'string') throw new Error('Wallet returned an invalid signature')
    const signature = splitSignature(raw)
    await api.submitHyperliquidApproval(action, Number(action.nonce), signature)
  }

  async function approveAgent() {
    setError('')
    if (!state.agentAddress) return
    setBusy(true)
    try {
      const nonce = Date.now()
      const action = {
        type: 'approveAgent',
        signatureChainId: '0x66eee',
        hyperliquidChain: 'Mainnet',
        agentAddress: state.agentAddress,
        agentName: AGENT_NAME,
        nonce,
      }
      await signAndSubmit(action, 'HyperliquidTransaction:ApproveAgent', [
        { name: 'hyperliquidChain', type: 'string' },
        { name: 'agentAddress', type: 'address' },
        { name: 'agentName', type: 'string' },
        { name: 'nonce', type: 'uint64' },
      ])
      setState((prev) => ({ ...prev, agentApproved: true, savedExchangeId: undefined }))
      toast.success('Hyperliquid agent approved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Agent approval failed')
    } finally {
      setBusy(false)
    }
  }

  async function approveBuilderFee() {
    setError('')
    setBusy(true)
    try {
      const nonce = Date.now()
      const action = {
        type: 'approveBuilderFee',
        signatureChainId: '0x66eee',
        hyperliquidChain: 'Mainnet',
        maxFeeRate: HYPERLIQUID_BUILDER_MAX_FEE,
        builder: normalizeAddress(HYPERLIQUID_BUILDER_ADDRESS),
        nonce,
      }
      await signAndSubmit(action, 'HyperliquidTransaction:ApproveBuilderFee', [
        { name: 'hyperliquidChain', type: 'string' },
        { name: 'maxFeeRate', type: 'string' },
        { name: 'builder', type: 'address' },
        { name: 'nonce', type: 'uint64' },
      ])
      if (isLoggedIn && state.savedExchangeId && state.mainWallet) {
        await api.updateExchangeConfigsEncrypted({
          exchanges: {
            [state.savedExchangeId]: {
              enabled: true,
              api_key: '',
              secret_key: '',
              passphrase: '',
              hyperliquid_wallet_addr: state.mainWallet,
              hyperliquid_builder_approved: true,
              testnet: false,
            },
          },
        })
      }
      setState((prev) => ({
        ...prev,
        builderApproved: true,
        savedExchangeId: prev.reusedSavedExchange ? prev.savedExchangeId : undefined,
      }))
      toast.success(language === 'zh' ? '交易授权已完成' : 'Trading authorization finalized')
    } catch (err) {
      setError(err instanceof Error ? err.message : (language === 'zh' ? '交易授权失败' : 'Trading authorization failed'))
    } finally {
      setBusy(false)
    }
  }

  async function saveExchange() {
    setError('')
    if (!isLoggedIn) {
      setError(language === 'zh' ? '请先登录 NOFX，再保存 Agent 钱包用于交易。' : 'Please sign in before saving the agent wallet for trading.')
      return
    }
    if (!state.mainWallet || !state.builderApproved) return
    setBusy(true)
    try {
      const existing = (await api.getExchangeConfigs()).find((exchange) =>
        exchange.exchange_type === 'hyperliquid' &&
        normalizeAddress(exchange.hyperliquidWalletAddr || '') === normalizeAddress(state.mainWallet!)
      )
      if (existing) {
        await api.updateExchangeConfigsEncrypted({
          exchanges: {
            [existing.id]: {
              enabled: true,
              api_key: state.agentPrivateKey || '',
              secret_key: '',
              passphrase: '',
              hyperliquid_wallet_addr: state.mainWallet,
              hyperliquid_builder_approved: true,
              testnet: false,
            },
          },
        })
        setState((prev) => ({ ...prev, agentPrivateKey: undefined, savedExchangeId: existing.id, reusedSavedExchange: !state.agentPrivateKey, builderApproved: true }))
        toast.success(state.agentPrivateKey ? 'Hyperliquid account updated in NOFX' : 'Existing Hyperliquid account authorization updated')
        return
      }
      if (!state.agentPrivateKey) {
        throw new Error('Generate and authorize a new agent wallet before saving')
      }
      const result = await api.createExchangeEncrypted({
        exchange_type: 'hyperliquid',
        account_name: `Hyperliquid ${shortAddress(state.mainWallet)}`,
        enabled: true,
        api_key: state.agentPrivateKey,
        hyperliquid_wallet_addr: state.mainWallet,
        hyperliquid_builder_approved: true,
        testnet: false,
      })
      setState((prev) => ({ ...prev, agentPrivateKey: undefined, savedExchangeId: result.id, reusedSavedExchange: false }))
      toast.success('Hyperliquid account saved to NOFX')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save Hyperliquid account')
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
      reusedSavedExchange: Boolean(prev.savedExchangeId) || prev.reusedSavedExchange,
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
            complete
              ? 'bg-emerald-500/10 border-emerald-400/30 text-emerald-300'
              : 'bg-nofx-gold/10 border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/20'
          }`}
        >
          <Wallet className="w-4 h-4" />
          <span>{complete ? shortAddress(state.mainWallet) : text.connect}</span>
          <ChevronDown className="w-4 h-4" />
        </button>
      )}

      {(open || inline) && (
        <div className={`${inline ? 'relative w-full' : 'absolute right-0 top-full mt-2 w-[420px] shadow-2xl shadow-black/50'} rounded-2xl border border-nofx-gold/20 bg-[#11151B] z-[80] overflow-hidden`}>
          <div className="flex items-center justify-between p-4 border-b border-white/10">
            <div>
              <div className="font-bold text-white">{text.title}</div>
              <div className="text-xs text-nofx-text-muted mt-1">{text.noCustody}</div>
              <div className="text-[11px] text-nofx-gold/80 mt-1">{walletSupportLabel(language)}</div>
            </div>
            {!inline && (
              <button type="button" onClick={() => setOpen(false)} className="p-1 rounded hover:bg-white/10 text-zinc-500">
                <X className="w-4 h-4" />
              </button>
            )}
          </div>

          <div className="p-4 space-y-4">
            <div className="space-y-2">
              {steps.map((step, index) => (
                <div key={step.key} className="flex items-center gap-3 text-sm">
                  <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
                    step.status === 'done'
                      ? 'bg-emerald-400 text-black'
                      : step.status === 'active'
                        ? 'bg-nofx-gold text-black'
                        : 'bg-zinc-800 text-zinc-500'
                  }`}
                  >
                    {step.status === 'done' ? <Check className="w-3.5 h-3.5" /> : index + 1}
                  </div>
                  <span className={step.status === 'pending' ? 'text-zinc-500' : 'text-zinc-200'}>{step.label}</span>
                </div>
              ))}
            </div>

            {error && (
              <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-xs text-red-300">
                {error}
              </div>
            )}

            <div className="rounded-xl border border-white/10 bg-black/25 p-3 space-y-2 text-xs">
              {state.mainWallet && (
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">Main</span>
                  <button type="button" onClick={() => copy(state.mainWallet!, 'Main wallet')} className="font-mono text-zinc-200 hover:text-nofx-gold flex items-center gap-1">
                    {shortAddress(state.mainWallet)} <Copy className="w-3 h-3" />
                  </button>
                </div>
              )}
              {state.agentAddress && (
                <div className="flex items-center justify-between gap-3">
                  <span className="text-zinc-500">Agent</span>
                  <button type="button" onClick={() => copy(state.agentAddress!, 'Agent wallet')} className="font-mono text-zinc-200 hover:text-nofx-gold flex items-center gap-1">
                    {shortAddress(state.agentAddress)} <Copy className="w-3 h-3" />
                  </button>
                </div>
              )}
              <div className="flex items-center justify-between gap-3">
                <span className="text-zinc-500">Network</span>
                <span className="font-mono text-zinc-300">Hyperliquid Mainnet</span>
              </div>
            </div>

            {state.mainWallet && (
              <div className="rounded-xl border border-nofx-gold/20 bg-nofx-gold/5 p-3 space-y-3 text-xs">
                <div className="flex items-center justify-between gap-3">
                  <span className="font-bold text-zinc-100">{text.balance}</span>
                  <button
                    type="button"
                    onClick={() => void refreshBalance()}
                    disabled={balanceLoading}
                    className="flex items-center gap-1 text-zinc-400 hover:text-nofx-gold disabled:opacity-60"
                  >
                    <RefreshCw className={`w-3 h-3 ${balanceLoading ? 'animate-spin' : ''}`} />
                    {text.refresh}
                  </button>
                </div>
                {balanceError ? (
                  <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-2 text-red-300">{balanceError}</div>
                ) : (
                  <div className="grid grid-cols-2 gap-2">
                    <div className="rounded-lg bg-black/25 p-2">
                      <div className="text-zinc-500">{text.withdrawable}</div>
                      <div className="mt-1 font-mono text-sm font-bold text-emerald-300">{balanceLoading && !account ? 'Loading…' : `${formatUSDC(account?.withdrawable)} USDC`}</div>
                    </div>
                    <div className="rounded-lg bg-black/25 p-2">
                      <div className="text-zinc-500">{text.equity}</div>
                      <div className="mt-1 font-mono text-sm font-bold text-zinc-100">{balanceLoading && !account ? 'Loading…' : `${formatUSDC(account?.accountValue)} USDC`}</div>
                    </div>
                    <div className="rounded-lg bg-black/25 p-2">
                      <div className="text-zinc-500">{text.marginUsed}</div>
                      <div className="mt-1 font-mono text-sm font-bold text-zinc-100">{formatUSDC(account?.totalMarginUsed)} USDC</div>
                    </div>
                    <div className="rounded-lg bg-black/25 p-2">
                      <div className="text-zinc-500">{text.unrealizedPnl}</div>
                      <div className={`mt-1 font-mono text-sm font-bold ${(account?.unrealizedPnl ?? 0) >= 0 ? 'text-emerald-300' : 'text-red-300'}`}>{formatSignedUSDC(account?.unrealizedPnl)} USDC</div>
                    </div>
                  </div>
                )}
              </div>
            )}

            <div className="grid grid-cols-1 gap-2">
              {!state.mainWallet && <ActionButton busy={busy} onClick={connectWallet} label={text.connect} />}
              {state.mainWallet && !agentReady && <ActionButton busy={busy} onClick={generateAgentWallet} label={text.generateAgent} />}
              {agentReady && !agentApprovedReady && <ActionButton busy={busy} onClick={approveAgent} label={text.approveAgent} />}
              {agentApprovedReady && !builderReady && <ActionButton busy={busy} onClick={approveBuilderFee} label={text.approveBuilder} />}
              {builderReady && !state.savedExchangeId && <ActionButton busy={busy} onClick={saveExchange} label={text.save} />}
              {complete && (
                <>
                  <div className="rounded-lg border border-emerald-400/30 bg-emerald-500/10 p-3 text-sm text-emerald-200 flex items-center gap-2">
                    <Shield className="w-4 h-4" /> {text.done}
                  </div>
                  <button
                    type="button"
                    onClick={resetTradingAuthorization}
                    className="w-full flex items-center justify-center gap-2 rounded-xl border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-3 text-sm font-bold text-nofx-gold transition hover:bg-nofx-gold/20"
                  >
                    {language === 'zh' ? '重新授权交易' : 'Re-authorize trading'}
                  </button>
                </>
              )}
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-white/10">
              <a href="https://app.hyperliquid.xyz/" target="_blank" rel="noopener noreferrer" className="text-xs text-zinc-500 hover:text-nofx-gold flex items-center gap-1">
                Open Hyperliquid <ExternalLink className="w-3 h-3" />
              </a>
              <button type="button" onClick={resetFlow} className="text-xs text-zinc-500 hover:text-red-300">
                Reset
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function ActionButton({ busy, onClick, label }: { busy: boolean; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      className="w-full flex items-center justify-center gap-2 rounded-xl bg-nofx-gold px-4 py-3 text-sm font-bold text-black transition hover:bg-yellow-400 disabled:opacity-60 disabled:cursor-not-allowed"
    >
      {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
      {label}
    </button>
  )
}
