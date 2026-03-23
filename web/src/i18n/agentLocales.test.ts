import { describe, expect, it } from 'vitest'

type AgentMessages = {
  mode: string
  modeDescription: string
  autoConfirm: string
  autoConfirmDescription: string
}

type LocaleMessages = {
  agent?: Partial<AgentMessages>
}

const requiredKeys = ['mode', 'modeDescription', 'autoConfirm', 'autoConfirmDescription'] as const

function fileNameFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop() ?? modulePath
}

const localeModules = import.meta.glob<{ default: LocaleMessages }>('./locales/*.ts', {
  eager: true,
})
const localeSourceModules = import.meta.glob('./locales/*.ts', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>
const localeFiles = Object.keys(localeModules).map(fileNameFromModulePath).sort()
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

describe('agent locale coverage', () => {
  it('is explicitly declared in every locale source file', () => {
    for (const file of localeFiles) {
      const source = localeSourceByFile.get(file)
      expect(source, `${file} should be loadable as raw source`).toBeTruthy()
      expect(source, `${file} should declare agent`).toMatch(/agent:\s*\{/)
      for (const key of requiredKeys) {
        expect(source, `${file} should include agent.${key}`).toMatch(new RegExp(`\\b${key}:`))
      }
    }
  })

  it('exposes all required runtime strings in every locale', () => {
    expect(localeFiles.length).toBe(27)

    for (const file of localeFiles) {
      const messages = localeMessagesByFile.get(file)
      expect(messages, `${file} should be loadable via import.meta.glob`).toBeTruthy()

      const section = messages?.agent as Partial<AgentMessages> | undefined
      expect(section, `${file} should expose agent`).toBeTruthy()

      for (const key of requiredKeys) {
        const value = section?.[key]
        expect(typeof value, `${file} should expose a string for agent.${key}`).toBe('string')
        expect(
          String(value).trim().length,
          `${file} should expose a non-empty agent.${key}`
        ).toBeGreaterThan(0)
      }
    }
  })
})
