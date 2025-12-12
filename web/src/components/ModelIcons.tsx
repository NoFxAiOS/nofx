interface IconProps {
  width?: number
  height?: number
  className?: string
}

// AI model colors for fallback display
const MODEL_COLORS: Record<string, string> = {
  deepseek: '#4A90E2',
  qwen: '#9B59B6',
  claude: '#D97757',
  kimi: '#6366F1',
  gemini: '#4285F4',
  grok: '#000000',
  openai: '#10A37F',
}

// 获取AI模型图标的函数
export const getModelIcon = (modelType: string, props: IconProps = {}) => {
  // 支持完整ID或类型名
  const type = modelType.includes('_') ? modelType.split('_').pop() : modelType

  let iconPath: string | null = null

  switch (type) {
    case 'deepseek':
      iconPath = '/icons/deepseek.svg'
      break
    case 'qwen':
      iconPath = '/icons/qwen.svg'
      break
    case 'claude':
      iconPath = '/icons/claude.svg'
      break
    case 'kimi':
      iconPath = '/icons/kimi.svg'
      break
    case 'gemini':
      iconPath = '/icons/gemini.svg'
      break
    case 'grok':
      iconPath = '/icons/grok.svg'
      break
    case 'openai':
      iconPath = '/icons/openai.svg'
      break
    default:
      return null
  }

  return (
    <img
      src={iconPath}
      alt={`${type} icon`}
      width={props.width || 24}
      height={props.height || 24}
      className={props.className}
    />
  )
}

// 获取模型颜色（用于没有图标时的fallback）
export const getModelColor = (modelType: string): string => {
  const type = modelType.includes('_') ? modelType.split('_').pop() : modelType
  return MODEL_COLORS[type || ''] || '#60a5fa'
}
