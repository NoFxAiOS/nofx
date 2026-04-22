import { useState, useEffect, useCallback, useRef } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import {
  Plus,
  Copy,
  Trash2,
  Check,
  ChevronDown,
  ChevronRight,
  Settings,
  BarChart3,
  Target,
  Shield,
  Zap,
  Activity,
  Save,
  Sparkles,
  Eye,
  Play,
  FileText,
  Loader2,
  RefreshCw,
  Clock,
  Bot,
  Terminal,
  Code,
  Send,
  Download,
  Upload,
  Globe,
  AlertTriangle,
} from 'lucide-react'
import type { Strategy, StrategyConfig, AIModel, StrategyControlPolicyMode } from '../types'
import { confirmToast, notify } from '../lib/notify'
import { CoinSourceEditor } from '../components/strategy/CoinSourceEditor'
import { IndicatorEditor } from '../components/strategy/IndicatorEditor'
import { RiskControlEditor } from '../components/strategy/RiskControlEditor'
import { PromptSectionsEditor } from '../components/strategy/PromptSectionsEditor'
import { PublishSettingsEditor } from '../components/strategy/PublishSettingsEditor'
import { GridConfigEditor, defaultGridConfig } from '../components/strategy/GridConfigEditor'
import { ProtectionEditor, defaultProtectionConfig, normalizeProtectionConfig } from '../components/strategy/ProtectionEditor'
import { EntryStructureEditor, normalizeEntryStructureConfig } from '../components/strategy/EntryStructureEditor'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { t } from '../i18n/translations'
import { getJson, sendJson } from '../lib/httpClient'

const API_BASE = import.meta.env.VITE_API_BASE || ''

export function buildStrategySavePayload(
  selectedStrategy: Pick<Strategy, 'name' | 'description' | 'is_public' | 'config_visible'>,
  editingConfig: StrategyConfig,
  language: 'zh' | 'en'
) {
  const normalizedProtection = normalizeProtectionConfig(editingConfig.protection)

  const configWithLanguage = {
    ...editingConfig,
    language,
    protection: {
      ...normalizedProtection,
      drawdown_take_profit: {
        ...normalizedProtection.drawdown_take_profit,
        mode: normalizedProtection.drawdown_take_profit.mode || (normalizedProtection.drawdown_take_profit.enabled ? 'manual' : 'disabled'),
      },
    },
    strategy_control_policy: {
      ...editingConfig.strategy_control_policy,
      mode: editingConfig.strategy_control_policy?.mode || 'strict',
    },
    entry_structure: normalizeEntryStructureConfig(editingConfig.entry_structure),
  }

  return {
    name: selectedStrategy.name,
    description: selectedStrategy.description,
    config: configWithLanguage,
    is_public: selectedStrategy.is_public,
    config_visible: selectedStrategy.config_visible,
  }
}

