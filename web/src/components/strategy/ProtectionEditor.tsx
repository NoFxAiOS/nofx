import { ShieldCheck, TrendingUp, TrendingDown, Layers, Activity, RotateCcw, Filter, Plus, Trash2 } from 'lucide-react'
import type {
  ProtectionConfig,
  FullTPSLConfig,
  LadderTPSLConfig,
  DrawdownTakeProfitConfig,
  BreakEvenStopConfig,
  RegimeFilterConfig,
  LadderTPSLRule,
  DrawdownTakeProfitRule,
  ProtectionMode,
  ProtectionValueSource,
} from '../../types'


interface ProtectionEditorProps {
  config: ProtectionConfig
  onChange: (config: ProtectionConfig) => void
  disabled?: boolean
  language: string
}

export const defaultProtectionConfig: ProtectionConfig = {
  full_tp_sl: {
    enabled: false,
    mode: 'manual',
    take_profit: { mode: 'manual', value: 0 },
    stop_loss: { mode: 'manual', value: 0 },
    fallback_max_loss: { mode: 'disabled', value: 0 },
  },
  ladder_tp_sl: {
    enabled: false,
    mode: 'manual',
    take_profit_enabled: false,
    stop_loss_enabled: false,
    take_profit_price: { mode: 'manual', value: 0 },
    take_profit_size: { mode: 'manual', value: 0 },
    stop_loss_price: { mode: 'manual', value: 0 },
    stop_loss_size: { mode: 'manual', value: 0 },
    fallback_max_loss: { mode: 'disabled', value: 0 },
    rules: [],
  },
  drawdown_take_profit: {
    enabled: false,
    rules: [{ min_profit_pct: 5, max_drawdown_pct: 40, close_ratio_pct: 100, poll_interval_seconds: 60 }],
  },
  break_even_stop: {
    enabled: false,
    trigger_mode: 'profit_pct',
    trigger_value: 3,
    offset_pct: 0.1,
  },
  regime_filter: {
    enabled: false,
    allowed_regimes: ['narrow', 'standard', 'wide'],
    block_high_funding: false,
    max_funding_rate_abs: 0.01,
    block_high_volatility: false,
    max_atr14_pct: 3,
    require_trend_alignment: false,
  },
}

export const normalizeProtectionConfig = (config?: Partial<ProtectionConfig> | null): ProtectionConfig => ({
  ...defaultProtectionConfig,
  ...config,
  full_tp_sl: {
    ...defaultProtectionConfig.full_tp_sl,
    ...(config?.full_tp_sl || {}),
    take_profit: { ...defaultProtectionConfig.full_tp_sl.take_profit, ...(config?.full_tp_sl?.take_profit || {}) },
    stop_loss: { ...defaultProtectionConfig.full_tp_sl.stop_loss, ...(config?.full_tp_sl?.stop_loss || {}) },
    fallback_max_loss: { ...defaultProtectionConfig.full_tp_sl.fallback_max_loss, ...(config?.full_tp_sl?.fallback_max_loss || {}) },
  },
  ladder_tp_sl: {
    ...defaultProtectionConfig.ladder_tp_sl,
    ...(config?.ladder_tp_sl || {}),
    take_profit_price: { ...defaultProtectionConfig.ladder_tp_sl.take_profit_price, ...(config?.ladder_tp_sl?.take_profit_price || {}) },
    take_profit_size: { ...defaultProtectionConfig.ladder_tp_sl.take_profit_size, ...(config?.ladder_tp_sl?.take_profit_size || {}) },
    stop_loss_price: { ...defaultProtectionConfig.ladder_tp_sl.stop_loss_price, ...(config?.ladder_tp_sl?.stop_loss_price || {}) },
    stop_loss_size: { ...defaultProtectionConfig.ladder_tp_sl.stop_loss_size, ...(config?.ladder_tp_sl?.stop_loss_size || {}) },
    fallback_max_loss: { ...defaultProtectionConfig.ladder_tp_sl.fallback_max_loss, ...(config?.ladder_tp_sl?.fallback_max_loss || {}) },
    rules: config?.ladder_tp_sl?.rules || defaultProtectionConfig.ladder_tp_sl.rules,
  },
  drawdown_take_profit: {
    ...defaultProtectionConfig.drawdown_take_profit,
    ...(config?.drawdown_take_profit || {}),
    rules: config?.drawdown_take_profit?.rules || defaultProtectionConfig.drawdown_take_profit.rules,
  },
  break_even_stop: {
    ...defaultProtectionConfig.break_even_stop,
    ...(config?.break_even_stop || {}),
  },
  regime_filter: {
    ...defaultProtectionConfig.regime_filter,
    ...(config?.regime_filter || {}),
  },
})

