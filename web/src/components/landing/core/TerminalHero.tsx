import { motion } from 'framer-motion'
import { ArrowRight, Terminal as TerminalIcon, Star, GitFork, Users, Activity, Layers, Cpu, Network } from 'lucide-react'
import { useState, useEffect } from 'react'
import { OFFICIAL_LINKS } from '../../../constants/branding'

export default function TerminalHero() {
    const [text, setText] = useState('')
    const [githubData, setGithubData] = useState({ stars: '9.4k', forks: '2.4k', subscribers: '74' })
    const fullText = "INITIALIZING NOFX KERNEL... CRYPTO | STOCKS | FOREX | METALS... SYSTEM READY."

    useEffect(() => {
        // Typing effect
        let i = 0
        const timer = setInterval(() => {
            setText(fullText.slice(0, i))
            i++
            if (i > fullText.length) clearInterval(timer)
        }, 30)

        // Fetch GitHub Data
        fetch('https://api.github.com/repos/NoFxAiOS/nofx')
            .then(res => res.json())
            .then(data => {
                if (data.stargazers_count) {
                    setGithubData({
                        stars: (data.stargazers_count / 1000).toFixed(1) + 'k',
                        forks: (data.forks_count / 1000).toFixed(1) + 'k',
                        subscribers: data.subscribers_count?.toString() || '74'
                    })
                }
            })
            .catch(err => console.error("Failed to fetch GitHub stats", err))

        return () => clearInterval(timer)
    }, [])

    return (
        <section className="relative w-full min-h-screen bg-nofx-bg text-nofx-text overflow-hidden flex flex-col items-center justify-center pt-20">

            {/* 1. ARCHITECTURAL BACKGROUND / HOLOGRAPHIC CONSTRUCT */}
            <div className="absolute inset-0 z-0 overflow-hidden pointer-events-none select-none">

                {/* The Mascot "Ghost" in the Machine - PREMIUM & CLEAN */}
                <div className="absolute right-0 bottom-0 w-[80vw] lg:w-[45vw] h-[85vh] opacity-90 mix-blend-normal flex items-end justify-end">
                    <div className="relative w-full h-full">
                        <img
                            src="/images/nofx_mascot.png"
                            alt=""
                            className="w-full h-full object-contain object-bottom drop-shadow-[0_0_50px_rgba(240,185,11,0.2)]"
                            style={{
                                maskImage: 'linear-gradient(to top, black 60%, transparent 100%)',
                                filter: 'grayscale(100%) contrast(110%) brightness(110%) sepia(20%) hue-rotate(320deg)'
                            }}
                        />

                        {/* Clean Horizontal Scanline Overlay */}
                        <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0IiBoZWlnaHQ9IjQiPgo8cmVjdCB3aWR0aD0iNCIgaGVpZ2h0PSIxIiBmaWxsPSJyZ2JhKDAsIDAsIDAsIDAuMykiIC8+Cjwvc3ZnPg==')] opacity-50 mix-blend-overlay pointer-events-none" />

                        {/* Subtle Glow Behind */}
                        <div className="absolute right-10 bottom-10 w-64 h-64 bg-nofx-gold/20 rounded-full blur-[100px] -z-10" />
                    </div>
                </div>

                {/* Clean Geometric Grid */}
                <svg className="absolute inset-0 w-full h-full opacity-10" xmlns="http://www.w3.org/2000/svg">
                    <defs>
                        <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
                            <path d="M 40 0 L 0 0 0 40" fill="none" stroke="currentColor" strokeWidth="0.5" className="text-zinc-500" />
                        </pattern>
                    </defs>
                    <rect width="100%" height="100%" fill="url(#grid)" />
                </svg>
            </div>

            <div className="relative z-10 flex flex-col items-center text-center max-w-[1400px] px-6 w-full h-full justify-center">

                <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 w-full items-center">

                    {/* LEFT COLUMN: Main System Interface */}
                    <div className="col-span-1 lg:col-span-8 text-left z-30 flex flex-col justify-center h-full">

                        {/* System Status Tag */}
                        <motion.div
                            initial={{ opacity: 0, y: -10 }}
                            animate={{ opacity: 1, y: 0 }}
                            className="flex flex-wrap items-center gap-3 mb-8"
                        >
                            <div className="px-3 py-1 border border-nofx-gold/30 bg-nofx-gold/5 rounded-sm text-nofx-gold text-xs font-mono flex items-center gap-2 shadow-[0_0_15px_rgba(240,185,11,0.2)]">
                                <div className="w-1.5 h-1.5 bg-nofx-gold rounded-full animate-pulse" />
                                SYSTEM ONLINE
                            </div>
                        </motion.div>

                        {/* Main Headline with Project Specifics */}
                        <motion.div
                            initial={{ opacity: 0, x: -20 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ delay: 0.2 }}
                            className="relative"
                        >
                            <h1 className="text-6xl md:text-8xl xl:text-9xl font-black tracking-tighter leading-[0.85] mb-8 text-white">
                                AGENTIC <br />
                                <span className="text-transparent bg-clip-text bg-gradient-to-r from-nofx-gold via-white to-nofx-gold animate-shimmer bg-[length:200%_100%]">TRADING OS</span>
                            </h1>

                            {/* SVG Connector Line */}
                            <div className="absolute -left-10 top-2 bottom-2 w-px bg-zinc-800 hidden lg:block">
                                <div className="absolute top-0 left-[-1px] w-[3px] h-8 bg-nofx-gold" />
                                <div className="absolute bottom-0 left-[-1px] w-[3px] h-8 bg-zinc-600" />
                            </div>
                        </motion.div>

                        {/* Typing Terminal Output */}
                        <div className="h-24 mb-10 font-mono text-zinc-400 text-sm flex flex-col justify-start gap-3 max-w-2xl border-l-2 border-zinc-800 pl-6">
                            <div className="flex items-center gap-2 text-nofx-gold">
                                <span>&gt;</span> {text}<span className="animate-pulse bg-nofx-gold w-2 h-4 block"></span>
                            </div>

                            {/* Clean Markets Row */}
                            <div className="flex gap-6 text-[10px] md:text-xs text-zinc-500 font-bold tracking-widest uppercase">
                                <span className="flex items-center gap-2 hover:text-white transition-colors"><Network className="w-3 h-3" /> CRYPTO</span>
                                <span className="flex items-center gap-2 hover:text-white transition-colors"><Activity className="w-3 h-3" /> STOCKS</span>
                                <span className="flex items-center gap-2 hover:text-white transition-colors"><Layers className="w-3 h-3" /> FOREX</span>
                                <span className="flex items-center gap-2 hover:text-white transition-colors"><Cpu className="w-3 h-3" /> METALS</span>
                            </div>
                        </div>

                        {/* Primary Actions */}
                        <div className="flex flex-col sm:flex-row gap-4 w-full max-w-lg">
                            <button
                                onClick={() => document.getElementById('market-scanner')?.scrollIntoView({ behavior: 'smooth' })}
                                className="group relative px-8 py-4 bg-nofx-gold text-black font-bold font-mono hover:bg-white transition-all flex items-center justify-between min-w-[200px] hover:shadow-[0_0_20px_rgba(240,185,11,0.4)]"
                                style={{ clipPath: 'polygon(0 0, 100% 0, 100% 80%, 90% 100%, 0% 100%)' }}
                            >
                                <span>DEPLOY TRADERS</span>
                                <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />
                            </button>

                            <a
                                href={OFFICIAL_LINKS.github}
                                target="_blank"
                                rel="noreferrer"
                                className="px-8 py-4 border border-zinc-700 bg-black/50 backdrop-blur-sm text-zinc-300 font-mono hover:border-nofx-accent hover:text-nofx-accent transition-all flex items-center justify-between min-w-[200px]"
                                style={{ clipPath: 'polygon(0 0, 100% 0, 100% 100%, 10% 100%, 0% 80%)' }}
                            >
                                <span>SOURCE CODE</span>
                                <TerminalIcon className="w-4 h-4" />
                            </a>
                        </div>
                    </div>

                    {/* RIGHT COLUMN: Modules & Data HUD */}
                    <div className="col-span-1 lg:col-span-4 flex flex-col gap-6 mt-12 lg:mt-0 z-20">

                        {/* Module 1: GitHub Intelligence */}
                        <div className="border border-zinc-800 bg-black/80 backdrop-blur-md p-6 relative group overflow-hidden">
                            <div className="absolute top-0 right-0 w-20 h-20 bg-nofx-gold/5 rounded-bl-full -mr-10 -mt-10 transition-transform group-hover:scale-150" />

                            <div className="flex justify-between items-start mb-4">
                                <div className="text-xs font-mono text-zinc-500 flex items-center gap-2">
                                    <Users className="w-4 h-4 text-nofx-gold" /> COMMUNITY UPLINK
                                </div>
                                <div className="flex gap-1">
                                    <div className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" />
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <div className="text-2xl font-bold text-white flex items-center gap-1">
                                        {githubData.stars} <Star className="w-3 h-3 text-nofx-gold fill-nofx-gold" />
                                    </div>
                                    <div className="text-[10px] text-zinc-500 uppercase tracking-wider">Active Star-gazers</div>
                                </div>
                                <div>
                                    <div className="text-2xl font-bold text-white flex items-center gap-1">
                                        {githubData.forks} <GitFork className="w-3 h-3 text-zinc-500" />
                                    </div>
                                    <div className="text-[10px] text-zinc-500 uppercase tracking-wider">Protocol Forks</div>
                                </div>
                            </div>
                        </div>

                        {/* Module 2: System Capabilities (Specific to NoFX) */}
                        <div className="border border-zinc-800 bg-black/60 backdrop-blur-sm p-6 space-y-3 hidden md:block">
                            <div className="text-xs font-mono text-zinc-500 mb-2">ACTIVE MODULES</div>

                            <div className="flex justify-between items-center text-sm font-mono border-b border-zinc-900 pb-2">
                                <span className="text-zinc-300">STRATEGY STUDIO</span>
                                <span className="text-green-500 text-xs">READY</span>
                            </div>
                            <div className="flex justify-between items-center text-sm font-mono border-b border-zinc-900 pb-2">
                                <span className="text-zinc-300">DEBATE ARENA</span>
                                <span className="text-green-500 text-xs text-nofx-accent animate-pulse">Running</span>
                            </div>
                            <div className="flex justify-between items-center text-sm font-mono pb-2">
                                <span className="text-zinc-300">BACKTEST LAB</span>
                                <span className="text-zinc-500 text-xs">Idle</span>
                            </div>
                        </div>

                    </div>

                </div>

            </div>

            {/* Decorative Footer */}
            <div className="absolute bottom-0 w-full border-t border-zinc-800 bg-black/90 backdrop-blur-md p-3 flex flex-wrap justify-between items-center text-[10px] md:text-xs text-zinc-500 font-mono z-20">
                <div className="flex gap-6 px-4">
                    <span className="flex items-center gap-2">
                        <div className="w-2 h-2 bg-nofx-gold/50 rounded-full" />
                        NOFX-OS
                    </span>
                    <span className="hidden sm:inline">24H VOL: $42.8M</span>
                    <span className="hidden sm:inline">ACTIVE AGENTS: 1,024</span>
                </div>
                <div className="px-4 flex gap-4">
                    <span className="text-nofx-gold">ENCRYPTED CONNECTION</span>
                </div>
            </div>
        </section>
    )
}
