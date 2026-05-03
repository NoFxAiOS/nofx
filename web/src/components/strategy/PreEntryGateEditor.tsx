import { Filter, Shield, Radio, Layers } from 'lucide-react'
import type { RegimeFilterConfig, EntryStructureConfig, StrategyControlPolicyMode } from '../../types'
import { EntryStructureEditor } from './EntryStructureEditor'
import { preEntryGate, ts } from '../../i18n/strategy-translations'

interface PreEntryGateEditorProps {
  config: RegimeFilterConfig
  onChange: (config: RegimeFilterConfig) => void
  disabled?: boolean
  language: string
}

const inputStyle = {
  background: '#1E2329',
  border: '1px solid #2B3139',
  color: '#EAECEF',
}

const sectionStyle = {
  background: '#0B0E11',
  border: '1px solid #2B3139',
}

const regimeOptions = [
  { value: 'narrow', zh: '窄波动', en: 'Narrow' },
  { value: 'standard', zh: '标准波动', en: 'Standard' },
  { value: 'wide', zh: '宽波动', en: 'Wide' },
  { value: 'trending', zh: '趋势（双向）', en: 'Trending (Both)' },
  { value: 'trending_up', zh: '上涨趋势', en: 'Uptrend' },
  { value: 'trending_down', zh: '下跌趋势', en: 'Downtrend' },
  { value: 'volatile', zh: '极端波动', en: 'Volatile' },
]

const policyModes: { value: StrategyControlPolicyMode; labelKey: 'strict' | 'auditOnly' | 'recommendOnly'; descKey: 'strictDesc' | 'auditOnlyDesc' | 'recommendOnlyDesc'; color: string }[] = [
  { value: 'strict', labelKey: 'strict', descKey: 'strictDesc', color: '#F6465D' },
  { value: 'audit_only', labelKey: 'auditOnly', descKey: 'auditOnlyDesc', color: '#F0B90B' },
  { value: 'recommend_only', labelKey: 'recommendOnly', descKey: 'recommendOnlyDesc', color: '#0ECB81' },
]

