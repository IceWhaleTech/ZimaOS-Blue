import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredLabelKeys = [
  'action_ms',
  'action',
  'async',
  'cache_hit',
  'candidate_count',
  'description',
  'download_url',
  'end_to_end_ms',
  'execution_mode',
  'fallbacks',
  'host_os',
  'image_path',
  'input_method',
  'intent',
  'node_count',
  'output_path',
  'output_ref',
  'outputs',
  'query',
  'rank',
  'results',
  'route',
  'selected',
  'selected_source_rank',
  'snapshot_revision',
  'source',
  'target_format',
  'target_hit',
  'title',
  'total_count',
  'url',
  'verification_method',
  'verification_passed',
  'warning_count',
  'warnings',
  'window_id',
] as const

const requiredMessageKeys = [
  'host_action_completed',
  'keys_sent',
  'host_action_completed_and_submitted',
  'host_screenshot_captured',
  'host_windows_listed',
  'window_focused',
] as const

const requiredValuePaths = [
  'resultCard.values.execution_mode.input',
  'resultCard.values.execution_mode.semantic',
  'resultCard.values.fallbacks.click',
  'resultCard.values.fallbacks.input_click',
  'resultCard.values.host_os.darwin',
  'resultCard.values.input_method.clipboard',
  'resultCard.values.input_method.input_click',
  'resultCard.values.intent.click',
  'resultCard.values.verification_method.input_action',
  'resultCard.values.verification_method.focused_text',
] as const

const localizedValuePaths = [
  'resultCard.values.execution_mode.input',
  'resultCard.values.execution_mode.semantic',
  'resultCard.values.fallbacks.click',
  'resultCard.values.fallbacks.input_click',
  'resultCard.values.input_method.clipboard',
  'resultCard.values.input_method.input_click',
  'resultCard.values.intent.click',
  'resultCard.values.verification_method.input_action',
  'resultCard.values.verification_method.focused_text',
] as const

const localizedLabelKeys = [
  'action_ms',
  'cache_hit',
  'candidate_count',
  'end_to_end_ms',
  'execution_mode',
  'fallbacks',
  'host_os',
  'image_path',
  'input_method',
  'intent',
  'node_count',
  'output_path',
  'query',
  'selected_source_rank',
  'snapshot_revision',
  'target_format',
  'target_hit',
  'total_count',
  'verification_method',
  'verification_passed',
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
        expect(
          String(value).trim().length,
          `${locale} should not leave ${path} empty`
        ).toBeGreaterThan(0)
        expect(value, `${locale} should not leave ${path} as the raw compact key`).not.toBe(key)
      }

      for (const key of requiredMessageKeys) {
        const path = `resultCard.messages.${key}`
        const value = getPathValue(enhancedMessages, path)
        expect(typeof value, `${locale} should expose ${path}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${path} empty`
        ).toBeGreaterThan(0)
      }

      for (const path of requiredValuePaths) {
        const value = getPathValue(enhancedMessages, path)
        expect(typeof value, `${locale} should expose ${path}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${path} empty`
        ).toBeGreaterThan(0)
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
