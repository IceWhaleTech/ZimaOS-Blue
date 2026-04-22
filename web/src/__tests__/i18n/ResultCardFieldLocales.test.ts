import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredLabelKeys = [
  'action_ms',
  'action',
  'async',
  'automation',
  'browser',
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
  'recovered_target_id',
  'ref_map',
  'results',
  'route',
  'selected',
  'selected_source_rank',
  'snapshot_revision',
  'source',
  'surface',
  'target_format',
  'target_hit',
  'title',
  'total_count',
  'tree_fetch_ms',
  'url',
  'verification_method',
  'verification_passed',
  'warning_count',
  'warnings',
  'window_id',
  'windows',
] as const

const requiredMessageKeys = [
  'application_activated',
  'browser_tab_focus_recovered_using_active_tab',
  'browser_tab_focused',
  'browser_computer_use_bridge_ready',
  'browser_keys_sent',
  'host_action_completed',
  'host_accessibility_snapshot_ready',
  'keys_sent',
  'host_action_completed_and_submitted',
  'host_computer_use_snapshot_ready',
  'host_screenshot_captured',
  'host_windows_listed',
  'pointer_moved',
  'scroll_completed',
  'window_focused',
] as const

const requiredValuePaths = [
  'resultCard.values.execution_mode.automation',
  'resultCard.values.execution_mode.input',
  'resultCard.values.execution_mode.semantic',
  'resultCard.values.fallbacks.click',
  'resultCard.values.fallbacks.input_click',
  'resultCard.values.host_os.browser',
  'resultCard.values.host_os.darwin',
  'resultCard.values.host_os.windows',
  'resultCard.values.input_method.clipboard',
  'resultCard.values.input_method.input_click',
  'resultCard.values.input_method.semantic_action',
  'resultCard.values.input_method.set_value',
  'resultCard.values.intent.message',
  'resultCard.values.intent.select',
  'resultCard.values.intent.toggle',
  'resultCard.values.intent.type',
  'resultCard.values.intent.click',
  'resultCard.values.surface.browser',
  'resultCard.values.verification_method.ax_action',
  'resultCard.values.verification_method.ax_value',
  'resultCard.values.verification_method.input_action',
  'resultCard.values.verification_method.focused_text',
  'resultCard.values.verification_method.point_click',
  'resultCard.values.verification_method.semantic_action',
] as const

const localizedValuePaths = [
  'resultCard.values.execution_mode.automation',
  'resultCard.values.execution_mode.input',
  'resultCard.values.execution_mode.semantic',
  'resultCard.values.fallbacks.click',
  'resultCard.values.fallbacks.input_click',
  'resultCard.values.host_os.browser',
  'resultCard.values.input_method.clipboard',
  'resultCard.values.input_method.input_click',
  'resultCard.values.input_method.semantic_action',
  'resultCard.values.input_method.set_value',
  'resultCard.values.intent.select',
  'resultCard.values.intent.toggle',
  'resultCard.values.intent.type',
  'resultCard.values.intent.click',
  'resultCard.values.surface.browser',
  'resultCard.values.verification_method.ax_action',
  'resultCard.values.verification_method.ax_value',
  'resultCard.values.verification_method.input_action',
  'resultCard.values.verification_method.focused_text',
  'resultCard.values.verification_method.point_click',
  'resultCard.values.verification_method.semantic_action',
] as const

const localizedLabelKeys = [
  'action_ms',
  'automation',
  'browser',
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
  'recovered_target_id',
  'ref_map',
  'selected_source_rank',
  'snapshot_revision',
  'surface',
  'target_format',
  'target_hit',
  'total_count',
  'tree_fetch_ms',
  'verification_method',
  'verification_passed',
  'warning_count',
  'warnings',
  'window_id',
  'windows',
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
