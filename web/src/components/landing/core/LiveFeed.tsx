import { motion } from 'framer-motion'
import { Activity, BarChart3, Globe } from 'lucide-react'

// Mock Data for "Live" Feed
const logs = [
    { time: "14:02:23", type: "EXE", msg: "Bot-Alpha executed BUY BTC-USDT @ 64230.50", color: "text-green-500" },
    { time: "14:02:24", type: "SIG", msg: "High vol detected in ETH-PERP. Signal strength: 0.89", color: "text-nofx-gold" },
    { time: "14:02:25", type: "NET", msg: "Block propagation delay < 2ms", color: "text-zinc-500" },
    { time: "14:02:27", type: "EXE", msg: "Bot-Beta executed SELL SOL-USDT @ 145.20", color: "text-red-500" },
    { time: "14:02:28", type: "SYS", msg: "Memory pool optimization complete.", color: "text-nofx-accent" },
    { time: "14:02:30", type: "ARB", msg: "Arbitrage opportunity found: BINANCE vs BYBIT (0.4%)", color: "text-blue-400" },
]

export default function LiveFeed() {
    return (
        <section className="w-full bg-black border-y border-zinc-800 py-4 overflow-hidden">
            <div className="max-w-[1920px] mx-auto px-6 flex flex-col md:flex-row gap-6">

                {/* Left Status Panel */}
                <div className="w-full md:w-1/3 flex items-center justify-between md:justify-start gap-8 text-xs font-mono text-zinc-500 border-b md:border-b-0 md:border-r border-zinc-900 pb-4 md:pb-0">
                    <div className="flex items-center gap-3">
                        <Activity className="w-4 h-4 text-nofx-gold" />
                        <div>
                            <div className="text-zinc-300 font-bold">SYSTEM LOAD</div>
                            <div className="text-nofx-gold">42%</div>
                        </div>
                    </div>
                    <div className="flex items-center gap-3">
                        <Globe className="w-4 h-4 text-nofx-accent" />
                        <div>
                            <div className="text-zinc-300 font-bold">ACTIVE NODES</div>
                            <div className="text-nofx-accent">8,249</div>
                        </div>
                    </div>
                    <div className="flex items-center gap-3">
                        <BarChart3 className="w-4 h-4 text-green-500" />
                        <div>
                            <div className="text-zinc-300 font-bold">24H VOL</div>
                            <div className="text-green-500">$4.2B</div>
                        </div>
                    </div>
                </div>

                {/* Right Scrolling Log */}
                <div className="flex-1 font-mono text-xs md:text-sm h-32 md:h-12 overflow-hidden relative mask-image-b">
                    <div className="absolute inset-0 flex flex-col gap-1 animate-slide-up">
                        {logs.map((log, i) => (
                            <motion.div
                                key={i}
                                initial={{ opacity: 0, x: -10 }}
                                animate={{ opacity: 1, x: 0 }}
                                transition={{ delay: i * 0.2 }}
                                className="flex gap-4"
                            >
                                <span className="text-zinc-600">[{log.time}]</span>
                                <span className="text-zinc-400 font-bold w-8">{log.type}</span>
                                <span className={log.color}>{log.msg}</span>
                            </motion.div>
                        ))}
                    </div>
                </div>

            </div>
        </section>
    )
}
