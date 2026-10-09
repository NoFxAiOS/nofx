import { useCallback, useEffect, useState } from 'react'
import { ArrowDownUp, Loader2, QrCode, RefreshCw } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import { api } from '../../lib/api'
import type { HyperliquidAccountSummary } from '../../lib/api/wallet'
import type { Language } from '../../i18n/translations'
import { copyWithToast } from '../../lib/clipboard'
import {
  formatUSDC,
  getPreferredWalletProvider,
  normalizeAddress,
  shortAddress,
  signHyperliquidUserAction,
} from '../../lib/hyperliquidWallet'

// Hyperliquid only credits one canonical deposit route: NATIVE USDC on
// Arbitrum One sent to the validator-controlled Bridge2 contract. The sender
// address is the account that gets credited (min 5 USDC, ~1 minute).
const ARBITRUM_CHAIN_ID = '0xa4b1' // 42161
const ARBITRUM_NATIVE_USDC = '0xaf88d065e77c8cC2239327C5EDb3A432268e5831'
const HYPERLIQUID_BRIDGE2 = '0x2Df1c51E09aECF9cacB7bc98cB1742757f163dF7'
const MIN_BRIDGE_DEPOSIT_USDC = 5
const ARBITRUM_RPC = 'https://arb1.arbitrum.io/rpc'

function erc20TransferData(to: string, amountUnits: bigint) {
  const addr = to.toLowerCase().replace(/^0x/, '').padStart(64, '0')
  const amount = amountUnits.toString(16).padStart(64, '0')
  return `0xa9059cbb${addr}${amount}`
}

async function arbitrumRpc(method: string, params: unknown[]): Promise<string> {
  const res = await fetch(ARBITRUM_RPC, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  })
  const data = (await res.json()) as { result?: unknown }
  return typeof data.result === 'string' ? data.result : '0x0'
}

/** On-chain Arbitrum balances of the main wallet — the funds a bridge deposit can draw from. */
async function fetchArbitrumBalances(address: string) {
  const padded = address.replace(/^0x/, '').padStart(64, '0')
  const [usdcHex, ethHex] = await Promise.all([
    arbitrumRpc('eth_call', [
      { to: ARBITRUM_NATIVE_USDC, data: `0x70a08231${padded}` },
      'latest',
    ]),
    arbitrumRpc('eth_getBalance', [address, 'latest']),
  ])
  return {
    usdc: Number(BigInt(usdcHex)) / 1e6,
    eth: Number(BigInt(ethHex)) / 1e18,
  }
}

interface HyperliquidFundsPanelProps {
  language: Language
  walletAddress: string
  /**
   * Hyperliquid "unified account" mode (the default, and HL's recommendation):
   * one USDC balance collateralizes spot, validator perps and HIP-3 perps, so
   * there is no spot/perp split and no class transfer — the transfer tab is
   * hidden and a single account card is shown. Manual/standard-mode accounts
   * keep the split view with the spot<->perp transfer.
   */
  unifiedAccount?: boolean
  onTransferred?: () => void | Promise<void>
}

