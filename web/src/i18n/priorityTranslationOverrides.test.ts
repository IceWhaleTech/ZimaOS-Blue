import { describe, expect, it } from 'vitest'

import { deepMergeMessages, type LocaleMessages } from './merge'
import priorityBillingOverrides from './priority-billing-overrides'
import priorityLocaleOverrides from './priority-overrides'
import prioritySettingsOverrides from './priority-settings-overrides'
import prioritySmallModelOverrides from './priority-small-model-overrides'
import priorityTranslationOverrides from './priority-translation-overrides'

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

const localeFiles = Object.keys(localeModules).map(fileNameFromModulePath).sort()
const localeMessagesByCode = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    localeCodeFromFile(fileNameFromModulePath(modulePath)),
    mod.default,
  ])
)

const localeCodes = [...localeMessagesByCode.keys()].sort()
const baseLocale = localeMessagesByCode.get('en-US') as LocaleMessages

const voiceWakeKeys = [
  'speech.voiceWake.title',
  'speech.voiceWake.description',
  'speech.voiceWake.desktopNoteDescription',
  'speech.voiceWake.enableTitle',
  'speech.voiceWake.enableDescription',
  'speech.voiceWake.statusRunning',
  'speech.voiceWake.statusIdle',
  'speech.voiceWake.speechStatusLabel',
  'speech.voiceWake.microphoneStatusLabel',
  'speech.voiceWake.statusReady',
  'speech.voiceWake.statusMissing',
  'speech.voiceWake.statusNotReady',
  'speech.voiceWake.messages.running',
  'speech.voiceWake.messages.disabled',
  'speech.voiceWake.messages.targetMissing',
  'speech.voiceWake.messages.targetUnavailable',
  'speech.voiceWake.messages.speechPermissionDenied',
  'speech.voiceWake.messages.microphoneUnavailable',
  'speech.voiceWake.messages.startFailed',
  'speech.voiceWake.messages.sendFailed',
  'speech.voiceWake.messages.runtimeError',
  'speech.voiceWake.messages.unsupported',
  'speech.voiceWake.messages.unavailable',
  'speech.voiceWake.lastError',
  'speech.voiceWake.activeTriggers',
  'speech.voiceWake.targetConversation',
  'speech.voiceWake.lastTriggered',
  'speech.voiceWake.lastSent',
  'speech.voiceWake.wakeWords',
  'speech.voiceWake.wakeWordsHelp',
  'speech.voiceWake.listeningTitle',
  'speech.voiceWake.listeningDescription',
  'speech.voiceWake.localeOverride',
  'speech.voiceWake.localeHelp',
  'speech.voiceWake.selectConversation',
  'speech.voiceWake.currentChatPrefix',
  'speech.voiceWake.savedTargetPrefix',
  'speech.voiceWake.useCurrentChat',
  'speech.voiceWake.routingTitle',
  'speech.voiceWake.targetFieldLabel',
  'speech.voiceWake.targetDescription',
  'speech.voiceWake.openTargetChat',
  'speech.voiceWake.fixedTargetWarning',
  'speech.voiceWake.openSpeechSettings',
  'speech.voiceWake.openMicrophoneSettings',
  'speech.voiceWake.chooseConversation',
  'speech.voiceWake.notSelected',
  'speech.voiceWake.never',
] as const

