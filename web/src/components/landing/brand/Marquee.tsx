import { useRef } from 'react'

export function Marquee({
    children,
    direction = 'left',
    speed = 30,
    className = '',
}: {
    children: React.ReactNode
    direction?: 'left' | 'right'
    speed?: number
    className?: string
}) {
    const scrollerRef = useRef<HTMLDivElement>(null)

    // Clone children to create seamless loop
    return (
        <div className={`overflow-hidden whitespace-nowrap ${className}`}>
            <div
                ref={scrollerRef}
                className="inline-flex w-max"
                style={{
                    animation: `marquee ${speed}s linear infinite ${direction === 'right' ? 'reverse' : 'normal'}`
                }}
            >
                <div className="flex shrink-0 min-w-full justify-around items-center">
                    {children}
                </div>
                <div className="flex shrink-0 min-w-full justify-around items-center" aria-hidden="true">
                    {children}
                </div>
            </div>
        </div>
    )
}