const TEXT = {
  zh: {
    deposit: '充值',
    transfer: '划转',
    balances: '余额',
    spot: '现货(主钱包)',
    perp: '合约(交易账户)',
    available: '可用',
    withdrawable: '可提取',
    depositHint:
      '二维码是你的主钱包地址。第一步:通过 Arbitrum One 链把 USDC 转到这个地址(进钱包)。第二步:用下方按钮存入 Hyperliquid,约 1 分钟到账,到账后即可直接用于交易。',
    hlAccount: 'Hyperliquid 账户',
    total: '总额',
    tradable: '可交易',
    marginInUse: '保证金占用',
    depositWarn:
      '入金只认 Arbitrum One 上的原生 USDC。USDT 或其他链的资产需先兑换成 Arbitrum USDC。',
    bridgeTitle: '存入 Hyperliquid(钱包 → 交易账户)',
    bridgeAmount: '入金数量 (USDC)',
    bridgeSubmit: '存入交易账户',
    bridgeSubmitting: '存入中…',
    bridgeMin: `最低入金 ${MIN_BRIDGE_DEPOSIT_USDC} USDC,低于此额度会丢失`,
    bridgeGasHint: '需要钱包里有少量 Arbitrum ETH 作为 gas。',
    bridgeSubmitted: '入金交易已提交,约 1 分钟后到账合约账户',
    wallet: '钱包 (Arbitrum)',
    depositable: '可入金',
    gas: 'Gas',
    noGas: 'Arbitrum ETH 为 0,无法支付 gas',
    copyAddress: '复制地址',
    addressCopied: '地址已复制',
    spotToPerp: '现货 → 合约',
    perpToSpot: '合约 → 现货',
    amount: '划转数量 (USDC)',
    max: '全部',
    submit: '签名并划转',
    submitting: '划转中…',
    connectFirst: '连接主钱包以签名划转',
    connect: '连接钱包',
    noProvider: '未检测到浏览器钱包插件(如 MetaMask / OKX)',
    wrongWallet: (want: string, got: string) =>
      `钱包地址不匹配:需要 ${want},当前连接 ${got}。划转签名必须来自主钱包本身。`,
    invalidAmount: '请输入有效的划转数量',
    exceedsBalance: '超出可用余额',
    success: '划转成功,余额稍后刷新',
    failed: '划转失败',
  },
  en: {
    deposit: 'Deposit',
    transfer: 'Transfer',
    balances: 'Balances',
    spot: 'Spot (main wallet)',
    perp: 'Perp (trading account)',
    available: 'available',
    withdrawable: 'withdrawable',
    depositHint:
      'The QR is your main wallet address. Step 1: send USDC to it on Arbitrum One (funds the wallet). Step 2: deposit into Hyperliquid with the button below — credited in about a minute and immediately tradable.',
    hlAccount: 'Hyperliquid account',
    total: 'Total',
    tradable: 'Tradable',
    marginInUse: 'Margin in use',
    depositWarn:
      'Deposits only accept native USDC on Arbitrum One. Swap USDT or assets on other chains into Arbitrum USDC first.',
    bridgeTitle: 'Deposit to Hyperliquid (wallet → trading account)',
    bridgeAmount: 'Deposit amount (USDC)',
    bridgeSubmit: 'Deposit to trading account',
    bridgeSubmitting: 'Depositing…',
    bridgeMin: `Minimum deposit ${MIN_BRIDGE_DEPOSIT_USDC} USDC — smaller amounts are lost`,
    bridgeGasHint: 'Requires a little Arbitrum ETH in the wallet for gas.',
    bridgeSubmitted: 'Deposit submitted, credited to the perp account in ~1 minute',
    wallet: 'Wallet (Arbitrum)',
    depositable: 'depositable',
    gas: 'Gas',
    noGas: 'No Arbitrum ETH for gas',
    copyAddress: 'Copy address',
    addressCopied: 'Address copied',
    spotToPerp: 'Spot → Perp',
    perpToSpot: 'Perp → Spot',
    amount: 'Amount (USDC)',
    max: 'Max',
    submit: 'Sign & transfer',
    submitting: 'Transferring…',
    connectFirst: 'Connect the main wallet to sign the transfer',
    connect: 'Connect wallet',
    noProvider: 'No browser wallet extension detected (MetaMask / OKX etc.)',
    wrongWallet: (want: string, got: string) =>
      `Wallet mismatch: expected ${want} but connected ${got}. The transfer must be signed by the main wallet itself.`,
    invalidAmount: 'Enter a valid transfer amount',
    exceedsBalance: 'Amount exceeds the available balance',
    success: 'Transfer submitted, balances refresh shortly',
    failed: 'Transfer failed',
  },
}

