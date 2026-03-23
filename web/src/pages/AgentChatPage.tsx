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
  ArrowUp,
  Zap,
  BarChart3,
  Lightbulb,
  Search,
} from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { MarketTicker } from '../components/agent/MarketTicker'
import { PositionsPanel } from '../components/agent/PositionsPanel'
import { TraderStatusPanel } from '../components/agent/TraderStatusPanel'
import { renderMessageContent } from '../components/agent/MessageRenderer'

interface Message {
  id: string
  role: 'user' | 'bot'
  text: string
  time: string
  streaming?: boolean
}

let msgIdCounter = 0
function nextId() {
  return `msg-${Date.now()}-${++msgIdCounter}`
}

// Suggestion cards for welcome state
interface SuggestionCard {
  icon: JSX.Element
  title: string
  subtitle: string
  cmd: string
}

export function AgentChatPage() {
  const { language } = useLanguage()
  const { token } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 1024)
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [composing, setComposing] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // Sidebar section collapse state
  const [sections, setSections] = useState({
    market: true,
    positions: true,
    traders: false,
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

  // Keyboard shortcut: Cmd+K to focus input
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
      }
      if (e.key === 'Escape' && window.innerWidth <= 768) {
        setSidebarOpen(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  // Auto-resize textarea
  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      setInput(e.target.value)
      const el = e.target
      el.style.height = 'auto'
      el.style.height = Math.min(el.scrollHeight, 150) + 'px'
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
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ message: msg, lang: language }),
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
          await new Promise((r) => setTimeout(r, 10))
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

  // Welcome suggestions (ChatGPT style)
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

  const quickActions = language === 'zh'
    ? [
        { label: '💼 持仓', cmd: '/positions' },
        { label: '💰 余额', cmd: '/balance' },
        { label: '📋 Traders', cmd: '/traders' },
        { label: '🧹 清除记忆', cmd: '/clear' },
        { label: '❓ 帮助', cmd: '/help' },
      ]
    : [
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
      title: 'Traders',
      component: <TraderStatusPanel />,
    },
  ]

  const isWelcomeState = messages.length === 0

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
        {/* Top bar with quick actions */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '8px 16px',
            borderBottom: '1px solid rgba(255,255,255,0.04)',
            overflowX: 'auto',
            flexShrink: 0,
            backdropFilter: 'blur(12px)',
            background: 'rgba(9,9,11,0.8)',
          }}
          className="hide-scrollbar"
        >
          {quickActions.map((a, i) => (
            <button
              key={i}
              onClick={() => send(a.cmd)}
              className="quick-action-btn"
              style={{
                padding: '5px 12px',
                background: 'rgba(255,255,255,0.03)',
                border: '1px solid rgba(255,255,255,0.06)',
                borderRadius: 20,
                color: '#6c6c82',
                fontSize: 12,
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                fontFamily: 'inherit',
                transition: 'all 0.2s ease',
              }}
            >
              {a.label}
            </button>
          ))}

          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            style={{
              marginLeft: 'auto',
              padding: 6,
              background: 'transparent',
              border: 'none',
              color: '#4c4c62',
              cursor: 'pointer',
              borderRadius: 8,
              display: 'flex',
              alignItems: 'center',
              flexShrink: 0,
              transition: 'color 0.2s',
            }}
            title={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
            onMouseEnter={(e) => { e.currentTarget.style.color = '#8a8aa0' }}
            onMouseLeave={(e) => { e.currentTarget.style.color = '#4c4c62' }}
          >
            {sidebarOpen ? (
              <PanelRightClose size={18} />
            ) : (
              <PanelRightOpen size={18} />
            )}
          </button>
        </div>

        {/* Messages area or Welcome state */}
        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: '20px 0',
          }}
          className="custom-scrollbar"
        >
          {isWelcomeState ? (
            /* ========== WELCOME STATE ========== */
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
                    onClick={() => send(s.cmd)}
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
          ) : (
            /* ========== MESSAGES ========== */
            <div style={{ maxWidth: 720, margin: '0 auto', padding: '0 20px' }}>
              {messages.map((m) => (
                <motion.div
                  key={m.id}
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.2 }}
                  style={{
                    display: 'flex',
                    gap: 12,
                    marginBottom: 24,
                    flexDirection: m.role === 'user' ? 'row-reverse' : 'row',
                  }}
                >
                  {/* Avatar */}
                  <div
                    style={{
                      width: 30,
                      height: 30,
                      borderRadius: 10,
                      display: 'grid',
                      placeItems: 'center',
                      fontSize: 14,
                      flexShrink: 0,
                      marginTop: 2,
                      background:
                        m.role === 'user'
                          ? 'linear-gradient(135deg, rgba(139,92,246,.12), rgba(139,92,246,.04))'
                          : 'linear-gradient(135deg, rgba(240,185,11,.08), rgba(0,229,160,.04))',
                      border:
                        '1px solid ' +
                        (m.role === 'user'
                          ? 'rgba(139,92,246,.15)'
                          : 'rgba(240,185,11,.1)'),
                    }}
                  >
                    {m.role === 'user' ? '👤' : '⚡'}
                  </div>

                  {/* Message content */}
                  <div style={{ maxWidth: '78%', minWidth: 0 }}>
                    {m.role === 'user' ? (
                      <div
                        style={{
                          padding: '10px 16px',
                          borderRadius: 18,
                          borderTopRightRadius: 4,
                          fontSize: 13.5,
                          lineHeight: 1.7,
                          whiteSpace: 'pre-wrap',
                          wordBreak: 'break-word',
                          background: 'linear-gradient(135deg, #7c3aed, #6d28d9)',
                          color: '#fff',
                        }}
                      >
                        {m.text}
                      </div>
                    ) : (
                      <div
                        style={{
                          padding: '12px 16px',
                          borderRadius: 18,
                          borderTopLeftRadius: 4,
                          fontSize: 13.5,
                          lineHeight: 1.7,
                          wordBreak: 'break-word',
                          background: 'rgba(255,255,255,0.03)',
                          color: '#dcdce8',
                          border: '1px solid rgba(255,255,255,0.05)',
                        }}
                      >
                        {renderMessageContent(m.text)}
                        {m.streaming && m.text === '' && (
                          <div style={{ display: 'flex', gap: 4, padding: '4px 0' }}>
                            <span className="typing-dot" style={{ animationDelay: '0ms' }} />
                            <span className="typing-dot" style={{ animationDelay: '150ms' }} />
                            <span className="typing-dot" style={{ animationDelay: '300ms' }} />
                          </div>
                        )}
                        {m.streaming && m.text !== '' && (
                          <span
                            style={{
                              display: 'inline-block',
                              width: 2,
                              height: 15,
                              background: '#F0B90B',
                              marginLeft: 1,
                              borderRadius: 1,
                              animation: 'blink 0.8s infinite',
                              verticalAlign: 'text-bottom',
                            }}
                          />
                        )}
                      </div>
                    )}
                    {m.time && !m.streaming && (
                      <div
                        style={{
                          fontSize: 10,
                          color: '#2c2c42',
                          marginTop: 4,
                          textAlign: m.role === 'user' ? 'right' : 'left',
                          paddingLeft: m.role === 'bot' ? 4 : 0,
                          paddingRight: m.role === 'user' ? 4 : 0,
                        }}
                      >
                        {m.role === 'bot' && 'NOFXi · '}{m.time}
                      </div>
                    )}
                  </div>
                </motion.div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* Input area */}
        <div
          style={{
            padding: '12px 16px 20px',
            borderTop: '1px solid rgba(255,255,255,0.04)',
            background: 'linear-gradient(to top, #09090b 80%, transparent)',
          }}
        >
          <div
            className="chat-input-wrapper"
            style={{
              maxWidth: 720,
              margin: '0 auto',
              display: 'flex',
              gap: 8,
              background: 'rgba(255,255,255,0.03)',
              border: '1px solid rgba(255,255,255,0.07)',
              borderRadius: 18,
              padding: '4px 4px 4px 16px',
              alignItems: 'flex-end',
              transition: 'all 0.2s ease',
            }}
          >
            <textarea
              ref={inputRef}
              value={input}
              onChange={handleInputChange}
              onCompositionStart={() => setComposing(true)}
              onCompositionEnd={() => setComposing(false)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !composing) {
                  e.preventDefault()
                  send()
                }
              }}
              placeholder={
                language === 'zh'
                  ? '跟 NOFXi 聊点什么...  ⌘K'
                  : 'Ask NOFXi anything...  ⌘K'
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
                maxHeight: 150,
              }}
            />
            <button
              onClick={() => send()}
              disabled={loading || !input.trim()}
              style={{
                width: 36,
                height: 36,
                borderRadius: 12,
                border: 'none',
                background:
                  loading || !input.trim()
                    ? 'rgba(255,255,255,0.04)'
                    : 'linear-gradient(135deg, #F0B90B, #d4a30a)',
                color: loading || !input.trim() ? '#3c3c52' : '#000',
                cursor: loading || !input.trim() ? 'not-allowed' : 'pointer',
                display: 'grid',
                placeItems: 'center',
                flexShrink: 0,
                transition: 'all 0.2s ease',
              }}
            >
              <ArrowUp size={16} strokeWidth={2.5} />
            </button>
          </div>
          <div
            style={{
              maxWidth: 720,
              margin: '6px auto 0',
              textAlign: 'center',
              fontSize: 10,
              color: '#1e1e32',
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
            animate={{ width: 280, opacity: 1 }}
            exit={{ width: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
            style={{
              borderLeft: '1px solid rgba(255,255,255,0.04)',
              background: 'rgba(11,11,19,0.6)',
              backdropFilter: 'blur(12px)',
              overflowY: 'auto',
              overflowX: 'hidden',
              flexShrink: 0,
            }}
            className="custom-scrollbar"
          >
            <div style={{ padding: '12px 10px 20px', width: 280 }}>
              {/* Sidebar header */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 12,
                  padding: '4px 6px',
                }}
              >
                <span
                  style={{
                    fontSize: 10,
                    fontWeight: 700,
                    color: '#4c4c62',
                    textTransform: 'uppercase',
                    letterSpacing: 1.5,
                  }}
                >
                  {language === 'zh' ? '交易面板' : 'Trading Panel'}
                </span>
              </div>

              {/* Sidebar sections */}
              {sidebarSections.map((section) => (
                <div key={section.key} style={{ marginBottom: 8 }}>
                  <button
                    onClick={() => toggleSection(section.key)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      width: '100%',
                      padding: '7px 8px',
                      background: 'transparent',
                      border: 'none',
                      color: '#7a7a90',
                      fontSize: 12,
                      fontWeight: 600,
                      cursor: 'pointer',
                      borderRadius: 8,
                      transition: 'all 0.15s ease',
                      fontFamily: 'inherit',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = 'rgba(255,255,255,0.03)'
                      e.currentTarget.style.color = '#a0a0b0'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'transparent'
                      e.currentTarget.style.color = '#7a7a90'
                    }}
                  >
                    {section.icon}
                    <span>{section.title}</span>
                    <span style={{ marginLeft: 'auto', transition: 'transform 0.2s' }}>
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

      {/* Animations */}
      <style>{`
        @keyframes blink {
          0%, 50% { opacity: 1; }
          51%, 100% { opacity: 0; }
        }

        @keyframes typingBounce {
          0%, 60%, 100% { transform: translateY(0); opacity: 0.3; }
          30% { transform: translateY(-4px); opacity: 0.8; }
        }

        .typing-dot {
          width: 5px;
          height: 5px;
          border-radius: 50%;
          background: #F0B90B;
          display: inline-block;
          animation: typingBounce 1.2s infinite;
        }

        .suggestion-card:hover {
          background: rgba(240,185,11,0.04) !important;
          border-color: rgba(240,185,11,0.15) !important;
          transform: translateY(-1px);
        }

        .quick-action-btn:hover {
          border-color: rgba(240,185,11,0.2) !important;
          color: #F0B90B !important;
          background: rgba(240,185,11,0.04) !important;
        }

        .chat-input-wrapper:focus-within {
          border-color: rgba(240,185,11,0.25) !important;
          box-shadow: 0 0 0 1px rgba(240,185,11,0.08);
        }

        .custom-scrollbar::-webkit-scrollbar {
          width: 4px;
        }
        .custom-scrollbar::-webkit-scrollbar-track {
          background: transparent;
        }
        .custom-scrollbar::-webkit-scrollbar-thumb {
          background: rgba(255,255,255,0.06);
          border-radius: 4px;
        }
        .custom-scrollbar::-webkit-scrollbar-thumb:hover {
          background: rgba(255,255,255,0.1);
        }

        .hide-scrollbar::-webkit-scrollbar {
          display: none;
        }
        .hide-scrollbar {
          -ms-overflow-style: none;
          scrollbar-width: none;
        }

        @media (max-width: 640px) {
          .suggestion-card {
            padding: 12px !important;
          }
        }
      `}</style>
    </div>
  )
}
