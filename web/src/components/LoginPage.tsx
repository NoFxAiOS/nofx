import React, { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import HeaderBar from './landing/HeaderBar'

export function LoginPage() {
  const { language } = useLanguage()
  const { login, verifyOTP } = useAuth()
  const [step, setStep] = useState<'login' | 'otp'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (result.success) {
      if (result.requiresOTP && result.userID) {
        setUserID(result.userID)
        setStep('otp')
      }
    } else {
      setError(result.message || t('loginFailed', language))
    }

    setLoading(false)
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await verifyOTP(userID, otpCode)

    if (!result.success) {
      setError(result.message || t('verificationFailed', language))
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  return (
    <div className="min-h-screen" style={{ background: 'var(--brand-black)' }}>
      <HeaderBar
        onLoginClick={() => {}}
        isLoggedIn={false}
        isHomePage={false}
        currentPage="login"
        language={language}
        onLanguageChange={() => {}}
        onPageChange={(page) => {
          console.log('LoginPage onPageChange called with:', page)
          if (page === 'competition') {
            window.location.href = '/competition'
          }
        }}
      />

      <div
        className="flex items-center justify-center pt-20"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md">
          {/* Logo */}
          <div className="text-center mb-8">
            <div className="w-16 h-16 mx-auto mb-4 flex items-center justify-center">
              <img
                src="/icons/nofx.svg"
                alt="NoFx Logo"
                className="w-16 h-16 object-contain"
              />
            </div>
            <h1
              className="text-2xl font-bold"
              style={{ color: 'var(--brand-light-gray)' }}
            >
              登录 NOFX
            </h1>
            <p
              className="text-sm mt-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {step === 'login' ? '请输入您的邮箱和密码' : '请输入两步验证码'}
            </p>
          </div>

          {/* Login Form */}
          <div
            className="rounded-lg p-6"
            style={{
              background: 'var(--panel-bg)',
              border: '1px solid var(--panel-border)',
            }}
          >
            {step === 'login' ? (
              <form onSubmit={handleLogin} className="space-y-4">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('email', language)}
                  </label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--brand-black)',
                      border: '1px solid var(--panel-border)',
                      color: 'var(--brand-light-gray)',
                    }}
                    placeholder={t('emailPlaceholder', language)}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('password', language)}
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--brand-black)',
                      border: '1px solid var(--panel-border)',
                      color: 'var(--brand-light-gray)',
                    }}
                    placeholder={t('passwordPlaceholder', language)}
                    required
                  />
                </div>

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded"
                    style={{
                      background: 'var(--binance-red-bg)',
                      color: 'var(--binance-red)',
                    }}
                  >
                    {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                  style={{
                    background: 'var(--brand-yellow)',
                    color: 'var(--brand-black)',
                  }}
                >
                  {loading
                    ? t('loading', language)
                    : t('loginButton', language)}
                </button>
              </form>
            ) : (
              <form onSubmit={handleOTPVerify} className="space-y-4">
                <div className="text-center mb-4">
                  <div className="text-4xl mb-2">📱</div>
                  <p className="text-sm" style={{ color: '#848E9C' }}>
                    {t('scanQRCodeInstructions', language)}
                    <br />
                    {t('enterOTPCode', language)}
                  </p>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('otpCode', language)}
                  </label>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                    style={{
                      background: 'var(--brand-black)',
                      border: '1px solid var(--panel-border)',
                      color: 'var(--brand-light-gray)',
                    }}
                    placeholder={t('otpPlaceholder', language)}
                    maxLength={6}
                    required
                  />
                </div>

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded"
                    style={{
                      background: 'var(--binance-red-bg)',
                      color: 'var(--binance-red)',
                    }}
                  >
                    {error}
                  </div>
                )}

                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={() => setStep('login')}
                    className="flex-1 px-4 py-2 rounded text-sm font-semibold"
                    style={{
                      background: 'var(--panel-bg-hover)',
                      color: 'var(--text-secondary)',
                    }}
                  >
                    {t('back', language)}
                  </button>
                  <button
                    type="submit"
                    disabled={loading || otpCode.length !== 6}
                    className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                    style={{ background: '#F0B90B', color: '#000' }}
                  >
                    {loading
                      ? t('loading', language)
                      : t('verifyOTP', language)}
                  </button>
                </div>
              </form>
            )}
          </div>

          {/* Register Link */}
          <div className="text-center mt-6">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              还没有账户？{' '}
              <button
                onClick={() => {
                  window.history.pushState({}, '', '/register')
                  window.dispatchEvent(new PopStateEvent('popstate'))
                }}
                className="font-semibold hover:underline transition-colors"
                style={{ color: 'var(--brand-yellow)' }}
              >
                立即注册
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
