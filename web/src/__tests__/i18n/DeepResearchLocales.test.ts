import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harnessLocaleAdditions'
import type { LocaleKey } from '@/i18n/locale-catalog'

import {
  localizeDeepResearchAction,
  localizeDeepResearchGap,
  localizeDeepResearchSegment,
  localizeDeepResearchStatus,
  localizeResearchProgressLabel,
  localizeResearchRunningElsewhereLabel,
  localizeResearchRunningTasksLabel,
  localizeResearchSurfaceTitle,
} from '@/utils/deepResearchText'

type LocaleMessages = Record<string, unknown>

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

function translateFor(messages: LocaleMessages) {
  return (key: string, fallback: string) => {
    const value = getPathValue(messages, key)
    return typeof value === 'string' && value.trim() ? value : fallback
  }
}

function resolveRuntimeMessages(locale: string, messages: LocaleMessages): LocaleMessages {
  return mergeHarnessLocale(locale as LocaleKey, messages)
}

describe('Deep research locale coverage', () => {
  it('keeps completed-state localization working for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const translate = translateFor(messages)
      const stageCompleted = getPathValue(messages, 'chat.deepResearchStageCompleted')
      const actionCompleted = getPathValue(messages, 'chat.deepResearchActionCompleted')
      const workflowCompleted = getPathValue(messages, 'chat.deepResearchWorkflowCompleted')

      expect(typeof stageCompleted, `${file} missing chat.deepResearchStageCompleted`).toBe(
        'string'
      )
      expect(typeof actionCompleted, `${file} missing chat.deepResearchActionCompleted`).toBe(
        'string'
      )
      expect(typeof workflowCompleted, `${file} missing chat.deepResearchWorkflowCompleted`).toBe(
        'string'
      )

      expect(localizeDeepResearchStatus('[completed]', translate), `${file} status []`).toBe(
        stageCompleted
      )
      expect(localizeDeepResearchStatus('【completed】', translate), `${file} status 【】`).toBe(
        stageCompleted
      )
      expect(localizeDeepResearchAction('[completed]', translate), `${file} action []`).toBe(
        actionCompleted
      )
      expect(localizeDeepResearchSegment('[completed]', translate), `${file} segment []`).toBe(
        stageCompleted
      )
    }
  })

  it('keeps deep research gap localization working for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    const cases: Array<[token: string, key: string]> = [
      ['need_evidence_coverage', 'chat.deepResearchGapNeedEvidenceCoverage'],
      ['need_primary_or_official_sources', 'chat.deepResearchGapNeedPrimaryOrOfficialSources'],
      ['need_broader_evidence_coverage', 'chat.deepResearchGapNeedBroaderEvidenceCoverage'],
      ['need_broader_source_diversity', 'chat.deepResearchGapNeedBroaderSourceDiversity'],
      ['need_fresher_sources', 'chat.deepResearchGapNeedFresherSources'],
      ['resolve_conflicting_claims', 'chat.deepResearchGapResolveConflictingClaims'],
    ]

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const translate = translateFor(messages)

      for (const [token, key] of cases) {
        const localized = getPathValue(messages, key)
        expect(typeof localized, `${file} missing ${key}`).toBe('string')
        expect(
          localizeDeepResearchGap(token, translate),
          `${file} failed to localize ${token}`
        ).toBe(localized)
      }

      const evidenceCoverage = getPathValue(messages, 'chat.deepResearchGapNeedEvidenceCoverage')
      const broaderSourceDiversity = getPathValue(
        messages,
        'chat.deepResearchGapNeedBroaderSourceDiversity'
      )

      expect(
        localizeDeepResearchGap(
          'need_evidence_coverage • need_broader_source_diversity',
          translate
        ),
        `${file} failed to localize combined gap string`
      ).toBe(`${evidenceCoverage} · ${broaderSourceDiversity}`)
    }
  })

  it('resolves deep research surface labels for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const translate = translateFor(messages)
      const deepResearchTitle = getPathValue(messages, 'chat.deepResearchTitle')
      const deepResearchProgress = getPathValue(messages, 'chat.deepResearchProgress')
      const deepResearchRunningTasks = getPathValue(messages, 'chat.deepResearchRunningTasks')
      const deepResearchRunningElsewhere = getPathValue(
        messages,
        'chat.deepResearchRunningElsewhere'
      )

      expect(typeof deepResearchTitle, `${file} missing chat.deepResearchTitle`).toBe('string')
      expect(typeof deepResearchProgress, `${file} missing chat.deepResearchProgress`).toBe(
        'string'
      )
      expect(typeof deepResearchRunningTasks, `${file} missing chat.deepResearchRunningTasks`).toBe(
        'string'
      )
      expect(
        typeof deepResearchRunningElsewhere,
        `${file} missing chat.deepResearchRunningElsewhere`
      ).toBe('string')

      expect(localizeResearchSurfaceTitle(translate), `${file} deep research title`).toBe(
        deepResearchTitle
      )
      expect(localizeResearchProgressLabel(translate), `${file} deep research progress`).toBe(
        deepResearchProgress
      )
      expect(
        localizeResearchRunningTasksLabel(translate),
        `${file} deep research running tasks`
      ).toBe(deepResearchRunningTasks)
      expect(
        localizeResearchRunningElsewhereLabel(translate),
        `${file} deep research running elsewhere`
      ).toBe(deepResearchRunningElsewhere)
    }
  })

  it('keeps deep research tool labels aligned for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const deepResearchTitle = getPathValue(messages, 'chat.deepResearchTitle')
      const resultCardResearch = getPathValue(messages, 'resultCard.titles.deep_research')
      const researchRunName = getPathValue(messages, 'tools.names.research_run')
      const researchStatusName = getPathValue(messages, 'tools.names.research_status')
      const researchRunDescription = getPathValue(messages, 'tools.descriptions.research_run')
      const researchStatusDescription = getPathValue(messages, 'tools.descriptions.research_status')

      expect(typeof deepResearchTitle, `${file} missing chat.deepResearchTitle`).toBe('string')
      expect(typeof resultCardResearch, `${file} missing resultCard.titles.deep_research`).toBe(
        'string'
      )
      expect(typeof researchRunName, `${file} missing tools.names.research_run`).toBe('string')
      expect(typeof researchStatusName, `${file} missing tools.names.research_status`).toBe(
        'string'
      )
      expect(typeof researchRunDescription, `${file} missing tools.descriptions.research_run`).toBe(
        'string'
      )
      expect(
        typeof researchStatusDescription,
        `${file} missing tools.descriptions.research_status`
      ).toBe('string')

      expect(resultCardResearch, `${file} result card deep research title`).toBe(deepResearchTitle)
      expect(researchRunName, `${file} deep research tool name`).toBe(deepResearchTitle)
      expect(
        (researchStatusName as string).trim().length,
        `${file} deep research status tool name`
      ).toBeGreaterThan(0)
      expect(
        (researchRunDescription as string).trim().length,
        `${file} deep research run description`
      ).toBeGreaterThan(0)
      expect(
        (researchStatusDescription as string).trim().length,
        `${file} deep research status description`
      ).toBeGreaterThan(0)
    }
  })

  it('keeps research hover copy present and free of Deep Research branding', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const hoverDescription = getPathValue(messages, 'chat.deepResearchHoverDescription')

      expect(typeof hoverDescription, `${file} missing chat.deepResearchHoverDescription`).toBe(
        'string'
      )
      expect(String(hoverDescription).trim().length, `${file} empty hover copy`).toBeGreaterThan(0)
      expect(
        String(hoverDescription),
        `${file} hover copy should not expose Deep Research branding`
      ).not.toMatch(/Deep Research|deep research|DeepResearch/)
    }
  })

  it('keeps deep research branding separate from generic harness research copy', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const messages = mod.default
      const translate = translateFor(messages)
      const uiDeepResearchTitle = getPathValue(messages, 'ui.deepResearchTitle')
      const chatDeepResearchTitle = getPathValue(messages, 'chat.deepResearchTitle')
      const chatDeepResearchProgress = getPathValue(messages, 'chat.deepResearchProgress')
      const chatDeepResearchRunningTasks = getPathValue(messages, 'chat.deepResearchRunningTasks')
      const chatDeepResearchRunningElsewhere = getPathValue(
        messages,
        'chat.deepResearchRunningElsewhere'
      )
      const processTraceDeepResearch = getPathValue(
        messages,
        'chat.processTrace.fields.deepResearch'
      )
      const deepResearchFallbacks = getPathValue(
        messages,
        'settings.smallModel.deepResearchFallbacks'
      )
      const researchTitle = getPathValue(messages, 'chat.researchTitle')
      expect(uiDeepResearchTitle, `${file} ui.deepResearchTitle`).toBe(chatDeepResearchTitle)
      expect(chatDeepResearchTitle, `${file} chat.deepResearchTitle`).not.toBe(researchTitle)
      expect(chatDeepResearchProgress, `${file} chat.deepResearchProgress`).toBe(
        localizeResearchProgressLabel(translate)
      )
      expect(chatDeepResearchRunningTasks, `${file} chat.deepResearchRunningTasks`).toBe(
        localizeResearchRunningTasksLabel(translate)
      )
      expect(chatDeepResearchRunningElsewhere, `${file} chat.deepResearchRunningElsewhere`).toBe(
        localizeResearchRunningElsewhereLabel(translate)
      )
      expect(processTraceDeepResearch, `${file} chat.processTrace.fields.deepResearch`).toBe(
        chatDeepResearchTitle
      )
      expect(
        typeof deepResearchFallbacks,
        `${file} settings.smallModel.deepResearchFallbacks`
      ).toBe('string')
      expect(
        String(deepResearchFallbacks),
        `${file} should not expose DeepResearch branding`
      ).not.toContain('DeepResearch')
    }
  })

  it('provides structured deep research value copy for all 27 locales at runtime', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    const requiredPaths = [
      'chat.deepResearchSourceTypeLaw',
      'chat.deepResearchSourceTypeFiling',
      'chat.deepResearchSourceTypePaper',
      'chat.deepResearchSourceTypeWeb',
      'chat.deepResearchMetaOfficial',
      'chat.deepResearchMetaFinancial',
      'chat.deepResearchWorkflowScope',
      'chat.deepResearchWorkflowSources',
      'chat.deepResearchWorkflowExtraction',
    ] as const

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const locale = file.replace(/\.ts$/, '')
      const runtimeMessages = resolveRuntimeMessages(locale, mod.default)

      for (const path of requiredPaths) {
        const value = getPathValue(runtimeMessages, path)
        expect(typeof value, `${file} missing runtime ${path}`).toBe('string')
        expect(String(value).trim().length, `${file} empty runtime ${path}`).toBeGreaterThan(0)
      }
    }
  })
})
