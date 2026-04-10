import { describe, expect, it } from 'vitest'
import { agentcoreRunnerChatOverrides } from './agentcore-runner-chat-locales'
import { agentcoreRunnerPartDescriptionOverrides } from './agentcore-runner-part-descriptions'
import { agentcoreRunnerPartSettingsOverrides } from './agentcore-runner-part-settings'
import { mergeHarnessLocale } from './harness-locale-additions'
import { localeKeys, type LocaleKey } from './locale-catalog'

type LocaleNode = Record<string, unknown>

function isPlainObject(value: unknown): value is LocaleNode {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function getAgentcoreRunnerMessages(localeKey: LocaleKey): LocaleNode {
  const merged = mergeHarnessLocale<LocaleNode>(localeKey, {})
  const settings = isPlainObject(merged.settings) ? merged.settings : {}
  const agentcoreRunner = settings.agentcoreRunner

  expect(
    isPlainObject(agentcoreRunner),
    `${localeKey} should expose settings.agentcoreRunner as an object`
  ).toBe(true)

  return agentcoreRunner as LocaleNode
}

function collectLeafPaths(node: LocaleNode, prefix = ''): string[] {
  const paths: string[] = []

  for (const [key, value] of Object.entries(node)) {
    const nextPath = prefix ? `${prefix}.${key}` : key
    if (isPlainObject(value)) {
      paths.push(...collectLeafPaths(value, nextPath))
      continue
    }
    paths.push(nextPath)
  }

  return paths
}

function getValueAtPath(node: LocaleNode, path: string): unknown {
  let current: unknown = node

  for (const segment of path.split('.')) {
    if (!isPlainObject(current) || !(segment in current)) {
      return undefined
    }
    current = current[segment]
  }

  return current
}

describe('agentcore runner locale coverage', () => {
  it('declares part override tables for every non-base locale', () => {
    const expectedOverrides = localeKeys.filter((localeKey) => localeKey !== 'en-US').sort()

    expect(Object.keys(agentcoreRunnerChatOverrides).sort()).toEqual(expectedOverrides)
    expect(Object.keys(agentcoreRunnerPartSettingsOverrides).sort()).toEqual(expectedOverrides)
    expect(Object.keys(agentcoreRunnerPartDescriptionOverrides).sort()).toEqual(expectedOverrides)
  })

  it('declares localized chat selector copy for every non-base locale', () => {
    for (const localeKey of localeKeys) {
      if (localeKey === 'en-US') continue

      const override = agentcoreRunnerChatOverrides[localeKey]
      expect(
        typeof override?.refLabel,
        `${localeKey} should override settings.agentcoreRunner.refLabel`
      ).toBe('string')
      expect(
        typeof override?.defaultBranchLabel,
        `${localeKey} should override settings.agentcoreRunner.defaultBranchLabel`
      ).toBe('string')
      expect(
        typeof override?.mobileHint,
        `${localeKey} should override settings.agentcoreRunner.mobileHint`
      ).toBe('string')
      expect(String(override?.refLabel).trim().length).toBeGreaterThan(0)
      expect(String(override?.defaultBranchLabel).trim().length).toBeGreaterThan(0)
      expect(String(override?.mobileHint).trim().length).toBeGreaterThan(0)
    }
  })

  it('exposes non-empty runtime strings for all 27 locales', () => {
    expect(localeKeys).toHaveLength(27)

    const baseMessages = getAgentcoreRunnerMessages('en-US')
    const leafPaths = collectLeafPaths(baseMessages)

    expect(leafPaths.length).toBeGreaterThan(0)

    for (const localeKey of localeKeys) {
      const localeMessages = getAgentcoreRunnerMessages(localeKey)

      for (const path of leafPaths) {
        const value = getValueAtPath(localeMessages, path)
        expect(
          typeof value,
          `${localeKey} should expose a string for settings.agentcoreRunner.${path}`
        ).toBe('string')
        expect(
          String(value).trim().length,
          `${localeKey} should expose a non-empty string for settings.agentcoreRunner.${path}`
        ).toBeGreaterThan(0)
      }
    }
  })

  it('uses british english wording for en-GB-specific agentcore runner strings', () => {
    const messages = getAgentcoreRunnerMessages('en-GB')

    expect(messages.sourceOptimization).toBe('Source optimisation')
    expect(messages.activePartTooltip).toBe('This part was optimised in the current candidate')
    expect(messages.lastOptimizationRunId).toBe('Last optimisation run ID')
    expect(String(messages.description)).toContain('optimisation')
  })
})
