import { Globe } from 'lucide-react'
import { useLanguage } from '../../contexts/LanguageContext'
import type { Language } from '../../i18n/translations'

const languages: { code: Language; label: string }[] = [
  { code: 'zh', label: 'Chinese' },
  { code: 'en', label: 'EN' },
  { code: 'id', label: 'ID' },
  { code: 'ja', label: '日本語' },
]

export function LanguageSwitcher() {
  const { language, setLanguage } = useLanguage()

  return (
    <div className="absolute top-4 right-4 z-50 flex items-center gap-1 rounded-lg p-1 border border-[rgba(26,24,19,0.14)] bg-nofx-bg-lighter backdrop-blur-sm">
      <Globe size={14} className="text-nofx-text-muted ml-1.5 mr-0.5" />
      {languages.map(({ code, label }) => (
        <button
          key={code}
          type="button"
          onClick={() => setLanguage(code)}
          className={`px-2.5 py-1 rounded text-xs font-semibold transition-all ${
            language === code
              ? 'bg-nofx-gold/15 text-nofx-gold'
              : 'text-nofx-text-muted hover:text-nofx-text bg-transparent'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}
