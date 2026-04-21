import type { EntryStructureConfig } from '../../types'

interface EntryStructureEditorProps {
  config?: EntryStructureConfig
  onChange: (config: EntryStructureConfig) => void
  disabled?: boolean
  language?: string
}

const defaultEntryStructureConfig: EntryStructureConfig = {
  enabled: true,
  require_primary_timeframe: true,
  require_adjacent_timeframes: true,
  require_support_resistance: true,
  require_structural_anchors: true,
  require_fibonacci: false,
  max_support_levels: 3,
  max_resistance_levels: 3,
  max_anchor_count: 4,
}

export function normalizeEntryStructureConfig(config?: EntryStructureConfig): EntryStructureConfig {
  return {
    ...defaultEntryStructureConfig,
    ...(config || {}),
  }
}

export function EntryStructureEditor({ config, onChange, disabled = false, language = 'en' }: EntryStructureEditorProps) {
  const safe = normalizeEntryStructureConfig(config)

  const update = (patch: Partial<EntryStructureConfig>) => {
    onChange({ ...safe, ...patch })
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-white/10 bg-black/20 p-4 space-y-3">
        <label className="flex items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-nofx-text-main">
              {language === 'zh' ? '启用结构化开仓约束' : 'Enable structural entry contract'}
            </div>
            <div className="text-xs text-nofx-text-muted mt-1">
              {language === 'zh' ? '只要求开仓必要结构证据，不鼓励堆数据。' : 'Require only the structural evidence needed for opening; do not encourage noisy data.'}
            </div>
          </div>
          <input type="checkbox" checked={safe.enabled} disabled={disabled} onChange={(e) => update({ enabled: e.target.checked })} />
        </label>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {[
          ['require_primary_timeframe', language === 'zh' ? '必须主周期' : 'Require primary timeframe'],
          ['require_adjacent_timeframes', language === 'zh' ? '必须相邻周期' : 'Require adjacent timeframes'],
          ['require_support_resistance', language === 'zh' ? '必须支撑/阻力' : 'Require support/resistance'],
          ['require_structural_anchors', language === 'zh' ? '必须结构锚点' : 'Require structural anchors'],
          ['require_fibonacci', language === 'zh' ? '必须斐波那契' : 'Require fibonacci'],
        ].map(([key, label]) => (
          <label key={key} className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-center justify-between gap-3">
            <span className="text-sm text-nofx-text-main">{label}</span>
            <input
              type="checkbox"
              checked={Boolean(safe[key as keyof EntryStructureConfig])}
              disabled={disabled || !safe.enabled}
              onChange={(e) => update({ [key]: e.target.checked } as Partial<EntryStructureConfig>)}
            />
          </label>
        ))}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多支撑位数量' : 'Max support levels'}</div>
          <input type="number" min={1} max={8} value={safe.max_support_levels || 3} disabled={disabled || !safe.enabled} onChange={(e) => update({ max_support_levels: Number(e.target.value || 3) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多阻力位数量' : 'Max resistance levels'}</div>
          <input type="number" min={1} max={8} value={safe.max_resistance_levels || 3} disabled={disabled || !safe.enabled} onChange={(e) => update({ max_resistance_levels: Number(e.target.value || 3) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多结构锚点数量' : 'Max structural anchors'}</div>
          <input type="number" min={1} max={8} value={safe.max_anchor_count || 4} disabled={disabled || !safe.enabled} onChange={(e) => update({ max_anchor_count: Number(e.target.value || 4) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
      </div>
    </div>
  )
}
