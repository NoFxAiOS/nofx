import React, { useState, useEffect } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '../../contexts/AuthContext'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { DeepVoidBackground } from '../common/DeepVoidBackground'

export function LoginPage() {
  const { language } = useLanguage()
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [expiredToastId, setExpiredToastId] = useState<string | number | null>(null)

  useEffect(() => {
    if (sessionStorage.getItem('from401') === 'true') {
      const id = toast.warning(t('sessionExpired', language), { duration: Infinity })
      setExpiredToastId(id)
      sessionStorage.removeItem('from401')
    }
  }, [language])

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
      <div className="flex-1 flex items-center justify-center px-4 py-16">
        <div className="w-full max-w-sm">

          {/* Logo + Title */}
          <div className="text-center mb-10">
            <div className="flex justify-center mb-5">
              <div className="relative">
                <div className="absolute -inset-3 bg-nofx-gold/15 rounded-full blur-2xl" />
                <img src="/icons/nofx.svg" alt="NOFX" className="w-14 h-14 relative z-10" />
              </div>
            </div>
            <h1 className="text-2xl font-bold text-white mb-1.5">Welcome back</h1>
            <p className="text-zinc-500 text-sm">Sign in to your account</p>
          </div>

          {/* Card */}
          <div className="bg-zinc-900/60 backdrop-blur-xl border border-zinc-800/80 rounded-2xl p-8 shadow-2xl">
            <form onSubmit={handleLogin} className="space-y-5">

              {/* Email */}
              <div>
                <label className="block text-xs font-medium text-zinc-400 mb-2">
                  {t('email', language)}
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-zinc-950/80 border border-zinc-700/80 rounded-xl px-4 py-3 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                  placeholder="you@example.com"
                  required
                  autoFocus
                />
              </div>

              {/* Password */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-xs font-medium text-zinc-400">
                    {t('password', language)}
                  </label>
                  <button
                    type="button"
                    onClick={() => window.location.href = '/reset-password'}
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
                    className="w-full bg-zinc-950/80 border border-zinc-700/80 rounded-xl px-4 py-3 pr-11 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                    placeholder="••••••••"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 transition-colors"
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* Error */}
              {error && (
                <p className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
                  {error}
                </p>
              )}

              {/* Submit */}
              <button
                type="submit"
                disabled={loading}
                className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed mt-2"
              >
                {loading ? t('loggingIn', language) || 'Signing in...' : t('signIn', language) || 'Sign In'}
              </button>
            </form>
          </div>

        </div>
      </div>
    </DeepVoidBackground>
  )
}
