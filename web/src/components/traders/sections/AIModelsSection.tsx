import { Brain } from 'lucide-react'
import { t, Language } from '../../../i18n/translations'
import { getModelIcon } from '../../ModelIcons'
import { getShortName } from '../utils'
import type { AIModel } from '../../../types'

interface AIModelsSectionProps {
  language: Language
  configuredModels: AIModel[]
  isModelInUse: (modelId: string) => boolean
  onModelClick: (modelId: string) => void
}

export function AIModelsSection({
  language,
  configuredModels,
  isModelInUse,
  onModelClick,
}: AIModelsSectionProps) {
  return (
    <div className="binance-card p-3 md:p-4">
      <h3
        className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
        style={{ color: '#EAECEF' }}
      >
        <Brain className="w-4 h-4 md:w-5 md:h-5" style={{ color: '#60a5fa' }} />
        {t('aiModels', language)}
      </h3>
      <div className="space-y-2 md:space-y-3">
        {configuredModels.map((model) => {
          const inUse = isModelInUse(model.id)
          return (
            <div
              key={model.id}
              className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                inUse
                  ? 'cursor-not-allowed'
                  : 'cursor-pointer hover:bg-gray-700'
              }`}
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              onClick={() => onModelClick(model.id)}
            >
              <div className="flex items-center gap-2 md:gap-3">
                <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                  {getModelIcon(model.provider || model.id, {
                    width: 28,
                    height: 28,
                  }) || (
                    <div
                      className="w-7 h-7 md:w-8 md:h-8 rounded-full flex items-center justify-center text-xs md:text-sm font-bold"
                      style={{
                        background:
                          model.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                        color: '#fff',
                      }}
                    >
                      {getShortName(model.name)[0]}
                    </div>
                  )}
                </div>
                <div className="min-w-0">
                  <div
                    className="font-semibold text-sm md:text-base truncate"
                    style={{ color: '#EAECEF' }}
                  >
                    {getShortName(model.name)}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {inUse
                      ? t('inUse', language)
                      : model.enabled
                        ? t('enabled', language)
                        : t('configured', language)}
                  </div>
                </div>
              </div>
              <div
                className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${model.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
              />
            </div>
          )
        })}
        {configuredModels.length === 0 && (
          <div
            className="text-center py-6 md:py-8"
            style={{ color: '#848E9C' }}
          >
            <Brain className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
            <div className="text-xs md:text-sm">
              {t('noModelsConfigured', language)}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
