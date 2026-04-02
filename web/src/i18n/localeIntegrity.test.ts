import { describe, expect, it } from 'vitest'

type LocaleMessages = Record<string, unknown>

const localeModules = import.meta.glob<{ default: LocaleMessages }>('./locales/*.ts', {
  eager: true,
})

const localeSourceModules = import.meta.glob('./locales/*.ts', {
  eager: true,
  query: '?raw',
  import: 'default',
}) as Record<string, string>

function localeFromModulePath(modulePath: string): string {
  return modulePath.split('/').pop()?.replace(/\.ts$/, '') ?? modulePath
}

function isPlainObject(value: unknown): value is LocaleMessages {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function collectMissingPaths(reference: unknown, candidate: unknown, prefix = ''): string[] {
  if (Array.isArray(reference)) {
    return Array.isArray(candidate) ? [] : [prefix]
  }

  if (isPlainObject(reference)) {
    if (!isPlainObject(candidate)) {
      return [prefix || '<root>']
    }

    return Object.entries(reference).flatMap(([key, value]) =>
      collectMissingPaths(value, candidate[key], prefix ? `${prefix}.${key}` : key)
    )
  }

  return typeof candidate === 'undefined' ? [prefix] : []
}

function collectTypeMismatches(reference: unknown, candidate: unknown, prefix = ''): string[] {
  if (typeof candidate === 'undefined') {
    return []
  }

  if (Array.isArray(reference)) {
    return Array.isArray(candidate) ? [] : [prefix]
  }

  if (isPlainObject(reference)) {
    if (!isPlainObject(candidate)) {
      return [prefix || '<root>']
    }

    return Object.entries(reference).flatMap(([key, value]) =>
      collectTypeMismatches(value, candidate[key], prefix ? `${prefix}.${key}` : key)
    )
  }

  return typeof candidate === typeof reference ? [] : [prefix]
}

function getPathValue(root: unknown, path: string): unknown {
  return path
    .split('.')
    .reduce<unknown>(
      (value, segment) =>
        isPlainObject(value) || Array.isArray(value)
          ? (value as Record<string, unknown>)[segment]
          : undefined,
      root
    )
}

describe('locale integrity', () => {
  it('keeps the full 27-locale set', () => {
    expect(Object.keys(localeModules)).toHaveLength(27)
    expect(Object.keys(localeSourceModules)).toHaveLength(27)
  })

  it('keeps every locale file self-contained', () => {
    for (const [modulePath, source] of Object.entries(localeSourceModules)) {
      const locale = localeFromModulePath(modulePath)
      expect(source, `${locale} should not import en-US at runtime`).not.toMatch(
        /from\s+['"]\.\/en-US['"]/
      )
      expect(source, `${locale} should not import harness locale overrides`).not.toMatch(
        /harness-locale-overrides/
      )
      expect(source, `${locale} should not import browser monitor overlays`).not.toMatch(
        /browser-monitor-locales/
      )
    }
  })

  it('covers the en-US key tree in every locale module', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = enUSEntry[1].default

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const missingPaths = collectMissingPaths(referenceMessages, mod.default)
      const typeMismatches = collectTypeMismatches(referenceMessages, mod.default)

      expect(missingPaths, `${locale} is missing locale keys`).toEqual([])
      expect(typeMismatches, `${locale} has locale type mismatches`).toEqual([])
    }
  })

  it('keeps key UI status terms translated outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const referenceMessages = enUSEntry[1].default
    const protectedPaths = [
      'common.backToTop',
      'chat.streamConnecting',
      'chat.streamStreaming',
      'chat.streamExecuting',
      'chat.streamRecovering',
      'chat.streamAwaitingConfirmation',
      'chat.streamInterrupted',
      'dashboard.healthy',
      'metrics.label',
      'settings.externalAgents.status.verified',
      'skillStore.status.verified',
      'skillStore.status.verificationFailed',
      'skillStore.detail.openLink',
      'skillStore.marketplace.security.signals',
      'skillStore.marketplace.sources.skillhub.description',
      'skillStore.marketplace.sources.github.description',
      'skillStore.marketplace.sources.external.description',
      'skillStore.marketplace.sources.seed.description',
      'skillStore.marketplace.evidenceTypes.command_injection',
      'skillStore.marketplace.dynamic.permissions.network',
      'skillStore.marketplace.dynamic.valuePrefixes.matched',
      'skillStore.marketplace.dynamic.messages.commandInjectionAttemptDetected',
    ]

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') {
        continue
      }

      for (const path of protectedPaths) {
        expect(getPathValue(mod.default, path), `${locale} should translate ${path}`).not.toEqual(
          getPathValue(referenceMessages, path)
        )
      }
    }
  })
})
