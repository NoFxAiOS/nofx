import { useState } from 'react'
import { Plus, X, Database, TrendingUp, List, Link, AlertCircle } from 'lucide-react'
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

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      sourceType: { zh: '数据来源类型', en: 'Source Type' },
      static: { zh: '静态列表', en: 'Static List' },
      coinpool: { zh: 'AI500 币种池', en: 'AI500 Coin Pool' },
      oi_top: { zh: 'OI Top 持仓增长', en: 'OI Top' },
      mixed: { zh: '混合模式', en: 'Mixed Mode' },
      staticCoins: { zh: '自定义币种', en: 'Custom Coins' },
      addCoin: { zh: '添加币种', en: 'Add Coin' },
      useCoinPool: { zh: '启用 AI500 币种池', en: 'Enable AI500 Coin Pool' },
      coinPoolLimit: { zh: '币种池数量上限', en: 'Coin Pool Limit' },
      coinPoolApiUrl: { zh: 'AI500 API URL', en: 'AI500 API URL' },
      coinPoolApiUrlPlaceholder: { zh: '输入 AI500 币种池 API 地址...', en: 'Enter AI500 coin pool API URL...' },
      useOITop: { zh: '启用 OI Top 数据', en: 'Enable OI Top' },
      oiTopLimit: { zh: 'OI Top 数量上限', en: 'OI Top Limit' },
      oiTopApiUrl: { zh: 'OI Top API URL', en: 'OI Top API URL' },
      oiTopApiUrlPlaceholder: { zh: '输入 OI Top 持仓数据 API 地址...', en: 'Enter OI Top API URL...' },
      staticDesc: { zh: '手动指定交易币种列表', en: 'Manually specify trading coins' },
      coinpoolDesc: {
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
      apiUrlRequired: { zh: '需要填写 API URL 才能获取数据', en: 'API URL required to fetch data' },
      dataSourceConfig: { zh: '数据源配置', en: 'Data Source Configuration' },
    }
    return translations[key]?.[language] || key
  }

  const sourceTypes = [
    { value: 'static', icon: List, color: '#848E9C' },
    { value: 'coinpool', icon: Database, color: '#F0B90B' },
    { value: 'oi_top', icon: TrendingUp, color: '#0ECB81' },
    { value: 'mixed', icon: Database, color: '#60a5fa' },
  ] as const

  const handleAddCoin = () => {
    if (!newCoin.trim()) return
    const symbol = newCoin.toUpperCase().trim()
    const formattedSymbol = symbol.endsWith('USDT') ? symbol : `${symbol}USDT`
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

  return (
    <div className="space-y-6">
      {/* Source Type Selector */}
      <div>
        <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
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
              className={`p-4 rounded-lg border transition-all ${
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
              <Icon className="w-6 h-6 mx-auto mb-2" style={{ color }} />
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {t(value)}
              </div>
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                {t(`${value}Desc`)}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Static Coins */}
      {(config.source_type === 'static' || config.source_type === 'mixed') && (
        <div>
          <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
            {t('staticCoins')}
          </label>
          <div className="flex flex-wrap gap-2 mb-3">
            {(config.static_coins || []).map((coin) => (
              <span
                key={coin}
                className="flex items-center gap-1 px-3 py-1.5 rounded-full text-sm"
                style={{ background: '#2B3139', color: '#EAECEF' }}
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
                className="flex-1 px-4 py-2 rounded-lg"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <button
                onClick={handleAddCoin}
                className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors"
                style={{ background: '#F0B90B', color: '#0B0E11' }}
              >
                <Plus className="w-4 h-4" />
                {t('addCoin')}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Coin Pool Options */}
      {(config.source_type === 'coinpool' || config.source_type === 'mixed') && (
        <div className="space-y-4">
          <div className="flex items-center gap-2 mb-2">
            <Link className="w-4 h-4" style={{ color: '#F0B90B' }} />
            <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              {t('dataSourceConfig')} - AI500
            </span>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="flex items-center gap-3 mb-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={config.use_coin_pool}
                  onChange={(e) =>
                    !disabled && onChange({ ...config, use_coin_pool: e.target.checked })
                  }
                  disabled={disabled}
                  className="w-5 h-5 rounded accent-yellow-500"
                />
                <span style={{ color: '#EAECEF' }}>{t('useCoinPool')}</span>
              </label>
              {config.use_coin_pool && (
                <div className="flex items-center gap-3">
                  <span className="text-sm" style={{ color: '#848E9C' }}>
                    {t('coinPoolLimit')}:
                  </span>
                  <input
                    type="number"
                    value={config.coin_pool_limit || 30}
                    onChange={(e) =>
                      !disabled &&
                      onChange({ ...config, coin_pool_limit: parseInt(e.target.value) || 30 })
                    }
                    disabled={disabled}
                    min={1}
                    max={100}
                    className="w-20 px-3 py-1.5 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                </div>
              )}
            </div>
          </div>

          {config.use_coin_pool && (
            <div>
              <label className="block text-sm mb-2" style={{ color: '#848E9C' }}>
                {t('coinPoolApiUrl')}
              </label>
              <input
                type="url"
                value={config.coin_pool_api_url || ''}
                onChange={(e) =>
                  !disabled && onChange({ ...config, coin_pool_api_url: e.target.value })
                }
                disabled={disabled}
                placeholder={t('coinPoolApiUrlPlaceholder')}
                className="w-full px-4 py-2.5 rounded-lg font-mono text-sm"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              {!config.coin_pool_api_url && (
                <div className="flex items-center gap-2 mt-2">
                  <AlertCircle className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  <span className="text-xs" style={{ color: '#F0B90B' }}>
                    {t('apiUrlRequired')}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* OI Top Options */}
      {(config.source_type === 'oi_top' || config.source_type === 'mixed') && (
        <div className="space-y-4">
          <div className="flex items-center gap-2 mb-2">
            <Link className="w-4 h-4" style={{ color: '#0ECB81' }} />
            <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              {t('dataSourceConfig')} - OI Top
            </span>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="flex items-center gap-3 mb-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={config.use_oi_top}
                  onChange={(e) =>
                    !disabled && onChange({ ...config, use_oi_top: e.target.checked })
                  }
                  disabled={disabled}
                  className="w-5 h-5 rounded accent-yellow-500"
                />
                <span style={{ color: '#EAECEF' }}>{t('useOITop')}</span>
              </label>
              {config.use_oi_top && (
                <div className="flex items-center gap-3">
                  <span className="text-sm" style={{ color: '#848E9C' }}>
                    {t('oiTopLimit')}:
                  </span>
                  <input
                    type="number"
                    value={config.oi_top_limit || 20}
                    onChange={(e) =>
                      !disabled &&
                      onChange({ ...config, oi_top_limit: parseInt(e.target.value) || 20 })
                    }
                    disabled={disabled}
                    min={1}
                    max={50}
                    className="w-20 px-3 py-1.5 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                </div>
              )}
            </div>
          </div>

          {config.use_oi_top && (
            <div>
              <label className="block text-sm mb-2" style={{ color: '#848E9C' }}>
                {t('oiTopApiUrl')}
              </label>
              <input
                type="url"
                value={config.oi_top_api_url || ''}
                onChange={(e) =>
                  !disabled && onChange({ ...config, oi_top_api_url: e.target.value })
                }
                disabled={disabled}
                placeholder={t('oiTopApiUrlPlaceholder')}
                className="w-full px-4 py-2.5 rounded-lg font-mono text-sm"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              {!config.oi_top_api_url && (
                <div className="flex items-center gap-2 mt-2">
                  <AlertCircle className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  <span className="text-xs" style={{ color: '#F0B90B' }}>
                    {t('apiUrlRequired')}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
