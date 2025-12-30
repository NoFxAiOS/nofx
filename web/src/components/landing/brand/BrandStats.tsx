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

            <div className="max-w-[1920px] mx-auto px-6 lg:px-16 relative z-10">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-12 text-center md:text-left">
                    {stats.map((stat, i) => (
                        <motion.div
                            key={i}
                            initial={{ opacity: 0 }}
                            whileInView={{ opacity: 1 }}
                            transition={{ delay: i * 0.1 }}
                        >
                            <div className="text-5xl md:text-6xl font-black text-white tracking-tighter mb-2">
                                {stat.value}
                            </div>
                            <div className="text-sm md:text-base font-bold text-black/60 uppercase tracking-widest bg-white/20 inline-block px-2 py-1">
                                {stat.label}
                            </div>
                        </motion.div>
                    ))}
                </div>
            </div>
        </section>
    )
}
