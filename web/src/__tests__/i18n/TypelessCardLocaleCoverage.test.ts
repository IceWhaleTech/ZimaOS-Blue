import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from '@/i18n/harness-locale-additions'
import type { LocaleKey } from '@/i18n/locale-catalog'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

const requiredPaths = [
  'accordionCard.thinking',
  'askQuestion.browserCheckpoint.screenshotAlt',
  'askQuestion.other',
  'askQuestion.otherPlaceholder',
  'browserProgress.title',
  'browserProgress.steps.screenshot',
  'browserProgress.steps.recipe',
  'codeBlock.toolCall',
  'countdownCard.expired',
  'countdownCard.days',
  'countdownCard.hours',
  'countdownCard.minutes',
  'countdownCard.seconds',
  'diffCard.unified',
  'diffCard.split',
  'diffCard.original',
  'diffCard.modified',
  'execCard.outputUnavailable',
  'execCard.running',
  'execCard.noOutput',
  'execCard.title',
  'execCard.output',
  'execCard.duration',
  'execCard.exitCode',
  'execCard.session',
  'execCard.lines',
  'execCard.outputTruncated',
  'execCard.riskLabel',
  'execCard.command',
  'execCard.commandHidden',
  'execCard.noCommand',
  'execCard.hideCommand',
  'execCard.showCommand',
  'execCard.copied',
  'execCard.copyCommand',
  'execCard.collapse',
  'execCard.expand',
  'mapCard.openInMaps',
  'media.modelDownload.status.downloading',
  'media.modelDownload.status.ready',
  'media.modelDownload.status.error',
  'media.modelDownload.status.pending',
  'media.modelDownload.auto.fileName',
  'media.modelDownload.completed',
  'media.modelDownload.pending',
  'mermaid.copyCode',
  'search.summaryTitle',
  'search.resultCount',
  'search.moreResults',
  'search.emptyState',
  'search.partialState',
  'speech.convertTask.task',
  'speech.convertTask.sources',
  'speech.convertTask.cancelling',
  'speech.convertTask.downloadAudio',
  'speech.convertTask.downloadVideo',
  'speech.convertTask.downloadPdf',
  'speech.convertTask.downloadText',
  'speech.convertTask.downloadFile',
  'speech.convertTask.action.convert',
  'speech.convertTask.action.merge',
  'speech.convertTask.action.split',
  'speech.convertTask.action.trim',
  'speech.convertTask.action.extractAudio',
  'speech.convertTask.action.extractFrames',
  'speech.convertTask.action.tts',
  'speech.convertTask.action.asr',
  'speech.convertTask.previewKind.file',
  'speech.convertTask.previewKind.audio',
  'speech.convertTask.previewKind.video',
  'speech.convertTask.previewKind.image',
  'speech.convertTask.previewKind.pdf',
  'speech.convertTask.previewKind.text',
  'speech.convertTask.message.queued',
  'speech.convertTask.message.processing',
  'speech.convertTask.message.completed',
  'speech.convertTask.message.taskCancelled',
  'speech.convertTask.message.taskFailed',
  'speech.convertTask.message.convertCompleted',
  'speech.convertTask.message.mergeCompleted',
  'speech.convertTask.message.splitCompleted',
  'speech.convertTask.message.trimCompleted',
  'speech.convertTask.message.extractAudioCompleted',
  'speech.convertTask.message.extractFramesCompleted',
  'speech.convertTask.message.ttsCompleted',
  'speech.convertTask.message.asrCompleted',
  'terminalCard.title',
  'toolWarnings.warning',
  'uiReview.reviewing',
  'uiReview.error',
  'uiReview.title',
  'uiReview.issues',
  'uiReview.skipped',
  'uiReview.visual',
  'uiReview.functional',
  'uiReview.accessibility',
  'uiReview.issuesTitle',
  'uiReview.suggestions',
  'uiReview.showScreenshot',
  'uiReview.hideScreenshot',
  'webFetchCard.title',
  'webFetchCard.copyUrl',
  'webFetchCard.copyText',
  'webFetchCard.noContent',
  'webFetchCard.actions.use_browser',
] as const

