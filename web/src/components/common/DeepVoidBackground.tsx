import React from 'react'

interface DeepVoidBackgroundProps extends React.HTMLAttributes<HTMLDivElement> {
    children?: React.ReactNode
    className?: string
    disableAnimation?: boolean
}

export function DeepVoidBackground({ children, className = '', disableAnimation = false, ...props }: DeepVoidBackgroundProps) {
    return (
        <div className={`relative w-full min-h-screen bg-nofx-bg text-nofx-text overflow-hidden flex flex-col ${className}`} {...props}>
            {/* Background layers: neutralized to a plain cream surface for the light theme */}
            {disableAnimation ? (
                <>
                    <div className="absolute inset-0 pointer-events-none z-0 bg-nofx-bg"></div>
                    <div className="absolute inset-0 pointer-events-none z-0 opacity-[0.035] bg-[linear-gradient(to_right,rgba(26,24,19,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(26,24,19,0.08)_1px,transparent_1px)] bg-[size:36px_36px]"></div>
                </>
            ) : (
                <>
                    {/* Faint grid system on cream */}
                    <div className="absolute inset-0 pointer-events-none fixed z-0">
                        <div className="absolute inset-x-0 bottom-0 h-[50vh] bg-[linear-gradient(to_right,rgba(26,24,19,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(26,24,19,0.07)_1px,transparent_1px)] bg-[size:40px_40px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] opacity-50" style={{ transform: 'perspective(500px) rotateX(60deg) translateY(100px) scale(2)' }}></div>
                    </div>
                </>
            )}

            {/* Content Layer */}
            <div className="relative z-10 flex-1 flex flex-col h-full w-full">
                {children}
            </div>
        </div>
    )
}