export function HyperliquidFundsPanel({
  language,
  walletAddress,
  unifiedAccount = true,
  onTransferred,
}: HyperliquidFundsPanelProps) {
  const t = TEXT[language === 'zh' ? 'zh' : 'en']
  const address = normalizeAddress(walletAddress)
  const [tab, setTab] = useState<'deposit' | 'transfer'>('deposit')
  const [account, setAccount] = useState<HyperliquidAccountSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [toPerp, setToPerp] = useState(true)
  const [amount, setAmount] = useState('')
  const [depositAmount, setDepositAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [walletUsdc, setWalletUsdc] = useState<number | undefined>()
  const [walletEth, setWalletEth] = useState<number | undefined>()

  const refresh = useCallback(async () => {
    if (!address) return
    setLoading(true)
    try {
      const [summary, chain] = await Promise.allSettled([
        api.getHyperliquidAccount(address),
        fetchArbitrumBalances(address),
      ])
      if (summary.status === 'fulfilled') setAccount(summary.value)
      if (chain.status === 'fulfilled') {
        setWalletUsdc(chain.value.usdc)
        setWalletEth(chain.value.eth)
      }
    } finally {
      setLoading(false)
    }
  }, [address])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const availableFrom = toPerp
    ? (account?.spotUsdcAvailable ?? 0)
    : (account?.withdrawable ?? 0)

  async function submitTransfer() {
    setError('')
    const parsed = Number(amount)
    if (!Number.isFinite(parsed) || parsed <= 0) {
      setError(t.invalidAmount)
      return
    }
    if (parsed > availableFrom + 1e-9) {
      setError(t.exceedsBalance)
      return
    }
    const provider = getPreferredWalletProvider()
    if (!provider) {
      setError(t.noProvider)
      return
    }
    setBusy(true)
    try {
      const accounts = (await provider.request({
        method: 'eth_requestAccounts',
      })) as string[]
      const signer = normalizeAddress(accounts?.[0] ?? '')
      if (!signer) throw new Error(t.connectFirst)
      if (signer !== address) {
        throw new Error(t.wrongWallet(shortAddress(address), shortAddress(signer)))
      }
      const nonce = Date.now()
      const action = {
        type: 'usdClassTransfer',
        hyperliquidChain: 'Mainnet',
        amount: String(parsed),
        toPerp,
        nonce,
      }
      const { action: signedAction, signature } =
        await signHyperliquidUserAction(
          provider,
          signer,
          action,
          'HyperliquidTransaction:UsdClassTransfer',
          [
            { name: 'hyperliquidChain', type: 'string' },
            { name: 'amount', type: 'string' },
            { name: 'toPerp', type: 'bool' },
            { name: 'nonce', type: 'uint64' },
          ]
        )
      await api.submitHyperliquidApproval(signedAction, nonce, signature)
      toast.success(t.success)
      setAmount('')
      await onTransferred?.()
      // Hyperliquid settles the class transfer near-instantly; one refresh
      // shortly after covers indexing lag.
      setTimeout(() => void refresh(), 1500)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.failed)
    } finally {
      setBusy(false)
    }
  }

  async function submitBridgeDeposit() {
    setError('')
    const parsed = Number(depositAmount)
    if (!Number.isFinite(parsed) || parsed <= 0) {
      setError(t.invalidAmount)
      return
    }
    if (parsed < MIN_BRIDGE_DEPOSIT_USDC) {
      setError(t.bridgeMin)
      return
    }
    if (walletUsdc !== undefined && parsed > walletUsdc + 1e-9) {
      setError(t.exceedsBalance)
      return
    }
    const provider = getPreferredWalletProvider()
    if (!provider) {
      setError(t.noProvider)
      return
    }
    setBusy(true)
    try {
      const accounts = (await provider.request({
        method: 'eth_requestAccounts',
      })) as string[]
      const signer = normalizeAddress(accounts?.[0] ?? '')
      if (!signer) throw new Error(t.connectFirst)
      // The bridge credits the SENDER: sending from any other wallet would
      // fund that wallet's Hyperliquid account instead of this one.
      if (signer !== address) {
        throw new Error(t.wrongWallet(shortAddress(address), shortAddress(signer)))
      }
      await provider.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: ARBITRUM_CHAIN_ID }],
      })
      const units = BigInt(Math.round(parsed * 1e6))
      await provider.request({
        method: 'eth_sendTransaction',
        params: [
          {
            from: signer,
            to: ARBITRUM_NATIVE_USDC,
            data: erc20TransferData(HYPERLIQUID_BRIDGE2, units),
          },
        ],
      })
      toast.success(t.bridgeSubmitted)
      setDepositAmount('')
      setTimeout(() => void refresh(), 60_000)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.failed)
    } finally {
      setBusy(false)
    }
  }

  if (!address) return null

  return (
    <div className="rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper p-4 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setTab('deposit')}
            className={`px-3 py-1.5 rounded-xl text-sm font-semibold flex items-center gap-1.5 ${
              tab === 'deposit'
                ? 'bg-nofx-gold text-white'
                : 'bg-nofx-bg text-nofx-text-muted hover:text-nofx-text'
            }`}
          >
            <QrCode size={14} />
            {t.deposit}
          </button>
          {!unifiedAccount && (
            <button
              type="button"
              onClick={() => setTab('transfer')}
              className={`px-3 py-1.5 rounded-xl text-sm font-semibold flex items-center gap-1.5 ${
                tab === 'transfer'
                  ? 'bg-nofx-gold text-white'
                  : 'bg-nofx-bg text-nofx-text-muted hover:text-nofx-text'
              }`}
            >
              <ArrowDownUp size={14} />
              {t.transfer}
            </button>
          )}
        </div>
        <button
          type="button"
          onClick={() => void refresh()}
          className="text-nofx-text-muted hover:text-nofx-text"
          title={t.balances}
        >
          {loading ? (
            <Loader2 size={16} className="animate-spin" />
          ) : (
            <RefreshCw size={16} />
          )}
        </button>
      </div>

      <div className="grid grid-cols-3 gap-3 text-sm">
        <div className="rounded-xl border border-nofx-gold/30 bg-nofx-gold/5 p-3">
          <div className="text-nofx-text-muted text-xs">{t.wallet}</div>
          <div className="font-mono font-medium text-nofx-text">
            {formatUSDC(walletUsdc)} USDC
          </div>
          <div className="text-xs text-nofx-text-muted">
            {walletEth !== undefined && walletEth <= 0 ? (
              <span className="text-red-500">{t.noGas}</span>
            ) : (
              <>
                {t.gas}: {walletEth === undefined ? '--' : walletEth.toFixed(4)}{' '}
                ETH
              </>
            )}
          </div>
        </div>
        {unifiedAccount ? (
          <div className="rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg p-3 col-span-2">
            <div className="text-nofx-text-muted text-xs">{t.hlAccount}</div>
            <div className="font-mono font-medium text-nofx-text">
              {formatUSDC(account?.spotUsdc)} USDC
            </div>
            <div className="text-xs text-nofx-text-muted">
              {t.tradable}: {formatUSDC(account?.spotUsdcAvailable)} ·{' '}
              {t.marginInUse}:{' '}
              {account
                ? formatUSDC(account.spotUsdc - account.spotUsdcAvailable)
                : '--'}
            </div>
          </div>
        ) : (
          <>
            <div className="rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg p-3">
              <div className="text-nofx-text-muted text-xs">{t.spot}</div>
              <div className="font-mono font-medium text-nofx-text">
                {formatUSDC(account?.spotUsdc)} USDC
              </div>
              <div className="text-xs text-nofx-text-muted">
                {t.available}: {formatUSDC(account?.spotUsdcAvailable)}
              </div>
            </div>
            <div className="rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg p-3">
              <div className="text-nofx-text-muted text-xs">{t.perp}</div>
              <div className="font-mono font-medium text-nofx-text">
                {formatUSDC(account?.accountValue)} USDC
              </div>
              <div className="text-xs text-nofx-text-muted">
                {t.withdrawable}: {formatUSDC(account?.withdrawable)}
              </div>
            </div>
          </>
        )}
      </div>

      {tab === 'deposit' ? (
        <div className="space-y-3">
          <div className="flex justify-center rounded-xl bg-white p-4">
            <QRCodeSVG value={walletAddress} size={168} marginSize={1} />
          </div>
          <button
            type="button"
            onClick={() => void copyWithToast(walletAddress, t.addressCopied)}
            className="w-full font-mono text-xs text-center break-all text-nofx-text-muted hover:text-nofx-gold"
            title={t.copyAddress}
          >
            {walletAddress}
          </button>
          <p className="text-xs text-nofx-text-muted leading-5">{t.depositHint}</p>
          <p className="text-xs text-amber-500">{t.depositWarn}</p>
          <div className="rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg p-3 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-nofx-text">
                {t.bridgeTitle}
              </span>
              <span className="text-xs text-nofx-text-muted">
                {t.depositable}: {formatUSDC(walletUsdc)} USDC
              </span>
            </div>
            <div className="flex gap-2">
              <input
                type="number"
                min={MIN_BRIDGE_DEPOSIT_USDC}
                step="0.01"
                value={depositAmount}
                onChange={(e) => setDepositAmount(e.target.value)}
                className="flex-1 rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper px-3 py-1.5 text-sm font-mono text-nofx-text"
                placeholder={t.bridgeAmount}
              />
              <button
                type="button"
                onClick={() =>
                  walletUsdc !== undefined &&
                  setDepositAmount((Math.floor(walletUsdc * 100) / 100).toString())
                }
                className="px-3 py-1.5 rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper text-sm text-nofx-text-muted hover:text-nofx-text"
              >
                {t.max}
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => void submitBridgeDeposit()}
                className="flex items-center gap-2 rounded-xl border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-1.5 text-sm font-bold text-nofx-gold transition hover:bg-nofx-gold/20 disabled:opacity-60 disabled:cursor-not-allowed"
              >
                {busy && <Loader2 size={14} className="animate-spin" />}
                {busy ? t.bridgeSubmitting : t.bridgeSubmit}
              </button>
            </div>
            {error && <p className="text-xs text-red-500">{error}</p>}
            <p className="text-xs text-nofx-text-muted">
              {t.bridgeMin} · {t.bridgeGasHint}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setToPerp(true)}
              className={`flex-1 px-3 py-1.5 rounded-xl text-sm font-semibold ${
                toPerp
                  ? 'bg-nofx-gold text-white'
                  : 'bg-nofx-bg text-nofx-text-muted hover:text-nofx-text'
              }`}
            >
              {t.spotToPerp}
            </button>
            <button
              type="button"
              onClick={() => setToPerp(false)}
              className={`flex-1 px-3 py-1.5 rounded-xl text-sm font-semibold ${
                !toPerp
                  ? 'bg-nofx-gold text-white'
                  : 'bg-nofx-bg text-nofx-text-muted hover:text-nofx-text'
              }`}
            >
              {t.perpToSpot}
            </button>
          </div>
          <div>
            <label className="text-xs text-nofx-text-muted">{t.amount}</label>
            <div className="flex gap-2 mt-1">
              <input
                type="number"
                min="0"
                step="0.01"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className="flex-1 rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg px-3 py-1.5 text-sm font-mono text-nofx-text"
                placeholder="0.00"
              />
              <button
                type="button"
                onClick={() => setAmount(availableFrom.toFixed(2))}
                className="px-3 py-1.5 rounded-xl border border-[rgba(26,24,19,0.14)] bg-nofx-bg text-sm text-nofx-text-muted hover:text-nofx-text"
              >
                {t.max}
              </button>
            </div>
          </div>
          {error && <p className="text-xs text-red-500">{error}</p>}
          <button
            type="button"
            disabled={busy}
            onClick={() => void submitTransfer()}
            className="w-full flex items-center justify-center gap-2 rounded-xl border border-nofx-gold/30 bg-nofx-gold/10 px-4 py-2.5 text-sm font-bold text-nofx-gold transition hover:bg-nofx-gold/20 disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {busy && <Loader2 size={14} className="animate-spin" />}
            {busy ? t.submitting : t.submit}
          </button>
          <p className="text-xs text-nofx-text-muted">{t.connectFirst}</p>
        </div>
      )}
    </div>
  )
}
