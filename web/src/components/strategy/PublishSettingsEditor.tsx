import { Globe, Lock, Eye, EyeOff } from 'lucide-react'

interface PublishSettingsEditorProps {
  isPublic: boolean
  configVisible: boolean
  onIsPublicChange: (value: boolean) => void
  onConfigVisibleChange: (value: boolean) => void
  disabled?: boolean
  language: string
}

export function PublishSettingsEditor({
  isPublic,
  configVisible,
  onIsPublicChange,
  onConfigVisibleChange,
  disabled = false,
  language,
}: PublishSettingsEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      publishToMarket: { zh: '发布到策略市场', en: 'Publish to Market' },
      publishDesc: { zh: '策略将在市场公开展示，其他用户可发现并使用', en: 'Strategy will be publicly visible in the marketplace' },
      showConfig: { zh: '公开配置参数', en: 'Show Config' },
      showConfigDesc: { zh: '允许他人查看和复制详细配置', en: 'Allow others to view and clone config details' },
      private: { zh: '私有', en: 'PRIVATE' },
      public: { zh: '公开', en: 'PUBLIC' },
      hidden: { zh: '隐藏', en: 'HIDDEN' },
      visible: { zh: '可见', en: 'VISIBLE' },
    }
    return translations[key]?.[language] || key
  }

  return (
    <div className="space-y-3">
      {/* 发布开关 */}
      <div
        className={`relative overflow-hidden rounded-lg transition-all duration-300 ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
        style={{
          background: isPublic
            ? 'linear-gradient(135deg, rgba(14, 203, 129, 0.15) 0%, rgba(14, 203, 129, 0.05) 100%)'
            : 'linear-gradient(135deg, #1E2329 0%, #0B0E11 100%)',
          border: isPublic ? '1px solid rgba(14, 203, 129, 0.4)' : '1px solid #2B3139',
          boxShadow: isPublic ? '0 0 20px rgba(14, 203, 129, 0.1)' : 'none',
        }}
        onClick={() => !disabled && onIsPublicChange(!isPublic)}
      >
        {/* Top glow line */}
        <div
          className="absolute top-0 left-0 w-full h-[1px] transition-opacity duration-300"
          style={{
            background: isPublic
              ? 'linear-gradient(90deg, transparent, #0ECB81, transparent)'
              : 'linear-gradient(90deg, transparent, #2B3139, transparent)',
            opacity: isPublic ? 1 : 0.5
          }}
        />

        <div className="p-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className="p-2.5 rounded-lg transition-all duration-300"
              style={{
                background: isPublic ? 'rgba(14, 203, 129, 0.2)' : '#0B0E11',
                border: isPublic ? '1px solid rgba(14, 203, 129, 0.3)' : '1px solid #2B3139'
              }}
            >
              {isPublic ? (
                <Globe className="w-5 h-5" style={{ color: '#0ECB81' }} />
              ) : (
                <Lock className="w-5 h-5" style={{ color: '#848E9C' }} />
              )}
            </div>
            <div>
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {t('publishToMarket')}
              </div>
              <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>
                {t('publishDesc')}
              </div>
            </div>
          </div>

          {/* Toggle with status */}
          <div className="flex items-center gap-3">
            <span
              className="text-[10px] font-mono font-bold tracking-wider"
              style={{ color: isPublic ? '#0ECB81' : '#848E9C' }}
            >
              {isPublic ? t('public') : t('private')}
            </span>
            <div
              className="relative w-12 h-6 rounded-full transition-all duration-300"
              style={{
                background: isPublic
                  ? 'linear-gradient(90deg, #0ECB81, #4ade80)'
                  : '#2B3139',
                boxShadow: isPublic ? '0 0 10px rgba(14, 203, 129, 0.4)' : 'none'
              }}
            >
              <div
                className="absolute top-1 w-4 h-4 rounded-full transition-all duration-300"
                style={{
                  background: '#EAECEF',
                  left: isPublic ? '28px' : '4px',
                  boxShadow: '0 2px 4px rgba(0,0,0,0.3)'
                }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* 配置可见性开关 - 仅在公开时显示 */}
      {isPublic && (
        <div
          className={`relative overflow-hidden rounded-lg transition-all duration-300 ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
          style={{
            background: configVisible
              ? 'linear-gradient(135deg, rgba(168, 85, 247, 0.15) 0%, rgba(168, 85, 247, 0.05) 100%)'
              : 'linear-gradient(135deg, #1E2329 0%, #0B0E11 100%)',
            border: configVisible ? '1px solid rgba(168, 85, 247, 0.4)' : '1px solid #2B3139',
            boxShadow: configVisible ? '0 0 20px rgba(168, 85, 247, 0.1)' : 'none',
          }}
          onClick={() => !disabled && onConfigVisibleChange(!configVisible)}
        >
          {/* Top glow line */}
          <div
            className="absolute top-0 left-0 w-full h-[1px] transition-opacity duration-300"
            style={{
              background: configVisible
                ? 'linear-gradient(90deg, transparent, #a855f7, transparent)'
                : 'linear-gradient(90deg, transparent, #2B3139, transparent)',
              opacity: configVisible ? 1 : 0.5
            }}
          />

          <div className="p-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div
                className="p-2.5 rounded-lg transition-all duration-300"
                style={{
                  background: configVisible ? 'rgba(168, 85, 247, 0.2)' : '#0B0E11',
                  border: configVisible ? '1px solid rgba(168, 85, 247, 0.3)' : '1px solid #2B3139'
                }}
              >
                {configVisible ? (
                  <Eye className="w-5 h-5" style={{ color: '#a855f7' }} />
                ) : (
                  <EyeOff className="w-5 h-5" style={{ color: '#848E9C' }} />
                )}
              </div>
              <div>
                <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                  {t('showConfig')}
                </div>
                <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>
                  {t('showConfigDesc')}
                </div>
              </div>
            </div>

            {/* Toggle with status */}
            <div className="flex items-center gap-3">
              <span
                className="text-[10px] font-mono font-bold tracking-wider"
                style={{ color: configVisible ? '#a855f7' : '#848E9C' }}
              >
                {configVisible ? t('visible') : t('hidden')}
              </span>
              <div
                className="relative w-12 h-6 rounded-full transition-all duration-300"
                style={{
                  background: configVisible
                    ? 'linear-gradient(90deg, #a855f7, #c084fc)'
                    : '#2B3139',
                  boxShadow: configVisible ? '0 0 10px rgba(168, 85, 247, 0.4)' : 'none'
                }}
              >
                <div
                  className="absolute top-1 w-4 h-4 rounded-full transition-all duration-300"
                  style={{
                    background: '#EAECEF',
                    left: configVisible ? '28px' : '4px',
                    boxShadow: '0 2px 4px rgba(0,0,0,0.3)'
                  }}
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default PublishSettingsEditor
