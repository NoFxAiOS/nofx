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
            <div className="relative bg-nofx-bg-lighter rounded-2xl overflow-hidden shadow-lg border border-[rgba(26,24,19,0.14)]">

                {/* Header bar - macOS style */}
                <div className="flex items-center justify-between px-4 py-2.5 bg-nofx-bg-deeper border-b border-[rgba(26,24,19,0.14)]">
                    {/* Window controls */}
                    <div className="flex items-center gap-2">
                        <div className="flex items-center gap-1.5">
                            <div className="w-3 h-3 rounded-full bg-[#D6433A] hover:brightness-110 transition-all" />
                            <div className="w-3 h-3 rounded-full bg-[#E0483B] hover:brightness-110 transition-all" />
                            <div className="w-3 h-3 rounded-full bg-[#2E8B57] hover:brightness-110 transition-all" />
                        </div>
                    </div>
                    {/* Title */}
                    <div className="absolute left-1/2 -translate-x-1/2 flex items-center gap-2">
                        <span className="text-nofx-text-muted text-xs font-mono">NOFX Trader Terminal</span>
                    </div>
                    {/* Live indicator */}
                    <div className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-nofx-success/10 border border-nofx-success/20">
                        <div className="w-1.5 h-1.5 bg-nofx-success rounded-full animate-pulse" />
                        <span className="text-nofx-success text-[10px] font-mono uppercase tracking-wider">Live</span>
                    </div>
                </div>

                {/* Portfolio PnL Section */}
                <div className="p-4 border-b border-[rgba(26,24,19,0.14)]">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-nofx-text-muted text-xs font-mono uppercase tracking-wider">Portfolio PnL</span>
                        <div className="flex gap-1">
                            <button className="px-2 py-0.5 bg-nofx-gold/20 border border-nofx-gold/30 rounded text-[10px] text-nofx-gold font-mono">24H</button>
                            <button className="px-2 py-0.5 text-[10px] text-nofx-text-muted font-mono hover:text-nofx-text transition-colors">7D</button>
                            <button className="px-2 py-0.5 text-[10px] text-nofx-text-muted font-mono hover:text-nofx-text transition-colors">30D</button>
                        </div>
                    </div>
                    <div className="flex items-baseline gap-3">
                        <span className="text-3xl font-bold text-nofx-success font-mono tracking-tight">+$12,847.50</span>
                        <span className="text-nofx-success/80 text-sm font-mono">+8.42%</span>
                    </div>

                    {/* Chart Area */}
                    <div className="mt-4 h-16 rounded-lg overflow-hidden relative">
                        <svg className="w-full h-full" preserveAspectRatio="none" viewBox="0 0 400 64">
                            <defs>
                                <linearGradient id="chartGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" stopColor="#2E8B57" stopOpacity="0.2" />
                                    <stop offset="100%" stopColor="#2E8B57" stopOpacity="0" />
                                </linearGradient>
                            </defs>
                            <path
                                d="M0,56 C40,52 80,48 120,40 C160,32 200,28 240,24 C280,20 320,16 360,12 L400,8 L400,64 L0,64 Z"
                                fill="url(#chartGradient)"
                            />
                            <path
                                d="M0,56 C40,52 80,48 120,40 C160,32 200,28 240,24 C280,20 320,16 360,12 L400,8"
                                fill="none"
                                stroke="#2E8B57"
                                strokeWidth="1.5"
                            />
                        </svg>
                    </div>
                </div>

                {/* Metrics Row */}
                <div className="grid grid-cols-3 divide-x divide-[rgba(26,24,19,0.14)] border-b border-[rgba(26,24,19,0.14)]">
                    <div className="p-3 text-center">
                        <div className="text-nofx-text-muted text-[10px] font-mono uppercase tracking-wider mb-1">OI</div>
                        <div className="text-nofx-text font-bold font-mono">$847M</div>
                        <div className="text-nofx-success text-[10px] font-mono">↑ 2.1%</div>
                    </div>
                    <div className="p-3 text-center">
                        <div className="text-nofx-text-muted text-[10px] font-mono uppercase tracking-wider mb-1">Netflow</div>
                        <div className="text-nofx-success font-bold font-mono">+$124M</div>
                        <div className="text-nofx-text-muted text-[10px] font-mono">24h inflow</div>
                    </div>
                    <div className="p-3 text-center">
                        <div className="text-nofx-text-muted text-[10px] font-mono uppercase tracking-wider mb-1">L/S Ratio</div>
                        <div className="text-nofx-text font-bold font-mono">1.24</div>
                        <div className="flex gap-0.5 mt-1 px-2">
                            <div className="h-1 bg-nofx-success/60 rounded-l flex-[55]" />
                            <div className="h-1 bg-nofx-danger/60 rounded-r flex-[45]" />
                        </div>
                    </div>
                </div>

                {/* Order Book */}
                <div className="p-4 border-b border-[rgba(26,24,19,0.14)]">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-nofx-text text-xs font-mono uppercase tracking-wider">Order Book</span>
                        <span className="text-nofx-text-muted text-[10px] font-mono">Spread: <span className="text-nofx-gold">0.02%</span></span>
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
                                    <div className="absolute inset-0 bg-nofx-danger/10 rounded-sm" style={{ width: `${ask.depth}%` }} />
                                    <span className="relative text-nofx-danger font-mono">{ask.price}</span>
                                    <span className="relative text-nofx-text-muted font-mono">{ask.amount}</span>
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
                                    <div className="absolute inset-0 bg-nofx-success/10 rounded-sm" style={{ width: `${bid.depth}%` }} />
                                    <span className="relative text-nofx-success font-mono">{bid.price}</span>
                                    <span className="relative text-nofx-text-muted font-mono">{bid.amount}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                {/* Active Positions */}
                <div className="p-4">
                    <div className="flex items-center justify-between mb-3">
                        <span className="text-nofx-text text-xs font-mono uppercase tracking-wider">Positions</span>
                        <span className="text-nofx-success text-xs font-mono font-medium">+$12,847</span>
                    </div>
                    <div className="space-y-2">
                        {[
                            { coin: 'BTC', name: 'BTC-PERP', size: '0.5', profit: '+$6,420', percent: '+12.8%', color: '#F7931A' },
                            { coin: 'ETH', name: 'ETH-PERP', size: '3.2', profit: '+$4,127', percent: '+7.6%', color: '#627EEA' },
                            { coin: 'BNB', name: 'BNB-PERP', size: '8.5', profit: '+$2,300', percent: '+5.2%', color: '#F3BA2F' },
                        ].map((pos, i) => (
                            <div key={i} className="flex items-center justify-between py-2 px-2 rounded-lg bg-nofx-bg-deeper hover:bg-nofx-bg transition-colors">
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
                                        <div className="text-nofx-text text-sm font-mono">{pos.name}</div>
                                        <div className="flex items-center gap-2 text-[10px]">
                                            <span className="text-nofx-success bg-nofx-success/10 px-1.5 py-0.5 rounded font-mono">LONG</span>
                                            <span className="text-nofx-text-muted font-mono">{pos.size} {pos.coin}</span>
                                        </div>
                                    </div>
                                </div>
                                <div className="text-right">
                                    <div className="text-nofx-success font-mono font-medium">{pos.profit}</div>
                                    <div className="text-nofx-success/70 text-[10px] font-mono">{pos.percent}</div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                {/* Footer status bar */}
                <div className="px-4 py-2 bg-nofx-bg-deeper border-t border-[rgba(26,24,19,0.14)] flex items-center justify-between">
                    <div className="flex items-center gap-3 text-[10px] font-mono text-nofx-text-muted">
                        <span className="flex items-center gap-1">
                            <div className="w-1.5 h-1.5 bg-nofx-success rounded-full" />
                            Connected
                        </span>
                        <span>Latency: 12ms</span>
                    </div>
                    <div className="text-[10px] font-mono text-nofx-text-muted">
                        mainnet • v2.4.0
                    </div>
                </div>
            </div>
        </motion.div>
    )
}
