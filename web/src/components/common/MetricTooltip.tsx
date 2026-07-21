import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { HelpCircle } from 'lucide-react'
import katex from 'katex'
import 'katex/dist/katex.min.css'
import { t } from '../../i18n/translations'

export interface MetricDefinition {
  key: string
  nameEn: string
  nameZh: string
  formula: string // LaTeX formula
  descriptionEn: string
  descriptionZh: string
}

// Metric definitions with formulas
export const METRIC_DEFINITIONS: Record<string, MetricDefinition> = {
  total_return: {
    key: 'total_return',
    nameEn: 'Total Return',
    nameZh: 'Total Return',
    formula: 'R_{total} = \\frac{V_{end} - V_{start}}{V_{start}} \\times 100\\%',
    descriptionEn: 'Measures overall portfolio performance from start to end',
    descriptionZh: 'Measures overall portfolio performance from start to end',
  },
  annualized_return: {
    key: 'annualized_return',
    nameEn: 'Annualized Return',
    nameZh: 'Annualized Return',
    formula: 'R_{ann} = \\left(1 + R_{total}\\right)^{\\frac{252}{n}} - 1',
    descriptionEn: 'Standardized yearly return rate (252 trading days)',
    descriptionZh: 'Standardized yearly return rate (252 trading days)',
  },
  max_drawdown: {
    key: 'max_drawdown',
    nameEn: 'Maximum Drawdown',
    nameZh: 'Maximum Drawdown',
    formula: 'MDD = \\max_{t} \\left( \\frac{Peak_t - Trough_t}{Peak_t} \\right)',
    descriptionEn: 'Largest peak-to-trough decline during the period',
    descriptionZh: 'Largest peak-to-trough decline during the period',
  },
  sharpe_ratio: {
    key: 'sharpe_ratio',
    nameEn: 'Sharpe Ratio',
    nameZh: 'Sharpe Ratio',
    formula: 'SR = \\frac{\\bar{r} - r_f}{\\sigma}',
    descriptionEn: 'Risk-adjusted return per unit of volatility (r̄=avg return, rf=risk-free rate, σ=std dev)',
    descriptionZh: 'Risk-adjusted return per unit of volatility (r̄=avg return, rf=risk-free rate, σ=std dev)',
  },
  sortino_ratio: {
    key: 'sortino_ratio',
    nameEn: 'Sortino Ratio',
    nameZh: 'Sortino Ratio',
    formula: 'Sortino = \\frac{\\bar{r} - r_f}{\\sigma_d}',
    descriptionEn: 'Return per unit of downside risk (σd=downside deviation)',
    descriptionZh: 'Return per unit of downside risk (σd=downside deviation)',
  },
  calmar_ratio: {
    key: 'calmar_ratio',
    nameEn: 'Calmar Ratio',
    nameZh: 'Calmar Ratio',
    formula: 'Calmar = \\frac{R_{ann}}{|MDD|}',
    descriptionEn: 'Annualized return divided by maximum drawdown',
    descriptionZh: 'Annualized return divided by maximum drawdown',
  },
  win_rate: {
    key: 'win_rate',
    nameEn: 'Win Rate',
    nameZh: 'Win Rate',
    formula: 'WinRate = \\frac{N_{win}}{N_{total}} \\times 100\\%',
    descriptionEn: 'Percentage of profitable trades',
    descriptionZh: 'Percentage of profitable trades',
  },
  profit_factor: {
    key: 'profit_factor',
    nameEn: 'Profit Factor',
    nameZh: 'Profit Factor',
    formula: 'PF = \\frac{\\sum Profits}{|\\sum Losses|}',
    descriptionEn: 'Ratio of gross profit to gross loss',
    descriptionZh: 'Ratio of gross profit to gross loss',
  },
  volatility: {
    key: 'volatility',
    nameEn: 'Volatility',
    nameZh: 'Volatility',
    formula: '\\sigma = \\sqrt{\\frac{1}{n}\\sum_{i=1}^{n}(r_i - \\bar{r})^2}',
    descriptionEn: 'Standard deviation of returns',
    descriptionZh: 'Standard deviation of returns',
  },
  var_95: {
    key: 'var_95',
    nameEn: 'VaR (95%)',
    nameZh: 'VaR (95%)',
    formula: 'P(R < VaR_{95\\%}) = 5\\%',
    descriptionEn: '95% confidence level maximum expected loss',
    descriptionZh: '95% confidence level maximum expected loss',
  },
  alpha: {
    key: 'alpha',
    nameEn: 'Alpha',
    nameZh: 'Alpha',
    formula: '\\alpha = R_{portfolio} - R_{benchmark}',
    descriptionEn: 'Excess return over benchmark',
    descriptionZh: 'Excess return over benchmark',
  },
  beta: {
    key: 'beta',
    nameEn: 'Beta',
    nameZh: 'Beta',
    formula: '\\beta = \\frac{Cov(R_p, R_m)}{Var(R_m)}',
    descriptionEn: 'Portfolio sensitivity to market movements',
    descriptionZh: 'Portfolio sensitivity to market movements',
  },
  information_ratio: {
    key: 'information_ratio',
    nameEn: 'Information Ratio',
    nameZh: 'Information Ratio',
    formula: 'IR = \\frac{\\alpha}{\\sigma_{tracking}}',
    descriptionEn: 'Alpha per unit of tracking error',
    descriptionZh: 'Alpha per unit of tracking error',
  },
  avg_trade_pnl: {
    key: 'avg_trade_pnl',
    nameEn: 'Avg Trade PnL',
    nameZh: 'Avg Trade PnL',
    formula: '\\bar{PnL} = \\frac{\\sum PnL_i}{N}',
    descriptionEn: 'Average profit/loss per trade',
    descriptionZh: 'Average profit/loss per trade',
  },
  expectancy: {
    key: 'expectancy',
    nameEn: 'Expectancy',
    nameZh: 'Expectancy',
    formula: 'E = (WinRate \\times \\bar{W}) - (LossRate \\times \\bar{L})',
    descriptionEn: 'Expected return per trade',
    descriptionZh: 'Expected return per trade',
  },
}

