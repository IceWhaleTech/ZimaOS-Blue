import { describe, expect, it } from 'vitest'

import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import type { LocaleKey } from '@/i18n/locale-catalog'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function localeFromModulePath(modulePath: string): LocaleKey {
  return fileNameFromModulePath(modulePath).replace(/\.ts$/, '') as LocaleKey
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

const protectedEvolutionPaths = [
  {
    path: 'evolution.skills.timeline',
    englishValue: 'Case Lifecycle',
  },
  {
    path: 'evolution.skills.lineage',
    englishValue: 'Revision Lineage Summary',
  },
  {
    path: 'evolution.noDiff',
    englishValue: 'No candidate patch available yet.',
  },
  {
    path: 'evolution.skills.caseStatus.open',
    englishValue: 'Open intake, waiting candidate',
    disallowPattern: /\bintake\b/i,
  },
  {
    path: 'evolution.skills.timelineHint',
    englishValue:
      'Follow the selected case from intake through candidate creation, gate decision, and final promotion without reconstructing the history from scattered timestamps.',
    disallowPattern: /\bintake\b/i,
  },
  {
    path: 'evolution.runner.links.followupEval',
    englishValue: 'Follow-up eval run',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.links.openFollowupEval',
    englishValue: 'Open follow-up eval',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.links.subtitle',
    englishValue:
      'Use these links to jump from the runner evidence into the follow-up eval, the linked skill revision, or the original source eval that produced the optimization trigger.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.summary.followupHint',
    englishValue:
      'The linked follow-up eval decides whether the candidate is accepted, rejected, or still running.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.metrics.followupTitle',
    englishValue: 'Follow-up Eval Snapshot',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.metrics.followupSubtitle',
    englishValue:
      'These metrics come from the linked follow-up eval and should drive the accept or reject decision before any human promote step.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.runner.metrics.empty',
    englishValue: 'No structured follow-up eval metrics are attached yet.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.skills.comparisonHint',
    englishValue:
      'When a follow-up eval includes a baseline, these deltas make it clear why the selected revision looks better or worse than the previous version.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.skills.metricsHint',
    englishValue:
      'Quality, runtime, and token metrics come from the linked follow-up eval report when available.',
    disallowPattern: /follow-up eval/i,
  },
  {
    path: 'evolution.skills.scorecardEvidenceHint',
    englishValue: 'No structured evidence summary is attached to this revision yet.',
    disallowPattern: /structured evidence summary/i,
  },
  {
    path: 'evolution.skills.scorecardOpenComparison',
    englishValue: 'Open comparison',
    disallowPattern: /\bopen comparison\b/i,
  },
  {
    path: 'evolution.skills.scorecardOpenMetrics',
    englishValue: 'Open metrics',
    disallowPattern: /\bopen metrics\b/i,
  },
  {
    path: 'evolution.skills.scorecardOpenEvidence',
    englishValue: 'Open evidence',
    disallowPattern: /\bopen evidence\b/i,
  },
] as const

describe('Evolution locale coverage', () => {
  it('keeps requested Evolution labels localized for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const file = fileNameFromModulePath(modulePath)
      const runtimeMessages = mergeHarnessLocale(locale, mod.default)

      for (const { path, englishValue, disallowPattern } of protectedEvolutionPaths) {
        const localizedValue = getPathValue(runtimeMessages, path)

        expect(typeof localizedValue, `${file} missing ${path}`).toBe('string')
        expect(String(localizedValue).trim().length, `${file} empty ${path}`).toBeGreaterThan(0)

        if (locale === 'en-US') {
          expect(localizedValue, `${file} English copy for ${path}`).toBe(englishValue)
          continue
        }

        if (locale === 'en-GB') {
          continue
        }

        expect(localizedValue, `${file} should not fall back to en-US for ${path}`).not.toBe(
          englishValue
        )

        if (disallowPattern) {
          expect(
            String(localizedValue),
            `${file} should not keep English token leaks for ${path}`
          ).not.toMatch(disallowPattern)
        }
      }
    }
  })
})
