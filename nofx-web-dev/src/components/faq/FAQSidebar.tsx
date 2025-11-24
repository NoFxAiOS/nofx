import { t, type Language } from '../../i18n/translations'
import type { FAQCategory } from '../../data/faqData'

interface FAQSidebarProps {
  categories: FAQCategory[]
  activeItemId: string | null
  language: Language
  onItemClick: (categoryId: string, itemId: string) => void
}

export function FAQSidebar({
  categories,
  activeItemId,
  language,
  onItemClick,
}: FAQSidebarProps) {
  return (
    <nav
      className="sticky top-24 h-[calc(100vh-120px)] overflow-y-auto pr-4"
      style={{
        scrollbarWidth: 'thin',
        scrollbarColor: 'var(--panel-border) var(--background)',
      }}
    >
      <div className="space-y-6">
        {categories.map((category) => (
          <div key={category.id}>
            {/* Category Title */}
            <div className="flex items-center gap-2 mb-3 px-3">
              <category.icon
                className="w-5 h-5"
                style={{ color: 'var(--brand-red)' }}
              />
              <h3
                className="text-sm font-bold uppercase tracking-wide"
                style={{ color: 'var(--brand-red)' }}
              >
                {t(category.titleKey, language)}
              </h3>
            </div>

            {/* Category Items */}
            <ul className="space-y-1">
              {category.items.map((item) => {
                const isActive = activeItemId === item.id
                return (
                  <li key={item.id}>
                    <button
                      onClick={() => onItemClick(category.id, item.id)}
                      className="w-full text-left px-3 py-2 rounded-lg text-sm transition-all"
                      style={{
                        background: isActive
                          ? 'rgba(229, 0, 18, 0.1)'
                          : 'transparent',
                        color: isActive
                          ? 'var(--brand-red)'
                          : 'var(--text-secondary)',
                        borderLeft: isActive
                          ? '3px solid var(--brand-red)'
                          : '3px solid transparent',
                        paddingLeft: isActive ? '9px' : '12px',
                      }}
                      onMouseEnter={(e) => {
                        if (!isActive) {
                          e.currentTarget.style.background =
                            'rgba(229, 0, 18, 0.05)'
                          e.currentTarget.style.color = 'var(--text-primary)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (!isActive) {
                          e.currentTarget.style.background = 'transparent'
                          e.currentTarget.style.color = 'var(--text-secondary)'
                        }
                      }}
                    >
                      {t(item.questionKey, language)}
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </div>
    </nav>
  )
}
