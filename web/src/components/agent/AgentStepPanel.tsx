import type { AgentStep } from '../../types/agent'
import { useLanguage } from '../../contexts/LanguageContext'

interface AgentStepPanelProps {
  steps?: AgentStep[]
  visible?: boolean
}

const statusStyles: Record<AgentStep['status'], { dot: string; text: string }> = {
  planning: { dot: '#7c3aed', text: '#c4b5fd' },
  pending: { dot: 'rgba(255,255,255,0.18)', text: '#818198' },
  running: { dot: '#F0B90B', text: '#f6d67a' },
  completed: { dot: '#00e5a0', text: '#9cf5d5' },
  replanned: { dot: '#38bdf8', text: '#9bdcf7' },
}

// Map raw backend tool names to friendly user-facing labels.
// Backend emits `step.label` like `tool:get_positions` and we render that as
// "📊 Checking your positions" instead of hiding it from the user.
const toolLabels: Record<string, { zh: string; en: string; id: string }> = {
  // Read-only state
  get_positions: { zh: '📊 检查持仓', en: '📊 Checking positions', id: '📊 Memeriksa posisi' },
  get_balance: { zh: '💰 查余额', en: '💰 Reading balance', id: '💰 Membaca saldo' },
  get_trade_history: { zh: '📜 查交易历史', en: '📜 Reading trade history', id: '📜 Membaca riwayat' },
  get_decisions: { zh: '🤖 查 AI 决策记录', en: '🤖 Reading AI decisions', id: '🤖 Membaca keputusan AI' },
  get_strategies: { zh: '📋 查策略列表', en: '📋 Listing strategies', id: '📋 Daftar strategi' },
  get_candidate_coins: { zh: '🎯 查标的池', en: '🎯 Reading candidate pool', id: '🎯 Kandidat' },
  get_exchange_configs: { zh: '🔌 查交易所配置', en: '🔌 Reading exchanges', id: '🔌 Bursa' },
  get_model_configs: { zh: '🧠 查 AI 模型', en: '🧠 Reading AI models', id: '🧠 Model AI' },
  get_preferences: { zh: '⚙️ 查偏好', en: '⚙️ Reading preferences', id: '⚙️ Preferensi' },
  get_backend_logs: { zh: '🪵 查后台日志', en: '🪵 Reading logs', id: '🪵 Membaca log' },
  get_watchlist: { zh: '👁 查关注列表', en: '👁 Reading watchlist', id: '👁 Membaca watchlist' },

  // Market data
  search_stock: { zh: '🔍 搜索股票', en: '🔍 Searching stocks', id: '🔍 Mencari saham' },
  get_market_price: { zh: '📈 查实时价格', en: '📈 Fetching price', id: '📈 Mengambil harga' },
  get_market_snapshot: { zh: '📈 查市场快照', en: '📈 Reading market snapshot', id: '📈 Snapshot pasar' },
  get_kline: { zh: '📈 查 K 线', en: '📈 Reading candlesticks', id: '📈 Membaca candlestick' },

  // Mutating
  manage_trader: { zh: '🤖 管理 Trader', en: '🤖 Managing trader', id: '🤖 Mengelola trader' },
  manage_strategy: { zh: '📋 管理策略', en: '📋 Managing strategy', id: '📋 Mengelola strategi' },
  manage_exchange_config: { zh: '🔌 管理交易所', en: '🔌 Managing exchange', id: '🔌 Mengelola bursa' },
  manage_model_config: { zh: '🧠 管理 AI 模型', en: '🧠 Managing AI model', id: '🧠 Mengelola model' },
  manage_preferences: { zh: '⚙️ 更新偏好', en: '⚙️ Updating preferences', id: '⚙️ Memperbarui preferensi' },
  manage_watchlist: { zh: '👁 更新关注列表', en: '👁 Updating watchlist', id: '👁 Memperbarui watchlist' },
  execute_trade: { zh: '⚡ 准备下单', en: '⚡ Preparing trade', id: '⚡ Menyiapkan order' },
}

function friendlyStepLabel(rawLabel: string, lang: 'zh' | 'en' | 'id'): string {
  const trimmed = rawLabel.trim()
  if (trimmed.toLowerCase().startsWith('tool:')) {
    const toolName = trimmed.slice(5).trim().toLowerCase()
    const entry = toolLabels[toolName]
    if (entry) return entry[lang]
    // Unknown tool — surface a generic but still informative label
    const generic = {
      zh: `🔧 调用 ${toolName}`,
      en: `🔧 Calling ${toolName}`,
      id: `🔧 Memanggil ${toolName}`,
    }
    return generic[lang]
  }
  return rawLabel
}

export function AgentStepPanel({ steps, visible }: AgentStepPanelProps) {
  const { language } = useLanguage()
  const lang = (language === 'zh' || language === 'id' ? language : 'en') as
    | 'zh'
    | 'en'
    | 'id'

  if (!visible || !steps || steps.length === 0) {
    return null
  }

  // Drop only the internal-routing chatter (central_brain); keep tool steps —
  // they are exactly what the user wants to see ("agent is actually doing something").
  const visibleSteps = steps.filter((step) => {
    const detail = (step.detail || '').trim().toLowerCase()
    return detail !== 'central_brain'
  })

  if (visibleSteps.length === 0) {
    return null
  }

  const liveRunHeading = lang === 'zh' ? 'AGENT 实时动作' : lang === 'id' ? 'AKSI AGENT' : 'LIVE RUN'

  return (
    <div
      style={{
        marginBottom: 12,
        padding: '10px 12px',
        borderRadius: 12,
        background: 'linear-gradient(180deg, rgba(255,255,255,0.03), rgba(255,255,255,0.015))',
        border: '1px solid rgba(255,255,255,0.06)',
      }}
    >
      <div
        style={{
          fontSize: 11,
          fontWeight: 700,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          color: '#7b7b91',
          marginBottom: 10,
        }}
      >
        {liveRunHeading}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {visibleSteps.map((step) => {
          const style = statusStyles[step.status]
          const label = friendlyStepLabel(step.label, lang)
          return (
            <div
              key={step.id}
              style={{
                display: 'grid',
                gridTemplateColumns: '14px 1fr',
                gap: 8,
                alignItems: 'start',
              }}
            >
              <span
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 999,
                  marginTop: 5,
                  background: style.dot,
                  boxShadow:
                    step.status === 'running'
                      ? '0 0 0 4px rgba(240,185,11,0.08)'
                      : 'none',
                }}
              />
              <div>
                <div
                  style={{
                    fontSize: 12.5,
                    lineHeight: 1.5,
                    color: style.text,
                    fontWeight: step.status === 'running' ? 600 : 500,
                  }}
                >
                  {label}
                </div>
                {step.detail && step.detail.trim().toLowerCase() !== 'central_brain' && (
                  <div
                    style={{
                      fontSize: 11.5,
                      lineHeight: 1.45,
                      color: '#6e6e86',
                      marginTop: 2,
                    }}
                  >
                    {step.detail}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
