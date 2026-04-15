import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'

type LocaleMessages = Record<string, unknown>

const requiredMessageKeys = [
  'browser_tab_ready',
  'browser_not_running',
  'browser_service_not_available',
  'browser_service_not_available_cannot_review_url',
  'navigation_failed_url_not_allowed',
  'proxy_bridge_not_available',
  'proxy_bridge_not_available_cannot_call_vlm',
  'browser_page_structure_section',
  'browser_interactive_elements_section',
  'browser_scroll_direction_up',
  'browser_scroll_direction_down',
  'browser_scroll_direction_left',
  'browser_scroll_direction_right',
  'screenshot_captured',
  'screenshot_captured_interactive_elements_unavailable',
  'screenshot_captured_for_active_tab',
] as const

const requiredTemplateKeys = [
  'browser_start_failed',
  'navigation_failed',
  'browser_action_performed_on_ref',
  'browser_scrolled_page',
  'browser_open_tabs',
  'browser_tab_closed',
  'browser_recipes_available',
  'browser_large_dom_note',
  'browser_page',
  'browser_page_with_interactive_count',
  'browser_page_with_screenshot_interactive_count',
  'browser_page_main_content',
  'browser_page_main_content_with_section',
  'screenshot_captured_for',
  'screenshot_captured_for_tab',
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

describe('browser result-card locale coverage', () => {
  it('exposes browser result-card messages in every enhanced locale', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSPath = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))?.[0]

    expect(entries).toHaveLength(27)
    expect(enUSPath).toBeTruthy()
    if (!enUSPath) {
      throw new Error('Missing en-US locale module')
    }

    const enhancedReference = mergeHarnessLocale('en-US', localeModules[enUSPath]!.default)

    for (const [modulePath, mod] of entries) {
      const locale = fileNameFromModulePath(modulePath).replace(/\.ts$/, '')
      const enhancedMessages = mergeHarnessLocale(locale as never, mod.default)

      for (const key of requiredMessageKeys) {
        const value = getPathValue(enhancedMessages, `resultCard.messages.${key}`)
        expect(typeof value, `${locale} should expose resultCard.messages.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave resultCard.messages.${key} empty`
        ).toBeGreaterThan(0)
      }

      for (const key of requiredTemplateKeys) {
        const value = getPathValue(enhancedMessages, `resultCard.messageTemplates.${key}`)
        expect(typeof value, `${locale} should expose resultCard.messageTemplates.${key}`).toBe(
          'string'
        )
        expect(
          String(value).trim().length,
          `${locale} should not leave resultCard.messageTemplates.${key} empty`
        ).toBeGreaterThan(0)
      }

      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const key of requiredMessageKeys) {
        expect(
          getPathValue(enhancedMessages, `resultCard.messages.${key}`),
          `${locale} should localize resultCard.messages.${key}`
        ).not.toBe(getPathValue(enhancedReference, `resultCard.messages.${key}`))
      }

      for (const key of requiredTemplateKeys) {
        expect(
          getPathValue(enhancedMessages, `resultCard.messageTemplates.${key}`),
          `${locale} should localize resultCard.messageTemplates.${key}`
        ).not.toBe(getPathValue(enhancedReference, `resultCard.messageTemplates.${key}`))
      }
    }
  })
})