const localizedPaths = [
  'accordionCard.thinking',
  'browserProgress.title',
  'browserProgress.steps.screenshot',
  'browserProgress.steps.recipe',
  'countdownCard.expired',
  'countdownCard.days',
  'countdownCard.hours',
  'countdownCard.minutes',
  'countdownCard.seconds',
  'execCard.outputUnavailable',
  'execCard.running',
  'execCard.noOutput',
  'execCard.title',
  'execCard.output',
  'execCard.duration',
  'execCard.exitCode',
  'execCard.outputTruncated',
  'execCard.riskLabel',
  'execCard.command',
  'execCard.commandHidden',
  'execCard.noCommand',
  'execCard.hideCommand',
  'execCard.showCommand',
  'execCard.copied',
  'execCard.copyCommand',
  'execCard.collapse',
  'execCard.expand',
  'mapCard.openInMaps',
  'mermaid.copyCode',
  'search.summaryTitle',
  'search.resultCount',
  'search.moreResults',
  'search.emptyState',
  'search.partialState',
  'speech.convertTask.task',
  'speech.convertTask.cancelling',
  'speech.convertTask.downloadAudio',
  'speech.convertTask.downloadVideo',
  'speech.convertTask.downloadPdf',
  'speech.convertTask.downloadText',
  'speech.convertTask.downloadFile',
  'speech.convertTask.action.convert',
  'speech.convertTask.action.merge',
  'speech.convertTask.action.split',
  'speech.convertTask.action.extractAudio',
  'speech.convertTask.action.extractFrames',
  'speech.convertTask.action.tts',
  'speech.convertTask.action.asr',
  'speech.convertTask.previewKind.file',
  'speech.convertTask.previewKind.video',
  'speech.convertTask.previewKind.image',
  'speech.convertTask.message.queued',
  'speech.convertTask.message.processing',
  'speech.convertTask.message.completed',
  'speech.convertTask.message.taskCancelled',
  'speech.convertTask.message.taskFailed',
  'speech.convertTask.message.convertCompleted',
  'speech.convertTask.message.mergeCompleted',
  'speech.convertTask.message.splitCompleted',
  'speech.convertTask.message.trimCompleted',
  'speech.convertTask.message.extractAudioCompleted',
  'speech.convertTask.message.extractFramesCompleted',
  'speech.convertTask.message.ttsCompleted',
  'speech.convertTask.message.asrCompleted',
  'uiReview.reviewing',
  'uiReview.error',
  'uiReview.title',
  'uiReview.issues',
  'uiReview.skipped',
  'uiReview.functional',
  'uiReview.accessibility',
  'uiReview.issuesTitle',
  'uiReview.showScreenshot',
  'uiReview.hideScreenshot',
  'webFetchCard.title',
  'webFetchCard.copyUrl',
  'webFetchCard.copyText',
  'webFetchCard.noContent',
  'webFetchCard.actions.use_browser',
] as const

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

describe('typeless card locale coverage', () => {
  it('exposes uncovered card copy in runtime locale merges for all 27 locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    expect(entries).toHaveLength(27)

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const localeKey = file.replace(/\.ts$/, '') as LocaleKey
      const runtimeMessages = mergeHarnessLocale(localeKey, mod.default)

      for (const path of requiredPaths) {
        const value = getPathValue(runtimeMessages, path)
        expect(typeof value, `${file} missing ${path}`).toBe('string')
        expect(String(value).trim().length, `${file} empty ${path}`).toBeGreaterThan(0)
      }
    }
  })

  it('localizes uncovered card copy outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const englishReference = mergeHarnessLocale('en-US', enUSEntry[1].default)
    const untranslated: string[] = []

    for (const [modulePath, mod] of entries) {
      const file = fileNameFromModulePath(modulePath)
      const localeKey = file.replace(/\.ts$/, '') as LocaleKey
      if (localeKey === 'en-US' || localeKey === 'en-GB') continue

      const runtimeMessages = mergeHarnessLocale(localeKey, mod.default)

      for (const path of localizedPaths) {
        if (getPathValue(runtimeMessages, path) === getPathValue(englishReference, path)) {
          untranslated.push(`${file}:${path}`)
        }
      }
    }

    expect(untranslated, `Unlocalized card copy: ${untranslated.join(', ')}`).toEqual([])
  })
})
