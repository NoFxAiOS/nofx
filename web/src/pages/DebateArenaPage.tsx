import { useState, useEffect } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { notify } from '../lib/notify'
import { useLanguage } from '../contexts/LanguageContext'
import { PunkAvatar } from '../components/PunkAvatar'
import type {
  DebateSession,
  DebateSessionWithDetails,
  DebateMessage,
  CreateDebateRequest,
  AIModel,
  Strategy,
  DebatePersonality,
  TraderInfo,
} from '../types'
import {
  Plus,
  X,
  Trophy,
  Loader2,
  TrendingUp,
  TrendingDown,
  Minus,
  Clock,
  Zap,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'

// Translations
const T: Record<string, Record<string, string>> = {
  newDebate: { zh: '新建辩论', en: 'New Debate' },
  debateSessions: { zh: '辩论会话', en: 'Sessions' },
  onlineTraders: { zh: '在线交易员', en: 'Online Traders' },
  offline: { zh: '离线', en: 'Offline' },
  noTraders: { zh: '暂无交易员', en: 'No traders' },
  start: { zh: '开始', en: 'Start' },
  delete: { zh: '删除', en: 'Delete' },
  discussionRecords: { zh: '讨论记录', en: 'Discussion' },
  finalVotes: { zh: '最终投票', en: 'Final Votes' },
  consensus: { zh: '共识', en: 'Consensus' },
  confidence: { zh: '信心', en: 'Confidence' },
  leverage: { zh: '杠杆', en: 'Leverage' },
  position: { zh: '仓位', en: 'Position' },
  execute: { zh: '执行', en: 'Execute' },
  executed: { zh: '已执行', en: 'Executed' },
  selectOrCreate: { zh: '选择或创建辩论', en: 'Select or create a debate' },
  clickToStart: { zh: '点击左侧"开始"启动辩论', en: 'Click "Start" to begin' },
  waitingAI: { zh: '等待AI发言...', en: 'Waiting for AI...' },
  createDebate: { zh: '创建辩论', en: 'Create Debate' },
  debateName: { zh: '辩论名称', en: 'Debate Name' },
  tradingPair: { zh: '交易对', en: 'Trading Pair' },
  strategy: { zh: '策略', en: 'Strategy' },
  rounds: { zh: '轮数', en: 'Rounds' },
  participants: { zh: '参与者', en: 'Participants' },
  addAI: { zh: '添加AI', en: 'Add AI' },
  cancel: { zh: '取消', en: 'Cancel' },
  create: { zh: '创建', en: 'Create' },
  creating: { zh: '创建中...', en: 'Creating...' },
  executeTitle: { zh: '执行交易', en: 'Execute Trade' },
  selectTrader: { zh: '选择交易员', en: 'Select Trader' },
  executing: { zh: '执行中...', en: 'Executing...' },
  fillNameAdd2AI: { zh: '请填写名称并添加至少2个AI', en: 'Please fill name and add at least 2 AI' },
}
const t = (key: string, lang: string) => T[key]?.[lang] || T[key]?.en || key

// Personality config
const PERS: Record<DebatePersonality, { emoji: string; color: string; name: string; nameEn: string }> = {
  bull: { emoji: '🐂', color: '#22C55E', name: '多头', nameEn: 'Bull' },
  bear: { emoji: '🐻', color: '#EF4444', name: '空头', nameEn: 'Bear' },
  analyst: { emoji: '📊', color: '#3B82F6', name: '分析', nameEn: 'Analyst' },
  contrarian: { emoji: '🔄', color: '#F59E0B', name: '逆势', nameEn: 'Contrarian' },
  risk_manager: { emoji: '🛡️', color: '#8B5CF6', name: '风控', nameEn: 'Risk Mgr' },
}

// Action config
const ACT: Record<string, { color: string; bg: string; icon: JSX.Element; label: string }> = {
  open_long: { color: 'text-green-400', bg: 'bg-green-500/20', icon: <TrendingUp size={14} />, label: 'LONG' },
  open_short: { color: 'text-red-400', bg: 'bg-red-500/20', icon: <TrendingDown size={14} />, label: 'SHORT' },
  hold: { color: 'text-blue-400', bg: 'bg-blue-500/20', icon: <Minus size={14} />, label: 'HOLD' },
  wait: { color: 'text-gray-400', bg: 'bg-gray-500/20', icon: <Clock size={14} />, label: 'WAIT' },
  close_long: { color: 'text-yellow-400', bg: 'bg-yellow-500/20', icon: <X size={14} />, label: 'CLOSE' },
  close_short: { color: 'text-yellow-400', bg: 'bg-yellow-500/20', icon: <X size={14} />, label: 'CLOSE' },
}

// Status colors
const STATUS_COLOR: Record<string, string> = {
  pending: 'bg-gray-500',
  running: 'bg-blue-500 animate-pulse',
  voting: 'bg-yellow-500 animate-pulse',
  completed: 'bg-green-500',
  cancelled: 'bg-red-500',
}

// AI Provider Avatar
function AIAvatar({ name, size = 24, className = '' }: { name: string; size?: number; className?: string }) {
  const providers: Record<string, { bg: string; text: string; letter: string }> = {
    claude: { bg: 'bg-orange-500', text: 'text-white', letter: 'C' },
    deepseek: { bg: 'bg-blue-600', text: 'text-white', letter: 'D' },
    gemini: { bg: 'bg-blue-400', text: 'text-white', letter: 'G' },
    grok: { bg: 'bg-gray-700', text: 'text-white', letter: 'X' },
    kimi: { bg: 'bg-purple-500', text: 'text-white', letter: 'K' },
    qwen: { bg: 'bg-indigo-500', text: 'text-white', letter: 'Q' },
    openai: { bg: 'bg-emerald-600', text: 'text-white', letter: 'O' },
    gpt: { bg: 'bg-emerald-600', text: 'text-white', letter: 'O' },
  }
  const lower = name.toLowerCase()
  const p = Object.entries(providers).find(([k]) => lower.includes(k))?.[1]
    || { bg: 'bg-gray-600', text: 'text-white', letter: name[0]?.toUpperCase() || '?' }
  return (
    <div className={`${p.bg} ${p.text} rounded-md flex items-center justify-center font-bold ${className}`}
      style={{ width: size, height: size, fontSize: size * 0.5 }}>
      {p.letter}
    </div>
  )
}

// Message Card - Full content display like AI Testing
function MessageCard({ msg }: { msg: DebateMessage }) {
  const [open, setOpen] = useState(false)
  const p = PERS[msg.personality] || PERS.analyst
  const a = ACT[msg.decision?.action || 'wait'] || ACT.wait

  // Parse content into sections
  const parseContent = (c: string) => {
    const reasoning = c.match(/<reasoning>([\s\S]*?)<\/reasoning>/i)?.[1]?.trim()
    const analysis = c.match(/<analysis>([\s\S]*?)<\/analysis>/i)?.[1]?.trim()
    const argument = c.match(/<argument>([\s\S]*?)<\/argument>/i)?.[1]?.trim()
    const decision = c.match(/<decision>([\s\S]*?)<\/decision>/i)?.[1]?.trim()

    // Clean content - remove XML tags
    const cleanContent = c.replace(/<\/?[^>]+(>|$)/g, '').trim()

    return {
      reasoning: reasoning || analysis || argument,
      decision,
      fullContent: cleanContent
    }
  }

  const parsed = parseContent(msg.content)
  const previewText = parsed.reasoning?.slice(0, 150) || parsed.fullContent.slice(0, 150)

  return (
    <div
      className="p-2 sm:p-3 rounded-lg hover:bg-white/5 transition-all border border-white/5"
      style={{ borderLeft: `3px solid ${p.color}` }}
    >
      {/* Header - Always visible */}
      <div
        className="flex items-center gap-1.5 sm:gap-2 cursor-pointer flex-wrap"
        onClick={() => setOpen(!open)}
      >
        <AIAvatar name={msg.ai_model_name} size={20} className="sm:w-6 sm:h-6" />
        <span className="text-xs sm:text-sm text-white font-medium truncate flex-1 min-w-0">{msg.ai_model_name}</span>
        <span className="text-[10px] sm:text-xs text-gray-500 hidden sm:inline">{p.nameEn}</span>
        {msg.decision && (
          <span className={`flex items-center gap-0.5 sm:gap-1 text-[10px] sm:text-xs px-1.5 sm:px-2 py-0.5 rounded ${a.bg} ${a.color}`}>
            {a.icon} <span className="hidden sm:inline">{msg.decision.symbol || ''} </span>{a.label}
          </span>
        )}
        <span className="text-[10px] sm:text-xs text-yellow-400 font-medium">{msg.decision?.confidence || msg.confidence}%</span>
        {open ? <ChevronUp size={12} className="sm:w-3.5 sm:h-3.5 text-gray-500" /> : <ChevronDown size={12} className="sm:w-3.5 sm:h-3.5 text-gray-500" />}
      </div>

      {/* Preview when collapsed */}
      {!open && (
        <div className="mt-1.5 sm:mt-2 text-[10px] sm:text-xs text-gray-400 line-clamp-2">
          {previewText}...
        </div>
      )}

      {/* Expanded Content - Full display */}
      {open && (
        <div className="mt-2 sm:mt-3 space-y-2 sm:space-y-3">
          {/* Reasoning/Analysis Section */}
          {parsed.reasoning && (
            <div className="bg-black/20 rounded-lg p-2 sm:p-3">
              <div className="text-[10px] sm:text-xs text-blue-400 font-medium mb-1.5 sm:mb-2">💭 {language === 'zh' ? '思考过程' : 'Reasoning'}</div>
              <div className="text-[10px] sm:text-xs text-gray-300 leading-relaxed whitespace-pre-wrap max-h-48 sm:max-h-64 overflow-y-auto select-text">
                {parsed.reasoning}
              </div>
            </div>
          )}

          {/* Decision Section */}
          {msg.decision && (
            <div className="bg-black/20 rounded-lg p-2 sm:p-3">
              <div className="text-[10px] sm:text-xs text-green-400 font-medium mb-1.5 sm:mb-2">📊 {language === 'zh' ? '交易决策' : 'Decision'}</div>
              <div className="grid grid-cols-2 gap-1.5 sm:gap-2 text-[10px] sm:text-xs">
                {msg.decision.symbol && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">币种</span>
                    <span className="text-white font-medium">{msg.decision.symbol}</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-gray-500">方向</span>
                  <span className={a.color}>{a.label}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">信心</span>
                  <span className="text-yellow-400">{msg.decision.confidence}%</span>
                </div>
                {(msg.decision.leverage ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">杠杆</span>
                    <span className="text-white">{msg.decision.leverage}x</span>
                  </div>
                )}
                {(msg.decision.position_pct ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">仓位</span>
                    <span className="text-white">{((msg.decision.position_pct ?? 0) * 100).toFixed(0)}%</span>
                  </div>
                )}
                {(msg.decision.stop_loss ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">止损</span>
                    <span className="text-red-400">{((msg.decision.stop_loss ?? 0) * 100).toFixed(1)}%</span>
                  </div>
                )}
                {(msg.decision.take_profit ?? 0) > 0 && (
                  <div className="flex justify-between">
                    <span className="text-gray-500">止盈</span>
                    <span className="text-green-400">{((msg.decision.take_profit ?? 0) * 100).toFixed(1)}%</span>
                  </div>
                )}
              </div>
              {msg.decision.reasoning && (
                <div className="mt-2 pt-2 border-t border-white/10 text-xs text-gray-400">
                  {msg.decision.reasoning}
                </div>
              )}
            </div>
          )}

          {/* Full Raw Content (collapsible) */}
          {!parsed.reasoning && (
            <div className="bg-black/20 rounded-lg p-2 sm:p-3">
              <div className="text-[10px] sm:text-xs text-gray-400 font-medium mb-1.5 sm:mb-2">📝 {language === 'zh' ? '完整输出' : 'Full Output'}</div>
              <div className="text-[10px] sm:text-xs text-gray-300 leading-relaxed whitespace-pre-wrap max-h-64 sm:max-h-96 overflow-y-auto select-text">
                {parsed.fullContent}
              </div>
            </div>
          )}

          {/* Multi-coin decisions if available */}
          {msg.decisions && msg.decisions.length > 1 && (
            <div className="bg-black/20 rounded-lg p-2 sm:p-3">
              <div className="text-[10px] sm:text-xs text-purple-400 font-medium mb-1.5 sm:mb-2">🎯 {language === 'zh' ? '多币种决策' : 'Multi-coin Decisions'} ({msg.decisions.length})</div>
              <div className="space-y-1.5 sm:space-y-2">
                {msg.decisions.map((d, i) => {
                  const da = ACT[d.action] || ACT.wait
                  return (
                    <div key={i} className="flex items-center justify-between text-[10px] sm:text-xs p-1.5 sm:p-2 bg-white/5 rounded flex-wrap gap-1">
                      <span className="text-white font-medium">{d.symbol}</span>
                      <span className={da.color}>{da.icon} {da.label}</span>
                      <span className="text-yellow-400">{d.confidence}%</span>
                      <span className="text-gray-400">{d.leverage || 0}x / {((d.position_pct || 0) * 100).toFixed(0)}%</span>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// Vote Card - Beautiful detailed version
function VoteCard({ vote }: { vote: { ai_model_name: string; action: string; symbol?: string; confidence: number; leverage?: number; position_pct?: number; stop_loss_pct?: number; take_profit_pct?: number; reasoning: string } }) {
  const a = ACT[vote.action] || ACT.wait
  const confColor = vote.confidence >= 70 ? 'bg-green-500' : vote.confidence >= 50 ? 'bg-yellow-500' : 'bg-gray-500'
  return (
    <div className="bg-[#1a1f2e] rounded-xl p-2.5 sm:p-4 border border-white/10 hover:border-white/20 transition-all">
      <div className="flex items-center justify-between mb-2 sm:mb-3 flex-wrap gap-1.5 sm:gap-0">
        <div className="flex items-center gap-1.5 sm:gap-2 min-w-0 flex-1">
          <AIAvatar name={vote.ai_model_name} size={24} className="sm:w-7 sm:h-7" />
          <div className="min-w-0 flex-1">
            <span className="text-white font-semibold block text-xs sm:text-sm truncate">{vote.ai_model_name}</span>
            {vote.symbol && <span className="text-[10px] sm:text-xs text-gray-400">{vote.symbol}</span>}
          </div>
        </div>
        <span className={`flex items-center gap-0.5 sm:gap-1 px-1.5 sm:px-2.5 py-0.5 sm:py-1 rounded-lg text-[10px] sm:text-xs font-bold ${a.bg} ${a.color}`}>
          {a.icon} <span className="hidden sm:inline">{vote.action.replace('_', ' ').toUpperCase()}</span>
        </span>
      </div>
      <div className="mb-2 sm:mb-3">
        <div className="flex justify-between text-xs sm:text-sm mb-0.5 sm:mb-1">
          <span className="text-gray-400">Confidence</span>
          <span className="text-white font-bold">{vote.confidence}%</span>
        </div>
        <div className="h-1.5 sm:h-2 bg-gray-700 rounded-full overflow-hidden">
          <div className={`h-full ${confColor} rounded-full transition-all`} style={{ width: `${vote.confidence}%` }} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-x-2 sm:gap-x-4 gap-y-0.5 sm:gap-y-1 text-xs sm:text-sm">
        <div className="flex justify-between"><span className="text-gray-500">Leverage</span><span className="text-white font-semibold">{vote.leverage || '-'}x</span></div>
        <div className="flex justify-between"><span className="text-gray-500">Position</span><span className="text-white font-semibold">{vote.position_pct ? `${(vote.position_pct * 100).toFixed(0)}%` : '-'}</span></div>
        <div className="flex justify-between"><span className="text-gray-500">SL</span><span className="text-red-400 font-semibold">{vote.stop_loss_pct ? `${(vote.stop_loss_pct * 100).toFixed(1)}%` : '-'}</span></div>
        <div className="flex justify-between"><span className="text-gray-500">TP</span><span className="text-green-400 font-semibold">{vote.take_profit_pct ? `${(vote.take_profit_pct * 100).toFixed(1)}%` : '-'}</span></div>
      </div>
      {vote.reasoning && (
        <p className="mt-2 sm:mt-3 text-[10px] sm:text-xs text-gray-400 leading-relaxed line-clamp-2 border-t border-white/5 pt-1.5 sm:pt-2">{vote.reasoning}</p>
      )}
    </div>
  )
}

// Create Modal (simplified)
function CreateModal({
  isOpen, onClose, onCreate, aiModels, strategies, language
}: {
  isOpen: boolean; onClose: () => void; onCreate: (r: CreateDebateRequest) => Promise<void>
  aiModels: AIModel[]; strategies: Strategy[]; language: string
}) {
  const [name, setName] = useState('')
  const [symbol, setSymbol] = useState('')
  const [strategyId, setStrategyId] = useState('')
  const [maxRounds, setMaxRounds] = useState(3)
  const [participants, setParticipants] = useState<{ ai_model_id: string; personality: DebatePersonality }[]>([])
  const [creating, setCreating] = useState(false)

  // Get the selected strategy's coin source config
  const selectedStrategy = strategies.find(s => s.id === strategyId)
  const coinSource = selectedStrategy?.config?.coin_source
  const sourceType = coinSource?.source_type || 'static'
  const staticCoins = coinSource?.static_coins || []
  // Only show coin selector for static type with coins defined
  const isStaticWithCoins = sourceType === 'static' && staticCoins.length > 0

  useEffect(() => {
    if (isOpen) {
      const firstStrategy = strategies[0]
      const firstStrategyId = firstStrategy?.id || ''
      const firstCoinSource = firstStrategy?.config?.coin_source
      const firstSourceType = firstCoinSource?.source_type || 'static'
      const firstStaticCoins = firstCoinSource?.static_coins || []
      setName('')
      setStrategyId(firstStrategyId)
      // Only set symbol for static type, otherwise leave empty (backend will choose)
      setSymbol(firstSourceType === 'static' && firstStaticCoins.length > 0 ? firstStaticCoins[0] : '')
      setMaxRounds(3)
      setParticipants([])
    }
  }, [isOpen, strategies])

  // Update symbol when strategy changes
  useEffect(() => {
    if (isStaticWithCoins) {
      if (!staticCoins.includes(symbol)) {
        setSymbol(staticCoins[0])
      }
    } else {
      // Non-static strategy: clear symbol, backend will auto-select
      setSymbol('')
    }
  }, [strategyId, isStaticWithCoins, staticCoins, symbol])

  const addP = () => {
    if (participants.length >= 10 || aiModels.length === 0) return
    // Allow same AI model to be used multiple times with different personalities
    const order: DebatePersonality[] = ['bull', 'bear', 'analyst', 'contrarian', 'risk_manager']
    // Cycle through personalities
    const nextPersonality = order[participants.length % order.length]
    setParticipants([...participants, { ai_model_id: aiModels[0].id, personality: nextPersonality }])
  }

  const submit = async () => {
    if (!name || !strategyId || participants.length < 2) {
      notify.error(t('fillNameAdd2AI', language))
      return
    }
    setCreating(true)
    try {
      await onCreate({ name, symbol, strategy_id: strategyId, max_rounds: maxRounds, participants })
      onClose()
    } finally { setCreating(false) }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-3 sm:p-4">
      <div className="bg-[#1a1d24] rounded-xl w-full max-w-md p-3 sm:p-4 border border-white/10 max-h-[90vh] overflow-y-auto">
        <div className="flex justify-between items-center mb-3 sm:mb-4">
          <h3 className="text-base sm:text-lg font-bold text-white">{t('createDebate', language)}</h3>
          <button onClick={onClose}><X size={18} className="sm:w-5 sm:h-5 text-gray-400" /></button>
        </div>

        <div className="space-y-2 sm:space-y-3">
          <input
            value={name} onChange={e => setName(e.target.value)}
            placeholder={t('debateName', language)} className="w-full px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-white text-xs sm:text-sm"
          />

          {/* Strategy selector - moved up */}
          <select value={strategyId} onChange={e => setStrategyId(e.target.value)}
            className="w-full px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-white text-xs sm:text-sm">
            {strategies.map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>

          <div className="flex flex-col sm:flex-row gap-2">
            {/* Show dropdown only for static type with coins defined */}
            {isStaticWithCoins ? (
              <select value={symbol} onChange={e => setSymbol(e.target.value)}
                className="flex-1 px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-white text-xs sm:text-sm">
                {staticCoins.map(coin => <option key={coin} value={coin}>{coin}</option>)}
              </select>
            ) : (
              <div className="flex-1 px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-gray-400 text-xs sm:text-sm">
                {language === 'zh' ? '根据策略规则自动选择' : 'Auto-selected by strategy'}
              </div>
            )}
            <select value={maxRounds} onChange={e => setMaxRounds(+e.target.value)}
              className="px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-white text-xs sm:text-sm w-full sm:w-auto">
              {[2,3,4,5].map(n => <option key={n} value={n}>{n} {language === 'zh' ? '轮' : 'rounds'}</option>)}
            </select>
          </div>

          {/* Participants */}
          <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap">
            {participants.map((p, i) => (
              <div key={i} className="flex items-center gap-0.5 sm:gap-1 px-1.5 sm:px-2 py-0.5 sm:py-1 rounded-lg text-[10px] sm:text-xs"
                style={{ backgroundColor: `${PERS[p.personality].color}20`, border: `1px solid ${PERS[p.personality].color}40` }}>
                {/* Personality selector */}
                <select value={p.personality} onChange={e => {
                  const up = [...participants]; up[i].personality = e.target.value as DebatePersonality; setParticipants(up)
                }} className="bg-transparent text-white text-[10px] sm:text-xs border-0 outline-none cursor-pointer">
                  {Object.entries(PERS).map(([k, v]) => (
                    <option key={k} value={k}>{v.emoji} {language === 'zh' ? v.name : v.nameEn}</option>
                  ))}
                </select>
                {/* AI model selector */}
                <select value={p.ai_model_id} onChange={e => {
                  const up = [...participants]; up[i].ai_model_id = e.target.value; setParticipants(up)
                }} className="bg-transparent text-white text-[10px] sm:text-xs border-0 outline-none">
                  {aiModels.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
                </select>
                <button onClick={() => setParticipants(participants.filter((_, j) => j !== i))}
                  className="text-red-400 hover:text-red-300"><X size={10} className="sm:w-3 sm:h-3" /></button>
              </div>
            ))}
            <button onClick={addP} className="px-1.5 sm:px-2 py-0.5 sm:py-1 text-[10px] sm:text-xs text-yellow-400 hover:bg-yellow-500/10 rounded">
              + {t('addAI', language)}
            </button>
          </div>
        </div>

        <div className="flex flex-col sm:flex-row gap-2 mt-3 sm:mt-4">
          <button onClick={onClose} className="flex-1 py-1.5 sm:py-2 rounded-lg bg-white/5 text-white text-xs sm:text-sm">{t('cancel', language)}</button>
          <button onClick={submit} disabled={creating}
            className="flex-1 py-1.5 sm:py-2 rounded-lg bg-yellow-500 text-black font-semibold text-xs sm:text-sm disabled:opacity-50">
            {creating ? <Loader2 size={14} className="sm:w-4 sm:h-4 animate-spin mx-auto" /> : t('create', language)}
          </button>
        </div>
      </div>
    </div>
  )
}

// Main Page
export function DebateArenaPage() {
  const { language } = useLanguage()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [execId, setExecId] = useState<string | null>(null)
  const [traderId, setTraderId] = useState('')
  const [executing, setExecuting] = useState(false)

  const { data: debates, mutate: mutateList } = useSWR<DebateSession[]>('debates', api.getDebates, { refreshInterval: 5000 })
  const { data: aiModels } = useSWR<AIModel[]>('ai-models', api.getModelConfigs)
  const { data: strategies } = useSWR<Strategy[]>('strategies', api.getStrategies)
  const { data: traders } = useSWR<TraderInfo[]>('traders', api.getTraders)
  const { data: detail, mutate: mutateDetail } = useSWR<DebateSessionWithDetails>(
    selectedId ? `debate-${selectedId}` : null,
    () => api.getDebate(selectedId!),
    { refreshInterval: selectedId ? 3000 : 0 }
  )

  useEffect(() => {
    if (debates?.length && !selectedId) setSelectedId(debates[0].id)
  }, [debates, selectedId])

  const onCreate = async (r: CreateDebateRequest) => {
    const d = await api.createDebate(r)
    notify.success('创建成功')
    mutateList()
    setSelectedId(d.id)
  }

  const onStart = async (id: string) => {
    await api.startDebate(id)
    notify.success('已开始')
    mutateList(); mutateDetail()
  }

  const onDelete = async (id: string) => {
    await api.deleteDebate(id)
    notify.success('已删除')
    if (selectedId === id) setSelectedId(null)
    mutateList()
  }

  const onExecute = async () => {
    if (!execId || !traderId) return
    setExecuting(true)
    try {
      await api.executeDebate(execId, traderId)
      notify.success('已执行')
      mutateDetail(); mutateList()
      setExecId(null); setTraderId('')
    } catch (e: any) { notify.error(e.message) }
    finally { setExecuting(false) }
  }

  // Process data
  const messages = detail?.messages || []
  const participants = detail?.participants || []
  const votes = detail?.votes || []
  const decision = detail?.final_decision

  // Get strategy name
  const strategyName = strategies?.find(s => s.id === detail?.strategy_id)?.name || ''

  // Group by round
  const rounds: Record<number, DebateMessage[]> = {}
  messages.forEach(m => { if (!rounds[m.round]) rounds[m.round] = []; rounds[m.round].push(m) })

  // Vote summary
  const voteSum = votes.reduce((a, v) => { a[v.action] = (a[v.action] || 0) + 1; return a }, {} as Record<string, number>)

  return (
    <div className="h-full bg-[#0a0c10] flex flex-col lg:flex-row overflow-hidden">
      {/* Left - Debate List + Online Traders */}
      <div className="w-full lg:w-56 flex-shrink-0 bg-[#0d1017] border-b lg:border-b-0 lg:border-r border-white/5 flex flex-col" style={{ maxHeight: '40vh', lg: { maxHeight: 'none' } }}>
        {/* New Debate Button */}
        <button onClick={() => setShowCreate(true)}
          className="m-1.5 sm:m-2 py-1.5 sm:py-2 rounded-lg bg-yellow-500 text-black font-semibold text-xs sm:text-sm flex items-center justify-center gap-1">
          <Plus size={14} className="sm:w-4 sm:h-4" /> <span className="hidden sm:inline">{t('newDebate', language)}</span><span className="sm:hidden">{language === 'zh' ? '新建' : 'New'}</span>
        </button>

        {/* Debate List */}
        <div className="px-1.5 sm:px-2 py-0.5 sm:py-1 text-[10px] sm:text-xs text-gray-500 font-semibold">{t('debateSessions', language)}</div>
        <div className="overflow-y-auto flex-1" style={{ maxHeight: 'calc(40vh - 100px)', lg: { maxHeight: '30%' } }}>
          {debates?.map(d => (
            <div key={d.id} onClick={() => setSelectedId(d.id)}
              className={`p-2 cursor-pointer border-l-2 ${selectedId === d.id ? 'bg-yellow-500/10 border-yellow-500' : 'border-transparent hover:bg-white/5'}`}>
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${STATUS_COLOR[d.status]}`} />
                <span className="text-sm text-white truncate flex-1">{d.name}</span>
              </div>
              <div className="text-xs text-gray-500 mt-1">{d.symbol} · R{d.current_round}/{d.max_rounds}</div>
              {d.status === 'pending' && selectedId === d.id && (
                <div className="flex gap-1 mt-1">
                  <button onClick={e => { e.stopPropagation(); onStart(d.id) }}
                    className="text-xs px-2 py-0.5 bg-green-500/20 text-green-400 rounded">{t('start', language)}</button>
                  <button onClick={e => { e.stopPropagation(); onDelete(d.id) }}
                    className="text-xs px-2 py-0.5 bg-red-500/20 text-red-400 rounded">{t('delete', language)}</button>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Online Traders Section */}
        <div className="flex-1 border-t border-white/5 mt-2 overflow-hidden flex flex-col">
          <div className="px-2 py-2 text-xs text-gray-500 font-semibold flex items-center gap-1">
            <Zap size={12} className="text-green-400" />
            {t('onlineTraders', language)}
          </div>
          <div className="flex-1 overflow-y-auto px-2 space-y-2">
            {traders?.filter(tr => tr.is_running).map(tr => (
              <div key={tr.trader_id}
                onClick={() => { setTraderId(tr.trader_id); if (decision && !decision.executed) setExecId(detail?.id || null) }}
                className={`p-2 rounded-lg cursor-pointer transition-all ${traderId === tr.trader_id ? 'bg-green-500/20 ring-1 ring-green-500' : 'bg-white/5 hover:bg-white/10'}`}>
                <div className="flex items-center gap-2">
                  <PunkAvatar seed={tr.trader_id} size={32} className="rounded-lg" />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-white font-medium truncate">{tr.trader_name}</div>
                    <div className="text-xs text-gray-500 truncate">{tr.ai_model}</div>
                  </div>
                  <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                </div>
              </div>
            ))}
            {traders?.filter(tr => !tr.is_running).slice(0, 3).map(tr => (
              <div key={tr.trader_id} className="p-2 rounded-lg bg-white/5 opacity-50">
                <div className="flex items-center gap-2">
                  <div className="grayscale">
                    <PunkAvatar seed={tr.trader_id} size={32} className="rounded-lg" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-white font-medium truncate">{tr.trader_name}</div>
                    <div className="text-xs text-gray-500">{t('offline', language)}</div>
                  </div>
                </div>
              </div>
            ))}
            {(!traders || traders.length === 0) && (
              <div className="text-xs text-gray-500 text-center py-4">{t('noTraders', language)}</div>
            )}
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {detail ? (
          <>
            {/* Header Bar - Compact */}
            <div className="px-2 sm:px-3 py-1.5 sm:py-2 border-b border-white/5 bg-[#0d1017]/50 flex items-center gap-1.5 sm:gap-3 flex-shrink-0 flex-wrap">
              <span className={`w-1.5 h-1.5 sm:w-2 sm:h-2 rounded-full flex-shrink-0 ${STATUS_COLOR[detail.status]}`} />
              <span className="font-bold text-white truncate text-sm sm:text-base">{detail.name}</span>
              <span className="text-yellow-400 font-semibold text-xs sm:text-sm">{detail.symbol}</span>
              {strategyName && <span className="text-[10px] sm:text-xs px-1 sm:px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded">{strategyName}</span>}
              <span className="text-[10px] sm:text-xs text-gray-500">R{detail.current_round}/{detail.max_rounds}</span>

              {/* Participants */}
              <div className="flex gap-0.5 sm:gap-1 ml-1 sm:ml-2">
                {participants.map(p => {
                  const vote = votes.find(v => v.ai_model_id === p.ai_model_id)
                  const act = vote ? (ACT[vote.action] || ACT.wait) : null
                  return (
                    <div key={p.id} className="flex items-center gap-1 px-1 py-0.5 rounded bg-white/5 text-xs">
                      <AIAvatar name={p.ai_model_name} size={14} />
                      {act && <span className={`${act.color}`}>{act.icon}</span>}
                    </div>
                  )
                })}
              </div>

              <div className="flex-1" />

              {/* Vote Summary */}
              {votes.length > 0 && (
                <div className="flex gap-1">
                  {Object.entries(voteSum).map(([action, count]) => {
                    const cfg = ACT[action] || ACT.wait
                    return (
                      <div key={action} className={`flex items-center gap-1 px-1.5 py-0.5 rounded ${cfg.bg} ${cfg.color} text-xs font-semibold`}>
                        {cfg.icon} {cfg.label}×{count}
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Main Content Area - Two Column Layout */}
            <div className="flex-1 flex flex-col lg:flex-row overflow-hidden">
              {Object.keys(rounds).length === 0 ? (
                <div className="flex-1 flex flex-col items-center justify-center text-gray-500">
                  <div className="text-4xl sm:text-6xl mb-2 sm:mb-4">{detail.status === 'pending' ? '🎯' : '⏳'}</div>
                  <div className="text-sm sm:text-lg px-4 text-center">{detail.status === 'pending' ? t('clickToStart', language) : t('waitingAI', language)}</div>
                </div>
              ) : (
                <>
                  {/* Left - Rounds */}
                  <div className="flex-1 overflow-y-auto p-2 sm:p-4 border-b lg:border-b-0 lg:border-r border-white/5">
                    <div className="text-xs sm:text-sm text-gray-400 font-semibold mb-2 sm:mb-3 flex items-center gap-1.5 sm:gap-2">
                      <span className="w-1.5 h-1.5 sm:w-2 sm:h-2 bg-blue-500 rounded-full"></span>
                      {t('discussionRecords', language)}
                    </div>
                    <div className="space-y-2 sm:space-y-3">
                      {Object.entries(rounds).map(([round, msgs]) => (
                        <div key={round} className="bg-white/5 rounded-xl p-2 sm:p-3">
                          <div className="text-[10px] sm:text-xs text-blue-400 font-bold mb-1.5 sm:mb-2">Round {round}</div>
                          <div className="space-y-1.5 sm:space-y-2">
                            {msgs.map(m => <MessageCard key={m.id} msg={m} />)}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Right - Votes */}
                  {votes.length > 0 && (
                    <div className="w-full lg:w-[420px] flex-shrink-0 overflow-y-auto p-2 sm:p-4 bg-[#0d1017]/50 border-t lg:border-t-0">
                      <div className="text-xs sm:text-sm text-gray-400 font-semibold mb-2 sm:mb-3 flex items-center gap-1.5 sm:gap-2">
                        <Trophy size={14} className="sm:w-4 sm:h-4 text-yellow-400" />
                        {t('finalVotes', language)}
                      </div>
                      <div className="space-y-2 sm:space-y-3">
                        {votes.map(v => (
                          <VoteCard key={v.id} vote={{
                            ai_model_name: v.ai_model_name,
                            action: v.action,
                            symbol: v.symbol,
                            confidence: v.confidence,
                            leverage: v.leverage,
                            position_pct: v.position_pct,
                            stop_loss_pct: v.stop_loss_pct,
                            take_profit_pct: v.take_profit_pct,
                            reasoning: v.reasoning
                          }} />
                        ))}
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>

            {/* Consensus Bar - Show when votes exist */}
            {(decision || votes.length > 0) && (
              <div className="p-2 sm:p-3 border-t border-white/5 bg-gradient-to-r from-yellow-500/10 to-orange-500/10 flex flex-col sm:flex-row items-start sm:items-center gap-2 sm:gap-4 flex-shrink-0">
                <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap">
                  <Trophy size={16} className="sm:w-5 sm:h-5 text-yellow-400" />
                  <span className="text-xs sm:text-sm text-gray-400">{t('consensus', language)}:</span>
                  {decision ? (
                    <>
                      {decision.symbol && <span className="text-yellow-400 font-bold mr-1 text-xs sm:text-sm">{decision.symbol}</span>}
                      <span className={`flex items-center gap-0.5 sm:gap-1 px-1.5 sm:px-2 py-0.5 sm:py-1 rounded font-bold text-[10px] sm:text-xs ${(ACT[decision.action] || ACT.wait).bg} ${(ACT[decision.action] || ACT.wait).color}`}>
                        {(ACT[decision.action] || ACT.wait).icon}
                        {decision.action.replace('_', ' ').toUpperCase()}
                      </span>
                    </>
                  ) : (
                    <span className="flex items-center gap-1 px-1.5 sm:px-2 py-0.5 sm:py-1 rounded font-bold bg-gray-500/20 text-gray-400 text-[10px] sm:text-xs">
                      <Clock size={12} className="sm:w-3.5 sm:h-3.5" /> VOTING...
                    </span>
                  )}
                </div>
                {decision && (
                  <div className="flex items-center gap-2 sm:gap-4 text-xs sm:text-sm flex-wrap">
                    <span><span className="text-gray-500">{t('confidence', language)}</span> <span className="text-yellow-400 font-bold">{decision.confidence || 0}%</span></span>
                    {(decision.leverage ?? 0) > 0 && <span><span className="text-gray-500">{t('leverage', language)}</span> <span className="text-white font-bold">{decision.leverage}x</span></span>}
                    {(decision.position_pct ?? 0) > 0 && <span><span className="text-gray-500">{t('position', language)}</span> <span className="text-white font-bold">{((decision.position_pct ?? 0) * 100).toFixed(0)}%</span></span>}
                    {(decision.stop_loss ?? 0) > 0 && <span><span className="text-gray-500">SL</span> <span className="text-red-400 font-bold">{((decision.stop_loss ?? 0) * 100).toFixed(1)}%</span></span>}
                    {(decision.take_profit ?? 0) > 0 && <span><span className="text-gray-500">TP</span> <span className="text-green-400 font-bold">{((decision.take_profit ?? 0) * 100).toFixed(1)}%</span></span>}
                  </div>
                )}
                <div className="flex-1" />
                {decision && !decision.executed && (decision.action === 'open_long' || decision.action === 'open_short') && (
                  <button onClick={() => setExecId(detail.id)}
                    className="px-2 sm:px-4 py-1 sm:py-1.5 rounded-lg bg-yellow-500 text-black font-semibold text-xs sm:text-sm flex items-center gap-1 w-full sm:w-auto">
                    <Zap size={12} className="sm:w-3.5 sm:h-3.5" /> {t('execute', language)}
                  </button>
                )}
                {decision?.executed && <span className="text-green-400 text-xs sm:text-sm font-semibold">✓ {t('executed', language)}</span>}
              </div>
            )}
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-gray-500">
            <div className="text-center">
              <div className="text-4xl mb-2">🗳️</div>
              <div>{t('selectOrCreate', language)}</div>
            </div>
          </div>
        )}
      </div>

      {/* Create Modal */}
      <CreateModal isOpen={showCreate} onClose={() => setShowCreate(false)} onCreate={onCreate}
        aiModels={aiModels || []} strategies={strategies || []} language={language} />

      {/* Execute Modal */}
      {execId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-3 sm:p-4">
          <div className="bg-[#1a1d24] rounded-xl w-full max-w-sm p-3 sm:p-4 border border-white/10 max-h-[90vh] overflow-y-auto">
            <h3 className="text-base sm:text-lg font-bold text-white mb-3 sm:mb-4 flex items-center gap-1.5 sm:gap-2">
              <Zap size={16} className="sm:w-5 sm:h-5 text-yellow-400" /> {t('executeTitle', language)}
            </h3>
            <select value={traderId} onChange={e => setTraderId(e.target.value)}
              className="w-full px-2.5 sm:px-3 py-1.5 sm:py-2 rounded-lg bg-white/5 border border-white/10 text-white text-xs sm:text-sm mb-2 sm:mb-3">
              <option value="">{t('selectTrader', language)}...</option>
              {traders?.filter(tr => tr.is_running).map(tr => (
                <option key={tr.trader_id} value={tr.trader_id}>✅ {tr.trader_name}</option>
              ))}
              {traders?.filter(tr => !tr.is_running).map(tr => (
                <option key={tr.trader_id} value={tr.trader_id} disabled>⏹ {tr.trader_name} ({t('offline', language)})</option>
              ))}
            </select>
            <div className="text-[10px] sm:text-xs text-yellow-300 bg-yellow-500/10 p-1.5 sm:p-2 rounded mb-2 sm:mb-3">
              ⚠️ {language === 'zh' ? '将使用账户余额执行真实交易' : 'Will execute real trade with account balance'}
            </div>
            <div className="flex flex-col sm:flex-row gap-2">
              <button onClick={() => { setExecId(null); setTraderId('') }}
                className="flex-1 py-1.5 sm:py-2 rounded-lg bg-white/5 text-white text-xs sm:text-sm">{t('cancel', language)}</button>
              <button onClick={onExecute} disabled={!traderId || executing || !traders?.find(tr => tr.trader_id === traderId)?.is_running}
                className="flex-1 py-1.5 sm:py-2 rounded-lg bg-yellow-500 text-black font-semibold text-xs sm:text-sm disabled:opacity-50">
                {executing ? <Loader2 size={14} className="sm:w-4 sm:h-4 animate-spin mx-auto" /> : t('execute', language)}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
