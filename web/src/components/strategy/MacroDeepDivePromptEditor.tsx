import { useState } from 'react'
import { ChevronDown, ChevronRight, RotateCcw, FileText } from 'lucide-react'
import type { MacroPromptSectionsConfig, DeepDivePromptSectionsConfig } from '../../types'

interface MacroDeepDivePromptEditorProps {
  macroSections: MacroPromptSectionsConfig | undefined
  deepDiveSections: DeepDivePromptSectionsConfig | undefined
  onMacroChange: (config: MacroPromptSectionsConfig) => void
  onDeepDiveChange: (config: DeepDivePromptSectionsConfig) => void
  disabled?: boolean
  language: string
}

const defaultMacroSections: Required<MacroPromptSectionsConfig> = {
  role_context: `# Role & Context

You are a macro analyst for crypto markets. Focus on trend, risk level, and which symbols deserve a deep-dive. Use the market brief (indices, OI, NetFlow, price ranking, positions) to decide.`,
  output_guidance: `# Output & Symbol Guidance

Prioritize symbols with strong OI/flow alignment and clear momentum. Prefer quality over quantity. Must include every open position symbol in symbols_for_deep_dive.`,
}

const defaultDeepDiveSections: Required<DeepDivePromptSectionsConfig> = {
  symbol_rules: `# Symbol-Level Rules

Apply strict risk-reward. Prefer conservative sizing for altcoins. Use the macro focus_reason when evaluating entries.`,
}

