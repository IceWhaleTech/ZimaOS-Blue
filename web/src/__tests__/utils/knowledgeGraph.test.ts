import { describe, expect, it } from 'vitest'

import type { KnowledgePageSummary } from '@/api/knowledge'
import {
  buildKnowledgeGraphLayout,
  getKnowledgeGraphOrbitGuide,
  isKnowledgeGraphPointInsideEnvelope,
  pickKnowledgeGraphDefaultFocusSlug,
} from '@/utils/knowledgeGraph'

const pages: KnowledgePageSummary[] = [
  {
    title: 'Center',
    slug: 'center',
    page_type: 'source_summary',
    summary: 'Center node.',
    source_refs: ['CENTER.md'],
    keywords: ['center'],
    backlinks: ['near-a', 'near-b'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-1',
    status: 'active',
    confidence: 'high',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Near A',
    slug: 'near-a',
    page_type: 'concept',
    summary: 'Near node.',
    source_refs: ['A.md'],
    keywords: ['a'],
    backlinks: ['center', 'far'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-2',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Near B',
    slug: 'near-b',
    page_type: 'concept',
    summary: 'Near node.',
    source_refs: ['B.md'],
    keywords: ['b'],
    backlinks: ['center'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-3',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
  {
    title: 'Far',
    slug: 'far',
    page_type: 'concept',
    summary: 'Far node.',
    source_refs: ['FAR.md'],
    keywords: ['far'],
    backlinks: ['near-a'],
    generated_at: '2026-04-05T12:00:00Z',
    updated_at: '2026-04-05T12:00:00Z',
    source_hash: 'hash-4',
    status: 'active',
    confidence: 'medium',
    conflicts_with: [],
    superseded_by: [],
    derived_from_query: '',
  },
]

function distanceFromCenter(x: number, y: number, width: number, height: number) {
  const dx = x - width / 2
  const dy = y - height / 2
  return Math.hypot(dx, dy)
}

describe('buildKnowledgeGraphLayout', () => {
  it('uses taller orbital rings so first-hop nodes stay closer than second-hop nodes', () => {
    const graph = buildKnowledgeGraphLayout(pages, 'center')

    expect(graph.height).toBeGreaterThan(560)

    const center = graph.nodes.find((node) => node.slug === 'center')
    const nearA = graph.nodes.find((node) => node.slug === 'near-a')
    const nearB = graph.nodes.find((node) => node.slug === 'near-b')
    const far = graph.nodes.find((node) => node.slug === 'far')

    expect(center).toBeTruthy()
    expect(nearA).toBeTruthy()
    expect(nearB).toBeTruthy()
    expect(far).toBeTruthy()

    const centerDistance = distanceFromCenter(center!.x, center!.y, graph.width, graph.height)
    const nearADistance = distanceFromCenter(nearA!.x, nearA!.y, graph.width, graph.height)
    const nearBDistance = distanceFromCenter(nearB!.x, nearB!.y, graph.width, graph.height)
    const farDistance = distanceFromCenter(far!.x, far!.y, graph.width, graph.height)

    expect(centerDistance).toBeLessThan(2)
    expect(nearADistance).toBeLessThan(farDistance)
    expect(nearBDistance).toBeLessThan(farDistance)
    expect(nearADistance).toBeLessThan(nearBDistance)
  })

  it('returns wider orbit guides for deeper layers', () => {
    const near = getKnowledgeGraphOrbitGuide(1)
    const mid = getKnowledgeGraphOrbitGuide(2)
    const far = getKnowledgeGraphOrbitGuide(3)
    const detached = getKnowledgeGraphOrbitGuide(0, true)

    expect(near.radiusX).toBeLessThan(mid.radiusX)
    expect(mid.radiusY).toBeLessThan(far.radiusY)
    expect(far.radiusX).toBeLessThan(detached.radiusX)
  })

  it('keeps orbit and detached nodes inside an ellipse-shaped envelope', () => {
    const expansivePages: KnowledgePageSummary[] = [
      ...pages,
      ...Array.from({ length: 12 }, (_, index) => ({
        title: `Detached ${index + 1}`,
        slug: `detached-${index + 1}`,
        page_type: 'concept',
        summary: 'Detached node.',
        source_refs: [`DETACHED-${index + 1}.md`],
        keywords: ['detached'],
        backlinks: [],
        generated_at: '2026-04-05T12:00:00Z',
        updated_at: '2026-04-05T12:00:00Z',
        source_hash: `hash-detached-${index + 1}`,
        status: 'active',
        confidence: 'medium',
        conflicts_with: [],
        superseded_by: [],
        derived_from_query: '',
      })),
    ]

    const graph = buildKnowledgeGraphLayout(expansivePages, 'center')

    for (const node of graph.nodes) {
      expect(isKnowledgeGraphPointInsideEnvelope(node.x, node.y, node.radius)).toBe(true)
    }
  })

  it('prefers the most connected node as the default focus and shrinks detached nodes', () => {
    const expansivePages: KnowledgePageSummary[] = [
      ...pages,
      {
        title: 'Detached',
        slug: 'detached',
        page_type: 'concept',
        summary: 'Detached node.',
        source_refs: ['DETACHED.md'],
        keywords: ['detached'],
        backlinks: [],
        generated_at: '2026-04-05T12:00:00Z',
        updated_at: '2026-04-05T12:00:00Z',
        source_hash: 'hash-detached',
        status: 'active',
        confidence: 'medium',
        conflicts_with: [],
        superseded_by: [],
        derived_from_query: '',
      },
    ]

    expect(pickKnowledgeGraphDefaultFocusSlug(expansivePages)).toBe('center')

    const graph = buildKnowledgeGraphLayout(expansivePages, 'center')
    const center = graph.nodes.find((node) => node.slug === 'center')
    const detached = graph.nodes.find((node) => node.slug === 'detached')

    expect(center).toBeTruthy()
    expect(detached).toBeTruthy()
    expect(detached!.radius).toBeLessThan(center!.radius)
    expect(
      distanceFromCenter(detached!.x, detached!.y, graph.width, graph.height)
    ).toBeGreaterThan(distanceFromCenter(center!.x, center!.y, graph.width, graph.height))
  })
})
