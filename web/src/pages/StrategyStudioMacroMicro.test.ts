import { describe, it, expect } from 'vitest'

/**
 * Macro-micro feature tests for Strategy Studio UI.
 *
 * When EnableMacroMicroFlow is true:
 * - preview-prompt API returns `steps` array (no top-level system_prompt for multi-turn)
 * - test-run API returns `steps`, `decisions`, and merged AI output
 *
 * Single-turn: returns system_prompt / user_prompt; no steps.
 */

type PromptPreviewResponse =
  | { system_prompt: string; prompt_variant: string; config_summary: Record<string, unknown> }
  | { steps: Array<{ step: string; label: string; system_prompt: string; user_prompt: string }>; prompt_variant: string; config_summary: Record<string, unknown> }

type AITestResult =
  | {
      system_prompt?: string
      user_prompt?: string
      ai_response?: string
      decisions?: unknown[]
      steps?: Array<{ step: string; label: string; symbol?: string; system_prompt: string; user_prompt: string; response: string }>
      error?: string
    }
  | { error: string }

// Logic used by StrategyStudioPage to decide what to render
function shouldShowStepsPreview(preview: PromptPreviewResponse | null): boolean {
  if (!preview) return false
  return 'steps' in preview && Array.isArray(preview.steps) && preview.steps.length > 0
}

function shouldShowStepsTestResult(result: AITestResult | null): boolean {
  if (!result || result.error) return false
  return Array.isArray(result.steps) && result.steps.length > 0
}

describe('StrategyStudioPage - Macro-micro response shapes', () => {
  describe('Prompt preview response shape', () => {
    it('macro-micro preview has steps array with expected structure', () => {
      const macroMicroPreview: PromptPreviewResponse = {
        steps: [
          { step: 'macro', label: 'Macro', system_prompt: 'You are...', user_prompt: '[Generated at runtime]' },
          { step: 'deep_dive', label: 'Deep-dive (per symbol)', system_prompt: 'Analyze...', user_prompt: '[Generated per symbol]' },
          { step: 'position_check', label: 'Position check (if open positions)', system_prompt: 'Review...', user_prompt: '[Generated at runtime]' },
        ],
        prompt_variant: 'balanced',
        config_summary: { coin_source: 'ai500', primary_tf: '3m' },
      }
      expect(shouldShowStepsPreview(macroMicroPreview)).toBe(true)
      expect(macroMicroPreview.steps).toHaveLength(3)
      expect(macroMicroPreview.steps[0].step).toBe('macro')
      expect(macroMicroPreview.steps[1].step).toBe('deep_dive')
      expect(macroMicroPreview.steps[2].step).toBe('position_check')
    })

    it('single-turn preview has system_prompt and no steps', () => {
      const singleTurnPreview: PromptPreviewResponse = {
        system_prompt: 'You are a trading assistant...',
        prompt_variant: 'balanced',
        config_summary: { coin_source: 'static', primary_tf: '15m' },
      }
      expect(shouldShowStepsPreview(singleTurnPreview)).toBe(false)
      expect('system_prompt' in singleTurnPreview).toBe(true)
    })

    it('empty steps array should not show steps UI', () => {
      const previewWithEmptySteps: PromptPreviewResponse = {
        steps: [],
        prompt_variant: 'balanced',
        config_summary: {},
      }
      expect(shouldShowStepsPreview(previewWithEmptySteps)).toBe(false)
    })

    it('null preview should not show steps', () => {
      expect(shouldShowStepsPreview(null)).toBe(false)
    })
  })

  describe('AI test run response shape', () => {
    it('macro-micro test result has steps and decisions', () => {
      const macroMicroResult: AITestResult = {
        steps: [
          {
            step: 'macro',
            label: 'Macro',
            system_prompt: '...',
            user_prompt: '...',
            response: '<macrobrief>BTCUSDT, ETHUSDT</macrobrief>',
          },
          {
            step: 'deep_dive',
            label: 'Deep-dive: BTCUSDT',
            symbol: 'BTCUSDT',
            system_prompt: '...',
            user_prompt: '...',
            response: 'BUY 0.5 confidence',
          },
        ],
        decisions: [
          { symbol: 'BTCUSDT', action: 'buy', confidence: 0.5 },
        ],
        ai_response: 'Full CoT trace...',
      }
      expect(shouldShowStepsTestResult(macroMicroResult)).toBe(true)
      expect(Array.isArray((macroMicroResult as { steps?: unknown[] }).steps)).toBe(true)
      expect((macroMicroResult as { decisions?: unknown[] }).decisions).toHaveLength(1)
    })

    it('single-turn test result has no steps', () => {
      const singleTurnResult: AITestResult = {
        system_prompt: '...',
        user_prompt: 'Market data...',
        ai_response: 'Reasoning...',
        decisions: [{ symbol: 'BTCUSDT', action: 'hold' }],
        duration_ms: 2500,
      }
      expect(shouldShowStepsTestResult(singleTurnResult)).toBe(false)
    })

    it('error result should not show steps', () => {
      const errorResult: AITestResult = { error: 'AI API call failed' }
      expect(shouldShowStepsTestResult(errorResult)).toBe(false)
    })

    it('null result should not show steps', () => {
      expect(shouldShowStepsTestResult(null)).toBe(false)
    })
  })

  describe('Display logic', () => {
    it('macro-micro preview renders steps block when steps present', () => {
      const preview: PromptPreviewResponse = {
        steps: [{ step: 'macro', label: 'Macro', system_prompt: 'A', user_prompt: 'B' }],
        prompt_variant: 'balanced',
        config_summary: {},
      }
      const showStepsBlock = shouldShowStepsPreview(preview)
      const showSingleBlock = !showStepsBlock && 'system_prompt' in preview
      expect(showStepsBlock).toBe(true)
      expect(showSingleBlock).toBe(false)
    })

    it('single-turn preview renders single system prompt block', () => {
      const preview: PromptPreviewResponse = {
        system_prompt: 'You are...',
        prompt_variant: 'balanced',
        config_summary: {},
      }
      const showStepsBlock = shouldShowStepsPreview(preview)
      const showSingleBlock = !showStepsBlock && 'system_prompt' in preview
      expect(showStepsBlock).toBe(false)
      expect(showSingleBlock).toBe(true)
    })
  })
})