export function MacroDeepDivePromptEditor({
  macroSections,
  deepDiveSections,
  onMacroChange,
  onDeepDiveChange,
  disabled,
  language,
}: MacroDeepDivePromptEditorProps) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({
    macro_role_context: false,
    macro_output_guidance: false,
    deep_dive_symbol_rules: false,
  })

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      macroSectionIntro: {
        zh: '自定义宏观分析员行为和选币逻辑，以及每符号深度分析的规则。Macro 输出格式（trend、risk_level、symbols_for_deep_dive）固定；深度分析沿用主策略的决策格式。',
        en: 'Customize macro analyst behavior and symbol selection, and per-symbol deep-dive rules. Macro output schema (trend, risk_level, symbols_for_deep_dive) is fixed; deep-dive uses the same decision format as the main strategy.',
      },
      macroPrompts: { zh: 'Macro 提示', en: 'Macro Prompts' },
      macroPromptsDesc: { zh: '仅附加到 Macro（市场简报）AI 调用', en: 'Appended to the macro (market-brief) AI call only' },
      macroRoleContext: { zh: '角色与上下文', en: 'Role & Context' },
      macroRoleContextDesc: { zh: '定义宏观分析员的角色和关注点', en: 'Define macro analyst role and focus' },
      macroOutputGuidance: { zh: '输出与选币指引', en: 'Output & Symbol Guidance' },
      macroOutputGuidanceDesc: { zh: '如何选择深度分析标的与输出要求', en: 'How to select symbols and shape output' },
      deepDivePrompts: { zh: '深度分析提示', en: 'Deep-Dive Prompts' },
      deepDivePromptsDesc: { zh: '附加到每个符号的深度分析 AI 调用', en: 'Appended to each symbol deep-dive AI call' },
      deepDiveSymbolRules: { zh: '符号级规则', en: 'Symbol-Level Rules' },
      deepDiveSymbolRulesDesc: { zh: '仓位、风险、风格等每符号规则', en: 'Per-symbol rules for sizing, risk, style' },
      resetToDefault: { zh: '重置为默认', en: 'Reset to Default' },
      chars: { zh: '字符', en: 'chars' },
      modified: { zh: '已修改', en: 'Modified' },
    }
    return translations[key]?.[language] || key
  }

  const currentMacro = macroSections || {}
  const currentDeepDive = deepDiveSections || {}

  const updateMacro = (key: keyof MacroPromptSectionsConfig, value: string) => {
    if (!disabled) onMacroChange({ ...currentMacro, [key]: value })
  }
  const updateDeepDive = (key: keyof DeepDivePromptSectionsConfig, value: string) => {
    if (!disabled) onDeepDiveChange({ ...currentDeepDive, [key]: value })
  }

  const resetMacro = (key: keyof MacroPromptSectionsConfig) => {
    if (!disabled) onMacroChange({ ...currentMacro, [key]: defaultMacroSections[key] })
  }
  const resetDeepDive = (key: keyof DeepDivePromptSectionsConfig) => {
    if (!disabled) onDeepDiveChange({ ...currentDeepDive, [key]: defaultDeepDiveSections[key] })
  }

  const getMacroValue = (key: keyof MacroPromptSectionsConfig) =>
    currentMacro[key] ?? defaultMacroSections[key] ?? ''
  const getDeepDiveValue = (key: keyof DeepDivePromptSectionsConfig) =>
    currentDeepDive[key] ?? defaultDeepDiveSections[key] ?? ''

  const isMacroModified = (key: keyof MacroPromptSectionsConfig) =>
    currentMacro[key] !== undefined && currentMacro[key] !== defaultMacroSections[key]
  const isDeepDiveModified = (key: keyof DeepDivePromptSectionsConfig) =>
    currentDeepDive[key] !== undefined && currentDeepDive[key] !== defaultDeepDiveSections[key]

  const toggle = (id: string) => {
    setExpanded((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  const renderSection = (
    id: string,
    label: string,
    desc: string,
    value: string,
    modified: boolean,
    onChange: (v: string) => void,
    onReset: () => void
  ) => (
    <div
      key={id}
      className="rounded-lg overflow-hidden"
      style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
    >
      <button
        type="button"
        onClick={() => toggle(id)}
        className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-white/5 transition-colors text-left"
      >
        <div className="flex items-center gap-2">
          {expanded[id] ? (
            <ChevronDown className="w-4 h-4" style={{ color: '#848E9C' }} />
          ) : (
            <ChevronRight className="w-4 h-4" style={{ color: '#848E9C' }} />
          )}
          <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
            {label}
          </span>
          {modified && (
            <span
              className="px-1.5 py-0.5 text-[10px] rounded"
              style={{ background: 'rgba(168, 85, 247, 0.15)', color: '#a855f7' }}
            >
              {t('modified')}
            </span>
          )}
        </div>
        <span className="text-[10px]" style={{ color: '#848E9C' }}>
          {value.length} {t('chars')}
        </span>
      </button>
      {expanded[id] && (
        <div className="px-3 pb-3">
          <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
            {desc}
          </p>
          <textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled}
            rows={5}
            className="w-full px-3 py-2 rounded-lg resize-y font-mono text-xs"
            style={{
              background: '#1E2329',
              border: '1px solid #2B3139',
              color: '#EAECEF',
              minHeight: '100px',
            }}
          />
          <div className="flex justify-end mt-2">
            <button
              type="button"
              onClick={onReset}
              disabled={disabled || !modified}
              className="flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/5 disabled:opacity-30"
              style={{ color: '#848E9C' }}
            >
              <RotateCcw className="w-3 h-3" />
              {t('resetToDefault')}
            </button>
          </div>
        </div>
      )}
    </div>
  )

  return (
    <div className="space-y-6">
      <p className="text-xs mb-4" style={{ color: '#848E9C' }}>
        {t('macroSectionIntro')}
      </p>
      <div className="flex items-start gap-2 mb-4">
        <FileText className="w-5 h-5 mt-0.5" style={{ color: '#a855f7' }} />
        <div>
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('macroPrompts')}
          </h3>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('macroPromptsDesc')}
          </p>
        </div>
      </div>
      <div className="space-y-2">
        {renderSection(
          'macro_role_context',
          t('macroRoleContext'),
          t('macroRoleContextDesc'),
          getMacroValue('role_context'),
          isMacroModified('role_context'),
          (v) => updateMacro('role_context', v),
          () => resetMacro('role_context')
        )}
        {renderSection(
          'macro_output_guidance',
          t('macroOutputGuidance'),
          t('macroOutputGuidanceDesc'),
          getMacroValue('output_guidance'),
          isMacroModified('output_guidance'),
          (v) => updateMacro('output_guidance', v),
          () => resetMacro('output_guidance')
        )}
      </div>

      <div className="flex items-start gap-2 mt-6">
        <FileText className="w-5 h-5 mt-0.5" style={{ color: '#60a5fa' }} />
        <div>
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {t('deepDivePrompts')}
          </h3>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {t('deepDivePromptsDesc')}
          </p>
        </div>
      </div>
      <div className="space-y-2">
        {renderSection(
          'deep_dive_symbol_rules',
          t('deepDiveSymbolRules'),
          t('deepDiveSymbolRulesDesc'),
          getDeepDiveValue('symbol_rules'),
          isDeepDiveModified('symbol_rules'),
          (v) => updateDeepDive('symbol_rules', v),
          () => resetDeepDive('symbol_rules')
        )}
      </div>
    </div>
  )
}
