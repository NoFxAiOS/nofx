import { useState, useRef, useEffect, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  PanelRightClose,
  PanelRightOpen,
  TrendingUp,
  Wallet,
  Bot,
  ChevronDown,
  ChevronRight,
  Send,
  Sparkles,
} from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'
import { MarketTicker } from '../components/agent/MarketTicker'
import { PositionsPanel } from '../components/agent/PositionsPanel'
import { TraderStatusPanel } from '../components/agent/TraderStatusPanel'

interface Message {
  id: string
  role: 'user' | 'bot'
  text: string
  time: string
  streaming?: boolean
}

// Simple markdown-ish renderer: bold, code, newlines
function renderMessageContent(text: string) {
  // Split by code blocks first
  const parts = text.split(/(```[\s\S]*?```|`[^`]+`)/g)
  return parts.map((part, i) => {
    if (part.startsWith('```') && part.endsWith('```')) {
      const code = part.slice(3, -3).replace(/^\w+\n/, '') // strip language hint
      return (
        <pre
          key={i}
          style={{
            background: '#0a0a12',
            border: '1px solid #1f1f2c',
            borderRadius: 8,
            padding: '10px 12px',
            fontSize: 12,
            overflowX: 'auto',
            margin: '6px 0',
            fontFamily: 'monospace',
            color: '#c0c0d0',
          }}
        >
          {code}
        </pre>
      )
    }
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code
          key={i}
          style={{
            background: '#1a1a28',
            padding: '1px 5px',
            borderRadius: 4,
            fontSize: '0.9em',
            fontFamily: 'monospace',
            color: '#d0d0e0',
          }}
        >
          {part.slice(1, -1)}
        </code>
      )
    }
    // Handle bold **text**
    const boldParts = part.split(/(\*\*[^*]+\*\*)/g)
    return boldParts.map((bp, j) => {
      if (bp.startsWith('**') && bp.endsWith('**')) {
        return (
          <strong key={`${i}-${j}`} style={{ fontWeight: 600, color: '#f0f0f8' }}>
            {bp.slice(2, -2)}
          </strong>
        )
      }
      return <span key={`${i}-${j}`}>{bp}</span>
    })
  })
}

let msgIdCounter = 0
function nextId() {
  return `msg-${Date.now()}-${++msgIdCounter}`
}

export function AgentChatPage() {
  const { language } = useLanguage()
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 1024)
  const [messages, setMessages] = useState<Message[]>([
    {
      id: nextId(),
      role: 'bot',
      text:
        language === 'zh'
          ? '👋 你好！我是 NOFXi，你的 AI 交易 Agent。\n\n跟我说话就行，我能帮你分析市场、管理交易、配置策略。\n\n试试: "分析一下BTC" 或 /help'
          : "👋 Hi! I'm NOFXi, your AI trading agent.\n\nJust talk to me. I can analyze markets, manage trades, and configure strategies.\n\nTry: \"Analyze BTC\" or /help",
      time: new Date().toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
      }),
    },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [composing, setComposing] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // Sidebar section collapse state
  const [sections, setSections] = useState({
    market: true,
    positions: true,
    traders: true,
  })

  const toggleSection = (key: keyof typeof sections) => {
    setSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // Responsive sidebar
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth <= 768) setSidebarOpen(false)
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  // Auto-resize textarea
  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      setInput(e.target.value)
      const el = e.target
      el.style.height = 'auto'
      el.style.height = Math.min(el.scrollHeight, 120) + 'px'
    },
    []
  )

  const send = async (text?: string) => {
    const msg = text || input.trim()
    if (!msg || loading) return
    setInput('')
    if (inputRef.current) {
      inputRef.current.style.height = 'auto'
    }
    const time = new Date().toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })
    const userMsg: Message = { id: nextId(), role: 'user', text: msg, time }
    const botId = nextId()
    setMessages((prev) => [
      ...prev,
      userMsg,
      {
        id: botId,
        role: 'bot',
        text: '',
        time: '',
        streaming: true,
      },
    ])
    setLoading(true)

    try {
      const res = await fetch('/api/agent/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: msg, user_id: 1, lang: language }),
      })
      const data = await res.json()
      const responseText = data.response || data.error || 'No response'

      // Simulate streaming by revealing text progressively
      const words = responseText.split(/(\s+)/)
      let displayed = ''
      for (let i = 0; i < words.length; i++) {
        displayed += words[i]
        const current = displayed
        setMessages((prev) =>
          prev.map((m) =>
            m.id === botId
              ? {
                  ...m,
                  text: current,
                  time: new Date().toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit',
                  }),
                }
              : m
          )
        )
        if (i < words.length - 1) {
          await new Promise((r) => setTimeout(r, 12))
        }
      }
      // Mark streaming done
      setMessages((prev) =>
        prev.map((m) => (m.id === botId ? { ...m, streaming: false } : m))
      )
    } catch (e: any) {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === botId
            ? {
                ...m,
                text: '⚠️ Error: ' + e.message,
                time: new Date().toLocaleTimeString([], {
                  hour: '2-digit',
                  minute: '2-digit',
                }),
                streaming: false,
              }
            : m
        )
      )
    }
    setLoading(false)
    inputRef.current?.focus()
  }

  const quickActions =
    language === 'zh'
      ? [
          { label: '📊 分析 BTC', cmd: '/analyze BTC' },
          { label: '📊 分析 ETH', cmd: '/analyze ETH' },
          { label: '💼 持仓', cmd: '/positions' },
          { label: '💰 余额', cmd: '/balance' },
          { label: '📋 Traders', cmd: '/traders' },
          { label: '🧹 清除记忆', cmd: '/clear' },
          { label: '❓ 帮助', cmd: '/help' },
        ]
      : [
          { label: '📊 Analyze BTC', cmd: '/analyze BTC' },
          { label: '📊 Analyze ETH', cmd: '/analyze ETH' },
          { label: '💼 Positions', cmd: '/positions' },
          { label: '💰 Balance', cmd: '/balance' },
          { label: '📋 Traders', cmd: '/traders' },
          { label: '🧹 Clear', cmd: '/clear' },
          { label: '❓ Help', cmd: '/help' },
        ]

  const sidebarSections = [
    {
      key: 'market' as const,
      icon: <TrendingUp size={14} />,
      title: language === 'zh' ? '市场行情' : 'Market',
      component: <MarketTicker />,
    },
    {
      key: 'positions' as const,
      icon: <Wallet size={14} />,
      title: language === 'zh' ? '持仓' : 'Positions',
      component: <PositionsPanel />,
    },
    {
      key: 'traders' as const,
      icon: <Bot size={14} />,
      title: language === 'zh' ? 'Traders' : 'Traders',
      component: <TraderStatusPanel />,
    },
  ]

  return (
    <div
      style={{
        display: 'flex',
        height: 'calc(100vh - 64px)',
        background: '#09090b',
        overflow: 'hidden',
      }}
    >
      {/* ==================== MAIN CHAT AREA ==================== */}
      <div
        style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          minWidth: 0,
          position: 'relative',
        }}
      >
        {/* Quick actions bar */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '8px 16px',
            borderBottom: '1px solid #1a1a28',
            overflowX: 'auto',
            flexShrink: 0,
          }}
        >
          <Sparkles size={14} color="#F0B90B" style={{ flexShrink: 0 }} />
          {quickActions.map((a, i) => (
            <button
              key={i}
              onClick={() => send(a.cmd)}
              style={{
                padding: '5px 12px',
                background: '#111118',
                border: '1px solid #1f1f2c',
                borderRadius: 20,
                color: '#8a8aa0',
                fontSize: 12,
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                fontFamily: 'inherit',
                transition: 'all 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = '#F0B90B40'
                e.currentTarget.style.color = '#F0B90B'
                e.currentTarget.style.background = '#F0B90B08'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = '#1f1f2c'
                e.currentTarget.style.color = '#8a8aa0'
                e.currentTarget.style.background = '#111118'
              }}
            >
              {a.label}
            </button>
          ))}

          {/* Sidebar toggle (desktop) */}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            style={{
              marginLeft: 'auto',
              padding: 6,
              background: 'transparent',
              border: 'none',
              color: '#5c5c72',
              cursor: 'pointer',
              borderRadius: 6,
              display: 'flex',
              alignItems: 'center',
              flexShrink: 0,
            }}
            title={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
          >
            {sidebarOpen ? (
              <PanelRightClose size={18} />
            ) : (
              <PanelRightOpen size={18} />
            )}
          </button>
        </div>

        {/* Messages area */}
        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: '20px 0',
          }}
        >
          <div style={{ maxWidth: 720, margin: '0 auto', padding: '0 20px' }}>
            {messages.map((m) => (
              <motion.div
                key={m.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.2 }}
                style={{
                  display: 'flex',
                  gap: 12,
                  marginBottom: 20,
                  flexDirection: m.role === 'user' ? 'row-reverse' : 'row',
                }}
              >
                {/* Avatar */}
                <div
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 10,
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 15,
                    flexShrink: 0,
                    background:
                      m.role === 'user'
                        ? 'linear-gradient(135deg, rgba(139,92,246,.15), rgba(139,92,246,.05))'
                        : 'linear-gradient(135deg, rgba(240,185,11,.1), rgba(0,229,160,.05))',
                    border:
                      '1px solid ' +
                      (m.role === 'user'
                        ? 'rgba(139,92,246,.2)'
                        : 'rgba(240,185,11,.15)'),
                  }}
                >
                  {m.role === 'user' ? '👤' : '⚡'}
                </div>

                {/* Message bubble */}
                <div style={{ maxWidth: '75%', minWidth: 0 }}>
                  <div
                    style={{
                      padding: '12px 16px',
                      borderRadius: 16,
                      fontSize: 13.5,
                      lineHeight: 1.7,
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                      background:
                        m.role === 'user'
                          ? 'linear-gradient(135deg, #8b5cf6, #6d28d9)'
                          : '#111118',
                      color: m.role === 'user' ? '#fff' : '#eaeaf0',
                      border:
                        m.role === 'bot' ? '1px solid #1f1f2c' : 'none',
                      borderTopLeftRadius: m.role === 'bot' ? 4 : 16,
                      borderTopRightRadius: m.role === 'user' ? 4 : 16,
                    }}
                  >
                    {m.role === 'bot'
                      ? renderMessageContent(m.text)
                      : m.text}
                    {m.streaming && (
                      <span
                        style={{
                          display: 'inline-block',
                          width: 6,
                          height: 14,
                          background: '#F0B90B',
                          marginLeft: 2,
                          borderRadius: 1,
                          animation: 'blink 1s infinite',
                          verticalAlign: 'text-bottom',
                        }}
                      />
                    )}
                  </div>
                  {m.time && (
                    <div
                      style={{
                        fontSize: 10,
                        color: '#3c3c52',
                        marginTop: 4,
                        textAlign: m.role === 'user' ? 'right' : 'left',
                        paddingLeft: m.role === 'bot' ? 4 : 0,
                        paddingRight: m.role === 'user' ? 4 : 0,
                      }}
                    >
                      {m.role === 'bot' && 'NOFXi · '}
                      {m.time}
                    </div>
                  )}
                </div>
              </motion.div>
            ))}
            <div ref={messagesEndRef} />
          </div>
        </div>

        {/* Input area */}
        <div
          style={{
            padding: '12px 16px 20px',
            borderTop: '1px solid #1a1a28',
            background:
              'linear-gradient(to top, #09090b 60%, transparent)',
          }}
        >
          <div
            style={{
              maxWidth: 720,
              margin: '0 auto',
              display: 'flex',
              gap: 8,
              background: '#111118',
              border: '1px solid #1f1f2c',
              borderRadius: 16,
              padding: '4px 4px 4px 16px',
              alignItems: 'flex-end',
              transition: 'border-color 0.2s',
            }}
            onFocus={(e) => {
              ;(e.currentTarget as HTMLElement).style.borderColor =
                '#F0B90B40'
            }}
            onBlur={(e) => {
              ;(e.currentTarget as HTMLElement).style.borderColor =
                '#1f1f2c'
            }}
          >
            <textarea
              ref={inputRef}
              value={input}
              onChange={handleInputChange}
              onCompositionStart={() => setComposing(true)}
              onCompositionEnd={() => setComposing(false)}
              onKeyDown={(e) => {
                if (
                  e.key === 'Enter' &&
                  !e.shiftKey &&
                  !composing
                ) {
                  e.preventDefault()
                  send()
                }
              }}
              placeholder={
                language === 'zh'
                  ? '跟 NOFXi 聊点什么...'
                  : 'Ask NOFXi anything...'
              }
              rows={1}
              style={{
                flex: 1,
                background: 'none',
                border: 'none',
                color: '#eaeaf0',
                fontSize: 13.5,
                outline: 'none',
                padding: '10px 0',
                fontFamily: 'inherit',
                resize: 'none',
                lineHeight: 1.5,
                maxHeight: 120,
              }}
            />
            <button
              onClick={() => send()}
              disabled={loading || !input.trim()}
              style={{
                width: 38,
                height: 38,
                borderRadius: 12,
                border: 'none',
                background:
                  loading || !input.trim()
                    ? '#1a1a28'
                    : 'linear-gradient(135deg, #F0B90B, #d4a30a)',
                color: loading || !input.trim() ? '#3c3c52' : '#000',
                cursor:
                  loading || !input.trim()
                    ? 'not-allowed'
                    : 'pointer',
                display: 'grid',
                placeItems: 'center',
                flexShrink: 0,
                transition: 'all 0.2s',
              }}
            >
              <Send size={16} />
            </button>
          </div>
          <div
            style={{
              maxWidth: 720,
              margin: '6px auto 0',
              textAlign: 'center',
              fontSize: 10,
              color: '#2c2c42',
            }}
          >
            NOFXi may make mistakes. Always verify trading decisions.
          </div>
        </div>
      </div>

      {/* ==================== RIGHT SIDEBAR ==================== */}
      <AnimatePresence>
        {sidebarOpen && (
          <motion.div
            initial={{ width: 0, opacity: 0 }}
            animate={{ width: 300, opacity: 1 }}
            exit={{ width: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
            style={{
              borderLeft: '1px solid #1a1a28',
              background: '#0b0b13',
              overflowY: 'auto',
              overflowX: 'hidden',
              flexShrink: 0,
            }}
          >
            <div style={{ padding: '12px 12px 20px', width: 300 }}>
              {/* Sidebar header */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 14,
                  padding: '0 4px',
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 700,
                    color: '#5c5c72',
                    textTransform: 'uppercase',
                    letterSpacing: 1,
                  }}
                >
                  {language === 'zh' ? '交易面板' : 'Trading Panel'}
                </span>
              </div>

              {/* Sidebar sections */}
              {sidebarSections.map((section) => (
                <div key={section.key} style={{ marginBottom: 10 }}>
                  <button
                    onClick={() => toggleSection(section.key)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      width: '100%',
                      padding: '8px 8px',
                      background: 'transparent',
                      border: 'none',
                      color: '#8a8aa0',
                      fontSize: 12,
                      fontWeight: 600,
                      cursor: 'pointer',
                      borderRadius: 8,
                      transition: 'background 0.15s',
                      fontFamily: 'inherit',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = '#111118'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'transparent'
                    }}
                  >
                    {section.icon}
                    <span>{section.title}</span>
                    <span style={{ marginLeft: 'auto' }}>
                      {sections[section.key] ? (
                        <ChevronDown size={14} />
                      ) : (
                        <ChevronRight size={14} />
                      )}
                    </span>
                  </button>
                  <AnimatePresence>
                    {sections[section.key] && (
                      <motion.div
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: 'auto', opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.15 }}
                        style={{ overflow: 'hidden', padding: '0 4px' }}
                      >
                        <div style={{ paddingTop: 4 }}>
                          {section.component}
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Blinking cursor animation */}
      <style>{`
        @keyframes blink {
          0%, 50% { opacity: 1; }
          51%, 100% { opacity: 0; }
        }
      `}</style>
    </div>
  )
}
