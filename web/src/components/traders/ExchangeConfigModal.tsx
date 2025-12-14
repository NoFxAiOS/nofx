import React, { useState, useEffect } from 'react'
import type { Exchange } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import { getExchangeIcon } from '../ExchangeIcons'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from '../TwoStageKeyModal'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../WebCryptoEnvironmentCheck'
import { BookOpen, Trash2, HelpCircle, ExternalLink, UserPlus } from 'lucide-react'
import { toast } from 'sonner'
import { Tooltip } from './Tooltip'
import { getShortName } from './utils'

// Supported exchange templates for creating new accounts
const SUPPORTED_EXCHANGE_TEMPLATES = [
  { exchange_type: 'binance', name: 'Binance Futures', type: 'cex' as const },
  { exchange_type: 'bybit', name: 'Bybit Futures', type: 'cex' as const },
  { exchange_type: 'okx', name: 'OKX Futures', type: 'cex' as const },
  { exchange_type: 'bitget', name: 'Bitget Futures', type: 'cex' as const },
  { exchange_type: 'hyperliquid', name: 'Hyperliquid', type: 'dex' as const },
  { exchange_type: 'aster', name: 'Aster DEX', type: 'dex' as const },
  { exchange_type: 'lighter', name: 'Lighter', type: 'dex' as const },
]

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string | null, // null for creating new account
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string, // OKX专用
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

