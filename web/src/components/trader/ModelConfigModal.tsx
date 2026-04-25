import React, { useState, useEffect } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { Trash2, Brain, ExternalLink } from 'lucide-react'
import type { AIModel } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { api } from '../../lib/api'
import { getBeginnerWalletAddress } from '../../lib/onboarding'
import { getModelIcon } from '../common/ModelIcons'
import { ModelStepIndicator } from './ModelStepIndicator'
import { ModelCard } from './ModelCard'
import {
  CLAW402_MODELS,
  AI_PROVIDER_CONFIG,
  DEFAULT_CLAW402_MODEL,
  getShortName,
} from './model-constants'

interface ModelConfigModalProps {
  allModels: AIModel[]
  configuredModels: AIModel[]
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string
  ) => void
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}

export function ModelConfigModal({
  allModels,
  configuredModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: ModelConfigModalProps) {
  const [currentStep, setCurrentStep] = useState(editingModelId ? 1 : 0)
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')

  // Always prefer allModels (supportedModels) for provider/id lookup;
  // fall back to configuredModels for edit mode details (apiKey etc.)
  const selectedModel =
    allModels?.find((m) => m.id === selectedModelId) ||
    configuredModels?.find((m) => m.id === selectedModelId)
  const configuredModel =
    configuredModels?.find((m) => m.id === selectedModelId) || null

  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '')
      setBaseUrl(selectedModel.customApiUrl || '')
      setModelName(selectedModel.customModelName || '')
    }
  }, [editingModelId, selectedModel])

  const handleSelectModel = (modelId: string) => {
    setSelectedModelId(modelId)
    setCurrentStep(1)
  }

  const handleBack = () => {
    if (editingModelId) {
      onClose()
    } else {
      setCurrentStep(0)
      setSelectedModelId('')
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId) return
    const key = apiKey.trim()
    // Allow empty key when editing an existing model (backend preserves existing key)
    if (!key && !editingModelId) return
    const trimmedModelName = modelName.trim()
    const resolvedModelName =
      selectedModel?.provider === 'claw402'
        ? trimmedModelName || DEFAULT_CLAW402_MODEL
        : trimmedModelName || undefined
    onSave(selectedModelId, key, baseUrl.trim() || undefined, resolvedModelName)
  }

  const availableModels = allModels || []
  const configuredIds = new Set(configuredModels?.map((m) => m.id) || [])
  const isClaw402Selected =
    selectedModel?.provider === 'claw402' || selectedModel?.id === 'claw402'
  const stepLabels = [
    t('modelConfig.selectModel', language),
    t(
      !selectedModel
        ? 'modelConfig.configure'
        : isClaw402Selected
          ? 'modelConfig.configureWallet'
          : 'modelConfig.configure',
      language
    ),
  ]

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4 overflow-y-auto backdrop-blur-sm">
      <div
        className="rounded-2xl w-full max-w-[52rem] relative my-8 shadow-2xl"
        style={{
          background: 'linear-gradient(180deg, #1E2329 0%, #181A20 100%)',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {currentStep > 0 && !editingModelId && (
              <button
                type="button"
                onClick={handleBack}
                className="p-2 rounded-lg hover:bg-white/10 transition-colors"
              >
                <svg
                  className="w-5 h-5"
                  style={{ color: '#848E9C' }}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M15 19l-7-7 7-7"
                  />
                </svg>
              </button>
            )}
            <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
              {editingModelId
                ? t('editAIModel', language)
                : t('addAIModel', language)}
            </h3>
          </div>
          <div className="flex items-center gap-2">
            {editingModelId && (
              <button
                type="button"
                onClick={() => onDelete(editingModelId)}
                className="p-2 rounded-lg hover:bg-red-500/20 transition-colors"
                style={{ color: '#F6465D' }}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              className="p-2 rounded-lg hover:bg-white/10 transition-colors"
              style={{ color: '#848E9C' }}
            >
              ✕
            </button>
          </div>
        </div>

        {/* Step Indicator */}
        {!editingModelId && (
          <div className="px-6">
            <ModelStepIndicator currentStep={currentStep} labels={stepLabels} />
          </div>
        )}

        {/* Content */}
        <div
          className="px-6 pb-6 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Step 0: Select Model */}
          {currentStep === 0 && !editingModelId && (
            <ModelSelectionStep
              availableModels={availableModels}
              configuredIds={configuredIds}
              selectedModelId={selectedModelId}
              onSelectModel={handleSelectModel}
              language={language}
            />
          )}

          {/* Step 1: Configure — Claw402 Dedicated UI */}
          {(currentStep === 1 || editingModelId) &&
            selectedModel &&
            (selectedModel.provider === 'claw402' ||
              selectedModel.id === 'claw402') && (
              <Claw402ConfigForm
                apiKey={apiKey}
                modelName={modelName}
                configuredModel={configuredModel}
                editingModelId={editingModelId}
                onApiKeyChange={setApiKey}
                onModelNameChange={setModelName}
                onBack={handleBack}
                onSubmit={handleSubmit}
                language={language}
              />
            )}

          {/* Step 1: Configure — Standard Providers (non-claw402) */}
          {(currentStep === 1 || editingModelId) &&
            selectedModel &&
            selectedModel.provider !== 'claw402' &&
            selectedModel.id !== 'claw402' && (
              <StandardProviderConfigForm
                selectedModel={selectedModel}
                apiKey={apiKey}
                baseUrl={baseUrl}
                modelName={modelName}
                editingModelId={editingModelId}
                onApiKeyChange={setApiKey}
                onBaseUrlChange={setBaseUrl}
                onModelNameChange={setModelName}
                onBack={handleBack}
                onSubmit={handleSubmit}
                language={language}
              />
            )}
        </div>
      </div>
    </div>
  )
}

// --- Sub-components for ModelConfigModal ---

function ModelSelectionStep({
  availableModels,
  configuredIds,
  selectedModelId,
  onSelectModel,
  language,
}: {
  availableModels: AIModel[]
  configuredIds: Set<string>
  selectedModelId: string
  onSelectModel: (modelId: string) => void
  language: Language
}) {
  const [showOtherProviders, setShowOtherProviders] = useState(false)
  const claw402Model =
    availableModels.find((model) => model.provider === 'claw402') || null
  const otherProviders = availableModels.filter(
    (model) =>
      model.provider !== 'claw402' && !model.provider?.startsWith('blockrun')
  )

  return (
    <div className="space-y-4">
      <div className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {t('modelConfig.chooseProvider', language)}
      </div>

      {/* Claw402 Featured Card */}
      {claw402Model && (
        <button
          type="button"
          onClick={() => {
            onSelectModel(claw402Model.id)
          }}
          className="w-full p-5 rounded-xl text-left transition-all hover:scale-[1.01]"
          style={{
            background:
              'linear-gradient(135deg, rgba(37, 99, 235, 0.15) 0%, rgba(139, 92, 246, 0.15) 100%)',
            border: '1.5px solid rgba(37, 99, 235, 0.4)',
          }}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl flex items-center justify-center overflow-hidden">
                <img
                  src="/icons/claw402.png"
                  alt="Claw402"
                  width={40}
                  height={40}
                />
              </div>
              <div>
                <div
                  className="font-bold text-base"
                  style={{ color: '#EAECEF' }}
                >
                  Claw402
                  <a
                    href="https://claw402.ai"
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    className="ml-1.5 text-[10px] font-normal px-1.5 py-0.5 rounded"
                    style={{
                      color: '#60A5FA',
                      background: 'rgba(96, 165, 250, 0.1)',
                    }}
                  >
                    ↗ claw402.ai
                  </a>
                </div>
                <div className="text-xs mt-0.5" style={{ color: '#A0AEC0' }}>
                  {t('modelConfig.payPerCall', language)}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {configuredIds.has(claw402Model.id) && (
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ background: '#00E096' }}
                />
              )}
              <div
                className="px-3 py-1.5 rounded-full text-xs font-bold"
                style={{
                  background: 'linear-gradient(135deg, #2563EB, #7C3AED)',
                  color: '#fff',
                }}
              >
                {'🔥 ' + t('modelConfig.recommended', language)}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-3 ml-[52px]">
            <span
              className="text-[11px] px-2 py-0.5 rounded-full"
              style={{
                background: 'rgba(0, 224, 150, 0.1)',
                color: '#00E096',
                border: '1px solid rgba(0, 224, 150, 0.2)',
              }}
            >
              GPT · Claude · DeepSeek · Gemini · Grok · Qwen · Kimi
            </span>
          </div>
          <div
            className="mt-4 ml-[52px] text-[11px]"
            style={{ color: '#A0AEC0' }}
          >
            {t('modelConfig.claw402EntryDesc', language)}
          </div>
        </button>
      )}

      {otherProviders.length > 0 && (
        <div className="rounded-xl border border-white/10 bg-black/20 overflow-hidden">
          <button
            type="button"
            onClick={() => setShowOtherProviders((prev) => !prev)}
            className="w-full flex items-center justify-between px-4 py-4 text-left transition-all hover:bg-white/5"
          >
            <div>
              <div
                className="text-sm font-semibold"
                style={{ color: '#EAECEF' }}
              >
                {t('modelConfig.otherApiEntry', language)}
              </div>
              <div className="mt-1 text-xs" style={{ color: '#848E9C' }}>
                {t('modelConfig.otherApiEntryDesc', language)}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <span
                className="rounded-full border border-white/10 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.2em]"
                style={{ color: '#A0AEC0' }}
              >
                {otherProviders.length} API
              </span>
              <span className="text-sm" style={{ color: '#60A5FA' }}>
                {showOtherProviders ? '−' : '+'}
              </span>
            </div>
          </button>

          {showOtherProviders && (
            <div className="border-t border-white/5 px-4 py-4">
              <div className="grid grid-cols-3 sm:grid-cols-4 gap-3">
                {otherProviders.map((model) => (
                  <ModelCard
                    key={model.id}
                    model={model}
                    selected={selectedModelId === model.id}
                    onClick={() => onSelectModel(model.id)}
                    configured={configuredIds.has(model.id)}
                  />
                ))}
              </div>
              <div
                className="text-xs text-center pt-3"
                style={{ color: '#848E9C' }}
              >
                {t('modelConfig.modelsConfigured', language)}
              </div>
            </div>
          )}
        </div>
      )}
      <div className="text-xs text-center pt-2" style={{ color: '#848E9C' }}>
        {t('modelConfig.modelsConfigured', language)}
      </div>
    </div>
  )
}

