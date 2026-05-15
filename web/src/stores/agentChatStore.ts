import { create } from 'zustand'
import type { AgentMessage } from '../types/agent'

interface AgentChatStoreState {
  activeUserId?: string
  messages: AgentMessage[]
  draftText: string
  loading: boolean
  hydrated: boolean
  setActiveUserId: (userId?: string) => void
  setMessages: (messages: AgentMessage[]) => void
  setDraftText: (draftText: string) => void
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
  draftText: '',
  loading: false,
  hydrated: false,
  setActiveUserId: (userId) => set({ activeUserId: userId }),
  setMessages: (messages) => set({ messages }),
  setDraftText: (draftText) => set({ draftText }),
  updateMessages: (updater) =>
    set((state) => ({ messages: updater(state.messages) })),
  setLoading: (loading) => set({ loading }),
  setHydrated: (hydrated) => set({ hydrated }),
  resetForUser: (userId, messages = []) =>
    set({
      activeUserId: userId,
      messages,
      draftText: '',
      loading: false,
      hydrated: true,
    }),
}))
