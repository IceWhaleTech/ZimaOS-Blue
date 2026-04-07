import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

const requiredPaths = [
  'chat.processTrace.fields.message',
  'chat.processTrace.fields.provider',
  'chat.processTrace.fields.model',
  'chat.processTrace.fields.attachments',
  'chat.processTrace.fields.file',
  'chat.processTrace.fields.attempt',
  'chat.processTrace.fields.delay',
  'chat.processTrace.fields.format',
  'chat.processTrace.fields.size',
  'chat.processTrace.fields.duration',
  'chat.processTrace.fields.upload',
  'chat.processTrace.fields.transcript',
  'chat.processTrace.fields.mode',
  'chat.processTrace.fields.conversation',
  'chat.processTrace.summaryValues.continuePreviousReply',
  'chat.processTrace.summaryValues.resumePreviousRequest',
  'chat.processTrace.summaryValues.interruptCurrentReply',
  'chat.processTrace.summaryValues.sendNewRequest',
  'chat.processTrace.events.requestReady',
  'chat.processTrace.events.requestSent',
  'chat.processTrace.events.waitingForResponse',
  'chat.processTrace.events.recoveringResponse',
  'chat.processTrace.details.requestDispatched',
  'chat.processTrace.details.waitingForResponse',
  'chat.processTrace.details.recoveryStage1',
  'chat.processTrace.details.recoveryStage2',
  'chat.processTrace.details.voiceInterruptDispatched',
  'chat.processTrace.details.voiceRequestDispatched',
  'chat.processTrace.details.voiceWaitingForResponse',
  'chat.processTrace.details.bargeIn',
  'chat.processTrace.details.transcriptionFailed',
] as const

const localizedPaths = [
  'chat.processTrace.fields.provider',
  'chat.processTrace.summaryValues.continuePreviousReply',
  'chat.processTrace.summaryValues.sendNewRequest',
  'chat.processTrace.events.requestReady',
  'chat.processTrace.events.requestSent',
  'chat.processTrace.events.waitingForResponse',
  'chat.processTrace.events.recoveringResponse',
  'chat.processTrace.details.requestDispatched',
  'chat.processTrace.details.waitingForResponse',
  'chat.processTrace.details.recoveryStage2',
  'chat.processTrace.details.voiceRequestDispatched',
  'chat.processTrace.details.voiceWaitingForResponse',
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

describe('process trace locale coverage', () => {
  it('exposes visible process trace copy in every final locale module', () => {
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

  it('localizes visible process trace copy outside English locales', () => {
    const messagesByFile = new Map(
      Object.entries(localeModules).map(([modulePath, mod]) => [
        fileNameFromModulePath(modulePath),
        mod.default,
      ])
    )

    const englishReference = messagesByFile.get('en-US.ts')
    expect(englishReference).toBeTruthy()
    if (!englishReference) {
      throw new Error('Missing en-US locale module')
    }

    for (const [fileName, messages] of messagesByFile) {
      if (fileName === 'en-US.ts' || fileName === 'en-GB.ts') continue

      for (const path of localizedPaths) {
        expect(getPathValue(messages, path), `${fileName} should localize ${path}`).not.toBe(
          getPathValue(englishReference, path)
        )
      }
    }
  })
})
