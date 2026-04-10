import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale as mergeRuntimeHarnessLocale } from './harness-locale-additions'
import type { LocaleKey } from './locale-catalog'
import { smallModelFallbackReasonCodes } from '@/utils/smallModelFallbackReason'

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

const optionalLocaleTreePaths = ['settings.smallModel.fallbackReasonLabels']

function isOptionalLocaleTreePath(path: string): boolean {
  return optionalLocaleTreePaths.some((prefix) => path === prefix || path.startsWith(`${prefix}.`))
}

function localeMessages(locale: string): LocaleMessages {
  const modulePath = Object.keys(localeModules).find((path) => path.endsWith(`/${locale}.ts`))
  if (!modulePath) {
    throw new Error(`Missing locale module for ${locale}`)
  }
  return localeModules[modulePath]!.default
}

const providerRecoveryProtectedPaths = [
  'chat.noProvider.recoverableEyebrow',
  'chat.noProvider.recoverableTitle',
  'chat.noProvider.recoverableDescription',
  'chat.noProvider.retry',
  'chat.noProvider.reviewSingle',
  'chat.providerNeedsAttention',
  'settings.externalAgents.builtinProfile',
] as const

const harnessProviderRemediationProtectedPaths = [
  'harness.group.remediationInfraProviderAuth',
  'harness.group.remediationInfraProviderQuota',
  'harness.group.remediationInfraProviderBlocked',
] as const

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

  it('keeps provider recovery copy in raw locale files without runtime-only backfills', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const runtimeMessages = resolveRuntimeMessages(locale as LocaleKey, mod.default)

      for (const path of providerRecoveryProtectedPaths) {
        const rawValue = getPathValue(mod.default, path)
        const runtimeValue = getPathValue(runtimeMessages, path)

        expect(typeof rawValue, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(
          String(rawValue).trim().length,
          `${locale} empty raw locale key ${path}`
        ).toBeGreaterThan(0)
        expect(rawValue, `${locale} should keep raw locale key ${path} aligned with runtime copy`).toBe(
          runtimeValue
        )
      }
    }
  })

  it('keeps provider recovery copy translated outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const englishReference = enUSEntry[1].default

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const path of providerRecoveryProtectedPaths) {
        const value = getPathValue(mod.default, path)
        const englishValue = getPathValue(englishReference, path)

        expect(typeof value, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(value, `${locale} should localize ${path}`).not.toBe(englishValue)
      }
    }
  })

  it('keeps harness provider remediation copy in raw locale files without runtime-only backfills', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      const runtimeMessages = resolveRuntimeMessages(locale as LocaleKey, mod.default)

      for (const path of harnessProviderRemediationProtectedPaths) {
        const rawValue = getPathValue(mod.default, path)
        const runtimeValue = getPathValue(runtimeMessages, path)

        expect(typeof rawValue, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(
          String(rawValue).trim().length,
          `${locale} empty raw locale key ${path}`
        ).toBeGreaterThan(0)
        expect(rawValue, `${locale} should keep raw locale key ${path} aligned with runtime copy`).toBe(
          runtimeValue
        )
      }
    }
  })

  it('keeps harness provider remediation copy translated outside English locales', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSEntry = entries.find(([modulePath]) => modulePath.endsWith('/en-US.ts'))

    expect(enUSEntry).toBeTruthy()
    if (!enUSEntry) {
      throw new Error('Missing en-US locale module')
    }

    const englishReference = enUSEntry[1].default

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') continue

      for (const path of harnessProviderRemediationProtectedPaths) {
        const value = getPathValue(mod.default, path)
        const englishValue = getPathValue(englishReference, path)

        expect(typeof value, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(value, `${locale} should localize ${path}`).not.toBe(englishValue)
      }
    }
  })

  it('keeps small-model fallback reason labels in English and Chinese locales', () => {
    const protectedLocales = ['en-US', 'en-GB', 'zh-CN', 'zh-TW'] as const

    for (const locale of protectedLocales) {
      const messages = localeMessages(locale)

      for (const code of smallModelFallbackReasonCodes) {
        const path = `settings.smallModel.fallbackReasonLabels.${code}`
        const value = getPathValue(messages, path)
        expect(typeof value, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(String(value).trim().length, `${locale} empty raw locale key ${path}`).toBeGreaterThan(
          0
        )
      }
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
      const missingPaths = collectMissingPaths(referenceMessages, runtimeMessages).filter(
        (missingPath) => !isOptionalLocaleTreePath(missingPath)
      )
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
      'common.online',
      'nav.cache',
      'nav.workspaceTreeDirCount',
      'channels.authToken',
      'channels.placeholderViberAuthToken',
      'chat.streamConnecting',
      'chat.streamStreaming',
      'chat.streamExecuting',
      'chat.streamRecovering',
      'chat.streamAwaitingConfirmation',
      'chat.streamInterrupted',
      'dashboard.healthy',
      'dashboard.goroutines',
      'dashboard.cards.goroutines',
      'dashboard.cards.goroutinesChart',
      'metrics.label',
      'metrics.min',
      'apiProxy.prunerBackend',
      'settings.externalAgents.status.verified',
      'speech.online',
      'speech.convertTask.previewKind.video',
      'skillStore.status.verified',
      'skillStore.status.verificationFailed',
      'skillStore.detail.openLink',
      'skillStore.marketplace.security.signals',
      'skillStore.marketplace.sources.skillhub.description',
      'skillStore.marketplace.sources.github.description',
      'skillStore.marketplace.sources.githubAwesomeSkills.description',
      'skillStore.marketplace.sources.external.description',
      'skillStore.marketplace.sources.seed.description',
      'skillStore.marketplace.sourceImport.title',
      'skillStore.marketplace.sourceImport.description',
      'skillStore.marketplace.sourceImport.typeGithubRepo',
      'skillStore.marketplace.detail.meta.upstream',
      'skillStore.marketplace.modal.highRiskWarning',
      'skillStore.marketplace.modal.highRiskInstallTitle',
      'skillStore.marketplace.modal.highRiskInstallBody',
      'skillStore.marketplace.modal.confirmForceInstall',
      'skillStore.marketplace.security.riskDetails',
      'skillStore.marketplace.security.showRiskDetails',
      'skillStore.marketplace.security.hideRiskDetails',
      'skillStore.marketplace.security.riskDetailsHint',
      'skillStore.marketplace.signals.vulnerabilities',
      'skillStore.marketplace.categories.ai_intelligence',
      'skillStore.marketplace.categories.development_tools',
      'skillStore.marketplace.categories.productivity',
      'skillStore.marketplace.categories.data_analysis',
      'skillStore.marketplace.categories.content_creation',
      'skillStore.marketplace.categories.security_compliance',
      'skillStore.marketplace.categories.communication_collaboration',
      'skillStore.marketplace.evidenceTypes.command_injection',
      'skillStore.marketplace.dynamic.permissions.network',
      'skillStore.marketplace.dynamic.valuePrefixes.matched',
      'skillStore.marketplace.dynamic.messages.commandInjectionAttemptDetected',
      'security.directoryWhitelistAlias',
      'chat.noProvider.recoverableEyebrow',
      'chat.noProvider.recoverableTitle',
      'chat.noProvider.recoverableDescription',
      'chat.noProvider.retry',
      'chat.noProvider.reviewSingle',
      'chat.providerNeedsAttention',
      'chat.deepResearchSourceTypeWeb',
      'chat.alwaysOn',
      'chat.taskKindAgent',
      'chat.routingMode.auto',
      'profile.scopeAdmin',
      'settings.externalAgents.builtinProfile',
      'cli.eta',
      'tokenEconomy.cache',
      'settings.failover.chips.healthy',
      'companion.platforms.web',
      'settings.agentcoreRunner.eyebrow',
      'settings.agentcoreRunner.title',
      'settings.agentcoreRunner.description',
      'settings.agentcoreRunner.enabled',
      'settings.agentcoreRunner.enabledHint',
      'settings.agentcoreRunner.repoUrl',
      'settings.agentcoreRunner.repoPlaceholder',
      'settings.agentcoreRunner.ref',
      'settings.agentcoreRunner.refPlaceholder',
      'settings.agentcoreRunner.refHint',
      'settings.agentcoreRunner.refLabel',
      'settings.agentcoreRunner.defaultBranchLabel',
      'settings.agentcoreRunner.mobileHint',
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
      'system.cpuCache',
      'system.heapAlloc',
      'system.info',
      'system.kernel',
      'system.goroutines',
      'system.numGoroutines',
      'system.mountPoint',
      'resultCard.labels.backend',
      'tools.names.web',
      'userdata.memory.backend',
      'users.roleAdmin',
      'users.roles.admin',
    ]

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)
      if (locale === 'en-US' || locale === 'en-GB') {
        continue
      }

      const runtimeMessages = resolveRuntimeMessages(locale, mod.default)

      for (const path of protectedPaths) {
        expect(
          getPathValue(runtimeMessages, path),
          `${locale} should translate ${path}`
        ).not.toEqual(getPathValue(referenceMessages, path))
      }
    }
  })

  it('keeps formerly overridden locale resources in the locale files themselves', () => {
    const entries = Object.entries(localeModules).sort(([a], [b]) => a.localeCompare(b))
    const enUSMessages = localeMessages('en-US')
    const allowedEnglishRawPaths = new Set([
      'chat.uiReviewHoverState',
      'chat.uiReviewHoverDescription',
      'chat.uiReviewHoverUsability',
      'chat.uiReviewPrompt',
    ])
    const protectedPaths = [
      'chat.alwaysOn',
      'chat.uiReviewShortcutTitle',
      'chat.uiReviewHoverState',
      'chat.uiReviewHoverDescription',
      'chat.uiReviewHoverUsability',
      'chat.uiReviewPrompt',
      'system.cards.memoryChart.compactSubtitle',
      'system.cards.memoryChart.awaitingSample',
      'system.cards.memoryChart.currentValue',
      'system.cards.memoryChart.chartSubtitle',
      'system.cards.memoryChart.chartCaption',
      'system.cards.info.subtitle',
      'settings.failover.errorTypes.timeout',
    ]

    for (const [modulePath, mod] of entries) {
      const locale = localeFromModulePath(modulePath)

      for (const path of protectedPaths) {
        const value = getPathValue(mod.default, path)
        expect(typeof value, `${locale} missing raw locale key ${path}`).toBe('string')
        expect(
          String(value).trim().length,
          `${locale} empty raw locale key ${path}`
        ).toBeGreaterThan(0)

        if (locale === 'en-US' || locale === 'en-GB') {
          continue
        }

        if (allowedEnglishRawPaths.has(path)) {
          continue
        }

        expect(
          value,
          `${locale} should inline translated raw locale value for ${path}`
        ).not.toEqual(getPathValue(enUSMessages, path))
      }
    }
  })
})
