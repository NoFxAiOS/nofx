import type { FAQCategory } from '../../data/faqData'

interface FAQSidebarProps {
  categories: FAQCategory[]
  activeItemId: string | null
  onItemClick: (categoryId: string, itemId: string) => void
}

export function FAQSidebar({
  categories,
  activeItemId,
  onItemClick,
}: FAQSidebarProps) {
  return (
    <nav
      className="sticky top-24 h-[calc(100vh-120px)] overflow-y-auto pr-2"
      style={{ scrollbarWidth: 'thin', scrollbarColor: '#E8E2D5 transparent' }}
    >
      <div className="space-y-5">
        {categories.map((category) => (
          <div key={category.id}>
            <div className="mb-2 flex items-center gap-2 px-2">
              <category.icon className="h-3.5 w-3.5 text-nofx-gold" />
              <h3 className="font-mono text-[11px] font-bold uppercase tracking-[0.16em] text-nofx-gold">
                {category.title}
              </h3>
            </div>
            <ul className="space-y-0.5 border-l border-[rgba(26,24,19,0.12)]">
              {category.items.map((item) => {
                const isActive = activeItemId === item.id
                return (
                  <li key={item.id}>
                    <button
                      onClick={() => onItemClick(category.id, item.id)}
                      className={`-ml-px w-full border-l-2 py-1.5 pl-3 pr-2 text-left text-[13px] leading-5 transition-colors ${
                        isActive
                          ? 'border-nofx-gold bg-nofx-gold/5 font-medium text-nofx-text'
                          : 'border-transparent text-nofx-text-muted hover:border-nofx-gold/40 hover:text-nofx-text'
                      }`}
                    >
                      {item.question}
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
