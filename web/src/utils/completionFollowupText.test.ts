import { describe, expect, it } from 'vitest'

import { localizeCompletionFollowupHeading } from './completionFollowupText'

describe('completion follow-up heading localization', () => {
  it('replaces the exact english heading line with a localized heading', () => {
    const content = [
      'Summary: The task completed and verification passed.',
      '',
      "If you'd like, I can also help with:",
      '1. If you want, I can help extend the implementation.',
    ].join('\n')

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(
      [
        'Summary: The task completed and verification passed.',
        '',
        '如果你愿意，我还可以帮你：',
        '1. If you want, I can help extend the implementation.',
      ].join('\n')
    )
  })

  it('leaves content unchanged when the heading is absent', () => {
    const content = 'Summary: The task completed and verification passed.'

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(content)
  })

  it('does not replace partial matches inside a regular paragraph', () => {
    const content =
      "The docs literally say If you'd like, I can also help with: but this line is not a section heading."

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(content)
  })
})
