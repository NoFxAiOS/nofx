import { useState } from 'react'
import { Plus, X, Database, TrendingUp, TrendingDown, List, Ban, Zap, Shuffle } from 'lucide-react'
import type { CoinSourceConfig } from '../../types'
import { coinSource, ts } from '../../i18n/strategy-translations'
import { NofxSelect } from '../ui/select'

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

  const sourceTypes = [
    { value: 'static', icon: List, color: '#848E9C' },
    { value: 'ai500', icon: Database, color: '#F0B90B' },
    { value: 'oi_top', icon: TrendingUp, color: '#0ECB81' },
    { value: 'oi_low', icon: TrendingDown, color: '#F6465D' },
  ] as const

  // Calculate mixed mode summary
  const getMixedSummary = () => {
    const sources: string[] = []
    let totalLimit = 0

    if (config.use_ai500) {
      sources.push(`AI500(${config.ai500_limit || 10})`)
      totalLimit += config.ai500_limit || 10
    }
    if (config.use_oi_top) {
      sources.push(`${ts(coinSource.oiIncreaseShort, language)}(${config.oi_top_limit || 10})`)
      totalLimit += config.oi_top_limit || 10
    }
    if (config.use_oi_low) {
      sources.push(`${ts(coinSource.oiDecreaseShort, language)}(${config.oi_low_limit || 10})`)
      totalLimit += config.oi_low_limit || 10
    }
    if ((config.static_coins || []).length > 0) {
      sources.push(`${ts(coinSource.custom, language)}(${config.static_coins?.length || 0})`)
      totalLimit += config.static_coins?.length || 0
    }

    return { sources, totalLimit }
  }

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

  const MAX_STATIC_COINS = 3

  const showToast = (msg: string) => {
    const toast = document.createElement('div')
    toast.textContent = msg
    toast.className = 'fixed top-4 left-1/2 -translate-x-1/2 px-4 py-2 rounded-lg text-sm z-50 shadow-lg'
    toast.style.cssText = 'background:#F6465D;color:#fff;'
    document.body.appendChild(toast)
    setTimeout(() => toast.remove(), 2000)
  }

  const handleAddCoin = () => {
    if (!newCoin.trim()) return

    const currentCoins = config.static_coins || []
    if (currentCoins.length >= MAX_STATIC_COINS) {
      showToast(language === 'zh' ? `最多添加 ${MAX_STATIC_COINS} 个币种` : `Maximum ${MAX_STATIC_COINS} coins allowed`)
      return
    }

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
          {ts(coinSource.sourceType, language)}
        </label>
        <div className="grid grid-cols-4 gap-2">
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
                {ts(coinSource[value as keyof typeof coinSource], language)}
              </div>
              <div className="text-xs mt-1 text-nofx-text-muted">
                {ts(coinSource[`${value}Desc` as keyof typeof coinSource], language)}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Static Coins - only for static mode */}
      {config.source_type === 'static' && (
        <div>
          <label className="block text-sm font-medium mb-3 text-nofx-text">
            {ts(coinSource.staticCoins, language)}
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
                {ts(coinSource.addCoin, language)}
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
            {ts(coinSource.excludedCoins, language)}
          </label>
        </div>
        <p className="text-xs mb-3 text-nofx-text-muted">
          {ts(coinSource.excludedCoinsDesc, language)}
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
              {ts(coinSource.excludedNone, language)}
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
              {ts(coinSource.addExcludedCoin, language)}
            </button>
          </div>
        )}
      </div>

      {/* AI500 Options - only for ai500 mode */}
      {config.source_type === 'ai500' && (
        <div
          className="p-4 rounded-lg bg-nofx-gold/5 border border-nofx-gold/20"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4 text-nofx-gold" />
              <span className="text-sm font-medium text-nofx-text">
                AI500 {ts(coinSource.dataSourceConfig, language)}
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
              <span className="text-nofx-text">{ts(coinSource.useAI500, language)}</span>
            </label>

            {config.use_ai500 && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm text-nofx-text-muted">
                  {ts(coinSource.ai500Limit, language)}:
                </span>
                <NofxSelect
                  value={config.ai500_limit || 10}
                  onChange={(val) =>
                    !disabled &&
                    onChange({ ...config, ai500_limit: parseInt(val) || 10 })
                  }
                  disabled={disabled}
                  options={[1, 2, 3].map(n => ({ value: n, label: String(n) }))}
                  className="px-3 py-1.5 rounded bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                />
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* OI Top Options - only for oi_top mode */}
      {config.source_type === 'oi_top' && (
        <div
          className="p-4 rounded-lg bg-nofx-success/5 border border-nofx-success/20"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-nofx-success" />
              <span className="text-sm font-medium text-nofx-text">
                {ts(coinSource.oiIncreaseTitle, language)} {ts(coinSource.dataSourceConfig, language)}
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
              <span className="text-nofx-text">{ts(coinSource.useOITop, language)}</span>
            </label>

            {config.use_oi_top && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm text-nofx-text-muted">
                  {ts(coinSource.oiTopLimit, language)}:
                </span>
                <NofxSelect
                  value={config.oi_top_limit || 10}
                  onChange={(val) =>
                    !disabled &&
                    onChange({ ...config, oi_top_limit: parseInt(val) || 10 })
                  }
                  disabled={disabled}
                  options={[1, 2, 3].map(n => ({ value: n, label: String(n) }))}
                  className="px-3 py-1.5 rounded bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                />
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* OI Low Options - only for oi_low mode */}
      {config.source_type === 'oi_low' && (
        <div
          className="p-4 rounded-lg bg-nofx-danger/5 border border-nofx-danger/20"
        >
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingDown className="w-4 h-4 text-nofx-danger" />
              <span className="text-sm font-medium text-nofx-text">
                {ts(coinSource.oiDecreaseTitle, language)} {ts(coinSource.dataSourceConfig, language)}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_oi_low}
                onChange={(e) =>
                  !disabled && onChange({ ...config, use_oi_low: e.target.checked })
                }
                disabled={disabled}
                className="w-5 h-5 rounded accent-red-500"
              />
              <span className="text-nofx-text">{ts(coinSource.useOILow, language)}</span>
            </label>

            {config.use_oi_low && (
              <div className="flex items-center gap-3 pl-8">
                <span className="text-sm text-nofx-text-muted">
                  {ts(coinSource.oiLowLimit, language)}:
                </span>
                <NofxSelect
                  value={config.oi_low_limit || 10}
                  onChange={(val) =>
                    !disabled &&
                    onChange({ ...config, oi_low_limit: parseInt(val) || 10 })
                  }
                  disabled={disabled}
                  options={[1, 2, 3].map(n => ({ value: n, label: String(n) }))}
                  className="px-3 py-1.5 rounded bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                />
              </div>
            )}

            <p className="text-xs pl-8 text-nofx-text-muted">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* Mixed Mode - Unified Card Selector */}
      {config.source_type === 'mixed' && (
        <div className="p-4 rounded-lg bg-blue-500/5 border border-blue-500/20">
          <div className="flex items-center gap-2 mb-4">
            <Shuffle className="w-4 h-4 text-blue-400" />
            <span className="text-sm font-medium text-nofx-text">
              {ts(coinSource.mixedConfig, language)}
            </span>
          </div>

          {/* 4 Source Cards in 2x2 Grid */}
          <div className="grid grid-cols-2 gap-3 mb-4">
            {/* AI500 Card */}
            <div
              className={`p-3 rounded-lg border transition-all cursor-pointer ${
                config.use_ai500
                  ? 'bg-nofx-gold/10 border-nofx-gold/50'
                  : 'bg-nofx-bg border-nofx-border hover:border-nofx-gold/30'
              }`}
              onClick={() => !disabled && onChange({ ...config, use_ai500: !config.use_ai500 })}
            >
              <div className="flex items-center gap-2 mb-2">
                <input
                  type="checkbox"
                  checked={config.use_ai500}
                  onChange={(e) => !disabled && onChange({ ...config, use_ai500: e.target.checked })}
                  disabled={disabled}
                  className="w-4 h-4 rounded accent-nofx-gold"
                  onClick={(e) => e.stopPropagation()}
                />
                <Database className="w-4 h-4 text-nofx-gold" />
                <span className="text-sm font-medium text-nofx-text">AI500</span>
                <NofxOSBadge />
              </div>
              {config.use_ai500 && (
                <div className="flex items-center gap-2 mt-2 pl-6">
                  <span className="text-xs text-nofx-text-muted">Limit:</span>
                  <NofxSelect
                    value={config.ai500_limit || 10}
                    onChange={(val) => !disabled && onChange({ ...config, ai500_limit: parseInt(val) || 10 })}
                    disabled={disabled}
                    options={[5, 10, 15, 20, 30, 50].map(n => ({ value: n, label: String(n) }))}
                    className="px-2 py-1 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                  />
                </div>
              )}
            </div>

            {/* OI Top Card */}
            <div
              className={`p-3 rounded-lg border transition-all cursor-pointer ${
                config.use_oi_top
                  ? 'bg-nofx-success/10 border-nofx-success/50'
                  : 'bg-nofx-bg border-nofx-border hover:border-nofx-success/30'
              }`}
              onClick={() => !disabled && onChange({ ...config, use_oi_top: !config.use_oi_top })}
            >
              <div className="flex items-center gap-2 mb-2">
                <input
                  type="checkbox"
                  checked={config.use_oi_top}
                  onChange={(e) => !disabled && onChange({ ...config, use_oi_top: e.target.checked })}
                  disabled={disabled}
                  className="w-4 h-4 rounded accent-nofx-success"
                  onClick={(e) => e.stopPropagation()}
                />
                <TrendingUp className="w-4 h-4 text-nofx-success" />
                <span className="text-sm font-medium text-nofx-text">
                  {ts(coinSource.oiIncreaseLabel, language)}
                </span>
              </div>
              <p className="text-xs text-nofx-text-muted pl-6 mb-1">
                {ts(coinSource.forLong, language)}
              </p>
              {config.use_oi_top && (
                <div className="flex items-center gap-2 mt-2 pl-6">
                  <span className="text-xs text-nofx-text-muted">Limit:</span>
                  <NofxSelect
                    value={config.oi_top_limit || 10}
                    onChange={(val) => !disabled && onChange({ ...config, oi_top_limit: parseInt(val) || 10 })}
                    disabled={disabled}
                    options={[5, 10, 15, 20, 30, 50].map(n => ({ value: n, label: String(n) }))}
                    className="px-2 py-1 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                  />
                </div>
              )}
            </div>

            {/* OI Low Card */}
            <div
              className={`p-3 rounded-lg border transition-all cursor-pointer ${
                config.use_oi_low
                  ? 'bg-nofx-danger/10 border-nofx-danger/50'
                  : 'bg-nofx-bg border-nofx-border hover:border-nofx-danger/30'
              }`}
              onClick={() => !disabled && onChange({ ...config, use_oi_low: !config.use_oi_low })}
            >
              <div className="flex items-center gap-2 mb-2">
                <input
                  type="checkbox"
                  checked={config.use_oi_low}
                  onChange={(e) => !disabled && onChange({ ...config, use_oi_low: e.target.checked })}
                  disabled={disabled}
                  className="w-4 h-4 rounded accent-red-500"
                  onClick={(e) => e.stopPropagation()}
                />
                <TrendingDown className="w-4 h-4 text-nofx-danger" />
                <span className="text-sm font-medium text-nofx-text">
                  {ts(coinSource.oiDecreaseLabel, language)}
                </span>
              </div>
              <p className="text-xs text-nofx-text-muted pl-6 mb-1">
                {ts(coinSource.forShort, language)}
              </p>
              {config.use_oi_low && (
                <div className="flex items-center gap-2 mt-2 pl-6">
                  <span className="text-xs text-nofx-text-muted">Limit:</span>
                  <NofxSelect
                    value={config.oi_low_limit || 10}
                    onChange={(val) => !disabled && onChange({ ...config, oi_low_limit: parseInt(val) || 10 })}
                    disabled={disabled}
                    options={[5, 10, 15, 20, 30, 50].map(n => ({ value: n, label: String(n) }))}
                    className="px-2 py-1 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                  />
                </div>
              )}
            </div>

            {/* Static/Custom Card */}
            <div
              className={`p-3 rounded-lg border transition-all cursor-pointer ${
                (config.static_coins || []).length > 0
                  ? 'bg-gray-500/10 border-gray-500/50'
                  : 'bg-nofx-bg border-nofx-border hover:border-gray-500/30'
              }`}
            >
              <div className="flex items-center gap-2 mb-2">
                <List className="w-4 h-4 text-gray-400" />
                <span className="text-sm font-medium text-nofx-text">
                  {ts(coinSource.custom, language)}
                </span>
                {(config.static_coins || []).length > 0 && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-gray-500/20 text-gray-400">
                    {config.static_coins?.length}
                  </span>
                )}
              </div>
              <div className="flex flex-wrap gap-1 mt-2">
                {(config.static_coins || []).slice(0, 3).map((coin) => (
                  <span
                    key={coin}
                    className="flex items-center gap-1 px-2 py-0.5 rounded text-xs bg-nofx-bg-lighter text-nofx-text"
                  >
                    {coin}
                    {!disabled && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          handleRemoveCoin(coin)
                        }}
                        className="hover:text-red-400 transition-colors"
                      >
                        <X className="w-2.5 h-2.5" />
                      </button>
                    )}
                  </span>
                ))}
                {(config.static_coins || []).length > 3 && (
                  <span className="text-xs text-nofx-text-muted">
                    +{(config.static_coins?.length || 0) - 3}
                  </span>
                )}
              </div>
              {!disabled && (
                <div className="flex gap-1 mt-2">
                  <input
                    type="text"
                    value={newCoin}
                    onChange={(e) => setNewCoin(e.target.value)}
                    onKeyDown={(e) => {
                      e.stopPropagation()
                      if (e.key === 'Enter') handleAddCoin()
                    }}
                    onClick={(e) => e.stopPropagation()}
                    placeholder="BTC, ETH..."
                    className="flex-1 px-2 py-1 rounded text-xs bg-nofx-bg border border-nofx-gold/20 text-nofx-text"
                  />
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      handleAddCoin()
                    }}
                    className="px-2 py-1 rounded text-xs bg-nofx-gold text-black hover:bg-yellow-500"
                  >
                    <Plus className="w-3 h-3" />
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Summary */}
          {(() => {
            const { sources, totalLimit } = getMixedSummary()
            if (sources.length === 0) return null
            return (
              <div className="p-2 rounded bg-nofx-bg border border-nofx-border">
                <div className="flex items-center justify-between text-xs">
                  <span className="text-nofx-text-muted">{ts(coinSource.mixedSummary, language)}:</span>
                  <span className="text-nofx-text font-medium">
                    {sources.join(' + ')}
                  </span>
                </div>
                <div className="text-xs text-nofx-text-muted mt-1">
                  {ts(coinSource.maxCoins, language)} {totalLimit} {ts(coinSource.coins, language)}
                </div>
              </div>
            )
          })()}
        </div>
      )}
    </div>
  )
}
