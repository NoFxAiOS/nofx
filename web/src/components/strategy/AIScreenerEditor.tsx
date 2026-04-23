import { useState } from 'react'
import { Eye, Loader2 } from 'lucide-react'
import type { AIScreenerConfig } from '../../types'
import { aiScreener, ts } from '../../i18n/strategy-translations'

interface AIScreenerEditorProps {
  config: AIScreenerConfig
  onChange: (config: AIScreenerConfig) => void
  disabled?: boolean
  language: string
}

interface PreviewCoin {
  symbol: string
  volume_24h: number
  open_interest: number
  price_change_pct: number
}

const intervalOptions = [
  { value: 15, label: { zh: '15分钟', en: '15 min' } },
  { value: 30, label: { zh: '30分钟', en: '30 min' } },
  { value: 60, label: { zh: '1小时', en: '1 hour' } },
  { value: 120, label: { zh: '2小时', en: '2 hours' } },
  { value: 240, label: { zh: '4小时', en: '4 hours' } },
]

function formatNumber(n: number): string {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return n.toFixed(0)
}

export function AIScreenerEditor({
  config,
  onChange,
  disabled,
  language,
}: AIScreenerEditorProps) {
  const [previewData, setPreviewData] = useState<PreviewCoin[]>([])
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState('')

  const update = (partial: Partial<AIScreenerConfig>) => {
    onChange({ ...config, ...partial })
  }

  const handlePreview = async () => {
    setPreviewLoading(true)
    setPreviewError('')
    try {
      const params = new URLSearchParams({ limit: String(config.max_coins || 10) })
      if (config.min_volume_24h) params.set('min_volume', String(config.min_volume_24h))
      if (config.min_oi) params.set('min_oi', String(config.min_oi))
      const res = await fetch(`/api/market/hot-coins?${params}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      setPreviewData(Array.isArray(data) ? data.slice(0, config.max_coins || 10) : [])
    } catch (err) {
      setPreviewError(String(err))
      setPreviewData([])
    } finally {
      setPreviewLoading(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 rounded-lg bg-nofx-bg border border-nofx-border text-nofx-text text-sm focus:outline-none focus:border-nofx-gold/50'
  const labelClass = 'block text-sm text-nofx-text-muted mb-1'
  const sectionClass = 'space-y-3'
  const sectionTitleClass = 'text-sm font-medium text-nofx-text'

  return (
    <div className="space-y-6">
      {/* Basic Settings */}
      <div className={sectionClass}>
        <div className={sectionTitleClass}>{ts(aiScreener.basicSettings, language)}</div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>{ts(aiScreener.interval, language)}</label>
            <select
              value={config.screening_interval_minutes || 60}
              onChange={(e) => update({ screening_interval_minutes: Number(e.target.value) })}
              disabled={disabled}
              className={inputClass}
            >
              {intervalOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {language === 'zh' ? opt.label.zh : opt.label.en}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={labelClass}>{ts(aiScreener.maxCoins, language)}</label>
            <input
              type="number"
              min={1}
              max={50}
              value={config.max_coins || 10}
              onChange={(e) => update({ max_coins: Math.min(50, Math.max(1, Number(e.target.value))) })}
              disabled={disabled}
              className={inputClass}
            />
          </div>
        </div>
      </div>

      {/* Volume & OI Filters */}
      <div className={sectionClass}>
        <div className={sectionTitleClass}>{ts(aiScreener.volumeOiFilters, language)}</div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>{ts(aiScreener.minVolume, language)}</label>
            <input
              type="number"
              value={config.min_volume_24h ?? ''}
              onChange={(e) => update({ min_volume_24h: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="50000000"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-xs text-nofx-text-muted mt-1 block">{ts(aiScreener.volumeUnit, language)}</span>
          </div>
          <div>
            <label className={labelClass}>{ts(aiScreener.minOI, language)}</label>
            <input
              type="number"
              value={config.min_oi ?? ''}
              onChange={(e) => update({ min_oi: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="15000000"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-xs text-nofx-text-muted mt-1 block">USDT</span>
          </div>
        </div>
        <div>
          <label className={labelClass}>{ts(aiScreener.priceChangeRange, language)}</label>
          <div className="flex items-center gap-2">
            <input
              type="number"
              value={config.min_price_change_pct ?? ''}
              onChange={(e) => update({ min_price_change_pct: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="-30"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-nofx-text-muted text-sm">%</span>
            <span className="text-nofx-text-muted text-sm">~</span>
            <input
              type="number"
              value={config.max_price_change_pct ?? ''}
              onChange={(e) => update({ max_price_change_pct: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="30"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-nofx-text-muted text-sm">%</span>
          </div>
        </div>
      </div>

      {/* Sentiment Preferences */}
      <div className={sectionClass}>
        <div className={sectionTitleClass}>{ts(aiScreener.sentimentPrefs, language)}</div>
        <div className="grid grid-cols-2 gap-3">
          {([
            { key: 'prefer_long_bias' as const, label: aiScreener.preferLong },
            { key: 'prefer_short_bias' as const, label: aiScreener.preferShort },
            { key: 'prefer_high_oi_growth' as const, label: aiScreener.preferOIGrowth },
            { key: 'prefer_high_volume_growth' as const, label: aiScreener.preferVolumeGrowth },
          ]).map(({ key, label }) => (
            <label key={key} className="flex items-center gap-3 cursor-pointer">
              <button
                type="button"
                role="switch"
                aria-checked={!!config[key]}
                onClick={() => !disabled && update({ [key]: !config[key] })}
                disabled={disabled}
                className={`relative w-10 h-5 rounded-full transition-colors ${
                  config[key] ? 'bg-nofx-gold' : 'bg-nofx-border'
                }`}
              >
                <span
                  className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                    config[key] ? 'translate-x-5' : ''
                  }`}
                />
              </button>
              <span className="text-sm text-nofx-text">{ts(label, language)}</span>
            </label>
          ))}
        </div>
        <p className="text-xs text-nofx-text-muted">{ts(aiScreener.sentimentNote, language)}</p>
      </div>

      {/* Volatility Range */}
      <div className={sectionClass}>
        <div className={sectionTitleClass}>{ts(aiScreener.volatilityRange, language)}</div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>{ts(aiScreener.minAtr, language)}</label>
            <input
              type="number"
              step="0.1"
              value={config.min_atr_pct ?? ''}
              onChange={(e) => update({ min_atr_pct: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="1"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-xs text-nofx-text-muted mt-1 block">{ts(aiScreener.atrUnit, language)}</span>
          </div>
          <div>
            <label className={labelClass}>{ts(aiScreener.maxAtr, language)}</label>
            <input
              type="number"
              step="0.1"
              value={config.max_atr_pct ?? ''}
              onChange={(e) => update({ max_atr_pct: e.target.value ? Number(e.target.value) : undefined })}
              placeholder="10"
              disabled={disabled}
              className={inputClass}
            />
            <span className="text-xs text-nofx-text-muted mt-1 block">{ts(aiScreener.atrUnit, language)}</span>
          </div>
        </div>
      </div>

      {/* Custom AI Instruction */}
      <div className={sectionClass}>
        <div className={sectionTitleClass}>{ts(aiScreener.customInstruction, language)}</div>
        <textarea
          value={config.custom_instruction || ''}
          onChange={(e) => update({ custom_instruction: e.target.value || undefined })}
          placeholder={ts(aiScreener.customPlaceholder, language)}
          disabled={disabled}
          rows={3}
          className={`${inputClass} resize-none`}
        />
      </div>

      {/* Preview */}
      <div className={sectionClass}>
        <button
          type="button"
          onClick={handlePreview}
          disabled={disabled || previewLoading}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-nofx-gold/20 text-nofx-gold text-sm font-medium hover:bg-nofx-gold/30 transition-colors disabled:opacity-50"
        >
          {previewLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Eye className="w-4 h-4" />}
          {ts(aiScreener.preview, language)}
        </button>

        {previewError && (
          <p className="text-xs text-red-400">{previewError}</p>
        )}

        {previewData.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-nofx-text-muted border-b border-nofx-border">
                  <th className="text-left py-2 pr-4">{ts(aiScreener.previewSymbol, language)}</th>
                  <th className="text-right py-2 pr-4">{ts(aiScreener.previewVolume, language)}</th>
                  <th className="text-right py-2 pr-4">{ts(aiScreener.previewOI, language)}</th>
                  <th className="text-right py-2">{ts(aiScreener.previewChange, language)}</th>
                </tr>
              </thead>
              <tbody>
                {previewData.map((coin) => (
                  <tr key={coin.symbol} className="border-b border-nofx-border/50">
                    <td className="py-1.5 pr-4 text-nofx-text font-medium">{coin.symbol}</td>
                    <td className="py-1.5 pr-4 text-right text-nofx-text-muted">{formatNumber(coin.volume_24h)}</td>
                    <td className="py-1.5 pr-4 text-right text-nofx-text-muted">{formatNumber(coin.open_interest)}</td>
                    <td className={`py-1.5 text-right ${coin.price_change_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {coin.price_change_pct >= 0 ? '+' : ''}{coin.price_change_pct.toFixed(2)}%
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!previewLoading && previewData.length === 0 && previewError === '' && null}
      </div>
    </div>
  )
}
