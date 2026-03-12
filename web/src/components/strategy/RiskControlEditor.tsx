import { Shield, AlertTriangle, Target } from 'lucide-react'
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
      maxPositionsDesc: {
        zh: '同时持有的最大币种数量',
        en: 'Maximum coins held simultaneously',
      },
      // Trading leverage (exchange leverage)
      tradingLeverage: {
        zh: '交易杠杆（交易所杠杆）',
        en: 'Trading Leverage (Exchange)',
      },
      btcEthLeverage: {
        zh: 'BTC/ETH 交易杠杆',
        en: 'BTC/ETH Trading Leverage',
      },
      btcEthLeverageDesc: {
        zh: '交易所开仓使用的杠杆倍数',
        en: 'Exchange leverage for opening positions',
      },
      altcoinLeverage: { zh: '山寨币交易杠杆', en: 'Altcoin Trading Leverage' },
      altcoinLeverageDesc: {
        zh: '交易所开仓使用的杠杆倍数',
        en: 'Exchange leverage for opening positions',
      },
      // Position value ratio (risk control) - CODE ENFORCED
      positionValueRatio: {
        zh: '仓位价值比例（代码强制）',
        en: 'Position Value Ratio (CODE ENFORCED)',
      },
      positionValueRatioDesc: {
        zh: '单仓位名义价值 / 账户净值，由代码强制执行',
        en: 'Position notional value / equity, enforced by code',
      },
      btcEthPositionValueRatio: {
        zh: 'BTC/ETH 仓位价值比例',
        en: 'BTC/ETH Position Value Ratio',
      },
      btcEthPositionValueRatioDesc: {
        zh: '单仓最大名义价值 = 净值 × 此值（代码强制）',
        en: 'Max position value = equity × this ratio (CODE ENFORCED)',
      },
      altcoinPositionValueRatio: {
        zh: '山寨币仓位价值比例',
        en: 'Altcoin Position Value Ratio',
      },
      altcoinPositionValueRatioDesc: {
        zh: '单仓最大名义价值 = 净值 × 此值（代码强制）',
        en: 'Max position value = equity × this ratio (CODE ENFORCED)',
      },
      riskParameters: { zh: '风险参数', en: 'Risk Parameters' },
      minRiskReward: { zh: '最小风险回报比', en: 'Min Risk/Reward Ratio' },
      minRiskRewardDesc: {
        zh: '开仓要求的最低盈亏比',
        en: 'Minimum profit ratio for opening',
      },
      maxMarginUsage: {
        zh: '最大保证金使用率（代码强制）',
        en: 'Max Margin Usage (CODE ENFORCED)',
      },
      maxMarginUsageDesc: {
        zh: '保证金使用率上限，由代码强制执行',
        en: 'Maximum margin utilization, enforced by code',
      },
      entryRequirements: { zh: '开仓要求', en: 'Entry Requirements' },
      minPositionSize: { zh: '最小开仓金额', en: 'Min Position Size' },
      minPositionSizeDesc: {
        zh: 'USDT 最小名义价值',
        en: 'Minimum notional value in USDT',
      },
      minConfidence: { zh: '最小信心度', en: 'Min Confidence' },
      minConfidenceDesc: {
        zh: 'AI 开仓信心度阈值',
        en: 'AI confidence threshold for entry',
      },
      positionManagement: { zh: '持仓管理', en: 'Position Management' },
      breakevenThreshold: { zh: '保本目标', en: 'Breakeven Target' },
      breakevenThresholdDesc: {
        zh: '未实现盈亏达到此百分比时，自动将止损调整到入场价格（保本）',
        en: 'When UnrealizedPnL% reaches this threshold, automatically adjust stop loss to entry price (breakeven)',
      },
      updateStopLossEnabled: { zh: '动态止损', en: 'Dynamic Stop-Loss' },
      updateStopLossEnabledDesc: {
        zh: '启用动态止损功能，AI可通过update_stop_loss action实时更新止损价格（仅支持币安交易所）',
        en: 'Enable dynamic stop-loss updates, AI can update stop-loss price via update_stop_loss action in real-time (Binance only)',
      },
      exitSignals: { zh: '出场信号', en: 'Exit Signals' },
      hardStopLoss: { zh: '硬止损', en: 'Hard Stop Loss' },
      hardStopLossDesc: {
        zh: '单个持仓亏损达到此百分比时建议止损（AI建议，通过设置stop_loss字段执行）',
        en: 'Recommend stop-loss when single position loss reaches this % (AI guided, execute via stop_loss field)',
      },
      trailingStop: { zh: '跟踪止盈', en: 'Trailing Stop' },
      trailingStopPct: { zh: '回撤百分比', en: 'Pullback Percentage' },
      trailingStopPctDesc: {
        zh: '当持仓盈亏从峰值回撤此百分比时，建议部分或全部止盈（AI建议）',
        en: 'Recommend partial/full profit-taking when PnL pulls back this % from peak (AI guided)',
      },
      trailingStopMinProfit: { zh: '最小盈利要求', en: 'Min Profit Required' },
      trailingStopMinProfitDesc: {
        zh: '跟踪止盈激活所需的最小盈利百分比（代码强制执行）',
        en: 'Minimum profit % required before trailing stop activates (CODE ENFORCED)',
      },
      trailingStopDrawdown: {
        zh: '触发平仓回撤',
        en: 'Close Trigger Drawdown',
      },
      trailingStopDrawdownDesc: {
        zh: '从峰值回撤此百分比时自动平仓（代码强制执行）',
        en: 'Auto-close position when drawdown from peak reaches this % (CODE ENFORCED)',
      },
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('maxPositions')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('maxPositionsDesc')}
            </p>
            <div className="flex items-center gap-2">
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
              <span className="text-sm" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '个' : ''}
              </span>
            </div>
          </div>
        </div>

        {/* Trading Leverage (Exchange) */}
        <div className="mb-2">
          <p
            className="text-xs font-medium mb-2 break-words"
            style={{ color: '#F0B90B' }}
          >
            {t('tradingLeverage')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('btcEthLeverage')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('altcoinLeverage')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
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
          <p
            className="text-xs font-medium break-words"
            style={{ color: '#0ECB81' }}
          >
            {t('positionValueRatio')}
          </p>
          <p className="text-xs mt-1 break-words" style={{ color: '#848E9C' }}>
            {t('positionValueRatioDesc')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('btcEthPositionValueRatio')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('btcEthPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.btc_eth_max_position_value_ratio ?? 5}
                onChange={(e) =>
                  updateField(
                    'btc_eth_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('altcoinPositionValueRatio')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('altcoinPositionValueRatioDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={config.altcoin_max_position_value_ratio ?? 1}
                onChange={(e) =>
                  updateField(
                    'altcoin_max_position_value_ratio',
                    parseFloat(e.target.value)
                  )
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

      {/* Position Management */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Target className="w-5 h-5" style={{ color: '#0ECB81' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('positionManagement')}
          </h3>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('breakevenThreshold')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('breakevenThresholdDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={config.breakeven_threshold ?? 3.0}
                onChange={(e) =>
                  updateField(
                    'breakeven_threshold',
                    parseFloat(e.target.value) || 3.0
                  )
                }
                disabled={disabled}
                min={0.5}
                max={10.0}
                step={0.5}
                className="w-32 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #0ECB81',
                  color: '#EAECEF',
                }}
              />
              <span className="text-sm" style={{ color: '#848E9C' }}>
                %
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #0ECB81' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('updateStopLossEnabled')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('updateStopLossEnabledDesc')}
            </p>
            <div className="flex items-center gap-2 min-w-0">
              <input
                type="checkbox"
                checked={config.update_stop_loss_enabled ?? false}
                onChange={(e) =>
                  updateField('update_stop_loss_enabled', e.target.checked)
                }
                disabled={disabled}
                className="w-5 h-5 rounded flex-shrink-0"
                style={{
                  accentColor: '#0ECB81',
                }}
              />
              <span
                className="text-sm break-words min-w-0"
                style={{ color: '#EAECEF' }}
              >
                {config.update_stop_loss_enabled
                  ? language === 'zh'
                    ? '已启用'
                    : 'Enabled'
                  : language === 'zh'
                    ? '已禁用'
                    : 'Disabled'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Exit Signals */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <AlertTriangle className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('exitSignals')}
          </h3>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #F0B90B' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('hardStopLoss')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('hardStopLossDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={config.hard_stop_loss_pct ?? -5.0}
                onChange={(e) =>
                  updateField(
                    'hard_stop_loss_pct',
                    parseFloat(e.target.value) || -5.0
                  )
                }
                disabled={disabled}
                min={-20.0}
                max={-1.0}
                step={0.5}
                className="w-32 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #F0B90B',
                  color: '#EAECEF',
                }}
              />
              <span className="text-sm" style={{ color: '#848E9C' }}>
                %
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #F0B90B' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('trailingStopPct')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('trailingStopPctDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={config.trailing_stop_pct ?? 30.0}
                onChange={(e) =>
                  updateField(
                    'trailing_stop_pct',
                    parseFloat(e.target.value) || 30.0
                  )
                }
                disabled={disabled}
                min={10.0}
                max={80.0}
                step={5.0}
                className="w-32 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #F0B90B',
                  color: '#EAECEF',
                }}
              />
              <span className="text-sm" style={{ color: '#848E9C' }}>
                %
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #F0B90B' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('trailingStopMinProfit')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('trailingStopMinProfitDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={config.trailing_stop_min_profit ?? 5.0}
                onChange={(e) =>
                  updateField(
                    'trailing_stop_min_profit',
                    parseFloat(e.target.value) || 5.0
                  )
                }
                disabled={disabled}
                min={1.0}
                max={20.0}
                step={0.5}
                className="w-32 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #F0B90B',
                  color: '#EAECEF',
                }}
              />
              <span className="text-sm" style={{ color: '#848E9C' }}>
                %
              </span>
            </div>
          </div>

          <div
            className="p-4 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #F0B90B' }}
          >
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('trailingStopDrawdown')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('trailingStopDrawdownDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="number"
                value={config.trailing_stop_drawdown ?? 40.0}
                onChange={(e) =>
                  updateField(
                    'trailing_stop_drawdown',
                    parseFloat(e.target.value) || 40.0
                  )
                }
                disabled={disabled}
                min={20.0}
                max={80.0}
                step={5.0}
                className="w-32 px-3 py-2 rounded"
                style={{
                  background: '#1E2329',
                  border: '1px solid #F0B90B',
                  color: '#EAECEF',
                }}
              />
              <span className="text-sm" style={{ color: '#848E9C' }}>
                %
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('minRiskReward')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('minRiskRewardDesc')}
            </p>
            <div className="flex items-center">
              <span style={{ color: '#848E9C' }}>1:</span>
              <input
                type="number"
                value={config.min_risk_reward_ratio ?? 3}
                onChange={(e) =>
                  updateField(
                    'min_risk_reward_ratio',
                    parseFloat(e.target.value) || 3
                  )
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('maxMarginUsage')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('maxMarginUsageDesc')}
            </p>
            <div className="flex items-center gap-2">
              <input
                type="range"
                value={(config.max_margin_usage ?? 0.9) * 100}
                onChange={(e) =>
                  updateField(
                    'max_margin_usage',
                    parseInt(e.target.value) / 100
                  )
                }
                disabled={disabled}
                min={10}
                max={100}
                className="flex-1 accent-green-500"
              />
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('minPositionSize')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
              {t('minPositionSizeDesc')}
            </p>
            <div className="flex items-center">
              <input
                type="number"
                value={config.min_position_size ?? 12}
                onChange={(e) =>
                  updateField(
                    'min_position_size',
                    parseFloat(e.target.value) || 12
                  )
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
            <label
              className="block text-sm mb-1 break-words"
              style={{ color: '#EAECEF' }}
            >
              {t('minConfidence')}
            </label>
            <p
              className="text-xs mb-2 break-words"
              style={{ color: '#848E9C' }}
            >
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
              <span
                className="w-12 text-center font-mono"
                style={{ color: '#0ECB81' }}
              >
                {config.min_confidence ?? 75}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
