import { cn } from '../../lib/cn'
import { Button, Input, Badge } from '../ui'
import { useState } from 'react'
import {
  Plus,
  X,
  Database,
  TrendingUp,
  TrendingDown,
  List,
  Ban,
  Zap,
  Shuffle,
  ArrowDownRight,
} from 'lucide-react'
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
    { value: 'static', icon: List, color: 'text-fg-3' },
    { value: 'ai500', icon: Database, color: 'text-brand' },
    { value: 'oi_top', icon: TrendingUp, color: 'text-up' },
    { value: 'oi_low', icon: TrendingDown, color: 'text-down' },
    { value: 'piggy_dash', icon: Zap, color: 'text-ai' },
    { value: 'short_scan', icon: ArrowDownRight, color: 'text-down' },
  ] as const

  type SourceKey = (typeof sourceTypes)[number]['value']

  // The set of sources currently selected, derived from the config. A single
  // source maps to its own source_type; 2+ map to mixed + use_* flags.
  const getSelectedSources = (): SourceKey[] => {
    if (config.source_type === 'mixed') {
      const s: SourceKey[] = []
      if (config.use_ai500) s.push('ai500')
      if (config.use_oi_top) s.push('oi_top')
      if (config.use_oi_low) s.push('oi_low')
      if (config.use_piggy_dash) s.push('piggy_dash')
      if (config.use_short_scan) s.push('short_scan')
      // Legacy mixed configs (no use_static flag) always included static coins
      if (config.use_static !== false) s.push('static')
      return s.length > 0 ? s : ['static']
    }
    if (sourceTypes.some((t) => t.value === config.source_type)) {
      return [config.source_type as SourceKey]
    }
    return []
  }

  // Map a source selection back to the persisted config shape: one source
  // keeps its dedicated source_type, several combine into mixed mode.
  const buildConfig = (sources: SourceKey[]): CoinSourceConfig => {
    const base = { ...config }
    if (sources.length === 1) {
      const t = sources[0]
      return {
        ...base,
        source_type: t as CoinSourceConfig['source_type'],
        use_static: undefined,
        use_ai500: t === 'ai500',
        use_oi_top: t === 'oi_top',
        use_oi_low: t === 'oi_low',
        use_piggy_dash: t === 'piggy_dash',
        use_short_scan: t === 'short_scan',
      }
    }
    return {
      ...base,
      source_type: 'mixed',
      use_static: sources.includes('static'),
      use_ai500: sources.includes('ai500'),
      use_oi_top: sources.includes('oi_top'),
      use_oi_low: sources.includes('oi_low'),
      use_piggy_dash: sources.includes('piggy_dash'),
      use_short_scan: sources.includes('short_scan'),
    }
  }

  const selectedSources = getSelectedSources()
  const isMixed = config.source_type === 'mixed'

  const toggleSource = (value: SourceKey) => {
    if (disabled) return
    let next: SourceKey[]
    if (selectedSources.includes(value)) {
      // Keep at least one source selected
      if (selectedSources.length === 1) return
      next = selectedSources.filter((s) => s !== value)
    } else {
      next = [...selectedSources, value]
    }
    onChange(buildConfig(next))
  }

  // Calculate mixed mode summary
  const getMixedSummary = () => {
    const sources: string[] = []
    let totalLimit = 0

    if (config.use_ai500) {
      sources.push(`AI500(${config.ai500_limit || 3})`)
      totalLimit += config.ai500_limit || 3
    }
    if (config.use_oi_top) {
      sources.push(
        `${ts(coinSource.oiIncreaseShort, language)}(${config.oi_top_limit || 3})`
      )
      totalLimit += config.oi_top_limit || 3
    }
    if (config.use_oi_low) {
      sources.push(
        `${ts(coinSource.oiDecreaseShort, language)}(${config.oi_low_limit || 3})`
      )
      totalLimit += config.oi_low_limit || 3
    }
    if (config.use_piggy_dash) {
      sources.push(
        `${ts(coinSource.piggyDash, language)}(${config.piggy_dash_limit || 5})`
      )
      totalLimit += config.piggy_dash_limit || 5
    }
    if (config.use_short_scan) {
      sources.push(
        `${ts(coinSource.shortScan, language)}(${config.short_scan_limit || 5})`
      )
      totalLimit += config.short_scan_limit || 5
    }
    if (config.use_static !== false && (config.static_coins || []).length > 0) {
      sources.push(
        `${ts(coinSource.custom, language)}(${config.static_coins?.length || 0})`
      )
      totalLimit += config.static_coins?.length || 0
    }

    return { sources, totalLimit }
  }

  // xyz dex assets (stocks, forex, commodities) - should NOT get USDT suffix
  const xyzDexAssets = new Set([
    // Stocks
    'TSLA',
    'NVDA',
    'AAPL',
    'MSFT',
    'META',
    'AMZN',
    'GOOGL',
    'AMD',
    'COIN',
    'NFLX',
    'PLTR',
    'HOOD',
    'INTC',
    'MSTR',
    'TSM',
    'ORCL',
    'MU',
    'RIVN',
    'COST',
    'LLY',
    'CRCL',
    'SKHX',
    'SNDK',
    // Forex
    'EUR',
    'JPY',
    // Commodities
    'GOLD',
    'SILVER',
    // Index
    'XYZ100',
  ])

  const isXyzDexAsset = (symbol: string): boolean => {
    const base = symbol
      .toUpperCase()
      .replace(/^XYZ:/, '')
      .replace(/USDT$|USD$|-USDC$/, '')
    return xyzDexAssets.has(base)
  }

  const MAX_STATIC_COINS = 10

  const showToast = (msg: string) => {
    const toast = document.createElement('div')
    toast.textContent = msg
    toast.className =
      'fixed top-4 left-1/2 -translate-x-1/2 px-4 py-2 rounded-lg text-sm z-50 shadow-pop border border-down/30 bg-down-soft text-down'
    document.body.appendChild(toast)
    setTimeout(() => toast.remove(), 2000)
  }

  const handleAddCoin = () => {
    if (!newCoin.trim()) return

    const currentCoins = config.static_coins || []
    if (currentCoins.length >= MAX_STATIC_COINS) {
      showToast(
        language === 'zh'
          ? `最多添加 ${MAX_STATIC_COINS} 个币种`
          : `Maximum ${MAX_STATIC_COINS} coins allowed`
      )
      return
    }

    const symbol = newCoin.toUpperCase().trim()

    // For xyz dex assets (stocks, forex, commodities), use xyz: prefix without USDT
    let formattedSymbol: string
    if (isXyzDexAsset(symbol)) {
      // Remove xyz: prefix (case-insensitive) and any USD suffixes
      const base = symbol
        .replace(/^xyz:/i, '')
        .replace(/USDT$|USD$|-USDC$/i, '')
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
      const base = symbol
        .replace(/^xyz:/i, '')
        .replace(/USDT$|USD$|-USDC$/i, '')
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
    <Badge variant="ai" size="xs">
      NofxOS
    </Badge>
  )

  return (
    <div className="space-y-4">
      {/* Source Type Multi-Selector */}
      <div>
        <label className="block text-[13px] font-medium mb-1 text-fg-2">
          {ts(coinSource.sourceType, language)}
        </label>
        <p className="text-xs mb-3 text-fg-3">
          {ts(coinSource.multiSelectHint, language)}
        </p>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 2xl:grid-cols-6">
          {sourceTypes.map(({ value, icon: Icon, color }) => {
            const selected = selectedSources.includes(value)
            return (
              <button
                key={value}
                onClick={() => toggleSource(value)}
                disabled={disabled}
                aria-pressed={selected}
                className={cn(
                  `relative p-3 rounded-md border transition-colors ${
                    selected
                      ? 'border-brand bg-brand-soft'
                      : 'hover:bg-surface-hover bg-bg'
                  } border-line`,
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50'
                )}
              >
                {selected && (
                  <span className="absolute top-1.5 right-1.5 w-4 h-4 rounded-full bg-brand text-brand-fg text-xs font-bold flex items-center justify-center">
                    ✓
                  </span>
                )}
                <Icon className={cn('w-5 h-5 mx-auto mb-2', color)} />
                <div className="text-sm font-medium text-fg">
                  {ts(coinSource[value as keyof typeof coinSource], language)}
                </div>
                <div className="text-xs mt-1 text-fg-3">
                  {ts(
                    coinSource[`${value}Desc` as keyof typeof coinSource],
                    language
                  )}
                </div>
              </button>
            )
          })}
        </div>
        {isMixed && (
          <div className="mt-2 text-xs font-medium text-info">
            {ts(coinSource.mixedMode, language).replace(
              '{n}',
              String(selectedSources.length)
            )}
          </div>
        )}
      </div>

      {/* Static Coins - when the static source is selected */}
      {selectedSources.includes('static') && (
        <div>
          <label className="block text-[13px] font-medium mb-3 text-fg-2">
            {ts(coinSource.staticCoins, language)}
          </label>
          <div className="flex flex-wrap gap-2 mb-3">
            {(config.static_coins || []).map((coin) => (
              <span
                key={coin}
                className="flex max-w-full items-center gap-1 rounded-md bg-surface-2 pl-2 text-xs text-fg"
              >
                {coin}
                {!disabled && (
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => handleRemoveCoin(coin)}
                    className="w-7 px-0"
                  >
                    <X className="w-3 h-3" />
                  </Button>
                )}
              </span>
            ))}
          </div>
          {!disabled && (
            <div className="flex gap-2">
              <Input
                type="text"
                value={newCoin}
                onChange={(e) => setNewCoin(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddCoin()}
                placeholder="BTC, ETH, SOL..."
                className="flex-1 px-4"
              />
              <Button
                variant="primary"
                onClick={handleAddCoin}
                className="flex items-center gap-2 text-brand-fg"
              >
                <Plus className="w-4 h-4" />
                {ts(coinSource.addCoin, language)}
              </Button>
            </div>
          )}
        </div>
      )}

      {/* Excluded Coins */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <Ban className="w-4 h-4 text-down" />
          <label className="text-[13px] font-medium text-fg-2">
            {ts(coinSource.excludedCoins, language)}
          </label>
        </div>
        <p className="text-xs mb-3 text-fg-3">
          {ts(coinSource.excludedCoinsDesc, language)}
        </p>
        <div className="flex flex-wrap gap-2 mb-3">
          {(config.excluded_coins || []).map((coin) => (
            <span
              key={coin}
              className="flex max-w-full items-center gap-1 rounded-md bg-down-soft pl-2 text-xs text-down"
            >
              {coin}
              {!disabled && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleRemoveExcludedCoin(coin)}
                  className="w-7 px-0"
                >
                  <X className="w-3 h-3" />
                </Button>
              )}
            </span>
          ))}
          {(config.excluded_coins || []).length === 0 && (
            <span className="text-xs italic text-fg-3">
              {ts(coinSource.excludedNone, language)}
            </span>
          )}
        </div>
        {!disabled && (
          <div className="flex gap-2">
            <Input
              type="text"
              value={newExcludedCoin}
              onChange={(e) => setNewExcludedCoin(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddExcludedCoin()}
              placeholder="BTC, ETH, DOGE..."
              className="flex-1 px-4"
            />
            <Button
              variant="secondary"
              onClick={handleAddExcludedCoin}
              className="flex items-center gap-2 text-fg"
            >
              <Ban className="w-4 h-4" />
              {ts(coinSource.addExcludedCoin, language)}
            </Button>
          </div>
        )}
      </div>

      {/* Min OI Value Filter — applies to all sources */}
      <div>
        <label className="text-[13px] font-medium text-fg-2">
          {ts(coinSource.minOILabel, language)}
        </label>
        <p className="text-xs mb-2 text-fg-3">
          {ts(coinSource.minOIDesc, language)}
        </p>
        <div className="flex items-center gap-2">
          <Input
            type="number"
            min={0}
            max={1000}
            step={0.5}
            value={config.min_oi_value_millions ?? 15}
            onChange={(e) =>
              onChange({
                ...config,
                min_oi_value_millions: Math.max(
                  0,
                  Math.min(1000, Number(e.target.value) || 0)
                ),
              })
            }
            disabled={disabled}
            className="w-28 px-3 num text-right"
          />
          <span className="text-xs text-fg-3">M USD</span>
          {(config.min_oi_value_millions ?? 15) === 0 && (
            <span className="text-xs text-fg-3">
              ({ts(coinSource.minOIDefault, language)})
            </span>
          )}
        </div>
      </div>

      {/* AI500 Options - when the ai500 source is selected */}
      {selectedSources.includes('ai500') && (
        <div className="p-4 rounded-lg bg-brand/5 border border-line">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap className="w-4 h-4 text-brand" />
              <span className="text-sm font-medium text-fg">
                AI500 {ts(coinSource.dataSourceConfig, language)}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center gap-3 pl-2">
              <span className="text-sm text-fg-3">
                {ts(coinSource.ai500Limit, language)}:
              </span>
              <NofxSelect
                value={config.ai500_limit || 3}
                onChange={(val) =>
                  !disabled &&
                  onChange({ ...config, ai500_limit: parseInt(val) || 3 })
                }
                disabled={disabled}
                options={[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((n) => ({
                  value: n,
                  label: String(n),
                }))}
                className="px-3 border border-line text-fg h-8 rounded-md bg-surface-2 text-[13px] hover:border-line-strong"
              />
            </div>

            <p className="text-xs pl-2 text-fg-3">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* Piggy Dash Options - when the piggy_dash source is selected */}
      {selectedSources.includes('piggy_dash') && (
        <div className="space-y-4 p-4 rounded-lg bg-bg border border-line">
          <div className="flex items-center gap-2">
            <Zap className="w-4 h-4 text-ai" />
            <span className="text-sm font-medium text-fg">
              {ts(coinSource.piggyDashTitle, language)}
            </span>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.piggyDashLimit, language)}
            </label>
            <Input
              type="number"
              min={1}
              max={30}
              value={config.piggy_dash_limit || 5}
              onChange={(e) =>
                onChange({
                  ...config,
                  piggy_dash_limit: Math.max(
                    1,
                    Math.min(30, Number(e.target.value) || 5)
                  ),
                })
              }
              disabled={disabled}
              className="w-24 px-3 num text-right"
            />
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.piggyDashDirection, language)}
            </label>
            <div className="flex gap-2">
              {[
                { value: '', label: coinSource.piggyDashAll },
                { value: 'breakout', label: coinSource.piggyDashOnlyBreakout },
                {
                  value: 'breakdown',
                  label: coinSource.piggyDashOnlyBreakdown,
                },
              ].map(({ value, label }) => (
                <button
                  key={value || 'all'}
                  onClick={() =>
                    !disabled &&
                    onChange({ ...config, piggy_dash_direction: value })
                  }
                  disabled={disabled}
                  className={cn(
                    `px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      (config.piggy_dash_direction || '') === value
                        ? 'bg-brand/15 text-brand border border-brand/40'
                        : 'bg-bg text-fg-3 border border-brand/10 hover:bg-surface-2'
                    }`,
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50'
                  )}
                >
                  {ts(label, language)}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.piggyDashMinGrade, language)}
            </label>
            <div className="flex gap-2">
              {[
                { value: 'weak', label: coinSource.gradeWeak },
                { value: 'medium', label: coinSource.gradeMedium },
                { value: 'strong', label: coinSource.gradeStrong },
              ].map(({ value, label }) => (
                <button
                  key={value}
                  onClick={() =>
                    !disabled &&
                    onChange({ ...config, piggy_dash_min_grade: value })
                  }
                  disabled={disabled}
                  className={cn(
                    `px-3 py-1.5 rounded-lg text-sm transition-colors ${
                      (config.piggy_dash_min_grade || 'medium') === value
                        ? 'bg-brand/15 text-brand border border-brand/40'
                        : 'bg-bg text-fg-3 border border-brand/10 hover:bg-surface-2'
                    }`,
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50'
                  )}
                >
                  {ts(label, language)}
                </button>
              ))}
            </div>
            <p className="text-xs pl-2 mt-1 text-fg-3">
              {ts(coinSource.piggyDashMinGradeDesc, language)}
            </p>
          </div>

          <div className="text-xs text-fg-3 flex items-start gap-1.5">
            <span>🐷</span>
            <span>{ts(coinSource.piggyDashNote, language)}</span>
          </div>
        </div>
      )}

      {/* OI Top Options - when the oi_top source is selected */}
      {selectedSources.includes('oi_top') && (
        <div className="p-4 rounded-lg bg-up/5 border border-up/20">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-up" />
              <span className="text-sm font-medium text-fg">
                {ts(coinSource.oiIncreaseTitle, language)}{' '}
                {ts(coinSource.dataSourceConfig, language)}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center gap-3 pl-2">
              <span className="text-sm text-fg-3">
                {ts(coinSource.oiTopLimit, language)}:
              </span>
              <NofxSelect
                value={config.oi_top_limit || 3}
                onChange={(val) =>
                  !disabled &&
                  onChange({ ...config, oi_top_limit: parseInt(val) || 3 })
                }
                disabled={disabled}
                options={[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((n) => ({
                  value: n,
                  label: String(n),
                }))}
                className="px-3 border border-line text-fg h-8 rounded-md bg-surface-2 text-[13px] hover:border-line-strong"
              />
            </div>

            <p className="text-xs pl-2 text-fg-3">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* Short Scan Options - when the short_scan source is selected */}
      {selectedSources.includes('short_scan') && (
        <div className="space-y-4 p-4 rounded-lg bg-bg border border-line">
          <div className="flex items-center gap-2">
            <ArrowDownRight className="w-4 h-4 text-down" />
            <span className="text-sm font-medium text-fg">
              {ts(coinSource.shortScanTitle, language)}
            </span>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.shortScanLimit, language)}
            </label>
            <Input
              type="number"
              min={1}
              max={30}
              value={config.short_scan_limit || 5}
              onChange={(e) =>
                onChange({
                  ...config,
                  short_scan_limit: Math.max(
                    1,
                    Math.min(30, Number(e.target.value) || 5)
                  ),
                })
              }
              disabled={disabled}
              className="w-24 px-3 num text-right"
            />
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.shortScanMinScore, language)}
            </label>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                min={0}
                max={100}
                step={1}
                value={config.short_scan_min_score ?? 0}
                onChange={(e) =>
                  onChange({
                    ...config,
                    // 0 = built-in default (55) — the backend resolves ≤0 that way.
                    short_scan_min_score: Math.max(
                      0,
                      Math.min(100, Number(e.target.value) || 0)
                    ),
                  })
                }
                disabled={disabled}
                className="w-24 px-3 num text-right"
              />
              <span className="text-xs text-fg-3">
                {ts(coinSource.shortScanMinScoreUnit, language)}
              </span>
              {(config.short_scan_min_score ?? 0) === 0 && (
                <span className="text-xs text-fg-3">
                  ({ts(coinSource.shortScanMinScoreDefault, language)})
                </span>
              )}
            </div>
            <p className="text-xs pl-2 mt-1 text-fg-3">
              {ts(coinSource.shortScanMinScoreDesc, language)}
            </p>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.shortScanFunding, language)}
            </label>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                min={0}
                max={1}
                step={0.01}
                value={config.short_scan_funding_rate_pct ?? 0.03}
                onChange={(e) =>
                  onChange({
                    ...config,
                    short_scan_funding_rate_pct: Math.max(
                      0,
                      Math.min(1, Number(e.target.value) || 0)
                    ),
                  })
                }
                disabled={disabled}
                className="w-28 px-3 num text-right"
              />
              <span className="text-xs text-fg-3">%</span>
              {(config.short_scan_funding_rate_pct ?? 0.03) === 0 && (
                <span className="text-xs text-fg-3">
                  ({ts(coinSource.shortScanFundingDefault, language)})
                </span>
              )}
            </div>
            <p className="text-xs pl-2 mt-1 text-fg-3">
              {ts(coinSource.shortScanFundingDesc, language)}
            </p>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.shortScanHistoryDays, language)}
            </label>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                min={-1}
                max={30}
                value={config.short_scan_history_days ?? 0}
                onChange={(e) =>
                  onChange({
                    ...config,
                    short_scan_history_days: Math.max(
                      -1,
                      Math.min(30, Number(e.target.value) || 0)
                    ),
                  })
                }
                disabled={disabled}
                className="w-24 px-3 num text-right"
              />
              <span className="text-xs text-fg-3">
                {ts(coinSource.shortScanHistoryDaysUnit, language)}
              </span>
              {(config.short_scan_history_days ?? 0) === 0 && (
                <span className="text-xs text-fg-3">
                  ({ts(coinSource.shortScanHistoryDaysDefault, language)})
                </span>
              )}
            </div>
            <p className="text-xs pl-2 mt-1 text-fg-3">
              {ts(coinSource.shortScanHistoryDaysDesc, language)}
            </p>
          </div>

          <div>
            <label className="block text-[13px] mb-2 text-fg-2">
              {ts(coinSource.shortScanHistoryMax, language)}
            </label>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                min={0}
                max={100}
                value={config.short_scan_history_max ?? 0}
                onChange={(e) =>
                  onChange({
                    ...config,
                    // 0 = built-in default (30) — the backend resolves ≤0 that way,
                    // so the UI must be able to express it (round-4 review R4-27).
                    short_scan_history_max: Math.max(
                      0,
                      Math.min(100, Number(e.target.value) || 0)
                    ),
                  })
                }
                disabled={disabled}
                className="w-24 px-3 num text-right"
              />
              <span className="text-xs text-fg-3">
                {ts(coinSource.shortScanHistoryMaxUnit, language)}
              </span>
              {(config.short_scan_history_max ?? 0) === 0 && (
                <span className="text-xs text-fg-3">
                  ({ts(coinSource.shortScanHistoryMaxDefault, language)})
                </span>
              )}
            </div>
            <p className="text-xs pl-2 mt-1 text-fg-3">
              {ts(coinSource.shortScanHistoryMaxDesc, language)}
            </p>
          </div>

          <p className="text-xs pl-2 text-fg-3">
            {ts(coinSource.shortScanNote, language)}
          </p>
        </div>
      )}

      {/* OI Low Options - when the oi_low source is selected */}
      {selectedSources.includes('oi_low') && (
        <div className="p-4 rounded-lg bg-down/5 border border-down/20">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <TrendingDown className="w-4 h-4 text-down" />
              <span className="text-sm font-medium text-fg">
                {ts(coinSource.oiDecreaseTitle, language)}{' '}
                {ts(coinSource.dataSourceConfig, language)}
              </span>
              <NofxOSBadge />
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center gap-3 pl-2">
              <span className="text-sm text-fg-3">
                {ts(coinSource.oiLowLimit, language)}:
              </span>
              <NofxSelect
                value={config.oi_low_limit || 3}
                onChange={(val) =>
                  !disabled &&
                  onChange({ ...config, oi_low_limit: parseInt(val) || 3 })
                }
                disabled={disabled}
                options={[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((n) => ({
                  value: n,
                  label: String(n),
                }))}
                className="px-3 border border-line text-fg h-8 rounded-md bg-surface-2 text-[13px] hover:border-line-strong"
              />
            </div>

            <p className="text-xs pl-2 text-fg-3">
              {ts(coinSource.nofxosNote, language)}
            </p>
          </div>
        </div>
      )}

      {/* Mixed Mode Summary */}
      {isMixed &&
        (() => {
          const { sources, totalLimit } = getMixedSummary()
          if (sources.length === 0) return null
          return (
            <div className="p-3 rounded-lg bg-info/5 border border-info/20">
              <div className="flex items-center gap-2 mb-2">
                <Shuffle className="w-4 h-4 text-info" />
                <span className="text-sm font-medium text-fg">
                  {ts(coinSource.mixedSummary, language)}
                </span>
              </div>
              <div className="flex items-center justify-between text-xs">
                <span className="text-fg font-medium">
                  {sources.join(' + ')}
                </span>
                <span className="text-fg-3">
                  {ts(coinSource.maxCoins, language)} {totalLimit}{' '}
                  {ts(coinSource.coins, language)}
                </span>
              </div>
            </div>
          )
        })()}
    </div>
  )
}