export function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  // Selected exchange type for creating new accounts
  const [selectedExchangeType, setSelectedExchangeType] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
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

  // 币安配置指南展开状态
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster 特定字段
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid 特定字段
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // LIGHTER 特定字段
  const [lighterWalletAddr, setLighterWalletAddr] = useState('')
  const [lighterApiKeyPrivateKey, setLighterApiKeyPrivateKey] = useState('')
  const [lighterApiKeyIndex, setLighterApiKeyIndex] = useState(0)

  // 安全输入状态
  const [secureInputTarget, setSecureInputTarget] = useState<
    null | 'hyperliquid' | 'aster' | 'lighter'
  >(null)

  // 保存中状态
  const [isSaving, setIsSaving] = useState(false)

  // 账户名称
  const [accountName, setAccountName] = useState('')

  // 获取当前编辑的交易所信息或模板
  // For editing: find the existing account by id (UUID)
  // For creating: use the selected exchange template
  const selectedExchange = editingExchangeId
    ? allExchanges?.find((e) => e.id === editingExchangeId)
    : null

  // Get the exchange template for displaying UI fields
  const selectedTemplate = editingExchangeId
    ? SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchange?.exchange_type)
    : SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchangeType)

  // Get the current exchange type (from existing account or selected template)
  const currentExchangeType = editingExchangeId
    ? selectedExchange?.exchange_type
    : selectedExchangeType

  // 交易所注册链接配置
  const exchangeRegistrationLinks: Record<string, { url: string; hasReferral?: boolean }> = {
    binance: { url: 'https://www.binance.com/join?ref=NOFXENG', hasReferral: true },
    okx: { url: 'https://www.okx.com/join/1865360', hasReferral: true },
    bybit: { url: 'https://partner.bybit.com/b/83856', hasReferral: true },
    bitget: { url: 'https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172', hasReferral: true },
    hyperliquid: { url: 'https://app.hyperliquid.xyz/join/AITRADING', hasReferral: true },
    aster: { url: 'https://www.asterdex.com/en/referral/fdfc0e', hasReferral: true },
    lighter: { url: 'https://app.lighter.xyz/?referral=68151432', hasReferral: true },
  }

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setAccountName(selectedExchange.account_name || '')
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('') // Don't load existing passphrase for security
      setTestnet(selectedExchange.testnet || false)

      // Aster 字段
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('') // Don't load existing private key for security

      // Hyperliquid 字段
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')

      // LIGHTER 字段
      setLighterWalletAddr(selectedExchange.lighterWalletAddr || '')
      setLighterApiKeyPrivateKey('') // Don't load existing API key for security
      setLighterApiKeyIndex(selectedExchange.lighterApiKeyIndex || 0)
    }
  }, [editingExchangeId, selectedExchange])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (currentExchangeType === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [currentExchangeType])

  const handleCopyIP = async (ip: string) => {
    try {
      // 优先使用现代 Clipboard API
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        // 降级方案: 使用传统的 execCommand 方法
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()

        try {
          const successful = document.execCommand('copy')
          if (successful) {
            setCopiedIP(true)
            setTimeout(() => setCopiedIP(false), 2000)
            toast.success(t('ipCopied', language))
          } else {
            throw new Error('复制命令执行失败')
          }
        } finally {
          document.body.removeChild(textArea)
        }
      }
    } catch (err) {
      console.error('复制失败:', err)
      // 显示错误提示
      toast.error(
        t('copyIPFailed', language) || `复制失败: ${ip}\n请手动复制此IP地址`
      )
    }
  }

  // 安全输入处理函数
  const secureInputContextLabel =
    secureInputTarget === 'aster'
      ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid'
        ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputCancel = () => {
    setSecureInputTarget(null)
  }

  const handleSecureInputComplete = ({
    value,
    obfuscationLog,
  }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') {
      setApiKey(trimmed)
    }
    if (secureInputTarget === 'aster') {
      setAsterPrivateKey(trimmed)
    }
    if (secureInputTarget === 'lighter') {
      setLighterApiKeyPrivateKey(trimmed)
      toast.success(t('lighterApiKeyImported', language))
    }
    // 仅在开发环境输出调试信息
    if (import.meta.env.DEV) {
      console.log('Secure input obfuscation log:', obfuscationLog)
    }
    setSecureInputTarget(null)
  }

  // 掩盖敏感数据显示
  const maskSecret = (secret: string) => {
    if (!secret || secret.length === 0) return ''
    if (secret.length <= 8) return '*'.repeat(secret.length)
    return (
      secret.slice(0, 4) +
      '*'.repeat(Math.max(secret.length - 8, 4)) +
      secret.slice(-4)
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSaving) return

    // For creating, we need the exchange type
    if (!editingExchangeId && !selectedExchangeType) return

    // Validate account name
    const trimmedAccountName = accountName.trim()
    if (!trimmedAccountName) {
      toast.error(language === 'zh' ? '请输入账户名称' : 'Please enter account name')
      return
    }

    const exchangeId = editingExchangeId || null
    const exchangeType = currentExchangeType || ''

    setIsSaving(true)
    try {
      // 根据交易所类型验证不同字段
      if (currentExchangeType === 'binance') {
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), secretKey.trim(), '', testnet)
      } else if (currentExchangeType === 'okx') {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), secretKey.trim(), passphrase.trim(), testnet)
      } else if (currentExchangeType === 'bitget') {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), secretKey.trim(), passphrase.trim(), testnet)
      } else if (currentExchangeType === 'hyperliquid') {
        if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return // 验证私钥和钱包地址
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
          '', // apiKey not used for Lighter
          '',
          '',
          testnet,
          undefined, // hyperliquidWalletAddr
          undefined, // asterUser
          undefined, // asterSigner
          undefined, // asterPrivateKey
          lighterWalletAddr.trim(),
          '', // lighterPrivateKey (L1) no longer needed
          lighterApiKeyPrivateKey.trim(),
          lighterApiKeyIndex
        )
      } else {
        // 默认情况（其他CEX交易所）
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), secretKey.trim(), '', testnet)
      }
    } finally {
      setIsSaving(false)
    }
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
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: '#1E2329' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {currentExchangeType === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  color: '#F0B90B',
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
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('delete', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingExchangeId && (
              <div className="space-y-3">
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('environmentSteps.checkTitle', language)}
                  </div>
                  <WebCryptoEnvironmentCheck
                    language={language}
                    variant="card"
                    onStatusChange={setWebCryptoStatus}
                  />
                </div>
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: '#F0B90B' }}
                  >
                    {t('environmentSteps.selectTitle', language)}
                  </div>
                  <select
                    value={selectedExchangeType}
                    onChange={(e) => setSelectedExchangeType(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    aria-label={t('selectExchange', language)}
                    disabled={
                      webCryptoStatus !== 'secure' &&
                      webCryptoStatus !== 'disabled'
                    }
                    required
                  >
                    <option value="">
                      {t('pleaseSelectExchange', language)}
                    </option>
                    {SUPPORTED_EXCHANGE_TEMPLATES.map((template) => (
                      <option key={template.exchange_type} value={template.exchange_type}>
                        {getShortName(template.name)} (
                        {template.type.toUpperCase()})
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            {selectedTemplate && (
              <div
                className="p-4 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getExchangeIcon(selectedTemplate.exchange_type, {
                      width: 32,
                      height: 32,
                    })}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedTemplate.name)}
                      {editingExchangeId && selectedExchange?.account_name && (
                        <span className="text-sm font-normal ml-2" style={{ color: '#848E9C' }}>
                          - {selectedExchange.account_name}
                        </span>
                      )}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedTemplate.type.toUpperCase()} •{' '}
                      {selectedTemplate.exchange_type}
                    </div>
                  </div>
                </div>

                {/* 账户名称输入 */}
                <div className="mt-3">
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {language === 'zh' ? '账户名称' : 'Account Name'} *
                  </label>
                  <input
                    type="text"
                    value={accountName}
                    onChange={(e) => setAccountName(e.target.value)}
                    placeholder={language === 'zh' ? '例如：主账户、套利账户' : 'e.g., Main Account, Arbitrage Account'}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    required
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {language === 'zh'
                      ? '为此账户设置一个易于识别的名称，以便区分同一交易所的多个账户'
                      : 'Set an easily recognizable name for this account to distinguish multiple accounts on the same exchange'}
                  </div>
                </div>

                {/* 注册链接 */}
                <a
                  href={exchangeRegistrationLinks[currentExchangeType || '']?.url || '#'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center justify-between p-3 rounded-lg mt-3 transition-all hover:scale-[1.02]"
                  style={{
                    background: 'rgba(240, 185, 11, 0.08)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                >
                  <div className="flex items-center gap-2">
                    <UserPlus className="w-4 h-4" style={{ color: '#F0B90B' }} />
                    <span className="text-sm" style={{ color: '#EAECEF' }}>
                      {language === 'zh' ? '还没有交易所账号？点击注册' : "No exchange account? Register here"}
                    </span>
                    {exchangeRegistrationLinks[currentExchangeType || '']?.hasReferral && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded"
                        style={{ background: 'rgba(14, 203, 129, 0.2)', color: '#0ECB81' }}
                      >
                        {language === 'zh' ? '折扣优惠' : 'Discount'}
                      </span>
                    )}
                  </div>
                  <ExternalLink className="w-4 h-4" style={{ color: '#848E9C' }} />
                </a>
              </div>
            )}

            {selectedTemplate && (
              <>
                {/* Binance/Bybit/OKX/Bitget 的输入字段 */}
                {(currentExchangeType === 'binance' ||
                  currentExchangeType === 'bybit' ||
                  currentExchangeType === 'okx' ||
                  currentExchangeType === 'bitget') && (
                    <>
                      {/* 币安用户配置提示 (D1 方案) */}
                      {currentExchangeType === 'binance' && (
                        <div
                          className="mb-4 p-3 rounded cursor-pointer transition-colors"
                          style={{
                            background: '#1a3a52',
                            border: '1px solid #2b5278',
                          }}
                          onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span style={{ color: '#58a6ff' }}>ℹ️</span>
                              <span
                                className="text-sm font-medium"
                                style={{ color: '#EAECEF' }}
                              >
                                <strong>币安用户必读：</strong>
                                使用「现货与合约交易」API，不要用「统一账户
                                API」
                              </span>
                            </div>
                            <span style={{ color: '#8b949e' }}>
                              {showBinanceGuide ? '▲' : '▼'}
                            </span>
                          </div>

                          {/* 展开的详细说明 */}
                          {showBinanceGuide && (
                            <div
                              className="mt-3 pt-3"
                              style={{
                                borderTop: '1px solid #2b5278',
                                fontSize: '0.875rem',
                                color: '#c9d1d9',
                              }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <p className="mb-2" style={{ color: '#8b949e' }}>
                                <strong>原因：</strong>统一账户 API
                                权限结构不同，会导致订单提交失败
                              </p>

                              <p
                                className="font-semibold mb-1"
                                style={{ color: '#EAECEF' }}
                              >
                                正确配置步骤：
                              </p>
                              <ol
                                className="list-decimal list-inside space-y-1 mb-3"
                                style={{ paddingLeft: '0.5rem' }}
                              >
                                <li>
                                  登录币安 → 个人中心 →{' '}
                                  <strong>API 管理</strong>
                                </li>
                                <li>
                                  创建 API → 选择「
                                  <strong>系统生成的 API 密钥</strong>」
                                </li>
                                <li>
                                  勾选「<strong>现货与合约交易</strong>」（
                                  <span style={{ color: '#f85149' }}>
                                    不选统一账户
                                  </span>
                                  ）
                                </li>
                                <li>
                                  IP 限制选「<strong>无限制</strong>
                                  」或添加服务器 IP
                                </li>
                              </ol>

                              <p
                                className="mb-2 p-2 rounded"
                                style={{
                                  background: '#3d2a00',
                                  border: '1px solid #9e6a03',
                                }}
                              >
                                💡 <strong>多资产模式用户注意：</strong>
                                如果您开启了多资产模式，将强制使用全仓模式。建议关闭多资产模式以支持逐仓交易。
                              </p>

                              <a
                                href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-block text-sm hover:underline"
                                style={{ color: '#58a6ff' }}
                              >
                                📖 查看币安官方教程 ↗
                              </a>
                            </div>
                          )}
                        </div>
                      )}

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('apiKey', language)}
                        </label>
                        <input
                          type="password"
                          value={apiKey}
                          onChange={(e) => setApiKey(e.target.value)}
                          placeholder={t('enterAPIKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('secretKey', language)}
                        </label>
                        <input
                          type="password"
                          value={secretKey}
                          onChange={(e) => setSecretKey(e.target.value)}
                          placeholder={t('enterSecretKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      {(currentExchangeType === 'okx' || currentExchangeType === 'bitget') && (
                        <div>
                          <label
                            className="block text-sm font-semibold mb-2"
                            style={{ color: '#EAECEF' }}
                          >
                            {t('passphrase', language)}
                          </label>
                          <input
                            type="password"
                            value={passphrase}
                            onChange={(e) => setPassphrase(e.target.value)}
                            placeholder={t('enterPassphrase', language)}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                            required
                          />
                        </div>
                      )}

                      {/* Binance 白名单IP提示 */}
                      {currentExchangeType === 'binance' && (
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
                            {t('whitelistIP', language)}
                          </div>
                          <div
                            className="text-xs mb-3"
                            style={{ color: '#848E9C' }}
                          >
                            {t('whitelistIPDesc', language)}
                          </div>

                          {loadingIP ? (
                            <div
                              className="text-xs"
                              style={{ color: '#848E9C' }}
                            >
                              {t('loadingServerIP', language)}
                            </div>
                          ) : serverIP && serverIP.public_ip ? (
                            <div
                              className="flex items-center gap-2 p-2 rounded"
                              style={{ background: '#0B0E11' }}
                            >
                              <code
                                className="flex-1 text-sm font-mono"
                                style={{ color: '#F0B90B' }}
                              >
                                {serverIP.public_ip}
                              </code>
                              <button
                                type="button"
                                onClick={() => handleCopyIP(serverIP.public_ip)}
                                className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                                style={{
                                  background: 'rgba(240, 185, 11, 0.2)',
                                  color: '#F0B90B',
                                }}
                              >
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

                {/* Aster 交易所的字段 */}
                {currentExchangeType === 'aster' && (
                  <>
                    {/* API Pro 代理钱包说明 banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(240, 185, 11, 0.1)',
                        border: '1px solid rgba(240, 185, 11, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: '#F0B90B', fontSize: '16px' }}>
                          🔐
                        </span>
                        <div className="flex-1">
                          <div
                            className="text-sm font-semibold mb-1"
                            style={{ color: '#F0B90B' }}
                          >
                            {t('asterApiProTitle', language)}
                          </div>
                          <div
                            className="text-xs"
                            style={{ color: '#848E9C', lineHeight: '1.5' }}
                          >
                            {t('asterApiProDesc', language)}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* 主钱包地址 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('asterUserLabel', language)}
                        <Tooltip content={t('asterUserDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterUser}
                        onChange={(e) => setAsterUser(e.target.value)}
                        placeholder={t('enterAsterUser', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('asterUserDesc', language)}
                      </div>
                    </div>

                    {/* API Pro 代理钱包地址 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('asterSignerLabel', language)}
                        <Tooltip content={t('asterSignerDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterSigner}
                        onChange={(e) => setAsterSigner(e.target.value)}
                        placeholder={t('enterAsterSigner', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('asterSignerDesc', language)}
                      </div>
                    </div>

                    {/* API Pro 代理钱包私钥 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('asterPrivateKeyLabel', language)}
                        <Tooltip content={t('asterPrivateKeyDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="password"
                        value={asterPrivateKey}
                        onChange={(e) => setAsterPrivateKey(e.target.value)}
                        placeholder={t('enterAsterPrivateKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('asterPrivateKeyDesc', language)}
                      </div>
                    </div>
                  </>
                )}

                {/* Hyperliquid 交易所的字段 */}
                {currentExchangeType === 'hyperliquid' && (
                  <>
                    {/* 安全提示 banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(240, 185, 11, 0.1)',
                        border: '1px solid rgba(240, 185, 11, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: '#F0B90B', fontSize: '16px' }}>
                          🔐
                        </span>
                        <div className="flex-1">
                          <div
                            className="text-sm font-semibold mb-1"
                            style={{ color: '#F0B90B' }}
                          >
                            {t('hyperliquidAgentWalletTitle', language)}
                          </div>
                          <div
                            className="text-xs"
                            style={{ color: '#848E9C', lineHeight: '1.5' }}
                          >
                            {t('hyperliquidAgentWalletDesc', language)}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Agent Private Key 字段 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidAgentPrivateKey', language)}
                      </label>
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={maskSecret(apiKey)}
                            readOnly
                            placeholder={t(
                              'enterHyperliquidAgentPrivateKey',
                              language
                            )}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: '#0B0E11',
                              border: '1px solid #2B3139',
                              color: '#EAECEF',
                            }}
                          />
                          <button
                            type="button"
                            onClick={() => setSecureInputTarget('hyperliquid')}
                            className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                            style={{
                              background: '#F0B90B',
                              color: '#000',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {apiKey
                              ? t('secureInputReenter', language)
                              : t('secureInputButton', language)}
                          </button>
                          {apiKey && (
                            <button
                              type="button"
                              onClick={() => setApiKey('')}
                              className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: '#1B1F2B',
                                color: '#848E9C',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {t('secureInputClear', language)}
                            </button>
                          )}
                        </div>
                        {apiKey && (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('secureInputHint', language)}
                          </div>
                        )}
                      </div>
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidAgentPrivateKeyDesc', language)}
                      </div>
                    </div>

                    {/* Main Wallet Address 字段 */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidMainWalletAddress', language)}
                      </label>
                      <input
                        type="text"
                        value={hyperliquidWalletAddr}
                        onChange={(e) =>
                          setHyperliquidWalletAddr(e.target.value)
                        }
                        placeholder={t(
                          'enterHyperliquidMainWalletAddress',
                          language
                        )}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidMainWalletAddressDesc', language)}
                      </div>
                    </div>
                  </>
                )}

                {/* LIGHTER 特定配置 */}
                {currentExchangeType === 'lighter' && (
                  <>
                    {/* Info banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(240, 185, 11, 0.1)',
                        border: '1px solid rgba(240, 185, 11, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: '#F0B90B', fontSize: '16px' }}>🔐</span>
                        <div className="flex-1">
                          <div className="text-sm font-semibold mb-1" style={{ color: '#F0B90B' }}>
                            {language === 'zh' ? 'Lighter API Key 配置' : 'Lighter API Key Setup'}
                          </div>
                          <div className="text-xs" style={{ color: '#848E9C', lineHeight: '1.5' }}>
                            {language === 'zh'
                              ? '请在 Lighter 网站生成 API Key，然后填写钱包地址、API Key 私钥和索引。'
                              : 'Generate an API Key on the Lighter website, then enter your wallet address, API Key private key, and index.'}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* L1 Wallet Address */}
                    <div className="mb-4">
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('lighterWalletAddress', language)} *
                      </label>
                      <input
                        type="text"
                        value={lighterWalletAddr}
                        onChange={(e) => setLighterWalletAddr(e.target.value)}
                        placeholder={t('enterLighterWalletAddress', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                        {t('lighterWalletAddressDesc', language)}
                      </div>
                    </div>

                    {/* API Key Private Key */}
                    <div className="mb-4">
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('lighterApiKeyPrivateKey', language)} *
                        <button
                          type="button"
                          onClick={() => setSecureInputTarget('lighter')}
                          className="ml-2 text-xs underline"
                          style={{ color: '#F0B90B' }}
                        >
                          {t('secureInputButton', language)}
                        </button>
                      </label>
                      <input
                        type="password"
                        value={lighterApiKeyPrivateKey}
                        onChange={(e) => setLighterApiKeyPrivateKey(e.target.value)}
                        placeholder={t('enterLighterApiKeyPrivateKey', language)}
                        className="w-full px-3 py-2 rounded font-mono text-sm"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                        {t('lighterApiKeyPrivateKeyDesc', language)}
                      </div>
                    </div>

                    {/* API Key Index */}
                    <div className="mb-4">
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {language === 'zh' ? 'API Key 索引' : 'API Key Index'}
                        <Tooltip content={
                          language === 'zh'
                            ? 'Lighter 允许每个账户创建多个 API Key（最多256个）。索引值对应您创建的第几个 API Key，从0开始计数。如果您只创建了一个 API Key，请使用默认值 0。'
                            : 'Lighter allows creating multiple API Keys per account (up to 256). The index corresponds to which API Key you created, starting from 0. If you only created one API Key, use the default value 0.'
                        }>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: '#F0B90B' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="number"
                        min={0}
                        max={255}
                        value={lighterApiKeyIndex}
                        onChange={(e) => setLighterApiKeyIndex(parseInt(e.target.value) || 0)}
                        placeholder="0"
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                      />
                      <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                        {language === 'zh'
                          ? '默认为 0。如果您在 Lighter 创建了多个 API Key，请填写对应的索引号（0-255）。'
                          : 'Default is 0. If you created multiple API Keys on Lighter, enter the corresponding index (0-255).'}
                      </div>
                    </div>
                  </>
                )}
              </>
            )}
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
              disabled={
                isSaving ||
                !selectedTemplate ||
                !accountName.trim() ||
                (currentExchangeType === 'binance' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (currentExchangeType === 'okx' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (currentExchangeType === 'bitget' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (currentExchangeType === 'hyperliquid' &&
                  (!apiKey.trim() || !hyperliquidWalletAddr.trim())) || // 验证私钥和钱包地址
                (currentExchangeType === 'aster' &&
                  (!asterUser.trim() ||
                    !asterSigner.trim() ||
                    !asterPrivateKey.trim())) ||
                (currentExchangeType === 'lighter' &&
                  (!lighterWalletAddr.trim() || !lighterApiKeyPrivateKey.trim())) ||
                (currentExchangeType === 'bybit' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (selectedTemplate?.type === 'cex' &&
                  currentExchangeType !== 'hyperliquid' &&
                  currentExchangeType !== 'aster' &&
                  currentExchangeType !== 'lighter' &&
                  currentExchangeType !== 'binance' &&
                  currentExchangeType !== 'bybit' &&
                  currentExchangeType !== 'okx' &&
                  currentExchangeType !== 'bitget' &&
                  (!apiKey.trim() || !secretKey.trim()))
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {isSaving ? t('saving', language) || '保存中...' : t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: '#1E2329' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: '#F0B90B' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#2B3139', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded"
              />
            </div>
          </div>
        </div>
      )}

      {/* Two Stage Key Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={handleSecureInputCancel}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
