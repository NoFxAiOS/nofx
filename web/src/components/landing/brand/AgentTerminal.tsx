import { motion } from 'framer-motion'

export default function AgentTerminal() {
    return (
        <motion.div
            initial={{ opacity: 0, y: 30, rotate: 0 }}
            animate={{ opacity: 1, y: 0, rotate: 2 }}
            transition={{ duration: 0.8, delay: 0.3 }}
            className="w-[380px] lg:w-[440px] relative group"
        >
            {/* Terminal frame */}
            <div className="relative bg-[#0B0F14] rounded-2xl overflow-hidden shadow-2xl shadow-black/80 border border-zinc-800/80">

                {/* Scanline overlay */}
                <div className="absolute inset-0 pointer-events-none z-50 opacity-[0.02]" style={{
                    backgroundImage: 'repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(255,255,255,0.03) 2px, rgba(255,255,255,0.03) 4px)'
                }} />

                {/* Header bar - macOS style */}
                <div className="flex items-center justify-between px-4 py-2.5 bg-[#0D1117] border-b border-zinc-800/60">
                    {/* Window controls */}
                    <div className="flex items-center gap-2">
                        <div className="flex items-center gap-1.5">
                            <div className="w-3 h-3 rounded-full bg-[#ff5f57] hover:brightness-110 transition-all" />
                            <div className="w-3 h-3 rounded-full bg-[#febc2e] hover:brightness-110 transition-all" />
                            <div className="w-3 h-3 rounded-full bg-[#28c840] hover:brightness-110 transition-all" />
                        </div>
                    </div>
                    {/* Title */}
                    <div className="absolute left-1/2 -translate-x-1/2 flex items-center gap-2">
                        <span className="text-zinc-400 text-xs font-mono">NOFX Agent Terminal</span>
                    </div>
                    {/* Live indicator */}
                    <div className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-green-500/10 border border-green-500/20">
                        <div className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" />
                        <span className="text-green-400 text-[10px] font-mono uppercase tracking-wider">Live</span>
                    </div>
                </div>

                {/* Portfolio PnL Section */}
                <div className="p-4 border-b border-zinc-800/40">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-zinc-500 text-xs font-mono uppercase tracking-wider">Portfolio PnL</span>
                        <div className="flex gap-1">
                            <button className="px-2 py-0.5 bg-nofx-gold/20 border border-nofx-gold/30 rounded text-[10px] text-nofx-gold font-mono">24H</button>
                            <button className="px-2 py-0.5 text-[10px] text-zinc-600 font-mono hover:text-zinc-400 transition-colors">7D</button>
                            <button className="px-2 py-0.5 text-[10px] text-zinc-600 font-mono hover:text-zinc-400 transition-colors">30D</button>
                        </div>
                    </div>
                    <div className="flex items-baseline gap-3">
                        <span className="text-3xl font-bold text-green-400 font-mono tracking-tight">+$12,847.50</span>
                        <span className="text-green-500/80 text-sm font-mono">+8.42%</span>
                    </div>

                    {/* Chart Area */}
                    <div className="mt-4 h-16 rounded-lg overflow-hidden relative">
                        <svg className="w-full h-full" preserveAspectRatio="none" viewBox="0 0 400 64">
                            <defs>
                                <linearGradient id="chartGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" stopColor="#22C55E" stopOpacity="0.2" />
                                    <stop offset="100%" stopColor="#22C55E" stopOpacity="0" />
                                </linearGradient>
                            </defs>
                            <path
                                d="M0,56 C40,52 80,48 120,40 C160,32 200,28 240,24 C280,20 320,16 360,12 L400,8 L400,64 L0,64 Z"
                                fill="url(#chartGradient)"
                            />
                            <path
                                d="M0,56 C40,52 80,48 120,40 C160,32 200,28 240,24 C280,20 320,16 360,12 L400,8"
                                fill="none"
                                stroke="#22C55E"
                                strokeWidth="1.5"
                            />
                        </svg>
                    </div>
                </div>

                {/* Metrics Row */}
                <div className="grid grid-cols-3 divide-x divide-zinc-800/40 border-b border-zinc-800/40">
                    <div className="p-3 text-center">
                        <div className="text-zinc-500 text-[10px] font-mono uppercase tracking-wider mb-1">OI</div>
                        <div className="text-white font-bold font-mono">$847M</div>
                        <div className="text-green-500 text-[10px] font-mono">↑ 2.1%</div>
                    </div>
                    <div className="p-3 text-center">
                        <div className="text-zinc-500 text-[10px] font-mono uppercase tracking-wider mb-1">Netflow</div>
                        <div className="text-green-400 font-bold font-mono">+$124M</div>
                        <div className="text-zinc-500 text-[10px] font-mono">24h inflow</div>
                    </div>
                    <div className="p-3 text-center">
                        <div className="text-zinc-500 text-[10px] font-mono uppercase tracking-wider mb-1">L/S Ratio</div>
                        <div className="text-white font-bold font-mono">1.24</div>
                        <div className="flex gap-0.5 mt-1 px-2">
                            <div className="h-1 bg-green-500/60 rounded-l flex-[55]" />
                            <div className="h-1 bg-red-500/60 rounded-r flex-[45]" />
                        </div>
                    </div>
                </div>

                {/* Order Book */}
                <div className="p-4 border-b border-zinc-800/40">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-zinc-400 text-xs font-mono uppercase tracking-wider">Order Book</span>
                        <span className="text-zinc-600 text-[10px] font-mono">Spread: <span className="text-nofx-gold">0.02%</span></span>
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                        {/* Asks */}
                        <div className="space-y-1">
                            {[
                                { price: '97,289.50', amount: '2.451', depth: 70 },
                                { price: '97,267.00', amount: '1.832', depth: 55 },
                                { price: '97,251.00', amount: '0.945', depth: 30 },
                            ].map((ask, i) => (
                                <div key={i} className="relative flex justify-between text-[11px] py-1 px-1.5 rounded">
                                    <div className="absolute inset-0 bg-red-500/10 rounded-sm" style={{ width: `${ask.depth}%` }} />
                                    <span className="relative text-red-400 font-mono">{ask.price}</span>
                                    <span className="relative text-zinc-500 font-mono">{ask.amount}</span>
                                </div>
                            ))}
                        </div>
                        {/* Bids */}
                        <div className="space-y-1">
                            {[
                                { price: '97,244.50', amount: '3.127', depth: 85 },
                                { price: '97,221.00', amount: '4.592', depth: 100 },
                                { price: '97,198.00', amount: '1.845', depth: 50 },
                            ].map((bid, i) => (
                                <div key={i} className="relative flex justify-between text-[11px] py-1 px-1.5 rounded">
                                    <div className="absolute inset-0 bg-green-500/10 rounded-sm" style={{ width: `${bid.depth}%` }} />
                                    <span className="relative text-green-400 font-mono">{bid.price}</span>
                                    <span className="relative text-zinc-500 font-mono">{bid.amount}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                {/* Active Positions */}
                <div className="p-4">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-zinc-400 text-xs font-mono uppercase tracking-wider">Positions</span>
                        <span className="text-green-400 text-xs font-mono font-medium">+$12,847</span>
                    </div>
                    <div className="space-y-2">
                        {[
                            { coin: 'BTC', name: 'BTC-PERP', size: '0.5', profit: '+$6,420', percent: '+12.8%', color: '#F7931A' },
                            { coin: 'ETH', name: 'ETH-PERP', size: '3.2', profit: '+$4,127', percent: '+7.6%', color: '#627EEA' },
                            { coin: 'BNB', name: 'BNB-PERP', size: '8.5', profit: '+$2,300', percent: '+5.2%', color: '#F3BA2F' },
                        ].map((pos, i) => (
                            <div key={i} className="flex items-center justify-between py-2 px-2 rounded-lg bg-zinc-900/50 hover:bg-zinc-800/50 transition-colors">
                                <div className="flex items-center gap-3">
                                    <div
                                        className="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold border"
                                        style={{
                                            backgroundColor: pos.color + '15',
                                            borderColor: pos.color + '30',
                                            color: pos.color
                                        }}
                                    >
                                        {pos.coin}
                                    </div>
                                    <div>
                                        <div className="text-white text-sm font-mono">{pos.name}</div>
                                        <div className="flex items-center gap-2 text-[10px]">
                                            <span className="text-green-400 bg-green-500/10 px-1.5 py-0.5 rounded font-mono">LONG</span>
                                            <span className="text-zinc-500 font-mono">{pos.size} {pos.coin}</span>
                                        </div>
                                    </div>
                                </div>
                                <div className="text-right">
                                    <div className="text-green-400 font-mono font-medium">{pos.profit}</div>
                                    <div className="text-green-500/70 text-[10px] font-mono">{pos.percent}</div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                {/* Footer status bar */}
                <div className="px-4 py-2 bg-[#0D1117] border-t border-zinc-800/60 flex items-center justify-between">
                    <div className="flex items-center gap-3 text-[10px] font-mono text-zinc-600">
                        <span className="flex items-center gap-1">
                            <div className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                            Connected
                        </span>
                        <span>Latency: 12ms</span>
                    </div>
                    <div className="text-[10px] font-mono text-zinc-600">
                        mainnet • v2.4.0
                    </div>
                </div>
            </div>
        </motion.div>
    )
}
