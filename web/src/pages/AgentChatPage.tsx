import { useState, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  PanelRightClose,
  PanelRightOpen,
  TrendingUp,
  Wallet,
  Bot,
  Bookmark,
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
import { UserPreferencesPanel } from '../components/agent/UserPreferencesPanel'
import {
  useAgentChatStore,
} from '../stores/agentChatStore'
import type { AgentMessage as Message, AgentStep } from '../types/agent'
import {
  chatStorageKey,
  clearAgentMessages,
  getStoredAuthUserId,
  loadAgentMessages,
  migrateAgentMessages,
  prepareAgentMessagesForPersistence,
  persistAgentMessages,
} from '../lib/agentChatStorage'

let msgIdCounter = 0
let activeStreamAbortController: AbortController | null = null
let activeStreamReader: ReadableStreamDefaultReader<Uint8Array> | null = null

function nextId() {
  return `msg-${Date.now()}-${++msgIdCounter}`
}

function cleanupActiveAgentStream() {
  activeStreamAbortController?.abort()
  activeStreamAbortController = null
  void activeStreamReader?.cancel().catch(() => {
    // Ignore stream cancellation races during teardown.
  })
  activeStreamReader = null
}

function persistMessagesSnapshotForUser(userId?: string) {
  const { hydrated, messages } = useAgentChatStore.getState()
  if (!hydrated) return
  const persistable = prepareAgentMessagesForPersistence(messages).slice(-100)
  persistAgentMessages(window.localStorage, userId, persistable)
}

function replaceMessagesInStore(nextMessages: Message[], userId?: string) {
  useAgentChatStore.getState().setMessages(nextMessages)
  persistMessagesSnapshotForUser(userId)
}

function patchMessagesInStore(
  updater: (prev: Message[]) => Message[],
  userId?: string
) {
  const nextMessages = updater(useAgentChatStore.getState().messages)
  useAgentChatStore.getState().updateMessages(() => nextMessages)
  persistMessagesSnapshotForUser(userId)
}

async function runAgentStream(params: {
  text: string
  token?: string | null
  language: string
  storageUserId?: string
  onDone?: () => void
}) {
  const { text, token, language, storageUserId, onDone } = params
  if (!text || useAgentChatStore.getState().loading) return

  const time = new Date().toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
  const userMsg: Message = { id: nextId(), role: 'user', text, time }
  const botId = nextId()
  const nextConversation: Message[] = [
    userMsg,
    {
      id: botId,
      role: 'bot',
      text: '',
      time: '',
      streaming: true,
    },
  ]

  replaceMessagesInStore(
    text.trim() === '/clear'
      ? nextConversation
      : [...useAgentChatStore.getState().messages, ...nextConversation],
    storageUserId
  )
  useAgentChatStore.getState().setLoading(true)

  if (text.trim() === '/clear') {
    try {
      clearAgentMessages(window.localStorage, storageUserId)
    } catch {
      // Ignore storage cleanup failure.
    }
  }

  let controller: AbortController | null = null
  try {
    activeStreamAbortController?.abort()
    controller = new AbortController()
    activeStreamAbortController = controller

    const res = await fetch('/api/agent/chat/stream', {
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

    const reader = res.body?.getReader()
    const decoder = new TextDecoder()
    if (!reader) throw new Error('No response body')
    activeStreamReader = reader
    controller.signal.addEventListener(
      'abort',
      () => {
        void reader.cancel().catch(() => {
          // Ignore double-cancel races.
        })
      },
      { once: true }
    )

    let buffer = ''
    let finalText = ''
    let stepCounter = 0
    const now = () =>
      new Date().toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
      })
    const mergeStreamText = (current: string, incoming: string) => {
      if (!incoming) return current
      if (!current) return incoming
      if (incoming === current) return current
      if (incoming.startsWith(current)) return incoming
      if (current.startsWith(incoming)) return current
      return current + incoming
    }

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      let eventType = ''
      for (const line of lines) {
        if (line.startsWith('event: ')) {
          eventType = line.slice(7).trim()
        } else if (line.startsWith('data: ') && eventType) {
          const rawData = line.slice(6)
          let data: string
          try {
            data = JSON.parse(rawData)
          } catch {
            eventType = ''
            continue
          }
          if (eventType === 'delta') {
            finalText = mergeStreamText(finalText, data)
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? { ...m, text: finalText, time: now() }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'plan') {
            const parsedSteps = parsePlanSteps(data)
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? {
                        ...m,
                        steps: parsedSteps.length > 0 ? parsedSteps : m.steps,
                        time: now(),
                      }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'step_start') {
            stepCounter += 1
            const nextStep = parseStepEvent(data, stepCounter)
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? {
                        ...m,
                        steps: appendStep(m.steps, nextStep),
                        time: now(),
                      }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'step_complete') {
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? {
                        ...m,
                        steps: markLatestRunningCompleted(m.steps, data),
                        time: now(),
                      }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'replan') {
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? {
                        ...m,
                        steps: appendStep(m.steps, {
                          id: `replan-${Date.now()}`,
                          label: data,
                          status: 'replanned',
                          detail: data,
                        }),
                        time: now(),
                      }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'done') {
            patchMessagesInStore(
              (prev) =>
                prev.map((m) =>
                  m.id === botId
                    ? {
                        ...m,
                        text: finalText || m.text || data,
                        time: now(),
                        streaming: false,
                      }
                    : m
                ),
              storageUserId
            )
          } else if (eventType === 'error') {
            throw new Error(data)
          }
          eventType = ''
        }
      }
    }

    patchMessagesInStore(
      (prev) =>
        prev.map((m) =>
          m.id === botId && m.streaming
            ? {
                ...m,
                text: finalText || m.text || 'No response',
                streaming: false,
                time: now(),
              }
            : m
        ),
      storageUserId
    )
    window.dispatchEvent(new CustomEvent('agent-preferences-refresh'))
    window.dispatchEvent(new CustomEvent('agent-config-refresh'))
  } catch (e: any) {
    if (e.name === 'AbortError') {
      patchMessagesInStore(
        (prev) =>
          prev.map((m) =>
            m.id === botId
              ? {
                  ...m,
                  streaming: false,
                  time:
                    m.time ||
                    new Date().toLocaleTimeString([], {
                      hour: '2-digit',
                      minute: '2-digit',
                    }),
                }
              : m
          ),
        storageUserId
      )
    } else {
      patchMessagesInStore(
        (prev) =>
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
          ),
        storageUserId
      )
    }
  }

  if (controller && activeStreamAbortController === controller) {
    activeStreamAbortController = null
  }
  if (activeStreamReader) {
    try {
      activeStreamReader.releaseLock()
    } catch {
      // Ignore lock-release races when the stream is already closed.
    }
    activeStreamReader = null
  }
  useAgentChatStore.getState().setLoading(false)
  onDone?.()
}

