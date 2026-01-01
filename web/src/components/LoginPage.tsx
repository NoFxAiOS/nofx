import React, { useState, useEffect } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { Eye, EyeOff } from 'lucide-react'
// import { Input } from './ui/input' // Removed unused import
import { toast } from 'sonner'
import { useSystemConfig } from '../hooks/useSystemConfig'

export function LoginPage() {
  const { language } = useLanguage()
  const { login, loginAdmin, verifyOTP } = useAuth()
  const [step, setStep] = useState<'login' | 'otp'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const adminMode = false
  const { config: systemConfig } = useSystemConfig()
  const registrationEnabled = systemConfig?.registration_enabled !== false
  const [expiredToastId, setExpiredToastId] = useState<string | number | null>(null)

  // Show notification if user was redirected here due to 401
  useEffect(() => {
    if (sessionStorage.getItem('from401') === 'true') {
      const id = toast.warning(t('sessionExpired', language), {
        duration: Infinity // Keep showing until user dismisses or logs in
      })
      setExpiredToastId(id)
      sessionStorage.removeItem('from401')
    }
  }, [language])

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await loginAdmin(adminPassword)
    if (!result.success) {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    } else {
      // Dismiss the "login expired" toast on successful login
      if (expiredToastId) {
        toast.dismiss(expiredToastId)
      }
    }
    setLoading(false)
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (result.success) {
      if (result.requiresOTP && result.userID) {
        setUserID(result.userID)
        setStep('otp')
      } else {
        // Dismiss the "login expired" toast on successful login (no OTP required)
        if (expiredToastId) {
          toast.dismiss(expiredToastId)
        }
      }
    } else {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }

    setLoading(false)
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await verifyOTP(userID, otpCode)

    if (!result.success) {
      const msg = result.message || t('verificationFailed', language)
      setError(msg)
      toast.error(msg)
    } else {
      // Dismiss the "login expired" toast on successful OTP verification
      if (expiredToastId) {
        toast.dismiss(expiredToastId)
      }
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  return (
    <div className="min-h-screen bg-black text-zinc-300 font-mono relative overflow-hidden flex items-center justify-center py-12">
      {/* Background Effects */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px] pointer-events-none"></div>
      <div className="absolute inset-0 bg-gradient-to-t from-black via-transparent to-transparent pointer-events-none"></div>

      {/* Scanline Effect */}
      <div className="absolute inset-0 pointer-events-none opacity-[0.03] bg-[linear-gradient(transparent_50%,rgba(0,0,0,0.5)_50%)] bg-[length:100%_4px]"></div>

      <div className="w-full max-w-md relative z-10 px-6">
        {/* Navigation - Top Bar (Mobile/Desktop Friendly) */}
        <div className="flex justify-between items-center mb-8">
          <button
            onClick={() => window.location.href = '/'}
            className="flex items-center gap-2 text-zinc-500 hover:text-white transition-colors group px-3 py-1.5 rounded border border-transparent hover:border-zinc-700 bg-black/20 backdrop-blur-sm"
          >
            <div className="w-2 h-2 rounded-full bg-red-500 group-hover:animate-pulse"></div>
            <span className="text-xs font-mono uppercase tracking-widest">&lt; CANCEL_LOGIN</span>
          </button>
        </div>

        {/* Terminal Header */}
        <div className="mb-8 text-center">
          <div className="flex justify-center mb-6">
            <div className="relative">
              <div className="absolute -inset-2 bg-nofx-gold/20 rounded-full blur-xl animate-pulse"></div>
              <img
                src="/icons/nofx.svg"
                alt="NoFx Logo"
                className="w-16 h-16 object-contain relative z-10 opacity-90"
              />
            </div>
          </div>
          <h1 className="text-3xl font-bold tracking-tighter text-white uppercase mb-2">
            <span className="text-nofx-gold">SYSTEM</span> ACCESS
          </h1>
          <p className="text-zinc-500 text-xs tracking-[0.2em] uppercase">
            {step === 'login' ? 'Authentication Protocol v3.0' : 'Multi-Factor Verification'}
          </p>
        </div>

        {/* Terminal Output / Form Container */}
        <div className="bg-zinc-900/40 backdrop-blur-md border border-zinc-800 rounded-lg overflow-hidden shadow-2xl relative group">
          <div className="absolute inset-0 bg-zinc-900/50 opacity-0 group-hover:opacity-100 transition duration-700 pointer-events-none"></div>

          {/* Window Bar */}
          <div className="flex items-center justify-between px-4 py-2 bg-zinc-900/80 border-b border-zinc-800">
            <div className="flex gap-1.5">
              <div
                className="w-2.5 h-2.5 rounded-full bg-red-500/50 hover:bg-red-500 cursor-pointer transition-colors"
                onClick={() => window.location.href = '/'}
                title="Close / Return Home"
              ></div>
              <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/50"></div>
              <div className="w-2.5 h-2.5 rounded-full bg-green-500/50"></div>
            </div>
            <div className="text-[10px] text-zinc-600 font-mono flex items-center gap-1">
              <span className="text-emerald-500">➜</span> login.exe
            </div>
          </div>

          <div className="p-6 md:p-8 relative">
            {/* Status Output */}
            <div className="mb-6 font-mono text-xs space-y-1 text-zinc-500 border-b border-zinc-800/50 pb-4">
              <div className="flex gap-2">
                <span className="text-emerald-500">➜</span>
                <span>Initiating handshake...</span>
              </div>
              <div className="flex gap-2">
                <span className="text-emerald-500">➜</span>
                <span>Target: NOFX CORE HUB</span>
              </div>
              <div className="flex gap-2">
                <span className="text-emerald-500">➜</span>
                <span>Status: <span className="text-zinc-300">AWAITING CREDENTIALS</span></span>
              </div>
            </div>

            {adminMode ? (
              <form onSubmit={handleAdminLogin} className="space-y-5">
                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-gold mb-1.5 ml-1">Admin Key</label>
                  <input
                    type="password"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                    className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-700 text-white font-mono"
                    placeholder="ENTER_ROOT_PASSWORD"
                    required
                  />
                </div>

                {error && (
                  <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono">
                    [ERROR]: {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-all transform active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed font-mono shadow-[0_0_20px_rgba(255,215,0,0.1)] hover:shadow-[0_0_30px_rgba(255,215,0,0.3)]"
                >
                  {loading ? '> VERIFYING...' : '> EXECUTE_LOGIN'}
                </button>
              </form>
            ) : step === 'login' ? (
              <form onSubmit={handleLogin} className="space-y-5">
                <div className="space-y-4">
                  <div>
                    <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5 ml-1 font-bold">{t('email', language)}</label>
                    <input
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-700 text-white font-mono"
                      placeholder="user@nofx.os"
                      required
                    />
                  </div>

                  <div>
                    <div className="flex items-center justify-between mb-1.5 ml-1">
                      <label className="block text-xs uppercase tracking-wider text-zinc-500 font-bold">{t('password', language)}</label>
                    </div>

                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-700 text-white font-mono pr-10"
                        placeholder="••••••••••••"
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-600 hover:text-zinc-400 transition-colors"
                      >
                        {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </button>
                    </div>
                    <div className="text-right mt-2">
                      <button
                        type="button"
                        onClick={() => window.location.href = '/reset-password'}
                        className="text-[10px] uppercase tracking-wide text-zinc-500 hover:text-nofx-gold transition-colors"
                      >
                        &gt; {t('forgotPassword', language)}
                      </button>
                    </div>
                  </div>
                </div>

                {error && (
                  <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono flex gap-2 items-start">
                    <span>⚠</span> <span>{error}</span>
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-all transform active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed font-mono shadow-[0_0_15px_rgba(255,215,0,0.1)] hover:shadow-[0_0_25px_rgba(255,215,0,0.25)] flex items-center justify-center gap-2 group"
                >
                  {loading ? (
                    <span className="animate-pulse">PROCESSING...</span>
                  ) : (
                    <>
                      <span>AUTHENTICATE</span>
                      <span className="group-hover:translate-x-1 transition-transform">-&gt;</span>
                    </>
                  )}
                </button>
              </form>
            ) : (
              <form onSubmit={handleOTPVerify} className="space-y-6">
                <div className="text-center py-2">
                  <div className="w-12 h-12 bg-zinc-900 rounded-full flex items-center justify-center mx-auto mb-4 border border-zinc-700 text-2xl">
                    🔐
                  </div>
                  <p className="text-xs text-zinc-400 font-mono leading-relaxed">
                    {t('scanQRCodeInstructions', language)}<br />
                    {t('enterOTPCode', language)}
                  </p>
                </div>

                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-gold mb-2 text-center font-bold">
                    {t('otpCode', language)}
                  </label>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full bg-black border border-zinc-700 rounded px-4 py-4 text-center text-2xl tracking-[0.5em] font-mono text-white focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800"
                    placeholder="000000"
                    maxLength={6}
                    required
                    autoFocus
                  />
                </div>

                {error && (
                  <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono text-center">
                    [ACCESS DENIED]: {error}
                  </div>
                )}

                <div className="flex gap-3 pt-2">
                  <button
                    type="button"
                    onClick={() => setStep('login')}
                    className="flex-1 bg-zinc-900 border border-zinc-700 text-zinc-400 py-3 rounded text-xs font-mono uppercase hover:bg-zinc-800 transition-colors"
                  >
                    &lt; ABORT
                  </button>
                  <button
                    type="submit"
                    disabled={loading || otpCode.length !== 6}
                    className="flex-1 bg-nofx-gold text-black font-bold py-3 rounded text-xs font-mono uppercase hover:bg-yellow-400 transition-colors disabled:opacity-50"
                  >
                    {loading ? 'VERIFYING...' : 'CONFIRM IDENTITY'}
                  </button>
                </div>
              </form>
            )}
          </div>

          {/* Terminal Footer Info */}
          <div className="bg-zinc-900/50 p-3 flex justify-between items-center text-[10px] font-mono text-zinc-600 border-t border-zinc-800">
            <div>SECURE_CONNECTION: ENCRYPTED</div>
            <div>{new Date().toISOString().split('T')[0]}</div>
          </div>
        </div>

        {/* Register Link */}
        {!adminMode && registrationEnabled && (
          <div className="text-center mt-8 space-y-4">
            <p className="text-xs font-mono text-zinc-500">
              NEW_USER_DETECTED?{' '}
              <button
                onClick={() => window.location.href = '/register'}
                className="text-nofx-gold hover:underline hover:text-yellow-300 transition-colors ml-1 uppercase"
              >
                INITIALIZE REGISTRATION
              </button>
            </p>
            <button
              onClick={() => window.location.href = '/'}
              className="text-[10px] text-zinc-600 hover:text-red-500 transition-colors uppercase tracking-widest hover:underline decoration-red-500/30 font-mono"
            >
              [ ABORT_SESSION_RETURN_HOME ]
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
