import { notify } from './notify'

/**
 * 复制文本到剪贴板，并显示轻量提示。
 */
export async function copyWithToast(text: string, successMsg = '已复制') {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      // 兼容降级：创建临时文本域执行复制
      const el = document.createElement('textarea')
      el.value = text
      el.style.position = 'fixed'
      el.style.left = '-9999px'
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    notify.success(successMsg)
    return true
  } catch (err) {
    console.error('Clipboard copy failed:', err)
    notify.error('复制失败')
    return false
  }
}

export default { copyWithToast }
