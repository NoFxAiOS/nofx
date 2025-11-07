import * as SelectPrimitive from '@radix-ui/react-select'
import { Check, ChevronDown } from 'lucide-react'
import { ReactNode } from 'react'

export interface SelectProps {
  value: string
  onValueChange: (value: string) => void
  children: ReactNode
  placeholder?: string
  disabled?: boolean
  required?: boolean
}

export interface SelectItemProps {
  value: string
  children: ReactNode
  disabled?: boolean
}

/**
 * Select 主容器
 * 使用 Radix UI Select 实现
 */
export function Select({
  value,
  onValueChange,
  children,
  placeholder,
  disabled,
  required,
}: SelectProps) {
  return (
    <SelectPrimitive.Root
      value={value}
      onValueChange={onValueChange}
      disabled={disabled}
      required={required}
    >
      <SelectPrimitive.Trigger
        className="flex w-full items-center justify-between rounded px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-[#F0B90B] focus:ring-offset-2 focus:ring-offset-[#1E2329] disabled:cursor-not-allowed disabled:opacity-50"
        style={{
          background: '#0B0E11',
          border: '1px solid #2B3139',
          color: value ? '#EAECEF' : '#848E9C',
        }}
      >
        <SelectPrimitive.Value placeholder={placeholder} />
        <SelectPrimitive.Icon>
          <ChevronDown className="h-4 w-4 opacity-50" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          className="relative z-50 min-w-[8rem] overflow-hidden rounded-md border shadow-md"
          style={{
            background: '#1E2329',
            border: '1px solid #2B3139',
          }}
          position="popper"
          sideOffset={4}
        >
          <SelectPrimitive.Viewport className="p-1">
            {children}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  )
}

/**
 * Select 选项
 */
export function SelectItem({ value, children, disabled }: SelectItemProps) {
  return (
    <SelectPrimitive.Item
      value={value}
      disabled={disabled}
      className="relative flex w-full cursor-pointer select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none transition-colors focus:bg-[#2B3139] focus:text-[#EAECEF] data-[disabled]:pointer-events-none data-[disabled]:opacity-50"
      style={{
        color: '#EAECEF',
      }}
    >
      <span className="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
        <SelectPrimitive.ItemIndicator>
          <Check className="h-4 w-4" style={{ color: '#F0B90B' }} />
        </SelectPrimitive.ItemIndicator>
      </span>
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  )
}

// 导出整个命名空间方便使用
Select.Item = SelectItem
