import { describe, expect, it } from 'vitest'
import { t, translations } from './translations'

function leafKeys(value: unknown, prefix = ''): string[] {
  if (typeof value === 'string') return [prefix]
  if (!value || typeof value !== 'object') return []

  return Object.entries(value).flatMap(([key, child]) =>
    leafKeys(child, prefix ? `${prefix}.${key}` : key)
  )
}

function leafValues(
  value: unknown,
  prefix = '',
  result: Record<string, string> = {}
): Record<string, string> {
  if (typeof value === 'string') {
    result[prefix] = value
    return result
  }
  if (!value || typeof value !== 'object') return result

  for (const [key, child] of Object.entries(value)) {
    leafValues(child, prefix ? `${prefix}.${key}` : key, result)
  }
  return result
}

function placeholders(value: string): string[] {
  return [...value.matchAll(/\{([^}]+)\}/g)].map((match) => match[1]).sort()
}

describe('Japanese translations', () => {
  it('covers every English translation key', () => {
    expect(leafKeys(translations.ja).sort()).toEqual(
      leafKeys(translations.en).sort()
    )
  })

  it('replaces interpolation values', () => {
    expect(t('lastCycles', 'ja', { count: 3 })).toContain('3')
  })

  it('preserves every interpolation placeholder', () => {
    const english = leafValues(translations.en)
    const japanese = leafValues(translations.ja)

    for (const key of Object.keys(english)) {
      expect(placeholders(japanese[key]), key).toEqual(placeholders(english[key]))
    }
  })
})
