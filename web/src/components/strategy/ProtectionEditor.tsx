import { ShieldCheck, TrendingUp, TrendingDown, Layers, Activity, RotateCcw, Filter } from 'lucide-react'
import type {
  ProtectionConfig,
  FullTPSLConfig,
  LadderTPSLConfig,
  DrawdownTakeProfitConfig,
  BreakEvenStopConfig,
  RegimeFilterConfig,
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
    take_profit: { enabled: false, price_move_pct: 0 },
    stop_loss: { enabled: false, price_move_pct: 0 },
  },
  ladder_tp_sl: {
    enabled: false,
    mode: 'manual',
    take_profit_enabled: false,
    stop_loss_enabled: false,
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

  const firstDrawdownRule = config.drawdown_take_profit.rules?.[0] || defaultProtectionConfig.drawdown_take_profit.rules[0]

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
                ? '当前已完成 Full TP/SL、Ladder、Drawdown、Break-even 执行闭环，并继续补上 AI protection mode + Regime Filter。'
                : 'Full TP/SL, Ladder, Drawdown, and Break-even are already executable; this panel now continues into AI protection mode + Regime Filter.'}
            </p>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? 'Full TP/SL（首轮可执行）' : 'Full TP/SL (Executable in Phase 1)'}
          </h3>
        </div>

        <div className="space-y-4">
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <div>
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {isZh ? '启用 Full TP/SL' : 'Enable Full TP/SL'}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {isZh ? '开仓后为整仓挂统一止盈/止损保护单。' : 'Attach unified full-position TP/SL protection orders after opening.'}
                </p>
              </div>
              <input
                type="checkbox"
                checked={config.full_tp_sl.enabled}
                onChange={(e) => updateFull('enabled', e.target.checked)}
                disabled={disabled}
                className="h-4 w-4 accent-yellow-500"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {isZh ? '模式' : 'Mode'}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {isZh ? '当前后端已优先支持 manual。' : 'Backend currently prioritizes manual mode.'}
              </p>
              <select
                value={config.full_tp_sl.mode}
                onChange={(e) => updateFull('mode', e.target.value as FullTPSLConfig['mode'])}
                disabled={disabled}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              >
                <option value="manual">manual</option>
                <option value="ai">ai</option>
              </select>
            </div>

            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {isZh ? '止盈 Take Profit' : 'Take Profit'}
                </label>
                <input
                  type="checkbox"
                  checked={config.full_tp_sl.take_profit.enabled}
                  onChange={(e) => updateFull('take_profit', { ...config.full_tp_sl.take_profit, enabled: e.target.checked })}
                  disabled={disabled}
                  className="h-4 w-4 accent-green-500"
                />
              </div>
              <input
                type="number"
                min={0}
                step={0.1}
                value={config.full_tp_sl.take_profit.price_move_pct}
                onChange={(e) => updateFull('take_profit', { ...config.full_tp_sl.take_profit, price_move_pct: parseFloat(e.target.value) || 0 })}
                disabled={disabled}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
              <p className="text-xs mt-2" style={{ color: '#848E9C' }}>
                {isZh ? '相对开仓价涨跌幅百分比。Long 为上涨止盈，Short 为下跌止盈。' : 'Percentage move from entry price. Long takes profit upward; short takes profit downward.'}
              </p>
            </div>

            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {isZh ? '止损 Stop Loss' : 'Stop Loss'}
                </label>
                <input
                  type="checkbox"
                  checked={config.full_tp_sl.stop_loss.enabled}
                  onChange={(e) => updateFull('stop_loss', { ...config.full_tp_sl.stop_loss, enabled: e.target.checked })}
                  disabled={disabled}
                  className="h-4 w-4 accent-red-500"
                />
              </div>
              <input
                type="number"
                min={0}
                step={0.1}
                value={config.full_tp_sl.stop_loss.price_move_pct}
                onChange={(e) => updateFull('stop_loss', { ...config.full_tp_sl.stop_loss, price_move_pct: parseFloat(e.target.value) || 0 })}
                disabled={disabled}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
              <p className="text-xs mt-2" style={{ color: '#848E9C' }}>
                {isZh ? '相对开仓价涨跌幅百分比。Long 为下跌止损，Short 为上涨止损。' : 'Percentage move from entry price. Long stops on price drop; short stops on price rise.'}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center gap-2 mb-4">
          <Layers className="w-5 h-5" style={{ color: '#60A5FA' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {isZh ? 'Ladder TP/SL（已可执行）' : 'Ladder TP/SL (Executable)'}
          </h3>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用 Ladder' : 'Enable Ladder'}</label>
              <input type="checkbox" checked={config.ladder_tp_sl.enabled} onChange={(e) => updateLadder('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-blue-500" />
            </div>
            <select value={config.ladder_tp_sl.mode} onChange={(e) => updateLadder('mode', e.target.value as LadderTPSLConfig['mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
              <option value="manual">manual</option>
              <option value="ai">ai</option>
            </select>
          </div>
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用分批止盈' : 'Enable Ladder TP'}</label>
              <input type="checkbox" checked={config.ladder_tp_sl.take_profit_enabled} onChange={(e) => updateLadder('take_profit_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-green-500" />
            </div>
            <p className="text-xs" style={{ color: '#848E9C' }}>{isZh ? 'manual 已可执行，ai 模式在当前阶段作为 Phase 3 最小闭环接入。' : 'Manual mode is already executable; AI mode is now wired as the minimal Phase 3 closure.'}</p>
          </div>
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用分批止损' : 'Enable Ladder SL'}</label>
              <input type="checkbox" checked={config.ladder_tp_sl.stop_loss_enabled} onChange={(e) => updateLadder('stop_loss_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-red-500" />
            </div>
            <p className="text-xs" style={{ color: '#848E9C' }}>{isZh ? '规则数组保留；执行链已落地，后续可再补更强的多档编辑体验。' : 'Rule arrays are preserved; execution is already wired, and richer multi-rule editing can be added later.'}</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <div className="flex items-center gap-2 mb-4">
            <Activity className="w-5 h-5" style={{ color: '#A855F7' }} />
            <h3 className="font-medium" style={{ color: '#EAECEF' }}>
              {isZh ? 'Drawdown Take Profit（已可执行）' : 'Drawdown Take Profit (Executable)'}
            </h3>
          </div>
          <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用回撤止盈' : 'Enable Drawdown TP'}</label>
              <input type="checkbox" checked={config.drawdown_take_profit.enabled} onChange={(e) => updateDrawdown('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-purple-500" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '最小利润 %' : 'Min Profit %'}</label>
                <input type="number" value={firstDrawdownRule.min_profit_pct} min={0} step={0.1} onChange={(e) => updateDrawdown('rules', [{ ...firstDrawdownRule, min_profit_pct: parseFloat(e.target.value) || 0 }])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '最大回撤 %' : 'Max Drawdown %'}</label>
                <input type="number" value={firstDrawdownRule.max_drawdown_pct} min={0} step={0.1} onChange={(e) => updateDrawdown('rules', [{ ...firstDrawdownRule, max_drawdown_pct: parseFloat(e.target.value) || 0 }])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '平仓比例 %' : 'Close Ratio %'}</label>
                <input type="number" value={firstDrawdownRule.close_ratio_pct} min={0} max={100} step={1} onChange={(e) => updateDrawdown('rules', [{ ...firstDrawdownRule, close_ratio_pct: parseFloat(e.target.value) || 0 }])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '轮询秒数' : 'Poll Seconds'}</label>
                <input type="number" value={firstDrawdownRule.poll_interval_seconds} min={5} step={5} onChange={(e) => updateDrawdown('rules', [{ ...firstDrawdownRule, poll_interval_seconds: parseInt(e.target.value) || 60 }])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
            </div>
          </div>
        </div>

        <div>
          <div className="flex items-center gap-2 mb-4">
            <RotateCcw className="w-5 h-5" style={{ color: '#F97316' }} />
            <h3 className="font-medium" style={{ color: '#EAECEF' }}>
              {isZh ? 'Break-even Stop（已可执行）' : 'Break-even Stop (Executable)'}
            </h3>
          </div>
          <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用保本止损' : 'Enable Break-even Stop'}</label>
              <input type="checkbox" checked={config.break_even_stop.enabled} onChange={(e) => updateBreakEven('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-orange-500" />
            </div>
            <div>
              <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '触发模式' : 'Trigger Mode'}</label>
              <select value={config.break_even_stop.trigger_mode} onChange={(e) => updateBreakEven('trigger_mode', e.target.value as BreakEvenStopConfig['trigger_mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                <option value="profit_pct">profit_pct</option>
                <option value="r_multiple">r_multiple</option>
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
            {isZh ? 'Regime Filter（Phase 3）' : 'Regime Filter (Phase 3)'}
          </h3>
        </div>
        <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用 Regime Filter' : 'Enable Regime Filter'}</label>
            <input type="checkbox" checked={config.regime_filter.enabled} onChange={(e) => updateRegimeFilter('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-sky-500" />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '允许的 regime' : 'Allowed regimes'}</label>
              <input type="text" value={config.regime_filter.allowed_regimes.join(',')} onChange={(e) => updateRegimeFilter('allowed_regimes', e.target.value.split(',').map(v => v.trim()).filter(Boolean))} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} placeholder="narrow,standard,wide" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '资金费率上限(abs)' : 'Max funding abs'}</label>
                <input type="number" value={config.regime_filter.max_funding_rate_abs} min={0} step={0.001} onChange={(e) => updateRegimeFilter('max_funding_rate_abs', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? 'ATR14 上限 %' : 'Max ATR14 %'}</label>
                <input type="number" value={config.regime_filter.max_atr14_pct} min={0} step={0.1} onChange={(e) => updateRegimeFilter('max_atr14_pct', parseFloat(e.target.value) || 0)} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              </div>
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
            {isZh ? '当前状态说明' : 'Current Status'}
          </span>
        </div>
        <ul className="text-xs space-y-1 list-disc pl-4" style={{ color: '#848E9C' }}>
          <li>{isZh ? '后端已接入 manual / ai protection 的最小闭环，开仓后会尝试按计划挂保护单并做校验。' : 'Backend now supports a minimal manual / AI protection closure that applies and verifies protection after opening.'}</li>
          <li>{isZh ? '若校验失败或交易所能力不满足安全条件，会立即平仓，避免裸仓残留。' : 'If verification fails or exchange capability is unsafe, the system closes the position immediately to avoid unprotected exposure.'}</li>
          <li>{isZh ? 'Regime Filter 已作为开仓前门禁接入；更复杂的多规则 UI 和仿真验证仍可继续补强。' : 'Regime Filter is now wired as a pre-entry gate; richer multi-rule UI and simulation validation can still be strengthened later.'}</li>
        </ul>
      </div>
    </div>
  )
}
