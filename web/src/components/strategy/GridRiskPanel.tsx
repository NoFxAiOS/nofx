import { useState, useEffect, useCallback } from 'react'
import { Shield, TrendingUp, AlertTriangle, Activity, Box, ChevronDown, ChevronUp } from 'lucide-react'
import type { GridRiskInfo } from '../../types'
import { gridRisk, ts } from '../../i18n/strategy-translations'

interface GridRiskPanelProps {
  traderId: string
  language?: string
  refreshInterval?: number // ms, default 5000
}

export function GridRiskPanel({
  traderId,
  language = 'en',
  refreshInterval = 5000,
}: GridRiskPanelProps) {
  const [riskInfo, setRiskInfo] = useState<GridRiskInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)

  const fetchRiskInfo = useCallback(async () => {
    try {
      const token = localStorage.getItem('auth_token')
      const response = await fetch(`/api/traders/${traderId}/grid-risk`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const data = await response.json()
      setRiskInfo(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [traderId])

  useEffect(() => {
    fetchRiskInfo()
    const interval = setInterval(fetchRiskInfo, refreshInterval)
    return () => clearInterval(interval)
  }, [fetchRiskInfo, refreshInterval])

  const getRegimeColor = (regime: string) => {
    switch (regime) {
      case 'narrow': return '#2E8B57'
      case 'standard': return '#E0483B'
      case 'wide': return '#E0483B'
      case 'volatile': return '#D6433A'
      case 'trending': return '#E0483B'
      default: return '#8A8478'
    }
  }

  const getBreakoutColor = (level: string) => {
    switch (level) {
      case 'none': return '#2E8B57'
      case 'short': return '#E0483B'
      case 'mid': return '#E0483B'
      case 'long': return '#D6433A'
      default: return '#8A8478'
    }
  }

  const getPositionColor = (percent: number) => {
    if (percent < 50) return '#2E8B57'
    if (percent < 80) return '#E0483B'
    return '#D6433A'
  }

  const formatPrice = (price: number) => {
    if (price === 0) return '-'
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    if (price >= 1) return price.toFixed(4)
    return price.toFixed(6)
  }

  const formatUSD = (value: number) => {
    return `$${value.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`
  }

  const cardStyle = {
    background: '#F7F4EC',
    border: '1px solid rgba(26,24,19,0.14)',
  }

  if (loading) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: '#8A8478' }}>
        {ts(gridRisk.loading, language)}
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: '#D6433A' }}>
        {ts(gridRisk.error, language)}: {error}
      </div>
    )
  }

  if (!riskInfo) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: '#8A8478' }}>
        {ts(gridRisk.noData, language)}
      </div>
    )
  }

  return (
    <div className="rounded-lg" style={cardStyle}>
      {/* Collapsible Header */}
      <div
        className="flex items-center justify-between p-3 cursor-pointer hover:bg-[#E8E2D5] transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-2">
          <Shield className="w-4 h-4" style={{ color: '#E0483B' }} />
          <span className="font-medium text-sm" style={{ color: '#1A1813' }}>
            {ts(gridRisk.gridRisk, language)}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {/* Summary badges when collapsed */}
          <div className="flex items-center gap-2 text-xs">
            <span
              className="px-2 py-0.5 rounded"
              style={{ background: getRegimeColor(riskInfo.regime_level) + '20', color: getRegimeColor(riskInfo.regime_level) }}
            >
              {ts(gridRisk[(riskInfo.regime_level || 'standard') as keyof typeof gridRisk], language)}
            </span>
            <span className="font-mono" style={{ color: '#1A1813' }}>
              {riskInfo.effective_leverage.toFixed(1)}x
            </span>
            <span
              className="font-mono"
              style={{ color: getPositionColor(riskInfo.position_percent) }}
            >
              {riskInfo.position_percent.toFixed(0)}%
            </span>
          </div>
          {expanded ? (
            <ChevronUp className="w-4 h-4" style={{ color: '#8A8478' }} />
          ) : (
            <ChevronDown className="w-4 h-4" style={{ color: '#8A8478' }} />
          )}
        </div>
      </div>

      {/* Expanded Content */}
      {expanded && (
        <div className="px-3 pb-3 space-y-3">
          {/* Row 1: Leverage & Position */}
          <div className="grid grid-cols-2 gap-3">
            {/* Leverage */}
            <div className="p-2 rounded" style={{ background: '#E8E2D5' }}>
              <div className="flex items-center gap-1 mb-2">
                <TrendingUp className="w-3 h-3" style={{ color: '#E0483B' }} />
                <span className="text-xs font-medium" style={{ color: '#8A8478' }}>{ts(gridRisk.leverageInfo, language)}</span>
              </div>
              <div className="grid grid-cols-3 gap-1 text-xs">
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.currentLeverage, language)}</div>
                  <div className="font-mono" style={{ color: '#1A1813' }}>{riskInfo.current_leverage}x</div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.effectiveLeverage, language)}</div>
                  <div className="font-mono" style={{ color: '#E0483B' }}>{riskInfo.effective_leverage.toFixed(2)}x</div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.recommendedLeverage, language)}</div>
                  <div
                    className="font-mono"
                    style={{ color: riskInfo.current_leverage > riskInfo.recommended_leverage ? '#D6433A' : '#2E8B57' }}
                  >
                    {riskInfo.recommended_leverage}x
                  </div>
                </div>
              </div>
            </div>

            {/* Position */}
            <div className="p-2 rounded" style={{ background: '#E8E2D5' }}>
              <div className="flex items-center gap-1 mb-2">
                <Activity className="w-3 h-3" style={{ color: '#E0483B' }} />
                <span className="text-xs font-medium" style={{ color: '#8A8478' }}>{ts(gridRisk.positionInfo, language)}</span>
              </div>
              <div className="grid grid-cols-3 gap-1 text-xs">
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.currentPosition, language)}</div>
                  <div className="font-mono" style={{ color: '#1A1813' }}>{formatUSD(riskInfo.current_position)}</div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.maxPosition, language)}</div>
                  <div className="font-mono" style={{ color: '#1A1813' }}>{formatUSD(riskInfo.max_position)}</div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.positionPercent, language)}</div>
                  <div className="font-mono" style={{ color: getPositionColor(riskInfo.position_percent) }}>
                    {riskInfo.position_percent.toFixed(1)}%
                  </div>
                </div>
              </div>
              {/* Mini progress bar */}
              <div className="h-1 mt-2 rounded-full overflow-hidden" style={{ background: '#E8E2D5' }}>
                <div
                  className="h-full rounded-full"
                  style={{ width: `${Math.min(riskInfo.position_percent, 100)}%`, background: getPositionColor(riskInfo.position_percent) }}
                />
              </div>
            </div>
          </div>

          {/* Row 2: Market State & Liquidation */}
          <div className="grid grid-cols-2 gap-3">
            {/* Market State */}
            <div className="p-2 rounded" style={{ background: '#E8E2D5' }}>
              <div className="flex items-center gap-1 mb-2">
                <Shield className="w-3 h-3" style={{ color: '#E0483B' }} />
                <span className="text-xs font-medium" style={{ color: '#8A8478' }}>{ts(gridRisk.marketState, language)}</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.regimeLevel, language)}</div>
                  <div className="font-medium" style={{ color: getRegimeColor(riskInfo.regime_level) }}>
                    {ts(gridRisk[(riskInfo.regime_level || 'standard') as keyof typeof gridRisk], language)}
                  </div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.currentPrice, language)}</div>
                  <div className="font-mono" style={{ color: '#1A1813' }}>{formatPrice(riskInfo.current_price)}</div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.breakoutLevel, language)}</div>
                  <div className="font-medium" style={{ color: getBreakoutColor(riskInfo.breakout_level) }}>
                    {ts(gridRisk[(riskInfo.breakout_level || 'none') as keyof typeof gridRisk], language)}
                  </div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.breakoutDirection, language)}</div>
                  <div
                    className="font-medium"
                    style={{ color: riskInfo.breakout_direction === 'up' ? '#2E8B57' : riskInfo.breakout_direction === 'down' ? '#D6433A' : '#8A8478' }}
                  >
                    {riskInfo.breakout_direction ? ts(gridRisk[riskInfo.breakout_direction as keyof typeof gridRisk], language) : '-'}
                  </div>
                </div>
              </div>
            </div>

            {/* Liquidation */}
            <div className="p-2 rounded" style={{ background: '#E8E2D5' }}>
              <div className="flex items-center gap-1 mb-2">
                <AlertTriangle className="w-3 h-3" style={{ color: '#D6433A' }} />
                <span className="text-xs font-medium" style={{ color: '#8A8478' }}>{ts(gridRisk.liquidationInfo, language)}</span>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.liquidationPrice, language)}</div>
                  <div className="font-mono" style={{ color: '#D6433A' }}>
                    {riskInfo.liquidation_price > 0 ? formatPrice(riskInfo.liquidation_price) : '-'}
                  </div>
                </div>
                <div>
                  <div style={{ color: '#8A8478' }}>{ts(gridRisk.liquidationDistance, language)}</div>
                  <div className="font-mono" style={{ color: '#D6433A' }}>
                    {riskInfo.liquidation_distance > 0 ? `${riskInfo.liquidation_distance.toFixed(1)}%` : '-'}
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Row 3: Box State */}
          <div className="p-2 rounded" style={{ background: '#E8E2D5' }}>
            <div className="flex items-center gap-1 mb-2">
              <Box className="w-3 h-3" style={{ color: '#E0483B' }} />
              <span className="text-xs font-medium" style={{ color: '#8A8478' }}>{ts(gridRisk.boxState, language)}</span>
            </div>
            <div className="grid grid-cols-3 gap-2 text-xs">
              <div className="flex justify-between">
                <span style={{ color: '#8A8478' }}>{ts(gridRisk.shortBox, language)}</span>
                <span className="font-mono" style={{ color: '#1A1813' }}>
                  {formatPrice(riskInfo.short_box_lower)} - {formatPrice(riskInfo.short_box_upper)}
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#8A8478' }}>{ts(gridRisk.midBox, language)}</span>
                <span className="font-mono" style={{ color: '#1A1813' }}>
                  {formatPrice(riskInfo.mid_box_lower)} - {formatPrice(riskInfo.mid_box_upper)}
                </span>
              </div>
              <div className="flex justify-between">
                <span style={{ color: '#8A8478' }}>{ts(gridRisk.longBox, language)}</span>
                <span className="font-mono" style={{ color: '#1A1813' }}>
                  {formatPrice(riskInfo.long_box_lower)} - {formatPrice(riskInfo.long_box_upper)}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
