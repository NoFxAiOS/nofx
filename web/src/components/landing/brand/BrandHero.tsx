import { motion } from 'framer-motion'
import { ArrowRight, Github } from 'lucide-react'
import { Marquee } from './Marquee'
import { OFFICIAL_LINKS } from '../../../constants/branding'
import AgentTerminal from './AgentTerminal'

export default function BrandHero() {
    const handleScroll = () => {
        const element = document.getElementById('features')
        if (element) {
            element.scrollIntoView({ behavior: 'smooth' })
        }
    }

    return (
        <section className="relative w-full min-h-screen bg-nofx-bg text-nofx-text overflow-hidden flex flex-col pt-16">

            {/* Top Marquee */}
            <div className="w-full bg-nofx-gold text-black font-bold py-2 border-y border-black z-20">
                <Marquee speed={40}>
                    <span className="mx-8 text-sm md:text-base uppercase tracking-widest">NOFX AI TRADING • AUTOMATED WEALTH • DECENTRALIZED INTELLIGENCE • PUNK ETHOS •</span>
                    <span className="mx-8 text-sm md:text-base uppercase tracking-widest">NOFX AI TRADING • AUTOMATED WEALTH • DECENTRALIZED INTELLIGENCE • PUNK ETHOS •</span>
                </Marquee>
            </div>

            <div className="flex flex-col lg:flex-row flex-1 relative z-10">

                {/* Left Content */}
                <div className="flex-1 flex flex-col justify-center px-6 lg:px-16 pt-12 lg:pt-0 relative z-20">
                    <motion.div
                        initial={{ opacity: 0, x: -50 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ duration: 0.8, ease: "circOut" }}
                    >
                        <h1 className="text-6xl md:text-8xl lg:text-[7rem] font-black leading-[0.9] tracking-tighter mb-6">
                            AI TRADING<br />
                            <span className="text-nofx-gold">EVOLVED</span>
                        </h1>

                        <p className="text-xl md:text-2xl text-zinc-400 max-w-xl mb-10 font-mono leading-relaxed">
                            Autonomous trading agents. High-frequency execution.
                            <br />
                            Institutional-grade strategies for the
                            <span className="text-white font-bold ml-2 bg-nofx-accent px-2 py-0.5">DEGENERATES</span>.
                        </p>

                        <div className="flex flex-wrap gap-4">
                            <button
                                onClick={handleScroll}
                                className="bg-nofx-gold text-black text-lg font-black px-8 py-4 uppercase tracking-wider hover:bg-white hover:scale-105 transition-all flex items-center gap-2 clip-path-slant"
                                style={{ clipPath: 'polygon(0 0, 100% 0, 95% 100%, 0% 100%)' }}
                            >
                                Start Trading <ArrowRight className="w-6 h-6" />
                            </button>

                            <a
                                href={OFFICIAL_LINKS.github}
                                target="_blank"
                                rel="noreferrer"
                                className="border-2 border-white/20 text-white text-lg font-bold px-8 py-4 uppercase tracking-wider hover:bg-white/10 hover:border-white transition-all flex items-center gap-2"
                            >
                                <Github className="w-5 h-5" /> Source
                            </a>
                        </div>

                        <div className="mt-12 flex items-center gap-8 text-zinc-500 font-mono text-xs md:text-sm">
                            <div className="flex items-center gap-2">
                                <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                                SYSTEM ONLINE
                            </div>
                            <div className="flex items-center gap-2">
                                <div className="w-2 h-2 bg-nofx-accent rounded-full" />
                                VP v2.4.0
                            </div>
                        </div>
                    </motion.div>
                </div>

                {/* Right Visual - Agent Terminal */}
                <div className="flex-1 relative overflow-visible flex items-center justify-center py-8 lg:py-0 min-h-[600px]">
                    {/* Background gradient orbs */}
                    <div className="absolute top-1/2 right-[15%] -translate-y-1/2 w-[450px] h-[450px] rounded-full bg-gradient-to-br from-nofx-gold/20 via-nofx-gold/5 to-transparent blur-[80px]" />
                    <div className="absolute top-[25%] right-[35%] w-[250px] h-[250px] rounded-full bg-nofx-accent/10 blur-[60px]" />

                    {/* Subtle dot grid */}
                    <div
                        className="absolute inset-0 opacity-[0.04]"
                        style={{
                            backgroundImage: 'radial-gradient(circle at 1px 1px, rgba(255,255,255,0.4) 1px, transparent 0)',
                            backgroundSize: '32px 32px'
                        }}
                    />

                    {/* Terminal Panel */}
                    <div className="relative z-10">
                        <AgentTerminal />
                    </div>
                </div>
            </div>
        </section>
    )
}
