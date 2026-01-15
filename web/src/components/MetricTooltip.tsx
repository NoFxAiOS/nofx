import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { HelpCircle } from 'lucide-react'
import katex from 'katex'
import 'katex/dist/katex.min.css'
import { t, type Language } from '../i18n/translations'

export interface MetricDefinition {
  key: string
  name: Partial<Record<Language, string>> & { en: string }
  formula: string // LaTeX formula
  description: Partial<Record<Language, string>> & { en: string }
}

// Metric definitions with formulas
export const METRIC_DEFINITIONS: Record<string, MetricDefinition> = {
  total_return: {
    key: 'total_return',
    name: { en: 'Total Return', zh: '总收益率', es: 'Retorno total' },
    formula: 'R_{total} = \\frac{V_{end} - V_{start}}{V_{start}} \\times 100\\%',
    description: {
      en: 'Measures overall portfolio performance from start to end',
      zh: '衡量投资组合从开始到结束的整体收益表现',
      es: 'Mide el rendimiento total del portafolio de inicio a fin',
    },
  },
  annualized_return: {
    key: 'annualized_return',
    name: { en: 'Annualized Return', zh: '年化收益率', es: 'Retorno anualizado' },
    formula: 'R_{ann} = \\left(1 + R_{total}\\right)^{\\frac{252}{n}} - 1',
    description: {
      en: 'Standardized yearly return rate (252 trading days)',
      zh: '标准化年度收益率（按252个交易日计算）',
      es: 'Tasa de retorno anual estandarizada (252 dias de trading)',
    },
  },
  max_drawdown: {
    key: 'max_drawdown',
    name: { en: 'Maximum Drawdown', zh: '最大回撤', es: 'Maximo drawdown' },
    formula: 'MDD = \\max_{t} \\left( \\frac{Peak_t - Trough_t}{Peak_t} \\right)',
    description: {
      en: 'Largest peak-to-trough decline during the period',
      zh: '期间内从峰值到谷底的最大跌幅',
      es: 'Mayor descenso pico a valle en el periodo',
    },
  },
  sharpe_ratio: {
    key: 'sharpe_ratio',
    name: { en: 'Sharpe Ratio', zh: '夏普比率', es: 'Ratio Sharpe' },
    formula: 'SR = \\frac{\\bar{r} - r_f}{\\sigma}',
    description: {
      en: 'Risk-adjusted return per unit of volatility (r̄=avg return, rf=risk-free rate, σ=std dev)',
      zh: '单位波动风险下的超额收益（r̄=平均收益，rf=无风险利率，σ=标准差）',
      es: 'Retorno ajustado por riesgo por unidad de volatilidad (r prom, rf tasa libre de riesgo, sigma desviacion estandar)',
    },
  },
  sortino_ratio: {
    key: 'sortino_ratio',
    name: { en: 'Sortino Ratio', zh: '索提诺比率', es: 'Ratio Sortino' },
    formula: 'Sortino = \\frac{\\bar{r} - r_f}{\\sigma_d}',
    description: {
      en: 'Return per unit of downside risk (σd=downside deviation)',
      zh: '单位下行风险的收益（σd=下行标准差）',
      es: 'Retorno por unidad de riesgo a la baja (sigma_d = desviacion a la baja)',
    },
  },
  calmar_ratio: {
    key: 'calmar_ratio',
    name: { en: 'Calmar Ratio', zh: '卡玛比率', es: 'Ratio Calmar' },
    formula: 'Calmar = \\frac{R_{ann}}{|MDD|}',
    description: {
      en: 'Annualized return divided by maximum drawdown',
      zh: '年化收益率与最大回撤的比值',
      es: 'Retorno anualizado dividido por el maximo drawdown',
    },
  },
  win_rate: {
    key: 'win_rate',
    name: { en: 'Win Rate', zh: '胜率', es: 'Tasa de acierto' },
    formula: 'WinRate = \\frac{N_{win}}{N_{total}} \\times 100\\%',
    description: {
      en: 'Percentage of profitable trades',
      zh: '盈利交易占总交易数的百分比',
      es: 'Porcentaje de trades rentables',
    },
  },
  profit_factor: {
    key: 'profit_factor',
    name: { en: 'Profit Factor', zh: '盈亏比', es: 'Factor de beneficio' },
    formula: 'PF = \\frac{\\sum Profits}{|\\sum Losses|}',
    description: {
      en: 'Ratio of gross profit to gross loss',
      zh: '总盈利与总亏损的比值',
      es: 'Relacion entre ganancia bruta y perdida bruta',
    },
  },
  volatility: {
    key: 'volatility',
    name: { en: 'Volatility', zh: '波动率', es: 'Volatilidad' },
    formula: '\\sigma = \\sqrt{\\frac{1}{n}\\sum_{i=1}^{n}(r_i - \\bar{r})^2}',
    description: {
      en: 'Standard deviation of returns',
      zh: '收益率的标准差',
      es: 'Desviacion estandar de los retornos',
    },
  },
  var_95: {
    key: 'var_95',
    name: { en: 'VaR (95%)', zh: '风险价值', es: 'VaR (95%)' },
    formula: 'P(R < VaR_{95\\%}) = 5\\%',
    description: {
      en: '95% confidence level maximum expected loss',
      zh: '95%置信水平下的最大预期损失',
      es: 'Perdida maxima esperada con confianza del 95%',
    },
  },
  alpha: {
    key: 'alpha',
    name: { en: 'Alpha', zh: '超额收益', es: 'Alfa' },
    formula: '\\alpha = R_{portfolio} - R_{benchmark}',
    description: {
      en: 'Excess return over benchmark',
      zh: '相对于基准的超额收益',
      es: 'Exceso de retorno sobre el benchmark',
    },
  },
  beta: {
    key: 'beta',
    name: { en: 'Beta', zh: '贝塔系数', es: 'Beta' },
    formula: '\\beta = \\frac{Cov(R_p, R_m)}{Var(R_m)}',
    description: {
      en: 'Portfolio sensitivity to market movements',
      zh: '投资组合对市场波动的敏感度',
      es: 'Sensibilidad de la cartera a los movimientos del mercado',
    },
  },
  information_ratio: {
    key: 'information_ratio',
    name: { en: 'Information Ratio', zh: '信息比率', es: 'Ratio de informacion' },
    formula: 'IR = \\frac{\\alpha}{\\sigma_{tracking}}',
    description: {
      en: 'Alpha per unit of tracking error',
      zh: '单位跟踪误差的超额收益',
      es: 'Alfa por unidad de tracking error',
    },
  },
  avg_trade_pnl: {
    key: 'avg_trade_pnl',
    name: { en: 'Avg Trade PnL', zh: '平均盈亏', es: 'PnL promedio por trade' },
    formula: '\\bar{PnL} = \\frac{\\sum PnL_i}{N}',
    description: {
      en: 'Average profit/loss per trade',
      zh: '每笔交易的平均盈亏',
      es: 'Promedio de ganancia/perdida por trade',
    },
  },
  expectancy: {
    key: 'expectancy',
    name: { en: 'Expectancy', zh: '期望收益', es: 'Expectativa' },
    formula: 'E = (WinRate \\times \\bar{W}) - (LossRate \\times \\bar{L})',
    description: {
      en: 'Expected return per trade',
      zh: '每笔交易的期望收益',
      es: 'Retorno esperado por trade',
    },
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
  language?: Language
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

  const name = metric.name[language as Language] || metric.name.en
  const description = metric.description[language as Language] || metric.description.en

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
            {t('metricTooltip.formula', language as Language)}
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
  language?: Language
  className?: string
}

export function MetricLabel({ metricKey, label, language = 'en', className = '' }: MetricLabelProps) {
  const metric = METRIC_DEFINITIONS[metricKey]
  const displayLabel = label || metric?.name[language] || metricKey

  return (
    <span className={`inline-flex items-center gap-1 ${className}`}>
      {displayLabel}
      <MetricTooltip metricKey={metricKey} language={language} size={12} />
    </span>
  )
}
