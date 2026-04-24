// Constants for AI model and provider configuration

export interface Claw402Model {
  id: string
  name: string
  provider: string
  desc: string
  icon: string
  price: number  // USD per call
  isNew?: boolean
}

export interface AIProviderConfig {
  defaultModel: string
  apiUrl: string
  apiName: string
}

export interface BlockrunModel {
  id: string
  name: string
  desc: string
}

// Get friendly AI model display name
export function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// Extract name part after underscore
export function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

export const DEFAULT_CLAW402_MODEL = 'deepseek-v4-flash'

// Models available through Claw402 (x402 USDC payment protocol)
export const CLAW402_MODELS: Claw402Model[] = [
  { id: 'deepseek-v4-flash', name: 'DeepSeek V4 Flash', provider: 'DeepSeek', desc: '$0.003/call', icon: '⚡', price: 0.003, isNew: true },
  { id: 'deepseek-v4-pro', name: 'DeepSeek V4 Pro', provider: 'DeepSeek', desc: '$0.01/call', icon: '🧠', price: 0.01, isNew: true },
  { id: 'deepseek', name: 'DeepSeek V3', provider: 'DeepSeek', desc: '$0.003/call', icon: '🔥', price: 0.003 },
  { id: 'deepseek-reasoner', name: 'DeepSeek R1', provider: 'DeepSeek', desc: '$0.005/call', icon: '🤔', price: 0.005 },
  { id: 'gpt-5-mini', name: 'GPT-5 Mini', provider: 'OpenAI', desc: '$0.005/call', icon: '🚀', price: 0.005 },
  { id: 'qwen-turbo', name: 'Qwen Turbo', provider: 'Alibaba', desc: '$0.002/call', icon: '⚡', price: 0.002 },
  { id: 'qwen-flash', name: 'Qwen Flash', provider: 'Alibaba', desc: '$0.002/call', icon: '⚡', price: 0.002 },
  { id: 'qwen-plus', name: 'Qwen Plus', provider: 'Alibaba', desc: '$0.005/call', icon: '✨', price: 0.005 },
  { id: 'kimi-k2.5', name: 'Kimi K2.5', provider: 'Moonshot', desc: '$0.008/call', icon: '🌙', price: 0.008 },
  { id: 'gpt-5.3', name: 'GPT-5.3', provider: 'OpenAI', desc: '$0.01/call', icon: '💡', price: 0.01 },
  { id: 'qwen-max', name: 'Qwen Max', provider: 'Alibaba', desc: '$0.01/call', icon: '🌟', price: 0.01 },
  { id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro', provider: 'Google', desc: '$0.03/call', icon: '💎', price: 0.03 },
  { id: 'gpt-5.4', name: 'GPT-5.4', provider: 'OpenAI', desc: '$0.05/call', icon: '⚡', price: 0.05 },
  { id: 'grok-4.1', name: 'Grok 4.1', provider: 'xAI', desc: '$0.06/call', icon: '⚡', price: 0.06 },
  { id: 'claude-opus', name: 'Claude Opus', provider: 'Anthropic', desc: '$0.12/call', icon: '🎯', price: 0.12 },
  { id: 'gpt-5.4-pro', name: 'GPT-5.4 Pro', provider: 'OpenAI', desc: '$0.50/call', icon: '🧠', price: 0.50 },
]

export const BLOCKRUN_MODELS: BlockrunModel[] = [
  {
    id: 'gpt-5.2',
    name: 'GPT-5.2',
    desc: 'Base wallet payment',
  },
  {
    id: 'claude-opus-4-6',
    name: 'Claude Opus 4.6',
    desc: 'Base wallet payment',
  },
  {
    id: 'gemini-3.1-pro',
    name: 'Gemini 3.1 Pro',
    desc: 'Base wallet payment',
  },
  {
    id: 'qwen3-max',
    name: 'Qwen 3 Max',
    desc: 'Base wallet payment',
  },
]

// AI Provider configuration - default models and API links
export const AI_PROVIDER_CONFIG: Record<string, AIProviderConfig> = {
  deepseek: {
    defaultModel: 'deepseek-chat',
    apiUrl: 'https://platform.deepseek.com/api_keys',
    apiName: 'DeepSeek',
  },
  qwen: {
    defaultModel: 'qwen3-max',
    apiUrl: 'https://dashscope.console.aliyun.com/apiKey',
    apiName: 'Alibaba Cloud',
  },
  openai: {
    defaultModel: 'gpt-5.2',
    apiUrl: 'https://platform.openai.com/api-keys',
    apiName: 'OpenAI',
  },
  claude: {
    defaultModel: 'claude-opus-4-6',
    apiUrl: 'https://console.anthropic.com/settings/keys',
    apiName: 'Anthropic',
  },
  gemini: {
    defaultModel: 'gemini-3-pro-preview',
    apiUrl: 'https://aistudio.google.com/app/apikey',
    apiName: 'Google AI Studio',
  },
  grok: {
    defaultModel: 'grok-3-latest',
    apiUrl: 'https://console.x.ai/',
    apiName: 'xAI',
  },
  kimi: {
    defaultModel: 'moonshot-v1-auto',
    apiUrl: 'https://platform.moonshot.ai/console/api-keys',
    apiName: 'Moonshot',
  },
  minimax: {
    defaultModel: 'MiniMax-M2.7',
    apiUrl: 'https://platform.minimax.io',
    apiName: 'MiniMax',
  },
  claw402: {
    defaultModel: DEFAULT_CLAW402_MODEL,
    apiUrl: 'https://claw402.ai',
    apiName: 'Claw402',
  },
}

// Helper function to get exchange display name from exchange ID (UUID)
export function getExchangeDisplayName(exchangeId: string | undefined, exchanges: { id: string; exchange_type?: string; name: string; account_name?: string }[]): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges.find(e => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...' // Show truncated UUID if not found
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name ? `${typeName} - ${exchange.account_name}` : typeName
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
export function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
export function getWalletAddress(exchange: { exchange_type?: string; hyperliquidWalletAddr?: string; lighterWalletAddr?: string; asterSigner?: string } | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
export function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}
