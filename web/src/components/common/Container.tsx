import { ReactNode, CSSProperties } from 'react'

interface ContainerProps {
  children: ReactNode
  className?: string
  as?: 'div' | 'main' | 'header' | 'section'
  style?: CSSProperties
  /** 是否充满宽度（取消 max-width） */
  fluid?: boolean
  /** 是否取消水平内边距 */
  noPadding?: boolean
  /** 自定义最大宽度类（默认 max-w-[1920px]） */
  maxWidthClass?: string
}

/**
 * 统一的容器组件，确保所有页面元素使用一致的最大宽度和内边距
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
