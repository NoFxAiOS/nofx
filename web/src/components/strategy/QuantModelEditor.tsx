import { useState, useCallback } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import type { QuantModel, QuantModelConfig, ModelIndicator, ModelRule, ModelParameters, SignalGenerationConfig } from '../../types/strategy'
import { notify, confirmToast } from '../../lib/notify'
import {
  Brain,
  Plus,
  Trash2,
  Download,
  Upload,
  Copy,
  Save,
  X,
  ChevronDown,
  ChevronUp,
  Settings,
  Activity,
  BarChart3,
  GitBranch,
  Play,
  TrendingUp,
  AlertTriangle,
  Info,
  CheckCircle2,
  Code2,
} from 'lucide-react'

const API_BASE = import.meta.env.VITE_API_BASE || ''

interface QuantModelEditorProps {
  models: QuantModel[]
  onModelsChange: (models: QuantModel[]) => void
  selectedModelId?: string
  onSelectModel: (model: QuantModel | null) => void
  disabled?: boolean
}

type ModelType = 'indicator_based' | 'rule_based' | 'ml_classifier' | 'ensemble'

interface ModelTemplate {
  id: string
  name: string
  description: string
  model_type: ModelType
  config: QuantModelConfig
}

export function QuantModelEditor({
  models,
  onModelsChange,
  selectedModelId,
  onSelectModel,
  disabled = false,
}: QuantModelEditorProps) {
  const { token } = useAuth()
  const { language } = useLanguage()
  const [isCreating, setIsCreating] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [showImportDialog, setShowImportDialog] = useState(false)
  const [templates, setTemplates] = useState<ModelTemplate[]>([])
  const [expandedSection, setExpandedSection] = useState<string | null>('indicators')
  
  // Form state for creating/editing
  const [formState, setFormState] = useState<{
    name: string
    description: string
    model_type: ModelType
    config: QuantModelConfig
  }>({
    name: '',
    description: '',
    model_type: 'indicator_based',
    config: getDefaultConfig(),
  })

  const tr = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      en: {
        'quantModels': 'Quant Models',
        'createNew': 'Create New Model',
        'importModel': 'Import Model',
        'exportModel': 'Export',
        'cloneModel': 'Clone',
        'deleteModel': 'Delete',
        'saveModel': 'Save',
        'cancel': 'Cancel',
        'editModel': 'Edit',
        'modelName': 'Model Name',
        'modelDescription': 'Description',
        'modelType': 'Model Type',
        'indicators': 'Indicators',
        'rules': 'Rules',
        'parameters': 'Parameters',
        'signalConfig': 'Signal Generation',
        'addIndicator': 'Add Indicator',
        'addRule': 'Add Rule',
        'remove': 'Remove',
        'indicatorName': 'Indicator',
        'period': 'Period',
        'timeframe': 'Timeframe',
        'weight': 'Weight',
        'ruleName': 'Rule Name',
        'condition': 'Condition',
        'action': 'Action',
        'confidence': 'Confidence',
        'priority': 'Priority',
        'lookbackPeriods': 'Lookback Periods',
        'entryThreshold': 'Entry Threshold',
        'exitThreshold': 'Exit Threshold',
        'maxHoldTime': 'Max Hold Time (bars)',
        'minHoldTime': 'Min Hold Time (bars)',
        'maxDailyTrades': 'Max Daily Trades',
        'signalType': 'Signal Type',
        'minConfidence': 'Min Confidence',
        'requireConfirmation': 'Require Confirmation',
        'confirmationDelay': 'Confirmation Delay',
        'importDialogTitle': 'Import Model',
        'pasteImportData': 'Paste exported JSON data here',
        'import': 'Import',
        'invalidImport': 'Invalid import data',
        'modelCreated': 'Model created successfully',
        'modelUpdated': 'Model updated successfully',
        'modelDeleted': 'Model deleted successfully',
        'modelCloned': 'Model cloned successfully',
        'modelExported': 'Model exported',
        'confirmDelete': 'Are you sure you want to delete this model?',
        'noModels': 'No quant models yet. Create or import one.',
        'backtestStats': 'Backtest Stats',
        'winRate': 'Win Rate',
        'avgProfit': 'Avg Profit',
        'maxDrawdown': 'Max Drawdown',
        'sharpeRatio': 'Sharpe',
        'usageCount': 'Usage Count',
        'publicModel': 'Public',
        'privateModel': 'Private',
        'indicatorBased': 'Indicator Based',
        'ruleBased': 'Rule Based',
        'mlClassifier': 'ML Classifier',
        'ensemble': 'Ensemble',
        'selectModel': 'Select a model to view or edit',
        'templateIndicator': 'RSI + EMA + MACD Strategy',
        'templateRule': 'Multi-Rule Strategy',
        'useTemplate': 'Use Template',
        'creating': 'Creating...',
        'saving': 'Saving...',
        'fallbackToAI': 'Fallback to AI if model fails',
        'backtestBeforeLive': 'Require backtest before live trading',
        'modelConfidenceThreshold': 'Model Confidence Threshold',
      },
      zh: {
        'quantModels': '量化模型',
        'createNew': '创建新模型',
        'importModel': '导入模型',
        'exportModel': '导出',
        'cloneModel': '克隆',
        'deleteModel': '删除',
        'saveModel': '保存',
        'cancel': '取消',
        'editModel': '编辑',
        'modelName': '模型名称',
        'modelDescription': '描述',
        'modelType': '模型类型',
        'indicators': '指标',
        'rules': '规则',
        'parameters': '参数',
        'signalConfig': '信号生成',
        'addIndicator': '添加指标',
        'addRule': '添加规则',
        'remove': '移除',
        'indicatorName': '指标名称',
        'period': '周期',
        'timeframe': '时间框架',
        'weight': '权重',
        'ruleName': '规则名称',
        'condition': '条件',
        'action': '操作',
        'confidence': '置信度',
        'priority': '优先级',
        'lookbackPeriods': '回看周期',
        'entryThreshold': '入场阈值',
        'exitThreshold': '出场阈值',
        'maxHoldTime': '最大持仓时间(K线)',
        'minHoldTime': '最小持仓时间(K线)',
        'maxDailyTrades': '每日最大交易次数',
        'signalType': '信号类型',
        'minConfidence': '最小置信度',
        'requireConfirmation': '需要确认',
        'confirmationDelay': '确认延迟',
        'importDialogTitle': '导入模型',
        'pasteImportData': '在此粘贴导出的JSON数据',
        'import': '导入',
        'invalidImport': '导入数据无效',
        'modelCreated': '模型创建成功',
        'modelUpdated': '模型更新成功',
        'modelDeleted': '模型删除成功',
        'modelCloned': '模型克隆成功',
        'modelExported': '模型已导出',
        'confirmDelete': '确定要删除此模型吗？',
        'noModels': '暂无量化模型。创建或导入一个。',
        'backtestStats': '回测统计',
        'winRate': '胜率',
        'avgProfit': '平均收益',
        'maxDrawdown': '最大回撤',
        'sharpeRatio': '夏普比率',
        'usageCount': '使用次数',
        'publicModel': '公开',
        'privateModel': '私有',
        'indicatorBased': '指标型',
        'ruleBased': '规则型',
        'mlClassifier': '机器学习分类器',
        'ensemble': '集成模型',
        'selectModel': '选择模型以查看或编辑',
        'templateIndicator': 'RSI + EMA + MACD 策略',
        'templateRule': '多规则策略',
        'useTemplate': '使用模板',
        'creating': '创建中...',
        'saving': '保存中...',
        'fallbackToAI': '模型失败时回退到AI',
        'backtestBeforeLive': '实盘前需要回测',
        'modelConfidenceThreshold': '模型置信度阈值',
      },
    }
    return translations[language]?.[key] || translations['en'][key] || key
  }

  // Fetch templates on mount
  const fetchTemplates = useCallback(async () => {
    if (!token) return
    try {
      const response = await fetch(`${API_BASE}/api/quant-models/templates`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.ok) {
        const data = await response.json()
        setTemplates(data.templates || [])
      }
    } catch (err) {
      console.error('Failed to fetch templates:', err)
    }
  }, [token])

  // Create new model
  const handleCreate = async () => {
    if (!token || !formState.name.trim()) return
    
    try {
      const response = await fetch(`${API_BASE}/api/quant-models`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: formState.name,
          description: formState.description,
          model_type: formState.model_type,
          config: formState.config,
          is_public: false,
        }),
      })
      
      if (!response.ok) throw new Error('Failed to create model')
      
      const result = await response.json()
      notify.success(tr('modelCreated'))
      
      // Refresh models list
      await refreshModels()
      
      setIsCreating(false)
      setFormState({
        name: '',
        description: '',
        model_type: 'indicator_based',
        config: getDefaultConfig(),
      })
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Update existing model
  const handleUpdate = async () => {
    if (!token || !selectedModelId) return
    
    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${selectedModelId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: formState.name,
          description: formState.description,
          config: formState.config,
        }),
      })
      
      if (!response.ok) throw new Error('Failed to update model')
      
      notify.success(tr('modelUpdated'))
      await refreshModels()
      setIsEditing(false)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Delete model
  const handleDelete = async (modelId: string) => {
    if (!token) return
    
    const confirmed = await confirmToast(tr('confirmDelete'), {
      title: tr('deleteModel'),
      okText: tr('deleteModel'),
      cancelText: tr('cancel'),
    })
    
    if (!confirmed) return
    
    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${modelId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      
      if (!response.ok) throw new Error('Failed to delete model')
      
      notify.success(tr('modelDeleted'))
      await refreshModels()
      
      if (selectedModelId === modelId) {
        onSelectModel(null)
      }
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Clone model
  const handleClone = async (modelId: string) => {
    if (!token) return
    
    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${modelId}/clone`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      
      if (!response.ok) throw new Error('Failed to clone model')
      
      notify.success(tr('modelCloned'))
      await refreshModels()
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Export model
  const handleExport = async (model: QuantModel) => {
    if (!token) return
    
    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${model.id}/export`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })
      
      if (!response.ok) throw new Error('Failed to export model')
      
      const exportData = await response.json()
      
      // Download as JSON file
      const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `quant_model_${model.name.replace(/\s+/g, '_')}_${new Date().toISOString().split('T')[0]}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      
      notify.success(tr('modelExported'))
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Import model
  const handleImport = async (importData: string) => {
    if (!token) return
    
    try {
      let data: Record<string, unknown>
      try {
        data = JSON.parse(importData)
      } catch {
        throw new Error(tr('invalidImport'))
      }
      
      const response = await fetch(`${API_BASE}/api/quant-models/import`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(data),
      })
      
      if (!response.ok) {
        const err = await response.json()
        throw new Error(err.error || 'Import failed')
      }
      
      notify.success(tr('modelCreated'))
      await refreshModels()
      setShowImportDialog(false)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  // Refresh models list
  const refreshModels = async () => {
    if (!token) return
    try {
      const response = await fetch(`${API_BASE}/api/quant-models`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.ok) {
        const data = await response.json()
        onModelsChange(data.models || [])
      }
    } catch (err) {
      console.error('Failed to refresh models:', err)
    }
  }

  // Start editing a model
  const startEditing = (model: QuantModel) => {
    setFormState({
      name: model.name,
      description: model.description,
      model_type: model.model_type,
      config: model.config,
    })
    setIsEditing(true)
  }

  // Add indicator
  const addIndicator = () => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        indicators: [
          ...(prev.config.indicators || []),
          {
            name: 'RSI',
            period: 14,
            timeframe: '1h',
            weight: 1.0,
            params: {},
          },
        ],
      },
    }))
  }

  // Remove indicator
  const removeIndicator = (index: number) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        indicators: prev.config.indicators?.filter((_, i) => i !== index) || [],
      },
    }))
  }

  // Update indicator
  const updateIndicator = (index: number, field: keyof ModelIndicator, value: unknown) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        indicators: prev.config.indicators?.map((ind, i) =>
          i === index ? { ...ind, [field]: value } : ind
        ) || [],
      },
    }))
  }

  // Add rule
  const addRule = () => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        rules: [
          ...(prev.config.rules || []),
          {
            name: 'New Rule',
            condition: 'RSI_14 < 30',
            action: 'buy',
            confidence: 70,
            priority: 1,
          },
        ],
      },
    }))
  }

  // Remove rule
  const removeRule = (index: number) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        rules: prev.config.rules?.filter((_, i) => i !== index) || [],
      },
    }))
  }

  // Update rule
  const updateRule = (index: number, field: keyof ModelRule, value: unknown) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        rules: prev.config.rules?.map((rule, i) =>
          i === index ? { ...rule, [field]: value } : rule
        ) || [],
      },
    }))
  }

  // Update parameters
  const updateParameters = (field: keyof ModelParameters, value: number) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        parameters: {
          ...prev.config.parameters,
          [field]: value,
        },
      },
    }))
  }

  // Update signal config
  const updateSignalConfig = (field: keyof SignalGenerationConfig, value: unknown) => {
    setFormState(prev => ({
      ...prev,
      config: {
        ...prev.config,
        signal_config: {
          ...prev.config.signal_config,
          [field]: value,
        },
      },
    }))
  }

  const selectedModel = models.find(m => m.id === selectedModelId)
  const isCreateOrEdit = isCreating || isEditing

  return (
    <div className="space-y-4">
      {/* Header Actions */}
      {!isCreateOrEdit && (
        <div className="flex items-center justify-between">
          <span className="text-xs text-nofx-text-muted">
            {models.length} {tr('quantModels')}
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => { setIsCreating(true); fetchTemplates() }}
              disabled={disabled}
              className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 bg-nofx-gold/10 text-nofx-gold hover:bg-nofx-gold/20"
            >
              <Plus className="w-3.5 h-3.5" />
              {tr('createNew')}
            </button>
            <button
              onClick={() => setShowImportDialog(true)}
              disabled={disabled}
              className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50 bg-nofx-bg-lighter text-nofx-text hover:bg-white/10"
            >
              <Upload className="w-3.5 h-3.5" />
              {tr('importModel')}
            </button>
          </div>
        </div>
      )}

      {/* Model List (when not editing) */}
      {!isCreateOrEdit && (
        <div className="space-y-2 max-h-[300px] overflow-y-auto">
          {models.length === 0 ? (
            <div className="text-center py-8 text-nofx-text-muted">
              <Brain className="w-10 h-10 mx-auto mb-2 opacity-30" />
              <p className="text-sm">{tr('noModels')}</p>
            </div>
          ) : (
            models.map(model => (
              <div
                key={model.id}
                onClick={() => onSelectModel(model.id === selectedModelId ? null : model)}
                className={`p-3 rounded-lg border cursor-pointer transition-all ${
                  selectedModelId === model.id
                    ? 'border-nofx-gold bg-nofx-gold/10'
                    : 'border-nofx-border hover:border-nofx-gold/50'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-nofx-text text-sm truncate">
                        {model.name}
                      </span>
                      {model.is_public && (
                        <span className="px-1.5 py-0.5 text-[10px] rounded bg-blue-500/20 text-blue-400">
                          {tr('publicModel')}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-nofx-text-muted mt-1 line-clamp-1">
                      {model.description || `${tr(model.model_type)}`}
                    </p>
                    <div className="flex items-center gap-3 mt-2 text-[10px] text-nofx-text-muted">
                      {model.win_rate > 0 && (
                        <span className="flex items-center gap-1">
                          <TrendingUp className="w-3 h-3" />
                          {(model.win_rate * 100).toFixed(1)}%
                        </span>
                      )}
                      {model.backtest_count > 0 && (
                        <span className="flex items-center gap-1">
                          <Activity className="w-3 h-3" />
                          {model.backtest_count} tests
                        </span>
                      )}
                      {model.usage_count > 0 && (
                        <span className="flex items-center gap-1">
                          <CheckCircle2 className="w-3 h-3" />
                          {model.usage_count} uses
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleExport(model) }}
                      className="p-1.5 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white"
                      title={tr('exportModel')}
                    >
                      <Download className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleClone(model.id) }}
                      className="p-1.5 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white"
                      title={tr('cloneModel')}
                    >
                      <Copy className="w-3.5 h-3.5" />
                    </button>
                    {selectedModelId === model.id && (
                      <button
                        onClick={(e) => { e.stopPropagation(); startEditing(model) }}
                        className="p-1.5 rounded hover:bg-white/10 text-nofx-gold"
                        title={tr('editModel')}
                      >
                        <Settings className="w-3.5 h-3.5" />
                      </button>
                    )}
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDelete(model.id) }}
                      className="p-1.5 rounded hover:bg-nofx-danger/20 text-nofx-danger"
                      title={tr('deleteModel')}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* Create/Edit Form */}
      {isCreateOrEdit && (
        <div className="space-y-4">
          {/* Basic Info */}
          <div className="space-y-3">
            <div>
              <label className="text-xs text-nofx-text-muted mb-1 block">{tr('modelName')}</label>
              <input
                type="text"
                value={formState.name}
                onChange={e => setFormState(prev => ({ ...prev, name: e.target.value }))}
                className="w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:border-nofx-gold outline-none"
                placeholder={tr('modelName')}
              />
            </div>
            <div>
              <label className="text-xs text-nofx-text-muted mb-1 block">{tr('modelDescription')}</label>
              <textarea
                value={formState.description}
                onChange={e => setFormState(prev => ({ ...prev, description: e.target.value }))}
                className="w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:border-nofx-gold outline-none resize-none h-16"
                placeholder={tr('modelDescription')}
              />
            </div>
            <div>
              <label className="text-xs text-nofx-text-muted mb-1 block">{tr('modelType')}</label>
              <select
                value={formState.model_type}
                onChange={e => setFormState(prev => ({ ...prev, model_type: e.target.value as ModelType }))}
                className="w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:border-nofx-gold outline-none"
              >
                <option value="indicator_based">{tr('indicatorBased')}</option>
                <option value="rule_based">{tr('ruleBased')}</option>
                <option value="ml_classifier">{tr('mlClassifier')}</option>
                <option value="ensemble">{tr('ensemble')}</option>
              </select>
            </div>
          </div>

          {/* Indicators Section (for indicator_based) */}
          {formState.model_type === 'indicator_based' && (
            <CollapsibleSection
              title={tr('indicators')}
              icon={BarChart3}
              isOpen={expandedSection === 'indicators'}
              onToggle={() => setExpandedSection(expandedSection === 'indicators' ? null : 'indicators')}
            >
              <div className="space-y-3">
                {formState.config.indicators?.map((indicator, idx) => (
                  <div key={idx} className="p-3 rounded-lg bg-nofx-bg border border-nofx-border">
                    <div className="grid grid-cols-4 gap-2 mb-2">
                      <select
                        value={indicator.name}
                        onChange={e => updateIndicator(idx, 'name', e.target.value)}
                        className="px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                      >
                        <option value="RSI">RSI</option>
                        <option value="EMA">EMA</option>
                        <option value="MACD">MACD</option>
                        <option value="ATR">ATR</option>
                        <option value="BOLL">Bollinger</option>
                        <option value="SMA">SMA</option>
                      </select>
                      <input
                        type="number"
                        value={indicator.period}
                        onChange={e => updateIndicator(idx, 'period', parseInt(e.target.value))}
                        className="px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                        placeholder={tr('period')}
                      />
                      <select
                        value={indicator.timeframe}
                        onChange={e => updateIndicator(idx, 'timeframe', e.target.value)}
                        className="px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                      >
                        <option value="1m">1m</option>
                        <option value="5m">5m</option>
                        <option value="15m">15m</option>
                        <option value="1h">1h</option>
                        <option value="4h">4h</option>
                        <option value="1d">1d</option>
                      </select>
                      <input
                        type="number"
                        step="0.1"
                        value={indicator.weight}
                        onChange={e => updateIndicator(idx, 'weight', parseFloat(e.target.value))}
                        className="px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                        placeholder={tr('weight')}
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-nofx-text-muted">Weight: {indicator.weight}</span>
                      <button
                        onClick={() => removeIndicator(idx)}
                        className="text-nofx-danger hover:text-nofx-danger/80"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  onClick={addIndicator}
                  className="w-full py-2 rounded-lg border border-dashed border-nofx-border hover:border-nofx-gold text-nofx-text-muted hover:text-nofx-gold text-xs flex items-center justify-center gap-1 transition-colors"
                >
                  <Plus className="w-3.5 h-3.5" />
                  {tr('addIndicator')}
                </button>
              </div>
            </CollapsibleSection>
          )}

          {/* Rules Section (for rule_based) */}
          {formState.model_type === 'rule_based' && (
            <CollapsibleSection
              title={tr('rules')}
              icon={GitBranch}
              isOpen={expandedSection === 'rules'}
              onToggle={() => setExpandedSection(expandedSection === 'rules' ? null : 'rules')}
            >
              <div className="space-y-3">
                {formState.config.rules?.map((rule, idx) => (
                  <div key={idx} className="p-3 rounded-lg bg-nofx-bg border border-nofx-border space-y-2">
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={rule.name}
                        onChange={e => updateRule(idx, 'name', e.target.value)}
                        className="flex-1 px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                        placeholder={tr('ruleName')}
                      />
                      <select
                        value={rule.action}
                        onChange={e => updateRule(idx, 'action', e.target.value)}
                        className="px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                      >
                        <option value="buy">Buy</option>
                        <option value="sell">Sell</option>
                        <option value="hold">Hold</option>
                      </select>
                    </div>
                    <input
                      type="text"
                      value={rule.condition}
                      onChange={e => updateRule(idx, 'condition', e.target.value)}
                      className="w-full px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs font-mono"
                      placeholder="RSI_14 < 30 AND Close > EMA_20"
                    />
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-nofx-text-muted">{tr('confidence')}:</span>
                      <input
                        type="number"
                        min="0"
                        max="100"
                        value={rule.confidence}
                        onChange={e => updateRule(idx, 'confidence', parseInt(e.target.value))}
                        className="w-16 px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                      />
                      <span className="text-xs text-nofx-text-muted">{tr('priority')}:</span>
                      <input
                        type="number"
                        value={rule.priority}
                        onChange={e => updateRule(idx, 'priority', parseInt(e.target.value))}
                        className="w-16 px-2 py-1 rounded bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-xs"
                      />
                      <button
                        onClick={() => removeRule(idx)}
                        className="ml-auto text-nofx-danger hover:text-nofx-danger/80"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
                <button
                  onClick={addRule}
                  className="w-full py-2 rounded-lg border border-dashed border-nofx-border hover:border-nofx-gold text-nofx-text-muted hover:text-nofx-gold text-xs flex items-center justify-center gap-1 transition-colors"
                >
                  <Plus className="w-3.5 h-3.5" />
                  {tr('addRule')}
                </button>
              </div>
            </CollapsibleSection>
          )}

          {/* Parameters Section */}
          <CollapsibleSection
            title={tr('parameters')}
            icon={Settings}
            isOpen={expandedSection === 'parameters'}
            onToggle={() => setExpandedSection(expandedSection === 'parameters' ? null : 'parameters')}
          >
            <div className="grid grid-cols-2 gap-3">
              <NumberField
                label={tr('lookbackPeriods')}
                value={formState.config.parameters.lookback_periods}
                onChange={v => updateParameters('lookback_periods', v)}
              />
              <NumberField
                label={tr('entryThreshold')}
                value={formState.config.parameters.entry_threshold}
                step={0.1}
                onChange={v => updateParameters('entry_threshold', v)}
              />
              <NumberField
                label={tr('exitThreshold')}
                value={formState.config.parameters.exit_threshold}
                step={0.1}
                onChange={v => updateParameters('exit_threshold', v)}
              />
              <NumberField
                label={tr('maxHoldTime')}
                value={formState.config.parameters.max_position_hold_time}
                onChange={v => updateParameters('max_position_hold_time', v)}
              />
              <NumberField
                label={tr('minHoldTime')}
                value={formState.config.parameters.min_position_hold_time}
                onChange={v => updateParameters('min_position_hold_time', v)}
              />
              <NumberField
                label={tr('maxDailyTrades')}
                value={formState.config.parameters.max_daily_trades}
                onChange={v => updateParameters('max_daily_trades', v)}
              />
            </div>
          </CollapsibleSection>

          {/* Signal Config Section */}
          <CollapsibleSection
            title={tr('signalConfig')}
            icon={Activity}
            isOpen={expandedSection === 'signalConfig'}
            onToggle={() => setExpandedSection(expandedSection === 'signalConfig' ? null : 'signalConfig')}
          >
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-nofx-text-muted mb-1 block">{tr('signalType')}</label>
                  <select
                    value={formState.config.signal_config.signal_type}
                    onChange={e => updateSignalConfig('signal_type', e.target.value)}
                    className="w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:border-nofx-gold outline-none"
                  >
                    <option value="discrete">Discrete</option>
                    <option value="continuous">Continuous</option>
                    <option value="probabilistic">Probabilistic</option>
                  </select>
                </div>
                <NumberField
                  label={tr('minConfidence')}
                  value={formState.config.signal_config.min_confidence}
                  min={0}
                  max={100}
                  onChange={v => updateSignalConfig('min_confidence', v)}
                />
              </div>
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={formState.config.signal_config.require_confirmation}
                    onChange={e => updateSignalConfig('require_confirmation', e.target.checked)}
                    className="rounded border-nofx-border bg-nofx-bg text-nofx-gold"
                  />
                  <span className="text-sm text-nofx-text">{tr('requireConfirmation')}</span>
                </label>
                {formState.config.signal_config.require_confirmation && (
                  <NumberField
                    label={tr('confirmationDelay')}
                    value={formState.config.signal_config.confirmation_delay}
                    min={0}
                    onChange={v => updateSignalConfig('confirmation_delay', v)}
                  />
                )}
              </div>
            </div>
          </CollapsibleSection>

          {/* Templates (only when creating) */}
          {isCreating && templates.length > 0 && (
            <div className="p-3 rounded-lg bg-nofx-bg border border-nofx-gold/30">
              <div className="flex items-center gap-2 mb-2">
                <Info className="w-4 h-4 text-nofx-gold" />
                <span className="text-sm font-medium text-nofx-gold">{tr('useTemplate')}</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {templates.map(template => (
                  <button
                    key={template.id}
                    onClick={() => setFormState(prev => ({
                      ...prev,
                      model_type: template.model_type,
                      config: template.config,
                    }))}
                    className="px-3 py-1.5 rounded-lg bg-nofx-bg-lighter border border-nofx-border hover:border-nofx-gold text-xs text-nofx-text transition-colors"
                  >
                    {template.name}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Action Buttons */}
          <div className="flex items-center justify-end gap-2 pt-2 border-t border-nofx-border">
            <button
              onClick={() => {
                setIsCreating(false)
                setIsEditing(false)
                setFormState({
                  name: '',
                  description: '',
                  model_type: 'indicator_based',
                  config: getDefaultConfig(),
                })
              }}
              className="px-4 py-2 rounded-lg text-sm font-medium text-nofx-text-muted hover:text-nofx-text transition-colors"
            >
              {tr('cancel')}
            </button>
            <button
              onClick={isCreating ? handleCreate : handleUpdate}
              disabled={!formState.name.trim()}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 bg-nofx-gold text-black hover:bg-yellow-500"
            >
              <Save className="w-4 h-4" />
              {isCreating ? tr('creating') : tr('saving')}
            </button>
          </div>
        </div>
      )}

      {/* Model Details (when selected but not editing) */}
      {selectedModel && !isCreateOrEdit && (
        <div className="p-4 rounded-lg bg-nofx-bg border border-nofx-gold/20 space-y-4">
          <div className="flex items-start justify-between">
            <div>
              <h3 className="font-medium text-nofx-text">{selectedModel.name}</h3>
              <p className="text-xs text-nofx-text-muted mt-1">{selectedModel.description}</p>
              <span className="inline-block mt-2 px-2 py-0.5 rounded bg-nofx-bg-lighter text-xs text-nofx-text-muted">
                {tr(selectedModel.model_type)}
              </span>
            </div>
            <button
              onClick={() => startEditing(selectedModel)}
              className="p-2 rounded-lg hover:bg-white/10 text-nofx-text-muted hover:text-nofx-gold transition-colors"
            >
              <Settings className="w-4 h-4" />
            </button>
          </div>

          {/* Stats */}
          {(selectedModel.win_rate > 0 || selectedModel.backtest_count > 0) && (
            <div className="grid grid-cols-4 gap-2 pt-3 border-t border-nofx-border">
              <StatBox
                label={tr('winRate')}
                value={selectedModel.win_rate > 0 ? `${(selectedModel.win_rate * 100).toFixed(1)}%` : '-'}
                color={selectedModel.win_rate > 0.5 ? 'green' : selectedModel.win_rate > 0.3 ? 'yellow' : 'red'}
              />
              <StatBox
                label={tr('avgProfit')}
                value={selectedModel.avg_profit_pct > 0 ? `+${selectedModel.avg_profit_pct.toFixed(2)}%` : 
                      selectedModel.avg_profit_pct < 0 ? `${selectedModel.avg_profit_pct.toFixed(2)}%` : '-'}
                color={selectedModel.avg_profit_pct > 0 ? 'green' : selectedModel.avg_profit_pct < 0 ? 'red' : 'neutral'}
              />
              <StatBox
                label={tr('maxDrawdown')}
                value={selectedModel.max_drawdown_pct > 0 ? `-${selectedModel.max_drawdown_pct.toFixed(2)}%` : '-'}
                color="red"
              />
              <StatBox
                label={tr('sharpeRatio')}
                value={selectedModel.sharpe_ratio > 0 ? selectedModel.sharpe_ratio.toFixed(2) : '-'}
                color={selectedModel.sharpe_ratio > 1 ? 'green' : selectedModel.sharpe_ratio > 0 ? 'yellow' : 'neutral'}
              />
            </div>
          )}

          {/* Config Preview */}
          <div className="pt-3 border-t border-nofx-border">
            <div className="flex items-center gap-2 text-xs text-nofx-text-muted">
              <Code2 className="w-3.5 h-3.5" />
              <span>Config Preview</span>
            </div>
            <pre className="mt-2 p-3 rounded-lg bg-nofx-bg-lighter text-[10px] font-mono text-nofx-text-muted overflow-auto max-h-[150px]">
              {JSON.stringify(selectedModel.config, null, 2)}
            </pre>
          </div>
        </div>
      )}

      {/* Import Dialog */}
      {showImportDialog && (
        <ImportDialog
          title={tr('importDialogTitle')}
          placeholder={tr('pasteImportData')}
          onImport={handleImport}
          onCancel={() => setShowImportDialog(false)}
        />
      )}
    </div>
  )
}

// Helper Components

function CollapsibleSection({
  title,
  icon: Icon,
  isOpen,
  onToggle,
  children,
}: {
  title: string
  icon: typeof Brain
  isOpen: boolean
  onToggle: () => void
  children: React.ReactNode
}) {
  return (
    <div className="rounded-lg border border-nofx-border overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-3 py-2 bg-nofx-bg-lighter hover:bg-white/5 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Icon className="w-4 h-4 text-nofx-gold" />
          <span className="text-sm font-medium text-nofx-text">{title}</span>
        </div>
        {isOpen ? (
          <ChevronUp className="w-4 h-4 text-nofx-text-muted" />
        ) : (
          <ChevronDown className="w-4 h-4 text-nofx-text-muted" />
        )}
      </button>
      {isOpen && (
        <div className="p-3 space-y-3">
          {children}
        </div>
      )}
    </div>
  )
}

function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
}: {
  label: string
  value: number
  onChange: (value: number) => void
  min?: number
  max?: number
  step?: number
}) {
  return (
    <div>
      <label className="text-xs text-nofx-text-muted mb-1 block">{label}</label>
      <input
        type="number"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={e => onChange(parseFloat(e.target.value))}
        className="w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:border-nofx-gold outline-none"
      />
    </div>
  )
}

function StatBox({
  label,
  value,
  color,
}: {
  label: string
  value: string
  color: 'green' | 'red' | 'yellow' | 'neutral'
}) {
  const colorClasses = {
    green: 'text-nofx-success',
    red: 'text-nofx-danger',
    yellow: 'text-nofx-gold',
    neutral: 'text-nofx-text',
  }

  return (
    <div className="text-center">
      <div className="text-xs text-nofx-text-muted">{label}</div>
      <div className={`text-sm font-medium ${colorClasses[color]}`}>{value}</div>
    </div>
  )
}

function ImportDialog({
  title,
  placeholder,
  onImport,
  onCancel,
}: {
  title: string
  placeholder: string
  onImport: (data: string) => void
  onCancel: () => void
}) {
  const [data, setData] = useState('')

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="w-full max-w-lg bg-nofx-bg border border-nofx-gold/20 rounded-xl p-6 shadow-2xl">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-medium text-nofx-text">{title}</h3>
          <button onClick={onCancel} className="text-nofx-text-muted hover:text-nofx-text">
            <X className="w-5 h-5" />
          </button>
        </div>
        <textarea
          value={data}
          onChange={e => setData(e.target.value)}
          className="w-full h-48 px-4 py-3 rounded-lg bg-nofx-bg-lighter border border-nofx-border text-nofx-text text-sm font-mono focus:border-nofx-gold outline-none resize-none"
          placeholder={placeholder}
        />
        <div className="flex items-center justify-end gap-3 mt-4">
          <button
            onClick={onCancel}
            className="px-4 py-2 rounded-lg text-sm text-nofx-text-muted hover:text-nofx-text transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => onImport(data)}
            disabled={!data.trim()}
            className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 bg-nofx-gold text-black hover:bg-yellow-500"
          >
            <Upload className="w-4 h-4" />
            Import
          </button>
        </div>
      </div>
    </div>
  )
}

function getDefaultConfig(): QuantModelConfig {
  return {
    type: 'indicator_based',
    indicators: [
      { name: 'RSI', period: 14, timeframe: '1h', weight: 0.4, params: {} },
      { name: 'EMA', period: 20, timeframe: '1h', weight: 0.3, params: { second_period: 50 } },
      { name: 'MACD', period: 12, timeframe: '1h', weight: 0.3, params: { fast: 12, slow: 26, signal: 9 } },
    ],
    rules: [],
    parameters: {
      lookback_periods: 100,
      entry_threshold: 70,
      exit_threshold: 30,
      max_position_hold_time: 48,
      min_position_hold_time: 4,
      max_daily_trades: 3,
    },
    signal_config: {
      signal_type: 'discrete',
      min_confidence: 65,
      require_confirmation: true,
      confirmation_delay: 1,
    },
  }
}
