import { useState } from 'react'
import { Plus, X, Database, TrendingUp, List, Ban, Zap } from 'lucide-react'
import type { CoinSourceConfig } from '../../types'

interface CoinSourceEditorProps {
  config: CoinSourceConfig
  onChange: (config: CoinSourceConfig) => void
  disabled?: boolean
  language: string
}

export function CoinSourceEditor({
  config,
  onChange,
  disabled,
  language,
}: CoinSourceEditorProps) {
  const [newCoin, setNewCoin] = useState('')
  const [newExcludedCoin, setNewExcludedCoin] = useState('')

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      sourceType: { zh: '数据来源类型', en: 'Source Type' },
      static: { zh: '静态列表', en: 'Static List' },
      ai500: { zh: 'AI500 数据源', en: 'AI500 Data Provider' },
      oi_top: { zh: 'OI Top 持仓增长', en: 'OI Top' },
      mixed: { zh: '混合模式', en: 'Mixed Mode' },
      staticCoins: { zh: '自定义币种', en: 'Custom Coins' },
      addCoin: { zh: '添加币种', en: 'Add Coin' },
      useAI500: { zh: '启用 AI500 数据源', en: 'Enable AI500 Data Provider' },
      ai500Limit: { zh: '数量上限', en: 'Limit' },
      useOITop: { zh: '启用 OI Top 数据', en: 'Enable OI Top' },
      oiTopLimit: { zh: '数量上限', en: 'Limit' },
      staticDesc: { zh: '手动指定交易币种列表', en: 'Manually specify trading coins' },
      ai500Desc: {
        zh: '使用 AI500 智能筛选的热门币种',
        en: 'Use AI500 smart-filtered popular coins',
      },
      oiTopDesc: {
        zh: '使用持仓量增长最快的币种',
        en: 'Use coins with fastest OI growth',
      },
      mixedDesc: {
        zh: '组合多种数据源，AI500 + OI Top + 自定义',
        en: 'Combine multiple sources: AI500 + OI Top + Custom',
      },
      dataSourceConfig: { zh: '数据源配置', en: 'Data Source Configuration' },
      excludedCoins: { zh: '排除币种', en: 'Excluded Coins' },
      excludedCoinsDesc: { zh: '这些币种将从所有数据源中排除，不会被交易', en: 'These coins will be excluded from all sources and will not be traded' },
      addExcludedCoin: { zh: '添加排除', en: 'Add Excluded' },
      nofxosNote: { zh: '使用 NofxOS API Key（在指标配置中设置）', en: 'Uses NofxOS API Key (set in Indicators config)' },
    }
    return translations[key]?.[language] || key
  }

  const sourceTypes = [
    { value: 'static', icon: List, color: '#848E9C' },
    { value: 'ai500', icon: Database, color: '#F0B90B' },
    { value: 'oi_top', icon: TrendingUp, color: '#0ECB81' },
    { value: 'mixed', icon: Database, color: '#60a5fa' },
  ] as const

  // xyz dex assets (stocks, forex, commodities) - should NOT get USDT suffix
  const xyzDexAssets = new Set([
    // Stocks
    'TSLA', 'NVDA', 'AAPL', 'MSFT', 'META', 'AMZN', 'GOOGL', 'AMD', 'COIN', 'NFLX',
    'PLTR', 'HOOD', 'INTC', 'MSTR', 'TSM', 'ORCL', 'MU', 'RIVN', 'COST', 'LLY',
    'CRCL', 'SKHX', 'SNDK',
    // Forex
    'EUR', 'JPY',
    // Commodities
    'GOLD', 'SILVER',
    // Index
    'XYZ100',
  ])

  const isXyzDexAsset = (symbol: string): boolean => {
    const base = symbol.toUpperCase().replace(/^XYZ:/, '').replace(/USDT$|USD$|-USDC$/, '')
    return xyzDexAssets.has(base)
  }

  const handleAddCoin = () => {
    if (!newCoin.trim()) return
    const symbol = newCoin.toUpperCase().trim()

    // For xyz dex assets (stocks, forex, commodities), use xyz: prefix without USDT
    let formattedSymbol: string
    if (isXyzDexAsset(symbol)) {
      // Remove xyz: prefix (case-insensitive) and any USD suffixes
      const base = symbol.replace(/^xyz:/i, '').replace(/USDT$|USD$|-USDC$/i, '')
      formattedSymbol = `xyz:${base}`
    } else {
      formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
    }

    const currentCoins = config.static_coins || []
    if (!currentCoins.includes(formattedSymbol)) {
      onChange({
        ...config,
        static_coins: [...currentCoins, formattedSymbol],
      })
    }
    setNewCoin('')
  }

  const handleRemoveCoin = (coin: string) => {
    onChange({
      ...config,
      static_coins: (config.static_coins || []).filter((c) => c !== coin),
    })
  }

  const handleAddExcludedCoin = () => {
    if (!newExcludedCoin.trim()) return
    const symbol = newExcludedCoin.toUpperCase().trim()

    // For xyz dex assets, use xyz: prefix without USDT
    let formattedSymbol: string
    if (isXyzDexAsset(symbol)) {
      const base = symbol.replace(/^xyz:/i, '').replace(/USDT$|USD$|-USDC$/i, '')
      formattedSymbol = `xyz:${base}`
    } else {
      formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
    }

    const currentExcluded = config.excluded_coins || []
    if (!currentExcluded.includes(formattedSymbol)) {
      onChange({
        ...config,
        excluded_coins: [...currentExcluded, formattedSymbol],
      })
    }
    setNewExcludedCoin('')
  }

  const handleRemoveExcludedCoin = (coin: string) => {
    onChange({
      ...config,
      excluded_coins: (config.excluded_coins || []).filter((c) => c !== coin),
    })
  }

  // NofxOS badge component
  const NofxOSBadge = () => (
    <span
      className="text-[9px] px-1.5 py-0.5 rounded font-medium"
      style={{
        background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(168, 85, 247, 0.2))',
        color: '#a855f7',
        border: '1px solid rgba(139, 92, 246, 0.3)'
      }}
    >
      NofxOS
    </span>
  )

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Source Type Selector */}
      <div>
        <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
          {t('sourceType')}
        </label>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3">
          {sourceTypes.map(({ value, icon: Icon, color }) => (
            <button
              key={value}
              onClick={() =>
                !disabled &&
                onChange({ ...config, source_type: value as CoinSourceConfig['source_type'] })
              }
              disabled={disabled}
              className={`p-3 sm:p-4 rounded-lg border transition-all ${
                config.source_type === value
                  ? 'ring-2 ring-yellow-500'
                  : 'hover:bg-white/5'
              }`}
              style={{
                background:
                  config.source_type === value
                    ? 'rgba(240, 185, 11, 0.1)'
                    : '#0B0E11',
                borderColor: '#2B3139',
              }}
            >
              <Icon className="w-5 h-5 sm:w-6 sm:h-6 mx-auto mb-1.5 sm:mb-2" style={{ color }} />
              <div className="text-xs sm:text-sm font-medium" style={{ color: '#EAECEF' }}>
                {t(value)}
              </div>
              <div className="text-[10px] sm:text-xs mt-0.5 sm:mt-1 leading-tight" style={{ color: '#848E9C' }}>
                {t(`${value}Desc`)}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Static Coins */}
      {(config.source_type === 'static' || config.source_type === 'mixed') && (
        <div>
          <label className="block text-xs sm:text-sm font-medium mb-2 sm:mb-3" style={{ color: '#EAECEF' }}>
            {t('staticCoins')}
          </label>
          <div className="flex flex-wrap gap-1.5 sm:gap-2 mb-3">
            {(config.static_coins || []).map((coin) => (
              <span
                key={coin}
                className="flex items-center gap-0.5 sm:gap-1 px-2 sm:px-3 py-1 sm:py-1.5 rounded-full text-xs sm:text-sm"
                style={{ background: '#2B3139', color: '#EAECEF' }}
              >
                <span className="truncate max-w-[120px] sm:max-w-none">{coin}</span>
                {!disabled && (
                  <button
                    onClick={() => handleRemoveCoin(coin)}
                    className="ml-0.5 sm:ml-1 hover:text-red-400 transition-colors flex-shrink-0"
                  >
                    <X className="w-2.5 h-2.5 sm:w-3 sm:h-3" />
                  </button>
                )}
              </span>
            ))}
          </div>
          {!disabled && (
            <div className="flex flex-col sm:flex-row gap-2">
              <input
                type="text"
                value={newCoin}
                onChange={(e) => setNewCoin(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddCoin()}
                placeholder="BTC, ETH, SOL..."
                className="flex-1 px-3 sm:px-4 py-1.5 sm:py-2 rounded-lg text-sm sm:text-base"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <button
                onClick={handleAddCoin}
                className="px-3 sm:px-4 py-1.5 sm:py-2 rounded-lg flex items-center justify-center gap-1.5 sm:gap-2 transition-colors text-sm sm:text-base whitespace-nowrap"
                style={{ background: '#F0B90B', color: '#0B0E11' }}
              >
                <Plus className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
                <span>{t('addCoin')}</span>
              </button>
            </div>
          )}
        </div>
      )}

      {/* Excluded Coins */}
      <div>
        <div className="flex items-center gap-1.5 sm:gap-2 mb-2 sm:mb-3">
          <Ban className="w-3.5 h-3.5 sm:w-4 sm:h-4" style={{ color: '#F6465D' }} />
          <label className="text-xs sm:text-sm font-medium" style={{ color: '#EAECEF' }}>
            {t('excludedCoins')}
          </label>
        </div>
        <p className="text-[10px] sm:text-xs mb-2 sm:mb-3 leading-relaxed" style={{ color: '#848E9C' }}>
          {t('excludedCoinsDesc')}
        </p>
        <div className="flex flex-wrap gap-1.5 sm:gap-2 mb-3">
          {(config.excluded_coins || []).map((coin) => (
            <span
              key={coin}
              className="flex items-center gap-0.5 sm:gap-1 px-2 sm:px-3 py-1 sm:py-1.5 rounded-full text-xs sm:text-sm"
              style={{ background: 'rgba(246, 70, 93, 0.15)', color: '#F6465D' }}
            >
              <span className="truncate max-w-[120px] sm:max-w-none">{coin}</span>
              {!disabled && (
                <button
                  onClick={() => handleRemoveExcludedCoin(coin)}
                  className="ml-0.5 sm:ml-1 hover:text-white transition-colors flex-shrink-0"
                >
                  <X className="w-2.5 h-2.5 sm:w-3 sm:h-3" />
                </button>
              )}
            </span>
          ))}
          {(config.excluded_coins || []).length === 0 && (
            <span className="text-xs italic" style={{ color: '#5E6673' }}>
              {language === 'zh' ? '无' : 'None'}
            </span>
          )}
        </div>
        {!disabled && (
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              value={newExcludedCoin}
              onChange={(e) => setNewExcludedCoin(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddExcludedCoin()}
              placeholder="BTC, ETH, DOGE..."
              className="flex-1 px-3 sm:px-4 py-1.5 sm:py-2 rounded-lg text-sm sm:text-base"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <button
              onClick={handleAddExcludedCoin}
              className="px-3 sm:px-4 py-1.5 sm:py-2 rounded-lg flex items-center justify-center gap-1.5 sm:gap-2 transition-colors text-sm sm:text-base whitespace-nowrap"
              style={{ background: '#F6465D', color: '#EAECEF' }}
            >
              <Ban className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
              <span>{t('addExcludedCoin')}</span>
            </button>
          </div>
        )}
      </div>

      {/* AI500 Options */}
      {(config.source_type === 'ai500' || config.source_type === 'mixed') && (
        <div
          className="p-4 rounded-lg"
          style={{
            background: 'rgba(240, 185, 11, 0.05)',
            border: '1px solid rgba(240, 185, 11, 0.2)',
          }}
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4" style={{ color: '#F0B90B' }} />
              <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                AI500 {t('dataSourceConfig')}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_ai500}
                onChange={(e) =>
                  !disabled && onChange({ ...config, use_ai500: e.target.checked })
                }
                disabled={disabled}
                className="w-5 h-5 rounded accent-yellow-500"
              />
              <span style={{ color: '#EAECEF' }}>{t('useAI500')}</span>
            </label>

            {config.use_ai500 && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm" style={{ color: '#848E9C' }}>
                  {t('ai500Limit')}:
                </span>
                <select
                  value={config.ai500_limit || 10}
                  onChange={(e) =>
                    !disabled &&
                    onChange({ ...config, ai500_limit: parseInt(e.target.value) || 10 })
                  }
                  disabled={disabled}
                  className="px-3 py-1.5 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                >
                  {[5, 10, 15, 20, 30, 50].map(n => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
              </div>
            )}

            <p className="text-xs pl-8" style={{ color: '#5E6673' }}>
              {t('nofxosNote')}
            </p>
          </div>
        </div>
      )}

      {/* OI Top Options */}
      {(config.source_type === 'oi_top' || config.source_type === 'mixed') && (
        <div
          className="p-4 rounded-lg"
          style={{
            background: 'rgba(14, 203, 129, 0.05)',
            border: '1px solid rgba(14, 203, 129, 0.2)',
          }}
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4" style={{ color: '#0ECB81' }} />
              <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                OI Top {t('dataSourceConfig')}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_oi_top}
                onChange={(e) =>
                  !disabled && onChange({ ...config, use_oi_top: e.target.checked })
                }
                disabled={disabled}
                className="w-5 h-5 rounded accent-green-500"
              />
              <span style={{ color: '#EAECEF' }}>{t('useOITop')}</span>
            </label>

            {config.use_oi_top && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm" style={{ color: '#848E9C' }}>
                  {t('oiTopLimit')}:
                </span>
                <select
                  value={config.oi_top_limit || 20}
                  onChange={(e) =>
                    !disabled &&
                    onChange({ ...config, oi_top_limit: parseInt(e.target.value) || 20 })
                  }
                  disabled={disabled}
                  className="px-3 py-1.5 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                >
                  {[5, 10, 15, 20, 30, 50].map(n => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
              </div>
            )}

            <p className="text-xs pl-8" style={{ color: '#5E6673' }}>
              {t('nofxosNote')}
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