const baseLocaleKeys = [
  'common.saveFailed',
  'common.success',
  'chat.taskLoop',
  'chat.deepResearchParallelism',
  'chat.deepResearchCalibrationPublish',
  'chat.deepResearchCalibrationCaution',
  'chat.deepResearchCalibrationConflictBlocking',
  'chat.deepResearchCalibrationConflictLow',
  'chat.deepResearchExpandDetails',
  'chat.deepResearchCollapseDetails',
  'chat.deepResearchGapNeedEvidenceCoverage',
  'chat.deepResearchGapNeedPrimaryOrOfficialSources',
  'chat.deepResearchGapNeedBroaderEvidenceCoverage',
  'chat.deepResearchGapNeedBroaderSourceDiversity',
  'chat.deepResearchGapNeedFresherSources',
  'chat.deepResearchGapResolveConflictingClaims',
  'chat.deepResearchSearchQuery',
  'chat.deepResearchFocus',
  'chat.presetQuestions.title',
  'chat.presetQuestions.focus',
  'chat.presetQuestions.interests.personalKnowledge',
  'chat.presetQuestions.interests.learningGrowth',
  'chat.presetQuestions.interests.contentCreation',
  'chat.presetQuestions.interests.marketInvesting',
  'chat.presetQuestions.interests.productDesign',
  'chat.presetQuestions.interests.userResearch',
  'chat.presetQuestions.interests.psychologicalExploration',
  'chat.presetQuestions.interests.philosophicalDialogue',
  'resourceChart.recentTrend',
  'settings.workspaceBasics',
  'settings.providerMatrix',
  'settings.codingRuntime',
  'settings.advancedCodingTools',
  'settings.requestFlow',
  'settings.proxyRouting',
  'settings.assistiveRouting',
  'settings.localAcceleration',
  'settings.voicePipeline',
  'settings.portability',
  'settings.userDataExchange',
  'settings.recoveryRail',
  'settings.backupRecovery',
  'settings.tabDescriptions.general',
  'settings.tabDescriptions.llm',
  'settings.tabDescriptions.proxy',
  'settings.tabDescriptions.speech',
  'settings.tabDescriptions.userdata',
  ...voiceWakeKeys,
  'security.tabs.controls',
  'security.tabDescriptions.overview',
  'security.tabDescriptions.controls',
  'security.tabDescriptions.approvals',
  'security.tabDescriptions.firewall',
  'security.tabDescriptions.network',
  'security.tabDescriptions.masking',
  'security.tabDescriptions.monitoring',
  'security.tabDescriptions.logs',
] as const

const overrideKeys = [
  'chat.deepResearchParallelism',
  'chat.deepResearchCalibrationPublish',
  'chat.deepResearchCalibrationCaution',
  'chat.deepResearchCalibrationConflictBlocking',
  'chat.deepResearchCalibrationConflictLow',
  'chat.deepResearchExpandDetails',
  'chat.deepResearchCollapseDetails',
  'chat.deepResearchGapNeedEvidenceCoverage',
  'chat.deepResearchGapNeedPrimaryOrOfficialSources',
  'chat.deepResearchGapNeedBroaderEvidenceCoverage',
  'chat.deepResearchGapNeedBroaderSourceDiversity',
  'chat.deepResearchGapNeedFresherSources',
  'chat.deepResearchGapResolveConflictingClaims',
  'chat.deepResearchSearchQuery',
  'chat.deepResearchFocus',
  'chat.presetQuestions.title',
  'chat.presetQuestions.focus',
  'chat.presetQuestions.interests.personalKnowledge',
  'chat.presetQuestions.interests.learningGrowth',
  'chat.presetQuestions.interests.contentCreation',
  'chat.presetQuestions.interests.marketInvesting',
  'chat.presetQuestions.interests.productDesign',
  'chat.presetQuestions.interests.userResearch',
  'chat.presetQuestions.interests.psychologicalExploration',
  'chat.presetQuestions.interests.philosophicalDialogue',
  'resourceChart.recentTrend',
  'settings.workspaceBasics',
  'settings.providerMatrix',
  'settings.codingRuntime',
  'settings.advancedCodingTools',
  'settings.requestFlow',
  'settings.proxyRouting',
  'settings.assistiveRouting',
  'settings.localAcceleration',
  'settings.voicePipeline',
  'settings.portability',
  'settings.userDataExchange',
  'settings.recoveryRail',
  'settings.backupRecovery',
  'settings.tabDescriptions.general',
  'settings.tabDescriptions.llm',
  'settings.tabDescriptions.proxy',
  'settings.tabDescriptions.speech',
  'settings.tabDescriptions.userdata',
  ...voiceWakeKeys,
  'security.tabDescriptions.controls',
  'security.tabDescriptions.overview',
  'security.tabDescriptions.approvals',
  'security.tabDescriptions.firewall',
  'security.tabDescriptions.network',
  'security.tabDescriptions.masking',
  'security.tabDescriptions.monitoring',
  'security.tabDescriptions.logs',
] as const

const wakeWordPlaceholderKey = 'voiceView.wakeWordPlaceholder'
const directLocaleKeys = ['security.tabs.controls'] as const

