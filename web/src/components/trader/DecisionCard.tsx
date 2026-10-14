import { useState } from 'react'
import { toast } from 'sonner'
import {
  Bot,
  ChevronDown,
  ChevronRight,
  Copy,
  Download,
  Lightbulb,
  Target,
} from 'lucide-react'
import type { DecisionRecord, DecisionAction } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { gateCodeLabel } from '../../lib/gateCodeLabels'
import { cn } from '../../lib/cn'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'

interface DecisionCardProps {
  decision: DecisionRecord
  language: Language
  onSymbolClick?: (symbol: string) => void
}

type BadgeVariant = 'up' | 'down' | 'neutral' | 'brand'

// Action badge configuration: direction decides the colour
// (open_long / close_short -> up, open_short / close_long -> down).
const ACTION_CONFIG: Record<string, { variant: BadgeVariant; label: string }> =
  {
    open_long: { variant: 'up', label: 'LONG' },
    open_short: { variant: 'down', label: 'SHORT' },
    close_long: { variant: 'down', label: 'CLOSE LONG' },
    close_short: { variant: 'up', label: 'CLOSE SHORT' },
    hold: { variant: 'neutral', label: 'HOLD' },
    wait: { variant: 'neutral', label: 'WAIT' },
  }

// Format price with proper decimals
function formatPrice(price: number | undefined): string {
  if (!price || price === 0) return '-'
  if (price >= 1000) return price.toFixed(2)
  if (price >= 1) return price.toFixed(4)
  return price.toFixed(6)
}

// Calculate percentage change
function calcPctChange(
  entry: number | undefined,
  target: number | undefined,
  isLong: boolean
): string {
  if (!entry || !target || entry === 0) return '-'
  const pct = ((target - entry) / entry) * 100
  const adjustedPct = isLong ? pct : -pct
  return `${adjustedPct >= 0 ? '+' : ''}${adjustedPct.toFixed(2)}%`
}

function confidenceVariant(confidence: number): BadgeVariant {
  if (confidence >= 80) return 'up'
  if (confidence >= 60) return 'brand'
  return 'down'
}

function Cell({
  label,
  value,
  sub,
  tone,
}: {
  label: string
  value: string
  sub?: string
  tone?: string
}) {
  return (
    <div className="min-w-0">
      <div className={cn('truncate text-[11px]', tone ?? 'text-fg-3')}>
        {label}
      </div>
      <div className={cn('num text-[13px] font-semibold', tone ?? 'text-fg')}>
        {value}
      </div>
      {sub && <div className="num text-[11px] text-fg-3">{sub}</div>}
    </div>
  )
}

