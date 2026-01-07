import { motion } from 'framer-motion'
import { ArrowRight, Shield, Activity, CircuitBoard, Cpu, Wifi, Globe, Lock, Zap, Star, GitFork, Users, MessageCircle } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useGitHubStats } from '../../../hooks/useGitHubStats'

export default function TerminalHero() {

    // Real-time price state
    const [prices, setPrices] = useState<Record<string, string>>({
        BTC: '...',
        ETH: '...',
        SOL: '...',
        BNB: '...',
        XRP: '...',
        DOGE: '...',
        ADA: '...',
        AVAX: '...'
    })

    useEffect(() => {
        const fetchPrices = async () => {
            const symbols = ['BTC', 'ETH', 'SOL', 'BNB', 'XRP', 'DOGE', 'ADA', 'AVAX']

            // We use Promise.all to fetch them in parallel for now, or sequentially if rate limited. 
            // Parallel is better for UI responsiveness.
            try {
                const results = await Promise.all(symbols.map(async (sym) => {
                    try {
                        // Use native fetch to bypass global error handlers (toasts) in httpClient
                        const response = await fetch(`/api/klines?symbol=${sym}USDT&interval=1m&limit=1`)
                        if (!response.ok) return null

                        const res = await response.json()
                        // Check for standard API response structure or direct array
                        const klineData = res.data || res

                        if (Array.isArray(klineData) && klineData.length > 0) {
                            const closePrice = parseFloat(klineData[0].close || klineData[0][4]) // Handle object or array format
                            if (isNaN(closePrice)) return null

                            // Format price: < 1 use 4 decimals, > 1 use 2
                            const formatted = closePrice < 1
                                ? closePrice.toFixed(4)
                                : closePrice.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
                            return { symbol: sym, price: formatted }
                        }
                    } catch (err) {
                        // Silent failure for background polling
                    }
                    return null
                }))

                const newPrices: Record<string, string> = {}
                results.forEach(r => {
                    if (r) newPrices[r.symbol] = r.price
                })

                setPrices(prev => ({ ...prev, ...newPrices }))

            } catch (e) {
                console.error("Failed to fetch market prices", e)
            }
        }

        // Only fetch once on mount, cache the result
        fetchPrices()
    }, [])

    return (
        <section className="relative w-full min-h-screen bg-nofx-bg text-nofx-text overflow-hidden flex flex-col pt-20">

            {/* BACKGROUND LAYERS */}
            {/* 1. Grid */}
            <div className="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-20 mix-blend-soft-light pointer-events-none"></div>
            <div className="absolute inset-x-0 bottom-0 h-[50vh] bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:40px_40px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] pointer-events-none md:hidden" style={{ transform: 'perspective(500px) rotateX(60deg) translateY(100px) scale(2)' }}></div>
            <div className="absolute inset-0 bg-grid-pattern opacity-[0.03] pointer-events-none"></div>

            {/* 2. World Map / Data Viz Background (Abstract) */}
            <div className="absolute inset-0 flex items-center justify-center opacity-10 pointer-events-none">
                <div className="w-[80vw] h-[80vw] rounded-full border border-nofx-gold/20 animate-pulse-slow"></div>
                <div className="absolute w-[60vw] h-[60vw] rounded-full border border-dashed border-nofx-accent/20 animate-[spin_60s_linear_infinite]"></div>
            </div>

            {/* 3. Gradient Spots - Intensified for Mobile */}
            <div className="absolute top-[-10%] left-[-10%] w-[40vw] h-[40vw] bg-nofx-gold/20 rounded-full blur-[120px] pointer-events-none mix-blend-screen"></div>
            <div className="absolute bottom-[-10%] right-[-10%] w-[40vw] h-[40vw] bg-nofx-accent/10 rounded-full blur-[120px] pointer-events-none mix-blend-screen"></div>

            {/* Mobile Bottom Fade */}
            <div className="absolute bottom-0 left-0 w-full h-32 bg-gradient-to-t from-nofx-bg to-transparent z-20 pointer-events-none md:hidden" />

            {/* Mobile Floating HUD - Moved to Left to avoid covering face */}
            <div className="md:hidden absolute top-24 left-4 z-0 opacity-40 pointer-events-none">
                <div className="w-24 h-24 border border-dashed border-nofx-gold/30 rounded-full animate-spin-slow flex items-center justify-center">
                    <div className="w-16 h-16 border border-nofx-accent/30 rounded-full"></div>
                </div>
            </div>

            {/* CONTENT GRID */}
            <div className="relative z-10 flex-1 grid grid-cols-1 lg:grid-cols-12 gap-0 lg:gap-8 max-w-[1800px] mx-auto w-full px-6 h-full pb-20 pt-10 pointer-events-none">

                {/* LEFT COLUMN: TELEMETRY & STATUS */}
                <div className="hidden lg:flex col-span-3 flex-col justify-between h-full border-r border-white/5 pr-8 py-10 pointer-events-auto">

                    {/* Top: System Health */}
                    <div className="space-y-6">
                        <div className="tech-border p-4 bg-black/40 backdrop-blur-sm">
                            <h3 className="text-xs font-mono text-nofx-gold mb-4 flex items-center gap-2">
                                <Activity className="w-3 h-3" /> SYSTEM_DIAGNOSTICS
                            </h3>
                            <div className="space-y-3 font-mono text-[10px] text-zinc-400">
                                <div className="flex justify-between items-center">
                                    <span>KERNEL_LATENCY</span>
                                    <span className="text-nofx-accent">12ms</span>
                                </div>
                                <div className="w-full h-1 bg-zinc-800 rounded-full overflow-hidden">
                                    <div className="w-[90%] h-full bg-nofx-accent/50"></div>
                                </div>

                                <div className="flex justify-between items-center">
                                    <span>MEMORY_INTEGRITY</span>
                                    <span className="text-nofx-success">100%</span>
                                </div>
                                <div className="w-full h-1 bg-zinc-800 rounded-full overflow-hidden">
                                    <div className="w-full h-full bg-nofx-success/50"></div>
                                </div>

                                <div className="flex justify-between items-center">
                                    <span>UPTIME</span>
                                    <span className="text-white">99.999%</span>
                                </div>
                            </div>
                        </div>

                        <div className="p-4 border border-zinc-800/50 rounded bg-zinc-900/20">
                            <div className="flex items-center gap-3 text-zinc-500 mb-2">
                                <Shield className="w-4 h-4" />
                                <span className="text-[10px] font-mono tracking-widest">SECURITY PROTOCOLS</span>
                            </div>
                            <div className="flex gap-1">
                                <div className="h-1 flex-1 bg-nofx-gold"></div>
                                <div className="h-1 flex-1 bg-nofx-gold"></div>
                                <div className="h-1 flex-1 bg-nofx-gold"></div>
                                <div className="h-1 flex-1 bg-zinc-800"></div>
                            </div>
                            <div className="mt-2 text-right text-[10px] text-nofx-gold/80 font-mono">LEVEL 3 ACTIVATE</div>
                        </div>
                    </div>

                    {/* Bottom: Network Log */}
                    <div className="font-mono text-[10px] text-zinc-600 space-y-1 opacity-70">
                        <div>&gt; CONNECTING TO MAINNET... OK</div>
                        <div>&gt; SYNCING NODES (424/424)... OK</div>
                        <div>&gt; LOADING ASSETS... DONE</div>
                        <div className="animate-pulse">&gt; AWAITING USER INPUT_</div>
                    </div>
                </div>

                {/* CENTER COLUMN: MAIN ACTION */}
                <div className="col-span-1 lg:col-span-6 flex flex-col items-center justify-center text-center relative z-20 pointer-events-auto">

                    {/* Project Identity Chip */}
                    <motion.div
                        initial={{ opacity: 0, y: -20 }}
                        animate={{ opacity: 1, y: 0 }}
                        className="mb-8 inline-flex items-center gap-3 px-4 py-2 rounded-full border border-nofx-gold/20 bg-nofx-gold/5 backdrop-blur-md"
                    >
                        <span className="relative flex h-2 w-2">
                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-nofx-gold opacity-75"></span>
                            <span className="relative inline-flex rounded-full h-2 w-2 bg-nofx-gold"></span>
                        </span>
                        <span className="text-xs font-mono text-nofx-gold tracking-widest">NOFX OPEN-SOURCE AGENTIC OS</span>
                    </motion.div>

                    {/* Main Title - Massive & Impactful */}
                    {/* Main Title - Massive & Impactful */}
                    <div className="relative z-20 mix-blend-hard-light md:mix-blend-normal">
                        <h1 className="text-5xl sm:text-6xl md:text-8xl lg:text-9xl font-black tracking-tighter leading-[0.9] md:leading-[0.8] mb-6 select-none bg-clip-text text-transparent bg-gradient-to-b from-white via-white to-zinc-600 drop-shadow-2xl">
                            AGENTIC<br />
                            <span className="text-transparent bg-clip-text bg-gradient-to-r from-nofx-gold via-white to-nofx-gold animate-shimmer bg-[length:200%_auto] tracking-tight filter drop-shadow-[0_0_15px_rgba(234,179,8,0.3)]">TRADING</span>
                        </h1>

                        <p className="max-w-xl text-zinc-200 md:text-zinc-400 text-lg mb-6 font-light leading-relaxed drop-shadow-md">
                            The World's First Open-Source Agentic Trading OS.
                            Deploy autonomous high-frequency trading agents powered by advanced LLMs.
                        </p>
                    </div>

                    {/* Market Access Strip - Prominent Display */}
                    {/* Market Access Strip - Prominent Display */}
                    <div className="flex flex-col gap-4 mb-14">
                        <div className="text-nofx-gold/80 font-mono text-xs tracking-[0.3em] uppercase flex items-center gap-2 ml-1">
                            <span className="relative flex h-2 w-2">
                                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-nofx-success opacity-75"></span>
                                <span className="relative inline-flex rounded-full h-2 w-2 bg-nofx-success"></span>
                            </span>
                            Live Data Feeds Active
                        </div>
                        <div className="flex flex-wrap gap-4 font-mono">
                            {['CRYPTO', 'US STOCKS', 'FOREX', 'METALS'].map((market) => (
                                <div key={market} className="relative group cursor-default">
                                    <div className="absolute -inset-0.5 bg-gradient-to-r from-nofx-gold/20 to-blue-600/20 rounded-lg blur opacity-0 group-hover:opacity-100 transition duration-500"></div>
                                    <div className="relative flex items-center gap-3 px-6 py-3 rounded-lg bg-zinc-900/80 border border-zinc-700 hover:border-nofx-gold/50 transition-all duration-300 backdrop-blur-sm">
                                        <div className="w-1.5 h-1.5 rounded-full bg-nofx-success shadow-[0_0_8px_rgba(74,222,128,0.6)] animate-pulse"></div>
                                        <span className="text-lg md:text-xl font-bold text-white tracking-wider group-hover:text-nofx-gold transition-colors">{market}</span>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>

                    {/* Command Line Input Simulation */}
                    <div className="w-full max-w-lg h-12 bg-black/50 border border-zinc-800 rounded flex items-center px-4 mb-10 font-mono text-sm shadow-2xl backdrop-blur-sm group hover:border-nofx-gold/50 transition-colors cursor-text" onClick={() => document.getElementById('market-scanner')?.scrollIntoView({ behavior: 'smooth' })}>
                        <span className="text-nofx-success mr-2">➜</span>
                        <span className="text-nofx-accent mr-2">~</span>
                        <span className="text-zinc-500">deploy agent --strategy=hft</span>
                        <span className="w-2 h-4 bg-nofx-gold ml-1 animate-pulse"></span>
                    </div>

                    {/* CTA Buttons */}
                    <div className="flex flex-col sm:flex-row gap-4 w-full justify-center">
                        <button
                            onClick={() => document.getElementById('market-scanner')?.scrollIntoView({ behavior: 'smooth' })}
                            className="group relative overflow-hidden bg-nofx-gold text-black px-8 py-4 font-bold font-mono tracking-wider hover:scale-105 transition-transform duration-200"
                            style={{ clipPath: 'polygon(10% 0, 100% 0, 100% 70%, 90% 100%, 0 100%, 0 30%)' }}
                        >
                            <span className="relative z-10 flex items-center gap-2">
                                INITIALIZE PROTOCOL <ArrowRight className="w-4 h-4" />
                            </span>
                            <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300"></div>
                        </button>
                    </div>

                    {/* Community Stats Row */}
                    <CommunityStats />

                </div>
            </div>

            {/* RIGHT COLUMN: HOLOGRAPHIC DISPLAY - Absolute Overlay for "Far Right" Effect on Desktop, Background on Mobile */}
            <div className="absolute top-20 md:top-0 right-0 h-[50vh] md:h-full w-full lg:w-[45vw] flex pointer-events-none items-center justify-center z-0 opacity-80 lg:opacity-100 mix-blend-normal">
                <div className="relative w-full h-full flex items-center justify-center perspective-1000">
                    {/* 3D Hologram Effect Container */}
                    <div className="relative w-full h-[90%] flex items-center justify-center transform-style-3d rotate-y-[-12deg]">

                        {/* Scanning Grid behind Mascot - Mobile Optimized */}
                        <div className="absolute inset-x-0 top-[10%] bottom-[10%] bg-[linear-gradient(rgba(0,240,255,0.05)_1px,transparent_1px),linear-gradient(90deg,rgba(0,240,255,0.05)_1px,transparent_1px)] bg-[size:20px_20px] [mask-image:radial-gradient(ellipse_at_center,black_40%,transparent_80%)] mobile-grid-pulse"></div>

                        {/* The Mascot Image with Glitch/Holo Effects */}
                        <div className="relative z-10 w-full h-full opacity-100 transition-all duration-500 group flex flex-col justify-end pointer-events-auto">
                            <div className="absolute inset-x-0 bottom-0 top-1/2 bg-nofx-accent/5 blur-[60px] rounded-full animate-pulse-slow transition-colors duration-500 group-hover:bg-nofx-gold/20"></div>

                            {/* Mobile Holo-Portrait Style - Full Color & Optimized & Premium Desktop */}
                            <div className="relative w-full h-full flex items-end justify-center">
                                <img
                                    src="/images/nofx_mascot.png"
                                    alt="Agent NoFX"
                                    className="w-full h-full object-contain object-bottom char-premium-effects animate-breath-mobile transition-all duration-500"
                                    style={{
                                        maskImage: 'radial-gradient(ellipse at center, black 60%, transparent 100%), linear-gradient(to bottom, black 0%, black 85%, transparent 100%)',
                                        WebkitMaskImage: 'radial-gradient(ellipse at center, black 60%, transparent 100%), linear-gradient(to bottom, black 0%, black 85%, transparent 100%)',
                                        maskComposite: 'intersect',
                                        WebkitMaskComposite: 'source-in'
                                    }}
                                />
                                {/* Dynamic Holographic Overlay - Premium Noise & Gradient */}
                                <div className="absolute inset-0 w-full h-full holo-overlay animate-holo opacity-80 pointer-events-none"
                                    style={{
                                        maskImage: 'url(/images/nofx_mascot.png)',
                                        WebkitMaskImage: 'url(/images/nofx_mascot.png)',
                                        maskSize: 'contain',
                                        WebkitMaskSize: 'contain',
                                        maskPosition: 'bottom center',
                                        WebkitMaskPosition: 'bottom center',
                                        maskRepeat: 'no-repeat',
                                        WebkitMaskRepeat: 'no-repeat'
                                    }}
                                />
                            </div>

                            {/* Holo Scan Line - Subtle on Mobile */}
                            <div className="absolute w-full h-1 bg-nofx-accent/30 drop-shadow-[0_0_10px_rgba(0,240,255,0.8)] top-0 animate-scan-fast pointer-events-none mix-blend-overlay"></div>

                            {/* Mobile Glitch Overlay - Reduced Intensity */}
                            <div className="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-10 mix-blend-overlay md:hidden animate-pulse-fast"></div>
                        </div>
                    </div>

                    {/* Floating Data Widgets around Hologram */}
                    <motion.div
                        animate={{ y: [0, -10, 0] }}
                        transition={{ duration: 4, repeat: Infinity, ease: "easeInOut" }}
                        className="absolute top-[30%] left-[10%] bg-black/80 border border-nofx-accent/30 p-2 rounded backdrop-blur-md shadow-neon-blue hidden md:block"
                    >
                        <Cpu className="w-5 h-5 text-nofx-accent" />
                    </motion.div>

                    <motion.div
                        animate={{ y: [0, 10, 0] }}
                        transition={{ duration: 5, repeat: Infinity, ease: "easeInOut", delay: 1 }}
                        className="absolute bottom-[20%] right-[20%] bg-black/80 border border-nofx-gold/30 p-2 rounded backdrop-blur-md shadow-neon hidden md:block"
                    >
                        <Lock className="w-5 h-5 text-nofx-gold" />
                    </motion.div>

                </div>
            </div>

            {/* FLOATING TICKER FOOTER */}
            <div className="absolute bottom-0 w-full bg-black/80 border-t border-zinc-800/50 backdrop-blur-md z-30 overflow-hidden py-2 flex items-center">
                <div className="flex animate-marquee whitespace-nowrap gap-12 text-xs font-mono text-zinc-500 px-4">
                    <span className="flex items-center gap-2"><Globe className="w-3 h-3 text-zinc-600" /> GLOBAL MARKET ACCESS</span>
                    <span className="flex items-center gap-2 text-nofx-gold"><Zap className="w-3 h-3" /> FLASH LOANS ENABLED</span>
                    <span className="flex items-center gap-2"><Wifi className="w-3 h-3 text-green-500" /> LOW LATENCY LINK: 12ms</span>

                    {/* Dynamic Coins */}
                    {Object.entries(prices).map(([symbol, price]) => (
                        <span key={symbol} className="flex items-center gap-2">
                            {symbol.toUpperCase()}/USDT <span className="text-nofx-success">${price}</span>
                        </span>
                    ))}

                    <span className="flex items-center gap-2"><CircuitBoard className="w-3 h-3 text-nofx-accent" /> AI MODEL: GEMINI-PRO-1.5</span>

                    {/* Duplicate sequence for seamless loop effect (basic set) */}
                    {Object.entries(prices).map(([symbol, price]) => (
                        <span key={`${symbol} -dup`} className="flex items-center gap-2 md:hidden">
                            {symbol.toUpperCase()}/USDT <span className="text-nofx-success">${price}</span>
                        </span>
                    ))}
                </div>
            </div>

            {/* CRT OVERLAY (Global) */}
            <div className="absolute inset-0 crt-overlay pointer-events-none z-50 opacity-40"></div>
        </section >
    )
}

import { OFFICIAL_LINKS } from '../../../constants/branding'

function CommunityStats() {
    const { stars, forks, contributors, isLoading, error } = useGitHubStats('NoFxAiOS', 'nofx')

    const stats = [
        {
            label: 'GITHUB STARS',
            value: isLoading ? '...' : (error ? '9,700+' : stars.toLocaleString()),
            icon: Star,
            color: 'text-yellow-400',
            href: OFFICIAL_LINKS.github
        },
        {
            label: 'FORKS',
            value: isLoading ? '...' : (error ? '2,600+' : forks.toLocaleString()),
            icon: GitFork,
            color: 'text-blue-400',
            href: `${OFFICIAL_LINKS.github}/fork`
        },
        {
            label: 'CONTRIBUTORS',
            value: isLoading ? '...' : (contributors > 0 ? contributors : '50+'),
            icon: Users,
            color: 'text-green-400',
            href: `${OFFICIAL_LINKS.github}/graphs/contributors`
        },
        {
            label: 'DEV COMMUNITY',
            value: '6,000+', // Updated as per user request
            icon: MessageCircle,
            color: 'text-blue-500',
            href: OFFICIAL_LINKS.telegram
        }
    ]

    return (
        <div className="mt-12 grid grid-cols-2 md:grid-cols-4 gap-4 w-full max-w-4xl">
            {stats.map((stat, i) => (
                <a
                    key={i}
                    href={stat.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex flex-col items-center justify-center p-3 rounded bg-black/40 border border-zinc-800/50 backdrop-blur-sm group hover:border-nofx-gold/30 transition-all cursor-pointer hover:bg-white/5"
                >
                    <div className="flex items-center gap-2 mb-1">
                        <stat.icon className={`w-4 h-4 ${stat.color}`} />
                        <span className="text-[10px] font-mono text-zinc-500 tracking-wider">{stat.label}</span>
                    </div>
                    <span className="text-xl font-bold font-mono text-white group-hover:text-nofx-gold transition-colors">{stat.value}</span>
                </a>
            ))}
        </div>
    )
}
