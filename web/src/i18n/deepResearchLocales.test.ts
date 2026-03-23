import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import prioritySmallModelOverrides from './priority-small-model-overrides'
import priorityTranslationOverrides from './priority-translation-overrides'
import selfReflectProposalOverrides from './self-reflect-proposal-overrides'

const requiredDeepResearchKeys = [
  'ui.deepResearchTitle',
  'chat.deepResearchTitle',
  'chat.deepResearchProgress',
  'chat.deepResearchRunningTasks',
  'chat.deepResearchRunningElsewhere',
  'chat.deepResearchBackToTask',
  'chat.deepResearchViewTask',
  'chat.deepResearchCancelTask',
  'chat.deepResearchTaskCompleted',
  'chat.deepResearchTaskFailed',
  'chat.deepResearchTaskCancelled',
  'chat.deepResearchEvidence',
  'chat.deepResearchSupport',
  'chat.deepResearchConflict',
  'chat.deepResearchHasConflict',
  'chat.deepResearchCitationCoverage',
  'chat.deepResearchStatus',
  'chat.deepResearchIterations',
  'chat.deepResearchLatestAction',
  'chat.deepResearchLatestGap',
  'chat.deepResearchDetails',
  'chat.deepResearchProcess',
  'chat.deepResearchParallelism',
  'chat.deepResearchResearchBrief',
  'chat.deepResearchMustVerify',
  'chat.deepResearchRetryGuidance',
  'chat.deepResearchRetryQueries',
  'chat.deepResearchPlannedTasks',
  'chat.deepResearchLiveSources',
  'chat.deepResearchFollowUpQuery',
  'chat.deepResearchExpandDetails',
  'chat.deepResearchCollapseDetails',
  'chat.deepResearchFocus',
  'chat.deepResearchSearchQuery',
  'chat.deepResearchCitations',
  'chat.deepResearchTimeline',
  'chat.deepResearchWorkflowPhases',
  'chat.deepResearchWorkflowCompleted',
  'chat.deepResearchWorkflowCurrent',
  'chat.deepResearchWorkflowPending',
  'chat.deepResearchSourceInventory',
  'chat.deepResearchPublishedAt',
  'chat.deepResearchFetchedAt',
  'chat.deepResearchRelevance',
  'chat.deepResearchCredibility',
  'chat.deepResearchCoverageSummary',
  'chat.deepResearchTasks',
  'chat.deepResearchDomains',
  'chat.deepResearchObjectMap',
  'chat.deepResearchOpenQuestions',
  'chat.deepResearchIteration',
  'chat.deepResearchTraceEntries',
  'chat.deepResearchEvidenceAdded',
  'chat.deepResearchVerificationSummary',
  'chat.deepResearchVerificationResolved',
  'chat.deepResearchVerificationConflicted',
  'chat.deepResearchCalibrationPublish',
  'chat.deepResearchCalibrationCaution',
  'chat.deepResearchCalibrationConflictBlocking',
  'chat.deepResearchCalibrationConflictLow',
  'chat.deepResearchVerificationInsufficient',
  'chat.deepResearchStageErrors',
  'chat.deepResearchStageIntake',
  'chat.deepResearchStagePlanning',
  'chat.deepResearchStageRetrieve',
  'chat.deepResearchStageVerify',
  'chat.deepResearchStageSynthesize',
  'chat.deepResearchStageCompleted',
  'chat.deepResearchStageFailed',
  'chat.deepResearchStageCancelled',
  'chat.deepResearchStopReason',
  'chat.deepResearchStopReasonCoverage',
  'chat.deepResearchStopReasonNoNewEvidence',
  'chat.deepResearchStopReasonBudget',
  'chat.deepResearchActionAugmentQuery',
  'chat.deepResearchActionInitialRetrieve',
  'chat.deepResearchActionFollowupRetrieve',
  'chat.deepResearchActionVerification',
  'chat.deepResearchActionVerificationCompleted',
  'chat.deepResearchActionFollowupPlanned',
  'chat.deepResearchActionLoopStopped',
  'chat.deepResearchActionSynthesizing',
  'chat.deepResearchActionCompleted',
  'chat.deepResearchModeFast',
  'chat.deepResearchModeStandard',
  'chat.deepResearchModeDeep',
  'chat.deepResearchHoverDescription',
  'settings.smallModel.deepResearchFallbacks',
  'memory.proposalCalibration',
  'memory.proposalCoverage',
  'memory.proposalGroundedness',
  'memory.proposalFreshness',
  'memory.proposalConfidence',
  'memory.proposalConflictRisk',
  'memory.proposalCandidateCount',
] as const

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

function localeCodeFromFile(fileName: string): string {
  return fileName.replace(/\.ts$/, '')
}

function getByPath(source: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, part) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as Record<string, unknown>)[part]
  }, source)
}

const localeModules = import.meta.glob<{ default: LocaleMessages }>('./locales/*.ts', {
  eager: true,
})

const localeMessagesByCode = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    localeCodeFromFile(fileNameFromModulePath(modulePath)),
    mod.default,
  ])
)

const localeCodes = [...localeMessagesByCode.keys()].sort()
const baseLocale = localeMessagesByCode.get('en-US') as LocaleMessages

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  const withSelfReflectOverrides = deepMergeMessages(
    localeBase,
    (selfReflectProposalOverrides as Record<string, LocaleMessages>)[locale] || {}
  )

  const withPrioritySmallModelOverrides = deepMergeMessages(
    withSelfReflectOverrides,
    (priorityTranslationOverrides as Record<string, LocaleMessages>)[locale] || {}
  )

  return deepMergeMessages(
    withPrioritySmallModelOverrides,
    (prioritySmallModelOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('deep research locale coverage', () => {
  it('exposes deep research card labels in all 27 merged locales', () => {
    expect(localeCodes).toHaveLength(27)

    for (const locale of localeCodes) {
      const localeMessages = buildMergedLocaleMessages(locale)
      expect(localeMessages, `${locale} should be loadable`).toBeTruthy()

      for (const key of requiredDeepResearchKeys) {
        const value = getByPath(localeMessages as LocaleMessages, key)
        expect(typeof value, `${locale} should expose ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})
