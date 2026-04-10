import { describe, expect, it } from 'vitest'

import { mergeHarnessLocale } from './harness-locale-additions'
import { localeKeys, type LocaleKey } from './locale-catalog'

type LocaleNode = Record<string, unknown>

function isPlainObject(value: unknown): value is LocaleNode {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function getChatMessages(localeKey: LocaleKey): LocaleNode {
  const merged = mergeHarnessLocale<LocaleNode>(localeKey, {})
  const chat = merged.chat

  expect(isPlainObject(chat), `${localeKey} should expose chat messages`).toBe(true)

  return chat as LocaleNode
}

describe('completion follow-up locale coverage', () => {
  it('exposes a non-empty localized heading for all 27 locales', () => {
    expect(localeKeys).toHaveLength(27)

    for (const localeKey of localeKeys) {
      const chat = getChatMessages(localeKey)
      const value = chat.completionFollowupHeading

      expect(typeof value, `${localeKey} should expose chat.completionFollowupHeading`).toBe(
        'string'
      )
      expect(
        String(value).trim().length,
        `${localeKey} should expose a non-empty chat.completionFollowupHeading`
      ).toBeGreaterThan(0)
    }
  })

  it('keeps english copy for en-US and provides localized chinese copy', () => {
    expect(getChatMessages('en-US').completionFollowupHeading).toBe(
      "If you'd like, I can also help with:"
    )
    expect(getChatMessages('zh-CN').completionFollowupHeading).toBe('如果你愿意，我还可以帮你：')
    expect(getChatMessages('zh-TW').completionFollowupHeading).toBe('如果你願意，我還可以幫你：')
  })
})
