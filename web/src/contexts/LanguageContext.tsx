import { createContext, useContext, ReactNode } from 'react'
import type { Language } from '../i18n/translations'

interface LanguageContextType {
  language: Language
}

const LanguageContext = createContext<LanguageContextType | undefined>(
  undefined
)

export function LanguageProvider({ children }: { children: ReactNode }) {
  // 固定使用中文，不再提供切换功能
  const language: Language = 'zh'

  return (
    <LanguageContext.Provider value={{ language }}>
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
