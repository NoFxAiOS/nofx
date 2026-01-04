import React from 'react'

interface DeepVoidBackgroundProps extends React.HTMLAttributes<HTMLDivElement> {
    children?: React.ReactNode
    className?: string
    disableAnimation?: boolean
}

export function DeepVoidBackground({ children, className = '', disableAnimation = false, ...props }: DeepVoidBackgroundProps) {
    return (
        <div className={`relative w-full min-h-screen bg-nofx-bg text-nofx-text overflow-hidden flex flex-col ${className}`} {...props}>
            {/* BACKGROUND LAYERS */}

            {/* 1. Grain/Noise Texture */}
            <div className="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-20 mix-blend-soft-light pointer-events-none fixed z-0"></div>

            {/* 2. Grid System */}
            <div className="absolute inset-0 pointer-events-none fixed z-0">
                <div className="absolute inset-x-0 bottom-0 h-[50vh] bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:40px_40px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] opacity-50" style={{ transform: 'perspective(500px) rotateX(60deg) translateY(100px) scale(2)' }}></div>
                <div className="absolute inset-0 bg-grid-pattern opacity-[0.03]"></div>
            </div>

            {/* 3. Ambient Glow Spots */}
            <div className="absolute inset-0 overflow-hidden pointer-events-none fixed z-0">
                <div className="absolute top-[-10%] left-[-10%] w-[40vw] h-[40vw] bg-nofx-gold/10 rounded-full blur-[120px] mix-blend-screen animate-pulse-slow"></div>
                <div className="absolute bottom-[-10%] right-[-10%] w-[40vw] h-[40vw] bg-nofx-accent/5 rounded-full blur-[120px] mix-blend-screen animate-pulse-slow" style={{ animationDelay: '2s' }}></div>
            </div>

            {/* 4. CRT/Scanline Overlay */}
            <div className="absolute inset-0 pointer-events-none fixed z-[9999] opacity-40">
                <div className="absolute inset-0 bg-[linear-gradient(rgba(18,16,16,0)_50%,rgba(0,0,0,0.25)_50%),linear-gradient(90deg,rgba(255,0,0,0.06),rgba(0,255,0,0.02),rgba(0,0,255,0.06))] bg-[length:100%_4px,3px_100%] pointer-events-none"></div>
            </div>

            {/* Content Layer */}
            <div className="relative z-10 flex-1 flex flex-col h-full w-full">
                {children}
            </div>
        </div>
    )
}
