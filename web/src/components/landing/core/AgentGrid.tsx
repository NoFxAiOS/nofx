import { motion } from 'framer-motion'
import { TrendingUp, Layers, Zap, Hexagon, Crosshair } from 'lucide-react'
import { useAuth } from '../../../contexts/AuthContext'

const agents = [
    {
        name: "ALPHA-1",
        // ... (rest of agents array remains, but I can't skip lines in replacement content easily without context. Wait, let's just replace the top section)
        // Actually, I'll use multi_replace for targeted cleanup.
        class: "SCALPER",
        desc: "High-frequency microstructure exploitation.",
        apy: "142%",
        winRate: "68%",
        risk: "HIGH",
        color: "text-nofx-gold",
        border: "border-nofx-gold/50",
        bg_glow: "shadow-[0_0_30px_rgba(240,185,11,0.1)]",
        icon: Zap
    },
    {
        name: "BETA-X",
        class: "SWING_OPS",
        desc: "Multi-day trend extraction engine.",
        apy: "89%",
        winRate: "55%",
        risk: "MED",
        color: "text-blue-400",
        border: "border-blue-400/30",
        bg_glow: "shadow-[0_0_30px_rgba(96,165,250,0.1)]",
        icon: TrendingUp
    },
    {
        name: "GAMMA-RAY",
        class: "ARBITRAGE",
        desc: "Low-risk spatial price equalization.",
        apy: "24%",
        winRate: "99%",
        risk: "LOW",
        color: "text-purple-400",
        border: "border-purple-400/30",
        bg_glow: "shadow-[0_0_30px_rgba(192,132,252,0.1)]",
        icon: Layers
    },
]

export default function AgentGrid() {
    const { user } = useAuth()

    const handleInitialize = () => {
        if (user) {
            window.location.href = '/strategy-market'
        } else {
            window.location.href = '/login'
        }
    }

    return (
        <section id="market-scanner" className="py-16 md:py-24 bg-nofx-bg relative overflow-hidden">

            {/* Background Details */}
            <div className="absolute top-0 right-0 p-10 opacity-20 pointer-events-none">
                <Hexagon className="w-64 h-64 text-zinc-800" strokeWidth={0.5} />
            </div>

            <div className="max-w-7xl mx-auto px-6 relative z-10">

                <div className="flex flex-col md:flex-row justify-between items-end mb-10 md:mb-16 gap-6">
                    <div>
                        <div className="flex items-center gap-2 text-nofx-gold font-mono text-xs mb-2 tracking-widest uppercase">
                            <Crosshair className="w-4 h-4" /> MARKET SELECT
                        </div>
                        <h2 className="text-4xl md:text-5xl font-black text-white uppercase tracking-tighter">
                            STRATEGY <span className="text-transparent bg-clip-text bg-gradient-to-r from-nofx-gold to-white">UNITS</span>
                        </h2>
                    </div>
                    <div className="font-mono text-right text-xs text-zinc-500 max-w-xs">
                        SELECT AN AUTONOMOUS AGENT TO BEGIN DEPLOYMENT. UNITS ARE PRE-TRAINED ON HISTORICAL TICKS.
                    </div>
                </div>

                {/* Grid Container - Removing scroll tracking for stability test */}
                <div className="flex flex-row md:grid md:grid-cols-3 gap-4 md:gap-8 overflow-x-auto md:overflow-visible pb-12 md:pb-0 snap-x snap-mandatory -mx-6 px-6 md:mx-0 md:px-0 scrollbar-hide">
                    {agents.map((agent, i) => {
                        const Icon = agent.icon

                        return (
                            <motion.div
                                key={i}
                                initial={{ opacity: 0, y: 20 }}
                                whileInView={{ opacity: 1, y: 0 }}
                                transition={{ delay: i * 0.1 }}
                                className={`group relative bg-black/40 backdrop-blur-xl border ${agent.border} overflow-hidden transition-all duration-300 min-w-[85vw] md:min-w-0 snap-center shrink-0 rounded-xl md:rounded-none`}
                            >
                                {/* Top "Hinge" decoration */}
                                <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-white/10 to-transparent"></div>

                                <div className="p-8 relative z-10">
                                    {/* Header */}
                                    <div className="flex justify-between items-start mb-6">
                                        <div className="p-3 bg-zinc-900/80 rounded border border-zinc-700">
                                            <Icon className={`w-8 h-8 ${agent.color}`} />
                                        </div>
                                        <div className="text-right">
                                            <div className="text-[10px] font-mono text-zinc-500 uppercase">Class</div>
                                            <div className={`font-bold font-mono tracking-wider ${agent.color}`}>{agent.class}</div>
                                        </div>
                                    </div>

                                    {/* Name & Desc */}
                                    <h3 className="text-3xl font-bold text-white mb-2 tracking-tight group-hover:text-nofx-accent transition-colors">{agent.name}</h3>
                                    <p className="text-zinc-500 text-sm mb-8 leading-relaxed h-10">{agent.desc}</p>

                                    {/* Stats Grid */}
                                    <div className="grid grid-cols-3 gap-px bg-zinc-800/50 border border-zinc-800 rounded overflow-hidden mb-8">
                                        <div className="bg-black/60 p-3 text-center group-hover:bg-zinc-900/60 transition-colors">
                                            <div className="text-[10px] text-zinc-500 uppercase font-mono mb-1">APY</div>
                                            <div className="text-green-400 font-bold">{agent.apy}</div>
                                        </div>
                                        <div className="bg-black/60 p-3 text-center group-hover:bg-zinc-900/60 transition-colors">
                                            <div className="text-[10px] text-zinc-500 uppercase font-mono mb-1">Win %</div>
                                            <div className="text-white font-bold">{agent.winRate}</div>
                                        </div>
                                        <div className="bg-black/60 p-3 text-center group-hover:bg-zinc-900/60 transition-colors">
                                            <div className="text-[10px] text-zinc-500 uppercase font-mono mb-1">Risk</div>
                                            <div className={`${agent.color} font-bold`}>{agent.risk}</div>
                                        </div>
                                    </div>

                                    {/* Action Btn */}
                                    <button
                                        onClick={handleInitialize}
                                        className={`w-full py-4 text-xs font-bold font-mono uppercase tracking-[0.2em] border border-zinc-700 hover:border-${agent.color === 'text-nofx-gold' ? 'nofx-gold' : 'white'} hover:bg-white/5 transition-all flex items-center justify-center gap-2 group-hover:text-white cursor-pointer`}
                                    >
                                        <span className={agent.color}>[</span> INITIALIZE <span className={agent.color}>]</span>
                                    </button>
                                </div>

                                {/* Decorative Background Elements */}
                                <div className="absolute -right-10 -bottom-10 w-40 h-40 bg-gradient-to-br from-white/5 to-transparent rounded-full blur-2xl group-hover:opacity-50 transition-opacity opacity-20"></div>
                                <div className="absolute inset-0 bg-scanlines opacity-20 pointer-events-none"></div>

                            </motion.div>
                        )
                    })}
                </div>
            </div>
        </section>
    )
}
