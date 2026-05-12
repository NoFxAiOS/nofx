import { useState } from 'react'
import type { DecisionRecord, DecisionAction } from '../../types'
import { t, type Language } from '../../i18n/translations'

interface DecisionCardProps {
  decision: DecisionRecord
  language: Language
  onSymbolClick?: (symbol: string) => void
}

// Action type configuration
const ACTION_CONFIG: Record<
  string,
  { color: string; bg: string; icon: string; label: string }
> = {
  open_long: {
    color: '#0ECB81',
    bg: 'rgba(14, 203, 129, 0.15)',
    icon: '📈',
    label: 'LONG',
  },
  open_short: {
    color: '#F6465D',
    bg: 'rgba(246, 70, 93, 0.15)',
    icon: '📉',
    label: 'SHORT',
  },
  close_long: {
    color: '#F0B90B',
    bg: 'rgba(240, 185, 11, 0.15)',
    icon: '💰',
    label: 'CLOSE',
  },
  close_short: {
    color: '#F0B90B',
    bg: 'rgba(240, 185, 11, 0.15)',
    icon: '💰',
    label: 'CLOSE',
  },
  hold: {
    color: '#848E9C',
    bg: 'rgba(132, 142, 156, 0.15)',
    icon: '⏸️',
    label: 'HOLD',
  },
  wait: {
    color: '#848E9C',
    bg: 'rgba(132, 142, 156, 0.15)',
    icon: '⏳',
    label: 'WAIT',
  },
}

// Format price with proper decimals
function formatPrice(price: number | undefined): string {
  if (!price || price === 0) return '-'
  if (price >= 1000) return price.toFixed(2)
  if (price >= 1) return price.toFixed(4)
  return price.toFixed(6)
}

// Get confidence color
function getConfidenceColor(confidence: number | undefined): string {
  if (!confidence) return '#848E9C'
  if (confidence >= 80) return '#0ECB81'
  if (confidence >= 60) return '#F0B90B'
  return '#F6465D'
}

function formatControlDecisionLabel(
  decision?: string
): { label: string; tone: 'danger' | 'warn' | 'neutral' } | null {
  const normalized = String(decision || '')
    .trim()
    .toLowerCase()
  if (!normalized) return null
  if (normalized === 'rejected') return { label: 'rejected', tone: 'danger' }
  if (normalized === 'downgraded_to_wait')
    return { label: 'downgraded to wait', tone: 'warn' }
  if (normalized === 'accepted') return { label: 'accepted', tone: 'neutral' }
  return { label: normalized.replace(/_/g, ' '), tone: 'neutral' }
}

function toneColors(tone: 'danger' | 'warn' | 'neutral') {
  if (tone === 'danger')
    return {
      border: '1px solid rgba(246, 70, 93, 0.25)',
      bg: 'rgba(246, 70, 93, 0.12)',
      color: '#FCA5A5',
    }
  if (tone === 'warn')
    return {
      border: '1px solid rgba(240, 185, 11, 0.25)',
      bg: 'rgba(240, 185, 11, 0.12)',
      color: '#FCD34D',
    }
  return {
    border: '1px solid rgba(56, 189, 248, 0.25)',
    bg: 'rgba(56, 189, 248, 0.12)',
    color: '#7DD3FC',
  }
}

