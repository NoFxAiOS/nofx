import React, { useState, useEffect } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { getSystemConfig } from '../lib/config'
import HeaderBar from './landing/HeaderBar'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from './ui/input'
import PasswordChecklist from 'react-password-checklist'

export function RegisterPage() {
  const { language } = useLanguage()
  const { register } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [betaCode, setBetaCode] = useState('')
  const [betaMode, setBetaMode] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  useEffect(() => {
    // 获取系统配置，检查是否开启内测模式
    getSystemConfig()
      .then((config) => {
        setBetaMode(config.beta_mode || false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
      })
  }, [])

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    // 客户端强校验：长度>=8，包含大小写、数字、特殊字符，且两次一致
    const strong = isStrongPassword(password)
    if (!strong || password !== confirmPassword) {
      setError(t('passwordNotMeetRequirements', language))
      return
    }

    if (betaMode && !betaCode.trim()) {
      setError('内测期间，注册需要提供内测码')
      return
    }

    setLoading(true)

    const result = await register(email, password, betaCode.trim() || undefined)

    if (!result.success) {
      setError(result.message || t('registrationFailed', language))
    }
    // Success handled automatically by AuthContext (redirects to /traders)

    setLoading(false)
  }

  return (
    <div className="min-h-screen" style={{ background: 'var(--brand-black)' }}>
      <HeaderBar
        isLoggedIn={false}
        currentPage="register"
        language={language}
        onLanguageChange={() => {}}
        onPageChange={(page) => {
          console.log('RegisterPage onPageChange called with:', page)
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
            <h1 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
              {t('register', language)}
            </h1>
            <p className="text-sm mt-2" style={{ color: '#848E9C' }}>
              {t('createAccount', language)}
            </p>
          </div>

          {/* Registration Form */}
          <div
            className="rounded-lg p-6"
            style={{
              background: 'var(--panel-bg)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <form onSubmit={handleRegister} className="space-y-4">
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
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('confirmPassword', language)}
                </label>
                <div className="relative">
                  <Input
                    type={showConfirmPassword ? 'text' : 'password'}
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className="pr-10"
                    placeholder={t('confirmPasswordPlaceholder', language)}
                    required
                  />
                  <button
                    type="button"
                    aria-label={showConfirmPassword ? '隐藏密码' : '显示密码'}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => setShowConfirmPassword((v) => !v)}
                    className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center rounded bg-transparent p-0 m-0 border-0 outline-none focus:outline-none focus:ring-0 appearance-none cursor-pointer btn-icon"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {showConfirmPassword ? (
                      <EyeOff size={18} />
                    ) : (
                      <Eye size={18} />
                    )}
                  </button>
                </div>
              </div>

              {/* 密码规则清单（通过才允许提交） */}
              <div
                className="mt-1 text-xs"
                style={{ color: 'var(--text-secondary)' }}
              >
                <div
                  className="mb-1"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('passwordRequirements', language)}
                </div>
                <PasswordChecklist
                  rules={[
                    'minLength',
                    'capital',
                    'lowercase',
                    'number',
                    'specialChar',
                    'match',
                  ]}
                  minLength={8}
                  specialCharsRegex={/[@#$%!&*?]/}
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
                  className="space-y-1"
                  onChange={(isValid) => setPasswordValid(isValid)}
                />
              </div>

              {betaMode && (
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    内测码 *
                  </label>
                  <input
                    type="text"
                    value={betaCode}
                    onChange={(e) =>
                      setBetaCode(
                        e.target.value.replace(/[^a-z0-9]/gi, '').toLowerCase()
                      )
                    }
                    className="w-full px-3 py-2 rounded font-mono"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    placeholder="请输入6位内测码"
                    maxLength={6}
                    required={betaMode}
                  />
                  <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    内测码由6位字母数字组成，区分大小写
                  </p>
                </div>
              )}

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
                disabled={
                  loading || (betaMode && !betaCode.trim()) || !passwordValid
                }
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{
                  background: 'var(--brand-yellow)',
                  color: 'var(--brand-black)',
                }}
              >
                {loading
                  ? t('loading', language)
                  : t('registerButton', language)}
              </button>
            </form>
          </div>

          {/* Login Link */}
          <div className="text-center mt-6">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              已有账户？{' '}
              <button
                onClick={() => {
                  window.history.pushState({}, '', '/login')
                  window.dispatchEvent(new PopStateEvent('popstate'))
                }}
                className="font-semibold hover:underline transition-colors"
                style={{ color: 'var(--brand-yellow)' }}
              >
                立即登录
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

// 本地密码强度校验（与 UI 规则一致）
function isStrongPassword(pwd: string): boolean {
  if (!pwd || pwd.length < 8) return false
  const hasUpper = /[A-Z]/.test(pwd)
  const hasLower = /[a-z]/.test(pwd)
  const hasNumber = /\d/.test(pwd)
  const hasSpecial = /[@#$%!&*?]/.test(pwd)
  return hasUpper && hasLower && hasNumber && hasSpecial
}
