import { useState, useMemo } from 'react'
import { HelpCircle } from 'lucide-react'
import { t, type Language } from '../../i18n/translations'
import { FAQSearchBar } from './FAQSearchBar'
import { FAQSidebar } from './FAQSidebar'
import { FAQContent } from './FAQContent'
import { faqCategories } from '../../data/faqData'
import type { FAQCategory } from '../../data/faqData'

interface FAQLayoutProps {
  language: Language
}

export function FAQLayout({ language }: FAQLayoutProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [activeItemId, setActiveItemId] = useState<string | null>(null)

  // Filter categories based on search term
  const filteredCategories = useMemo(() => {
    if (!searchTerm.trim()) {
      return faqCategories
    }

    const term = searchTerm.toLowerCase()
    const filtered: FAQCategory[] = []

    faqCategories.forEach((category) => {
      const matchingItems = category.items.filter((item) => {
        const question = t(item.questionKey, language).toLowerCase()
        const answer = t(item.answerKey, language).toLowerCase()
        return question.includes(term) || answer.includes(term)
      })

      if (matchingItems.length > 0) {
        filtered.push({
          ...category,
          items: matchingItems,
        })
      }
    })

    return filtered
  }, [searchTerm, language])

  const handleItemClick = (_categoryId: string, itemId: string) => {
    const element = document.getElementById(itemId)
    if (element) {
      const offset = 100
      const elementPosition = element.getBoundingClientRect().top
      const offsetPosition = elementPosition + window.pageYOffset - offset

      window.scrollTo({
        top: offsetPosition,
        behavior: 'smooth',
      })
    }
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 pt-24">
      {/* Page Header */}
      <div className="text-center mb-12">
        <div className="flex items-center justify-center gap-3 mb-4">
          <div
            className="w-16 h-16 rounded-full flex items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, #E50012 0%, #FF1A2E 100%)',
              boxShadow: '0 8px 24px rgba(229, 0, 18, 0.4)',
            }}
          >
            <HelpCircle className="w-8 h-8" style={{ color: '#FFFFFF' }} />
          </div>
        </div>
        <h1
          className="text-4xl font-bold mb-4"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('faqTitle', language)}
        </h1>
        <p className="text-lg mb-8" style={{ color: 'var(--text-secondary)' }}>
          {t('faqSubtitle', language)}
        </p>

        {/* Search Bar */}
        <div className="max-w-2xl mx-auto">
          <FAQSearchBar
            searchTerm={searchTerm}
            onSearchChange={setSearchTerm}
            placeholder={
              language === 'zh' ? '搜索常见问题...' : 'Search FAQ...'
            }
          />
        </div>
      </div>

      {/* Main Content */}
      <div className="flex gap-8">
        {/* Sidebar - Hidden on mobile, visible on desktop */}
        <aside className="hidden lg:block w-64 flex-shrink-0">
          <FAQSidebar
            categories={filteredCategories}
            activeItemId={activeItemId}
            language={language}
            onItemClick={handleItemClick}
          />
        </aside>

        {/* Content Area */}
        <main className="flex-1 min-w-0">
          {filteredCategories.length > 0 ? (
            <FAQContent
              categories={filteredCategories}
              language={language}
              onActiveItemChange={setActiveItemId}
            />
          ) : (
            <div className="text-center py-12">
              <p className="text-lg" style={{ color: 'var(--text-secondary)' }}>
                {language === 'zh'
                  ? '没有找到匹配的问题'
                  : 'No matching questions found'}
              </p>
              <button
                onClick={() => setSearchTerm('')}
                className="mt-4 px-6 py-2 rounded-lg font-semibold transition-all hover:opacity-90"
                style={{
                  background:
                    'linear-gradient(135deg, #E50012 0%, #FF1A2E 100%)',
                  color: '#FFFFFF',
                }}
              >
                {language === 'zh' ? '清除搜索' : 'Clear Search'}
              </button>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
