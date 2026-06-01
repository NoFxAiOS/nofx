import React, { useState, useEffect } from 'react'
import { Eye, EyeOff, Loader2, ArrowRight } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { useAuth } from '../../contexts/AuthContext'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { LanguageSwitcher } from '../common/LanguageSwitcher'
import { invalidateSystemConfig } from '../../lib/config'

export function LoginPage() {
  const { language } = useLanguage()
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [expiredToastId, setExpiredToastId] = useState<string | number | null>(
    null
  )

  useEffect(() => {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
    localStorage.removeItem('user_id')
  }, [])

  useEffect(() => {
    if (sessionStorage.getItem('from401') === 'true') {
      const id = toast.warning(t('sessionExpired', language), {
        duration: Infinity,
      })
      setExpiredToastId(id)
      sessionStorage.removeItem('from401')
    }
  }, [language])

  const handleResetAccount = async () => {
    if (!window.confirm(t('forgotAccountConfirm', language))) return
    try {
      const res = await fetch('/api/reset-account', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          confirm: 'I_UNDERSTAND_THIS_DELETES_EVERYTHING',
        }),
      })
      if (res.ok) {
        localStorage.removeItem('auth_token')
        localStorage.removeItem('auth_user')
        localStorage.removeItem('user_id')
        sessionStorage.removeItem('from401')
        invalidateSystemConfig()
        toast.success(t('forgotAccountSuccess', language))
        setTimeout(() => navigate('/setup'), 1500)
      } else {
        const data = await res.json()
        toast.error(data.error || 'Reset failed')
      }
    } catch {
      toast.error('Network error')
    }
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await login(email, password)
    setLoading(false)
    if (result.success) {
      if (expiredToastId) toast.dismiss(expiredToastId)
    } else {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }
  }

  return (
    <DeepVoidBackground disableAnimation>
      <LanguageSwitcher />

      {/* Self-contained centering grid — works regardless of parent flex setup */}
      <main className="flex-1 grid lg:grid-cols-2">
        {/* ───────── LEFT: brand panel (desktop only) ───────── */}
        <section className="hidden lg:flex flex-col justify-between p-12 xl:p-16 relative overflow-hidden">
          {/* Ambient gold halo */}
          <div className="absolute -left-32 top-1/3 w-[28rem] h-[28rem] bg-nofx-gold/[0.06] rounded-full blur-3xl pointer-events-none" />
          <div className="absolute -right-16 bottom-0 w-72 h-72 bg-nofx-accent/[0.04] rounded-full blur-3xl pointer-events-none" />

          {/* Brand mark */}
          <div className="flex items-center gap-3 relative">
            <img src="/icons/nofx.svg" alt="NOFX" className="w-9 h-9" />
            <div className="font-mono font-bold text-xl tracking-tight text-white">
              NOFX<span className="text-nofx-gold">.</span>
            </div>
          </div>

          {/* Headline */}
          <div className="relative max-w-lg">
            <div className="inline-flex items-center gap-2 mb-7 px-3 py-1 rounded-full border border-nofx-success/25 bg-nofx-success/[0.06]">
              <div className="w-1.5 h-1.5 rounded-full bg-nofx-success animate-pulse" />
              <span className="text-[10.5px] font-mono tracking-[0.18em] text-nofx-success uppercase">
                Terminal Online
              </span>
            </div>
            <h2 className="text-4xl xl:text-5xl font-bold tracking-tight text-white leading-[1.05]">
              {language === 'zh' ? (
                <>
                  AI 驱动的<br />
                  <span className="bg-gradient-to-r from-nofx-gold to-yellow-300 bg-clip-text text-transparent">
                    多市场交易终端
                  </span>
                </>
              ) : language === 'id' ? (
                <>
                  Terminal Trading<br />
                  <span className="bg-gradient-to-r from-nofx-gold to-yellow-300 bg-clip-text text-transparent">
                    Multi-Pasar AI
                  </span>
                </>
              ) : (
                <>
                  AI-Powered<br />
                  <span className="bg-gradient-to-r from-nofx-gold to-yellow-300 bg-clip-text text-transparent">
                    Trading Terminal
                  </span>
                </>
              )}
            </h2>
            <p className="mt-5 text-zinc-400 text-base leading-relaxed max-w-md">
              {language === 'zh'
                ? '一键接入 Hyperliquid、OKX、Aster 等 10+ 交易所与 7 个 LLM 模型, 用自然语言部署 24/7 自动化策略.'
                : language === 'id'
                ? 'Hubungkan ke 10+ bursa termasuk Hyperliquid, OKX, Aster dan 7 model LLM. Terapkan strategi otomatis 24/7 dengan bahasa alami.'
                : 'Plug into 10+ exchanges including Hyperliquid, OKX, Aster, and 7 LLM models. Deploy 24/7 automated strategies with natural language.'}
            </p>
          </div>

          {/* Stats strip */}
          <div className="relative grid grid-cols-3 gap-8 max-w-md">
            <Stat
              value="10+"
              label={
                language === 'zh'
                  ? '交易所'
                  : language === 'id'
                  ? 'Bursa'
                  : 'Exchanges'
              }
            />
            <Stat
              value="7"
              label={
                language === 'zh'
                  ? 'AI 模型'
                  : language === 'id'
                  ? 'Model AI'
                  : 'AI Models'
              }
            />
            <Stat
              value="24/7"
              label={
                language === 'zh'
                  ? '全天候'
                  : language === 'id'
                  ? 'Sepanjang Waktu'
                  : 'Always On'
              }
            />
          </div>
        </section>

        {/* ───────── RIGHT: form panel ───────── */}
        <section className="flex items-center justify-center p-6 sm:p-12 relative">
          <div className="w-full max-w-sm">
            {/* Mobile brand */}
            <div className="lg:hidden flex flex-col items-center gap-3 mb-10">
              <img src="/icons/nofx.svg" alt="NOFX" className="w-12 h-12" />
              <div className="font-mono font-bold text-lg tracking-tight text-white">
                NOFX<span className="text-nofx-gold">.</span>
              </div>
            </div>

            {/* Form header */}
            <div className="mb-7">
              <h1 className="text-[26px] sm:text-3xl font-bold tracking-tight text-white">
                {t('signIn', language)}
              </h1>
              <p className="mt-1.5 text-sm text-zinc-500">
                {language === 'zh'
                  ? '使用您的邮箱继续'
                  : language === 'id'
                  ? 'Lanjutkan dengan email Anda'
                  : 'Continue with your email'}
              </p>
            </div>

            {/* Form */}
            <form onSubmit={handleLogin} className="space-y-4">
              {/* Email */}
              <div>
                <label className="block text-[10.5px] font-medium uppercase tracking-[0.14em] text-zinc-500 mb-2">
                  {t('email', language)}
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-zinc-900/60 border border-white/[0.08] rounded-lg px-4 py-[11px] text-[14px] text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/50 focus:bg-zinc-900 focus:ring-2 focus:ring-nofx-gold/20 transition-all"
                  placeholder="you@example.com"
                  required
                  autoFocus
                  autoComplete="email"
                />
              </div>

              {/* Password */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-[10.5px] font-medium uppercase tracking-[0.14em] text-zinc-500">
                    {t('password', language)}
                  </label>
                  <button
                    type="button"
                    onClick={() => navigate('/reset-password')}
                    className="text-xs text-zinc-500 hover:text-nofx-gold transition-colors"
                  >
                    {t('forgotPassword', language)}
                  </button>
                </div>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full bg-zinc-900/60 border border-white/[0.08] rounded-lg px-4 py-[11px] pr-11 text-[14px] text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/50 focus:bg-zinc-900 focus:ring-2 focus:ring-nofx-gold/20 transition-all"
                    placeholder="••••••••"
                    required
                    autoComplete="current-password"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 transition-colors"
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* Error banner */}
              {error && (
                <div className="flex items-start gap-2 rounded-lg border border-red-500/25 bg-red-500/[0.08] px-3 py-2.5 text-xs text-red-300">
                  <span className="text-red-400 font-bold mt-px">!</span>
                  <span className="leading-relaxed">{error}</span>
                </div>
              )}

              {/* Submit */}
              <button
                type="submit"
                disabled={loading}
                className="group mt-2 flex w-full items-center justify-center gap-2 rounded-lg bg-nofx-gold py-[11px] text-sm font-semibold text-black shadow-lg shadow-nofx-gold/10 transition-all hover:bg-yellow-400 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60"
              >
                {loading ? (
                  <>
                    <Loader2 size={16} className="animate-spin" />
                    {t('loggingIn', language) || 'Signing in...'}
                  </>
                ) : (
                  <>
                    {t('signIn', language)}
                    <ArrowRight
                      size={16}
                      className="transition-transform group-hover:translate-x-0.5"
                    />
                  </>
                )}
              </button>
            </form>

            {/* Footer */}
            <div className="mt-8 pt-5 border-t border-white/[0.06] flex items-center justify-between text-[11px]">
              <span className="font-mono text-zinc-600">v1.0</span>
              <button
                type="button"
                onClick={handleResetAccount}
                className="text-zinc-600 transition-colors hover:text-red-400"
              >
                {t('forgotAccount', language)}
              </button>
            </div>
          </div>
        </section>
      </main>
    </DeepVoidBackground>
  )
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div>
      <div className="font-mono text-2xl xl:text-3xl font-bold text-white">
        {value}
      </div>
      <div className="mt-1 text-[10.5px] uppercase tracking-[0.14em] text-zinc-500">
        {label}
      </div>
    </div>
  )
}
