import React, { useState, useEffect } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { getSystemConfig } from '../lib/config'
import { toast } from 'sonner'
import { copyWithToast } from '../lib/clipboard'
import { Eye, EyeOff } from 'lucide-react'
import { DeepVoidBackground } from './DeepVoidBackground'
// import { Input } from './ui/input' // Removed unused import
import PasswordChecklist from 'react-password-checklist'
import { RegistrationDisabled } from './RegistrationDisabled'
import { WhitelistFullPage } from './WhitelistFullPage'

export function RegisterPage() {
  const { language } = useLanguage()
  const { register, completeRegistration } = useAuth()
  const [step, setStep] = useState<'register' | 'setup-otp' | 'verify-otp' | 'whitelist-full'>(
    'register'
  )
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [betaCode, setBetaCode] = useState('')
  const [betaMode, setBetaMode] = useState(false)
  const [registrationEnabled, setRegistrationEnabled] = useState(true)
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [otpSecret, setOtpSecret] = useState('')
  const [qrCodeURL, setQrCodeURL] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  useEffect(() => {
    // 获取系统配置，检查是否开启内测模式和注册功能
    getSystemConfig()
      .then((config) => {
        setBetaMode(config.beta_mode || false)
        setRegistrationEnabled(config.registration_enabled !== false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
      })
  }, [])

  // 如果注册功能被禁用，显示注册已关闭页面
  if (!registrationEnabled) {
    return <RegistrationDisabled />
  }

  // 如果白名单已满，显示容量已满页面
  if (step === 'whitelist-full') {
    return <WhitelistFullPage onBack={() => setStep('register')} />
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    // 使用 PasswordChecklist 的校验结果
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

      // Helper to check for whitelist errors
      const isWhitelistError = (msg: string) => {
        const lowerMsg = msg.toLowerCase()
        return lowerMsg.includes('whitelist') ||
          lowerMsg.includes('capacity') ||
          lowerMsg.includes('limit') ||
          lowerMsg.includes('permission denied') ||
          lowerMsg.includes('not on whitelist')
      }

      if (result.success && result.userID) {
        setUserID(result.userID)
        setOtpSecret(result.otpSecret || '')
        setQrCodeURL(result.qrCodeURL || '')
        setStep('setup-otp')
      } else {
        // Check for whitelist/capacity limit error
        const msg = result.message || t('registrationFailed', language)
        if (isWhitelistError(msg)) {
          setStep('whitelist-full')
          return
        }
        setError(msg)
        toast.error(msg)
      }
    } catch (e) {
      console.error('Registration error:', e)
      const errorMsg = e instanceof Error ? e.message : 'Registration failed due to server error'

      // Check for whitelist error in catch block too
      const lowerMsg = errorMsg.toLowerCase()
      if (lowerMsg.includes('whitelist') ||
        lowerMsg.includes('capacity') ||
        lowerMsg.includes('limit') ||
        lowerMsg.includes('permission denied') ||
        lowerMsg.includes('not on whitelist')) {
        setStep('whitelist-full')
        return
      }

      setError(errorMsg)
      toast.error(errorMsg)
    } finally {
      setLoading(false)
    }
  }

  const handleSetupComplete = () => {
    setStep('verify-otp')
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await completeRegistration(userID, otpCode)

    if (!result.success) {
      const msg = result.message || t('registrationFailed', language)
      setError(msg)
      toast.error(msg)
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  const copyToClipboard = (text: string) => {
    copyWithToast(text)
  }

  return (
    <DeepVoidBackground className="min-h-screen flex items-center justify-center py-12 font-mono" disableAnimation>

      <div className="w-full max-w-lg relative z-10 px-6">
        {/* Navigation - Top Bar (Mobile/Desktop Friendly) */}
        <div className="flex justify-between items-center mb-8">
          <button
            onClick={() => window.location.href = '/'}
            className="flex items-center gap-2 text-zinc-500 hover:text-white transition-colors group px-3 py-1.5 rounded border border-transparent hover:border-zinc-700 bg-black/20 backdrop-blur-sm"
          >
            <div className="w-2 h-2 rounded-full bg-red-500 group-hover:animate-pulse"></div>
            <span className="text-xs font-mono uppercase tracking-widest">&lt; ABORT_REGISTRATION</span>
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
            <span className="text-nofx-gold">NEW_USER</span> ONBOARDING
          </h1>
          <p className="text-zinc-500 text-xs tracking-[0.2em] uppercase">
            {step === 'register' && 'Initializing Registration Sequence...'}
            {step === 'setup-otp' && 'Configuring Security Protocols...'}
            {step === 'verify-otp' && 'Finalizing Authentication...'}
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
              <span className="text-emerald-500">➜</span> setup_account.sh
            </div>
          </div>

          <div className="p-6 md:p-8 relative">
            {/* Status Output */}
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

            {step === 'register' && (
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
            )}

            {step === 'setup-otp' && (
              <div className="space-y-6">
                <div className="text-center bg-zinc-900/50 p-4 rounded border border-zinc-800">
                  <div className="text-xs font-mono text-zinc-400 mb-2">SCAN_QR_CODE_SEQUENCE</div>
                  {qrCodeURL ? (
                    <div className="bg-white p-2 rounded inline-block shadow-[0_0_30px_rgba(255,255,255,0.1)]">
                      <img
                        src={`https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=${encodeURIComponent(`otpauth://totp/NoFX:${email}?secret=${otpSecret}&issuer=NoFX`)}`}
                        alt="QR Code"
                        className="w-32 h-32"
                      />
                    </div>
                  ) : (
                    <div className="w-32 h-32 bg-zinc-800 animate-pulse rounded inline-block"></div>
                  )}
                  <div className="mt-4">
                    <p className="text-[10px] text-zinc-500 uppercase tracking-widest mb-1">Backup Secret Key</p>
                    <div className="flex items-center gap-2 justify-center bg-black/50 p-2 rounded border border-zinc-700/50 max-w-[200px] mx-auto">
                      <code className="text-xs font-mono text-nofx-gold">{otpSecret}</code>
                      <button
                        onClick={() => copyToClipboard(otpSecret)}
                        className="text-zinc-500 hover:text-white transition-colors"
                      >
                        <span className="text-[10px] uppercase border border-zinc-700 px-1 rounded">Copy</span>
                      </button>
                    </div>
                  </div>
                </div>

                <div className="space-y-4 font-mono text-xs text-zinc-400 bg-black/20 p-4 rounded border border-zinc-800/50">
                  <div className="flex gap-3 items-start">
                    <span className="text-nofx-gold font-bold mt-0.5">01</span>
                    <div>
                      <p className="font-bold text-white mb-1">Install Authenticator App</p>
                      <p className="mb-2">We highly recommend <span className="text-nofx-gold">Google Authenticator</span> for compatibility.</p>
                      <div className="flex gap-2">
                        <span className="px-1.5 py-0.5 bg-zinc-800 rounded text-[10px] text-zinc-300 border border-zinc-700">iOS</span>
                        <span className="px-1.5 py-0.5 bg-zinc-800 rounded text-[10px] text-zinc-300 border border-zinc-700">Android</span>
                      </div>
                    </div>
                  </div>

                  <div className="w-full h-px bg-zinc-800/50"></div>

                  <div className="flex gap-3 items-start">
                    <span className="text-nofx-gold font-bold mt-0.5">02</span>
                    <div>
                      <p className="font-bold text-white mb-1">Scan QR Code</p>
                      <p>Open Google Authenticator, tap the <span className="text-white">+</span> button, and scan the code above.</p>
                      <p className="text-[10px] text-zinc-500 mt-1 italic">Protocol: Time-Based OTP (TOTP)</p>
                    </div>
                  </div>

                  <div className="w-full h-px bg-zinc-800/50"></div>

                  <div className="flex gap-3 items-start">
                    <span className="text-nofx-gold font-bold mt-0.5">03</span>
                    <div>
                      <p className="font-bold text-white mb-1">Verify Token</p>
                      <p>Enter the 6-digit code generated by the app.</p>
                      <div className="mt-2 p-2 bg-yellow-500/10 border border-yellow-500/20 rounded text-[10px] text-yellow-500/80 flex gap-2 items-start">
                        <span className="mt-px">⚠️</span>
                        <span>Stuck? Ensure your phone's time is set to "Automatic". Time drift causes codes to fail.</span>
                      </div>
                    </div>
                  </div>
                </div>

                <button
                  onClick={handleSetupComplete}
                  className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-colors font-mono shadow-lg"
                >
                  PROCEED TO VERIFICATION
                </button>
              </div>
            )}

            {step === 'verify-otp' && (
              <form onSubmit={handleOTPVerify} className="space-y-6">
                <div className="text-center">
                  <p className="text-xs text-zinc-400 font-mono mb-6">
                    ENTER 6-DIGIT SECURITY TOKEN TO FINALIZE ONBOARDING
                  </p>
                </div>

                <div>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full bg-black border border-zinc-700 rounded px-4 py-4 text-center text-3xl tracking-[0.5em] font-mono text-white focus:border-nofx-gold focus:ring-1 focus:ring-nofx-gold/50 outline-none transition-all placeholder-zinc-800"
                    placeholder="000000"
                    maxLength={6}
                    required
                    autoFocus
                  />
                </div>

                {error && (
                  <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono text-center">
                    [VERIFICATION_FAILED]: {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6}
                  className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-colors font-mono shadow-lg disabled:opacity-50"
                >
                  {loading ? 'VALIDATING...' : 'ACTIVATE ACCOUNT'}
                </button>
              </form>
            )}

          </div>

          {/* Terminal Footer Info */}
          <div className="bg-zinc-900/50 p-3 flex justify-between items-center text-[10px] font-mono text-zinc-600 border-t border-zinc-800">
            <div>ENCRYPTION: AES-256</div>
            <div>SECURE_REGISTRY</div>
          </div>
        </div>

        {/* Login Link */}
        {step === 'register' && (
          <div className="text-center mt-8 space-y-4">
            <p className="text-xs font-mono text-zinc-500">
              EXISTING_OPERATOR?{' '}
              <button
                onClick={() => window.location.href = '/login'}
                className="text-nofx-gold hover:underline hover:text-yellow-300 transition-colors ml-1 uppercase"
              >
                ACCESS TERMINAL
              </button>
            </p>
            <button
              onClick={() => window.location.href = '/'}
              className="text-[10px] text-zinc-600 hover:text-red-500 transition-colors uppercase tracking-widest hover:underline decoration-red-500/30 font-mono"
            >
              [ ABORT_REGISTRATION_RETURN_HOME ]
            </button>
          </div>
        )}

      </div>
    </DeepVoidBackground>
  )
}
