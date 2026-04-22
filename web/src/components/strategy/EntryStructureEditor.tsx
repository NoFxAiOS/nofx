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
  audit_primary_timeframe: true,
  audit_adjacent_timeframes: true,
  audit_support_resistance: true,
  audit_structural_anchors: true,
  audit_fibonacci: true,
  require_invalidation_target_linkage: true,
}

export function normalizeEntryStructureConfig(config?: EntryStructureConfig): EntryStructureConfig {
  return {
    ...defaultEntryStructureConfig,
    ...(config || {}),
  }
}

export function EntryStructureEditor({ config, onChange, disabled = false, language = 'en' }: EntryStructureEditorProps) {
  const safe = normalizeEntryStructureConfig(config)
  const inactive = disabled || !safe.enabled

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

      <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/5 p-4 space-y-3">
        <div>
          <div className="text-sm font-medium text-nofx-text-main">
            {language === 'zh' ? '结构化开仓要求' : 'Open-action structure requirements'}
          </div>
          <div className="text-xs text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '这些开关会影响运行时校验：缺少必要结构证据时，系统应输出 wait / []，而不是强行开仓。'
              : 'These toggles affect runtime validation: when required structure is missing, the system should return wait / [] instead of forcing an open.'}
          </div>
        </div>
        {safe.enabled && safe.require_fibonacci && (!safe.require_support_resistance || !safe.require_structural_anchors) && (
          <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs" style={{ color: '#FCD34D' }}>
            {language === 'zh'
              ? '提示：Fibonacci 单独启用时语义偏弱，建议同时启用“支撑/阻力”和“结构锚点”。'
              : 'Hint: Fibonacci is weak in isolation. Pair it with support/resistance and structural anchors for a stronger contract.'}
          </div>
        )}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {[
            ['require_primary_timeframe', language === 'zh' ? '必须主周期' : 'Require primary timeframe'],
            ['require_adjacent_timeframes', language === 'zh' ? '必须相邻周期' : 'Require adjacent timeframes'],
            ['require_support_resistance', language === 'zh' ? '必须支撑/阻力' : 'Require support/resistance'],
            ['require_structural_anchors', language === 'zh' ? '必须结构锚点' : 'Require structural anchors'],
            ['require_fibonacci', language === 'zh' ? '必须斐波那契' : 'Require fibonacci'],
            ['require_invalidation_target_linkage', language === 'zh' ? '要求失效/目标联动审计' : 'Require invalidation/target linkage'],
          ].map(([key, label]) => (
            <label key={key} className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-center justify-between gap-3">
              <span className="text-sm text-nofx-text-main">{label}</span>
              <input
                type="checkbox"
                checked={Boolean(safe[key as keyof EntryStructureConfig])}
                disabled={inactive}
                onChange={(e) => update({ [key]: e.target.checked } as Partial<EntryStructureConfig>)}
              />
            </label>
          ))}
        </div>
      </div>

      <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4 space-y-3">
        <div>
          <div className="text-sm font-medium text-nofx-text-main">
            {language === 'zh' ? '实盘审计可见性' : 'Runtime audit visibility'}
          </div>
          <div className="text-xs text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '控制持仓历史里应该突出哪些结构证据，方便复盘真实开仓。当前先作为 UI / 数据契约前置。'
              : 'Control which structural evidence should be highlighted in position history so real opens are easier to audit. For now this acts as a UI/data-contract readiness layer.'}
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {[
            ['audit_primary_timeframe', language === 'zh' ? '显示主周期' : 'Show primary timeframe'],
            ['audit_adjacent_timeframes', language === 'zh' ? '显示相邻周期' : 'Show adjacent timeframes'],
            ['audit_support_resistance', language === 'zh' ? '显示支撑/阻力' : 'Show support/resistance'],
            ['audit_structural_anchors', language === 'zh' ? '显示结构锚点' : 'Show structural anchors'],
            ['audit_fibonacci', language === 'zh' ? '显示斐波那契' : 'Show fibonacci'],
          ].map(([key, label]) => (
            <label key={key} className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-center justify-between gap-3">
              <span className="text-sm text-nofx-text-main">{label}</span>
              <input
                type="checkbox"
                checked={Boolean(safe[key as keyof EntryStructureConfig])}
                disabled={inactive}
                onChange={(e) => update({ [key]: e.target.checked } as Partial<EntryStructureConfig>)}
              />
            </label>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多支撑位数量' : 'Max support levels'}</div>
          <input type="number" min={1} max={8} value={safe.max_support_levels || 3} disabled={inactive} onChange={(e) => update({ max_support_levels: Number(e.target.value || 3) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多阻力位数量' : 'Max resistance levels'}</div>
          <input type="number" min={1} max={8} value={safe.max_resistance_levels || 3} disabled={inactive} onChange={(e) => update({ max_resistance_levels: Number(e.target.value || 3) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
        <label className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs text-nofx-text-muted">{language === 'zh' ? '最多结构锚点数量' : 'Max structural anchors'}</div>
          <input type="number" min={1} max={8} value={safe.max_anchor_count || 4} disabled={inactive} onChange={(e) => update({ max_anchor_count: Number(e.target.value || 4) })} className="w-full px-3 py-2 rounded bg-[#0B0E11] border border-[#2B3139] text-sm text-nofx-text-main" />
        </label>
      </div>
    </div>
  )
}
