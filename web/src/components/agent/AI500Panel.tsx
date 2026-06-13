import { useEffect, useState } from 'react'
import { Sparkles } from 'lucide-react'
import { api } from '../../lib/api'
import type { AI500Coin } from '../../lib/api/data'

interface AI500PanelProps {
  language: string
  disabled?: boolean
  onAnalyzeSymbol: (coin: AI500Coin) => void
}

const REFRESH_INTERVAL_MS = 5 * 60 * 1000 // matches the backend cache TTL

function formatGain(value?: number) {
  const n = Number(value || 0)
  if (!Number.isFinite(n) || n === 0) return '—'
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`
}

function scoreColor(score: number) {
  if (score >= 80) return '#0ECB81'
  if (score >= 60) return '#F0B90B'
  return '#848E9C'
}

export function AI500Panel({ language, disabled, onAnalyzeSymbol }: AI500PanelProps) {
  const [coins, setCoins] = useState<AI500Coin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    const load = () => {
      api
        .getAI500List(20)
        .then((res) => {
          if (cancelled) return
          setCoins(res.coins || [])
          setError('')
        })
        .catch((err) => {
          if (cancelled) return
          setError(err?.message || 'Failed to load AI500')
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }
    load()
    const timer = setInterval(load, REFRESH_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [])

  if (loading) {
    return (
      <div style={{ padding: 12, color: '#848E9C', fontSize: 12 }}>
        {language === 'zh' ? '正在加载 AI500 榜单…' : 'Loading AI500 board…'}
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 12, color: '#F6465D', fontSize: 12 }}>
        {language === 'zh' ? 'AI500 榜单加载失败：' : 'Failed to load AI500: '}
        {error}
      </div>
    )
  }

  if (coins.length === 0) {
    return (
      <div style={{ padding: 12, color: '#848E9C', fontSize: 12 }}>
        {language === 'zh' ? '当前没有符合条件的 AI500 标的。' : 'No AI500 constituents right now.'}
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: '#848E9C', fontSize: 10.5, padding: '0 2px' }}>
        <Sparkles size={11} color="#F0B90B" />
        {language === 'zh'
          ? 'AI 评分精选 · 点击标的让 Agent 分析'
          : 'AI-scored picks · click to ask the agent'}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 340, overflowY: 'auto', paddingRight: 2 }}>
        {coins.map((coin, idx) => {
          const display = coin.pair.replace(/USDT$/i, '')
          const gain = Number(coin.increase_percent || 0)
          return (
            <button
              key={coin.pair}
              disabled={disabled}
              onClick={() => onAnalyzeSymbol(coin)}
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr auto',
                gap: 8,
                alignItems: 'center',
                textAlign: 'left',
                padding: '10px 11px',
                borderRadius: 10,
                border: '1px solid rgba(255,255,255,0.05)',
                background: 'rgba(255,255,255,0.025)',
                cursor: disabled ? 'not-allowed' : 'pointer',
                opacity: disabled ? 0.6 : 1,
              }}
              title={
                language === 'zh' ? `让 Agent 分析 ${display}` : `Ask the agent to analyze ${display}`
              }
            >
              <div style={{ minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span style={{ color: '#4c4c62', fontSize: 10, width: 16 }}>#{idx + 1}</span>
                  <span style={{ color: '#EAECEF', fontWeight: 700, fontSize: 12.5 }}>{display}</span>
                </div>
                <div style={{ color: '#6c6c82', fontSize: 10.5, marginTop: 3, paddingLeft: 22 }}>
                  {language === 'zh' ? '入选以来' : 'Since entry'}{' '}
                  <span style={{ color: gain >= 0 ? '#0ECB81' : '#F6465D', fontWeight: 700 }}>
                    {formatGain(coin.increase_percent)}
                  </span>
                </div>
              </div>
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'flex-end',
                  gap: 2,
                }}
              >
                <span style={{ color: scoreColor(coin.score), fontWeight: 700, fontSize: 14 }}>
                  {coin.score.toFixed(1)}
                </span>
                <span style={{ color: '#4c4c62', fontSize: 9.5 }}>
                  {language === 'zh' ? 'AI 评分' : 'AI score'}
                </span>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