function buildMergedLocaleMessages(locale: string): LocaleMessages {
  const localeMessages = localeMessagesByCode.get(locale)
  if (!localeMessages) {
    throw new Error(`Missing locale: ${locale}`)
  }

  const localeBase = locale === 'en-US' ? baseLocale : deepMergeMessages(baseLocale, localeMessages)
  const withPriorityOverrides = deepMergeMessages(
    localeBase,
    (priorityLocaleOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
  const withBillingOverrides = deepMergeMessages(
    withPriorityOverrides,
    (priorityBillingOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
  const withSettingsOverrides = deepMergeMessages(
    withBillingOverrides,
    (prioritySettingsOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
  const withTranslationOverrides = deepMergeMessages(
    withSettingsOverrides,
    (priorityTranslationOverrides as Record<string, LocaleMessages>)[locale] || {}
  )

  return deepMergeMessages(
    withTranslationOverrides,
    (prioritySmallModelOverrides as Record<string, LocaleMessages>)[locale] || {}
  )
}

describe('priority translation override coverage', () => {
  it('keeps the newly added core keys in the primary locale files', () => {
    expect(localeFiles.length).toBe(27)

    for (const locale of ['en-US', 'zh-CN'] as const) {
      const messages = localeMessagesByCode.get(locale)
      expect(messages, `${locale} should be loadable`).toBeTruthy()

      for (const key of baseLocaleKeys) {
        const value = getByPath(messages as LocaleMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('declares locale-specific security control tab labels in all 27 locale files', () => {
    for (const locale of localeCodes) {
      const messages = localeMessagesByCode.get(locale)
      expect(messages, `${locale} should be loadable`).toBeTruthy()

      for (const key of directLocaleKeys) {
        const value = getByPath(messages as LocaleMessages, key)
        expect(typeof value, `${locale} should declare ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('defines the recent translation keys for every non-primary locale in the dedicated override file', () => {
    const overrideLocales = Object.keys(priorityTranslationOverrides)
      .filter((locale) => !['en-US', 'zh-CN'].includes(locale))
      .sort()
    const expectedLocales = localeCodes
      .filter((locale) => !['en-US', 'zh-CN'].includes(locale))
      .sort()

    expect(overrideLocales).toEqual(expectedLocales)

    for (const locale of expectedLocales) {
      const overrides = (
        priorityTranslationOverrides as Record<string, LocaleMessages | undefined>
      )[locale]
      expect(overrides, `${locale} should have dedicated translation overrides`).toBeTruthy()
      const localeOverrides = overrides as LocaleMessages

      for (const key of overrideKeys) {
        const value = getByPath(localeOverrides, key)
        expect(typeof value, `${locale} should override ${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('exposes the recent translation keys in every merged locale', () => {
    for (const locale of localeCodes) {
      const messages = buildMergedLocaleMessages(locale)

      for (const key of baseLocaleKeys) {
        const value = getByPath(messages, key)
        expect(typeof value, `${locale} should expose ${key} after merge`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} should not leave ${key} empty after merge`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the Hey Blue wake-word placeholder aligned across all 27 locales', () => {
    for (const locale of ['en-US', 'zh-CN'] as const) {
      const messages = localeMessagesByCode.get(locale)
      const value = getByPath(messages as LocaleMessages, wakeWordPlaceholderKey)
      expect(typeof value, `${locale} should declare ${wakeWordPlaceholderKey}`).toBe('string')
      expect(String(value)).toContain('Hey Blue')
    }

    for (const locale of localeCodes.filter((code) => code !== 'en-US')) {
      const overrides = (prioritySettingsOverrides as Record<string, LocaleMessages | undefined>)[
        locale
      ]
      expect(overrides, `${locale} should have settings overrides`).toBeTruthy()

      const overrideValue = getByPath(overrides as LocaleMessages, wakeWordPlaceholderKey)
      expect(typeof overrideValue, `${locale} should override ${wakeWordPlaceholderKey}`).toBe(
        'string'
      )
      expect(String(overrideValue)).toContain('Hey Blue')

      const mergedValue = getByPath(buildMergedLocaleMessages(locale), wakeWordPlaceholderKey)
      expect(
        typeof mergedValue,
        `${locale} should expose ${wakeWordPlaceholderKey} after merge`
      ).toBe('string')
      expect(String(mergedValue)).toContain('Hey Blue')
    }
  })
})
