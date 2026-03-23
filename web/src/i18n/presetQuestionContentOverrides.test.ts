import { beforeEach, describe, expect, it } from 'vitest'

import { PRESET_QUESTION_EXAMPLE_KEYS } from '@/utils/presetQuestionI18n'
import { i18n, setLocale } from './index'
import type { LocaleMessages } from './merge'
import presetQuestionContentOverrides from './preset-question-content-overrides'

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function localeCodeFromFile(fileName: string): string {
  return fileName.replace(/\.ts$/, '')
}

function getByPath(source: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, part) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }

    return (value as Record<string, unknown>)[part]
  }, source)
}

const localeModules = import.meta.glob<{ default: LocaleMessages }>('./locales/*.ts', {
  eager: true,
})

const localeCodes = Object.keys(localeModules)
  .map(fileNameFromModulePath)
  .map(localeCodeFromFile)
  .sort()

describe('preset question content override coverage', () => {
  beforeEach(async () => {
    const storageState: Record<string, string> = {}
    const storage = {
      getItem: (key: string) => storageState[key] ?? null,
      setItem: (key: string, value: string) => {
        storageState[key] = String(value)
      },
      removeItem: (key: string) => {
        delete storageState[key]
      },
      clear: () => {
        for (const key of Object.keys(storageState)) {
          delete storageState[key]
        }
      },
    }

    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: storage,
    })
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: storage,
    })

    await setLocale('en-US')
  })

  it('defines example-card copy overrides for all 27 locales', () => {
    const overrides = presetQuestionContentOverrides as Record<string, LocaleMessages>
    const overrideLocales = Object.keys(overrides).sort()

    expect(localeCodes).toHaveLength(27)
    expect(overrideLocales).toEqual(localeCodes)

    for (const locale of overrideLocales) {
      const localeMessages = overrides[locale]
      expect(localeMessages, `${locale} should define preset-question overrides`).toBeTruthy()

      for (const key of PRESET_QUESTION_EXAMPLE_KEYS) {
        const value = getByPath(localeMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('merges localized example-card copy into the active locale bundle', async () => {
    const key = 'chat.presetQuestions.examples.memoryBank.title'

    await setLocale('ca-ES')
    expect(i18n.global.te(key)).toBe(true)
    expect(String(i18n.global.t(key))).toBe('Banc de memòria digital')

    await setLocale('nb-NO')
    expect(i18n.global.te(key)).toBe(true)
    expect(String(i18n.global.t(key))).toBe('Digital Memory Bank')
  })
})
