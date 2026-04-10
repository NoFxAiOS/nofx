import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import type { QuantModel, QuantModelConfig, QuantModelIntegration } from '../types'
import { notify, confirmToast } from '../lib/notify'
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
  Info,
  CheckCircle2,
  Code2,
  ArrowLeft,
} from 'lucide-react'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'

const API_BASE = import.meta.env.VITE_API_BASE || ''

export default function QuantModelsPage() {
  const { token } = useAuth()
  const { language } = useLanguage()

  const [models, setModels] = useState<QuantModel[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedModelId, setSelectedModelId] = useState<string | null>(null)

  // Form state for creating/editing
  const [isCreating, setIsCreating] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [showImportDialog, setShowImportDialog] = useState(false)

  const tr = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      en: {
        'title': 'Quant Models',
        'subtitle': 'Create and manage custom trading models',
        'createNew': 'Create New Model',
        'importModel': 'Import',
        'exportModel': 'Export',
        'cloneModel': 'Clone',
        'deleteModel': 'Delete',
        'editModel': 'Edit',
        'back': 'Back to List',
        'modelDetails': 'Model Details',
        'noModels': 'No quant models yet. Create or import one to get started.',
        'modelCreated': 'Model created successfully',
        'modelUpdated': 'Model updated successfully',
        'modelDeleted': 'Model deleted successfully',
        'modelCloned': 'Model cloned successfully',
        'confirmDelete': 'Are you sure you want to delete this model?',
        'backtestStats': 'Backtest Statistics',
        'winRate': 'Win Rate',
        'avgProfit': 'Avg Profit',
        'maxDrawdown': 'Max Drawdown',
        'sharpeRatio': 'Sharpe Ratio',
        'usageCount': 'Usage Count',
        'backtestCount': 'Backtest Count',
        'publicModel': 'Public',
        'privateModel': 'Private',
        'modelType': 'Model Type',
        'indicatorBased': 'Indicator Based',
        'ruleBased': 'Rule Based',
        'mlClassifier': 'ML Classifier',
        'ensemble': 'Ensemble',
        'created': 'Created',
        'updated': 'Updated',
      },
      zh: {
        'title': '量化模型',
        'subtitle': '创建和管理自定义交易模型',
        'createNew': '创建新模型',
        'importModel': '导入',
        'exportModel': '导出',
        'cloneModel': '克隆',
        'deleteModel': '删除',
        'editModel': '编辑',
        'back': '返回列表',
        'modelDetails': '模型详情',
        'noModels': '暂无量化模型。创建或导入一个开始使用。',
        'modelCreated': '模型创建成功',
        'modelUpdated': '模型更新成功',
        'modelDeleted': '模型删除成功',
        'modelCloned': '模型克隆成功',
        'confirmDelete': '确定要删除此模型吗？',
        'backtestStats': '回测统计',
        'winRate': '胜率',
        'avgProfit': '平均收益',
        'maxDrawdown': '最大回撤',
        'sharpeRatio': '夏普比率',
        'usageCount': '使用次数',
        'backtestCount': '回测次数',
        'publicModel': '公开',
        'privateModel': '私有',
        'modelType': '模型类型',
        'indicatorBased': '指标型',
        'ruleBased': '规则型',
        'mlClassifier': '机器学习分类器',
        'ensemble': '集成模型',
        'created': '创建时间',
        'updated': '更新时间',
      },
    }
    return translations[language]?.[key] || translations['en'][key] || key
  }

  // Fetch models on mount
  useEffect(() => {
    fetchModels()
  }, [token])

  const fetchModels = async () => {
    if (!token) return
    setIsLoading(true)
    try {
      const response = await fetch(`${API_BASE}/api/quant-models`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (response.ok) {
        const data = await response.json()
        setModels(data.models || [])
      }
    } catch (err) {
      console.error('Failed to fetch models:', err)
    } finally {
      setIsLoading(false)
    }
  }

  const handleDelete = async (modelId: string) => {
    if (!token) return

    const confirmed = await confirmToast(tr('confirmDelete'), {
      title: tr('deleteModel'),
      okText: tr('deleteModel'),
      cancelText: tr('back'),
    })

    if (!confirmed) return

    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${modelId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })

      if (!response.ok) throw new Error('Failed to delete model')

      notify.success(tr('modelDeleted'))
      await fetchModels()
      if (selectedModelId === modelId) {
        setSelectedModelId(null)
      }
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

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

      notify.success(tr('exportModel'))
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  const handleClone = async (model: QuantModel) => {
    if (!token) return

    try {
      const response = await fetch(`${API_BASE}/api/quant-models/${model.id}/clone`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
      })

      if (!response.ok) throw new Error('Failed to clone model')

      notify.success(tr('modelCloned'))
      await fetchModels()
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Unknown error')
    }
  }

  const selectedModel = models.find(m => m.id === selectedModelId)

  if (isLoading) {
    return (
      <DeepVoidBackground className="h-[calc(100vh-64px)] flex items-center justify-center">
        <div className="text-center">
          <div className="relative">
            <div className="w-16 h-16 rounded-full border-4 border-purple-500/20 border-t-purple-500 animate-spin" />
            <Brain className="w-6 h-6 text-purple-500 absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2" />
          </div>
        </div>
      </DeepVoidBackground>
    )
  }

  // Model Detail View
  if (selectedModel) {
    return (
      <DeepVoidBackground className="h-[calc(100vh-64px)] flex flex-col bg-nofx-bg relative overflow-hidden">
        {/* Header */}
        <div className="flex-shrink-0 px-6 py-4 border-b border-nofx-gold/20 bg-nofx-bg/60 backdrop-blur-md">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <button
                onClick={() => setSelectedModelId(null)}
                className="p-2 rounded-lg hover:bg-white/10 text-nofx-text-muted hover:text-nofx-text transition-colors"
              >
                <ArrowLeft className="w-5 h-5" />
              </button>
              <div>
                <h1 className="text-xl font-bold text-nofx-text">{selectedModel.name}</h1>
                <div className="flex items-center gap-3 mt-1">
                  <span className="text-xs text-nofx-text-muted">{tr('modelType')}: {tr(selectedModel.model_type)}</span>
                  {selectedModel.is_public && (
                    <span className="px-2 py-0.5 rounded text-xs bg-blue-500/20 text-blue-400">
                      {tr('publicModel')}
                    </span>
                  )}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => handleExport(selectedModel)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors bg-nofx-bg-lighter text-nofx-text hover:bg-white/10"
              >
                <Download className="w-4 h-4" />
                {tr('exportModel')}
              </button>
              <button
                onClick={() => handleClone(selectedModel)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors bg-nofx-bg-lighter text-nofx-text hover:bg-white/10"
              >
                <Copy className="w-4 h-4" />
                {tr('cloneModel')}
              </button>
              <button
                onClick={() => handleDelete(selectedModel.id)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors bg-nofx-danger/10 text-nofx-danger hover:bg-nofx-danger/20"
              >
                <Trash2 className="w-4 h-4" />
                {tr('deleteModel')}
              </button>
            </div>
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          <div className="max-w-4xl mx-auto space-y-6">
            {/* Description */}
            {selectedModel.description && (
              <div className="p-4 rounded-lg bg-nofx-bg-lighter border border-nofx-border">
                <p className="text-sm text-nofx-text">{selectedModel.description}</p>
              </div>
            )}

            {/* Stats */}
            {(selectedModel.win_rate > 0 || selectedModel.backtest_count > 0) && (
              <div className="p-4 rounded-lg bg-nofx-bg-lighter border border-nofx-border">
                <h3 className="text-sm font-medium text-nofx-text mb-4">{tr('backtestStats')}</h3>
                <div className="grid grid-cols-4 gap-4">
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
                <div className="mt-4 pt-4 border-t border-nofx-border flex items-center gap-6 text-xs text-nofx-text-muted">
                  <span>{tr('usageCount')}: {selectedModel.usage_count}</span>
                  <span>{tr('backtestCount')}: {selectedModel.backtest_count}</span>
                  <span>{tr('created')}: {new Date(selectedModel.created_at).toLocaleDateString()}</span>
                  <span>{tr('updated')}: {new Date(selectedModel.updated_at).toLocaleDateString()}</span>
                </div>
              </div>
            )}

            {/* Config */}
            <div className="p-4 rounded-lg bg-nofx-bg-lighter border border-nofx-border">
              <h3 className="text-sm font-medium text-nofx-text mb-4">Configuration</h3>
              <pre className="p-4 rounded-lg bg-nofx-bg text-xs font-mono text-nofx-text-muted overflow-auto max-h-[400px]">
                {JSON.stringify(selectedModel.config, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      </DeepVoidBackground>
    )
  }

  // Model List View
  return (
    <DeepVoidBackground className="h-[calc(100vh-64px)] flex flex-col bg-nofx-bg relative overflow-hidden">
      {/* Header */}
      <div className="flex-shrink-0 px-6 py-4 border-b border-nofx-gold/20 bg-nofx-bg/60 backdrop-blur-md">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="p-3 rounded-lg bg-gradient-to-br from-purple-500 to-purple-600">
              <Brain className="w-6 h-6 text-white" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-nofx-text">{tr('title')}</h1>
              <p className="text-sm text-nofx-text-muted">{tr('subtitle')}</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowImportDialog(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors bg-nofx-bg-lighter text-nofx-text hover:bg-white/10"
            >
              <Upload className="w-4 h-4" />
              {tr('importModel')}
            </button>
            <button
              onClick={() => {
                setIsCreating(true)
                // Navigate to create page or show modal
              }}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors bg-nofx-gold text-black hover:bg-yellow-500"
            >
              <Plus className="w-4 h-4" />
              {tr('createNew')}
            </button>
          </div>
        </div>
      </div>

      {/* Model List */}
      <div className="flex-1 overflow-y-auto p-6">
        {models.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-nofx-text-muted">
            <Brain className="w-20 h-20 mb-4 opacity-20" />
            <p className="text-lg">{tr('noModels')}</p>
          </div>
        ) : (
          <div className="max-w-4xl mx-auto space-y-4">
            {models.map((model) => (
              <div
                key={model.id}
                onClick={() => setSelectedModelId(model.id)}
                className="p-4 rounded-xl bg-nofx-bg-lighter border border-nofx-border hover:border-nofx-gold/50 cursor-pointer transition-all hover:bg-white/5"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg font-medium text-nofx-text">{model.name}</h3>
                      {model.is_public && (
                        <span className="px-2 py-0.5 rounded text-xs bg-blue-500/20 text-blue-400">
                          {tr('publicModel')}
                        </span>
                      )}
                      <span className="px-2 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                        {tr(model.model_type)}
                      </span>
                    </div>
                    {model.description && (
                      <p className="text-sm text-nofx-text-muted line-clamp-2">{model.description}</p>
                    )}
                    <div className="flex items-center gap-6 mt-3 text-xs text-nofx-text-muted">
                      {model.win_rate > 0 && (
                        <span className="flex items-center gap-1">
                          <TrendingUp className="w-3 h-3" />
                          {tr('winRate')}: {(model.win_rate * 100).toFixed(1)}%
                        </span>
                      )}
                      {model.backtest_count > 0 && (
                        <span className="flex items-center gap-1">
                          <Activity className="w-3 h-3" />
                          {tr('backtestCount')}: {model.backtest_count}
                        </span>
                      )}
                      {model.usage_count > 0 && (
                        <span className="flex items-center gap-1">
                          <CheckCircle2 className="w-3 h-3" />
                          {tr('usageCount')}: {model.usage_count}
                        </span>
                      )}
                      <span>{tr('created')}: {new Date(model.created_at).toLocaleDateString()}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleExport(model) }}
                      className="p-2 rounded-lg hover:bg-white/10 text-nofx-text-muted hover:text-nofx-text transition-colors"
                      title={tr('exportModel')}
                    >
                      <Download className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleClone(model) }}
                      className="p-2 rounded-lg hover:bg-white/10 text-nofx-text-muted hover:text-nofx-text transition-colors"
                      title={tr('cloneModel')}
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDelete(model.id) }}
                      className="p-2 rounded-lg hover:bg-nofx-danger/20 text-nofx-text-muted hover:text-nofx-danger transition-colors"
                      title={tr('deleteModel')}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Import Dialog */}
      {showImportDialog && (
        <ImportDialog
          title={language === 'zh' ? '导入模型' : 'Import Model'}
          placeholder={language === 'zh' ? '在此粘贴导出的JSON数据' : 'Paste exported JSON data here'}
          onImport={async (data) => {
            if (!token) return
            try {
              const response = await fetch(`${API_BASE}/api/quant-models/import`, {
                method: 'POST',
                headers: {
                  'Content-Type': 'application/json',
                  Authorization: `Bearer ${token}`,
                },
                body: data,
              })
              if (!response.ok) throw new Error('Import failed')
              notify.success(tr('modelCreated'))
              await fetchModels()
              setShowImportDialog(false)
            } catch (err) {
              notify.error(err instanceof Error ? err.message : 'Import failed')
            }
          }}
          onCancel={() => setShowImportDialog(false)}
        />
      )}
    </DeepVoidBackground>
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
    <div className="text-center p-3 rounded-lg bg-nofx-bg">
      <div className="text-xs text-nofx-text-muted mb-1">{label}</div>
      <div className={`text-lg font-bold ${colorClasses[color]}`}>{value}</div>
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
