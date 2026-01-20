import { useLanguage } from '../contexts/LanguageContext'

export function DataPage() {
  const { language } = useLanguage()

  return (
    <div className="w-full h-[calc(100vh-64px)]">
      <iframe
        src="https://nofxos.ai/dashboard"
        title={language === 'zh' ? '数据中心' : 'Data Center'}
        className="w-full h-full border-0"
        allow="fullscreen"
      />
    </div>
  )
}
