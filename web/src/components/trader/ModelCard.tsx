import { Check } from 'lucide-react'
import type { AIModel } from '../../types'
import { getModelIcon } from '../common/ModelIcons'
import { getShortName } from './model-constants'

interface ModelCardProps {
  model: AIModel
  selected: boolean
  onClick: () => void
  configured?: boolean
}

export function ModelCard({ model, selected, onClick, configured }: ModelCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col items-center gap-2 p-4 rounded-xl transition-all hover:scale-105"
      style={{
        background: selected ? 'rgba(139, 92, 246, 0.15)' : '#0B0E11',
        border: selected ? '2px solid #8B5CF6' : '2px solid #2B3139',
      }}
    >
      <div className="relative">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black border border-white/10">
          {getModelIcon(model.provider || model.id, { width: 32, height: 32 }) || (
            <span className="text-lg font-bold" style={{ color: '#A78BFA' }}>{model.name[0]}</span>
          )}
        </div>
        {selected && (
          <div
            className="absolute -top-1 -right-1 w-5 h-5 rounded-full flex items-center justify-center"
            style={{ background: '#0ECB81' }}
          >
            <Check className="w-3 h-3 text-black" />
          </div>
        )}
        {configured && !selected && (
          <div
            className="absolute -top-1 -right-1 w-4 h-4 rounded-full flex items-center justify-center"
            style={{ background: '#F0B90B' }}
          >
            <Check className="w-2.5 h-2.5 text-black" />
          </div>
        )}
      </div>
      <span className="text-sm font-semibold" style={{ color: '#EAECEF' }}>
        {getShortName(model.name)}
      </span>
      <span
        className="text-[10px] px-2 py-0.5 rounded-full uppercase tracking-wide"
        style={{ background: 'rgba(139, 92, 246, 0.2)', color: '#A78BFA' }}
      >
        {model.provider}
      </span>
    </button>
  )
}
