import { useState, useEffect } from 'react'

interface IndicatorConfig {
  indicators: string[]
  timeframes: string[]
  data_points: { [key: string]: number }
  parameters: { [key: string]: number }
}

interface IndicatorConfigPanelProps {
  config?: IndicatorConfig | null
  onConfigChange?: (config: IndicatorConfig) => void
  isEditing?: boolean
}

const AVAILABLE_INDICATORS = [
  { id: 'ema', name: 'EMA', description: '指数移动平均线' },
  { id: 'macd', name: 'MACD', description: '异同移动平均线' },
  { id: 'rsi', name: 'RSI', description: '相对强弱指数' },
  { id: 'atr', name: 'ATR', description: '平均真实波幅' },
  { id: 'volume', name: 'Volume', description: '成交量' },
  { id: 'bollinger', name: 'Bollinger', description: '布林带' },
]

const AVAILABLE_TIMEFRAMES = [
  { id: '1m', name: '1分钟', bars: (n: number) => `${n}分钟` },
  { id: '3m', name: '3分钟', bars: (n: number) => `${(n * 3) / 60}小时` },
  { id: '5m', name: '5分钟', bars: (n: number) => `${(n * 5) / 60}小时` },
  { id: '15m', name: '15分钟', bars: (n: number) => `${(n * 15) / 60}小时` },
  { id: '30m', name: '30分钟', bars: (n: number) => `${(n * 30) / 60}小时` },
  { id: '1h', name: '1小时', bars: (n: number) => `${n}小时` },
  { id: '2h', name: '2小时', bars: (n: number) => `${n * 2}小时` },
  { id: '4h', name: '4小时', bars: (n: number) => `${(n * 4) / 24}天` },
  { id: '6h', name: '6小时', bars: (n: number) => `${(n * 6) / 24}天` },
  { id: '12h', name: '12小时', bars: (n: number) => `${(n * 12) / 24}天` },
  { id: '1d', name: '1天', bars: (n: number) => `${n}天` },
]

const DEFAULT_CONFIG: IndicatorConfig = {
  indicators: ['ema', 'macd', 'rsi', 'atr', 'volume'],
  timeframes: ['3m', '4h'],
  data_points: {
    '3m': 40,
    '4h': 25,
  },
  parameters: {
    rsi_period: 14,
    ema_period: 20,
    macd_fast: 12,
    macd_slow: 26,
    macd_signal: 9,
    atr_period: 14,
  },
}

const PRESETS = {
  conservative: {
    name: '保守型',
    description: '较少指标，较短时间跨度',
    config: {
      indicators: ['ema', 'rsi'],
      timeframes: ['3m', '4h'],
      data_points: { '3m': 30, '4h': 20 },
      parameters: { rsi_period: 14, ema_period: 20 },
    },
  },
  balanced: {
    name: '平衡型',
    description: '中等指标和时间跨度（推荐）',
    config: DEFAULT_CONFIG,
  },
  aggressive: {
    name: '激进型',
    description: '更多指标，更长时间跨度',
    config: {
      indicators: ['ema', 'macd', 'rsi', 'atr', 'volume', 'bollinger'],
      timeframes: ['3m', '15m', '4h'],
      data_points: { '3m': 50, '15m': 40, '4h': 30 },
      parameters: {
        rsi_period: 7,
        ema_period: 20,
        macd_fast: 12,
        macd_slow: 26,
        macd_signal: 9,
        atr_period: 14,
      },
    },
  },
}

