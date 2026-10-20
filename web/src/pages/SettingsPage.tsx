import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import {
  User,
  Cpu,
  Building2,
  MessageCircle,
  Eye,
  EyeOff,
  ChevronRight,
  Plus,
  Pencil,
} from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import {
  getPostAuthPath,
  getUserMode,
  setUserMode,
  type UserMode,
} from '../lib/onboarding'
import { ExchangeConfigModal } from '../components/trader/ExchangeConfigModal'
import { TelegramConfigModal } from '../components/trader/TelegramConfigModal'
import { ModelConfigModal } from '../components/trader/ModelConfigModal'
import type { Exchange, AIModel, PaperConfig } from '../types'
import { Badge, Button, Card, Input, Tabs } from '../components/ui'

import { PaperBadge } from '../components/trader/PaperBadge'

type Tab = 'account' | 'models' | 'exchanges' | 'telegram'

export function SettingsPage() {
  const { user, token, logout } = useAuth()
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<Tab>('account')
  const [userMode, setUserModeState] = useState<UserMode>(
    () => getUserMode() ?? 'advanced'
  )

  // Account state
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [changingPassword, setChangingPassword] = useState(false)

  // AI Models state
  const [configuredModels, setConfiguredModels] = useState<AIModel[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [showModelModal, setShowModelModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)

  // Exchanges state
  const [exchanges, setExchanges] = useState<Exchange[]>([])
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)

  // Telegram state
  const [showTelegramModal, setShowTelegramModal] = useState(false)

  // Fetch data when tabs are visited
  useEffect(() => {
    if (activeTab === 'models') {
      Promise.all([api.getModelConfigs(), api.getSupportedModels()])
        .then(([configs, supported]) => {
          setConfiguredModels(configs)
          setSupportedModels(supported)
        })
        .catch(() => toast.error('Failed to load AI models'))
    }
    if (activeTab === 'exchanges') {
      api
        .getExchangeConfigs()
        .then(setExchanges)
        .catch(() => toast.error('Failed to load exchanges'))
    }
  }, [activeTab])

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }
    setChangingPassword(true)
    try {
      const res = await fetch('/api/user/password', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token || ''}`,
        },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to update password')
      }
      toast.success('Password updated. Please sign in again.')
      setCurrentPassword('')
      logout()
      setNewPassword('')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to update password'
      )
    } finally {
      setChangingPassword(false)
    }
  }

  const handleSwitchMode = (nextMode: UserMode) => {
    if (nextMode === userMode) {
      return
    }

    setUserMode(nextMode)
    setUserModeState(nextMode)
    toast.success(
      language === 'zh'
        ? `已切换到${nextMode === 'beginner' ? '新手模式' : '老手模式'}`
        : nextMode === 'beginner'
          ? 'Switched to beginner mode'
          : 'Switched to advanced mode'
    )

    const nextPath = getPostAuthPath(nextMode)
    window.history.pushState({}, '', nextPath)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  const handleSaveModel = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      const existingModel = configuredModels.find((m) => m.id === modelId)
      const modelTemplate = supportedModels.find((m) => m.id === modelId)
      const modelToUpdate = existingModel || modelTemplate
      if (!modelToUpdate) {
        toast.error('Model not found')
        return
      }

      let updatedModels: AIModel[]
      if (existingModel) {
        updatedModels = configuredModels.map((m) =>
          m.id === modelId
            ? {
                ...m,
                apiKey,
                customApiUrl: customApiUrl || '',
                customModelName: customModelName || '',
                enabled: true,
              }
            : m
        )
      } else {
        updatedModels = [
          ...configuredModels,
          {
            ...modelToUpdate,
            apiKey,
            customApiUrl: customApiUrl || '',
            customModelName: customModelName || '',
            enabled: true,
          },
        ]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [
            m.id,
            {
              enabled: m.enabled,
              api_key: m.apiKey || '',
              custom_api_url: m.customApiUrl || '',
              custom_model_name: m.customModelName || '',
            },
          ])
        ),
      }
      await api.updateModelConfigs(request)
      toast.success('Model config saved')
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
    } catch {
      toast.error('Failed to save model config')
    }
  }

  const handleDeleteModel = async (modelId: string) => {
    try {
      const updatedModels = configuredModels.map((m) =>
        m.id === modelId
          ? {
              ...m,
              apiKey: '',
              customApiUrl: '',
              customModelName: '',
              enabled: false,
            }
          : m
      )
      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [
            m.id,
            {
              enabled: m.enabled,
              api_key: m.apiKey || '',
              clear_api_key: m.id === modelId,
              custom_api_url: m.customApiUrl || '',
              custom_model_name: m.customModelName || '',
            },
          ])
        ),
      }
      await api.updateModelConfigs(request)
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
      toast.success('Model config removed')
    } catch {
      toast.error('Failed to remove model config')
    }
  }

  const handleSaveExchange = async (
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
  ) => {
    try {
      if (exchangeId) {
        const request = {
          exchanges: {
            [exchangeId]: {
              paper_config: paperConfig,
              account_name: accountName,
              enabled: true,
              api_key: apiKey || '',
              secret_key: secretKey || '',
              passphrase: passphrase || '',
              testnet: testnet || false,
              hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
              aster_user: asterUser || '',
              aster_signer: asterSigner || '',
              aster_private_key: asterPrivateKey || '',
              lighter_wallet_addr: lighterWalletAddr || '',
              lighter_private_key: lighterPrivateKey || '',
              lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
              lighter_api_key_index: lighterApiKeyIndex || 0,
            },
          },
        }
        await api.updateExchangeConfigsEncrypted(request)
        toast.success('Exchange config updated')
      } else {
        const createRequest = {
          exchange_type: exchangeType,
          paper_config: paperConfig,
          account_name: accountName,
          enabled: true,
          api_key: apiKey || '',
          secret_key: secretKey || '',
          passphrase: passphrase || '',
          testnet: testnet || false,
          hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
          aster_user: asterUser || '',
          aster_signer: asterSigner || '',
          aster_private_key: asterPrivateKey || '',
          lighter_wallet_addr: lighterWalletAddr || '',
          lighter_private_key: lighterPrivateKey || '',
          lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
          lighter_api_key_index: lighterApiKeyIndex || 0,
        }
        await api.createExchangeEncrypted(createRequest)
        toast.success('Exchange account created')
      }
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error('Failed to save exchange config')
    }
  }

  const handleDeleteExchange = async (exchangeId: string) => {
    try {
      await api.deleteExchange(exchangeId)
      toast.success('Exchange account deleted')
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error('Failed to delete exchange account')
    }
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'account', label: 'Account', icon: <User size={14} /> },
    { key: 'models', label: 'AI Models', icon: <Cpu size={14} /> },
    { key: 'exchanges', label: 'Exchanges', icon: <Building2 size={14} /> },
    { key: 'telegram', label: 'Telegram', icon: <MessageCircle size={14} /> },
  ]

  return (
    <div className="min-h-screen bg-bg px-4 pb-10 pt-20">
      <div className="mx-auto max-w-2xl">
        <h1 className="mb-4 text-xl font-semibold text-fg">Settings</h1>

        {/* Tabs */}
        <Tabs
          className="mb-4"
          items={tabs.map((tab) => ({
            key: tab.key,
            icon: tab.icon,
            label: <span className="hidden sm:inline">{tab.label}</span>,
          }))}
          value={activeTab}
          onChange={(k) => setActiveTab(k)}
        />

        {/* Tab Content */}
        <Card className="p-4">
          {/* Account Tab */}
          {activeTab === 'account' && (
            <div className="space-y-4">
              <div>
                <p className="mb-0.5 text-xs text-fg-3">Email</p>
                <p className="text-sm text-fg font-medium">{user?.email}</p>
              </div>

              <div className="border-t border-line pt-4">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-semibold text-fg">
                      {language === 'zh' ? '使用模式' : 'Usage Mode'}
                    </h3>
                    <p className="mt-1 text-xs text-fg-3">
                      {language === 'zh'
                        ? '新手模式会显示钱包引导和 4 步卡片；老手模式保持原来的专业界面。'
                        : 'Beginner mode shows wallet onboarding and quickstart cards. Advanced mode keeps the original pro workflow.'}
                    </p>
                  </div>
                  <Badge variant="brand" className="shrink-0">
                    {userMode === 'beginner'
                      ? language === 'zh'
                        ? '当前：新手模式'
                        : 'Current: Beginner'
                      : language === 'zh'
                        ? '当前：老手模式'
                        : 'Current: Advanced'}
                  </Badge>
                </div>

                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  <button
                    type="button"
                    onClick={() => handleSwitchMode('beginner')}
                    className={`rounded-lg border px-3 py-3 text-left transition-colors ${
                      userMode === 'beginner'
                        ? 'border-brand bg-brand-soft'
                        : 'border-line bg-surface-2 hover:border-line-strong'
                    }`}
                  >
                    <div className="text-sm font-semibold text-fg">
                      {language === 'zh' ? '新手模式' : 'Beginner Mode'}
                    </div>
                    <div className="mt-1 text-xs text-fg-3">
                      {language === 'zh'
                        ? '更简单，优先显示钱包、充值和快速上手引导。'
                        : 'Simpler flow with wallet, funding, and quickstart guidance first.'}
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => handleSwitchMode('advanced')}
                    className={`rounded-lg border px-3 py-3 text-left transition-colors ${
                      userMode === 'advanced'
                        ? 'border-brand bg-brand-soft'
                        : 'border-line bg-surface-2 hover:border-line-strong'
                    }`}
                  >
                    <div className="text-sm font-semibold text-fg">
                      {language === 'zh' ? '老手模式' : 'Advanced Mode'}
                    </div>
                    <div className="mt-1 text-xs text-fg-3">
                      {language === 'zh'
                        ? '保持原来的配置与交易流程，不展示新手引导。'
                        : 'Keeps the original configuration and trading workflow without beginner hints.'}
                    </div>
                  </button>
                </div>
              </div>

              <div className="border-t border-line pt-4">
                <h3 className="mb-3 text-sm font-semibold text-fg">
                  Change Password
                </h3>
                <form onSubmit={handleChangePassword} className="space-y-3">
                  <label className="block text-xs font-medium text-fg-3">
                    Current Password
                    <Input
                      type="password"
                      autoComplete="current-password"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      required
                      className="mt-1"
                    />
                  </label>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-fg-3">
                      New Password
                    </label>
                    <div className="relative">
                      <Input
                        type={showPassword ? 'text' : 'password'}
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="pr-9"
                        placeholder="At least 8 characters"
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-fg-3 transition-colors hover:text-fg-2"
                      >
                        {showPassword ? (
                          <EyeOff size={16} />
                        ) : (
                          <Eye size={16} />
                        )}
                      </button>
                    </div>
                  </div>
                  <Button
                    type="submit"
                    variant="primary"
                    disabled={
                      changingPassword ||
                      !currentPassword ||
                      newPassword.length < 8
                    }
                    className="w-full"
                  >
                    {changingPassword ? 'Updating...' : 'Update Password'}
                  </Button>
                </form>
              </div>
            </div>
          )}

          {/* AI Models Tab */}
          {activeTab === 'models' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-[13px] text-fg-3">
                  {configuredModels.length} model
                  {configuredModels.length !== 1 ? 's' : ''} configured
                </p>
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingModel(null)
                    setShowModelModal(true)
                  }}
                  className="text-brand"
                >
                  <Plus size={14} />
                  Add Model
                </Button>
              </div>

              {configuredModels.length === 0 ? (
                <div className="py-8 text-center text-[13px] text-fg-3">
                  No AI models configured yet
                </div>
              ) : (
                <div className="space-y-2">
                  {configuredModels.map((model) => (
                    <button
                      key={model.id}
                      onClick={() => {
                        setEditingModel(model.id)
                        setShowModelModal(true)
                      }}
                      className="group flex w-full items-center justify-between rounded-md border border-line bg-surface-2 px-3 py-2 transition-colors hover:border-line-strong hover:bg-surface-hover"
                    >
                      <div className="flex items-center gap-3">
                        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-surface-hover">
                          <Cpu size={14} className="text-fg-2" />
                        </div>
                        <div className="text-left">
                          <p className="text-[13px] font-medium text-fg">
                            {model.name}
                          </p>
                          <p className="text-xs text-fg-3">{model.provider}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant={model.enabled ? 'up' : 'neutral'}>
                          {model.enabled ? 'Active' : 'Inactive'}
                        </Badge>
                        <Pencil
                          size={14}
                          className="text-fg-3 transition-colors group-hover:text-fg"
                        />
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Exchanges Tab */}
          {activeTab === 'exchanges' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-[13px] text-fg-3">
                  {exchanges.length} account{exchanges.length !== 1 ? 's' : ''}{' '}
                  connected
                </p>
                <Button
                  size="sm"
                  onClick={() => {
                    setEditingExchange(null)
                    setShowExchangeModal(true)
                  }}
                  className="text-brand"
                >
                  <Plus size={14} />
                  Add Exchange
                </Button>
              </div>

              {exchanges.length === 0 ? (
                <div className="py-8 text-center text-[13px] text-fg-3">
                  No exchange accounts connected yet
                </div>
              ) : (
                <div className="space-y-2">
                  {exchanges.map((exchange) => (
                    <button
                      key={exchange.id}
                      onClick={() => {
                        setEditingExchange(exchange.id)
                        setShowExchangeModal(true)
                      }}
                      className="group flex w-full items-center justify-between rounded-md border border-line bg-surface-2 px-3 py-2 transition-colors hover:border-line-strong hover:bg-surface-hover"
                    >
                      <div className="flex items-center gap-3">
                        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-surface-hover">
                          <Building2 size={14} className="text-fg-2" />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium text-fg">
                            {exchange.account_name || exchange.name}
                            <PaperBadge
                              exchangeType={exchange.exchange_type}
                              language={language}
                            />
                          </p>
                          <p className="text-xs text-fg-3 capitalize">
                            {exchange.exchange_type || exchange.type}
                          </p>
                        </div>
                      </div>
                      <ChevronRight
                        size={14}
                        className="text-fg-3 transition-colors group-hover:text-fg"
                      />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Telegram Tab */}
          {activeTab === 'telegram' && (
            <div className="space-y-4">
              <p className="text-[13px] text-fg-3">
                Connect a Telegram bot to receive trading notifications and
                interact with your traders.
              </p>
              <button
                onClick={() => setShowTelegramModal(true)}
                className="group flex w-full items-center justify-between rounded-md border border-line bg-surface-2 px-3 py-2 transition-colors hover:border-line-strong hover:bg-surface-hover"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-7 w-7 items-center justify-center rounded-md bg-info-soft">
                    <MessageCircle size={14} className="text-info" />
                  </div>
                  <span className="text-sm font-medium text-fg">
                    Configure Telegram Bot
                  </span>
                </div>
                <ChevronRight
                  size={14}
                  className="text-fg-3 transition-colors group-hover:text-fg"
                />
              </button>
            </div>
          )}
        </Card>
      </div>

      {/* AI Model Modal */}
      {showModelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--overlay)] px-4">
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={configuredModels}
            editingModelId={editingModel}
            onSave={handleSaveModel}
            onDelete={handleDeleteModel}
            onClose={() => {
              setShowModelModal(false)
              setEditingModel(null)
            }}
            language={language}
          />
        </div>
      )}

      {/* Exchange Modal */}
      {showExchangeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--overlay)] px-4">
          <ExchangeConfigModal
            allExchanges={exchanges}
            editingExchangeId={editingExchange}
            onSave={handleSaveExchange}
            onDelete={handleDeleteExchange}
            onClose={() => {
              setShowExchangeModal(false)
              setEditingExchange(null)
            }}
            language={language}
          />
        </div>
      )}

      {/* Telegram Modal */}
      {showTelegramModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--overlay)] px-4">
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        </div>
      )}
    </div>
  )
}
