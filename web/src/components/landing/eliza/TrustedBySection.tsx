import { motion } from 'framer-motion'

const exchanges = [
  { name: 'Binance', type: 'CEX' },
  { name: 'Bybit', type: 'CEX' },
  { name: 'OKX', type: 'CEX' },
  { name: 'Bitget', type: 'CEX' },
  { name: 'Hyperliquid', type: 'DEX' },
  { name: 'Aster DEX', type: 'DEX' },
  { name: 'Lighter', type: 'DEX' },
]

const aiModels = [
  'DeepSeek',
  'Qwen',
  'GPT',
  'Claude',
  'Gemini',
  'Grok',
  'Kimi',
]

export default function TrustedBySection() {
  return (
    <section
      className="w-full min-h-screen bg-zinc-900 flex items-center"
      style={{ scrollSnapAlign: 'start' }}
    >
      <div className="max-w-7xl mx-auto px-6 py-20 w-full">

        {/* Main content centered */}
        <motion.div
          className="flex flex-col items-center text-center mb-16"
          initial={{ opacity: 0, y: 40 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: 'easeOut' }}
          viewport={{ once: true, amount: 0.3 }}
        >
          <p className="text-zinc-500 text-sm font-medium uppercase tracking-wider mb-8">Supported Exchanges</p>

          {/* Featured - Multi Exchange */}
          <div className="mb-12">
            <motion.div
              className="text-4xl md:text-5xl font-black text-white mb-4"
              initial={{ opacity: 0, scale: 0.9 }}
              whileInView={{ opacity: 1, scale: 1 }}
              transition={{ duration: 0.5, delay: 0.2 }}
              viewport={{ once: true }}
            >
              7 Exchanges
            </motion.div>
            <p className="text-zinc-400 max-w-md">
              Trade on CEX and Perp-DEX from one platform. Binance, Bybit, OKX, Bitget, Hyperliquid, Aster DEX, Lighter.
            </p>
          </div>
        </motion.div>

        {/* Exchanges Grid */}
        <div className="flex flex-wrap items-center justify-center gap-6 md:gap-12 mb-16">
          {exchanges.map((exchange, index) => (
            <motion.div
              key={index}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 0.6, y: 0 }}
              whileHover={{ opacity: 1 }}
              transition={{ duration: 0.4, delay: index * 0.08 }}
              viewport={{ once: true, amount: 0.3 }}
              className="flex flex-col items-center cursor-pointer"
            >
              <span className="text-zinc-300 font-bold text-lg md:text-xl">{exchange.name}</span>
              <span className="text-zinc-600 text-xs uppercase tracking-wider">{exchange.type}</span>
            </motion.div>
          ))}
        </div>

        {/* AI Models Section */}
        <motion.div
          className="text-center"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.3 }}
          viewport={{ once: true, amount: 0.3 }}
        >
          <p className="text-zinc-500 text-sm font-medium uppercase tracking-wider mb-6">Powered by AI</p>
          <div className="flex flex-wrap items-center justify-center gap-4 md:gap-8">
            {aiModels.map((model, index) => (
              <motion.span
                key={index}
                initial={{ opacity: 0 }}
                whileInView={{ opacity: 0.5 }}
                whileHover={{ opacity: 1 }}
                transition={{ duration: 0.3, delay: index * 0.05 }}
                viewport={{ once: true }}
                className="text-zinc-400 font-medium text-sm md:text-base cursor-pointer"
              >
                {model}
              </motion.span>
            ))}
          </div>
        </motion.div>

      </div>
    </section>
  )
}
