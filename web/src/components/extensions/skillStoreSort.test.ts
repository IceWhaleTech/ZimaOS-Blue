import { describe, expect, it } from 'vitest'

import { skillStoreSortTranslationPath } from './skillStoreSort'

describe('skillStoreSortTranslationPath', () => {
  it('maps most_used to the localized marketplace key', () => {
    expect(skillStoreSortTranslationPath('most_used')).toBe('sort.mostUsed')
  })

  it('keeps the other sort modes unchanged', () => {
    expect(skillStoreSortTranslationPath('featured')).toBe('sort.featured')
    expect(skillStoreSortTranslationPath('trending')).toBe('sort.trending')
    expect(skillStoreSortTranslationPath('newest')).toBe('sort.newest')
  })
})
