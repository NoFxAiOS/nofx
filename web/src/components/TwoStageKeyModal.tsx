import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { t, type Language } from '../i18n/translations'
import { toast } from 'sonner'
import { WebCryptoEnvironmentCheck } from './WebCryptoEnvironmentCheck'

const DEFAULT_LENGTH = 64

function generateObfuscation(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join(
    ''
  )
}

function validatePrivateKeyFormat(
  value: string,
  expectedLength: number
): boolean {
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
  const [clipboardStatus, setClipboardStatus] = useState<
    'idle' | 'copied' | 'failed'
  >('idle')
  const [obfuscationLog, setObfuscationLog] = useState<string[]>([])
  const [processing, setProcessing] = useState(false)
  const [manualObfuscationValue, setManualObfuscationValue] = useState<
    string | null
  >(null)

  const stage1Ref = useRef<HTMLInputElement>(null)
  const stage2Ref = useRef<HTMLInputElement>(null)

  // UX improvement: Use 58 + 6 split (most of the key + last 6 chars)
  // Advantage: Second stage only requires entering 6 characters, much easier to count
  const expectedPart1Length = expectedLength - 6  // 64 - 6 = 58
  const expectedPart2Length = 6  // Last 6 characters

  useEffect(() => {
    if (isOpen && stage === 1 && stage1Ref.current) {
      stage1Ref.current.focus()
    } else if (isOpen && stage === 2 && stage2Ref.current) {
      stage2Ref.current.focus()
    }
  }, [isOpen, stage])

  const handleStage1Next = async () => {
    // ✅ Normalize input (remove possible 0x prefix) before validating length
    const normalized1 = part1.startsWith('0x') ? part1.slice(2) : part1
    if (normalized1.length < expectedPart1Length) {
      setError(
        t('errors.privatekeyIncomplete', language, {
          expected: expectedPart1Length,
        })
      )
      return
    }

    setError(null)
    setProcessing(true)

    try {
      // 生成混淆字符串
      const obfuscation = generateObfuscation()
      setManualObfuscationValue(obfuscation)

      // 尝试复制到剪贴板
      if (navigator.clipboard) {
        try {
          await navigator.clipboard.writeText(obfuscation)
          setClipboardStatus('copied')
          setObfuscationLog([
            ...obfuscationLog,
            `Stage 1: ${new Date().toISOString()} - Auto copied obfuscation`,
          ])
          toast.success('已复制混淆字符串到剪贴板')
        } catch {
          setClipboardStatus('failed')
          setObfuscationLog([
            ...obfuscationLog,
            `Stage 1: ${new Date().toISOString()} - Auto copy failed, manual required`,
          ])
          toast.error('复制失败，请手动复制混淆字符串')
        }
      } else {
        setClipboardStatus('failed')
        setObfuscationLog([
          ...obfuscationLog,
          `Stage 1: ${new Date().toISOString()} - Clipboard API not available`,
        ])
        toast('当前浏览器不支持自动复制，请手动复制')
      }

      setTimeout(() => {
        setStage(2)
        setProcessing(false)
      }, 2000)
    } catch (err) {
      setError(t('errors.privatekeyObfuscationFailed', language))
      setProcessing(false)
    }
  }

  const handleStage2Complete = () => {
    // ✅ Normalize input (remove possible 0x prefix) before validating length
    const normalized2 = part2.startsWith('0x') ? part2.slice(2) : part2
    if (normalized2.length < expectedPart2Length) {
      setError(
        t('errors.privatekeyIncomplete', language, {
          expected: expectedPart2Length,
        })
      )
      return
    }

    // ✅ Concatenate after removing 0x prefix from both parts
    const normalized1 = part1.startsWith('0x') ? part1.slice(2) : part1
    const fullKey = normalized1 + normalized2
    if (!validatePrivateKeyFormat(fullKey, expectedLength)) {
      setError(t('errors.privatekeyInvalidFormat', language))
      return
    }

    const finalLog = [
      ...obfuscationLog,
      `Stage 2: ${new Date().toISOString()} - Completed`,
    ]
    onComplete({
      value: fullKey,
      obfuscationLog: finalLog,
    })
  }

  const handleReset = () => {
    setStage(1)
    setPart1('')
    setPart2('')
    setError(null)
    setClipboardStatus('idle')
    setObfuscationLog([])
    setProcessing(false)
    setManualObfuscationValue(null)
  }

  const modalContent = useMemo(() => {
    if (!isOpen) return null

    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80">
        <div className="bg-gray-900 p-8 rounded-xl max-w-lg w-full mx-4 border border-gray-700">
          <div className="text-center mb-6">
            <h2 className="text-xl font-bold text-white mb-2">
              🔐 {t('twoStageKey.title', language)}
              {contextLabel && (
                <span className="text-gray-300 text-base font-normal ml-2">
                  ({contextLabel})
                </span>
              )}
            </h2>
            <p className="text-gray-300 text-sm">
              {stage === 1
                ? t('twoStageKey.stage1Description', language, {
                    length: expectedPart1Length,
                  })
                : t('twoStageKey.stage2Description', language, {
                    length: expectedPart2Length,
                  })}
            </p>
          </div>

          <div className="mb-6">
            <WebCryptoEnvironmentCheck language={language} variant="compact" />
          </div>

          {/* Stage 1 */}
          {stage === 1 && (
            <div className="space-y-4">
              <div>
                <label className="block text-gray-300 text-sm mb-2">
                  {t('twoStageKey.stage1InputLabel', language)} (
                  {expectedPart1Length} {t('twoStageKey.characters', language)})
                </label>
                <input
                  ref={stage1Ref}
                  type="password"
                  value={part1}
                  onChange={(e) => setPart1(e.target.value)}
                  placeholder="0x1234..."
                  className="w-full bg-gray-800 border border-gray-600 rounded-lg px-4 py-3 text-white font-mono text-sm focus:border-blue-500 focus:outline-none"
                  maxLength={expectedPart1Length + 2} // +2 for optional 0x prefix
                  disabled={processing}
                />
              </div>

              {error && <div className="text-red-400 text-sm">{error}</div>}

              <div className="flex gap-3">
                <button
                  onClick={handleStage1Next}
                  disabled={
                    (part1.startsWith('0x') ? part1.slice(2) : part1).length <
                      expectedPart1Length || processing
                  }
                  className="flex-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 text-white font-medium py-3 px-4 rounded-lg transition-colors"
                >
                  {processing
                    ? t('twoStageKey.processing', language)
                    : t('twoStageKey.nextButton', language)}
                </button>
                <button
                  onClick={onCancel}
                  disabled={processing}
                  className="px-6 py-3 text-gray-300 hover:text-white border border-gray-600 rounded-lg transition-colors"
                >
                  {t('twoStageKey.cancelButton', language)}
                </button>
              </div>
            </div>
          )}

          {/* Transition Message */}
          {stage === 2 && clipboardStatus !== 'idle' && (
            <div className="mb-4 p-4 rounded-lg bg-blue-900/50 border border-blue-600">
              {clipboardStatus === 'copied' && (
                <div className="text-blue-300">
                  <div className="font-medium">
                    {t('twoStageKey.obfuscationCopied', language)}
                  </div>
                  <div className="text-sm mt-1">
                    {t('twoStageKey.obfuscationInstruction', language)}
                  </div>
                </div>
              )}
              {clipboardStatus === 'failed' && manualObfuscationValue && (
                <div className="text-yellow-300">
                  <div className="font-medium">
                    {t('twoStageKey.obfuscationManual', language)}
                  </div>
                  <div className="text-xs mt-2 p-2 bg-gray-800 rounded font-mono break-all border">
                    {manualObfuscationValue}
                  </div>
                  <div className="text-sm mt-1">
                    {t('twoStageKey.obfuscationInstruction', language)}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Stage 2 */}
          {stage === 2 && (
            <div className="space-y-4">
              <div>
                <label className="block text-gray-300 text-sm mb-2">
                  {t('twoStageKey.stage2InputLabel', language)} (
                  {expectedPart2Length} {t('twoStageKey.characters', language)})
                </label>
                <input
                  ref={stage2Ref}
                  type="password"
                  value={part2}
                  onChange={(e) => setPart2(e.target.value)}
                  placeholder="...5678"
                  className="w-full bg-gray-800 border border-gray-600 rounded-lg px-4 py-3 text-white font-mono text-sm focus:border-blue-500 focus:outline-none"
                  maxLength={expectedPart2Length + 2}
                />
              </div>

              {error && <div className="text-red-400 text-sm">{error}</div>}

              <div className="flex gap-3">
                <button
                  onClick={handleStage2Complete}
                  disabled={
                    (part2.startsWith('0x') ? part2.slice(2) : part2).length <
                    expectedPart2Length
                  }
                  className="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 text-white font-medium py-3 px-4 rounded-lg transition-colors"
                >
                  🔒 {t('twoStageKey.encryptButton', language)}
                </button>
                <button
                  onClick={handleReset}
                  className="px-6 py-3 text-gray-300 hover:text-white border border-gray-600 rounded-lg transition-colors"
                >
                  {t('twoStageKey.backButton', language)}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    )
  }, [
    isOpen,
    stage,
    part1,
    part2,
    error,
    processing,
    clipboardStatus,
    manualObfuscationValue,
    language,
    expectedPart1Length,
    expectedPart2Length,
    contextLabel,
    obfuscationLog,
    onCancel,
    onComplete,
  ])

  if (!isOpen) return null

  return createPortal(modalContent, document.body)
}
