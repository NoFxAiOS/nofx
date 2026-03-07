import React, { useEffect, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import PasswordChecklist from 'react-password-checklist'
import { toast } from 'sonner'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { getSystemConfig } from '../lib/config'
import { DeepVoidBackground } from './DeepVoidBackground'
import { RegistrationDisabled } from './RegistrationDisabled'
import { WhitelistFullPage } from './WhitelistFullPage'

export function RegisterPage() {
  const { language } = useLanguage()
  const { register } = useAuth()
  const [view, setView] = useState<'register' | 'whitelist-full'>('register')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [betaCode, setBetaCode] = useState('')
  const [betaMode, setBetaMode] = useState(false)
  const [registrationEnabled, setRegistrationEnabled] = useState(true)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  useEffect(() => {
    getSystemConfig()
      .then((config) => {
        setBetaMode(config.beta_mode || false)
        setRegistrationEnabled(config.registration_enabled !== false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
      })
  }, [])

  if (!registrationEnabled) {
    return <RegistrationDisabled />
  }

  if (view === 'whitelist-full') {
    return <WhitelistFullPage onBack={() => setView('register')} />
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!passwordValid) {
      setError(t('passwordNotMeetRequirements', language))
      return
    }

    if (betaMode && !betaCode.trim()) {
      setError('内测期间，注册需要提供内测码')
      return
    }

    setLoading(true)
    try {
      const result = await register(email, password, betaCode.trim() || undefined)

      const isWhitelistError = (msg: string) => {
        const lowerMsg = msg.toLowerCase()
        return (
          lowerMsg.includes('whitelist') ||
          lowerMsg.includes('capacity') ||
          lowerMsg.includes('limit') ||
          lowerMsg.includes('permission denied') ||
          lowerMsg.includes('not on whitelist')
        )
      }

      if (!result.success) {
        const msg = (result as any).message || t('registrationFailed', language)
        if (isWhitelistError(msg)) {
          setView('whitelist-full')
          return
        }
        setError(msg)
        toast.error(msg)
      }
      // success path is handled in AuthContext (auto login + navigation)
    } catch (e) {
      console.error('Registration error:', e)
      const errorMsg = e instanceof Error ? e.message : 'Registration failed due to server error'
      const lowerMsg = errorMsg.toLowerCase()
      if (
        lowerMsg.includes('whitelist') ||
        lowerMsg.includes('capacity') ||
        lowerMsg.includes('limit') ||
        lowerMsg.includes('permission denied') ||
        lowerMsg.includes('not on whitelist')
      ) {
        setView('whitelist-full')
        return
      }
      setError(errorMsg)
      toast.error(errorMsg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <DeepVoidBackground className="min-h-screen flex items-center justify-center py-12 font-mono" disableAnimation>
      <div className="w-full max-w-lg relative z-10 px-6">
        <div className="flex justify-between items-center mb-8">
          <button
            onClick={() => (window.location.href = '/')}
            className="flex items-center gap-2 text-zinc-500 hover:text-white transition-colors group px-3 py-1.5 rounded border border-transparent hover:border-zinc-700 bg-black/20 backdrop-blur-sm"
          >
            <div className="w-2 h-2 rounded-full bg-red-500 group-hover:animate-pulse"></div>
            <span className="text-xs font-mono uppercase tracking-widest">&lt; ABORT_REGISTRATION</span>
          </button>
        </div>

        <div className="mb-8 text-center">
          <div className="flex justify-center mb-6">
            <div className="relative">
              <div className="absolute -inset-2 bg-nofx-gold/20 rounded-full blur-xl animate-pulse"></div>
              <img src="/icons/nofx.svg" alt="NoFx Logo" className="w-16 h-16 object-contain relative z-10 opacity-90" />
            </div>
          </div>
          <h1 className="text-3xl font-bold tracking-tighter text-white uppercase mb-2">
            <span className="text-nofx-gold">NEW_USER</span> ONBOARDING
          </h1>
          <p className="text-zinc-500 text-xs tracking-[0.2em] uppercase">
            Initializing Registration Sequence...
          </p>
        </div>

        <div className="bg-zinc-900/40 backdrop-blur-md border border-zinc-800 rounded-lg overflow-hidden shadow-2xl relative group">
          <div className="absolute inset-0 bg-zinc-900/50 opacity-0 group-hover:opacity-100 transition duration-700 pointer-events-none"></div>

          <div className="flex items-center justify-between px-4 py-2 bg-zinc-900/80 border-b border-zinc-800">
            <div className="flex gap-1.5">
              <div
                className="w-2.5 h-2.5 rounded-full bg-red-500/50 hover:bg-red-500 cursor-pointer transition-colors"
                onClick={() => (window.location.href = '/')}
                title="Close / Return Home"
              ></div>
              <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/50"></div>
              <div className="w-2.5 h-2.5 rounded-full bg-green-500/50"></div>
            </div>
            <div className="text-[10px] text-zinc-600 font-mono flex items-center gap-1">
              <span className="text-emerald-500">➜</span> setup_account.sh
            </div>
          </div>

          <div className="p-6 md:p-8 relative">
            <div className="mb-6 font-mono text-xs space-y-1 text-zinc-500 border-b border-zinc-800/50 pb-4">
              <div className="flex gap-2">
                <span className="text-emerald-500">➜</span>
                <span>System Check: <span className="text-emerald-500">READY</span></span>
              </div>
              <div className="flex gap-2">
                <span className="text-emerald-500">➜</span>
                <span>Mode: {betaMode ? 'CLOSED_BETA CA1' : 'PUBLIC'}</span>
              </div>
            </div>

            <form onSubmit={handleRegister} className="space-y-5">
              <div>
                <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5 ml-1 font-bold">{t('email', language)}</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800 text-white font-mono"
                  placeholder="user@nofx.os"
                  required
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5 ml-1 font-bold">{t('password', language)}</label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800 text-white font-mono pr-10"
                      placeholder="••••••••"
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
                </div>

                <div>
                  <label className="block text-xs uppercase tracking-wider text-zinc-500 mb-1.5 ml-1 font-bold">{t('confirmPassword', language)}</label>
                  <div className="relative">
                    <input
                      type={showConfirmPassword ? 'text' : 'password'}
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800 text-white font-mono pr-10"
                      placeholder="••••••••"
                      required
                    />
                    <button
                      type="button"
                      onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-600 hover:text-zinc-400 transition-colors"
                    >
                      {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>
              </div>

              <div className="bg-zinc-900/50 p-3 rounded border border-zinc-800/50">
                <div className="text-[10px] uppercase tracking-wider text-zinc-500 mb-2 font-bold flex items-center gap-2">
                  <div className="w-1 h-1 rounded-full bg-zinc-500"></div>
                  Password Strength Protocol
                </div>
                <div className="text-xs font-mono text-zinc-400">
                  <PasswordChecklist
                    rules={['minLength', 'capital', 'lowercase', 'number', 'specialChar', 'match']}
                    minLength={8}
                    value={password}
                    valueAgain={confirmPassword}
                    messages={{
                      minLength: t('passwordRuleMinLength', language),
                      capital: t('passwordRuleUppercase', language),
                      lowercase: t('passwordRuleLowercase', language),
                      number: t('passwordRuleNumber', language),
                      specialChar: t('passwordRuleSpecial', language),
                      match: t('passwordRuleMatch', language),
                    }}
                    className="grid grid-cols-2 gap-x-4 gap-y-1"
                    onChange={(isValid) => setPasswordValid(isValid)}
                    iconSize={10}
                  />
                </div>
              </div>

              {betaMode && (
                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-gold mb-1.5 ml-1 font-bold">Priority Access Code</label>
                  <input
                    type="text"
                    value={betaCode}
                    onChange={(e) => setBetaCode(e.target.value.replace(/[^a-z0-9]/gi, '').toLowerCase())}
                    className="w-full bg-black/50 border border-zinc-700 rounded px-4 py-3 text-sm focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800 text-white font-mono tracking-widest"
                    placeholder="XXXXXX"
                    maxLength={6}
                    required={betaMode}
                  />
                  <p className="text-[10px] text-zinc-600 font-mono mt-1 ml-1">* CASE SENSITIVE ALPHANUMERIC</p>
                </div>
              )}

              {error && (
                <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono">
                  [REGISTRATION_ERROR]: {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading || (betaMode && !betaCode.trim()) || !passwordValid}
                className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-all transform active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed font-mono shadow-[0_0_15px_rgba(255,215,0,0.1)] hover:shadow-[0_0_25px_rgba(255,215,0,0.25)] flex items-center justify-center gap-2 group mt-4"
              >
                {loading ? (
                  <span className="animate-pulse">INITIALIZING...</span>
                ) : (
                  <>
                    <span>CREATE_ACCOUNT</span>
                    <span className="group-hover:translate-x-1 transition-transform">-&gt;</span>
                  </>
                )}
              </button>
            </form>
          </div>

          <div className="bg-zinc-900/50 p-3 flex justify-between items-center text-[10px] font-mono text-zinc-600 border-t border-zinc-800">
            <div>ENCRYPTION: AES-256</div>
            <div>SECURE_REGISTRY</div>
          </div>
        </div>

        <div className="text-center mt-8 space-y-4">
          <p className="text-xs font-mono text-zinc-500">
            EXISTING_OPERATOR?{' '}
            <button
              onClick={() => (window.location.href = '/login')}
              className="text-nofx-gold hover:underline hover:text-yellow-300 transition-colors ml-1 uppercase"
            >
              ACCESS TERMINAL
            </button>
          </p>
          <button
            onClick={() => (window.location.href = '/')}
            className="text-[10px] text-zinc-600 hover:text-red-500 transition-colors uppercase tracking-widest hover:underline decoration-red-500/30 font-mono"
          >
            [ ABORT_REGISTRATION_RETURN_HOME ]
          </button>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
