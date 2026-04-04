import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const requiredKeys = [
  'advancedOptions',
  'collapse',
  'expand',
  'location',
  'locationCloud',
  'locationLocal',
  'locationHint',
] as const

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

describe('provider pool locale coverage', () => {
  it('exposes add-provider advanced and location copy in every final locale module', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    expect(messagesByFile.size).toBe(27)

    for (const [fileName, messages] of messagesByFile) {
      for (const key of requiredKeys) {
        const value = getPathValue(messages, `providerPool.${key}`)
        expect(typeof value, `${fileName} should expose providerPool.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${fileName} should not leave providerPool.${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('does not fall back to raw Cloud or Local in non-English locales', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    for (const [fileName, messages] of messagesByFile) {
      if (fileName === 'en-US.ts' || fileName === 'en-GB.ts') continue

      expect(getPathValue(messages, 'providerPool.locationCloud'), `${fileName} should localize Cloud`).not.toBe(
        'Cloud'
      )
      expect(getPathValue(messages, 'providerPool.locationLocal'), `${fileName} should localize Local`).not.toBe(
        'Local'
      )
    }
  })
})
