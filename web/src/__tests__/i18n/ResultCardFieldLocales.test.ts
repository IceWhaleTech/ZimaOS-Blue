import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredLabelKeys = [
  'action',
  'async',
  'description',
  'download_url',
  'execution_mode',
  'host_os',
  'image_path',
  'output_path',
  'output_ref',
  'outputs',
  'query',
  'rank',
  'results',
  'route',
  'selected',
  'selected_source_rank',
  'source',
  'target_format',
  'title',
  'total_count',
  'url',
  'warning_count',
  'warnings',
  'window_id',
] as const

const requiredMessageKeys = [
  'host_screenshot_captured',
  'host_windows_listed',
  'window_focused',
] as const

const requiredValuePaths = [
  'resultCard.values.execution_mode.semantic',
  'resultCard.values.host_os.darwin',
] as const

const localizedValuePaths = ['resultCard.values.execution_mode.semantic'] as const

const localizedLabelKeys = [
  'execution_mode',
  'host_os',
  'image_path',
  'output_path',
  'query',
  'selected_source_rank',
  'target_format',
  'total_count',
  'warning_count',
  'warnings',
  'window_id',
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

describe('result-card field locale coverage', () => {
  it('backfills host/web-query result labels and messages in every enhanced locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]

    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const enhancedReference = mergeHarnessLocale('en-US', localeModules[enUSPath]!.default)

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const enhancedMessages = mergeHarnessLocale(locale as never, mod.default)

      for (const key of requiredLabelKeys) {
        const path = `resultCard.labels.${key}`
        const value = getPathValue(enhancedMessages, path)
        expect(typeof value, `${locale} should expose ${path}`).toBe('string')
        expect(String(value).trim().length, `${locale} should not leave ${path} empty`).toBeGreaterThan(
          0
        )
        expect(value, `${locale} should not leave ${path} as the raw compact key`).not.toBe(key)
      }

      for (const key of requiredMessageKeys) {
        const path = `resultCard.messages.${key}`
        const value = getPathValue(enhancedMessages, path)
        expect(typeof value, `${locale} should expose ${path}`).toBe('string')
        expect(String(value).trim().length, `${locale} should not leave ${path} empty`).toBeGreaterThan(
          0
        )
      }

      for (const path of requiredValuePaths) {
        const value = getPathValue(enhancedMessages, path)
        expect(typeof value, `${locale} should expose ${path}`).toBe('string')
        expect(String(value).trim().length, `${locale} should not leave ${path} empty`).toBeGreaterThan(
          0
        )
      }

      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const key of localizedLabelKeys) {
        expect(
          getPathValue(enhancedMessages, `resultCard.labels.${key}`),
          `${locale} should localize resultCard.labels.${key}`
        ).not.toBe(getPathValue(enhancedReference, `resultCard.labels.${key}`))
      }

      for (const key of requiredMessageKeys) {
        expect(
          getPathValue(enhancedMessages, `resultCard.messages.${key}`),
          `${locale} should localize resultCard.messages.${key}`
        ).not.toBe(getPathValue(enhancedReference, `resultCard.messages.${key}`))
      }

      for (const path of localizedValuePaths) {
        expect(getPathValue(enhancedMessages, path), `${locale} should localize ${path}`).not.toBe(
          getPathValue(enhancedReference, path)
        )
      }
    }
  })
})
