import React, { useState, useEffect } from 'react'
import type { Exchange, PaperConfig } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import { getExchangeIcon } from '../common/ExchangeIcons'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from '../modals/TwoStageKeyModal'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../common/WebCryptoEnvironmentCheck'
import {
  BookOpen,
  Trash2,
  HelpCircle,
  ExternalLink,
  UserPlus,
  Key,
  Shield,
  ChevronLeft,
  Check,
  Copy,
  ArrowRight,
} from 'lucide-react'
import { toast } from 'sonner'
import { Tooltip } from './Tooltip'
import { getShortName } from './utils'
import { PaperBadge } from './PaperBadge'

// Supported exchange templates
const SUPPORTED_EXCHANGE_TEMPLATES = [
  { exchange_type: 'paper', name: 'Paper Futures', type: 'paper' as const },
  { exchange_type: 'binance', name: 'Binance Futures', type: 'cex' as const },
  {
    exchange_type: 'binance_stocks',
    name: 'Binance Stocks (美股)',
    type: 'stock' as const,
  },
  { exchange_type: 'bybit', name: 'Bybit Futures', type: 'cex' as const },
  { exchange_type: 'okx', name: 'OKX Futures', type: 'cex' as const },
  { exchange_type: 'bitget', name: 'Bitget Futures', type: 'cex' as const },
  { exchange_type: 'gate', name: 'Gate.io Futures', type: 'cex' as const },
  { exchange_type: 'kucoin', name: 'KuCoin Futures', type: 'cex' as const },
  { exchange_type: 'hyperliquid', name: 'Hyperliquid', type: 'dex' as const },
  { exchange_type: 'aster', name: 'Aster DEX', type: 'dex' as const },
  { exchange_type: 'lighter', name: 'Lighter', type: 'dex' as const },
  { exchange_type: 'indodax', name: 'Indodax', type: 'cex' as const },
]

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string | null,
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number,
    paperConfig?: PaperConfig
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

