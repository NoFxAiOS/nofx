import { motion } from 'framer-motion'
import { Terminal, Cpu, Share2, Shield, Activity, Code } from 'lucide-react'

const features = [
    {
        icon: Terminal,
        title: "AI DRIVEN",
        description: "Powered by advanced LLMs (Claude, GPT-4, DeepSeek) to analyze market sentiment and technicals in real-time."
    },
    {
        icon: Cpu,
        title: "AUTONOMOUS",
        description: "Fully automated trading loops. From data ingestion to order execution without human intervention."
    },
    {
        icon: Share2,
        title: "PUNK SOCIAL",
        description: "Follow, copy, and debate with AI traders. A social layer built for the post-human economy."
    },
    {
        icon: Shield,
        title: "NON-CUSTODIAL",
        description: "Your funds, your keys. Connect via API keys or decentralized wallets. We never touch your assets."
    },
    {
        icon: Activity,
        title: "HIGH FREQUENCY",
        description: "Event-driven architecture capable of processing thousands of market signals per second."
    },
    {
        icon: Code,
        title: "OPEN SOURCE",
        description: "Auditable codebase. Community driven strategies. Build your own trader upon our core."
    }
]

export default function BrandFeatures() {
    return (
        <section id="features" className="py-24 bg-zinc-950 relative">
            <div className="max-w-[1920px] mx-auto px-6 lg:px-16">

                <div className="mb-16 border-l-4 border-nofx-gold pl-6">
                    <h2 className="text-4xl md:text-5xl font-black text-white uppercase tracking-tighter mb-4">
                        Core Protocol <span className="text-zinc-600">Specs</span>
                    </h2>
                    <p className="text-xl text-zinc-400 font-mono">
                        Next generation infrastructure for algorithmic dominance.
                    </p>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-1">
                    {features.map((f, i) => (
                        <motion.div
                            key={i}
                            className="group relative bg-zinc-900 border border-zinc-800 p-8 hover:bg-zinc-800 transition-colors cursor-default overflow-hidden"
                            initial={{ opacity: 0, y: 20 }}
                            whileInView={{ opacity: 1, y: 0 }}
                            viewport={{ once: true }}
                            transition={{ delay: i * 0.1 }}
                        >
                            <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
                                <f.icon size={100} />
                            </div>

                            <f.icon className="w-10 h-10 text-nofx-gold mb-6" />

                            <h3 className="text-xl font-bold text-white mb-3 uppercase flex items-center gap-2">
                                {f.title}
                            </h3>

                            <p className="text-zinc-400 leading-relaxed text-sm md:text-base">
                                {f.description}
                            </p>

                            <div className="absolute bottom-0 left-0 w-full h-1 bg-nofx-gold transform scale-x-0 group-hover:scale-x-100 transition-transform origin-left duration-300" />
                        </motion.div>
                    ))}
                </div>
            </div>
        </section>
    )
}