interface FormulaRendererProps {
  formula: string
  displayMode?: boolean
}

function FormulaRenderer({ formula, displayMode = true }: FormulaRendererProps) {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (containerRef.current) {
      try {
        katex.render(formula, containerRef.current, {
          throwOnError: false,
          displayMode,
          output: 'html',
        })
      } catch (e) {
        console.error('KaTeX render error:', e)
        containerRef.current.textContent = formula
      }
    }
  }, [formula, displayMode])

  return <div ref={containerRef} className="formula-container" />
}

interface TooltipPosition {
  top: number
  left: number
  placement: 'top' | 'bottom'
}

interface MetricTooltipProps {
  metricKey: string
  language?: string
  size?: number
  className?: string
}

export function MetricTooltip({
  metricKey,
  language = 'en',
  size = 14,
  className = '',
}: MetricTooltipProps) {
  const [show, setShow] = useState(false)
  const [position, setPosition] = useState<TooltipPosition>({ top: 100, left: 100, placement: 'bottom' })
  const buttonRef = useRef<HTMLButtonElement>(null)
  const tooltipWidth = 340
  const tooltipHeight = 220

  const metric = METRIC_DEFINITIONS[metricKey]

  const calculatePosition = useCallback(() => {
    if (!buttonRef.current) return

    const rect = buttonRef.current.getBoundingClientRect()
    const viewportHeight = window.innerHeight
    const viewportWidth = window.innerWidth

    // Calculate center position (fixed positioning uses viewport coordinates)
    let left = rect.left + rect.width / 2 - tooltipWidth / 2

    // Clamp to viewport bounds with padding
    const padding = 16
    left = Math.max(padding, Math.min(left, viewportWidth - tooltipWidth - padding))

    // Decide placement: prefer bottom for reliability
    const spaceBelow = viewportHeight - rect.bottom

    let placement: 'top' | 'bottom' = 'bottom'
    let top: number

    if (spaceBelow >= tooltipHeight + 20) {
      // Enough space below
      placement = 'bottom'
      top = rect.bottom + 8
    } else {
      // Show above
      placement = 'top'
      top = Math.max(8, rect.top - tooltipHeight - 8)
    }

    // Ensure top is never negative
    top = Math.max(8, top)

    setPosition({ top, left, placement })
  }, [])

  const handleMouseEnter = useCallback(() => {
    calculatePosition()
    setShow(true)
  }, [calculatePosition])

  const handleMouseLeave = useCallback(() => {
    setShow(false)
  }, [])

  if (!metric) {
    return null
  }

  const name = language === 'zh' ? metric.nameZh : metric.nameEn
  const description = language === 'zh' ? metric.descriptionZh : metric.descriptionEn
  const formulaLabel = t('metricTooltip.formula', language as 'en' | 'zh' | 'id' | 'ja')

  const tooltipContent = (
    <div
      onMouseEnter={() => setShow(true)}
      onMouseLeave={() => setShow(false)}
      style={{
        position: 'fixed',
        top: `${position.top}px`,
        left: `${position.left}px`,
        width: `${tooltipWidth}px`,
        zIndex: 99999,
        pointerEvents: 'auto',
      }}
    >
      <div
        style={{
          background: '#F7F4EC',
          border: '1px solid rgba(26,24,19,0.14)',
          borderRadius: '12px',
          padding: '16px',
          boxShadow: '0 25px 50px -12px rgba(26, 24, 19, 0.18)',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          marginBottom: '12px',
          paddingBottom: '8px',
          borderBottom: '1px solid rgba(26,24,19,0.14)'
        }}>
          <div style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: '#E0483B'
          }} />
          <span style={{ fontWeight: 'bold', fontSize: '14px', color: '#1A1813' }}>
            {name}
          </span>
        </div>

        {/* Formula */}
        <div style={{
          background: '#E8E2D5',
          borderRadius: '8px',
          padding: '12px',
          marginBottom: '12px'
        }}>
          <div style={{ fontSize: '12px', color: '#8A8478', marginBottom: '8px' }}>
            {formulaLabel}
          </div>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            padding: '8px 4px',
            color: '#1A1813',
            overflowX: 'auto',
            overflowY: 'hidden',
            maxWidth: '100%',
            WebkitOverflowScrolling: 'touch',
          }}>
            <FormulaRenderer formula={metric.formula} displayMode={false} />
          </div>
        </div>

        {/* Description */}
        <p style={{ fontSize: '12px', lineHeight: '1.5', color: '#8A8478', margin: 0 }}>
          {description}
        </p>
      </div>
    </div>
  )

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        onClick={(e) => {
          e.stopPropagation()
          if (!show) {
            calculatePosition()
          }
          setShow(!show)
        }}
        className={`p-0.5 rounded-full transition-colors hover:bg-[rgba(26,24,19,0.06)] ${className}`}
        style={{ color: '#8A8478' }}
        aria-label={`Info about ${name}`}
      >
        <HelpCircle size={size} />
      </button>

      {show && createPortal(tooltipContent, document.body)}
    </>
  )
}

// Convenience component for inline metric label with tooltip
interface MetricLabelProps {
  metricKey: string
  label?: string
  language?: string
  className?: string
}

export function MetricLabel({ metricKey, label, language = 'en', className = '' }: MetricLabelProps) {
  const metric = METRIC_DEFINITIONS[metricKey]
  const displayLabel = label || (language === 'zh' ? metric?.nameZh : metric?.nameEn) || metricKey

  return (
    <span className={`inline-flex items-center gap-1 ${className}`}>
      {displayLabel}
      <MetricTooltip metricKey={metricKey} language={language} size={12} />
    </span>
  )
}
