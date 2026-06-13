import { useState, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  PanelRightClose,
  PanelRightOpen,
  TrendingUp,
  Wallet,
  Bot,
  Bookmark,
  Zap,
  Sparkles,
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
import { HyperliquidSymbolsPanel } from '../components/agent/HyperliquidSymbolsPanel'
import { AI500Panel } from '../components/agent/AI500Panel'
import type { AI500Coin } from '../lib/api/data'
import { createHyperliquidQuickTrader } from '../lib/hyperliquidQuickTrade'
import { useAgentChatStore } from '../stores/agentChatStore'
import type { AgentMessage as Message, AgentStep } from '../types/agent'
import {
  chatStorageKey,
  clearAgentMessages,
  getStoredAuthUserId,
  loadAgentDraft,
  loadAgentMessages,
  migrateAgentMessages,
  prepareAgentMessagesForPersistence,
  persistAgentDraft,
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

function stopActiveAgentStream(userId?: string, language = 'zh') {
  if (!activeStreamAbortController && !activeStreamReader) return
  const stoppedText =
    language === 'zh' ? '已中止当前回复。' : 'Stopped the current response.'
  const now = new Date().toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
  patchMessagesInStore(
    (prev) =>
      prev.map((m) => {
        if (m.role !== 'bot' || !m.streaming) return m
        const text = m.text?.trim()
          ? `${m.text.trimEnd()}\n\n${stoppedText}`
          : stoppedText
        return {
          ...m,
          text,
          streaming: false,
          time: m.time || now,
        }
      }),
    userId
  )
  cleanupActiveAgentStream()
  useAgentChatStore.getState().setLoading(false)
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
      useAgentChatStore.getState().setDraftText('')
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
                  m.id === botId ? { ...m, text: finalText, time: now() } : m
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
  const draftText = useAgentChatStore((state) => state.draftText)
  const loading = useAgentChatStore((state) => state.loading)
  const historyHydrated = useAgentChatStore((state) => state.hydrated)
  const activeUserId = useAgentChatStore((state) => state.activeUserId)
  const resetForUser = useAgentChatStore((state) => state.resetForUser)
  const setDraftText = useAgentChatStore((state) => state.setDraftText)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const chatInputRef = useRef<ChatInputHandle>(null)
  const pendingHyperSymbolRef = useRef<string | null>(null)

  // Sidebar section collapse state
  const [sections, setSections] = useState({
    market: true,
    ai500: true,
    positions: true,
    traders: false,
    preferences: true,
    hyperliquid: true,
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
    setDraftText(loadAgentDraft(window.localStorage, nextUserId))
  }, [
    activeUserId,
    historyHydrated,
    resetForUser,
    setDraftText,
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

  // Persist the unsent draft so navigating away from the Agent page does not
  // wipe what the user was typing.
  useEffect(() => {
    if (!historyHydrated) return
    try {
      persistAgentDraft(
        window.localStorage,
        user?.id || storageUserId,
        draftText
      )
    } catch {
      // Ignore storage failures and keep typing responsive.
    }
  }, [draftText, historyHydrated, storageKey, storageUserId, user?.id])

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

  const stopCurrentResponse = () => {
    stopActiveAgentStream(user?.id || storageUserId, language)
    chatInputRef.current?.focus()
  }

  const analyzeAI500Symbol = (coin: AI500Coin) => {
    const display = coin.pair.replace(/USDT$/i, '')
    const text =
      language === 'zh'
        ? `分析一下 AI500 里的 ${display}（当前 AI 评分 ${coin.score.toFixed(1)}），给出趋势判断和交易建议`
        : `Analyze ${display} from the AI500 index (current AI score ${coin.score.toFixed(1)}) and give me a trend read plus a trading suggestion`
    void send(text)
  }

  const tradeHyperliquidSymbol = async (symbol: { symbol: string; display?: string; category?: string }) => {
    const label = symbol.display || symbol.symbol
    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    patchMessagesInStore(
      (prev) => [
        ...prev,
        {
          id: nextId(),
          role: 'user',
          text:
            language === 'zh'
              ? `快速创建 Hyperliquid ${label} 单标的交易员`
              : `Quick-create a Hyperliquid ${label} single-symbol trader`,
          time,
        },
        {
          id: nextId(),
          role: 'bot',
          text: language === 'zh' ? '正在直接创建策略和 Trader，不再走聊天意图猜测…' : 'Creating the strategy and trader directly, without routing through chat intent guessing…',
          time,
          streaming: true,
        },
      ],
      user?.id || storageUserId
    )
    try {
      const result = await createHyperliquidQuickTrader(symbol, language === 'zh' ? 'zh' : 'en')

      // The reply tells the truth about what happened: created OR reused, AND
      // whether the trader is now actually running. If start failed we say so
      // explicitly so the user knows there's still work to do — but we no
      // longer pretend the user has to "go push a button" when we could've
      // done it ourselves.
      const isZh = language === 'zh'
      let text: string
      if (result.started) {
        const verb = isZh
          ? result.reusedTrader
            ? '已复用并启动'
            : '已创建并启动'
          : result.reusedTrader
            ? 'Reused and started'
            : 'Created and started'
        text = isZh
          ? `${verb} Hyperliquid ${result.display} 单标的 Trader: ${result.traderName}\n\n• 策略: ${result.strategyName}\n• 标的: ${result.display}\n• 扫描间隔: 每 5 分钟\n\nAgent 已在工作。要看实时持仓在 Dashboard, 想停掉随时回这里说"停掉 ${result.traderName}"。`
          : `${verb} Hyperliquid ${result.display} single-symbol trader: ${result.traderName}\n\n• Strategy: ${result.strategyName}\n• Symbol: ${result.display}\n• Scan interval: every 5 min\n\nThe agent is live. Watch positions in Dashboard, or tell me "stop ${result.traderName}" to halt it.`
      } else {
        const verb = isZh
          ? result.reusedTrader
            ? '已找到 Trader'
            : '已创建 Trader'
          : result.reusedTrader
            ? 'Found existing trader'
            : 'Created trader'
        const reason = result.startError ? `\n\n启动失败原因: ${result.startError}` : ''
        const reasonEn = result.startError ? `\n\nStart failed: ${result.startError}` : ''
        text = isZh
          ? `${verb}: ${result.traderName}\n\n• 策略: ${result.strategyName}\n• 标的: ${result.display}${reason}\n\n需要先解决启动问题, 我才能让它跑起来。`
          : `${verb}: ${result.traderName}\n\n• Strategy: ${result.strategyName}\n• Symbol: ${result.display}${reasonEn}\n\nResolve the start error before the trader can run.`
      }

      patchMessagesInStore(
        (prev) =>
          prev.map((m) =>
            m.streaming
              ? {
                  ...m,
                  streaming: false,
                  time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                  text,
                }
              : m
          ),
        user?.id || storageUserId
      )
      window.dispatchEvent(new CustomEvent('agent-config-refresh'))
    } catch (err: any) {
      patchMessagesInStore(
        (prev) =>
          prev.map((m) =>
            m.streaming
              ? {
                  ...m,
                  streaming: false,
                  time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                  text: `⚠️ ${err?.message || 'Failed to create quick trader'}`,
                }
              : m
          ),
        user?.id || storageUserId
      )
    }
  }

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const symbol = params.get('hyperSymbol')
    if (!symbol || pendingHyperSymbolRef.current === symbol || loading) return
    pendingHyperSymbolRef.current = symbol
    void tradeHyperliquidSymbol({
      symbol,
      display: params.get('hyperDisplay') || symbol,
      category: params.get('hyperCategory') || 'stock',
    })
    const cleanUrl = `${window.location.pathname}${window.location.hash}`
    window.history.replaceState({}, '', cleanUrl)
  }, [loading, language])

  const quickActions =
    language === 'zh'
      ? [
          { label: '🇺🇸 创建美股Agent', cmd: '创建一个美股趋势交易 Agent，默认选择5个强势美股，严格风控' },
          { label: '🗣 一句话策略', cmd: '我想做美股强趋势突破，帮我生成策略和Agent' },
          { label: '💼 持仓', cmd: '/positions' },
          { label: '💰 余额', cmd: '/balance' },
          { label: '📋 Traders', cmd: '/traders' },
          { label: '📊 系统状态', cmd: '/status' },
          { label: '🧹 清除记忆', cmd: '/clear' },
          { label: '❓ 帮助', cmd: '/help' },
        ]
      : [
          { label: '🇺🇸 Create US Stock Agent', cmd: 'Create a US stock trend-following agent with 5 strong stocks and strict risk control' },
          { label: '🗣 One-line strategy', cmd: 'I want a US stock breakout strategy; build the strategy and agent' },
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
      key: 'ai500' as const,
      icon: <Sparkles size={14} />,
      title: language === 'zh' ? 'AI500 精选' : 'AI500 Picks',
      component: (
        <AI500Panel
          language={language}
          disabled={loading}
          onAnalyzeSymbol={analyzeAI500Symbol}
        />
      ),
    },
    {
      key: 'hyperliquid' as const,
      icon: <Zap size={14} />,
      title: language === 'zh' ? '美股/全球标的' : 'US & Global Markets',
      component: (
        <HyperliquidSymbolsPanel
          language={language}
          disabled={loading}
          onTradeSymbol={tradeHyperliquidSymbol}
        />
      ),
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
        background: '#0B0E11',
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
            background: 'rgba(11,14,17,0.8)',
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
          value={draftText}
          onChange={setDraftText}
          onSend={send}
          onStop={stopCurrentResponse}
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
