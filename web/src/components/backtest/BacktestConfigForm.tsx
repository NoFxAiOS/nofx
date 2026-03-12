import { useMemo, type FormEvent } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  ChevronRight,
  ChevronLeft,
  RefreshCw,
  Zap,
} from 'lucide-react'
import type { AIModel, Strategy } from '../../types'

// ============ Types ============

type WizardStep = 1 | 2 | 3

export interface BacktestFormState {
  runId: string
  symbols: string
  timeframes: string[]
  decisionTf: string
  cadence: number
  start: string
  end: string
  balance: number
  fee: number
  slippage: number
  btcEthLeverage: number
  altcoinLeverage: number
  fill: string
  prompt: string
  promptTemplate: string
  customPrompt: string
  overridePrompt: boolean
  cacheAI: boolean
  replayOnly: boolean
  aiModelId: string
  strategyId: string
}

const TIMEFRAME_OPTIONS = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d']
const POPULAR_SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT']

// ============ Config Form ============

interface BacktestConfigFormProps {
  formState: BacktestFormState
  wizardStep: WizardStep
  isStarting: boolean
  aiModels: AIModel[] | undefined
  strategies: Strategy[] | undefined
  language: string
  tr: (key: string, params?: Record<string, string | number>) => string
  onFormChange: (key: string, value: string | number | boolean | string[]) => void
  onWizardStepChange: (step: WizardStep) => void
  onStart: (event: FormEvent) => void
}

