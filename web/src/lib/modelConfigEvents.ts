const MODEL_CONFIGS_UPDATED_EVENT = 'nofx:model-configs-updated'

export function notifyModelConfigsUpdated() {
  if (typeof window === 'undefined') {
    return
  }

  window.dispatchEvent(new CustomEvent(MODEL_CONFIGS_UPDATED_EVENT))
}

export function subscribeModelConfigsUpdated(
  listener: () => void
): () => void {
  if (typeof window === 'undefined') {
    return () => {}
  }

  const handler: EventListener = () => listener()
  window.addEventListener(MODEL_CONFIGS_UPDATED_EVENT, handler)

  return () => {
    window.removeEventListener(MODEL_CONFIGS_UPDATED_EVENT, handler)
  }
}
