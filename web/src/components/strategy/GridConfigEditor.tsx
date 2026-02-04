import { Grid, DollarSign, TrendingUp, Shield, Compass } from 'lucide-react'
import type { GridStrategyConfig } from '../../types'

interface GridConfigEditorProps {
  config: GridStrategyConfig
  onChange: (config: GridStrategyConfig) => void
  disabled?: boolean
  language: string
}

// Default grid config
export const defaultGridConfig: GridStrategyConfig = {
  symbol: 'BTCUSDT',
  grid_count: 10,
  total_investment: 1000,
  leverage: 5,
  upper_price: 0,
  lower_price: 0,
  use_atr_bounds: true,
  atr_multiplier: 2.0,
  distribution: 'gaussian',
  max_drawdown_pct: 15,
  stop_loss_pct: 5,
  daily_loss_limit_pct: 10,
  use_maker_only: true,
  enable_direction_adjust: false,
  direction_bias_ratio: 0.7,
}

export function GridConfigEditor({
  config,
  onChange,
  disabled,
  language,
}: GridConfigEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      // Section titles
      tradingPair: { zh: '交易设置', en: 'Trading Setup' },
      gridParameters: { zh: '网格参数', en: 'Grid Parameters' },
      priceBounds: { zh: '价格边界', en: 'Price Bounds' },
      riskControl: { zh: '风险控制', en: 'Risk Control' },

      // Trading pair
      symbol: { zh: '交易对', en: 'Trading Pair' },
      symbolDesc: { zh: '选择要进行网格交易的交易对', en: 'Select trading pair for grid trading' },

      // Investment
      totalInvestment: { zh: '投资金额 (USDT)', en: 'Investment (USDT)' },
      totalInvestmentDesc: { zh: '网格策略的总投资金额', en: 'Total investment for grid strategy' },
      leverage: { zh: '杠杆倍数', en: 'Leverage' },
      leverageDesc: { zh: '交易使用的杠杆倍数 (1-5)', en: 'Leverage for trading (1-5)' },

      // Grid parameters
      gridCount: { zh: '网格数量', en: 'Grid Count' },
      gridCountDesc: { zh: '网格层级数量 (5-50)', en: 'Number of grid levels (5-50)' },
      distribution: { zh: '资金分配方式', en: 'Distribution' },
      distributionDesc: { zh: '网格层级的资金分配方式', en: 'Fund allocation across grid levels' },
      uniform: { zh: '均匀分配', en: 'Uniform' },
      gaussian: { zh: '高斯分配 (推荐)', en: 'Gaussian (Recommended)' },
      pyramid: { zh: '金字塔分配', en: 'Pyramid' },

      // Price bounds
      useAtrBounds: { zh: '自动计算边界 (ATR)', en: 'Auto-calculate Bounds (ATR)' },
      useAtrBoundsDesc: { zh: '基于 ATR 自动计算网格上下边界', en: 'Auto-calculate bounds based on ATR' },
      atrMultiplier: { zh: 'ATR 倍数', en: 'ATR Multiplier' },
      atrMultiplierDesc: { zh: '边界距离当前价格的 ATR 倍数', en: 'ATR multiplier for bounds distance' },
      upperPrice: { zh: '上边界价格', en: 'Upper Price' },
      upperPriceDesc: { zh: '网格上边界价格 (0=自动计算)', en: 'Grid upper bound (0=auto)' },
      lowerPrice: { zh: '下边界价格', en: 'Lower Price' },
      lowerPriceDesc: { zh: '网格下边界价格 (0=自动计算)', en: 'Grid lower bound (0=auto)' },

      // Risk control
      maxDrawdown: { zh: '最大回撤 (%)', en: 'Max Drawdown (%)' },
      maxDrawdownDesc: { zh: '触发紧急退出的最大回撤百分比', en: 'Max drawdown before emergency exit' },
      stopLoss: { zh: '止损 (%)', en: 'Stop Loss (%)' },
      stopLossDesc: { zh: '单仓位止损百分比', en: 'Stop loss per position' },
      dailyLossLimit: { zh: '日损失限制 (%)', en: 'Daily Loss Limit (%)' },
      dailyLossLimitDesc: { zh: '每日最大亏损百分比', en: 'Maximum daily loss percentage' },
      useMakerOnly: { zh: '仅使用 Maker 订单', en: 'Maker Only Orders' },
      useMakerOnlyDesc: { zh: '使用限价单以降低手续费', en: 'Use limit orders for lower fees' },

      // Direction adjustment
      directionAdjust: { zh: '方向自动调整', en: 'Direction Auto-Adjust' },
      enableDirectionAdjust: { zh: '启用方向调整', en: 'Enable Direction Adjust' },
      enableDirectionAdjustDesc: { zh: '根据箱体突破自动调整网格方向（做多/做空/偏多/偏空）', en: 'Auto-adjust grid direction based on box breakouts (long/short/long_bias/short_bias)' },
      directionBiasRatio: { zh: '偏向比例', en: 'Bias Ratio' },
      directionBiasRatioDesc: { zh: '偏多/偏空模式下的买卖比例（如 0.7 表示 70% 买 + 30% 卖）', en: 'Buy/sell ratio for bias modes (e.g., 0.7 = 70% buy + 30% sell)' },
      directionExplain: { zh: '短期箱体突破 → 偏向，中期箱体突破 → 全仓，价格回归 → 逐步恢复中性', en: 'Short box breakout → bias, Mid box breakout → full, Price return → gradually recover to neutral' },
    }
    return translations[key]?.[language] || key
  }

  const updateField = <K extends keyof GridStrategyConfig>(
    key: K,
    value: GridStrategyConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
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

  return (
    <div className="space-y-6">
      {/* Trading Setup */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <DollarSign className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('tradingPair')}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Symbol */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('symbol')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('symbolDesc')}
            </p>
            <select
              value={config.symbol}
              onChange={(e) => updateField('symbol', e.target.value)}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <option value="BTCUSDT">BTC/USDT</option>
              <option value="ETHUSDT">ETH/USDT</option>
              <option value="SOLUSDT">SOL/USDT</option>
              <option value="BNBUSDT">BNB/USDT</option>
              <option value="XRPUSDT">XRP/USDT</option>
              <option value="DOGEUSDT">DOGE/USDT</option>
            </select>
          </div>

          {/* Investment */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('totalInvestment')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('totalInvestmentDesc')}
            </p>
            <input
              type="number"
              value={config.total_investment}
              onChange={(e) => updateField('total_investment', parseFloat(e.target.value) || 1000)}
              disabled={disabled}
              min={100}
              step={100}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          {/* Leverage */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('leverage')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('leverageDesc')}
            </p>
            <input
              type="number"
              value={config.leverage}
              onChange={(e) => updateField('leverage', parseInt(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={5}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>
      </div>

      {/* Grid Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Grid className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('gridParameters')}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Grid Count */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('gridCount')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('gridCountDesc')}
            </p>
            <input
              type="number"
              value={config.grid_count}
              onChange={(e) => updateField('grid_count', parseInt(e.target.value) || 10)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          {/* Distribution */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('distribution')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('distributionDesc')}
            </p>
            <select
              value={config.distribution}
              onChange={(e) => updateField('distribution', e.target.value as 'uniform' | 'gaussian' | 'pyramid')}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <option value="uniform">{t('uniform')}</option>
              <option value="gaussian">{t('gaussian')}</option>
              <option value="pyramid">{t('pyramid')}</option>
            </select>
          </div>
        </div>
      </div>

      {/* Price Bounds */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('priceBounds')}
          </h3>
        </div>

        {/* ATR Toggle */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>
                {t('useAtrBounds')}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {t('useAtrBoundsDesc')}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_atr_bounds}
                onChange={(e) => updateField('use_atr_bounds', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
            </label>
          </div>
        </div>

        {config.use_atr_bounds ? (
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('atrMultiplier')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('atrMultiplierDesc')}
            </p>
            <input
              type="number"
              value={config.atr_multiplier}
              onChange={(e) => updateField('atr_multiplier', parseFloat(e.target.value) || 2.0)}
              disabled={disabled}
              min={1}
              max={5}
              step={0.5}
              className="w-32 px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {t('upperPrice')}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {t('upperPriceDesc')}
              </p>
              <input
                type="number"
                value={config.upper_price}
                onChange={(e) => updateField('upper_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {t('lowerPrice')}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {t('lowerPriceDesc')}
              </p>
              <input
                type="number"
                value={config.lower_price}
                onChange={(e) => updateField('lower_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
          </div>
        )}
      </div>

      {/* Risk Control */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('riskControl')}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('maxDrawdown')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('maxDrawdownDesc')}
            </p>
            <input
              type="number"
              value={config.max_drawdown_pct}
              onChange={(e) => updateField('max_drawdown_pct', parseFloat(e.target.value) || 15)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('stopLoss')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('stopLossDesc')}
            </p>
            <input
              type="number"
              value={config.stop_loss_pct}
              onChange={(e) => updateField('stop_loss_pct', parseFloat(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={20}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {t('dailyLossLimit')}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {t('dailyLossLimitDesc')}
            </p>
            <input
              type="number"
              value={config.daily_loss_limit_pct}
              onChange={(e) => updateField('daily_loss_limit_pct', parseFloat(e.target.value) || 10)}
              disabled={disabled}
              min={1}
              max={30}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>

        {/* Maker Only Toggle */}
        <div className="p-4 rounded-lg" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>
                {t('useMakerOnly')}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {t('useMakerOnlyDesc')}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_maker_only}
                onChange={(e) => updateField('use_maker_only', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
            </label>
          </div>
        </div>
      </div>

      {/* Direction Auto-Adjust */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Compass className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('directionAdjust')}
          </h3>
        </div>

        {/* Enable Toggle */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>
                {t('enableDirectionAdjust')}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {t('enableDirectionAdjustDesc')}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.enable_direction_adjust ?? false}
                onChange={(e) => updateField('enable_direction_adjust', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
            </label>
          </div>
        </div>

        {config.enable_direction_adjust && (
          <>
            {/* Direction Explanation */}
            <div className="p-3 rounded-lg mb-4" style={{ background: '#1E2329', border: '1px solid #F0B90B33' }}>
              <p className="text-xs" style={{ color: '#F0B90B' }}>
                💡 {t('directionExplain')}
              </p>
            </div>

            {/* Bias Ratio */}
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {t('directionBiasRatio')}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {t('directionBiasRatioDesc')}
              </p>
              <div className="flex items-center gap-3">
                <input
                  type="range"
                  value={(config.direction_bias_ratio ?? 0.7) * 100}
                  onChange={(e) => updateField('direction_bias_ratio', parseInt(e.target.value) / 100)}
                  disabled={disabled}
                  min={55}
                  max={90}
                  step={5}
                  className="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
                  style={{ background: '#2B3139' }}
                />
                <span className="text-sm font-mono w-16 text-right" style={{ color: '#F0B90B' }}>
                  {Math.round((config.direction_bias_ratio ?? 0.7) * 100)}% / {Math.round((1 - (config.direction_bias_ratio ?? 0.7)) * 100)}%
                </span>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
