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

  it('replaces wrapped english heading variants with the localized heading', () => {
    const content = [
      'Summary: The task completed and verification passed.',
      '',
      "【If you'd like, I can also help with:】",
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

  it('normalizes mixed chinese-english heading variants to the localized heading', () => {
    const content = [
      '任务已完成。',
      '',
      "【如果你'd like，我还可以帮你：】",
      '1. 当前无需进一步操作。',
    ].join('\n')

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(
      [
        '任务已完成。',
        '',
        '如果你愿意，我还可以帮你：',
        '1. 当前无需进一步操作。',
      ].join('\n')
    )
  })

  it('normalizes unwrapped mixed chinese-english heading variants to the localized heading', () => {
    const content = [
      '任务已完成。',
      '',
      "如果你'd like，我还可以帮你：",
      '1. 当前无需进一步操作。',
    ].join('\n')

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(
      [
        '任务已完成。',
        '',
        '如果你愿意，我还可以帮你：',
        '1. 当前无需进一步操作。',
      ].join('\n')
    )
  })

  it('localizes exact follow-up body lines for the resolved locale', () => {
    const content = [
      '任务已完成。',
      '',
      "If you'd like, I can also help with:",
      '1. No further action needed.',
      "2. If you'd like, I can expand this into a fuller report.",
    ].join('\n')

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(
      [
        '任务已完成。',
        '',
        '如果你愿意，我还可以帮你：',
        '1. 当前无需进一步操作。',
        '2. 如果你愿意，我可以继续把这份结果扩展成更完整的总结。',
      ].join('\n')
    )
  })

  it('normalizes mixed follow-up line prefixes while preserving the localized tail', () => {
    const content = [
      '任务已完成。',
      '',
      '如果你愿意，我还可以帮你：',
      "1. 如果你'd like，我可以帮你运行一轮回归测试。",
    ].join('\n')

    expect(localizeCompletionFollowupHeading(content, '如果你愿意，我还可以帮你：')).toBe(
      [
        '任务已完成。',
        '',
        '如果你愿意，我还可以帮你：',
        '1. 如果你愿意，我可以帮你运行一轮回归测试。',
      ].join('\n')
    )
  })
})
