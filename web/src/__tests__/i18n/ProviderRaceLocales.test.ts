import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: LocaleMessages }
>

const requiredPaths = [
  'apiProxy.providerRaceTitle',
  'apiProxy.providerRaceDesc',
  'apiProxy.providerRaceEnabled',
  'apiProxy.providerRaceDisabled',
  'apiProxy.providerRaceMaxParallel',
  'apiProxy.providerRaceMinProviders',
  'apiProxy.providerRaceSinkThreshold',
  'apiProxy.providerRaceExcludeThreshold',
  'apiProxy.providerRaceCooldownRule',
] as const

const localizedPaths = [
  'apiProxy.providerRaceTitle',
  'apiProxy.providerRaceDesc',
  'apiProxy.providerRaceEnabled',
  'apiProxy.providerRaceDisabled',
  'apiProxy.providerRaceMaxParallel',
  'apiProxy.providerRaceMinProviders',
  'apiProxy.providerRaceSinkThreshold',
  'apiProxy.providerRaceExcludeThreshold',
  'apiProxy.providerRaceCooldownRule',
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

describe('provider race locale coverage', () => {
  it('exposes provider race copy in every final locale module', () => {
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

  it('localizes provider race copy outside English locales', () => {
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
