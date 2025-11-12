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
          'bg-[var(--brand-black)] border border-[var(--panel-border)]',
          'text-[var(--brand-light-gray)] focus:outline-none',
          className
        )}
        {...props}
      />
    )
  }
)

Input.displayName = 'Input'
