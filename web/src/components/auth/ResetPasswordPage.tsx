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
    <div className="min-h-screen" style={{ background: '#0B0E11' }}>
      <Header simple />

      <div
        className="flex items-center justify-center"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md">
          {/* Back to Login */}
          <button
            onClick={() => navigate('/login')}
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
          </div>

          {/* CLI recovery instructions */}
          <div
            className="rounded-lg p-6"
            style={{ background: '#1E2329', border: '1px solid #2B3139' }}
          >
            <p
              className="text-sm leading-relaxed mb-4"
              style={{ color: '#EAECEF' }}
            >
              {t('resetPasswordCliIntro', language)}
            </p>

            <div
              className="flex items-center justify-between gap-3 rounded px-3 py-3 font-mono text-xs"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <code
                className="break-all"
                style={{ color: '#F0B90B' }}
              >
                {RESET_PASSWORD_COMMAND}
              </code>
              <button
                type="button"
                onClick={handleCopy}
                className="shrink-0 btn-icon"
                style={{ color: '#848E9C' }}
                aria-label={t('copy', language)}
              >
                {copied ? (
                  <Check className="w-4 h-4" style={{ color: '#0ECB81' }} />
                ) : (
                  <Copy className="w-4 h-4" />
                )}
              </button>
            </div>

            <p
              className="text-xs leading-relaxed mt-4"
              style={{ color: '#848E9C' }}
            >
              {t('resetPasswordCliSecurityNote', language)}
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
