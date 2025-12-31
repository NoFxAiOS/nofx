import { motion } from 'framer-motion'
import { ArrowRight } from 'lucide-react'
import { OFFICIAL_LINKS } from '../../../constants/branding'

const features = [
  { label: 'Crypto Trading', desc: 'BTC, ETH, Altcoins', active: true },
  { label: 'US Stocks', desc: 'AAPL, TSLA, NVDA', active: true },
  { label: 'Forex Trading', desc: 'EUR/USD, GBP/USD', active: true },
  { label: 'Metals Trading', desc: 'Gold, Silver', active: true },
  { label: 'Strategy Studio', desc: 'Visual strategy builder', active: true },
  { label: 'Backtest Lab', desc: 'Historical simulation', active: true },
]

export default function WelcomeSection() {
  return (
    <section
      id="welcome-section"
      className="w-full min-h-screen bg-zinc-100 flex items-center"
      style={{ scrollSnapAlign: 'start' }}
    >
      <div className="max-w-7xl mx-auto px-6 py-20 w-full">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-16 items-center">

          {/* Left Column - Introduction */}
          <motion.div
            initial={{ opacity: 0, x: -50 }}
            whileInView={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.6, ease: 'easeOut' }}
            viewport={{ once: true, amount: 0.3 }}
          >
            <h2 className="text-4xl md:text-5xl font-black leading-tight mb-6">
              <span className="text-zinc-900">Welcome to</span>
              <br />
              <span className="text-nofx-gold">NOFX.</span>
            </h2>

            <p className="text-zinc-600 text-lg leading-relaxed mb-8 max-w-md">
              The next evolution of software — building systems that don't just execute, they co-create.
              Deploy intelligent agents across multiple markets.
            </p>

            <a
              href={OFFICIAL_LINKS.telegram}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 border-2 border-zinc-900 text-zinc-900 font-bold rounded-sm hover:bg-zinc-900 hover:text-white transition-all group"
            >
              <span>Join Community</span>
              <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
            </a>
          </motion.div>

          {/* Right Column - Feature Tags */}
          <div className="flex flex-col gap-2">
            {features.map((feature, index) => (
              <motion.div
                key={index}
                initial={{ opacity: 0, x: 50 }}
                whileInView={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.4, delay: index * 0.1, ease: 'easeOut' }}
                viewport={{ once: true, amount: 0.3 }}
                className="flex items-center justify-between py-4 px-6 border-b border-zinc-300 group cursor-default transition-all hover:bg-zinc-200 border-l-4 border-l-nofx-gold bg-white"
              >
                <div>
                  <span className="text-lg font-medium text-zinc-900 block">
                    {feature.label}
                  </span>
                  <span className="text-sm text-zinc-500">
                    {feature.desc}
                  </span>
                </div>
                <span className="text-xs font-bold text-nofx-gold bg-nofx-gold/10 px-2 py-1 rounded">
                  ✓ Supported
                </span>
              </motion.div>
            ))}
          </div>

        </div>
      </div>
    </section>
  )
}
