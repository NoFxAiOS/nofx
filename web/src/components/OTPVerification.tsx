import React, { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { ArrowLeft, Shield } from 'lucide-react'

// Simple QR code display as text (for now, can be replaced with proper QR library)
const QRCodeDisplay = ({ value }: { value: string }) => {
  const extractSecret = (url: string) => {
    const match = url.match(/secret=([^&]+)/)
    return match ? decodeURIComponent(match[1]) : ''
  }

  const secret = extractSecret(value)

  return (
    <div className="p-6 bg-white rounded-lg text-black">
      <div className="text-center mb-4">
        <div className="w-32 h-32 mx-auto mb-4 border-2 border-black rounded flex items-center justify-center">
          <div className="text-xs text-center font-mono break-all p-2">
            {secret || 'QR Code'}
          </div>
        </div>
        <p className="text-sm font-semibold">Secret Key:</p>
        <p className="font-mono text-xs bg-gray-100 p-2 rounded mt-1 break-all">
          {secret}
        </p>
      </div>
    </div>
  )
}

interface OTPVerificationProps {
  mode: 'register' | 'login'
  userId: string
  qrCodeUrl?: string
  onBack: () => void
}

export function OTPVerification({ mode, userId, qrCodeUrl, onBack }: OTPVerificationProps) {
  const { language } = useLanguage()
  const { completeRegistration, verifyOTP } = useAuth()
  const [otpCode, setOtpCode] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      let result
      if (mode === 'register') {
        result = await completeRegistration(userId, otpCode)
      } else {
        result = await verifyOTP(userId, otpCode)
      }

      if (!result.success) {
        setError(result.message || t('otpVerificationFailed', language))
      }
      // Success handled automatically by AuthContext (redirects to /traders)
    } catch (err) {
      setError(t('otpVerificationFailed', language))
    }

    setLoading(false)
  }

  const title = mode === 'register' ? t('completeRegistration', language) : t('verifyIdentity', language)
  const description = mode === 'register'
    ? t('scanQRCodeCompleteRegistration', language)
    : t('enterAuthenticationCode', language)

  return (
    <div className="min-h-screen" style={{ background: 'var(--brand-black)' }}>
      <div className="max-w-md mx-auto pt-20 px-4">
        {/* Back Button */}
        <button
          onClick={onBack}
          className="mb-6 flex items-center gap-2 text-sm hover:opacity-80 transition-opacity"
          style={{ color: 'var(--text-secondary)' }}
        >
          <ArrowLeft size={16} />
          {t('back', language)}
        </button>

        {/* Header */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 flex items-center justify-center rounded-full"
               style={{ background: 'var(--brand-yellow)', color: 'var(--brand-black)' }}>
            <Shield size={32} />
          </div>
          <h1 className="text-2xl font-bold mb-2" style={{ color: 'var(--brand-light-gray)' }}>
            {title}
          </h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {description}
          </p>
        </div>

        {/* QR Code for Registration */}
        {mode === 'register' && qrCodeUrl && (
          <div className="mb-8">
            <div className="text-center mb-4">
              <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--brand-light-gray)' }}>
                {t('scanQRCode', language)}
              </h3>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {t('useGoogleAuthenticator', language)}
              </p>
            </div>
            <div className="flex justify-center mb-4">
              <QRCodeDisplay value={qrCodeUrl} />
            </div>
            <div className="text-center">
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {t('orEnterCodeManually', language)}
              </p>
              <button
                onClick={() => {
                  const match = qrCodeUrl.match(/secret=([^&]+)/)
                  const secret = match ? decodeURIComponent(match[1]) : ''
                  navigator.clipboard.writeText(secret)
                }}
                className="text-xs mt-2 px-3 py-1 rounded"
                style={{
                  background: 'var(--panel-bg)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--brand-light-gray)'
                }}
              >
                {t('copySecretKey', language)}
              </button>
            </div>
          </div>
        )}

        {/* OTP Input Form */}
        <div
          className="rounded-lg p-6"
          style={{
            background: 'var(--panel-bg)',
            border: '1px solid var(--panel-border)',
          }}
        >
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {t('authenticationCode', language)}
              </label>
              <input
                type="text"
                value={otpCode}
                onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                className="w-full px-4 py-3 rounded text-center text-2xl font-mono tracking-widest"
                style={{
                  background: 'var(--brand-black)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--brand-light-gray)',
                  letterSpacing: '0.2em',
                }}
                maxLength={6}
                pattern="[0-9]{6}"
                required
                autoFocus
              />
              <p className="text-xs mt-2 text-center" style={{ color: 'var(--text-secondary)' }}>
                {t('enter6DigitCode', language)}
              </p>
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
              disabled={loading || otpCode.length !== 6}
              className="w-full px-4 py-3 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'var(--brand-yellow)',
                color: 'var(--brand-black)',
              }}
            >
              {loading ? t('verifying', language) : t('verify', language)}
            </button>
          </form>
        </div>

        {/* Instructions */}
        <div className="mt-6 text-center">
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            {t('otpInstructions', language)}
          </p>
        </div>
      </div>
    </div>
  )
}