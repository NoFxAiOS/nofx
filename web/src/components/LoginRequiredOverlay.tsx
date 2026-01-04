import { motion, AnimatePresence } from 'framer-motion'
import { LogIn, UserPlus, X, AlertTriangle, Terminal } from 'lucide-react'
import { DeepVoidBackground } from './DeepVoidBackground'
import { useLanguage } from '../contexts/LanguageContext'

interface LoginRequiredOverlayProps {
  isOpen: boolean
  onClose: () => void
  featureName?: string
}

export function LoginRequiredOverlay({ isOpen, onClose, featureName }: LoginRequiredOverlayProps) {
  const { language } = useLanguage()

  const texts = {
    zh: {
      title: '系统访问受限',
      subtitle: featureName ? `访问「${featureName}」需要更高权限` : '此模块需要授权访问',
      description: '初始化身份验证协议以解锁完整系统功能：AI 交易员配置、策略市场数据流、回测模拟核心。',
      benefits: [
        'AI 交易员控制权',
        '高频策略核心市场',
        '历史数据回测引擎',
        '全系统数据可视化'
      ],
      login: '执行登录指令',
      register: '注册新用户 ID',
      later: '中止操作'
    },
    en: {
      title: 'SYSTEM ACCESS DENIED',
      subtitle: featureName ? `Module "${featureName}" requires elevated privileges` : 'Authorization required for this module',
      description: 'Initialize authentication protocol to unlock full system capabilities: AI Trader configuration, Strategy Market data streams, and Backtest Simulation core.',
      benefits: [
        'AI Trader Control',
        'HFT Strategy Market',
        'Historical Backtest Engine',
        'Full System Visualization'
      ],
      login: 'EXECUTE LOGIN',
      register: 'REGISTER NEW ID',
      later: 'ABORT'
    }
  }

  const t = texts[language]

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50"
        >
          <DeepVoidBackground
            className="w-full h-full bg-nofx-bg/95 backdrop-blur-md flex items-center justify-center p-4 text-nofx-text"
            disableAnimation
            onClick={onClose}
          >

            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              transition={{ type: 'spring', damping: 20, stiffness: 300 }}
              className="relative max-w-md w-full overflow-hidden bg-nofx-bg border border-nofx-gold/30 shadow-neon rounded-sm group font-mono"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Terminal Window Header */}
              <div className="flex items-center justify-between px-3 py-2 bg-nofx-bg-lighter border-b border-nofx-gold/20">
                <div className="flex items-center gap-2">
                  <Terminal size={12} className="text-nofx-gold" />
                  <span className="text-[10px] text-nofx-text-muted uppercase tracking-wider">auth_protocol.exe</span>
                </div>
                <button
                  onClick={onClose}
                  className="text-nofx-text-muted hover:text-nofx-danger transition-colors"
                >
                  <X size={14} />
                </button>
              </div>

              {/* Main Content */}
              <div className="p-8 relative">
                {/* Background Grid */}
                <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808008_1px,transparent_1px),linear-gradient(to_bottom,#80808008_1px,transparent_1px)] bg-[size:14px_14px] pointer-events-none"></div>

                <div className="relative z-10">
                  {/* Flashing Access Denied */}
                  <div className="flex justify-center mb-6">
                    <div className="relative">
                      <div className="absolute inset-0 bg-red-500/20 blur-xl animate-pulse"></div>
                      <div className="bg-nofx-bg border border-red-500/50 text-red-500 px-4 py-2 flex items-center gap-3 shadow-[0_0_15px_rgba(239,68,68,0.2)]">
                        <AlertTriangle size={18} className="animate-pulse" />
                        <span className="font-bold tracking-widest text-sm uppercase">{language === 'zh' ? '访问被拒绝' : 'ACCESS DENIED'}</span>
                      </div>
                    </div>
                  </div>

                  {/* Terminal Text */}
                  <div className="space-y-4 mb-8">
                    <div className="text-center">
                      <h2 className="text-xl font-bold text-white uppercase tracking-wider mb-2">{t.title}</h2>
                      <p className="text-nofx-gold text-xs uppercase tracking-widest border-b border-nofx-gold/20 pb-4 inline-block">{t.subtitle}</p>
                    </div>

                    <div className="bg-nofx-bg-lighter border-l-2 border-nofx-gold/20 p-3 my-4">
                      <p className="text-xs text-nofx-text-muted leading-relaxed font-mono">
                        <span className="text-green-500 mr-2">$</span>
                        {t.description}
                      </p>
                    </div>

                    <div className="grid grid-cols-2 gap-2">
                      {t.benefits.map((benefit, i) => (
                        <div key={i} className="flex items-center gap-2 text-[10px] text-nofx-text-muted uppercase tracking-wide">
                          <span className="text-nofx-gold">✓</span> {benefit}
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Action Buttons */}
                  <div className="space-y-3">
                    <a
                      href="/login"
                      className="flex items-center justify-center gap-2 w-full py-3 bg-nofx-gold text-black font-bold text-xs uppercase tracking-widest hover:bg-yellow-400 transition-all shadow-neon hover:shadow-[0_0_25px_rgba(240,185,11,0.4)] group"
                    >
                      <LogIn size={14} />
                      <span>{t.login}</span>
                      <span className="opacity-0 group-hover:opacity-100 transition-opacity -ml-2 group-hover:ml-0">-&gt;</span>
                    </a>

                    <a
                      href="/register"
                      className="flex items-center justify-center gap-2 w-full py-3 bg-transparent border border-nofx-gold/20 text-nofx-text-muted hover:text-white hover:border-nofx-gold font-bold text-xs uppercase tracking-widest transition-all hover:bg-nofx-gold/10"
                    >
                      <UserPlus size={14} />
                      <span>{t.register}</span>
                    </a>
                  </div>

                  <div className="mt-4 text-center">
                    <button
                      onClick={onClose}
                      className="text-[10px] text-nofx-text-muted hover:text-nofx-danger uppercase tracking-widest hover:underline decoration-red-500/30"
                    >
                      [ {t.later} ]
                    </button>
                  </div>

                </div>
              </div>

              {/* Corner Accents */}
              <div className="absolute top-0 right-0 w-2 h-2 border-t border-r border-nofx-gold"></div>
              <div className="absolute bottom-0 left-0 w-2 h-2 border-b border-l border-nofx-gold"></div>

            </motion.div>
          </DeepVoidBackground>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
