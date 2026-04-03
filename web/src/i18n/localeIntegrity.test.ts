import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale as mergeRuntimeHarnessLocale } from './harnessLocaleAdditions'
import type { LocaleKey } from './locale-catalog'

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

function resolveRuntimeMessages(locale: string, messages: LocaleMessages): LocaleMessages {
  return mergeRuntimeHarnessLocale(locale as LocaleKey, messages)
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

    const referenceLocale = localeFromModulePath(enUSEntry[0])
    const referenceMessages = resolveRuntimeMessages(referenceLocale, enUSEntry[1].default)

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const runtimeMessages = resolveRuntimeMessages(locale, mod.default)
      const missingPaths = collectMissingPaths(referenceMessages, runtimeMessages)
      const typeMismatches = collectTypeMismatches(referenceMessages, runtimeMessages)

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

    const referenceLocale = localeFromModulePath(enUSEntry[0])
    const referenceMessages = resolveRuntimeMessages(referenceLocale, enUSEntry[1].default)
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
      'skillStore.marketplace.modal.highRiskWarning',
      'skillStore.marketplace.modal.highRiskInstallTitle',
      'skillStore.marketplace.modal.highRiskInstallBody',
      'skillStore.marketplace.modal.confirmForceInstall',
      'skillStore.marketplace.evidenceTypes.command_injection',
      'skillStore.marketplace.dynamic.permissions.network',
      'skillStore.marketplace.dynamic.valuePrefixes.matched',
      'skillStore.marketplace.dynamic.messages.commandInjectionAttemptDetected',
      'settings.agentcoreRunner.eyebrow',
      'settings.agentcoreRunner.title',
      'settings.agentcoreRunner.description',
      'settings.agentcoreRunner.enabled',
      'settings.agentcoreRunner.enabledHint',
      'settings.agentcoreRunner.repoUrl',
      'settings.agentcoreRunner.repoPlaceholder',
      'settings.agentcoreRunner.ref',
      'settings.agentcoreRunner.refPlaceholder',
      'settings.agentcoreRunner.prepareHint',
      'settings.agentcoreRunner.prepare',
      'settings.agentcoreRunner.preparing',
      'settings.agentcoreRunner.prepareSuccess',
      'settings.agentcoreRunner.prepareFailed',
      'settings.agentcoreRunner.status',
      'settings.agentcoreRunner.resolvedCommit',
      'settings.agentcoreRunner.requiredGoVersion',
      'settings.agentcoreRunner.installedGoVersion',
      'settings.agentcoreRunner.toolchainReady',
      'settings.agentcoreRunner.binaryReady',
      'settings.agentcoreRunner.lastPrepareState',
      'settings.agentcoreRunner.binaryPath',
      'settings.agentcoreRunner.binaryChecksum',
      'settings.agentcoreRunner.lastPrepareAt',
      'settings.agentcoreRunner.lastOptimizationRunId',
      'settings.agentcoreRunner.lastError',
      'settings.agentcoreRunner.empty',
    ]

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') {
        continue
      }

      const runtimeMessages = resolveRuntimeMessages(locale, mod.default)

      for (const path of protectedPaths) {
        expect(getPathValue(runtimeMessages, path), `${locale} should translate ${path}`).not.toEqual(
          getPathValue(referenceMessages, path)
        )
      }
    }
  })
})
