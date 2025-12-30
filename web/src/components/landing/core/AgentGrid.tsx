import { motion } from 'framer-motion'
import { Bot, TrendingUp, Layers } from 'lucide-react'

const agents = [
    { name: "Alpha-1", type: "Scalper", apy: "142%", winRate: "68%", exposure: "Low", avatar: "/images/nofx_mascot.png", color: "text-nofx-gold" },
    { name: "Beta-X", type: "Swing", apy: "89%", winRate: "55%", exposure: "Med", icon: TrendingUp, color: "text-blue-400" },
    { name: "Gamma-Ray", type: "Arbitrage", apy: "24%", winRate: "99%", exposure: "Zero", icon: Layers, color: "text-purple-400" },
]

export default function AgentGrid() {
    return (
        <section id="market-scanner" className="py-24 bg-nofx-bg relative">
            <div className="max-w-7xl mx-auto px-6">

                <div className="flex items-center gap-4 mb-12">
                    <div className="w-2 h-8 bg-nofx-gold" />
                    <h2 className="text-3xl font-black text-white uppercase tracking-tighter">
                        Deployable <span className="text-nofx-gold">Agents</span>
                    </h2>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                    {agents.map((agent, i) => {
                        const Icon = agent.icon
                        return (
                            <motion.div
                                key={i}
                                initial={{ opacity: 0, scale: 0.95 }}
                                whileInView={{ opacity: 1, scale: 1 }}
                                transition={{ delay: i * 0.1 }}
                                className={`relative bg-zinc-900/50 border border-zinc-800 p-6 overflow-hidden hover:border-zinc-600 transition-colors group ${i === 0 ? 'border-nofx-gold/50 shadow-[0_0_30px_rgba(240,185,11,0.1)]' : ''}`}
                            >
                                {/* Header */}
                                <div className="flex justify-between items-start mb-6">
                                    <div>
                                        <div className="text-zinc-400 text-xs font-mono uppercase mb-1">{agent.type} CLASS</div>
                                        <div className="text-2xl font-bold text-white flex items-center gap-2">
                                            {agent.name}
                                            {i === 0 && <span className="text-[10px] bg-nofx-gold text-black px-1.5 py-0.5 rounded font-bold">TOP RATED</span>}
                                        </div>
                                    </div>
                                    <Bot className={`w-8 h-8 ${agent.color}`} />
                                </div>

                                {/* Stats Grid */}
                                <div className="grid grid-cols-3 gap-2 mb-6 font-mono text-sm">
                                    <div className="bg-black/40 p-2 rounded border border-zinc-800 group-hover:border-zinc-700 transition-colors">
                                        <div className="text-zinc-500 text-[10px] uppercase">APY</div>
                                        <div className="text-green-400 font-bold">{agent.apy}</div>
                                    </div>
                                    <div className="bg-black/40 p-2 rounded border border-zinc-800 group-hover:border-zinc-700 transition-colors">
                                        <div className="text-zinc-500 text-[10px] uppercase">Win Rate</div>
                                        <div className={`font-bold ${agent.color}`}>{agent.winRate}</div>
                                    </div>
                                    <div className="bg-black/40 p-2 rounded border border-zinc-800 group-hover:border-zinc-700 transition-colors">
                                        <div className="text-zinc-500 text-[10px] uppercase">Risk</div>
                                        <div className="text-white font-bold">{agent.exposure}</div>
                                    </div>
                                </div>

                                {/* Visual Asset (Avatar or Abstract Icon) */}
                                <div className="absolute right-[-20px] bottom-[-20px] opacity-10 group-hover:opacity-20 transition-all duration-500 group-hover:scale-110 pointer-events-none">
                                    {agent.avatar ? (
                                        <img
                                            src={agent.avatar}
                                            alt="Agent"
                                            className="w-40 h-40 object-cover grayscale mix-blend-screen"
                                        />
                                    ) : (
                                        Icon && <Icon strokeWidth={1} className={`w-40 h-40 ${agent.color}`} />
                                    )}
                                </div>

                                {/* Action */}
                                <button className={`w-full py-3 font-bold uppercase tracking-wider text-sm transition-colors ${i === 0
                                    ? 'bg-nofx-gold text-black hover:bg-white'
                                    : 'bg-zinc-800 text-zinc-400 hover:bg-zinc-700'
                                    }`}>
                                    Initialize Agent
                                </button>

                            </motion.div>
                        )
                    })}
                </div>
            </div>
        </section>
    )
}
