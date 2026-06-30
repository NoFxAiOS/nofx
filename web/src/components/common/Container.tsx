import { ReactNode, CSSProperties } from 'react'

interface ContainerProps {
  children: ReactNode
  className?: string
  as?: 'div' | 'main' | 'header' | 'section'
  style?: CSSProperties
  /** Whether to fill the full width (removes max-width) */
  fluid?: boolean
  /** Whether to remove horizontal padding */
  noPadding?: boolean
  /** Custom max-width class (default max-w-[1920px]) */
  maxWidthClass?: string
}

/**
 * Unified container component that ensures all page elements use a consistent max width and padding
 * - max-width: 1920px
 * - padding: 24px (mobile) -> 32px (tablet) -> 48px (desktop)
 */
export function Container({
  children,
  className = '',
  as: Component = 'div',
  style,
  fluid = false,
  noPadding = false,
  maxWidthClass = 'max-w-[1920px]',
}: ContainerProps) {
  const maxWidth = fluid ? 'w-full' : maxWidthClass
  const padding = noPadding ? 'px-0' : 'px-6 sm:px-8 lg:px-12'
  return (
    <Component
      className={`${maxWidth} mx-auto ${padding} ${className}`}
      style={style}
    >
      {children}
    </Component>
  )
}
