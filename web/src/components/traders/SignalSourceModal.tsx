import { useState } from 'react'
import { t, type Language } from '../../i18n/translations'

interface SignalSourceModalProps {
  coinPoolUrl: string
  oiTopUrl: string
  onSave: (coinPoolUrl: string, oiTopUrl: string) => void
  onClose: () => void
  language: Language
}

export function SignalSourceModal({
  coinPoolUrl,
  oiTopUrl,
  onSave,
  onClose,
  language,
}: SignalSourceModalProps) {
  const [coinPool, setCoinPool] = useState(coinPoolUrl || '')
  const [oiTop, setOiTop] = useState(oiTopUrl || '')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(coinPool.trim(), oiTop.trim())
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('signalSourceConfig', language)}
        </h3>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                COIN POOL URL
              </label>
              <input
                type="url"
                value={coinPool}
                onChange={(e) => setCoinPool(e.target.value)}
                placeholder="https://api.example.com/coinpool"
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {t('coinPoolDescription', language)}
              </div>
            </div>

            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                OI TOP URL
              </label>
              <input
                type="url"
                value={oiTop}
                onChange={(e) => setOiTop(e.target.value)}
                placeholder="https://api.example.com/oitop"
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {t('oiTopDescription', language)}
              </div>
            </div>

            <div
              className="p-4 rounded"
              style={{
                background: 'rgba(240, 185, 11, 0.1)',
                border: '1px solid rgba(240, 185, 11, 0.2)',
              }}
            >
              <div
                className="text-sm font-semibold mb-2"
                style={{ color: '#F0B90B' }}
              >
                ℹ️ {t('information', language)}
              </div>
              <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                <div>{t('signalSourceInfo1', language)}</div>
                <div>{t('signalSourceInfo2', language)}</div>
                <div>{t('signalSourceInfo3', language)}</div>
              </div>
            </div>
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