export function BacktestConfigForm({
  formState,
  wizardStep,
  isStarting,
  aiModels,
  strategies,
  language,
  tr,
  onFormChange,
  onWizardStepChange,
  onStart,
}: BacktestConfigFormProps) {
  const selectedModel = aiModels?.find((m) => m.id === formState.aiModelId)
  const selectedStrategy = strategies?.find((s) => s.id === formState.strategyId)

  const strategyHasDynamicCoins = useMemo(() => {
    const cs = selectedStrategy?.config?.coin_source
    if (!cs) return false
    const st = cs.source_type as string
    if (st === 'ai500' || st === 'oi_top') return true
    if (st === 'mixed' && (cs.use_ai500 || cs.use_oi_top)) return true
    if (!st && (cs.use_ai500 || cs.use_oi_top)) return true
    return false
  }, [selectedStrategy])

  const coinSourceDescription = useMemo(() => {
    const cs = selectedStrategy?.config?.coin_source
    if (!cs) return null
    let st = cs.source_type as string
    if (!st) {
      if (cs.use_ai500 && cs.use_oi_top) st = 'mixed'
      else if (cs.use_ai500) st = 'ai500'
      else if (cs.use_oi_top) st = 'oi_top'
      else if (cs.static_coins?.length) st = 'static'
    }
    switch (st) {
      case 'ai500': return { type: 'AI500', limit: cs.ai500_limit || 30 }
      case 'oi_top': return { type: 'OI Top', limit: cs.oi_top_limit || 30 }
      case 'mixed': {
        const parts: string[] = []
        if (cs.use_ai500) parts.push(`AI500(${cs.ai500_limit || 30})`)
        if (cs.use_oi_top) parts.push(`OI Top(${cs.oi_top_limit || 30})`)
        if (cs.static_coins?.length) parts.push(`Static(${cs.static_coins.length})`)
        return { type: 'Mixed', desc: parts.join(' + ') }
      }
      case 'static': return { type: 'Static', coins: cs.static_coins || [] }
      default: return null
    }
  }, [selectedStrategy])

  const zh = language === 'zh'
  const quickRanges = [
    { label: zh ? '24小时' : '24h', hours: 24 },
    { label: zh ? '3天' : '3d', hours: 72 },
    { label: zh ? '7天' : '7d', hours: 168 },
    { label: zh ? '30天' : '30d', hours: 720 },
  ]

  const applyQuickRange = (hours: number) => {
    const end = new Date()
    const start = new Date(end.getTime() - hours * 3600 * 1000)
    const fmt = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
    onFormChange('start', fmt(start))
    onFormChange('end', fmt(end))
  }

  return (
    <div className="binance-card p-5">
      <div className="flex items-center gap-2 mb-4">
        {[1, 2, 3].map((step) => (
          <div key={step} className="flex items-center">
            <button
              onClick={() => onWizardStepChange(step as WizardStep)}
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: wizardStep >= step ? '#F0B90B' : '#2B3139',
                color: wizardStep >= step ? '#0B0E11' : '#848E9C',
              }}
            >
              {step}
            </button>
            {step < 3 && (
              <div
                className="w-8 h-0.5 mx-1"
                style={{ background: wizardStep > step ? '#F0B90B' : '#2B3139' }}
              />
            )}
          </div>
        ))}
        <span className="ml-2 text-xs" style={{ color: '#848E9C' }}>
          {wizardStep === 1 ? (zh ? '选择模型' : 'Select Model')
            : wizardStep === 2 ? (zh ? '配置参数' : 'Configure')
            : (zh ? '确认启动' : 'Confirm')}
        </span>
      </div>

      <form onSubmit={onStart}>
        <AnimatePresence mode="wait">
          {/* Step 1: Model & Symbols */}
          {wizardStep === 1 && (
            <motion.div
              key="step1"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                  {tr('form.aiModelLabel')}
                </label>
                <select
                  className="w-full p-3 rounded-lg text-sm"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  value={formState.aiModelId}
                  onChange={(e) => onFormChange('aiModelId', e.target.value)}
                >
                  <option value="">{tr('form.selectAiModel')}</option>
                  {aiModels?.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.name} ({m.provider}) {!m.enabled && '⚠️'}
                    </option>
                  ))}
                </select>
                {selectedModel && (
                  <div className="mt-2 flex items-center gap-2 text-xs">
                    <span
                      className="px-2 py-0.5 rounded"
                      style={{
                        background: selectedModel.enabled ? 'rgba(14,203,129,0.1)' : 'rgba(246,70,93,0.1)',
                        color: selectedModel.enabled ? '#0ECB81' : '#F6465D',
                      }}
                    >
                      {selectedModel.enabled ? tr('form.enabled') : tr('form.disabled')}
                    </span>
                  </div>
                )}
              </div>

              {/* Strategy Selection (Optional) */}
              <div>
                <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                  {zh ? '策略配置（可选）' : 'Strategy (Optional)'}
                </label>
                <select
                  className="w-full p-3 rounded-lg text-sm"
                  style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                  value={formState.strategyId}
                  onChange={(e) => onFormChange('strategyId', e.target.value)}
                >
                  <option value="">{zh ? '不使用保存的策略' : 'No saved strategy'}</option>
                  {strategies?.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} {s.is_active && '✓'} {s.is_default && '⭐'}
                    </option>
                  ))}
                </select>
                {formState.strategyId && coinSourceDescription && (
                  <div className="mt-2 p-2 rounded" style={{ background: 'rgba(240,185,11,0.1)', border: '1px solid rgba(240,185,11,0.2)' }}>
                    <div className="flex items-center gap-2 text-xs">
                      <span style={{ color: '#F0B90B' }}>
                        {zh ? '币种来源:' : 'Coin Source:'}
                      </span>
                      <span className="font-medium" style={{ color: '#EAECEF' }}>
                        {coinSourceDescription.type}
                        {coinSourceDescription.limit && ` (${coinSourceDescription.limit})`}
                        {coinSourceDescription.desc && ` - ${coinSourceDescription.desc}`}
                      </span>
                    </div>
                    {strategyHasDynamicCoins && (
                      <div className="text-xs mt-1" style={{ color: '#F0B90B' }}>
                        {zh
                          ? '⚡ 清空下方币种输入框即可使用策略的动态币种'
                          : '⚡ Clear the symbols field below to use strategy\'s dynamic coins'}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div>
                <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                  {tr('form.symbolsLabel')}
                  {strategyHasDynamicCoins && (
                    <span className="ml-2" style={{ color: '#5E6673' }}>
                      ({zh ? '可选 - 策略已配置币种来源' : 'Optional - strategy has coin source'})
                    </span>
                  )}
                </label>
                {!strategyHasDynamicCoins && (
                  <div className="flex flex-wrap gap-1 mb-2">
                    {POPULAR_SYMBOLS.map((sym) => {
                      const isSelected = formState.symbols.includes(sym)
                      return (
                        <button
                          key={sym}
                          type="button"
                          onClick={() => {
                            const current = formState.symbols.split(',').map((s) => s.trim()).filter(Boolean)
                            const updated = isSelected
                              ? current.filter((s) => s !== sym)
                              : [...current, sym]
                            onFormChange('symbols', updated.join(','))
                          }}
                          className="px-2 py-1 rounded text-xs transition-all"
                          style={{
                            background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                            border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                            color: isSelected ? '#F0B90B' : '#848E9C',
                          }}
                        >
                          {sym.replace('USDT', '')}
                        </button>
                      )
                    })}
                  </div>
                )}
                <div className="relative">
                  <textarea
                    className="w-full p-2 rounded-lg text-xs font-mono"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                    value={formState.symbols}
                    onChange={(e) => onFormChange('symbols', e.target.value)}
                    rows={2}
                    placeholder={strategyHasDynamicCoins
                      ? (zh ? '留空将使用策略配置的币种来源' : 'Leave empty to use strategy coin source')
                      : ''
                    }
                  />
                  {strategyHasDynamicCoins && formState.symbols && (
                    <button
                      type="button"
                      onClick={() => onFormChange('symbols', '')}
                      className="absolute top-2 right-2 px-2 py-1 rounded text-xs"
                      style={{ background: '#F0B90B', color: '#0B0E11' }}
                    >
                      {zh ? '清空使用策略币种' : 'Clear to use strategy'}
                    </button>
                  )}
                </div>
              </div>

              <button
                type="button"
                onClick={() => onWizardStepChange(2)}
                disabled={!selectedModel?.enabled}
                className="w-full py-2.5 rounded-lg font-medium flex items-center justify-center gap-2 transition-all disabled:opacity-50"
                style={{ background: '#F0B90B', color: '#0B0E11' }}
              >
                {zh ? '下一步' : 'Next'}
                <ChevronRight className="w-4 h-4" />
              </button>
            </motion.div>
          )}

          {/* Step 2: Parameters */}
          {wizardStep === 2 && (
            <motion.div
              key="step2"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                  {tr('form.timeRangeLabel')}
                </label>
                <div className="flex flex-wrap gap-1 mb-2">
                  {quickRanges.map((r) => (
                    <button
                      key={r.hours}
                      type="button"
                      onClick={() => applyQuickRange(r.hours)}
                      className="px-3 py-1 rounded text-xs"
                      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                    >
                      {r.label}
                    </button>
                  ))}
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <input
                    type="datetime-local"
                    className="p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.start}
                    onChange={(e) => onFormChange('start', e.target.value)}
                  />
                  <input
                    type="datetime-local"
                    className="p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.end}
                    onChange={(e) => onFormChange('end', e.target.value)}
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs mb-2" style={{ color: '#848E9C' }}>
                  {zh ? '时间周期' : 'Timeframes'}
                </label>
                <div className="flex flex-wrap gap-1">
                  {TIMEFRAME_OPTIONS.map((tf) => {
                    const isSelected = formState.timeframes.includes(tf)
                    return (
                      <button
                        key={tf}
                        type="button"
                        onClick={() => {
                          const updated = isSelected
                            ? formState.timeframes.filter((t) => t !== tf)
                            : [...formState.timeframes, tf]
                          if (updated.length > 0) onFormChange('timeframes', updated)
                        }}
                        className="px-2 py-1 rounded text-xs transition-all"
                        style={{
                          background: isSelected ? 'rgba(240,185,11,0.15)' : '#1E2329',
                          border: `1px solid ${isSelected ? '#F0B90B' : '#2B3139'}`,
                          color: isSelected ? '#F0B90B' : '#848E9C',
                        }}
                      >
                        {tf}
                      </button>
                    )
                  })}
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.initialBalanceLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.balance}
                    onChange={(e) => onFormChange('balance', Number(e.target.value))}
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.decisionTfLabel')}
                  </label>
                  <select
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.decisionTf}
                    onChange={(e) => onFormChange('decisionTf', e.target.value)}
                  >
                    {formState.timeframes.map((tf) => (
                      <option key={tf} value={tf}>
                        {tf}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => onWizardStepChange(1)}
                  className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                >
                  <ChevronLeft className="w-4 h-4" />
                  {zh ? '上一步' : 'Back'}
                </button>
                <button
                  type="button"
                  onClick={() => onWizardStepChange(3)}
                  className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                  style={{ background: '#F0B90B', color: '#0B0E11' }}
                >
                  {zh ? '下一步' : 'Next'}
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            </motion.div>
          )}

          {/* Step 3: Advanced & Confirm */}
          {wizardStep === 3 && (
            <motion.div
              key="step3"
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              className="space-y-4"
            >
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.btcEthLeverageLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.btcEthLeverage}
                    onChange={(e) => onFormChange('btcEthLeverage', Number(e.target.value))}
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.altcoinLeverageLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.altcoinLeverage}
                    onChange={(e) => onFormChange('altcoinLeverage', Number(e.target.value))}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.feeLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.fee}
                    onChange={(e) => onFormChange('fee', Number(e.target.value))}
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.slippageLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.slippage}
                    onChange={(e) => onFormChange('slippage', Number(e.target.value))}
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                    {tr('form.cadenceLabel')}
                  </label>
                  <input
                    type="number"
                    className="w-full p-2 rounded-lg text-xs"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                    value={formState.cadence}
                    onChange={(e) => onFormChange('cadence', Number(e.target.value))}
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                  {zh ? '策略风格' : 'Strategy Style'}
                </label>
                <div className="flex flex-wrap gap-1">
                  {['baseline', 'aggressive', 'conservative', 'scalping'].map((p) => (
                    <button
                      key={p}
                      type="button"
                      onClick={() => onFormChange('prompt', p)}
                      className="px-3 py-1.5 rounded text-xs transition-all"
                      style={{
                        background: formState.prompt === p ? 'rgba(240,185,11,0.15)' : '#1E2329',
                        border: `1px solid ${formState.prompt === p ? '#F0B90B' : '#2B3139'}`,
                        color: formState.prompt === p ? '#F0B90B' : '#848E9C',
                      }}
                    >
                      {tr(`form.promptPresets.${p}`)}
                    </button>
                  ))}
                </div>
              </div>

              <div className="flex flex-wrap gap-4 text-xs" style={{ color: '#848E9C' }}>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={formState.cacheAI}
                    onChange={(e) => onFormChange('cacheAI', e.target.checked)}
                    className="accent-[#F0B90B]"
                  />
                  {tr('form.cacheAiLabel')}
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={formState.replayOnly}
                    onChange={(e) => onFormChange('replayOnly', e.target.checked)}
                    className="accent-[#F0B90B]"
                  />
                  {tr('form.replayOnlyLabel')}
                </label>
              </div>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => onWizardStepChange(2)}
                  className="flex-1 py-2 rounded-lg font-medium flex items-center justify-center gap-2"
                  style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                >
                  <ChevronLeft className="w-4 h-4" />
                  {zh ? '上一步' : 'Back'}
                </button>
                <button
                  type="submit"
                  disabled={isStarting}
                  className="flex-1 py-2 rounded-lg font-bold flex items-center justify-center gap-2 disabled:opacity-50"
                  style={{ background: '#F0B90B', color: '#0B0E11' }}
                >
                  {isStarting ? (
                    <RefreshCw className="w-4 h-4 animate-spin" />
                  ) : (
                    <Zap className="w-4 h-4" />
                  )}
                  {isStarting ? tr('starting') : tr('start')}
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </form>
    </div>
  )
}

export type { WizardStep }
