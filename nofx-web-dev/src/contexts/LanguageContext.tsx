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
  // Force language to Chinese (zh) - no user override allowed
  const [language] = useState<Language>('zh')

  // No-op setLanguage function to maintain interface compatibility
  const handleSetLanguage = (_lang: Language) => {
    // Language is fixed to Chinese, do nothing
    console.warn('Language is fixed to Chinese (zh) and cannot be changed')
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
