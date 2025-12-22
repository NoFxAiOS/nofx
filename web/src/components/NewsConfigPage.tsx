import { useState, useEffect } from 'react';
import { Settings, Newspaper, AlertTriangle } from 'lucide-react';
import { useLanguage } from '../contexts/LanguageContext';
import { useAuth } from '../contexts/AuthContext';
import { NewsSourceModal } from './NewsSourceModal';

interface NewsConfig {
  id: number;
  user_id: string;
  enabled: boolean;
  news_sources: string;
  news_sources_list: string[];
  auto_fetch_interval_minutes: number;
  max_articles_per_fetch: number;
  sentiment_threshold: number;
  created_at: number;
  updated_at: number;
}

export function NewsConfigPage() {
  const { language } = useLanguage();
  const { user, token } = useAuth();
  const [showModal, setShowModal] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newsConfig, setNewsConfig] = useState<NewsConfig | null>(null);

  // 加载新闻配置
  useEffect(() => {
    const loadNewsConfig = async () => {
      if (!user || !token) {
        setLoading(false);
        return;
      }

      try {
        setLoading(true);
        const response = await fetch('/api/user/news-config', {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        if (response.status === 404) {
          // 配置不存在，这是正常的
          setNewsConfig(null);
          setError(null);
        } else if (response.ok) {
          const data = await response.json();
          setNewsConfig(data.data);
          setError(null);
        } else {
          const errorData = await response.json();
          setError(errorData.message || '加载配置失败');
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : '加载配置失败');
      } finally {
        setLoading(false);
      }
    };

    loadNewsConfig();
  }, [user, token]);

  const handleSave = async (data: any) => {
    // 刷新配置
    const response = await fetch('/api/user/news-config', {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (response.ok) {
      const result = await response.json();
      setNewsConfig(result.data);
    }
  };

  const handleDelete = async () => {
    if (
      !window.confirm(
        '确定要删除新闻配置吗？'
      )
    ) {
      return;
    }

    try {
      const response = await fetch('/api/user/news-config', {
        method: 'DELETE',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (response.ok) {
        setNewsConfig(null);
      } else {
        const errorData = await response.json();
        setError(errorData.message || '删除失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  };

  if (!user) {
    return (
      <div className="p-6 text-center text-gray-600 dark:text-gray-400">
        请先登录
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center gap-3">
        <Newspaper size={28} className="text-blue-500" />
        <h1 className="text-3xl font-bold dark:text-white">新闻源配置</h1>
      </div>

      {/* 配置卡片 */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">
        {loading ? (
          <div className="text-center py-8">
            <div className="inline-block animate-spin text-2xl">⚙️</div>
            <p className="mt-2 text-gray-600 dark:text-gray-400">加载中...</p>
          </div>
        ) : error ? (
          <div className="flex items-center gap-3 p-4 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-700 rounded-lg text-red-700 dark:text-red-200">
            <AlertTriangle size={20} />
            <span>{error}</span>
          </div>
        ) : newsConfig ? (
          <div className="space-y-4">
            {/* 状态 */}
            <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <div>
                <p className="text-sm text-gray-600 dark:text-gray-400">状态</p>
                <p className="font-medium dark:text-white">
                  {newsConfig.enabled ? '✅ 已启用' : '❌ 已禁用'}
                </p>
              </div>
              <button
                onClick={() => setShowModal(true)}
                className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
              >
                编辑配置
              </button>
            </div>

            {/* 新闻源 */}
            <div className="grid grid-cols-2 gap-4">
              <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-sm text-gray-600 dark:text-gray-400">新闻源</p>
                <div className="flex flex-wrap gap-2 mt-2">
                  {newsConfig.news_sources_list?.map(source => (
                    <span
                      key={source}
                      className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200 rounded text-sm"
                    >
                      {source}
                    </span>
                  ))}
                </div>
              </div>

              <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  抓取间隔
                </p>
                <p className="font-medium dark:text-white mt-2">
                  {newsConfig.auto_fetch_interval_minutes} 分钟
                </p>
              </div>

              <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  每次最多文章数
                </p>
                <p className="font-medium dark:text-white mt-2">
                  {newsConfig.max_articles_per_fetch}
                </p>
              </div>

              <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  情绪阈值
                </p>
                <p className="font-medium dark:text-white mt-2">
                  {newsConfig.sentiment_threshold.toFixed(2)}
                </p>
              </div>
            </div>

            {/* 删除按钮 */}
            <div className="flex gap-2">
              <button
                onClick={handleDelete}
                className="flex-1 px-4 py-2 border border-red-500 text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900 transition-colors"
              >
                删除配置
              </button>
            </div>
          </div>
        ) : (
          <div className="text-center py-8 space-y-4">
            <Newspaper
              size={48}
              className="mx-auto text-gray-400"
            />
            <p className="text-gray-600 dark:text-gray-400">
              还未配置新闻源
            </p>
            <button
              onClick={() => setShowModal(true)}
              className="mx-auto px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
            >
              创建配置
            </button>
          </div>
        )}
      </div>

      {/* 信息提示 */}
      <div className="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-700 rounded-lg p-4 text-blue-700 dark:text-blue-200">
        <h3 className="font-medium mb-2">💡 新闻源配置说明</h3>
        <ul className="text-sm space-y-1 list-disc list-inside">
          <li>启用新闻功能后，系统将自动从配置的新闻源获取最新信息</li>
          <li>抓取间隔决定了多久更新一次新闻（1-1440分钟）</li>
          <li>情绪阈值用于过滤具有特定情绪倾向的新闻文章</li>
          <li>新闻信息将被整合到交易决策中，帮助改进交易策略</li>
        </ul>
      </div>

      {/* 模态框 */}
      <NewsSourceModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        initialData={newsConfig}
        onSave={handleSave}
      />
    </div>
  );
}
