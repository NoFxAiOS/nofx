import HeaderBar from '../components/landing/HeaderBar'
import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { t } from '../i18n/translations'

/**
 * FAQ 页面
 *
 * 这个页面只是组件的集合，负责：
 * - 组装 HeaderBar 和 FAQLayout
 * - 提供全局状态（语言、用户、系统配置）
 * - 处理页面级别的导航
 *
 * 所有 FAQ 相关的逻辑都在子组件中：
 * - FAQLayout: 整体布局和搜索逻辑
 * - FAQSearchBar: 搜索框
 * - FAQSidebar: 左侧目录
 * - FAQContent: 右侧内容区
 *
 * FAQ 数据配置在 data/faqData.ts
 */
export function FAQPage() {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  useSystemConfig() // Load system config but don't use it

  return (
    <div
      className="min-h-screen"
      style={{ background: '#000000', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage="faq"
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            window.location.href = '/competition'
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            window.location.href = '/traders'
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            window.location.href = '/dashboard'
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            window.location.href = '/faq'
          }
        }}
      />

      <FAQLayout language={language} />

      {/* Footer */}
      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <div
          className="max-w-7xl mx-auto px-6 py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </footer>
    </div>
  )
}