// Single action row
function ActionCard({
  action,
  language,
  onSymbolClick,
}: {
  action: DecisionAction
  language: Language
  onSymbolClick?: (symbol: string) => void
}) {
  const config = ACTION_CONFIG[action.action] || ACTION_CONFIG.wait
  const isLong = action.action.includes('long')
  const isOpen = action.action.includes('open')

  return (
    <div className="rounded-md border border-line bg-surface-2 px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className="num cursor-pointer text-sm font-semibold text-fg hover:underline"
            onClick={() => onSymbolClick?.(action.symbol)}
            title="Click to view chart"
          >
            {action.symbol.replace('USDT', '')}
          </span>
          <Badge variant={config.variant} size="xs">
            {config.label}
          </Badge>
        </div>
        <div className="flex items-center gap-1.5">
          {action.confidence !== undefined && action.confidence > 0 && (
            <Badge variant={confidenceVariant(action.confidence)} size="xs">
              <span className="num">{action.confidence.toFixed(0)}%</span>
            </Badge>
          )}
          <Badge variant={action.success ? 'up' : 'down'} size="xs">
            {action.success ? 'OK' : 'ERR'}
          </Badge>
        </div>
      </div>

      {isOpen && (
        <div className="mt-2 grid grid-cols-4 gap-2 border-t border-line pt-2">
          <Cell
            label={t('entryPrice', language)}
            value={formatPrice(action.price)}
          />
          <Cell
            label={t('stopLoss', language)}
            value={formatPrice(action.stop_loss)}
            sub={
              action.stop_loss && action.price
                ? calcPctChange(action.price, action.stop_loss, isLong)
                : undefined
            }
            tone="text-down"
          />
          <Cell
            label={t('takeProfit', language)}
            value={formatPrice(action.take_profit)}
            sub={
              action.take_profit && action.price
                ? calcPctChange(action.price, action.take_profit, isLong)
                : undefined
            }
            tone="text-up"
          />
          <Cell label={t('leverage', language)} value={`${action.leverage}x`} />
        </div>
      )}

      {isOpen && action.stop_loss && action.take_profit && action.price && (
        <div className="mt-2 flex items-center justify-between border-t border-line pt-2">
          <span className="text-[11px] text-fg-3">
            {t('riskReward', language)}
          </span>
          {(() => {
            const slDist = Math.abs(action.price - action.stop_loss)
            const tpDist = Math.abs(action.take_profit - action.price)
            const ratio = slDist > 0 ? tpDist / slDist : 0
            const bar =
              ratio >= 3 ? 'bg-up' : ratio >= 2 ? 'bg-brand' : 'bg-down'
            return (
              <div className="flex items-center gap-2">
                <span className="num text-xs">
                  <span className="text-down">1</span>
                  <span className="text-fg-3">:</span>
                  <span className="text-up">{ratio.toFixed(1)}</span>
                </span>
                <div className="h-1.5 w-[60px] rounded-full bg-line">
                  <div
                    className={cn('h-full rounded-full', bar)}
                    style={{ width: `${Math.min((ratio / 5) * 100, 100)}%` }}
                  />
                </div>
              </div>
            )
          })()}
        </div>
      )}

      {action.reasoning && (
        <div className="mt-2 flex gap-1.5 border-t border-line pt-2 text-xs text-fg-3">
          <Lightbulb className="mt-0.5 h-3 w-3 shrink-0" />
          <span className="line-clamp-2">{action.reasoning}</span>
        </div>
      )}

      {action.error && (
        <div className="mt-2 rounded border border-down/30 bg-down-soft p-2 text-xs text-down">
          {action.error}
        </div>
      )}
    </div>
  )
}

