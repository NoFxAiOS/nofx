import { Clock, Activity, Database } from 'lucide-react'
import type { IndicatorConfig } from '../../types'

interface IndicatorEditorProps {
  config: IndicatorConfig
  onChange: (config: IndicatorConfig) => void
  disabled?: boolean
  language: string
}

// 所有可用时间周期
const allTimeframes = [
  { value: '1m', label: '1m', category: 'scalp' },
  { value: '3m', label: '3m', category: 'scalp' },
  { value: '5m', label: '5m', category: 'scalp' },
  { value: '15m', label: '15m', category: 'intraday' },
  { value: '30m', label: '30m', category: 'intraday' },
  { value: '1h', label: '1h', category: 'intraday' },
  { value: '2h', label: '2h', category: 'swing' },
  { value: '4h', label: '4h', category: 'swing' },
  { value: '6h', label: '6h', category: 'swing' },
  { value: '8h', label: '8h', category: 'swing' },
  { value: '12h', label: '12h', category: 'swing' },
  { value: '1d', label: '1D', category: 'position' },
  { value: '3d', label: '3D', category: 'position' },
  { value: '1w', label: '1W', category: 'position' },
]

export function IndicatorEditor({
  config,
  onChange,
  disabled,
  language,
}: IndicatorEditorProps) {
  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      timeframes: { zh: '时间周期', en: 'Timeframes' },
      timeframesDesc: { zh: '选择要分析的K线周期（可多选）', en: 'Select K-line timeframes to analyze (multi-select)' },
      primaryTimeframe: { zh: '主周期', en: 'Primary' },
      klineCount: { zh: 'K线数量', en: 'K-line Count' },
      technicalIndicators: { zh: '技术指标', en: 'Technical Indicators' },
      ema: { zh: 'EMA 均线', en: 'EMA' },
      macd: { zh: 'MACD', en: 'MACD' },
      rsi: { zh: 'RSI', en: 'RSI' },
      atr: { zh: 'ATR', en: 'ATR' },
      volume: { zh: '成交量', en: 'Volume' },
      oi: { zh: '持仓量', en: 'OI' },
      fundingRate: { zh: '资金费率', en: 'Funding' },
      periods: { zh: '周期', en: 'Periods' },
      scalp: { zh: '剥头皮', en: 'Scalp' },
      intraday: { zh: '日内', en: 'Intraday' },
      swing: { zh: '波段', en: 'Swing' },
      position: { zh: '趋势', en: 'Position' },
      quantData: { zh: '量化数据', en: 'Quant Data' },
      quantDataDesc: { zh: '资金流向、持仓变化、价格变化（按币种查询）', en: 'Netflow, OI delta, price change (per coin)' },
      quantDataUrl: { zh: '量化数据 API', en: 'Quant Data API' },
    }
    return translations[key]?.[language] || key
  }

  // 获取当前选中的时间周期
  const selectedTimeframes = config.klines.selected_timeframes || [config.klines.primary_timeframe]

  // 切换时间周期选择
  const toggleTimeframe = (tf: string) => {
    if (disabled) return
    const current = [...selectedTimeframes]
    const index = current.indexOf(tf)

    if (index >= 0) {
      // 如果已选中，取消选择（但保留至少一个）
      if (current.length > 1) {
        current.splice(index, 1)
        // 如果取消的是主周期，则选第一个为主周期
        const newPrimary = tf === config.klines.primary_timeframe ? current[0] : config.klines.primary_timeframe
        onChange({
          ...config,
          klines: {
            ...config.klines,
            selected_timeframes: current,
            primary_timeframe: newPrimary,
            enable_multi_timeframe: current.length > 1,
          },
        })
      }
    } else {
      // 添加新的时间周期
      current.push(tf)
      onChange({
        ...config,
        klines: {
          ...config.klines,
          selected_timeframes: current,
          enable_multi_timeframe: current.length > 1,
        },
      })
    }
  }

  // 设置主时间周期
  const setPrimaryTimeframe = (tf: string) => {
    if (disabled) return
    onChange({
      ...config,
      klines: {
        ...config.klines,
        primary_timeframe: tf,
      },
    })
  }

  const indicators = [
    { key: 'enable_ema', label: 'ema', color: '#F0B90B', periodKey: 'ema_periods' },
    { key: 'enable_macd', label: 'macd', color: '#0ECB81' },
    { key: 'enable_rsi', label: 'rsi', color: '#F6465D', periodKey: 'rsi_periods' },
    { key: 'enable_atr', label: 'atr', color: '#60a5fa', periodKey: 'atr_periods' },
    { key: 'enable_volume', label: 'volume', color: '#c084fc' },
    { key: 'enable_oi', label: 'oi', color: '#34d399' },
    { key: 'enable_funding_rate', label: 'fundingRate', color: '#fbbf24' },
  ]

  const categoryColors: Record<string, string> = {
    scalp: '#F6465D',
    intraday: '#F0B90B',
    swing: '#0ECB81',
    position: '#60a5fa',
  }

  return (
    <div className="space-y-4">
      {/* Timeframe Selection */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Clock className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>{t('timeframes')}</span>
        </div>
        <p className="text-xs mb-3" style={{ color: '#848E9C' }}>{t('timeframesDesc')}</p>

        {/* Timeframe Grid by Category */}
        <div className="space-y-2">
          {(['scalp', 'intraday', 'swing', 'position'] as const).map((category) => {
            const categoryTfs = allTimeframes.filter((tf) => tf.category === category)
            return (
              <div key={category} className="flex items-center gap-2">
                <span
                  className="text-[10px] w-14 flex-shrink-0"
                  style={{ color: categoryColors[category] }}
                >
                  {t(category)}
                </span>
                <div className="flex flex-wrap gap-1">
                  {categoryTfs.map((tf) => {
                    const isSelected = selectedTimeframes.includes(tf.value)
                    const isPrimary = config.klines.primary_timeframe === tf.value
                    return (
                      <div key={tf.value} className="relative">
                        <button
                          onClick={() => toggleTimeframe(tf.value)}
                          onDoubleClick={() => setPrimaryTimeframe(tf.value)}
                          disabled={disabled}
                          className={`px-2.5 py-1 rounded text-xs font-medium transition-all ${
                            isSelected ? 'ring-1' : 'opacity-50 hover:opacity-100'
                          }`}
                          style={{
                            background: isSelected ? `${categoryColors[category]}20` : '#0B0E11',
                            border: `1px solid ${isSelected ? categoryColors[category] : '#2B3139'}`,
                            color: isSelected ? categoryColors[category] : '#848E9C',
                            boxShadow: isPrimary ? `0 0 0 2px ${categoryColors[category]}` : undefined,
                          }}
                          title={isPrimary ? `${tf.label} (${t('primaryTimeframe')})` : tf.label}
                        >
                          {tf.label}
                          {isPrimary && (
                            <span className="ml-1 text-[8px]">★</span>
                          )}
                        </button>
                      </div>
                    )
                  })}
                </div>
              </div>
            )
          })}
        </div>

        <p className="text-[10px] mt-2" style={{ color: '#5E6673' }}>
          {language === 'zh' ? '★ = 主周期 (双击设置)' : '★ = Primary (double-click to set)'}
        </p>

        {/* K-line Count */}
        <div className="mt-3 flex items-center gap-3">
          <span className="text-xs" style={{ color: '#848E9C' }}>{t('klineCount')}:</span>
          <input
            type="number"
            value={config.klines.primary_count}
            onChange={(e) =>
              !disabled &&
              onChange({
                ...config,
                klines: { ...config.klines, primary_count: parseInt(e.target.value) || 30 },
              })
            }
            disabled={disabled}
            min={10}
            max={200}
            className="w-20 px-2 py-1 rounded text-xs"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
        </div>
      </div>

      {/* Technical Indicators */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Activity className="w-4 h-4" style={{ color: '#0ECB81' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>{t('technicalIndicators')}</span>
        </div>

        <div className="grid grid-cols-2 gap-2">
          {indicators.map(({ key, label, color, periodKey }) => (
            <div
              key={key}
              className="flex items-center justify-between p-2 rounded-lg"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full" style={{ background: color }} />
                <span className="text-xs" style={{ color: '#EAECEF' }}>{t(label)}</span>
              </div>
              <div className="flex items-center gap-2">
                {periodKey && config[key as keyof IndicatorConfig] && (
                  <input
                    type="text"
                    value={(config[periodKey as keyof IndicatorConfig] as number[])?.join(',') || ''}
                    onChange={(e) => {
                      if (disabled) return
                      const periods = e.target.value
                        .split(',')
                        .map((s) => parseInt(s.trim()))
                        .filter((n) => !isNaN(n) && n > 0)
                      onChange({ ...config, [periodKey]: periods })
                    }}
                    disabled={disabled}
                    placeholder="7,14"
                    className="w-16 px-1.5 py-0.5 rounded text-[10px] text-center"
                    style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
                  />
                )}
                <input
                  type="checkbox"
                  checked={config[key as keyof IndicatorConfig] as boolean}
                  onChange={(e) =>
                    !disabled && onChange({ ...config, [key]: e.target.checked })
                  }
                  disabled={disabled}
                  className="w-4 h-4 rounded accent-yellow-500"
                />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Quant Data Source */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Database className="w-4 h-4" style={{ color: '#22c55e' }} />
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>{t('quantData')}</span>
        </div>
        <p className="text-xs mb-3" style={{ color: '#848E9C' }}>{t('quantDataDesc')}</p>

        <div
          className="p-3 rounded-lg space-y-3"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          {/* Enable Toggle */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full" style={{ background: '#22c55e' }} />
              <span className="text-xs" style={{ color: '#EAECEF' }}>{t('quantData')}</span>
            </div>
            <input
              type="checkbox"
              checked={config.enable_quant_data || false}
              onChange={(e) =>
                !disabled && onChange({ ...config, enable_quant_data: e.target.checked })
              }
              disabled={disabled}
              className="w-4 h-4 rounded accent-green-500"
            />
          </div>

          {/* API URL */}
          {config.enable_quant_data && (
            <div>
              <label className="text-[10px] mb-1 block" style={{ color: '#848E9C' }}>
                {t('quantDataUrl')} <span style={{ color: '#5E6673' }}>({'{symbol}'} = 币种)</span>
              </label>
              <input
                type="text"
                value={config.quant_data_api_url || ''}
                onChange={(e) =>
                  !disabled && onChange({ ...config, quant_data_api_url: e.target.value })
                }
                disabled={disabled}
                placeholder="http://example.com/api/coin/{symbol}?include=netflow,oi,price"
                className="w-full px-2 py-1.5 rounded text-xs font-mono"
                style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
