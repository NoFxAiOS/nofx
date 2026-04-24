export const LEGACY_AGENT_CHAT_STORAGE_KEY = 'nofxi-agent-chat'

export function normalizeStorageUserId(value: unknown): string | undefined {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed || undefined
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  return undefined
}

export function chatStorageKey(userId?: string) {
  return `nofxi-agent-chat:${userId || 'guest'}`
}

export function getStoredAuthUserId(storage: Storage = window.localStorage) {
  try {
    const raw = storage.getItem('auth_user')
    if (!raw) return undefined
    const parsed = JSON.parse(raw)
    return normalizeStorageUserId(parsed?.id)
  } catch {
    return undefined
  }
}

function loadMessagesFromKey<T>(storage: Storage, key: string): T[] {
  try {
    const raw = storage.getItem(key)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function candidateStorageKeys(userId?: string): string[] {
  const keys = [chatStorageKey(userId)]
  if (userId) {
    keys.push(chatStorageKey('guest'))
  }
  keys.push(LEGACY_AGENT_CHAT_STORAGE_KEY)
  return [...new Set(keys)]
}

export function loadAgentMessages<T>(storage: Storage, userId?: string) {
  const keys = candidateStorageKeys(userId)
  for (const key of keys) {
    const messages = loadMessagesFromKey<T>(storage, key)
    if (messages.length > 0) {
      return { messages, sourceKey: key }
    }
  }
  return { messages: [] as T[], sourceKey: chatStorageKey(userId) }
}

export function persistAgentMessages<T>(
  storage: Storage,
  userId: string | undefined,
  messages: T[]
) {
  storage.setItem(chatStorageKey(userId), JSON.stringify(messages))
}

export function prepareAgentMessagesForPersistence<
  T extends { streaming?: boolean; text?: string; steps?: unknown[]; time?: string }
>(messages: T[]): T[] {
  return messages.map((message) => {
    if (!message.streaming) {
      return message
    }
    return {
      ...message,
      // Persist the latest visible snapshot, but don't restore it as an
      // actively streaming message after the user leaves and comes back.
      streaming: false,
      time: message.time || '',
    }
  })
}

export function migrateAgentMessages(storage: Storage, userId?: string) {
  if (!userId) return

  const targetKey = chatStorageKey(userId)
  const targetMessages = loadMessagesFromKey(storage, targetKey)
  if (targetMessages.length > 0) return

  for (const sourceKey of [chatStorageKey('guest'), LEGACY_AGENT_CHAT_STORAGE_KEY]) {
    const sourceMessages = loadMessagesFromKey(storage, sourceKey)
    if (sourceMessages.length === 0) continue
    storage.setItem(targetKey, JSON.stringify(sourceMessages))
    return
  }
}

export function clearAgentMessages(storage: Storage, userId?: string) {
  for (const key of candidateStorageKeys(userId)) {
    storage.removeItem(key)
  }
}
