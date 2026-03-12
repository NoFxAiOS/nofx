import React, { useState, useEffect } from 'react'
import { Trash2, Brain, ExternalLink } from 'lucide-react'
import type { AIModel } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { getModelIcon } from '../common/ModelIcons'
import { ModelStepIndicator } from './ModelStepIndicator'
import { ModelCard } from './ModelCard'
import {
  BLOCKRUN_MODELS,
  CLAW402_MODELS,
  AI_PROVIDER_CONFIG,
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
    if (!selectedModelId || !apiKey.trim()) return
    onSave(selectedModelId, apiKey.trim(), baseUrl.trim() || undefined, modelName.trim() || undefined)
  }

  const availableModels = allModels || []
  const configuredIds = new Set(configuredModels?.map(m => m.id) || [])
  const stepLabels = language === 'zh' ? ['选择模型', '配置 API'] : ['Select Model', 'Configure API']

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4 overflow-y-auto backdrop-blur-sm">
      <div
        className="rounded-2xl w-full max-w-2xl relative my-8 shadow-2xl"
        style={{ background: 'linear-gradient(180deg, #1E2329 0%, #181A20 100%)', maxHeight: 'calc(100vh - 4rem)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {currentStep > 0 && !editingModelId && (
              <button type="button" onClick={handleBack} className="p-2 rounded-lg hover:bg-white/10 transition-colors">
                <svg className="w-5 h-5" style={{ color: '#848E9C' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
            )}
            <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
              {editingModelId ? t('editAIModel', language) : t('addAIModel', language)}
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
            <button type="button" onClick={onClose} className="p-2 rounded-lg hover:bg-white/10 transition-colors" style={{ color: '#848E9C' }}>
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
        <div className="px-6 pb-6 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 16rem)' }}>
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
          {(currentStep === 1 || editingModelId) && selectedModel && (selectedModel.provider === 'claw402' || selectedModel.id === 'claw402') && (
            <Claw402ConfigForm
              apiKey={apiKey}
              modelName={modelName}
              editingModelId={editingModelId}
              onApiKeyChange={setApiKey}
              onModelNameChange={setModelName}
              onBack={handleBack}
              onSubmit={handleSubmit}
              language={language}
            />
          )}

          {/* Step 1: Configure — Standard Providers (non-claw402) */}
          {(currentStep === 1 || editingModelId) && selectedModel && selectedModel.provider !== 'claw402' && selectedModel.id !== 'claw402' && (
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
  return (
    <div className="space-y-4">
      <div className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {language === 'zh' ? '选择 AI 模型提供商' : 'Choose Your AI Provider'}
      </div>

      {/* Claw402 Featured Card */}
      {availableModels.some(m => m.provider === 'claw402') && (
        <button
          type="button"
          onClick={() => {
            const claw = availableModels.find(m => m.provider === 'claw402')
            if (claw) onSelectModel(claw.id)
          }}
          className="w-full p-5 rounded-xl text-left transition-all hover:scale-[1.01]"
          style={{ background: 'linear-gradient(135deg, rgba(37, 99, 235, 0.15) 0%, rgba(139, 92, 246, 0.15) 100%)', border: '1.5px solid rgba(37, 99, 235, 0.4)' }}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl flex items-center justify-center overflow-hidden">
                <img src="/icons/claw402.png" alt="Claw402" width={40} height={40} />
              </div>
              <div>
                <div className="font-bold text-base" style={{ color: '#EAECEF' }}>
                  Claw402
                  <a href="https://claw402.ai" target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()} className="ml-1.5 text-[10px] font-normal px-1.5 py-0.5 rounded" style={{ color: '#60A5FA', background: 'rgba(96, 165, 250, 0.1)' }}>↗ claw402.ai</a>
                </div>
                <div className="text-xs mt-0.5" style={{ color: '#A0AEC0' }}>
                  {language === 'zh'
                    ? 'USDC 按次付费 · 支持全部 AI 模型 · 无需 API Key'
                    : 'Pay-per-call USDC · All AI Models · No API Key'}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {configuredIds.has(availableModels.find(m => m.provider === 'claw402')?.id || '') && (
                <div className="w-2 h-2 rounded-full" style={{ background: '#00E096' }} />
              )}
              <div className="px-3 py-1.5 rounded-full text-xs font-bold" style={{ background: 'linear-gradient(135deg, #2563EB, #7C3AED)', color: '#fff' }}>
                {language === 'zh' ? '🔥 推荐' : '🔥 Best'}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-3 ml-[52px]">
            <span className="text-[11px] px-2 py-0.5 rounded-full" style={{ background: 'rgba(0, 224, 150, 0.1)', color: '#00E096', border: '1px solid rgba(0, 224, 150, 0.2)' }}>
              GPT · Claude · DeepSeek · Gemini · Grok · Qwen · Kimi
            </span>
          </div>
        </button>
      )}

      <div className="grid grid-cols-3 sm:grid-cols-4 gap-3">
        {availableModels.filter(m => !m.provider?.startsWith('blockrun') && m.provider !== 'claw402').map((model) => (
          <ModelCard
            key={model.id}
            model={model}
            selected={selectedModelId === model.id}
            onClick={() => onSelectModel(model.id)}
            configured={configuredIds.has(model.id)}
          />
        ))}
      </div>
      {availableModels.some(m => m.provider?.startsWith('blockrun')) && (
        <>
          <div className="flex items-center gap-3 pt-2">
            <div className="flex-1 h-px" style={{ background: '#2B3139' }} />
            <span className="text-xs font-medium px-2" style={{ color: '#848E9C' }}>
              {language === 'zh' ? '通过钱包支付' : 'Via BlockRun Wallet'}
            </span>
            <div className="flex-1 h-px" style={{ background: '#2B3139' }} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            {availableModels.filter(m => m.provider?.startsWith('blockrun')).map((model) => (
              <ModelCard
                key={model.id}
                model={model}
                selected={selectedModelId === model.id}
                onClick={() => onSelectModel(model.id)}
                configured={configuredIds.has(model.id)}
              />
            ))}
          </div>
        </>
      )}
      <div className="text-xs text-center pt-2" style={{ color: '#848E9C' }}>
        {language === 'zh' ? '带金色标记的模型已配置' : 'Models with gold badge are already configured'}
      </div>
    </div>
  )
}

function Claw402ConfigForm({
  apiKey,
  modelName,
  editingModelId,
  onApiKeyChange,
  onModelNameChange,
  onBack,
  onSubmit,
  language,
}: {
  apiKey: string
  modelName: string
  editingModelId: string | null
  onApiKeyChange: (value: string) => void
  onModelNameChange: (value: string) => void
  onBack: () => void
  onSubmit: (e: React.FormEvent) => void
  language: Language
}) {
  return (
    <form onSubmit={onSubmit} className="space-y-5">
      {/* Claw402 Hero Header */}
      <div className="p-5 rounded-xl text-center" style={{ background: 'linear-gradient(135deg, rgba(37, 99, 235, 0.12) 0%, rgba(139, 92, 246, 0.12) 100%)', border: '1px solid rgba(37, 99, 235, 0.3)' }}>
        <div className="w-14 h-14 mx-auto rounded-2xl flex items-center justify-center mb-3 overflow-hidden">
          <img src="/icons/claw402.png" alt="Claw402" width={56} height={56} />
        </div>
        <a href="https://claw402.ai" target="_blank" rel="noopener noreferrer" className="text-lg font-bold inline-flex items-center gap-1.5 hover:underline" style={{ color: '#EAECEF' }}>
          Claw402 <span className="text-xs font-normal" style={{ color: '#60A5FA' }}>↗</span>
        </a>
        <div className="text-sm mt-1" style={{ color: '#A0AEC0' }}>
          {language === 'zh'
            ? '用 USDC 按次付费，支持所有主流 AI 模型'
            : 'Pay-per-call with USDC — supports all major AI models'}
        </div>
        <div className="flex items-center justify-center gap-3 mt-3 flex-wrap">
          {['GPT', 'Claude', 'DeepSeek', 'Gemini', 'Grok', 'Qwen', 'Kimi'].map(name => (
            <span key={name} className="text-[11px] px-2 py-0.5 rounded-full" style={{ background: 'rgba(255,255,255,0.06)', color: '#A0AEC0' }}>
              {name}
            </span>
          ))}
        </div>
      </div>

      {/* Step 1: Select AI Model */}
      <div className="space-y-3">
        <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
          <Brain className="w-4 h-4" style={{ color: '#2563EB' }} />
          {language === 'zh' ? '① 选择 AI 模型' : '① Choose AI Model'}
        </label>
        <div className="text-xs mb-2" style={{ color: '#848E9C' }}>
          {language === 'zh'
            ? '所有模型通过 Claw402 统一调用，创建后可随时切换'
            : 'All models unified via Claw402. Switch anytime after setup.'}
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
          {CLAW402_MODELS.map((m) => {
            const isSelected = (modelName || 'deepseek') === m.id
            return (
              <button
                key={m.id}
                type="button"
                onClick={() => onModelNameChange(m.id)}
                className="flex items-start gap-2 px-3 py-2.5 rounded-xl text-left transition-all hover:scale-[1.02]"
                style={{
                  background: isSelected ? 'rgba(37, 99, 235, 0.2)' : '#0B0E11',
                  border: isSelected ? '1.5px solid #2563EB' : '1px solid #2B3139',
                }}
              >
                <span className="text-base mt-0.5">{m.icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-semibold truncate" style={{ color: isSelected ? '#60A5FA' : '#EAECEF' }}>
                    {m.name}
                  </div>
                  <div className="text-[10px] truncate" style={{ color: '#848E9C' }}>
                    {m.provider} · {m.desc}
                  </div>
                </div>
                {isSelected && (
                  <span className="text-[10px] mt-1" style={{ color: '#60A5FA' }}>✓</span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* Step 2: Wallet Setup */}
      <div className="space-y-3">
        <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
          <svg className="w-4 h-4" style={{ color: '#2563EB' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
          </svg>
          {language === 'zh' ? '② 设置钱包' : '② Setup Wallet'}
        </label>

        <div className="p-3 rounded-xl" style={{ background: 'rgba(37, 99, 235, 0.06)', border: '1px solid rgba(37, 99, 235, 0.15)' }}>
          <div className="text-xs mb-2" style={{ color: '#A0AEC0' }}>
            {language === 'zh'
              ? '💡 Claw402 使用 Base 链上的 USDC 付费，你需要一个 EVM 钱包'
              : '💡 Claw402 uses USDC on Base chain. You need an EVM wallet.'}
          </div>
          <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
            <div className="flex items-center gap-1.5">
              <span style={{ color: '#00E096' }}>•</span>
              {language === 'zh'
                ? '可以用 MetaMask、Rabby 等钱包导出私钥'
                : 'Export private key from MetaMask, Rabby, etc.'}
            </div>
            <div className="flex items-center gap-1.5">
              <span style={{ color: '#00E096' }}>•</span>
              {language === 'zh'
                ? '建议新建一个专用钱包，充入少量 USDC 即可'
                : 'Recommended: create a dedicated wallet with a small USDC balance'}
            </div>
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="text-xs font-medium" style={{ color: '#A0AEC0' }}>
            {language === 'zh' ? '钱包私钥（Base 链 EVM）' : 'Wallet Private Key (Base Chain EVM)'}
          </div>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => onApiKeyChange(e.target.value)}
            placeholder="0x..."
            className="w-full px-4 py-3 rounded-xl font-mono text-sm"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
            required
          />
          <div className="flex items-start gap-1.5 text-[11px]" style={{ color: '#848E9C' }}>
            <span className="mt-px">🔒</span>
            <span>
              {language === 'zh'
                ? '私钥仅在本地签名使用，不会上传或发送交易。无需 ETH，无 Gas 费用。'
                : 'Private key is only used locally for signing. Never uploaded. No ETH or gas needed.'}
            </span>
          </div>
        </div>
      </div>

      {/* USDC Recharge Guide */}
      <div className="p-4 rounded-xl" style={{ background: 'rgba(0, 224, 150, 0.05)', border: '1px solid rgba(0, 224, 150, 0.15)' }}>
        <div className="text-sm font-semibold mb-2 flex items-center gap-2" style={{ color: '#00E096' }}>
          💰 {language === 'zh' ? '如何充值 USDC' : 'How to Fund USDC'}
        </div>
        <div className="text-xs space-y-1.5" style={{ color: '#848E9C' }}>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>1.</span>
            <span>{language === 'zh' ? '从交易所（Binance / OKX / Coinbase）提 USDC 到你的钱包地址' : 'Withdraw USDC from exchange (Binance/OKX/Coinbase) to your wallet'}</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>2.</span>
            <span>{language === 'zh' ? '选择 Base 网络（手续费极低）' : 'Select Base network (very low fees)'}</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="font-bold" style={{ color: '#A0AEC0' }}>3.</span>
            <span>{language === 'zh' ? '充入 $5-10 USDC 即可使用很长时间（约 $0.003/次调用）' : '$5-10 USDC lasts a long time (~$0.003/call)'}</span>
          </div>
        </div>
      </div>

      {/* Buttons */}
      <div className="flex gap-3 pt-2">
        <button type="button" onClick={onBack} className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5" style={{ background: '#2B3139', color: '#848E9C' }}>
          {editingModelId ? t('cancel', language) : (language === 'zh' ? '返回' : 'Back')}
        </button>
        <button
          type="submit"
          disabled={!apiKey.trim()}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
          style={{ background: apiKey.trim() ? 'linear-gradient(135deg, #2563EB, #7C3AED)' : '#2B3139', color: '#fff' }}
        >
          {language === 'zh' ? '🚀 开始交易' : '🚀 Start Trading'}
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
      <div className="p-4 rounded-xl flex items-center gap-4" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black border border-white/10">
          {getModelIcon(selectedModel.provider || selectedModel.id, { width: 32, height: 32 }) || (
            <span className="text-lg font-bold" style={{ color: '#A78BFA' }}>{selectedModel.name[0]}</span>
          )}
        </div>
        <div className="flex-1">
          <div className="font-semibold text-lg" style={{ color: '#EAECEF' }}>
            {getShortName(selectedModel.name)}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {selectedModel.provider} • {AI_PROVIDER_CONFIG[selectedModel.provider]?.defaultModel || selectedModel.id}
          </div>
        </div>
        {AI_PROVIDER_CONFIG[selectedModel.provider] && (
          <a
            href={AI_PROVIDER_CONFIG[selectedModel.provider].apiUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-4 py-2 rounded-lg transition-all hover:scale-105"
            style={{ background: 'rgba(139, 92, 246, 0.1)', border: '1px solid rgba(139, 92, 246, 0.3)' }}
          >
            <ExternalLink className="w-4 h-4" style={{ color: '#A78BFA' }} />
            <span className="text-sm font-medium" style={{ color: '#A78BFA' }}>
              {selectedModel.provider?.startsWith('blockrun')
                ? (language === 'zh' ? '开始使用' : 'Get Started')
                : (language === 'zh' ? '获取 API Key' : 'Get API Key')}
            </span>
          </a>
        )}
      </div>

      {/* Kimi Warning */}
      {selectedModel.provider === 'kimi' && (
        <div className="p-4 rounded-xl" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.3)' }}>
          <div className="flex items-start gap-2">
            <span style={{ fontSize: '16px' }}>⚠️</span>
            <div className="text-sm" style={{ color: '#F6465D' }}>
              {t('kimiApiNote', language)}
            </div>
          </div>
        </div>
      )}

      {/* API Key / Wallet Private Key */}
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
          <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
          </svg>
          {selectedModel.provider?.startsWith('blockrun')
            ? (language === 'zh' ? '钱包私钥 *' : 'Wallet Private Key *')
            : 'API Key *'}
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => onApiKeyChange(e.target.value)}
          placeholder={
            selectedModel.provider === 'blockrun-base'
              ? '0x... (EVM private key)'
              : selectedModel.provider === 'blockrun-sol'
              ? 'bs58 encoded key (Solana)'
              : t('enterAPIKey', language)
          }
          className="w-full px-4 py-3 rounded-xl"
          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          required
        />
      </div>

      {/* Custom Base URL (hidden for BlockRun) */}
      {!selectedModel.provider?.startsWith('blockrun') && (
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
            <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
            </svg>
            {t('customBaseURL', language)}
          </label>
          <input
            type="url"
            value={baseUrl}
            onChange={(e) => onBaseUrlChange(e.target.value)}
            placeholder={t('customBaseURLPlaceholder', language)}
            className="w-full px-4 py-3 rounded-xl"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('leaveBlankForDefault', language)}
          </div>
        </div>
      )}

      {/* Custom Model Name (hidden for BlockRun) */}
      {!selectedModel.provider?.startsWith('blockrun') && (
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
            <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
            </svg>
            {t('customModelName', language)}
          </label>
          <input
            type="text"
            value={modelName}
            onChange={(e) => onModelNameChange(e.target.value)}
            placeholder={t('customModelNamePlaceholder', language)}
            className="w-full px-4 py-3 rounded-xl"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('leaveBlankForDefaultModel', language)}
          </div>
        </div>
      )}

      {/* BlockRun Model Selector */}
      {selectedModel.provider?.startsWith('blockrun') && (
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: '#EAECEF' }}>
            <svg className="w-4 h-4" style={{ color: '#A78BFA' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            {language === 'zh' ? '选择模型' : 'Select Model'}
          </label>
          <div className="grid grid-cols-2 gap-2">
            {BLOCKRUN_MODELS.map((m) => {
              const isSelected = (modelName || BLOCKRUN_MODELS[0].id) === m.id
              return (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => onModelNameChange(m.id)}
                  className="flex flex-col items-start px-3 py-2 rounded-xl text-left transition-all"
                  style={{
                    background: isSelected ? 'rgba(37, 99, 235, 0.2)' : '#0B0E11',
                    border: isSelected ? '1px solid #2563EB' : '1px solid #2B3139',
                  }}
                >
                  <span className="text-xs font-semibold" style={{ color: isSelected ? '#60A5FA' : '#EAECEF' }}>
                    {m.name}
                  </span>
                  <span className="text-[10px]" style={{ color: '#848E9C' }}>{m.desc}</span>
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Info Box */}
      <div className="p-4 rounded-xl" style={{ background: 'rgba(139, 92, 246, 0.1)', border: '1px solid rgba(139, 92, 246, 0.2)' }}>
        <div className="text-sm font-semibold mb-2 flex items-center gap-2" style={{ color: '#A78BFA' }}>
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
        <button type="button" onClick={onBack} className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5" style={{ background: '#2B3139', color: '#848E9C' }}>
          {editingModelId ? t('cancel', language) : (language === 'zh' ? '返回' : 'Back')}
        </button>
        <button
          type="submit"
          disabled={!selectedModel || !apiKey.trim()}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
          style={{ background: '#8B5CF6', color: '#fff' }}
        >
          {t('saveConfig', language)}
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
          </svg>
        </button>
      </div>
    </form>
  )
}
