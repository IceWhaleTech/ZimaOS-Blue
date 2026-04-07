import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import type { LocaleKey } from '@/i18n/locale-catalog'
import fs from 'node:fs'

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

const localizedBundleLocaleKeys = localeKeys.filter(
  (locale) => locale !== 'en-US' && locale !== 'en-GB'
) as readonly LocaleKey[]

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
  'automation.tabs.knowledge',
  'automation.tabs.knowledgeDesc',
  'automation.tabs.evolution',
  'automation.tabs.evolutionDesc',
  'knowledge.eyebrow',
  'knowledge.title',
  'knowledge.description',
  'knowledge.pageListHint',
  'knowledge.askTitle',
  'knowledge.askHint',
  'knowledge.archiveAnswer',
  'evolution.title',
  'evolution.subtitle',
  'evolution.tabs.knowledge',
  'evolution.tabs.skills',
  'evolution.tabs.runner',
  'evolution.tabs.instructions',
  'evolution.lanes.knowledgeDescription',
  'evolution.lanes.skillsDescription',
  'evolution.lanes.runnerDescription',
  'evolution.lanes.instructionsDescription',
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
  'harness.groups.explainerTitle',
  'harness.groups.explainerPlainTitle',
  'harness.groups.explainerPlainBody',
  'harness.groups.explainerRoleTitle',
  'harness.groups.explainerRoleBody',
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
  'harness.dataset.importBundle',
  'harness.dataset.importBundleHint',
  'harness.dataset.githubBundle',
  'harness.dataset.localBundle',
  'harness.dataset.bundleImportBehavior',
  'harness.dataset.bundlePath',
  'harness.dataset.bundlePathHint',
  'harness.dataset.bundlePathOptionalHint',
  'harness.dataset.bundlePathRequired',
  'harness.dataset.bundleSource',
  'harness.dataset.bundleSourceHint',
  'harness.dataset.bundleSourceLabel',
  'harness.dataset.bundleSourceRequired',
  'harness.dataset.bundleImported',
  'harness.dataset.bundlePreviewRequired',
  'harness.dataset.previewBundle',
  'harness.dataset.previewCases',
  'harness.dataset.previewEvalSpecs',
  'harness.dataset.previewFailed',
  'harness.dataset.previewLoaded',
  'harness.dataset.previewLoading',
  'harness.dataset.previewReady',
  'harness.dataset.refreshPreview',
  'harness.dataset.versionOverride',
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
  'settings.agentcoreRunner.refPlaceholder',
  'settings.agentcoreRunner.refHint',
  'settings.agentcoreRunner.prepareHint',
] as const

const localizedOperatorPaths = [
  'automation.tabs.knowledge',
  'automation.tabs.evolution',
  'automation.title',
  'nav.automation',
  'evolution.tabs.knowledge',
  'evolution.tabs.skills',
  'evolution.tabs.runner',
  'evolution.lanes.knowledgeDescription',
  'settings.agentcoreRunner.source',
  'settings.agentcoreRunner.parts.orchestrator_policy',
  'harness.dataset.githubBundle',
  'harness.dataset.bundleSource',
  'harness.dataset.versionOverride',
] as const

const localizedWebTermLocales = [
  'ca-ES',
  'cs-CZ',
  'da-DK',
  'de-DE',
  'es-ES',
  'fr-FR',
  'hr-HR',
  'hu-HU',
  'it-IT',
  'nb-NO',
  'nl-NL',
  'pt-BR',
  'pt-PT',
  'ro-RO',
  'sk-SK',
] as const satisfies readonly LocaleKey[]

