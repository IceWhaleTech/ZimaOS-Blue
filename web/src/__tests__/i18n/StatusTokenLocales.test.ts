import { describe, expect, it } from 'vitest'

import { localizeStatusToken } from '@/utils/statusTokens'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

function translateFor(messages: LocaleMessages) {
  return (key: string, fallback: string) => {
    const value = getPathValue(messages, key)
    return typeof value === 'string' && value.trim() ? value : fallback
  }
}

describe('Shared status token locale coverage', () => {
  it('keeps completed and ok tokens localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const translate = translateFor(messages)
      const completed = getPathValue(messages, 'chat.taskStageCompleted')
      const ok = getPathValue(messages, 'system.statusOk')

      expect(typeof completed, `${file} missing chat.taskStageCompleted`).toBe('string')
      expect(typeof ok, `${file} missing system.statusOk`).toBe('string')
      expect(localizeStatusToken('[completed]', translate), `${file} completed token`).toBe(
        completed
      )
      expect(localizeStatusToken('【ok】', translate), `${file} ok token`).toBe(ok)
    }
  })
})
