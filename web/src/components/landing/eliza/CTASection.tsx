import { motion } from 'framer-motion'
import { ArrowRight } from 'lucide-react'
import { OFFICIAL_LINKS } from '../../../constants/branding'

export default function CTASection() {
  return (
    <section
      className="relative w-full min-h-screen overflow-hidden flex items-center bg-nofx-bg"
      style={{ scrollSnapAlign: 'start' }}
    >
      {/* Background gradient */}
      <div className="absolute inset-0 bg-gradient-to-b from-zinc-900 via-nofx-bg to-black pointer-events-none" />

      {/* Grid pattern */}
      <div
        className="absolute inset-0 opacity-[0.03] pointer-events-none"
        style={{
          backgroundImage: `linear-gradient(rgba(240,185,11,0.5) 1px, transparent 1px),
                           linear-gradient(90deg, rgba(240,185,11,0.5) 1px, transparent 1px)`,
          backgroundSize: '80px 80px'
        }}
      />

      {/* Large glow */}
      <motion.div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] bg-nofx-gold/10 rounded-full blur-[200px] pointer-events-none"
        initial={{ opacity: 0, scale: 0.5 }}
        whileInView={{ opacity: 1, scale: 1 }}
        transition={{ duration: 1.2, ease: 'easeOut' }}
        viewport={{ once: true }}
      />

      {/* Mascot silhouette */}
      <motion.div
        className="absolute bottom-0 left-1/2 -translate-x-1/2 w-[350px] md:w-[500px] opacity-30 pointer-events-none"
        initial={{ opacity: 0, y: 100 }}
        whileInView={{ opacity: 0.3, y: 0 }}
        transition={{ duration: 0.8, delay: 0.3 }}
        viewport={{ once: true }}
      >
        <img
          src="/images/nofx_mascot.png"
          alt=""
          className="w-full h-auto object-contain"
          style={{
            maskImage: 'linear-gradient(to top, black 20%, transparent 80%)',
            filter: 'grayscale(100%)'
          }}
        />
      </motion.div>

      <div className="relative z-10 max-w-4xl mx-auto px-6 text-center">
        <motion.h2
          className="text-5xl md:text-6xl lg:text-7xl font-black text-white mb-8 leading-tight"
          initial={{ opacity: 0, y: 40 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: 'easeOut' }}
          viewport={{ once: true, amount: 0.3 }}
        >
          Build the <span className="text-nofx-gold">Future</span>
        </motion.h2>

        <motion.p
          className="text-zinc-400 text-xl mb-12 max-w-2xl mx-auto"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
          viewport={{ once: true, amount: 0.3 }}
        >
          Join the next generation of AI-powered trading. Deploy your first agent in minutes.
        </motion.p>

        <motion.div
          className="flex flex-col sm:flex-row items-center justify-center gap-4"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.4 }}
          viewport={{ once: true, amount: 0.3 }}
        >
          <a
            href={OFFICIAL_LINKS.github}
            target="_blank"
            rel="noreferrer"
            className="group px-10 py-5 bg-nofx-gold text-black font-bold text-lg rounded-sm hover:bg-white transition-all flex items-center gap-3"
          >
            <span>Start Building</span>
            <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
          </a>

          <a
            href={OFFICIAL_LINKS.telegram}
            target="_blank"
            rel="noreferrer"
            className="px-10 py-5 border-2 border-zinc-700 text-zinc-300 font-bold text-lg rounded-sm hover:border-nofx-gold hover:text-nofx-gold transition-all"
          >
            Contact Us
          </a>
        </motion.div>
      </div>
    </section>
  )
}
