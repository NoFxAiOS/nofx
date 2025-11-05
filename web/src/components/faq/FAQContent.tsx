import { useEffect, useRef } from 'react'
import { t, type Language } from '../../i18n/translations'
import type { FAQCategory } from '../../data/faqData'

interface FAQContentProps {
  categories: FAQCategory[]
  language: Language
  onActiveItemChange: (itemId: string) => void
}

export function FAQContent({
  categories,
  language,
  onActiveItemChange,
}: FAQContentProps) {
  const sectionRefs = useRef<Map<string, HTMLElement>>(new Map())

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const itemId = entry.target.getAttribute('data-item-id')
            if (itemId) {
              onActiveItemChange(itemId)
            }
          }
        })
      },
      {
        rootMargin: '-100px 0px -80% 0px',
        threshold: 0,
      }
    )

    sectionRefs.current.forEach((ref) => {
      if (ref) observer.observe(ref)
    })

    return () => {
      sectionRefs.current.forEach((ref) => {
        if (ref) observer.unobserve(ref)
      })
    }
  }, [onActiveItemChange])

  const setRef = (itemId: string, element: HTMLElement | null) => {
    if (element) {
      sectionRefs.current.set(itemId, element)
    } else {
      sectionRefs.current.delete(itemId)
    }
  }

  return (
    <div className="space-y-12">
      {categories.map((category) => (
        <div key={category.id}>
          {/* Category Header */}
          <div
            className="flex items-center gap-3 mb-6 pb-3"
            style={{ borderBottom: '2px solid #2B3139' }}
          >
            <category.icon className="w-7 h-7" style={{ color: '#F0B90B' }} />
            <h2 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
              {t(category.titleKey, language)}
            </h2>
          </div>

          {/* FAQ Items */}
          <div className="space-y-8">
            {category.items.map((item) => (
              <section
                key={item.id}
                id={item.id}
                data-item-id={item.id}
                ref={(el) => setRef(item.id, el)}
                className="scroll-mt-24"
              >
                {/* Question */}
                <h3
                  className="text-xl font-semibold mb-3"
                  style={{ color: '#EAECEF' }}
                >
                  {t(item.questionKey, language)}
                </h3>

                {/* Answer */}
                <div
                  className="prose prose-invert max-w-none"
                  style={{
                    color: '#B7BDC6',
                    lineHeight: '1.7',
                  }}
                >
                  <p className="text-base">{t(item.answerKey, language)}</p>
                </div>

                {/* Divider */}
                <div className="mt-6 h-px" style={{ background: '#2B3139' }} />
              </section>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
