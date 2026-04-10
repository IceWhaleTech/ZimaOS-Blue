import { describe, expect, it } from 'vitest'
import { mergeHarnessLocale } from './harness-locale-additions'
import { localeKeys, type LocaleKey } from './locale-catalog'

type LocaleNode = Record<string, unknown>

const graphPaths = [
  'graphNodeCount',
  'graphEdgeCount',
  'graphLegendReference',
  'graphLegendConflict',
  'graphLegendSuperseded',
  'graphEmpty',
  'graphFocus',
  'graphZoomIn',
  'graphZoomOut',
  'graphReset',
  'graphPanHint',
  'graphLayer',
  'graphLayerIdle',
  'graphLayerNear',
  'graphLayerMid',
  'graphLayerFar',
  'graphLayerFull',
  'graphTooltipLinks',
] as const

function isPlainObject(value: unknown): value is LocaleNode {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function getKnowledgeMessages(localeKey: LocaleKey): LocaleNode {
  const merged = mergeHarnessLocale<LocaleNode>(localeKey, {})
  const knowledge = merged.knowledge

  expect(isPlainObject(knowledge), `${localeKey} should expose knowledge as an object`).toBe(true)

  return knowledge as LocaleNode
}

describe('knowledge graph locale coverage', () => {
  it('exposes all graph copy keys for every locale', () => {
    expect(localeKeys).toHaveLength(27)

    for (const localeKey of localeKeys) {
      const knowledge = getKnowledgeMessages(localeKey)

      for (const path of graphPaths) {
        const value = knowledge[path]
        expect(typeof value, `${localeKey} should expose knowledge.${path}`).toBe('string')
        expect(String(value).trim().length, `${localeKey} should localize knowledge.${path}`).toBeGreaterThan(0)
      }
    }
  })
})
