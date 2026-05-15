import * as React from 'react'
import { cn } from '../../lib/cn'

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = 'text', ...props }, ref) => {
    return (
      <input
        ref={ref}
        type={type}
        className={cn(
          'flex h-10 w-full rounded px-3 py-2 text-sm',
          'bg-[var(--panel-bg)] border border-[var(--panel-border)]',
          'text-[var(--text-primary)] focus:outline-none focus:border-[var(--binance-yellow)] focus:ring-1 focus:ring-[rgba(240,185,11,0.15)]',
          className
        )}
        {...props}
      />
    )
  }
)

Input.displayName = 'Input'
