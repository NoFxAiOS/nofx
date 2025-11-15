import { useState, useEffect } from 'react'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import {
  Plus,
  Edit,
  Trash2,
  FileText,
  Lock,
  User,
  BookOpen,
} from 'lucide-react'
import StrategyEditor from '../components/StrategyEditor'
import { getAuthHeaders } from '../lib/api'

interface Strategy {
  name: string
  content?: string
  isSystem: boolean
  fileName?: string
}

export default function StrategiesPage() {
  const { language } = useLanguage()
  const [strategies, setStrategies] = useState<Strategy[]>([])
  const [loading, setLoading] = useState(true)
  const [isEditorOpen, setIsEditorOpen] = useState(false)
  const [editingStrategy, setEditingStrategy] = useState<Strategy | null>(null)
  const [viewingStrategy, setViewingStrategy] = useState<Strategy | null>(null)

  useEffect(() => {
    loadStrategies()
  }, [])

  const requireAuthHeaders = () => {
    const token = localStorage.getItem('auth_token')
    if (!token) {
      alert(
        language === 'zh'
          ? '登录已过期，请重新登录后再试'
          : 'Login expired, please sign in again and retry.'
      )
      return null
    }
    return getAuthHeaders()
  }

  const loadStrategies = async () => {
    try {
      const response = await fetch('/api/prompt-templates')

      if (response.ok) {
        const data = await response.json()
        const allStrategies: Strategy[] = data.templates.map((t: any) => ({
          name: t.name,
          isSystem: !t.name.startsWith('user_'),
          fileName: t.name,
        }))
        setStrategies(allStrategies)
      }
    } catch (error) {
      console.error('Failed to load strategies:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleCreateNew = () => {
    setEditingStrategy(null)
    setIsEditorOpen(true)
  }

  const handleEdit = async (strategy: Strategy) => {
    try {
      const response = await fetch(`/api/prompt-templates/${strategy.name}`)

      if (response.ok) {
        const data = await response.json()
        setEditingStrategy({
          ...strategy,
          content: data.content,
        })
        setIsEditorOpen(true)
      }
    } catch (error) {
      console.error('Failed to load strategy content:', error)
    }
  }

  const handleView = async (strategy: Strategy) => {
    try {
      const response = await fetch(`/api/prompt-templates/${strategy.name}`)

      if (response.ok) {
        const data = await response.json()
        setViewingStrategy({
          ...strategy,
          content: data.content,
        })
      }
    } catch (error) {
      console.error('Failed to load strategy content:', error)
    }
  }

  const handleDelete = async (strategy: Strategy) => {
    if (!confirm(t('confirmDelete', language))) {
      return
    }

    try {
      const headers = requireAuthHeaders()
      if (!headers) return
      // Extract pure strategy name from template ID (removes user_<userid>_ prefix and .txt suffix)
      // e.g., "user_123_mystrategy" -> "mystrategy"
      const parts = strategy.name.split('_')
      let strategyName = strategy.name
      
      // If it starts with user_, remove the first two parts (user and userid)
      if (parts.length >= 3 && parts[0] === 'user') {
        strategyName = parts.slice(2).join('_')
      }
      
      // Remove .txt suffix if present
      strategyName = strategyName.replace(/\.txt$/, '')

      const response = await fetch(`/api/strategies/${strategyName}`, {
        method: 'DELETE',
        headers,
      })

      if (response.ok) {
        alert(t('strategyDeletedSuccess', language))
        loadStrategies()
      } else {
        const error = await response.json()
        alert(error.error || 'Failed to delete strategy')
      }
    } catch (error) {
      console.error('Failed to delete strategy:', error)
      alert('Failed to delete strategy')
    }
  }

  const handleSave = async (name: string, content: string) => {
    try {
      const isUpdate = editingStrategy !== null

      const url = isUpdate ? `/api/strategies/${name}` : '/api/strategies'
      const method = isUpdate ? 'PUT' : 'POST'

      const headers = requireAuthHeaders()
      if (!headers) return

      headers['Content-Type'] = 'application/json'

      const response = await fetch(url, {
        method,
        headers,
        body: JSON.stringify({
          name,
          content,
          description: '',
        }),
      })

      if (response.ok) {
        alert(
          t(
            isUpdate ? 'strategyUpdatedSuccess' : 'strategyCreatedSuccess',
            language
          )
        )
        setIsEditorOpen(false)
        loadStrategies()
      } else {
        const error = await response.json()
        alert(error.error || 'Failed to save strategy')
      }
    } catch (error) {
      console.error('Failed to save strategy:', error)
      alert('Failed to save strategy')
    }
  }

  const systemStrategies = strategies.filter((s) => s.isSystem)
  const userStrategies = strategies.filter((s) => !s.isSystem)

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-400">Loading...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-6">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-2 flex items-center gap-3">
            <BookOpen className="w-8 h-8 text-blue-400" />
            {t('strategyManagement', language)}
          </h1>
          <p className="text-gray-400">
            {language === 'zh'
              ? '创建和管理您的交易策略，策略将自动保存到 prompts 文件夹'
              : 'Create and manage your trading strategies, automatically saved to prompts folder'}
          </p>
        </div>

        {/* Create Button */}
        <div className="mb-6">
          <button
            onClick={handleCreateNew}
            className="flex items-center gap-2 px-6 py-3 bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors"
          >
            <Plus className="w-5 h-5" />
            {t('createStrategy', language)}
          </button>
        </div>

        {/* User Strategies */}
        <div className="mb-8">
          <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
            <User className="w-5 h-5 text-green-400" />
            {t('myStrategies', language)}
          </h2>

          {userStrategies.length === 0 ? (
            <div className="bg-gray-800/50 rounded-xl p-12 text-center">
              <FileText className="w-16 h-16 text-gray-600 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-gray-400 mb-2">
                {t('noStrategies', language)}
              </h3>
              <p className="text-gray-500">{t('noStrategiesDesc', language)}</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {userStrategies.map((strategy) => (
                <StrategyCard
                  key={strategy.name}
                  strategy={strategy}
                  language={language}
                  onEdit={handleEdit}
                  onDelete={handleDelete}
                  onView={handleView}
                />
              ))}
            </div>
          )}
        </div>

        {/* System Templates */}
        <div>
          <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
            <Lock className="w-5 h-5 text-purple-400" />
            {t('systemTemplates', language)}
          </h2>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {systemStrategies.map((strategy) => (
              <StrategyCard
                key={strategy.name}
                strategy={strategy}
                language={language}
                onView={handleView}
              />
            ))}
          </div>
        </div>
      </div>

      {/* Strategy Editor Modal */}
      {isEditorOpen && (
        <StrategyEditor
          strategy={editingStrategy}
          onSave={handleSave}
          onClose={() => setIsEditorOpen(false)}
          language={language}
        />
      )}

      {/* View Modal */}
      {viewingStrategy && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
          <div className="bg-gray-800 rounded-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
            <div className="p-6 border-b border-gray-700 flex items-center justify-between">
              <h3 className="text-xl font-semibold text-white flex items-center gap-2">
                <FileText className="w-5 h-5" />
                {viewingStrategy.name}
                {viewingStrategy.isSystem && (
                  <span className="text-xs px-2 py-1 bg-purple-500/20 text-purple-400 rounded">
                    {t('systemTemplate', language)}
                  </span>
                )}
              </h3>
              <button
                onClick={() => setViewingStrategy(null)}
                className="text-gray-400 hover:text-white"
                aria-label="Close"
              >
                ✕
              </button>
            </div>
            <div className="p-6 overflow-y-auto flex-1">
              <pre className="text-sm text-gray-300 whitespace-pre-wrap font-mono bg-gray-900/50 p-4 rounded-lg">
                {viewingStrategy.content}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

interface StrategyCardProps {
  strategy: Strategy
  language: 'en' | 'zh'
  onEdit?: (strategy: Strategy) => void
  onDelete?: (strategy: Strategy) => void
  onView: (strategy: Strategy) => void
}

function StrategyCard({
  strategy,
  language,
  onEdit,
  onDelete,
  onView,
}: StrategyCardProps) {
  const isSystem = strategy.isSystem
  const displayName = strategy.name.replace(/^user_[^_]+_/, '')

  return (
    <div className="bg-gray-800/70 rounded-xl p-5 border border-gray-700/50 hover:border-gray-600 transition-all">
      <div className="flex items-start justify-between mb-3">
        <div className="flex-1">
          <h3 className="text-lg font-medium text-white mb-1 flex items-center gap-2">
            {displayName}
            {isSystem && <Lock className="w-4 h-4 text-purple-400" />}
          </h3>
          <span
            className={`text-xs px-2 py-1 rounded ${
              isSystem
                ? 'bg-purple-500/20 text-purple-400'
                : 'bg-green-500/20 text-green-400'
            }`}
          >
            {t(isSystem ? 'systemTemplate' : 'userStrategy', language)}
          </span>
        </div>
      </div>

      <div className="flex gap-2 mt-4">
        <button
          onClick={() => onView(strategy)}
          className="flex-1 px-3 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded transition-colors flex items-center justify-center gap-1"
        >
          <FileText className="w-4 h-4" />
          {t('viewTemplate', language)}
        </button>

        {!isSystem && onEdit && (
          <button
            onClick={() => onEdit(strategy)}
            className="px-3 py-2 bg-blue-500/20 hover:bg-blue-500/30 text-blue-400 text-sm rounded transition-colors"
            aria-label="Edit strategy"
          >
            <Edit className="w-4 h-4" />
          </button>
        )}

        {!isSystem && onDelete && (
          <button
            onClick={() => onDelete(strategy)}
            className="px-3 py-2 bg-red-500/20 hover:bg-red-500/30 text-red-400 text-sm rounded transition-colors"
            aria-label="Delete strategy"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        )}
      </div>
    </div>
  )
}
