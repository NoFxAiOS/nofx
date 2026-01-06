import { Shield, AlertTriangle } from 'lucide-react'
import type { RiskControlConfig } from '../../types'

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
      enableDrawdownProtection: { zh: '启用回撤保护', en: 'Enable Drawdown Protection' },
      enableDrawdownProtectionDesc: { zh: '当利润回撤超过40%时自动平仓', en: 'Auto close position when profit drawdown exceeds 40%' },
      profitLocking: { zh: '盈利锁定', en: 'Profit Locking' },
      enableProfitLocking: { zh: '启用盈利锁定', en: 'Enable Profit Locking' },
      enableProfitLockingDesc: { zh: '当浮动盈亏达到设定的R倍数时，系统自动调整止损位锁定部分或全部利润。内部逻辑：每分钟检查持仓，计算当前R倍数，达到目标时执行止损调整，避免利润回吐。', en: 'When floating profit reaches set R multiples, the system automatically adjusts stop loss to lock partial or full profit. Internal logic: checks positions every minute, calculates current R multiple, and adjusts stop loss when targets are met to prevent profit erosion.' },
      profitLockTargets: { zh: '锁定目标R倍数', en: 'Lock Target R Multiples' },
      profitLockTargetsDesc: { zh: '输入R倍数列表（逗号分隔），例如：1,2,3。R倍数=当前利润/初始风险（入场价到止损价的距离）。系统会按顺序检查目标，达到1R时锁定一次，达到2R时再次锁定，以此类推。', en: 'Enter R multiples (comma-separated), e.g., 1,2,3. R multiple = current profit / initial risk (distance from entry to stop loss). The system checks targets in order, locking once at 1R, again at 2R, and so on.' },
      profitLockMode: { zh: '锁定模式', en: 'Lock Mode' },
      profitLockModeBreakeven: { zh: '盈亏平衡', en: 'Breakeven' },
      profitLockModeBreakevenDesc: { zh: '将止损移动至盈亏平衡点（考虑手续费），确保即使行情反转也不会亏损。计算公式：多单盈亏平衡价=入场价*(1+手续费率)，空单盈亏平衡价=入场价*(1-手续费率)。', en: 'Moves stop loss to breakeven price (considering fees), ensuring no loss even if the market reverses. Formula: long breakeven = entry price*(1+fee rate), short breakeven = entry price*(1-fee rate).' },
      profitLockModeTrailing: { zh: '移动止损', en: 'Trailing' },
      profitLockModeTrailingDesc: { zh: '根据当前R倍数动态调整止损位，锁定已获得的部分利润。例如：在3R模式下，止损会移动到确保获得2R利润的位置，即使行情反转也能保留大部分收益。', en: 'Dynamically adjusts stop loss based on current R multiple, locking in partial profits. For example: in 3R mode, stop loss moves to secure 2R profit, preserving most gains even if the market reverses.' },
      profitLockPercentage: { zh: '锁定仓位比例', en: 'Lock Position Percentage' },
      profitLockPercentageDesc: { zh: '每次达到锁定目标时平仓的比例（0.3=30%），用于实现分批锁定利润的策略。', en: 'Percentage of position to close when reaching lock targets (0.3=30%), used to implement partial profit locking strategy.' },
      feeRate: { zh: '手续费率', en: 'Fee Rate' },
      feeRateDesc: { zh: '用于精确计算盈亏平衡点的手续费率，默认0.05%。系统会根据此费率调整止损位，确保真正达到无亏损状态。', en: 'Fee rate used to accurately calculate breakeven price, default 0.05%. The system adjusts stop loss based on this rate to ensure true breakeven status.' },
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

  return (
    <div className="space-y-6">
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

          {/* Drawdown Protection */}
          <div
            className="p-4 rounded-lg col-span-2"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <div className="flex items-center justify-between">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('enableDrawdownProtection')}
                </label>
                <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                  {t('enableDrawdownProtectionDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={config.enable_drawdown_protection ?? true}
                onChange={(e) =>
                  updateField('enable_drawdown_protection', e.target.checked)
                }
                disabled={disabled}
                className="h-5 w-5 accent-yellow-500"
              />
            </div>
          </div>

          {/* Profit Locking */}
          <div
            className="p-4 rounded-lg col-span-2"
            style={{ background: '#0B0E11', border: '1px solid #F0B90B' }}
          >
            <div className="flex items-center justify-between mb-4">
              <div>
                <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                  {t('enableProfitLocking')}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {t('enableProfitLockingDesc')}
                </p>
              </div>
              <input
                type="checkbox"
                checked={config.enable_profit_locking ?? true}
                onChange={(e) =>
                  updateField('enable_profit_locking', e.target.checked)
                }
                disabled={disabled}
                className="h-5 w-5 accent-yellow-500"
              />
            </div>

            {/* Show profit locking settings when enabled */}
            {(config.enable_profit_locking ?? true) && (
              <div className="mt-4 space-y-4">
                {/* Profit Lock Targets */}
                <div>
                  <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                    {t('profitLockTargets')}
                  </label>
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {t('profitLockTargetsDesc')}
                  </p>
                  <input
                    type="text"
                    value={(config.profit_lock_targets ?? [1, 2, 3]).join(', ')}
                    onChange={(e) => {
                      const values = e.target.value.split(',')
                        .map(v => parseFloat(v.trim()))
                        .filter(v => !isNaN(v))
                      updateField('profit_lock_targets', values)
                    }}
                    disabled={disabled}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                </div>

                {/* Profit Lock Mode */}
                <div>
                  <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                    {t('profitLockMode')}
                  </label>
                  <div className="flex gap-4">
                    <label className="flex items-center">
                      <input
                        type="radio"
                        name="profitLockMode"
                        value="breakeven"
                        checked={(config.profit_lock_mode ?? 'breakeven') === 'breakeven'}
                        onChange={(e) =>
                          updateField('profit_lock_mode', e.target.value)
                        }
                        disabled={disabled}
                        className="mr-2 accent-yellow-500"
                      />
                      <span className="text-sm" style={{ color: '#EAECEF' }}>
                        {t('profitLockModeBreakeven')}
                      </span>
                    </label>
                    <label className="flex items-center">
                      <input
                        type="radio"
                        name="profitLockMode"
                        value="trailing"
                        checked={(config.profit_lock_mode ?? 'breakeven') === 'trailing'}
                        onChange={(e) =>
                          updateField('profit_lock_mode', e.target.value)
                        }
                        disabled={disabled}
                        className="mr-2 accent-yellow-500"
                      />
                      <span className="text-sm" style={{ color: '#EAECEF' }}>
                        {t('profitLockModeTrailing')}
                      </span>
                    </label>
                  </div>
                </div>

                {/* Profit Lock Percentage */}
                <div>
                  <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                    {t('profitLockPercentage')}
                  </label>
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {t('profitLockPercentageDesc')}
                  </p>
                  <div className="flex items-center gap-2">
                    <input
                      type="range"
                      value={(config.profit_lock_percentage ?? 0.3) * 100}
                      onChange={(e) =>
                        updateField('profit_lock_percentage', parseFloat(e.target.value) / 100)
                      }
                      disabled={disabled}
                      min={0}
                      max={100}
                      step={5}
                      className="flex-1 accent-yellow-500"
                    />
                    <span className="w-16 text-center font-mono" style={{ color: '#F0B90B' }}>
                      {((config.profit_lock_percentage ?? 0.3) * 100).toFixed(0)}%
                    </span>
                  </div>
                </div>

                {/* Fee Rate */}
                <div>
                  <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                    {t('feeRate')}
                  </label>
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {t('feeRateDesc')}
                  </p>
                  <div className="flex items-center gap-2">
                    <input
                      type="range"
                      value={(config.fee_rate ?? 0.0005) * 10000}
                      onChange={(e) =>
                        updateField('fee_rate', parseInt(e.target.value) / 10000)
                      }
                      disabled={disabled}
                      min={1}
                      max={20}
                      className="flex-1 accent-yellow-500"
                    />
                    <span className="w-16 text-center font-mono" style={{ color: '#F0B90B' }}>
                      {((config.fee_rate ?? 0.0005) * 100).toFixed(2)}%
                    </span>
                  </div>
                </div>
              </div>
            )}
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
