import { motion } from 'framer-motion'
import { Brain, Swords, BarChart3, Shield, Blocks, LineChart } from 'lucide-react'
import { t, Language } from '../../i18n/translations'

interface FeaturesSectionProps {
  language: Language
}

export default function FeaturesSection({ language }: FeaturesSectionProps) {
  const features = [
    {
      icon: Brain,
      title: language === 'zh' ? 'AI Strategy Orchestration' : 'AI Strategy Orchestration',
      desc: language === 'zh'
        ? 'Support DeepSeek, GPT, Claude, Qwen and more. Custom prompts, AI autonomously analyzes markets and makes trading decisions'
        : 'Support DeepSeek, GPT, Claude, Qwen and more. Custom prompts, AI autonomously analyzes markets and makes trading decisions',
      highlight: true,
      badge: language === 'zh' ? 'Core' : 'Core',
    },
    {
      icon: Swords,
      title: language === 'zh' ? 'Multi-AI Arena' : 'Multi-AI Arena',
      desc: language === 'zh'
        ? 'Multiple AI traders compete in real-time, live PnL leaderboard, automatic survival of the fittest'
        : 'Multiple AI traders compete in real-time, live PnL leaderboard, automatic survival of the fittest',
      highlight: true,
      badge: language === 'zh' ? 'Unique' : 'Unique',
    },
    {
      icon: LineChart,
      title: language === 'zh' ? 'Pro Quant Data' : 'Pro Quant Data',
      desc: language === 'zh'
        ? 'Integrated candlesticks, indicators, order book, funding rates, open interest - comprehensive data for AI decisions'
        : 'Integrated candlesticks, indicators, order book, funding rates, open interest - comprehensive data for AI decisions',
      highlight: true,
      badge: language === 'zh' ? 'Pro' : 'Pro',
    },
    {
      icon: Blocks,
      title: language === 'zh' ? 'Multi-Exchange Support' : 'Multi-Exchange Support',
      desc: language === 'zh'
        ? 'Binance, OKX, Bybit, Hyperliquid, Aster DEX - one system, multiple exchanges'
        : 'Binance, OKX, Bybit, Hyperliquid, Aster DEX - one system, multiple exchanges',
    },
    {
      icon: BarChart3,
      title: language === 'zh' ? 'Real-time Dashboard' : 'Real-time Dashboard',
      desc: language === 'zh'
        ? 'Trade monitoring, PnL curves, position analysis, AI decision logs at a glance'
        : 'Trade monitoring, PnL curves, position analysis, AI decision logs at a glance',
    },
    {
      icon: Shield,
      title: language === 'zh' ? 'Open Source & Self-Hosted' : 'Open Source & Self-Hosted',
      desc: language === 'zh'
        ? 'Fully open source, data stored locally, API keys never leave your server'
        : 'Fully open source, data stored locally, API keys never leave your server',
    },
  ]

  return (
    <section className="py-24 relative" style={{ background: '#F1ECE2' }}>
      {/* Background */}
      <div
        className="absolute inset-0 opacity-[0.04]"
        style={{
          backgroundImage: `linear-gradient(#E0483B 1px, transparent 1px), linear-gradient(90deg, #E0483B 1px, transparent 1px)`,
          backgroundSize: '40px 40px',
        }}
      />

      <div className="max-w-6xl mx-auto px-4 relative z-10">
        {/* Header */}
        <motion.div
          className="text-center mb-16"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="text-4xl lg:text-5xl font-bold mb-4" style={{ color: '#1A1813' }}>
            {t('whyChooseNofx', language)}
          </h2>
          <p className="text-lg max-w-2xl mx-auto" style={{ color: '#8A8478' }}>
            {language === 'zh'
              ? 'Not just a trading bot, but a complete AI trading operating system'
              : 'Not just a trading bot, but a complete AI trading operating system'}
          </p>
        </motion.div>

        {/* Features Grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
          {features.map((feature, index) => (
            <motion.div
              key={feature.title}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: index * 0.1 }}
              className={`
                relative group rounded-2xl p-6 transition-all duration-300
                ${feature.highlight ? 'md:col-span-1 lg:col-span-1' : ''}
              `}
              style={{
                background: feature.highlight
                  ? 'rgba(224, 72, 59, 0.06)'
                  : '#F7F4EC',
                border: feature.highlight
                  ? '1px solid rgba(224, 72, 59, 0.2)'
                  : '1px solid rgba(26, 24, 19, 0.14)',
              }}
            >
              {/* Badge */}
              {feature.badge && (
                <div
                  className="absolute top-4 right-4 px-2 py-1 rounded text-xs font-medium"
                  style={{
                    background: 'rgba(224, 72, 59, 0.15)',
                    color: '#E0483B',
                  }}
                >
                  {feature.badge}
                </div>
              )}

              {/* Icon */}
              <motion.div
                className="w-12 h-12 rounded-xl flex items-center justify-center mb-4"
                style={{
                  background: feature.highlight
                    ? 'rgba(224, 72, 59, 0.15)'
                    : 'rgba(224, 72, 59, 0.1)',
                  border: '1px solid rgba(224, 72, 59, 0.2)',
                }}
                whileHover={{ scale: 1.1, rotate: 5 }}
              >
                <feature.icon
                  className="w-6 h-6"
                  style={{ color: '#E0483B' }}
                />
              </motion.div>

              {/* Text */}
              <h3
                className="text-xl font-bold mb-3"
                style={{ color: '#1A1813' }}
              >
                {feature.title}
              </h3>
              <p
                className="text-sm leading-relaxed"
                style={{ color: '#8A8478' }}
              >
                {feature.desc}
              </p>

              {/* Hover Glow */}
              <div
                className="absolute -bottom-10 -right-10 w-32 h-32 rounded-full blur-3xl opacity-0 group-hover:opacity-20 transition-opacity duration-500"
                style={{ background: '#E0483B' }}
              />
            </motion.div>
          ))}
        </div>

        {/* Bottom Stats */}
        <motion.div
          className="mt-16 grid grid-cols-2 md:grid-cols-4 gap-6"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          {[
            { value: '10+', label: language === 'zh' ? 'AI Models' : 'AI Models' },
            { value: '5+', label: language === 'zh' ? 'Exchanges' : 'Exchanges' },
            { value: '24/7', label: language === 'zh' ? 'Auto Trading' : 'Auto Trading' },
            { value: '100%', label: language === 'zh' ? 'Open Source' : 'Open Source' },
          ].map((stat) => (
            <div
              key={stat.label}
              className="text-center p-4 rounded-xl"
              style={{
                background: '#F7F4EC',
                border: '1px solid rgba(26, 24, 19, 0.14)',
              }}
            >
              <div
                className="text-2xl font-bold mb-1"
                style={{
                  color: '#E0483B',
                }}
              >
                {stat.value}
              </div>
              <div className="text-xs" style={{ color: '#8A8478' }}>
                {stat.label}
              </div>
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  )
}
