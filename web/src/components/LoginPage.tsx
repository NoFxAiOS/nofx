import React, { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import HeaderBar from './landing/HeaderBar'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from './ui/input'

export function LoginPage() {
  const { language } = useLanguage()
  const { login, loginAdmin } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const adminMode = false

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await loginAdmin(adminPassword)
    if (!result.success) {
      setError(result.message || t('loginFailed', language))
    }
    setLoading(false)
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (!result.success) {
      setError(result.message || t('loginFailed', language))
    }
    // Success handled automatically by AuthContext (redirects to /traders)

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
              请输入您的邮箱和密码
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
            {adminMode ? (
              <form onSubmit={handleAdminLogin} className="space-y-4">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    管理员密码
                  </label>
                  <input
                    type="password"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--brand-black)',
                      border: '1px solid var(--panel-border)',
                      color: 'var(--brand-light-gray)',
                    }}
                    placeholder="请输入管理员密码"
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
                  {loading ? t('loading', language) : '登录'}
                </button>
              </form>
            ) : (
              <form onSubmit={handleLogin} className="space-y-4">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('email', language)}
                  </label>
                  <Input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
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
                  <div className="relative">
                    <Input
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="pr-10"
                      placeholder={t('passwordPlaceholder', language)}
                      required
                    />
                    <button
                      type="button"
                      aria-label={showPassword ? '隐藏密码' : '显示密码'}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => setShowPassword((v) => !v)}
                      className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center rounded bg-transparent p-0 m-0 border-0 outline-none focus:outline-none focus:ring-0 appearance-none cursor-pointer btn-icon"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                    </button>
                  </div>
                  <div className="text-right mt-2">
                    <button
                      type="button"
                      onClick={() => {
                        window.history.pushState({}, '', '/reset-password')
                        window.dispatchEvent(new PopStateEvent('popstate'))
                      }}
                      className="text-xs hover:underline"
                      style={{ color: '#F0B90B' }}
                    >
                      {t('forgotPassword', language)}
                    </button>
                  </div>
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
            )}
          </div>

          {/* Register Link */}
          {!adminMode && (
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
          )}
        </div>
      </div>
    </div>
  )
}