export function PreEntryGateEditor({ config, onChange, disabled, language }: PreEntryGateEditorProps) {
  const isZh = language === 'zh'

  const update = <K extends keyof RegimeFilterConfig>(key: K, value: RegimeFilterConfig[K]) => {
    if (!disabled) onChange({ ...config, [key]: value })
  }

  const toggleRegime = (regime: string) => {
    const current = config.allowed_regimes || []
    const exists = current.includes(regime)
    update('allowed_regimes', exists ? current.filter((r) => r !== regime) : [...current, regime])
  }

  return (
    <div className="space-y-6">
      {/* Header + Gate Flow */}
      <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #38BDF833' }}>
        <div className="flex items-center gap-2 mb-3">
          <Filter className="w-5 h-5" style={{ color: '#38BDF8' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>{ts(preEntryGate.title, language)}</h3>
        </div>
        <div className="flex items-center justify-between mb-3">
          <label className="text-sm" style={{ color: '#EAECEF' }}>{ts(preEntryGate.enableRegimeFilter, language)}</label>
          <input type="checkbox" checked={config.enabled} onChange={(e) => update('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
        </div>
        <div className="p-3 rounded-lg font-mono text-xs" style={{ background: '#11161C', border: '1px solid #2B3139', color: '#38BDF8' }}>
          {ts(preEntryGate.gateFlow, language)}
        </div>
        <div className="mt-2 text-xs" style={{ color: '#848E9C' }}>
          {ts(preEntryGate.mutualExclusion, language)}
        </div>
      </div>

      {/* Section 1: Market State Gate */}
      <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4" style={{ color: '#38BDF8' }} />
          <h4 className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            ① {ts(preEntryGate.marketStateGate, language)}
          </h4>
        </div>

        {/* Allowed Regimes */}
        <div>
          <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>{ts(preEntryGate.allowedRegimes, language)}</label>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            {regimeOptions.map((option) => {
              const active = (config.allowed_regimes || []).includes(option.value)
              return (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => toggleRegime(option.value)}
                  disabled={disabled}
                  className="px-3 py-2 rounded text-sm border"
                  style={{
                    background: active ? '#1E3A5F' : '#11161C',
                    borderColor: active ? '#38BDF8' : '#2B3139',
                    color: active ? '#EAECEF' : '#848E9C',
                  }}
                >
                  {isZh ? option.zh : option.en}
                </button>
              )
            })}
          </div>
        </div>

        {/* Funding Rate */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <input type="checkbox" checked={config.block_high_funding} onChange={(e) => update('block_high_funding', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
              <label className="text-sm" style={{ color: '#EAECEF' }}>{ts(preEntryGate.blockHighFunding, language)}</label>
            </div>
            <input type="number" value={config.max_funding_rate_abs} min={0} step={0.001} onChange={(e) => update('max_funding_rate_abs', parseFloat(e.target.value) || 0)} disabled={disabled || !config.block_high_funding} className="w-full px-3 py-2 rounded" style={inputStyle} />
            <div className="text-[11px] mt-1" style={{ color: '#848E9C' }}>{ts(preEntryGate.fundingRateUnit, language)}</div>
          </div>

          <div>
            <div className="flex items-center gap-2 mb-1">
              <input type="checkbox" checked={config.block_high_volatility} onChange={(e) => update('block_high_volatility', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
              <label className="text-sm" style={{ color: '#EAECEF' }}>{ts(preEntryGate.blockHighVolatility, language)}</label>
            </div>
            <input type="number" value={config.max_atr14_pct} min={0} step={0.1} onChange={(e) => update('max_atr14_pct', parseFloat(e.target.value) || 0)} disabled={disabled || !config.block_high_volatility} className="w-full px-3 py-2 rounded" style={inputStyle} />
            <div className="text-[11px] mt-1" style={{ color: '#848E9C' }}>{ts(preEntryGate.atrUnit, language)}</div>
          </div>
        </div>

        {/* Trend Alignment */}
        <div className="space-y-2 rounded-lg p-3" style={{ background: '#11161C', border: '1px solid #2B3139' }}>
          <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}>
            <input type="checkbox" checked={config.require_trend_alignment} onChange={(e) => update('require_trend_alignment', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
            {ts(preEntryGate.requireTrendAlignment, language)}
          </label>
          <div className="text-[11px] leading-relaxed" style={{ color: '#848E9C' }}>
            {isZh ? '注意：即使允许了“下跌趋势”，开启趋势同向后仍会拒绝下跌趋势里的做多；允许“上涨趋势”也仍会拒绝上涨趋势里的做空。' : 'Note: allowing a regime does not allow both directions; with trend alignment on, longs are blocked in downtrends and shorts are blocked in uptrends.'}
          </div>
          {config.require_trend_alignment && (
            <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}>
              <input
                type="checkbox"
                checked={(config.trend_alignment_mode || 'strict') === 'allow_range_edge_reversal'}
                onChange={(e) => update('trend_alignment_mode', (e.target.checked ? 'allow_range_edge_reversal' : 'strict') as RegimeFilterConfig['trend_alignment_mode'])}
                disabled={disabled}
                className="h-4 w-4 accent-amber-500"
              />
              {isZh ? '允许 range_edge 支撑/阻力逆势例外' : 'Allow range_edge support/resistance reversal exception'}
            </label>
          )}
          {config.require_trend_alignment && (config.trend_alignment_mode || 'strict') === 'allow_range_edge_reversal' && (
            <div className="text-[11px] leading-relaxed" style={{ color: '#F0B90B' }}>
              {isZh ? '仅对 setup_type=range_edge 生效；仍要求价格接近布林/结构边缘、短线动量不过度极端，并继续经过 RR、结构、保护门禁。' : 'Only applies to setup_type=range_edge; price must be near a band/structure edge, momentum must not be extreme, and RR/structure/protection gates still apply.'}
            </div>
          )}
        </div>
      </div>

      {/* Section 2: Entry Structure */}
      <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
        <div className="flex items-center gap-2">
          <Layers className="w-4 h-4" style={{ color: '#60A5FA' }} />
          <h4 className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            ② {ts(preEntryGate.entryStructure, language)}
          </h4>
        </div>

        <div className="p-3 rounded-lg space-y-3" style={{ background: '#11161C', border: '1px solid #2B3139' }}>
          <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}>
            <input
              type="checkbox"
              checked={config.entry_structure?.entry_gate?.enabled ?? true}
              onChange={(e) => update('entry_structure', {
                ...(config.entry_structure || {}),
                entry_gate: { ...(config.entry_structure?.entry_gate || {}), enabled: e.target.checked },
              } as EntryStructureConfig)}
              disabled={disabled}
              className="h-4 w-4 accent-amber-500"
            />
            {isZh ? '启用 ATR / 入场位置门禁' : 'Enable ATR / entry-position gate'}
          </label>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {[
              ['min_atr14_pct', isZh ? '最小 ATR14 %' : 'Min ATR14 %', 1.2, 0.1],
              ['min_risk_distance_pct', isZh ? '最小止损距离 %' : 'Min risk distance %', 0.4, 0.05],
              ['entry_proximity_atr_mul', isZh ? '入场贴近 ATR倍数' : 'Entry proximity ATR x', 0.6, 0.1],
              ['entry_proximity_max_pct', isZh ? '入场最大偏离 %' : 'Max entry gap %', 1.5, 0.1],
              ['invalidation_structure_atr_mul', isZh ? '止损结构 ATR倍数' : 'Invalidation ATR x', 0.5, 0.1],
              ['max_blocking_levels', isZh ? '最多路径阻力/支撑层' : 'Max blocking levels', 4, 1],
            ].map(([key, label, def, step]) => (
              <div key={key as string}>
                <label className="block text-[11px] mb-1" style={{ color: '#848E9C' }}>{label as string}</label>
                <input
                  type="number"
                  value={(config.entry_structure?.entry_gate as any)?.[key as string] ?? def}
                  step={step as number}
                  min={0}
                  onChange={(e) => update('entry_structure', {
                    ...(config.entry_structure || {}),
                    entry_gate: { ...(config.entry_structure?.entry_gate || {}), [key as string]: parseFloat(e.target.value) || 0 },
                  } as EntryStructureConfig)}
                  disabled={disabled || !(config.entry_structure?.entry_gate?.enabled ?? true)}
                  className="w-full px-3 py-2 rounded text-sm"
                  style={inputStyle}
                />
              </div>
            ))}
          </div>
          <div className="text-[11px] leading-relaxed" style={{ color: '#F0B90B' }}>
            {isZh
              ? '门禁顺序：市场状态/窄波动 → 结构字段 → ATR/入场贴近 → 信心/RR → 保护计划。目标位只用于 RR 与保护分层，不再要求强贴近最近阻力/支撑。'
              : 'Gate order: market state/narrow volatility → structure fields → ATR/entry proximity → confidence/RR → protection plan. Target is used for RR/protection tiers, not strict nearest S/R alignment.'}
          </div>
        </div>

        <EntryStructureEditor
          config={config.entry_structure}
          onChange={(entryStructure: EntryStructureConfig) => update('entry_structure', entryStructure)}
          disabled={disabled}
          language={language}
        />
      </div>

      {/* Section 3: Confidence Gate */}
      <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
        <div className="flex items-center gap-2">
          <Shield className="w-4 h-4" style={{ color: '#0ECB81' }} />
          <h4 className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            ③ {ts(preEntryGate.confidenceGate, language)}
          </h4>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Min Confidence */}
          <div className="p-3 rounded-lg" style={{ background: '#11161C', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>{ts(preEntryGate.minConfidence, language)}</label>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) => update('min_confidence', parseInt(e.target.value))}
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {config.min_confidence ?? 75}
              </span>
            </div>
            <div className="text-[11px] mt-1" style={{ color: '#848E9C' }}>{ts(preEntryGate.confidenceUnit, language)}</div>
          </div>

          {/* Min Risk-Reward Ratio */}
          <div className="p-3 rounded-lg" style={{ background: '#11161C', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>{ts(preEntryGate.minRiskReward, language)}</label>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) => update('min_risk_reward_ratio', parseFloat(e.target.value) || 3)}
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded ml-2"
                style={inputStyle}
              />
            </div>
            <div className="text-[11px] mt-1" style={{ color: '#848E9C' }}>{ts(preEntryGate.rrUnit, language)}</div>
          </div>
        </div>
      </div>

      {/* Section 4: Policy Mode */}
      <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
        <div className="flex items-center gap-2">
          <Radio className="w-4 h-4" style={{ color: '#A855F7' }} />
          <h4 className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            {ts(preEntryGate.policyMode, language)}
          </h4>
        </div>

        <div className="space-y-2">
          {policyModes.map((pm) => {
            const selected = (config.policy_mode || 'strict') === pm.value
            return (
              <label
                key={pm.value}
                className="flex items-start gap-3 p-3 rounded-lg cursor-pointer border"
                style={{
                  background: selected ? '#11161C' : 'transparent',
                  borderColor: selected ? pm.color : '#2B3139',
                }}
              >
                <input
                  type="radio"
                  name="policy_mode"
                  value={pm.value}
                  checked={selected}
                  onChange={() => update('policy_mode', pm.value)}
                  disabled={disabled}
                  className="mt-0.5 accent-purple-500"
                />
                <div>
                  <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{ts(preEntryGate[pm.labelKey], language)}</div>
                  <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>{ts(preEntryGate[pm.descKey], language)}</div>
                </div>
              </label>
            )
          })}
        </div>
      </div>
    </div>
  )
}
