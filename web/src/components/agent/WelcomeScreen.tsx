import { motion } from 'framer-motion'
import {
  Zap,
  BarChart3,
  Lightbulb,
  Search,
} from 'lucide-react'

interface SuggestionCard {
  icon: JSX.Element
  title: string
  subtitle: string
  cmd: string
}

interface WelcomeScreenProps {
  language: string
  onSend: (cmd: string) => void
}

export function WelcomeScreen({ language, onSend }: WelcomeScreenProps) {
  const suggestions: SuggestionCard[] = language === 'zh'
    ? [
        { icon: <BarChart3 size={18} />, title: '分析 BTC 走势', subtitle: '技术分析 + 市场情绪', cmd: '分析一下 BTC 的走势' },
        { icon: <Zap size={18} />, title: '做多 ETH', subtitle: 'Agent 帮你自动下单', cmd: '帮我做多 ETH 0.01 手' },
        { icon: <Search size={18} />, title: '搜索股票', subtitle: '输入名称或代码即可', cmd: '搜索一下中远海控' },
        { icon: <Lightbulb size={18} />, title: '策略建议', subtitle: '根据当前市场给出建议', cmd: '当前市场适合什么策略？' },
      ]
    : [
        { icon: <BarChart3 size={18} />, title: 'Analyze BTC', subtitle: 'Technical analysis + sentiment', cmd: 'Analyze BTC price action' },
        { icon: <Zap size={18} />, title: 'Trade ETH', subtitle: 'Agent executes for you', cmd: 'Open a long position on ETH 0.01' },
        { icon: <Search size={18} />, title: 'Search Stocks', subtitle: 'Enter name or ticker', cmd: 'Search for NVIDIA stock' },
        { icon: <Lightbulb size={18} />, title: 'Strategy Ideas', subtitle: 'Market-based suggestions', cmd: 'What strategy fits the current market?' },
      ]

  return (
    <div style={{
      maxWidth: 640,
      margin: '0 auto',
      padding: '0 20px',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100%',
      minHeight: 400,
    }}>
      {/* Logo / greeting */}
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: 'easeOut' }}
        style={{ textAlign: 'center', marginBottom: 40 }}
      >
        <div style={{
          width: 56,
          height: 56,
          borderRadius: 16,
          background: 'linear-gradient(135deg, rgba(240,185,11,0.12), rgba(0,229,160,0.06))',
          border: '1px solid rgba(240,185,11,0.15)',
          display: 'grid',
          placeItems: 'center',
          margin: '0 auto 16px',
          fontSize: 24,
        }}>
          ⚡
        </div>
        <h1 style={{
          fontSize: 22,
          fontWeight: 700,
          color: '#f0f0f8',
          margin: '0 0 8px',
          letterSpacing: '-0.02em',
        }}>
          {language === 'zh' ? '跟 NOFXi 聊点什么' : 'What can I help with?'}
        </h1>
        <p style={{
          fontSize: 13.5,
          color: '#5c5c72',
          margin: 0,
          lineHeight: 1.5,
        }}>
          {language === 'zh'
            ? '分析行情、执行交易、搜索股票 — 用自然语言就行'
            : 'Analyze markets, execute trades, search stocks — just ask'}
        </p>
      </motion.div>

      {/* Suggestion cards grid */}
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.1, ease: 'easeOut' }}
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(2, 1fr)',
          gap: 10,
          width: '100%',
          maxWidth: 520,
        }}
      >
        {suggestions.map((s, i) => (
          <button
            key={i}
            onClick={() => onSend(s.cmd)}
            className="suggestion-card"
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'flex-start',
              gap: 6,
              padding: '16px 14px',
              background: 'rgba(255,255,255,0.02)',
              border: '1px solid rgba(255,255,255,0.06)',
              borderRadius: 14,
              cursor: 'pointer',
              textAlign: 'left',
              fontFamily: 'inherit',
              transition: 'all 0.2s ease',
            }}
          >
            <div style={{ color: '#F0B90B', opacity: 0.7 }}>
              {s.icon}
            </div>
            <div>
              <div style={{ fontSize: 13, fontWeight: 600, color: '#d0d0e0', marginBottom: 2 }}>
                {s.title}
              </div>
              <div style={{ fontSize: 11.5, color: '#5c5c72' }}>
                {s.subtitle}
              </div>
            </div>
          </button>
        ))}
      </motion.div>
    </div>
  )
}
