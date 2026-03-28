import { useState, useEffect, useRef } from 'react'
import { Loader2, Info } from 'lucide-react'
import type { StrategyConfig } from '../../types'
import { t, type Language } from '../../i18n/translations'

const API_BASE = import.meta.env.VITE_API_BASE || ''

interface ModelLimit {
  name: string
  context_limit: number
  usage_pct: number
  level: string
}

interface TokenEstimateResult {
  total: number
  model_limits: ModelLimit[]
  suggestions: string[]
}

interface TokenEstimateBarProps {
  config: StrategyConfig | null
  language: Language
  onTokenCountChange?: (total: number) => void
}

export function TokenEstimateBar({ config, language, onTokenCountChange }: TokenEstimateBarProps) {
  const [estimate, setEstimate] = useState<TokenEstimateResult | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const tr = (key: string) => t(`strategyStudio.${key}`, language)

  useEffect(() => {
    if (!config) {
      setEstimate(null)
      return
    }

    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }

    debounceRef.current = setTimeout(async () => {
      setIsLoading(true)
      try {
        const response = await fetch(`${API_BASE}/api/strategies/estimate-tokens`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ config }),
        })
        if (response.ok) {
          const data = await response.json()
          setEstimate(data)
          onTokenCountChange?.(data.total)
        }
      } catch {
        // silently ignore — non-critical UI element
      } finally {
        setIsLoading(false)
      }
    }, 800)

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current)
      }
    }
  }, [config])

  if (!config) return null

  if (isLoading && !estimate) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-nofx-text-muted">
        <Loader2 className="w-3 h-3 animate-spin" />
        <span>{tr('tokenEstimating')}</span>
      </div>
    )
  }

  if (!estimate) return null

  // Display based on 200K reference
  const pct = Math.round(estimate.total * 100 / 200000)
  const barWidth = Math.min(pct, 100)

  let barColor = '#0ECB81' // green
  let textColor = '#848E9C'
  if (pct >= 100) {
    barColor = '#F6465D' // red
    textColor = '#F6465D'
  } else if (pct >= 80) {
    barColor = '#F0B90B' // yellow
    textColor = '#F0B90B'
  }

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <div
          className="flex-1 h-1.5 rounded-full overflow-hidden"
          style={{ background: '#1E2329' }}
        >
          <div
            className="h-full rounded-full transition-all duration-500"
            style={{ width: `${barWidth}%`, background: barColor }}
          />
        </div>
        <span className="text-xs font-mono whitespace-nowrap" style={{ color: textColor }}>
          {isLoading ? <Loader2 className="w-3 h-3 animate-spin inline" /> : `${pct}%`}
        </span>
        <div className="relative group">
          <Info className="w-3 h-3 text-nofx-text-muted cursor-help" />
          <div className="absolute bottom-full right-0 mb-1.5 px-2.5 py-1.5 rounded-lg text-[10px] whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-50 bg-nofx-bg-lighter border border-nofx-border text-nofx-text-muted shadow-lg">
            {tr('tokenTooltip')} (~{estimate.total.toLocaleString()} / 200K)
          </div>
        </div>
      </div>
    </div>
  )
}
