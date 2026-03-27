import { useRef, useState, useLayoutEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown } from 'lucide-react'
import { cn } from '../../lib/cn'

export interface SelectOption {
  value: string | number
  label: string
}

interface NofxSelectProps {
  value: string | number
  onChange: (value: string) => void
  options: SelectOption[]
  disabled?: boolean
  className?: string
  style?: React.CSSProperties
}

export function NofxSelect({ value, onChange, options, disabled, className, style }: NofxSelectProps) {
  const [open, setOpen] = useState(false)
  const triggerRef = useRef<HTMLDivElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ top: 0, left: 0, width: 0 })
  const selected = options.find(o => String(o.value) === String(value))

  const updatePos = useCallback(() => {
    if (!triggerRef.current) return
    const rect = triggerRef.current.getBoundingClientRect()
    setPos({ top: rect.bottom + 4, left: rect.left, width: rect.width })
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    updatePos()
    const handleClose = (e: MouseEvent) => {
      const target = e.target as Node
      if (triggerRef.current?.contains(target)) return
      if (dropdownRef.current?.contains(target)) return
      setOpen(false)
    }
    const handleScroll = (e: Event) => {
      if (dropdownRef.current?.contains(e.target as Node)) return
      setOpen(false)
    }
    document.addEventListener('mousedown', handleClose)
    window.addEventListener('scroll', handleScroll, true)
    return () => {
      document.removeEventListener('mousedown', handleClose)
      window.removeEventListener('scroll', handleScroll, true)
    }
  }, [open, updatePos])

  return (
    <div
      ref={triggerRef}
      className={cn('relative', className)}
      style={style}
    >
      <div
        className={cn(
          'flex items-center justify-between gap-1.5 w-full h-full cursor-pointer',
          disabled && 'opacity-50 cursor-not-allowed',
        )}
        onClick={(e) => {
          e.stopPropagation()
          if (!disabled) setOpen(!open)
        }}
      >
        <span className="truncate">{selected?.label ?? String(value)}</span>
        <ChevronDown className={cn('w-3 h-3 shrink-0 opacity-50 transition-transform', open && 'rotate-180')} />
      </div>
      {open && createPortal(
        <div
          ref={dropdownRef}
          className="fixed z-[9999] rounded border border-[#2B3139] bg-[#0B0E11] shadow-xl shadow-black/50 max-h-60 overflow-y-auto"
          style={{ top: pos.top, left: pos.left, minWidth: pos.width }}
        >
          {options.map((opt) => (
            <div
              key={opt.value}
              className={cn(
                'px-3 py-1.5 text-sm cursor-pointer transition-colors whitespace-nowrap',
                String(opt.value) === String(value)
                  ? 'bg-[#F0B90B]/10 text-[#F0B90B]'
                  : 'text-[#EAECEF] hover:bg-[#1E2329]',
              )}
              onClick={(e) => {
                e.stopPropagation()
                onChange(String(opt.value))
                setOpen(false)
              }}
            >
              {opt.label}
            </div>
          ))}
        </div>,
        document.body,
      )}
    </div>
  )
}
