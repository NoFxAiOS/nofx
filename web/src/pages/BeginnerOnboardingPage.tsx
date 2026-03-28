import { useEffect, useRef, useState } from 'react'
import { Copy, Eye, EyeOff, RefreshCw, Shield, Wallet } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import type { BeginnerOnboardingResponse } from '../types'
import { setBeginnerWalletAddress } from '../lib/onboarding'

export function BeginnerOnboardingPage() {
  const { language } = useLanguage()
  const [data, setData] = useState<BeginnerOnboardingResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showPrivateKey, setShowPrivateKey] = useState(true)
  const [refreshingBalance, setRefreshingBalance] = useState(false)
  const hasRequestedRef = useRef(false)
  const isZh = language === 'zh'

  const loadOnboarding = async (showLoading: boolean) => {
    if (showLoading) setLoading(true)
    else setRefreshingBalance(true)
    setError('')
    try {
      const result = await api.prepareBeginnerOnboarding()
      setData(result)
      setBeginnerWalletAddress(result.address)
    } catch (err) {
      setError(err instanceof Error ? err.message : isZh ? '新手钱包准备失败' : 'Failed to prepare beginner wallet')
    } finally {
      if (showLoading) setLoading(false)
      else setRefreshingBalance(false)
    }
  }

  useEffect(() => {
    if (hasRequestedRef.current) return
    hasRequestedRef.current = true
    void loadOnboarding(true)
  }, [])

  const copyText = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(isZh ? `${label}已复制` : `${label} copied`)
    } catch {
      toast.error(isZh ? '复制失败' : 'Copy failed')
    }
  }

  const handleContinue = () => {
    window.history.pushState({}, '', '/traders')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <DeepVoidBackground disableAnimation>
      <div className="mx-auto flex h-screen max-w-4xl flex-col justify-center px-4">
        {/* Header - compact */}
        <div className="mb-5 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-nofx-gold/15 text-nofx-gold">
            <Shield className="h-5 w-5" />
          </div>
          <div>
            <div className="text-[10px] font-semibold uppercase tracking-[0.3em] text-nofx-gold/80">
              {isZh ? '新手保护' : 'Beginner Guard'}
            </div>
            <h1 className="text-xl font-bold text-white">
              {isZh ? '钱包已经帮你准备好了' : 'Your wallet is ready'}
            </h1>
          </div>
          <div className="ml-auto text-xs text-zinc-500">
            Claw402 + DeepSeek · {isZh ? '按次付费' : 'Pay per call'}
          </div>
        </div>

        {error ? (
          <div className="mb-4 rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-2 text-sm text-red-300">{error}</div>
        ) : null}

        {/* Main card */}
        <section className="rounded-[24px] border border-white/10 bg-zinc-950/70 shadow-2xl backdrop-blur-xl">
          {loading ? (
            <div className="flex h-[400px] items-center justify-center text-sm text-zinc-400">
              {isZh ? '正在准备你的 Base 钱包...' : 'Preparing your Base wallet...'}
            </div>
          ) : data ? (
            <div className="grid gap-0 lg:grid-cols-[280px_1fr]">
              {/* Left: QR + Balance */}
              <div className="flex flex-col items-center border-b border-white/8 p-6 lg:border-b-0 lg:border-r">
                <div className="rounded-2xl bg-white p-2.5">
                  <QRCodeSVG value={data.address} size={120} level="M" />
                </div>
                <div className="mt-3 text-xs font-medium text-zinc-400">
                  {isZh ? '充值地址（Base USDC）' : 'Deposit (Base USDC)'}
                </div>
                <div className="mt-3 flex items-center gap-2 rounded-xl border border-emerald-500/20 bg-emerald-500/8 px-3 py-2">
                  <div>
                    <div className="text-lg font-bold text-emerald-300">{data.balance_usdc} USDC</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => void loadOnboarding(false)}
                    disabled={refreshingBalance}
                    className="rounded-lg border border-emerald-500/20 p-1.5 text-emerald-400 transition hover:bg-emerald-500/10 disabled:opacity-50"
                  >
                    <RefreshCw className={`h-3 w-3 ${refreshingBalance ? 'animate-spin' : ''}`} />
                  </button>
                </div>
                <div className="mt-2 text-[11px] text-zinc-600">
                  {isZh ? '$5-$10 可以用很久' : '$5-$10 lasts a long time'}
                </div>
              </div>

              {/* Right: Address + Key + Action */}
              <div className="flex flex-col gap-4 p-6">
                {/* Address */}
                <div>
                  <div className="flex items-center gap-2 text-xs font-medium text-zinc-400">
                    <Wallet className="h-3.5 w-3.5 text-nofx-gold" />
                    {isZh ? '钱包地址' : 'Wallet Address'}
                  </div>
                  <div className="mt-1.5 flex items-center gap-2">
                    <div className="min-w-0 flex-1 truncate rounded-lg bg-black/30 px-3 py-2 font-mono text-xs text-zinc-300">
                      {data.address}
                    </div>
                    <button
                      type="button"
                      onClick={() => copyText(data.address, isZh ? '地址' : 'Address')}
                      className="shrink-0 rounded-lg bg-white/10 p-2 text-zinc-400 transition hover:bg-white/15 hover:text-white"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                {/* Private Key */}
                <div>
                  <div className="flex items-center gap-2 text-xs font-medium text-amber-300/80">
                    <Shield className="h-3.5 w-3.5" />
                    {isZh ? '私钥 — 请立即备份' : 'Private Key — back up now'}
                    <button
                      type="button"
                      onClick={() => setShowPrivateKey((p) => !p)}
                      className="ml-auto rounded-lg p-1 text-amber-300/60 transition hover:text-amber-200"
                    >
                      {showPrivateKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                    </button>
                  </div>
                  <div className="mt-1.5 flex items-center gap-2">
                    <div className="min-w-0 flex-1 truncate rounded-lg bg-amber-500/8 border border-amber-500/15 px-3 py-2 font-mono text-xs text-amber-100">
                      {showPrivateKey ? data.private_key : '0x' + '•'.repeat(64)}
                    </div>
                    <button
                      type="button"
                      onClick={() => copyText(data.private_key, isZh ? '私钥' : 'Private key')}
                      className="shrink-0 rounded-lg bg-amber-500/10 border border-amber-500/15 p-2 text-amber-300 transition hover:bg-amber-500/20"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                {/* Tips */}
                <div className="rounded-lg bg-white/3 border border-white/5 px-3 py-2 text-[11px] leading-5 text-zinc-500">
                  {isZh
                    ? '• 此钱包仅用于大模型调用费用，不会自动充值交易所 • 私钥丢失后无法恢复 • 只充 Base 链 USDC'
                    : '• This wallet only covers LLM costs, not exchange funding • Private key cannot be recovered • Base USDC only'}
                </div>

                {/* Continue */}
                <button
                  type="button"
                  onClick={handleContinue}
                  className="mt-auto w-full rounded-xl bg-nofx-gold px-5 py-3 text-sm font-bold text-black transition hover:bg-yellow-400"
                >
                  {isZh ? '我已保存，进入下一步 →' : 'I saved it, continue →'}
                </button>
              </div>
            </div>
          ) : null}
        </section>
      </div>
    </DeepVoidBackground>
  )
}
