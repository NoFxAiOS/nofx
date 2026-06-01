import { motion } from 'framer-motion'
import {
  Zap,
  Lightbulb,
  Search,
  Bot,
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
        { icon: <Bot size={18} />, title: '创建美股 Agent', subtitle: '强势股 + 严格风控', cmd: '创建一个美股趋势交易 Agent，默认选择5个强势美股，严格风控' },
        { icon: <Zap size={18} />, title: '一句话建策略', subtitle: '从想法到 Agent', cmd: '我想做美股强趋势突破，帮我生成策略和Agent' },
        { icon: <Search size={18} />, title: '搜索美股', subtitle: '输入名称或代码即可', cmd: '搜索一下 NVIDIA 和 Apple' },
        { icon: <Lightbulb size={18} />, title: '全球资产策略', subtitle: '美股/黄金/外汇', cmd: '当前美股、黄金、外汇适合什么策略？' },
      ]
    : [
        { icon: <Bot size={18} />, title: 'Create US Stock Agent', subtitle: 'Strong stocks + strict risk', cmd: 'Create a US stock trend-following agent with 5 strong stocks and strict risk control' },
        { icon: <Zap size={18} />, title: 'One-line strategy', subtitle: 'Idea to agent', cmd: 'I want a US stock breakout strategy; build the strategy and agent' },
        { icon: <Search size={18} />, title: 'Search US stocks', subtitle: 'Name or ticker', cmd: 'Search for NVIDIA and Apple stocks' },
        { icon: <Lightbulb size={18} />, title: 'Global strategy', subtitle: 'Stocks/gold/FX', cmd: 'What strategy fits US stocks, gold, and FX now?' },
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
          {language === 'zh' ? '快速创建你的美股 Agent' : 'Create your US stock agent'}
        </h1>
        <p style={{
          fontSize: 13.5,
          color: '#5c5c72',
          margin: 0,
          lineHeight: 1.5,
        }}>
          {language === 'zh'
            ? '美股、大宗、外汇、Pre-IPO — 用自然语言描述策略即可'
            : 'US stocks, commodities, FX, Pre-IPO — describe the strategy in plain English'}
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
