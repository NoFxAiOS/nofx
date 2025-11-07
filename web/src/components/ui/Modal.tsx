import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { ReactNode } from 'react'

export interface ModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
  size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
}

export interface ModalContentProps {
  children: ReactNode
  className?: string
}

export interface ModalHeaderProps {
  children: ReactNode
  onClose?: () => void
  showCloseButton?: boolean
  className?: string
}

export interface ModalBodyProps {
  children: ReactNode
  className?: string
}

export interface ModalFooterProps {
  children: ReactNode
  className?: string
}

const sizeClasses = {
  sm: 'max-w-md',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-3xl',
  full: 'max-w-7xl',
}

/**
 * Modal 主容器
 * 使用 Radix UI Dialog 实现，自动处理焦点管理、滚动锁定、可访问性
 */
export function Modal({
  open,
  onOpenChange,
  children,
  size = 'md',
}: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <Dialog.Content
          className={`fixed left-[50%] top-[50%] z-50 w-full ${sizeClasses[size]} translate-x-[-50%] translate-y-[-50%] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]`}
          style={{
            maxHeight: 'calc(100vh - 4rem)',
          }}
        >
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

/**
 * Modal 内容包装器
 * 提供统一的背景、边框、圆角等样式
 */
export function ModalContent({ children, className = '' }: ModalContentProps) {
  return (
    <div
      className={`flex flex-col bg-[#1E2329] border border-[#2B3139] rounded-xl shadow-2xl mx-4 my-8 ${className}`}
      style={{ maxHeight: 'calc(100vh - 4rem)' }}
    >
      {children}
    </div>
  )
}

/**
 * Modal 头部
 * 包含标题和关闭按钮
 */
export function ModalHeader({
  children,
  onClose,
  showCloseButton = true,
  className = '',
}: ModalHeaderProps) {
  return (
    <div
      className={`flex items-center justify-between p-6 pb-4 border-b border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky top-0 z-10 rounded-t-xl ${className}`}
    >
      <div className="flex-1">{children}</div>
      {showCloseButton && (
        <Dialog.Close asChild>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center ml-4"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </Dialog.Close>
      )}
    </div>
  )
}

/**
 * Modal 主体内容
 * 可滚动区域
 */
export function ModalBody({ children, className = '' }: ModalBodyProps) {
  return (
    <div
      className={`p-6 overflow-y-auto ${className}`}
      style={{ maxHeight: 'calc(100vh - 16rem)' }}
    >
      {children}
    </div>
  )
}

/**
 * Modal 底部
 * 通常放置操作按钮
 */
export function ModalFooter({ children, className = '' }: ModalFooterProps) {
  return (
    <div
      className={`flex justify-end gap-3 p-6 pt-4 border-t border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky bottom-0 z-10 rounded-b-xl ${className}`}
    >
      {children}
    </div>
  )
}

// 导出 Dialog.Close 用于自定义关闭按钮
export const ModalClose = Dialog.Close

// 导出整个命名空间方便使用
Modal.Content = ModalContent
Modal.Header = ModalHeader
Modal.Body = ModalBody
Modal.Footer = ModalFooter
Modal.Close = ModalClose
