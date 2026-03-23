import { describe, expect, it } from 'vitest'

import { browserMonitorLocales } from '@/i18n/browser-monitor-locales'
import type { LocaleMessages } from '@/i18n/merge'

const requiredKeys = [
  'blockerFallback',
  'buttonLabel',
  'capabilityBlocker',
  'capabilityBrowser',
  'capabilityBrowserIdle',
  'capabilityHeading',
  'capabilityPreview',
  'capabilityPreviewIdle',
  'capabilityTask',
  'capabilityTaskIdle',
  'collapse',
  'compactTab',
  'compactTabHint',
  'compactTabIdle',
  'compactTask',
  'compactTaskIdle',
  'expand',
  'eyebrow',
  'hideTooltip',
  'justNow',
  'launcherActive',
  'launcherIdle',
  'launcherMeta',
  'noCurrentConversation',
  'noCurrentTasks',
  'noTasks',
  'overviewError',
  'previewAlt',
  'previewEmptyBody',
  'previewEmptyTitle',
  'previewHeading',
  'previewIdleTitle',
  'previewIdleUrl',
  'previewLoading',
  'previewPending',
  'refresh',
  'refreshing',
  'screenshotError',
  'showTooltip',
  'stageRunning',
  'subtitle',
  'tabsShort',
  'taskViewAll',
  'taskViewAllShort',
  'taskViewCurrent',
  'taskViewCurrentShort',
  'taskViewSmart',
  'tasksCount',
  'tasksHeading',
  'tasksShort',
  'title',
  'untitledTab',
  'updated',
] as const

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

const localeSourceModules = import.meta.glob('@/i18n/locales/*.ts', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>

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

describe('browser monitor locale coverage', () => {
  it('declares browserMonitor in all 27 locale source files', () => {
    const sources = new Map(
      Object.entries(localeSourceModules).map(([modulePath, source]) => [
        fileNameFromModulePath(modulePath),
        source,
      ])
    )

    expect(sources.size).toBe(27)

    for (const [fileName, source] of sources) {
      expect(source, `${fileName} should declare browserMonitor`).toMatch(/\bbrowserMonitor:\s*/)
    }
  })

  it('ships browser monitor translations for all 27 locales', () => {
    expect(Object.keys(browserMonitorLocales).sort()).toEqual([
      'ca-ES',
      'cs-CZ',
      'da-DK',
      'de-DE',
      'el-GR',
      'en-GB',
      'en-US',
      'es-ES',
      'fr-FR',
      'ga-IE',
      'hr-HR',
      'hu-HU',
      'it-IT',
      'ja-JP',
      'ko-KR',
      'ml-IN',
      'nb-NO',
      'nl-NL',
      'pl-PL',
      'pt-BR',
      'pt-PT',
      'ro-RO',
      'ru-RU',
      'sk-SK',
      'sv-SE',
      'zh-CN',
      'zh-TW',
    ])

    for (const [locale, messages] of Object.entries(browserMonitorLocales)) {
      for (const key of requiredKeys) {
        const value = messages[key]
        expect(typeof value, `${locale} should provide browserMonitor.${key}`).toBe('string')
        expect(value.trim().length, `${locale} should not leave browserMonitor.${key} empty`).toBeGreaterThan(0)
      }
    }
  })

  it('exposes browserMonitor keys in every merged locale module', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    expect(messagesByFile.size).toBe(27)

    for (const [fileName, messages] of messagesByFile) {
      for (const key of requiredKeys) {
        const value = getPathValue(messages, `browserMonitor.${key}`)
        expect(typeof value, `${fileName} should expose browserMonitor.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${fileName} should not leave browserMonitor.${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})

