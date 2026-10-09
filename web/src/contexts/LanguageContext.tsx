import { createContext, useContext, useState, ReactNode } from 'react'
import type { Language } from '../i18n/translations'

interface LanguageContextType {
  language: Language
  setLanguage: (lang: Language) => void
}

const LanguageContext = createContext<LanguageContextType | undefined>(
  undefined
)

export function LanguageProvider({ children }: { children: ReactNode }) {
  // The product UI is English-only. Normalize legacy browser state left by the
  // removed language switcher so an old `language=zh|id` value cannot revive a
  // partially translated, layout-breaking interface.
  const [language] = useState<Language>(() => {
    localStorage.setItem('language', 'en')
    return 'en'
  })

  const handleSetLanguage = (_lang: Language) => {
    localStorage.setItem('language', 'en')
  }

  return (
    <LanguageContext.Provider
      value={{ language, setLanguage: handleSetLanguage }}
    >
      {children}
    </LanguageContext.Provider>
  )
}

export function useLanguage() {
  const context = useContext(LanguageContext)
  if (!context) {
    throw new Error('useLanguage must be used within LanguageProvider')
  }
  return context
}
