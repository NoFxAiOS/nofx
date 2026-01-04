import { DeepVoidBackground } from '../components/DeepVoidBackground'
import { AlertCircle, Home } from 'lucide-react'

export function PageNotFound() {
    return (
        <DeepVoidBackground className="flex items-center justify-center text-center p-4">
            <div className="bg-nofx-bg border border-nofx-gold/20 p-8 rounded-lg max-w-md w-full relative overflow-hidden group">

                {/* Background Grid inside Card */}
                <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808008_1px,transparent_1px),linear-gradient(to_bottom,#80808008_1px,transparent_1px)] bg-[size:16px_16px] pointer-events-none"></div>

                <div className="relative z-10 flex flex-col items-center gap-6">
                    <div className="relative">
                        <div className="absolute inset-0 bg-red-500/20 blur-xl animate-pulse"></div>
                        <AlertCircle size={64} className="text-nofx-danger relative z-10" />
                    </div>

                    <div className="space-y-2">
                        <h1 className="text-4xl font-bold font-mono tracking-tighter text-white">
                            404
                        </h1>
                        <div className="text-xs uppercase tracking-[0.3em] text-nofx-danger font-mono border-b border-nofx-danger/30 pb-2 inline-block">
                            SIGNAL_LOST
                        </div>
                    </div>

                    <p className="text-sm text-nofx-text-muted font-mono leading-relaxed">
                        The requested coordinates do not exist in the current sector. The page may have been moved, deleted, or never existed in this timeline.
                    </p>

                    <a
                        href="/"
                        className="flex items-center gap-2 px-6 py-3 bg-nofx-gold text-black font-bold text-sm uppercase tracking-widest rounded hover:bg-yellow-400 transition-all shadow-neon hover:shadow-[0_0_20px_rgba(240,185,11,0.4)] group mt-4"
                    >
                        <Home size={16} />
                        <span>RETURN_BASE</span>
                        <span className="opacity-0 group-hover:opacity-100 transition-opacity -ml-2 group-hover:ml-0">-&gt;</span>
                    </a>
                </div>
            </div>
        </DeepVoidBackground>
    )
}
