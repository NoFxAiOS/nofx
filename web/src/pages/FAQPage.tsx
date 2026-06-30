import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'

/**
 * FAQ page
 *
 * HeaderBar and Footer are now provided by MainLayout
 *
 * All FAQ-related logic lives in child components:
 * - FAQLayout: overall layout and search logic
 * - FAQSearchBar: search box
 * - FAQSidebar: left-side table of contents
 * - FAQContent: right-side content area
 *
 * FAQ data is configured in data/faqData.ts
 */
export function FAQPage() {
  const { language } = useLanguage()

  return <FAQLayout language={language} />
}
