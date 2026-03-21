import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'

type LocaleLeaf = string | number | boolean | null | undefined
type LocaleValue = LocaleLeaf | LocaleNode | LocaleLeaf[] | LocaleNode[]
interface LocaleNode {
  [key: string]: LocaleValue
}
type ChannelsMessages = { channels?: LocaleNode }

const localeModules = import.meta.glob<{ default: ChannelsMessages }>('./locales/*.ts', {
  eager: true,
})

function localeFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop()?.replace(/\.ts$/, '') ?? modulePath
}

function collectLeafKeys(value: unknown, prefix = ''): string[] {
  if (Array.isArray(value)) {
    return []
  }
  if (!value || typeof value !== 'object') {
    return prefix ? [prefix] : []
  }

  const keys: string[] = []
  for (const [key, child] of Object.entries(value)) {
    const nextPrefix = prefix ? `${prefix}.${key}` : key
    if (Array.isArray(child)) {
      keys.push(nextPrefix)
      continue
    }
    if (child && typeof child === 'object') {
      keys.push(...collectLeafKeys(child, nextPrefix))
      continue
    }
    keys.push(nextPrefix)
  }
  return keys
}

describe('channels locale coverage', () => {
  it('exposes all en-US channels keys in every locale module', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries.length).toBe(27)

    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceChannels = localeModules[enUSPath]?.default?.channels
    expect(referenceChannels).toBeTruthy()
    if (!referenceChannels) {
      throw new Error('Missing en-US channels reference')
    }

    const requiredKeys = collectLeafKeys(referenceChannels)
    expect(requiredKeys.length).toBeGreaterThan(0)

    const missingKeys: string[] = []
    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const channels = mod.default?.channels
      expect(channels, `${locale} should expose channels`).toBeTruthy()
      if (!channels) {
        missingKeys.push(`${locale}: channels`)
        continue
      }

      for (const key of requiredKeys) {
        const segments = key.split('.')
        let current: unknown = channels
        for (const segment of segments) {
          current =
            current && typeof current === 'object' && !Array.isArray(current)
              ? (current as Record<string, unknown>)[segment]
              : undefined
        }
        if (typeof current === 'undefined') {
          missingKeys.push(`${locale}: channels.${key}`)
        }
      }
    }

    expect(missingKeys).toEqual([])
  })

  it('compiles every channels string in every locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries.length).toBe(27)

    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const referenceChannels = localeModules[enUSPath]?.default?.channels
    expect(referenceChannels).toBeTruthy()
    if (!referenceChannels) {
      throw new Error('Missing en-US channels reference')
    }

    const requiredKeys = collectLeafKeys(referenceChannels)
    expect(requiredKeys.length).toBeGreaterThan(0)

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const i18n = createI18n({
        legacy: false,
        locale,
        fallbackLocale: locale,
        missingWarn: false,
        fallbackWarn: false,
        messages: {
          [locale]: mod.default,
        },
      })

      for (const key of requiredKeys) {
        expect(() => i18n.global.t(`channels.${key}`), `${locale} should compile channels.${key}`).not.toThrow()
      }
    }
  })
})
