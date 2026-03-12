import { notify } from './notify'

/**
 * Copy text to clipboard and show a toast notification.
 */
export async function copyWithToast(text: string, successMsg = 'Copied') {
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      // Fallback: create temporary textarea for copy
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
    notify.error('Copy failed')
    return false
  }
}

export default { copyWithToast }