const localizedWebTermPaths = [
  'chat.deepResearchSourceTypeWeb',
  'companion.platforms.web',
  'tools.names.web',
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

function resolveRuntimeMessages(locale: string, messages: LocaleMessages): LocaleMessages {
  return mergeHarnessLocale(locale as LocaleKey, messages)
}

function getPathValue(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

function extractEvolutionViewKeys(): string[] {
  const source = fs.readFileSync(`${process.cwd()}/src/views/EvolutionView.vue`, 'utf8')
  const matches = source.matchAll(/trp?\(\s*'([^']+)'\s*,\s*'/g)
  const keys = new Set<string>()

  for (const match of matches) {
    const key = match[1]
    if (key?.startsWith('evolution.')) {
      keys.add(key)
    }
  }

  return [...keys].sort((a, b) => a.localeCompare(b))
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
    const runtimeMessages = resolveRuntimeMessages(locale, mergedMessages as LocaleMessages)

    for (const path of requiredPaths) {
      const value = getPathValue(runtimeMessages, path)
      expect(typeof value).toBe('string')
      expect(String(value).trim().length).toBeGreaterThan(0)
    }
  })

  it.each(localizedBundleLocaleKeys)(
    'localizes bundle import copy for %s',
    (locale) => {
      const file = `${locale}.ts`
      const mergedMessages = localeMessagesByFile.get(file)
      expect(mergedMessages, `${file} should be loadable via import.meta.glob`).toBeTruthy()
      const runtimeMessages = resolveRuntimeMessages(locale, mergedMessages as LocaleMessages)

      expect(getPathValue(runtimeMessages, 'harness.dataset.importBundle')).not.toBe(
        'Import bundle'
      )
      expect(getPathValue(runtimeMessages, 'harness.dataset.previewFailed')).not.toBe(
        'Bundle preview failed'
      )
      expect(getPathValue(runtimeMessages, 'harness.dataset.previewLoading')).not.toBe(
        'Checking bundle metadata, manifest, and bundled eval specs...'
      )
    }
  )

  it.each(localizedBundleLocaleKeys)(
    'keeps current Evolution and Runner labels localized for %s',
    (locale) => {
      const file = `${locale}.ts`
      const mergedMessages = localeMessagesByFile.get(file)
      expect(mergedMessages, `${file} should be loadable via import.meta.glob`).toBeTruthy()
      const runtimeMessages = resolveRuntimeMessages(locale, mergedMessages as LocaleMessages)
      const enUSRuntimeMessages = resolveRuntimeMessages(
        'en-US',
        localeMessagesByFile.get('en-US.ts') as LocaleMessages
      )

      for (const path of localizedOperatorPaths) {
        expect(getPathValue(runtimeMessages, path)).not.toBe(getPathValue(enUSRuntimeMessages, path))
      }
    }
  )

  it.each(localizedWebTermLocales)(
    'keeps runtime Web terminology localized for %s',
    (locale) => {
      const file = `${locale}.ts`
      const mergedMessages = localeMessagesByFile.get(file)
      expect(mergedMessages, `${file} should be loadable via import.meta.glob`).toBeTruthy()
      const runtimeMessages = resolveRuntimeMessages(locale, mergedMessages as LocaleMessages)
      const enUSRuntimeMessages = resolveRuntimeMessages(
        'en-US',
        localeMessagesByFile.get('en-US.ts') as LocaleMessages
      )

      for (const path of localizedWebTermPaths) {
        expect(getPathValue(runtimeMessages, path), `${file} should localize ${path}`).not.toBe(
          getPathValue(enUSRuntimeMessages, path)
        )
      }
    }
  )

  it('preserves the localized automation harness copy for Chinese locales', () => {
    const zhCN = resolveRuntimeMessages(
      'zh-CN',
      localeMessagesByFile.get('zh-CN.ts') as LocaleMessages
    )
    const zhTW = resolveRuntimeMessages(
      'zh-TW',
      localeMessagesByFile.get('zh-TW.ts') as LocaleMessages
    )

    expect(getPathValue(zhCN, 'nav.automation')).toBe('运维')
    expect(getPathValue(zhCN, 'automation.title')).toBe('运维')
    expect(getPathValue(zhCN, 'automation.tabs.harness')).toBe('Harness')
    expect(getPathValue(zhCN, 'automation.tabs.harnessDesc')).toBe(
      '查看运行记录、评测分组和评分结果。'
    )
    expect(getPathValue(zhCN, 'automation.tabs.knowledge')).toBe('知识')
    expect(getPathValue(zhCN, 'knowledge.title')).toBe('增量摄入、持久 wiki 页面与查询沉淀')
    expect(getPathValue(zhCN, 'knowledge.askTitle')).toBe('向编译后的知识空间提问')
    expect(getPathValue(zhCN, 'evolution.title')).toBe('自修复与演进控制台')
    expect(getPathValue(zhCN, 'evolution.tabs.knowledge')).toBe('知识')
    expect(getPathValue(zhCN, 'evolution.lanes.knowledgeDescription')).toBe(
      '以编译页面、有依据的查询与冲突或空洞信号作为整个工作台的知识底座。'
    )
    expect(getPathValue(zhCN, 'evolution.health.subtitle')).toBe(
      '快速查看当前轨道的筛选、审批与安全版本操作信号。'
    )
    expect(getPathValue(zhCN, 'evolution.runner.subtitle')).toContain('Runner 演进与技能演进分开追踪')
    expect(getPathValue(zhCN, 'evolution.instructions.subtitle')).toContain('AGENTS.md')
    expect(getPathValue(zhCN, 'evolution.skills.catalog')).toBe('规范技能')
    expect(getPathValue(zhCN, 'evolution.skills.scorecard')).toBe('评估评分卡')
    expect(getPathValue(zhCN, 'evolution.skills.cases')).toBe('演进案例')
    expect(getPathValue(zhCN, 'evolution.skills.revisions')).toBe('修订谱系')
    expect(getPathValue(zhCN, 'automation.tabs.evolution')).toBe('进化')
    expect(getPathValue(zhCN, 'evolution.tabs.skills')).toBe('技能')
    expect(getPathValue(zhCN, 'evolution.tabs.runner')).toBe('运行器')
    expect(getPathValue(zhCN, 'evolution.tabs.instructions')).toBe('审查队列')
    expect(getPathValue(zhCN, 'settings.agentcoreRunner.source')).toBe('GitHub 仓库与引用')
    expect(getPathValue(zhCN, 'harness.groups.eyebrow')).toBe('执行基底')
    expect(getPathValue(zhCN, 'harness.groups.title')).toBe('分组')
    expect(getPathValue(zhCN, 'harness.groups.totalGroups')).toBe('分组数')
    expect(getPathValue(zhCN, 'harness.groups.explainerTitle')).toBe('Harness是什么')
    expect(getPathValue(zhCN, 'harness.groups.explainerRoleTitle')).toBe('它在系统里做什么')
    expect(getPathValue(zhCN, 'harness.group.outcomeTitle')).toBe('分组结果')
    expect(getPathValue(zhCN, 'harness.evalRun.selectReportHint')).toBe(
      '选择一个评测运行，即可打开关联报告、基线控制和对比视图。'
    )
    expect(getPathValue(zhCN, 'harness.dataset.githubBundle')).toBe('GitHub 包')
    expect(getPathValue(zhCN, 'harness.dataset.importBundle')).toBe('导入 bundle')
    expect(getPathValue(zhCN, 'harness.dataset.bundleImported')).toBe('数据集 bundle 已导入')
    expect(getPathValue(zhCN, 'harness.dataset.previewBundle')).toBe('预览 bundle')
    expect(getPathValue(zhCN, 'harness.dataset.previewLoaded')).toBe('Bundle 预览已加载')
    expect(getPathValue(zhCN, 'harness.group.retryFailed')).toBe('重试失败项')
    expect(getPathValue(zhCN, 'harness.group.failedItems')).toBe('失败样本')
    expect(getPathValue(zhTW, 'nav.automation')).toBe('運維')
    expect(getPathValue(zhTW, 'automation.title')).toBe('運維')
    expect(getPathValue(zhTW, 'automation.tabs.harness')).toBe('Harness')
    expect(getPathValue(zhTW, 'automation.tabs.harnessDesc')).toBe(
      '查看執行記錄、評測群組和評分結果。'
    )
    expect(getPathValue(zhTW, 'automation.tabs.knowledge')).toBe('知識')
    expect(getPathValue(zhTW, 'knowledge.title')).toBe('編譯頁面、檢查健康與提問歸檔')
    expect(getPathValue(zhTW, 'knowledge.askTitle')).toBe('向編譯後的知識空間提問')
    expect(getPathValue(zhTW, 'evolution.title')).toBe('自我修復與演進主控台')
    expect(getPathValue(zhTW, 'evolution.tabs.knowledge')).toBe('知識')
    expect(getPathValue(zhTW, 'evolution.lanes.knowledgeDescription')).toBe(
      '以編譯頁面、有依據的查詢與衝突或空洞訊號作為整個工作台的知識底座。'
    )
    expect(getPathValue(zhTW, 'evolution.health.subtitle')).toBe(
      '快速查看目前軌道的篩選、審批與安全版本操作訊號。'
    )
    expect(getPathValue(zhTW, 'evolution.runner.subtitle')).toContain('Runner 演進與技能演進分開追蹤')
    expect(getPathValue(zhTW, 'evolution.instructions.subtitle')).toContain('AGENTS.md')
    expect(getPathValue(zhTW, 'evolution.skills.catalog')).toBe('規範技能')
    expect(getPathValue(zhTW, 'evolution.skills.scorecard')).toBe('評估評分卡')
    expect(getPathValue(zhTW, 'evolution.skills.cases')).toBe('演進案例')
    expect(getPathValue(zhTW, 'evolution.skills.revisions')).toBe('修訂譜系')
    expect(getPathValue(zhTW, 'automation.tabs.evolution')).toBe('演進')
    expect(getPathValue(zhTW, 'evolution.tabs.skills')).toBe('技能')
    expect(getPathValue(zhTW, 'evolution.tabs.runner')).toBe('執行器')
    expect(getPathValue(zhTW, 'evolution.tabs.instructions')).toBe('審查佇列')
    expect(getPathValue(zhTW, 'settings.agentcoreRunner.source')).toBe('GitHub 儲存庫與引用')
    expect(getPathValue(zhTW, 'harness.groups.explainerTitle')).toBe('Harness是什麼')
    expect(getPathValue(zhTW, 'harness.groups.explainerRoleTitle')).toBe('它在系統裡做什麼')
    expect(getPathValue(zhTW, 'harness.groups.eyebrow')).toBe('執行基底')
    expect(getPathValue(zhTW, 'harness.groups.title')).toBe('群組')
    expect(getPathValue(zhTW, 'harness.groups.totalGroups')).toBe('群組數')
    expect(getPathValue(zhTW, 'harness.group.outcomeTitle')).toBe('群組結果')
    expect(getPathValue(zhTW, 'harness.evalRun.selectReportHint')).toBe(
      '選擇一個評測執行，即可開啟關聯報告、基線控制與比較檢視。'
    )
    expect(getPathValue(zhTW, 'harness.dataset.githubBundle')).toBe('GitHub 套件')
    expect(getPathValue(zhTW, 'harness.dataset.importBundle')).toBe('匯入 bundle')
    expect(getPathValue(zhTW, 'harness.dataset.bundleImported')).toBe('資料集 bundle 已匯入')
    expect(getPathValue(zhTW, 'harness.dataset.previewBundle')).toBe('預覽 bundle')
    expect(getPathValue(zhTW, 'harness.dataset.previewLoaded')).toBe('Bundle 預覽已載入')
    expect(getPathValue(zhTW, 'harness.group.retryFailed')).toBe('重試失敗項')
    expect(getPathValue(zhTW, 'harness.group.failedItems')).toBe('失敗樣本')
  })

  it('keeps EvolutionView Chinese locales off the en-US fallback copy', () => {
    const evolutionPaths = extractEvolutionViewKeys()
    const enUSMessages = resolveRuntimeMessages('en-US', localeMessagesByFile.get('en-US.ts')!)
    const zhCNMessages = resolveRuntimeMessages('zh-CN', localeMessagesByFile.get('zh-CN.ts')!)
    const zhTWMessages = resolveRuntimeMessages('zh-TW', localeMessagesByFile.get('zh-TW.ts')!)

    const missingByLocale = {
      'zh-CN': [] as string[],
      'zh-TW': [] as string[],
    }

    for (const path of evolutionPaths) {
      const enValue = getPathValue(enUSMessages, path)
      if (typeof enValue !== 'string') {
        continue
      }

      if (getPathValue(zhCNMessages, path) === enValue) {
        missingByLocale['zh-CN'].push(path)
      }

      if (getPathValue(zhTWMessages, path) === enValue) {
        missingByLocale['zh-TW'].push(path)
      }
    }

    expect(missingByLocale['zh-CN'], 'zh-CN should not fall back to en-US on EvolutionView').toEqual(
      []
    )
    expect(missingByLocale['zh-TW'], 'zh-TW should not fall back to en-US on EvolutionView').toEqual(
      []
    )
  })
})
