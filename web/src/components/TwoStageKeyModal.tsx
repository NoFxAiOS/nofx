import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { t, type Language } from '../i18n/translations'

const DEFAULT_LENGTH = 64

function generateObfuscation(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

function validatePrivateKeyFormat(value: string, expectedLength: number): boolean {
  const normalized = value.startsWith('0x') ? value.slice(2) : value
  if (normalized.length !== expectedLength) {
    return false
  }
  return /^[0-9a-fA-F]+$/.test(normalized)
}

export interface TwoStageKeyModalResult {
  value: string
  obfuscationLog: string[]
}

interface TwoStageKeyModalProps {
  isOpen: boolean
  language: Language
  onCancel: () => void
  onComplete: (result: TwoStageKeyModalResult) => void
  expectedLength?: number
  contextLabel?: string
}

export function TwoStageKeyModal({
  isOpen,
  language,
  onCancel,
  onComplete,
  expectedLength = DEFAULT_LENGTH,
  contextLabel,
}: TwoStageKeyModalProps) {
  const [stage, setStage] = useState<1 | 2>(1)
  const [part1, setPart1] = useState('')
  const [part2, setPart2] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [clipboardStatus, setClipboardStatus] = useState<'idle' | 'copied' | 'failed'>('idle')
  const [obfuscationLog, setObfuscationLog] = useState<string[]>([])
  const [processing, setProcessing] = useState(false)
  const [manualObfuscationValue, setManualObfuscationValue] = useState<string | null>(null)
  const stage1InputRef = useRef<HTMLInputElement | null>(null)
  const stage2InputRef = useRef<HTMLInputElement | null>(null)

  useEffect(() => {
    if (!isOpen) return
    const handler = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCancel()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onCancel])

  useEffect(() => {
    if (!isOpen) {
      setStage(1)
      setPart1('')
      setPart2('')
      setError(null)
      setClipboardStatus('idle')
      setObfuscationLog([])
      setProcessing(false)
      setManualObfuscationValue(null)
      return
    }

    const focusTimer = setTimeout(() => {
      if (stage === 1) {
        stage1InputRef.current?.focus()
      } else {
        stage2InputRef.current?.focus()
      }
    }, 10)

    return () => clearTimeout(focusTimer)
  }, [isOpen, stage])

  const heading = useMemo(() => {
    if (!contextLabel) {
      return t('twoStageModalTitle', language)
    }
    return `${t('twoStageModalTitle', language)} · ${contextLabel}`
  }, [contextLabel, language])

  if (!isOpen) {
    return null
  }

  const handleOverlayClick = () => {
    if (!processing) {
      onCancel()
    }
  }

  const handleStage1Next = async () => {
    if (!part1.trim()) {
      setError(t('twoStageStage1Error', language))
      return
    }
    setProcessing(true)
    const obfuscation = generateObfuscation()
    let copied = false
    try {
      await navigator.clipboard.writeText(obfuscation)
      copied = true
      setClipboardStatus('copied')
      setManualObfuscationValue(null)
    } catch (err) {
      console.warn('Clipboard write failed', err)
      setClipboardStatus('failed')
      setManualObfuscationValue(obfuscation)
    }
    setObfuscationLog((prev) => [...prev, `stage1:${new Date().toISOString()}`])
    setProcessing(false)
    setError(null)
    setStage(2)
    if (copied) {
      setManualObfuscationValue(null)
    }
  }

  const handleSubmit = () => {
    const cleanedPart1 = part1.trim()
    const cleanedPart2 = part2.trim()
    const combined = (cleanedPart1 + cleanedPart2).replace(/\s+/g, '')

    if (!validatePrivateKeyFormat(combined, expectedLength)) {
      setError(t('twoStageInvalidFormat', language, { length: expectedLength }))
      return
    }

    setObfuscationLog((prev) => [...prev, `stage2:${new Date().toISOString()}`])
    const result: TwoStageKeyModalResult = {
      value: combined,
      obfuscationLog: [...obfuscationLog, `stage2:${new Date().toISOString()}`],
    }
    onComplete(result)
  }

  const modalContent = (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4"
      onClick={handleOverlayClick}
    >
      <div
        className="w-full max-w-md rounded-xl border border-[#2B3139] bg-[#0B0E11] p-6 shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-4">
          <h2 className="text-lg font-semibold" style={{ color: '#EAECEF' }}>
            {heading}
          </h2>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('twoStageModalDescription', language, { length: expectedLength })}
          </p>
        </div>

        {stage === 1 ? (
          <div className="space-y-4">
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                {t('twoStageStage1Title', language)}
              </label>
              <input
                ref={stage1InputRef}
                type="password"
                value={part1}
                onChange={(event) => setPart1(event.target.value)}
                placeholder={t('twoStageStage1Placeholder', language)}
                className="w-full rounded border border-[#2B3139] bg-[#0F111C] px-3 py-2 text-sm text-[#EAECEF] outline-none focus:ring-2 focus:ring-[#F0B90B]/40"
                disabled={processing}
              />
              <p className="mt-2 text-xs" style={{ color: '#848E9C' }}>
                {t('twoStageStage1Hint', language)}
              </p>
            </div>

            {clipboardStatus === 'failed' && (
              <div
                className="rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs"
                style={{ color: '#F6465D' }}
              >
                <div>{t('twoStageClipboardManual', language)}</div>
                {manualObfuscationValue && (
                  <code className="mt-2 block select-all rounded bg-black/40 px-2 py-1 text-[11px] text-[#F0B90B]">
                    {manualObfuscationValue}
                  </code>
                )}
              </div>
            )}

            {error && (
              <div
                className="rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs"
                style={{ color: '#F6465D' }}
              >
                {error}
              </div>
            )}

            <div className="flex gap-2">
              <button
                type="button"
                onClick={onCancel}
                className="flex-1 rounded px-3 py-2 text-sm font-semibold transition-all hover:scale-[1.01]"
                style={{ background: '#1B1F2B', color: '#848E9C' }}
                disabled={processing}
              >
                {t('twoStageCancel', language)}
              </button>
              <button
                type="button"
                onClick={handleStage1Next}
                className="flex-1 rounded px-3 py-2 text-sm font-semibold transition-all hover:scale-[1.01]"
                style={{
                  background: processing ? '#3d2e0d' : '#F0B90B',
                  color: processing ? '#a18a43' : '#000',
                  opacity: part1.trim().length === 0 ? 0.7 : 1,
                }}
                disabled={processing || part1.trim().length === 0}
              >
                {processing ? t('twoStageProcessing', language) : t('twoStageNext', language)}
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                {t('twoStageStage2Title', language)}
              </label>
              <input
                ref={stage2InputRef}
                type="password"
                value={part2}
                onChange={(event) => setPart2(event.target.value)}
                placeholder={t('twoStageStage2Placeholder', language)}
                className="w-full rounded border border-[#2B3139] bg-[#0F111C] px-3 py-2 text-sm text-[#EAECEF] outline-none focus:ring-2 focus:ring-[#F0B90B]/40"
              />
              <p className="mt-2 text-xs" style={{ color: '#848E9C' }}>
                {t('twoStageStage2Hint', language)}
              </p>
            </div>

            {clipboardStatus === 'copied' && (
              <div
                className="rounded border border-[#F0B90B]/40 bg-[#F0B90B]/10 px-3 py-2 text-xs"
                style={{ color: '#F0B90B' }}
              >
                {t('twoStageClipboardSuccess', language)}
              </div>
            )}

            {clipboardStatus === 'failed' && manualObfuscationValue && (
              <div
                className="rounded border border-[#2B3139] bg-[#141821] px-3 py-2 text-xs"
                style={{ color: '#EAECEF' }}
              >
                {t('twoStageClipboardReminder', language)}
              </div>
            )}

            {error && (
              <div
                className="rounded border border-red-500/40 bg-red-500/10 px-3 py-2 text-xs"
                style={{ color: '#F6465D' }}
              >
                {error}
              </div>
            )}

            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => {
                  setStage(1)
                  setPart2('')
                  setError(null)
                  setClipboardStatus('idle')
                }}
                className="rounded px-3 py-2 text-sm font-semibold transition-all hover:scale-[1.01]"
                style={{ background: '#1B1F2B', color: '#848E9C' }}
              >
                {t('twoStageBack', language)}
              </button>
              <button
                type="button"
                onClick={handleSubmit}
                className="flex-1 rounded px-3 py-2 text-sm font-semibold transition-all hover:scale-[1.01]"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                {t('twoStageSubmit', language)}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )

  return createPortal(modalContent, document.body)
}
