import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Loader2, ShieldAlert, ShieldCheck } from 'lucide-react'
import { diagnoseWebCryptoEnvironment } from '../lib/crypto'
import { t, type Language } from '../i18n/translations'

export type WebCryptoCheckStatus =
  | 'idle'
  | 'checking'
  | 'secure'
  | 'insecure'
  | 'unsupported'

interface WebCryptoEnvironmentCheckProps {
  language: Language
  variant?: 'card' | 'compact'
  onStatusChange?: (status: WebCryptoCheckStatus) => void
}

export function WebCryptoEnvironmentCheck({
  language,
  variant = 'card',
  onStatusChange,
}: WebCryptoEnvironmentCheckProps) {
  const [status, setStatus] = useState<WebCryptoCheckStatus>('idle')
  const [summary, setSummary] = useState<string | null>(null)

  useEffect(() => {
    onStatusChange?.(status)
  }, [onStatusChange, status])

  const runCheck = useCallback(() => {
    setStatus('checking')
    setSummary(null)

    setTimeout(() => {
      const result = diagnoseWebCryptoEnvironment()
      setSummary(
        t('environmentCheck.summary', language, {
          origin: result.origin || 'N/A',
          protocol: result.protocol || 'unknown',
        })
      )

      if (!result.isBrowser || !result.hasSubtleCrypto) {
        setStatus('unsupported')
        return
      }

      if (!result.isSecureContext) {
        setStatus('insecure')
        return
      }

      setStatus('secure')
    }, 0)
  }, [language, t])

  useEffect(() => {
    runCheck()
  }, [runCheck])

  const isCompact = variant === 'compact'
  const containerClass = isCompact
    ? 'p-3 rounded border border-gray-700 bg-gray-900 space-y-3'
    : 'p-4 rounded border border-[#2B3139] bg-[#0B0E11] space-y-4'

  const descriptionColor = isCompact ? '#CBD5F5' : '#A1AEC8'
  const showInfo = status !== 'idle'

  const statusRendererMap: Record<WebCryptoCheckStatus, () => ReactNode> = {
    secure: () => (
      <div className="flex items-start gap-2 text-green-400 text-xs">
        <ShieldCheck className="w-4 h-4 flex-shrink-0" />
        <div>
          <div className="font-semibold">
            {t('environmentCheck.secureTitle', language)}
          </div>
          <div>{t('environmentCheck.secureDesc', language)}</div>
        </div>
      </div>
    ),
    insecure: () => (
      <div className="text-xs" style={{ color: '#F59E0B' }}>
        <div className="flex items-start gap-2 mb-1">
          <ShieldAlert className="w-4 h-4 flex-shrink-0" />
          <div className="font-semibold">
            {t('environmentCheck.insecureTitle', language)}
          </div>
        </div>
        <div>{t('environmentCheck.insecureDesc', language)}</div>
        <div className="mt-2 font-semibold">
          {t('environmentCheck.tipsTitle', language)}
        </div>
        <ul className="list-disc pl-5 space-y-1 mt-1">
          <li>{t('environmentCheck.tipHTTPS', language)}</li>
          <li>{t('environmentCheck.tipLocalhost', language)}</li>
          <li>{t('environmentCheck.tipIframe', language)}</li>
        </ul>
      </div>
    ),
    unsupported: () => (
      <div className="text-xs" style={{ color: '#F87171' }}>
        <div className="flex items-start gap-2 mb-1">
          <ShieldAlert className="w-4 h-4 flex-shrink-0" />
          <div className="font-semibold">
            {t('environmentCheck.unsupportedTitle', language)}
          </div>
        </div>
        <div>{t('environmentCheck.unsupportedDesc', language)}</div>
      </div>
    ),
    checking: () => (
      <div
        className="flex items-center gap-2 text-xs"
        style={{ color: '#EAECEF' }}
      >
        <Loader2 className="w-4 h-4 animate-spin" />
        <span>{t('environmentCheck.checking', language)}</span>
      </div>
    ),
    idle: () => null,
  }

  const renderStatus = () => statusRendererMap[status]()

  return (
    <div className={containerClass}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        {showInfo && (
          <div className="text-xs" style={{ color: descriptionColor }}>
            {summary ?? t('environmentCheck.description', language)}
          </div>
        )}
      </div>
      {showInfo && <div className="min-h-[1.5rem]">{renderStatus()}</div>}
    </div>
  )
}