function appendStep(
  existing: AgentStep[] | undefined,
  step: AgentStep
): AgentStep[] {
  const prev = existing ?? []
  const index = prev.findIndex((item) => item.id === step.id)
  if (index === -1) return [...prev, step]
  return prev.map((item, i) => (i === index ? { ...item, ...step } : item))
}

function parsePlanSteps(data: string): AgentStep[] {
  const text = data.replace(/^🗺️\s*(Plan|计划):\s*/i, '').trim()
  if (!text) return []
  return text.split(/\s*->\s*/).map((part, index) => {
    const cleaned = part.replace(/^\d+\./, '').trim()
    return {
      id: `action-${index + 1}`,
      label: cleaned || `Step ${index + 1}`,
      status: 'pending',
    }
  })
}

function parseStepEvent(data: string, fallbackIndex: number): AgentStep {
  const match =
    data.match(/Step\s+(\d+)\/(\d+):\s+(.+)$/i) ||
    data.match(/步骤\s+(\d+)\/(\d+):\s+(.+)$/)
  if (match) {
    const id = `action-${match[1]}`
    return {
      id,
      label: match[3].trim(),
      status: 'running',
      detail: data,
    }
  }
  return {
    id: `step-${fallbackIndex}`,
    label: data,
    status: 'running',
    detail: data,
  }
}

function markLatestRunningCompleted(
  existing: AgentStep[] | undefined,
  detail: string
): AgentStep[] {
  const prev = existing ?? []
  for (let i = prev.length - 1; i >= 0; i--) {
    if (prev[i].status === 'running') {
      return prev.map((step, index) =>
        index === i ? { ...step, status: 'completed', detail } : step
      )
    }
  }
  return prev
}

