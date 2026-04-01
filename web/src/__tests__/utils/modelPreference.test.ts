import { describe, expect, it } from 'vitest'
import { compareModelPreference, sortItemsByModelPreference } from '@/utils/modelPreference'

describe('compareModelPreference', () => {
  it('sorts model names in descending alphabetical order', () => {
    const sorted = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5'].sort(
      compareModelPreference
    )

    expect(sorted).toEqual(['o3-mini', 'gpt-4o-mini', 'gemini-2.5-pro', 'claude-haiku-4-5'])
  })

  it('sorts by normalized model name after stripping provider namespaces', () => {
    const sorted = [
      'openai/o3-mini',
      'google/gemini-2.5-pro',
      'openai/gpt-4.1-mini',
      'anthropic/claude-sonnet-4-5',
    ].sort(compareModelPreference)

    expect(sorted).toEqual([
      'openai/o3-mini',
      'openai/gpt-4.1-mini',
      'google/gemini-2.5-pro',
      'anthropic/claude-sonnet-4-5',
    ])
  })

  it('sorts small lists instead of preserving original order', () => {
    const items = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5']

    expect(sortItemsByModelPreference(items, (item) => item)).toEqual([
      'o3-mini',
      'gpt-4o-mini',
      'gemini-2.5-pro',
      'claude-haiku-4-5',
    ])
  })

  it('sorts large lists with the same descending rule', () => {
    const items = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5']
    for (let index = 0; index < 100; index += 1) {
      items.push(`misc-${index.toString().padStart(3, '0')}`)
    }

    const sorted = sortItemsByModelPreference(items, (item) => item)

    expect(sorted.slice(0, 4)).toEqual(['o3-mini', 'misc-099', 'misc-098', 'misc-097'])
  })
})
