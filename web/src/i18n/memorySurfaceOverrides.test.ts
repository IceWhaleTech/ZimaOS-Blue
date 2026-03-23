import { describe, expect, it } from 'vitest'

import type { LocaleMessages } from './merge'
import memorySurfaceOverrides from './memory-surface-overrides'

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

const requiredKeys = [
  'memory.recallSettings',
  'settings.memorySurface',
  'settings.memoryManagement',
] as const

describe('memory surface localization coverage', () => {
  it('defines memory surface copy for all 27 locales', () => {
    const overrides = memorySurfaceOverrides as Record<string, LocaleMessages>
    const overrideLocales = Object.keys(overrides).sort()

    expect(localeCodes).toHaveLength(27)
    expect(overrideLocales).toEqual(localeCodes)

    for (const locale of overrideLocales) {
      const localeMessages = overrides[locale]
      expect(localeMessages, `${locale} should define memory surface overrides`).toBeTruthy()
      if (!localeMessages) continue

      for (const key of requiredKeys) {
        const value = getByPath(localeMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})