export function ProtectionEditor({ config, onChange, disabled, language }: ProtectionEditorProps) {
  const isZh = language === 'zh'

  const inputStyle = {
    background: '#1E2329',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }

  const sectionStyle = {
    background: '#0B0E11',
    border: '1px solid #2B3139',
  }

  const helpCardStyle = {
    background: '#11161C',
    border: '1px solid #2B3139',
  }

  const updateSection = <K extends keyof ProtectionConfig>(key: K, value: ProtectionConfig[K]) => {
    if (!disabled) onChange({ ...config, [key]: value })
  }

  const updateFull = <K extends keyof FullTPSLConfig>(key: K, value: FullTPSLConfig[K]) => {
    updateSection('full_tp_sl', { ...config.full_tp_sl, [key]: value })
  }

  const updateLadder = <K extends keyof LadderTPSLConfig>(key: K, value: LadderTPSLConfig[K]) => {
    updateSection('ladder_tp_sl', { ...config.ladder_tp_sl, [key]: value })
  }

  const updateDrawdown = <K extends keyof DrawdownTakeProfitConfig>(key: K, value: DrawdownTakeProfitConfig[K]) => {
    updateSection('drawdown_take_profit', { ...config.drawdown_take_profit, [key]: value })
  }

  const updateBreakEven = <K extends keyof BreakEvenStopConfig>(key: K, value: BreakEvenStopConfig[K]) => {
    updateSection('break_even_stop', { ...config.break_even_stop, [key]: value })
  }

  const updateRegimeFilter = <K extends keyof RegimeFilterConfig>(key: K, value: RegimeFilterConfig[K]) => {
    updateSection('regime_filter', { ...config.regime_filter, [key]: value })
  }

  const drawdownRules = config.drawdown_take_profit.rules || []
  const ladderRules = config.ladder_tp_sl.rules || []

  const addLadderRule = () => {
    const nextRule: LadderTPSLRule = {
      take_profit_pct: 3,
      take_profit_close_ratio_pct: 30,
      stop_loss_pct: 2,
      stop_loss_close_ratio_pct: 50,
    }
    updateLadder('rules', [...ladderRules, nextRule])
  }

  const updateLadderRule = (index: number, patch: Partial<LadderTPSLRule>) => {
    const nextRules = [...ladderRules]
    nextRules[index] = { ...nextRules[index], ...patch }
    updateLadder('rules', nextRules)
  }

  const removeLadderRule = (index: number) => {
    updateLadder('rules', ladderRules.filter((_, i) => i !== index))
  }

  const addDrawdownRule = () => {
    const nextRule: DrawdownTakeProfitRule = {
      min_profit_pct: 5,
      max_drawdown_pct: 40,
      close_ratio_pct: 100,
      poll_interval_seconds: 60,
    }
    updateDrawdown('rules', [...drawdownRules, nextRule])
  }

  const updateDrawdownRule = (index: number, patch: Partial<DrawdownTakeProfitRule>) => {
    const nextRules = [...drawdownRules]
    nextRules[index] = { ...nextRules[index], ...patch }
    updateDrawdown('rules', nextRules)
  }

  const removeDrawdownRule = (index: number) => {
    updateDrawdown('rules', drawdownRules.filter((_, i) => i !== index))
  }

  const toggleAllowedRegime = (regime: string) => {
    const current = config.regime_filter.allowed_regimes || []
    const exists = current.includes(regime)
    updateRegimeFilter('allowed_regimes', exists ? current.filter((item) => item !== regime) : [...current, regime])
  }

  const regimeOptions = [
    { value: 'narrow', zh: '窄波动', en: 'Narrow' },
    { value: 'standard', zh: '标准波动', en: 'Standard' },
    { value: 'wide', zh: '宽波动', en: 'Wide' },
    { value: 'trending', zh: '趋势强化', en: 'Trending' },
  ]

  const protectionModeOptions: ProtectionMode[] = ['disabled', 'manual', 'ai']
  const valueModeOptions: ProtectionMode[] = ['disabled', 'manual', 'ai']

  const modeLabel = (mode: ProtectionMode) => {
    if (mode === 'disabled') return isZh ? '禁用' : 'Disabled'
    return mode === 'ai'
      ? (isZh ? 'AI 动态保护模式' : 'AI Dynamic Protection')
      : (isZh ? '手动阈值模式' : 'Manual Threshold Mode')
  }

  const updateValueSource = (current: ProtectionValueSource, patch: Partial<ProtectionValueSource>): ProtectionValueSource => ({
    ...current,
    ...patch,
  })

  const triggerModeLabel = (mode: 'profit_pct' | 'r_multiple') => mode === 'r_multiple'
    ? (isZh ? '按 R 倍数触发' : 'Trigger by R Multiple')
    : (isZh ? '按盈利百分比触发' : 'Trigger by Profit %')

  const infoBlock = (title: string, description: string, recommend: string) => (
    <div className="p-3 rounded-lg space-y-1" style={helpCardStyle}>
      <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{title}</div>
      <div className="text-xs" style={{ color: '#AAB2BD' }}>{description}</div>
      <div className="text-xs" style={{ color: '#F0B90B' }}>{recommend}</div>
    </div>
  )

  const statusChip = (active: boolean, label: string) => (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium"
      style={{
        background: active ? 'rgba(14, 203, 129, 0.12)' : 'rgba(132, 142, 156, 0.12)',
        color: active ? '#0ECB81' : '#848E9C',
        border: active ? '1px solid rgba(14, 203, 129, 0.25)' : '1px solid rgba(132, 142, 156, 0.2)',
      }}
    >
      {label}
    </span>
  )

  const fullStateSummary = isZh
    ? `执行开关：${config.full_tp_sl.enabled ? '已启用' : '未启用'} · 整体模式：${modeLabel(config.full_tp_sl.mode)} · TP：${modeLabel(config.full_tp_sl.take_profit.mode)} · SL：${modeLabel(config.full_tp_sl.stop_loss.mode)}`
    : `Execution: ${config.full_tp_sl.enabled ? 'enabled' : 'disabled'} · Global mode: ${modeLabel(config.full_tp_sl.mode)} · TP: ${modeLabel(config.full_tp_sl.take_profit.mode)} · SL: ${modeLabel(config.full_tp_sl.stop_loss.mode)}`

  const ladderStateSummary = isZh
    ? `执行开关：${config.ladder_tp_sl.enabled ? '已启用' : '未启用'} · 整体模式：${modeLabel(config.ladder_tp_sl.mode)} · TP侧：${config.ladder_tp_sl.take_profit_enabled ? '开启' : '关闭'} · SL侧：${config.ladder_tp_sl.stop_loss_enabled ? '开启' : '关闭'}`
    : `Execution: ${config.ladder_tp_sl.enabled ? 'enabled' : 'disabled'} · Global mode: ${modeLabel(config.ladder_tp_sl.mode)} · TP side: ${config.ladder_tp_sl.take_profit_enabled ? 'on' : 'off'} · SL side: ${config.ladder_tp_sl.stop_loss_enabled ? 'on' : 'off'}`

  const fullModeMismatch = !config.full_tp_sl.enabled && config.full_tp_sl.mode === 'ai'
  const ladderModeMismatch = !config.ladder_tp_sl.enabled && config.ladder_tp_sl.mode === 'ai'


  const drawdownOwnsTp = config.drawdown_take_profit.enabled && (config.drawdown_take_profit.rules || []).length > 0
  const ladderTpEnabled = config.ladder_tp_sl.enabled && config.ladder_tp_sl.take_profit_enabled
  const fullTpEnabled = config.full_tp_sl.enabled && config.full_tp_sl.take_profit.mode !== 'disabled'

  return (
    <div className="space-y-6">
      <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #F0B90B33' }}>
        <div className="flex items-start gap-3">
          <ShieldCheck className="w-5 h-5 mt-0.5" style={{ color: '#F0B90B' }} />
          <div>
            <h3 className="font-medium mb-1" style={{ color: '#EAECEF' }}>
              {isZh ? '交易保护 / 盈利控制' : 'Trading Protection / Profit Control'}
            </h3>
            <p className="text-xs" style={{ color: '#848E9C' }}>
              {isZh
                ? '这里同时包含两类能力：一类是开仓后尽快挂到交易所的保护委托，另一类是系统在持仓期间持续监控并动态执行的运行态保护。'
                : 'This section contains both exchange-order-based protections and runtime-monitored protections enforced by the system while positions are open.'}
            </p>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? 'Full TP/SL（委托型保护）' : 'Full TP/SL (Order-based Protection)'}
          </h3>
        </div>

        <div className="space-y-4">
          {drawdownOwnsTp && fullTpEnabled && (
            <div className="p-3 rounded-lg text-xs" style={{ background: '#2B1619', border: '1px solid #41272B', color: '#F0B90B' }}>
              {isZh ? 'Drawdown Take Profit 已接管止盈侧，Full TP 会被抑制；Full SL 仍保留为长期止损。' : 'Drawdown Take Profit owns the take-profit side, so Full TP is suppressed while Full SL remains active as long-lived stop-loss.'}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2">
            {statusChip(config.full_tp_sl.enabled, isZh ? '执行开关' : 'Execution')}
            {statusChip(config.full_tp_sl.mode === 'ai', isZh ? '整体 AI' : 'Global AI')}
            {statusChip(config.full_tp_sl.take_profit.mode === 'ai', isZh ? 'TP 由 AI' : 'TP via AI')}
            {statusChip(config.full_tp_sl.stop_loss.mode === 'ai', isZh ? 'SL 由 AI' : 'SL via AI')}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>{fullStateSummary}</div>
          {fullModeMismatch && (
            <div className="p-3 rounded-lg text-xs" style={{ background: '#11161C', border: '1px solid #2B3139', color: '#F0B90B' }}>
              {isZh
                ? '注意：当前 Full 的“整体模式”是 AI，但“执行开关”仍关闭。页面会保留 AI 模式配置，但运行时不会实际挂 Full 保护单，直到你打开执行开关。'
                : 'Note: Full global mode is AI, but execution is still disabled. The page preserves the AI setting, but runtime will not place Full protection orders until execution is enabled.'}
            </div>
          )}

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <div>
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {isZh ? '启用 Full TP/SL' : 'Enable Full TP/SL'}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {isZh ? '开仓后为整仓挂统一止盈/止损保护单，并可额外挂最大损失兜底。' : 'Attach one unified TP/SL order set after opening, with optional max-loss fallback.'}
                </p>
              </div>
              <input type="checkbox" checked={config.full_tp_sl.enabled} onChange={(e) => updateFull('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-yellow-500" />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>{isZh ? '整体模式' : 'Global Mode'}</label>
              <select value={config.full_tp_sl.mode} onChange={(e) => updateFull('mode', e.target.value as FullTPSLConfig['mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {protectionModeOptions.map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
            </div>

            <div className="p-4 rounded-lg space-y-2" style={sectionStyle}>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止盈价格模式' : 'TP Price Mode'}</label>
              <select value={config.full_tp_sl.take_profit.mode} onChange={(e) => updateFull('take_profit', updateValueSource(config.full_tp_sl.take_profit, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
              {config.full_tp_sl.take_profit.mode === 'manual' && (
                <input type="number" min={0} step={0.1} value={config.full_tp_sl.take_profit.value} onChange={(e) => updateFull('take_profit', updateValueSource(config.full_tp_sl.take_profit, { value: parseFloat(e.target.value) || 0 }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              )}
            </div>

            <div className="p-4 rounded-lg space-y-2" style={sectionStyle}>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止损价格模式' : 'SL Price Mode'}</label>
              <select value={config.full_tp_sl.stop_loss.mode} onChange={(e) => updateFull('stop_loss', updateValueSource(config.full_tp_sl.stop_loss, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
              {config.full_tp_sl.stop_loss.mode === 'manual' && (
                <input type="number" min={0} step={0.1} value={config.full_tp_sl.stop_loss.value} onChange={(e) => updateFull('stop_loss', updateValueSource(config.full_tp_sl.stop_loss, { value: parseFloat(e.target.value) || 0 }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              )}
            </div>

            <div className="p-4 rounded-lg space-y-2" style={sectionStyle}>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '最大损失兜底' : 'Max Loss Fallback'}</label>
              <select value={config.full_tp_sl.fallback_max_loss.mode} onChange={(e) => updateFull('fallback_max_loss', updateValueSource(config.full_tp_sl.fallback_max_loss, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {(['disabled', 'manual'] as const).map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
              {config.full_tp_sl.fallback_max_loss.mode === 'manual' && (
                <input type="number" min={0} step={0.1} value={config.full_tp_sl.fallback_max_loss.value} onChange={(e) => updateFull('fallback_max_loss', updateValueSource(config.full_tp_sl.fallback_max_loss, { value: parseFloat(e.target.value) || 0 }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              )}
            </div>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <Layers className="w-5 h-5" style={{ color: '#60A5FA' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? 'Ladder TP/SL（分批委托型保护）' : 'Ladder TP/SL (Ladder Order Protection)'}
          </h3>
        </div>

        <div className="space-y-4">
          {ladderTpEnabled && drawdownOwnsTp && (
            <div className="p-3 rounded-lg text-xs" style={{ background: '#2B1619', border: '1px solid #41272B', color: '#F0B90B' }}>
              {isZh ? 'Drawdown Take Profit 已接管止盈侧，Ladder TP 会被抑制；Ladder SL 继续保留。' : 'Drawdown Take Profit owns the take-profit side, so Ladder TP is suppressed while Ladder SL remains active.'}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2">
            {statusChip(config.ladder_tp_sl.enabled, isZh ? '执行开关' : 'Execution')}
            {statusChip(config.ladder_tp_sl.mode === 'ai', isZh ? '整体 AI' : 'Global AI')}
            {statusChip(config.ladder_tp_sl.take_profit_enabled, isZh ? 'TP 侧开启' : 'TP side on')}
            {statusChip(config.ladder_tp_sl.stop_loss_enabled, isZh ? 'SL 侧开启' : 'SL side on')}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>{ladderStateSummary}</div>
          {ladderModeMismatch && (
            <div className="p-3 rounded-lg text-xs" style={{ background: '#11161C', border: '1px solid #2B3139', color: '#F0B90B' }}>
              {isZh
                ? '注意：当前 Ladder 的“整体模式”是 AI，但“执行开关”仍关闭。页面会保留 AI 模式配置，但运行时不会实际挂 Ladder 保护单，直到你打开执行开关。'
                : 'Note: Ladder global mode is AI, but execution is still disabled. The page preserves the AI setting, but runtime will not place Ladder protection orders until execution is enabled.'}
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用 Ladder' : 'Enable Ladder'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.enabled} onChange={(e) => updateLadder('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-blue-500" />
              </div>
              <select value={config.ladder_tp_sl.mode} onChange={(e) => updateLadder('mode', e.target.value as LadderTPSLConfig['mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {protectionModeOptions.map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
            </div>
            <div className="p-4 rounded-lg space-y-2" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止盈侧' : 'TP Side'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.take_profit_enabled} onChange={(e) => updateLadder('take_profit_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-green-500" />
              </div>
              <select value={config.ladder_tp_sl.take_profit_price.mode} onChange={(e) => updateLadder('take_profit_price', updateValueSource(config.ladder_tp_sl.take_profit_price, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{isZh ? `价格：${modeLabel(mode)}` : `Price: ${modeLabel(mode)}`}</option>)}
              </select>
              <select value={config.ladder_tp_sl.take_profit_size.mode} onChange={(e) => updateLadder('take_profit_size', updateValueSource(config.ladder_tp_sl.take_profit_size, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{isZh ? `仓位：${modeLabel(mode)}` : `Size: ${modeLabel(mode)}`}</option>)}
              </select>
            </div>
            <div className="p-4 rounded-lg space-y-2" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止损侧' : 'SL Side'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.stop_loss_enabled} onChange={(e) => updateLadder('stop_loss_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-red-500" />
              </div>
              <select value={config.ladder_tp_sl.stop_loss_price.mode} onChange={(e) => updateLadder('stop_loss_price', updateValueSource(config.ladder_tp_sl.stop_loss_price, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{isZh ? `价格：${modeLabel(mode)}` : `Price: ${modeLabel(mode)}`}</option>)}
              </select>
              <select value={config.ladder_tp_sl.stop_loss_size.mode} onChange={(e) => updateLadder('stop_loss_size', updateValueSource(config.ladder_tp_sl.stop_loss_size, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {valueModeOptions.map((mode) => <option key={mode} value={mode}>{isZh ? `仓位：${modeLabel(mode)}` : `Size: ${modeLabel(mode)}`}</option>)}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-2" style={{ color: '#EAECEF' }}>{isZh ? 'Ladder 最大损失兜底' : 'Ladder Max Loss Fallback'}</label>
              <select value={config.ladder_tp_sl.fallback_max_loss.mode} onChange={(e) => updateLadder('fallback_max_loss', updateValueSource(config.ladder_tp_sl.fallback_max_loss, { mode: e.target.value as ProtectionMode }))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                {(['disabled', 'manual'] as const).map((mode) => <option key={mode} value={mode}>{modeLabel(mode)}</option>)}
              </select>
              {config.ladder_tp_sl.fallback_max_loss.mode === 'manual' && (
                <input type="number" min={0} step={0.1} value={config.ladder_tp_sl.fallback_max_loss.value} onChange={(e) => updateLadder('fallback_max_loss', updateValueSource(config.ladder_tp_sl.fallback_max_loss, { value: parseFloat(e.target.value) || 0 }))} disabled={disabled} className="w-full mt-2 px-3 py-2 rounded" style={inputStyle} />
              )}
            </div>
          </div>

          {infoBlock(
            isZh ? 'Ladder 参数说明' : 'Ladder Parameter Guide',
            isZh ? '每一档都是一组“触发幅度 + 平仓比例”。只有配置了规则，Ladder 才会生成多档委托；当 Drawdown 接管止盈侧时，仅 Ladder SL 保留。' : 'Each ladder level is a trigger plus close ratio. Ladder generates multi-level orders only when rules exist; if Drawdown owns the TP side, only Ladder SL remains active.',
            isZh ? '建议：先控制总平仓比例，再决定每档分配。' : 'Recommendation: control the total close ratio first, then distribute it across levels.'
          )}

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{isZh ? '分批规则' : 'Ladder Rules'}</div>
              <button type="button" onClick={addLadderRule} disabled={disabled} className="inline-flex items-center gap-1 px-3 py-1.5 rounded text-sm bg-[#1E2329] border border-[#2B3139] text-[#EAECEF] hover:border-[#F0B90B]">
                <Plus className="w-4 h-4" />
                {isZh ? '新增一档' : 'Add Level'}
              </button>
            </div>

            {ladderRules.length === 0 && (
              <div className="p-3 rounded-lg text-xs" style={helpCardStyle}>
                {isZh ? '当前还没有配置任何 Ladder 规则，所以不会真正生成分批止盈/止损委托。请至少新增 1 档规则。' : 'No ladder rules configured yet, so no ladder protection orders will be generated.'}
              </div>
            )}

            {ladderRules.map((rule, index) => (
              <div key={index} className="p-4 rounded-lg space-y-3" style={sectionStyle}>
                <div className="flex items-center justify-between">
                  <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{isZh ? `第 ${index + 1} 档` : `Level ${index + 1}`}</div>
                  <button type="button" onClick={() => removeLadderRule(index)} disabled={disabled} className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-[#F6465D] border border-[#41272B] hover:bg-[#2B1619]">
                    <Trash2 className="w-3.5 h-3.5" />
                    {isZh ? '删除' : 'Remove'}
                  </button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止盈触发 %' : 'TP Trigger %'}</label>
                    {config.ladder_tp_sl.take_profit_price.mode === 'manual' ? (
                      <input type="number" min={0} step={0.1} value={rule.take_profit_pct || 0} onChange={(e) => updateLadderRule(index, { take_profit_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    ) : (
                      <div className="px-3 py-2 rounded text-xs" style={helpCardStyle}>{isZh ? '由 AI 生成或已禁用' : 'Generated by AI or disabled'}</div>
                    )}
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止盈平仓比例 %' : 'TP Close Ratio %'}</label>
                    {config.ladder_tp_sl.take_profit_size.mode === 'manual' ? (
                      <input type="number" min={0} max={100} step={1} value={rule.take_profit_close_ratio_pct || 0} onChange={(e) => updateLadderRule(index, { take_profit_close_ratio_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    ) : (
                      <div className="px-3 py-2 rounded text-xs" style={helpCardStyle}>{isZh ? '由 AI 生成或已禁用' : 'Generated by AI or disabled'}</div>
                    )}
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止损触发 %' : 'SL Trigger %'}</label>
                    {config.ladder_tp_sl.stop_loss_price.mode === 'manual' ? (
                      <input type="number" min={0} step={0.1} value={rule.stop_loss_pct || 0} onChange={(e) => updateLadderRule(index, { stop_loss_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    ) : (
                      <div className="px-3 py-2 rounded text-xs" style={helpCardStyle}>{isZh ? '由 AI 生成或已禁用' : 'Generated by AI or disabled'}</div>
                    )}
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止损平仓比例 %' : 'SL Close Ratio %'}</label>
                    {config.ladder_tp_sl.stop_loss_size.mode === 'manual' ? (
                      <input type="number" min={0} max={100} step={1} value={rule.stop_loss_close_ratio_pct || 0} onChange={(e) => updateLadderRule(index, { stop_loss_close_ratio_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    ) : (
                      <div className="px-3 py-2 rounded text-xs" style={helpCardStyle}>{isZh ? '由 AI 生成或已禁用' : 'Generated by AI or disabled'}</div>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <div className="flex items-center gap-2 mb-4">
            <Activity className="w-5 h-5" style={{ color: '#A855F7' }} />
            <h3 className="font-medium" style={{ color: '#EAECEF' }}>
              {isZh ? 'Drawdown Take Profit（运行态保护）' : 'Drawdown Take Profit (Runtime Protection)'}
            </h3>
          </div>
          <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用回撤止盈' : 'Enable Drawdown TP'}</label>
              <input type="checkbox" checked={config.drawdown_take_profit.enabled} onChange={(e) => updateDrawdown('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-purple-500" />
            </div>
            {infoBlock(
              isZh ? '盈利控制主链' : 'Primary profit-control path',
              isZh ? 'Drawdown / Native Trailing 接管止盈侧。达到最小利润门槛后，系统按回撤阈值动态保护利润；不再同时依赖 Full / Ladder TP。' : 'Drawdown / Native Trailing owns the take-profit side. After the minimum profit gate is reached, the system protects gains using drawdown thresholds instead of relying on Full / Ladder TP at the same time.',
              isZh ? '建议：把它当成主止盈链路，只保留 Full / Ladder 的止损侧。' : 'Recommendation: treat this as the main take-profit path and keep only the stop-loss side from Full / Ladder.'
            )}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{isZh ? '回撤止盈规则' : 'Drawdown Rules'}</div>
                <button type="button" onClick={addDrawdownRule} disabled={disabled} className="inline-flex items-center gap-1 px-3 py-1.5 rounded text-sm bg-[#1E2329] border border-[#2B3139] text-[#EAECEF] hover:border-[#F0B90B]">
                  <Plus className="w-4 h-4" />
                  {isZh ? '新增规则' : 'Add Rule'}
                </button>
              </div>

              {drawdownRules.length === 0 && (
                <div className="p-3 rounded-lg text-xs" style={helpCardStyle}>
                  {isZh ? '当前还没有配置任何回撤止盈规则。请至少新增 1 条规则。' : 'No drawdown rules configured yet. Add at least one rule.'}
                </div>
              )}

              {drawdownRules.map((rule, index) => (
                <div key={index} className="p-4 rounded-lg space-y-3" style={sectionStyle}>
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{isZh ? `规则 ${index + 1}` : `Rule ${index + 1}`}</div>
                    <button type="button" onClick={() => removeDrawdownRule(index)} disabled={disabled || drawdownRules.length <= 1} className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-[#F6465D] border border-[#41272B] hover:bg-[#2B1619]">
                      <Trash2 className="w-3.5 h-3.5" />
                      {isZh ? '删除' : 'Remove'}
                    </button>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '最小利润 %' : 'Min Profit %'}</label>
                      <input type="number" value={rule.min_profit_pct} min={0} step={0.1} onChange={(e) => updateDrawdownRule(index, { min_profit_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    </div>
                    <div>
                      <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '最大回撤 %' : 'Max Drawdown %'}</label>
                      <input type="number" value={rule.max_drawdown_pct} min={0} step={0.1} onChange={(e) => updateDrawdownRule(index, { max_drawdown_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    </div>
                    <div>
                      <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '平仓比例 %' : 'Close Ratio %'}</label>
                      <input type="number" value={rule.close_ratio_pct} min={0} max={100} step={1} onChange={(e) => updateDrawdownRule(index, { close_ratio_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    </div>
                    <div>
                      <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '轮询秒数' : 'Poll Seconds'}</label>
                      <input type="number" value={rule.poll_interval_seconds} min={5} step={5} onChange={(e) => updateDrawdownRule(index, { poll_interval_seconds: parseInt(e.target.value) || 60 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div>
          <div className="flex items-center gap-2 mb-4">
            <RotateCcw className="w-5 h-5" style={{ color: '#F97316' }} />
            <h3 className="font-medium" style={{ color: '#EAECEF' }}>
              {isZh ? 'Break-even Stop（运行态保护）' : 'Break-even Stop (Runtime Protection)'}
            </h3>
          </div>
          <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用保本止损' : 'Enable Break-even Stop'}</label>
              <input type="checkbox" checked={config.break_even_stop.enabled} onChange={(e) => updateBreakEven('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-orange-500" />
            </div>
            {infoBlock(
              isZh ? 'Break-even 独立管理' : 'Break-even is independent',
              isZh ? 'Break-even 只负责把止损抬到保本附近，不接管 Drawdown 的盈利控制，也不替代 Full / Ladder 的长期止损结构。' : 'Break-even only raises stop-loss toward breakeven. It does not take over Drawdown profit control or replace the long-lived stop-loss structure from Full / Ladder.',
              isZh ? '建议：把它当成盈利后附加的一层止损保护。' : 'Recommendation: use it as an extra stop-loss layer after profit appears.'
            )}
            <div>
              <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '触发模式' : 'Trigger Mode'}</label>
              <select value={config.break_even_stop.trigger_mode} onChange={(e) => updateBreakEven('trigger_mode', e.target.value as BreakEvenStopConfig['trigger_mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                <option value="profit_pct">{triggerModeLabel('profit_pct')}</option>
                <option value="r_multiple">{triggerModeLabel('r_multiple')}</option>
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '触发值' : 'Trigger Value'}</label>
                <input type="number" value={config.break_even_stop.trigger_value} min={0} step={0.1} onChange={(e) => updateBreakEven('trigger_value', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '偏移 %' : 'Offset %'}</label>
                <input type="number" value={config.break_even_stop.offset_pct} min={0} step={0.1} onChange={(e) => updateBreakEven('offset_pct', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <Filter className="w-5 h-5" style={{ color: '#38BDF8' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? 'Regime Filter（开仓前门禁）' : 'Regime Filter (Pre-entry Gate)'}
          </h3>
        </div>
        <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用 Regime Filter' : 'Enable Regime Filter'}</label>
            <input type="checkbox" checked={config.regime_filter.enabled} onChange={(e) => updateRegimeFilter('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
          </div>
          {infoBlock(
            isZh ? 'Regime Filter 不会挂交易所委托' : 'Regime Filter does not create exchange orders',
            isZh ? '它决定“这笔交易能不能开”。只有市场状态、资金费率、波动、趋势方向满足条件时，系统才允许开仓。' : 'It decides whether a new trade is allowed before entry based on market regime and risk conditions.',
            isZh ? '建议：把它当成开仓门禁，而不是持仓保护。' : 'Recommendation: treat it as a pre-entry gate instead of a position protection tool.'
          )}

          <div>
            <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>{isZh ? '允许的市场状态' : 'Allowed Regimes'}</label>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {regimeOptions.map((option) => {
                const active = (config.regime_filter.allowed_regimes || []).includes(option.value)
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => toggleAllowedRegime(option.value)}
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

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '资金费率绝对值上限' : 'Max Funding Abs'}</label>
              <input type="number" value={config.regime_filter.max_funding_rate_abs} min={0} step={0.001} onChange={(e) => updateRegimeFilter('max_funding_rate_abs', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
            </div>
            <div>
              <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? 'ATR14 波动率上限 %' : 'Max ATR14 %'}</label>
              <input type="number" value={config.regime_filter.max_atr14_pct} min={0} step={0.1} onChange={(e) => updateRegimeFilter('max_atr14_pct', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
            </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}><input type="checkbox" checked={config.regime_filter.block_high_funding} onChange={(e) => updateRegimeFilter('block_high_funding', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />{isZh ? '屏蔽高资金费率' : 'Block high funding'}</label>
            <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}><input type="checkbox" checked={config.regime_filter.block_high_volatility} onChange={(e) => updateRegimeFilter('block_high_volatility', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />{isZh ? '屏蔽高波动' : 'Block high volatility'}</label>
            <label className="flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}><input type="checkbox" checked={config.regime_filter.require_trend_alignment} onChange={(e) => updateRegimeFilter('require_trend_alignment', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />{isZh ? '要求趋势同向' : 'Require trend alignment'}</label>
          </div>
        </div>
      </div>

      <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
        <div className="flex items-center gap-2 mb-2">
          <TrendingDown className="w-4 h-4" style={{ color: '#F6465D' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? '当前执行说明' : 'Execution Notes'}
          </span>
        </div>
        <ul className="text-xs space-y-1 list-disc pl-4" style={{ color: '#848E9C' }}>
          <li>{isZh ? 'Full TP/SL 与 Ladder TP/SL 属于“委托型保护”，目标是在开仓后尽快挂到交易所并做校验。' : 'Full TP/SL and Ladder TP/SL are order-based protections that should be posted and verified after opening.'}</li>
          <li>{isZh ? 'Drawdown / Break-even 属于“运行态保护”，由系统在持仓期间持续监控并动态执行。' : 'Drawdown and Break-even are runtime protections enforced continuously while a position is live.'}</li>
          <li>{isZh ? 'Regime Filter 属于“开仓前门禁”，不会直接生成委托。' : 'Regime Filter is a pre-entry gate and does not generate exchange orders by itself.'}</li>
          <li>{isZh ? '若交易所能力不满足要求或保护校验失败，系统应进入 fail-safe 处理，避免长期裸仓。' : 'If exchange capability is insufficient or verification fails, the system should enter fail-safe handling to avoid naked exposure.'}</li>
        </ul>
      </div>
    </div>
  )
}
