import { describe, expect, it } from 'vitest'

type AgentReflectionMessages = {
  title: string
  description: string
  hint: string
  saved: string
  saveFailed: string
}

type LocaleMessages = {
  settings?: {
    agentReflection?: Partial<AgentReflectionMessages>
  }
}

const requiredKeys = ['title', 'description', 'hint', 'saved', 'saveFailed'] as const

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

describe('settings.agentReflection locale coverage', () => {
  it('is explicitly declared in every locale source file', () => {
    for (const file of localeFiles) {
      const source = localeSourceByFile.get(file)
      expect(source, `${file} should be loadable as raw source`).toBeTruthy()
      expect(source, `${file} should declare settings.agentReflection`).toMatch(
        /agentReflection:\s*\{/
      )
      for (const key of requiredKeys) {
        expect(source, `${file} should include agentReflection.${key}`).toMatch(
          new RegExp(`\\b${key}:`)
        )
      }
    }
  })

  it('exposes all required runtime strings in every locale', () => {
    expect(localeFiles.length).toBeGreaterThan(0)

    for (const file of localeFiles) {
      const messages = localeMessagesByFile.get(file)
      expect(messages, `${file} should be loadable via import.meta.glob`).toBeTruthy()

      const section = messages?.settings?.agentReflection as
        | Partial<AgentReflectionMessages>
        | undefined
      expect(section, `${file} should expose settings.agentReflection`).toBeTruthy()

      for (const key of requiredKeys) {
        const value = section?.[key]
        expect(typeof value, `${file} should expose a string for agentReflection.${key}`).toBe(
          'string'
        )
        expect(
          String(value).trim().length,
          `${file} should expose a non-empty agentReflection.${key}`
        ).toBeGreaterThan(0)
      }
    }
  })
})
