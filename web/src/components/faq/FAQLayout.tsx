import { useMemo, useState } from 'react'
import { HelpCircle } from 'lucide-react'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { FAQSearchBar } from './FAQSearchBar'
import { FAQSidebar } from './FAQSidebar'
import { FAQContent } from './FAQContent'
import { faqCategories, faqItemSearchText } from '../../data/faqData'
import type { FAQCategory } from '../../data/faqData'

export function FAQLayout() {
  const [searchTerm, setSearchTerm] = useState('')
  const [activeItemId, setActiveItemId] = useState<string | null>(null)

  const filteredCategories = useMemo(() => {
    if (!searchTerm.trim()) return faqCategories

    const term = searchTerm.toLowerCase()
    const filtered: FAQCategory[] = []
    faqCategories.forEach((category) => {
      const matchingItems = category.items.filter((item) =>
        faqItemSearchText(item).includes(term)
      )
      if (matchingItems.length > 0) {
        filtered.push({ ...category, items: matchingItems })
      }
    })
    return filtered
  }, [searchTerm])

  const totalItems = useMemo(
    () => faqCategories.reduce((sum, category) => sum + category.items.length, 0),
    []
  )

  const handleItemClick = (_categoryId: string, itemId: string) => {
    const element = document.getElementById(itemId)
    if (!element) return
    const offset = 100
    const top =
      element.getBoundingClientRect().top + window.pageYOffset - offset
    window.scrollTo({ top, behavior: 'smooth' })
  }

  return (
    <DeepVoidBackground className="py-8 pt-24" disableAnimation>
      <div className="mx-auto w-full max-w-6xl px-4 md:px-8">
        {/* page header — same strip language as the other terminal pages */}
        <div className="mb-8 border-b border-nofx-gold/20 pb-6">
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-nofx-gold/30 bg-nofx-bg-lighter text-nofx-gold md:h-14 md:w-14">
                <HelpCircle className="h-6 w-6 md:h-7 md:w-7" />
              </div>
              <div>
                <h1 className="font-mono text-2xl font-bold tracking-tight text-nofx-text md:text-3xl">
                  FAQ
                </h1>
                <p className="mt-1 font-mono text-xs uppercase tracking-[0.14em] text-nofx-text-muted">
                  {totalItems} answers · wallets · launch · trading · self-hosting
                </p>
              </div>
            </div>
            <div className="w-full md:w-80">
              <FAQSearchBar
                searchTerm={searchTerm}
                onSearchChange={setSearchTerm}
              />
            </div>
          </div>
        </div>

        {/* content */}
        <div className="flex gap-8">
          <aside className="hidden w-64 flex-shrink-0 lg:block">
            <FAQSidebar
              categories={filteredCategories}
              activeItemId={activeItemId}
              onItemClick={handleItemClick}
            />
          </aside>

          <main className="min-w-0 flex-1">
            {filteredCategories.length > 0 ? (
              <FAQContent
                categories={filteredCategories}
                onActiveItemChange={setActiveItemId}
              />
            ) : (
              <div className="rounded-xl border border-nofx-gold/20 bg-nofx-bg-lighter py-16 text-center">
                <p className="font-mono text-sm text-nofx-text-muted">
                  No matching questions for “{searchTerm}”.
                </p>
                <button
                  onClick={() => setSearchTerm('')}
                  className="mt-4 rounded-lg border border-nofx-gold/30 bg-nofx-gold/10 px-5 py-2 font-mono text-xs font-bold uppercase tracking-[0.12em] text-nofx-gold hover:bg-nofx-gold/20"
                >
                  Clear search
                </button>
              </div>
            )}
          </main>
        </div>

        {/* still stuck */}
        <div className="mt-12 rounded-xl border border-nofx-gold/20 bg-nofx-bg-lighter p-6 text-center md:p-8">
          <h3 className="font-mono text-sm font-bold uppercase tracking-[0.16em] text-nofx-text">
            Still have questions?
          </h3>
          <p className="mt-2 text-sm text-nofx-text-muted">
            Ask in the community or open an issue — both are answered by the
            people building NOFX.
          </p>
          <div className="mt-5 flex items-center justify-center gap-3">
            <a
              href="https://github.com/NoFxAiOS/nofx"
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-lg border border-[rgba(26,24,19,0.14)] bg-nofx-bg-deeper px-5 py-2.5 font-mono text-xs font-bold uppercase tracking-[0.12em] text-nofx-text hover:border-nofx-gold/40"
            >
              GitHub
            </a>
            <a
              href="https://t.me/nofx_dev_community"
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-lg bg-nofx-gold px-5 py-2.5 font-mono text-xs font-bold uppercase tracking-[0.12em] text-white hover:bg-nofx-accent"
            >
              Telegram community
            </a>
          </div>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
