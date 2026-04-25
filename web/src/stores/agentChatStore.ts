import { create } from 'zustand'
import type { AgentMessage } from '../types/agent'

interface AgentChatStoreState {
  activeUserId?: string
  messages: AgentMessage[]
  loading: boolean
  hydrated: boolean
  setActiveUserId: (userId?: string) => void
  setMessages: (messages: AgentMessage[]) => void
  updateMessages: (
    updater: (messages: AgentMessage[]) => AgentMessage[]
  ) => void
  setLoading: (loading: boolean) => void
  setHydrated: (hydrated: boolean) => void
  resetForUser: (userId?: string, messages?: AgentMessage[]) => void
}

export const useAgentChatStore = create<AgentChatStoreState>((set) => ({
  activeUserId: undefined,
  messages: [],
  loading: false,
  hydrated: false,
  setActiveUserId: (userId) => set({ activeUserId: userId }),
  setMessages: (messages) => set({ messages }),
  updateMessages: (updater) =>
    set((state) => ({ messages: updater(state.messages) })),
  setLoading: (loading) => set({ loading }),
  setHydrated: (hydrated) => set({ hydrated }),
  resetForUser: (userId, messages = []) =>
    set({
      activeUserId: userId,
      messages,
      loading: false,
      hydrated: true,
    }),
}))