export function IndicatorConfigPanel({
  config,
  onConfigChange,
  isEditing = true,
}: IndicatorConfigPanelProps) {
  const [localConfig, setLocalConfig] = useState<IndicatorConfig>(
    config || DEFAULT_CONFIG
  )
  const [showAdvanced, setShowAdvanced] = useState(false)

  useEffect(() => {
    if (config) {
      setLocalConfig(config)
    }
  }, [config])

  const handleIndicatorToggle = (indicatorId: string) => {
    if (!isEditing) return

    const newIndicators = localConfig.indicators.includes(indicatorId)
      ? localConfig.indicators.filter((id) => id !== indicatorId)
      : [...localConfig.indicators, indicatorId]

    const newConfig = { ...localConfig, indicators: newIndicators }
    setLocalConfig(newConfig)
    onConfigChange?.(newConfig)
  }

  const handleTimeframeToggle = (timeframeId: string) => {
    if (!isEditing) return

    const newTimeframes = localConfig.timeframes.includes(timeframeId)
      ? localConfig.timeframes.filter((id) => id !== timeframeId)
      : [...localConfig.timeframes, timeframeId]

    const newConfig = { ...localConfig, timeframes: newTimeframes }
    
    // 如果添加新时间框架，设置默认数据点
    if (!localConfig.timeframes.includes(timeframeId)) {
      newConfig.data_points[timeframeId] = 30
    }
    
    setLocalConfig(newConfig)
    onConfigChange?.(newConfig)
  }

  const handleDataPointsChange = (timeframeId: string, value: number) => {
    if (!isEditing) return

    const newConfig = {
      ...localConfig,
      data_points: { ...localConfig.data_points, [timeframeId]: value },
    }
    setLocalConfig(newConfig)
    onConfigChange?.(newConfig)
  }

  const handleParameterChange = (key: string, value: number) => {
    if (!isEditing) return

    const newConfig = {
      ...localConfig,
      parameters: { ...localConfig.parameters, [key]: value },
    }
    setLocalConfig(newConfig)
    onConfigChange?.(newConfig)
  }

  const applyPreset = (presetKey: keyof typeof PRESETS) => {
    if (!isEditing) return

    const preset = PRESETS[presetKey]
    setLocalConfig(preset.config)
    onConfigChange?.(preset.config)
  }

  const resetToDefault = () => {
    if (!isEditing) return

    setLocalConfig(DEFAULT_CONFIG)
    onConfigChange?.(DEFAULT_CONFIG)
  }

  return (
    <div className="space-y-4">
      {/* Preset Templates */}
      {isEditing && (
        <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-4">
          <h4 className="text-sm font-semibold text-[#EAECEF] mb-3">
            📋 预设模板
          </h4>
          <div className="grid grid-cols-3 gap-2">
            {Object.entries(PRESETS).map(([key, preset]) => (
              <button
                key={key}
                type="button"
                onClick={() => applyPreset(key as keyof typeof PRESETS)}
                className="px-3 py-2 bg-[#1E2329] hover:bg-[#2B3139] border border-[#2B3139] rounded text-sm text-[#EAECEF] transition-colors"
              >
                <div className="font-medium">{preset.name}</div>
                <div className="text-xs text-[#848E9C] mt-1">
                  {preset.description}
                </div>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Indicator Selection */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-4">
        <h4 className="text-sm font-semibold text-[#EAECEF] mb-3">
          📊 技术指标选择
        </h4>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
          {AVAILABLE_INDICATORS.map((indicator) => {
            const isSelected = localConfig.indicators.includes(indicator.id)
            return (
              <button
                key={indicator.id}
                type="button"
                onClick={() => handleIndicatorToggle(indicator.id)}
                disabled={!isEditing}
                className={`px-3 py-2 rounded text-sm transition-colors ${
                  isSelected
                    ? 'bg-[#F0B90B] text-black'
                    : 'bg-[#1E2329] text-[#848E9C] border border-[#2B3139] hover:border-[#F0B90B]'
                } ${!isEditing && 'cursor-not-allowed opacity-60'}`}
              >
                <div className="font-medium">{indicator.name}</div>
                <div className="text-xs mt-0.5 opacity-75">
                  {indicator.description}
                </div>
              </button>
            )
          })}
        </div>
      </div>

      {/* Timeframe Selection */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-4">
        <h4 className="text-sm font-semibold text-[#EAECEF] mb-3">
          ⏱️ 时间框架选择
        </h4>
        <div className="grid grid-cols-3 md:grid-cols-4 gap-2">
          {AVAILABLE_TIMEFRAMES.map((timeframe) => {
            const isSelected = localConfig.timeframes.includes(timeframe.id)
            return (
              <button
                key={timeframe.id}
                type="button"
                onClick={() => handleTimeframeToggle(timeframe.id)}
                disabled={!isEditing}
                className={`px-2 py-1.5 rounded text-xs transition-colors ${
                  isSelected
                    ? 'bg-[#F0B90B] text-black font-medium'
                    : 'bg-[#1E2329] text-[#848E9C] border border-[#2B3139] hover:border-[#F0B90B]'
                } ${!isEditing && 'cursor-not-allowed opacity-60'}`}
              >
                {timeframe.name}
              </button>
            )
          })}
        </div>
      </div>

      {/* Data Points Configuration */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-4">
        <h4 className="text-sm font-semibold text-[#EAECEF] mb-3">
          📈 数据点配置
        </h4>
        <div className="space-y-3">
          {localConfig.timeframes.map((tfId) => {
            const tf = AVAILABLE_TIMEFRAMES.find((t) => t.id === tfId)
            if (!tf) return null

            const dataPoints = localConfig.data_points[tfId] || 30
            const timeSpan = tf.bars(dataPoints)

            return (
              <div key={tfId} className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm text-[#EAECEF]">
                    {tf.name} K线
                  </label>
                  <span className="text-xs text-[#848E9C]">
                    {dataPoints} 根 ≈ {timeSpan}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="10"
                    max="100"
                    step="5"
                    value={dataPoints}
                    onChange={(e) =>
                      handleDataPointsChange(tfId, parseInt(e.target.value))
                    }
                    disabled={!isEditing}
                    className="flex-1 h-2 bg-[#2B3139] rounded-lg appearance-none cursor-pointer slider"
                  />
                  <input
                    type="number"
                    min="10"
                    max="100"
                    value={dataPoints}
                    onChange={(e) =>
                      handleDataPointsChange(tfId, parseInt(e.target.value))
                    }
                    disabled={!isEditing}
                    className="w-16 px-2 py-1 bg-[#1E2329] border border-[#2B3139] rounded text-xs text-[#EAECEF] text-center focus:border-[#F0B90B] focus:outline-none"
                  />
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Advanced Parameters */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-4">
        <button
          type="button"
          onClick={() => setShowAdvanced(!showAdvanced)}
          className="w-full flex items-center justify-between text-sm font-semibold text-[#EAECEF] mb-3"
        >
          <span>⚙️ 高级参数</span>
          <span className="text-xs text-[#848E9C]">
            {showAdvanced ? '▲ 收起' : '▼ 展开'}
          </span>
        </button>

        {showAdvanced && (
          <div className="space-y-3 pt-2">
            {Object.entries(localConfig.parameters).map(([key, value]) => {
              const labels: { [k: string]: string } = {
                rsi_period: 'RSI 周期',
                ema_period: 'EMA 周期',
                macd_fast: 'MACD 快线',
                macd_slow: 'MACD 慢线',
                macd_signal: 'MACD 信号线',
                atr_period: 'ATR 周期',
              }

              return (
                <div key={key} className="flex items-center justify-between">
                  <label className="text-sm text-[#EAECEF]">
                    {labels[key] || key}
                  </label>
                  <input
                    type="number"
                    min="3"
                    max="50"
                    value={value}
                    onChange={(e) =>
                      handleParameterChange(key, parseInt(e.target.value))
                    }
                    disabled={!isEditing}
                    className="w-20 px-2 py-1 bg-[#1E2329] border border-[#2B3139] rounded text-sm text-[#EAECEF] text-center focus:border-[#F0B90B] focus:outline-none"
                  />
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Reset Button */}
      {isEditing && (
        <div className="flex justify-end">
          <button
            type="button"
            onClick={resetToDefault}
            className="px-4 py-2 bg-[#1E2329] hover:bg-[#2B3139] border border-[#2B3139] rounded text-sm text-[#EAECEF] transition-colors"
          >
            🔄 恢复默认配置
          </button>
        </div>
      )}

      <style>{`
        .slider::-webkit-slider-thumb {
          appearance: none;
          width: 16px;
          height: 16px;
          background: #F0B90B;
          cursor: pointer;
          border-radius: 50%;
        }
        .slider::-moz-range-thumb {
          width: 16px;
          height: 16px;
          background: #F0B90B;
          cursor: pointer;
          border-radius: 50%;
          border: none;
        }
        .slider:disabled::-webkit-slider-thumb {
          background: #848E9C;
          cursor: not-allowed;
        }
        .slider:disabled::-moz-range-thumb {
          background: #848E9C;
          cursor: not-allowed;
        }
      `}</style>
    </div>
  )
}