export function AgentChatPage() {
  const { language } = useLanguage()
  const { token, user } = useAuth()
  const [storageUserId, setStorageUserId] = useState<string | undefined>(() =>
    getStoredAuthUserId()
  )
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 1024)
  const storageKey = chatStorageKey(user?.id || storageUserId)
  const messages = useAgentChatStore((state) => state.messages)
  const loading = useAgentChatStore((state) => state.loading)
  const historyHydrated = useAgentChatStore((state) => state.hydrated)
  const activeUserId = useAgentChatStore((state) => state.activeUserId)
  const resetForUser = useAgentChatStore((state) => state.resetForUser)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const chatInputRef = useRef<ChatInputHandle>(null)

  // Sidebar section collapse state
  const [sections, setSections] = useState({
    market: true,
    positions: true,
    traders: false,
    preferences: true,
  })

  const toggleSection = (key: keyof typeof sections) => {
    setSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  useEffect(() => {
    setStorageUserId(user?.id || getStoredAuthUserId())
  }, [user?.id])

  useEffect(() => {
    if (!user?.id) return
    migrateAgentMessages(window.localStorage, user.id)
  }, [user?.id])

  // Restore chat history for the current user when opening the agent page.
  useEffect(() => {
    const nextUserId = user?.id || storageUserId
    if (activeUserId === nextUserId && historyHydrated) return
    resetForUser(
      nextUserId,
      loadAgentMessages<Message>(window.localStorage, nextUserId).messages
    )
  }, [
    activeUserId,
    historyHydrated,
    resetForUser,
    storageKey,
    storageUserId,
    user?.id,
  ])

  // Persist chat history locally so page navigation does not wipe the conversation.
  useEffect(() => {
    if (!historyHydrated) return
    try {
      const persistable =
        prepareAgentMessagesForPersistence(messages).slice(-100)
      persistAgentMessages(
        window.localStorage,
        user?.id || storageUserId,
        persistable
      )
    } catch {
      // Ignore storage failures and keep the chat usable.
    }
  }, [historyHydrated, messages, storageKey, storageUserId, user?.id])

  // Responsive sidebar
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth <= 768) setSidebarOpen(false)
    }
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  useEffect(() => {
    const handlePageHide = () => cleanupActiveAgentStream()
    window.addEventListener('pagehide', handlePageHide)
    return () => window.removeEventListener('pagehide', handlePageHide)
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
    await runAgentStream({
      text,
      token,
      language,
      storageUserId: user?.id || storageUserId,
      onDone: () => chatInputRef.current?.focus(),
    })
  }

  const quickActions =
    language === 'zh'
      ? [
          { label: '💼 持仓', cmd: '/positions' },
          { label: '💰 余额', cmd: '/balance' },
          { label: '📋 Traders', cmd: '/traders' },
          { label: '📊 系统状态', cmd: '/status' },
          { label: '🧹 清除记忆', cmd: '/clear' },
          { label: '❓ 帮助', cmd: '/help' },
        ]
      : [
          { label: '💼 Positions', cmd: '/positions' },
          { label: '💰 Balance', cmd: '/balance' },
          { label: '📋 Traders', cmd: '/traders' },
          { label: '📊 Status', cmd: '/status' },
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
    {
      key: 'preferences' as const,
      icon: <Bookmark size={14} />,
      title: language === 'zh' ? '用户偏好' : 'Preferences',
      component: <UserPreferencesPanel token={token} language={language} />,
    },
  ]

  const isWelcomeState = messages.length === 0

  return (
    <div
      style={{
        display: 'flex',
        height: 'calc(100dvh - 64px)',
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
              onClick={() => void send(a.cmd)}
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
            onMouseEnter={(e) => {
              e.currentTarget.style.color = '#8a8aa0'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = '#4c4c62'
            }}
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
                      e.currentTarget.style.background =
                        'rgba(255,255,255,0.03)'
                      e.currentTarget.style.color = '#a0a0b0'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'transparent'
                      e.currentTarget.style.color = '#7a7a90'
                    }}
                  >
                    {section.icon}
                    <span>{section.title}</span>
                    <span
                      style={{
                        marginLeft: 'auto',
                        transition: 'transform 0.2s',
                      }}
                    >
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
                        <div style={{ paddingTop: 4 }}>{section.component}</div>
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
