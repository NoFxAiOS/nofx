import { useEffect, useMemo, useRef, useState } from 'react'
import { Copy, Eye, EyeOff, RefreshCw, Shield, Wallet, Sparkles } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { api } from '../lib/api'
import type { BeginnerOnboardingResponse } from '../types'
import { setBeginnerWalletAddress } from '../lib/onboarding'

export function BeginnerOnboardingPage() {
  const [data, setData] = useState<BeginnerOnboardingResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showPrivateKey, setShowPrivateKey] = useState(false)
  const [refreshingBalance, setRefreshingBalance] = useState(false)
  const hasRequestedRef = useRef(false)
  const isZh = navigator.language.toLowerCase().includes('zh')

  const loadOnboarding = async (showLoading: boolean) => {
    if (showLoading) {
      setLoading(true)
    } else {
      setRefreshingBalance(true)
    }

    setError('')
    try {
      const result = await api.prepareBeginnerOnboarding()
      setData(result)
      setBeginnerWalletAddress(result.address)
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : isZh
            ? '新手钱包准备失败'
            : 'Failed to prepare beginner wallet'
      )
    } finally {
      if (showLoading) {
        setLoading(false)
      } else {
        setRefreshingBalance(false)
      }
    }
  }

  useEffect(() => {
    if (hasRequestedRef.current) {
      return
    }
    hasRequestedRef.current = true
    void loadOnboarding(true)
  }, [isZh])

  const hints = useMemo(
    () =>
      isZh
        ? [
            '这是你的专属 Base 钱包，只用于后续调用大模型。',
            '请保存私钥。丢失后无法恢复。',
            '只往这个地址充值 Base 链 USDC，不要充到别的链。',
          ]
        : [
            'This dedicated Base wallet is only used to pay for model calls.',
            'Save the private key now. It cannot be recovered later.',
            'Deposit USDC on Base only. Do not send funds from another chain.',
          ],
    [isZh]
  )

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
      <div className="mx-auto flex min-h-screen max-w-5xl items-center px-4 py-12">
        <div className="grid w-full gap-8 lg:grid-cols-[1.05fr_0.95fr]">
          <section className="rounded-[28px] border border-white/10 bg-zinc-950/70 p-8 shadow-2xl backdrop-blur-xl">
            <div className="mb-6 flex items-center gap-3">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-nofx-gold/15 text-nofx-gold">
                <Shield className="h-6 w-6" />
              </div>
              <div>
                <div className="text-xs font-semibold uppercase tracking-[0.28em] text-nofx-gold/80">
                  {isZh ? '新手保护' : 'Beginner Guard'}
                </div>
                <h1 className="mt-1 text-3xl font-bold text-white">
                  {isZh ? '钱包已经帮你准备好了' : 'Your wallet is ready'}
                </h1>
              </div>
            </div>

            <p className="max-w-xl text-sm leading-7 text-zinc-300">
              {isZh
                ? '我们已经为你生成了一个专属钱包，并默认接入 Claw402 + DeepSeek。你现在只需要保存私钥，然后往这个地址充值 Base 链 USDC，后面调用大模型时会自动从这里扣费。'
                : 'We generated a dedicated wallet for you and preconfigured Claw402 + DeepSeek. Save the private key, then deposit Base USDC to this address so future model calls can be paid automatically.'}
            </p>

            <div className="mt-6 grid gap-3">
              {hints.map((hint) => (
                <div
                  key={hint}
                  className="flex items-start gap-3 rounded-2xl border border-white/8 bg-white/5 px-4 py-3"
                >
                  <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-nofx-gold" />
                  <div className="text-sm leading-6 text-zinc-300">{hint}</div>
                </div>
              ))}
            </div>

            <div className="mt-8 rounded-[24px] border border-sky-500/20 bg-sky-500/5 p-5">
              <div className="flex items-center gap-2 text-sm font-semibold text-sky-300">
                <Wallet className="h-4 w-4" />
                <span>{isZh ? '为什么要充值？' : 'Why fund this wallet?'}</span>
              </div>
              <p className="mt-2 text-sm leading-6 text-zinc-300">
                {isZh
                  ? '这里只负责大模型调用费用，不会自动替你充值交易所。先充少量 USDC 就够了，通常 $5-$10 可以用很久。'
                  : 'This wallet only covers LLM usage costs. It does not fund your exchange automatically. A small amount of USDC is enough to get started, usually $5-$10.'}
              </p>
            </div>

            {error ? (
              <div className="mt-6 rounded-2xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                {error}
              </div>
            ) : null}
          </section>

          <section className="rounded-[28px] border border-white/10 bg-black/60 p-8 shadow-2xl backdrop-blur-xl">
            {loading ? (
              <div className="flex min-h-[420px] items-center justify-center text-sm text-zinc-400">
                {isZh ? '正在准备你的 Base 钱包...' : 'Preparing your Base wallet...'}
              </div>
            ) : data ? (
              <div className="space-y-6">
                <div className="rounded-[24px] border border-white/10 bg-white/5 p-5">
                  <div className="text-xs uppercase tracking-[0.28em] text-zinc-500">
                    {isZh ? '默认模型' : 'Default Model'}
                  </div>
                  <div className="mt-2 text-2xl font-bold text-white">Claw402 + DeepSeek</div>
                  <div className="mt-2 text-sm text-zinc-400">
                    {isZh ? '按次付费，无需 API Key' : 'Pay per call, no API key needed'}
                  </div>
                </div>

                <div className="rounded-[24px] border border-white/10 bg-white p-5 text-center">
                  <div className="inline-flex rounded-2xl bg-white p-3">
                    <QRCodeSVG value={data.address} size={180} level="M" />
                  </div>
                  <div className="mt-4 text-sm font-semibold text-zinc-900">
                    {isZh ? '充值地址（Base 链 USDC）' : 'Deposit Address (Base USDC)'}
                  </div>
                  <div className="mt-2 break-all rounded-2xl bg-zinc-100 px-3 py-3 font-mono text-xs text-zinc-700">
                    {data.address}
                  </div>
                  <button
                    type="button"
                    onClick={() => copyText(data.address, isZh ? '地址' : 'Address')}
                    className="mt-3 inline-flex items-center gap-2 rounded-xl bg-zinc-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-zinc-800"
                  >
                    <Copy className="h-4 w-4" />
                    {isZh ? '复制地址' : 'Copy address'}
                  </button>

                  <div className="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-left">
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-emerald-700">
                          {isZh ? '当前余额' : 'Current Balance'}
                        </div>
                        <div className="mt-1 text-2xl font-bold text-emerald-900">
                          {data.balance_usdc} USDC
                        </div>
                        <div className="mt-1 text-xs text-emerald-700/80">
                          {isZh ? 'Base 链钱包余额' : 'Base wallet balance'}
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => void loadOnboarding(false)}
                        disabled={refreshingBalance}
                        className="inline-flex items-center gap-2 rounded-xl border border-emerald-300 bg-white px-3 py-2 text-xs font-semibold text-emerald-800 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <RefreshCw className={`h-3.5 w-3.5 ${refreshingBalance ? 'animate-spin' : ''}`} />
                        {isZh ? '刷新余额' : 'Refresh'}
                      </button>
                    </div>
                  </div>
                </div>

                <div className="rounded-[24px] border border-amber-500/20 bg-amber-500/8 p-5">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-amber-200">
                        {isZh ? '钱包私钥' : 'Wallet Private Key'}
                      </div>
                      <div className="mt-1 text-xs leading-5 text-amber-100/75">
                        {isZh ? '请先备份，再进入下一步。' : 'Back this up before you continue.'}
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => setShowPrivateKey((prev) => !prev)}
                      className="rounded-xl border border-amber-400/20 px-3 py-2 text-amber-200 transition hover:bg-amber-400/10"
                    >
                      {showPrivateKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  <div className="mt-4 break-all rounded-2xl bg-black/25 px-3 py-3 font-mono text-xs text-amber-50">
                    {showPrivateKey ? data.private_key : '0x' + '•'.repeat(64)}
                  </div>
                  <button
                    type="button"
                    onClick={() => copyText(data.private_key, isZh ? '私钥' : 'Private key')}
                    className="mt-3 inline-flex items-center gap-2 rounded-xl border border-amber-300/20 px-4 py-2 text-sm font-semibold text-amber-100 transition hover:bg-amber-400/10"
                  >
                    <Copy className="h-4 w-4" />
                    {isZh ? '复制私钥' : 'Copy private key'}
                  </button>
                </div>

                <div className="rounded-[24px] border border-white/10 bg-white/5 p-4 text-xs leading-6 text-zinc-400">
                  <div>
                    {data.env_saved
                      ? isZh
                        ? `已同步保存到环境文件：${data.env_path || '.env'}`
                        : `Also saved to env: ${data.env_path || '.env'}`
                      : isZh
                        ? '当前运行环境没有成功写回 .env，但产品已完成默认配置。'
                        : 'The app is configured, but this runtime could not write back to .env.'}
                  </div>
                  {data.env_warning ? <div className="mt-2 text-amber-300">{data.env_warning}</div> : null}
                </div>

                <button
                  type="button"
                  onClick={handleContinue}
                  className="w-full rounded-2xl bg-nofx-gold px-5 py-4 text-sm font-bold text-black transition hover:bg-yellow-400"
                >
                  {isZh ? '我已保存，进入下一步' : 'I saved it, continue'}
                </button>
              </div>
            ) : null}
          </section>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
