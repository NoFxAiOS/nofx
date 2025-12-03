import React from 'react';
import { useUserProfile, useUserCredits } from '../hooks/useUserProfile';
import { useAuth } from '../contexts/AuthContext';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';

/**
 * 用户详情页面组件
 *
 * 设计理念：
 * 1. 单一职责 - 只负责用户信息的展示
 * 2. 关注点分离 - 数据获取由Hook处理，组件专注渲染
 * 3. 渐进增强 - 从基础信息开始，支持未来扩展
 * 4. 响应式设计 - 移动优先，自适应各种屏幕
 *
 * 架构思考：
 * 就像内核的VFS层，这个组件提供了一个统一的抽象界面，
 * 隐藏了底层数据获取的复杂性，让UI逻辑保持纯粹和优雅。
 */
const UserProfilePage: React.FC = () => {
  const { user } = useAuth();
  const { userProfile, loading, error, refetch } = useUserProfile();
  const { language } = useLanguage();
  const { credits, loading: creditsLoading, error: creditsError } = useUserCredits();

  // 渲染加载状态
  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
          <UserProfileSkeleton />
        </div>
      </div>
    );
  }

  // 渲染错误状态
  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6 text-center">
            <div className="text-red-500 dark:text-red-400 mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">{t('profile.profile_error', language) || '获取用户信息失败'}</h3>
            <p className="text-gray-600 dark:text-gray-400 mb-4">{error}</p>
            <button
              onClick={refetch}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
            >
              {t('profile.retry', language)}
            </button>
          </div>
        </div>
      </div>
    );
  }

  // 渲染用户资料
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* 页面标题 */}
        <div className="mb-8 flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
              {t('profile.userInfo', language)}
            </h1>
            <p className="mt-2 text-gray-600 dark:text-gray-400">
              {t('profile.userProfileSubtitle', language)}
            </p>
          </div>
          <button
            onClick={() => window.history.back()}
            className="flex items-center text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            {t('profile.profile_back', language)}
          </button>
        </div>

        {/* 用户信息卡片 */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* 基本信息卡片 */}
          <div className="lg:col-span-1">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                {t('profile.basicInfo', language)}
              </h3>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">
                    {t('profile.profile_email', language)}
                  </label>
                  <p className="mt-1 text-gray-900 dark:text-white">{user?.email}</p>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">
                    {t('profile.memberSince', language)}
                  </label>
                  <p className="mt-1 text-gray-900 dark:text-white">
                    {userProfile?.created_at ? new Date(userProfile.created_at).toLocaleDateString() : '-'}
                  </p>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">
                    {t('profile.lastLogin', language)}
                  </label>
                  <p className="mt-1 text-gray-900 dark:text-white">
                    {userProfile?.last_login_at ? new Date(userProfile.last_login_at).toLocaleString() : '-'}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* 统计信息卡片 */}
          <div className="lg:col-span-2 space-y-6">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                {t('profile.accountOverview', language)}
              </h3>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="text-center">
                  <div className="text-2xl font-bold text-gray-900 dark:text-white">
                    {userProfile?.total_equity ? `$${userProfile.total_equity.toFixed(2)}` : '$0.00'}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    总净值
                  </div>
                  <div className={`text-xs mt-1 ${
                    (userProfile?.daily_pnl || 0) >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
                  }`}>
                    {(userProfile?.daily_pnl || 0) >= 0 ? '+' : ''}{userProfile?.daily_pnl?.toFixed(2) || 0} 日收益
                  </div>
                </div>

                <div className="text-center">
                  <div className="text-2xl font-bold text-gray-900 dark:text-white">
                    {userProfile?.total_pnl ? `$${userProfile.total_pnl.toFixed(2)}` : '$0.00'}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    总盈亏
                  </div>
                </div>

                <div className="text-center">
                  <div className="text-2xl font-bold text-gray-900 dark:text-white">
                    {userProfile?.active_traders || 0}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    活跃交易员
                  </div>
                  <div className="text-xs text-gray-400 dark:text-gray-500">
                    /{userProfile?.trader_count || 0} 总计
                  </div>
                </div>

                <div className="text-center">
                  <div className="text-2xl font-bold text-gray-900 dark:text-white">
                    {userProfile?.position_count || 0}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    持仓数量
                  </div>
                </div>
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                {t('profile.creditSystem', language)}
              </h3>

              {creditsLoading ? (
                <div className="text-center py-8">
                  <div className="text-4xl font-bold text-blue-600 dark:text-blue-400 mb-2">
                    ⏳
                  </div>
                  <p className="text-gray-600 dark:text-gray-400">
                    加载积分数据中...
                  </p>
                </div>
              ) : creditsError ? (
                <div className="text-center py-8">
                  <div className="text-4xl font-bold text-red-600 dark:text-red-400 mb-2">
                    ⚠️
                  </div>
                  <p className="text-red-600 dark:text-red-400">
                    积分数据加载失败
                  </p>
                </div>
              ) : (
                <div className="grid grid-cols-3 gap-4">
                  <div className="text-center">
                    <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                      {credits?.total_credits || 0}
                    </div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      总积分
                    </div>
                    <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                      账户总余额
                    </div>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                      {credits?.available_credits || 0}
                    </div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      可用积分
                    </div>
                    <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                      可用于消费
                    </div>
                  </div>
                  <div className="text-center">
                    <div className="text-2xl font-bold text-orange-600 dark:text-orange-400">
                      {credits?.used_credits || 0}
                    </div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      已用积分
                    </div>
                    <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                      历史累计消费
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 交易员概览 */}
        {(userProfile?.trader_count || 0) > 0 && (
          <div className="mt-8">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                {t('profile.traderOverview', language)}
              </h3>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="text-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div className="text-2xl font-bold text-gray-900 dark:text-white">
                    {userProfile?.trader_count || 0}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    总交易员数
                  </div>
                </div>

                <div className="text-center p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-green-600 dark:text-green-400">
                    {userProfile?.active_traders || 0}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    活跃交易员
                  </div>
                </div>

                <div className="text-center p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                  <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                    {userProfile?.position_count || 0}
                  </div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    总持仓数
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

/**
 * 骨架屏组件
 * 提升感知性能，遵循渐进式增强原则
 */
const UserProfileSkeleton: React.FC = () => {
  return (
    <div className="space-y-6">
      {/* 标题骨架 */}
      <div className="space-y-2">
        <div className="h-8 w-48 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
        <div className="h-4 w-64 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
      </div>

      {/* 卡片骨架 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-1">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
            <div className="h-6 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mb-4"></div>
            <div className="space-y-4">              {[1, 2, 3].map((i) => (
                <div key={i}>
                  <div className="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mb-1"></div>
                  <div className="h-5 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="lg:col-span-2 space-y-6">
          {[1, 2].map((i) => (
            <div key={i} className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-6">
              <div className="h-6 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mb-4"></div>
              <div className="grid grid-cols-3 gap-4">
                {[1, 2, 3].map((j) => (
                  <div key={j} className="text-center">
                    <div className="h-8 w-20 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mx-auto mb-1"></div>
                    <div className="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mx-auto"></div>
                    <div className="h-3 w-12 bg-gray-200 dark:bg-gray-700 rounded animate-pulse mx-auto mt-1"></div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default UserProfilePage;