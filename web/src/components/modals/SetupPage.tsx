import React, { useState, useEffect } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import { invalidateSystemConfig } from '../../lib/config'
import { OnboardingModeSelector } from '../auth/OnboardingModeSelector'
import type { UserMode } from '../../lib/onboarding'
import { useLanguage } from '../../contexts/LanguageContext'
import { LanguageSwitcher } from '../common/LanguageSwitcher'

const labels = {
  zh: {
    welcome: '欢迎使用 NOFX',
    subtitle: '创建账号开始使用',
    email: '邮箱',
    emailPlaceholder: 'you@example.com',
    password: '密码',
    passwordPlaceholder: '至少 8 个字符',
    passwordError: '密码至少需要 8 个字符',
    submit: '开始使用',
    submitting: '创建中...',
    setupFailed: '创建失败，请重试',
    singleUser: '单用户系统 — 这是唯一的账号',
  },
  en: {
    welcome: 'Welcome to NOFX',
    subtitle: 'Create your account to get started',
    email: 'Email',
    emailPlaceholder: 'you@example.com',
    password: 'Password',
    passwordPlaceholder: 'At least 8 characters',
    passwordError: 'Password must be at least 8 characters',
    submit: 'Get Started',
    submitting: 'Creating account...',
    setupFailed: 'Setup failed, please try again',
    singleUser: 'Single-user system — this is the only account',
  },
  id: {
    welcome: 'Selamat Datang di NOFX',
    subtitle: 'Buat akun untuk memulai',
    email: 'Email',
    emailPlaceholder: 'you@example.com',
    password: 'Kata Sandi',
    passwordPlaceholder: 'Minimal 8 karakter',
    passwordError: 'Kata sandi minimal 8 karakter',
    submit: 'Mulai',
    submitting: 'Membuat akun...',
    setupFailed: 'Gagal membuat akun, coba lagi',
    singleUser: 'Sistem pengguna tunggal — ini satu-satunya akun',
  },
} as const

export function SetupPage() {
  const { language } = useLanguage()
  const { register } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [mode, setMode] = useState<UserMode>('beginner')

  // Clean up any stale auth/onboarding state on setup page load
  useEffect(() => {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
    localStorage.removeItem('user_id')
    localStorage.removeItem('nofx_beginner_onboarding_completed')
    localStorage.removeItem('nofx_beginner_wallet_address')
  }, [])

  const l = labels[language as keyof typeof labels] || labels.en

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (password.length < 8) {
      setError(l.passwordError)
      return
    }
    setLoading(true)
    const result = await register(email, password, undefined, mode)
    setLoading(false)
    if (result.success) {
      invalidateSystemConfig()
    } else {
      setError(result.message || l.setupFailed)
    }
  }

  return (
    <div className="relative min-h-screen w-full overflow-hidden bg-[#0a0a0f]">
      {/* Decorative background - simulates the main app behind a modal */}

      {/* Grid */}
      <div className="absolute inset-0 pointer-events-none">
        <div className="absolute inset-x-0 bottom-0 h-[60vh] bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:40px_40px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] opacity-40" style={{ transform: 'perspective(500px) rotateX(60deg) translateY(80px) scale(2)' }} />
      </div>

      {/* Glow spots */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-[10%] left-[15%] w-[500px] h-[500px] bg-nofx-gold/8 rounded-full blur-[150px]" />
        <div className="absolute bottom-[5%] right-[10%] w-[400px] h-[400px] bg-indigo-500/6 rounded-full blur-[140px]" />
        <div className="absolute top-[40%] right-[30%] w-[300px] h-[300px] bg-emerald-500/4 rounded-full blur-[120px]" />
      </div>

      {/* Faux UI elements in background to simulate the app */}
      <div className="absolute inset-0 pointer-events-none opacity-[0.06]">
        {/* Fake header bar */}
        <div className="h-14 border-b border-white/20 flex items-center px-6 gap-4">
          <div className="w-8 h-8 rounded-lg bg-white/40" />
          <div className="h-3 w-20 rounded bg-white/30" />
          <div className="h-3 w-16 rounded bg-white/20 ml-4" />
          <div className="h-3 w-16 rounded bg-white/20" />
          <div className="h-3 w-16 rounded bg-white/20" />
        </div>
        {/* Fake content cards */}
        <div className="p-6 grid grid-cols-4 gap-4 mt-2">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-24 rounded-xl border border-white/15 bg-white/5" />
          ))}
        </div>
        <div className="px-6 mt-2">
          <div className="h-64 rounded-xl border border-white/15 bg-white/5" />
        </div>
      </div>

      {/* Blur overlay */}
      <div className="absolute inset-0 backdrop-blur-md bg-black/60" />

      <LanguageSwitcher />

      {/* Modal card */}
      <div className="relative z-10 flex min-h-screen items-center justify-center px-4 py-16">
        <div className="w-full max-w-sm animate-[fadeInUp_0.4s_ease-out]">

          {/* Logo + Title */}
          <div className="text-center mb-8">
            <div className="flex justify-center mb-4">
              <div className="relative">
                <div className="absolute -inset-4 bg-nofx-gold/20 rounded-full blur-2xl" />
                <img src="/icons/nofx.svg" alt="NOFX" className="w-14 h-14 relative z-10 drop-shadow-[0_0_15px_rgba(240,185,11,0.3)]" />
              </div>
            </div>
            <h1 className="text-2xl font-bold text-white mb-1.5">{l.welcome}</h1>
            <p className="text-zinc-500 text-sm">{l.subtitle}</p>
          </div>

          {/* Card */}
          <div className="bg-zinc-900/80 backdrop-blur-2xl border border-white/10 rounded-2xl p-8 shadow-[0_25px_60px_-15px_rgba(0,0,0,0.5),0_0_40px_-10px_rgba(240,185,11,0.08)]">
            <form onSubmit={handleSubmit} className="space-y-5">

              {/* Email */}
              <div>
                <label className="block text-xs font-medium text-zinc-400 mb-2">{l.email}</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                  placeholder={l.emailPlaceholder}
                  required
                  autoFocus
                />
              </div>

              {/* Password */}
              <div>
                <label className="block text-xs font-medium text-zinc-400 mb-2">{l.password}</label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full bg-black/40 border border-white/10 rounded-xl px-4 py-3 pr-11 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                    placeholder={l.passwordPlaceholder}
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

              <OnboardingModeSelector
                language={language}
                mode={mode}
                onChange={setMode}
              />

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
                className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed mt-2 shadow-[0_0_20px_rgba(240,185,11,0.2)]"
              >
                {loading ? l.submitting : l.submit}
              </button>
            </form>
          </div>

          <p className="text-center text-xs text-zinc-600 mt-6">
            {l.singleUser}
          </p>
        </div>
      </div>

      <style>{`
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>
  )
}
