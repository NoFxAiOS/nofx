import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { Header } from '../common/Header'
import { ArrowLeft, KeyRound, Copy, Check } from 'lucide-react'
import { toast } from 'sonner'

const RESET_PASSWORD_COMMAND = 'nofx reset-password --email you@example.com'

export function ResetPasswordPage() {
  const { language } = useLanguage()
  const navigate = useNavigate()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(RESET_PASSWORD_COMMAND)
      setCopied(true)
      toast.success(t('copy', language))
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('copy', language))
    }
  }

  return (
    <div className="min-h-screen" style={{ background: '#F1ECE2' }}>
      <Header simple />

      <div
        className="flex items-center justify-center"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md">
          {/* Back to Login */}
          <button
            onClick={() => navigate('/login')}
            className="flex items-center gap-2 mb-6 text-sm hover:text-[#E0483B] transition-colors"
            style={{ color: '#8A8478' }}
          >
            <ArrowLeft className="w-4 h-4" />
            {t('backToLogin', language)}
          </button>

          {/* Logo */}
          <div className="text-center mb-8">
            <div
              className="w-16 h-16 mx-auto mb-4 flex items-center justify-center rounded-full"
              style={{ background: 'rgba(224, 72, 59, 0.1)' }}
            >
              <KeyRound className="w-8 h-8" style={{ color: '#E0483B' }} />
            </div>
            <h1 className="text-2xl font-bold" style={{ color: '#1A1813' }}>
              {t('resetPasswordTitle', language)}
            </h1>
          </div>

          {/* CLI recovery instructions */}
          <div
            className="rounded-lg p-6"
            style={{ background: '#F7F4EC', border: '1px solid rgba(26,24,19,0.14)' }}
          >
            <p
              className="text-sm leading-relaxed mb-4"
              style={{ color: '#1A1813' }}
            >
              {t('resetPasswordCliIntro', language)}
            </p>

            <div
              className="flex items-center justify-between gap-3 rounded px-3 py-3 font-mono text-xs"
              style={{ background: '#E8E2D5', border: '1px solid rgba(26,24,19,0.14)' }}
            >
              <code
                className="break-all"
                style={{ color: '#E0483B' }}
              >
                {RESET_PASSWORD_COMMAND}
              </code>
              <button
                type="button"
                onClick={handleCopy}
                className="shrink-0 btn-icon"
                style={{ color: '#8A8478' }}
                aria-label={t('copy', language)}
              >
                {copied ? (
                  <Check className="w-4 h-4" style={{ color: '#2E8B57' }} />
                ) : (
                  <Copy className="w-4 h-4" />
                )}
              </button>
            </div>

            <p
              className="text-xs leading-relaxed mt-4"
              style={{ color: '#8A8478' }}
            >
              {t('resetPasswordCliSecurityNote', language)}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
