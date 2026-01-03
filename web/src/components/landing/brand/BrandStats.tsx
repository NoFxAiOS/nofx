import { motion } from 'framer-motion'

const stats = [
    { label: "TRADING VOL", value: "$4.2B+" },
    { label: "AI AGENTS", value: "850+" },
    { label: "STRATEGIES", value: "Infinite" },
    { label: "UPTIME", value: "99.9%" },
]

export default function BrandStats() {
    return (
        <section className="bg-nofx-accent py-20 relative overflow-hidden">
            {/* Halftone Pattern */}
            <div
                className="absolute inset-0 opacity-10 pointer-events-none"
                style={{
                    backgroundImage: 'radial-gradient(circle, #000 2px, transparent 2.5px)',
                    backgroundSize: '20px 20px'
                }}
            />

            <div className="max-w-[1920px] mx-auto px-4 lg:px-16 relative z-10">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-12 md:text-left">
                    {stats.map((stat, i) => (
                        <motion.div
                            key={i}
                            initial={{ opacity: 0 }}
                            whileInView={{ opacity: 1 }}
                            transition={{ delay: i * 0.1 }}
                            className="relative overflow-hidden group bg-black/40 backdrop-blur-md border border-white/10 p-6 rounded-lg md:bg-transparent md:border-0 md:p-0 md:backdrop-blur-none"
                        >
                            {/* Mobile Neon Corners */}
                            <div className="absolute top-0 right-0 w-3 h-3 border-t-2 border-r-2 border-nofx-gold md:hidden opacity-80 shadow-[0_0_10px_rgba(234,179,8,0.5)]"></div>
                            <div className="absolute bottom-0 left-0 w-3 h-3 border-b-2 border-l-2 border-nofx-gold md:hidden opacity-80 shadow-[0_0_10px_rgba(234,179,8,0.5)]"></div>

                            {/* Mobile Inner Glow */}
                            <div className="absolute inset-0 bg-nofx-gold/5 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none md:hidden"></div>

                            <div className="text-3xl md:text-6xl font-black text-white tracking-tighter mb-2 group-hover:scale-110 transition-transform duration-300 origin-left relative z-10">
                                {stat.value}
                            </div>
                            <div className="text-[10px] md:text-base font-bold text-zinc-400 md:text-black/60 uppercase tracking-widest bg-white/5 md:bg-white/20 inline-block px-2 py-1 rounded relative z-10">
                                {stat.label}
                            </div>
                        </motion.div>
                    ))}
                </div>
            </div>
        </section>
    )
}
