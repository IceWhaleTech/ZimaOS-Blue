import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function getLocaleCode(modulePath: string): string {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

const protectedServicePaths = {
  'service.autoStart': 'Start on Boot',
  'service.autoStartDescription': 'Automatically start the service when the system boots',
  'service.enableSuccess': 'Auto-start enabled successfully',
  'service.disableSuccess': 'Auto-start disabled successfully',
  'service.enableFailed': 'Failed to enable auto-start',
  'service.disableFailed': 'Failed to disable auto-start',
} as const

describe('service auto-start locale coverage', () => {
  it('keeps auto-start labels and status messages localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = getLocaleCode(modulePath)
      const file = fileNameFromModulePath(modulePath)

      for (const [path, englishValue] of Object.entries(protectedServicePaths)) {
        const localizedValue = getPathValue(mod.default, path)
        expect(typeof localizedValue, `${file} missing ${path}`).toBe('string')
        expect(String(localizedValue).trim().length, `${file} empty ${path}`).toBeGreaterThan(0)

        if (locale === 'en-US' || locale === 'en-GB') {
          expect(localizedValue, `${file} English copy for ${path}`).toBe(englishValue)
          continue
        }

        expect(localizedValue, `${file} should not fall back to English for ${path}`).not.toBe(
          englishValue
        )
      }
    }
  })
})