// Step indicator component
function StepIndicator({
  currentStep,
  labels,
}: {
  currentStep: number
  labels: string[]
}) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background:
                  index < currentStep
                    ? 'var(--up)'
                    : index === currentStep
                      ? 'var(--brand)'
                      : 'var(--line)',
                color: index <= currentStep ? 'var(--brand-fg)' : 'var(--fg-3)',
              }}
            >
              {index < currentStep ? <Check className="w-4 h-4" /> : index + 1}
            </div>
            <span
              className="text-xs font-medium hidden sm:block"
              style={{
                color: index === currentStep ? 'var(--fg)' : 'var(--fg-3)',
              }}
            >
              {label}
            </span>
          </div>
          {index < labels.length - 1 && (
            <div
              className="w-8 h-0.5 mx-1"
              style={{
                background: index < currentStep ? 'var(--up)' : 'var(--line)',
              }}
            />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

// Exchange card component
function ExchangeCard({
  template,
  selected,
  onClick,
  disabled,
}: {
  template: (typeof SUPPORTED_EXCHANGE_TEMPLATES)[0]
  selected: boolean
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex flex-col items-center gap-2 p-4 rounded-xl transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
      style={{
        background: selected ? 'var(--brand-soft)' : 'var(--surface-2)',
        border: selected ? '2px solid var(--brand)' : '2px solid var(--line)',
      }}
    >
      <div className="relative">
        {getExchangeIcon(template.exchange_type, { width: 48, height: 48 })}
        {selected && (
          <div
            className="absolute -top-1 -right-1 w-5 h-5 rounded-full flex items-center justify-center"
            style={{ background: 'var(--up)' }}
          >
            <Check className="w-3 h-3 text-brand-fg" />
          </div>
        )}
      </div>
      <span className="text-sm font-semibold" style={{ color: 'var(--fg)' }}>
        {getShortName(template.name)}
      </span>
      <span
        className="text-xs px-2 py-0.5 rounded-full"
        style={{
          background:
            template.type === 'cex' ? 'var(--brand-soft)' : 'var(--ai-soft)',
          color: template.type === 'cex' ? 'var(--brand)' : 'var(--ai)',
        }}
      >
        {template.type.toUpperCase()}
      </span>
    </button>
  )
}

export function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  // Step: 0 = select exchange, 1 = configure
  const [currentStep, setCurrentStep] = useState(editingExchangeId ? 1 : 0)
  const [selectedExchangeType, setSelectedExchangeType] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [paperConfig, setPaperConfig] = useState<PaperConfig>({
    initial_balance: 1000,
    taker_fee_bps: 5,
    maker_fee_bps: 2,
    slippage_bps: 3,
    default_leverage: 5,
  })
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [webCryptoStatus, setWebCryptoStatus] =
    useState<WebCryptoCheckStatus>('idle')
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster fields
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid fields
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // Lighter fields
  const [lighterWalletAddr, setLighterWalletAddr] = useState('')
  const [lighterApiKeyPrivateKey, setLighterApiKeyPrivateKey] = useState('')
  const [lighterApiKeyIndex, setLighterApiKeyIndex] = useState(0)

  // Other state
  const [secureInputTarget, setSecureInputTarget] = useState<
    null | 'hyperliquid' | 'aster' | 'lighter'
  >(null)
  const [isSaving, setIsSaving] = useState(false)
  const [accountName, setAccountName] = useState('')

  const selectedExchange = editingExchangeId
    ? allExchanges?.find((e) => e.id === editingExchangeId)
    : null

  const selectedTemplate = editingExchangeId
    ? SUPPORTED_EXCHANGE_TEMPLATES.find(
        (t) => t.exchange_type === selectedExchange?.exchange_type
      )
    : SUPPORTED_EXCHANGE_TEMPLATES.find(
        (t) => t.exchange_type === selectedExchangeType
      )

  const currentExchangeType = editingExchangeId
    ? selectedExchange?.exchange_type
    : selectedExchangeType

  const exchangeRegistrationLinks: Record<
    string,
    { url: string; hasReferral?: boolean }
  > = {
    binance: {
      url: 'https://www.binance.com/join?ref=NOFXENG',
      hasReferral: true,
    },
    okx: { url: 'https://www.okx.com/join/1865360', hasReferral: true },
    bybit: { url: 'https://partner.bybit.com/b/83856', hasReferral: true },
    bitget: {
      url: 'https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172',
      hasReferral: true,
    },
    gate: { url: 'https://www.gatenode.xyz/share/VQBGUAxY', hasReferral: true },
    kucoin: {
      url: 'https://www.kucoin.com/r/broker/CXEV7XKK',
      hasReferral: true,
    },
    hyperliquid: {
      url: 'https://app.hyperliquid.xyz/join/AITRADING',
      hasReferral: true,
    },
    aster: {
      url: 'https://www.asterdex.com/en/referral/fdfc0e',
      hasReferral: true,
    },
    lighter: {
      url: 'https://app.lighter.xyz/?referral=68151432',
      hasReferral: true,
    },
    indodax: { url: 'https://indodax.com/ref/Saep23/1', hasReferral: true },
  }

  // Initialize form when editing
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setAccountName(selectedExchange.account_name || '')
      if (selectedExchange.paper_config)
        setPaperConfig(selectedExchange.paper_config)
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('')
      setTestnet(selectedExchange.testnet || false)
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('')
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')
      setLighterWalletAddr(selectedExchange.lighterWalletAddr || '')
      setLighterApiKeyPrivateKey('')
      setLighterApiKeyIndex(selectedExchange.lighterApiKeyIndex || 0)
    }
  }, [editingExchangeId, selectedExchange])

  // Load server IP for Binance
  useEffect(() => {
    if (currentExchangeType === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => setServerIP(data))
        .catch((err) => console.error('Failed to load server IP:', err))
        .finally(() => setLoadingIP(false))
    }
  }, [currentExchangeType, serverIP])

  const handleCopyIP = async (ip: string) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        document.body.appendChild(textArea)
        textArea.select()
        document.execCommand('copy')
        document.body.removeChild(textArea)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      }
    } catch {
      toast.error(t('copyIPFailed', language))
    }
  }

  const secureInputContextLabel =
    secureInputTarget === 'aster'
      ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid'
        ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputComplete = ({ value }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') setApiKey(trimmed)
    if (secureInputTarget === 'aster') setAsterPrivateKey(trimmed)
    if (secureInputTarget === 'lighter') {
      setLighterApiKeyPrivateKey(trimmed)
      toast.success(t('lighterApiKeyImported', language))
    }
    setSecureInputTarget(null)
  }

  const maskSecret = (secret: string) => {
    if (!secret || secret.length === 0) return ''
    if (secret.length <= 8) return '*'.repeat(secret.length)
    return (
      secret.slice(0, 4) +
      '*'.repeat(Math.max(secret.length - 8, 4)) +
      secret.slice(-4)
    )
  }

  const handleSelectExchange = (exchangeType: string) => {
    setSelectedExchangeType(exchangeType)
    setCurrentStep(1)
  }

  const handleBack = () => {
    if (editingExchangeId) {
      onClose()
    } else {
      setCurrentStep(0)
      setSelectedExchangeType('')
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSaving) return
    if (!editingExchangeId && !selectedExchangeType) return

    const trimmedAccountName = accountName.trim()
    if (!trimmedAccountName) {
      toast.error(t('exchangeConfig.pleaseEnterAccountName', language))
      return
    }

    const exchangeId = editingExchangeId || null
    const exchangeType = currentExchangeType || ''

    if (currentExchangeType === 'paper') {
      const bps = [
        paperConfig.taker_fee_bps,
        paperConfig.maker_fee_bps,
        paperConfig.slippage_bps,
      ]
      if (
        !Number.isFinite(paperConfig.initial_balance) ||
        paperConfig.initial_balance <= 0 ||
        bps.some(
          (value) => !Number.isFinite(value) || value < 0 || value > 100
        ) ||
        !Number.isInteger(paperConfig.default_leverage) ||
        paperConfig.default_leverage < 1 ||
        paperConfig.default_leverage > 125
      ) {
        toast.error(t('paper.validation', language))
        return
      }
    }
    setIsSaving(true)
    try {
      if (currentExchangeType === 'paper') {
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          '',
          '',
          '',
          false,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          paperConfig
        )
      } else if (
        currentExchangeType === 'binance' ||
        currentExchangeType === 'bybit' ||
        currentExchangeType === 'indodax'
      ) {
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          apiKey.trim(),
          secretKey.trim(),
          '',
          testnet
        )
      } else if (
        currentExchangeType === 'okx' ||
        currentExchangeType === 'bitget' ||
        currentExchangeType === 'kucoin'
      ) {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          apiKey.trim(),
          secretKey.trim(),
          passphrase.trim(),
          testnet
        )
      } else if (currentExchangeType === 'hyperliquid') {
        if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          apiKey.trim(),
          '',
          '',
          testnet,
          hyperliquidWalletAddr.trim()
        )
      } else if (currentExchangeType === 'aster') {
        if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim())
          return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          '',
          '',
          '',
          testnet,
          undefined,
          asterUser.trim(),
          asterSigner.trim(),
          asterPrivateKey.trim()
        )
      } else if (currentExchangeType === 'lighter') {
        if (!lighterWalletAddr.trim() || !lighterApiKeyPrivateKey.trim()) return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          '',
          '',
          '',
          testnet,
          undefined,
          undefined,
          undefined,
          undefined,
          lighterWalletAddr.trim(),
          '',
          lighterApiKeyPrivateKey.trim(),
          lighterApiKeyIndex
        )
      } else {
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(
          exchangeId,
          exchangeType,
          trimmedAccountName,
          apiKey.trim(),
          secretKey.trim(),
          '',
          testnet
        )
      }
    } finally {
      setIsSaving(false)
    }
  }

  const stepLabels = [
    t('exchangeConfig.selectExchange', language),
    t('exchangeConfig.configure', language),
  ]
  const cexExchanges = SUPPORTED_EXCHANGE_TEMPLATES.filter(
    (t) => t.type === 'cex'
  )
  const dexExchanges = SUPPORTED_EXCHANGE_TEMPLATES.filter(
    (t) => t.type === 'dex'
  )

  return (
    <div className="fixed inset-0 bg-surface-hover flex items-center justify-center z-50 p-4 overflow-y-auto ">
      <div
        className="rounded-2xl w-full max-w-2xl relative my-8 shadow-2xl"
        style={{
          background:
            'linear-gradient(180deg, var(--surface-hover) 0%, var(--surface-hover) 100%)',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {currentStep > 0 && !editingExchangeId && (
              <button
                type="button"
                onClick={handleBack}
                className="p-2 rounded-lg hover:bg-fg/10 transition-colors"
              >
                <ChevronLeft
                  className="w-5 h-5"
                  style={{ color: 'var(--fg-3)' }}
                />
              </button>
            )}
            <h3 className="text-xl font-bold" style={{ color: 'var(--fg)' }}>
              {editingExchangeId
                ? t('editExchange', language)
                : t('addExchange', language)}
            </h3>
          </div>
          <div className="flex items-center gap-2">
            {currentExchangeType === 'binance' && currentStep === 1 && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded-lg text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'var(--brand-soft)',
                  color: 'var(--brand)',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded-lg hover:bg-down-soft transition-colors"
                style={{ color: 'var(--down)' }}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              className="p-2 rounded-lg hover:bg-fg/10 transition-colors"
              style={{ color: 'var(--fg-3)' }}
            >
              ✕
            </button>
          </div>
        </div>

        {/* Step Indicator */}
        {!editingExchangeId && (
          <div className="px-6">
            <StepIndicator currentStep={currentStep} labels={stepLabels} />
          </div>
        )}

        {/* Content */}
        <div
          className="px-6 pb-6 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Step 0: Select Exchange */}
          {currentStep === 0 && !editingExchangeId && (
            <div className="space-y-6">
              {/* WebCrypto Check */}
              <div className="space-y-2">
                <div
                  className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide"
                  style={{ color: 'var(--fg-3)' }}
                >
                  <Shield className="w-4 h-4" />
                  {t('environmentSteps.checkTitle', language)}
                </div>
                <WebCryptoEnvironmentCheck
                  language={language}
                  variant="card"
                  onStatusChange={setWebCryptoStatus}
                />
              </div>

              {/* Exchange Grid */}
              <div className="space-y-4">
                <div
                  className="text-sm font-semibold"
                  style={{ color: 'var(--fg)' }}
                >
                  {t('exchangeConfig.chooseExchange', language)}
                </div>

                <div className="space-y-3">
                  <div className="text-xs font-medium text-ai">
                    {t('paper.name', language)}
                  </div>
                  <button
                    type="button"
                    onClick={() => handleSelectExchange('paper')}
                    className="w-full rounded-xl border border-ai/40 bg-ai-soft p-4 text-left font-semibold text-ai"
                  >
                    {t('paper.name', language)}
                  </button>
                </div>

                {/* CEX */}
                <div className="space-y-3">
                  <div
                    className="text-xs font-medium uppercase tracking-wide"
                    style={{ color: 'var(--brand)' }}
                  >
                    {t('exchangeConfig.centralizedExchanges', language)}
                  </div>
                  <div className="grid grid-cols-3 sm:grid-cols-5 gap-3">
                    {cexExchanges.map((template) => (
                      <ExchangeCard
                        key={template.exchange_type}
                        template={template}
                        selected={
                          selectedExchangeType === template.exchange_type
                        }
                        onClick={() =>
                          handleSelectExchange(template.exchange_type)
                        }
                        disabled={
                          webCryptoStatus !== 'secure' &&
                          webCryptoStatus !== 'disabled'
                        }
                      />
                    ))}
                  </div>
                </div>

                {/* DEX */}
                <div className="space-y-3">
                  <div
                    className="text-xs font-medium uppercase tracking-wide"
                    style={{ color: 'var(--ai)' }}
                  >
                    {t('exchangeConfig.decentralizedExchanges', language)}
                  </div>
                  <div className="grid grid-cols-3 sm:grid-cols-5 gap-3">
                    {dexExchanges.map((template) => (
                      <ExchangeCard
                        key={template.exchange_type}
                        template={template}
                        selected={
                          selectedExchangeType === template.exchange_type
                        }
                        onClick={() =>
                          handleSelectExchange(template.exchange_type)
                        }
                        disabled={
                          webCryptoStatus !== 'secure' &&
                          webCryptoStatus !== 'disabled'
                        }
                      />
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Step 1: Configure */}
          {(currentStep === 1 || editingExchangeId) && selectedTemplate && (
            <form onSubmit={handleSubmit} className="space-y-5">
              {/* Selected Exchange Header */}
              <div
                className="p-4 rounded-xl flex items-center gap-4"
                style={{
                  background: 'var(--surface-2)',
                  border: '1px solid var(--line)',
                }}
              >
                {getExchangeIcon(selectedTemplate.exchange_type, {
                  width: 48,
                  height: 48,
                })}
                <div className="flex-1">
                  <div
                    className="font-semibold text-lg"
                    style={{ color: 'var(--fg)' }}
                  >
                    {currentExchangeType === 'paper'
                      ? t('paper.name', language)
                      : getShortName(selectedTemplate.name)}
                    <PaperBadge
                      exchangeType={currentExchangeType}
                      language={language}
                    />
                  </div>
                  <div className="text-xs" style={{ color: 'var(--fg-3)' }}>
                    {selectedTemplate.type.toUpperCase()} •{' '}
                    {selectedTemplate.exchange_type}
                  </div>
                </div>
                {currentExchangeType !== 'paper' && (
                  <a
                    href={
                      exchangeRegistrationLinks[currentExchangeType || '']
                        ?.url || '#'
                    }
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-2 rounded-lg transition-all hover:scale-105"
                    style={{
                      background: 'var(--brand-soft)',
                      border:
                        '1px solid color-mix(in srgb, var(--brand) 30%, transparent)',
                    }}
                  >
                    <UserPlus
                      className="w-4 h-4"
                      style={{ color: 'var(--brand)' }}
                    />
                    <span
                      className="text-sm font-medium"
                      style={{ color: 'var(--brand)' }}
                    >
                      {t('exchangeConfig.register', language)}
                    </span>
                    {exchangeRegistrationLinks[currentExchangeType || '']
                      ?.hasReferral && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded"
                        style={{
                          background: 'var(--up-soft)',
                          color: 'var(--up)',
                        }}
                      >
                        {t('exchangeConfig.bonus', language)}
                      </span>
                    )}
                  </a>
                )}
              </div>

              {/* Account Name */}
              <div className="space-y-2">
                <label
                  className="flex items-center gap-2 text-sm font-semibold"
                  style={{ color: 'var(--fg)' }}
                >
                  <Key className="w-4 h-4" style={{ color: 'var(--brand)' }} />
                  {t('exchangeConfig.accountName', language)} *
                </label>
                <input
                  aria-label={t('exchangeConfig.accountName', language)}
                  type="text"
                  value={accountName}
                  onChange={(e) => setAccountName(e.target.value)}
                  placeholder={t(
                    'exchangeConfig.accountNamePlaceholder',
                    language
                  )}
                  className="w-full px-4 py-3 rounded-xl text-base"
                  style={{
                    background: 'var(--surface-2)',
                    border: '1px solid var(--line)',
                    color: 'var(--fg)',
                  }}
                  required
                />
              </div>

              {currentExchangeType === 'paper' && (
                <>
                  <p className="text-xs text-fg-3">
                    {t('paper.description', language)}
                  </p>
                  {(
                    [
                      [
                        'initial_balance',
                        'initialBalance',
                        0,
                        undefined,
                        'any',
                      ],
                      ['taker_fee_bps', 'takerFee', 0, 100, 'any'],
                      ['maker_fee_bps', 'makerFee', 0, 100, 'any'],
                      ['slippage_bps', 'slippage', 0, 100, 'any'],
                      ['default_leverage', 'leverage', 1, 125, 1],
                    ] as const
                  ).map(([field, label, min, max, step]) => (
                    <div className="space-y-2" key={field}>
                      <label
                        htmlFor={`paper-${field}`}
                        className="text-sm font-semibold text-fg"
                      >
                        {t(`paper.${label}`, language)}
                      </label>
                      <input
                        id={`paper-${field}`}
                        type="number"
                        min={min}
                        max={max}
                        step={step}
                        required
                        value={
                          Number.isNaN(paperConfig[field])
                            ? ''
                            : paperConfig[field]
                        }
                        onChange={(e) =>
                          setPaperConfig((previous) => ({
                            ...previous,
                            [field]:
                              e.target.value === ''
                                ? NaN
                                : Number(e.target.value),
                          }))
                        }
                        className="w-full px-4 py-3 rounded-xl bg-surface-2 border border-line text-fg"
                      />
                    </div>
                  ))}
                </>
              )}

              {/* CEX Fields */}
              {(currentExchangeType === 'binance' ||
                currentExchangeType === 'bybit' ||
                currentExchangeType === 'okx' ||
                currentExchangeType === 'bitget' ||
                currentExchangeType === 'gate' ||
                currentExchangeType === 'kucoin' ||
                currentExchangeType === 'indodax') && (
                <>
                  {currentExchangeType === 'binance' && (
                    <div
                      className="p-4 rounded-xl cursor-pointer transition-colors"
                      style={{
                        background: 'var(--info-soft)',
                        border:
                          '1px solid color-mix(in srgb, var(--info) 35%, transparent)',
                      }}
                      onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span style={{ color: 'var(--info)' }}>ℹ️</span>
                          <span
                            className="text-sm font-medium"
                            style={{ color: 'var(--fg)' }}
                          >
                            {t('exchangeConfig.useBinanceFuturesApi', language)}
                          </span>
                        </div>
                        <span style={{ color: 'var(--fg-3)' }}>
                          {showBinanceGuide ? '▲' : '▼'}
                        </span>
                      </div>
                      {showBinanceGuide && (
                        <div
                          className="mt-3 pt-3 text-sm"
                          style={{
                            borderTop: '1px solid var(--line)',
                            color: 'var(--fg-2)',
                          }}
                        >
                          <a
                            href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 hover:underline"
                            style={{ color: 'var(--info)' }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            {t('exchangeConfig.viewTutorial', language)}{' '}
                            <ExternalLink className="w-3 h-3" />
                          </a>
                        </div>
                      )}
                    </div>
                  )}

                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      <Key
                        className="w-4 h-4"
                        style={{ color: 'var(--brand)' }}
                      />
                      {t('apiKey', language)}
                    </label>
                    <input
                      type="password"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={t('enterAPIKey', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      <Shield
                        className="w-4 h-4"
                        style={{ color: 'var(--brand)' }}
                      />
                      {t('secretKey', language)}
                    </label>
                    <input
                      type="password"
                      value={secretKey}
                      onChange={(e) => setSecretKey(e.target.value)}
                      placeholder={t('enterSecretKey', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>

                  {(currentExchangeType === 'okx' ||
                    currentExchangeType === 'bitget' ||
                    currentExchangeType === 'kucoin') && (
                    <div className="space-y-2">
                      <label
                        className="flex items-center gap-2 text-sm font-semibold"
                        style={{ color: 'var(--fg)' }}
                      >
                        <Key
                          className="w-4 h-4"
                          style={{ color: 'var(--brand)' }}
                        />
                        {t('passphrase', language)}
                      </label>
                      <input
                        type="password"
                        value={passphrase}
                        onChange={(e) => setPassphrase(e.target.value)}
                        placeholder={t('enterPassphrase', language)}
                        className="w-full px-4 py-3 rounded-xl"
                        style={{
                          background: 'var(--surface-2)',
                          border: '1px solid var(--line)',
                          color: 'var(--fg)',
                        }}
                        required
                      />
                    </div>
                  )}

                  {currentExchangeType === 'binance' && (
                    <div
                      className="p-4 rounded-xl"
                      style={{
                        background: 'var(--brand-soft)',
                        border:
                          '1px solid color-mix(in srgb, var(--brand) 20%, transparent)',
                      }}
                    >
                      <div
                        className="text-sm font-semibold mb-2"
                        style={{ color: 'var(--brand)' }}
                      >
                        {t('whitelistIP', language)}
                      </div>
                      <div
                        className="text-xs mb-3"
                        style={{ color: 'var(--fg-3)' }}
                      >
                        {t('whitelistIPDesc', language)}
                      </div>
                      {loadingIP ? (
                        <div
                          className="text-xs"
                          style={{ color: 'var(--fg-3)' }}
                        >
                          {t('loadingServerIP', language)}
                        </div>
                      ) : serverIP?.public_ip ? (
                        <div
                          className="flex items-center gap-2 p-3 rounded-lg"
                          style={{ background: 'var(--surface-2)' }}
                        >
                          <code
                            className="flex-1 text-sm font-mono"
                            style={{ color: 'var(--brand)' }}
                          >
                            {serverIP.public_ip}
                          </code>
                          <button
                            type="button"
                            onClick={() => handleCopyIP(serverIP.public_ip)}
                            className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all hover:scale-105"
                            style={{
                              background: 'var(--brand-soft)',
                              color: 'var(--brand)',
                            }}
                          >
                            <Copy className="w-3 h-3" />
                            {copiedIP
                              ? t('ipCopied', language)
                              : t('copyIP', language)}
                          </button>
                        </div>
                      ) : null}
                    </div>
                  )}
                </>
              )}

              {/* Aster Fields */}
              {currentExchangeType === 'aster' && (
                <>
                  <div
                    className="p-4 rounded-xl"
                    style={{
                      background: 'var(--ai-soft)',
                      border:
                        '1px solid color-mix(in srgb, var(--ai) 30%, transparent)',
                    }}
                  >
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div
                          className="text-sm font-semibold mb-1"
                          style={{ color: 'var(--ai)' }}
                        >
                          {t('asterApiProTitle', language)}
                        </div>
                        <div
                          className="text-xs"
                          style={{ color: 'var(--fg-3)' }}
                        >
                          {t('asterApiProDesc', language)}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('asterUserLabel', language)}
                      <Tooltip content={t('asterUserDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: 'var(--ai)' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterUser}
                      onChange={(e) => setAsterUser(e.target.value)}
                      placeholder={t('enterAsterUser', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('asterSignerLabel', language)}
                      <Tooltip content={t('asterSignerDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: 'var(--ai)' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterSigner}
                      onChange={(e) => setAsterSigner(e.target.value)}
                      placeholder={t('enterAsterSigner', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('asterPrivateKeyLabel', language)}
                      <Tooltip content={t('asterPrivateKeyDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: 'var(--ai)' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="password"
                      value={asterPrivateKey}
                      onChange={(e) => setAsterPrivateKey(e.target.value)}
                      placeholder={t('enterAsterPrivateKey', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                </>
              )}

              {/* Hyperliquid Fields */}
              {currentExchangeType === 'hyperliquid' && (
                <>
                  <div
                    className="p-4 rounded-xl"
                    style={{
                      background: 'rgba(127, 231, 204, 0.1)',
                      border: '1px solid rgba(127, 231, 204, 0.3)',
                    }}
                  >
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div
                          className="text-sm font-semibold mb-1"
                          style={{ color: '#7FE7CC' }}
                        >
                          {t('hyperliquidAgentWalletTitle', language)}
                        </div>
                        <div
                          className="text-xs"
                          style={{ color: 'var(--fg-3)' }}
                        >
                          {t('hyperliquidAgentWalletDesc', language)}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label
                      className="text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('hyperliquidAgentPrivateKey', language)}
                    </label>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={maskSecret(apiKey)}
                        readOnly
                        placeholder={t(
                          'enterHyperliquidAgentPrivateKey',
                          language
                        )}
                        className="flex-1 px-4 py-3 rounded-xl"
                        style={{
                          background: 'var(--surface-2)',
                          border: '1px solid var(--line)',
                          color: 'var(--fg)',
                        }}
                      />
                      <button
                        type="button"
                        onClick={() => setSecureInputTarget('hyperliquid')}
                        className="px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:scale-105"
                        style={{ background: '#7FE7CC', color: '#000' }}
                      >
                        {apiKey
                          ? t('secureInputReenter', language)
                          : t('secureInputButton', language)}
                      </button>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label
                      className="text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('hyperliquidMainWalletAddress', language)}
                    </label>
                    <input
                      type="text"
                      value={hyperliquidWalletAddr}
                      onChange={(e) => setHyperliquidWalletAddr(e.target.value)}
                      placeholder={t(
                        'enterHyperliquidMainWalletAddress',
                        language
                      )}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                </>
              )}

              {/* Lighter Fields */}
              {currentExchangeType === 'lighter' && (
                <>
                  <div
                    className="p-4 rounded-xl"
                    style={{
                      background: 'var(--info-soft)',
                      border:
                        '1px solid color-mix(in srgb, var(--info) 30%, transparent)',
                    }}
                  >
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div
                          className="text-sm font-semibold mb-1"
                          style={{ color: 'var(--info)' }}
                        >
                          {t('exchangeConfig.lighterApiKeySetup', language)}
                        </div>
                        <div
                          className="text-xs"
                          style={{ color: 'var(--fg-3)' }}
                        >
                          {t('exchangeConfig.lighterApiKeyDesc', language)}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label
                      className="text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('lighterWalletAddress', language)} *
                    </label>
                    <input
                      type="text"
                      value={lighterWalletAddr}
                      onChange={(e) => setLighterWalletAddr(e.target.value)}
                      placeholder={t('enterLighterWalletAddress', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('lighterApiKeyPrivateKey', language)} *
                      <button
                        type="button"
                        onClick={() => setSecureInputTarget('lighter')}
                        className="text-xs underline"
                        style={{ color: 'var(--info)' }}
                      >
                        {t('secureInputButton', language)}
                      </button>
                    </label>
                    <input
                      type="password"
                      value={lighterApiKeyPrivateKey}
                      onChange={(e) =>
                        setLighterApiKeyPrivateKey(e.target.value)
                      }
                      placeholder={t('enterLighterApiKeyPrivateKey', language)}
                      className="w-full px-4 py-3 rounded-xl font-mono"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      className="flex items-center gap-2 text-sm font-semibold"
                      style={{ color: 'var(--fg)' }}
                    >
                      {t('exchangeConfig.apiKeyIndex', language)}
                      <Tooltip
                        content={t(
                          'exchangeConfig.apiKeyIndexTooltip',
                          language
                        )}
                      >
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: 'var(--info)' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="number"
                      min={0}
                      max={255}
                      value={lighterApiKeyIndex}
                      onChange={(e) =>
                        setLighterApiKeyIndex(parseInt(e.target.value) || 0)
                      }
                      className="w-full px-4 py-3 rounded-xl"
                      style={{
                        background: 'var(--surface-2)',
                        border: '1px solid var(--line)',
                        color: 'var(--fg)',
                      }}
                    />
                  </div>
                </>
              )}

              {/* Buttons */}
              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={handleBack}
                  className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-fg/5"
                  style={{ background: 'var(--line)', color: 'var(--fg-3)' }}
                >
                  {editingExchangeId
                    ? t('cancel', language)
                    : t('exchangeConfig.back', language)}
                </button>
                <button
                  type="submit"
                  disabled={isSaving || !accountName.trim()}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{
                    background: 'var(--brand)',
                    color: 'var(--brand-fg)',
                  }}
                >
                  {isSaving ? (
                    t('saving', language)
                  ) : (
                    <>
                      {t('saveConfig', language)}{' '}
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>

      {/* Binance Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-surface-2/75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="rounded-2xl p-6 w-full max-w-4xl"
            style={{ background: 'var(--surface-hover)' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: 'var(--fg)' }}
              >
                <BookOpen
                  className="w-6 h-6"
                  style={{ color: 'var(--brand)' }}
                />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded-lg text-sm font-semibold"
                style={{ background: 'var(--line)', color: 'var(--fg-3)' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded-lg"
              />
            </div>
          </div>
        </div>
      )}

      {/* Secure Input Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={() => setSecureInputTarget(null)}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
