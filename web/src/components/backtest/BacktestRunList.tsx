import {
  Activity,
  CheckCircle2,
  XCircle,
  Pause,
  Clock,
  Layers,
  Eye,
} from 'lucide-react'
import { t, type Language } from '../../i18n/translations'

// ============ Types ============

export interface BacktestRunItem {
  run_id: string
  state: string
  summary: {
    progress_pct: number
    equity_last: number
    decision_tf?: string
    symbol_count?: number
  }
}

// ============ State Helpers ============

export function getStateColor(state: string) {
  switch (state) {
    case 'running':
      return '#F0B90B'
    case 'completed':
      return '#0ECB81'
    case 'failed':
    case 'liquidated':
      return '#F6465D'
    case 'paused':
      return '#848E9C'
    default:
      return '#848E9C'
  }
}

export function getStateIcon(state: string) {
  switch (state) {
    case 'running':
      return <Activity className="w-4 h-4" />
    case 'completed':
      return <CheckCircle2 className="w-4 h-4" />
    case 'failed':
    case 'liquidated':
      return <XCircle className="w-4 h-4" />
    case 'paused':
      return <Pause className="w-4 h-4" />
    default:
      return <Clock className="w-4 h-4" />
  }
}

// ============ Run History List ============

interface BacktestRunListProps {
  runs: BacktestRunItem[]
  selectedRunId: string | undefined
  compareRunIds: string[]
  language: Language
  tr: (key: string, params?: Record<string, string | number>) => string
  onSelectRun: (runId: string) => void
  onToggleCompare: (runId: string) => void
}

export function BacktestRunList({
  runs,
  selectedRunId,
  compareRunIds,
  language,
  tr,
  onSelectRun,
  onToggleCompare,
}: BacktestRunListProps) {
  return (
    <div className="binance-card p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-bold flex items-center gap-2" style={{ color: '#EAECEF' }}>
          <Layers className="w-4 h-4" style={{ color: '#F0B90B' }} />
          {tr('runList.title')}
        </h3>
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {runs.length} {t('backtestPageExtra.runs', language)}
        </span>
      </div>

      <div className="space-y-2 max-h-[300px] overflow-y-auto">
        {runs.length === 0 ? (
          <div className="py-8 text-center text-sm" style={{ color: '#5E6673' }}>
            {tr('emptyStates.noRuns')}
          </div>
        ) : (
          runs.map((run) => (
            <button
              key={run.run_id}
              onClick={() => onSelectRun(run.run_id)}
              className="w-full p-3 rounded-lg text-left transition-all"
              style={{
                background: run.run_id === selectedRunId ? 'rgba(240,185,11,0.1)' : '#1E2329',
                border: `1px solid ${run.run_id === selectedRunId ? '#F0B90B' : '#2B3139'}`,
              }}
            >
              <div className="flex items-center justify-between">
                <span className="font-mono text-xs" style={{ color: '#EAECEF' }}>
                  {run.run_id.slice(0, 20)}...
                </span>
                <span
                  className="flex items-center gap-1 text-xs"
                  style={{ color: getStateColor(run.state) }}
                >
                  {getStateIcon(run.state)}
                  {tr(`states.${run.state}`)}
                </span>
              </div>
              <div className="flex items-center justify-between mt-1">
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {run.summary.progress_pct.toFixed(0)}% · ${run.summary.equity_last.toFixed(0)}
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    onToggleCompare(run.run_id)
                  }}
                  className="p-1 rounded"
                  style={{
                    background: compareRunIds.includes(run.run_id)
                      ? 'rgba(240,185,11,0.2)'
                      : 'transparent',
                  }}
                  title={t('backtestPageExtra.addToCompare', language)}
                >
                  <Eye
                    className="w-3 h-3"
                    style={{
                      color: compareRunIds.includes(run.run_id) ? '#F0B90B' : '#5E6673',
                    }}
                  />
                </button>
              </div>
            </button>
          ))
        )}
      </div>
    </div>
  )
}
