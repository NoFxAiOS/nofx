import { motion } from 'framer-motion'
import { Brain, BarChart3, Swords, FlaskConical } from 'lucide-react'
import { OFFICIAL_LINKS } from '../../../constants/branding'

const architectureFeatures = [
  {
    icon: Brain,
    title: 'Multi-AI Support',
    description: 'Run DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi - switch models anytime. Each AI makes independent trading decisions.'
  },
  {
    icon: BarChart3,
    title: 'Strategy Studio',
    description: 'Visual strategy builder with coin sources, technical indicators (EMA, MACD, RSI, ATR), and risk controls. No JSON editing required.'
  },
  {
    icon: Swords,
    title: 'AI Debate Arena',
    description: 'Multiple AI models debate trading decisions with different roles (Bull, Bear, Analyst). Consensus voting drives execution.'
  },
  {
    icon: FlaskConical,
    title: 'Backtest Lab',
    description: 'Historical simulation with equity curves, trade markers, and performance metrics. Test strategies before deploying real capital.'
  }
]

export default function ArchitectureSection() {
  return (
    <section
      className="w-full min-h-screen bg-white flex items-center"
      style={{ scrollSnapAlign: 'start' }}
    >
      <div className="max-w-7xl mx-auto px-6 py-20 w-full">

        {/* Section Header */}
        <motion.div
          className="mb-16"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: 'easeOut' }}
          viewport={{ once: true, amount: 0.3 }}
        >
          <p className="text-zinc-400 text-sm font-medium uppercase tracking-wider mb-3">Architecture</p>
          <div className="flex flex-col md:flex-row md:items-end md:justify-between gap-6">
            <div>
              <h2 className="text-3xl md:text-4xl font-black text-zinc-900 mb-4">
                Universally adaptable,
                <br />
                <span className="text-nofx-gold">completely yours.</span>
              </h2>
            </div>
            <a
              href={OFFICIAL_LINKS.github}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-zinc-900 text-white font-bold text-sm rounded-sm hover:bg-nofx-gold hover:text-black transition-colors"
            >
              Start Building
            </a>
          </div>
        </motion.div>

        {/* Feature Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {architectureFeatures.map((feature, index) => {
            const Icon = feature.icon
            return (
              <motion.div
                key={index}
                initial={{ opacity: 0, y: 40, scale: 0.95 }}
                whileInView={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ duration: 0.5, delay: index * 0.1, ease: 'easeOut' }}
                viewport={{ once: true, amount: 0.2 }}
                className="group p-8 bg-zinc-50 border border-zinc-200 rounded-sm hover:border-zinc-300 hover:shadow-lg transition-all"
              >
                <div className="flex items-start gap-4">
                  <div className="p-3 bg-nofx-gold/10 rounded-sm group-hover:bg-nofx-gold/20 transition-colors">
                    <Icon className="w-6 h-6 text-nofx-gold" />
                  </div>
                  <div className="flex-1">
                    <h3 className="text-lg font-bold text-zinc-900 mb-2">{feature.title}</h3>
                    <p className="text-zinc-500 text-sm leading-relaxed">{feature.description}</p>
                  </div>
                </div>
              </motion.div>
            )
          })}
        </div>

      </div>
    </section>
  )
}
