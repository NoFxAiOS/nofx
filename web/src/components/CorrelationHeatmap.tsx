import { useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { AlertTriangle, TrendingUp, TrendingDown } from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'

interface CorrelationMatrix {
  assets: string[]
  matrix: number[][]
  timeframe: string
  calculated_at: string
  stats: {
    avg_correlation: number
    max_correlation: number
    min_correlation: number
    highly_correlated: Array<{
      asset1: string
      asset2: string
      correlation: number
    }>
    low_correlated: Array<{
      asset1: string
      asset2: string
      correlation: number
    }>
  }
}

interface CorrelationHeatmapProps {
  traderId: string
  symbols: string[]
  timeframe?: string
}

export function CorrelationHeatmap({
  traderId,
  symbols,
  timeframe = '1h',
}: CorrelationHeatmapProps) {
  // const { language } = useLanguage() // Unused for now
  const [hoveredCell, setHoveredCell] = useState<{
    row: number
    col: number
  } | null>(null)

  const { data: correlation, error } = useSWR<CorrelationMatrix>(
    traderId && symbols.length >= 2
      ? `correlation-${traderId}-${symbols.join(',')}-${timeframe}`
      : null,
    () => api.getCorrelationMatrix(traderId, symbols, timeframe),
    {
      refreshInterval: 60000, // 1分钟刷新
      revalidateOnFocus: false,
    }
  )

  if (error) {
    return (
      <div className="binance-card p-6">
        <div
          className="flex items-center gap-3 p-4 rounded"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>
              Failed to load correlation data
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (!correlation) {
    return (
      <div className="binance-card p-6">
        <div className="animate-pulse space-y-4">
          <div className="skeleton h-6 w-48"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  // 计算颜色：相关性从-1（红）到+1（绿）
  const getCorrelationColor = (value: number): string => {
    if (value === 1) return '#F0B90B' // 自相关 - 金色
    if (value > 0.7) return '#0ECB81' // 强正相关 - 绿色
    if (value > 0.3) return '#4CAF50' // 中等正相关 - 浅绿
    if (value > -0.3) return '#848E9C' // 弱相关 - 灰色
    if (value > -0.7) return '#FF9800' // 中等负相关 - 橙色
    return '#F6465D' // 强负相关 - 红色
  }

  const getCorrelationLabel = (value: number): string => {
    if (value === 1) return 'Self'
    if (value > 0.7) return 'Strong +'
    if (value > 0.3) return 'Moderate +'
    if (value > -0.3) return 'Weak'
    if (value > -0.7) return 'Moderate -'
    return 'Strong -'
  }

  const cellSize = 80 // Size of each cell in pixels

  return (
    <div className="binance-card p-6 animate-fade-in">
      {/* Header */}
      <div className="mb-6">
        <h3 className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
          📊 Correlation Matrix
        </h3>
        <div className="flex flex-wrap gap-2 text-sm" style={{ color: '#848E9C' }}>
          <span>Timeframe: {timeframe}</span>
          <span>•</span>
          <span>Assets: {correlation.assets.length}</span>
          <span>•</span>
          <span>
            Avg Correlation:{' '}
            <span
              style={{
                color: getCorrelationColor(correlation.stats.avg_correlation),
                fontWeight: 600,
              }}
            >
              {correlation.stats.avg_correlation.toFixed(3)}
            </span>
          </span>
        </div>
      </div>

      {/* Heatmap */}
      <div className="overflow-x-auto">
        <div className="inline-block min-w-full">
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: `100px repeat(${correlation.assets.length}, ${cellSize}px)`,
              gap: '2px',
            }}
          >
            {/* Empty top-left corner */}
            <div></div>

            {/* Column headers */}
            {correlation.assets.map((asset, idx) => (
              <div
                key={`header-${idx}`}
                className="text-xs font-bold flex items-center justify-center"
                style={{
                  color: '#EAECEF',
                  height: `${cellSize}px`,
                  writingMode: 'vertical-rl',
                  transform: 'rotate(180deg)',
                }}
              >
                {asset}
              </div>
            ))}

            {/* Rows */}
            {correlation.assets.map((rowAsset, rowIdx) => (
              <>
                {/* Row header */}
                <div
                  key={`row-header-${rowIdx}`}
                  className="text-xs font-bold flex items-center justify-end pr-2"
                  style={{ color: '#EAECEF', height: `${cellSize}px` }}
                >
                  {rowAsset}
                </div>

                {/* Cells */}
                {correlation.matrix[rowIdx].map((value, colIdx) => (
                  <div
                    key={`cell-${rowIdx}-${colIdx}`}
                    className="relative flex items-center justify-center cursor-pointer transition-all hover:scale-105"
                    style={{
                      height: `${cellSize}px`,
                      backgroundColor: getCorrelationColor(value),
                      opacity:
                        hoveredCell?.row === rowIdx || hoveredCell?.col === colIdx
                          ? 1
                          : 0.7,
                      borderRadius: '4px',
                    }}
                    onMouseEnter={() => setHoveredCell({ row: rowIdx, col: colIdx })}
                    onMouseLeave={() => setHoveredCell(null)}
                  >
                    <div className="text-center">
                      <div
                        className="text-sm font-bold"
                        style={{ color: value === 1 ? '#000' : '#FFF' }}
                      >
                        {value.toFixed(2)}
                      </div>
                      <div
                        className="text-xs mt-1"
                        style={{
                          color: value === 1 ? '#000' : '#FFF',
                          opacity: 0.8,
                        }}
                      >
                        {getCorrelationLabel(value)}
                      </div>
                    </div>

                    {/* Tooltip on hover */}
                    {hoveredCell?.row === rowIdx && hoveredCell?.col === colIdx && (
                      <div
                        className="absolute z-10 p-2 rounded shadow-xl"
                        style={{
                          background: '#1E2329',
                          border: '1px solid #2B3139',
                          bottom: '100%',
                          left: '50%',
                          transform: 'translateX(-50%)',
                          marginBottom: '8px',
                          minWidth: '150px',
                        }}
                      >
                        <div className="text-xs font-semibold" style={{ color: '#EAECEF' }}>
                          {rowAsset} × {correlation.assets[colIdx]}
                        </div>
                        <div
                          className="text-sm font-bold mt-1"
                          style={{ color: getCorrelationColor(value) }}
                        >
                          r = {value.toFixed(4)}
                        </div>
                        <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                          {getCorrelationLabel(value)}
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </>
            ))}
          </div>
        </div>
      </div>

      {/* Statistics */}
      <div className="mt-6 grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Highly Correlated */}
        <div
          className="p-4 rounded"
          style={{ background: 'rgba(14, 203, 129, 0.1)', border: '1px solid rgba(14, 203, 129, 0.2)' }}
        >
          <div className="flex items-center gap-2 mb-3">
            <TrendingUp className="w-5 h-5" style={{ color: '#0ECB81' }} />
            <h4 className="font-bold" style={{ color: '#0ECB81' }}>
              Highly Correlated Pairs
            </h4>
          </div>
          {correlation.stats.highly_correlated.length > 0 ? (
            <div className="space-y-2">
              {correlation.stats.highly_correlated.map((pair, idx) => (
                <div
                  key={idx}
                  className="flex justify-between items-center text-sm"
                  style={{ color: '#EAECEF' }}
                >
                  <span>
                    {pair.asset1} × {pair.asset2}
                  </span>
                  <span className="font-bold" style={{ color: '#0ECB81' }}>
                    {pair.correlation.toFixed(3)}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-sm" style={{ color: '#848E9C' }}>
              No highly correlated pairs found
            </div>
          )}
        </div>

        {/* Low Correlated */}
        <div
          className="p-4 rounded"
          style={{ background: 'rgba(132, 142, 156, 0.1)', border: '1px solid rgba(132, 142, 156, 0.2)' }}
        >
          <div className="flex items-center gap-2 mb-3">
            <TrendingDown className="w-5 h-5" style={{ color: '#848E9C' }} />
            <h4 className="font-bold" style={{ color: '#848E9C' }}>
              Low Correlated Pairs (Diversification)
            </h4>
          </div>
          {correlation.stats.low_correlated.length > 0 ? (
            <div className="space-y-2">
              {correlation.stats.low_correlated.map((pair, idx) => (
                <div
                  key={idx}
                  className="flex justify-between items-center text-sm"
                  style={{ color: '#EAECEF' }}
                >
                  <span>
                    {pair.asset1} × {pair.asset2}
                  </span>
                  <span className="font-bold" style={{ color: '#848E9C' }}>
                    {pair.correlation.toFixed(3)}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-sm" style={{ color: '#848E9C' }}>
              No low correlated pairs found
            </div>
          )}
        </div>
      </div>

      {/* Legend */}
      <div className="mt-6 pt-4" style={{ borderTop: '1px solid #2B3139' }}>
        <div className="text-xs font-semibold mb-2" style={{ color: '#848E9C' }}>
          CORRELATION SCALE
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {[
            { label: 'Strong -', color: '#F6465D' },
            { label: 'Moderate -', color: '#FF9800' },
            { label: 'Weak', color: '#848E9C' },
            { label: 'Moderate +', color: '#4CAF50' },
            { label: 'Strong +', color: '#0ECB81' },
            { label: 'Self', color: '#F0B90B' },
          ].map((item) => (
            <div key={item.label} className="flex items-center gap-1">
              <div
                className="w-3 h-3 rounded"
                style={{ backgroundColor: item.color }}
              ></div>
              <span className="text-xs" style={{ color: '#EAECEF' }}>
                {item.label}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