// Single Action Card Component — Layered Design
function ActionCard({
  action,
  language,
  onSymbolClick,
}: {
  action: DecisionAction
  language: Language
  onSymbolClick?: (symbol: string) => void
}) {
  const [showDetails, setShowDetails] = useState(false)
  const config = ACTION_CONFIG[action.action] || ACTION_CONFIG.wait
  const isOpen = action.action.includes('open')
  const isClose = action.action.includes('close')
  const isHoldWait = !isOpen && !isClose
  const control = action.review_context?.control
  const controlStatus = formatControlDecisionLabel(control?.decision)
  const review = action.review_context
  const selectedLevels = review?.selected_levels || []

  // Hold/Wait: compact single-line card
  if (isHoldWait) {
    return (
      <div
        className="rounded-lg px-3 py-2 flex items-center gap-2"
        style={{
          background: '#1A1E23',
          border: '1px solid #2B3139',
        }}
      >
        <span className="text-sm">{config.icon}</span>
        <span
          className="px-2 py-0.5 rounded text-[10px] font-bold uppercase"
          style={{ background: config.bg, color: config.color }}
        >
          {config.label}
        </span>
        <span
          className="font-mono text-xs cursor-pointer hover:underline"
          style={{ color: '#EAECEF' }}
          onClick={() => onSymbolClick?.(action.symbol)}
        >
          {action.symbol.replace('USDT', '')}
        </span>
        {action.confidence !== undefined && action.confidence > 0 && (
          <span className="text-[10px]" style={{ color: '#848E9C' }}>
            {action.confidence}%
          </span>
        )}
        <span className="flex-1 text-xs truncate" style={{ color: '#848E9C' }}>
          {action.reasoning}
        </span>
        {controlStatus && (
          <span
            className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium shrink-0"
            style={toneColors(controlStatus.tone)}
          >
            {controlStatus.label}
          </span>
        )}
      </div>
    )
  }

  // Open/Close: layered card
  return (
    <div
      className="rounded-lg p-4 transition-all duration-200 hover:scale-[1.01]"
      style={{
        background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
        border: `1px solid ${config.color}33`,
        boxShadow: `0 4px 12px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.03)`,
      }}
    >
      {/* Layer 1: Core Summary */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-lg">{config.icon}</span>
          <span
            className="font-mono font-bold text-base cursor-pointer hover:scale-110 transition-transform"
            style={{ color: '#EAECEF' }}
            onClick={() => onSymbolClick?.(action.symbol)}
          >
            {action.symbol.replace('USDT', '')}
          </span>
          <span
            className="px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider"
            style={{
              background: config.bg,
              color: config.color,
              border: `1px solid ${config.color}55`,
            }}
          >
            {config.label}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {action.confidence !== undefined && action.confidence > 0 && (
            <span
              className="px-2 py-0.5 rounded text-xs font-semibold"
              style={{
                background: `${getConfidenceColor(action.confidence)}22`,
                color: getConfidenceColor(action.confidence),
              }}
            >
              {action.confidence}%
            </span>
          )}
          {/* RR ratio inline */}
          {isOpen &&
            action.stop_loss &&
            action.take_profit &&
            action.price &&
            (() => {
              const slDist = Math.abs(action.price - action.stop_loss)
              const tpDist = Math.abs(action.take_profit - action.price)
              const ratio = slDist > 0 ? tpDist / slDist : 0
              return (
                <span
                  className="px-2 py-0.5 rounded text-[10px] font-semibold"
                  style={{
                    background:
                      ratio >= 2
                        ? 'rgba(14, 203, 129, 0.15)'
                        : 'rgba(240, 185, 11, 0.15)',
                    color:
                      ratio >= 3
                        ? '#0ECB81'
                        : ratio >= 2
                          ? '#F0B90B'
                          : '#F6465D',
                  }}
                >
                  RR 1:{ratio.toFixed(1)}
                </span>
              )
            })()}
          {controlStatus && (
            <span
              className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium"
              style={toneColors(controlStatus.tone)}
            >
              {controlStatus.label}
            </span>
          )}
        </div>
      </div>

      {/* Layer 2: Selected Levels / Structure Basis */}
      {selectedLevels.length > 0 && (
        <div className="mt-3 pt-3" style={{ borderTop: '1px solid #2B3139' }}>
          <div className="text-[10px] mb-1.5" style={{ color: '#848E9C' }}>
            AI Selected Levels
          </div>
          <div className="flex flex-wrap gap-1.5">
            {selectedLevels.map((level, idx) => {
              const isSL =
                level.used_for === 'stop_loss' ||
                level.used_for === 'invalidation'
              const isTP =
                level.used_for.startsWith('tp') ||
                level.used_for === 'take_profit'
              const color = isSL ? '#F6465D' : isTP ? '#0ECB81' : '#F0B90B'
              const basisIcon =
                level.basis_type === 'structural'
                  ? '🎯'
                  : level.basis_type === 'atr_based'
                    ? '📐'
                    : level.basis_type === 'fibonacci'
                      ? '🌀'
                      : '📊'
              return (
                <span
                  key={idx}
                  className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-[10px]"
                  style={{
                    background: `${color}15`,
                    border: `1px solid ${color}30`,
                    color,
                  }}
                  title={level.reason || ''}
                >
                  {basisIcon} {level.used_for}: {formatPrice(level.price)}
                  {level.timeframe && (
                    <span style={{ color: '#848E9C' }}>
                      {' '}
                      ({level.timeframe})
                    </span>
                  )}
                </span>
              )
            })}
          </div>
        </div>
      )}

      {/* Fallback: show key_levels if no selected_levels (old data) */}
      {selectedLevels.length === 0 && review?.key_levels && (
        <div className="mt-3 pt-3" style={{ borderTop: '1px solid #2B3139' }}>
          <div className="flex flex-wrap gap-1.5 text-[10px]">
            {(review.key_levels.support || []).slice(0, 2).map((level) => (
              <span
                key={`s-${level}`}
                className="inline-flex items-center rounded px-2 py-0.5"
                style={{
                  border: '1px solid rgba(14, 203, 129, 0.25)',
                  background: 'rgba(14, 203, 129, 0.12)',
                  color: '#86EFAC',
                }}
              >
                S {formatPrice(level)}
              </span>
            ))}
            {(review.key_levels.resistance || []).slice(0, 2).map((level) => (
              <span
                key={`r-${level}`}
                className="inline-flex items-center rounded px-2 py-0.5"
                style={{
                  border: '1px solid rgba(246, 70, 93, 0.25)',
                  background: 'rgba(246, 70, 93, 0.12)',
                  color: '#FDA4AF',
                }}
              >
                R {formatPrice(level)}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Reasoning (always visible for open/close) */}
      {action.reasoning && (
        <div className="mt-2 text-xs" style={{ color: '#848E9C' }}>
          {action.reasoning}
        </div>
      )}

      {/* Layer 3: Expandable Details */}
      <div className="mt-3 pt-2" style={{ borderTop: '1px solid #2B3139' }}>
        <button
          onClick={() => setShowDetails(!showDetails)}
          className="text-[10px] font-medium transition-colors hover:opacity-80"
          style={{ color: '#848E9C' }}
        >
          {showDetails ? '▼ Hide details' : '▶ Trading details & audit'}
        </button>

        {showDetails && (
          <div className="mt-2 space-y-3">
            {/* Trading Details */}
            {isOpen && (
              <div className="grid grid-cols-4 gap-2 text-center">
                <div>
                  <div className="text-[10px]" style={{ color: '#848E9C' }}>
                    {t('entryPrice', language)}
                  </div>
                  <div
                    className="font-mono text-xs font-semibold"
                    style={{ color: '#EAECEF' }}
                  >
                    {formatPrice(action.price)}
                  </div>
                </div>
                {action.stop_loss && (
                  <div>
                    <div className="text-[10px]" style={{ color: '#848E9C' }}>
                      SL
                    </div>
                    <div
                      className="font-mono text-xs font-semibold"
                      style={{ color: '#F6465D' }}
                    >
                      {formatPrice(action.stop_loss)}
                    </div>
                  </div>
                )}
                {action.take_profit && (
                  <div>
                    <div className="text-[10px]" style={{ color: '#848E9C' }}>
                      TP
                    </div>
                    <div
                      className="font-mono text-xs font-semibold"
                      style={{ color: '#0ECB81' }}
                    >
                      {formatPrice(action.take_profit)}
                    </div>
                  </div>
                )}
                {action.leverage > 0 && (
                  <div>
                    <div className="text-[10px]" style={{ color: '#848E9C' }}>
                      {t('leverage', language)}
                    </div>
                    <div
                      className="font-mono text-xs font-semibold"
                      style={{ color: '#F0B90B' }}
                    >
                      {action.leverage}x
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Protection Plan Summary */}
            {review?.protection && (
              <div
                className="text-[10px] space-y-1"
                style={{ color: '#9CA3AF' }}
              >
                <div className="flex flex-wrap gap-1.5">
                  {review.protection.stop_beyond_invalidation && (
                    <span style={{ color: '#0ECB81' }}>
                      SL beyond invalidation
                    </span>
                  )}
                  {review.protection.target_aligned && (
                    <span style={{ color: '#0ECB81' }}>TP aligned</span>
                  )}
                  {review.protection.policy_status && (
                    <span>{review.protection.policy_status}</span>
                  )}
                </div>
                {review.protection.notes &&
                  review.protection.notes.length > 0 && (
                    <div>{review.protection.notes.slice(0, 2).join(' | ')}</div>
                  )}
              </div>
            )}

            {/* Gate Attribution (only for rejected/downgraded) */}
            {review?.quality_gate && !review.quality_gate.passed && (
              <div
                className="rounded p-2"
                style={{
                  background: 'rgba(246, 70, 93, 0.08)',
                  border: '1px solid rgba(246, 70, 93, 0.2)',
                }}
              >
                <div
                  className="text-[10px] font-medium mb-1"
                  style={{ color: '#F6465D' }}
                >
                  Gate: {review.quality_gate.decision || 'blocked'}
                  {review.quality_gate.blocked_stage &&
                    ` @ ${review.quality_gate.blocked_stage}`}
                </div>
                {review.quality_gate.gate_checks && (
                  <div className="space-y-0.5">
                    {review.quality_gate.gate_checks
                      .filter((gc) => !gc.passed)
                      .slice(0, 4)
                      .map((gc, i) => (
                        <div
                          key={i}
                          className="text-[10px]"
                          style={{ color: '#FDA4AF' }}
                        >
                          {gc.enforced ? '✗' : '⚠'} {gc.code}
                          {gc.detail ? `: ${gc.detail}` : ''}
                        </div>
                      ))}
                  </div>
                )}
              </div>
            )}

            {/* Control outcome details (only if not already shown via gate) */}
            {control && !review?.quality_gate && (
              <div
                className="text-[10px] space-y-0.5"
                style={{ color: '#9CA3AF' }}
              >
                {control.failed_checks && control.failed_checks.length > 0 && (
                  <div>Checks: {control.failed_checks.join(', ')}</div>
                )}
                {control.reasons && control.reasons.length > 0 && (
                  <div>Reason: {control.reasons.join(' | ')}</div>
                )}
                {control.regime_current && (
                  <div>Regime: {control.regime_current}</div>
                )}
              </div>
            )}
          </div>
        )}
      </div>
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
      alert(`${label} copied!`)
    } catch (err) {
      console.error('Failed to copy:', err)
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
    <div
      className="rounded-xl p-5 transition-all duration-300 hover:translate-y-[-2px]"
      style={{
        border: '1px solid #2B3139',
        background: 'linear-gradient(180deg, #1E2329 0%, #181C21 100%)',
        boxShadow: '0 4px 16px rgba(0, 0, 0, 0.3)',
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div
            className="w-10 h-10 rounded-lg flex items-center justify-center"
            style={{ background: 'rgba(240, 185, 11, 0.15)' }}
          >
            <span className="text-xl">🤖</span>
          </div>
          <div>
            <div className="font-bold" style={{ color: '#EAECEF' }}>
              {t('cycle', language)} #{decision.cycle_number}
            </div>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {new Date(decision.timestamp).toLocaleString()}
            </div>
          </div>
        </div>
        <div
          className="px-4 py-1.5 rounded-full text-xs font-bold tracking-wider"
          style={
            decision.success
              ? {
                  background: 'rgba(14, 203, 129, 0.15)',
                  color: '#0ECB81',
                  border: '1px solid rgba(14, 203, 129, 0.3)',
                }
              : {
                  background: 'rgba(246, 70, 93, 0.15)',
                  color: '#F6465D',
                  border: '1px solid rgba(246, 70, 93, 0.3)',
                }
          }
        >
          {t(decision.success ? 'success' : 'failed', language)}
        </div>
      </div>

      {/* AI Control Snapshot */}
      <div className="flex flex-wrap items-center gap-2 mb-4">
        <div
          className="px-2.5 py-1 rounded-full text-[11px] font-semibold"
          style={{
            background: 'rgba(168, 85, 247, 0.12)',
            color: '#C084FC',
            border: '1px solid rgba(168,85,247,0.25)',
          }}
        >
          AI Open: {decision.allow_ai_open === false ? 'OFF' : 'ON'}
        </div>
        <div
          className="px-2.5 py-1 rounded-full text-[11px] font-semibold"
          style={{
            background: 'rgba(240, 185, 11, 0.12)',
            color: '#F0B90B',
            border: '1px solid rgba(240,185,11,0.25)',
          }}
        >
          SL: {decision.allow_ai_stop_close === false ? 'OFF' : 'ON'} | TP:{' '}
          {decision.allow_ai_take_profit === false ? 'OFF' : 'ON'}
        </div>
        <div
          className="px-2.5 py-1 rounded-full text-[11px] font-semibold"
          style={{
            background: 'rgba(96, 165, 250, 0.12)',
            color: '#60A5FA',
            border: '1px solid rgba(96,165,250,0.25)',
          }}
        >
          Mode: {decision.ai_decision_mode || 'balanced'}
        </div>
      </div>

      {/* Decision Actions - Beautiful Grid */}
      {decision.decisions && decision.decisions.length > 0 && (
        <div className="space-y-3 mb-4">
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

      {/* Collapsible Sections */}
      <div className="space-y-2">
        {/* System Prompt */}
        {decision.system_prompt && (
          <div>
            <button
              onClick={() => setShowSystemPrompt(!showSystemPrompt)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded hover:bg-white/5"
            >
              <div className="flex items-center gap-2">
                <span className="text-base">⚙️</span>
                <span className="font-semibold" style={{ color: '#a78bfa' }}>
                  System Prompt
                </span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    copyToClipboard(decision.system_prompt, 'System Prompt')
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{
                    background: 'rgba(167, 139, 250, 0.2)',
                    color: '#a78bfa',
                    border: '1px solid rgba(167, 139, 250, 0.3)',
                  }}
                  title="Copy to clipboard"
                >
                  <span>📋</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    downloadAsFile(
                      decision.system_prompt,
                      `system-prompt-cycle-${decision.cycle_number}.txt`
                    )
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{
                    background: 'rgba(167, 139, 250, 0.2)',
                    color: '#a78bfa',
                    border: '1px solid rgba(167, 139, 250, 0.3)',
                  }}
                  title="Download as file"
                >
                  <span>💾</span>
                </button>
                <span
                  className="text-xs px-2 py-0.5 rounded"
                  style={{
                    background: 'rgba(167, 139, 250, 0.15)',
                    color: '#a78bfa',
                  }}
                >
                  {showSystemPrompt
                    ? t('collapse', language)
                    : t('expand', language)}
                </span>
              </div>
            </button>
            {showSystemPrompt && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                {decision.system_prompt}
              </div>
            )}
          </div>
        )}

        {/* User/Input Prompt */}
        {decision.input_prompt && (
          <div>
            <button
              onClick={() => setShowInputPrompt(!showInputPrompt)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded hover:bg-white/5"
            >
              <div className="flex items-center gap-2">
                <span className="text-base">📥</span>
                <span className="font-semibold" style={{ color: '#60a5fa' }}>
                  User Prompt
                </span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    copyToClipboard(decision.input_prompt, 'User Prompt')
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{
                    background: 'rgba(96, 165, 250, 0.2)',
                    color: '#60a5fa',
                    border: '1px solid rgba(96, 165, 250, 0.3)',
                  }}
                  title="Copy to clipboard"
                >
                  <span>📋</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    downloadAsFile(
                      decision.input_prompt,
                      `user-prompt-cycle-${decision.cycle_number}.txt`
                    )
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{
                    background: 'rgba(96, 165, 250, 0.2)',
                    color: '#60a5fa',
                    border: '1px solid rgba(96, 165, 250, 0.3)',
                  }}
                  title="Download as file"
                >
                  <span>💾</span>
                </button>
                <span
                  className="text-xs px-2 py-0.5 rounded"
                  style={{
                    background: 'rgba(96, 165, 250, 0.15)',
                    color: '#60a5fa',
                  }}
                >
                  {showInputPrompt
                    ? t('collapse', language)
                    : t('expand', language)}
                </span>
              </div>
            </button>
            {showInputPrompt && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                {decision.input_prompt}
              </div>
            )}
          </div>
        )}

        {/* AI Thinking */}
        {decision.cot_trace && (
          <div>
            <button
              onClick={() => setShowCoT(!showCoT)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded hover:bg-white/5"
            >
              <div className="flex items-center gap-2">
                <span className="text-base">🧠</span>
                <span className="font-semibold" style={{ color: '#F0B90B' }}>
                  {t('aiThinking', language)}
                </span>
              </div>
              <span
                className="text-xs px-2 py-0.5 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.15)',
                  color: '#F0B90B',
                }}
              >
                {showCoT ? t('collapse', language) : t('expand', language)}
              </span>
            </button>
            {showCoT && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                {decision.cot_trace}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Execution Log */}
      {decision.execution_log && decision.execution_log.length > 0 && (
        <div
          className="rounded-lg p-3 mt-4 text-xs font-mono space-y-1"
          style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
        >
          {decision.execution_log.map((log, index) => (
            <div key={`${log}-${index}`} style={{ color: '#EAECEF' }}>
              {log}
            </div>
          ))}
        </div>
      )}

      {/* Error Message */}
      {decision.error_message && (
        <div
          className="rounded-lg p-3 mt-4 text-sm"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.4)',
            color: '#F6465D',
          }}
        >
          ❌ {decision.error_message}
        </div>
      )}
    </div>
  )
}
