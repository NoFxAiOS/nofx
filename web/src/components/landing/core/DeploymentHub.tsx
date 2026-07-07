import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Terminal, Copy, Check, ChevronRight, Server, Command, Shield } from 'lucide-react'

export default function DeploymentHub() {
    const [copied, setCopied] = useState(false)
    const installCmd = "curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash"

    const handleCopy = () => {
        navigator.clipboard.writeText(installCmd)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <section className="py-24 bg-nofx-bg relative overflow-hidden border-t border-[rgba(26,24,19,0.14)]">
            {/* Background Grids */}
            <div className="absolute inset-0 bg-[linear-gradient(to_right,#1a181310_1px,transparent_1px),linear-gradient(to_bottom,#1a181310_1px,transparent_1px)] bg-[size:24px_24px]"></div>

            <div className="max-w-7xl mx-auto px-6 relative z-10">
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-16 items-center">

                    {/* Left Column: Context */}
                    <div className="space-y-8">
                        <div className="flex items-center gap-2 text-nofx-gold font-mono text-xs tracking-[0.2em] uppercase">
                            <Server className="w-4 h-4" /> System Deployment
                        </div>

                        <h2 className="text-4xl md:text-6xl font-black text-nofx-text leading-tight">
                            DEPLOY <span className="text-nofx-gold">INSTANTLY</span>
                        </h2>

                        <p className="text-nofx-text-muted text-lg leading-relaxed font-light">
                            One command on your laptop or any server installs
                            everything. Open the address it prints, create your
                            account, and the guided launch takes you to your
                            first AI trade in about five minutes — around $13
                            is enough to start.
                        </p>

                        {/* the first five minutes, in plain words */}
                        <ol className="space-y-2 pt-2 font-mono text-sm text-nofx-text-muted">
                            {[
                                'Register — the first account owns this instance.',
                                'Fund two small wallets: $1+ for AI fees, $12+ to trade with (guided, with QR codes).',
                                'Press Start — the AI trades on its own; stop it anytime.',
                            ].map((step, i) => (
                                <li key={i} className="flex gap-3">
                                    <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded border border-nofx-gold/30 bg-nofx-gold/10 text-[11px] font-bold text-nofx-gold">
                                        {i + 1}
                                    </span>
                                    <span>{step}</span>
                                </li>
                            ))}
                        </ol>

                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-4">
                            {[
                                { icon: Command, label: "One-Line Install", desc: "Docker handles every dependency" },
                                { icon: Shield, label: "Your Keys Stay Home", desc: "Runs on your machine, keys encrypted locally" }
                            ].map((item, i) => (
                                <div key={i} className="flex gap-4 items-start p-4 rounded bg-nofx-bg-lighter border border-[rgba(26,24,19,0.14)] hover:border-nofx-gold/30 transition-colors group">
                                    <div className="p-2 rounded bg-nofx-bg-deeper border border-[rgba(26,24,19,0.14)] text-nofx-gold group-hover:bg-nofx-gold/10 transition-colors">
                                        <item.icon className="w-5 h-5" />
                                    </div>
                                    <div>
                                        <h4 className="text-nofx-text font-bold font-mono text-sm mb-1">{item.label}</h4>
                                        <p className="text-nofx-text-muted text-xs">{item.desc}</p>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>

                    {/* Right Column: Terminal */}
                    <motion.div
                        initial={{ opacity: 0, x: 50 }}
                        whileInView={{ opacity: 1, x: 0 }}
                        viewport={{ once: true }}
                        className="relative"
                    >
                        {/* Glow effect */}
                        <div className="absolute -inset-1 bg-nofx-gold/10 rounded-xl blur-xl opacity-50"></div>

                        <div className="relative rounded-xl overflow-hidden bg-nofx-bg-lighter border border-[rgba(26,24,19,0.14)] shadow-lg">
                            {/* Terminal Header */}
                            <div className="flex items-center justify-between px-4 py-3 bg-nofx-bg-deeper border-b border-[rgba(26,24,19,0.14)]">
                                <div className="flex gap-2">
                                    <div className="w-3 h-3 rounded-full bg-nofx-danger/80"></div>
                                    <div className="w-3 h-3 rounded-full bg-nofx-gold/80"></div>
                                    <div className="w-3 h-3 rounded-full bg-nofx-success/80"></div>
                                </div>
                                <div className="text-[10px] font-mono text-nofx-text-muted flex items-center gap-1.5">
                                    <Terminal className="w-3 h-3" />
                                    root@nofx-os:~
                                </div>
                            </div>

                            {/* Terminal Content */}
                            <div className="p-8 font-mono text-sm md:text-base bg-nofx-bg-lighter min-h-[200px] flex flex-col justify-center">
                                <div className="mb-2 text-nofx-text-muted text-xs tracking-wide"># Initialize NoFX Core Protocol</div>
                                <div
                                    className="group relative flex items-start gap-3 p-4 rounded-lg bg-nofx-bg-deeper border border-[rgba(26,24,19,0.14)] hover:border-nofx-gold/50 cursor-pointer transition-all hover:bg-nofx-bg"
                                    onClick={handleCopy}
                                >
                                    <span className="text-nofx-gold mt-1"><ChevronRight className="w-4 h-4" /></span>
                                    <code className="text-nofx-text flex-1 break-all">
                                        {installCmd}
                                    </code>

                                    <div className="absolute right-4 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity">
                                        <AnimatePresence mode='wait'>
                                            {copied ? (
                                                <motion.div
                                                    initial={{ scale: 0.5, opacity: 0 }}
                                                    animate={{ scale: 1, opacity: 1 }}
                                                    exit={{ scale: 0.5, opacity: 0 }}
                                                    className="flex items-center gap-1 text-nofx-success bg-nofx-success/10 px-2 py-1 rounded text-xs font-bold"
                                                >
                                                    <Check className="w-3 h-3" />
                                                </motion.div>
                                            ) : (
                                                <div className="text-nofx-text-muted bg-nofx-bg-deeper p-1.5 rounded hover:text-nofx-text hover:bg-nofx-bg">
                                                    <Copy className="w-4 h-4" />
                                                </div>
                                            )}
                                        </AnimatePresence>
                                    </div>
                                </div>
                                <div className="mt-4 flex gap-2">
                                    <div className="w-2 h-4 bg-nofx-gold animate-pulse"></div>
                                </div>
                            </div>
                        </div>
                    </motion.div>
                </div>
            </div>
        </section>
    )
}
