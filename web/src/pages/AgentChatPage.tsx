import { useState, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  PanelRightClose,
  PanelRightOpen,
  TrendingUp,
  Wallet,
  Bot,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { MarketTicker } from '../components/agent/MarketTicker'
import { PositionsPanel } from '../components/agent/PositionsPanel'
import { TraderStatusPanel } from '../components/agent/TraderStatusPanel'
import { WelcomeScreen } from '../components/agent/WelcomeScreen'
import { ChatMessages } from '../components/agent/ChatMessages'
import { ChatInput, type ChatInputHandle } from '../components/agent/ChatInput'

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

export function AgentChatPage() {
  const { language } = useLanguage()
  const { token } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 1024)
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const chatInputRef = useRef<ChatInputHandle>(null)
  const abortRef = useRef<AbortController | null>(null)

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

  // Escape to close sidebar on mobile
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && window.innerWidth <= 768) {
        setSidebarOpen(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const send = async (text: string) => {
    if (!text || loading) return
    const time = new Date().toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })
    const userMsg: Message = { id: nextId(), role: 'user', text, time }
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
      // Abort any in-flight request
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      const res = await fetch('/api/agent/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ message: text, lang: language }),
        signal: controller.signal,
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || `Server error (${res.status})`)
      }
      const data = await res.json()
      const responseText = data.response || data.error || 'No response'

      // Simulate streaming by revealing text in chunks (batched to reduce renders)
      const words = responseText.split(/(\s+)/)
      const chunkSize = Math.max(3, Math.ceil(words.length / 40)) // ~40 frames max
      let displayed = ''
      for (let i = 0; i < words.length; i += chunkSize) {
        const chunk = words.slice(i, i + chunkSize).join('')
        displayed += chunk
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
        if (i + chunkSize < words.length) {
          await new Promise((r) => setTimeout(r, 20))
        }
      }
      // Mark streaming done
      setMessages((prev) =>
        prev.map((m) => (m.id === botId ? { ...m, streaming: false } : m))
      )
    } catch (e: any) {
      if (e.name === 'AbortError') {
        // Request was cancelled (e.g. user sent a new message), clean up silently
        setMessages((prev) => prev.filter((m) => m.id !== botId))
      } else {
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
    }
    setLoading(false)
    chatInputRef.current?.focus()
  }

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
            <WelcomeScreen language={language} onSend={send} />
          ) : (
            <ChatMessages messages={messages} ref={messagesEndRef} />
          )}
        </div>

        {/* Input area */}
        <ChatInput
          ref={chatInputRef}
          language={language}
          loading={loading}
          onSend={send}
        />
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
