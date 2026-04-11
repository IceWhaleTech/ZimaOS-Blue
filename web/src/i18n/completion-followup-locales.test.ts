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
  it('exposes non-empty localized completion follow-up copy for all 27 locales', () => {
    expect(localeKeys).toHaveLength(27)

    for (const localeKey of localeKeys) {
      const chat = getChatMessages(localeKey)
      const keys = [
        'completionFollowupHeading',
        'completionFollowupNoFurtherActionNeeded',
        'completionFollowupExpandFullerReport',
        'completionFollowupVerifyKeyEvidence',
        'completionFollowupReorderTakeaways',
        'completionFollowupOptimizationIdeas',
        'completionFollowupInspectFailedSteps',
        'completionFollowupRerunValidation',
        'completionFollowupKeepFixing',
        'completionFollowupVerifyDeliverables',
        'completionFollowupRunRelevantTests',
        'completionFollowupOptimizeNextArea',
      ] as const

      for (const key of keys) {
        const value = chat[key]
        expect(typeof value, `${localeKey} should expose chat.${key}`).toBe('string')
        expect(String(value).trim().length, `${localeKey} should expose a non-empty chat.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps english copy for en-US and provides localized chinese copy for follow-up text', () => {
    expect(getChatMessages('en-US').completionFollowupHeading).toBe("If you'd like, I can also help with:")
    expect(getChatMessages('en-US').completionFollowupNoFurtherActionNeeded).toBe(
      'No further action needed.'
    )
    expect(getChatMessages('en-US').completionFollowupExpandFullerReport).toBe(
      "If you'd like, I can expand this into a fuller report."
    )
    expect(getChatMessages('zh-CN').completionFollowupHeading).toBe('如果你愿意，我还可以帮你：')
    expect(getChatMessages('zh-CN').completionFollowupNoFurtherActionNeeded).toBe('当前无需进一步操作。')
    expect(getChatMessages('zh-CN').completionFollowupExpandFullerReport).toBe(
      '如果你愿意，我可以继续把这份结果扩展成更完整的总结。'
    )
    expect(getChatMessages('zh-TW').completionFollowupHeading).toBe('如果你願意，我還可以幫你：')
    expect(getChatMessages('zh-TW').completionFollowupNoFurtherActionNeeded).toBe('目前無需進一步操作。')
  })
})
