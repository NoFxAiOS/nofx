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
              ? '这些是开仓硬约束：缺少必要结构证据时，系统应输出 wait / []，而不是强行开仓。各项不是互斥关系，而是逐层叠加。'
              : 'These are hard entry gates: when required structure is missing, return wait / [] instead of forcing an open. Toggles are additive, not mutually exclusive.'}
          </div>
        </div>
        {safe.enabled && safe.require_fibonacci && (!safe.require_support_resistance || !safe.require_structural_anchors) && (
          <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs" style={{ color: '#FCD34D' }}>
            {language === 'zh'
              ? '提示：Fibonacci 单独启用时语义偏弱，建议同时启用“支撑/阻力地图”和“本单结构锚点”。'
              : 'Hint: Fibonacci is weak in isolation. Pair it with support/resistance and structural anchors for a stronger contract.'}
          </div>
        )}

        <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs font-medium text-cyan-300">{language === 'zh' ? '1. 时间框架要求' : '1. Timeframe requirements'}</div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {[
              ['require_primary_timeframe', language === 'zh' ? '必须主周期' : 'Require primary timeframe', language === 'zh' ? '要求 AI 明确主判断周期，例如 15m。' : 'Require the primary decision timeframe, e.g. 15m.'],
              ['require_adjacent_timeframes', language === 'zh' ? '必须相邻周期' : 'Require adjacent timeframe', language === 'zh' ? '要求至少一个 lower 或 higher 周期辅助确认。' : 'Require at least one lower or higher timeframe for confirmation.'],
            ].map(([key, label, desc]) => (
              <label key={key} className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-start justify-between gap-3">
                <span>
                  <span className="block text-sm text-nofx-text-main">{label}</span>
                  <span className="block text-[11px] text-nofx-text-muted mt-1">{desc}</span>
                </span>
                <input type="checkbox" checked={Boolean(safe[key as keyof EntryStructureConfig])} disabled={inactive} onChange={(e) => update({ [key]: e.target.checked } as Partial<EntryStructureConfig>)} />
              </label>
            ))}
          </div>
        </div>

        <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs font-medium text-cyan-300">{language === 'zh' ? '2. 结构证据要求' : '2. Structural evidence requirements'}</div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {[
              ['require_support_resistance', language === 'zh' ? '必须支撑/阻力地图' : 'Require support/resistance map', language === 'zh' ? '要求给出支撑和阻力列表。' : 'Require support and resistance level lists.'],
              ['require_structural_anchors', language === 'zh' ? '必须本单结构锚点' : 'Require trade-specific anchors', language === 'zh' ? '要求说明本单具体引用哪些结构位。' : 'Require the specific anchors used by this trade.'],
              ['require_fibonacci', language === 'zh' ? '必须 Fibonacci 共振' : 'Require Fibonacci confluence', language === 'zh' ? '要求 fib swing 和 levels；更严格，开仓会更少。' : 'Require fib swing anchors and levels; stricter, fewer entries.'],
            ].map(([key, label, desc]) => (
              <label key={key} className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-start justify-between gap-3">
                <span>
                  <span className="block text-sm text-nofx-text-main">{label}</span>
                  <span className="block text-[11px] text-nofx-text-muted mt-1">{desc}</span>
                </span>
                <input type="checkbox" checked={Boolean(safe[key as keyof EntryStructureConfig])} disabled={inactive} onChange={(e) => update({ [key]: e.target.checked } as Partial<EntryStructureConfig>)} />
              </label>
            ))}
          </div>
        </div>

        <div className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-2">
          <div className="text-xs font-medium text-cyan-300">{language === 'zh' ? '3. 价格联动要求' : '3. Price-linkage requirement'}</div>
          <label className="rounded-lg border border-white/10 bg-black/20 p-3 flex items-start justify-between gap-3">
            <span>
              <span className="block text-sm text-nofx-text-main">{language === 'zh' ? '止损/目标必须贴近结构位' : 'SL/target must link to structure'}</span>
              <span className="block text-[11px] text-nofx-text-muted mt-1">{language === 'zh' ? '检查 invalidation / first target 是否真的靠近支撑、阻力、锚点或 fib，而不是只在文字里说参考结构。' : 'Check whether invalidation / first target are truly near support, resistance, anchors, or fib levels.'}</span>
            </span>
            <input type="checkbox" checked={Boolean(safe.require_invalidation_target_linkage)} disabled={inactive} onChange={(e) => update({ require_invalidation_target_linkage: e.target.checked })} />
          </label>
        </div>
      </div>

      <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4 space-y-3">
        <div>
          <div className="text-sm font-medium text-nofx-text-main">
            {language === 'zh' ? '复盘展示' : 'Review visibility'}
          </div>
          <div className="text-xs text-nofx-text-muted mt-1">
            {language === 'zh'
              ? '只控制持仓历史和面板里突出展示哪些结构证据，方便复盘真实开仓；不作为开仓硬约束。'
              : 'Only controls which structural evidence is highlighted in position history/panels for review; not a hard entry gate.'}
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
