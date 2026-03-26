import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const requiredPaths = [
  'chat.backgroundTasks',
  'chat.taskNotificationTitle',
  'chat.taskStagePlanning',
  'chat.taskStageWorking',
  'chat.taskStageVerifying',
  'chat.taskStageWaiting',
  'chat.taskStageCompleted',
  'chat.taskStageFailed',
  'chat.taskStageCancelled',
  'chat.taskKindResearch',
  'chat.taskKindAgent',
  'chat.taskOpenConversation',
  'chat.taskBackToConversation',
  'chat.taskCancel',
  'chat.taskSendUpdate',
  'chat.taskSendUpdatePlaceholder',
  'chat.taskRunningElsewhere',
  'chat.taskCompleted',
  'chat.taskFailed',
  'chat.taskCancelled',
  'chat.taskWaitingForApproval',
  'chat.taskWaitingForAnswer',
  'chat.taskDefaultResearchTitle',
  'chat.taskDefaultAgentTitle',
  'chat.taskRuntimePending',
  'chat.taskRuntimeIntake',
  'chat.taskRuntimeClarify',
  'chat.taskRuntimePlan',
  'chat.taskRuntimeConfirmGate',
  'chat.taskRuntimeExecute',
  'chat.taskRuntimeVerify',
  'chat.taskRuntimeReflect',
  'chat.taskRuntimeReport',
  'chat.taskRuntimeRecover',
  'chat.taskRuntimeDone',
  'chat.taskRuntimeAborted',
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

describe('task projection locale coverage', () => {
  it('declares task projection chat keys in all 27 locale source files', () => {
    const sources = new Map(
      Object.entries(localeSourceModules).map(([modulePath, source]) => [
        fileNameFromModulePath(modulePath),
        source,
      ])
    )

    expect(sources.size).toBe(27)

    for (const [fileName, source] of sources) {
      for (const path of requiredPaths) {
        const leaf = path.split('.').pop() as string
        expect(source, `${fileName} should explicitly declare ${path}`).toMatch(
          new RegExp(`"${leaf}"\\s*:`)
        )
      }
    }
  })

  it('exposes task projection chat keys in every merged locale module', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    expect(messagesByFile.size).toBe(27)

    for (const [fileName, messages] of messagesByFile) {
      for (const path of requiredPaths) {
        const value = getPathValue(messages, path)
        expect(typeof value, `${fileName} should expose ${path}`).toBe('string')
        expect(
          String(value).trim().length,
          `${fileName} should not leave ${path} empty`
        ).toBeGreaterThan(0)
      }
    }
  })
})
