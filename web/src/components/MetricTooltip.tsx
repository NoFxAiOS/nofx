import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { HelpCircle } from 'lucide-react'
import katex from 'katex'
import 'katex/dist/katex.min.css'

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
    nameZh: '总收益率',
    formula: 'R_{total} = \\frac{V_{end} - V_{start}}{V_{start}} \\times 100\\%',
    descriptionEn: 'Measures overall portfolio performance from start to end',
    descriptionZh: '衡量投资组合从开始到结束的整体收益表现',
  },
  annualized_return: {
    key: 'annualized_return',
    nameEn: 'Annualized Return',
    nameZh: '年化收益率',
    formula: 'R_{ann} = \\left(1 + R_{total}\\right)^{\\frac{252}{n}} - 1',
    descriptionEn: 'Standardized yearly return rate (252 trading days)',
    descriptionZh: '标准化年度收益率（按252个交易日计算）',
  },
  max_drawdown: {
    key: 'max_drawdown',
    nameEn: 'Maximum Drawdown',
    nameZh: '最大回撤',
    formula: 'MDD = \\max_{t} \\left( \\frac{Peak_t - Trough_t}{Peak_t} \\right)',
    descriptionEn: 'Largest peak-to-trough decline during the period',
    descriptionZh: '期间内从峰值到谷底的最大跌幅',
  },
  sharpe_ratio: {
    key: 'sharpe_ratio',
    nameEn: 'Sharpe Ratio',
    nameZh: '夏普比率',
    formula: 'SR = \\frac{\\bar{r} - r_f}{\\sigma}',
    descriptionEn: 'Risk-adjusted return per unit of volatility (r̄=avg return, rf=risk-free rate, σ=std dev)',
    descriptionZh: '单位波动风险下的超额收益（r̄=平均收益，rf=无风险利率，σ=标准差）',
  },
  sortino_ratio: {
    key: 'sortino_ratio',
    nameEn: 'Sortino Ratio',
    nameZh: '索提诺比率',
    formula: 'Sortino = \\frac{\\bar{r} - r_f}{\\sigma_d}',
    descriptionEn: 'Return per unit of downside risk (σd=downside deviation)',
    descriptionZh: '单位下行风险的收益（σd=下行标准差）',
  },
  calmar_ratio: {
    key: 'calmar_ratio',
    nameEn: 'Calmar Ratio',
    nameZh: '卡玛比率',
    formula: 'Calmar = \\frac{R_{ann}}{|MDD|}',
    descriptionEn: 'Annualized return divided by maximum drawdown',
    descriptionZh: '年化收益率与最大回撤的比值',
  },
  win_rate: {
    key: 'win_rate',
    nameEn: 'Win Rate',
    nameZh: '胜率',
    formula: 'WinRate = \\frac{N_{win}}{N_{total}} \\times 100\\%',
    descriptionEn: 'Percentage of profitable trades',
    descriptionZh: '盈利交易占总交易数的百分比',
  },
  profit_factor: {
    key: 'profit_factor',
    nameEn: 'Profit Factor',
    nameZh: '盈亏比',
    formula: 'PF = \\frac{\\sum Profits}{|\\sum Losses|}',
    descriptionEn: 'Ratio of gross profit to gross loss',
    descriptionZh: '总盈利与总亏损的比值',
  },
  volatility: {
    key: 'volatility',
    nameEn: 'Volatility',
    nameZh: '波动率',
    formula: '\\sigma = \\sqrt{\\frac{1}{n}\\sum_{i=1}^{n}(r_i - \\bar{r})^2}',
    descriptionEn: 'Standard deviation of returns',
    descriptionZh: '收益率的标准差',
  },
  var_95: {
    key: 'var_95',
    nameEn: 'VaR (95%)',
    nameZh: '风险价值',
    formula: 'P(R < VaR_{95\\%}) = 5\\%',
    descriptionEn: '95% confidence level maximum expected loss',
    descriptionZh: '95%置信水平下的最大预期损失',
  },
  alpha: {
    key: 'alpha',
    nameEn: 'Alpha',
    nameZh: '超额收益',
    formula: '\\alpha = R_{portfolio} - R_{benchmark}',
    descriptionEn: 'Excess return over benchmark',
    descriptionZh: '相对于基准的超额收益',
  },
  beta: {
    key: 'beta',
    nameEn: 'Beta',
    nameZh: '贝塔系数',
    formula: '\\beta = \\frac{Cov(R_p, R_m)}{Var(R_m)}',
    descriptionEn: 'Portfolio sensitivity to market movements',
    descriptionZh: '投资组合对市场波动的敏感度',
  },
  information_ratio: {
    key: 'information_ratio',
    nameEn: 'Information Ratio',
    nameZh: '信息比率',
    formula: 'IR = \\frac{\\alpha}{\\sigma_{tracking}}',
    descriptionEn: 'Alpha per unit of tracking error',
    descriptionZh: '单位跟踪误差的超额收益',
  },
  avg_trade_pnl: {
    key: 'avg_trade_pnl',
    nameEn: 'Avg Trade PnL',
    nameZh: '平均盈亏',
    formula: '\\bar{PnL} = \\frac{\\sum PnL_i}{N}',
    descriptionEn: 'Average profit/loss per trade',
    descriptionZh: '每笔交易的平均盈亏',
  },
  expectancy: {
    key: 'expectancy',
    nameEn: 'Expectancy',
    nameZh: '期望收益',
    formula: 'E = (WinRate \\times \\bar{W}) - (LossRate \\times \\bar{L})',
    descriptionEn: 'Expected return per trade',
    descriptionZh: '每笔交易的期望收益',
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
          background: 'linear-gradient(145deg, #1E2329 0%, #2B3139 100%)',
          border: '1px solid #3B4149',
          borderRadius: '12px',
          padding: '16px',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.8)',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          marginBottom: '12px',
          paddingBottom: '8px',
          borderBottom: '1px solid #3B4149'
        }}>
          <div style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: '#F0B90B'
          }} />
          <span style={{ fontWeight: 'bold', fontSize: '14px', color: '#EAECEF' }}>
            {name}
          </span>
        </div>

        {/* Formula */}
        <div style={{
          background: 'rgba(0,0,0,0.3)',
          borderRadius: '8px',
          padding: '12px',
          marginBottom: '12px'
        }}>
          <div style={{ fontSize: '12px', color: '#848E9C', marginBottom: '8px' }}>
            {language === 'zh' ? '计算公式' : 'Formula'}
          </div>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            padding: '8px 4px',
            color: '#EAECEF',
            overflowX: 'auto',
            overflowY: 'hidden',
            maxWidth: '100%',
            WebkitOverflowScrolling: 'touch',
          }}>
            <FormulaRenderer formula={metric.formula} displayMode={false} />
          </div>
        </div>

        {/* Description */}
        <p style={{ fontSize: '12px', lineHeight: '1.5', color: '#B7BDC6', margin: 0 }}>
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
        className={`p-0.5 rounded-full transition-colors hover:bg-white/10 ${className}`}
        style={{ color: '#848E9C' }}
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