export function StrategyStudioPage() {
  const { token } = useAuth()
  const { language } = useLanguage()

  const [strategies, setStrategies] = useState<Strategy[]>([])
  const [selectedStrategy, setSelectedStrategy] = useState<Strategy | null>(null)
  const [editingConfig, setEditingConfig] = useState<StrategyConfig | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [hasChanges, setHasChanges] = useState(false)

  // AI Models for test run
  const [aiModels, setAiModels] = useState<AIModel[]>([])
  const [selectedModelId, setSelectedModelId] = useState<string>('')

  // Accordion states for left panel
  const [expandedSections, setExpandedSections] = useState({
    gridConfig: true,
    coinSource: true,
    indicators: false,
    riskControl: false,
    protection: false,
    entryStructure: false,
    promptSections: false,
    customPrompt: false,
    publishSettings: false,
  })

  // Right panel states
  const [activeRightTab, setActiveRightTab] = useState<'prompt' | 'test'>('prompt')
  const [promptPreview, setPromptPreview] = useState<{
    system_prompt: string
    user_prompt?: string
    prompt_variant: string
    config_summary: Record<string, unknown>
  } | null>(null)
  const [isLoadingPrompt, setIsLoadingPrompt] = useState(false)
  const [selectedVariant, setSelectedVariant] = useState('balanced')

  // AI Test Run states
  const [aiTestResult, setAiTestResult] = useState<{
    system_prompt?: string
    user_prompt?: string
    ai_response?: string
    reasoning?: string
    decisions?: unknown[]
    parsed_decisions?: unknown[]
    parse_error?: string
    error?: string
    duration_ms?: number
  } | null>(null)
  const [isRunningAiTest, setIsRunningAiTest] = useState(false)

  const toggleSection = (section: keyof typeof expandedSections) => {
    setExpandedSections((prev) => ({
      ...prev,
      [section]: !prev[section],
    }))
  }

  // Fetch AI Models
  const fetchAiModels = useCallback(async () => {
    if (!token) return
    try {
      const data = await getJson<AIModel[] | { models?: AIModel[] }>(`${API_BASE}/api/models`)
      // Backend returns an array, not { models: [] }
      const allModels = Array.isArray(data) ? data : (data.models || [])
      const enabledModels = allModels.filter((m: AIModel) => m.enabled)
      setAiModels(enabledModels)
      if (enabledModels.length > 0 && !selectedModelId) {
        setSelectedModelId(enabledModels[0].id)
      }
    } catch (err) {
      console.error('Failed to fetch AI models:', err)
    }
  }, [selectedModelId])

  // Fetch strategies
  const fetchStrategies = useCallback(async () => {
    if (!token) return
    try {
      const data = await getJson<{ strategies?: Strategy[] }>(`${API_BASE}/api/strategies`)
      const normalizedStrategies = (data.strategies || []).map((strategy: Strategy) => ({
        ...strategy,
        config: {
          ...strategy.config,
          protection: normalizeProtectionConfig(strategy.config?.protection),
          entry_structure: normalizeEntryStructureConfig(strategy.config?.entry_structure),
          strategy_control_policy: {
            ...strategy.config?.strategy_control_policy,
            mode: strategy.config?.strategy_control_policy?.mode || 'strict',
          },
        },
      }))
      setStrategies(normalizedStrategies)

      // Select active or first strategy
      const active = normalizedStrategies.find((s: Strategy) => s.is_active)
      if (active) {
        setSelectedStrategy(active)
        setEditingConfig(active.config)
      } else if (normalizedStrategies.length > 0) {
        setSelectedStrategy(normalizedStrategies[0])
        setEditingConfig(normalizedStrategies[0].config)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setIsLoading(false)
    }
  }, [token])

  useEffect(() => {
    fetchStrategies()
    fetchAiModels()
  }, [fetchStrategies, fetchAiModels])

  // Track previous language to detect actual changes
  const prevLanguageRef = useRef(language)

  // When language changes, update prompt sections to match the new language
  useEffect(() => {
    const updatePromptSectionsForLanguage = async () => {
      // Only update if language actually changed (not on initial mount)
      if (prevLanguageRef.current === language) return
      prevLanguageRef.current = language

      if (!token) return

      try {
        // Fetch default config for the new language
        const defaultConfig = await getJson<StrategyConfig>(
          `${API_BASE}/api/strategies/default-config?lang=${language}`
        )

        // Update only the prompt sections and language field
        setEditingConfig(prev => {
          if (!prev) return prev
          return {
            ...prev,
            language: language as 'zh' | 'en',
            prompt_sections: defaultConfig.prompt_sections,
            protection: normalizeProtectionConfig(prev.protection),
            entry_structure: normalizeEntryStructureConfig(prev.entry_structure),
            strategy_control_policy: {
              ...prev.strategy_control_policy,
              mode: prev.strategy_control_policy?.mode || 'strict',
            },
          }
        })
        setHasChanges(true)
      } catch (err) {
        console.error('Failed to update prompt sections for language:', err)
      }
    }

    updatePromptSectionsForLanguage()
  }, [language, token]) // Only trigger when language changes

  // Create new strategy
  const handleCreateStrategy = async () => {
    if (!token) return
    try {
      const defaultConfig = await getJson<StrategyConfig>(
        `${API_BASE}/api/strategies/default-config?lang=${language}`
      )
      defaultConfig.protection = normalizeProtectionConfig(defaultConfig.protection || defaultProtectionConfig)
      defaultConfig.entry_structure = normalizeEntryStructureConfig(defaultConfig.entry_structure)
      defaultConfig.strategy_control_policy = {
        ...defaultConfig.strategy_control_policy,
        mode: defaultConfig.strategy_control_policy?.mode || 'strict',
      }

      const result = await sendJson<{ id?: string }>(`${API_BASE}/api/strategies`, {
        method: 'POST',
        data: {
          name: tr('newStrategyName'),
          description: '',
          config: defaultConfig,
        },
      })
      await fetchStrategies()
      // Auto-select the newly created strategy
      if (result.id) {
        const now = new Date().toISOString()
        const newStrategy = {
          id: result.id,
          name: tr('newStrategyName'),
          description: '',
          is_active: false,
          is_default: false,
          is_public: false,
          config_visible: true,
          config: defaultConfig,
          created_at: now,
          updated_at: now,
        }
        setSelectedStrategy(newStrategy)
        setEditingConfig(defaultConfig)
        setHasChanges(false)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Delete strategy
  const handleDeleteStrategy = async (id: string) => {
    if (!token) return

    const confirmed = await confirmToast(
      tr('confirmDeleteStrategy'),
      {
        title: tr('confirmDelete'),
        okText: tr('delete'),
        cancelText: tr('cancel'),
      }
    )
    if (!confirmed) return

    try {
      await sendJson<void>(`${API_BASE}/api/strategies/${id}`, {
        method: 'DELETE',
      })
      notify.success(tr('strategyDeleted'))
      // Clear selection if deleted strategy was selected
      if (selectedStrategy?.id === id) {
        setSelectedStrategy(null)
        setEditingConfig(null)
        setHasChanges(false)
      }
      await fetchStrategies()
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Unknown error'
      setError(errorMsg)
      notify.error(errorMsg)
    }
  }

  // Duplicate strategy
  const handleDuplicateStrategy = async (id: string) => {
    if (!token) return
    try {
      await sendJson(`${API_BASE}/api/strategies/${id}/duplicate`, {
        method: 'POST',
        data: {
          name: tr('strategyCopy'),
        },
      })
      await fetchStrategies()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Activate strategy
  const handleActivateStrategy = async (id: string) => {
    if (!token) return
    try {
      await sendJson(`${API_BASE}/api/strategies/${id}/activate`, {
        method: 'POST',
      })
      await fetchStrategies()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Export strategy as JSON file
  const handleExportStrategy = (strategy: Strategy) => {
    const exportData = {
      name: strategy.name,
      description: strategy.description,
      config: strategy.config,
      exported_at: new Date().toISOString(),
      version: '1.0',
    }
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `strategy_${strategy.name.replace(/\s+/g, '_')}_${new Date().toISOString().split('T')[0]}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    notify.success(tr('strategyExported'))
  }

  // Import strategy from JSON file
  const handleImportStrategy = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file || !token) return

    try {
      const text = await file.text()
      const importData = JSON.parse(text)

      // Validate imported data
      if (!importData.config || !importData.name) {
        throw new Error(tr('invalidStrategyFile'))
      }

      await sendJson(`${API_BASE}/api/strategies`, {
        method: 'POST',
        data: {
          name: `${importData.name} (${tr('imported')})`,
          description: importData.description || '',
          config: importData.config,
        },
      })

      notify.success(tr('strategyImported'))
      await fetchStrategies()
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Unknown error'
      notify.error(errorMsg)
    } finally {
      // Reset file input
      event.target.value = ''
    }
  }

  // Save strategy
  const handleSaveStrategy = async () => {
    if (!token || !selectedStrategy || !editingConfig) return
    setIsSaving(true)
    try {
      await sendJson(`${API_BASE}/api/strategies/${selectedStrategy.id}`, {
        method: 'PUT',
        data: buildStrategySavePayload(selectedStrategy, editingConfig, language as 'zh' | 'en'),
      })
      setHasChanges(false)
      notify.success(tr('strategySaved'))
      await fetchStrategies()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setIsSaving(false)
    }
  }

  // Update config section
  const updateConfig = <K extends keyof StrategyConfig>(
    section: K,
    value: StrategyConfig[K]
  ) => {
    if (!editingConfig) return
    setEditingConfig({
      ...editingConfig,
      [section]: value,
    })
    setHasChanges(true)
  }

  const updateStrategyControlPolicyMode = (mode: StrategyControlPolicyMode) => {
    if (!editingConfig) return
    updateConfig('strategy_control_policy', {
      ...editingConfig.strategy_control_policy,
      mode,
    })
  }

  // Fetch prompt preview
  const fetchPromptPreview = async () => {
    if (!token || !editingConfig) return
    setIsLoadingPrompt(true)
    try {
      const data = await sendJson<{
        system_prompt: string
        user_prompt?: string
        prompt_variant: string
        config_summary: Record<string, unknown>
      }>(`${API_BASE}/api/strategies/preview-prompt`, {
        method: 'POST',
        data: {
          config: {
            ...editingConfig,
            protection: normalizeProtectionConfig(editingConfig.protection),
          },
          account_equity: 1000,
          prompt_variant: selectedVariant,
        },
      })
      setPromptPreview(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setIsLoadingPrompt(false)
    }
  }

  // Run AI test with real AI model
  const runAiTest = async () => {
    if (!token || !editingConfig || !selectedModelId) return
    setIsRunningAiTest(true)
    setAiTestResult(null)
    try {
      const data = await sendJson<{
        system_prompt?: string
        user_prompt?: string
        ai_response?: string
        reasoning?: string
        decisions?: unknown[]
        parsed_decisions?: unknown[]
        parse_error?: string
        error?: string
        duration_ms?: number
      }>(`${API_BASE}/api/strategies/test-run`, {
        method: 'POST',
        data: {
          config: {
            ...editingConfig,
            protection: normalizeProtectionConfig(editingConfig.protection),
          },
          prompt_variant: selectedVariant,
          ai_model_id: selectedModelId,
          run_real_ai: true,
        },
      })
      setAiTestResult(data)
    } catch (err) {
      setAiTestResult({
        error: err instanceof Error ? err.message : 'Unknown error',
      })
    } finally {
      setIsRunningAiTest(false)
    }
  }

  const tr = (key: string) => t(`strategyStudio.${key}`, language)

  const formatDecisionTitle = (decision: any, index: number) => {
    const symbol = decision?.symbol || `#${index + 1}`
    const action = decision?.action || 'unknown'
    return `${symbol} · ${action}`
  }

  const renderProtectionPlanSummary = (decision: any) => {
    const plan = decision?.protection_plan
    if (!plan) return null
    const mode = plan.mode || 'unknown'
    return (
      <div className="mt-2 p-2 rounded border border-yellow-500/20 bg-yellow-500/5">
        <div className="text-[11px] font-medium text-yellow-300">Protection Plan · {mode}</div>
        {mode === 'full' && (
          <div className="mt-1 text-[11px] text-nofx-text-muted">
            TP%: {String(plan.take_profit_pct ?? '-')} · SL%: {String(plan.stop_loss_pct ?? '-')}
          </div>
        )}
        {mode === 'ladder' && Array.isArray(plan.ladder_rules) && (
          <div className="mt-1 space-y-1">
            {plan.ladder_rules.map((rule: any, idx: number) => (
              <div key={idx} className="text-[11px] text-nofx-text-muted">
                #{idx + 1} TP {String(rule.take_profit_pct ?? '-')}% / {String(rule.take_profit_close_ratio_pct ?? '-')}% · SL {String(rule.stop_loss_pct ?? '-')}% / {String(rule.stop_loss_close_ratio_pct ?? '-')}%
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[70vh]">
        <div className="text-center">
          <div className="relative">
            <div className="w-16 h-16 rounded-full border-4 border-yellow-500/20 border-t-yellow-500 animate-spin" />
            <Zap className="w-6 h-6 text-yellow-500 absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2" />
          </div>
        </div>
      </div>
    )
  }

  // Get current strategy type (default to ai_trading if not set)
  const currentStrategyType = editingConfig?.strategy_type || 'ai_trading'

  const configSections = [
    // Grid Config - only for grid_trading
    {
      key: 'gridConfig' as const,
      icon: Activity,
      color: '#0ECB81',
      title: tr('gridConfig'),
      forStrategyType: 'grid_trading' as const,
      content: editingConfig?.grid_config && (
        <GridConfigEditor
          config={editingConfig.grid_config}
          onChange={(gridConfig) => updateConfig('grid_config', gridConfig)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    // AI Trading sections
    {
      key: 'coinSource' as const,
      icon: Target,
      color: '#F0B90B',
      title: tr('coinSource'),
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <CoinSourceEditor
          config={editingConfig.coin_source}
          onChange={(coinSource) => updateConfig('coin_source', coinSource)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    {
      key: 'indicators' as const,
      icon: BarChart3,
      color: '#0ECB81',
      title: tr('indicators'),
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <IndicatorEditor
          config={editingConfig.indicators}
          onChange={(indicators) => updateConfig('indicators', indicators)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    {
      key: 'riskControl' as const,
      icon: Shield,
      color: '#F6465D',
      title: tr('riskControl'),
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <div className="space-y-4">
          <div className="rounded-xl border border-sky-500/20 bg-sky-500/5 p-4 space-y-3">
            <div className="flex items-center gap-2">
              <AlertTriangle className="w-4 h-4" style={{ color: '#38BDF8' }} />
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? '开仓前门禁 / Entry Gates' : 'Entry Gates'}
              </div>
            </div>
            <div className="text-xs leading-6" style={{ color: '#C9D1D9' }}>
              {language === 'zh'
                ? '这三块并不是平行开关，而是一条开仓决策链：先看 Regime Filter（环境是否允许）→ 再看开仓结构判断（这笔单的结构证据是否合格）→ 最后由策略控制策略决定不合格时是拒绝、仅审计，还是降级为 wait。'
                : 'These are not parallel toggles. They form an entry decision chain: Regime Filter checks whether the market environment allows a trade → Entry Structure checks whether the single setup has enough structural evidence → Strategy Control Policy decides whether failed opens are rejected, only audited, or downgraded to wait.'}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                <div className="font-medium mb-1" style={{ color: '#EAECEF' }}>{language === 'zh' ? '判断层 ①：Regime Filter' : 'Gate ①: Regime Filter'}</div>
                <div style={{ color: '#848E9C' }}>
                  {language === 'zh'
                    ? '判断当前市场状态、波动、资金费率、趋势方向是否适合开新仓。它不会挂单，也不负责持仓保护。'
                    : 'Checks whether market regime, volatility, funding, and trend alignment allow a new open. It does not create orders and is not a live-position protection tool.'}
                </div>
              </div>
              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                <div className="font-medium mb-1" style={{ color: '#EAECEF' }}>{language === 'zh' ? '判断层 ②：开仓结构判断' : 'Gate ②: Entry Structure'}</div>
                <div style={{ color: '#848E9C' }}>
                  {language === 'zh'
                    ? '要求 AI 给出主周期、相邻周期、支撑阻力、结构锚点，以及失效位/目标位联动。若启用 Fibonacci，它属于更强的附加要求。'
                    : 'Requires AI to provide timeframe context, support/resistance, structural anchors, and invalidation/target linkage. Fibonacci is a stricter optional layer on top.'}
                </div>
              </div>
              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                <div className="font-medium mb-1" style={{ color: '#EAECEF' }}>{language === 'zh' ? '处理层：策略控制策略' : 'Handling Layer: Strategy Control Policy'}</div>
                <div style={{ color: '#848E9C' }}>
                  {language === 'zh'
                    ? '它不负责判断市场或结构本身，而是决定不合格开仓如何处理：strict 直接拒绝，audit_only 只记录，recommend_only 尽量降级成 wait。'
                    : 'It does not judge market or structure by itself. It decides how failed opens are handled: strict rejects, audit_only records, recommend_only tries to downgrade to wait.'}
                </div>
              </div>
            </div>
            <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs" style={{ color: '#FCD34D' }}>
              {language === 'zh'
                ? '逻辑提示：若未启用“开仓结构判断”，下面的结构约束项都不会真正生效；若启用 Fibonacci，建议同时启用支撑/阻力与结构锚点，否则语义会偏弱。'
                : 'Logic hint: if Entry Structure is disabled, its structural requirement toggles do not meaningfully apply. If Fibonacci is enabled, it should usually be paired with support/resistance and structural anchors.'}
            </div>
          </div>
          <RiskControlEditor
            config={editingConfig.risk_control}
            onChange={(riskControl) => updateConfig('risk_control', riskControl)}
            disabled={selectedStrategy?.is_default}
            language={language}
          />
          <div className="pt-3 border-t border-nofx-gold/10">
            <label className="block text-xs font-medium mb-2" style={{ color: '#EAECEF' }}>
              {language === 'zh' ? '策略控制策略' : 'Strategy Control Policy'}
            </label>
            <select
              value={editingConfig.strategy_control_policy?.mode || 'strict'}
              onChange={(e) => updateStrategyControlPolicyMode(e.target.value as StrategyControlPolicyMode)}
              disabled={selectedStrategy?.is_default}
              className="w-full px-3 py-2 rounded-lg text-sm outline-none disabled:opacity-50"
              style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
            >
              <option value="strict">strict</option>
              <option value="audit_only">audit_only</option>
              <option value="recommend_only">recommend_only</option>
            </select>
            <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {language === 'zh'
                ? '默认 strict；旧策略未配置时仍按 strict 保存。'
                : 'Defaults to strict; legacy configs save as strict when unset.'}
            </p>
          </div>
        </div>
      ),
    },
    {
      key: 'protection' as const,
      icon: Shield,
      color: '#F0B90B',
      title: language === 'zh' ? 'Protection / 盈利控制' : 'Protection / Profit Control',
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <ProtectionEditor
          config={normalizeProtectionConfig(editingConfig.protection)}
          onChange={(protection) => updateConfig('protection', protection)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    {
      key: 'entryStructure' as const,
      icon: Target,
      color: '#60a5fa',
      title: language === 'zh' ? '开仓结构判断' : 'Entry Structure',
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <EntryStructureEditor
          config={editingConfig.entry_structure}
          onChange={(entryStructure) => updateConfig('entry_structure', entryStructure)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    {
      key: 'promptSections' as const,
      icon: FileText,
      color: '#a855f7',
      title: tr('promptSections'),
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <PromptSectionsEditor
          config={editingConfig.prompt_sections}
          onChange={(promptSections) => updateConfig('prompt_sections', promptSections)}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
    {
      key: 'customPrompt' as const,
      icon: Settings,
      color: '#60a5fa',
      title: tr('customPrompt'),
      forStrategyType: 'ai_trading' as const,
      content: editingConfig && (
        <div>
          <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
            {tr('customPromptDesc')}
          </p>
          <textarea
            value={editingConfig.custom_prompt || ''}
            onChange={(e) => updateConfig('custom_prompt', e.target.value)}
            disabled={selectedStrategy?.is_default}
            placeholder={tr('customPromptPlaceholder')}
            className="w-full h-32 px-3 py-2 rounded-lg resize-none font-mono text-xs"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
        </div>
      ),
    },
    {
      key: 'publishSettings' as const,
      icon: Globe,
      color: '#0ECB81',
      title: tr('publishSettings'),
      forStrategyType: 'both' as const,
      content: selectedStrategy && (
        <PublishSettingsEditor
          isPublic={selectedStrategy.is_public ?? false}
          configVisible={selectedStrategy.config_visible ?? true}
          onIsPublicChange={(value) => {
            setSelectedStrategy({ ...selectedStrategy, is_public: value })
            setHasChanges(true)
          }}
          onConfigVisibleChange={(value) => {
            setSelectedStrategy({ ...selectedStrategy, config_visible: value })
            setHasChanges(true)
          }}
          disabled={selectedStrategy?.is_default}
          language={language}
        />
      ),
    },
  ].filter(section =>
    section.forStrategyType === 'both' || section.forStrategyType === currentStrategyType
  )

  return (
    <DeepVoidBackground className="h-[calc(100vh-64px)] flex flex-col bg-nofx-bg relative overflow-hidden">

      {/* Header */}
      {/* Header */}
      <div className="flex-shrink-0 px-4 py-3 border-b border-nofx-gold/20 bg-nofx-bg/60 backdrop-blur-md z-10">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-gradient-to-br from-nofx-gold to-yellow-500">
              <Sparkles className="w-5 h-5 text-black" />
            </div>
            <div>
              <h1 className="text-lg font-bold text-nofx-text">{tr('strategyStudio')}</h1>
              <p className="text-xs text-nofx-text-muted">{tr('subtitle')}</p>
            </div>
          </div>
          {error && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs bg-nofx-danger/10 text-nofx-danger">
              {error}
              <button onClick={() => setError(null)} className="hover:underline">×</button>
            </div>
          )}
        </div>
      </div>

      {/* Main Content - Three Columns */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Column - Strategy List */}
        <div className="w-48 flex-shrink-0 border-r border-nofx-gold/20 overflow-y-auto bg-nofx-bg/30 backdrop-blur-sm z-10">
          <div className="p-2">
            <div className="flex items-center justify-between mb-2 px-2">
              <span className="text-xs font-medium text-nofx-text-muted">{tr('strategies')}</span>
              <div className="flex items-center gap-1">
                {/* Import button with hidden file input */}
                <label className="p-1 rounded hover:bg-white/10 transition-colors cursor-pointer text-nofx-text-muted hover:text-white" title={tr('importStrategy')}>
                  <Upload className="w-4 h-4" />
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleImportStrategy}
                    className="hidden"
                  />
                </label>
                <button
                  onClick={handleCreateStrategy}
                  className="p-1 rounded hover:bg-white/10 transition-colors text-nofx-gold"
                  title={tr('newStrategyTooltip')}
                >
                  <Plus className="w-4 h-4" />
                </button>
              </div>
            </div>
            <div className="space-y-1">
              {strategies.map((strategy) => (
                <div
                  key={strategy.id}
                  onClick={() => {
                    setSelectedStrategy(strategy)
                    setEditingConfig(strategy.config)
                    setHasChanges(false)
                    setPromptPreview(null)
                    setAiTestResult(null)
                  }}
                  className={`group px-2 py-2 rounded-lg cursor-pointer transition-all ${selectedStrategy?.id === strategy.id
                    ? 'ring-1 ring-nofx-gold/50 bg-nofx-gold/10 shadow-[0_0_15px_rgba(240,185,11,0.1)]'
                    : 'hover:bg-nofx-bg-lighter/60 hover:ring-1 hover:ring-nofx-gold/20 bg-transparent'
                    }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm truncate text-nofx-text">{strategy.name}</span>
                    <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={(e) => { e.stopPropagation(); handleExportStrategy(strategy) }}
                        className="p-1 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white"
                        title={tr('export')}
                      >
                        <Download className="w-3 h-3" />
                      </button>
                      {!strategy.is_default && (
                        <>
                          <button
                            onClick={(e) => { e.stopPropagation(); handleDuplicateStrategy(strategy.id) }}
                            className="p-1 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white"
                            title={tr('duplicate')}
                          >
                            <Copy className="w-3 h-3" />
                          </button>
                          <button
                            onClick={(e) => { e.stopPropagation(); handleDeleteStrategy(strategy.id) }}
                            className="p-1 rounded hover:bg-nofx-danger/20 text-nofx-danger"
                            title={tr('deleteTooltip')}
                          >
                            <Trash2 className="w-3 h-3" />
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-1 mt-1 flex-wrap">
                    {strategy.is_active && (
                      <span className="px-1.5 py-0.5 text-[10px] rounded bg-nofx-success/15 text-nofx-success">
                        {tr('active')}
                      </span>
                    )}
                    {strategy.is_default && (
                      <span className="px-1.5 py-0.5 text-[10px] rounded bg-nofx-gold/15 text-nofx-gold">
                        {tr('default')}
                      </span>
                    )}
                    {strategy.is_public && (
                      <span className="px-1.5 py-0.5 text-[10px] rounded flex items-center gap-0.5 bg-blue-400/15 text-blue-400">
                        <Globe className="w-2.5 h-2.5" />
                        {tr('public')}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Middle Column - Config Editor */}
        <div className="flex-1 min-w-0 overflow-y-auto border-r border-nofx-gold/20">
          {selectedStrategy && editingConfig ? (
            <div className="p-4">
              {/* Strategy Name & Actions */}
              <div className="flex items-center justify-between mb-4">
                <div className="flex-1 min-w-0">
                  <input
                    type="text"
                    value={selectedStrategy.name}
                    onChange={(e) => {
                      setSelectedStrategy({ ...selectedStrategy, name: e.target.value })
                      setHasChanges(true)
                    }}
                    disabled={selectedStrategy.is_default}
                    className="text-lg font-bold bg-transparent border-none outline-none w-full text-nofx-text placeholder-nofx-text-muted"
                  />
                  <input
                    type="text"
                    value={selectedStrategy.description || ''}
                    onChange={(e) => {
                      setSelectedStrategy({ ...selectedStrategy, description: e.target.value })
                      setHasChanges(true)
                    }}
                    disabled={selectedStrategy.is_default}
                    placeholder={tr('addDescription')}
                    className="text-xs bg-transparent border-none outline-none w-full text-nofx-text-muted placeholder-nofx-text-muted/50 mt-1"
                  />
                  {hasChanges && (
                    <span className="text-xs text-nofx-gold">● {tr('unsaved')}</span>
                  )}
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  {!selectedStrategy.is_active && (
                    <button
                      onClick={() => handleActivateStrategy(selectedStrategy.id)}
                      className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs transition-colors bg-nofx-success/10 border border-nofx-success/30 text-nofx-success hover:bg-nofx-success/20"
                    >
                      <Check className="w-3 h-3" />
                      {tr('activate')}
                    </button>
                  )}
                  {!selectedStrategy.is_default && (
                    <button
                      onClick={handleSaveStrategy}
                      disabled={isSaving || !hasChanges}
                      className={`flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50
                        ${hasChanges ? 'bg-nofx-gold text-black hover:bg-yellow-500' : 'bg-nofx-bg-lighter text-nofx-text-muted cursor-not-allowed'}`}
                    >
                      <Save className="w-3 h-3" />
                      {isSaving ? tr('saving') : tr('save')}
                    </button>
                  )}
                </div>
              </div>

              {/* Strategy Type Selector */}
              {editingConfig && (
                <div className="mb-4 p-4 rounded-lg bg-nofx-bg-lighter border border-nofx-gold/20">
                  <div className="flex items-center gap-2 mb-3">
                    <Zap className="w-4 h-4" style={{ color: '#F0B90B' }} />
                    <span className="text-sm font-medium text-nofx-text">{tr('strategyType')}</span>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <button
                      onClick={() => {
                        if (!selectedStrategy?.is_default) {
                          updateConfig('strategy_type', 'ai_trading')
                          // Clear grid config when switching to AI trading
                          updateConfig('grid_config', undefined)
                        }
                      }}
                      disabled={selectedStrategy?.is_default}
                      className={`p-3 rounded-lg border transition-all ${
                        (!editingConfig.strategy_type || editingConfig.strategy_type === 'ai_trading')
                          ? 'border-nofx-gold bg-nofx-gold/10'
                          : 'border-nofx-border hover:border-nofx-gold/50'
                      }`}
                    >
                      <div className="flex items-center gap-2 mb-1">
                        <Bot className="w-4 h-4" style={{ color: '#F0B90B' }} />
                        <span className="text-sm font-medium text-nofx-text">{tr('aiTrading')}</span>
                      </div>
                      <p className="text-xs text-nofx-text-muted text-left">{tr('aiTradingDesc')}</p>
                    </button>
                    <button
                      onClick={() => {
                        if (!selectedStrategy?.is_default) {
                          updateConfig('strategy_type', 'grid_trading')
                          // Initialize grid config if not exists
                          if (!editingConfig.grid_config) {
                            updateConfig('grid_config', defaultGridConfig)
                          }
                        }
                      }}
                      disabled={selectedStrategy?.is_default}
                      className={`p-3 rounded-lg border transition-all ${
                        editingConfig.strategy_type === 'grid_trading'
                          ? 'border-nofx-gold bg-nofx-gold/10'
                          : 'border-nofx-border hover:border-nofx-gold/50'
                      }`}
                    >
                      <div className="flex items-center gap-2 mb-1">
                        <Activity className="w-4 h-4" style={{ color: '#0ECB81' }} />
                        <span className="text-sm font-medium text-nofx-text">{tr('gridTrading')}</span>
                      </div>
                      <p className="text-xs text-nofx-text-muted text-left">{tr('gridTradingDesc')}</p>
                    </button>
                  </div>
                </div>
              )}

              {/* Config Sections */}
              <div className="space-y-2">
                {configSections.map(({ key, icon: Icon, color, title, content }) => (
                  <div
                    key={key}
                    className="rounded-lg overflow-hidden bg-nofx-bg-lighter border border-nofx-gold/20"
                  >
                    <button
                      onClick={() => toggleSection(key)}
                      className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-white/5 transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <Icon className="w-4 h-4" style={{ color }} />
                        <span className="text-sm font-medium text-nofx-text">{title}</span>
                      </div>
                      {expandedSections[key] ? (
                        <ChevronDown className="w-4 h-4 text-nofx-text-muted" />
                      ) : (
                        <ChevronRight className="w-4 h-4 text-nofx-text-muted" />
                      )}
                    </button>
                    {expandedSections[key] && (
                      <div className="px-3 pb-3">
                        {content}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-center h-full">
              <div className="text-center">
                <Activity className="w-12 h-12 mx-auto mb-2 opacity-30 text-nofx-text-muted" />
                <p className="text-sm text-nofx-text-muted">
                  {tr('selectOrCreate')}
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Right Column - Prompt Preview & AI Test */}
        <div className="w-[420px] flex-shrink-0 flex flex-col overflow-hidden">
          {/* Tabs */}
          <div className="flex-shrink-0 flex border-b border-nofx-gold/20">
            <button
              onClick={() => setActiveRightTab('prompt')}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium transition-colors ${activeRightTab === 'prompt' ? 'border-b-2 border-purple-500 text-purple-500' : 'opacity-60 hover:opacity-100 text-nofx-text-muted'
                }`}
            >
              <Eye className="w-4 h-4" />
              {tr('promptPreview')}
            </button>
            <button
              onClick={() => setActiveRightTab('test')}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium transition-colors ${activeRightTab === 'test' ? 'border-b-2 border-green-500 text-green-500' : 'opacity-60 hover:opacity-100 text-nofx-text-muted'
                }`}
            >
              <Play className="w-4 h-4" />
              {tr('aiTestRun')}
            </button>
          </div>

          {/* Tab Content */}
          <div className="flex-1 overflow-y-auto">
            {activeRightTab === 'prompt' ? (
              /* Prompt Preview Tab */
              <div className="p-3 space-y-3">
                {/* Controls */}
                <div className="flex items-center gap-2 flex-wrap">
                  <select
                    value={selectedVariant}
                    onChange={(e) => setSelectedVariant(e.target.value)}
                    className="px-2 py-1.5 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text outline-none focus:border-nofx-gold"
                  >
                    <option value="balanced">{tr('balanced')}</option>
                    <option value="aggressive">{tr('aggressive')}</option>
                    <option value="conservative">{tr('conservative')}</option>
                  </select>
                  <button
                    onClick={fetchPromptPreview}
                    disabled={isLoadingPrompt || !editingConfig}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium transition-colors disabled:opacity-50 bg-purple-600 hover:bg-purple-700 text-white"
                  >
                    {isLoadingPrompt ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
                    {promptPreview ? tr('refreshPrompt') : tr('loadPrompt')}
                  </button>
                </div>

                {promptPreview ? (
                  <>
                    {/* Config Summary */}
                    <div className="p-2 rounded-lg bg-nofx-bg border border-nofx-gold/20">
                      <div className="flex items-center gap-1.5 mb-2">
                        <Code className="w-3 h-3 text-purple-500" />
                        <span className="text-xs font-medium text-purple-500">Config</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 text-xs">
                        {Object.entries(promptPreview.config_summary || {}).map(([key, value]) => (
                          <div key={key}>
                            <div className="text-nofx-text-muted">{key.replace(/_/g, ' ')}</div>
                            <div className="text-nofx-text">{String(value)}</div>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* System Prompt */}
                    <div>
                      <div className="flex items-center justify-between mb-1.5">
                        <div className="flex items-center gap-1.5">
                          <FileText className="w-3 h-3 text-purple-500" />
                          <span className="text-xs font-medium text-nofx-text">{tr('systemPrompt')}</span>
                        </div>
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-nofx-bg-lighter text-nofx-text-muted">
                          {promptPreview.system_prompt.length.toLocaleString()} chars
                        </span>
                      </div>
                      <pre
                        className="p-2 rounded-lg text-[11px] font-mono overflow-auto bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                        style={{ maxHeight: '400px' }}
                      >
                        {promptPreview.system_prompt}
                      </pre>
                    </div>
                  </>
                ) : (
                  <div className="flex flex-col items-center justify-center py-12 text-nofx-text-muted">
                    <Eye className="w-10 h-10 mb-2 opacity-30" />
                    <p className="text-sm">{tr('generatePromptPreview')}</p>
                  </div>
                )}
              </div>
            ) : (
              /* AI Test Tab */
              <div className="p-3 space-y-3">
                {/* Controls */}
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Bot className="w-4 h-4 text-green-500" />
                    <span className="text-xs font-medium text-nofx-text">{tr('selectModel')}</span>
                  </div>
                  {aiModels.length > 0 ? (
                    <select
                      value={selectedModelId}
                      onChange={(e) => setSelectedModelId(e.target.value)}
                      className="w-full px-3 py-2 rounded-lg text-sm bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                    >
                      {aiModels.map((model) => (
                        <option key={model.id} value={model.id}>
                          {model.name} ({model.provider})
                        </option>
                      ))}
                    </select>
                  ) : (
                    <div className="px-3 py-2 rounded-lg text-sm bg-nofx-danger/10 text-nofx-danger">
                      {tr('noModel')}
                    </div>
                  )}

                  <div className="flex items-center gap-2">
                    <select
                      value={selectedVariant}
                      onChange={(e) => setSelectedVariant(e.target.value)}
                      className="px-2 py-1.5 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                    >
                      <option value="balanced">{tr('balanced')}</option>
                      <option value="aggressive">{tr('aggressive')}</option>
                      <option value="conservative">{tr('conservative')}</option>
                    </select>
                    <button
                      onClick={runAiTest}
                      disabled={isRunningAiTest || !editingConfig || !selectedModelId}
                      className="flex-1 flex items-center justify-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all disabled:opacity-50 text-white shadow-lg shadow-green-500/20 bg-gradient-to-br from-green-500 to-green-600"
                    >
                      {isRunningAiTest ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          {tr('running')}
                        </>
                      ) : (
                        <>
                          <Send className="w-4 h-4" />
                          {tr('runTest')}
                        </>
                      )}
                    </button>
                  </div>
                  <p className="text-[10px] text-nofx-text-muted">{tr('testNote')}</p>
                </div>

                {/* Test Results */}
                {aiTestResult ? (
                  <div className="space-y-3">
                    {aiTestResult.error ? (
                      <div className="p-3 rounded-lg bg-nofx-danger/10 border border-nofx-danger/30">
                        <p className="text-sm text-nofx-danger">{aiTestResult.error}</p>
                      </div>
                    ) : (
                      <>
                        {aiTestResult.duration_ms && (
                          <div className="flex items-center gap-2">
                            <Clock className="w-3 h-3 text-nofx-text-muted" />
                            <span className="text-xs text-nofx-text-muted">
                              {tr('duration')}: {(aiTestResult.duration_ms / 1000).toFixed(2)}s
                            </span>
                          </div>
                        )}

                        {/* User Prompt Input */}
                        {aiTestResult.user_prompt && (
                          <div>
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <Terminal className="w-3 h-3 text-blue-400" />
                              <span className="text-xs font-medium text-nofx-text">{tr('userPrompt')} (Input)</span>
                            </div>
                            <pre
                              className="p-2 rounded-lg text-[10px] font-mono overflow-auto bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                              style={{ maxHeight: '200px' }}
                            >
                              {aiTestResult.user_prompt}
                            </pre>
                          </div>
                        )}

                        {/* AI Reasoning */}
                        {aiTestResult.reasoning && (
                          <div>
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <Sparkles className="w-3 h-3 text-nofx-gold" />
                              <span className="text-xs font-medium text-nofx-text">{tr('reasoning')}</span>
                            </div>
                            <pre
                              className="p-2 rounded-lg text-[10px] font-mono overflow-auto whitespace-pre-wrap bg-nofx-bg border border-nofx-gold/30 text-nofx-text"
                              style={{ maxHeight: '200px' }}
                            >
                              {aiTestResult.reasoning}
                            </pre>
                          </div>
                        )}

                        {aiTestResult.parsed_decisions && aiTestResult.parsed_decisions.length > 0 && (
                          <div className="space-y-2">
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <Activity className="w-3 h-3 text-green-500" />
                              <span className="text-xs font-medium text-nofx-text">Parsed Decisions</span>
                            </div>
                            {aiTestResult.parsed_decisions.map((decision: any, index: number) => (
                              <div key={index} className="p-2 rounded-lg bg-nofx-bg border border-green-500/20">
                                <div className="text-xs font-medium text-nofx-text">{formatDecisionTitle(decision, index)}</div>
                                <div className="mt-1 text-[11px] text-nofx-text-muted">
                                  Leverage: {String(decision?.leverage ?? '-')} · Size: {String(decision?.position_size_usd ?? '-')} · Confidence: {String(decision?.confidence ?? '-')}
                                </div>
                                {decision?.reasoning && (
                                  <div className="mt-1 text-[11px] text-nofx-text-muted whitespace-pre-wrap">
                                    {decision.reasoning}
                                  </div>
                                )}
                                {renderProtectionPlanSummary(decision)}
                              </div>
                            ))}
                            <details className="p-2 rounded-lg bg-nofx-bg border border-green-500/20">
                              <summary className="cursor-pointer text-[11px] text-green-400">Raw Parsed Decision JSON</summary>
                              <pre className="mt-2 text-[10px] font-mono overflow-auto text-nofx-text" style={{ maxHeight: '220px' }}>
                                {JSON.stringify(aiTestResult.parsed_decisions, null, 2)}
                              </pre>
                            </details>
                          </div>
                        )}

                        {aiTestResult.parse_error && (
                          <div>
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <AlertTriangle className="w-3 h-3 text-yellow-500" />
                              <span className="text-xs font-medium text-yellow-400">Parse Error</span>
                            </div>
                            <pre
                              className="p-2 rounded-lg text-[10px] font-mono overflow-auto whitespace-pre-wrap bg-nofx-bg border border-yellow-500/30 text-yellow-300"
                              style={{ maxHeight: '160px' }}
                            >
                              {aiTestResult.parse_error}
                            </pre>
                          </div>
                        )}

                        {/* AI Decisions */}
                        {aiTestResult.decisions && aiTestResult.decisions.length > 0 && (
                          <div>
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <Activity className="w-3 h-3 text-green-500" />
                              <span className="text-xs font-medium text-nofx-text">{tr('decisions')}</span>
                            </div>
                            <pre
                              className="p-2 rounded-lg text-[10px] font-mono overflow-auto bg-nofx-bg border border-green-500/30 text-nofx-text"
                              style={{ maxHeight: '200px' }}
                            >
                              {JSON.stringify(aiTestResult.decisions, null, 2)}
                            </pre>
                          </div>
                        )}

                        {/* Raw AI Response */}
                        {aiTestResult.ai_response && (
                          <div>
                            <div className="flex items-center gap-1.5 mb-1.5">
                              <FileText className="w-3 h-3 text-nofx-text-muted" />
                              <span className="text-xs font-medium text-nofx-text">{tr('aiOutput')} (Raw)</span>
                            </div>
                            <pre
                              className="p-2 rounded-lg text-[10px] font-mono overflow-auto whitespace-pre-wrap bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                              style={{ maxHeight: '300px' }}
                            >
                              {aiTestResult.ai_response}
                            </pre>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                ) : (
                  <div className="flex flex-col items-center justify-center py-12 text-nofx-text-muted">
                    <Play className="w-10 h-10 mb-2 opacity-30" />
                    <p className="text-sm">{tr('runAiTestHint')}</p>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}

export default StrategyStudioPage
