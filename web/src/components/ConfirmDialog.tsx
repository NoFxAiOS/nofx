import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
} from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
} from './ui/alert-dialog'
import { setGlobalConfirm } from '../lib/notify'

interface ConfirmOptions {
  title?: string
  message: string
  okText?: string
  cancelText?: string
}

interface ConfirmDialogContextType {
  confirm: (options: ConfirmOptions) => Promise<boolean>
}

const ConfirmDialogContext = createContext<
  ConfirmDialogContextType | undefined
>(undefined)

export function useConfirmDialog() {
  const context = useContext(ConfirmDialogContext)
  if (!context) {
    throw new Error(
      'useConfirmDialog must be used within ConfirmDialogProvider'
    )
  }
  return context
}

interface ConfirmState {
  isOpen: boolean
  title?: string
  message: string
  okText: string
  cancelText: string
  resolve?: (value: boolean) => void
}

export function ConfirmDialogProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [state, setState] = useState<ConfirmState>({
    isOpen: false,
    message: '',
    okText: '确认',
    cancelText: '取消',
  })

  const confirm = useCallback((options: ConfirmOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      setState({
        isOpen: true,
        title: options.title,
        message: options.message,
        okText: options.okText || '确认',
        cancelText: options.cancelText || '取消',
        resolve,
      })
    })
  }, [])

  // 注册全局 confirm 函数
  useEffect(() => {
    setGlobalConfirm(confirm)
  }, [confirm])

  const handleClose = useCallback((result: boolean) => {
    setState((prev) => {
      prev.resolve?.(result)
      return {
        ...prev,
        isOpen: false,
      }
    })
  }, [])

  return (
    <ConfirmDialogContext.Provider value={{ confirm }}>
      {children}
      <AlertDialog
        open={state.isOpen}
        onOpenChange={(open) => !open && handleClose(false)}
      >
        <AlertDialogContent>
          <div className="flex flex-col gap-5 text-center">
            {state.title && (
              <AlertDialogTitle className="text-xl">
                {state.title}
              </AlertDialogTitle>
            )}
            <AlertDialogDescription className="text-[var(--text-primary)] text-base font-medium">
              {state.message}
            </AlertDialogDescription>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => handleClose(false)}>
              {state.cancelText}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => handleClose(true)}>
              {state.okText}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmDialogContext.Provider>
  )
}
