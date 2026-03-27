import { ShieldCheck, TrendingUp, TrendingDown, Layers, Activity, RotateCcw, Filter, Plus, Trash2 } from 'lucide-react'
import type {
  ProtectionConfig,
  FullTPSLConfig,
  LadderTPSLConfig,
  DrawdownTakeProfitConfig,
  BreakEvenStopConfig,
  RegimeFilterConfig,
  LadderTPSLRule,
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

  const firstDrawdownRule = config.drawdown_take_profit.rules?.[0] || defaultProtectionConfig.drawdown_take_profit.rules[0]
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

  const modeLabel = (mode: 'manual' | 'ai') => mode === 'ai'
    ? (isZh ? 'AI 动态保护模式' : 'AI Dynamic Protection')
    : (isZh ? '手动阈值模式' : 'Manual Threshold Mode')

  const triggerModeLabel = (mode: 'profit_pct' | 'r_multiple') => mode === 'r_multiple'
    ? (isZh ? '按 R 倍数触发' : 'Trigger by R Multiple')
    : (isZh ? '按盈利百分比触发' : 'Trigger by Profit %')

  const infoBlock = (title: string, description: string, example: string, recommend: string) => (
    <div className="p-3 rounded-lg space-y-1" style={helpCardStyle}>
      <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>{title}</div>
      <div className="text-xs" style={{ color: '#AAB2BD' }}>{description}</div>
      <div className="text-xs" style={{ color: '#848E9C' }}>{example}</div>
      <div className="text-xs" style={{ color: '#F0B90B' }}>{recommend}</div>
    </div>
  )

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
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <div>
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {isZh ? '启用 Full TP/SL' : 'Enable Full TP/SL'}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {isZh ? '开仓后为整仓挂统一止盈/止损保护单。通常你在交易所里会看到两张保护委托。' : 'Attach one unified TP and one unified SL order after opening.'}
                </p>
              </div>
              <input type="checkbox" checked={config.full_tp_sl.enabled} onChange={(e) => updateFull('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-yellow-500" />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>{isZh ? '模式' : 'Mode'}</label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {isZh ? '手动模式按你填写的阈值执行；AI 模式由 AI 决定保护计划。' : 'Manual mode uses your thresholds; AI mode expects AI-generated protection plans.'}
              </p>
              <select value={config.full_tp_sl.mode} onChange={(e) => updateFull('mode', e.target.value as FullTPSLConfig['mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                <option value="manual">{modeLabel('manual')}</option>
                <option value="ai">{modeLabel('ai')}</option>
              </select>
            </div>

            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止盈' : 'Take Profit'}</label>
                <input type="checkbox" checked={config.full_tp_sl.take_profit.enabled} onChange={(e) => updateFull('take_profit', { ...config.full_tp_sl.take_profit, enabled: e.target.checked })} disabled={disabled} className="h-4 w-4 accent-green-500" />
              </div>
              <input type="number" min={0} step={0.1} value={config.full_tp_sl.take_profit.price_move_pct} onChange={(e) => updateFull('take_profit', { ...config.full_tp_sl.take_profit, price_move_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              <p className="text-xs mt-2" style={{ color: '#848E9C' }}>
                {isZh ? '数值表示相对开仓价的目标涨跌幅百分比。示例：填 5 表示盈利达到 5% 触发止盈。新手建议 3%~8%。' : 'Percentage move from entry price. Example: 5 means trigger TP at +5% favorable move.'}
              </p>
            </div>

            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '止损' : 'Stop Loss'}</label>
                <input type="checkbox" checked={config.full_tp_sl.stop_loss.enabled} onChange={(e) => updateFull('stop_loss', { ...config.full_tp_sl.stop_loss, enabled: e.target.checked })} disabled={disabled} className="h-4 w-4 accent-red-500" />
              </div>
              <input type="number" min={0} step={0.1} value={config.full_tp_sl.stop_loss.price_move_pct} onChange={(e) => updateFull('stop_loss', { ...config.full_tp_sl.stop_loss, price_move_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
              <p className="text-xs mt-2" style={{ color: '#848E9C' }}>
                {isZh ? '数值表示相对开仓价的容忍回撤百分比。示例：填 2 表示亏损达到 2% 触发止损。新手建议 1%~3%。' : 'Allowed adverse move from entry. Example: 2 means stop at -2% adverse move.'}
              </p>
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
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用 Ladder' : 'Enable Ladder'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.enabled} onChange={(e) => updateLadder('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-blue-500" />
              </div>
              <select value={config.ladder_tp_sl.mode} onChange={(e) => updateLadder('mode', e.target.value as LadderTPSLConfig['mode'])} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle}>
                <option value="manual">{modeLabel('manual')}</option>
                <option value="ai">{modeLabel('ai')}</option>
              </select>
            </div>
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用分批止盈' : 'Enable Ladder TP'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.take_profit_enabled} onChange={(e) => updateLadder('take_profit_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-green-500" />
              </div>
              <p className="text-xs" style={{ color: '#848E9C' }}>{isZh ? '示例：第 1 档盈利 3% 平 30%，第 2 档盈利 5% 再平 30%。' : 'Example: close 30% at +3%, another 30% at +5%.'}</p>
            </div>
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用分批止损' : 'Enable Ladder SL'}</label>
                <input type="checkbox" checked={config.ladder_tp_sl.stop_loss_enabled} onChange={(e) => updateLadder('stop_loss_enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-red-500" />
              </div>
              <p className="text-xs" style={{ color: '#848E9C' }}>{isZh ? '示例：价格逆向 2% 平 50%，逆向 4% 再平剩余。' : 'Example: close 50% at -2% adverse move, rest at -4%.'}</p>
            </div>
          </div>

          {infoBlock(
            isZh ? 'Ladder 参数说明' : 'Ladder Parameter Guide',
            isZh ? '每一档都是一组“触发幅度 + 平仓比例”。只有配置了规则，Ladder 才会真正生成多档委托。' : 'Each ladder level is a trigger percentage plus close ratio. Ladder only works when rules are configured.',
            isZh ? '示例：止盈 3% 平 30%，止盈 5% 平 30%，止盈 8% 平 40%。' : 'Example: TP 3% close 30%, TP 5% close 30%, TP 8% close 40%.',
            isZh ? '建议：先从 2~3 档开始，总平仓比例不要超过 100%。' : 'Recommendation: start with 2-3 levels and keep total ratio within 100%.'
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
                    <input type="number" min={0} step={0.1} value={rule.take_profit_pct || 0} onChange={(e) => updateLadderRule(index, { take_profit_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止盈平仓比例 %' : 'TP Close Ratio %'}</label>
                    <input type="number" min={0} max={100} step={1} value={rule.take_profit_close_ratio_pct || 0} onChange={(e) => updateLadderRule(index, { take_profit_close_ratio_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止损触发 %' : 'SL Trigger %'}</label>
                    <input type="number" min={0} step={0.1} value={rule.stop_loss_pct || 0} onChange={(e) => updateLadderRule(index, { stop_loss_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
                  </div>
                  <div>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>{isZh ? '止损平仓比例 %' : 'SL Close Ratio %'}</label>
                    <input type="number" min={0} max={100} step={1} value={rule.stop_loss_close_ratio_pct || 0} onChange={(e) => updateLadderRule(index, { stop_loss_close_ratio_pct: parseFloat(e.target.value) || 0 })} disabled={disabled} className="w-full px-3 py-2 rounded" style={inputStyle} />
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
              isZh ? '这不是固定挂单，而是系统运行态监控' : 'This is runtime-monitored, not a fixed exchange order',
              isZh ? '系统会持续跟踪持仓浮盈峰值。当利润先达到门槛、随后回撤到设定比例时，再触发平仓。' : 'The system tracks peak unrealized profit, then closes when drawdown from peak reaches threshold.',
              isZh ? '示例：先盈利到 10%，后来回落到 6%，说明从峰值回撤了 40%，可触发止盈。' : 'Example: profit peaks at 10% then falls to 6%, which is a 40% drawdown from peak.',
              isZh ? '建议：最小利润 5%，最大回撤 30%~40%，平仓比例 50% 或 100%。' : 'Recommendation: min profit 5%, max drawdown 30%-40%, close ratio 50% or 100%.'
            )}
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
              {isZh ? 'Break-even Stop（运行态保护）' : 'Break-even Stop (Runtime Protection)'}
            </h3>
          </div>
          <div className="p-4 rounded-lg space-y-3" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <label className="block text-sm" style={{ color: '#EAECEF' }}>{isZh ? '启用保本止损' : 'Enable Break-even Stop'}</label>
              <input type="checkbox" checked={config.break_even_stop.enabled} onChange={(e) => updateBreakEven('enabled', e.target.checked)} disabled={disabled} className="h-4 w-4 accent-orange-500" />
            </div>
            {infoBlock(
              isZh ? '保本止损会在盈利后上移止损' : 'Moves stop-loss upward after profit appears',
              isZh ? '达到触发门槛后，系统会把止损上移到开仓价附近，避免盈利单最后变亏损。' : 'After the trigger threshold is met, the system raises stop-loss near breakeven.',
              isZh ? '示例：盈利达到 3% 后，把止损抬到开仓价上方 0.2%。' : 'Example: once profit reaches 3%, move stop to entry +0.2%.',
              isZh ? '建议：触发值 2%~3%，偏移 0.1%~0.3%。' : 'Recommendation: trigger 2%-3%, offset 0.1%-0.3%.'
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
            isZh ? '示例：只允许标准波动 / 宽波动；资金费率过高时不准进场。' : 'Example: allow only standard/wide regime and block entries when funding is too high.',
            isZh ? '建议：新手开启高资金费率屏蔽、高波动屏蔽、趋势同向。' : 'Recommendation: enable high-funding block, high-volatility block, and trend alignment for safer setups.'
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
