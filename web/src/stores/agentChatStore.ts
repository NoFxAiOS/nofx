import { create } from 'zustand'

export interface AgentStep {
  id: string
  label: string
  status: 'planning' | 'pending' | 'running' | 'completed' | 'replanned'
  detail?: string
}

export interface AgentMessage {
  id: string
  role: 'user' | 'bot'
  text: string
  time: string
  streaming?: boolean
  steps?: AgentStep[]
}

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
