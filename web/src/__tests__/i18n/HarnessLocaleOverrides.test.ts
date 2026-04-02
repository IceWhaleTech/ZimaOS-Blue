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
  'common.active',
  'common.all',
  'common.cancel',
  'common.error',
  'common.loading',
  'common.notAvailable',
  'common.refresh',
  'common.updatedAt',
  'nav.automation',
  'automation.tabs.harness',
  'automation.tabs.harnessDesc',
  'harness.groups.subtitle',
  'harness.groups.totalGroups',
  'harness.groups.avgPassRate',
  'harness.groups.loading',
  'harness.groups.eyebrow',
  'harness.groups.title',
  'harness.groups.terminalOnly',
  'harness.groups.searchPlaceholder',
  'harness.groups.noSubject',
  'harness.groups.kind',
  'harness.groups.owner',
  'harness.groups.itemCount',
  'harness.groups.passRate',
  'harness.groups.score',
  'harness.groups.running',
  'harness.groups.failed',
  'harness.groups.passed',
  'harness.groups.emptyDescription',
  'harness.groups.startedAt',
  'harness.groups.finishedAt',
  'harness.group.noSubject',
  'harness.group.cancelled',
  'harness.group.retryResult',
  'harness.group.retryFailed',
  'harness.group.loading',
  'harness.group.totalAttempts',
  'harness.group.scoreDistribution',
  'harness.group.scoringMode',
  'harness.group.passVerdict',
  'harness.group.failVerdict',
  'harness.group.partialVerdict',
  'harness.group.errorVerdict',
  'harness.group.queuedCount',
  'harness.group.failedCount',
  'harness.group.failureLabel',
  'harness.group.noFailureLabels',
  'harness.group.failedItems',
  'harness.group.noFailedItems',
  'harness.group.attempts',
  'harness.group.outcomeTitle',
  'harness.group.outcomeDescription',
  'harness.group.noFailureReason',
  'harness.group.linkedRuns',
  'harness.group.noLinkedRuns',
  'harness.group.artifacts',
  'harness.group.noArtifacts',
  'harness.builder.title',
  'harness.datasets.title',
  'harness.evalSpecs.title',
  'harness.evalRuns.title',
  'harness.baseline.title',
  'harness.compare.title',
  'harness.evalRun.selectReportHint',
  'harness.quickEval.title',
  'chat.deepResearchHoverDescription',
  'chat.taskStagePartial',
  'chat.taskHarnessRunStatus',
  'chat.taskHarnessVerificationStatus',
  'chat.taskHarnessEvidenceCount',
  'chat.taskHarnessRecorded',
  'chat.taskHarnessViewDetails',
  'chat.taskHarnessRerun',
  'chat.taskHarnessVerificationPassed',
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
      expect(source, `${file} should expose common.all`).toMatch(/['"]?all['"]?\s*:/)
      expect(source, `${file} should expose common.notAvailable`).toMatch(
        /['"]?notAvailable['"]?\s*:/
      )
      expect(source, `${file} should expose common.updatedAt`).toMatch(/['"]?updatedAt['"]?\s*:/)
      expect(source, `${file} should expose nav.automation`).toMatch(/['"]?automation['"]?\s*:/)
      expect(source, `${file} should expose automation.tabs.harness`).toMatch(
        /['"]?harness['"]?\s*:/
      )
      expect(source, `${file} should expose automation.tabs.harnessDesc`).toMatch(
        /['"]?harnessDesc['"]?\s*:/
      )
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

  it('preserves the localized automation harness copy for Chinese locales', () => {
    const zhCN = localeMessagesByFile.get('zh-CN.ts') as LocaleMessages
    const zhTW = localeMessagesByFile.get('zh-TW.ts') as LocaleMessages

    expect(getPathValue(zhCN, 'automation.tabs.harness')).toBe('Harness')
    expect(getPathValue(zhCN, 'automation.tabs.harnessDesc')).toBe(
      '查看运行记录、评测分组和评分结果。'
    )
    expect(getPathValue(zhCN, 'harness.groups.eyebrow')).toBe('执行基底')
    expect(getPathValue(zhCN, 'harness.groups.title')).toBe('分组')
    expect(getPathValue(zhCN, 'harness.groups.totalGroups')).toBe('分组数')
    expect(getPathValue(zhCN, 'harness.group.outcomeTitle')).toBe('分组结果')
    expect(getPathValue(zhCN, 'harness.evalRun.selectReportHint')).toBe(
      '选择一个评测运行，即可打开关联报告、基线控制和对比视图。'
    )
    expect(getPathValue(zhCN, 'harness.group.retryFailed')).toBe('重试失败项')
    expect(getPathValue(zhCN, 'harness.group.failedItems')).toBe('失败样本')
    expect(getPathValue(zhTW, 'automation.tabs.harness')).toBe('Harness')
    expect(getPathValue(zhTW, 'automation.tabs.harnessDesc')).toBe(
      '查看執行記錄、評測群組和評分結果。'
    )
    expect(getPathValue(zhTW, 'harness.groups.eyebrow')).toBe('執行基底')
    expect(getPathValue(zhTW, 'harness.groups.title')).toBe('群組')
    expect(getPathValue(zhTW, 'harness.groups.totalGroups')).toBe('群組數')
    expect(getPathValue(zhTW, 'harness.group.outcomeTitle')).toBe('群組結果')
    expect(getPathValue(zhTW, 'harness.evalRun.selectReportHint')).toBe(
      '選擇一個評測執行，即可開啟關聯報告、基線控制與比較檢視。'
    )
    expect(getPathValue(zhTW, 'harness.group.retryFailed')).toBe('重試失敗項')
    expect(getPathValue(zhTW, 'harness.group.failedItems')).toBe('失敗樣本')
  })
})
