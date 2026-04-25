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
