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
} from 'lucide-react'
import type { Strategy, StrategyConfig, AIModel, TrailingStopConfig } from '../types'
import { confirmToast, notify } from '../lib/notify'
import { t, type Language } from '../i18n/translations'
import { CoinSourceEditor } from '../components/strategy/CoinSourceEditor'
import { IndicatorEditor } from '../components/strategy/IndicatorEditor'
import { RiskControlEditor } from '../components/strategy/RiskControlEditor'
import { PromptSectionsEditor } from '../components/strategy/PromptSectionsEditor'
import { PublishSettingsEditor } from '../components/strategy/PublishSettingsEditor'
import { DeepVoidBackground } from '../components/DeepVoidBackground'

const API_BASE = import.meta.env.VITE_API_BASE || ''

export function StrategyStudioPage() {
  const { token } = useAuth()
  const { language } = useLanguage()
  const apiLanguage: 'zh' | 'en' = language === 'zh' ? 'zh' : 'en'
  const tr = (key: string, params?: Record<string, string | number>) =>
    t(`strategyStudioPage.${key}`, language as Language, params)

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
    coinSource: true,
    indicators: false,
    riskControl: false,
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

  const withTrailingDefaults = useCallback(
    (config: StrategyConfig): StrategyConfig => {
      const trailing: TrailingStopConfig | undefined = config?.risk_control?.trailing_stop
      const normalizedTrailing: TrailingStopConfig = {
        enabled: trailing?.enabled ?? true,
        mode: trailing?.mode || 'pnl_pct',
        activation_pct: trailing?.activation_pct ?? 0,
        trail_pct: trailing?.trail_pct ?? 3,
        check_interval_sec: trailing?.check_interval_sec ?? 30,
        check_interval_ms: trailing?.check_interval_ms,
        tighten_bands: trailing?.tighten_bands || [],
        close_pct: trailing?.close_pct ?? 1,
      }
      const riskControl = config.risk_control || ({} as any)
      return {
        ...config,
        risk_control: {
          ...riskControl,
          trailing_stop: normalizedTrailing,
        },
      }
    },
    []
  )

  // AI Test Run states
  const [aiTestResult, setAiTestResult] = useState<{
    system_prompt?: string
    user_prompt?: string
    ai_response?: string
    reasoning?: string
    decisions?: unknown[]
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
      const response = await fetch(`${API_BASE}/api/models`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.ok) {
        const data = await response.json()
        // 后端返回的是数组，不是 { models: [] }
        const allModels = Array.isArray(data) ? data : (data.models || [])
        const enabledModels = allModels.filter((m: AIModel) => m.enabled)
        setAiModels(enabledModels)
        if (enabledModels.length > 0 && !selectedModelId) {
          setSelectedModelId(enabledModels[0].id)
        }
      }
    } catch (err) {
      console.error('Failed to fetch AI models:', err)
    }
  }, [token, selectedModelId])

  // Fetch strategies
  const fetchStrategies = useCallback(async () => {
    if (!token) return
    try {
      const response = await fetch(`${API_BASE}/api/strategies`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!response.ok) throw new Error('Failed to fetch strategies')
      const data = await response.json()
      const normalizedStrategies: Strategy[] = (data.strategies || []).map((s: Strategy) => ({
        ...s,
        config: withTrailingDefaults(s.config),
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
  }, [token, withTrailingDefaults])

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
        const response = await fetch(
          `${API_BASE}/api/strategies/default-config?lang=${apiLanguage}`,
          { headers: { Authorization: `Bearer ${token}` } }
        )
        if (!response.ok) return
        const defaultConfig = await response.json()

        // Update only the prompt sections and language field
        setEditingConfig(prev => {
          if (!prev) return prev
          return {
            ...prev,
            language: apiLanguage,
            prompt_sections: defaultConfig.prompt_sections,
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
      const configResponse = await fetch(
        `${API_BASE}/api/strategies/default-config?lang=${apiLanguage}`,
        { headers: { Authorization: `Bearer ${token}` } }
      )
      const defaultConfig = withTrailingDefaults(await configResponse.json())

      const response = await fetch(`${API_BASE}/api/strategies`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: tr('newStrategyName'),
          description: '',
          config: defaultConfig,
        }),
      })
      if (!response.ok) throw new Error('Failed to create strategy')
      const result = await response.json()
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

    const confirmed = await confirmToast(tr('confirmDeleteMessage'), {
      title: tr('confirmDeleteTitle'),
      okText: tr('confirmDeleteOk'),
      cancelText: tr('confirmDeleteCancel'),
    })
    if (!confirmed) return

    try {
      const response = await fetch(`${API_BASE}/api/strategies/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!response.ok) throw new Error('Failed to delete strategy')
      notify.success(tr('toastDeleted'))
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
      const response = await fetch(`${API_BASE}/api/strategies/${id}/duplicate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: tr('strategyCopyName'),
        }),
      })
      if (!response.ok) throw new Error('Failed to duplicate strategy')
      await fetchStrategies()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Activate strategy
  const handleActivateStrategy = async (id: string) => {
    if (!token) return
    try {
      const response = await fetch(`${API_BASE}/api/strategies/${id}/activate`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!response.ok) throw new Error('Failed to activate strategy')
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
    notify.success(tr('toastExported'))
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
        throw new Error(tr('invalidFile'))
      }

      // Create new strategy with imported config
      const response = await fetch(`${API_BASE}/api/strategies`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: `${importData.name} (${tr('importedSuffix')})`,
          description: importData.description || '',
          config: importData.config,
        }),
      })
      if (!response.ok) throw new Error('Failed to import strategy')

      notify.success(tr('toastImported'))
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
      // Always sync the config language with the current interface language
      const configWithLanguage = {
        ...editingConfig,
        language: apiLanguage,
      }
      const response = await fetch(
        `${API_BASE}/api/strategies/${selectedStrategy.id}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            name: selectedStrategy.name,
            description: selectedStrategy.description,
            config: configWithLanguage,
            is_public: selectedStrategy.is_public,
            config_visible: selectedStrategy.config_visible,
          }),
        }
      )
      if (!response.ok) throw new Error('Failed to save strategy')
      setHasChanges(false)
      notify.success(tr('toastSaved'))
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

  // Fetch prompt preview
  const fetchPromptPreview = async () => {
    if (!token || !editingConfig) return
    setIsLoadingPrompt(true)
    try {
      const response = await fetch(`${API_BASE}/api/strategies/preview-prompt`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          config: editingConfig,
          account_equity: 1000,
          prompt_variant: selectedVariant,
        }),
      })
      if (!response.ok) throw new Error('Failed to fetch prompt preview')
      const data = await response.json()
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
      const response = await fetch(`${API_BASE}/api/strategies/test-run`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          config: editingConfig,
          prompt_variant: selectedVariant,
          ai_model_id: selectedModelId,
          run_real_ai: true,
        }),
      })
      if (!response.ok) throw new Error('Failed to run AI test')
      const data = await response.json()
      setAiTestResult(data)
    } catch (err) {
      setAiTestResult({
        error: err instanceof Error ? err.message : 'Unknown error',
      })
    } finally {
      setIsRunningAiTest(false)
    }
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

  const configSections = [
    {
      key: 'coinSource' as const,
      icon: Target,
      color: '#F0B90B',
      title: tr('coinSource'),
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
      content: editingConfig && (
        <RiskControlEditor
          config={editingConfig.risk_control}
          onChange={(riskControl) => updateConfig('risk_control', riskControl)}
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
      content: editingConfig && (
        <div>
          <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
            {tr('customPromptDescription')}
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
  ]

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
              <h1 className="text-lg font-bold text-nofx-text">{tr('title')}</h1>
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
                  title={tr('newStrategyName')}
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
                        title={tr('exportStrategy')}
                      >
                        <Download className="w-3 h-3" />
                      </button>
                      {!strategy.is_default && (
                        <>
                          <button
                            onClick={(e) => { e.stopPropagation(); handleDuplicateStrategy(strategy.id) }}
                            className="p-1 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white"
                            title={tr('duplicateStrategy')}
                          >
                            <Copy className="w-3 h-3" />
                          </button>
                          <button
                            onClick={(e) => { e.stopPropagation(); handleDeleteStrategy(strategy.id) }}
                            className="p-1 rounded hover:bg-nofx-danger/20 text-nofx-danger"
                            title={tr('deleteStrategy')}
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
                      {tr('publicTag')}
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
                    placeholder={tr('descriptionPlaceholder')}
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
                  {tr('emptyState')}
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
                        <span className="text-xs font-medium text-purple-500">{tr('configLabel')}</span>
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
                          {tr('chars', { count: promptPreview.system_prompt.length.toLocaleString() })}
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
                    <p className="text-sm">{tr('promptPreviewCta')}</p>
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
                    <p className="text-sm">{tr('aiTestCta')}</p>
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
