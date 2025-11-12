import React, { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { Header } from './Header'
import { ArrowLeft, KeyRound, Eye, EyeOff } from 'lucide-react'
import PasswordChecklist from 'react-password-checklist'
import { Input } from './ui/input'
import { toast } from 'sonner'

export function ResetPasswordPage() {
  const { language } = useLanguage()
  const { resetPassword } = useAuth()
  const [email, setEmail] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [otpCode, setOtpCode] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess(false)

    // 验证两次密码是否一致
    if (newPassword !== confirmPassword) {
      setError(t('passwordMismatch', language))
      return
    }

    setLoading(true)

    const result = await resetPassword(email, newPassword, otpCode)

    if (result.success) {
      setSuccess(true)
      toast.success(t('resetPasswordSuccess', language) || '重置成功')
      // 3秒后跳转到登录页面
      setTimeout(() => {
        window.history.pushState({}, '', '/login')
        window.dispatchEvent(new PopStateEvent('popstate'))
      }, 3000)
    } else {
      const msg = result.message || t('resetPasswordFailed', language)
      setError(msg)
      toast.error(msg)
    }

    setLoading(false)
  }

  return (
    <div className="min-h-screen" style={{ background: '#0B0E11' }}>
      <Header simple />

      <div
        className="flex items-center justify-center"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md">
          {/* Back to Login */}
          <button
            onClick={() => {
              window.history.pushState({}, '', '/login')
              window.dispatchEvent(new PopStateEvent('popstate'))
            }}
            className="flex items-center gap-2 mb-6 text-sm hover:text-[#F0B90B] transition-colors"
            style={{ color: '#848E9C' }}
          >
            <ArrowLeft className="w-4 h-4" />
            {t('backToLogin', language)}
          </button>

          {/* Logo */}
          <div className="text-center mb-8">
            <div
              className="w-16 h-16 mx-auto mb-4 flex items-center justify-center rounded-full"
              style={{ background: 'rgba(240, 185, 11, 0.1)' }}
            >
              <KeyRound className="w-8 h-8" style={{ color: '#F0B90B' }} />
            </div>
            <h1 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
              {t('resetPasswordTitle', language)}
            </h1>
            <p className="text-sm mt-2" style={{ color: '#848E9C' }}>
              使用邮箱和 Google Authenticator 重置密码
            </p>
          </div>

          {/* Reset Password Form */}
          <div
            className="rounded-lg p-6"
            style={{ background: '#1E2329', border: '1px solid #2B3139' }}
          >
            {success ? (
              <div className="text-center py-8">
                <div className="text-5xl mb-4">✅</div>
                <p
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('resetPasswordSuccess', language)}
                </p>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  3秒后将自动跳转到登录页面...
                </p>
              </div>
            ) : (
              <form onSubmit={handleResetPassword} className="space-y-4">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
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
                    style={{ color: '#EAECEF' }}
                  >
                    {t('newPassword', language)}
                  </label>
                  <div className="relative">
                    <Input
                      type={showPassword ? 'text' : 'password'}
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="pr-10"
                      placeholder={t('newPasswordPlaceholder', language)}
                      required
                    />
                    <button
                      type="button"
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center btn-icon"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {showPassword ? (
                        <EyeOff className="w-5 h-5" />
                      ) : (
                        <Eye className="w-5 h-5" />
                      )}
                    </button>
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
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
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() =>
                        setShowConfirmPassword(!showConfirmPassword)
                      }
                      className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center btn-icon"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {showConfirmPassword ? (
                        <EyeOff className="w-5 h-5" />
                      ) : (
                        <Eye className="w-5 h-5" />
                      )}
                    </button>
                  </div>
                </div>

                {/* 密码强度检查（必须通过才允许提交） */}
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
                    value={newPassword}
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

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {t('otpCode', language)}
                  </label>
                  <div className="text-center mb-3">
                    <div className="text-3xl">📱</div>
                    <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      打开 Google Authenticator 获取6位验证码
                    </p>
                  </div>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
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
                      background: 'rgba(246, 70, 93, 0.1)',
                      color: '#F6465D',
                    }}
                  >
                    {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6 || !passwordValid}
                  className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                  style={{ background: '#F0B90B', color: '#000' }}
                >
                  {loading
                    ? t('loading', language)
                    : t('resetPasswordButton', language)}
                </button>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
