import { describe, expect, it } from 'vitest'
import {
  LEGACY_AGENT_CHAT_STORAGE_KEY,
  chatStorageKey,
  clearAgentMessages,
  loadAgentMessages,
  migrateAgentMessages,
  normalizeStorageUserId,
  prepareAgentMessagesForPersistence,
} from './agentChatStorage'

function createStorage(): Storage {
  const data = new Map<string, string>()
  return {
    get length() {
      return data.size
    },
    clear() {
      data.clear()
    },
    getItem(key: string) {
      return data.has(key) ? data.get(key)! : null
    },
    key(index: number) {
      return Array.from(data.keys())[index] ?? null
    },
    removeItem(key: string) {
      data.delete(key)
    },
    setItem(key: string, value: string) {
      data.set(key, value)
    },
  }
}

describe('agentChatStorage', () => {
  it('normalizes string and numeric user ids', () => {
    expect(normalizeStorageUserId(' user-1 ')).toBe('user-1')
    expect(normalizeStorageUserId(42)).toBe('42')
    expect(normalizeStorageUserId('')).toBeUndefined()
  })

  it('falls back to guest history for a logged-in user when user history is empty', () => {
    const storage = createStorage()
    const guestMessages = [{ id: '1', text: 'hello' }]
    storage.setItem(chatStorageKey('guest'), JSON.stringify(guestMessages))

    expect(loadAgentMessages(storage, 'user-1')).toEqual({
      messages: guestMessages,
      sourceKey: chatStorageKey('guest'),
    })
  })

  it('migrates guest history into the user-specific key after login', () => {
    const storage = createStorage()
    const guestMessages = [{ id: '1', text: 'hello' }]
    storage.setItem(chatStorageKey('guest'), JSON.stringify(guestMessages))

    migrateAgentMessages(storage, 'user-1')

    expect(storage.getItem(chatStorageKey('user-1'))).toBe(JSON.stringify(guestMessages))
  })

  it('clears primary and fallback chat storage keys', () => {
    const storage = createStorage()
    storage.setItem(chatStorageKey('user-1'), JSON.stringify([{ id: '1' }]))
    storage.setItem(chatStorageKey('guest'), JSON.stringify([{ id: '2' }]))
    storage.setItem(LEGACY_AGENT_CHAT_STORAGE_KEY, JSON.stringify([{ id: '3' }]))

    clearAgentMessages(storage, 'user-1')

    expect(storage.getItem(chatStorageKey('user-1'))).toBeNull()
    expect(storage.getItem(chatStorageKey('guest'))).toBeNull()
    expect(storage.getItem(LEGACY_AGENT_CHAT_STORAGE_KEY)).toBeNull()
  })

  it('persists streaming messages as non-streaming snapshots', () => {
    const messages = [
      { id: '1', text: 'hello', streaming: true, steps: [{ id: 's1' }] },
      { id: '2', text: 'done', streaming: false },
    ]

    expect(prepareAgentMessagesForPersistence(messages)).toEqual([
      { id: '1', text: 'hello', streaming: false, steps: [{ id: 's1' }], time: '' },
      { id: '2', text: 'done', streaming: false },
    ])
  })
})