// Candidate watchlist: coins considered but with NO explicit decision, each
// with a status badge and the long/short gate-code verdicts.
function CandidateWatchlist({
  decision,
  language,
  onSymbolClick,
}: {
  decision: DecisionRecord
  language: Language
  onSymbolClick?: (symbol: string) => void
}) {
  // Only show coins that were considered but received NO explicit decision —
  // coins with decisions are already rendered as ActionCards above, and
  // repeating them here is duplication.
  const decidedSymbols = new Set(
    (decision.decisions || []).map((d) => d.symbol.toUpperCase())
  )
  const undecided = decision.candidate_coins.filter(
    (c) => !decidedSymbols.has(c.toUpperCase())
  )

  if (undecided.length === 0) return null

  // 2026-10-10 per-candidate block reasons. Old records have no verdicts and
  // render exactly as before. Filtered coins may be absent from
  // candidate_coins (dropped before the pool was persisted), so add them.
  const verdicts = new Map(
    (decision.candidate_verdicts || []).map((v) => [v.symbol.toUpperCase(), v])
  )
  const rows = [...undecided]
  verdicts.forEach((v, sym) => {
    if (
      v.status === 'filtered' &&
      !rows.some((r) => r.toUpperCase() === sym) &&
      !decidedSymbols.has(sym)
    ) {
      rows.push(v.symbol)
    }
  })
  const filteredCount = rows.filter(
    (s) => verdicts.get(s.toUpperCase())?.status === 'filtered'
  ).length
  const heldCount = rows.filter(
    (s) => verdicts.get(s.toUpperCase())?.status === 'held'
  ).length
  const evaluatedCount = rows.length - filteredCount - heldCount

  const codes = (list: string[] | undefined) =>
    !list || list.length === 0
      ? '—'
      : list.map((c, i) => {
          const { label, raw } = gateCodeLabel(c, language)
          return (
            <span key={c + i} title={raw}>
              {i > 0 ? ', ' : ''}
              {label}
            </span>
          )
        })

  return (
    <div className="mb-3 rounded-md border border-line bg-surface-2 px-3 py-2">
      <div className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold text-fg-3">
        <Target className="h-3 w-3" />
        <span>
          {t('candidateCoinsThisCycle', language)} ({evaluatedCount})
        </span>
        {filteredCount > 0 && (
          <span className="ml-1 font-normal opacity-70">
            {t('candidatesFiltered', language, { n: filteredCount })}
          </span>
        )}
      </div>
      <div className="space-y-1">
        {/* Coins the model stood down on this cycle */}
        {rows.map((symbol) => {
          const v = verdicts.get(symbol.toUpperCase())
          const isFiltered = v?.status === 'filtered'
          const isHeld = v?.status === 'held'
          return (
            <div key={symbol} className="flex items-start gap-2">
              <span
                className="num w-14 shrink-0 cursor-pointer text-xs font-semibold leading-[18px] text-fg-2 hover:underline"
                onClick={() => onSymbolClick?.(symbol)}
              >
                {symbol.replace('USDT', '')}
              </span>
              <Badge
                size="xs"
                variant="neutral"
                className={cn(
                  'shrink-0 bg-surface-hover',
                  (isFiltered || isHeld) && 'opacity-70'
                )}
              >
                {isHeld
                  ? t('candidateHeldBadge', language)
                  : isFiltered
                    ? t('candidateFilteredBadge', language)
                    : t('standingBy', language)}
              </Badge>
              {v && (
                <span className="min-w-0 break-words text-[11px] leading-[18px] text-fg-3">
                  {isHeld ? (
                    t('candidateHeldReason', language)
                  ) : isFiltered ? (
                    v.reason
                  ) : (
                    <>
                      {t('candidateLongLabel', language)}:{' '}
                      {codes(v.long_failed)}
                      {' | '}
                      {t('candidateShortLabel', language)}:{' '}
                      {codes(v.short_failed)}
                    </>
                  )}
                </span>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function PromptSection({
  title,
  tone,
  open,
  onToggle,
  text,
  copyLabel,
  filename,
  language,
  onCopy,
  onDownload,
}: {
  title: string
  tone: 'ai' | 'info' | 'brand'
  open: boolean
  onToggle: () => void
  text: string
  copyLabel: string
  filename: string
  language: Language
  onCopy: (text: string, label: string) => void
  onDownload: (text: string, filename: string) => void
}) {
  const color =
    tone === 'ai' ? 'text-ai' : tone === 'info' ? 'text-info' : 'text-brand'
  return (
    <div>
      <div
        role="button"
        tabIndex={0}
        aria-expanded={open}
        onClick={onToggle}
        onKeyDown={(e) => {
          if (e.target !== e.currentTarget) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onToggle()
          }
        }}
        className="flex h-8 w-full cursor-pointer items-center justify-between rounded-md px-2 text-[13px] transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50"
      >
        <div className="flex items-center gap-1.5">
          {open ? (
            <ChevronDown className="h-3.5 w-3.5 text-fg-3" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 text-fg-3" />
          )}
          <span className={cn('font-semibold', color)}>{title}</span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-1.5"
            title="Copy to clipboard"
            onClick={(e) => {
              e.stopPropagation()
              onCopy(text, copyLabel)
            }}
          >
            <Copy className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-1.5"
            title="Download as file"
            onClick={(e) => {
              e.stopPropagation()
              onDownload(text, filename)
            }}
          >
            <Download className="h-3.5 w-3.5" />
          </Button>
          <span className="ml-1 text-[11px] text-fg-3">
            {open ? t('collapse', language) : t('expand', language)}
          </span>
        </div>
      </div>
      {open && (
        <div className="mt-1 max-h-96 overflow-y-auto whitespace-pre-wrap rounded-md border border-line bg-surface-2 p-3 font-mono text-xs text-fg">
          {text}
        </div>
      )}
    </div>
  )
}

export function DecisionCard({
  decision,
  language,
  onSymbolClick,
}: DecisionCardProps) {
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)

  // Copy text to clipboard
  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success(`${label} copied!`)
    } catch (err) {
      console.error('Failed to copy:', err)
      toast.error('Copy failed')
    }
  }

  // Download text as file
  const downloadAsFile = (text: string, filename: string) => {
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  return (
    <div className="rounded-lg border border-line bg-surface p-3">
      {/* Header */}
      <div className="mb-3 flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Bot className="h-4 w-4 shrink-0 text-ai" />
          <span className="text-[13px] font-semibold text-fg">
            {t('cycle', language)} #
            <span className="num">{decision.cycle_number}</span>
          </span>
          <span className="num truncate text-xs text-fg-3">
            {new Date(decision.timestamp).toLocaleString()}
          </span>
        </div>
        <Badge variant={decision.success ? 'up' : 'down'}>
          {t(decision.success ? 'success' : 'failed', language)}
        </Badge>
      </div>

      {/* Decision actions */}
      {decision.decisions && decision.decisions.length > 0 && (
        <div className="mb-3 space-y-2">
          {decision.decisions.map((action, index) => (
            <ActionCard
              key={`${action.symbol}-${index}`}
              action={action}
              language={language}
              onSymbolClick={onSymbolClick}
            />
          ))}
        </div>
      )}

      {/* Candidate watchlist (per-candidate verdicts with gate-code chips) */}
      {decision.candidate_coins && decision.candidate_coins.length > 0 && (
        <CandidateWatchlist
          decision={decision}
          language={language}
          onSymbolClick={onSymbolClick}
        />
      )}

      {/* Collapsible sections */}
      <div className="space-y-0.5">
        {decision.system_prompt && (
          <PromptSection
            title="System Prompt"
            tone="ai"
            open={showSystemPrompt}
            onToggle={() => setShowSystemPrompt(!showSystemPrompt)}
            text={decision.system_prompt}
            copyLabel="System Prompt"
            filename={`system-prompt-cycle-${decision.cycle_number}.txt`}
            language={language}
            onCopy={copyToClipboard}
            onDownload={downloadAsFile}
          />
        )}
        {decision.input_prompt && (
          <PromptSection
            title="User Prompt"
            tone="info"
            open={showInputPrompt}
            onToggle={() => setShowInputPrompt(!showInputPrompt)}
            text={decision.input_prompt}
            copyLabel="User Prompt"
            filename={`user-prompt-cycle-${decision.cycle_number}.txt`}
            language={language}
            onCopy={copyToClipboard}
            onDownload={downloadAsFile}
          />
        )}
        {decision.cot_trace && (
          <PromptSection
            title={t('aiThinking', language)}
            tone="brand"
            open={showCoT}
            onToggle={() => setShowCoT(!showCoT)}
            text={decision.cot_trace}
            copyLabel="AI Thinking Chain"
            filename={`ai-thinking-cycle-${decision.cycle_number}.txt`}
            language={language}
            onCopy={copyToClipboard}
            onDownload={downloadAsFile}
          />
        )}
      </div>

      {/* Execution log */}
      {decision.execution_log && decision.execution_log.length > 0 && (
        <div className="num mt-3 space-y-1 rounded-md border border-line bg-surface-2 p-2 text-xs text-fg">
          {decision.execution_log.map((log, index) => (
            <div key={`${log}-${index}`}>{log}</div>
          ))}
        </div>
      )}

      {decision.error_message && (
        <div className="mt-3 rounded-md border border-down/40 bg-down-soft p-2 text-[13px] text-down">
          {decision.error_message}
        </div>
      )}
    </div>
  )
}
