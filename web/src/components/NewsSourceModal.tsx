import { useState, useEffect } from 'react';
import { X, Plus, Trash2, AlertCircle } from 'lucide-react';
import { useLanguage } from '../contexts/LanguageContext';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../lib/api';

interface NewsConfigData {
  id?: number;
  enabled: boolean;
  news_sources: string;
  news_sources_list?: string[];
  auto_fetch_interval_minutes: number;
  max_articles_per_fetch: number;
  sentiment_threshold: number;
}

interface NewsSourceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave?: (data: NewsConfigData) => Promise<void>;
  initialData?: NewsConfigData | null;
}

const AVAILABLE_SOURCES = [
  { id: 'mlion', label: 'Mlion', description: '加密货币市场新闻' },
  { id: 'twitter', label: 'Twitter', description: 'Twitter 热点信息' },
  { id: 'reddit', label: 'Reddit', description: 'Reddit 社区讨论' },
  { id: 'telegram', label: 'Telegram', description: 'Telegram 频道更新' },
];

export function NewsSourceModal({
  isOpen,
  onClose,
  onSave,
  initialData,
}: NewsSourceModalProps) {
  const { language } = useLanguage();
  const { user, token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [formData, setFormData] = useState<NewsConfigData>({
    enabled: true,
    news_sources: 'mlion',
    auto_fetch_interval_minutes: 5,
    max_articles_per_fetch: 10,
    sentiment_threshold: 0.0,
  });

  // 初始化表单数据
  useEffect(() => {
    if (initialData) {
      setFormData(initialData);
    }
  }, [initialData]);

  if (!isOpen) return null;

  // 获取选中的新闻源列表
  const selectedSources = formData.news_sources
    .split(',')
    .map(s => s.trim())
    .filter(s => s);

  // 切换新闻源
  const toggleSource = (sourceId: string) => {
    const sources = selectedSources;
    const index = sources.indexOf(sourceId);

    if (index > -1) {
      sources.splice(index, 1);
    } else {
      sources.push(sourceId);
    }

    if (sources.length === 0) {
      setError('必须至少选择一个新闻源');
      return;
    }

    setFormData({
      ...formData,
      news_sources: sources.join(','),
    });
    setError(null);
  };

  // 处理保存
  const handleSave = async () => {
    // 验证
    if (selectedSources.length === 0) {
      setError('必须至少选择一个新闻源');
      return;
    }

    if (
      formData.auto_fetch_interval_minutes < 1 ||
      formData.auto_fetch_interval_minutes > 1440
    ) {
      setError('抓取间隔必须在1-1440分钟之间');
      return;
    }

    if (
      formData.max_articles_per_fetch < 1 ||
      formData.max_articles_per_fetch > 100
    ) {
      setError('每次抓取的最大文章数必须在1-100之间');
      return;
    }

    if (
      formData.sentiment_threshold < -1.0 ||
      formData.sentiment_threshold > 1.0
    ) {
      setError('情绪阈值必须在-1.0到1.0之间');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      if (onSave) {
        await onSave(formData);
      } else {
        // 调用API保存
        if (!token) {
          setError('未授权，请重新登录');
          return;
        }

        const response = await fetch('/api/user/news-config', {
          method: initialData?.id ? 'PUT' : 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            enabled: formData.enabled,
            news_sources: formData.news_sources,
            auto_fetch_interval_minutes: formData.auto_fetch_interval_minutes,
            max_articles_per_fetch: formData.max_articles_per_fetch,
            sentiment_threshold: formData.sentiment_threshold,
          }),
        });

        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.message || '保存失败');
        }
      }

      setSuccess(true);
      setTimeout(() => {
        onClose();
        setSuccess(false);
      }, 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-900 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        {/* 头部 */}
        <div className="sticky top-0 bg-white dark:bg-gray-900 border-b dark:border-gray-700 px-6 py-4 flex items-center justify-between">
          <h2 className="text-xl font-bold dark:text-white">新闻源配置</h2>
          <button
            onClick={onClose}
            className="p-1 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
          >
            <X size={24} className="dark:text-gray-400" />
          </button>
        </div>

        {/* 内容 */}
        <div className="p-6 space-y-6">
          {/* 启用开关 */}
          <div className="flex items-center justify-between">
            <label className="block">
              <span className="text-sm font-medium dark:text-gray-300">
                启用新闻功能
              </span>
            </label>
            <button
              onClick={() =>
                setFormData({ ...formData, enabled: !formData.enabled })
              }
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                formData.enabled
                  ? 'bg-blue-500'
                  : 'bg-gray-300 dark:bg-gray-600'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                  formData.enabled ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* 新闻源选择 */}
          <div>
            <label className="block text-sm font-medium dark:text-gray-300 mb-3">
              新闻源
            </label>
            <div className="space-y-2">
              {AVAILABLE_SOURCES.map(source => (
                <label
                  key={source.id}
                  className="flex items-center p-3 border dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer transition-colors"
                >
                  <input
                    type="checkbox"
                    checked={selectedSources.includes(source.id)}
                    onChange={() => toggleSource(source.id)}
                    className="w-4 h-4 rounded"
                  />
                  <div className="ml-3 flex-1">
                    <p className="font-medium dark:text-white">{source.label}</p>
                    <p className="text-sm text-gray-600 dark:text-gray-400">
                      {source.description}
                    </p>
                  </div>
                </label>
              ))}
            </div>
          </div>

          {/* 抓取间隔 */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium dark:text-gray-300 mb-2">
                自动抓取间隔 (分钟)
              </label>
              <input
                type="number"
                min="1"
                max="1440"
                value={formData.auto_fetch_interval_minutes}
                onChange={e =>
                  setFormData({
                    ...formData,
                    auto_fetch_interval_minutes: parseInt(e.target.value) || 5,
                  })
                }
                className="w-full px-3 py-2 border dark:border-gray-700 rounded-lg dark:bg-gray-800 dark:text-white"
              />
            </div>

            {/* 每次抓取最大文章数 */}
            <div>
              <label className="block text-sm font-medium dark:text-gray-300 mb-2">
                每次最多文章数
              </label>
              <input
                type="number"
                min="1"
                max="100"
                value={formData.max_articles_per_fetch}
                onChange={e =>
                  setFormData({
                    ...formData,
                    max_articles_per_fetch: parseInt(e.target.value) || 10,
                  })
                }
                className="w-full px-3 py-2 border dark:border-gray-700 rounded-lg dark:bg-gray-800 dark:text-white"
              />
            </div>
          </div>

          {/* 情绪阈值 */}
          <div>
            <label className="block text-sm font-medium dark:text-gray-300 mb-2">
              情绪阈值 ({formData.sentiment_threshold.toFixed(2)})
            </label>
            <input
              type="range"
              min="-1"
              max="1"
              step="0.1"
              value={formData.sentiment_threshold}
              onChange={e =>
                setFormData({
                  ...formData,
                  sentiment_threshold: parseFloat(e.target.value),
                })
              }
              className="w-full"
            />
            <div className="flex justify-between text-xs text-gray-600 dark:text-gray-400 mt-1">
              <span>极度负面 (-1.0)</span>
              <span>中立 (0.0)</span>
              <span>极度正面 (1.0)</span>
            </div>
          </div>

          {/* 错误信息 */}
          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-700 rounded-lg text-red-700 dark:text-red-200">
              <AlertCircle size={20} />
              <span>{error}</span>
            </div>
          )}

          {/* 成功信息 */}
          {success && (
            <div className="flex items-center gap-2 p-3 bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-700 rounded-lg text-green-700 dark:text-green-200">
              <Plus size={20} />
              <span>保存成功！</span>
            </div>
          )}
        </div>

        {/* 底部 */}
        <div className="sticky bottom-0 bg-white dark:bg-gray-900 border-t dark:border-gray-700 px-6 py-4 flex gap-3 justify-end">
          <button
            onClick={onClose}
            disabled={loading}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 border dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors disabled:opacity-50"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            disabled={loading}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {loading ? (
              <>
                <span className="inline-block animate-spin">⚙️</span>
                保存中...
              </>
            ) : (
              '保存配置'
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