function Claw402ConfigForm({
  apiKey,
  modelName,
  configuredModel,
  editingModelId,
  onApiKeyChange,
  onModelNameChange,
  onBack,
  onSubmit,
  language,
}: {
  apiKey: string
  modelName: string
  configuredModel: AIModel | null
  editingModelId: string | null
  onApiKeyChange: (value: string) => void
  onModelNameChange: (value: string) => void
  onBack: () => void
  onSubmit: (e: React.FormEvent) => void
  language: Language
}) {
  const [walletAddress, setWalletAddress] = useState('')
  const [copiedAddr, setCopiedAddr] = useState(false)
  const [showDeposit, setShowDeposit] = useState(false)
  const [showNewWalletBackup, setShowNewWalletBackup] = useState(false)
  const [newWalletKey, setNewWalletKey] = useState('')
  const [usdcBalance, setUsdcBalance] = useState<string | null>(null)
  const [keyError, setKeyError] = useState('')
  const [validating, setValidating] = useState(false)
  const [claw402Status, setClaw402Status] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<{
    status: string
    message: string
  } | null>(null)
  const [testing, setTesting] = useState(false)
  const [serverWalletAddress, setServerWalletAddress] = useState('')
  const [serverWalletBalance, setServerWalletBalance] = useState<string | null>(
    null
  )
  const localWalletAddress = getBeginnerWalletAddress()?.trim() || ''
  const configuredWalletAddress =
    configuredModel?.walletAddress?.trim() ||
    localWalletAddress ||
    serverWalletAddress
  const resolvedWalletAddress = walletAddress || configuredWalletAddress
  const resolvedUsdcBalance =
    usdcBalance ?? configuredModel?.balanceUsdc ?? serverWalletBalance ?? null
  const hasExistingWallet = Boolean(configuredWalletAddress)

  // Client-side validation helper
  const getClientError = (key: string): string => {
    if (!key) return ''
    if (!key.startsWith('0x'))
      return t('modelConfig.invalidKeyPrefix', language)
    if (key.length !== 66)
      return `${t('modelConfig.invalidKeyLength', language)} ${key.length}`
    if (!/^0x[0-9a-fA-F]{64}$/.test(key))
      return t('modelConfig.invalidKeyChars', language)
    return ''
  }

  const isKeyValid =
    apiKey.length === 66 &&
    apiKey.startsWith('0x') &&
    /^0x[0-9a-fA-F]{64}$/.test(apiKey)

  useEffect(() => {
    if (hasExistingWallet) {
      setShowDeposit(true)
    }
  }, [hasExistingWallet])

  useEffect(() => {
    if (
      configuredModel?.walletAddress ||
      localWalletAddress ||
      serverWalletAddress
    ) {
      return
    }

    let cancelled = false
    void api
      .getCurrentBeginnerWallet()
      .then((result) => {
        setClaw402Status(result.claw402_status || 'unknown')
        if (cancelled || !result.found || !result.address) {
          return
        }
        setServerWalletAddress(result.address)
        setServerWalletBalance(result.balance_usdc || null)
      })
      .catch(() => {
        // Ignore silently: this is a best-effort fallback for showing the current wallet.
      })
    return () => {
      cancelled = true
    }
  }, [configuredModel?.walletAddress, localWalletAddress, serverWalletAddress])

  // Debounced validation when apiKey changes
  useEffect(() => {
    setWalletAddress('')
    setUsdcBalance(null)
    setClaw402Status(null)
    setTestResult(null)

    const clientErr = getClientError(apiKey)
    setKeyError(clientErr)

    if (clientErr || !apiKey) {
      setValidating(false)
      return
    }

    setValidating(true)
    const timer = setTimeout(async () => {
      try {
        const res = await fetch('/api/wallet/validate', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ private_key: apiKey }),
        })
        const data = await res.json()
        if (data.valid) {
          setWalletAddress(data.address || '')
          setUsdcBalance(data.balance_usdc || '0.00')
          setClaw402Status(data.claw402_status || 'unknown')
          setKeyError('')
        } else {
          setKeyError(data.error || 'Invalid key')
        }
      } catch {
        setKeyError('Validation request failed')
      } finally {
        setValidating(false)
      }
    }, 500)

    return () => clearTimeout(timer)
  }, [apiKey])

  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      if (!apiKey && hasExistingWallet) {
        const result = await api.getCurrentBeginnerWallet()
        setClaw402Status(result.claw402_status || 'unknown')
        if (result.found && result.address) {
          setWalletAddress(result.address)
          setUsdcBalance(result.balance_usdc || '0.00')
          setShowDeposit(true)
        }
        setTestResult({
          status: result.claw402_status === 'ok' ? 'ok' : 'error',
          message:
            result.claw402_status === 'ok'
              ? t('modelConfig.claw402Connected', language)
              : t('modelConfig.claw402Unreachable', language),
        })
        return
      }

      const res = await fetch('/api/wallet/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ private_key: apiKey }),
      })
      const data = await res.json()
      if (data.valid) {
        setWalletAddress(data.address || '')
        setUsdcBalance(data.balance_usdc || '0.00')
        setClaw402Status(data.claw402_status || 'unknown')
        if (parseFloat(data.balance_usdc || '0') === 0) setShowDeposit(true)
        setTestResult({
          status: data.claw402_status === 'ok' ? 'ok' : 'error',
          message:
            data.claw402_status === 'ok'
              ? t('modelConfig.claw402Connected', language)
              : t('modelConfig.claw402Unreachable', language),
        })
      } else {
        setTestResult({ status: 'error', message: data.error || 'Invalid key' })
      }
    } catch {
      setTestResult({
        status: 'error',
        message: t('modelConfig.claw402Unreachable', language),
      })
    } finally {
      setTesting(false)
    }
  }

  const balanceNum = usdcBalance ? parseFloat(usdcBalance) : 0

  return (
    <form onSubmit={onSubmit} className="space-y-5">
      {/* Claw402 Hero Header */}
      <div
        className="p-5 rounded-xl text-center"
        style={{
          background:
            'linear-gradient(135deg, rgba(37, 99, 235, 0.12) 0%, rgba(139, 92, 246, 0.12) 100%)',
          border: '1px solid rgba(37, 99, 235, 0.3)',
        }}
      >
        <div className="w-14 h-14 mx-auto rounded-2xl flex items-center justify-center mb-3 overflow-hidden">
          <img src="/icons/claw402.png" alt="Claw402" width={56} height={56} />
        </div>
        <a
          href="https://claw402.ai"
          target="_blank"
          rel="noopener noreferrer"
          className="text-lg font-bold inline-flex items-center gap-1.5 hover:underline"
          style={{ color: '#EAECEF' }}
        >
          Claw402{' '}
          <span className="text-xs font-normal" style={{ color: '#60A5FA' }}>
            ↗
          </span>
        </a>
        <div className="text-sm mt-1" style={{ color: '#A0AEC0' }}>
          {t('modelConfig.allModelsClaw', language)}
        </div>
        <div className="flex items-center justify-center gap-3 mt-3 flex-wrap">
          {['GPT', 'Claude', 'DeepSeek', 'Gemini', 'Grok', 'Qwen', 'Kimi'].map(
            (name) => (
              <span
                key={name}
                className="text-[11px] px-2 py-0.5 rounded-full"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  color: '#A0AEC0',
                }}
              >
                {name}
              </span>
            )
          )}
        </div>
        <div className="mt-4 flex items-center justify-center gap-3 flex-wrap">
          <button
            type="button"
            onClick={handleTestConnection}
            disabled={testing || (!hasExistingWallet && !isKeyValid)}
            className="inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition-all hover:scale-[1.02] disabled:cursor-not-allowed disabled:opacity-50"
            style={{
              background: 'rgba(37, 99, 235, 0.15)',
              border: '1px solid rgba(37, 99, 235, 0.3)',
              color: '#60A5FA',
            }}
          >
            <span>🔗</span>
            {testing
              ? t('modelConfig.testingConnection', language)
              : t('modelConfig.testConnection', language)}
          </button>
          {claw402Status ? (
            <div
              className="text-xs"
              style={{ color: claw402Status === 'ok' ? '#00E096' : '#F59E0B' }}
            >
              {claw402Status === 'ok'
                ? t('modelConfig.claw402Connected', language)
                : t('modelConfig.claw402Unreachable', language)}
            </div>
          ) : null}
        </div>
      </div>

      {/* Step 1: Select AI Model */}
      <div className="space-y-3">
        <label
          className="flex items-center gap-2 text-sm font-semibold"
          style={{ color: '#EAECEF' }}
        >
          <Brain className="w-4 h-4" style={{ color: '#2563EB' }} />
          {t('modelConfig.selectAiModel', language)}
        </label>
        <div className="text-xs mb-2" style={{ color: '#848E9C' }}>
          {t('modelConfig.allModelsUnified', language)}
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
          {CLAW402_MODELS.map((m) => {
            const isSelected = (modelName || DEFAULT_CLAW402_MODEL) === m.id
            return (
              <button
                key={m.id}
                type="button"
                onClick={() => onModelNameChange(m.id)}
                className="flex items-start gap-2 px-3 py-2.5 rounded-xl text-left transition-all hover:scale-[1.02]"
                style={{
                  background: isSelected ? 'rgba(37, 99, 235, 0.2)' : '#0B0E11',
                  border: isSelected
                    ? '1.5px solid #2563EB'
                    : '1px solid #2B3139',
                }}
              >
                <span className="text-base mt-0.5">{m.icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <div
                      className="text-xs font-semibold truncate"
                      style={{ color: isSelected ? '#60A5FA' : '#EAECEF' }}
                    >
                      {m.name}
                    </div>
                    {m.isNew ? (
                      <span
                        className="shrink-0 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em]"
                        style={{
                          color: '#00E096',
                          background: 'rgba(0, 224, 150, 0.12)',
                          border: '1px solid rgba(0, 224, 150, 0.22)',
                        }}
                      >
                        NEW
                      </span>
                    ) : null}
                  </div>
                  <div
                    className="text-[10px] truncate"
                    style={{ color: '#848E9C' }}
                  >
                    {m.provider} · {m.desc}
                  </div>
                  <div className="text-[10px]" style={{ color: '#00E096' }}>
                    ~${m.price}/call
                  </div>
                </div>
                {isSelected && (
                  <span
                    className="text-[10px] mt-1"
                    style={{ color: '#60A5FA' }}
                  >
                    ✓
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* Step 2: Wallet Setup */}
      <div className="space-y-3">
        <label
          className="flex items-center gap-2 text-sm font-semibold"
          style={{ color: '#EAECEF' }}
        >
          <svg
            className="w-4 h-4"
            style={{ color: '#2563EB' }}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"
            />
          </svg>
          {t('modelConfig.setupWallet', language)}
        </label>

        <div
          className="p-3 rounded-xl"
          style={{
            background: 'rgba(37, 99, 235, 0.06)',
            border: '1px solid rgba(37, 99, 235, 0.15)',
          }}
        >
          <div className="text-xs mb-2" style={{ color: '#A0AEC0' }}>
            {t('modelConfig.walletInfo', language)}
          </div>
          <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
            <div className="flex items-center gap-1.5">
              <span style={{ color: '#00E096' }}>•</span>
              {t('modelConfig.exportKey', language)}
            </div>
            <div className="flex items-center gap-1.5">
              <span style={{ color: '#00E096' }}>•</span>
              {t('modelConfig.dedicatedWallet', language)}
            </div>
          </div>
        </div>

        {hasExistingWallet && (
          <div
            className="p-3 rounded-xl"
            style={{
              background: 'rgba(0, 224, 150, 0.05)',
              border: '1px solid rgba(0, 224, 150, 0.18)',
            }}
          >
            <div
              className="text-xs font-semibold mb-1.5"
              style={{ color: '#00E096' }}
            >
              {language === 'zh'
                ? '已自动提取当前钱包'
                : 'Current wallet loaded automatically'}
            </div>
            <div className="text-[11px] leading-5" style={{ color: '#A0AEC0' }}>
              {language === 'zh'
                ? '你现在可以直接查看当前钱包地址、余额和充值二维码。只有在想更换钱包时，才需要重新输入新的私钥。'
                : 'You can view the current wallet address, balance, and deposit QR code right away. Only enter a new private key if you want to replace this wallet.'}
            </div>
            {!configuredModel?.walletAddress && localWalletAddress ? (
              <div className="mt-2 text-[10px]" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '当前地址来自本地已保存的新手钱包。'
                  : 'This address comes from the locally saved beginner wallet.'}
              </div>
            ) : null}
            {!configuredModel?.walletAddress &&
            !localWalletAddress &&
            serverWalletAddress ? (
              <div className="mt-2 text-[10px]" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '当前地址来自后端保存的钱包配置。'
                  : 'This address comes from the wallet saved on the server.'}
              </div>
            ) : null}
          </div>
        )}

        <div className="space-y-1.5">
          <div className="text-xs font-medium" style={{ color: '#A0AEC0' }}>
            {t('modelConfig.walletPrivateKey', language)}
          </div>
          <div className="flex gap-2">
            <input
              type="password"
              value={apiKey}
              onChange={(e) => onApiKeyChange(e.target.value)}
              placeholder={
                hasExistingWallet
                  ? language === 'zh'
                    ? '已保存，如需更换钱包请重新输入私钥'
                    : 'Already saved. Enter a new private key only to replace the wallet.'
                  : '0x...'
              }
              className="flex-1 px-4 py-3 rounded-xl font-mono text-sm"
              style={{
                background: '#0B0E11',
                border: keyError
                  ? '1px solid #EF4444'
                  : walletAddress
                    ? '1px solid #00E096'
                    : '1px solid #2B3139',
                color: '#EAECEF',
              }}
              required={!hasExistingWallet}
            />
            {!apiKey && (
              <button
                type="button"
                onClick={async () => {
                  try {
                    const res = await fetch('/api/wallet/generate', {
                      method: 'POST',
                    })
                    const data = await res.json()
                    if (data.private_key) {
                      onApiKeyChange(data.private_key)
                      setShowNewWalletBackup(true)
                      setNewWalletKey(data.private_key)
                    }
                  } catch {
                    /* ignore */
                  }
                }}
                className="shrink-0 px-3 py-3 rounded-xl text-xs font-semibold transition-all hover:scale-[1.02]"
                style={{
                  background: 'linear-gradient(135deg, #2563EB, #7C3AED)',
                  color: '#fff',
                  border: 'none',
                  cursor: 'pointer',
                }}
              >
                {language === 'zh' ? '🔑 创建钱包' : '🔑 Create Wallet'}
              </button>
            )}
          </div>

          {/* New wallet backup warning */}
          {showNewWalletBackup && newWalletKey && (
            <div
              className="p-3 rounded-xl"
              style={{
                background: 'rgba(239, 68, 68, 0.08)',
                border: '1px solid rgba(239, 68, 68, 0.3)',
              }}
            >
              <div
                className="text-xs font-bold mb-2"
                style={{ color: '#EF4444' }}
              >
                🚨{' '}
                {language === 'zh'
                  ? '重要：请立即备份私钥！'
                  : 'Important: Backup your private key NOW!'}
              </div>
              <div className="text-[11px] mb-2" style={{ color: '#F87171' }}>
                {language === 'zh'
                  ? '这是你的钱包私钥，丢失后无法恢复，钱包里的资产将永久丢失。请复制并安全保存。'
                  : 'This is your wallet private key. If lost, it cannot be recovered and all assets will be permanently lost. Copy and save it securely.'}
              </div>
              <div className="flex items-center gap-2 mb-2">
                <code
                  className="text-[10px] font-mono break-all select-all flex-1 p-2 rounded"
                  style={{ background: '#0B0E11', color: '#F87171' }}
                >
                  {newWalletKey}
                </code>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(newWalletKey)
                    setCopiedAddr(true)
                    setTimeout(() => setCopiedAddr(false), 2000)
                  }}
                  className="shrink-0 text-[10px] px-2 py-1 rounded"
                  style={{
                    background: 'rgba(239,68,68,0.15)',
                    color: '#F87171',
                    border: 'none',
                    cursor: 'pointer',
                  }}
                >
                  {copiedAddr ? '✅ Copied' : '📋 Copy Key'}
                </button>
              </div>
              <div
                className="text-[10px] space-y-1"
                style={{ color: '#848E9C' }}
              >
                <div>
                  ✅{' '}
                  {language === 'zh'
                    ? '建议保存到密码管理器（1Password / Bitwarden）'
                    : 'Save to a password manager (1Password / Bitwarden)'}
                </div>
                <div>
                  ✅{' '}
                  {language === 'zh'
                    ? '或抄在纸上放安全的地方'
                    : 'Or write it down and store it safely'}
                </div>
                <div>
                  ❌{' '}
                  {language === 'zh'
                    ? '不要截图发给别人'
                    : 'Do NOT screenshot or share with anyone'}
                </div>
              </div>
            </div>
          )}

          <div
            className="flex items-start gap-1.5 text-[11px]"
            style={{ color: '#848E9C' }}
          >
            <span className="mt-px">🔒</span>
            <span>{t('modelConfig.privateKeyNote', language)}</span>
          </div>
        </div>

        {/* Wallet Validation Results */}
        {apiKey && (
          <div className="space-y-2 pl-1">
            {/* Validating spinner */}
            {validating && (
              <div
                className="flex items-center gap-2 text-xs"
                style={{ color: '#60A5FA' }}
              >
                <span className="animate-spin">⏳</span>
                {t('modelConfig.validating', language)}
              </div>
            )}

            {/* Error message */}
            {keyError && !validating && (
              <div
                className="flex items-center gap-2 text-xs"
                style={{ color: '#EF4444' }}
              >
                <span>❌</span>
                {keyError}
              </div>
            )}

            {/* Success: address + balance + status */}
            {walletAddress && !validating && !keyError && (
              <>
                <div
                  className="p-2.5 rounded-lg"
                  style={{
                    background: 'rgba(96,165,250,0.06)',
                    border: '1px solid rgba(96,165,250,0.15)',
                  }}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-[11px]" style={{ color: '#A0AEC0' }}>
                      {t('modelConfig.walletAddress', language)}:
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        navigator.clipboard.writeText(walletAddress)
                        setCopiedAddr(true)
                        setTimeout(() => setCopiedAddr(false), 2000)
                      }}
                      className="text-[10px] px-1.5 py-0.5 rounded"
                      style={{
                        background: 'rgba(96,165,250,0.1)',
                        color: '#60A5FA',
                        border: 'none',
                        cursor: 'pointer',
                      }}
                    >
                      {copiedAddr ? '✅' : '📋'}
                    </button>
                  </div>
                  <code
                    className="text-[11px] font-mono block select-all"
                    style={{ color: '#60A5FA' }}
                  >
                    {resolvedWalletAddress}
                  </code>
                  <div
                    className="text-[10px] mt-1.5"
                    style={{ color: '#F59E0B' }}
                  >
                    ⚠️{' '}
                    {language === 'zh'
                      ? '请确认这是你的钱包地址（可在 MetaMask 中核对）'
                      : 'Please confirm this is your wallet address (verify in MetaMask)'}
                  </div>
                </div>
                {usdcBalance !== null && (
                  <div className="flex items-center gap-2 text-xs">
                    <span>💰</span>
                    <span
                      style={{ color: balanceNum > 0 ? '#00E096' : '#F59E0B' }}
                    >
                      {t('modelConfig.usdcBalance', language)}: $
                      {resolvedUsdcBalance}
                    </span>
                    <button
                      type="button"
                      onClick={() => setShowDeposit(!showDeposit)}
                      className="text-[10px] px-2 py-0.5 rounded transition-all"
                      style={{
                        background: 'rgba(0,224,150,0.1)',
                        color: '#00E096',
                        border: 'none',
                        cursor: 'pointer',
                      }}
                    >
                      {showDeposit
                        ? language === 'zh'
                          ? '收起'
                          : 'Hide'
                        : language === 'zh'
                          ? '💳 充值'
                          : '💳 Deposit'}
                    </button>
                  </div>
                )}
                {showDeposit && (
                  <div
                    className="p-3 rounded-xl mt-1"
                    style={{
                      background: 'rgba(0, 224, 150, 0.04)',
                      border: '1px solid rgba(0, 224, 150, 0.15)',
                    }}
                  >
                    <div
                      className="text-xs font-semibold mb-2"
                      style={{ color: '#00E096' }}
                    >
                      💳{' '}
                      {language === 'zh'
                        ? '充值 USDC (Base 链)'
                        : 'Deposit USDC (Base Chain)'}
                    </div>
                    <div className="flex gap-3 items-start mb-3">
                      <div
                        className="shrink-0 p-1.5 rounded-lg"
                        style={{ background: '#fff' }}
                      >
                        <QRCodeSVG
                          value={resolvedWalletAddress}
                          size={80}
                          level="M"
                        />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div
                          className="text-[11px] mb-1"
                          style={{ color: '#A0AEC0' }}
                        >
                          {language === 'zh'
                            ? '扫码或复制地址转账'
                            : 'Scan QR or copy address to transfer'}
                        </div>
                        <code
                          className="text-[10px] font-mono break-all select-all block mb-1.5"
                          style={{ color: '#60A5FA' }}
                        >
                          {resolvedWalletAddress}
                        </code>
                        <button
                          type="button"
                          onClick={() => {
                            navigator.clipboard.writeText(walletAddress)
                            setCopiedAddr(true)
                            setTimeout(() => setCopiedAddr(false), 2000)
                          }}
                          className="text-[10px] px-2 py-0.5 rounded"
                          style={{
                            background: 'rgba(96,165,250,0.1)',
                            color: '#60A5FA',
                            border: 'none',
                            cursor: 'pointer',
                          }}
                        >
                          {copiedAddr ? '✅ Copied' : '📋 Copy Address'}
                        </button>
                      </div>
                    </div>
                    <div
                      className="text-[10px] space-y-1"
                      style={{ color: '#848E9C' }}
                    >
                      <div>
                        📱{' '}
                        {language === 'zh'
                          ? '用交易所 App 扫描二维码直接转账'
                          : 'Scan QR with exchange app to transfer'}
                      </div>
                      <div>
                        •{' '}
                        {language === 'zh'
                          ? '提币时网络选择 Base'
                          : 'Choose Base network when withdrawing'}
                      </div>
                      <div>
                        • {language === 'zh' ? '或跨链桥: ' : 'Or bridge: '}
                        <a
                          href="https://bridge.base.org"
                          target="_blank"
                          rel="noopener noreferrer"
                          className="underline"
                          style={{ color: '#60A5FA' }}
                        >
                          bridge.base.org
                        </a>
                      </div>
                      <div>
                        •{' '}
                        {language === 'zh'
                          ? '最低充值 $1 USDC 即可开始'
                          : 'Min $1 USDC to start'}
                      </div>
                    </div>
                  </div>
                )}
                {claw402Status && (
                  <div
                    className="flex items-center gap-2 text-xs"
                    style={{
                      color: claw402Status === 'ok' ? '#00E096' : '#EF4444',
                    }}
                  >
                    <span>{claw402Status === 'ok' ? '🟢' : '🔴'}</span>
                    {claw402Status === 'ok'
                      ? t('modelConfig.claw402Connected', language)
                      : t('modelConfig.claw402Unreachable', language)}
                  </div>
                )}
              </>
            )}

            {/* Test Connection button */}
            {(isKeyValid || hasExistingWallet) && !validating && (
              <button
                type="button"
                onClick={handleTestConnection}
                disabled={testing}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all hover:scale-[1.02] disabled:opacity-50"
                style={{
                  background: 'rgba(37, 99, 235, 0.15)',
                  border: '1px solid rgba(37, 99, 235, 0.3)',
                  color: '#60A5FA',
                }}
              >
                <span>🔗</span>
                {testing
                  ? t('modelConfig.testingConnection', language)
                  : t('modelConfig.testConnection', language)}
              </button>
            )}

            {/* Test result */}
            {testResult && !testing && (
              <div
                className="flex items-center gap-2 text-xs"
                style={{
                  color: testResult.status === 'ok' ? '#00E096' : '#EF4444',
                }}
              >
                <span>{testResult.status === 'ok' ? '✅' : '❌'}</span>
                {testResult.message}
              </div>
            )}
          </div>
        )}
      </div>

      {/* USDC Recharge Guide */}
      <div
        className="p-4 rounded-xl"
        style={{
          background: 'rgba(0, 224, 150, 0.05)',
          border: '1px solid rgba(0, 224, 150, 0.15)',
        }}
      >
        <div
          className="text-sm font-semibold mb-2 flex items-center gap-2"
          style={{ color: '#00E096' }}
        >
          {'💰 ' + t('modelConfig.howToFundUsdc', language)}
        </div>
        <div className="text-xs space-y-1.5" style={{ color: '#848E9C' }}>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>
              1.
            </span>
            <span>{t('modelConfig.fundStep1', language)}</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>
              2.
            </span>
            <span>{t('modelConfig.fundStep2', language)}</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>
              3.
            </span>
            <span>{t('modelConfig.fundStep3', language)}</span>
          </div>
        </div>
      </div>

      {/* Buttons */}
      <div className="flex gap-3 pt-2">
        <button
          type="button"
          onClick={onBack}
          className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5"
          style={{ background: '#2B3139', color: '#848E9C' }}
        >
          {editingModelId
            ? t('cancel', language)
            : t('modelConfig.back', language)}
        </button>
        <button
          type="submit"
          disabled={!isKeyValid && !hasExistingWallet}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
          style={{
            background:
              isKeyValid || hasExistingWallet
                ? 'linear-gradient(135deg, #2563EB, #7C3AED)'
                : '#2B3139',
            color: '#fff',
          }}
        >
          {'🚀 ' + t('modelConfig.startTrading', language)}
        </button>
      </div>
    </form>
  )
}

