import React, { useState, useEffect } from 'react'
import { Check, ChevronLeft, ExternalLink, MessageCircle, Unlink, ArrowRight } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../../lib/api'
import type { TelegramConfig, AIModel } from '../../types'
import type { Language } from '../../i18n/translations'

// Step indicator (reused pattern from ExchangeConfigModal)
function StepIndicator({ currentStep, labels }: { currentStep: number; labels: string[] }) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: index < currentStep ? '#0ECB81' : index === currentStep ? '#2AABEE' : '#2B3139',
                color: index <= currentStep ? '#000' : '#848E9C',
              }}
            >
              {index < currentStep ? <Check className="w-4 h-4" /> : index + 1}
            </div>
            <span
              className="text-xs font-medium hidden sm:block"
              style={{ color: index === currentStep ? '#EAECEF' : '#848E9C' }}
            >
              {label}
            </span>
          </div>
          {index < labels.length - 1 && (
            <div
              className="w-8 h-0.5 mx-1"
              style={{ background: index < currentStep ? '#0ECB81' : '#2B3139' }}
            />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

interface TelegramConfigModalProps {
  onClose: () => void
  language: Language
}

export function TelegramConfigModal({ onClose, language }: TelegramConfigModalProps) {
  const [step, setStep] = useState(0)
  const [token, setToken] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [config, setConfig] = useState<TelegramConfig | null>(null)
  const [models, setModels] = useState<AIModel[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isUnbinding, setIsUnbinding] = useState(false)

  const zh = language === 'zh'

  // Load current config and available models
  useEffect(() => {
    Promise.all([
      api.getTelegramConfig().catch(() => null),
      api.getModelConfigs().catch(() => [] as AIModel[]),
    ]).then(([cfg, allModels]) => {
      const enabledModels = allModels.filter((m) => m.enabled)
      setModels(enabledModels)

      if (cfg) {
        setConfig(cfg)
        setSelectedModelId(cfg.model_id ?? '')
        if (cfg.is_bound) {
          setStep(2)
        } else if (cfg.token_masked && cfg.token_masked !== '') {
          setStep(1)
        }
      }
    }).finally(() => setIsLoading(false))
  }, [])

  const handleSaveToken = async () => {
    if (!token.trim()) return
    if (isSaving) return

    // Basic format validation: looks like "123456789:ABCdef..."
    if (!/^\d+:[A-Za-z0-9_-]{35,}$/.test(token.trim())) {
      toast.error(zh ? 'Bot Token 格式不正确，应为 "数字:字母数字串"' : 'Invalid Bot Token format. Expected "numbers:alphanumeric"')
      return
    }

    setIsSaving(true)
    try {
      await api.updateTelegramConfig(token.trim(), selectedModelId || undefined)
      toast.success(zh ? 'Bot Token 已保存，等待绑定' : 'Bot Token saved, waiting for binding')
      const updated = await api.getTelegramConfig()
      setConfig(updated)
      setToken('')
      setStep(1)
    } catch (err) {
      toast.error(zh ? '保存失败，请检查 Token 是否正确' : 'Save failed, please verify the token')
    } finally {
      setIsSaving(false)
    }
  }

  const handleUnbind = async () => {
    if (isUnbinding) return
    setIsUnbinding(true)
    try {
      await api.unbindTelegram()
      toast.success(zh ? '已解绑 Telegram 账号' : 'Telegram account unbound')
      const updated = await api.getTelegramConfig()
      setConfig(updated)
      setStep(updated.token_masked ? 1 : 0)
    } catch {
      toast.error(zh ? '解绑失败' : 'Unbind failed')
    } finally {
      setIsUnbinding(false)
    }
  }

  const stepLabels = zh
    ? ['创建 Bot', '绑定账号', '完成']
    : ['Create Bot', 'Bind Account', 'Done']

  // Model selector shared between steps
  const ModelSelector = () => (
    <div className="space-y-2">
      <label className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {zh ? '选择 AI 模型（可选）' : 'Select AI Model (optional)'}
      </label>
      {models.length === 0 ? (
        <div
          className="px-4 py-3 rounded-xl text-xs"
          style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#848E9C' }}
        >
          {zh ? '暂无启用的模型，请先在「AI 模型」中配置' : 'No enabled models. Configure one in AI Models first.'}
        </div>
      ) : (
        <select
          value={selectedModelId}
          onChange={(e) => setSelectedModelId(e.target.value)}
          className="w-full px-4 py-3 rounded-xl text-sm appearance-none"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: selectedModelId ? '#EAECEF' : '#848E9C',
          }}
        >
          <option value="">{zh ? '— 自动选择（推荐）' : '— Auto-select (recommended)'}</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name} ({m.provider}{m.customModelName ? ` · ${m.customModelName}` : ''})
            </option>
          ))}
        </select>
      )}
      <div className="text-xs" style={{ color: '#848E9C' }}>
        {zh
          ? '不选则自动使用已启用的模型'
          : 'Leave blank to auto-use any enabled model'}
      </div>
    </div>
  )

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4 overflow-y-auto backdrop-blur-sm">
      <div
        className="rounded-2xl w-full max-w-lg relative my-8 shadow-2xl"
        style={{ background: 'linear-gradient(180deg, #1E2329 0%, #181A20 100%)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {step > 0 && !config?.is_bound && (
              <button
                type="button"
                onClick={() => setStep(step - 1)}
                className="p-2 rounded-lg hover:bg-white/10 transition-colors"
              >
                <ChevronLeft className="w-5 h-5" style={{ color: '#848E9C' }} />
              </button>
            )}
            <div className="flex items-center gap-2">
              <MessageCircle className="w-6 h-6" style={{ color: '#2AABEE' }} />
              <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                {zh ? 'Telegram Bot 配置' : 'Telegram Bot Setup'}
              </h3>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-white/10 transition-colors"
            style={{ color: '#848E9C' }}
          >
            ✕
          </button>
        </div>

        {/* Step Indicator */}
        <div className="px-6 pt-4">
          <StepIndicator currentStep={step} labels={stepLabels} />
        </div>

        {/* Content */}
        <div className="px-6 pb-6 space-y-5">
          {isLoading ? (
            <div className="text-center py-8 text-zinc-500 text-sm font-mono">
              {zh ? '加载中...' : 'Loading...'}
            </div>
          ) : (
            <>
              {/* Step 0: Create bot via BotFather */}
              {step === 0 && (
                <div className="space-y-5">
                  <div
                    className="p-4 rounded-xl space-y-3"
                    style={{ background: 'rgba(42, 171, 238, 0.1)', border: '1px solid rgba(42, 171, 238, 0.3)' }}
                  >
                    <div className="flex items-start gap-3">
                      <span className="text-2xl">🤖</span>
                      <div>
                        <div className="font-semibold mb-1" style={{ color: '#2AABEE' }}>
                          {zh ? '第一步：在 Telegram 创建你的 Bot' : 'Step 1: Create your Bot in Telegram'}
                        </div>
                        <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                          <div>1. {zh ? '打开 Telegram，搜索' : 'Open Telegram, search for'} <code className="text-blue-400">@BotFather</code></div>
                          <div>2. {zh ? '发送' : 'Send'} <code className="text-blue-400">/newbot</code> {zh ? '命令' : 'command'}</div>
                          <div>3. {zh ? '按提示输入 Bot 名称和用户名' : 'Follow prompts to set bot name and username'}</div>
                          <div>4. {zh ? 'BotFather 会返回一个 Token，复制它' : 'BotFather will return a Token, copy it'}</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <a
                    href="https://t.me/BotFather"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl font-semibold transition-all hover:scale-[1.02]"
                    style={{ background: '#2AABEE', color: '#000' }}
                  >
                    <ExternalLink className="w-4 h-4" />
                    {zh ? '打开 @BotFather' : 'Open @BotFather'}
                  </a>

                  <div className="space-y-2">
                    <label className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
                      {zh ? '粘贴 Bot Token' : 'Paste Bot Token'}
                    </label>
                    <input
                      type="password"
                      value={token}
                      onChange={(e) => setToken(e.target.value)}
                      placeholder="123456789:ABCdefGHIjklmNOPQRstuvwxYZ"
                      className="w-full px-4 py-3 rounded-xl font-mono text-sm"
                      style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    />
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {zh ? 'Token 格式：数字:字母数字串，如 123456789:ABCdef...' : 'Format: numbers:alphanumeric, e.g. 123456789:ABCdef...'}
                    </div>
                  </div>

                  <ModelSelector />

                  <button
                    onClick={handleSaveToken}
                    disabled={isSaving || !token.trim()}
                    className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{ background: '#2AABEE', color: '#000' }}
                  >
                    {isSaving
                      ? (zh ? '保存中...' : 'Saving...')
                      : (<>{zh ? '保存并继续' : 'Save & Continue'} <ArrowRight className="w-4 h-4" /></>)
                    }
                  </button>
                </div>
              )}

              {/* Step 1: Send /start to activate */}
              {step === 1 && (
                <div className="space-y-5">
                  <div
                    className="p-4 rounded-xl space-y-3"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', border: '1px solid rgba(14, 203, 129, 0.3)' }}
                  >
                    <div className="flex items-start gap-3">
                      <span className="text-2xl">📱</span>
                      <div>
                        <div className="font-semibold mb-1" style={{ color: '#0ECB81' }}>
                          {zh ? '第二步：向你的 Bot 发送 /start' : 'Step 2: Send /start to your Bot'}
                        </div>
                        <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                          <div>1. {zh ? '在 Telegram 中搜索你刚创建的 Bot' : 'Search for your newly created Bot in Telegram'}</div>
                          <div>2. {zh ? '点击 Start 或发送' : 'Click Start or send'} <code className="text-green-400">/start</code></div>
                          <div>3. {zh ? 'Bot 会自动绑定到你的账号' : 'Bot will automatically bind to your account'}</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  {config?.token_masked && (
                    <div
                      className="p-3 rounded-xl flex items-center gap-3"
                      style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                    >
                      <div className="w-2 h-2 rounded-full bg-yellow-500 animate-pulse flex-shrink-0" />
                      <div>
                        <div className="text-xs font-mono" style={{ color: '#848E9C' }}>
                          {zh ? '当前 Token' : 'Current Token'}
                        </div>
                        <div className="text-sm font-mono" style={{ color: '#EAECEF' }}>
                          {config.token_masked}
                        </div>
                      </div>
                    </div>
                  )}

                  <div
                    className="p-3 rounded-xl text-center"
                    style={{ background: 'rgba(240, 185, 11, 0.08)', border: '1px solid rgba(240, 185, 11, 0.2)' }}
                  >
                    <div className="text-xs" style={{ color: '#F0B90B' }}>
                      {zh
                        ? '⏳ 等待你发送 /start... 发送后刷新页面查看状态'
                        : '⏳ Waiting for you to send /start... Refresh page after sending'}
                    </div>
                  </div>

                  <div className="flex gap-3">
                    <button
                      onClick={() => { setStep(0); setToken('') }}
                      className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5"
                      style={{ background: '#2B3139', color: '#848E9C' }}
                    >
                      {zh ? '重新配置 Token' : 'Reconfigure Token'}
                    </button>
                    <button
                      onClick={async () => {
                        try {
                          const updated = await api.getTelegramConfig()
                          setConfig(updated)
                          if (updated.is_bound) {
                            setStep(2)
                            toast.success(zh ? '绑定成功！' : 'Bound successfully!')
                          } else {
                            toast.info(zh ? '尚未收到 /start，请先向 Bot 发送 /start' : 'No /start received yet. Please send /start to your Bot first')
                          }
                        } catch {
                          toast.error(zh ? '检查失败' : 'Check failed')
                        }
                      }}
                      className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02]"
                      style={{ background: '#0ECB81', color: '#000' }}
                    >
                      <Check className="w-4 h-4" />
                      {zh ? '检查绑定状态' : 'Check Status'}
                    </button>
                  </div>
                </div>
              )}

              {/* Step 2: Bound & active */}
              {step === 2 && (
                <div className="space-y-5">
                  <div
                    className="p-5 rounded-xl text-center space-y-3"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', border: '1px solid rgba(14, 203, 129, 0.3)' }}
                  >
                    <div className="text-4xl">🎉</div>
                    <div className="font-bold text-lg" style={{ color: '#0ECB81' }}>
                      {zh ? 'Telegram Bot 已绑定！' : 'Telegram Bot is Active!'}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {zh
                        ? '你现在可以通过 Telegram 用自然语言控制交易系统'
                        : 'You can now control the trading system via natural language in Telegram'}
                    </div>
                  </div>

                  {config?.token_masked && (
                    <div
                      className="p-3 rounded-xl flex items-center gap-3"
                      style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                    >
                      <div className="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
                      <div className="min-w-0">
                        <div className="text-xs font-mono" style={{ color: '#848E9C' }}>
                          {zh ? 'Bot Token' : 'Bot Token'}
                        </div>
                        <div className="text-sm font-mono truncate" style={{ color: '#EAECEF' }}>
                          {config.token_masked}
                        </div>
                      </div>
                    </div>
                  )}

                  {/* AI Model selector — works on active bot */}
                  <BoundModelSelector
                    zh={zh}
                    models={models}
                    currentModelId={config?.model_id ?? ''}
                    onSaved={(modelId) => {
                      setConfig((prev) => prev ? { ...prev, model_id: modelId } : prev)
                    }}
                  />

                  {/* What you can do */}
                  <div
                    className="p-4 rounded-xl space-y-2"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  >
                    <div className="text-xs font-semibold uppercase tracking-wide mb-2" style={{ color: '#848E9C' }}>
                      {zh ? '支持的命令' : 'Supported Commands'}
                    </div>
                    {[
                      { cmd: '/help', desc: zh ? '查看所有命令' : 'Show all commands' },
                      { cmd: zh ? '查看交易员状态' : 'Show trader status', desc: zh ? '自然语言查询' : 'Natural language' },
                      { cmd: zh ? '启动/停止交易员' : 'Start/stop trader', desc: zh ? '自然语言控制' : 'Natural language control' },
                      { cmd: zh ? '查看持仓' : 'View positions', desc: zh ? '实时持仓查询' : 'Real-time position query' },
                      { cmd: zh ? '配置策略' : 'Configure strategy', desc: zh ? '修改交易策略' : 'Modify trading strategy' },
                    ].map((item, i) => (
                      <div key={i} className="flex items-start gap-2 text-xs">
                        <code className="font-mono px-1.5 py-0.5 rounded flex-shrink-0" style={{ background: '#1E2329', color: '#2AABEE' }}>
                          {item.cmd}
                        </code>
                        <span style={{ color: '#848E9C' }}>{item.desc}</span>
                      </div>
                    ))}
                  </div>

                  <div className="flex gap-3">
                    <button
                      onClick={handleUnbind}
                      disabled={isUnbinding}
                      className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:bg-white/5 disabled:opacity-50"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.2)' }}
                    >
                      <Unlink className="w-4 h-4" />
                      {isUnbinding ? (zh ? '解绑中...' : 'Unbinding...') : (zh ? '解绑账号' : 'Unbind Account')}
                    </button>
                    <button
                      onClick={onClose}
                      className="flex-1 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02]"
                      style={{ background: '#2AABEE', color: '#000' }}
                    >
                      {zh ? '完成' : 'Done'}
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// BoundModelSelector — lets the user change the AI model when the bot is already active.
// It updates the model_id without requiring re-entry of the bot token.
function BoundModelSelector({
  zh,
  models,
  currentModelId,
  onSaved,
}: {
  zh: boolean
  models: AIModel[]
  currentModelId: string
  onSaved: (modelId: string) => void
}) {
  const [modelId, setModelId] = useState(currentModelId)
  const [isSaving, setIsSaving] = useState(false)

  // Keep in sync if parent updates
  useEffect(() => { setModelId(currentModelId) }, [currentModelId])

  const handleSave = async () => {
    setIsSaving(true)
    try {
      // POST /api/telegram/model — lightweight endpoint for model-only update
      await api.updateTelegramModel(modelId)
      onSaved(modelId)
      toast.success(zh ? 'AI 模型已更新' : 'AI model updated')
    } catch {
      toast.error(zh ? '更新失败' : 'Update failed')
    } finally {
      setIsSaving(false)
    }
  }

  if (models.length === 0) return null

  return (
    <div className="space-y-2">
      <label className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {zh ? 'AI 模型（用于自然语言解析）' : 'AI Model (for natural language)'}
      </label>
      <div className="flex gap-2">
        <select
          value={modelId}
          onChange={(e) => setModelId(e.target.value)}
          className="flex-1 px-3 py-2.5 rounded-xl text-sm appearance-none"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: modelId ? '#EAECEF' : '#848E9C',
          }}
        >
          <option value="">{zh ? '— 自动选择' : '— Auto-select'}</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}{m.customModelName ? ` · ${m.customModelName}` : ''}
            </option>
          ))}
        </select>
        <button
          onClick={handleSave}
          disabled={isSaving || modelId === currentModelId}
          className="px-4 py-2.5 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-40 disabled:cursor-not-allowed"
          style={{ background: '#F0B90B', color: '#000', whiteSpace: 'nowrap' }}
        >
          {isSaving ? '...' : (zh ? '保存' : 'Save')}
        </button>
      </div>
    </div>
  )
}
