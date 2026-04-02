import { describe, expect, it } from 'vitest'

import { getWorkspaceVisibleTokenCount } from './workspaceTokenEstimate'

describe('getWorkspaceVisibleTokenCount', () => {
  it('counts only the visible workspace files when per-file stats are present', () => {
    expect(
      getWorkspaceVisibleTokenCount(
        {
          files: [
            { name: 'SOUL.md', bytes: 100, tokens: 1200 },
            { name: 'USER.md', bytes: 100, tokens: 600 },
            { name: '2026-04-02.md', bytes: 100, tokens: 5000 },
          ],
          total_tokens: 6800,
        },
        ['SOUL.md', 'USER.md']
      )
    ).toBe(1800)
  })

  it('falls back to total tokens when the server does not provide per-file stats', () => {
    expect(
      getWorkspaceVisibleTokenCount(
        {
          files: [],
          total_tokens: 4096,
        },
        ['SOUL.md', 'USER.md']
      )
    ).toBe(4096)
  })
})
