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
      className="text-[9px] px-1.5 py-0.5 rounded font-medium bg-purple-500/20 text-purple-400 border border-purple-500/30"
    >
      NofxOS
    </span>
  )

  return (
    <div className="space-y-6">
      {/* Source Type Selector */}
      <div>
        <label className="block text-sm font-medium mb-3 text-nofx-text">
          {t('sourceType')}
        </label>
        <div className="grid grid-cols-4 gap-3">
          {sourceTypes.map(({ value, icon: Icon, color }) => (
            <button
              key={value}
              onClick={() =>
                !disabled &&
                onChange({ ...config, source_type: value as CoinSourceConfig['source_type'] })
              }
              disabled={disabled}
              className={`p-4 rounded-lg border transition-all ${config.source_type === value
                ? 'ring-2 ring-nofx-gold bg-nofx-gold/10'
                : 'hover:bg-white/5 bg-nofx-bg'
                } border-nofx-gold/20`}
            >
              <Icon className="w-6 h-6 mx-auto mb-2" style={{ color }} />
              <div className="text-sm font-medium text-nofx-text">
                {t(value)}
              </div>
              <div className="text-xs mt-1 text-nofx-text-muted">
                {t(`${value}Desc`)}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Static Coins */}
      {(config.source_type === 'static' || config.source_type === 'mixed') && (
        <div>
          <label className="block text-sm font-medium mb-3 text-nofx-text">
            {t('staticCoins')}
          </label>
          <div className="flex flex-wrap gap-2 mb-3">
            {(config.static_coins || []).map((coin) => (
              <span
                key={coin}
                className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm bg-nofx-bg-lighter text-nofx-text"
              >
                {coin}
                {!disabled && (
                  <button
                    onClick={() => handleRemoveCoin(coin)}
                    className="ml-1 hover:text-red-400 transition-colors"
                  >
                    <X className="w-3 h-3" />
                  </button>
                )}
              </span>
            ))}
          </div>
          {!disabled && (
            <div className="flex gap-2">
              <input
                type="text"
                value={newCoin}
                onChange={(e) => setNewCoin(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddCoin()}
                placeholder="BTC, ETH, SOL..."
                className="flex-1 px-4 py-2 rounded-lg bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
              />
              <button
                onClick={handleAddCoin}
                className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors bg-nofx-gold text-black hover:bg-yellow-500"
              >
                <Plus className="w-4 h-4" />
                {t('addCoin')}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Excluded Coins */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <Ban className="w-4 h-4 text-nofx-danger" />
          <label className="text-sm font-medium text-nofx-text">
            {t('excludedCoins')}
          </label>
        </div>
        <p className="text-xs mb-3 text-nofx-text-muted">
          {t('excludedCoinsDesc')}
        </p>
        <div className="flex flex-wrap gap-2 mb-3">
          {(config.excluded_coins || []).map((coin) => (
            <span
              key={coin}
              className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm bg-nofx-danger/15 text-nofx-danger"
            >
              {coin}
              {!disabled && (
                <button
                  onClick={() => handleRemoveExcludedCoin(coin)}
                  className="ml-1 hover:text-white transition-colors"
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </span>
          ))}
          {(config.excluded_coins || []).length === 0 && (
            <span className="text-xs italic text-nofx-text-muted">
              {language === 'zh' ? '无' : 'None'}
            </span>
          )}
        </div>
        {!disabled && (
          <div className="flex gap-2">
            <input
              type="text"
              value={newExcludedCoin}
              onChange={(e) => setNewExcludedCoin(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddExcludedCoin()}
              placeholder="BTC, ETH, DOGE..."
              className="flex-1 px-4 py-2 rounded-lg text-sm bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
            />
            <button
              onClick={handleAddExcludedCoin}
              className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors text-sm bg-nofx-danger text-white hover:bg-red-600"
            >
              <Ban className="w-4 h-4" />
              {t('addExcludedCoin')}
            </button>
          </div>
        )}
      </div>

      {/* AI500 Options */}
      {(config.source_type === 'ai500' || config.source_type === 'mixed') && (
        <div
          className="p-4 rounded-lg bg-nofx-gold/5 border border-nofx-gold/20"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4 text-nofx-gold" />
              <span className="text-sm font-medium text-nofx-text">
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
                className="w-5 h-5 rounded accent-nofx-gold"
              />
              <span className="text-nofx-text">{t('useAI500')}</span>
            </label>

            {config.use_ai500 && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm text-nofx-text-muted">
                  {t('ai500Limit')}:
                </span>
                <select
                  value={config.ai500_limit || 10}
                  onChange={(e) =>
                    !disabled &&
                    onChange({ ...config, ai500_limit: parseInt(e.target.value) || 10 })
                  }
                  disabled={disabled}
                  className="px-3 py-1.5 rounded bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                >
                  {[5, 10, 15, 20, 30, 50].map(n => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted">
              {t('nofxosNote')}
            </p>
          </div>
        </div>
      )}

      {/* OI Top Options */}
      {(config.source_type === 'oi_top' || config.source_type === 'mixed') && (
        <div
          className="p-4 rounded-lg bg-nofx-success/5 border border-nofx-success/20"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-nofx-success" />
              <span className="text-sm font-medium text-nofx-text">
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
                className="w-5 h-5 rounded accent-nofx-success"
              />
              <span className="text-nofx-text">{t('useOITop')}</span>
            </label>

            {config.use_oi_top && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm text-nofx-text-muted">
                  {t('oiTopLimit')}:
                </span>
                <select
                  value={config.oi_top_limit || 20}
                  onChange={(e) =>
                    !disabled &&
                    onChange({ ...config, oi_top_limit: parseInt(e.target.value) || 20 })
                  }
                  disabled={disabled}
                  className="px-3 py-1.5 rounded bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                >
                  {[5, 10, 15, 20, 30, 50].map(n => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted">
              {t('nofxosNote')}
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
