import { Shield, AlertTriangle } from 'lucide-react'
import type { RiskControlConfig, TrailingStopConfig } from '../../types'

interface RiskControlEditorProps {
  config: RiskControlConfig
  onChange: (config: RiskControlConfig) => void
  disabled?: boolean
  language: string
}

export function RiskControlEditor({
  config,
  onChange,
  disabled,
  language,
}: RiskControlEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      positionLimits: { zh: '仓位限制', en: 'Position Limits' },
      maxPositions: { zh: '最大持仓数量', en: 'Max Positions' },
      maxPositionsDesc: { zh: '同时持有的最大币种数量', en: 'Maximum coins held simultaneously' },
      trailingStop: { zh: '移动止盈', en: 'Trailing Stop' },
      trailingStopDesc: { zh: '常规移动止盈：跟随持仓盈亏或价格，触发即平仓（可选部分平仓）', en: 'Classic trailing stop on PnL% or price; closes when stop is hit (optional partial close)' },
      enableTrailing: { zh: '启用移动止盈', en: 'Enable trailing stop' },
      mode: { zh: '模式', en: 'Mode' },
      modeDesc: { zh: '按盈亏%或价格跟踪', en: 'Trail by PnL% or price' },
      activationPct: { zh: '启动阈值（%）', en: 'Activation Threshold (%)' },
      activationPctDesc: { zh: '盈亏达到该值后开始跟踪（0=立即）', en: 'Start trailing after this PnL% (0 = immediate)' },
      trailPct: { zh: '跟踪距离（%）', en: 'Trail Distance (%)' },
      trailPctDesc: { zh: '止损线=峰值-该百分比（百分比点）', en: 'Stop = peak – this percentage distance (percent points)' },
      checkInterval: { zh: '检查频率（毫秒）', en: 'Check Interval (ms)' },
      checkIntervalDesc: { zh: '监控间隔（支持毫秒，越短越及时，需 websocket）', en: 'Monitoring interval (ms, websocket friendly for fast response)' },
      closePct: { zh: '平仓比例', en: 'Close Portion' },
      closePctDesc: { zh: '触发后平掉多少仓位（1=全平）', en: 'Portion of position to close when triggered (1=full)' },
      tightenBands: { zh: '收紧梯度', en: 'Tighten Bands' },
      addBand: { zh: '添加梯度', en: 'Add band' },
      profitPct: { zh: '收益达到（%）', en: 'Profit ≥ (%)' },
      bandTrailPct: { zh: '跟踪距离（%）', en: 'Trail (%)' },
      // Trading leverage (exchange leverage)
      tradingLeverage: { zh: '交易杠杆（交易所杠杆）', en: 'Trading Leverage (Exchange)' },
      btcEthLeverage: { zh: 'BTC/ETH 交易杠杆', en: 'BTC/ETH Trading Leverage' },
      btcEthLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      altcoinLeverage: { zh: '山寨币交易杠杆', en: 'Altcoin Trading Leverage' },
      altcoinLeverageDesc: { zh: '交易所开仓使用的杠杆倍数', en: 'Exchange leverage for opening positions' },
      // Position value ratio (risk control) - CODE ENFORCED
      positionValueRatio: { zh: '仓位价值比例（代码强制）', en: 'Position Value Ratio (CODE ENFORCED)' },
      positionValueRatioDesc: { zh: '单仓位名义价值 / 账户净值，由代码强制执行', en: 'Position notional value / equity, enforced by code' },
      btcEthPositionValueRatio: { zh: 'BTC/ETH 仓位价值比例', en: 'BTC/ETH Position Value Ratio' },
      btcEthPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      altcoinPositionValueRatio: { zh: '山寨币仓位价值比例', en: 'Altcoin Position Value Ratio' },
      altcoinPositionValueRatioDesc: { zh: '单仓最大名义价值 = 净值 × 此值（代码强制）', en: 'Max position value = equity × this ratio (CODE ENFORCED)' },
      riskParameters: { zh: '风险参数', en: 'Risk Parameters' },
      minRiskReward: { zh: '最小风险回报比', en: 'Min Risk/Reward Ratio' },
      minRiskRewardDesc: { zh: '开仓要求的最低盈亏比', en: 'Minimum profit ratio for opening' },
      maxMarginUsage: { zh: '最大保证金使用率（代码强制）', en: 'Max Margin Usage (CODE ENFORCED)' },
      maxMarginUsageDesc: { zh: '保证金使用率上限，由代码强制执行', en: 'Maximum margin utilization, enforced by code' },
      entryRequirements: { zh: '开仓要求', en: 'Entry Requirements' },
      minPositionSize: { zh: '最小开仓金额', en: 'Min Position Size' },
      minPositionSizeDesc: { zh: 'USDT 最小名义价值', en: 'Minimum notional value in USDT' },
      minConfidence: { zh: '最小信心度', en: 'Min Confidence' },
      minConfidenceDesc: { zh: 'AI 开仓信心度阈值', en: 'AI confidence threshold for entry' },
    }
    return translations[key]?.[language] || key
  }

  const updateField = <K extends keyof RiskControlConfig>(
    key: K,
    value: RiskControlConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const trailingDefaults: TrailingStopConfig = {
    enabled: true,
    mode: 'pnl_pct',
    activation_pct: 0,
    trail_pct: 3,
    check_interval_sec: 5,
    check_interval_ms: undefined,
    tighten_bands: [],
    close_pct: 1,
  }

  const trailing = {
    ...trailingDefaults,
    ...(config.trailing_stop || {}),
  }

  const updateTrailing = (patch: Partial<TrailingStopConfig>) => {
    if (disabled) return
    onChange({
      ...config,
      trailing_stop: {
        ...trailing,
        ...patch,
      },
    })
  }

  return (
    <div className="space-y-6">
      {/* Trailing Stop */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('trailingStop')}
          </h3>
        </div>
        <p className="text-xs mb-4" style={{ color: '#848E9C' }}>
          {t('trailingStopDesc')}
        </p>
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-2" style={{ color: '#EAECEF' }}>
              {t('enableTrailing')}
            </label>
            <label className="inline-flex items-center gap-2 text-sm" style={{ color: '#EAECEF' }}>
              <input
                type="checkbox"
                checked={trailing.enabled}
                onChange={(e) => updateTrailing({ enabled: e.target.checked })}
                disabled={disabled}
                className="accent-green-500 w-4 h-4"
              />
              {trailing.enabled ? (language === 'zh' ? '已启用' : 'Enabled') : (language === 'zh' ? '已关闭' : 'Disabled')}
            </label>
          </div>

          <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('mode')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('modeDesc')}
            </p>
            <select
              value={trailing.mode || 'pnl_pct'}
              onChange={(e) => updateTrailing({ mode: e.target.value as TrailingStopConfig['mode'] })}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
            >
              <option value="pnl_pct">PnL %</option>
              <option value="price_pct">Price %</option>
            </select>
          </div>

          <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('activationPct')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('activationPctDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={trailing.activation_pct ?? 0}
                onChange={(e) =>
                  updateTrailing({ activation_pct: parseFloat(e.target.value) || 0 })
                }
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span style={{ color: '#848E9C' }}>%</span>
            </div>
          </div>

          <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('trailPct')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('trailPctDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={trailing.trail_pct ?? 3}
                onChange={(e) =>
                  updateTrailing({ trail_pct: parseFloat(e.target.value) || 0 })
                }
                disabled={disabled}
                min={0.01}
                max={100}
                step={0.01}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span style={{ color: '#848E9C' }}>%</span>
            </div>
          </div>

          <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('checkInterval')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('checkIntervalDesc')}
            </p>
            <div className="flex items-center gap-2">
              {(() => {
                const intervalMs =
                  trailing.check_interval_ms ??
                  (trailing.check_interval_sec ? trailing.check_interval_sec * 1000 : 30000)
                return (
                  <>
                    <input
                      type="number"
                      value={intervalMs}
                      onChange={(e) => {
                        const val = parseInt(e.target.value) || 0
                        updateTrailing({
                          check_interval_ms: val,
                          check_interval_sec: Math.round(val / 1000),
                        })
                      }}
                      disabled={disabled}
                      min={10}
                      max={600000}
                      step={10}
                      className="w-28 px-3 py-2 rounded"
                      style={{
                        background: '#1E2329',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                    />
                    <span style={{ color: '#848E9C' }}>ms</span>
                    <span className="text-xs" style={{ color: '#848E9C' }}>
                      (~{(intervalMs / 1000).toFixed(2)}s)
                    </span>
                  </>
                )
              })()}
            </div>
          </div>

          <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('closePct')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('closePctDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={trailing.close_pct ?? 1}
                onChange={(e) =>
                  updateTrailing({ close_pct: parseFloat(e.target.value) || 0 })
                }
                disabled={disabled}
                min={0.01}
                max={1}
                step={0.01}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span style={{ color: '#848E9C' }}>%</span>
            </div>
          </div>
        </div>

        {/* Tighten Bands */}
        <div className="mt-4">
          <div className="flex items-center justify-between mb-2">
            <div>
              <p className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {t('tightenBands')}
              </p>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '达到收益阈值后自动缩紧跟踪距离'
                  : 'Tighten trail after reaching profit bands'}
              </p>
            </div>
            <button
              onClick={() =>
                updateTrailing({
                  tighten_bands: [
                    ...(trailing.tighten_bands || []),
                    { profit_pct: 10, trail_pct: Math.max(0.2, (trailing.trail_pct ?? 3) / 2) },
                  ],
                })
              }
              disabled={disabled}
              className="px-3 py-1 text-xs rounded"
              style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
            >
              {t('addBand')}
            </button>
          </div>
          <div className="space-y-2">
            {(trailing.tighten_bands || []).map((band, idx) => (
              <div
                key={idx}
                className="grid grid-cols-1 md:grid-cols-5 gap-2 items-center p-3 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="md:col-span-2 flex items-center gap-2">
                  <span className="text-xs" style={{ color: '#EAECEF' }}>{t('profitPct')}</span>
                  <input
                    type="number"
                    value={band.profit_pct}
                onChange={(e) => {
                  const updated = [...(trailing.tighten_bands || [])]
                  updated[idx] = { ...band, profit_pct: parseFloat(e.target.value) || 0 }
                  updateTrailing({ tighten_bands: updated })
                }}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-24 px-2 py-1 rounded text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
              <span className="text-xs" style={{ color: '#848E9C' }}>%</span>
            </div>
                <div className="md:col-span-2 flex items-center gap-2">
                  <span className="text-xs" style={{ color: '#EAECEF' }}>{t('bandTrailPct')}</span>
                  <input
                    type="number"
                    value={band.trail_pct}
                onChange={(e) => {
                  const updated = [...(trailing.tighten_bands || [])]
                  updated[idx] = { ...band, trail_pct: parseFloat(e.target.value) || 0 }
                  updateTrailing({ tighten_bands: updated })
                }}
                disabled={disabled}
                min={0.01}
                step={0.01}
                className="w-24 px-2 py-1 rounded text-sm"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
                  <span className="text-xs" style={{ color: '#848E9C' }}>%</span>
                </div>
                <div className="flex justify-end">
                  <button
                    onClick={() => {
                      const updated = [...(trailing.tighten_bands || [])]
                      updated.splice(idx, 1)
                      updateTrailing({ tighten_bands: updated })
                    }}
                    disabled={disabled}
                    className="text-xs px-2 py-1 rounded"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#F6465D' }}
                  >
                    ✕
                  </button>
                </div>
              </div>
            ))}
            {(trailing.tighten_bands || []).length === 0 && (
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '未设置收紧梯度' : 'No tighten bands configured'}
              </p>
            )}
          </div>
        </div>
      </div>

      {/* Position Limits */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('positionLimits')}
          </h3>
        </div>

        <div className="grid grid-cols-1 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxPositions')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxPositionsDesc')}
            </p>
            <input
              type="number"
              value={config.max_positions ?? 3}
              onChange={(e) =>
                updateField('max_positions', parseInt(e.target.value) || 3)
              }
              disabled={disabled}
              min={1}
              max={10}
              className="w-32 px-3 py-2 rounded"
              style={{
                background: '#1E2329',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
          </div>
        </div>

        {/* Trading Leverage (Exchange) */}
        <div className="mb-2">
          <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
            {t('tradingLeverage')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.btc_eth_max_leverage ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('altcoinLeverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinLeverageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_leverage ?? 5}
                onChange={(e) =>
                  updateField('altcoin_max_leverage', parseInt(e.target.value))
                }
                disabled={disabled}
                min={1}
                max={20}
                className="flex-1 accent-yellow-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#F0B90B' }}
              >
                {config.altcoin_max_leverage ?? 5}x
              </span>
            </div>
          </div>
        </div>

        {/* Position Value Ratio (Risk Control - CODE ENFORCED) */}
        <div className="mb-2">
          <p className="text-xs font-medium" style={{ color: '#0ECB81' }}>
            {t('positionValueRatio')}
          </p>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('positionValueRatioDesc')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('btcEthPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('btcEthPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField('btc_eth_max_position_value_ratio', parseFloat(e.target.value))
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.btc_eth_max_position_value_ratio ?? 5}x
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('altcoinPositionValueRatio')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('altcoinPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField('altcoin_max_position_value_ratio', parseFloat(e.target.value))
                }
                disabled={disabled}
                min={0.5}
                max={10}
                step={0.5}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.altcoin_max_position_value_ratio ?? 1}x
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Risk Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F6465D' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('riskParameters')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minRiskReward')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minRiskRewardDesc')}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField('min_risk_reward_ratio', parseFloat(e.target.value) || 3)
                }
                disabled={disabled}
                min={1}
                max={10}
                step={0.5}
                className="w-20 px-3 py-2 rounded ml-2"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxMarginUsage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxMarginUsageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField('max_margin_usage', parseInt(e.target.value) / 100)
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {Math.round((config.max_margin_usage ?? 0.9) * 100)}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Entry Requirements */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('entryRequirements')}
          </h3>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minPositionSize')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minPositionSizeDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField('min_position_size', parseFloat(e.target.value) || 12)
                }
                disabled={disabled}
                min={10}
                max={1000}
                className="w-24 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <span className="ml-2" style={{ color: '#848E9C' }}>
                USDT
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('minConfidence')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('minConfidenceDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.min_confidence ?? 75}
                onChange={(e) =>
                  updateField('min_confidence', parseInt(e.target.value))
                }
                disabled={disabled}
                min={50}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span className="w-12 text-center font-mono" style={{ color: '#0ECB81' }}>
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