function StandardProviderConfigForm({
  selectedModel,
  apiKey,
  baseUrl,
  modelName,
  editingModelId,
  onApiKeyChange,
  onBaseUrlChange,
  onModelNameChange,
  onBack,
  onSubmit,
  language,
}: {
  selectedModel: AIModel
  apiKey: string
  baseUrl: string
  modelName: string
  editingModelId: string | null
  onApiKeyChange: (value: string) => void
  onBaseUrlChange: (value: string) => void
  onModelNameChange: (value: string) => void
  onBack: () => void
  onSubmit: (e: React.FormEvent) => void
  language: Language
}) {
  return (
    <form onSubmit={onSubmit} className="space-y-5">
      {/* Selected Model Header */}
      <div
        className="p-4 rounded-xl flex items-center gap-4"
        style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
      >
        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black border border-white/10">
          {getModelIcon(selectedModel.provider || selectedModel.id, {
            width: 32,
            height: 32,
          }) || (
            <span className="text-lg font-bold" style={{ color: '#A78BFA' }}>
              {selectedModel.name[0]}
            </span>
          )}
        </div>
        <div className="flex-1">
          <div className="font-semibold text-lg" style={{ color: '#EAECEF' }}>
            {getShortName(selectedModel.name)}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {selectedModel.provider} •{' '}
            {AI_PROVIDER_CONFIG[selectedModel.provider]?.defaultModel ||
              selectedModel.id}
          </div>
        </div>
        {AI_PROVIDER_CONFIG[selectedModel.provider] && (
          <a
            href={AI_PROVIDER_CONFIG[selectedModel.provider].apiUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 rounded-lg transition-all hover:scale-105"
            style={{
              background: 'rgba(139, 92, 246, 0.1)',
              border: '1px solid rgba(139, 92, 246, 0.3)',
            }}
          >
            <ExternalLink className="w-4 h-4" style={{ color: '#A78BFA' }} />
            <span className="text-sm font-medium" style={{ color: '#A78BFA' }}>
              {selectedModel.provider?.startsWith('blockrun')
                ? t('modelConfig.getStarted', language)
                : t('modelConfig.getApiKey', language)}
            </span>
          </a>
        )}
      </div>

      {/* Kimi Warning */}
      {selectedModel.provider === 'kimi' && (
        <div
          className="p-4 rounded-xl"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.3)',
          }}
        >
          <div className="flex items-start gap-2">
            <span style={{ fontSize: '16px' }}>⚠️</span>
            <div className="text-sm" style={{ color: '#F6465D' }}>
              {t('kimiApiNote', language)}
            </div>
          </div>
        </div>
      )}

      {/* API Key / Wallet Private Key */}
      {editingModelId && selectedModel && 'has_api_key' in selectedModel && (
        <div
          className="p-3 rounded-xl text-xs"
          style={{
            background: 'rgba(14, 203, 129, 0.08)',
            border: '1px solid rgba(14, 203, 129, 0.2)',
            color: '#9FE8C5',
          }}
        >
          当前模型密钥状态：
          {selectedModel.has_api_key ? '已配置 API Key' : '未配置 API Key'}
        </div>
      )}

      <div className="space-y-2">
        <label
          className="flex items-center gap-2 text-sm font-semibold"
          style={{ color: '#EAECEF' }}
        >
          <svg
            className="w-4 h-4"
            style={{ color: '#A78BFA' }}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
            />
          </svg>
          {selectedModel.provider?.startsWith('blockrun')
            ? t('modelConfig.walletPrivateKeyLabel', language)
            : 'API Key *'}
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => onApiKeyChange(e.target.value)}
          placeholder={
            editingModelId && selectedModel.has_api_key
              ? '已保存，如需更换请重新输入'
              : selectedModel.provider === 'blockrun-base'
                ? '0x... (EVM private key)'
                : selectedModel.provider === 'blockrun-sol'
                  ? 'bs58 encoded key (Solana)'
                  : t('enterAPIKey', language)
          }
          className="w-full px-4 py-3 rounded-xl"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: '#EAECEF',
          }}
          required
        />
      </div>

      {/* Custom Base URL */}
      {!selectedModel.provider?.startsWith('blockrun') && (
        <div className="space-y-2">
          <label
            className="flex items-center gap-2 text-sm font-semibold"
            style={{ color: '#EAECEF' }}
          >
            <svg
              className="w-4 h-4"
              style={{ color: '#A78BFA' }}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
              />
            </svg>
            {t('customBaseURL', language)}
          </label>
          <input
            type="url"
            value={baseUrl}
            onChange={(e) => onBaseUrlChange(e.target.value)}
            placeholder={t('customBaseURLPlaceholder', language)}
            className="w-full px-4 py-3 rounded-xl"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          />
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('leaveBlankForDefault', language)}
          </div>
        </div>
      )}

      {/* Custom Model Name */}
      {!selectedModel.provider?.startsWith('blockrun') && (
        <div className="space-y-2">
          <label
            className="flex items-center gap-2 text-sm font-semibold"
            style={{ color: '#EAECEF' }}
          >
            <svg
              className="w-4 h-4"
              style={{ color: '#A78BFA' }}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
              />
            </svg>
            {t('customModelName', language)}
          </label>
          <input
            type="text"
            value={modelName}
            onChange={(e) => onModelNameChange(e.target.value)}
            placeholder={t('customModelNamePlaceholder', language)}
            className="w-full px-4 py-3 rounded-xl"
            style={{
              background: '#0B0E11',
              border: '1px solid #2B3139',
              color: '#EAECEF',
            }}
          />
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('leaveBlankForDefaultModel', language)}
          </div>
        </div>
      )}

      {/* Info Box */}
      <div
        className="p-4 rounded-xl"
        style={{
          background: 'rgba(139, 92, 246, 0.1)',
          border: '1px solid rgba(139, 92, 246, 0.2)',
        }}
      >
        <div
          className="text-sm font-semibold mb-2 flex items-center gap-2"
          style={{ color: '#A78BFA' }}
        >
          <Brain className="w-4 h-4" />
          {t('information', language)}
        </div>
        <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
          <div>• {t('modelConfigInfo1', language)}</div>
          <div>• {t('modelConfigInfo2', language)}</div>
          <div>• {t('modelConfigInfo3', language)}</div>
        </div>
      </div>

      {/* Buttons */}
      <div className="flex gap-3 pt-4">
        <button
          type="button"
          onClick={onBack}
          className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5"
          style={{ background: '#2B3139', color: '#848E9C' }}
        >
          {editingModelId
            ? t('cancel', language)
            : t('modelConfig.back', language)}
        </button>
        <button
          type="submit"
          disabled={!selectedModel || !apiKey.trim()}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
          style={{ background: '#8B5CF6', color: '#fff' }}
        >
          {t('saveConfig', language)}
          <svg
            className="w-4 h-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M14 5l7 7m0 0l-7 7m7-7H3"
            />
          </svg>
        </button>
      </div>
    </form>
  )
}
