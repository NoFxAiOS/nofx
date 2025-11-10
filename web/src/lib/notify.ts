/**
 * 轻量确认对话封装，保持前端构建稳定，不依赖复杂 UI。
 * 在支持浏览器环境时优先使用原生 confirm，以获得同步确认体验；
 * 如需更好交互，可后续替换为 sonner 的自定义弹层实现。
 */

export interface ConfirmOptions {
  message?: string
  okText?: string
  cancelText?: string
}

/**
 * 返回 Promise<boolean> 以便在调用处使用 `await`。
 */
export function confirmToast(
  message: string,
  _options: ConfirmOptions = {}
): Promise<boolean> {
  // 最小可用实现：使用原生 confirm，避免引入 TSX/DOM 复杂度
  return new Promise((resolve) => {
    try {
      // 浏览器环境

      const ok = typeof window !== 'undefined' && window.confirm(message)
      resolve(!!ok)
    } catch {
      // 非浏览器环境（如 SSR/测试），默认返回 false，避免误删
      resolve(false)
    }
  })
}

export default { confirmToast }
