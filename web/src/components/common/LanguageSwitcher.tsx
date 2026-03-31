import { Globe } from 'lucide-react'
import { useLanguage } from '../../contexts/LanguageContext'
import type { Language } from '../../i18n/translations'

const languages: { code: Language; label: string }[] = [
  { code: 'zh', label: '中文' },
  { code: 'en', label: 'EN' },
  { code: 'id', label: 'ID' },
]

export function LanguageSwitcher() {
  const { language, setLanguage } = useLanguage()

  return (
    <div className="absolute top-4 right-4 z-50 flex items-center gap-1 rounded-lg p-1 border border-white/10 bg-white/5 backdrop-blur-sm">
      <Globe size={14} className="text-zinc-500 ml-1.5 mr-0.5" />
      {languages.map(({ code, label }) => (
        <button
          key={code}
          type="button"
          onClick={() => setLanguage(code)}
          className={`px-2.5 py-1 rounded text-xs font-semibold transition-all ${
            language === code
              ? 'bg-nofx-gold/15 text-nofx-gold'
              : 'text-zinc-500 hover:text-zinc-300 bg-transparent'
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  )
}
