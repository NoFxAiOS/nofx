import { toast } from 'sonner'
import type { ReactNode } from 'react'

export interface ConfirmOptions {
  title?: string
  message?: string
  okText?: string
  cancelText?: string
}

// 全局 confirm 函数的引用，将在 ConfirmDialogProvider 中设置
let globalConfirm:
  | ((options: ConfirmOptions & { message: string }) => Promise<boolean>)
  | null = null

export function setGlobalConfirm(
  confirmFn: (options: ConfirmOptions & { message: string }) => Promise<boolean>
) {
  globalConfirm = confirmFn
}

// 确认对话框函数，使用 shadcn AlertDialog
export function confirmToast(
  message: string,
  options: ConfirmOptions = {}
): Promise<boolean> {
  if (!globalConfirm) {
    console.error('ConfirmDialogProvider not initialized')
    return Promise.resolve(false)
  }

  return globalConfirm({
    message,
    ...options,
  })
}

// 统一通知封装，避免组件直接依赖 sonner
type Message = string | ReactNode

function message(msg: Message, options?: Parameters<typeof toast>[1]) {
  return toast(msg as any, options)
}

function success(msg: Message, options?: Parameters<typeof toast.success>[1]) {
  return toast.success(msg as any, options)
}

function error(msg: Message, options?: Parameters<typeof toast.error>[1]) {
  return toast.error(msg as any, options)
}

function info(msg: Message, options?: Parameters<typeof toast.info>[1]) {
  return toast.info?.(msg as any, options) ?? toast(msg as any, options)
}

function warning(msg: Message, options?: Parameters<typeof toast.warning>[1]) {
  return toast.warning?.(msg as any, options) ?? toast(msg as any, options)
}

function custom(
  renderer: Parameters<typeof toast.custom>[0],
  options?: Parameters<typeof toast.custom>[1]
) {
  return toast.custom(renderer, options)
}

function dismiss(id?: string | number) {
  return toast.dismiss(id as any)
}

function promise<T>(p: Promise<T> | (() => Promise<T>), msgs: any) {
  return toast.promise<T>(p as any, msgs as any)
}

export const notify = {
  message,
  success,
  error,
  info,
  warning,
  custom,
  dismiss,
  promise,
}

export default { confirmToast, notify }
