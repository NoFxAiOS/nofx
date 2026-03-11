import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { User, Cpu, Building2, MessageCircle, Eye, EyeOff, ChevronRight, Plus, Pencil } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import { ExchangeConfigModal } from '../components/trader/ExchangeConfigModal'
import { TelegramConfigModal } from '../components/trader/TelegramConfigModal'
import { ModelConfigModal } from '../components/trader/AITradersPage'
import type { Exchange, AIModel } from '../types'

type Tab = 'account' | 'models' | 'exchanges' | 'telegram'

export function SettingsPage() {
  const { user } = useAuth()
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<Tab>('account')

  // Account state
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
      api.getExchangeConfigs()
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
          Authorization: `Bearer ${localStorage.getItem('token') || ''}`,
        },
        body: JSON.stringify({ new_password: newPassword }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to update password')
      }
      toast.success('Password updated successfully')
      setNewPassword('')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update password')
    } finally {
      setChangingPassword(false)
    }
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
      if (!modelToUpdate) { toast.error('Model not found'); return }

      let updatedModels: AIModel[]
      if (existingModel) {
        updatedModels = configuredModels.map((m) =>
          m.id === modelId
            ? { ...m, apiKey, customApiUrl: customApiUrl || '', customModelName: customModelName || '', enabled: true }
            : m
        )
      } else {
        updatedModels = [...configuredModels, {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
        ),
      }
      await toast.promise(api.updateModelConfigs(request), {
        loading: 'Saving model config...',
        success: 'Model config saved',
        error: 'Failed to save model config',
      })
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
        m.id === modelId ? { ...m, apiKey: '', customApiUrl: '', customModelName: '', enabled: false } : m
      )
      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
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
    lighterApiKeyIndex?: number
  ) => {
    try {
      if (exchangeId) {
        const request = {
          exchanges: {
            [exchangeId]: {
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
        await toast.promise(api.updateExchangeConfigsEncrypted(request), {
          loading: 'Updating exchange config...',
          success: 'Exchange config updated',
          error: 'Failed to update exchange config',
        })
      } else {
        const createRequest = {
          exchange_type: exchangeType,
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
        await toast.promise(api.createExchangeEncrypted(createRequest), {
          loading: 'Creating exchange account...',
          success: 'Exchange account created',
          error: 'Failed to create exchange account',
        })
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
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: 'Deleting exchange account...',
        success: 'Exchange account deleted',
        error: 'Failed to delete exchange account',
      })
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error('Failed to delete exchange account')
    }
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'account', label: 'Account', icon: <User size={16} /> },
    { key: 'models', label: 'AI Models', icon: <Cpu size={16} /> },
    { key: 'exchanges', label: 'Exchanges', icon: <Building2 size={16} /> },
    { key: 'telegram', label: 'Telegram', icon: <MessageCircle size={16} /> },
  ]

  return (
    <div className="min-h-screen pt-20 pb-12 px-4" style={{ background: '#0B0E11' }}>
      <div className="max-w-2xl mx-auto">
        <h1 className="text-xl font-bold text-white mb-6">Settings</h1>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-zinc-900/60 border border-zinc-800 rounded-xl p-1">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-all
                ${activeTab === tab.key
                  ? 'bg-nofx-gold text-black'
                  : 'text-zinc-400 hover:text-white'
                }`}
            >
              {tab.icon}
              <span className="hidden sm:inline">{tab.label}</span>
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="bg-zinc-900/60 backdrop-blur-xl border border-zinc-800/80 rounded-2xl p-6">

          {/* Account Tab */}
          {activeTab === 'account' && (
            <div className="space-y-6">
              <div>
                <p className="text-xs text-zinc-500 mb-1">Email</p>
                <p className="text-sm text-white font-medium">{user?.email}</p>
              </div>

              <div className="border-t border-zinc-800 pt-6">
                <h3 className="text-sm font-semibold text-white mb-4">Change Password</h3>
                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div>
                    <label className="block text-xs font-medium text-zinc-400 mb-2">New Password</label>
                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="w-full bg-zinc-950/80 border border-zinc-700/80 rounded-xl px-4 py-3 pr-11 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                        placeholder="At least 8 characters"
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 transition-colors"
                      >
                        {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </button>
                    </div>
                  </div>
                  <button
                    type="submit"
                    disabled={changingPassword || newPassword.length < 8}
                    className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {changingPassword ? 'Updating...' : 'Update Password'}
                  </button>
                </form>
              </div>
            </div>
          )}

          {/* AI Models Tab */}
          {activeTab === 'models' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-zinc-400">
                  {configuredModels.length} model{configuredModels.length !== 1 ? 's' : ''} configured
                </p>
                <button
                  onClick={() => { setEditingModel(null); setShowModelModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  Add Model
                </button>
              </div>

              {configuredModels.length === 0 ? (
                <div className="text-center py-8 text-zinc-600 text-sm">
                  No AI models configured yet
                </div>
              ) : (
                <div className="space-y-2">
                  {configuredModels.map((model) => (
                    <button
                      key={model.id}
                      onClick={() => { setEditingModel(model.id); setShowModelModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-zinc-700 flex items-center justify-center">
                          <Cpu size={14} className="text-zinc-300" />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium text-white">{model.name}</p>
                          <p className="text-xs text-zinc-500">{model.provider}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${model.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-700 text-zinc-500'}`}>
                          {model.enabled ? 'Active' : 'Inactive'}
                        </span>
                        <Pencil size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Exchanges Tab */}
          {activeTab === 'exchanges' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-zinc-400">
                  {exchanges.length} account{exchanges.length !== 1 ? 's' : ''} connected
                </p>
                <button
                  onClick={() => { setEditingExchange(null); setShowExchangeModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  Add Exchange
                </button>
              </div>

              {exchanges.length === 0 ? (
                <div className="text-center py-8 text-zinc-600 text-sm">
                  No exchange accounts connected yet
                </div>
              ) : (
                <div className="space-y-2">
                  {exchanges.map((exchange) => (
                    <button
                      key={exchange.id}
                      onClick={() => { setEditingExchange(exchange.id); setShowExchangeModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-zinc-700 flex items-center justify-center">
                          <Building2 size={14} className="text-zinc-300" />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium text-white">{exchange.account_name || exchange.name}</p>
                          <p className="text-xs text-zinc-500 capitalize">{exchange.exchange_type || exchange.type}</p>
                        </div>
                      </div>
                      <ChevronRight size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Telegram Tab */}
          {activeTab === 'telegram' && (
            <div className="space-y-4">
              <p className="text-sm text-zinc-400">
                Connect a Telegram bot to receive trading notifications and interact with your traders.
              </p>
              <button
                onClick={() => setShowTelegramModal(true)}
                className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-[#0088cc]/20 flex items-center justify-center">
                    <MessageCircle size={14} className="text-[#0088cc]" />
                  </div>
                  <span className="text-sm font-medium text-white">Configure Telegram Bot</span>
                </div>
                <ChevronRight size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* AI Model Modal */}
      {showModelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={configuredModels}
            editingModelId={editingModel}
            onSave={handleSaveModel}
            onDelete={handleDeleteModel}
            onClose={() => { setShowModelModal(false); setEditingModel(null) }}
            language={language}
          />
        </div>
      )}

      {/* Exchange Modal */}
      {showExchangeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <ExchangeConfigModal
            allExchanges={exchanges}
            editingExchangeId={editingExchange}
            onSave={handleSaveExchange}
            onDelete={handleDeleteExchange}
            onClose={() => { setShowExchangeModal(false); setEditingExchange(null) }}
            language={language}
          />
        </div>
      )}

      {/* Telegram Modal */}
      {showTelegramModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        </div>
      )}
    </div>
  )
}
