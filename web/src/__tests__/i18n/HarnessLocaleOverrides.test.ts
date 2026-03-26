import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeKeys = [
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
] as const

const requiredPaths = [
  'common.all',
  'common.notAvailable',
  'common.updatedAt',
  'nav.harness',
  'harness.builder.title',
  'harness.datasets.title',
  'harness.dataset.publishVersion',
  'harness.evalSpec.title',
  'harness.evalRuns.title',
  'harness.evalRun.report',
  'harness.quickEval.title',
  'harness.quickEval.systemWillDo',
  'harness.quickEval.caseRequired',
  'harness.quickEval.caseTemplateHint',
  'harness.quickEval.conversationCaseCount',
  'harness.quickEval.conversationEmpty',
  'harness.quickEval.conversationHint',
  'harness.quickEval.conversationLoadFailed',
  'harness.quickEval.conversationRequired',
  'harness.quickEval.draftCases',
  'harness.quickEval.editManifest',
  'harness.quickEval.launchHint',
  'harness.quickEval.previewManifest',
  'harness.quickEval.smokeLabel',
  'harness.quickEval.regressionLabel',
  'harness.quickEval.researchLabel',
  'harness.quickEval.selectConversation',
  'harness.quickEval.smokeDatasetName',
  'harness.quickEval.untitledConversation',
  'harness.quickEval.useConversation',
  'harness.baseline.title',
  'harness.compare.title',
  'harness.groups.subtitle',
  'harness.groups.controlPlaneDescription',
  'harness.groups.totalGroups',
  'harness.groups.emptyDescription',
  'harness.group.artifacts',
  'harness.group.retryFailed',
  'harness.group.verificationPassRate',
  'harness.group.evidenceBackedPassRate',
  'harness.group.retryRecovered',
  'harness.group.failureLabels',
  'harness.group.noFailureLabels',
  'harness.group.verification',
  'harness.group.evidenceScore',
  'harness.group.failureLabel',
  'harness.group.retryable',
  'harness.group.outcomeScore',
  'harness.group.executionScore',
  'harness.group.remediation',
  'harness.group.remediationMissingArtifact',
  'harness.group.remediationRunFailed',
  'harness.group.remediationGeneric',
  'harness.group.verificationChecks',
  'harness.group.expectedArtifacts',
  'harness.group.traceSummary',
  'harness.group.expectedValue',
  'harness.group.actualValue',
  'harness.group.eventCount',
  'harness.group.artifactCount',
  'harness.group.observedTools',
  'harness.group.scorecardInsights',
  'harness.group.scorecardInsightsHint',
  'harness.group.reviewProposals',
  'harness.group.noScorecardInsights',
  'harness.group.calibrationRef',
  'harness.group.takeawayCandidateCount',
  'harness.group.proposalCount',
  'harness.group.proposalIds',
  'harness.group.noProposalIds',
  'harness.group.unprofiled',
  'harness.compare.verificationPassRateDelta',
  'harness.compare.evidenceBackedPassRateDelta',
  'harness.compare.retryRecoveredDelta',
  'harness.compare.failureLabelDelta',
  'harness.compare.noFailureLabelDelta',
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

const localeMessagesByFile = new Map(
  Object.entries(localeModules).map(([modulePath, mod]) => [
    fileNameFromModulePath(modulePath),
    mod.default,
  ])
)

const localeSourceByFile = new Map(
  Object.entries(localeSourceModules).map(([modulePath, source]) => [
    fileNameFromModulePath(modulePath),
    source,
  ])
)

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('Harness locale coverage', () => {
  it('is wired through all 27 locale source files', () => {
    expect(localeMessagesByFile.size).toBe(27)

    for (const locale of localeKeys) {
      const file = `${locale}.ts`
      const source = localeSourceByFile.get(file)
      expect(source, `${file} should be loadable as raw source`).toBeTruthy()
      expect(source, `${file} should expose common.all`).toMatch(/"all"\s*:/)
      expect(source, `${file} should expose common.notAvailable`).toMatch(/"notAvailable"\s*:/)
      expect(source, `${file} should expose common.updatedAt`).toMatch(/"updatedAt"\s*:/)
      expect(source, `${file} should expose nav.harness`).toMatch(/"harness"\s*:/)
    }
  })

  it.each(localeKeys)('provides harness copy for %s', (locale) => {
    const file = `${locale}.ts`
    const mergedMessages = localeMessagesByFile.get(file)
    expect(mergedMessages, `${file} should be loadable via import.meta.glob`).toBeTruthy()

    for (const path of requiredPaths) {
      const value = getPathValue(mergedMessages as LocaleMessages, path)
      expect(typeof value).toBe('string')
      expect(String(value).trim().length).toBeGreaterThan(0)
    }
  })

  it('preserves the Harness product name for Chinese locales', () => {
    const zhCN = localeMessagesByFile.get('zh-CN.ts') as LocaleMessages
    const zhTW = localeMessagesByFile.get('zh-TW.ts') as LocaleMessages

    expect(getPathValue(zhCN, 'nav.harness')).toBe('Harness')
    expect(getPathValue(zhCN, 'harness.quickEval.systemWillDo')).toBe('Harness 将自动完成')
    expect(getPathValue(zhCN, 'harness.quickEval.useConversation')).toBe('使用会话')
    expect(getPathValue(zhCN, 'harness.quickEval.selectConversation')).toBe('选择会话')
    expect(getPathValue(zhCN, 'harness.quickEval.draftCases')).toBe('草稿用例')
    expect(getPathValue(zhCN, 'harness.quickEval.previewManifest')).toBe('预览用例 JSON')
    expect(getPathValue(zhCN, 'harness.quickEval.editManifest')).toBe('编辑用例 JSON')
    expect(getPathValue(zhCN, 'harness.quickEval.smokeLabel')).toBe('冒烟')
    expect(getPathValue(zhCN, 'harness.quickEval.regressionLabel')).toBe('回归')
    expect(getPathValue(zhCN, 'harness.quickEval.researchLabel')).toBe('研究')
    expect(getPathValue(zhCN, 'harness.quickEval.smokeDatasetName')).toBe('冒烟数据集')
    expect(getPathValue(zhTW, 'nav.harness')).toBe('Harness')
    expect(getPathValue(zhTW, 'harness.quickEval.systemWillDo')).toBe('Harness 將自動完成')
    expect(getPathValue(zhTW, 'harness.quickEval.useConversation')).toBe('使用對話')
    expect(getPathValue(zhTW, 'harness.quickEval.selectConversation')).toBe('選擇對話')
    expect(getPathValue(zhTW, 'harness.quickEval.draftCases')).toBe('草稿案例')
    expect(getPathValue(zhTW, 'harness.quickEval.previewManifest')).toBe('預覽案例 JSON')
    expect(getPathValue(zhTW, 'harness.quickEval.editManifest')).toBe('編輯案例 JSON')
    expect(getPathValue(zhTW, 'harness.quickEval.smokeLabel')).toBe('冒煙')
    expect(getPathValue(zhTW, 'harness.quickEval.regressionLabel')).toBe('回歸')
    expect(getPathValue(zhTW, 'harness.quickEval.researchLabel')).toBe('研究')
    expect(getPathValue(zhTW, 'harness.quickEval.smokeDatasetName')).toBe('冒煙資料集')
  })
})
