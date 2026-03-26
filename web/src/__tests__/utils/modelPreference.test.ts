import { describe, expect, it } from 'vitest'
import { compareModelPreference, sortItemsByModelPreference } from '@/utils/modelPreference'

describe('compareModelPreference', () => {
  it('prioritizes claude and gpt prefixes over other model families', () => {
    const sorted = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5'].sort(
      compareModelPreference
    )

    expect(sorted).toEqual(['claude-haiku-4-5', 'gpt-4o-mini', 'o3-mini', 'gemini-2.5-pro'])
  })

  it('prioritizes claude and gpt after stripping provider namespaces', () => {
    const sorted = [
      'openai/o3-mini',
      'google/gemini-2.5-pro',
      'openai/gpt-4.1-mini',
      'anthropic/claude-sonnet-4-5',
    ].sort(compareModelPreference)

    expect(sorted.slice(0, 2)).toEqual(['anthropic/claude-sonnet-4-5', 'openai/gpt-4.1-mini'])
  })

  it('leaves small lists unchanged', () => {
    const items = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5']

    expect(sortItemsByModelPreference(items, (item) => item)).toEqual(items)
  })

  it('only prioritizes claude and gpt when the list is large', () => {
    const items = ['o3-mini', 'gemini-2.5-pro', 'gpt-4o-mini', 'claude-haiku-4-5']
    for (let index = 0; index < 100; index += 1) {
      items.push(`misc-${index.toString().padStart(3, '0')}`)
    }

    const sorted = sortItemsByModelPreference(items, (item) => item)

    expect(sorted.slice(0, 2)).toEqual(['claude-haiku-4-5', 'gpt-4o-mini'])
  })
})
